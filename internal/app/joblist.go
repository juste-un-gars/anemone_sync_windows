package app

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// JobsList displays the list of sync jobs.
type JobsList struct {
	app       *App
	list      *widget.List
	jobs      []*SyncJob
	selected  int
	container *fyne.Container
}

// NewJobsList creates a new jobs list widget.
func NewJobsList(app *App) *JobsList {
	jl := &JobsList{
		app:      app,
		selected: -1,
	}

	jl.loadJobs()
	jl.createList()

	return jl
}

// loadJobs loads jobs from the app.
func (jl *JobsList) loadJobs() {
	jl.jobs = jl.app.GetSyncJobs()
}

// createList creates the list widget.
func (jl *JobsList) createList() {
	jl.list = widget.NewList(
		func() int {
			return len(jl.jobs)
		},
		func() fyne.CanvasObject {
			return jl.createJobItem()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			jl.updateJobItem(id, obj)
		},
	)

	jl.list.OnSelected = func(id widget.ListItemID) {
		jl.selected = int(id)
	}

	jl.list.OnUnselected = func(id widget.ListItemID) {
		if jl.selected == int(id) {
			jl.selected = -1
		}
	}

	// Empty state message
	emptyLabel := widget.NewLabel("No sync jobs configured.\nClick 'Add Job' to create one.")
	emptyLabel.Alignment = fyne.TextAlignCenter

	jl.container = container.NewStack(jl.list, emptyLabel)
	jl.updateEmptyState()
}

// createJobItem creates a template job list item.
func (jl *JobsList) createJobItem() fyne.CanvasObject {
	statusIndicator := canvas.NewCircle(theme.DisabledColor())
	statusIndicator.Resize(fyne.NewSize(12, 12))

	nameLabel := widget.NewLabel("Job Name")
	nameLabel.TextStyle = fyne.TextStyle{Bold: true}

	pathLabel := widget.NewLabel("Local: /path/to/folder")
	pathLabel.TextStyle = fyne.TextStyle{}

	remoteLabel := widget.NewLabel("Remote: \\\\server\\share")
	remoteLabel.TextStyle = fyne.TextStyle{}

	statusLabel := widget.NewLabel("Status: Idle")
	statusLabel.Alignment = fyne.TextAlignTrailing

	sizeLabel := widget.NewLabel("Size: --")
	sizeLabel.Alignment = fyne.TextAlignTrailing

	lastSyncLabel := widget.NewLabel("Last sync: Never")
	lastSyncLabel.Alignment = fyne.TextAlignTrailing

	leftContent := container.NewVBox(
		nameLabel,
		pathLabel,
		remoteLabel,
	)

	rightContent := container.NewVBox(
		statusLabel,
		sizeLabel,
		lastSyncLabel,
	)

	return container.NewBorder(
		nil, nil,
		container.NewCenter(statusIndicator),
		rightContent,
		leftContent,
	)
}

// updateJobItem updates a job list item with actual data.
func (jl *JobsList) updateJobItem(id widget.ListItemID, obj fyne.CanvasObject) {
	if id >= len(jl.jobs) {
		return
	}

	job := jl.jobs[id]

	// Get container components
	border := obj.(*fyne.Container)

	// Status indicator (left side)
	leftContainer := border.Objects[1].(*fyne.Container)
	statusCircle := leftContainer.Objects[0].(*canvas.Circle)
	statusCircle.FillColor = jl.getStatusColor(job.LastStatus)
	statusCircle.Refresh()

	// Left content (center in border)
	leftContent := border.Objects[0].(*fyne.Container)
	nameLabel := leftContent.Objects[0].(*widget.Label)
	pathLabel := leftContent.Objects[1].(*widget.Label)
	remoteLabel := leftContent.Objects[2].(*widget.Label)

	nameLabel.SetText(job.Name)
	pathLabel.SetText("Local: " + truncatePath(job.LocalPath, 40))
	remoteLabel.SetText("Remote: " + truncatePath(job.FullRemotePath(), 40))

	// Right content
	rightContent := border.Objects[2].(*fyne.Container)
	statusLabel := rightContent.Objects[0].(*widget.Label)
	sizeLabel := rightContent.Objects[1].(*widget.Label)
	lastSyncLabel := rightContent.Objects[2].(*widget.Label)

	statusLabel.SetText("Status: " + job.LastStatus.String())

	// Display size information
	if job.LocalSize > 0 {
		sizeLabel.SetText(fmt.Sprintf("Size: %s (%d files)", formatBytes(job.LocalSize), job.LocalFileCount))
	} else {
		sizeLabel.SetText("Size: calculating...")
	}

	if job.LastSync.IsZero() {
		lastSyncLabel.SetText("Last sync: Never")
	} else {
		lastSyncLabel.SetText("Last sync: " + job.LastSync.Format("2006-01-02 15:04"))
	}
}

// getStatusColor returns the color for a job status.
func (jl *JobsList) getStatusColor(status JobStatus) color.Color {
	switch status {
	case JobStatusSuccess:
		return color.RGBA{R: 0, G: 200, B: 83, A: 255} // Green
	case JobStatusSyncing:
		return color.RGBA{R: 33, G: 150, B: 243, A: 255} // Blue
	case JobStatusPartial:
		return color.RGBA{R: 255, G: 152, B: 0, A: 255} // Orange
	case JobStatusFailed:
		return color.RGBA{R: 244, G: 67, B: 54, A: 255} // Red
	case JobStatusDisabled:
		return theme.DisabledColor()
	default:
		return theme.DisabledColor()
	}
}

// Container returns the container for the jobs list.
func (jl *JobsList) Container() fyne.CanvasObject {
	return jl.container
}

// GetSelected returns the currently selected job, or nil if none.
func (jl *JobsList) GetSelected() *SyncJob {
	if jl.selected < 0 || jl.selected >= len(jl.jobs) {
		return nil
	}
	return jl.jobs[jl.selected]
}

// Refresh reloads the jobs and refreshes the list.
func (jl *JobsList) Refresh() {
	jl.loadJobs()
	fyne.Do(func() {
		jl.list.Refresh()
		jl.updateEmptyState()
	})
}

// updateEmptyState shows/hides empty state message.
func (jl *JobsList) updateEmptyState() {
	if len(jl.jobs) == 0 {
		jl.list.Hide()
		jl.container.Objects[1].Show()
	} else {
		jl.container.Objects[1].Hide()
		jl.list.Show()
	}
}

// truncatePath truncates a path to max length.
func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

// formatBytes formats bytes to human readable string.
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
