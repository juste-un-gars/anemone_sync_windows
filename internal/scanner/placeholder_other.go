//go:build !windows

package scanner

import "os"

// isPlaceholderInfo always returns false on non-Windows platforms.
func isPlaceholderInfo(info os.FileInfo) bool {
	return false
}
