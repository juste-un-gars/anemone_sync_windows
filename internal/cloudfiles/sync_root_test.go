//go:build windows
// +build windows

package cloudfiles

import (
	"os"
	"testing"
)

func TestNewSyncRootManager(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:            tempDir,
		ProviderName:    "TestProvider",
		ProviderVersion: "1.0.0",
		ProviderID:      DefaultProviderID(),
	}

	manager, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("NewSyncRootManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("manager should not be nil")
	}

	if manager.Path() != tempDir {
		t.Errorf("Path mismatch: got %s, want %s", manager.Path(), tempDir)
	}

	if manager.IsRegistered() {
		t.Error("should not be registered initially")
	}

	if manager.IsConnected() {
		t.Error("should not be connected initially")
	}
}

func TestNewSyncRootManagerDefaults(t *testing.T) {
	tempDir := t.TempDir()

	// Test with minimal config
	config := SyncRootConfig{
		Path: tempDir,
	}

	manager, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("NewSyncRootManager failed: %v", err)
	}

	// Should use defaults
	if manager.providerName != "AnemoneSync" {
		t.Errorf("Expected default provider name 'AnemoneSync', got '%s'", manager.providerName)
	}

	if manager.providerVersion != "1.0.0" {
		t.Errorf("Expected default version '1.0.0', got '%s'", manager.providerVersion)
	}
}

func TestNewSyncRootManagerEmptyPath(t *testing.T) {
	config := SyncRootConfig{
		Path: "",
	}

	_, err := NewSyncRootManager(config)
	if err == nil {
		t.Error("Expected error for empty path")
	}
}

func TestDefaultProviderID(t *testing.T) {
	guid := DefaultProviderID()

	// Verify it's not all zeros
	if guid.Data1 == 0 && guid.Data2 == 0 && guid.Data3 == 0 {
		t.Error("GUID should not be all zeros")
	}

	t.Logf("Default Provider GUID: {%08X-%04X-%04X-%X}",
		guid.Data1, guid.Data2, guid.Data3, guid.Data4)
}

func TestSyncRootManagerCallbacks(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path: tempDir,
	}

	manager, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("NewSyncRootManager failed: %v", err)
	}

	// Test setting callbacks (should not panic)
	fetchCalled := false
	manager.SetFetchDataCallback(func(info *FetchDataInfo) error {
		fetchCalled = true
		return nil
	})

	cancelCalled := false
	manager.SetCancelFetchCallback(func(filePath string) {
		cancelCalled = true
	})

	deleteCalled := false
	manager.SetNotifyDeleteCallback(func(filePath string, isDir bool) bool {
		deleteCalled = true
		return true
	})

	renameCalled := false
	manager.SetNotifyRenameCallback(func(src, dst string, isDir bool) bool {
		renameCalled = true
		return true
	})

	// Callbacks are set but not called yet
	if fetchCalled || cancelCalled || deleteCalled || renameCalled {
		t.Error("Callbacks should not be called yet")
	}
}

// Note: Tests that actually register/connect to sync roots require administrator privileges
// and may interfere with the file system, so they're skipped by default.
// Run them manually with: go test -run TestSyncRootRegister -v
func TestSyncRootRegister(t *testing.T) {
	if !IsAvailable() {
		t.Skip("Cloud Files API not available")
	}

	if os.Getenv("RUN_CLOUDFILES_INTEGRATION") != "1" {
		t.Skip("Skipping integration test. Set RUN_CLOUDFILES_INTEGRATION=1 to run")
	}

	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "AnemoneSync-Test",
	}

	manager, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("NewSyncRootManager failed: %v", err)
	}

	// Register
	if err := manager.Register(); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if !manager.IsRegistered() {
		t.Error("should be registered after Register()")
	}

	// Unregister
	if err := manager.Unregister(); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}

	if manager.IsRegistered() {
		t.Error("should not be registered after Unregister()")
	}
}
