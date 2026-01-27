// Package app provides the shutdown configuration dialog.
package app

import (
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShutdownDialog manages the sync & shutdown configuration dialog.
type ShutdownDialog struct {
	app    *App
	dialog dialog.Dialog
	window fyne.Window

	// Form fields
	jobSelect     *widget.Select
	timeoutSelect *widget.Select
	forceCheck    *widget.Check

	// Pre-selected job IDs (nil = show "All Jobs")
	preselectedJobIDs []int64
}

// NewShutdownDialog creates a new shutdown configuration dialog.
func NewShutdownDialog(app *App, preselectedJobIDs []int64) *ShutdownDialog {
	return &ShutdownDialog{
		app:               app,
		preselectedJobIDs: preselectedJobIDs,
	}
}

// Show displays the shutdown configuration dialog.
func (d *ShutdownDialog) Show(parent fyne.Window) {
	d.window = parent

	// Build job selection options
	jobs := d.app.GetSyncJobs()
	jobOptions := []string{"All Enabled Jobs"}
	jobNameToID := make(map[string]int64)

	for _, job := range jobs {
		if job.Enabled {
			jobOptions = append(jobOptions, job.Name)
			jobNameToID[job.Name] = job.ID
		}
	}

	// Job selection
	d.jobSelect = widget.NewSelect(jobOptions, nil)
	if len(d.preselectedJobIDs) == 1 {
		// Find job name for preselected ID
		for _, job := range jobs {
			if job.ID == d.preselectedJobIDs[0] {
				d.jobSelect.SetSelected(job.Name)
				break
			}
		}
	} else {
		d.jobSelect.SetSelected("All Enabled Jobs")
	}

	// Timeout selection
	d.timeoutSelect = widget.NewSelect([]string{
		"15 minutes",
		"30 minutes",
		"1 hour",
		"2 hours",
		"Unlimited",
	}, nil)
	d.timeoutSelect.SetSelected("1 hour")

	// Force shutdown checkbox
	d.forceCheck = widget.NewCheck("Force shutdown even if sync fails", nil)

	// Help text
	helpText := widget.NewLabel("Windows will shutdown 30 seconds after sync completes.\nYou can cancel during sync or during the 30-second countdown.")
	helpText.Wrapping = fyne.TextWrapWord

	// Warning text
	warningText := widget.NewLabel("Warning: This will shutdown your computer!")
	warningText.Importance = widget.DangerImportance

	// Form layout
	form := container.NewVBox(
		widget.NewLabel("Select Job:"),
		d.jobSelect,
		widget.NewSeparator(),
		widget.NewLabel("Timeout:"),
		d.timeoutSelect,
		widget.NewSeparator(),
		d.forceCheck,
		widget.NewSeparator(),
		helpText,
		warningText,
	)

	// Create custom dialog with buttons
	startBtn := widget.NewButton("Start Sync & Shutdown", func() {
		d.onStart(jobNameToID)
	})
	startBtn.Importance = widget.DangerImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		if d.dialog != nil {
			d.dialog.Hide()
		}
	})

	buttons := container.NewHBox(
		cancelBtn,
		startBtn,
	)

	content := container.NewVBox(
		form,
		widget.NewSeparator(),
		container.NewCenter(buttons),
	)

	d.dialog = dialog.NewCustom("Sync & Shutdown", "", content, parent)
	d.dialog.Resize(fyne.NewSize(400, 350))
	d.dialog.Show()
}

// onStart is called when the user clicks Start.
func (d *ShutdownDialog) onStart(jobNameToID map[string]int64) {
	// Build config
	config := &ShutdownConfig{}

	// Determine job IDs
	selected := d.jobSelect.Selected
	if selected != "All Enabled Jobs" && selected != "" {
		if id, ok := jobNameToID[selected]; ok {
			config.JobIDs = []int64{id}
		}
	}
	// Empty JobIDs = all enabled jobs

	// Parse timeout
	switch d.timeoutSelect.Selected {
	case "15 minutes":
		config.Timeout = 15 * time.Minute
	case "30 minutes":
		config.Timeout = 30 * time.Minute
	case "1 hour":
		config.Timeout = 1 * time.Hour
	case "2 hours":
		config.Timeout = 2 * time.Hour
	default:
		config.Timeout = 0 // Unlimited
	}

	// Force shutdown
	config.ForceShutdown = d.forceCheck.Checked

	// Close config dialog
	if d.dialog != nil {
		d.dialog.Hide()
	}

	// Start the shutdown process
	d.app.StartSyncAndShutdown(config)
}

// timeoutOptions returns available timeout options.
func timeoutOptions() []string {
	return []string{
		"15 minutes",
		"30 minutes",
		"1 hour",
		"2 hours",
		"Unlimited",
	}
}
