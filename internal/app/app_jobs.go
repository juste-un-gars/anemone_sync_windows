package app

import (
	"go.uber.org/zap"
)

// --- Sync Jobs Management ---

// GetSyncJobs returns all configured sync jobs.
func (a *App) GetSyncJobs() []*SyncJob {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.syncJobs
}

// AddSyncJob adds a new sync job.
func (a *App) AddSyncJob(job *SyncJob) error {
	// Convert to DB job and save
	dbJob := convertAppJobToDBJob(job)

	if a.db != nil {
		if err := a.db.CreateSyncJob(dbJob); err != nil {
			return err
		}
		job.ID = dbJob.ID
	}

	a.mu.Lock()
	job.LastStatus = JobStatusIdle
	a.syncJobs = append(a.syncJobs, job)
	a.mu.Unlock()

	a.logger.Info("Added sync job", zap.String("name", job.Name), zap.Int64("id", job.ID))

	// Schedule the new job if not manual
	if job.Enabled && job.TriggerMode != SyncTriggerManual {
		if a.scheduler != nil {
			a.scheduler.ScheduleJob(job)
		}
	}

	// Start file watcher if realtime mode
	if job.Enabled && job.TriggerMode == SyncTriggerRealtime {
		if a.watcher != nil {
			a.watcher.WatchJob(job)
		}
	}

	return nil
}

// UpdateSyncJob updates an existing sync job.
func (a *App) UpdateSyncJob(job *SyncJob) error {
	// Persist to database
	if a.db != nil {
		dbJob := convertAppJobToDBJob(job)
		if err := a.db.UpdateSyncJob(dbJob); err != nil {
			return err
		}
	}

	a.mu.Lock()
	for i, j := range a.syncJobs {
		if j.ID == job.ID {
			a.syncJobs[i] = job
			a.mu.Unlock()

			a.logger.Info("Updated sync job", zap.String("name", job.Name), zap.Int64("id", job.ID))

			// Update scheduler and watcher
			if a.scheduler != nil {
				a.scheduler.RescheduleJob(job)
			}
			if a.watcher != nil {
				a.watcher.RewatchJob(job)
			}

			return nil
		}
	}
	a.mu.Unlock()

	return errJobNotFound
}

// DeleteSyncJob removes a sync job.
func (a *App) DeleteSyncJob(id int64) error {
	// Remove from scheduler and watcher first
	if a.scheduler != nil {
		a.scheduler.UnscheduleJob(id)
	}
	if a.watcher != nil {
		a.watcher.UnwatchJob(id)
	}

	// Delete from database
	if a.db != nil {
		if err := a.db.DeleteSyncJob(id); err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i, j := range a.syncJobs {
		if j.ID == id {
			a.syncJobs = append(a.syncJobs[:i], a.syncJobs[i+1:]...)
			a.logger.Info("Deleted sync job", zap.Int64("id", id))
			return nil
		}
	}

	return errJobNotFound
}

// TriggerSyncJob triggers sync for a specific job.
func (a *App) TriggerSyncJob(id int64) {
	if a.IsSyncing() {
		a.logger.Warn("Sync already in progress")
		return
	}

	a.logger.Info("Manual sync triggered for job", zap.Int64("id", id))
	go a.ExecuteJobSync(id)
}

// TriggerSync manually triggers a sync for all enabled jobs.
func (a *App) TriggerSync() {
	if a.IsSyncing() {
		a.logger.Warn("Sync already in progress")
		return
	}

	a.logger.Info("Manual sync triggered for all jobs")

	// Sync all enabled jobs
	jobs := a.GetSyncJobs()
	for _, job := range jobs {
		if job.Enabled {
			go a.ExecuteJobSync(job.ID)
			break // Only sync one job at a time
		}
	}
}

// ExecuteJobSync executes sync for a specific job (called by scheduler/watcher).
func (a *App) ExecuteJobSync(jobID int64) {
	// Find job
	a.mu.RLock()
	var job *SyncJob
	for _, j := range a.syncJobs {
		if j.ID == jobID {
			job = j
			break
		}
	}
	a.mu.RUnlock()

	if job == nil {
		a.logger.Warn("Job not found for sync", zap.Int64("job_id", jobID))
		return
	}

	if !job.Enabled {
		a.logger.Debug("Job disabled, skipping sync", zap.String("name", job.Name))
		return
	}

	// Use sync manager if available
	if a.syncManager != nil {
		if err := a.syncManager.ExecuteSync(job); err != nil {
			// "Sync already in progress" is expected when file watcher detects
			// changes made by an ongoing sync - log as debug, not error
			if err.Error() == "sync already in progress for job "+job.Name {
				a.logger.Debug("Sync skipped (already running)",
					zap.String("name", job.Name),
				)
			} else {
				a.logger.Error("Sync failed",
					zap.String("name", job.Name),
					zap.Error(err),
				)
			}
		}
	} else {
		a.logger.Warn("Sync manager not available")
		a.SetStatus("Sync unavailable")
	}
}

// StopSync stops all running syncs.
func (a *App) StopSync() {
	if a.syncManager == nil {
		return
	}

	count := a.syncManager.CancelAllSyncs()
	if count > 0 {
		a.SetStatus("Sync stopped")
		if a.notifier != nil {
			a.notifier.Send("Sync Stopped", "Synchronization was cancelled", NotifyInfo)
		}
	}
}

// StopJobSync stops a specific running sync.
func (a *App) StopJobSync(id int64) {
	if a.syncManager == nil {
		return
	}

	if a.syncManager.CancelSync(id) {
		a.SetStatus("Sync stopped")
	}
}

// IsJobSyncing returns whether a specific job is currently syncing.
func (a *App) IsJobSyncing(id int64) bool {
	if a.syncManager == nil {
		return false
	}
	return a.syncManager.IsSyncing(id)
}

// DisableFilesOnDemand disables Files On Demand for a job by unregistering the sync root.
// This removes all placeholders and allows normal access to the folder.
func (a *App) DisableFilesOnDemand(jobID int64) error {
	a.mu.RLock()
	var job *SyncJob
	for _, j := range a.syncJobs {
		if j.ID == jobID {
			job = j
			break
		}
	}
	a.mu.RUnlock()

	if job == nil {
		return errJobNotFound
	}

	a.logger.Info("Disabling Files On Demand for job",
		zap.String("name", job.Name),
		zap.String("local_path", job.LocalPath),
	)

	// First try to unregister via provider if it exists
	if a.syncManager != nil {
		if err := a.syncManager.UnregisterProvider(jobID); err != nil {
			a.logger.Warn("Failed to unregister via provider, trying direct path",
				zap.Error(err),
			)
		}

		// Also try to unregister by path (in case provider wasn't in memory)
		if err := a.syncManager.UnregisterSyncRootByPath(job.LocalPath); err != nil {
			// This might fail if already unregistered, which is fine
			a.logger.Debug("UnregisterSyncRootByPath result", zap.Error(err))
		}
	}

	// Update job settings
	job.FilesOnDemand = false

	// Persist to database
	if a.db != nil {
		dbJob := convertAppJobToDBJob(job)
		if err := a.db.UpdateSyncJob(dbJob); err != nil {
			return err
		}
	}

	a.logger.Info("Files On Demand disabled for job", zap.String("name", job.Name))
	return nil
}
