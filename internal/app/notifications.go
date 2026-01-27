package app

import (
	"fmt"

	"fyne.io/fyne/v2"
	"go.uber.org/zap"
)

// NotificationType defines the type of notification.
type NotificationType int

const (
	NotifyInfo NotificationType = iota
	NotifySuccess
	NotifyWarning
	NotifyError
)

// Notifier handles system notifications.
type Notifier struct {
	app     *App
	enabled bool
}

// NewNotifier creates a new Notifier.
func NewNotifier(app *App) *Notifier {
	return &Notifier{
		app:     app,
		enabled: true,
	}
}

// SetEnabled enables or disables notifications.
func (n *Notifier) SetEnabled(enabled bool) {
	n.enabled = enabled
}

// IsEnabled returns whether notifications are enabled.
func (n *Notifier) IsEnabled() bool {
	return n.enabled
}

// Send sends a notification.
func (n *Notifier) Send(title, message string, notifyType NotificationType) {
	if !n.enabled {
		return
	}

	// Log the notification
	n.app.Logger().Info("Notification",
		zap.String("title", title),
		zap.String("message", message),
	)

	// Send via Fyne
	fyneApp := n.app.FyneApp()
	notification := fyne.NewNotification(title, message)
	fyneApp.SendNotification(notification)
}

// SyncStarted sends a notification when sync starts.
func (n *Notifier) SyncStarted(jobName string) {
	n.Send(
		"Sync Started",
		fmt.Sprintf("Syncing '%s'...", jobName),
		NotifyInfo,
	)
}

// SyncCompleted sends a notification when sync completes successfully.
func (n *Notifier) SyncCompleted(jobName string, filesCount int) {
	n.Send(
		"Sync Completed",
		fmt.Sprintf("'%s' synced successfully (%d files)", jobName, filesCount),
		NotifySuccess,
	)
}

// SyncFailed sends a notification when sync fails.
func (n *Notifier) SyncFailed(jobName string, err error) {
	n.Send(
		"Sync Failed",
		fmt.Sprintf("'%s' sync failed: %v", jobName, err),
		NotifyError,
	)
}

// SyncPartial sends a notification when sync partially succeeds.
func (n *Notifier) SyncPartial(jobName string, successCount, errorCount int) {
	n.Send(
		"Sync Partial",
		fmt.Sprintf("'%s': %d files synced, %d errors", jobName, successCount, errorCount),
		NotifyWarning,
	)
}

// ConflictDetected sends a notification when conflicts are detected.
func (n *Notifier) ConflictDetected(jobName string, count int) {
	n.Send(
		"Conflicts Detected",
		fmt.Sprintf("'%s': %d conflicts require attention", jobName, count),
		NotifyWarning,
	)
}

// ConnectionLost sends a notification when connection is lost.
func (n *Notifier) ConnectionLost(serverName string) {
	n.Send(
		"Connection Lost",
		fmt.Sprintf("Lost connection to '%s'", serverName),
		NotifyError,
	)
}

// ConnectionRestored sends a notification when connection is restored.
func (n *Notifier) ConnectionRestored(serverName string) {
	n.Send(
		"Connection Restored",
		fmt.Sprintf("Reconnected to '%s'", serverName),
		NotifySuccess,
	)
}

// ShutdownPending sends a notification when shutdown is imminent.
func (n *Notifier) ShutdownPending(seconds int) {
	n.Send(
		"Shutdown Pending",
		fmt.Sprintf("Computer will shutdown in %d seconds...", seconds),
		NotifyWarning,
	)
}

// ShutdownCancelled sends a notification when shutdown is cancelled.
func (n *Notifier) ShutdownCancelled() {
	n.Send(
		"Shutdown Cancelled",
		"Sync & Shutdown operation was cancelled",
		NotifyInfo,
	)
}
