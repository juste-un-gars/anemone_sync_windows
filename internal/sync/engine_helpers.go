package sync

import (
	"fmt"
	"path/filepath"
	"strings"
)

// parseUNCPath parses a UNC path into server, share, and relative path components.
// Format: \\server\share\path or //server/share/path
// Returns server, share, and the remaining path (empty if at share root)
func parseUNCPath(uncPath string) (server, share, relPath string) {
	path := uncPath

	// Remove leading slashes
	for len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		path = path[1:]
	}

	// Split by / or \
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' || c == '\\' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	if len(parts) >= 1 {
		server = parts[0]
	}
	if len(parts) >= 2 {
		share = parts[1]
	}
	if len(parts) >= 3 {
		// Join remaining parts with forward slash for SMB operations
		for i := 2; i < len(parts); i++ {
			if relPath != "" {
				relPath += "/"
			}
			relPath += parts[i]
		}
	}

	return server, share, relPath
}

// formatErrorSummary creates a summary string from errors
func formatErrorSummary(errors []*SyncError) string {
	if len(errors) == 0 {
		return ""
	}

	summary := fmt.Sprintf("%d error(s): ", len(errors))
	if len(errors) <= 3 {
		for i, err := range errors {
			if i > 0 {
				summary += "; "
			}
			summary += fmt.Sprintf("%s (%s)", err.FilePath, err.Operation)
		}
	} else {
		for i := 0; i < 3; i++ {
			if i > 0 {
				summary += "; "
			}
			summary += fmt.Sprintf("%s (%s)", errors[i].FilePath, errors[i].Operation)
		}
		summary += fmt.Sprintf("; and %d more", len(errors)-3)
	}

	return summary
}

// toRelativePath converts an absolute path to a relative path based on a base path.
// Returns the path with forward slashes for consistency.
func toRelativePath(absPath, basePath string) string {
	// Clean and normalize paths
	absPath = filepath.Clean(absPath)
	basePath = filepath.Clean(basePath)

	// Try to get relative path
	relPath, err := filepath.Rel(basePath, absPath)
	if err != nil {
		// If relative path fails, try string manipulation
		relPath = strings.TrimPrefix(absPath, basePath)
		relPath = strings.TrimPrefix(relPath, string(filepath.Separator))
		relPath = strings.TrimPrefix(relPath, "/")
	}

	// Convert to forward slashes for consistency
	relPath = filepath.ToSlash(relPath)

	return relPath
}
