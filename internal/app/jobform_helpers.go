// Package app provides helper methods for the job form.
package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// Conversion helpers

func (jf *JobForm) modeToIndex(mode syncpkg.SyncMode) int {
	switch mode {
	case syncpkg.SyncModeUpload:
		return 1
	case syncpkg.SyncModeDownload:
		return 2
	default:
		return 0 // Mirror
	}
}

func (jf *JobForm) indexToMode(index int) syncpkg.SyncMode {
	switch index {
	case 1:
		return syncpkg.SyncModeUpload
	case 2:
		return syncpkg.SyncModeDownload
	default:
		return syncpkg.SyncModeMirror
	}
}

func (jf *JobForm) conflictToIndex(conflict string) int {
	switch conflict {
	case "local":
		return 1
	case "remote":
		return 2
	case "ask":
		return 3
	default:
		return 0 // recent
	}
}

func (jf *JobForm) indexToConflict(index int) string {
	switch index {
	case 1:
		return "local"
	case 2:
		return "remote"
	case 3:
		return "ask"
	default:
		return "recent"
	}
}

func (jf *JobForm) triggerModeToIndex(mode SyncTriggerMode) int {
	switch mode {
	case SyncTriggerManual:
		return 0
	case SyncTrigger5Min:
		return 1
	case SyncTrigger15Min:
		return 2
	case SyncTrigger30Min:
		return 3
	case SyncTrigger1Hour:
		return 4
	case SyncTriggerRealtime:
		return 5
	default:
		return 0 // Manual
	}
}

func (jf *JobForm) indexToTriggerMode(index int) SyncTriggerMode {
	switch index {
	case 0:
		return SyncTriggerManual
	case 1:
		return SyncTrigger5Min
	case 2:
		return SyncTrigger15Min
	case 3:
		return SyncTrigger30Min
	case 4:
		return SyncTrigger1Hour
	case 5:
		return SyncTriggerRealtime
	default:
		return SyncTriggerManual
	}
}

// updateTriggerModeHelp updates the help text based on selected trigger mode.
func (jf *JobForm) updateTriggerModeHelp() {
	switch jf.triggerModeSelect.SelectedIndex() {
	case 0: // Manual
		jf.triggerModeHelpLabel.SetText("Sync only when you click 'Sync Now'. Good for initial setup.")
	case 1, 2, 3, 4: // Scheduled
		jf.triggerModeHelpLabel.SetText("Sync automatically at regular intervals. Checks both local and remote for changes.")
	case 5: // Realtime
		jf.triggerModeHelpLabel.SetText("Sync instantly when local files change. Also checks remote every 5 minutes.")
	default:
		jf.triggerModeHelpLabel.SetText("")
	}
}

func (jf *JobForm) autoDehydrateDaysToIndex(days int) int {
	switch days {
	case 0:
		return 0 // Never
	case 7:
		return 1
	case 14:
		return 2
	case 30:
		return 3
	case 60:
		return 4
	case 90:
		return 5
	default:
		return 0 // Never
	}
}

func (jf *JobForm) indexToAutoDehydrateDays(index int) int {
	switch index {
	case 0:
		return 0 // Never
	case 1:
		return 7
	case 2:
		return 14
	case 3:
		return 30
	case 4:
		return 60
	case 5:
		return 90
	default:
		return 0 // Never
	}
}

// updateModeHelp updates the help text based on selected sync mode.
func (jf *JobForm) updateModeHelp() {
	switch jf.modeSelect.SelectedIndex() {
	case 0: // Mirror
		jf.modeHelpLabel.SetText("Changes sync both ways: local changes upload to server, server changes download locally.")
	case 1: // Upload
		jf.modeHelpLabel.SetText("Local changes upload to server. Server changes are ignored.")
	case 2: // Download
		jf.modeHelpLabel.SetText("Server changes download locally. Local changes are ignored.")
	default:
		jf.modeHelpLabel.SetText("")
	}
}

// Browse and refresh methods

// browseLocalFolder opens a folder browser dialog.
func (jf *JobForm) browseLocalFolder(parent fyne.Window) {
	dialog.ShowFolderOpen(func(uri fyne.ListableURI, err error) {
		if err != nil || uri == nil {
			return
		}
		jf.localPathEntry.SetText(uri.Path())
	}, parent)
}

// browseRemotePath opens a dialog to browse folders in the SMB share.
func (jf *JobForm) browseRemotePath(parent fyne.Window) {
	// Check if SMB connection is selected
	selectedIdx := jf.smbConnectionSelect.SelectedIndex()
	if selectedIdx < 0 || selectedIdx >= len(jf.smbConnections) {
		dialog.ShowError(errFieldRequired("SMB Server"), parent)
		return
	}

	// Check if share is selected
	if jf.remoteShareSelect.Selected == "" {
		dialog.ShowError(errFieldRequired("Share (click Refresh first)"), parent)
		return
	}

	smbConn := jf.smbConnections[selectedIdx]
	share := jf.remoteShareSelect.Selected

	// Show the remote folder browser
	browser := NewRemoteFolderBrowser(jf.app, smbConn, share, jf.remotePathEntry.Text, func(selectedPath string) {
		jf.remotePathEntry.SetText(selectedPath)
	})
	browser.Show(parent)
}

// onSMBConnectionChanged is called when the SMB connection selection changes.
func (jf *JobForm) onSMBConnectionChanged() {
	// Guard against being called before remoteShareSelect is initialized
	if jf.remoteShareSelect == nil {
		return
	}

	// Clear the share selection when connection changes
	jf.availableShares = nil
	jf.remoteShareSelect.Options = []string{}
	jf.remoteShareSelect.ClearSelected()
	jf.remoteShareSelect.Refresh()
}

// refreshShares fetches available shares from the selected SMB server.
func (jf *JobForm) refreshShares(parent fyne.Window) {
	selectedIdx := jf.smbConnectionSelect.SelectedIndex()
	if selectedIdx < 0 || selectedIdx >= len(jf.smbConnections) {
		dialog.ShowError(errFieldRequired("SMB Server"), parent)
		return
	}
	smbConn := jf.smbConnections[selectedIdx]

	// Show progress
	progress := dialog.NewProgressInfinite("Loading Shares", "Connecting to SMB server...", parent)
	progress.Show()

	go func() {
		shares, err := jf.app.ListSMBSharesFromConnection(smbConn.ID)

		fyne.Do(func() {
			progress.Hide()

			if err != nil {
				dialog.ShowError(err, parent)
				return
			}

			// Filter out administrative shares ($ suffix)
			filteredShares := make([]string, 0)
			for _, share := range shares {
				if len(share) > 0 && share[len(share)-1] != '$' {
					filteredShares = append(filteredShares, share)
				}
			}

			jf.availableShares = filteredShares
			jf.remoteShareSelect.Options = filteredShares
			jf.remoteShareSelect.Refresh()

			if len(filteredShares) == 0 {
				dialog.ShowInformation("No Shares", "No accessible shares found on this server.", parent)
			} else if len(filteredShares) == 1 {
				jf.remoteShareSelect.SetSelected(filteredShares[0])
			}
		})
	}()
}

// First sync wizard methods

// showFirstSyncWizard shows the first sync wizard for new jobs.
func (jf *JobForm) showFirstSyncWizard(parent fyne.Window) {
	wizard := NewFirstSyncDialog(jf.app, jf.job, parent)

	wizard.Show(func(mode FirstSyncMode, trust TrustSource) {
		// Update job with trust source and conflict resolution
		jf.job.TrustSource = string(trust)
		jf.job.FirstSyncDone = true

		// Map TrustSource to ConflictResolution
		switch trust {
		case TrustSourceServer:
			jf.job.ConflictResolution = "remote"
		case TrustSourceLocal:
			jf.job.ConflictResolution = "local"
		case TrustSourceRecent:
			jf.job.ConflictResolution = "recent"
		case TrustSourceKeepBoth:
			jf.job.ConflictResolution = "keep_both"
		default:
			jf.job.ConflictResolution = "recent" // Safe default
		}

		// Save updated job
		if err := jf.app.UpdateSyncJob(jf.job); err != nil {
			jf.app.Logger().Error("Failed to update job after first sync wizard", zap.Error(err))
		}

		// Execute first sync based on chosen mode
		go jf.executeFirstSync(mode)

		if jf.onSave != nil {
			jf.onSave(jf.job)
		}
	}, func() {
		// Cancelled - still call onSave since job was already created
		jf.job.FirstSyncDone = true // Mark as done to avoid showing wizard again
		jf.app.UpdateSyncJob(jf.job)

		if jf.onSave != nil {
			jf.onSave(jf.job)
		}
	})
}

// executeFirstSync executes the first sync based on the chosen mode.
func (jf *JobForm) executeFirstSync(mode FirstSyncMode) {
	jf.app.Logger().Info("Executing first sync",
		zap.String("job", jf.job.Name),
		zap.String("mode", string(mode)))

	// TODO: Implement different modes
	// For now, just trigger a normal sync (which is essentially "merge")
	switch mode {
	case FirstSyncModeMerge:
		// Normal bidirectional sync - keep all files from both sides
		jf.app.ExecuteJobSync(jf.job.ID)

	case FirstSyncModeServerWins:
		// Server is reference: download everything from server, delete local extras
		// For now, just do a normal sync
		// TODO: Implement proper server-wins logic
		jf.app.ExecuteJobSync(jf.job.ID)

	case FirstSyncModeLocalWins:
		// Local is reference: upload everything to server, delete server extras
		// For now, just do a normal sync
		// TODO: Implement proper local-wins logic
		jf.app.ExecuteJobSync(jf.job.ID)
	}
}
