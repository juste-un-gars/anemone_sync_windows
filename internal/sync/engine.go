package sync

import (
	"context"
	"fmt"
	"sync"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"github.com/juste-un-gars/anemone_sync_windows/internal/scanner"
	"go.uber.org/zap"
)

// Engine is the main sync orchestrator
type Engine struct {
	db     *database.DB
	config *config.Config
	logger *zap.Logger

	// Components
	scanner  *scanner.Scanner
	cache    *cache.CacheManager
	detector *cache.ChangeDetector
	executor *Executor

	// State
	mu      sync.RWMutex
	syncing map[int64]context.CancelFunc // Maps job ID to cancel function
	closed  bool
}

// NewEngine creates a new sync engine
func NewEngine(cfg *config.Config, db *database.DB, logger *zap.Logger) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create scanner
	scan, err := scanner.NewScanner(cfg, db, logger.Named("scanner"))
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner: %w", err)
	}

	// Create cache manager
	cacheManager := cache.NewCacheManager(db, logger.Named("cache"))

	// Create change detector
	changeDetector := cache.NewChangeDetector(cacheManager, logger.Named("detector"))

	// Create executor
	bufferSizeMB := cfg.Sync.Performance.BufferSizeMB
	executor := NewExecutor(bufferSizeMB, logger.Named("executor"))

	return &Engine{
		db:       db,
		config:   cfg,
		logger:   logger,
		scanner:  scan,
		cache:    cacheManager,
		detector: changeDetector,
		executor: executor,
		syncing:  make(map[int64]context.CancelFunc),
		closed:   false,
	}, nil
}

// Sync performs a synchronization for the given request
func (e *Engine) Sync(ctx context.Context, req *SyncRequest) (*SyncResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sync request: %w", err)
	}

	// Check if engine is closed
	e.mu.RLock()
	if e.closed {
		e.mu.RUnlock()
		return nil, ErrEngineClosed
	}
	e.mu.RUnlock()

	// Check if already syncing
	e.mu.Lock()
	if _, exists := e.syncing[req.JobID]; exists {
		e.mu.Unlock()
		return nil, ErrSyncInProgress
	}

	// Create cancellable context
	syncCtx, cancel := context.WithCancel(ctx)
	e.syncing[req.JobID] = cancel
	e.mu.Unlock()

	// Ensure cleanup
	defer func() {
		e.mu.Lock()
		delete(e.syncing, req.JobID)
		e.mu.Unlock()
	}()

	// Initialize result
	result := NewSyncResult(req.JobID)

	e.logger.Info("starting sync",
		zap.Int64("job_id", req.JobID),
		zap.String("mode", string(req.Mode)),
		zap.String("local_path", req.LocalPath),
		zap.String("remote_path", req.RemotePath),
	)

	// Execute sync phases
	if err := e.executeSync(syncCtx, req, result); err != nil {
		e.logger.Error("sync failed", zap.Error(err))
		result.Status = SyncStatusFailed
		result.Finalize()
		return result, err
	}

	// Finalize result
	result.Finalize()

	e.logger.Info("sync completed",
		zap.Int64("job_id", req.JobID),
		zap.String("status", string(result.Status)),
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("downloaded", result.FilesDownloaded),
		zap.Int("placeholders", result.PlaceholdersCreated),
		zap.Int("deleted", result.FilesDeleted),
		zap.Int("errors", result.FilesError),
		zap.Int("conflicts", result.ConflictsFound),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// executeSync executes the 5 phases of synchronization
func (e *Engine) executeSync(ctx context.Context, req *SyncRequest, result *SyncResult) error {
	// Phase 1: Preparation
	e.reportProgress(req, &SyncProgress{
		Phase:      "preparation",
		Message:    "Preparing synchronization...",
		Percentage: 0,
	})

	smbClient, job, err := e.prepareSync(ctx, req)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}
	defer smbClient.Disconnect()

	// Phase 2: Scanning
	e.reportProgress(req, &SyncProgress{
		Phase:      "scanning",
		Message:    "Scanning files...",
		Percentage: 5,
	})

	localFiles, remoteFiles, cachedFiles, err := e.scanFiles(ctx, req, smbClient)
	if err != nil {
		return fmt.Errorf("scanning failed: %w", err)
	}

	result.TotalFiles = len(localFiles) + len(remoteFiles)

	// Phase 3: Detection
	e.reportProgress(req, &SyncProgress{
		Phase:          "detecting",
		Message:        "Detecting changes...",
		Percentage:     25,
		FilesTotal:     result.TotalFiles,
		FilesProcessed: 0,
	})

	decisions, conflicts, err := e.detectChanges(ctx, req, localFiles, remoteFiles, cachedFiles)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// Add conflicts to result
	for _, conflict := range conflicts {
		result.AddConflict(conflict)
	}

	e.logger.Info("change detection completed",
		zap.Int("actions", len(decisions)),
		zap.Int("conflicts", len(conflicts)),
	)

	// Phase 4: Execution
	if len(decisions) > 0 && !req.DryRun {
		e.reportProgress(req, &SyncProgress{
			Phase:          "executing",
			Message:        "Executing sync actions...",
			Percentage:     35,
			FilesTotal:     len(decisions),
			FilesProcessed: 0,
		})

		// Separate downloads from other actions if Files On Demand is enabled
		var downloadDecisions []*cache.SyncDecision
		var otherDecisions []*cache.SyncDecision
		if req.FilesOnDemand && req.PlaceholderCallback != nil {
			for _, d := range decisions {
				if d.Action == cache.ActionDownload {
					downloadDecisions = append(downloadDecisions, d)
				} else {
					otherDecisions = append(otherDecisions, d)
				}
			}
		} else {
			otherDecisions = decisions
		}

		// Create placeholders for downloads in Files On Demand mode
		if len(downloadDecisions) > 0 {
			e.logger.Info("creating placeholders instead of downloading (Files On Demand mode)",
				zap.Int("count", len(downloadDecisions)),
			)

			// Convert to PlaceholderFileInfo
			placeholderFiles := make([]PlaceholderFileInfo, len(downloadDecisions))
			for i, d := range downloadDecisions {
				placeholderFiles[i] = PlaceholderFileInfo{
					RelativePath: d.LocalPath,
					Size:         d.RemoteInfo.Size,
					ModTime:      d.RemoteInfo.MTime.Unix(),
				}
			}

			// Call placeholder callback
			created, err := req.PlaceholderCallback(placeholderFiles)
			if err != nil {
				e.logger.Error("failed to create placeholders", zap.Error(err))
				// Continue with other actions
			} else {
				result.PlaceholdersCreated = created
				e.logger.Info("placeholders created",
					zap.Int("count", created),
				)
			}
		}

		// Execute non-download actions (uploads, deletes)
		if len(otherDecisions) > 0 {
			actions, err := e.executeActions(ctx, req, otherDecisions, smbClient, job)
			if err != nil {
				return fmt.Errorf("execution failed: %w", err)
			}

			// Add actions to result
			for _, action := range actions {
				result.AddAction(action)
				if action.Error != nil {
					syncErr := NewSyncError(action.FilePath, string(action.Action), action.Error, 1)
					result.AddError(syncErr)
				}
			}
		}
	} else if req.DryRun {
		e.logger.Info("dry run mode - skipping execution",
			zap.Int("actions", len(decisions)),
		)
	}

	// Phase 5: Finalization
	e.reportProgress(req, &SyncProgress{
		Phase:      "finalizing",
		Message:    "Finalizing sync...",
		Percentage: 95,
	})

	if err := e.finalizeSync(ctx, req, result, job, localFiles, remoteFiles); err != nil {
		e.logger.Error("finalization failed", zap.Error(err))
		// Don't return error, sync already completed
	}

	return nil
}

// reportProgress reports progress to callback if provided
func (e *Engine) reportProgress(req *SyncRequest, progress *SyncProgress) {
	if req.ProgressCallback != nil {
		req.ProgressCallback(progress)
	}
}

// IsSyncing returns whether a job is currently syncing
func (e *Engine) IsSyncing(jobID int64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.syncing[jobID]
	return exists
}

// CancelSync attempts to cancel a running sync
func (e *Engine) CancelSync(jobID int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, exists := e.syncing[jobID]
	if !exists {
		return ErrSyncNotFound
	}

	cancel()
	return nil
}

// Close releases all resources
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	// Cancel all running syncs
	for jobID, cancel := range e.syncing {
		e.logger.Info("cancelling sync on close", zap.Int64("job_id", jobID))
		cancel()
	}

	e.closed = true
	return nil
}
