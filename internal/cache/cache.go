package cache

import (
	"fmt"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"go.uber.org/zap"
)

// ChangeType represents the type of change detected
type ChangeType string

const (
	ChangeTypeNone     ChangeType = "none"     // No change
	ChangeTypeNew      ChangeType = "new"      // New file (not in cache)
	ChangeTypeModified ChangeType = "modified" // File modified (size or hash changed)
	ChangeTypeDeleted  ChangeType = "deleted"  // File deleted (in cache but not found)
	ChangeTypeConflict ChangeType = "conflict" // Conflict (both local and remote changed)
)

// FileChange represents a detected change in a file
type FileChange struct {
	Type       ChangeType // Type of change detected
	LocalPath  string     // Local file path
	RemotePath string     // Remote file path
	LocalState *FileInfo  // Current local file state (nil if deleted)
	CacheState *FileInfo  // Cached file state (nil if new)
	RemoteState *FileInfo // Remote file state (nil if not checked yet)
}

// FileInfo represents metadata about a file
type FileInfo struct {
	Path  string    // File path
	Size  int64     // File size in bytes
	MTime time.Time // Modification time
	Hash  string    // SHA256 hash (empty if not computed)
}

// CacheManager handles intelligent caching and change detection
type CacheManager struct {
	db     *database.DB
	logger *zap.Logger
}

// NewCacheManager creates a new cache manager
func NewCacheManager(db *database.DB, logger *zap.Logger) *CacheManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CacheManager{
		db:     db,
		logger: logger.With(zap.String("component", "cache-manager")),
	}
}

// GetCachedState retrieves the cached state for a file
// Returns nil if the file is not in cache
func (cm *CacheManager) GetCachedState(jobID int64, localPath string) (*FileInfo, error) {
	state, err := cm.db.GetFileState(jobID, localPath)
	if err != nil {
		// File not in cache is not an error
		return nil, nil
	}

	return &FileInfo{
		Path:  state.LocalPath,
		Size:  state.Size,
		MTime: time.Unix(state.MTime, 0),
		Hash:  state.Hash,
	}, nil
}

// UpdateCache updates the cache with current file state
func (cm *CacheManager) UpdateCache(jobID int64, localPath, remotePath string, info *FileInfo) error {
	if info == nil {
		return fmt.Errorf("file info cannot be nil")
	}

	now := time.Now().Unix()
	state := &database.FileState{
		JobID:      jobID,
		LocalPath:  localPath,
		RemotePath: remotePath,
		Size:       info.Size,
		MTime:      info.MTime.Unix(),
		Hash:       info.Hash,
		LastSync:   &now,
		SyncStatus: "idle",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := cm.db.UpsertFileState(state); err != nil {
		return fmt.Errorf("failed to update cache: %w", err)
	}

	cm.logger.Debug("cache updated",
		zap.String("local_path", localPath),
		zap.Int64("size", info.Size),
		zap.Time("mtime", info.MTime))

	return nil
}

// UpdateCacheBatch updates multiple cache entries in a single transaction
func (cm *CacheManager) UpdateCacheBatch(jobID int64, updates map[string]*FileInfo, remotePaths map[string]string) error {
	if len(updates) == 0 {
		return nil
	}

	now := time.Now().Unix()
	states := make([]*database.FileState, 0, len(updates))

	for localPath, info := range updates {
		remotePath := remotePaths[localPath]
		if remotePath == "" {
			remotePath = localPath // Default to same path
		}

		state := &database.FileState{
			JobID:      jobID,
			LocalPath:  localPath,
			RemotePath: remotePath,
			Size:       info.Size,
			MTime:      info.MTime.Unix(),
			Hash:       info.Hash,
			LastSync:   &now,
			SyncStatus: "idle",
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		states = append(states, state)
	}

	if err := cm.db.BulkUpdateFileStates(states); err != nil {
		return fmt.Errorf("failed to batch update cache: %w", err)
	}

	cm.logger.Info("cache batch updated",
		zap.Int("count", len(updates)))

	return nil
}

// RemoveFromCache removes a file from the cache
func (cm *CacheManager) RemoveFromCache(jobID int64, localPath string) error {
	if err := cm.db.DeleteFileState(jobID, localPath); err != nil {
		return fmt.Errorf("failed to remove from cache: %w", err)
	}

	cm.logger.Debug("removed from cache",
		zap.String("local_path", localPath))

	return nil
}

// DetectLocalChange detects changes in a local file compared to cache
// Returns the type of change and relevant information
func (cm *CacheManager) DetectLocalChange(jobID int64, localPath string, currentInfo *FileInfo) (*FileChange, error) {
	cachedInfo, err := cm.GetCachedState(jobID, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached state: %w", err)
	}

	change := &FileChange{
		LocalPath:  localPath,
		LocalState: currentInfo,
		CacheState: cachedInfo,
	}

	// Case 1: File not in cache (new file)
	if cachedInfo == nil {
		if currentInfo != nil {
			change.Type = ChangeTypeNew
			cm.logger.Debug("detected new file",
				zap.String("path", localPath))
		} else {
			// Neither in cache nor exists locally - no change
			change.Type = ChangeTypeNone
		}
		return change, nil
	}

	// Case 2: File in cache but not found locally (deleted)
	if currentInfo == nil {
		change.Type = ChangeTypeDeleted
		cm.logger.Debug("detected deleted file",
			zap.String("path", localPath))
		return change, nil
	}

	// Case 3: File exists in both - check for modifications
	if cm.hasFileChanged(cachedInfo, currentInfo) {
		change.Type = ChangeTypeModified
		cm.logger.Debug("detected modified file",
			zap.String("path", localPath),
			zap.Int64("old_size", cachedInfo.Size),
			zap.Int64("new_size", currentInfo.Size))
	} else {
		change.Type = ChangeTypeNone
	}

	return change, nil
}

// hasFileChanged checks if a file has changed by comparing size, mtime, and hash
func (cm *CacheManager) hasFileChanged(cached, current *FileInfo) bool {
	// Size changed - definitely modified
	if cached.Size != current.Size {
		return true
	}

	// Modification time changed - likely modified
	// Note: We use truncation to second precision as some filesystems don't support subsecond precision
	if !cached.MTime.Truncate(time.Second).Equal(current.MTime.Truncate(time.Second)) {
		return true
	}

	// If both have hashes and they differ - definitely modified
	if cached.Hash != "" && current.Hash != "" && cached.Hash != current.Hash {
		return true
	}

	// No detectable changes
	return false
}

// GetAllCachedFiles retrieves all files in cache for a job
func (cm *CacheManager) GetAllCachedFiles(jobID int64) (map[string]*FileInfo, error) {
	states, err := cm.db.GetAllFileStates(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached files: %w", err)
	}

	result := make(map[string]*FileInfo, len(states))
	for _, state := range states {
		result[state.LocalPath] = &FileInfo{
			Path:  state.LocalPath,
			Size:  state.Size,
			MTime: time.Unix(state.MTime, 0),
			Hash:  state.Hash,
		}
	}

	return result, nil
}

// SetSyncStatus updates the sync status of a file in cache
func (cm *CacheManager) SetSyncStatus(jobID int64, localPath, status string, errorMsg *string) error {
	state, err := cm.db.GetFileState(jobID, localPath)
	if err != nil {
		return fmt.Errorf("file not in cache: %w", err)
	}

	state.SyncStatus = status
	state.ErrorMessage = errorMsg

	if err := cm.db.UpsertFileState(state); err != nil {
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	cm.logger.Debug("sync status updated",
		zap.String("path", localPath),
		zap.String("status", status))

	return nil
}

// GetFilesWithStatus retrieves all files with a specific sync status
func (cm *CacheManager) GetFilesWithStatus(jobID int64, status string) ([]*FileInfo, error) {
	allStates, err := cm.db.GetAllFileStates(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get file states: %w", err)
	}

	var result []*FileInfo
	for _, state := range allStates {
		if state.SyncStatus == status {
			result = append(result, &FileInfo{
				Path:  state.LocalPath,
				Size:  state.Size,
				MTime: time.Unix(state.MTime, 0),
				Hash:  state.Hash,
			})
		}
	}

	return result, nil
}
