package app

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	registryKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	registryKeyName = "AnemoneSync"
)

// AutoStart handles Windows auto-start functionality via registry.
type AutoStart struct {
	exePath string
}

// NewAutoStart creates a new AutoStart instance.
func NewAutoStart() (*AutoStart, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &AutoStart{exePath: exePath}, nil
}

// IsEnabled returns whether auto-start is currently enabled.
func (a *AutoStart) IsEnabled() bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()

	value, _, err := key.GetStringValue(registryKeyName)
	if err != nil {
		return false
	}

	// Check if the registered path matches our executable (with or without --autostart flag)
	// Expected format: "C:\path\to\exe" --autostart
	cleanExe := filepath.Clean(a.exePath)
	cleanValue := filepath.Clean(value)

	// Direct match (legacy without flag)
	if cleanValue == cleanExe {
		return true
	}

	// Match with quotes and flag
	expectedCmd := `"` + cleanExe + `" --autostart`
	return value == expectedCmd
}

// Enable enables auto-start by adding a registry entry.
// The --autostart flag is added to distinguish autostart from manual launch.
func (a *AutoStart) Enable() error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, registryKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	// Add --autostart flag so the app knows it was launched at Windows startup
	cmdLine := `"` + a.exePath + `" --autostart`
	return key.SetStringValue(registryKeyName, cmdLine)
}

// Disable disables auto-start by removing the registry entry.
func (a *AutoStart) Disable() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryKeyPath, registry.SET_VALUE)
	if err != nil {
		// Key doesn't exist, nothing to do
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer key.Close()

	err = key.DeleteValue(registryKeyName)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}

// SetEnabled enables or disables auto-start.
func (a *AutoStart) SetEnabled(enabled bool) error {
	if enabled {
		return a.Enable()
	}
	return a.Disable()
}
