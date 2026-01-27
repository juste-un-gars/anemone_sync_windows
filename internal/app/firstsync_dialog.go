package app

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// FirstSyncDialog handles the first sync wizard UI
type FirstSyncDialog struct {
	app      *App
	job      *SyncJob
	analysis *FirstSyncAnalysis
	window   fyne.Window

	// User choices
	selectedMode   FirstSyncMode
	selectedTrust  TrustSource

	// Callbacks
	onComplete func(mode FirstSyncMode, trust TrustSource)
	onCancel   func()
}

// NewFirstSyncDialog creates a new first sync dialog
func NewFirstSyncDialog(app *App, job *SyncJob, parent fyne.Window) *FirstSyncDialog {
	return &FirstSyncDialog{
		app:           app,
		job:           job,
		selectedMode:  FirstSyncModeMerge,
		selectedTrust: TrustSourceAsk,
	}
}

// Show displays the first sync wizard
func (d *FirstSyncDialog) Show(onComplete func(FirstSyncMode, TrustSource), onCancel func()) {
	d.onComplete = onComplete
	d.onCancel = onCancel

	// Create window
	d.window = d.app.fyneApp.NewWindow("First Sync - " + d.job.Name)
	d.window.Resize(fyne.NewSize(500, 400))
	d.window.CenterOnScreen()

	// Show analyzing screen first
	d.showAnalyzing()
	d.window.Show()

	// Run analysis in background
	go d.runAnalysis()
}

// showAnalyzing shows the analyzing progress screen
func (d *FirstSyncDialog) showAnalyzing() {
	progress := widget.NewProgressBarInfinite()

	content := container.NewVBox(
		widget.NewLabel("Analyzing differences..."),
		widget.NewLabel(""),
		widget.NewLabel(fmt.Sprintf("Local: %s", d.job.LocalPath)),
		widget.NewLabel(fmt.Sprintf("Remote: %s", d.job.FullRemotePath())),
		widget.NewLabel(""),
		progress,
		widget.NewLabel(""),
		widget.NewLabel("This may take a moment for large folders."),
	)

	d.window.SetContent(container.NewPadded(content))
}

// runAnalysis runs the analysis and updates the UI
func (d *FirstSyncDialog) runAnalysis() {
	analyzer := NewFirstSyncAnalyzer(d.app)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	analysis, err := analyzer.Analyze(ctx, d.job)
	if err != nil {
		fyne.Do(func() {
			d.showError(err)
		})
		return
	}

	d.analysis = analysis

	fyne.Do(func() {
		d.showResults()
	})
}

// showError shows an error message
func (d *FirstSyncDialog) showError(err error) {
	content := container.NewVBox(
		widget.NewIcon(theme.ErrorIcon()),
		widget.NewLabel("Analysis failed"),
		widget.NewLabel(err.Error()),
		widget.NewLabel(""),
		widget.NewButton("Close", func() {
			d.window.Close()
			if d.onCancel != nil {
				d.onCancel()
			}
		}),
	)

	d.window.SetContent(container.NewPadded(content))
}

// showResults shows the analysis results and options
func (d *FirstSyncDialog) showResults() {
	a := d.analysis

	// Summary section
	summaryTitle := widget.NewLabelWithStyle("Analysis Results", fyne.TextAlignCenter, fyne.TextStyle{Bold: true})

	localInfo := fmt.Sprintf("Local: %d files (%.2f MB)", a.LocalFileCount, float64(a.LocalTotalSize)/(1024*1024))
	remoteInfo := fmt.Sprintf("Remote: %d files (%.2f MB)", a.RemoteFileCount, float64(a.RemoteTotalSize)/(1024*1024))

	summaryBox := container.NewVBox(
		summaryTitle,
		widget.NewSeparator(),
		widget.NewLabel(localInfo),
		widget.NewLabel(remoteInfo),
		widget.NewLabel(fmt.Sprintf("Analysis time: %v", a.AnalysisDuration.Round(100*1000000))),
	)

	// Differences section
	var diffContent fyne.CanvasObject

	if !a.HasDifferences() {
		diffContent = container.NewVBox(
			widget.NewIcon(theme.ConfirmIcon()),
			widget.NewLabel("No differences found!"),
			widget.NewLabel("Local and remote are already in sync."),
		)
	} else {
		diffList := container.NewVBox()

		if len(a.LocalOnlyFiles) > 0 {
			diffList.Add(widget.NewLabelWithStyle(
				fmt.Sprintf("📤 %d files only on PC (will be uploaded)", len(a.LocalOnlyFiles)),
				fyne.TextAlignLeading, fyne.TextStyle{}))
		}
		if len(a.RemoteOnlyFiles) > 0 {
			diffList.Add(widget.NewLabelWithStyle(
				fmt.Sprintf("📥 %d files only on Server (will be downloaded)", len(a.RemoteOnlyFiles)),
				fyne.TextAlignLeading, fyne.TextStyle{}))
		}
		if len(a.ConflictFiles) > 0 {
			diffList.Add(widget.NewLabelWithStyle(
				fmt.Sprintf("⚠️  %d files exist on both with different content", len(a.ConflictFiles)),
				fyne.TextAlignLeading, fyne.TextStyle{}))
		}
		if a.SameFiles > 0 {
			diffList.Add(widget.NewLabelWithStyle(
				fmt.Sprintf("✓ %d files already in sync", a.SameFiles),
				fyne.TextAlignLeading, fyne.TextStyle{}))
		}

		diffContent = diffList
	}

	// Build content list
	contentItems := []fyne.CanvasObject{
		summaryBox,
		widget.NewSeparator(),
		diffContent,
	}

	// Conflict resolution - only show if there are conflicts
	if len(a.ConflictFiles) > 0 {
		conflictTitle := widget.NewLabelWithStyle(
			fmt.Sprintf("For the %d conflicting files:", len(a.ConflictFiles)),
			fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

		// Default to "recent" which is safest
		d.selectedTrust = TrustSourceRecent

		conflictGroup := widget.NewRadioGroup([]string{
			"Most recent wins (recommended)",
			"PC version wins (upload to server)",
			"Server version wins (download to PC)",
			"Keep both (download server version with .server suffix)",
		}, func(selected string) {
			switch selected {
			case "Most recent wins (recommended)":
				d.selectedTrust = TrustSourceRecent
			case "PC version wins (upload to server)":
				d.selectedTrust = TrustSourceLocal
			case "Server version wins (download to PC)":
				d.selectedTrust = TrustSourceServer
			case "Keep both (download server version with .server suffix)":
				d.selectedTrust = TrustSourceKeepBoth
			}
		})
		conflictGroup.SetSelected("Most recent wins (recommended)")

		contentItems = append(contentItems,
			widget.NewSeparator(),
			conflictTitle,
			conflictGroup,
		)
	} else {
		// No conflicts - set default trust source for future use
		d.selectedTrust = TrustSourceRecent
	}

	// Buttons
	cancelBtn := widget.NewButton("Cancel", func() {
		d.window.Close()
		if d.onCancel != nil {
			d.onCancel()
		}
	})

	startBtn := widget.NewButton("Start Sync", func() {
		d.window.Close()
		if d.onComplete != nil {
			d.onComplete(d.selectedMode, d.selectedTrust)
		}
	})
	startBtn.Importance = widget.HighImportance

	buttons := container.NewHBox(
		cancelBtn,
		widget.NewLabel(""), // Spacer
		startBtn,
	)

	contentItems = append(contentItems,
		widget.NewSeparator(),
		buttons,
	)

	// Build final layout
	content := container.NewVBox(contentItems...)

	scrollable := container.NewScroll(content)
	d.window.SetContent(container.NewPadded(scrollable))
	d.window.Resize(fyne.NewSize(550, 450))
}

// FormatBytes formats bytes to human readable string
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
