// Package app provides the shutdown progress dialog.
package app

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShutdownProgressDialog displays progress during sync & shutdown.
type ShutdownProgressDialog struct {
	app    *App
	dialog dialog.Dialog
	window fyne.Window

	// UI elements
	statusLabel      *widget.Label
	progressBar      *widget.ProgressBar
	elapsedLabel     *widget.Label
	remainingLabel   *widget.Label
	completedLabel   *widget.Label
	failedLabel      *widget.Label
	shutdownLabel    *widget.Label
	cancelBtn        *widget.Button
}

// NewShutdownProgressDialog creates a new progress dialog.
func NewShutdownProgressDialog(app *App) *ShutdownProgressDialog {
	return &ShutdownProgressDialog{
		app: app,
	}
}

// Show displays the progress dialog.
func (d *ShutdownProgressDialog) Show(parent fyne.Window) {
	d.window = parent

	// Status label
	d.statusLabel = widget.NewLabel("Preparing...")
	d.statusLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Progress bar
	d.progressBar = widget.NewProgressBar()

	// Time labels
	d.elapsedLabel = widget.NewLabel("Elapsed: 0s")
	d.remainingLabel = widget.NewLabel("")

	// Completed/Failed labels
	d.completedLabel = widget.NewLabel("Completed: 0")
	d.failedLabel = widget.NewLabel("Failed: 0")

	// Shutdown countdown label
	d.shutdownLabel = widget.NewLabel("")
	d.shutdownLabel.Importance = widget.DangerImportance
	d.shutdownLabel.Alignment = fyne.TextAlignCenter

	// Cancel button
	d.cancelBtn = widget.NewButton("Cancel", func() {
		d.app.CancelSyncAndShutdown()
	})
	d.cancelBtn.Importance = widget.DangerImportance

	// Layout
	timeRow := container.NewHBox(d.elapsedLabel, d.remainingLabel)
	statsRow := container.NewHBox(d.completedLabel, d.failedLabel)

	content := container.NewVBox(
		d.statusLabel,
		d.progressBar,
		timeRow,
		statsRow,
		widget.NewSeparator(),
		d.shutdownLabel,
		container.NewCenter(d.cancelBtn),
	)

	d.dialog = dialog.NewCustomWithoutButtons("Sync & Shutdown", content, parent)
	d.dialog.Resize(fyne.NewSize(400, 250))
	d.dialog.Show()
}

// Update updates the dialog with new progress information.
func (d *ShutdownProgressDialog) Update(progress *ShutdownProgress) {
	if d.dialog == nil {
		return
	}

	fyne.Do(func() {
		// Update status based on state
		switch progress.State {
		case ShutdownStateSyncing:
			if progress.TotalJobs > 0 {
				d.statusLabel.SetText(fmt.Sprintf("Syncing: %s (%d/%d)",
					progress.CurrentJobName,
					progress.CurrentJob,
					progress.TotalJobs))
				d.progressBar.SetValue(float64(progress.CurrentJob-1) / float64(progress.TotalJobs))
			} else {
				d.statusLabel.SetText("Preparing sync...")
				d.progressBar.SetValue(0)
			}
		case ShutdownStateWaitingShutdown:
			d.statusLabel.SetText("Sync complete!")
			d.progressBar.SetValue(1)
			if progress.ShutdownPending {
				d.shutdownLabel.SetText(fmt.Sprintf("Shutting down in %d seconds...", progress.ShutdownSeconds))
			}
		case ShutdownStateCancelled:
			d.statusLabel.SetText("Cancelled")
			d.shutdownLabel.SetText("Shutdown cancelled")
			d.cancelBtn.SetText("Close")
			d.cancelBtn.OnTapped = func() {
				d.Hide()
			}
		}

		// Update elapsed time
		d.elapsedLabel.SetText(fmt.Sprintf("Elapsed: %s", formatDuration(progress.ElapsedTime)))

		// Update remaining time (only if timeout is set)
		if progress.RemainingTime > 0 {
			d.remainingLabel.SetText(fmt.Sprintf(" | Remaining: %s", formatDuration(progress.RemainingTime)))
		} else {
			d.remainingLabel.SetText("")
		}

		// Update completed/failed counts
		d.completedLabel.SetText(fmt.Sprintf("Completed: %d", len(progress.CompletedJobs)))
		d.failedLabel.SetText(fmt.Sprintf("Failed: %d", len(progress.FailedJobs)))

		// Update failed label style
		if len(progress.FailedJobs) > 0 {
			d.failedLabel.Importance = widget.DangerImportance
		} else {
			d.failedLabel.Importance = widget.LowImportance
		}
	})
}

// Hide closes the dialog.
func (d *ShutdownProgressDialog) Hide() {
	if d.dialog != nil {
		d.dialog.Hide()
		d.dialog = nil
	}
}

// formatDuration formats a duration for display.
func formatDuration(dur time.Duration) string {
	if dur < time.Minute {
		return fmt.Sprintf("%ds", int(dur.Seconds()))
	}
	if dur < time.Hour {
		mins := int(dur.Minutes())
		secs := int(dur.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	hours := int(dur.Hours())
	mins := int(dur.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", hours, mins)
}
