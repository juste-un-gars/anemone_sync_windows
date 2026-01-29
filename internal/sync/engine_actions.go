package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// prepareSync handles Phase 1: Preparation
func (e *Engine) prepareSync(ctx context.Context, req *SyncRequest) (*smb.SMBClient, *database.SyncJob, error) {
	// Load job from database
	job, err := e.db.GetSyncJob(req.JobID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load job: %w", err)
	}

	if job == nil {
		return nil, nil, fmt.Errorf("job %d not found", req.JobID)
	}

	// Parse RemotePath (UNC format: \\server\share\path) to extract server and share
	server, share, _ := parseUNCPath(req.RemotePath)
	if server == "" {
		return nil, nil, fmt.Errorf("invalid remote path: server not found in %s", req.RemotePath)
	}
	if share == "" {
		return nil, nil, fmt.Errorf("invalid remote path: share not found in %s", req.RemotePath)
	}

	// Create SMB client from keyring (credentials stored by server host)
	smbClient, err := smb.NewSMBClientFromKeyring(server, share, e.logger.Named("smb"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SMB client: %w", err)
	}

	// Connect to SMB server
	if err := smbClient.Connect(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	// Update job status to syncing
	if err := e.db.UpdateJobStatus(req.JobID, "syncing"); err != nil {
		smbClient.Disconnect()
		return nil, nil, fmt.Errorf("failed to update job status: %w", err)
	}

	e.logger.Info("preparation completed",
		zap.String("server", server),
		zap.String("share", share),
	)

	return smbClient, job, nil
}

// detectChanges handles Phase 3: Detection
func (e *Engine) detectChanges(ctx context.Context, req *SyncRequest,
	localFiles, remoteFiles, cachedFiles map[string]*cache.FileInfo) (
	decisions []*cache.SyncDecision,
	conflicts []*cache.SyncDecision,
	err error,
) {
	// Use change detector for 3-way merge
	allDecisions, err := e.detector.BatchDetermineSyncActions(req.JobID, localFiles, remoteFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("change detection failed: %w", err)
	}

	// Separate conflicts from executable decisions
	initialConflicts := make([]*cache.SyncDecision, 0)
	decisions = make([]*cache.SyncDecision, 0)

	for _, decision := range allDecisions {
		if decision.NeedsResolution {
			initialConflicts = append(initialConflicts, decision)
		} else {
			decisions = append(decisions, decision)
		}
	}

	e.logger.Info("initial change detection completed",
		zap.Int("total_decisions", len(allDecisions)),
		zap.Int("executable", len(decisions)),
		zap.Int("conflicts", len(initialConflicts)),
	)

	// Resolve conflicts if there are any and a resolution policy is set
	if len(initialConflicts) > 0 && req.ConflictResolution != "" {
		resolver, err := NewConflictResolver(req.ConflictResolution, e.logger.Named("conflict_resolver"))
		if err != nil {
			e.logger.Warn("failed to create conflict resolver",
				zap.Error(err),
				zap.String("policy", req.ConflictResolution),
			)
			conflicts = initialConflicts
		} else {
			// Attempt to resolve conflicts
			resolved, unresolved := resolver.ResolveConflicts(initialConflicts)

			// Add resolved conflicts to decisions
			decisions = append(decisions, resolved...)
			conflicts = unresolved

			e.logger.Info("conflict resolution applied",
				zap.Int("initial_conflicts", len(initialConflicts)),
				zap.Int("resolved", len(resolved)),
				zap.Int("unresolved", len(unresolved)),
				zap.String("policy", req.ConflictResolution),
			)
		}
	} else {
		conflicts = initialConflicts
	}

	// Filter decisions based on sync mode
	decisions = e.filterDecisionsByMode(req.Mode, decisions)

	e.logger.Info("change detection completed",
		zap.Int("total_decisions", len(allDecisions)),
		zap.Int("executable", len(decisions)),
		zap.Int("final_conflicts", len(conflicts)),
	)

	return decisions, conflicts, nil
}

// filterDecisionsByMode filters decisions based on sync mode
func (e *Engine) filterDecisionsByMode(mode SyncMode, decisions []*cache.SyncDecision) []*cache.SyncDecision {
	filtered := make([]*cache.SyncDecision, 0, len(decisions))

	for _, decision := range decisions {
		include := false

		switch decision.Action {
		case cache.ActionUpload:
			include = mode.AllowsUpload()
		case cache.ActionDownload:
			include = mode.AllowsDownload()
		case cache.ActionDeleteLocal:
			include = mode.AllowsDownload() // Only delete local if we can sync from remote
		case cache.ActionDeleteRemote:
			include = mode.AllowsUpload() // Only delete remote if we can sync to remote
		default:
			include = false
		}

		if include {
			filtered = append(filtered, decision)
		}
	}

	return filtered
}

// executeActions handles Phase 4: Execution
func (e *Engine) executeActions(ctx context.Context, req *SyncRequest,
	decisions []*cache.SyncDecision, smbClient *smb.SMBClient, job *database.SyncJob) ([]*SyncAction, error) {

	// Convert relative paths to absolute/full paths for execution
	// LocalPath needs to be absolute for file operations (e.g., D:/SYNC/file.txt)
	// RemotePath needs to include the base path relative to share (e.g., TEST/TEST1/file.txt)
	localBasePath := filepath.Clean(req.LocalPath)

	// Extract remote base path from UNC path (e.g., "TEST/TEST1" from "\\server\share\TEST\TEST1")
	_, _, remoteBasePath := parseUNCPath(req.RemotePath)

	for _, decision := range decisions {
		// Convert relative LocalPath to absolute
		if !filepath.IsAbs(decision.LocalPath) {
			decision.LocalPath = filepath.Join(localBasePath, decision.LocalPath)
		}

		// Convert relative RemotePath to full path within share
		// RemotePath is relative (e.g., "file.txt"), needs to be "TEST/TEST1/file.txt"
		if remoteBasePath != "" && !strings.HasPrefix(decision.RemotePath, remoteBasePath) {
			decision.RemotePath = remoteBasePath + "/" + decision.RemotePath
		}
	}

	// Create progress callback
	progressFn := func(progress *SyncProgress) {
		e.reportProgress(req, progress)
	}

	// Execute using executor
	actions, err := e.executor.Execute(ctx, decisions, smbClient, progressFn)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	return actions, nil
}

// finalizeSync handles Phase 5: Finalization
func (e *Engine) finalizeSync(ctx context.Context, req *SyncRequest, result *SyncResult, job *database.SyncJob,
	localFiles, remoteFiles map[string]*cache.FileInfo) error {
	// Update cache for successful actions
	if !req.DryRun {
		if err := e.updateCacheFromActions(req.JobID, req.LocalPath, result.Actions); err != nil {
			return fmt.Errorf("failed to update cache: %w", err)
		}

		// Initialize cache for files that are already in sync (exist on both sides with same content)
		// This is critical for bidirectional sync to detect remote deletions correctly
		if err := e.initializeCacheForInSyncFiles(req.JobID, localFiles, remoteFiles); err != nil {
			e.logger.Warn("failed to initialize cache for in-sync files", zap.Error(err))
			// Non-fatal error, continue
		}
	}

	// Record sync history
	history := &database.SyncHistory{
		JobID:            req.JobID,
		Timestamp:        result.StartTime,
		FilesSynced:      result.FilesUploaded + result.FilesDownloaded,
		FilesFailed:      result.FilesError,
		BytesTransferred: result.BytesTransferred,
		Duration:         int(result.Duration.Seconds()),
		Status:           string(result.Status),
		ErrorSummary:     formatErrorSummary(result.Errors),
	}

	if err := e.db.InsertSyncHistory(history); err != nil {
		return fmt.Errorf("failed to insert sync history: %w", err)
	}

	// Update job status
	var finalStatus string
	switch result.Status {
	case SyncStatusSuccess:
		finalStatus = "idle"
	case SyncStatusPartial:
		finalStatus = "idle" // Still idle but with some errors
	case SyncStatusFailed:
		finalStatus = "error"
	}

	if err := e.db.UpdateJobStatus(req.JobID, finalStatus); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update job last run time
	if err := e.db.UpdateJobLastRun(req.JobID, result.EndTime); err != nil {
		return fmt.Errorf("failed to update job last run: %w", err)
	}

	e.logger.Info("finalization completed")
	return nil
}

// updateCacheFromActions updates cache based on successful actions
func (e *Engine) updateCacheFromActions(jobID int64, localBasePath string, actions []*SyncAction) error {
	updates := make(map[string]*cache.FileInfo)
	remotePaths := make(map[string]string)

	for _, action := range actions {
		if action.Status != ActionStatusSuccess {
			continue
		}

		// Convert absolute path back to relative for cache storage
		relPath := toRelativePath(action.FilePath, localBasePath)

		updates[relPath] = &cache.FileInfo{
			Path:  relPath,
			Size:  action.Size,
			MTime: timeNow(), // Current time after sync
			Hash:  "",        // Hash will be computed on next scan if needed
		}
		remotePaths[relPath] = action.RemotePath
	}

	if len(updates) > 0 {
		return e.cache.UpdateCacheBatch(jobID, updates, remotePaths)
	}

	return nil
}

// initializeCacheForInSyncFiles adds files to cache that are already synchronized
// (exist on both local and remote with same content). This is critical for
// bidirectional sync to correctly detect when files are deleted on one side.
func (e *Engine) initializeCacheForInSyncFiles(jobID int64, localFiles, remoteFiles map[string]*cache.FileInfo) error {
	updates := make(map[string]*cache.FileInfo)
	remotePaths := make(map[string]string)

	for path, localInfo := range localFiles {
		remoteInfo, existsRemote := remoteFiles[path]
		if !existsRemote || localInfo == nil || remoteInfo == nil {
			continue
		}

		// Check if files have the same content (size and hash if available)
		if localInfo.Size != remoteInfo.Size {
			continue
		}
		if localInfo.Hash != "" && remoteInfo.Hash != "" && localInfo.Hash != remoteInfo.Hash {
			continue
		}

		// Files are in sync - check if already in cache
		cachedInfo, _ := e.cache.GetCachedState(jobID, path)
		if cachedInfo != nil {
			// Already in cache, skip
			continue
		}

		// Add to cache
		updates[path] = &cache.FileInfo{
			Path:  path,
			Size:  localInfo.Size,
			MTime: localInfo.MTime,
			Hash:  localInfo.Hash,
		}
		remotePaths[path] = path
	}

	if len(updates) > 0 {
		e.logger.Info("initializing cache for in-sync files",
			zap.Int("count", len(updates)))
		return e.cache.UpdateCacheBatch(jobID, updates, remotePaths)
	}

	return nil
}
