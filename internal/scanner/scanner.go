package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"go.uber.org/zap"
)

// Scanner orchestrates file scanning, hashing, and change detection
type Scanner struct {
	db       *database.DB
	logger   *zap.Logger
	config   *config.Config
	excluder *Excluder
	hasher   *Hasher
	walker   *Walker

	mu         sync.Mutex
	scanning   bool
	batchSize  int           // Number of files to batch for DB updates
	batchDelay time.Duration // Time to wait before flushing batch
}

// ScanRequest represents a scan request
type ScanRequest struct {
	JobID      int64  // Job ID from sync_jobs table
	BasePath   string // Local base path to scan
	RemoteBase string // Remote base path for mapping
}

// ScanResult contains the result of a scan operation
type ScanResult struct {
	JobID          int64
	TotalFiles     int
	ProcessedFiles int
	SkippedFiles   int
	ErrorFiles     int
	NewFiles       []*FileInfo
	ModifiedFiles  []*FileInfo
	UnchangedFiles []*FileInfo
	DeletedFiles   []*FileInfo // Files in DB but not on disk
	Errors         []*ScanError
	Duration       time.Duration
	WalkStats      *WalkStatistics
}

// FileInfo contains information about a scanned file
type FileInfo struct {
	LocalPath  string
	RemotePath string
	Size       int64
	MTime      time.Time
	Hash       string
	Status     FileStatus
}

// FileStatus indicates the change detection result
type FileStatus int

const (
	StatusNew FileStatus = iota
	StatusModified
	StatusUnchanged
	StatusError
	StatusExcluded
	StatusDeleted
)

// String returns the string representation of FileStatus
func (s FileStatus) String() string {
	switch s {
	case StatusNew:
		return "new"
	case StatusModified:
		return "modified"
	case StatusUnchanged:
		return "unchanged"
	case StatusError:
		return "error"
	case StatusExcluded:
		return "excluded"
	case StatusDeleted:
		return "deleted"
	default:
		return "unknown"
	}
}

// NewScanner creates a new Scanner instance
func NewScanner(cfg *config.Config, db *database.DB, logger *zap.Logger) (*Scanner, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	logger = logger.With(zap.String("component", "scanner"))

	// Create excluder and load default exclusions
	excluder := NewExcluder(logger)
	defaultExclusionsPath := filepath.Join(cfg.Paths.ConfigDir, "..", "configs", "default_exclusions.json")
	if err := excluder.LoadDefaultExclusions(defaultExclusionsPath); err != nil {
		logger.Warn("failed to load default exclusions",
			zap.String("path", defaultExclusionsPath),
			zap.Error(err))
		// Continue without default exclusions
	}

	// Create hasher
	hasher := NewHasher(
		cfg.Sync.Performance.HashAlgorithm,
		cfg.Sync.Performance.BufferSizeMB,
		logger,
	)

	// Create walker
	walker := NewWalker(excluder, logger)

	return &Scanner{
		db:         db,
		logger:     logger,
		config:     cfg,
		excluder:   excluder,
		hasher:     hasher,
		walker:     walker,
		scanning:   false,
		batchSize:  100,             // Batch 100 files for DB updates
		batchDelay: 5 * time.Second, // Or 5 seconds, whichever comes first
	}, nil
}

// Scan performs a complete scan of the specified directory
// Implements the 3-step change detection algorithm
func (s *Scanner) Scan(ctx context.Context, req ScanRequest) (*ScanResult, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, fmt.Errorf("scan already in progress")
	}
	s.scanning = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	start := time.Now()

	s.logger.Info("starting file scan",
		zap.Int64("job_id", req.JobID),
		zap.String("base_path", req.BasePath),
		zap.String("remote_base", req.RemoteBase))

	result := &ScanResult{
		JobID:          req.JobID,
		NewFiles:       make([]*FileInfo, 0),
		ModifiedFiles:  make([]*FileInfo, 0),
		UnchangedFiles: make([]*FileInfo, 0),
		DeletedFiles:   make([]*FileInfo, 0),
		Errors:         make([]*ScanError, 0),
	}

	// Load exclusions from database for this job
	if err := s.loadJobExclusions(req.JobID); err != nil {
		s.logger.Warn("failed to load job exclusions",
			zap.Int64("job_id", req.JobID),
			zap.Error(err))
	}

	// Track files found during scan
	foundFiles := make(map[string]bool)

	// Batch for DB updates
	var batch []*database.FileState
	batchMu := sync.Mutex{}
	lastBatchFlush := time.Now()

	// Flush batch helper
	flushBatch := func() error {
		batchMu.Lock()
		defer batchMu.Unlock()

		if len(batch) == 0 {
			return nil
		}

		if err := s.bulkUpdateFileStates(batch); err != nil {
			return WrapError(err, "flush batch")
		}

		s.logger.Debug("flushed batch",
			zap.Int("count", len(batch)))

		batch = make([]*database.FileState, 0)
		lastBatchFlush = time.Now()
		return nil
	}

	// Walk the directory tree
	err := s.walker.Walk(req.JobID, req.BasePath, func(path string, metadata *FileMetadata) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return WrapError(ErrScanAborted, "context canceled")
		default:
		}

		result.TotalFiles++
		foundFiles[path] = true

		// Process file with 3-step algorithm
		fileInfo, err := s.processFile(ctx, req, path, metadata)
		if err != nil {
			scanErr := NewScanError(path, "process", err)
			result.Errors = append(result.Errors, scanErr)
			result.ErrorFiles++
			return nil // Continue despite errors
		}

		// Categorize result
		switch fileInfo.Status {
		case StatusNew:
			result.NewFiles = append(result.NewFiles, fileInfo)
		case StatusModified:
			result.ModifiedFiles = append(result.ModifiedFiles, fileInfo)
		case StatusUnchanged:
			result.UnchangedFiles = append(result.UnchangedFiles, fileInfo)
			result.SkippedFiles++
		case StatusError:
			result.ErrorFiles++
		}

		result.ProcessedFiles++

		// Convert to FileState for batch
		fileState := &database.FileState{
			JobID:      req.JobID,
			LocalPath:  fileInfo.LocalPath,
			RemotePath: fileInfo.RemotePath,
			Size:       fileInfo.Size,
			MTime:      fileInfo.MTime,
			Hash:       fileInfo.Hash,
			SyncStatus: "idle",
		}

		batchMu.Lock()
		batch = append(batch, fileState)
		batchLen := len(batch)
		timeSinceFlush := time.Since(lastBatchFlush)
		batchMu.Unlock()

		// Flush batch if size or time threshold reached
		if batchLen >= s.batchSize || timeSinceFlush >= s.batchDelay {
			if err := flushBatch(); err != nil {
				s.logger.Error("failed to flush batch",
					zap.Error(err))
			}
		}

		// Log progress every 1000 files
		if result.ProcessedFiles%1000 == 0 {
			s.logger.Info("scan progress",
				zap.Int("processed", result.ProcessedFiles),
				zap.Int("new", len(result.NewFiles)),
				zap.Int("modified", len(result.ModifiedFiles)),
				zap.Int("unchanged", len(result.UnchangedFiles)),
				zap.Int("errors", len(result.Errors)))
		}

		return nil
	})

	if err != nil {
		s.logger.Error("walk failed", zap.Error(err))
		return result, err
	}

	// Flush any remaining batch
	if err := flushBatch(); err != nil {
		s.logger.Error("failed to flush final batch", zap.Error(err))
	}

	// Detect deleted files (in DB but not found during walk)
	deletedFiles, err := s.detectDeletedFiles(req.JobID, foundFiles)
	if err != nil {
		s.logger.Warn("failed to detect deleted files", zap.Error(err))
	} else {
		result.DeletedFiles = deletedFiles
	}

	// Get walk statistics
	result.WalkStats = s.walker.GetStatistics()
	result.Duration = time.Since(start)

	s.logger.Info("scan completed",
		zap.Int64("job_id", req.JobID),
		zap.Int("total_files", result.TotalFiles),
		zap.Int("new_files", len(result.NewFiles)),
		zap.Int("modified_files", len(result.ModifiedFiles)),
		zap.Int("unchanged_files", len(result.UnchangedFiles)),
		zap.Int("deleted_files", len(result.DeletedFiles)),
		zap.Int("errors", len(result.Errors)),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// processFile implements the 3-step change detection algorithm
func (s *Scanner) processFile(ctx context.Context, req ScanRequest, path string, metadata *FileMetadata) (*FileInfo, error) {
	// Create FileInfo
	remotePath := s.mapToRemotePath(req.BasePath, req.RemoteBase, path)
	fileInfo := &FileInfo{
		LocalPath:  path,
		RemotePath: remotePath,
		Size:       metadata.Size,
		MTime:      metadata.MTime,
	}

	// Step 1: Get existing file state from DB
	dbState, err := s.getFileState(req.JobID, path)
	if err != nil {
		// File not in DB = NEW file
		fileInfo.Status = StatusNew

		// Compute hash for new file
		hashResult, err := s.hasher.ComputeHash(path)
		if err != nil {
			return nil, WrapError(err, "compute hash for new file")
		}
		fileInfo.Hash = hashResult.Hash

		return fileInfo, nil
	}

	// Step 2: Quick comparison (size + mtime)
	dbMetadata := &FileMetadata{
		Size:  dbState.Size,
		MTime: dbState.MTime,
	}
	if SameMetadata(metadata, dbMetadata) {
		// Unchanged (same size + mtime)
		fileInfo.Status = StatusUnchanged
		fileInfo.Hash = dbState.Hash
		return fileInfo, nil
	}

	// Step 3: Size or mtime changed, compute hash to check if content changed
	hashResult, err := s.hasher.ComputeHash(path)
	if err != nil {
		return nil, WrapError(err, "compute hash for modified file")
	}
	fileInfo.Hash = hashResult.Hash

	// Compare hash
	if fileInfo.Hash == dbState.Hash {
		// Hash matches, content unchanged (only mtime/size changed)
		fileInfo.Status = StatusUnchanged
	} else {
		// Hash differs, file modified
		fileInfo.Status = StatusModified
	}

	return fileInfo, nil
}

// mapToRemotePath maps a local path to a remote SMB path
func (s *Scanner) mapToRemotePath(localBase, remoteBase, localPath string) string {
	relPath, err := filepath.Rel(localBase, localPath)
	if err != nil {
		s.logger.Warn("failed to get relative path",
			zap.String("base", localBase),
			zap.String("path", localPath),
			zap.Error(err))
		return remoteBase
	}

	// Convert to forward slashes for SMB
	relPath = filepath.ToSlash(relPath)
	return filepath.Join(remoteBase, relPath)
}

// loadJobExclusions loads job-specific and individual exclusions from database
func (s *Scanner) loadJobExclusions(jobID int64) error {
	// Load job-specific and global exclusions
	exclusions, err := s.db.GetExclusions(jobID)
	if err != nil {
		return WrapError(err, "get exclusions for job %d", jobID)
	}

	for _, excl := range exclusions {
		if excl.Type == "job" {
			if err := s.excluder.AddJobPattern(jobID, excl.PatternOrPath); err != nil {
				s.logger.Warn("failed to add job pattern",
					zap.String("pattern", excl.PatternOrPath),
					zap.Error(err))
			}
		}
		// Global patterns are already loaded from default_exclusions.json
	}

	// Load individual path exclusions
	individualPaths, err := s.db.GetIndividualExclusions(jobID)
	if err != nil {
		return WrapError(err, "get individual exclusions for job %d", jobID)
	}

	for path := range individualPaths {
		s.excluder.AddIndividualPath(jobID, path)
	}

	s.logger.Info("loaded job exclusions",
		zap.Int64("job_id", jobID),
		zap.Int("pattern_count", len(exclusions)),
		zap.Int("individual_count", len(individualPaths)))

	return nil
}

// getFileState retrieves file state from database
func (s *Scanner) getFileState(jobID int64, localPath string) (*database.FileState, error) {
	state, err := s.db.GetFileState(jobID, localPath)
	if err != nil {
		// File not in database
		return nil, err
	}
	return state, nil
}

// bulkUpdateFileStates updates multiple file states in a single transaction
func (s *Scanner) bulkUpdateFileStates(states []*database.FileState) error {
	if len(states) == 0 {
		return nil
	}

	if err := s.db.BulkUpdateFileStates(states); err != nil {
		return WrapError(err, "bulk update %d file states", len(states))
	}

	return nil
}

// detectDeletedFiles detects files that are in DB but were not found during scan
func (s *Scanner) detectDeletedFiles(jobID int64, foundFiles map[string]bool) ([]*FileInfo, error) {
	// Get all files from database for this job
	dbStates, err := s.db.GetAllFileStates(jobID)
	if err != nil {
		return nil, WrapError(err, "get all file states for job %d", jobID)
	}

	deletedFiles := make([]*FileInfo, 0)

	for _, state := range dbStates {
		if !foundFiles[state.LocalPath] {
			// File is in DB but not found on disk = deleted
			deletedFiles = append(deletedFiles, &FileInfo{
				LocalPath:  state.LocalPath,
				RemotePath: state.RemotePath,
				Size:       state.Size,
				MTime:      state.MTime,
				Hash:       state.Hash,
				Status:     StatusDeleted,
			})

			// Optional: Delete from database or mark as deleted
			// For now, we'll leave it in DB for history
			s.logger.Debug("detected deleted file",
				zap.String("path", state.LocalPath))
		}
	}

	if len(deletedFiles) > 0 {
		s.logger.Info("detected deleted files",
			zap.Int64("job_id", jobID),
			zap.Int("count", len(deletedFiles)))
	}

	return deletedFiles, nil
}

// Close closes the scanner and releases resources
func (s *Scanner) Close() error {
	s.logger.Info("closing scanner")
	return nil
}

// IsScanning returns whether a scan is currently in progress
func (s *Scanner) IsScanning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.scanning
}
