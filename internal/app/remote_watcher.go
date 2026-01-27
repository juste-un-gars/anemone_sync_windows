// Package app provides remote SMB watching functionality for bidirectional sync.
package app

import (
	"context"
	"sync"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// RemoteSnapshot represents a point-in-time state of a remote share.
type RemoteSnapshot struct {
	FileCount  int
	TotalBytes int64
	Timestamp  time.Time
}

// jobRemoteWatcher holds the watcher state for a single job.
type jobRemoteWatcher struct {
	jobID        int64
	lastSnapshot *RemoteSnapshot
	ticker       *time.Ticker
	cancel       context.CancelFunc
}

// RemoteWatcher monitors remote SMB shares for changes.
type RemoteWatcher struct {
	app    *App
	logger *zap.Logger

	mu           sync.RWMutex
	watchers     map[int64]*jobRemoteWatcher // Job ID -> watcher
	running      bool
	ctx          context.Context
	cancel       context.CancelFunc
	pollInterval time.Duration
}

// Default polling interval for remote changes.
const defaultRemotePollInterval = 30 * time.Second

// NewRemoteWatcher creates a new remote watcher instance.
func NewRemoteWatcher(app *App, logger *zap.Logger) *RemoteWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &RemoteWatcher{
		app:          app,
		logger:       logger,
		watchers:     make(map[int64]*jobRemoteWatcher),
		ctx:          ctx,
		cancel:       cancel,
		pollInterval: defaultRemotePollInterval,
	}
}

// SetPollInterval sets the polling interval for all jobs.
func (rw *RemoteWatcher) SetPollInterval(interval time.Duration) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if interval < 10*time.Second {
		interval = 10 * time.Second // Minimum 10s to avoid hammering server
	}
	rw.pollInterval = interval
	rw.logger.Info("Remote poll interval updated", zap.Duration("interval", interval))
}

// Start begins watching all enabled jobs.
func (rw *RemoteWatcher) Start() {
	rw.mu.Lock()
	if rw.running {
		rw.mu.Unlock()
		return
	}
	rw.running = true
	rw.mu.Unlock()

	rw.logger.Info("Remote watcher starting")

	// Note: RemoteWatcher is deprecated - remote checking is now done by scheduler
	// This code is kept for potential future use but is not called
	jobs := rw.app.GetSyncJobs()
	for _, job := range jobs {
		if job.Enabled {
			rw.WatchJob(job)
		}
	}

	rw.logger.Info("Remote watcher started", zap.Int("watched_jobs", len(rw.watchers)))
}

// Stop stops all remote watchers.
func (rw *RemoteWatcher) Stop() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if !rw.running {
		return
	}

	rw.logger.Info("Remote watcher stopping")

	// Cancel context
	rw.cancel()

	// Stop all watchers
	for id, jw := range rw.watchers {
		rw.closeJobWatcher(jw)
		delete(rw.watchers, id)
	}

	rw.running = false
	rw.logger.Info("Remote watcher stopped")
}

// WatchJob starts watching a specific job's remote path.
func (rw *RemoteWatcher) WatchJob(job *SyncJob) error {
	if job.RemoteHost == "" || job.RemoteShare == "" {
		rw.logger.Debug("Job has no remote path configured", zap.String("name", job.Name))
		return nil
	}

	rw.mu.Lock()
	defer rw.mu.Unlock()

	// Close existing watcher if any
	if existing, ok := rw.watchers[job.ID]; ok {
		rw.closeJobWatcher(existing)
		delete(rw.watchers, job.ID)
	}

	// Create job watcher context
	ctx, cancel := context.WithCancel(rw.ctx)

	// Use default poll interval
	pollInterval := rw.pollInterval

	// Create ticker for polling
	ticker := time.NewTicker(pollInterval)

	jw := &jobRemoteWatcher{
		jobID:  job.ID,
		ticker: ticker,
		cancel: cancel,
	}

	rw.watchers[job.ID] = jw

	// Start polling loop
	go rw.pollLoop(ctx, jw, job)

	rw.logger.Info("Watching remote for job",
		zap.String("name", job.Name),
		zap.String("server", job.RemoteHost),
		zap.String("share", job.RemoteShare),
		zap.Duration("interval", pollInterval),
	)

	return nil
}

// UnwatchJob stops watching a specific job.
func (rw *RemoteWatcher) UnwatchJob(jobID int64) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	if jw, ok := rw.watchers[jobID]; ok {
		rw.closeJobWatcher(jw)
		delete(rw.watchers, jobID)
		rw.logger.Info("Unwatched remote for job", zap.Int64("job_id", jobID))
	}
}

// RewatchJob re-initializes watching for a job.
func (rw *RemoteWatcher) RewatchJob(job *SyncJob) error {
	rw.UnwatchJob(job.ID)
	if job.Enabled {
		return rw.WatchJob(job)
	}
	return nil
}

// closeJobWatcher closes a job watcher's resources.
func (rw *RemoteWatcher) closeJobWatcher(jw *jobRemoteWatcher) {
	jw.cancel()
	jw.ticker.Stop()
}

// pollLoop polls the remote server periodically.
func (rw *RemoteWatcher) pollLoop(ctx context.Context, jw *jobRemoteWatcher, job *SyncJob) {
	// Do initial check after a short delay (let app initialize)
	select {
	case <-ctx.Done():
		return
	case <-time.After(5 * time.Second):
	}

	// Initial snapshot
	snapshot, err := rw.takeSnapshot(ctx, job)
	if err != nil {
		rw.logger.Warn("Failed to take initial remote snapshot",
			zap.String("job", job.Name),
			zap.Error(err),
		)
	} else {
		jw.lastSnapshot = snapshot
		rw.logger.Debug("Initial remote snapshot taken",
			zap.String("job", job.Name),
			zap.Int("files", snapshot.FileCount),
			zap.Int64("bytes", snapshot.TotalBytes),
		)
	}

	// Poll loop
	for {
		select {
		case <-ctx.Done():
			return
		case <-jw.ticker.C:
			rw.checkForChanges(ctx, jw, job)
		}
	}
}

// checkForChanges checks if the remote has changed since last snapshot.
func (rw *RemoteWatcher) checkForChanges(ctx context.Context, jw *jobRemoteWatcher, job *SyncJob) {
	// Get fresh job data (might have been updated)
	jobs := rw.app.GetSyncJobs()
	var currentJob *SyncJob
	for _, j := range jobs {
		if j.ID == job.ID {
			currentJob = j
			break
		}
	}

	if currentJob == nil || !currentJob.Enabled {
		rw.logger.Debug("Job disabled, skipping",
			zap.Int64("job_id", job.ID),
		)
		return
	}

	// Skip if sync is already in progress
	if rw.app.syncManager != nil && rw.app.syncManager.IsSyncing(job.ID) {
		rw.logger.Debug("Sync already in progress, skipping remote check",
			zap.String("job", currentJob.Name),
		)
		return
	}

	// Take new snapshot
	snapshot, err := rw.takeSnapshot(ctx, currentJob)
	if err != nil {
		rw.logger.Warn("Failed to take remote snapshot",
			zap.String("job", currentJob.Name),
			zap.Error(err),
		)
		return
	}

	// Compare with last snapshot
	if jw.lastSnapshot == nil {
		jw.lastSnapshot = snapshot
		return
	}

	if rw.hasChanged(jw.lastSnapshot, snapshot) {
		rw.logger.Info("Remote changes detected",
			zap.String("job", currentJob.Name),
			zap.Int("old_files", jw.lastSnapshot.FileCount),
			zap.Int("new_files", snapshot.FileCount),
			zap.Int64("old_bytes", jw.lastSnapshot.TotalBytes),
			zap.Int64("new_bytes", snapshot.TotalBytes),
		)

		// Update snapshot before triggering sync
		jw.lastSnapshot = snapshot

		// Trigger sync
		rw.onRemoteChange(currentJob.ID)
	} else {
		rw.logger.Debug("No remote changes",
			zap.String("job", currentJob.Name),
			zap.Int("files", snapshot.FileCount),
		)
		jw.lastSnapshot = snapshot
	}
}

// takeSnapshot takes a snapshot of the remote state.
func (rw *RemoteWatcher) takeSnapshot(ctx context.Context, job *SyncJob) (*RemoteSnapshot, error) {
	// Check context
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Connect to SMB using credentials from keyring
	client, err := smb.NewSMBClientFromKeyring(job.RemoteHost, job.RemoteShare, rw.logger)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, err
	}
	defer client.Disconnect()

	// Scan remote (lightweight - just count files and bytes)
	snapshot := &RemoteSnapshot{
		Timestamp: time.Now(),
	}

	if err := rw.scanRemoteRecursive(ctx, client, job.RemotePath, snapshot); err != nil {
		return nil, err
	}

	return snapshot, nil
}

// scanRemoteRecursive recursively scans remote directories.
func (rw *RemoteWatcher) scanRemoteRecursive(ctx context.Context, client *smb.SMBClient, path string, snapshot *RemoteSnapshot) error {
	// Check context
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entries, err := client.ListRemote(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if entry.IsDir {
			if err := rw.scanRemoteRecursive(ctx, client, entry.Path, snapshot); err != nil {
				// Log but continue on directory errors
				rw.logger.Debug("Failed to scan subdirectory",
					zap.String("path", entry.Path),
					zap.Error(err),
				)
			}
		} else {
			snapshot.FileCount++
			snapshot.TotalBytes += entry.Size
		}
	}

	return nil
}

// hasChanged returns true if the snapshot has changed.
func (rw *RemoteWatcher) hasChanged(old, new *RemoteSnapshot) bool {
	if old == nil || new == nil {
		return true
	}

	// Check if file count or total bytes changed
	return old.FileCount != new.FileCount || old.TotalBytes != new.TotalBytes
}

// onRemoteChange is called when remote changes are detected.
func (rw *RemoteWatcher) onRemoteChange(jobID int64) {
	// Find the job to check if auto-sync is paused
	jobs := rw.app.GetSyncJobs()
	var job *SyncJob
	for _, j := range jobs {
		if j.ID == jobID {
			job = j
			break
		}
	}

	if job == nil || !job.Enabled {
		return
	}

	rw.logger.Info("Remote changes detected, triggering sync",
		zap.Int64("job_id", jobID),
	)

	// Delegate to app's sync execution
	rw.app.ExecuteJobSync(jobID)
}

// IsWatching returns true if the watcher is actively monitoring the job.
func (rw *RemoteWatcher) IsWatching(jobID int64) bool {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	_, ok := rw.watchers[jobID]
	return ok
}

// WatchedJobCount returns the number of jobs being watched.
func (rw *RemoteWatcher) WatchedJobCount() int {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return len(rw.watchers)
}

// GetLastSnapshot returns the last snapshot for a job.
func (rw *RemoteWatcher) GetLastSnapshot(jobID int64) *RemoteSnapshot {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if jw, ok := rw.watchers[jobID]; ok {
		return jw.lastSnapshot
	}
	return nil
}

// parsePollInterval parses a poll interval string to duration.
func (rw *RemoteWatcher) parsePollInterval(interval string) time.Duration {
	switch interval {
	case "30s":
		return 30 * time.Second
	case "1m":
		return 1 * time.Minute
	case "5m":
		return 5 * time.Minute
	default:
		// Try parsing as duration
		d, err := time.ParseDuration(interval)
		if err != nil {
			return 0
		}
		return d
	}
}
