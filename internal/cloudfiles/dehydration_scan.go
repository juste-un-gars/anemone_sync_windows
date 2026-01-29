//go:build windows
// +build windows

// Package cloudfiles provides scanning and dehydration operations.
package cloudfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// ScanHydratedFiles scans the sync root for hydrated files.
func (dm *DehydrationManager) ScanHydratedFiles(ctx context.Context) ([]HydratedFileInfo, error) {
	var files []HydratedFileInfo
	rootPath := dm.syncRoot.Path()
	now := time.Now()

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Check context
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}

		// Check if file is hydrated (has actual content on disk)
		isHydrated, lastAccess, err := dm.getFileHydrationStatus(path)
		if err != nil || !isHydrated {
			return nil
		}

		daysSinceAccess := int(now.Sub(lastAccess).Hours() / 24)

		files = append(files, HydratedFileInfo{
			Path:            relPath,
			FullPath:        path,
			Size:            info.Size(),
			LastAccessTime:  lastAccess,
			ModTime:         info.ModTime(),
			DaysSinceAccess: daysSinceAccess,
		})

		return nil
	})

	return files, err
}

// getFileHydrationStatus checks if a file is hydrated and gets its last access time.
func (dm *DehydrationManager) getFileHydrationStatus(path string) (bool, time.Time, error) {
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(path),
		0, // Query only
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return false, time.Time{}, err
	}
	defer windows.CloseHandle(handle)

	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fileInfo); err != nil {
		return false, time.Time{}, err
	}

	// Check placeholder state
	cfState := GetPlaceholderState(fileInfo.FileAttributes, IO_REPARSE_TAG_CLOUD)

	// File is hydrated if it's a placeholder but NOT partial (i.e., has full content)
	isPlaceholder := cfState&CF_PLACEHOLDER_STATE_PLACEHOLDER != 0
	isPartial := cfState&CF_PLACEHOLDER_STATE_PARTIAL != 0

	// If it's a placeholder and not partial, it's hydrated (has content on disk)
	isHydrated := isPlaceholder && !isPartial

	// Get last access time
	lastAccess := filetimeToTime(int64(fileInfo.LastAccessTime.HighDateTime)<<32 | int64(fileInfo.LastAccessTime.LowDateTime))

	return isHydrated, lastAccess, nil
}

// filterEligibleFiles filters files based on the dehydration policy.
func (dm *DehydrationManager) filterEligibleFiles(files []HydratedFileInfo, policy DehydrationPolicy) []HydratedFileInfo {
	var eligible []HydratedFileInfo

	for _, file := range files {
		// Check age
		if policy.MaxAgeDays > 0 && file.DaysSinceAccess < policy.MaxAgeDays {
			continue
		}

		// Check size
		if file.Size < policy.MinFileSize {
			continue
		}

		// Check exclude patterns
		excluded := false
		for _, pattern := range policy.ExcludePatterns {
			matched, _ := filepath.Match(pattern, filepath.Base(file.Path))
			if matched {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		eligible = append(eligible, file)
	}

	return eligible
}

// DehydrateFile dehydrates a single file by path.
func (dm *DehydrationManager) DehydrateFile(ctx context.Context, relativePath string) error {
	fullPath := filepath.Join(dm.syncRoot.Path(), relativePath)

	dm.logger.Debug("dehydrating file",
		zap.String("path", relativePath),
	)

	fmt.Printf("[DEBUG Dehydrate] File: %s\n", fullPath)

	// First, get the file size using regular file operations
	fi, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}
	fileSize := fi.Size()

	// Open the file with CfOpenFileWithOplock for exclusive access (required for dehydration)
	// Use EXCLUSIVE + WRITE_ACCESS flags for safe dehydration
	protectedHandle, err := OpenFileWithOplock(fullPath, CF_OPEN_FILE_FLAG_EXCLUSIVE|CF_OPEN_FILE_FLAG_WRITE_ACCESS)
	if err != nil {
		return fmt.Errorf("failed to open file with oplock: %w", err)
	}
	defer CloseHandle(protectedHandle)

	// Get Win32 handle from protected handle (required for Win32 API calls)
	win32Handle, err := GetWin32HandleFromProtectedHandle(protectedHandle)
	if err != nil {
		return fmt.Errorf("failed to get Win32 handle: %w", err)
	}
	fmt.Printf("[DEBUG Dehydrate] protectedHandle=%v, win32Handle=%v\n", protectedHandle, win32Handle)

	// Check placeholder state before dehydration using GetFileInformationByHandle
	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(win32Handle, &fileInfo); err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	stateBefore := GetPlaceholderState(fileInfo.FileAttributes, IO_REPARSE_TAG_CLOUD)
	fmt.Printf("[DEBUG Dehydrate] Before: attrs=0x%08X, state=0x%08X, size=%d\n",
		fileInfo.FileAttributes, stateBefore, fileSize)

	// Check if file is already a placeholder
	isPlaceholder := stateBefore&CF_PLACEHOLDER_STATE_PLACEHOLDER != 0
	isPartial := stateBefore&CF_PLACEHOLDER_STATE_PARTIAL != 0
	fmt.Printf("[DEBUG Dehydrate] isPlaceholder=%v, isPartial=%v (partial=already dehydrated)\n",
		isPlaceholder, isPartial)

	if !isPlaceholder {
		return fmt.Errorf("file is not a placeholder")
	}
	if isPartial {
		dm.logger.Info("file already dehydrated (partial)",
			zap.String("path", relativePath),
		)
		return nil // Already dehydrated
	}

	// Use CfUpdatePlaceholder with DEHYDRATE + MARK_IN_SYNC flags
	// This is the recommended approach per Microsoft docs:
	// - MARK_IN_SYNC ensures the file is marked as synchronized
	// - DEHYDRATE removes the local content in one atomic operation
	// Using the protected handle from CfOpenFileWithOplock (exclusive access required)
	fmt.Printf("[DEBUG Dehydrate] Calling CfUpdatePlaceholder with DEHYDRATE|MARK_IN_SYNC flags\n")
	if err := UpdatePlaceholder(protectedHandle, CF_UPDATE_FLAG_DEHYDRATE|CF_UPDATE_FLAG_MARK_IN_SYNC); err != nil {
		return fmt.Errorf("failed to dehydrate via CfUpdatePlaceholder: %w", err)
	}

	// Check state after dehydration
	if err := windows.GetFileInformationByHandle(win32Handle, &fileInfo); err == nil {
		stateAfter := GetPlaceholderState(fileInfo.FileAttributes, IO_REPARSE_TAG_CLOUD)
		fmt.Printf("[DEBUG Dehydrate] After: attrs=0x%08X, state=0x%08X\n",
			fileInfo.FileAttributes, stateAfter)
	}

	dm.logger.Info("file dehydrated",
		zap.String("path", relativePath),
		zap.Int64("size", fileSize),
	)

	return nil
}

// DehydrateAll dehydrates all eligible files immediately.
func (dm *DehydrationManager) DehydrateAll(ctx context.Context) (int, int64, error) {
	dm.logger.Info("dehydrating all eligible files")

	policy := dm.GetPolicy()

	// Find hydrated files
	hydratedFiles, err := dm.ScanHydratedFiles(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to scan: %w", err)
	}

	// Filter eligible files (but ignore MaxFilesToDehydrate)
	eligible := dm.filterEligibleFiles(hydratedFiles, policy)

	count := 0
	var bytesFreed int64

	for _, file := range eligible {
		if ctx.Err() != nil {
			break
		}

		if err := dm.DehydrateFile(ctx, file.Path); err != nil {
			dm.logger.Warn("failed to dehydrate file",
				zap.String("path", file.Path),
				zap.Error(err),
			)
			continue
		}

		count++
		bytesFreed += file.Size
	}

	dm.logger.Info("dehydration complete",
		zap.Int("files", count),
		zap.Int64("bytes_freed", bytesFreed),
	)

	return count, bytesFreed, nil
}

// GetSpaceUsage returns the current space usage by hydrated files.
func (dm *DehydrationManager) GetSpaceUsage(ctx context.Context) (SpaceUsage, error) {
	hydratedFiles, err := dm.ScanHydratedFiles(ctx)
	if err != nil {
		return SpaceUsage{}, err
	}

	var usage SpaceUsage
	for _, file := range hydratedFiles {
		usage.HydratedFiles++
		usage.HydratedBytes += file.Size
	}

	// TODO: Count placeholder files too
	// For now, we just return hydrated file count

	return usage, nil
}

// SpaceUsage represents disk space usage.
type SpaceUsage struct {
	HydratedFiles    int64
	HydratedBytes    int64
	PlaceholderFiles int64
	TotalFiles       int64
}

// FormatBytes formats bytes as human-readable string.
func FormatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
