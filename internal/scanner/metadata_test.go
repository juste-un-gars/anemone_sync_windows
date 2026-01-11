package scanner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestExtractMetadata_RegularFile(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	content := []byte("test content")
	testFile := h.CreateTestFile(filepath.Join(tempDir, "test.txt"), content)

	metadata, err := ExtractMetadata(testFile)
	h.AssertNoError(err, "extract metadata")

	h.AssertEqual(testFile, filepath.Clean(metadata.Path), "path")
	h.AssertEqual(int64(len(content)), metadata.Size, "size")
	if metadata.IsDir {
		t.Error("should not be directory")
	}
	if metadata.IsSymlink {
		t.Error("should not be symlink")
	}
	if !metadata.IsRegularFile() {
		t.Error("should be regular file")
	}
}

func TestExtractMetadata_Directory(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	testDir := filepath.Join(tempDir, "testdir")
	err := os.Mkdir(testDir, 0755)
	h.AssertNoError(err, "create directory")

	metadata, err := ExtractMetadata(testDir)
	h.AssertNoError(err, "extract metadata for directory")

	if !metadata.IsDir {
		t.Error("should be directory")
	}
	if metadata.IsRegularFile() {
		t.Error("directory should not be regular file")
	}
}

func TestExtractMetadata_FileNotFound(t *testing.T) {
	_, err := ExtractMetadata("/nonexistent/file.txt")
	if err == nil {
		t.Error("should error on nonexistent file")
	}
}

func TestExtractMetadataWithStat(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	content := []byte("test")
	testFile := h.CreateTestFile(filepath.Join(tempDir, "test.txt"), content)

	// Get FileInfo
	info, err := os.Stat(testFile)
	h.AssertNoError(err, "stat file")

	// Extract metadata using pre-existing FileInfo
	metadata := ExtractMetadataWithStat(testFile, info)

	h.AssertEqual(testFile, filepath.Clean(metadata.Path), "path")
	h.AssertEqual(int64(len(content)), metadata.Size, "size")
	if metadata.IsDir {
		t.Error("should not be directory")
	}
}

func TestSameMetadata(t *testing.T) {
	h := NewTestHelpers(t)

	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name     string
		a        *FileMetadata
		b        *FileMetadata
		expected bool
	}{
		{
			name: "identical",
			a: &FileMetadata{
				Size:  1024,
				MTime: now,
			},
			b: &FileMetadata{
				Size:  1024,
				MTime: now,
			},
			expected: true,
		},
		{
			name: "different size",
			a: &FileMetadata{
				Size:  1024,
				MTime: now,
			},
			b: &FileMetadata{
				Size:  2048,
				MTime: now,
			},
			expected: false,
		},
		{
			name: "different mtime",
			a: &FileMetadata{
				Size:  1024,
				MTime: now,
			},
			b: &FileMetadata{
				Size:  1024,
				MTime: now.Add(2 * time.Second),
			},
			expected: false,
		},
		{
			name: "mtime within same second",
			a: &FileMetadata{
				Size:  1024,
				MTime: now,
			},
			b: &FileMetadata{
				Size:  1024,
				MTime: now.Add(500 * time.Millisecond),
			},
			expected: true, // Truncated to seconds, so same
		},
		{
			name:     "nil metadata",
			a:        nil,
			b:        &FileMetadata{Size: 1024},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SameMetadata(tt.a, tt.b)
			h.AssertEqual(tt.expected, result, "SameMetadata result")
		})
	}
}

func TestMTimeDiffSeconds(t *testing.T) {
	h := NewTestHelpers(t)

	now := time.Now()

	tests := []struct {
		name     string
		a        *FileMetadata
		b        *FileMetadata
		expected int64
	}{
		{
			name: "same time",
			a: &FileMetadata{
				MTime: now,
			},
			b: &FileMetadata{
				MTime: now,
			},
			expected: 0,
		},
		{
			name: "5 seconds apart",
			a: &FileMetadata{
				MTime: now,
			},
			b: &FileMetadata{
				MTime: now.Add(5 * time.Second),
			},
			expected: 5,
		},
		{
			name: "negative difference",
			a: &FileMetadata{
				MTime: now.Add(10 * time.Second),
			},
			b: &FileMetadata{
				MTime: now,
			},
			expected: 10, // Absolute value
		},
		{
			name:     "nil metadata",
			a:        nil,
			b:        &FileMetadata{MTime: now},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MTimeDiffSeconds(tt.a, tt.b)
			h.AssertEqual(tt.expected, result, "MTimeDiffSeconds result")
		})
	}
}

func TestFileMetadata_String(t *testing.T) {
	now := time.Now()
	metadata := &FileMetadata{
		Path:  "/test/file.txt",
		Size:  1024,
		MTime: now,
		IsDir: false,
	}

	str := metadata.String()
	if str == "" {
		t.Error("String() should not be empty")
	}

	// Should contain path, size, and type
	if !contains(str, "/test/file.txt") {
		t.Error("String should contain path")
	}
	if !contains(str, "1024") {
		t.Error("String should contain size")
	}
	if !contains(str, "file") {
		t.Error("String should contain file type")
	}
}

func TestFileMetadata_IsRegularFile(t *testing.T) {
	tests := []struct {
		name     string
		metadata *FileMetadata
		expected bool
	}{
		{
			name: "regular file",
			metadata: &FileMetadata{
				IsDir:     false,
				IsSymlink: false,
			},
			expected: true,
		},
		{
			name: "directory",
			metadata: &FileMetadata{
				IsDir:     true,
				IsSymlink: false,
			},
			expected: false,
		},
		{
			name: "symlink",
			metadata: &FileMetadata{
				IsDir:     false,
				IsSymlink: true,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.metadata.IsRegularFile()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestExtractMetadata_MTimePreservation(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	testFile := h.CreateTestFile(filepath.Join(tempDir, "mtime.txt"), []byte("test"))

	// Get original mtime
	metadata1, err := ExtractMetadata(testFile)
	h.AssertNoError(err, "first extract")

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Extract again without modifying file
	metadata2, err := ExtractMetadata(testFile)
	h.AssertNoError(err, "second extract")

	// MTime should be exactly the same
	if !metadata1.MTime.Equal(metadata2.MTime) {
		t.Errorf("mtime changed without file modification: %v != %v",
			metadata1.MTime, metadata2.MTime)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAtIndex(s, substr))
}

func containsAtIndex(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
