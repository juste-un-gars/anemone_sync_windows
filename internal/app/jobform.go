package app

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
)

// JobForm is a form for creating or editing a sync job.
type JobForm struct {
	app    *App
	job    *SyncJob
	isNew  bool
	onSave func(*SyncJob)
	dialog *dialog.CustomDialog

	// Form fields
	nameEntry           *widget.Entry
	localPathEntry      *widget.Entry
	smbConnectionSelect *widget.Select
	remoteShareSelect   *widget.Select
	remotePathEntry     *widget.Entry
	modeSelect          *widget.Select
	conflictSelect      *widget.Select
	triggerModeSelect   *widget.Select
	enabledCheck        *widget.Check
	syncOnStartupCheck  *widget.Check
	// Files On Demand
	filesOnDemandCheck      *widget.Check
	autoDehydrateDaysSelect *widget.Select

	// SMB connections and shares
	smbConnections  []*SMBConnection
	availableShares []string

	// Help labels
	modeHelpLabel          *widget.Label
	triggerModeHelpLabel   *widget.Label
	filesOnDemandHelpLabel *widget.Label
}

// NewJobForm creates a new job form.
func NewJobForm(app *App, job *SyncJob, onSave func(*SyncJob)) *JobForm {
	jf := &JobForm{
		app:    app,
		job:    job,
		isNew:  job == nil,
		onSave: onSave,
	}

	if jf.isNew {
		jf.job = &SyncJob{
			Mode:               syncpkg.SyncModeMirror,
			ConflictResolution: "recent",
			Enabled:            true,
			TriggerMode:        SyncTriggerManual, // New jobs start in manual mode
		}
	}

	jf.smbConnections = app.GetSMBConnections()
	jf.createFields()
	return jf
}

// createFields creates the form fields.
func (jf *JobForm) createFields() {
	// Name
	jf.nameEntry = widget.NewEntry()
	jf.nameEntry.SetPlaceHolder("My Sync Job")
	jf.nameEntry.SetText(jf.job.Name)

	// Local path
	jf.localPathEntry = widget.NewEntry()
	jf.localPathEntry.SetPlaceHolder("C:\\Users\\Me\\Documents")
	jf.localPathEntry.SetText(jf.job.LocalPath)

	// SMB Connection selector
	connectionNames := make([]string, len(jf.smbConnections))
	selectedIndex := -1
	for i, conn := range jf.smbConnections {
		connectionNames[i] = conn.DisplayName()
		if conn.ID == jf.job.SMBConnectionID {
			selectedIndex = i
		}
	}

	jf.smbConnectionSelect = widget.NewSelect(connectionNames, func(selected string) {
		jf.onSMBConnectionChanged()
	})
	if selectedIndex >= 0 {
		jf.smbConnectionSelect.SetSelectedIndex(selectedIndex)
	}

	// Remote share select
	jf.remoteShareSelect = widget.NewSelect([]string{}, nil)
	jf.remoteShareSelect.PlaceHolder = "Select a share..."
	if jf.job.RemoteShare != "" {
		jf.availableShares = []string{jf.job.RemoteShare}
		jf.remoteShareSelect.Options = jf.availableShares
		jf.remoteShareSelect.SetSelected(jf.job.RemoteShare)
	}

	// Remote path (within share)
	jf.remotePathEntry = widget.NewEntry()
	jf.remotePathEntry.SetPlaceHolder("subfolder (optional)")
	jf.remotePathEntry.SetText(jf.job.RemotePath)

	// Sync mode
	jf.modeHelpLabel = widget.NewLabel("")
	jf.modeHelpLabel.Wrapping = fyne.TextWrapWord
	jf.modeHelpLabel.TextStyle = fyne.TextStyle{Italic: true}

	jf.modeSelect = widget.NewSelect([]string{
		"Mirror (bidirectional)",
		"Upload only",
		"Download only",
	}, func(selected string) {
		jf.updateModeHelp()
	})
	jf.modeSelect.SetSelectedIndex(jf.modeToIndex(jf.job.Mode))
	jf.updateModeHelp()

	// Conflict resolution
	jf.conflictSelect = widget.NewSelect([]string{
		"Keep most recent",
		"Keep local version",
		"Keep remote version",
		"Ask each time",
	}, nil)
	jf.conflictSelect.SetSelectedIndex(jf.conflictToIndex(jf.job.ConflictResolution))

	// Trigger mode (simplified sync scheduling)
	jf.triggerModeHelpLabel = widget.NewLabel("")
	jf.triggerModeHelpLabel.Wrapping = fyne.TextWrapWord
	jf.triggerModeHelpLabel.TextStyle = fyne.TextStyle{Italic: true}

	jf.triggerModeSelect = widget.NewSelect([]string{
		"Manual",
		"Every 5 minutes",
		"Every 15 minutes",
		"Every 30 minutes",
		"Every hour",
		"Realtime",
	}, func(selected string) {
		jf.updateTriggerModeHelp()
	})
	jf.triggerModeSelect.SetSelectedIndex(jf.triggerModeToIndex(jf.job.TriggerMode))
	jf.updateTriggerModeHelp()

	// Enabled
	jf.enabledCheck = widget.NewCheck("Enable this sync job", nil)
	jf.enabledCheck.SetChecked(jf.job.Enabled)

	// Sync on startup
	jf.syncOnStartupCheck = widget.NewCheck("Sync immediately on application startup", nil)
	jf.syncOnStartupCheck.SetChecked(jf.job.SyncOnStartup)

	// Files On Demand (Cloud Files API) - Windows 10 1709+ only
	jf.filesOnDemandHelpLabel = widget.NewLabel("Files appear in Explorer but are downloaded only when opened. Saves disk space. Requires Windows 10 1709+.")
	jf.filesOnDemandHelpLabel.Wrapping = fyne.TextWrapWord
	jf.filesOnDemandHelpLabel.TextStyle = fyne.TextStyle{Italic: true}

	jf.filesOnDemandCheck = widget.NewCheck("Download files on demand (placeholders)", nil)
	jf.filesOnDemandCheck.SetChecked(jf.job.FilesOnDemand)

	// Auto-dehydrate after X days
	jf.autoDehydrateDaysSelect = widget.NewSelect([]string{
		"Never (keep all files)",
		"After 7 days",
		"After 14 days",
		"After 30 days",
		"After 60 days",
		"After 90 days",
	}, nil)
	jf.autoDehydrateDaysSelect.SetSelectedIndex(jf.autoDehydrateDaysToIndex(jf.job.AutoDehydrateDays))
}

// Show displays the form dialog.
func (jf *JobForm) Show(parent fyne.Window) {
	title := "New Sync Job"
	if !jf.isNew {
		title = "Edit Sync Job"
	}

	// Check if there are SMB connections
	if len(jf.smbConnections) == 0 {
		dialog.ShowInformation("No SMB Servers",
			"Please add an SMB server in the 'SMB Servers' tab first before creating a sync job.",
			parent)
		return
	}

	// Create form layout
	form := container.NewVBox(
		widget.NewLabel("Job Name"),
		jf.nameEntry,
		widget.NewSeparator(),

		widget.NewLabel("Local Folder"),
		container.NewBorder(nil, nil, nil,
			widget.NewButton("Browse...", func() {
				jf.browseLocalFolder(parent)
			}),
			jf.localPathEntry,
		),
		widget.NewSeparator(),

		widget.NewLabel("SMB Server"),
		jf.smbConnectionSelect,
		container.NewVBox(
			widget.NewLabel("Share"),
			container.NewBorder(nil, nil, nil,
				widget.NewButton("Refresh", func() {
					jf.refreshShares(parent)
				}),
				jf.remoteShareSelect,
			),
		),
		container.NewVBox(
			widget.NewLabel("Path in Share (optional)"),
			container.NewBorder(nil, nil, nil,
				widget.NewButton("Browse...", func() {
					jf.browseRemotePath(parent)
				}),
				jf.remotePathEntry,
			),
		),
		widget.NewSeparator(),

		widget.NewLabel("Sync Settings"),
		container.NewGridWithColumns(2,
			container.NewVBox(
				widget.NewLabel("Sync Mode"),
				jf.modeSelect,
			),
			container.NewVBox(
				widget.NewLabel("On Conflict"),
				jf.conflictSelect,
			),
		),
		jf.modeHelpLabel,
		widget.NewSeparator(),

		widget.NewLabel("Sync Trigger"),
		jf.triggerModeSelect,
		jf.triggerModeHelpLabel,
		container.NewGridWithColumns(2,
			jf.enabledCheck,
			jf.syncOnStartupCheck,
		),
		widget.NewSeparator(),

		widget.NewLabel("Files On Demand (Windows 10+)"),
		jf.filesOnDemandHelpLabel,
		jf.filesOnDemandCheck,
		container.NewGridWithColumns(2,
			container.NewVBox(
				widget.NewLabel("Auto-free disk space"),
			),
			container.NewVBox(
				jf.autoDehydrateDaysSelect,
			),
		),
	)

	scroll := container.NewVScroll(form)
	scroll.SetMinSize(fyne.NewSize(450, 350))

	// Create buttons
	saveBtn := widget.NewButton("Save", func() {
		if jf.validate(parent) {
			jf.saveJob(parent)
			jf.dialog.Hide()
		}
	})
	saveBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		jf.dialog.Hide()
	})

	buttons := container.NewHBox(cancelBtn, saveBtn)
	content := container.NewBorder(nil, buttons, nil, nil, scroll)

	// Create dialog
	jf.dialog = dialog.NewCustomWithoutButtons(title, content, parent)
	jf.dialog.Resize(fyne.NewSize(500, 450))
	jf.dialog.Show()
}

// validate validates the form fields.
func (jf *JobForm) validate(parent fyne.Window) bool {
	if jf.nameEntry.Text == "" {
		dialog.ShowError(errFieldRequired("Job Name"), parent)
		return false
	}
	if jf.localPathEntry.Text == "" {
		dialog.ShowError(errFieldRequired("Local Folder"), parent)
		return false
	}
	if jf.smbConnectionSelect.SelectedIndex() < 0 {
		dialog.ShowError(errFieldRequired("SMB Server"), parent)
		return false
	}
	if jf.remoteShareSelect.Selected == "" {
		dialog.ShowError(errFieldRequired("Share"), parent)
		return false
	}
	return true
}

// saveJob saves the job configuration.
func (jf *JobForm) saveJob(parent fyne.Window) {
	// Get selected SMB connection
	selectedIdx := jf.smbConnectionSelect.SelectedIndex()
	if selectedIdx < 0 || selectedIdx >= len(jf.smbConnections) {
		dialog.ShowError(errFieldRequired("SMB Server"), parent)
		return
	}
	smbConn := jf.smbConnections[selectedIdx]

	// Update job from form
	jf.job.Name = jf.nameEntry.Text
	jf.job.LocalPath = jf.localPathEntry.Text
	jf.job.SMBConnectionID = smbConn.ID
	jf.job.RemoteHost = smbConn.Host
	jf.job.RemoteShare = jf.remoteShareSelect.Selected
	jf.job.Username = smbConn.Username
	jf.job.RemotePath = jf.remotePathEntry.Text
	jf.job.Mode = jf.indexToMode(jf.modeSelect.SelectedIndex())
	jf.job.ConflictResolution = jf.indexToConflict(jf.conflictSelect.SelectedIndex())
	jf.job.TriggerMode = jf.indexToTriggerMode(jf.triggerModeSelect.SelectedIndex())
	jf.job.Enabled = jf.enabledCheck.Checked
	jf.job.SyncOnStartup = jf.syncOnStartupCheck.Checked
	jf.job.FilesOnDemand = jf.filesOnDemandCheck.Checked
	jf.job.AutoDehydrateDays = jf.indexToAutoDehydrateDays(jf.autoDehydrateDaysSelect.SelectedIndex())

	// Save job first
	var err error
	if jf.isNew {
		err = jf.app.AddSyncJob(jf.job)
	} else {
		err = jf.app.UpdateSyncJob(jf.job)
	}

	if err != nil {
		dialog.ShowError(err, parent)
		return
	}

	// For new jobs in mirror mode, show the First Sync Wizard
	if jf.isNew && jf.job.Mode == syncpkg.SyncModeMirror && !jf.job.FirstSyncDone {
		jf.showFirstSyncWizard(parent)
		return
	}

	if jf.onSave != nil {
		jf.onSave(jf.job)
	}
}
