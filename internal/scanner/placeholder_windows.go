//go:build windows

package scanner

import (
	"os"
	"syscall"
)

const fileAttributeRecallOnDataAccess = 0x00400000

// isPlaceholderInfo checks if a file is a Cloud Files placeholder
// that would trigger hydration (download) on content access.
func isPlaceholderInfo(info os.FileInfo) bool {
	sys, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	return sys.FileAttributes&fileAttributeRecallOnDataAccess != 0
}
