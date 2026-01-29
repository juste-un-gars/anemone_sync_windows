package app

import (
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

// Tray manages the system tray icon and menu using Fyne's desktop driver.
type Tray struct {
	app        *App
	desktopApp desktop.App
	menu       *fyne.Menu
	ready      bool // True when systray is fully initialized
	mu         sync.Mutex // Protects menu refresh operations

	// Menu items that need dynamic updates
	syncNowItem         *fyne.MenuItem
	stopSyncItem        *fyne.MenuItem
	syncShutdownMenu    *fyne.MenuItem
	cancelShutdownItem  *fyne.MenuItem
	freeSpaceMenu       *fyne.MenuItem
}

// NewTray creates a new Tray instance.
func NewTray(app *App) *Tray {
	return &Tray{
		app: app,
	}
}

// Setup initializes the system tray. Must be called after FyneApp is created.
func (t *Tray) Setup() {
	// Check if we're running on a desktop that supports system tray
	desktopApp, ok := t.app.fyneApp.(desktop.App)
	if !ok {
		t.app.Logger().Warn("System tray not supported on this platform")
		return
	}
	t.desktopApp = desktopApp

	t.app.Logger().Debug("Setting up system tray")

	// Create menu items
	statusItem := fyne.NewMenuItem("Status: Idle", nil)
	statusItem.Disabled = true

	t.syncNowItem = fyne.NewMenuItem("Sync Now", func() {
		t.app.Logger().Info("Sync Now clicked")
		t.app.TriggerSync()
	})

	t.stopSyncItem = fyne.NewMenuItem("Stop Sync", func() {
		t.app.Logger().Info("Stop Sync clicked")
		t.app.StopSync()
	})
	t.stopSyncItem.Disabled = true // Initially disabled (no sync running)

	// Sync & Shutdown submenu
	t.syncShutdownMenu = t.buildSyncShutdownMenu()

	// Cancel Shutdown item (shown when shutdown is pending)
	t.cancelShutdownItem = fyne.NewMenuItem("Cancel Shutdown", func() {
		t.app.Logger().Info("Cancel Shutdown clicked")
		t.app.CancelSyncAndShutdown()
	})
	t.cancelShutdownItem.Disabled = true // Initially disabled

	// Free Up Space submenu
	t.freeSpaceMenu = t.buildFreeSpaceMenu()

	settingsItem := fyne.NewMenuItem("Settings...", func() {
		t.app.Logger().Info("Settings clicked")
		t.app.ShowSettings()
	})

	quitItem := fyne.NewMenuItem("Quit", func() {
		t.app.Logger().Info("Quit clicked")
		t.app.Quit()
	})

	// Build menu
	t.menu = fyne.NewMenu("AnemoneSync",
		statusItem,
		fyne.NewMenuItemSeparator(),
		t.syncNowItem,
		t.stopSyncItem,
		fyne.NewMenuItemSeparator(),
		t.syncShutdownMenu,
		t.cancelShutdownItem,
		fyne.NewMenuItemSeparator(),
		t.freeSpaceMenu,
		fyne.NewMenuItemSeparator(),
		settingsItem,
		fyne.NewMenuItemSeparator(),
		quitItem,
	)

	// Set system tray icon and menu
	iconResource := fyne.NewStaticResource("icon.png", iconData)
	t.desktopApp.SetSystemTrayIcon(iconResource)
	t.desktopApp.SetSystemTrayMenu(t.menu)

	t.ready = true
	t.app.Logger().Debug("System tray ready")
}

// UpdateStatus updates the status display in the tray menu.
func (t *Tray) UpdateStatus() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready || t.menu == nil || len(t.menu.Items) == 0 {
		return
	}

	status := t.app.GetStatus()
	t.menu.Items[0].Label = "Status: " + status

	// Update Sync Now / Stop Sync button states based on syncing status
	isSyncing := t.app.IsSyncing()
	if t.syncNowItem != nil {
		t.syncNowItem.Disabled = isSyncing
	}
	if t.stopSyncItem != nil {
		t.stopSyncItem.Disabled = !isSyncing
	}

	t.menu.Refresh()
}

// SetSyncEnabled enables/disables the sync menu item.
func (t *Tray) SetSyncEnabled(enabled bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.syncNowItem == nil {
		return
	}

	// Only enable Sync Now if not currently syncing
	if enabled && !t.app.IsSyncing() {
		t.syncNowItem.Disabled = false
	} else if !enabled {
		t.syncNowItem.Disabled = true
	}

	if t.menu != nil {
		t.menu.Refresh()
	}
}

// buildSyncShutdownMenu creates the Sync & Shutdown submenu.
func (t *Tray) buildSyncShutdownMenu() *fyne.MenuItem {
	// Create submenu items
	syncAllItem := fyne.NewMenuItem("Sync All & Shutdown...", func() {
		t.app.Logger().Info("Sync All & Shutdown clicked")
		t.app.ShowShutdownDialog(nil) // nil = all jobs
	})

	// Build the submenu with job-specific options
	menuItems := []*fyne.MenuItem{
		syncAllItem,
		fyne.NewMenuItemSeparator(),
	}

	// Add enabled jobs
	jobs := t.app.GetSyncJobs()
	for _, job := range jobs {
		if job.Enabled {
			j := job // capture for closure
			item := fyne.NewMenuItem(j.Name+" & Shutdown...", func() {
				t.app.Logger().Info("Sync job & Shutdown clicked")
				t.app.ShowShutdownDialog([]int64{j.ID})
			})
			menuItems = append(menuItems, item)
		}
	}

	// Create the parent menu item with submenu
	syncShutdownItem := fyne.NewMenuItem("Sync & Shutdown", nil)
	syncShutdownItem.ChildMenu = fyne.NewMenu("", menuItems...)

	return syncShutdownItem
}

// RefreshSyncShutdownMenu rebuilds the Sync & Shutdown submenu with current jobs.
func (t *Tray) RefreshSyncShutdownMenu() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready || t.menu == nil {
		return
	}

	// Rebuild the submenu
	t.syncShutdownMenu = t.buildSyncShutdownMenu()

	// Find and replace the menu item
	for i, item := range t.menu.Items {
		if item == t.syncShutdownMenu || (item.ChildMenu != nil && item.Label == "Sync & Shutdown") {
			t.menu.Items[i] = t.syncShutdownMenu
			break
		}
	}

	t.menu.Refresh()
}

// UpdateShutdownState updates the menu based on shutdown operation state.
func (t *Tray) UpdateShutdownState(active bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready {
		return
	}

	if t.syncShutdownMenu != nil {
		t.syncShutdownMenu.Disabled = active
	}
	if t.cancelShutdownItem != nil {
		t.cancelShutdownItem.Disabled = !active
	}

	if t.menu != nil {
		t.menu.Refresh()
	}
}

// buildFreeSpaceMenu creates the "Free Up Space" submenu.
func (t *Tray) buildFreeSpaceMenu() *fyne.MenuItem {
	menuItems := []*fyne.MenuItem{}

	// Add jobs with Files On Demand enabled
	jobs := t.app.GetSyncJobs()
	hasFilesOnDemand := false

	for _, job := range jobs {
		if job.FilesOnDemand && job.Enabled {
			hasFilesOnDemand = true
			j := job // capture for closure
			item := fyne.NewMenuItem(j.Name+"...", func() {
				t.app.Logger().Info("Free Up Space clicked for " + j.Name)
				t.app.ShowDehydrateDialog(j)
			})
			menuItems = append(menuItems, item)
		}
	}

	// Create the parent menu item
	freeSpaceItem := fyne.NewMenuItem("Free Up Space", nil)

	if hasFilesOnDemand {
		freeSpaceItem.ChildMenu = fyne.NewMenu("", menuItems...)
	} else {
		// No jobs with Files On Demand - disable the menu
		freeSpaceItem.Disabled = true
	}

	return freeSpaceItem
}

// RefreshFreeSpaceMenu rebuilds the Free Up Space submenu with current jobs.
func (t *Tray) RefreshFreeSpaceMenu() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready || t.menu == nil {
		return
	}

	// Rebuild the submenu
	t.freeSpaceMenu = t.buildFreeSpaceMenu()

	// Find and replace the menu item
	for i, item := range t.menu.Items {
		if item.Label == "Free Up Space" {
			t.menu.Items[i] = t.freeSpaceMenu
			break
		}
	}

	t.menu.Refresh()
}
