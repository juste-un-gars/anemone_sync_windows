package sync

import (
	"fmt"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"go.uber.org/zap"
)

// ConflictResolutionPolicy defines how to resolve sync conflicts
type ConflictResolutionPolicy string

const (
	// ConflictResolutionRecent resolves conflicts by choosing the file with the most recent modification time
	ConflictResolutionRecent ConflictResolutionPolicy = "recent"

	// ConflictResolutionLocal resolves conflicts by always keeping the local file
	ConflictResolutionLocal ConflictResolutionPolicy = "local"

	// ConflictResolutionRemote resolves conflicts by always keeping the remote file
	ConflictResolutionRemote ConflictResolutionPolicy = "remote"

	// ConflictResolutionAsk requires manual resolution (skipped for now)
	ConflictResolutionAsk ConflictResolutionPolicy = "ask"
)

// ConflictResolver resolves sync conflicts based on a policy
type ConflictResolver struct {
	policy ConflictResolutionPolicy
	logger *zap.Logger
}

// NewConflictResolver creates a new conflict resolver
func NewConflictResolver(policy string, logger *zap.Logger) (*ConflictResolver, error) {
	if !IsValidConflictResolution(policy) {
		return nil, fmt.Errorf("invalid conflict resolution policy: %s", policy)
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	return &ConflictResolver{
		policy: ConflictResolutionPolicy(policy),
		logger: logger,
	}, nil
}

// ResolveConflicts processes a list of sync decisions and resolves conflicts
// Returns:
// - resolved: decisions that have been resolved
// - unresolved: decisions that couldn't be resolved (e.g., "ask" policy)
func (cr *ConflictResolver) ResolveConflicts(decisions []*cache.SyncDecision) (resolved, unresolved []*cache.SyncDecision) {
	resolved = make([]*cache.SyncDecision, 0)
	unresolved = make([]*cache.SyncDecision, 0)

	for _, decision := range decisions {
		if !decision.NeedsResolution {
			// Not a conflict, keep as-is
			resolved = append(resolved, decision)
			continue
		}

		// Try to resolve conflict
		resolvedDecision := cr.resolveConflict(decision)
		if resolvedDecision != nil {
			resolved = append(resolved, resolvedDecision)
		} else {
			// Couldn't resolve (e.g., "ask" policy)
			unresolved = append(unresolved, decision)
		}
	}

	cr.logger.Info("conflict resolution complete",
		zap.Int("total_decisions", len(decisions)),
		zap.Int("resolved", len(resolved)),
		zap.Int("unresolved", len(unresolved)),
		zap.String("policy", string(cr.policy)),
	)

	return resolved, unresolved
}

// resolveConflict resolves a single conflict based on the policy
func (cr *ConflictResolver) resolveConflict(decision *cache.SyncDecision) *cache.SyncDecision {
	if decision.LocalInfo == nil || decision.RemoteInfo == nil {
		// Can't resolve without both files
		cr.logger.Warn("cannot resolve conflict: missing file info",
			zap.String("local_path", decision.LocalPath),
			zap.String("remote_path", decision.RemotePath),
		)
		return nil
	}

	switch cr.policy {
	case ConflictResolutionRecent:
		return cr.resolveByMostRecent(decision)

	case ConflictResolutionLocal:
		return cr.resolveByLocal(decision)

	case ConflictResolutionRemote:
		return cr.resolveByRemote(decision)

	case ConflictResolutionAsk:
		// Manual resolution required - return nil
		cr.logger.Info("manual resolution required",
			zap.String("local_path", decision.LocalPath),
			zap.String("remote_path", decision.RemotePath),
		)
		return nil

	default:
		cr.logger.Error("unknown conflict resolution policy",
			zap.String("policy", string(cr.policy)),
		)
		return nil
	}
}

// resolveByMostRecent chooses the file with the most recent modification time
func (cr *ConflictResolver) resolveByMostRecent(decision *cache.SyncDecision) *cache.SyncDecision {
	localTime := decision.LocalInfo.MTime
	remoteTime := decision.RemoteInfo.MTime

	resolved := &cache.SyncDecision{
		LocalPath:       decision.LocalPath,
		RemotePath:      decision.RemotePath,
		LocalInfo:       decision.LocalInfo,
		RemoteInfo:      decision.RemoteInfo,
		CachedInfo:      decision.CachedInfo,
		NeedsResolution: false,
	}

	if localTime.After(remoteTime) {
		// Local is newer - upload to remote
		resolved.Action = cache.ActionUpload
		resolved.Reason = fmt.Sprintf("conflict resolved: local newer (local: %s, remote: %s)",
			localTime.Format("2006-01-02 15:04:05"),
			remoteTime.Format("2006-01-02 15:04:05"))

		cr.logger.Debug("conflict resolved by most recent: local wins",
			zap.String("path", decision.LocalPath),
			zap.Time("local_time", localTime),
			zap.Time("remote_time", remoteTime),
		)
	} else if remoteTime.After(localTime) {
		// Remote is newer - download to local
		resolved.Action = cache.ActionDownload
		resolved.Reason = fmt.Sprintf("conflict resolved: remote newer (local: %s, remote: %s)",
			localTime.Format("2006-01-02 15:04:05"),
			remoteTime.Format("2006-01-02 15:04:05"))

		cr.logger.Debug("conflict resolved by most recent: remote wins",
			zap.String("path", decision.LocalPath),
			zap.Time("local_time", localTime),
			zap.Time("remote_time", remoteTime),
		)
	} else {
		// Same time - check file size as tiebreaker
		if decision.LocalInfo.Size != decision.RemoteInfo.Size {
			// Different sizes but same time - prefer larger file
			if decision.LocalInfo.Size > decision.RemoteInfo.Size {
				resolved.Action = cache.ActionUpload
				resolved.Reason = "conflict resolved: same time, local larger"
			} else {
				resolved.Action = cache.ActionDownload
				resolved.Reason = "conflict resolved: same time, remote larger"
			}
		} else {
			// Identical time and size - skip
			resolved.Action = cache.ActionNone
			resolved.Reason = "conflict resolved: files identical (time and size)"
		}

		cr.logger.Debug("conflict resolved by tiebreaker",
			zap.String("path", decision.LocalPath),
			zap.String("action", string(resolved.Action)),
		)
	}

	return resolved
}

// resolveByLocal always keeps the local file (upload to remote)
func (cr *ConflictResolver) resolveByLocal(decision *cache.SyncDecision) *cache.SyncDecision {
	resolved := &cache.SyncDecision{
		LocalPath:       decision.LocalPath,
		RemotePath:      decision.RemotePath,
		Action:          cache.ActionUpload,
		Reason:          "conflict resolved: local preference policy",
		LocalInfo:       decision.LocalInfo,
		RemoteInfo:      decision.RemoteInfo,
		CachedInfo:      decision.CachedInfo,
		NeedsResolution: false,
	}

	cr.logger.Debug("conflict resolved by local preference",
		zap.String("path", decision.LocalPath),
	)

	return resolved
}

// resolveByRemote always keeps the remote file (download to local)
func (cr *ConflictResolver) resolveByRemote(decision *cache.SyncDecision) *cache.SyncDecision {
	resolved := &cache.SyncDecision{
		LocalPath:       decision.LocalPath,
		RemotePath:      decision.RemotePath,
		Action:          cache.ActionDownload,
		Reason:          "conflict resolved: remote preference policy",
		LocalInfo:       decision.LocalInfo,
		RemoteInfo:      decision.RemoteInfo,
		CachedInfo:      decision.CachedInfo,
		NeedsResolution: false,
	}

	cr.logger.Debug("conflict resolved by remote preference",
		zap.String("path", decision.LocalPath),
	)

	return resolved
}

// GetPolicy returns the current conflict resolution policy
func (cr *ConflictResolver) GetPolicy() ConflictResolutionPolicy {
	return cr.policy
}

// SetPolicy changes the conflict resolution policy
func (cr *ConflictResolver) SetPolicy(policy string) error {
	if !IsValidConflictResolution(policy) {
		return fmt.Errorf("invalid conflict resolution policy: %s", policy)
	}

	cr.policy = ConflictResolutionPolicy(policy)
	cr.logger.Info("conflict resolution policy changed", zap.String("policy", policy))

	return nil
}

// CountConflicts counts the number of decisions that need resolution
func CountConflicts(decisions []*cache.SyncDecision) int {
	count := 0
	for _, decision := range decisions {
		if decision.NeedsResolution {
			count++
		}
	}
	return count
}

// SeparateConflicts separates decisions into conflicts and non-conflicts
func SeparateConflicts(decisions []*cache.SyncDecision) (conflicts, normal []*cache.SyncDecision) {
	conflicts = make([]*cache.SyncDecision, 0)
	normal = make([]*cache.SyncDecision, 0)

	for _, decision := range decisions {
		if decision.NeedsResolution {
			conflicts = append(conflicts, decision)
		} else {
			normal = append(normal, decision)
		}
	}

	return conflicts, normal
}
