// Package app provides the scheduler for periodic sync operations.
package app

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler manages scheduled sync operations for all jobs.
type Scheduler struct {
	app    *App
	logger *zap.Logger

	mu       sync.RWMutex
	timers   map[int64]*time.Timer // Job ID -> Timer
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewScheduler creates a new scheduler instance.
func NewScheduler(app *App, logger *zap.Logger) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		app:    app,
		logger: logger,
		timers: make(map[int64]*time.Timer),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins the scheduler and schedules all enabled jobs.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("Scheduler starting")

	// Schedule all enabled jobs (scheduled or realtime mode)
	jobs := s.app.GetSyncJobs()
	for _, job := range jobs {
		if job.Enabled && s.shouldSchedule(job.TriggerMode) {
			s.ScheduleJob(job)
		}
	}

	s.logger.Info("Scheduler started", zap.Int("scheduled_jobs", len(s.timers)))
}

// shouldSchedule returns true if the trigger mode requires scheduling.
func (s *Scheduler) shouldSchedule(mode SyncTriggerMode) bool {
	switch mode {
	case SyncTriggerManual:
		return false
	case SyncTriggerRealtime:
		return true // Realtime also schedules for remote checking
	default:
		return true // All interval modes
	}
}

// Stop stops the scheduler and cancels all pending timers.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.logger.Info("Scheduler stopping")

	// Cancel context
	s.cancel()

	// Stop all timers
	for id, timer := range s.timers {
		timer.Stop()
		delete(s.timers, id)
	}

	s.running = false
	s.logger.Info("Scheduler stopped")
}

// ScheduleJob schedules a specific job based on its trigger mode.
func (s *Scheduler) ScheduleJob(job *SyncJob) {
	if job.TriggerMode == SyncTriggerManual {
		s.logger.Debug("Job is manual, not scheduling", zap.String("name", job.Name))
		return
	}

	interval := s.getInterval(job.TriggerMode)
	if interval == 0 {
		s.logger.Warn("Invalid trigger mode", zap.String("mode", string(job.TriggerMode)))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel existing timer if any
	if existing, ok := s.timers[job.ID]; ok {
		existing.Stop()
		delete(s.timers, job.ID)
	}

	// Calculate next run time
	nextRun := time.Now().Add(interval)

	// Update job's next sync time
	s.updateJobNextSync(job.ID, nextRun)

	// Create new timer
	timer := time.AfterFunc(interval, func() {
		s.onJobTimer(job.ID)
	})
	s.timers[job.ID] = timer

	s.logger.Info("Job scheduled",
		zap.String("name", job.Name),
		zap.String("mode", string(job.TriggerMode)),
		zap.Duration("interval", interval),
		zap.Time("next_run", nextRun),
	)
}

// RescheduleJob reschedules a job (e.g., after trigger mode change).
func (s *Scheduler) RescheduleJob(job *SyncJob) {
	s.mu.Lock()
	if existing, ok := s.timers[job.ID]; ok {
		existing.Stop()
		delete(s.timers, job.ID)
	}
	s.mu.Unlock()

	if job.Enabled && s.shouldSchedule(job.TriggerMode) {
		s.ScheduleJob(job)
	}
}

// UnscheduleJob removes a job from the scheduler.
func (s *Scheduler) UnscheduleJob(jobID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if timer, ok := s.timers[jobID]; ok {
		timer.Stop()
		delete(s.timers, jobID)
		s.logger.Info("Job unscheduled", zap.Int64("job_id", jobID))
	}
}

// TriggerNow triggers a sync immediately for a job.
func (s *Scheduler) TriggerNow(jobID int64) {
	s.logger.Info("Manual sync triggered", zap.Int64("job_id", jobID))
	go s.executeSync(jobID)
}

// onJobTimer is called when a job's timer fires.
func (s *Scheduler) onJobTimer(jobID int64) {
	// Check if scheduler is still running
	s.mu.RLock()
	if !s.running {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	// Execute sync
	s.executeSync(jobID)

	// Reschedule the job
	s.mu.RLock()
	jobs := s.app.GetSyncJobs()
	s.mu.RUnlock()

	for _, job := range jobs {
		if job.ID == jobID && job.Enabled && s.shouldSchedule(job.TriggerMode) {
			s.ScheduleJob(job)
			break
		}
	}
}

// executeSync performs the actual sync for a job.
func (s *Scheduler) executeSync(jobID int64) {
	// Check context
	select {
	case <-s.ctx.Done():
		return
	default:
	}

	// Get job
	jobs := s.app.GetSyncJobs()
	var job *SyncJob
	for _, j := range jobs {
		if j.ID == jobID {
			job = j
			break
		}
	}

	if job == nil {
		s.logger.Warn("Job not found for scheduled sync", zap.Int64("job_id", jobID))
		return
	}

	if !job.Enabled {
		s.logger.Debug("Job disabled, skipping scheduled sync", zap.String("name", job.Name))
		return
	}

	// Delegate to app's sync manager
	s.logger.Info("Executing scheduled sync", zap.String("name", job.Name))
	s.app.ExecuteJobSync(jobID)
}

// getInterval returns the sync interval for a trigger mode.
func (s *Scheduler) getInterval(mode SyncTriggerMode) time.Duration {
	switch mode {
	case SyncTrigger5Min:
		return 5 * time.Minute
	case SyncTrigger15Min:
		return 15 * time.Minute
	case SyncTrigger30Min:
		return 30 * time.Minute
	case SyncTrigger1Hour:
		return 1 * time.Hour
	case SyncTriggerRealtime:
		return 5 * time.Minute // Remote check interval for realtime mode
	default:
		return 0
	}
}

// updateJobNextSync updates the job's NextSync field.
func (s *Scheduler) updateJobNextSync(jobID int64, nextRun time.Time) {
	jobs := s.app.GetSyncJobs()
	for _, job := range jobs {
		if job.ID == jobID {
			job.NextSync = nextRun
			break
		}
	}
}

// GetNextRun returns the next scheduled run time for a job.
func (s *Scheduler) GetNextRun(jobID int64) (time.Time, bool) {
	jobs := s.app.GetSyncJobs()
	for _, job := range jobs {
		if job.ID == jobID {
			if job.TriggerMode == SyncTriggerManual || job.NextSync.IsZero() {
				return time.Time{}, false
			}
			return job.NextSync, true
		}
	}
	return time.Time{}, false
}

// IsRunning returns whether the scheduler is active.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// ScheduledJobCount returns the number of currently scheduled jobs.
func (s *Scheduler) ScheduledJobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.timers)
}
