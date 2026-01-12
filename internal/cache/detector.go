package cache

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// SyncAction represents the action to take for a file
type SyncAction string

const (
	ActionNone          SyncAction = "none"            // No action needed
	ActionUpload        SyncAction = "upload"          // Upload local to remote
	ActionDownload      SyncAction = "download"        // Download remote to local
	ActionDelete        SyncAction = "delete"          // Delete file
	ActionConflict      SyncAction = "conflict"        // Conflict needs resolution
	ActionDeleteLocal   SyncAction = "delete_local"    // Delete local file
	ActionDeleteRemote  SyncAction = "delete_remote"   // Delete remote file
)

// SyncDecision represents a sync decision for a file
type SyncDecision struct {
	LocalPath   string     // Local file path
	RemotePath  string     // Remote file path
	Action      SyncAction // Action to take
	Reason      string     // Reason for the action
	LocalInfo   *FileInfo  // Current local state
	RemoteInfo  *FileInfo  // Current remote state
	CachedInfo  *FileInfo  // Cached state
	NeedsResolution bool   // True if requires user resolution
}

// ChangeDetector detects changes and determines sync actions
type ChangeDetector struct {
	cache  *CacheManager
	logger *zap.Logger
}

// NewChangeDetector creates a new change detector
func NewChangeDetector(cache *CacheManager, logger *zap.Logger) *ChangeDetector {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ChangeDetector{
		cache:  cache,
		logger: logger.With(zap.String("component", "change-detector")),
	}
}

// DetermineSyncAction determines what action to take for a file
// This implements the 3-way merge logic: Local vs Cache vs Remote
func (cd *ChangeDetector) DetermineSyncAction(jobID int64, localPath, remotePath string, localInfo, remoteInfo *FileInfo) (*SyncDecision, error) {
	decision := &SyncDecision{
		LocalPath:  localPath,
		RemotePath: remotePath,
		LocalInfo:  localInfo,
		RemoteInfo: remoteInfo,
	}

	// Get cached state
	cachedInfo, err := cd.cache.GetCachedState(jobID, localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached state: %w", err)
	}
	decision.CachedInfo = cachedInfo

	// Determine action based on 3-way comparison
	decision.Action, decision.Reason = cd.decide3Way(localInfo, remoteInfo, cachedInfo)

	// Check if needs user resolution
	decision.NeedsResolution = decision.Action == ActionConflict

	cd.logger.Debug("sync decision made",
		zap.String("path", localPath),
		zap.String("action", string(decision.Action)),
		zap.String("reason", decision.Reason))

	return decision, nil
}

// decide3Way implements 3-way merge decision logic
func (cd *ChangeDetector) decide3Way(local, remote, cached *FileInfo) (SyncAction, string) {
	localExists := local != nil
	remoteExists := remote != nil
	cachedExists := cached != nil

	// Case 1: File doesn't exist anywhere
	if !localExists && !remoteExists && !cachedExists {
		return ActionNone, "file does not exist anywhere"
	}

	// Case 2: New file (not in cache)
	if !cachedExists {
		if localExists && remoteExists {
			// Both created independently - conflict
			if cd.filesAreSame(local, remote) {
				return ActionNone, "file already in sync (identical content)"
			}
			return ActionConflict, "file created on both sides with different content"
		}
		if localExists {
			return ActionUpload, "new local file needs upload"
		}
		if remoteExists {
			return ActionDownload, "new remote file needs download"
		}
	}

	// Case 3: File was deleted locally but exists remotely
	if !localExists && remoteExists && cachedExists {
		if cd.filesAreSame(remote, cached) {
			// Remote unchanged, local deleted - delete remote
			return ActionDeleteRemote, "file deleted locally, remove from remote"
		}
		// Remote changed, local deleted - conflict
		return ActionConflict, "file deleted locally but modified remotely"
	}

	// Case 4: File exists locally but deleted remotely
	if localExists && !remoteExists && cachedExists {
		if cd.filesAreSame(local, cached) {
			// Local unchanged, remote deleted - delete local
			return ActionDeleteLocal, "file deleted remotely, remove local copy"
		}
		// Local changed, remote deleted - conflict
		return ActionConflict, "file modified locally but deleted remotely"
	}

	// Case 5: File deleted on both sides
	if !localExists && !remoteExists && cachedExists {
		return ActionNone, "file deleted on both sides"
	}

	// Case 6: File exists in all three places
	if localExists && remoteExists && cachedExists {
		localChanged := !cd.filesAreSame(local, cached)
		remoteChanged := !cd.filesAreSame(remote, cached)

		if !localChanged && !remoteChanged {
			// No changes anywhere
			return ActionNone, "file unchanged"
		}

		if localChanged && !remoteChanged {
			// Only local changed - upload
			return ActionUpload, "file modified locally"
		}

		if !localChanged && remoteChanged {
			// Only remote changed - download
			return ActionDownload, "file modified remotely"
		}

		// Both changed - conflict
		if cd.filesAreSame(local, remote) {
			// Changed to same content - no action needed
			return ActionNone, "file modified on both sides but content is identical"
		}
		return ActionConflict, "file modified on both sides with different content"
	}

	// Fallback - should not reach here
	return ActionNone, "unexpected state"
}

// filesAreSame checks if two files are the same
func (cd *ChangeDetector) filesAreSame(f1, f2 *FileInfo) bool {
	if f1 == nil || f2 == nil {
		return f1 == f2 // Both nil = same, one nil = different
	}

	// Size must match
	if f1.Size != f2.Size {
		return false
	}

	// If both have hashes, compare them
	if f1.Hash != "" && f2.Hash != "" {
		return f1.Hash == f2.Hash
	}

	// If no hashes available, compare size and mtime (less reliable)
	// Truncate to second precision for filesystem compatibility
	return f1.MTime.Truncate(time.Second).Equal(f2.MTime.Truncate(time.Second))
}

// BatchDetermineSyncActions determines sync actions for multiple files
func (cd *ChangeDetector) BatchDetermineSyncActions(jobID int64, files map[string]*FileInfo, remoteFiles map[string]*FileInfo) ([]*SyncDecision, error) {
	decisions := make([]*SyncDecision, 0)

	// Get all cached files
	cachedFiles, err := cd.cache.GetAllCachedFiles(jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get cached files: %w", err)
	}

	// Build a set of all unique paths
	allPaths := make(map[string]bool)
	for path := range files {
		allPaths[path] = true
	}
	for path := range remoteFiles {
		allPaths[path] = true
	}
	for path := range cachedFiles {
		allPaths[path] = true
	}

	// Determine action for each path
	for path := range allPaths {
		local := files[path]
		remote := remoteFiles[path]

		decision, err := cd.DetermineSyncAction(jobID, path, path, local, remote)
		if err != nil {
			cd.logger.Warn("failed to determine sync action",
				zap.String("path", path),
				zap.Error(err))
			continue
		}

		// Only include if action is needed
		if decision.Action != ActionNone {
			decisions = append(decisions, decision)
		}
	}

	cd.logger.Info("batch sync decisions made",
		zap.Int("total_paths", len(allPaths)),
		zap.Int("actions_needed", len(decisions)))

	return decisions, nil
}

// ResolveConflict resolves a conflict with a user-specified strategy
func (cd *ChangeDetector) ResolveConflict(decision *SyncDecision, resolution string) error {
	if decision.Action != ActionConflict {
		return fmt.Errorf("not a conflict decision")
	}

	switch resolution {
	case "local":
		// Keep local, overwrite remote
		decision.Action = ActionUpload
		decision.Reason = "conflict resolved: keep local version"
		decision.NeedsResolution = false
	case "remote":
		// Keep remote, overwrite local
		decision.Action = ActionDownload
		decision.Reason = "conflict resolved: keep remote version"
		decision.NeedsResolution = false
	case "recent":
		// Keep most recent
		if decision.LocalInfo != nil && decision.RemoteInfo != nil {
			if decision.LocalInfo.MTime.After(decision.RemoteInfo.MTime) {
				decision.Action = ActionUpload
				decision.Reason = "conflict resolved: local is more recent"
			} else {
				decision.Action = ActionDownload
				decision.Reason = "conflict resolved: remote is more recent"
			}
			decision.NeedsResolution = false
		} else {
			return fmt.Errorf("cannot determine most recent: missing file info")
		}
	default:
		return fmt.Errorf("unknown conflict resolution strategy: %s", resolution)
	}

	cd.logger.Info("conflict resolved",
		zap.String("path", decision.LocalPath),
		zap.String("resolution", resolution),
		zap.String("action", string(decision.Action)))

	return nil
}
