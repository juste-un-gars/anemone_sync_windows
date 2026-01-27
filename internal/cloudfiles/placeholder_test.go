//go:build windows
// +build windows

package cloudfiles

import (
	"testing"
	"time"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"foo/bar", "foo/bar"},
		{"foo\\bar", "foo/bar"},
		{"/foo/bar", "foo/bar"},
		{"\\foo\\bar", "foo/bar"},
		{"", ""},
		{"file.txt", "file.txt"},
	}

	for _, tt := range tests {
		result := normalizePath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCollectParentDirs(t *testing.T) {
	dirs := make(map[string]bool)
	collectParentDirs("a\\b\\c\\file.txt", dirs)

	// On Windows, filepath.Dir uses backslashes
	expected := map[string]bool{
		"a":       true,
		"a\\b":    true,
		"a\\b\\c": true,
	}

	for dir := range expected {
		if !dirs[dir] {
			t.Errorf("Expected %q in dirs, but not found. Got dirs: %v", dir, dirs)
		}
	}

	if len(dirs) != len(expected) {
		t.Errorf("Expected %d directories, got %d. Dirs: %v", len(expected), len(dirs), dirs)
	}
}

func TestSortDirectoriesByDepth(t *testing.T) {
	dirs := map[string]bool{
		"a/b/c": true,
		"a":     true,
		"a/b":   true,
		"x":     true,
		"x/y":   true,
	}

	sorted := sortDirectoriesByDepth(dirs)

	// Check that depth 0 comes before depth 1, which comes before depth 2
	depth0Seen := false
	depth1Seen := false

	for _, dir := range sorted {
		depth := countDepth(dir)
		switch depth {
		case 0:
			depth0Seen = true
			if depth1Seen {
				t.Error("Depth 0 should come before depth 1")
			}
		case 1:
			depth1Seen = true
			if !depth0Seen {
				// Depth 0 should have been seen already (if there were any)
				// But a and x are depth 0
			}
		case 2:
			if !depth1Seen {
				t.Error("Depth 2 should come after depth 1")
			}
		}
	}
}

func countDepth(path string) int {
	depth := 0
	for _, c := range path {
		if c == '/' || c == '\\' {
			depth++
		}
	}
	return depth
}

func TestTimeToFiletime(t *testing.T) {
	// Test a known time
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	ft := timeToFiletime(testTime)

	if ft <= 0 {
		t.Error("FILETIME should be positive for dates after 1601")
	}

	// Convert back and verify
	roundTrip := filetimeToTime(ft)
	diff := testTime.Sub(roundTrip)
	if diff > time.Second || diff < -time.Second {
		t.Errorf("Round-trip time conversion failed: got %v, want %v (diff: %v)", roundTrip, testTime, diff)
	}
}

func TestTimeToFiletimeZero(t *testing.T) {
	ft := timeToFiletime(time.Time{})
	if ft != 0 {
		t.Errorf("Zero time should convert to 0 FILETIME, got %d", ft)
	}
}

func TestFiletimeToTimeZero(t *testing.T) {
	result := filetimeToTime(0)
	if !result.IsZero() {
		t.Errorf("Zero FILETIME should convert to zero time, got %v", result)
	}
}

func TestFromManifestFiles(t *testing.T) {
	manifest := []ManifestFileEntry{
		{Path: "file1.txt", Size: 100, MTime: 1700000000, Hash: "abc123"},
		{Path: "dir/file2.txt", Size: 200, MTime: 1700000001, Hash: "def456"},
	}

	result := FromManifestFiles(manifest)

	if len(result) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(result))
	}

	if result[0].Path != "file1.txt" {
		t.Errorf("Expected path 'file1.txt', got '%s'", result[0].Path)
	}

	if result[0].Size != 100 {
		t.Errorf("Expected size 100, got %d", result[0].Size)
	}

	if result[0].IsDirectory {
		t.Error("File should not be marked as directory")
	}
}

func TestFromCacheFileInfoMap(t *testing.T) {
	cache := map[string]CacheFileInfo{
		"file1.txt": {Path: "file1.txt", Size: 100, MTime: time.Now(), Hash: "abc123"},
		"file2.txt": {Path: "file2.txt", Size: 200, MTime: time.Now(), Hash: "def456"},
	}

	result := FromCacheFileInfoMap(cache)

	if len(result) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(result))
	}

	// Find file1 in result
	found := false
	for _, f := range result {
		if f.Path == "file1.txt" {
			found = true
			if f.Size != 100 {
				t.Errorf("Expected size 100, got %d", f.Size)
			}
		}
	}

	if !found {
		t.Error("file1.txt not found in result")
	}
}

func TestPlaceholderFileState(t *testing.T) {
	state := PlaceholderFileState{
		Exists:        true,
		IsDirectory:   false,
		IsPlaceholder: true,
		IsHydrated:    false,
		InSync:        true,
		Size:          1024,
		ModTime:       time.Now(),
	}

	if !state.Exists {
		t.Error("State.Exists should be true")
	}

	if state.IsDirectory {
		t.Error("State.IsDirectory should be false")
	}

	if !state.IsPlaceholder {
		t.Error("State.IsPlaceholder should be true")
	}
}

func TestNewPlaceholderManager(t *testing.T) {
	tempDir := t.TempDir()

	config := SyncRootConfig{
		Path: tempDir,
	}

	syncRoot, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("NewSyncRootManager failed: %v", err)
	}

	pm := NewPlaceholderManager(syncRoot)
	if pm == nil {
		t.Fatal("PlaceholderManager should not be nil")
	}

	if pm.syncRoot != syncRoot {
		t.Error("PlaceholderManager.syncRoot mismatch")
	}
}

func TestRemoteFileInfo(t *testing.T) {
	now := time.Now()

	info := RemoteFileInfo{
		Path:         "test/file.txt",
		Size:         1024,
		ModTime:      now,
		IsDirectory:  false,
		Hash:         "sha256:abc123",
		FileIdentity: []byte("test-identity"),
	}

	if info.Path != "test/file.txt" {
		t.Errorf("Path mismatch: got %s", info.Path)
	}

	if info.Size != 1024 {
		t.Errorf("Size mismatch: got %d", info.Size)
	}

	if !info.ModTime.Equal(now) {
		t.Errorf("ModTime mismatch")
	}

	if len(info.FileIdentity) != 13 {
		t.Errorf("FileIdentity length mismatch: got %d", len(info.FileIdentity))
	}
}
