package sync

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/scanner"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// scanFiles handles Phase 2: Scanning
func (e *Engine) scanFiles(ctx context.Context, req *SyncRequest, smbClient *smb.SMBClient) (
	localFiles map[string]*cache.FileInfo,
	remoteFiles map[string]*cache.FileInfo,
	cachedFiles map[string]*cache.FileInfo,
	err error,
) {
	// Scan local files
	e.logger.Info("scanning local files", zap.String("path", req.LocalPath))
	scanResult, err := e.scanner.Scan(ctx, scanner.ScanRequest{
		JobID:      req.JobID,
		BasePath:   req.LocalPath,
		RemoteBase: req.RemotePath,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("local scan failed: %w", err)
	}

	// Convert scan result to FileInfo map (using relative paths for comparison with remote)
	localFiles = make(map[string]*cache.FileInfo)
	localBasePath := filepath.Clean(req.LocalPath)
	for _, file := range scanResult.NewFiles {
		relPath := toRelativePath(file.LocalPath, localBasePath)
		localFiles[relPath] = &cache.FileInfo{
			Path:  relPath,
			Size:  file.Size,
			MTime: file.MTime,
			Hash:  file.Hash,
		}
	}
	for _, file := range scanResult.ModifiedFiles {
		relPath := toRelativePath(file.LocalPath, localBasePath)
		localFiles[relPath] = &cache.FileInfo{
			Path:  relPath,
			Size:  file.Size,
			MTime: file.MTime,
			Hash:  file.Hash,
		}
	}
	for _, file := range scanResult.UnchangedFiles {
		relPath := toRelativePath(file.LocalPath, localBasePath)
		localFiles[relPath] = &cache.FileInfo{
			Path:  relPath,
			Size:  file.Size,
			MTime: file.MTime,
			Hash:  file.Hash,
		}
	}

	e.logger.Info("local scan completed",
		zap.Int("files", len(localFiles)),
	)

	// Scan remote files
	// Note: We always scan remote even in upload-only mode, because we need
	// remote state information to detect deletions (ActionDeleteRemote).
	// The filtering of downloads happens later in filterDecisionsByMode.
	var usedManifest bool
	e.logger.Info("scanning remote files", zap.String("path", req.RemotePath))
	remoteFiles, usedManifest, err = e.scanRemote(ctx, smbClient, req.RemotePath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("remote scan failed: %w", err)
	}
	e.logger.Info("remote scan completed",
		zap.Int("files", len(remoteFiles)),
		zap.Bool("used_manifest", usedManifest),
	)

	// Load cached state
	cachedFiles, err = e.cache.GetAllCachedFiles(req.JobID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load cache: %w", err)
	}

	e.logger.Info("cache loaded",
		zap.Int("files", len(cachedFiles)),
	)

	// Fallback SMB check: if we used manifest, verify cached files not in manifest
	// This handles the case where manifest hasn't been updated yet after an upload
	if usedManifest && len(cachedFiles) > 0 {
		fallbackCount := e.verifyCachedFilesViaSMB(ctx, smbClient, req.RemotePath, cachedFiles, remoteFiles)
		if fallbackCount > 0 {
			e.logger.Info("SMB fallback verification completed",
				zap.Int("files_verified", fallbackCount),
			)
		}
	}

	return localFiles, remoteFiles, cachedFiles, nil
}

// verifyCachedFilesViaSMB checks files that are in cache but not in remoteFiles via direct SMB.
// This handles the case where the Anemone manifest hasn't been updated yet.
// Returns the number of files verified and added to remoteFiles.
func (e *Engine) verifyCachedFilesViaSMB(ctx context.Context, smbClient *smb.SMBClient,
	remotePath string, cachedFiles, remoteFiles map[string]*cache.FileInfo) int {

	// Find files in cache that are not in remoteFiles (manifest)
	var missingFiles []string
	for path := range cachedFiles {
		if _, exists := remoteFiles[path]; !exists {
			missingFiles = append(missingFiles, path)
		}
	}

	if len(missingFiles) == 0 {
		return 0
	}

	e.logger.Debug("checking cached files not in manifest via SMB",
		zap.Int("count", len(missingFiles)),
	)

	// Extract relative path prefix from UNC path
	_, _, relPathPrefix := parseUNCPath(remotePath)

	verified := 0
	for _, filePath := range missingFiles {
		// Check context cancellation
		select {
		case <-ctx.Done():
			e.logger.Debug("SMB fallback cancelled", zap.Int("verified", verified))
			return verified
		default:
		}

		// Build full remote path relative to share
		smbPath := filePath
		if relPathPrefix != "" && relPathPrefix != "." {
			smbPath = relPathPrefix + "/" + filePath
		}

		// Try to get metadata via SMB
		metadata, err := smbClient.GetMetadata(smbPath)
		if err != nil {
			// File doesn't exist on remote - this is a real deletion
			e.logger.Debug("cached file not found on remote (deleted)",
				zap.String("path", filePath),
			)
			continue
		}

		// File exists on remote but not in manifest - add to remoteFiles
		e.logger.Debug("cached file found via SMB fallback",
			zap.String("path", filePath),
			zap.Int64("size", metadata.Size),
		)

		remoteFiles[filePath] = &cache.FileInfo{
			Path:  filePath,
			Size:  metadata.Size,
			MTime: metadata.ModTime,
			Hash:  "", // No hash from SMB metadata, will rely on size/mtime
		}
		verified++
	}

	return verified
}

// scanRemote scans remote files using Anemone manifest if available, otherwise falls back to SMB scan.
// Returns the remote files map, a bool indicating if manifest was used, and any error.
func (e *Engine) scanRemote(ctx context.Context, smbClient *smb.SMBClient, basePath string) (map[string]*cache.FileInfo, bool, error) {
	// Extract relative path from UNC path (ListRemote expects path relative to share)
	// basePath is UNC format: \\server\share\path -> we need just "path" (or "." for root)
	_, _, relPath := parseUNCPath(basePath)
	if relPath == "" {
		relPath = "." // Use "." for share root
	}

	e.logger.Debug("scanning remote with relative path",
		zap.String("unc_path", basePath),
		zap.String("relative_path", relPath),
	)

	// Try to load Anemone manifest first (much faster)
	manifestReader := NewManifestReader(smbClient, e.logger.Named("manifest"))
	manifestResult := manifestReader.ReadManifest(ctx, relPath)

	if manifestResult.Found && manifestResult.Error == nil {
		// Manifest found - use it directly
		e.logger.Info("using Anemone manifest for remote scan",
			zap.String("share", manifestResult.Manifest.ShareName),
			zap.Int("file_count", manifestResult.Manifest.FileCount),
			zap.Int64("total_size", manifestResult.Manifest.TotalSize),
			zap.Duration("duration", manifestResult.Duration),
		)
		return manifestResult.Manifest.ToFileInfoMap(), true, nil
	}

	if manifestResult.Error != nil {
		e.logger.Warn("failed to read manifest, falling back to SMB scan",
			zap.Error(manifestResult.Error),
		)
	} else {
		e.logger.Info("manifest not found, using SMB scan (slower)",
			zap.String("hint", "Install Anemone Server for faster sync"),
		)
	}

	// Fallback to traditional SMB recursive scan
	files, err := e.scanRemoteSMB(ctx, smbClient, relPath)
	return files, false, err
}

// scanRemoteSMB scans remote files recursively using SMB (fallback method).
func (e *Engine) scanRemoteSMB(ctx context.Context, smbClient *smb.SMBClient, relPath string) (map[string]*cache.FileInfo, error) {
	// Create progress callback for remote scanning
	progressCallback := func(progress RemoteScanProgress) {
		e.logger.Debug("remote scan progress",
			zap.Int("files", progress.FilesFound),
			zap.Int("dirs", progress.DirsScanned),
			zap.Int64("bytes", progress.BytesDiscovered),
			zap.String("current_dir", progress.CurrentDir),
		)
	}

	// Create remote scanner
	scanner := NewRemoteScanner(smbClient, e.logger.Named("remote_scanner"), progressCallback)

	// Perform scan with relative path (not full UNC path)
	result, err := scanner.Scan(ctx, relPath)
	if err != nil {
		return nil, fmt.Errorf("remote scan failed: %w", err)
	}

	// Log scan results
	e.logger.Info("remote SMB scan completed",
		zap.Int("files", result.TotalFiles),
		zap.Int("dirs", result.TotalDirs),
		zap.Int64("bytes", result.TotalBytes),
		zap.Duration("duration", result.Duration),
		zap.Int("errors", len(result.Errors)),
		zap.Bool("partial_success", result.PartialSuccess),
	)

	// Warn about any errors encountered
	if len(result.Errors) > 0 {
		e.logger.Warn("remote scan encountered errors",
			zap.Int("error_count", len(result.Errors)),
		)
		for i, scanErr := range result.Errors {
			if i < 5 { // Log first 5 errors
				e.logger.Warn("remote scan error", zap.Error(scanErr))
			}
		}
		if len(result.Errors) > 5 {
			e.logger.Warn("additional errors omitted", zap.Int("count", len(result.Errors)-5))
		}
	}

	return result.Files, nil
}
