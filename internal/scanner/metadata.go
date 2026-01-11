package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FileMetadata contains metadata about a file
type FileMetadata struct {
	Path      string      // Full file path
	Size      int64       // File size in bytes
	MTime     time.Time   // Modification time
	IsDir     bool        // Whether it's a directory
	IsSymlink bool        // Whether it's a symlink
	Mode      os.FileMode // File mode/permissions
}

// ExtractMetadata extracts metadata from a file path using os.Stat
// Returns ErrFileNotFound if file doesn't exist, ErrAccessDenied if permission denied
func ExtractMetadata(path string) (*FileMetadata, error) {
	// Clean the path for consistency
	cleanPath := filepath.Clean(path)

	// Get file info
	info, err := os.Lstat(cleanPath) // Use Lstat to detect symlinks
	if err != nil {
		if os.IsNotExist(err) {
			return nil, WrapError(ErrFileNotFound, "stat file %s", cleanPath)
		}
		if os.IsPermission(err) {
			return nil, WrapError(ErrAccessDenied, "stat file %s", cleanPath)
		}
		return nil, WrapError(err, "stat file %s", cleanPath)
	}

	// Extract metadata
	metadata := &FileMetadata{
		Path:      cleanPath,
		Size:      info.Size(),
		MTime:     info.ModTime(),
		IsDir:     info.IsDir(),
		IsSymlink: info.Mode()&os.ModeSymlink != 0,
		Mode:      info.Mode(),
	}

	return metadata, nil
}

// ExtractMetadataWithStat extracts metadata using pre-existing os.FileInfo
// Useful when already have FileInfo from filepath.Walk or ReadDir
func ExtractMetadataWithStat(path string, info os.FileInfo) *FileMetadata {
	return &FileMetadata{
		Path:      filepath.Clean(path),
		Size:      info.Size(),
		MTime:     info.ModTime(),
		IsDir:     info.IsDir(),
		IsSymlink: info.Mode()&os.ModeSymlink != 0,
		Mode:      info.Mode(),
	}
}

// IsRegularFile returns true if metadata represents a regular file (not dir, not symlink)
func (m *FileMetadata) IsRegularFile() bool {
	return !m.IsDir && !m.IsSymlink
}

// String returns a string representation of metadata
func (m *FileMetadata) String() string {
	fileType := "file"
	if m.IsDir {
		fileType = "directory"
	} else if m.IsSymlink {
		fileType = "symlink"
	}
	return fmt.Sprintf("%s: %s (size=%d, mtime=%s)",
		m.Path, fileType, m.Size, m.MTime.Format(time.RFC3339))
}

// SameMetadata compares two FileMetadata for equality (size + mtime)
// Used for quick change detection without hashing
func SameMetadata(a, b *FileMetadata) bool {
	if a == nil || b == nil {
		return false
	}
	// Compare size and mtime (truncate to seconds for SMB compatibility)
	return a.Size == b.Size &&
		a.MTime.Truncate(time.Second).Equal(b.MTime.Truncate(time.Second))
}

// MTimeDiffSeconds returns the absolute difference in modification time in seconds
func MTimeDiffSeconds(a, b *FileMetadata) int64 {
	if a == nil || b == nil {
		return 0
	}
	diff := a.MTime.Sub(b.MTime)
	if diff < 0 {
		diff = -diff
	}
	return int64(diff.Seconds())
}
