// Package app provides the shutdown manager for Sync & Shutdown functionality.
package app

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ShutdownConfig holds the configuration for a sync & shutdown operation.
type ShutdownConfig struct {
	JobIDs        []int64       // Empty = all enabled jobs
	Timeout       time.Duration // 0 = unlimited
	ForceShutdown bool          // Force shutdown even if sync incomplete
}

// ShutdownState represents the current state of a shutdown operation.
type ShutdownState int

const (
	ShutdownStateIdle ShutdownState = iota
	ShutdownStateSyncing
	ShutdownStateWaitingShutdown
	ShutdownStateCancelled
)

// ShutdownProgress holds progress information for the shutdown operation.
type ShutdownProgress struct {
	State           ShutdownState
	CurrentJob      int      // 1-based index of current job
	TotalJobs       int      // Total number of jobs to sync
	CurrentJobName  string   // Name of the current job being synced
	ElapsedTime     time.Duration
	RemainingTime   time.Duration // Only valid if timeout > 0
	CompletedJobs   []string      // Names of completed jobs
	FailedJobs      []string      // Names of failed jobs
	ShutdownPending bool          // True if shutdown command has been issued
	ShutdownSeconds int           // Seconds until shutdown
}

// ProgressCallback is called when shutdown progress updates.
type ShutdownProgressCallback func(*ShutdownProgress)

// ShutdownManager manages the sync & shutdown process.
type ShutdownManager struct {
	app    *App
	logger *zap.Logger

	mu       sync.RWMutex
	active   bool
	config   *ShutdownConfig
	ctx      context.Context
	cancel   context.CancelFunc
	progress *ShutdownProgress
	callback ShutdownProgressCallback
	start    time.Time
}

// NewShutdownManager creates a new shutdown manager.
func NewShutdownManager(app *App, logger *zap.Logger) *ShutdownManager {
	return &ShutdownManager{
		app:    app,
		logger: logger,
	}
}

// Start begins the sync & shutdown process.
func (m *ShutdownManager) Start(config *ShutdownConfig, callback ShutdownProgressCallback) error {
	m.mu.Lock()
	if m.active {
		m.mu.Unlock()
		return fmt.Errorf("shutdown operation already in progress")
	}

	m.active = true
	m.config = config
	m.callback = callback
	m.ctx, m.cancel = context.WithCancel(context.Background())
	m.start = time.Now()
	m.progress = &ShutdownProgress{
		State:         ShutdownStateSyncing,
		CompletedJobs: make([]string, 0),
		FailedJobs:    make([]string, 0),
	}
	m.mu.Unlock()

	m.logger.Info("Starting sync & shutdown operation",
		zap.Int("job_count", len(config.JobIDs)),
		zap.Duration("timeout", config.Timeout),
		zap.Bool("force", config.ForceShutdown),
	)

	// Run sync process in background
	go m.runSyncProcess()

	return nil
}

// Cancel cancels the shutdown operation.
func (m *ShutdownManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.active {
		return
	}

	m.logger.Info("Cancelling sync & shutdown operation")

	if m.cancel != nil {
		m.cancel()
	}

	m.progress.State = ShutdownStateCancelled
	m.active = false

	// Cancel any pending Windows shutdown
	m.cancelWindowsShutdown()

	// Notify
	if m.app.notifier != nil {
		m.app.notifier.ShutdownCancelled()
	}

	m.notifyProgress()
}

// IsActive returns whether a shutdown operation is in progress.
func (m *ShutdownManager) IsActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active
}

// GetProgress returns the current progress.
func (m *ShutdownManager) GetProgress() *ShutdownProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.progress == nil {
		return &ShutdownProgress{State: ShutdownStateIdle}
	}
	// Return a copy
	p := *m.progress
	return &p
}

// runSyncProcess executes the sync jobs and then triggers shutdown.
func (m *ShutdownManager) runSyncProcess() {
	defer func() {
		m.mu.Lock()
		m.active = false
		m.mu.Unlock()
	}()

	// Get jobs to sync
	jobs := m.getJobsToSync()
	if len(jobs) == 0 {
		m.logger.Warn("No jobs to sync, proceeding to shutdown")
		m.executeShutdown(false)
		return
	}

	m.mu.Lock()
	m.progress.TotalJobs = len(jobs)
	m.mu.Unlock()

	m.logger.Info("Syncing jobs before shutdown", zap.Int("count", len(jobs)))

	// Create context with timeout if specified
	ctx := m.ctx
	if m.config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(m.ctx, m.config.Timeout)
		defer cancel()
	}

	// Start elapsed time updater
	stopTimer := m.startElapsedTimer()
	defer stopTimer()

	// Sync each job sequentially
	allSuccess := true
	for i, job := range jobs {
		// Check for cancellation
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				m.logger.Warn("Timeout reached during sync")
				if m.config.ForceShutdown {
					m.logger.Info("Force shutdown enabled, proceeding despite timeout")
					m.executeShutdown(true)
				} else {
					m.logger.Info("Force shutdown disabled, cancelling operation")
					m.Cancel()
				}
			}
			return
		default:
		}

		// Update progress
		m.mu.Lock()
		m.progress.CurrentJob = i + 1
		m.progress.CurrentJobName = job.Name
		m.mu.Unlock()
		m.notifyProgress()

		m.logger.Info("Syncing job for shutdown",
			zap.String("name", job.Name),
			zap.Int("index", i+1),
			zap.Int("total", len(jobs)),
		)

		// Execute sync
		err := m.syncJob(ctx, job)

		m.mu.Lock()
		if err != nil {
			m.logger.Error("Job sync failed",
				zap.String("name", job.Name),
				zap.Error(err),
			)
			m.progress.FailedJobs = append(m.progress.FailedJobs, job.Name)
			allSuccess = false
		} else {
			m.logger.Info("Job sync completed", zap.String("name", job.Name))
			m.progress.CompletedJobs = append(m.progress.CompletedJobs, job.Name)
		}
		m.mu.Unlock()
		m.notifyProgress()
	}

	// All syncs completed (or attempted)
	if !allSuccess && !m.config.ForceShutdown {
		m.logger.Warn("Some syncs failed and force shutdown is disabled")
		m.mu.Lock()
		m.progress.State = ShutdownStateCancelled
		m.mu.Unlock()
		m.notifyProgress()

		if m.app.notifier != nil {
			m.app.notifier.Send("Sync & Shutdown",
				"Shutdown cancelled: some syncs failed",
				NotifyWarning)
		}
		return
	}

	// Proceed to shutdown
	m.executeShutdown(false)
}

// getJobsToSync returns the jobs to sync based on config.
func (m *ShutdownManager) getJobsToSync() []*SyncJob {
	allJobs := m.app.GetSyncJobs()

	// If specific job IDs are provided, filter to those
	if len(m.config.JobIDs) > 0 {
		jobIDSet := make(map[int64]bool)
		for _, id := range m.config.JobIDs {
			jobIDSet[id] = true
		}

		filtered := make([]*SyncJob, 0)
		for _, job := range allJobs {
			if jobIDSet[job.ID] && job.Enabled {
				filtered = append(filtered, job)
			}
		}
		return filtered
	}

	// Otherwise, sync all enabled jobs
	enabled := make([]*SyncJob, 0)
	for _, job := range allJobs {
		if job.Enabled {
			enabled = append(enabled, job)
		}
	}
	return enabled
}

// syncJob syncs a single job and waits for completion.
func (m *ShutdownManager) syncJob(ctx context.Context, job *SyncJob) error {
	if m.app.syncManager == nil {
		return fmt.Errorf("sync manager not available")
	}

	// Use the sync manager to execute sync with wait
	return m.app.syncManager.ExecuteSyncAndWait(ctx, job)
}

// executeShutdown triggers Windows shutdown.
func (m *ShutdownManager) executeShutdown(force bool) {
	m.mu.Lock()
	m.progress.State = ShutdownStateWaitingShutdown
	m.progress.ShutdownPending = true
	m.progress.ShutdownSeconds = 30
	m.mu.Unlock()
	m.notifyProgress()

	m.logger.Info("Initiating Windows shutdown",
		zap.Bool("force", force),
		zap.Int("delay_seconds", 30),
	)

	// Notify user
	if m.app.notifier != nil {
		m.app.notifier.ShutdownPending(30)
	}

	// Execute Windows shutdown command
	// shutdown /s /t 30 - shutdown with 30 second delay
	// shutdown /s /t 30 /f - force close applications
	args := []string{"/s", "/t", "30"}
	if force {
		args = append(args, "/f")
	}

	cmd := exec.Command("shutdown", args...)
	if err := cmd.Run(); err != nil {
		m.logger.Error("Failed to execute shutdown command", zap.Error(err))
		if m.app.notifier != nil {
			m.app.notifier.Send("Shutdown Failed",
				fmt.Sprintf("Failed to initiate shutdown: %v", err),
				NotifyError)
		}
	}
}

// cancelWindowsShutdown cancels a pending Windows shutdown.
func (m *ShutdownManager) cancelWindowsShutdown() {
	cmd := exec.Command("shutdown", "/a")
	if err := cmd.Run(); err != nil {
		// It's ok if this fails - might not be a pending shutdown
		m.logger.Debug("Failed to cancel Windows shutdown (may not be pending)", zap.Error(err))
	}
}

// startElapsedTimer starts a goroutine that updates elapsed/remaining time.
func (m *ShutdownManager) startElapsedTimer() func() {
	ticker := time.NewTicker(1 * time.Second)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				m.mu.Lock()
				if m.progress != nil {
					m.progress.ElapsedTime = time.Since(m.start)
					if m.config.Timeout > 0 {
						remaining := m.config.Timeout - m.progress.ElapsedTime
						if remaining < 0 {
							remaining = 0
						}
						m.progress.RemainingTime = remaining
					}
				}
				m.mu.Unlock()
				m.notifyProgress()
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		close(done)
	}
}

// notifyProgress calls the progress callback if set.
func (m *ShutdownManager) notifyProgress() {
	m.mu.RLock()
	callback := m.callback
	progress := m.progress
	m.mu.RUnlock()

	if callback != nil && progress != nil {
		// Send a copy
		p := *progress
		callback(&p)
	}
}
