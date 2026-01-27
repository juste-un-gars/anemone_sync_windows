//go:build windows
// +build windows

package cloudfiles

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultDehydrationPolicy(t *testing.T) {
	policy := DefaultDehydrationPolicy()

	if policy.Enabled {
		t.Error("Default policy should be disabled")
	}
	if policy.MaxAgeDays != 30 {
		t.Errorf("Expected MaxAgeDays 30, got %d", policy.MaxAgeDays)
	}
	if policy.MinFileSize != 1024*1024 {
		t.Errorf("Expected MinFileSize 1MB, got %d", policy.MinFileSize)
	}
	if policy.MaxFilesToDehydrate != 100 {
		t.Errorf("Expected MaxFilesToDehydrate 100, got %d", policy.MaxFilesToDehydrate)
	}
	if policy.ScanInterval != time.Hour {
		t.Errorf("Expected ScanInterval 1h, got %v", policy.ScanInterval)
	}
}

func TestNewDehydrationManager(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("Failed to create sync root manager: %v", err)
	}

	policy := DefaultDehydrationPolicy()
	dm := NewDehydrationManager(syncRoot, policy, nil)

	if dm == nil {
		t.Fatal("DehydrationManager should not be nil")
	}

	if dm.IsRunning() {
		t.Error("Manager should not be running initially")
	}
}

func TestDehydrationManagerSetPolicy(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)

	policy := DefaultDehydrationPolicy()
	dm := NewDehydrationManager(syncRoot, policy, nil)

	// Update policy
	newPolicy := DehydrationPolicy{
		Enabled:     true,
		MaxAgeDays:  7,
		MinFileSize: 512 * 1024,
	}
	dm.SetPolicy(newPolicy)

	got := dm.GetPolicy()
	if !got.Enabled {
		t.Error("Policy should be enabled")
	}
	if got.MaxAgeDays != 7 {
		t.Errorf("Expected MaxAgeDays 7, got %d", got.MaxAgeDays)
	}
	if got.MinFileSize != 512*1024 {
		t.Errorf("Expected MinFileSize 512KB, got %d", got.MinFileSize)
	}
}

func TestDehydrationManagerStartStop(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)

	policy := DefaultDehydrationPolicy()
	policy.ScanInterval = 100 * time.Millisecond
	dm := NewDehydrationManager(syncRoot, policy, nil)

	ctx := context.Background()

	// Start
	if err := dm.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !dm.IsRunning() {
		t.Error("Manager should be running after Start")
	}

	// Starting again should fail
	if err := dm.Start(ctx); err == nil {
		t.Error("Starting twice should return error")
	}

	// Stop
	dm.Stop()

	if dm.IsRunning() {
		t.Error("Manager should not be running after Stop")
	}
}

func TestDehydrationManagerGetStats(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)

	policy := DefaultDehydrationPolicy()
	dm := NewDehydrationManager(syncRoot, policy, nil)

	stats := dm.GetStats()

	if stats.FilesScanned != 0 {
		t.Errorf("Expected 0 files scanned, got %d", stats.FilesScanned)
	}
	if stats.FilesDehydrated != 0 {
		t.Errorf("Expected 0 files dehydrated, got %d", stats.FilesDehydrated)
	}
	if stats.BytesFreed != 0 {
		t.Errorf("Expected 0 bytes freed, got %d", stats.BytesFreed)
	}
}

func TestDehydrationManagerScanHydratedFiles(t *testing.T) {
	tempDir := t.TempDir()

	// Create some test files
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)

	policy := DefaultDehydrationPolicy()
	dm := NewDehydrationManager(syncRoot, policy, nil)

	ctx := context.Background()

	// Scan - note: files won't be detected as hydrated placeholders
	// because they're not actual Cloud Files placeholders
	files, err := dm.ScanHydratedFiles(ctx)
	if err != nil {
		t.Fatalf("ScanHydratedFiles failed: %v", err)
	}

	// Regular files won't show as hydrated (no placeholder attributes)
	t.Logf("Found %d hydrated files", len(files))
}

func TestFilterEligibleFiles(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)

	policy := DehydrationPolicy{
		MaxAgeDays:      7,
		MinFileSize:     1024,
		ExcludePatterns: []string{"*.log"},
	}
	dm := NewDehydrationManager(syncRoot, policy, nil)

	now := time.Now()
	files := []HydratedFileInfo{
		{Path: "recent.txt", Size: 2048, DaysSinceAccess: 3},     // Too recent
		{Path: "small.txt", Size: 512, DaysSinceAccess: 10},       // Too small
		{Path: "old.txt", Size: 2048, DaysSinceAccess: 10},        // Eligible
		{Path: "excluded.log", Size: 2048, DaysSinceAccess: 10},   // Excluded pattern
		{Path: "another.txt", Size: 4096, DaysSinceAccess: 30},    // Eligible
	}

	// Set last access times
	for i := range files {
		files[i].LastAccessTime = now.AddDate(0, 0, -files[i].DaysSinceAccess)
	}

	eligible := dm.filterEligibleFiles(files, policy)

	if len(eligible) != 2 {
		t.Errorf("Expected 2 eligible files, got %d", len(eligible))
	}

	// Check which files are eligible
	eligiblePaths := make(map[string]bool)
	for _, f := range eligible {
		eligiblePaths[f.Path] = true
	}

	if !eligiblePaths["old.txt"] {
		t.Error("old.txt should be eligible")
	}
	if !eligiblePaths["another.txt"] {
		t.Error("another.txt should be eligible")
	}
	if eligiblePaths["recent.txt"] {
		t.Error("recent.txt should not be eligible (too recent)")
	}
	if eligiblePaths["small.txt"] {
		t.Error("small.txt should not be eligible (too small)")
	}
	if eligiblePaths["excluded.log"] {
		t.Error("excluded.log should not be eligible (excluded pattern)")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{500, "500 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1610612736, "1.50 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, got, tt.expected)
		}
	}
}

func TestHydratedFileInfo(t *testing.T) {
	info := HydratedFileInfo{
		Path:            "test/file.txt",
		FullPath:        "C:\\Users\\test\\file.txt",
		Size:            1024,
		LastAccessTime:  time.Now().AddDate(0, 0, -10),
		ModTime:         time.Now().AddDate(0, 0, -20),
		DaysSinceAccess: 10,
	}

	if info.Path != "test/file.txt" {
		t.Errorf("Path mismatch: got %s", info.Path)
	}
	if info.Size != 1024 {
		t.Errorf("Size mismatch: got %d", info.Size)
	}
	if info.DaysSinceAccess != 10 {
		t.Errorf("DaysSinceAccess mismatch: got %d", info.DaysSinceAccess)
	}
}

func TestSpaceUsage(t *testing.T) {
	usage := SpaceUsage{
		HydratedFiles:    100,
		HydratedBytes:    1024 * 1024 * 500, // 500MB
		PlaceholderFiles: 50,
		TotalFiles:       150,
	}

	if usage.HydratedFiles != 100 {
		t.Errorf("HydratedFiles mismatch: got %d", usage.HydratedFiles)
	}
	if usage.HydratedBytes != 500*1024*1024 {
		t.Errorf("HydratedBytes mismatch: got %d", usage.HydratedBytes)
	}
}

func TestDehydrationManagerContextCancellation(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path:         tempDir,
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)

	policy := DefaultDehydrationPolicy()
	policy.ScanInterval = time.Hour // Long interval
	dm := NewDehydrationManager(syncRoot, policy, nil)

	ctx, cancel := context.WithCancel(context.Background())

	// Start
	if err := dm.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Cancel context
	cancel()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Should still report running (manager tracks its own state)
	// but the scan loop will have exited
	dm.Stop()

	if dm.IsRunning() {
		t.Error("Manager should not be running after Stop")
	}
}

func TestCloudFilesProviderDehydration(t *testing.T) {
	tempDir := t.TempDir()

	config := ProviderConfig{
		LocalPath:    tempDir,
		ProviderName: "TestProvider",
	}

	provider, err := NewCloudFilesProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Get default policy
	policy := provider.GetDehydrationPolicy()
	if policy.Enabled {
		t.Error("Default policy should be disabled")
	}

	// Set new policy
	newPolicy := DehydrationPolicy{
		Enabled:    true,
		MaxAgeDays: 14,
	}
	provider.SetDehydrationPolicy(newPolicy)

	got := provider.GetDehydrationPolicy()
	if !got.Enabled {
		t.Error("Policy should be enabled after setting")
	}
	if got.MaxAgeDays != 14 {
		t.Errorf("MaxAgeDays should be 14, got %d", got.MaxAgeDays)
	}

	// Get stats
	stats := provider.GetDehydrationStats()
	if stats.FilesScanned != 0 {
		t.Errorf("Expected 0 files scanned, got %d", stats.FilesScanned)
	}
}
