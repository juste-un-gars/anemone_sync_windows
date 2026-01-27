//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API.
package cloudfiles

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DehydrationManager manages automatic dehydration of placeholder files.
// It can free up disk space by converting hydrated files back to placeholders.
type DehydrationManager struct {
	mu sync.RWMutex

	syncRoot    *SyncRootManager
	policy      DehydrationPolicy
	logger      *zap.Logger

	// Statistics
	stats DehydrationStats

	// Control
	running bool
	cancel  context.CancelFunc
}

// DehydrationPolicy defines when files should be dehydrated.
type DehydrationPolicy struct {
	// Enabled controls whether automatic dehydration is active
	Enabled bool

	// MaxAgeDays is the number of days since last access before a file is dehydrated.
	// Files accessed within this period will not be dehydrated.
	// Set to 0 to disable age-based dehydration.
	MaxAgeDays int

	// MinFileSize is the minimum file size in bytes to consider for dehydration.
	// Files smaller than this will not be dehydrated.
	MinFileSize int64

	// ExcludePatterns are glob patterns for files to exclude from dehydration.
	ExcludePatterns []string

	// MaxFilesToDehydrate limits the number of files processed per scan.
	// Set to 0 for unlimited.
	MaxFilesToDehydrate int

	// ScanInterval is how often to scan for files to dehydrate.
	ScanInterval time.Duration
}

// DefaultDehydrationPolicy returns a reasonable default policy.
func DefaultDehydrationPolicy() DehydrationPolicy {
	return DehydrationPolicy{
		Enabled:             false, // Disabled by default
		MaxAgeDays:          30,    // 30 days
		MinFileSize:         1024 * 1024, // 1MB minimum
		ExcludePatterns:     []string{},
		MaxFilesToDehydrate: 100,
		ScanInterval:        time.Hour,
	}
}

// DehydrationStats tracks dehydration statistics.
type DehydrationStats struct {
	LastScanTime      time.Time
	FilesScanned      int64
	FilesDehydrated   int64
	BytesFreed        int64
	Errors            int64
}

// HydratedFileInfo contains information about a hydrated file.
type HydratedFileInfo struct {
	Path           string    // Relative path from sync root
	FullPath       string    // Full filesystem path
	Size           int64     // File size in bytes
	LastAccessTime time.Time // Last access time
	ModTime        time.Time // Modification time
	DaysSinceAccess int      // Days since last access
}

// NewDehydrationManager creates a new dehydration manager.
func NewDehydrationManager(syncRoot *SyncRootManager, policy DehydrationPolicy, logger *zap.Logger) *DehydrationManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &DehydrationManager{
		syncRoot: syncRoot,
		policy:   policy,
		logger:   logger,
	}
}

// SetPolicy updates the dehydration policy.
func (dm *DehydrationManager) SetPolicy(policy DehydrationPolicy) {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.policy = policy
}

// GetPolicy returns the current dehydration policy.
func (dm *DehydrationManager) GetPolicy() DehydrationPolicy {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.policy
}

// GetStats returns the current dehydration statistics.
func (dm *DehydrationManager) GetStats() DehydrationStats {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.stats
}

// Start begins automatic dehydration scanning.
func (dm *DehydrationManager) Start(ctx context.Context) error {
	dm.mu.Lock()
	if dm.running {
		dm.mu.Unlock()
		return fmt.Errorf("dehydration manager already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	dm.cancel = cancel
	dm.running = true
	dm.mu.Unlock()

	go dm.scanLoop(ctx)

	dm.logger.Info("dehydration manager started",
		zap.Int("max_age_days", dm.policy.MaxAgeDays),
		zap.Duration("scan_interval", dm.policy.ScanInterval),
	)

	return nil
}

// Stop stops automatic dehydration scanning.
func (dm *DehydrationManager) Stop() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if dm.cancel != nil {
		dm.cancel()
		dm.cancel = nil
	}
	dm.running = false

	dm.logger.Info("dehydration manager stopped")
}

// IsRunning returns whether the manager is running.
func (dm *DehydrationManager) IsRunning() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return dm.running
}

// scanLoop periodically scans for files to dehydrate.
func (dm *DehydrationManager) scanLoop(ctx context.Context) {
	// Initial scan after a short delay
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			dm.mu.RLock()
			policy := dm.policy
			dm.mu.RUnlock()

			if policy.Enabled {
				dm.runScan(ctx)
			}

			timer.Reset(policy.ScanInterval)
		}
	}
}

// runScan performs a single dehydration scan.
func (dm *DehydrationManager) runScan(ctx context.Context) {
	dm.logger.Debug("starting dehydration scan")

	dm.mu.Lock()
	dm.stats.LastScanTime = time.Now()
	policy := dm.policy
	dm.mu.Unlock()

	// Find hydrated files
	hydratedFiles, err := dm.ScanHydratedFiles(ctx)
	if err != nil {
		dm.logger.Error("failed to scan hydrated files", zap.Error(err))
		dm.mu.Lock()
		dm.stats.Errors++
		dm.mu.Unlock()
		return
	}

	dm.mu.Lock()
	dm.stats.FilesScanned = int64(len(hydratedFiles))
	dm.mu.Unlock()

	// Filter files eligible for dehydration
	eligible := dm.filterEligibleFiles(hydratedFiles, policy)

	dm.logger.Info("dehydration scan complete",
		zap.Int("scanned", len(hydratedFiles)),
		zap.Int("eligible", len(eligible)),
	)

	// Dehydrate eligible files
	count := 0
	for _, file := range eligible {
		if ctx.Err() != nil {
			break
		}

		if policy.MaxFilesToDehydrate > 0 && count >= policy.MaxFilesToDehydrate {
			break
		}

		if err := dm.DehydrateFile(ctx, file.Path); err != nil {
			dm.logger.Warn("failed to dehydrate file",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			dm.mu.Lock()
			dm.stats.Errors++
			dm.mu.Unlock()
			continue
		}

		dm.mu.Lock()
		dm.stats.FilesDehydrated++
		dm.stats.BytesFreed += file.Size
		dm.mu.Unlock()

		count++
	}

	if count > 0 {
		dm.logger.Info("dehydrated files",
			zap.Int("count", count),
		)
	}
}

