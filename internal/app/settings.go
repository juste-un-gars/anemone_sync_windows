package app

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SettingsWindow manages the settings window.
type SettingsWindow struct {
	app      *App
	window   fyne.Window
	jobsList *JobsList
	smbList  *SMBList

	// Dynamic buttons
	syncNowBtn *widget.Button
	stopBtn    *widget.Button
}

// NewSettingsWindow creates a new settings window.
func NewSettingsWindow(app *App) *SettingsWindow {
	sw := &SettingsWindow{
		app: app,
	}
	return sw
}

// Show displays the settings window.
func (sw *SettingsWindow) Show() {
	if sw.window != nil {
		sw.window.Show()
		sw.window.RequestFocus()
		return
	}

	sw.window = sw.app.FyneApp().NewWindow("AnemoneSync - Settings")
	sw.window.Resize(fyne.NewSize(700, 500))
	sw.window.SetFixedSize(false)

	// Create tabs
	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("SMB Servers", theme.ComputerIcon(), sw.createSMBTab()),
		container.NewTabItemWithIcon("Sync Jobs", theme.FolderIcon(), sw.createJobsTab()),
		container.NewTabItemWithIcon("General", theme.SettingsIcon(), sw.createGeneralTab()),
		container.NewTabItemWithIcon("About", theme.InfoIcon(), sw.createAboutTab()),
	)
	tabs.SetTabLocation(container.TabLocationLeading)

	sw.window.SetContent(tabs)

	// Handle window close
	sw.window.SetOnClosed(func() {
		sw.window = nil
	})

	sw.window.Show()
}

// createSMBTab creates the SMB servers management tab.
func (sw *SettingsWindow) createSMBTab() fyne.CanvasObject {
	// Create SMB connections list
	sw.smbList = NewSMBList(sw.app)

	// Toolbar buttons
	addBtn := widget.NewButtonWithIcon("Add Server", theme.ContentAddIcon(), func() {
		sw.showSMBForm(nil)
	})

	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		conn := sw.smbList.GetSelected()
		if conn != nil {
			sw.showSMBForm(conn)
		}
	})

	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		conn := sw.smbList.GetSelected()
		if conn != nil {
			sw.confirmDeleteSMBConnection(conn)
		}
	})

	toolbar := container.NewHBox(
		addBtn,
		editBtn,
		deleteBtn,
	)

	// Info label
	infoLabel := widget.NewLabel("Configure your SMB servers here. Credentials are stored securely in Windows Credential Manager.")
	infoLabel.Wrapping = fyne.TextWrapWord

	// Main content
	content := container.NewBorder(
		container.NewVBox(infoLabel, widget.NewSeparator(), toolbar), // top
		nil, // bottom
		nil, // left
		nil, // right
		sw.smbList.Container(),
	)

	return content
}

// showSMBForm displays the SMB connection creation/edit form.
func (sw *SettingsWindow) showSMBForm(conn *SMBConnection) {
	form := NewSMBForm(sw.app, conn, func(saved *SMBConnection) {
		sw.smbList.Refresh()
	})
	form.Show(sw.window)
}

// confirmDeleteSMBConnection shows a confirmation dialog before deleting an SMB connection.
func (sw *SettingsWindow) confirmDeleteSMBConnection(conn *SMBConnection) {
	dialog.ShowConfirm(
		"Delete SMB Connection",
		fmt.Sprintf("Are you sure you want to delete the SMB connection '%s'?\n\nNote: Sync jobs using this connection will stop working.", conn.DisplayName()),
		func(confirmed bool) {
			if confirmed {
				if err := sw.app.DeleteSMBConnection(conn.ID); err != nil {
					dialog.ShowError(err, sw.window)
				} else {
					sw.smbList.Refresh()
				}
			}
		},
		sw.window,
	)
}

// createJobsTab creates the sync jobs management tab.
func (sw *SettingsWindow) createJobsTab() fyne.CanvasObject {
	// Create jobs list
	sw.jobsList = NewJobsList(sw.app)

	// Toolbar buttons
	addBtn := widget.NewButtonWithIcon("Add Job", theme.ContentAddIcon(), func() {
		sw.showJobForm(nil)
	})

	editBtn := widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), func() {
		job := sw.jobsList.GetSelected()
		if job != nil {
			sw.showJobForm(job)
		}
	})

	deleteBtn := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
		job := sw.jobsList.GetSelected()
		if job != nil {
			sw.confirmDeleteJob(job)
		}
	})

	sw.syncNowBtn = widget.NewButtonWithIcon("Sync Now", theme.ViewRefreshIcon(), func() {
		job := sw.jobsList.GetSelected()
		if job != nil {
			sw.app.TriggerSyncJob(job.ID)
		}
	})

	sw.stopBtn = widget.NewButtonWithIcon("Stop", theme.MediaStopIcon(), func() {
		job := sw.jobsList.GetSelected()
		if job != nil {
			sw.app.StopJobSync(job.ID)
		} else {
			// Stop all syncs if no job selected
			sw.app.StopSync()
		}
	})
	sw.stopBtn.Importance = widget.DangerImportance

	// Fix Cloud Files button - unregisters sync root to fix "cloud operation timeout" errors
	fixCloudBtn := widget.NewButtonWithIcon("Fix Folder", theme.WarningIcon(), func() {
		job := sw.jobsList.GetSelected()
		if job != nil {
			sw.confirmDisableFilesOnDemand(job)
		}
	})

	// Update button states based on current sync status
	sw.updateSyncButtons()

	toolbar := container.NewHBox(
		addBtn,
		editBtn,
		deleteBtn,
		widget.NewSeparator(),
		sw.syncNowBtn,
		sw.stopBtn,
		widget.NewSeparator(),
		fixCloudBtn,
	)

	// Main content
	content := container.NewBorder(
		toolbar, // top
		nil,     // bottom
		nil,     // left
		nil,     // right
		sw.jobsList.Container(),
	)

	return content
}

// createGeneralTab creates the general settings tab.
func (sw *SettingsWindow) createGeneralTab() fyne.CanvasObject {
	// Auto-start with Windows
	autoStartCheck := widget.NewCheck("Start AnemoneSync with Windows", func(checked bool) {
		sw.app.SetAutoStart(checked)
	})
	autoStartCheck.SetChecked(sw.app.GetAutoStart())

	// Notifications
	notifyCheck := widget.NewCheck("Show sync notifications", func(checked bool) {
		sw.app.SetNotificationsEnabled(checked)
	})
	notifyCheck.SetChecked(sw.app.GetNotificationsEnabled())

	// Log level
	logLevelLabel := widget.NewLabel("Log Level:")
	currentLogLevel := sw.app.GetLogLevel()
	logLevelSelect := widget.NewSelect([]string{"Debug", "Info", "Warning", "Error"}, func(selected string) {
		if selected != sw.app.GetLogLevel() {
			sw.app.SetLogLevel(selected)
		}
	})
	logLevelSelect.SetSelected(currentLogLevel)

	// Sync interval
	intervalLabel := widget.NewLabel("Auto-sync interval:")
	currentInterval := sw.app.GetSyncInterval()
	intervalSelect := widget.NewSelect([]string{"Manual only", "5 minutes", "15 minutes", "30 minutes", "1 hour"}, func(selected string) {
		if selected != sw.app.GetSyncInterval() {
			sw.app.SetSyncInterval(selected)
		}
	})
	intervalSelect.SetSelected(currentInterval)

	form := container.NewVBox(
		widget.NewLabel("Startup"),
		autoStartCheck,
		widget.NewSeparator(),
		widget.NewLabel("Notifications"),
		notifyCheck,
		widget.NewSeparator(),
		widget.NewLabel("Logging"),
		container.NewHBox(logLevelLabel, logLevelSelect),
		widget.NewSeparator(),
		widget.NewLabel("Synchronization"),
		container.NewHBox(intervalLabel, intervalSelect),
	)

	return container.NewVScroll(form)
}

// createAboutTab creates the about tab.
func (sw *SettingsWindow) createAboutTab() fyne.CanvasObject {
	logo := widget.NewLabel("AnemoneSync")
	logo.TextStyle = fyne.TextStyle{Bold: true}
	logo.Alignment = fyne.TextAlignCenter

	version := widget.NewLabel("Version " + AppVersion)
	version.Alignment = fyne.TextAlignCenter

	description := widget.NewLabel("SMB File Synchronization Client\nBidirectional sync with conflict resolution")
	description.Alignment = fyne.TextAlignCenter

	copyright := widget.NewLabel("2026 - Open Source")
	copyright.Alignment = fyne.TextAlignCenter

	return container.NewCenter(
		container.NewVBox(
			logo,
			version,
			widget.NewSeparator(),
			description,
			widget.NewSeparator(),
			copyright,
		),
	)
}

// showJobForm displays the job creation/edit form.
func (sw *SettingsWindow) showJobForm(job *SyncJob) {
	form := NewJobForm(sw.app, job, func(saved *SyncJob) {
		sw.jobsList.Refresh()
	})
	form.Show(sw.window)
}

// confirmDeleteJob shows a confirmation dialog before deleting a job.
func (sw *SettingsWindow) confirmDeleteJob(job *SyncJob) {
	dialog.ShowConfirm(
		"Delete Sync Job",
		fmt.Sprintf("Are you sure you want to delete the sync job '%s'?", job.Name),
		func(confirmed bool) {
			if confirmed {
				if err := sw.app.DeleteSyncJob(job.ID); err != nil {
					dialog.ShowError(err, sw.window)
				} else {
					sw.jobsList.Refresh()
				}
			}
		},
		sw.window,
	)
}

// Close closes the settings window.
func (sw *SettingsWindow) Close() {
	if sw.window != nil {
		sw.window.Close()
		sw.window = nil
	}
}

// RefreshJobList refreshes the job list display.
func (sw *SettingsWindow) RefreshJobList() {
	if sw.jobsList != nil {
		sw.jobsList.Refresh()
	}
	sw.updateSyncButtons()
}

// updateSyncButtons updates the sync/stop button states based on sync status.
func (sw *SettingsWindow) updateSyncButtons() {
	if sw.syncNowBtn == nil || sw.stopBtn == nil {
		return
	}

	isSyncing := sw.app.IsSyncing()
	fyne.Do(func() {
		if isSyncing {
			sw.syncNowBtn.Disable()
			sw.stopBtn.Enable()
		} else {
			sw.syncNowBtn.Enable()
			sw.stopBtn.Disable()
		}
	})
}

// confirmDisableFilesOnDemand shows a confirmation dialog before disabling Files On Demand.
func (sw *SettingsWindow) confirmDisableFilesOnDemand(job *SyncJob) {
	dialog.ShowConfirm(
		"Fix Folder - Disable Files On Demand",
		fmt.Sprintf("This will unregister '%s' as a Cloud Files sync root.\n\n"+
			"Use this to fix 'cloud operation timeout' errors.\n\n"+
			"The folder will become a normal folder again.\n"+
			"You can re-enable Files On Demand later in the job settings.", job.Name),
		func(confirmed bool) {
			if confirmed {
				if err := sw.app.DisableFilesOnDemand(job.ID); err != nil {
					dialog.ShowError(err, sw.window)
				} else {
					dialog.ShowInformation("Success",
						"Files On Demand has been disabled for this job.\n"+
							"The folder should now be accessible normally.",
						sw.window)
					sw.jobsList.Refresh()
				}
			}
		},
		sw.window,
	)
}
