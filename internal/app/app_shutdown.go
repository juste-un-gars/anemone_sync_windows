package app

import (
	"fyne.io/fyne/v2"
	"go.uber.org/zap"
)

// --- Sync & Shutdown ---

// ShowShutdownDialog displays the sync & shutdown configuration dialog.
func (a *App) ShowShutdownDialog(jobIDs []int64) {
	a.logger.Debug("Opening shutdown dialog", zap.Int("preselected_jobs", len(jobIDs)))

	// Need a parent window for the dialog
	// If settings window is open, use that; otherwise create a temporary window
	var parent fyne.Window
	if a.settings != nil && a.settings.window != nil {
		parent = a.settings.window
	} else {
		// Create a temporary parent window (Fyne dialogs require a window)
		parent = a.FyneApp().NewWindow("AnemoneSync")
		parent.Resize(fyne.NewSize(1, 1))
		parent.Show()
		// The dialog will use this as parent
	}

	dialog := NewShutdownDialog(a, jobIDs)
	dialog.Show(parent)
}

// StartSyncAndShutdown starts the sync & shutdown process.
func (a *App) StartSyncAndShutdown(config *ShutdownConfig) {
	// Initialize shutdown manager if needed
	if a.shutdownMgr == nil {
		a.shutdownMgr = NewShutdownManager(a, a.logger.Named("shutdown"))
	}

	// Create progress dialog
	var parent fyne.Window
	if a.settings != nil && a.settings.window != nil {
		parent = a.settings.window
	} else {
		parent = a.FyneApp().NewWindow("AnemoneSync - Sync & Shutdown")
		parent.Resize(fyne.NewSize(400, 300))
		parent.Show()
	}

	a.shutdownProgressDialog = NewShutdownProgressDialog(a)
	a.shutdownProgressDialog.Show(parent)

	// Start shutdown process with progress callback
	err := a.shutdownMgr.Start(config, func(progress *ShutdownProgress) {
		if a.shutdownProgressDialog != nil {
			a.shutdownProgressDialog.Update(progress)
		}

		// Update tray state
		if a.tray != nil {
			a.tray.UpdateShutdownState(progress.State == ShutdownStateSyncing ||
				progress.State == ShutdownStateWaitingShutdown)
		}

		// Close progress dialog if cancelled or complete
		if progress.State == ShutdownStateCancelled {
			// Keep dialog open to show cancellation status
		}
	})

	if err != nil {
		a.logger.Error("Failed to start sync & shutdown", zap.Error(err))
		if a.notifier != nil {
			a.notifier.Send("Sync & Shutdown", "Failed to start: "+err.Error(), NotifyError)
		}
		if a.shutdownProgressDialog != nil {
			a.shutdownProgressDialog.Hide()
		}
	}
}

// CancelSyncAndShutdown cancels the ongoing sync & shutdown process.
func (a *App) CancelSyncAndShutdown() {
	if a.shutdownMgr != nil {
		a.shutdownMgr.Cancel()
	}

	// Update tray
	if a.tray != nil {
		a.tray.UpdateShutdownState(false)
	}
}

// IsShutdownActive returns whether a shutdown operation is in progress.
func (a *App) IsShutdownActive() bool {
	if a.shutdownMgr == nil {
		return false
	}
	return a.shutdownMgr.IsActive()
}

// --- Errors ---

var (
	errJobNotFound           = &appError{msg: "sync job not found"}
	errSMBConnectionNotFound = &appError{msg: "SMB connection not found"}
)

type appError struct {
	msg string
}

func (e *appError) Error() string {
	return e.msg
}
