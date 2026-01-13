package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// SMBClientInterface defines the interface for SMB operations needed by RemoteScanner
type SMBClientInterface interface {
	ListRemote(path string) ([]smb.RemoteFileInfo, error)
}

// RemoteScanProgress represents progress during remote scanning
type RemoteScanProgress struct {
	FilesFound      int
	DirsScanned     int
	CurrentDir      string
	BytesDiscovered int64
	Errors          int
}

// RemoteScanCallback is called periodically during scanning
type RemoteScanCallback func(progress RemoteScanProgress)

// RemoteScanResult contains the results of a remote scan
type RemoteScanResult struct {
	Files           map[string]*cache.FileInfo
	TotalFiles      int
	TotalDirs       int
	TotalBytes      int64
	Duration        time.Duration
	Errors          []error
	PartialSuccess  bool // True if scan completed with some errors
}

// RemoteScanner scans remote SMB shares recursively
type RemoteScanner struct {
	client   SMBClientInterface
	logger   *zap.Logger
	callback RemoteScanCallback

	// Stats (protected by mutex)
	mu              sync.RWMutex
	filesFound      int
	dirsScanned     int
	bytesDiscovered int64
	errors          []error
}

// NewRemoteScanner creates a new remote scanner
func NewRemoteScanner(client SMBClientInterface, logger *zap.Logger, callback RemoteScanCallback) *RemoteScanner {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &RemoteScanner{
		client:   client,
		logger:   logger,
		callback: callback,
		errors:   make([]error, 0),
	}
}

// Scan scans a remote path recursively and returns all files found
func (rs *RemoteScanner) Scan(ctx context.Context, basePath string) (*RemoteScanResult, error) {
	startTime := time.Now()

	rs.logger.Info("starting remote scan", zap.String("base_path", basePath))

	// Reset stats
	rs.mu.Lock()
	rs.filesFound = 0
	rs.dirsScanned = 0
	rs.bytesDiscovered = 0
	rs.errors = make([]error, 0)
	rs.mu.Unlock()

	// Normalize base path (remove trailing slash)
	basePath = strings.TrimSuffix(basePath, "/")
	basePath = strings.TrimSuffix(basePath, "\\")

	// Scan recursively
	files := make(map[string]*cache.FileInfo)
	if err := rs.scanDir(ctx, basePath, basePath, files); err != nil {
		// Check if it's a partial failure
		if len(files) > 0 {
			rs.logger.Warn("remote scan completed with errors",
				zap.Error(err),
				zap.Int("files_found", len(files)),
			)
		} else {
			return nil, fmt.Errorf("remote scan failed: %w", err)
		}
	}

	duration := time.Since(startTime)

	rs.mu.RLock()
	result := &RemoteScanResult{
		Files:          files,
		TotalFiles:     rs.filesFound,
		TotalDirs:      rs.dirsScanned,
		TotalBytes:     rs.bytesDiscovered,
		Duration:       duration,
		Errors:         rs.errors,
		PartialSuccess: len(rs.errors) > 0 && len(files) > 0,
	}
	rs.mu.RUnlock()

	rs.logger.Info("remote scan completed",
		zap.Int("files", result.TotalFiles),
		zap.Int("dirs", result.TotalDirs),
		zap.Int64("bytes", result.TotalBytes),
		zap.Duration("duration", duration),
		zap.Int("errors", len(result.Errors)),
		zap.Bool("partial", result.PartialSuccess),
	)

	return result, nil
}

// scanDir scans a single directory recursively
func (rs *RemoteScanner) scanDir(ctx context.Context, currentPath string, basePath string, files map[string]*cache.FileInfo) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// List directory contents
	entries, err := rs.client.ListRemote(currentPath)
	if err != nil {
		rs.addError(fmt.Errorf("failed to list directory %s: %w", currentPath, err))
		return err
	}

	// Update stats
	rs.mu.Lock()
	rs.dirsScanned++
	dirsScanned := rs.dirsScanned
	rs.mu.Unlock()

	// Report progress
	if rs.callback != nil && dirsScanned%10 == 0 {
		rs.reportProgress(currentPath)
	}

	// Process entries
	for _, entry := range entries {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if entry.IsDir {
			// Recurse into subdirectory
			if err := rs.scanDir(ctx, entry.Path, basePath, files); err != nil {
				// Continue scanning other directories even if one fails
				rs.logger.Warn("failed to scan subdirectory",
					zap.String("path", entry.Path),
					zap.Error(err),
				)
			}
		} else {
			// Add file to result
			relativePath := strings.TrimPrefix(entry.Path, basePath)
			relativePath = strings.TrimPrefix(relativePath, "/")
			relativePath = strings.TrimPrefix(relativePath, "\\")
			relativePath = filepath.ToSlash(relativePath)

			if relativePath == "" {
				relativePath = filepath.Base(entry.Path)
			}

			files[relativePath] = &cache.FileInfo{
				Path:  relativePath,
				Size:  entry.Size,
				MTime: entry.ModTime,
				Hash:  "", // Hash not available from remote listing
			}

			// Update stats
			rs.mu.Lock()
			rs.filesFound++
			rs.bytesDiscovered += entry.Size
			filesFound := rs.filesFound
			rs.mu.Unlock()

			// Report progress periodically (every 100 files)
			if rs.callback != nil && filesFound%100 == 0 {
				rs.reportProgress(currentPath)
			}

			rs.logger.Debug("found remote file",
				zap.String("path", relativePath),
				zap.Int64("size", entry.Size),
			)
		}
	}

	return nil
}

// addError adds an error to the error list (thread-safe)
func (rs *RemoteScanner) addError(err error) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.errors = append(rs.errors, err)
}

// reportProgress reports current progress via callback
func (rs *RemoteScanner) reportProgress(currentDir string) {
	if rs.callback == nil {
		return
	}

	rs.mu.RLock()
	progress := RemoteScanProgress{
		FilesFound:      rs.filesFound,
		DirsScanned:     rs.dirsScanned,
		CurrentDir:      currentDir,
		BytesDiscovered: rs.bytesDiscovered,
		Errors:          len(rs.errors),
	}
	rs.mu.RUnlock()

	rs.callback(progress)
}

// GetStats returns current scan statistics (thread-safe)
func (rs *RemoteScanner) GetStats() RemoteScanProgress {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	return RemoteScanProgress{
		FilesFound:      rs.filesFound,
		DirsScanned:     rs.dirsScanned,
		BytesDiscovered: rs.bytesDiscovered,
		Errors:          len(rs.errors),
	}
}
