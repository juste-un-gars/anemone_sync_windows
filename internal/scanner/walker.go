package scanner

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// WalkFunc is called for each file/directory found during walk
// Return filepath.SkipDir to skip a directory
type WalkFunc func(path string, metadata *FileMetadata) error

// Walker handles recursive directory traversal with exclusion filtering
type Walker struct {
	excluder       *Excluder
	logger         *zap.Logger
	followSymlinks bool            // Whether to follow symlinks (default: false)
	visited        map[string]bool // Track visited paths to prevent cycles
	stats          *WalkStatistics
}

// WalkStatistics contains statistics about a walk operation
type WalkStatistics struct {
	TotalFiles      int // Total files found
	TotalDirs       int // Total directories found
	ExcludedFiles   int // Files excluded by patterns
	ExcludedDirs    int // Directories excluded by patterns
	SymlinksSkipped int // Symlinks skipped
	Errors          int // Errors encountered
}

// NewWalker creates a new Walker instance
func NewWalker(excluder *Excluder, logger *zap.Logger) *Walker {
	if excluder == nil {
		excluder = NewExcluder(logger)
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Walker{
		excluder:       excluder,
		logger:         logger.With(zap.String("component", "walker")),
		followSymlinks: false,
		visited:        make(map[string]bool),
		stats:          &WalkStatistics{},
	}
}

// SetFollowSymlinks enables or disables symlink following
func (w *Walker) SetFollowSymlinks(follow bool) {
	w.followSymlinks = follow
}

// Walk recursively traverses a directory tree starting at basePath
// Applies exclusion rules and calls walkFn for each non-excluded file
func (w *Walker) Walk(jobID int64, basePath string, walkFn WalkFunc) error {
	// Clean the base path
	basePath = filepath.Clean(basePath)

	// Check if base path exists
	if _, err := os.Stat(basePath); err != nil {
		if os.IsNotExist(err) {
			return WrapError(ErrFileNotFound, "directory does not exist: %s", basePath)
		}
		return WrapError(ErrAccessDenied, "cannot access directory: %s", basePath)
	}

	w.logger.Info("starting directory walk",
		zap.Int64("job_id", jobID),
		zap.String("base_path", basePath))

	// Reset statistics and visited map for new walk
	w.stats = &WalkStatistics{}
	w.visited = make(map[string]bool)

	// Start walking
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		// Handle walk errors
		if err != nil {
			w.stats.Errors++
			w.logger.Warn("walk error",
				zap.String("path", path),
				zap.Error(err))

			// Continue walking despite errors
			if os.IsPermission(err) {
				return nil // Skip permission denied
			}
			return nil // Continue on other errors too
		}

		// Extract metadata
		metadata := ExtractMetadataWithStat(path, info)

		// Handle symlinks
		if metadata.IsSymlink {
			w.stats.SymlinksSkipped++
			if !w.followSymlinks {
				w.logger.Debug("skipping symlink",
					zap.String("path", path))
				if metadata.IsDir {
					return filepath.SkipDir
				}
				return nil
			}

			// Resolve symlink and check for cycles
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				w.logger.Warn("failed to resolve symlink",
					zap.String("path", path),
					zap.Error(err))
				return nil
			}

			// Check if we've already visited this path
			if w.visited[realPath] {
				w.logger.Debug("cycle detected, skipping symlink",
					zap.String("path", path),
					zap.String("real_path", realPath))
				if metadata.IsDir {
					return filepath.SkipDir
				}
				return nil
			}
			w.visited[realPath] = true
		}

		// Check exclusions
		result := w.excluder.ShouldExclude(jobID, path, metadata.IsDir)
		if result.Excluded {
			if metadata.IsDir {
				w.stats.ExcludedDirs++
				w.logger.Debug("excluding directory",
					zap.String("path", path),
					zap.String("level", result.Level.String()),
					zap.String("pattern", result.Pattern))
				return filepath.SkipDir // Skip entire directory
			} else {
				w.stats.ExcludedFiles++
				w.logger.Debug("excluding file",
					zap.String("path", path),
					zap.String("level", result.Level.String()),
					zap.String("pattern", result.Pattern))
				return nil // Skip this file
			}
		}

		// Update statistics
		if metadata.IsDir {
			w.stats.TotalDirs++
		} else if metadata.IsRegularFile() {
			w.stats.TotalFiles++
		}

		// Call the walk function for non-excluded entries
		// Only call for regular files (not directories, not symlinks)
		if metadata.IsRegularFile() {
			if err := walkFn(path, metadata); err != nil {
				if err == filepath.SkipDir {
					return filepath.SkipDir
				}
				// Propagate critical errors (scan aborted, context canceled)
				if errors.Is(err, ErrScanAborted) {
					return err
				}
				w.stats.Errors++
				w.logger.Warn("walk function error",
					zap.String("path", path),
					zap.Error(err))
				// Continue despite errors in walkFn
				return nil
			}
		}

		return nil
	})

	if err != nil {
		return WrapError(err, "walk directory %s", basePath)
	}

	w.logger.Info("directory walk completed",
		zap.Int64("job_id", jobID),
		zap.Int("total_files", w.stats.TotalFiles),
		zap.Int("total_dirs", w.stats.TotalDirs),
		zap.Int("excluded_files", w.stats.ExcludedFiles),
		zap.Int("excluded_dirs", w.stats.ExcludedDirs),
		zap.Int("errors", w.stats.Errors))

	return nil
}

// GetStatistics returns the statistics from the last walk
func (w *Walker) GetStatistics() *WalkStatistics {
	return w.stats
}

// String returns a string representation of walk statistics
func (s *WalkStatistics) String() string {
	return fmt.Sprintf("files=%d dirs=%d excluded_files=%d excluded_dirs=%d symlinks_skipped=%d errors=%d",
		s.TotalFiles, s.TotalDirs, s.ExcludedFiles, s.ExcludedDirs, s.SymlinksSkipped, s.Errors)
}
