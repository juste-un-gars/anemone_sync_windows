package app

import (
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// --- UI ---

// ShowSettings opens the settings window.
func (a *App) ShowSettings() {
	a.logger.Debug("Opening settings window")
	// Create or show the settings window
	// Note: We don't use fyne.Do() here because systray is the main loop,
	// not Fyne. Fyne handles thread-safety internally for window operations.
	if a.settings == nil {
		a.settings = NewSettingsWindow(a)
	}
	a.settings.Show()
}

// --- App Settings ---

// GetAutoStart returns whether auto-start is enabled.
func (a *App) GetAutoStart() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.appSettings.AutoStart
}

// SetAutoStart enables/disables auto-start with Windows.
func (a *App) SetAutoStart(enabled bool) {
	a.mu.Lock()
	a.appSettings.AutoStart = enabled
	a.mu.Unlock()

	// Update Windows registry
	if a.autoStart != nil {
		if err := a.autoStart.SetEnabled(enabled); err != nil {
			a.logger.Warn("Failed to update auto-start", zap.Error(err))
		}
	}

	// Persist to database
	if a.db != nil {
		value := "false"
		if enabled {
			value = "true"
		}
		a.db.SetAppConfig("auto_start", value, "bool")
	}

	a.logger.Info("AutoStart setting changed", zap.Bool("enabled", enabled))
}

// GetNotificationsEnabled returns whether notifications are enabled.
func (a *App) GetNotificationsEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.appSettings.NotificationsEnabled
}

// GetLogLevel returns the current log level.
func (a *App) GetLogLevel() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.appSettings.LogLevel
}

// GetSyncInterval returns the current sync interval.
func (a *App) GetSyncInterval() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.appSettings.SyncInterval
}

// SetNotificationsEnabled enables/disables notifications.
func (a *App) SetNotificationsEnabled(enabled bool) {
	a.mu.Lock()
	a.appSettings.NotificationsEnabled = enabled
	a.mu.Unlock()

	// Update notifier
	if a.notifier != nil {
		a.notifier.SetEnabled(enabled)
	}

	// Persist to database
	if a.db != nil {
		value := "true"
		if !enabled {
			value = "false"
		}
		a.db.SetAppConfig("notifications_enabled", value, "bool")
	}

	a.logger.Info("Notifications setting changed", zap.Bool("enabled", enabled))
}

// SetLogLevel changes the logging level.
func (a *App) SetLogLevel(level string) {
	a.mu.Lock()
	a.appSettings.LogLevel = level
	a.mu.Unlock()

	// Persist to database
	if a.db != nil {
		a.db.SetAppConfig("log_level", level, "string")
	}

	a.logger.Info("Log level changed", zap.String("level", level))
}

// SetSyncInterval changes the auto-sync interval.
func (a *App) SetSyncInterval(interval string) {
	a.mu.Lock()
	a.appSettings.SyncInterval = interval
	a.mu.Unlock()

	// Persist to database
	if a.db != nil {
		a.db.SetAppConfig("sync_interval", interval, "string")
	}

	a.logger.Info("Sync interval changed", zap.String("interval", interval))
}

// SaveCredential saves credentials to the keyring.
func (a *App) SaveCredential(host, username, password, domain string, port int) error {
	a.logger.Debug("Saving credential", zap.String("host", host), zap.String("user", username))
	if a.credMgr != nil {
		creds := &smb.Credentials{
			Server:   host,
			Username: username,
			Password: password,
			Domain:   domain,
			Port:     port,
		}
		return a.credMgr.Save(creds)
	}
	return nil
}
