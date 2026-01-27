package app

import (
	"path"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// RemoteFolderBrowser allows browsing folders on an SMB share.
type RemoteFolderBrowser struct {
	app        *App
	smbConn    *SMBConnection
	share      string
	currentPath string
	onSelect   func(string)
	dialog     dialog.Dialog

	// UI components
	pathLabel  *widget.Label
	folderList *widget.List
	folders    []string
}

// NewRemoteFolderBrowser creates a new remote folder browser.
func NewRemoteFolderBrowser(app *App, conn *SMBConnection, share string, initialPath string, onSelect func(string)) *RemoteFolderBrowser {
	return &RemoteFolderBrowser{
		app:         app,
		smbConn:     conn,
		share:       share,
		currentPath: initialPath,
		onSelect:    onSelect,
		folders:     []string{},
	}
}

// Show displays the browser dialog.
func (b *RemoteFolderBrowser) Show(parent fyne.Window) {
	// Path display
	b.pathLabel = widget.NewLabel("/" + b.currentPath)
	b.pathLabel.TextStyle = fyne.TextStyle{Bold: true}

	// Folder list
	b.folderList = widget.NewList(
		func() int { return len(b.folders) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.FolderIcon()),
				widget.NewLabel("folder name placeholder"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			box := obj.(*fyne.Container)
			label := box.Objects[1].(*widget.Label)
			label.SetText(b.folders[id])
		},
	)

	b.folderList.OnSelected = func(id widget.ListItemID) {
		if id >= 0 && id < len(b.folders) {
			selectedFolder := b.folders[id]
			// Navigate into the folder
			if b.currentPath == "" {
				b.currentPath = selectedFolder
			} else {
				b.currentPath = path.Join(b.currentPath, selectedFolder)
			}
			b.loadFolders(parent)
		}
	}

	// Navigation buttons
	upBtn := widget.NewButtonWithIcon("Up", theme.MoveUpIcon(), func() {
		b.navigateUp(parent)
	})

	rootBtn := widget.NewButtonWithIcon("Root", theme.HomeIcon(), func() {
		b.currentPath = ""
		b.loadFolders(parent)
	})

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		b.loadFolders(parent)
	})

	navBar := container.NewHBox(upBtn, rootBtn, refreshBtn)

	// Select and cancel buttons
	selectBtn := widget.NewButton("Select This Folder", func() {
		if b.onSelect != nil {
			b.onSelect(b.currentPath)
		}
		b.dialog.Hide()
	})
	selectBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		b.dialog.Hide()
	})

	buttons := container.NewHBox(cancelBtn, selectBtn)

	// Layout
	header := container.NewVBox(
		widget.NewLabel("Browse: \\\\"+b.smbConn.Host+"\\"+b.share),
		container.NewBorder(nil, nil, widget.NewLabel("Path:"), nil, b.pathLabel),
		navBar,
		widget.NewSeparator(),
	)

	listContainer := container.NewStack(b.folderList)

	content := container.NewBorder(header, buttons, nil, nil, listContainer)

	b.dialog = dialog.NewCustomWithoutButtons("Select Remote Folder", content, parent)
	b.dialog.Resize(fyne.NewSize(450, 400))
	b.dialog.Show()

	// Load initial folders
	b.loadFolders(parent)
}

// navigateUp goes to the parent folder.
func (b *RemoteFolderBrowser) navigateUp(parent fyne.Window) {
	if b.currentPath == "" {
		return
	}

	// Go to parent directory
	parts := strings.Split(b.currentPath, "/")
	if len(parts) <= 1 {
		b.currentPath = ""
	} else {
		b.currentPath = strings.Join(parts[:len(parts)-1], "/")
	}
	b.loadFolders(parent)
}

// loadFolders loads the folder list from the SMB share.
func (b *RemoteFolderBrowser) loadFolders(parent fyne.Window) {
	// Update path label
	displayPath := "/" + b.currentPath
	if b.currentPath == "" {
		displayPath = "/ (root)"
	}
	b.pathLabel.SetText(displayPath)

	// Show loading state
	b.folders = []string{"Loading..."}
	b.folderList.Refresh()

	go func() {
		folders, err := b.app.ListRemoteFolders(b.smbConn.ID, b.share, b.currentPath)

		fyne.Do(func() {
			if err != nil {
				b.folders = []string{}
				b.folderList.Refresh()
				dialog.ShowError(err, parent)
				return
			}

			// Sort folders alphabetically
			sort.Strings(folders)
			b.folders = folders
			b.folderList.Refresh()
		})
	}()
}
