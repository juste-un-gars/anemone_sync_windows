package sync

import (
	"context"
	"fmt"
	"os"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// Executor executes sync actions
type Executor struct {
	logger       *zap.Logger
	bufferSizeMB int
	retryPolicy  *RetryPolicy
	numWorkers   int // Number of workers for parallel execution (0 = sequential)
}

// NewExecutor creates a new executor
func NewExecutor(bufferSizeMB int, logger *zap.Logger) *Executor {
	if logger == nil {
		logger = zap.NewNop()
	}

	if bufferSizeMB <= 0 {
		bufferSizeMB = 4 // Default 4MB
	}

	return &Executor{
		logger:       logger,
		bufferSizeMB: bufferSizeMB,
		retryPolicy:  DefaultRetryPolicy(logger.Named("retry")),
		numWorkers:   0, // Default to sequential execution
	}
}

// SetRetryPolicy sets a custom retry policy
func (ex *Executor) SetRetryPolicy(policy *RetryPolicy) {
	ex.retryPolicy = policy
}

// SetParallelMode enables parallel execution with the specified number of workers
// Set numWorkers to 0 to disable parallel mode (sequential execution)
func (ex *Executor) SetParallelMode(numWorkers int) {
	if numWorkers < 0 {
		numWorkers = 0
	}
	ex.numWorkers = numWorkers
	ex.logger.Info("parallel mode configured", zap.Int("workers", numWorkers))
}

// Execute executes a batch of sync decisions
// Uses parallel execution if numWorkers > 0, otherwise sequential
func (ex *Executor) Execute(
	ctx context.Context,
	decisions []*cache.SyncDecision,
	smbClient *smb.SMBClient,
	progressFn ProgressCallback,
) ([]*SyncAction, error) {

	if len(decisions) == 0 {
		return []*SyncAction{}, nil
	}

	// Prioritize actions to minimize data loss risk
	decisions = ex.prioritizeActions(decisions)

	// Use parallel execution if configured
	if ex.numWorkers > 0 {
		ex.logger.Info("executing sync actions in parallel",
			zap.Int("count", len(decisions)),
			zap.Int("workers", ex.numWorkers),
		)
		return ExecuteParallel(ctx, decisions, smbClient, ex, ex.numWorkers, progressFn, ex.logger)
	}

	// Sequential execution
	ex.logger.Info("executing sync actions sequentially",
		zap.Int("count", len(decisions)),
	)

	actions := make([]*SyncAction, 0, len(decisions))
	var bytesTransferred int64

	// Execute actions sequentially
	for i, decision := range decisions {
		// Check context cancellation
		select {
		case <-ctx.Done():
			ex.logger.Warn("execution cancelled",
				zap.Int("completed", i),
				zap.Int("total", len(decisions)),
			)
			return actions, ctx.Err()
		default:
		}

		// Report progress
		if progressFn != nil {
			progressFn(&SyncProgress{
				Phase:            "executing",
				CurrentFile:      decision.LocalPath,
				FilesProcessed:   i,
				FilesTotal:       len(decisions),
				BytesTransferred: bytesTransferred,
				CurrentAction:    fmt.Sprintf("%s: %s", decision.Action, decision.LocalPath),
				Percentage:       35 + float64(i)/float64(len(decisions))*60, // 35-95%
			})
		}

		// Execute action
		action, err := ex.executeAction(ctx, decision, smbClient)
		if err != nil {
			ex.logger.Error("action failed",
				zap.String("action", string(decision.Action)),
				zap.String("path", decision.LocalPath),
				zap.Error(err),
			)
			action.Status = ActionStatusFailed
			action.Error = err
		} else {
			action.Status = ActionStatusSuccess
			bytesTransferred += action.BytesTransferred
		}

		actions = append(actions, action)
	}

	successCount := 0
	for _, action := range actions {
		if action.Status == ActionStatusSuccess {
			successCount++
		}
	}

	ex.logger.Info("execution completed",
		zap.Int("total", len(actions)),
		zap.Int("success", successCount),
		zap.Int("failed", len(actions)-successCount),
		zap.Int64("bytes_transferred", bytesTransferred),
	)

	return actions, nil
}

// executeAction executes a single sync action
func (ex *Executor) executeAction(
	ctx context.Context,
	decision *cache.SyncDecision,
	smbClient *smb.SMBClient,
) (*SyncAction, error) {

	action := &SyncAction{
		FilePath:   decision.LocalPath,
		RemotePath: decision.RemotePath,
		Action:     decision.Action,
		Status:     ActionStatusExecuting,
		Timestamp:  timeNow(),
	}

	startTime := timeNow()

	// Wrap action execution with retry logic
	operationName := fmt.Sprintf("%s:%s", decision.Action, decision.LocalPath)
	err := ex.retryPolicy.Retry(ctx, operationName, func() error {
		switch decision.Action {
		case cache.ActionUpload:
			return ex.executeUpload(ctx, decision, smbClient, action)

		case cache.ActionDownload:
			return ex.executeDownload(ctx, decision, smbClient, action)

		case cache.ActionDeleteLocal:
			return ex.executeDeleteLocal(ctx, decision, action)

		case cache.ActionDeleteRemote:
			return ex.executeDeleteRemote(ctx, decision, smbClient, action)

		default:
			return fmt.Errorf("unknown action: %s", decision.Action)
		}
	})

	action.Duration = timeNow().Sub(startTime)

	return action, err
}

// executeUpload uploads a file from local to remote
func (ex *Executor) executeUpload(
	ctx context.Context,
	decision *cache.SyncDecision,
	smbClient *smb.SMBClient,
	action *SyncAction,
) error {

	// Get file info to determine size
	info, err := os.Stat(decision.LocalPath)
	if err != nil {
		return WrapSyncError(err, decision.LocalPath, "stat")
	}

	action.Size = info.Size()

	// Upload file
	ex.logger.Debug("uploading file",
		zap.String("local", decision.LocalPath),
		zap.String("remote", decision.RemotePath),
		zap.Int64("size", action.Size),
	)

	if err := smbClient.Upload(decision.LocalPath, decision.RemotePath); err != nil {
		return WrapSyncError(err, decision.LocalPath, "upload")
	}

	action.BytesTransferred = action.Size

	ex.logger.Info("file uploaded",
		zap.String("path", decision.LocalPath),
		zap.Int64("size", action.Size),
		zap.Duration("duration", action.Duration),
	)

	return nil
}

// executeDownload downloads a file from remote to local
func (ex *Executor) executeDownload(
	ctx context.Context,
	decision *cache.SyncDecision,
	smbClient *smb.SMBClient,
	action *SyncAction,
) error {

	// Get remote file info to determine size
	if decision.RemoteInfo != nil {
		action.Size = decision.RemoteInfo.Size
	}

	// Download file
	ex.logger.Debug("downloading file",
		zap.String("remote", decision.RemotePath),
		zap.String("local", decision.LocalPath),
		zap.Int64("size", action.Size),
	)

	if err := smbClient.Download(decision.RemotePath, decision.LocalPath); err != nil {
		return WrapSyncError(err, decision.LocalPath, "download")
	}

	// Get actual size after download
	info, err := os.Stat(decision.LocalPath)
	if err == nil {
		action.Size = info.Size()
	}

	action.BytesTransferred = action.Size

	ex.logger.Info("file downloaded",
		zap.String("path", decision.LocalPath),
		zap.Int64("size", action.Size),
		zap.Duration("duration", action.Duration),
	)

	return nil
}

// executeDeleteLocal deletes a local file
func (ex *Executor) executeDeleteLocal(
	ctx context.Context,
	decision *cache.SyncDecision,
	action *SyncAction,
) error {

	ex.logger.Debug("deleting local file",
		zap.String("path", decision.LocalPath),
	)

	// Get file size before deletion
	info, err := os.Stat(decision.LocalPath)
	if err == nil {
		action.Size = info.Size()
	}

	// Delete file
	if err := os.Remove(decision.LocalPath); err != nil {
		// Ignore "file not found" errors (race condition acceptable)
		if os.IsNotExist(err) {
			ex.logger.Debug("file already deleted", zap.String("path", decision.LocalPath))
			return nil
		}
		return WrapSyncError(err, decision.LocalPath, "delete_local")
	}

	ex.logger.Info("local file deleted",
		zap.String("path", decision.LocalPath),
	)

	return nil
}

// executeDeleteRemote deletes a remote file
func (ex *Executor) executeDeleteRemote(
	ctx context.Context,
	decision *cache.SyncDecision,
	smbClient *smb.SMBClient,
	action *SyncAction,
) error {

	ex.logger.Debug("deleting remote file",
		zap.String("path", decision.RemotePath),
	)

	// Get file size from decision
	if decision.RemoteInfo != nil {
		action.Size = decision.RemoteInfo.Size
	}

	// Delete file
	if err := smbClient.Delete(decision.RemotePath); err != nil {
		// Check if file not found (acceptable race condition)
		if isFileNotFoundError(err) {
			ex.logger.Debug("remote file already deleted", zap.String("path", decision.RemotePath))
			return nil
		}
		return WrapSyncError(err, decision.RemotePath, "delete_remote")
	}

	ex.logger.Info("remote file deleted",
		zap.String("path", decision.RemotePath),
	)

	return nil
}

// prioritizeActions sorts actions to minimize data loss risk
// Priority order: Downloads → Uploads → Deletes
func (ex *Executor) prioritizeActions(decisions []*cache.SyncDecision) []*cache.SyncDecision {
	// Create a copy to avoid modifying the original
	prioritized := make([]*cache.SyncDecision, len(decisions))
	copy(prioritized, decisions)

	// Simple bubble sort by priority (good enough for now)
	n := len(prioritized)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if actionPriority(prioritized[j].Action) > actionPriority(prioritized[j+1].Action) {
				prioritized[j], prioritized[j+1] = prioritized[j+1], prioritized[j]
			}
		}
	}

	return prioritized
}

// actionPriority returns the priority of an action (lower = higher priority)
func actionPriority(action cache.SyncAction) int {
	switch action {
	case cache.ActionDownload:
		return 1 // Download first (get remote data)
	case cache.ActionUpload:
		return 2 // Upload second (send local data)
	case cache.ActionDeleteLocal, cache.ActionDeleteRemote:
		return 3 // Delete last (minimize data loss)
	default:
		return 4 // Unknown actions last
	}
}

// isFileNotFoundError checks if an error indicates file not found
func isFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if os.IsNotExist(err) {
		return true
	}

	// Check error message patterns
	msg := err.Error()
	notFoundPatterns := []string{
		"not found",
		"no such file",
		"does not exist",
		"cannot find",
	}

	for _, pattern := range notFoundPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	return false
}
