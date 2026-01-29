package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cloudfiles"
)

// DehydrateDialog shows the "Free up space" dialog for a job.
type DehydrateDialog struct {
	app    *App
	job    *SyncJob
	window fyne.Window

	// UI elements
	daysSlider     *widget.Slider
	daysLabel      *widget.Label
	fileList       *widget.List
	totalSizeLabel *widget.Label
	fileCountLabel *widget.Label
	dehydrateBtn   *widget.Button
	refreshBtn     *widget.Button

	// Data
	files         []cloudfiles.HydratedFileInfo
	filteredFiles []cloudfiles.HydratedFileInfo
	minDays       int
}

// ShowDehydrateDialog displays the dehydration dialog for a job.
func (a *App) ShowDehydrateDialog(job *SyncJob) {
	if job == nil || !job.FilesOnDemand {
		return
	}

	d := &DehydrateDialog{
		app:     a,
		job:     job,
		minDays: 0,
	}
	d.show()
}

func (d *DehydrateDialog) show() {
	d.window = d.app.fyneApp.NewWindow(fmt.Sprintf("Free Up Space - %s", d.job.Name))
	d.window.Resize(fyne.NewSize(600, 500))

	// Days filter
	d.daysLabel = widget.NewLabel("All hydrated files")
	d.daysSlider = widget.NewSlider(0, 90)
	d.daysSlider.Step = 1
	d.daysSlider.OnChanged = d.onDaysChanged

	daysContainer := container.NewBorder(
		nil, nil,
		widget.NewLabel("Filter:"),
		d.daysLabel,
		d.daysSlider,
	)

	// File list
	d.fileList = widget.NewList(
		func() int { return len(d.filteredFiles) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewLabel("filename.ext"),
				widget.NewLabel("100 MB"),
				widget.NewLabel("30 days ago"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(d.filteredFiles) {
				return
			}
			file := d.filteredFiles[id]
			hbox := obj.(*fyne.Container)

			// Filename
			nameLabel := hbox.Objects[0].(*widget.Label)
			nameLabel.SetText(truncatePathForDisplay(file.Path, 40))

			// Size
			sizeLabel := hbox.Objects[1].(*widget.Label)
			sizeLabel.SetText(cloudfiles.FormatBytes(file.Size))

			// Days since access
			daysLabel := hbox.Objects[2].(*widget.Label)
			daysLabel.SetText(fmt.Sprintf("%d days ago", file.DaysSinceAccess))
		},
	)

	// Stats
	d.fileCountLabel = widget.NewLabel("0 files")
	d.totalSizeLabel = widget.NewLabel("0 bytes")
	statsContainer := container.NewHBox(
		d.fileCountLabel,
		widget.NewLabel(" - "),
		d.totalSizeLabel,
		widget.NewLabel(" can be freed"),
	)

	// Buttons
	d.refreshBtn = widget.NewButton("Refresh", d.refresh)
	d.dehydrateBtn = widget.NewButton("Free Up Space", d.onDehydrate)
	d.dehydrateBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Close", func() {
		d.window.Close()
	})

	buttonContainer := container.NewHBox(
		d.refreshBtn,
		container.NewHBox(), // spacer
		cancelBtn,
		d.dehydrateBtn,
	)

	// Layout
	content := container.NewBorder(
		container.NewVBox(
			widget.NewLabel(fmt.Sprintf("Local folder: %s", d.job.LocalPath)),
			widget.NewSeparator(),
			daysContainer,
		),
		container.NewVBox(
			widget.NewSeparator(),
			statsContainer,
			buttonContainer,
		),
		nil, nil,
		d.fileList,
	)

	d.window.SetContent(content)

	// Initial scan
	d.refresh()

	d.window.Show()
}

func (d *DehydrateDialog) onDaysChanged(value float64) {
	d.minDays = int(value)
	if d.minDays == 0 {
		d.daysLabel.SetText("All hydrated files")
	} else {
		d.daysLabel.SetText(fmt.Sprintf("Not used for %d+ days", d.minDays))
	}
	d.filterFiles()
	d.updateStats()
}

func (d *DehydrateDialog) refresh() {
	d.refreshBtn.Disable()
	d.dehydrateBtn.Disable()

	go func() {
		// Get provider for this job
		provider := d.app.syncManager.GetProvider(d.job.ID)
		if provider == nil {
			fyne.Do(func() {
				d.refreshBtn.Enable()
				dialog.ShowError(fmt.Errorf("Files On Demand not active for this job"), d.window)
			})
			return
		}

		// Scan for hydrated files
		dm := provider.GetDehydrationManager()
		if dm == nil {
			fyne.Do(func() {
				d.refreshBtn.Enable()
				dialog.ShowError(fmt.Errorf("Dehydration manager not available"), d.window)
			})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		files, err := dm.ScanHydratedFiles(ctx)
		if err != nil {
			fyne.Do(func() {
				d.refreshBtn.Enable()
				dialog.ShowError(fmt.Errorf("Scan failed: %w", err), d.window)
			})
			return
		}

		// Sort by days since access (oldest first)
		sort.Slice(files, func(i, j int) bool {
			return files[i].DaysSinceAccess > files[j].DaysSinceAccess
		})

		fyne.Do(func() {
			d.files = files
			d.filterFiles()
			d.updateStats()
			d.refreshBtn.Enable()
			d.dehydrateBtn.Enable()
		})
	}()
}

func (d *DehydrateDialog) filterFiles() {
	if d.minDays == 0 {
		d.filteredFiles = d.files
	} else {
		d.filteredFiles = nil
		for _, f := range d.files {
			if f.DaysSinceAccess >= d.minDays {
				d.filteredFiles = append(d.filteredFiles, f)
			}
		}
	}
	d.fileList.Refresh()
}

func (d *DehydrateDialog) updateStats() {
	var totalSize int64
	for _, f := range d.filteredFiles {
		totalSize += f.Size
	}

	d.fileCountLabel.SetText(fmt.Sprintf("%d files", len(d.filteredFiles)))
	d.totalSizeLabel.SetText(cloudfiles.FormatBytes(totalSize))

	d.dehydrateBtn.Disable()
	if len(d.filteredFiles) > 0 {
		d.dehydrateBtn.Enable()
	}
}

func (d *DehydrateDialog) onDehydrate() {
	if len(d.filteredFiles) == 0 {
		return
	}

	var totalSize int64
	for _, f := range d.filteredFiles {
		totalSize += f.Size
	}

	msg := fmt.Sprintf("Free up %s from %d files?\n\nFiles will become placeholders and will be downloaded again when opened.",
		cloudfiles.FormatBytes(totalSize), len(d.filteredFiles))

	dialog.ShowConfirm("Confirm", msg, func(confirmed bool) {
		if confirmed {
			d.doDehydrate()
		}
	}, d.window)
}

func (d *DehydrateDialog) doDehydrate() {
	d.dehydrateBtn.Disable()
	d.refreshBtn.Disable()

	filesToDehydrate := make([]cloudfiles.HydratedFileInfo, len(d.filteredFiles))
	copy(filesToDehydrate, d.filteredFiles)

	go func() {
		provider := d.app.syncManager.GetProvider(d.job.ID)
		if provider == nil {
			fyne.Do(func() {
				d.refreshBtn.Enable()
				dialog.ShowError(fmt.Errorf("Provider not available"), d.window)
			})
			return
		}

		dm := provider.GetDehydrationManager()
		if dm == nil {
			fyne.Do(func() {
				d.refreshBtn.Enable()
				dialog.ShowError(fmt.Errorf("Dehydration manager not available"), d.window)
			})
			return
		}

		ctx := context.Background()
		successCount := 0
		var bytesFreed int64
		var lastErr error

		for _, file := range filesToDehydrate {
			if err := dm.DehydrateFile(ctx, file.Path); err != nil {
				lastErr = err
				d.app.Logger().Warn("Failed to dehydrate file: " + file.Path + ": " + err.Error())
				continue
			}
			successCount++
			bytesFreed += file.Size
		}

		fyne.Do(func() {
			d.refreshBtn.Enable()

			if successCount > 0 {
				msg := fmt.Sprintf("Freed %s from %d files", cloudfiles.FormatBytes(bytesFreed), successCount)
				if lastErr != nil {
					msg += fmt.Sprintf("\n\nSome files failed: %v", lastErr)
				}
				dialog.ShowInformation("Complete", msg, d.window)
			} else if lastErr != nil {
				dialog.ShowError(fmt.Errorf("Failed to dehydrate files: %w", lastErr), d.window)
			}

			// Refresh the list
			d.refresh()
		})
	}()
}

// truncatePathForDisplay truncates a path to maxLen characters with ellipsis.
func truncatePathForDisplay(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
