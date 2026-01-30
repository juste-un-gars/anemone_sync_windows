// Package app provides the main application lifecycle management for AnemoneSync.
package app

import (
	"context"
	"os"
	"path/filepath"
	gosync "sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

const (
	AppID      = "com.anemone.sync"
	AppName    = "AnemoneSync"
	AppVersion = "0.1.0-dev"
)

// App represents the main application instance.
type App struct {
	fyneApp    fyne.App
	tray       *Tray
	settings   *SettingsWindow
	logger     *zap.Logger
	logLevel   zap.AtomicLevel // Dynamic log level control
	ctx        context.Context
	cancel     context.CancelFunc
	wg         gosync.WaitGroup
	mu         gosync.RWMutex
	running    bool
	syncing    bool
	lastStatus string

	// Startup mode
	isAutoStart bool // True if launched via Windows autostart

	// Persistence & Services
	db        *database.DB
	notifier  *Notifier
	autoStart *AutoStart
	credMgr   *smb.CredentialManager

	// Background workers
	scheduler     *Scheduler
	watcher       *Watcher
	remoteWatcher *RemoteWatcher
	syncManager   *SyncManager
	shutdownMgr   *ShutdownManager

	// Shutdown dialog/progress
	shutdownProgressDialog *ShutdownProgressDialog

	// Configuration
	appSettings    *AppSettings
	syncJobs       []*SyncJob
	smbConnections []*SMBConnection
}

// New creates a new App instance.
func New(logger *zap.Logger, logLevel zap.AtomicLevel) *App {
	ctx, cancel := context.WithCancel(context.Background())

	a := &App{
		logger:         logger,
		logLevel:       logLevel,
		ctx:            ctx,
		cancel:         cancel,
		lastStatus:     "Idle",
		appSettings:    DefaultAppSettings(),
		syncJobs:       make([]*SyncJob, 0),
		smbConnections: make([]*SMBConnection, 0),
		credMgr:        smb.NewCredentialManager(logger),
	}

	// Initialize notifier
	a.notifier = NewNotifier(a)

	// Initialize auto-start
	autoStart, err := NewAutoStart()
	if err != nil {
		logger.Warn("Failed to initialize auto-start", zap.Error(err))
	} else {
		a.autoStart = autoStart
	}

	// Initialize database
	if err := a.initDatabase(); err != nil {
		logger.Error("Failed to initialize database", zap.Error(err))
	}

	return a
}

// initDatabase initializes the SQLite database.
func (a *App) initDatabase() error {
	// Get database path in user's app data
	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		localAppData = "."
	}
	dbPath := filepath.Join(localAppData, "AnemoneSync", "data", "anemonesync.db")

	// Create DB with a default key (in production, this should come from keyring)
	cfg := database.Config{
		Path:             dbPath,
		EncryptionKey:    "AnemoneSync_DefaultKey_ChangeMe", // TODO: Get from secure storage
		CreateIfNotExist: true,
	}

	db, err := database.Open(cfg)
	if err != nil {
		return err
	}
	a.db = db

	// Load settings from database
	a.loadSettingsFromDB()

	// Load SMB connections from database
	a.loadSMBConnectionsFromDB()

	// Load sync jobs from database
	a.loadJobsFromDB()

	return nil
}

// loadSettingsFromDB loads app settings from the database.
func (a *App) loadSettingsFromDB() {
	if a.db == nil {
		return
	}

	config, err := a.db.GetAllAppConfig()
	if err != nil {
		a.logger.Warn("Failed to load app config", zap.Error(err))
		return
	}

	if v, ok := config["auto_start"]; ok && v == "true" {
		a.appSettings.AutoStart = true
	}
	if v, ok := config["notifications_enabled"]; ok && v == "false" {
		a.appSettings.NotificationsEnabled = false
	}
	if v, ok := config["log_level"]; ok && v != "" {
		a.appSettings.LogLevel = v
		// Apply saved log level at startup
		a.applyLogLevel(v)
		a.logger.Info("Loaded log level from settings", zap.String("level", v))
	}
	if v, ok := config["sync_interval"]; ok && v != "" {
		a.appSettings.SyncInterval = v
	}
}

// loadSMBConnectionsFromDB loads SMB connections from the database.
func (a *App) loadSMBConnectionsFromDB() {
	if a.db == nil {
		return
	}

	dbServers, err := a.db.GetAllSMBServers()
	if err != nil {
		a.logger.Warn("Failed to load SMB connections", zap.Error(err))
		return
	}

	// Convert database.SMBServer to app.SMBConnection
	a.smbConnections = make([]*SMBConnection, 0, len(dbServers))
	for _, dbServer := range dbServers {
		conn := convertDBServerToAppConnection(dbServer)
		a.smbConnections = append(a.smbConnections, conn)
	}

	a.logger.Info("Loaded SMB connections from database", zap.Int("count", len(a.smbConnections)))
}

// loadJobsFromDB loads sync jobs from the database.
func (a *App) loadJobsFromDB() {
	if a.db == nil {
		return
	}

	dbJobs, err := a.db.GetAllSyncJobs()
	if err != nil {
		a.logger.Warn("Failed to load sync jobs", zap.Error(err))
		return
	}

	// Convert database.SyncJob to app.SyncJob
	a.syncJobs = make([]*SyncJob, 0, len(dbJobs))
	for _, dbJob := range dbJobs {
		job := convertDBJobToAppJob(dbJob)

		// Find SMBConnectionID based on RemoteHost
		for _, conn := range a.smbConnections {
			if conn.Host == job.RemoteHost {
				job.SMBConnectionID = conn.ID
				job.Username = conn.Username
				break
			}
		}

		a.syncJobs = append(a.syncJobs, job)
	}

	a.logger.Info("Loaded sync jobs from database", zap.Int("count", len(a.syncJobs)))
}

// Run starts the application and blocks until it exits.
func (a *App) Run() {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	a.mu.Unlock()

	a.logger.Info("Starting AnemoneSync",
		zap.String("version", AppVersion),
	)

	// Initialize Fyne app (this sets up the main event loop)
	fyneApp := a.FyneApp()

	// Set up lifecycle handler for cleanup
	fyneApp.Lifecycle().SetOnStopped(func() {
		a.logger.Info("Application exiting")
		a.shutdown()
	})

	// Initialize and setup system tray
	a.tray = NewTray(a)
	a.tray.Setup()

	// Start background workers
	a.startWorkers()

	// Run Fyne main loop (blocks until quit)
	// Note: We don't create a window, app runs in system tray only
	fyneApp.Run()
}

// Quit triggers application shutdown.
func (a *App) Quit() {
	a.logger.Info("Quit requested")
	if a.fyneApp != nil {
		a.fyneApp.Quit()
	}
}

// shutdown performs cleanup when the app exits.
func (a *App) shutdown() {
	a.logger.Info("Shutting down...")

	// Stop file watcher
	if a.watcher != nil {
		a.watcher.Stop()
	}

	// Stop remote watcher
	if a.remoteWatcher != nil {
		a.remoteWatcher.Stop()
	}

	// Stop scheduler
	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	// Close sync manager
	if a.syncManager != nil {
		if err := a.syncManager.Close(); err != nil {
			a.logger.Warn("Failed to close sync manager", zap.Error(err))
		}
	}

	// Cancel context to stop remaining workers
	a.cancel()

	// Wait for workers to finish
	a.wg.Wait()

	// Close settings window if open
	if a.settings != nil {
		a.settings.Close()
	}

	// Close database
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.logger.Warn("Failed to close database", zap.Error(err))
		}
	}

	a.mu.Lock()
	a.running = false
	a.mu.Unlock()

	a.logger.Info("Shutdown complete")
}

// SetAutoStartMode sets whether the app was launched via Windows autostart.
func (a *App) SetAutoStartMode(isAutoStart bool) {
	a.isAutoStart = isAutoStart
}

// startWorkers initializes background workers.
func (a *App) startWorkers() {
	// Initialize sync manager (requires DB)
	if a.db != nil {
		syncMgr, err := NewSyncManager(a, a.db, a.logger.Named("syncmanager"))
		if err != nil {
			a.logger.Error("Failed to create sync manager", zap.Error(err))
		} else {
			a.syncManager = syncMgr
		}
	}

	// Initialize and start scheduler
	a.scheduler = NewScheduler(a, a.logger.Named("scheduler"))
	a.scheduler.Start()

	// Initialize and start file watcher
	a.watcher = NewWatcher(a, a.logger.Named("watcher"))
	a.watcher.Start()

	// Initialize and start remote watcher
	// Note: RemoteWatcher is no longer used - remote checking is done by scheduler
	a.remoteWatcher = nil

	a.logger.Info("Background workers started",
		zap.Int("scheduled_jobs", a.scheduler.ScheduledJobCount()),
		zap.Int("watched_local", a.watcher.WatchedJobCount()),
	)

	// Reconnect Cloud Files providers for jobs with FilesOnDemand enabled
	// This is needed because sync roots remain registered after app close
	a.reconnectCloudFilesProviders()

	// Start size calculator
	a.startSizeUpdater()

	// Trigger sync on startup if launched via autostart
	// Delay slightly to let systray fully initialize
	if a.isAutoStart {
		go func() {
			time.Sleep(2 * time.Second)
			a.triggerStartupSync()
		}()
	}
}

// reconnectCloudFilesProviders reconnects Cloud Files providers for jobs that have
// FilesOnDemand enabled. This is necessary because sync roots remain registered
// with Windows even after the app closes, but the callbacks need to be reconnected.
func (a *App) reconnectCloudFilesProviders() {
	if a.syncManager == nil {
		return
	}

	a.mu.RLock()
	jobs := make([]*SyncJob, len(a.syncJobs))
	copy(jobs, a.syncJobs)
	a.mu.RUnlock()

	reconnected := 0
	for _, job := range jobs {
		if job.Enabled && job.FilesOnDemand {
			a.logger.Info("Reconnecting Cloud Files provider",
				zap.String("job", job.Name),
				zap.String("local_path", job.LocalPath),
			)

			// This will create/reconnect the provider
			if err := a.syncManager.ReconnectProvider(job); err != nil {
				a.logger.Error("Failed to reconnect Cloud Files provider",
					zap.String("job", job.Name),
					zap.Error(err),
				)
			} else {
				reconnected++
			}
		}
	}

	if reconnected > 0 {
		a.logger.Info("Reconnected Cloud Files providers", zap.Int("count", reconnected))
	}
}

// triggerStartupSync syncs all jobs that have SyncOnStartup enabled.
func (a *App) triggerStartupSync() {
	a.mu.RLock()
	jobs := make([]*SyncJob, len(a.syncJobs))
	copy(jobs, a.syncJobs)
	a.mu.RUnlock()

	startupJobs := make([]*SyncJob, 0)
	for _, job := range jobs {
		if job.Enabled && job.SyncOnStartup {
			startupJobs = append(startupJobs, job)
		}
	}

	if len(startupJobs) == 0 {
		a.logger.Info("No jobs configured for sync on startup")
		return
	}

	a.logger.Info("Triggering startup sync",
		zap.Int("job_count", len(startupJobs)),
	)

	// Sync each job in background
	for _, job := range startupJobs {
		j := job // capture for goroutine
		go func() {
			a.logger.Info("Startup sync for job", zap.String("name", j.Name), zap.Int64("id", j.ID))
			a.ExecuteJobSync(j.ID)
		}()
	}
}

// --- State Management ---

// IsSyncing returns whether a sync is currently in progress.
func (a *App) IsSyncing() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.syncing
}

// SetSyncing updates the syncing state.
func (a *App) SetSyncing(syncing bool) {
	a.mu.Lock()
	a.syncing = syncing
	a.mu.Unlock()

	if a.tray != nil {
		a.tray.UpdateStatus()
	}
}

// GetStatus returns the current status string.
func (a *App) GetStatus() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.syncing {
		return "Syncing..."
	}
	return a.lastStatus
}

// SetStatus updates the status string.
func (a *App) SetStatus(status string) {
	a.mu.Lock()
	a.lastStatus = status
	a.mu.Unlock()

	if a.tray != nil {
		a.tray.UpdateStatus()
	}
}

// --- Accessors ---

// FyneApp returns the underlying Fyne application, creating it if needed.
func (a *App) FyneApp() fyne.App {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.fyneApp == nil {
		a.fyneApp = app.NewWithID(AppID)
	}
	return a.fyneApp
}

// Logger returns the application logger.
func (a *App) Logger() *zap.Logger {
	return a.logger
}

// Context returns the application context.
func (a *App) Context() context.Context {
	return a.ctx
}

// SetWatcherSyncActive notifies the watcher that a sync is starting or ending.
// This prevents the watcher from triggering new syncs during an active sync.
func (a *App) SetWatcherSyncActive(jobID int64, active bool) {
	if a.watcher != nil {
		a.watcher.SetSyncActive(jobID, active)
	}
}
