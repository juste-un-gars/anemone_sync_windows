// Package app provides file watching functionality for real-time sync.
package app

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

// Watcher monitors file system changes and triggers syncs.
type Watcher struct {
	app    *App
	logger *zap.Logger

	mu        sync.RWMutex
	watchers  map[int64]*jobWatcher // Job ID -> watcher
	running   bool
	ctx       context.Context
	cancel    context.CancelFunc
}

// jobWatcher holds the watcher state for a single job.
type jobWatcher struct {
	jobID        int64
	localPath    string
	watcher      *fsnotify.Watcher
	debouncer    *debouncer
	cancel       context.CancelFunc
	syncActive   bool      // True while a sync is in progress
	syncCooldown time.Time // Ignore events until this time
}

// debouncer coalesces rapid file changes into single sync triggers.
type debouncer struct {
	mu       sync.Mutex
	timer    *time.Timer
	pending  bool
	delay    time.Duration
	callback func()
}

// Default debounce delay (wait for changes to settle).
const defaultDebounceDelay = 3 * time.Second

// NewWatcher creates a new file watcher instance.
func NewWatcher(app *App, logger *zap.Logger) *Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Watcher{
		app:      app,
		logger:   logger,
		watchers: make(map[int64]*jobWatcher),
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins watching all enabled jobs with realtime trigger mode.
func (w *Watcher) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.logger.Info("File watcher starting")

	// Watch all enabled jobs with Realtime trigger mode
	jobs := w.app.GetSyncJobs()
	for _, job := range jobs {
		if job.Enabled && job.TriggerMode == SyncTriggerRealtime {
			w.WatchJob(job)
		}
	}

	w.logger.Info("File watcher started", zap.Int("watched_jobs", len(w.watchers)))
}

// Stop stops all file watchers.
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	w.logger.Info("File watcher stopping")

	// Cancel context
	w.cancel()

	// Close all watchers
	for id, jw := range w.watchers {
		w.closeJobWatcher(jw)
		delete(w.watchers, id)
	}

	w.running = false
	w.logger.Info("File watcher stopped")
}

// WatchJob starts watching a specific job's local path.
func (w *Watcher) WatchJob(job *SyncJob) error {
	if job.LocalPath == "" {
		return nil
	}

	// Verify path exists
	info, err := os.Stat(job.LocalPath)
	if err != nil {
		w.logger.Warn("Cannot watch path",
			zap.String("path", job.LocalPath),
			zap.Error(err),
		)
		return err
	}
	if !info.IsDir() {
		w.logger.Warn("Watch path is not a directory",
			zap.String("path", job.LocalPath),
		)
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Close existing watcher if any
	if existing, ok := w.watchers[job.ID]; ok {
		w.closeJobWatcher(existing)
		delete(w.watchers, job.ID)
	}

	// Create fsnotify watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.logger.Error("Failed to create watcher", zap.Error(err))
		return err
	}

	// Create job watcher context
	ctx, cancel := context.WithCancel(w.ctx)

	// Create debouncer with default delay (3 seconds)
	deb := newDebouncer(defaultDebounceDelay, func() {
		w.onJobChange(job.ID)
	})

	jw := &jobWatcher{
		jobID:     job.ID,
		localPath: job.LocalPath,
		watcher:   fsWatcher,
		debouncer: deb,
		cancel:    cancel,
	}

	// Add directory and subdirectories
	if err := w.addRecursive(fsWatcher, job.LocalPath); err != nil {
		fsWatcher.Close()
		cancel()
		return err
	}

	w.watchers[job.ID] = jw

	// Start event loop
	go w.watchLoop(ctx, jw)

	w.logger.Info("Watching job",
		zap.String("name", job.Name),
		zap.String("path", job.LocalPath),
	)

	return nil
}

// UnwatchJob stops watching a specific job.
func (w *Watcher) UnwatchJob(jobID int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if jw, ok := w.watchers[jobID]; ok {
		w.closeJobWatcher(jw)
		delete(w.watchers, jobID)
		w.logger.Info("Unwatched job", zap.Int64("job_id", jobID))
	}
}

// RewatchJob re-initializes watching for a job (e.g., after path or trigger mode change).
func (w *Watcher) RewatchJob(job *SyncJob) error {
	w.UnwatchJob(job.ID)
	if job.Enabled && job.TriggerMode == SyncTriggerRealtime {
		return w.WatchJob(job)
	}
	return nil
}

// closeJobWatcher closes a job watcher's resources.
func (w *Watcher) closeJobWatcher(jw *jobWatcher) {
	jw.cancel()
	jw.debouncer.stop()
	if err := jw.watcher.Close(); err != nil {
		w.logger.Warn("Error closing watcher", zap.Error(err))
	}
}

// addRecursive adds a directory and all subdirectories to the watcher.
func (w *Watcher) addRecursive(fsWatcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip inaccessible paths
		}
		if info.IsDir() {
			// Skip hidden directories
			if len(info.Name()) > 1 && info.Name()[0] == '.' {
				return filepath.SkipDir
			}
			if err := fsWatcher.Add(path); err != nil {
				w.logger.Debug("Failed to watch directory",
					zap.String("path", path),
					zap.Error(err),
				)
			}
		}
		return nil
	})
}

// watchLoop processes events for a job watcher.
func (w *Watcher) watchLoop(ctx context.Context, jw *jobWatcher) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-jw.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(jw, event)

		case err, ok := <-jw.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn("Watcher error",
				zap.Int64("job_id", jw.jobID),
				zap.Error(err),
			)
		}
	}
}

// handleEvent processes a single file system event.
func (w *Watcher) handleEvent(jw *jobWatcher, event fsnotify.Event) {
	// Skip temporary and system files
	name := filepath.Base(event.Name)
	if w.shouldIgnore(name) {
		return
	}

	// Skip events during active sync or cooldown period
	if jw.syncActive {
		w.logger.Debug("File event ignored (sync active)",
			zap.Int64("job_id", jw.jobID),
			zap.String("path", event.Name),
		)
		return
	}
	if time.Now().Before(jw.syncCooldown) {
		w.logger.Debug("File event ignored (cooldown)",
			zap.Int64("job_id", jw.jobID),
			zap.String("path", event.Name),
		)
		return
	}

	w.logger.Debug("File event",
		zap.Int64("job_id", jw.jobID),
		zap.String("path", event.Name),
		zap.String("op", event.Op.String()),
	)

	// Handle new directories - add them to watch
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			w.addRecursive(jw.watcher, event.Name)
		}
	}

	// Trigger debounced sync
	jw.debouncer.trigger()
}

// shouldIgnore returns true if the file should be ignored.
func (w *Watcher) shouldIgnore(name string) bool {
	// Ignore hidden files
	if len(name) > 0 && name[0] == '.' {
		return true
	}

	// Ignore common temporary files
	ignoreSuffixes := []string{
		".tmp", ".temp", ".swp", ".swo", "~",
		".partial", ".crdownload", ".part",
	}
	for _, suffix := range ignoreSuffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return true
		}
	}

	// Ignore common system files
	ignoreNames := []string{
		"desktop.ini", "Thumbs.db", ".DS_Store",
		"$RECYCLE.BIN", "System Volume Information",
	}
	for _, ignore := range ignoreNames {
		if name == ignore {
			return true
		}
	}

	return false
}

// onJobChange is called when changes are detected for a job (after debounce).
func (w *Watcher) onJobChange(jobID int64) {
	// Find the job to verify it's still enabled
	jobs := w.app.GetSyncJobs()
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

	w.logger.Info("File changes detected, triggering sync",
		zap.Int64("job_id", jobID),
	)

	// Delegate to app's sync execution
	w.app.ExecuteJobSync(jobID)
}

// IsWatching returns true if the watcher is actively monitoring the job.
func (w *Watcher) IsWatching(jobID int64) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.watchers[jobID]
	return ok
}

// SetSyncActive marks a job's sync as active or inactive.
// When active, file events are ignored to prevent sync loops.
// When inactive, a cooldown period starts before events are processed again.
func (w *Watcher) SetSyncActive(jobID int64, active bool) {
	w.mu.Lock()
	defer w.mu.Unlock()

	jw, ok := w.watchers[jobID]
	if !ok {
		return
	}

	jw.syncActive = active
	if !active {
		// Set cooldown for 5 seconds after sync ends
		jw.syncCooldown = time.Now().Add(5 * time.Second)
		w.logger.Debug("Sync cooldown started",
			zap.Int64("job_id", jobID),
			zap.Time("cooldown_until", jw.syncCooldown),
		)
	}
}

// WatchedJobCount returns the number of jobs being watched.
func (w *Watcher) WatchedJobCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.watchers)
}

// --- Debouncer ---

// newDebouncer creates a new debouncer.
func newDebouncer(delay time.Duration, callback func()) *debouncer {
	return &debouncer{
		delay:    delay,
		callback: callback,
	}
}

// trigger schedules or resets the debounce timer.
func (d *debouncer) trigger() {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Reset timer if already pending
	if d.timer != nil {
		d.timer.Stop()
	}

	d.pending = true
	d.timer = time.AfterFunc(d.delay, func() {
		d.mu.Lock()
		d.pending = false
		d.timer = nil
		d.mu.Unlock()

		d.callback()
	})
}

// stop cancels any pending trigger.
func (d *debouncer) stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	}
	d.pending = false
}
