//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API.
package cloudfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// PlaceholderManager manages placeholder files within a sync root.
type PlaceholderManager struct {
	syncRoot *SyncRootManager
}

// NewPlaceholderManager creates a new placeholder manager for a sync root.
func NewPlaceholderManager(syncRoot *SyncRootManager) *PlaceholderManager {
	return &PlaceholderManager{
		syncRoot: syncRoot,
	}
}

// RemoteFileInfo represents information about a remote file.
type RemoteFileInfo struct {
	Path         string    // Relative path from sync root
	Size         int64     // File size in bytes
	ModTime      time.Time // Modification time
	IsDirectory  bool      // True if this is a directory
	Hash         string    // Optional hash for verification
	FileIdentity []byte    // Optional file identity blob
}

// CreatePlaceholders creates placeholder files for the given remote files.
// It creates any necessary parent directories as placeholder directories.
func (pm *PlaceholderManager) CreatePlaceholders(files []RemoteFileInfo) error {
	if len(files) == 0 {
		return nil
	}

	// Group files by parent directory for batch creation
	dirFiles := make(map[string][]RemoteFileInfo)
	directories := make(map[string]bool)

	for _, f := range files {
		// Normalize path
		f.Path = normalizePath(f.Path)

		if f.IsDirectory {
			directories[f.Path] = true
		} else {
			parent := filepath.Dir(f.Path)
			if parent == "." {
				parent = ""
			}
			dirFiles[parent] = append(dirFiles[parent], f)
		}

		// Collect all parent directories that need to be created
		collectParentDirs(f.Path, directories)
	}

	// Create directories first (sorted by depth)
	sortedDirs := sortDirectoriesByDepth(directories)
	for _, dir := range sortedDirs {
		if dir == "" || dir == "." {
			continue
		}
		if err := pm.ensureDirectoryPlaceholder(dir); err != nil {
			return fmt.Errorf("failed to create directory placeholder %s: %w", dir, err)
		}
	}

	// Create file placeholders by directory
	for parentDir, dirFileList := range dirFiles {
		if err := pm.createFilePlaceholders(parentDir, dirFileList); err != nil {
			return fmt.Errorf("failed to create placeholders in %s: %w", parentDir, err)
		}
	}

	return nil
}

// CreateSinglePlaceholder creates a single placeholder file or directory.
func (pm *PlaceholderManager) CreateSinglePlaceholder(file RemoteFileInfo) error {
	return pm.CreatePlaceholders([]RemoteFileInfo{file})
}

// UpdatePlaceholder updates an existing placeholder with new metadata.
func (pm *PlaceholderManager) UpdatePlaceholder(file RemoteFileInfo) error {
	fullPath := filepath.Join(pm.syncRoot.Path(), file.Path)

	// Open the file
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fullPath),
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer windows.CloseHandle(handle)

	// Update the file info
	basicInfo := FILE_BASIC_INFO{
		LastWriteTime:  timeToFiletime(file.ModTime),
		FileAttributes: windows.FILE_ATTRIBUTE_NORMAL,
	}

	if file.IsDirectory {
		basicInfo.FileAttributes = windows.FILE_ATTRIBUTE_DIRECTORY
	}

	// TODO: Call CfUpdatePlaceholder when implemented
	// For now, just update the basic file info
	_ = basicInfo

	return nil
}

// DeletePlaceholder deletes a placeholder file or directory.
func (pm *PlaceholderManager) DeletePlaceholder(relativePath string) error {
	fullPath := filepath.Join(pm.syncRoot.Path(), relativePath)
	return os.RemoveAll(fullPath)
}

// GetPlaceholderState returns the state of a file (placeholder, hydrated, etc.).
func (pm *PlaceholderManager) GetPlaceholderState(relativePath string) (PlaceholderFileState, error) {
	fullPath := filepath.Join(pm.syncRoot.Path(), relativePath)

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return PlaceholderFileState{Exists: false}, nil
		}
		return PlaceholderFileState{}, err
	}

	state := PlaceholderFileState{
		Exists:      true,
		IsDirectory: info.IsDir(),
		Size:        info.Size(),
		ModTime:     info.ModTime(),
	}

	// Get placeholder state from Windows
	// This requires FILE_ATTRIBUTE_REPARSE_POINT and IO_REPARSE_TAG_CLOUD
	fileAttr := uint32(0)
	reparseTag := uint32(0)

	// Try to get extended attributes
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fullPath),
		0, // Query only
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err == nil {
		defer windows.CloseHandle(handle)

		var fileInfo windows.ByHandleFileInformation
		if windows.GetFileInformationByHandle(handle, &fileInfo) == nil {
			fileAttr = fileInfo.FileAttributes
			// Check for reparse point
			if fileAttr&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				// Get reparse tag
				reparseTag = IO_REPARSE_TAG_CLOUD
			}
		}
	}

	cfState := GetPlaceholderState(fileAttr, reparseTag)

	state.IsPlaceholder = cfState&CF_PLACEHOLDER_STATE_PLACEHOLDER != 0
	state.IsHydrated = cfState&CF_PLACEHOLDER_STATE_PARTIAL == 0 && state.IsPlaceholder
	state.InSync = cfState&CF_PLACEHOLDER_STATE_IN_SYNC != 0

	return state, nil
}

// PlaceholderFileState represents the state of a placeholder file.
type PlaceholderFileState struct {
	Exists        bool
	IsDirectory   bool
	IsPlaceholder bool
	IsHydrated    bool
	InSync        bool
	Size          int64
	ModTime       time.Time
}

// ensureDirectoryPlaceholder creates a REAL NTFS directory (not a placeholder).
// This is critical: directories must be real NTFS directories so they remain
// accessible even when the sync provider is not running.
// Only FILES should be placeholders - directories are always real.
// This matches OneDrive's behavior: you can always navigate folders,
// but cloud-only files need the provider to hydrate.
func (pm *PlaceholderManager) ensureDirectoryPlaceholder(relativePath string) error {
	// Convert forward slashes to backslashes for Windows API
	relativePathWin := strings.ReplaceAll(relativePath, "/", "\\")
	fullPath := filepath.Join(pm.syncRoot.Path(), relativePathWin)

	// Check if directory already exists
	if _, err := os.Stat(fullPath); err == nil {
		return nil // Already exists
	}

	// Create the directory and all parent directories using standard NTFS mkdir
	// This ensures directories are ALWAYS accessible, even without the provider
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", relativePath, err)
	}

	return nil
}

// createFilePlaceholders creates file placeholders in a specific directory.
func (pm *PlaceholderManager) createFilePlaceholders(parentDir string, files []RemoteFileInfo) error {
	if len(files) == 0 {
		return nil
	}

	basePath := pm.syncRoot.Path()
	if parentDir != "" {
		// Convert forward slashes to backslashes for Windows API
		parentDirWin := strings.ReplaceAll(parentDir, "/", "\\")
		basePath = filepath.Join(basePath, parentDirWin)
	}

	// Convert to CF_PLACEHOLDER_CREATE_INFO
	placeholders := make([]CF_PLACEHOLDER_CREATE_INFO, len(files))

	// Keep file identity strings alive during the API call
	fileIdentities := make([][]uint16, len(files))

	for i, f := range files {
		// Get just the filename for the placeholder
		fileName := filepath.Base(f.Path)
		fileNamePtr, err := windows.UTF16PtrFromString(fileName)
		if err != nil {
			return fmt.Errorf("invalid filename %s: %w", fileName, err)
		}

		// FileIdentity is REQUIRED for files - use the relative path as identity
		// (this is what CloudMirror sample does)
		var fileIdentityPtr unsafe.Pointer
		var fileIdentityLen uint32

		if len(f.FileIdentity) > 0 {
			// Use provided identity
			fileIdentityPtr = unsafe.Pointer(&f.FileIdentity[0])
			fileIdentityLen = uint32(len(f.FileIdentity))
		} else {
			// Use relative path as identity (include null terminator)
			fileIdentities[i], _ = windows.UTF16FromString(f.Path)
			fileIdentityPtr = unsafe.Pointer(&fileIdentities[i][0])
			fileIdentityLen = uint32(len(fileIdentities[i]) * 2) // Size in bytes
		}

		placeholders[i] = CF_PLACEHOLDER_CREATE_INFO{
			RelativeFileName:   fileNamePtr,
			FileIdentity:       fileIdentityPtr,
			FileIdentityLength: fileIdentityLen,
			FsMetadata: CF_FS_METADATA{
				FileSize: f.Size,
				BasicInfo: FILE_BASIC_INFO{
					LastWriteTime:  timeToFiletime(f.ModTime),
					CreationTime:   timeToFiletime(f.ModTime),
					LastAccessTime: timeToFiletime(f.ModTime),
					ChangeTime:     timeToFiletime(f.ModTime),
					FileAttributes: windows.FILE_ATTRIBUTE_NORMAL,
				},
			},
			Flags: CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC,
		}
	}

	// Create the placeholders
	return CreatePlaceholders(basePath, placeholders)
}

// Helpers

// normalizePath normalizes a file path (forward slashes, no leading slash).
func normalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "/")
	return path
}

// collectParentDirs collects all parent directories of a path.
func collectParentDirs(path string, dirs map[string]bool) {
	parent := filepath.Dir(path)
	for parent != "." && parent != "" && parent != "/" {
		dirs[parent] = true
		parent = filepath.Dir(parent)
	}
}

// sortDirectoriesByDepth sorts directories by depth (shallowest first).
func sortDirectoriesByDepth(dirs map[string]bool) []string {
	// Group by depth
	depthMap := make(map[int][]string)
	maxDepth := 0

	for dir := range dirs {
		depth := strings.Count(dir, "/") + strings.Count(dir, "\\")
		depthMap[depth] = append(depthMap[depth], dir)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	// Flatten in order
	result := make([]string, 0, len(dirs))
	for d := 0; d <= maxDepth; d++ {
		result = append(result, depthMap[d]...)
	}

	return result
}

// timeToFiletime converts a time.Time to Windows FILETIME (as int64).
func timeToFiletime(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	// Windows FILETIME is 100-nanosecond intervals since January 1, 1601
	// Unix time is seconds since January 1, 1970
	// Difference is 11644473600 seconds
	const unixToFiletimeDiff = 116444736000000000
	return t.UnixNano()/100 + unixToFiletimeDiff
}

// filetimeToTime converts a Windows FILETIME (as int64) to time.Time.
func filetimeToTime(ft int64) time.Time {
	if ft == 0 {
		return time.Time{}
	}
	const unixToFiletimeDiff = 116444736000000000
	nsec := (ft - unixToFiletimeDiff) * 100
	return time.Unix(0, nsec)
}

// IO_REPARSE_TAG_CLOUD is the reparse tag for cloud files.
const IO_REPARSE_TAG_CLOUD = 0x9000001A

// FromManifestFiles converts manifest files to RemoteFileInfo slice.
func FromManifestFiles(files []ManifestFileEntry) []RemoteFileInfo {
	result := make([]RemoteFileInfo, len(files))
	for i, f := range files {
		result[i] = RemoteFileInfo{
			Path:        f.Path,
			Size:        f.Size,
			ModTime:     time.Unix(f.MTime, 0),
			IsDirectory: false,
			Hash:        f.Hash,
		}
	}
	return result
}

// ManifestFileEntry represents a file entry from an Anemone manifest.
// This is a simplified version to avoid circular imports.
type ManifestFileEntry struct {
	Path  string
	Size  int64
	MTime int64
	Hash  string
}

// FromCacheFileInfoMap converts a cache.FileInfo map to RemoteFileInfo slice.
func FromCacheFileInfoMap(files map[string]CacheFileInfo) []RemoteFileInfo {
	result := make([]RemoteFileInfo, 0, len(files))
	for _, f := range files {
		result = append(result, RemoteFileInfo{
			Path:        f.Path,
			Size:        f.Size,
			ModTime:     f.MTime,
			IsDirectory: false,
			Hash:        f.Hash,
		})
	}
	return result
}

// CacheFileInfo represents file info from the cache.
// This is a simplified version to avoid circular imports.
type CacheFileInfo struct {
	Path  string
	Size  int64
	MTime time.Time
	Hash  string
}
