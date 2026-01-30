// Package app provides the sync manager that coordinates sync operations.
package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cloudfiles"
	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// SyncManager coordinates sync operations between jobs and the sync engine.
type SyncManager struct {
	app    *App
	engine *syncpkg.Engine
	logger *zap.Logger

	mu         sync.RWMutex
	running    map[int64]context.CancelFunc // Job ID -> cancel func
	ctx        context.Context
	cancel     context.CancelFunc

	// Cloud Files (Files On Demand) providers per job
	providersMu sync.RWMutex
	providers   map[int64]*cloudfiles.CloudFilesProvider
}

// NewSyncManager creates a new sync manager.
func NewSyncManager(app *App, db *database.DB, logger *zap.Logger) (*SyncManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create config for engine
	cfg := createDefaultConfig()

	// Create sync engine
	engine, err := syncpkg.NewEngine(cfg, db, logger.Named("engine"))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create sync engine: %w", err)
	}

	return &SyncManager{
		app:       app,
		engine:    engine,
		logger:    logger,
		running:   make(map[int64]context.CancelFunc),
		providers: make(map[int64]*cloudfiles.CloudFilesProvider),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// createDefaultConfig creates a default config for the sync engine.
func createDefaultConfig() *config.Config {
	return &config.Config{
		Sync: config.SyncConfig{
			DefaultMode:               "mirror",
			DefaultConflictResolution: "recent",
			Performance: config.PerformanceConfig{
				ParallelTransfers: 4,
				BufferSizeMB:      8,
				HashAlgorithm:     "sha256",
			},
		},
		Logging: config.LoggingConfig{
			Levels: config.LogLevelsConfig{
				Console: "info",
				File:    "debug",
			},
		},
	}
}

// ExecuteSync runs a sync for the given job.
func (m *SyncManager) ExecuteSync(job *SyncJob) error {
	// Check if already running
	m.mu.Lock()
	if _, running := m.running[job.ID]; running {
		m.mu.Unlock()
		m.logger.Warn("Sync already in progress", zap.String("name", job.Name))
		return fmt.Errorf("sync already in progress for job %s", job.Name)
	}

	// Create cancellable context
	syncCtx, cancel := context.WithCancel(m.ctx)
	m.running[job.ID] = cancel
	m.mu.Unlock()

	// Notify watcher that sync is starting (prevents sync loops)
	m.app.SetWatcherSyncActive(job.ID, true)

	// Ensure cleanup
	defer func() {
		m.mu.Lock()
		delete(m.running, job.ID)
		m.mu.Unlock()
		// Notify watcher that sync is done (starts cooldown)
		m.app.SetWatcherSyncActive(job.ID, false)
	}()

	// Update job status
	m.updateJobStatus(job, JobStatusSyncing)
	m.app.SetSyncing(true)
	m.app.SetStatus("Syncing " + job.Name)

	// Notify sync start
	if m.app.notifier != nil {
		m.app.notifier.SyncStarted(job.Name)
	}

	m.logger.Info("Starting sync",
		zap.String("name", job.Name),
		zap.String("local", job.LocalPath),
		zap.String("remote", job.FullRemotePath()),
	)

	// Create sync request
	req := &syncpkg.SyncRequest{
		JobID:              job.ID,
		LocalPath:          job.LocalPath,
		RemotePath:         job.FullRemotePath(), // Full UNC path: \\host\share\path
		Mode:               job.Mode,
		ConflictResolution: job.ConflictResolution,
		DryRun:             false,
		ProgressCallback:   m.createProgressCallback(job),
		FilesOnDemand:      job.FilesOnDemand,
	}

	// Set up Files On Demand if enabled
	if job.FilesOnDemand {
		provider, err := m.getOrCreateProvider(job)
		if err != nil {
			m.logger.Error("Failed to initialize Files On Demand provider",
				zap.String("job", job.Name),
				zap.Error(err),
			)
			// Continue without Files On Demand
			req.FilesOnDemand = false
		} else {
			// Set placeholder callback
			req.PlaceholderCallback = m.createPlaceholderCallback(provider, job)
		}
	}

	// Execute sync
	startTime := time.Now()
	result, err := m.engine.Sync(syncCtx, req)
	duration := time.Since(startTime)

	// Update app state
	m.app.SetSyncing(false)

	if err != nil {
		m.logger.Error("Sync failed",
			zap.String("name", job.Name),
			zap.Error(err),
			zap.Duration("duration", duration),
		)

		m.updateJobStatus(job, JobStatusFailed)
		m.app.SetStatus("Sync failed: " + job.Name)

		if m.app.notifier != nil {
			m.app.notifier.SyncFailed(job.Name, err)
		}

		return err
	}

	// Determine final status
	var finalStatus JobStatus
	switch result.Status {
	case syncpkg.SyncStatusSuccess:
		finalStatus = JobStatusSuccess
	case syncpkg.SyncStatusPartial:
		finalStatus = JobStatusPartial
	case syncpkg.SyncStatusFailed:
		finalStatus = JobStatusFailed
	}

	m.updateJobStatus(job, finalStatus)
	job.LastSync = time.Now()

	m.logger.Info("Sync completed",
		zap.String("name", job.Name),
		zap.String("status", string(result.Status)),
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("downloaded", result.FilesDownloaded),
		zap.Int("errors", result.FilesError),
		zap.Duration("duration", duration),
	)

	// Notify completion
	if m.app.notifier != nil {
		if result.FilesError > 0 {
			m.app.notifier.SyncPartial(job.Name,
				result.FilesUploaded+result.FilesDownloaded,
				result.FilesError)
		} else if result.ConflictsFound > 0 {
			m.app.notifier.ConflictDetected(job.Name, result.ConflictsFound)
		} else {
			m.app.notifier.SyncCompleted(job.Name,
				result.FilesUploaded+result.FilesDownloaded)
		}
	}

	// Update status message
	if result.FilesUploaded+result.FilesDownloaded > 0 {
		m.app.SetStatus(fmt.Sprintf("Synced %d files", result.FilesUploaded+result.FilesDownloaded))
	} else {
		m.app.SetStatus("Up to date")
	}

	return nil
}

// CancelSync cancels a running sync.
func (m *SyncManager) CancelSync(jobID int64) bool {
	m.mu.Lock()
	cancel, exists := m.running[jobID]
	m.mu.Unlock()

	if exists {
		cancel()
		m.logger.Info("Sync cancelled", zap.Int64("job_id", jobID))
		return true
	}
	return false
}

// CancelAllSyncs cancels all running syncs.
func (m *SyncManager) CancelAllSyncs() int {
	m.mu.Lock()
	count := len(m.running)
	for jobID, cancel := range m.running {
		m.logger.Info("Cancelling sync", zap.Int64("job_id", jobID))
		cancel()
	}
	m.mu.Unlock()

	if count > 0 {
		m.logger.Info("All syncs cancelled", zap.Int("count", count))
	}
	return count
}

// GetRunningSyncJobIDs returns the IDs of all currently running sync jobs.
func (m *SyncManager) GetRunningSyncJobIDs() []int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]int64, 0, len(m.running))
	for jobID := range m.running {
		ids = append(ids, jobID)
	}
	return ids
}

// IsSyncing returns whether a job is currently syncing.
func (m *SyncManager) IsSyncing(jobID int64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, running := m.running[jobID]
	return running
}

// IsAnySyncing returns whether any sync is running.
func (m *SyncManager) IsAnySyncing() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.running) > 0
}

// Close shuts down the sync manager.
func (m *SyncManager) Close() error {
	m.logger.Info("Sync manager shutting down")

	// Cancel all running syncs
	m.mu.Lock()
	for jobID, cancel := range m.running {
		m.logger.Info("Cancelling running sync", zap.Int64("job_id", jobID))
		cancel()
	}
	m.mu.Unlock()

	// Close all Cloud Files providers
	m.closeAllProviders()

	// Cancel manager context
	m.cancel()

	// Close engine
	if m.engine != nil {
		return m.engine.Close()
	}

	return nil
}

// createProgressCallback creates a progress callback for the given job.
func (m *SyncManager) createProgressCallback(job *SyncJob) syncpkg.ProgressCallback {
	return func(progress *syncpkg.SyncProgress) {
		// Update tray status with progress
		var status string
		switch progress.Phase {
		case "preparation":
			status = fmt.Sprintf("Preparing %s...", job.Name)
		case "scanning":
			status = fmt.Sprintf("Scanning %s...", job.Name)
		case "detecting":
			status = fmt.Sprintf("Detecting changes in %s...", job.Name)
		case "executing":
			status = formatSyncProgress(job.Name, progress)
		case "finalizing":
			status = fmt.Sprintf("Finalizing %s...", job.Name)
		default:
			status = fmt.Sprintf("Syncing %s...", job.Name)
		}

		m.app.SetStatus(status)

		m.logger.Debug("Sync progress",
			zap.String("job", job.Name),
			zap.String("phase", progress.Phase),
			zap.Float64("percent", progress.Percentage),
			zap.Int("processed", progress.FilesProcessed),
			zap.Int("total", progress.FilesTotal),
			zap.Int64("bytes", progress.BytesTransferred),
			zap.Int64("bytesTotal", progress.BytesTotal),
		)
	}
}

// formatSyncProgress formats the sync progress for display.
func formatSyncProgress(jobName string, p *syncpkg.SyncProgress) string {
	if p.FilesTotal == 0 {
		return fmt.Sprintf("Syncing %s...", jobName)
	}

	// Build progress string with files count
	filesPart := fmt.Sprintf("%d/%d files", p.FilesProcessed, p.FilesTotal)

	// Add bytes if available
	if p.BytesTotal > 0 {
		transferred := formatBytes(p.BytesTransferred)
		total := formatBytes(p.BytesTotal)
		return fmt.Sprintf("Syncing %s: %s (%s/%s)", jobName, filesPart, transferred, total)
	}

	return fmt.Sprintf("Syncing %s: %s", jobName, filesPart)
}


// updateJobStatus updates the job's status in memory.
func (m *SyncManager) updateJobStatus(job *SyncJob, status JobStatus) {
	job.LastStatus = status
	// Refresh UI if settings window is open
	if m.app.settings != nil {
		m.app.settings.RefreshJobList()
	}
}

// ExecuteSyncAndWait runs a sync for the given job and blocks until completion.
// Unlike ExecuteSync, this method waits for the sync to finish and returns
// only when the sync is complete or cancelled via context.
func (m *SyncManager) ExecuteSyncAndWait(ctx context.Context, job *SyncJob) error {
	// Check if already running
	m.mu.Lock()
	if _, running := m.running[job.ID]; running {
		m.mu.Unlock()
		// Wait for existing sync to complete
		return m.waitForSync(ctx, job.ID)
	}

	// Create cancellable context that respects both parent ctx and manager ctx
	syncCtx, cancel := context.WithCancel(m.ctx)
	m.running[job.ID] = cancel
	m.mu.Unlock()

	// Notify watcher that sync is starting (prevents sync loops)
	m.app.SetWatcherSyncActive(job.ID, true)

	// Ensure cleanup
	defer func() {
		m.mu.Lock()
		delete(m.running, job.ID)
		m.mu.Unlock()
		// Notify watcher that sync is done (starts cooldown)
		m.app.SetWatcherSyncActive(job.ID, false)
	}()

	// Also cancel if parent context is done
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-syncCtx.Done():
		}
	}()

	// Update job status
	m.updateJobStatus(job, JobStatusSyncing)
	m.app.SetSyncing(true)
	m.app.SetStatus("Syncing " + job.Name)

	m.logger.Info("Starting sync (with wait)",
		zap.String("name", job.Name),
		zap.String("local", job.LocalPath),
		zap.String("remote", job.FullRemotePath()),
	)

	// Create sync request
	req := &syncpkg.SyncRequest{
		JobID:              job.ID,
		LocalPath:          job.LocalPath,
		RemotePath:         job.FullRemotePath(),
		Mode:               job.Mode,
		ConflictResolution: job.ConflictResolution,
		DryRun:             false,
		ProgressCallback:   m.createProgressCallback(job),
		FilesOnDemand:      job.FilesOnDemand,
	}

	// Set up Files On Demand if enabled
	if job.FilesOnDemand {
		provider, err := m.getOrCreateProvider(job)
		if err != nil {
			m.logger.Error("Failed to initialize Files On Demand provider",
				zap.String("job", job.Name),
				zap.Error(err),
			)
			// Continue without Files On Demand
			req.FilesOnDemand = false
		} else {
			// Set placeholder callback
			req.PlaceholderCallback = m.createPlaceholderCallback(provider, job)
		}
	}

	// Execute sync
	startTime := time.Now()
	result, err := m.engine.Sync(syncCtx, req)
	duration := time.Since(startTime)

	// Update app state
	m.app.SetSyncing(false)

	if err != nil {
		m.logger.Error("Sync failed",
			zap.String("name", job.Name),
			zap.Error(err),
			zap.Duration("duration", duration),
		)
		m.updateJobStatus(job, JobStatusFailed)
		m.app.SetStatus("Sync failed: " + job.Name)
		return err
	}

	// Determine final status
	var finalStatus JobStatus
	switch result.Status {
	case syncpkg.SyncStatusSuccess:
		finalStatus = JobStatusSuccess
	case syncpkg.SyncStatusPartial:
		finalStatus = JobStatusPartial
	case syncpkg.SyncStatusFailed:
		finalStatus = JobStatusFailed
	}

	m.updateJobStatus(job, finalStatus)
	job.LastSync = time.Now()

	m.logger.Info("Sync completed",
		zap.String("name", job.Name),
		zap.String("status", string(result.Status)),
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("downloaded", result.FilesDownloaded),
		zap.Int("errors", result.FilesError),
		zap.Duration("duration", duration),
	)

	// Update status message
	if result.FilesUploaded+result.FilesDownloaded > 0 {
		m.app.SetStatus(fmt.Sprintf("Synced %d files", result.FilesUploaded+result.FilesDownloaded))
	} else {
		m.app.SetStatus("Up to date")
	}

	// Return error if there were file errors
	if result.FilesError > 0 {
		return fmt.Errorf("sync completed with %d errors", result.FilesError)
	}

	return nil
}

// waitForSync waits for an existing sync to complete.
func (m *SyncManager) waitForSync(ctx context.Context, jobID int64) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			m.mu.RLock()
			_, running := m.running[jobID]
			m.mu.RUnlock()
			if !running {
				return nil
			}
		}
	}
}
