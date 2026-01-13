package sync

import (
	"context"
	"fmt"
	"sync"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"github.com/juste-un-gars/anemone_sync_windows/internal/scanner"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// Engine is the main sync orchestrator
type Engine struct {
	db     *database.DB
	config *config.Config
	logger *zap.Logger

	// Components
	scanner  *scanner.Scanner
	cache    *cache.CacheManager
	detector *cache.ChangeDetector
	executor *Executor

	// State
	mu      sync.RWMutex
	syncing map[int64]context.CancelFunc // Maps job ID to cancel function
	closed  bool
}

// NewEngine creates a new sync engine
func NewEngine(cfg *config.Config, db *database.DB, logger *zap.Logger) (*Engine, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if db == nil {
		return nil, fmt.Errorf("database cannot be nil")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// Create scanner
	scan, err := scanner.NewScanner(cfg, db, logger.Named("scanner"))
	if err != nil {
		return nil, fmt.Errorf("failed to create scanner: %w", err)
	}

	// Create cache manager
	cacheManager := cache.NewCacheManager(db, logger.Named("cache"))

	// Create change detector
	changeDetector := cache.NewChangeDetector(cacheManager, logger.Named("detector"))

	// Create executor
	bufferSizeMB := cfg.Sync.Performance.BufferSizeMB
	executor := NewExecutor(bufferSizeMB, logger.Named("executor"))

	return &Engine{
		db:       db,
		config:   cfg,
		logger:   logger,
		scanner:  scan,
		cache:    cacheManager,
		detector: changeDetector,
		executor: executor,
		syncing:  make(map[int64]context.CancelFunc),
		closed:   false,
	}, nil
}

// Sync performs a synchronization for the given request
func (e *Engine) Sync(ctx context.Context, req *SyncRequest) (*SyncResult, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid sync request: %w", err)
	}

	// Check if engine is closed
	e.mu.RLock()
	if e.closed {
		e.mu.RUnlock()
		return nil, ErrEngineClosed
	}
	e.mu.RUnlock()

	// Check if already syncing
	e.mu.Lock()
	if _, exists := e.syncing[req.JobID]; exists {
		e.mu.Unlock()
		return nil, ErrSyncInProgress
	}

	// Create cancellable context
	syncCtx, cancel := context.WithCancel(ctx)
	e.syncing[req.JobID] = cancel
	e.mu.Unlock()

	// Ensure cleanup
	defer func() {
		e.mu.Lock()
		delete(e.syncing, req.JobID)
		e.mu.Unlock()
	}()

	// Initialize result
	result := NewSyncResult(req.JobID)

	e.logger.Info("starting sync",
		zap.Int64("job_id", req.JobID),
		zap.String("mode", string(req.Mode)),
		zap.String("local_path", req.LocalPath),
		zap.String("remote_path", req.RemotePath),
	)

	// Execute sync phases
	if err := e.executeSync(syncCtx, req, result); err != nil {
		e.logger.Error("sync failed", zap.Error(err))
		result.Status = SyncStatusFailed
		result.Finalize()
		return result, err
	}

	// Finalize result
	result.Finalize()

	e.logger.Info("sync completed",
		zap.Int64("job_id", req.JobID),
		zap.String("status", string(result.Status)),
		zap.Int("uploaded", result.FilesUploaded),
		zap.Int("downloaded", result.FilesDownloaded),
		zap.Int("deleted", result.FilesDeleted),
		zap.Int("errors", result.FilesError),
		zap.Int("conflicts", result.ConflictsFound),
		zap.Duration("duration", result.Duration),
	)

	return result, nil
}

// executeSync executes the 5 phases of synchronization
func (e *Engine) executeSync(ctx context.Context, req *SyncRequest, result *SyncResult) error {
	// Phase 1: Preparation
	e.reportProgress(req, &SyncProgress{
		Phase:      "preparation",
		Message:    "Preparing synchronization...",
		Percentage: 0,
	})

	smbClient, job, err := e.prepareSync(ctx, req)
	if err != nil {
		return fmt.Errorf("preparation failed: %w", err)
	}
	defer smbClient.Disconnect()

	// Phase 2: Scanning
	e.reportProgress(req, &SyncProgress{
		Phase:      "scanning",
		Message:    "Scanning files...",
		Percentage: 5,
	})

	localFiles, remoteFiles, cachedFiles, err := e.scanFiles(ctx, req, smbClient)
	if err != nil {
		return fmt.Errorf("scanning failed: %w", err)
	}

	result.TotalFiles = len(localFiles) + len(remoteFiles)

	// Phase 3: Detection
	e.reportProgress(req, &SyncProgress{
		Phase:         "detecting",
		Message:       "Detecting changes...",
		Percentage:    25,
		FilesTotal:    result.TotalFiles,
		FilesProcessed: 0,
	})

	decisions, conflicts, err := e.detectChanges(ctx, req, localFiles, remoteFiles, cachedFiles)
	if err != nil {
		return fmt.Errorf("detection failed: %w", err)
	}

	// Add conflicts to result
	for _, conflict := range conflicts {
		result.AddConflict(conflict)
	}

	e.logger.Info("change detection completed",
		zap.Int("actions", len(decisions)),
		zap.Int("conflicts", len(conflicts)),
	)

	// Phase 4: Execution
	if len(decisions) > 0 && !req.DryRun {
		e.reportProgress(req, &SyncProgress{
			Phase:         "executing",
			Message:       "Executing sync actions...",
			Percentage:    35,
			FilesTotal:    len(decisions),
			FilesProcessed: 0,
		})

		actions, err := e.executeActions(ctx, req, decisions, smbClient, job)
		if err != nil {
			return fmt.Errorf("execution failed: %w", err)
		}

		// Add actions to result
		for _, action := range actions {
			result.AddAction(action)
			if action.Error != nil {
				syncErr := NewSyncError(action.FilePath, string(action.Action), action.Error, 1)
				result.AddError(syncErr)
			}
		}
	} else if req.DryRun {
		e.logger.Info("dry run mode - skipping execution",
			zap.Int("actions", len(decisions)),
		)
	}

	// Phase 5: Finalization
	e.reportProgress(req, &SyncProgress{
		Phase:      "finalizing",
		Message:    "Finalizing sync...",
		Percentage: 95,
	})

	if err := e.finalizeSync(ctx, req, result, job); err != nil {
		e.logger.Error("finalization failed", zap.Error(err))
		// Don't return error, sync already completed
	}

	return nil
}

// prepareSync handles Phase 1: Preparation
func (e *Engine) prepareSync(ctx context.Context, req *SyncRequest) (*smb.SMBClient, *database.SyncJob, error) {
	// Load job from database
	job, err := e.db.GetSyncJob(req.JobID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load job: %w", err)
	}

	if job == nil {
		return nil, nil, fmt.Errorf("job %d not found", req.JobID)
	}

	// Parse server credential ID to get server and share
	// Expected format: "server:share" (from SMB client credential manager)
	server, share := parseCredentialID(job.ServerCredentialID)

	// Create SMB client from keyring
	smbClient, err := smb.NewSMBClientFromKeyring(server, share, e.logger.Named("smb"))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create SMB client: %w", err)
	}

	// Connect to SMB server
	if err := smbClient.Connect(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	// Update job status to syncing
	if err := e.db.UpdateJobStatus(req.JobID, "syncing"); err != nil {
		smbClient.Disconnect()
		return nil, nil, fmt.Errorf("failed to update job status: %w", err)
	}

	e.logger.Info("preparation completed",
		zap.String("server", server),
		zap.String("share", share),
	)

	return smbClient, job, nil
}

// scanFiles handles Phase 2: Scanning
func (e *Engine) scanFiles(ctx context.Context, req *SyncRequest, smbClient *smb.SMBClient) (
	localFiles map[string]*cache.FileInfo,
	remoteFiles map[string]*cache.FileInfo,
	cachedFiles map[string]*cache.FileInfo,
	err error,
) {
	// Scan local files
	e.logger.Info("scanning local files", zap.String("path", req.LocalPath))
	scanResult, err := e.scanner.Scan(ctx, scanner.ScanRequest{
		JobID:      req.JobID,
		BasePath:   req.LocalPath,
		RemoteBase: req.RemotePath,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("local scan failed: %w", err)
	}

	// Convert scan result to FileInfo map
	localFiles = make(map[string]*cache.FileInfo)
	for _, file := range scanResult.NewFiles {
		localFiles[file.LocalPath] = &cache.FileInfo{
			Path:  file.LocalPath,
			Size:  file.Size,
			MTime: file.MTime,
			Hash:  file.Hash,
		}
	}
	for _, file := range scanResult.ModifiedFiles {
		localFiles[file.LocalPath] = &cache.FileInfo{
			Path:  file.LocalPath,
			Size:  file.Size,
			MTime: file.MTime,
			Hash:  file.Hash,
		}
	}
	for _, file := range scanResult.UnchangedFiles {
		localFiles[file.LocalPath] = &cache.FileInfo{
			Path:  file.LocalPath,
			Size:  file.Size,
			MTime: file.MTime,
			Hash:  file.Hash,
		}
	}

	e.logger.Info("local scan completed",
		zap.Int("files", len(localFiles)),
	)

	// Scan remote files (only if mode allows downloads)
	if req.Mode.AllowsDownload() {
		e.logger.Info("scanning remote files", zap.String("path", req.RemotePath))
		remoteFiles, err = e.scanRemote(ctx, smbClient, req.RemotePath)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("remote scan failed: %w", err)
		}
		e.logger.Info("remote scan completed",
			zap.Int("files", len(remoteFiles)),
		)
	} else {
		remoteFiles = make(map[string]*cache.FileInfo)
	}

	// Load cached state
	cachedFiles, err = e.cache.GetAllCachedFiles(req.JobID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load cache: %w", err)
	}

	e.logger.Info("cache loaded",
		zap.Int("files", len(cachedFiles)),
	)

	return localFiles, remoteFiles, cachedFiles, nil
}

// scanRemote scans remote files recursively using RemoteScanner
func (e *Engine) scanRemote(ctx context.Context, smbClient *smb.SMBClient, basePath string) (map[string]*cache.FileInfo, error) {
	// Create progress callback for remote scanning
	progressCallback := func(progress RemoteScanProgress) {
		e.logger.Debug("remote scan progress",
			zap.Int("files", progress.FilesFound),
			zap.Int("dirs", progress.DirsScanned),
			zap.Int64("bytes", progress.BytesDiscovered),
			zap.String("current_dir", progress.CurrentDir),
		)
	}

	// Create remote scanner
	scanner := NewRemoteScanner(smbClient, e.logger.Named("remote_scanner"), progressCallback)

	// Perform scan
	result, err := scanner.Scan(ctx, basePath)
	if err != nil {
		return nil, fmt.Errorf("remote scan failed: %w", err)
	}

	// Log scan results
	e.logger.Info("remote scan completed",
		zap.Int("files", result.TotalFiles),
		zap.Int("dirs", result.TotalDirs),
		zap.Int64("bytes", result.TotalBytes),
		zap.Duration("duration", result.Duration),
		zap.Int("errors", len(result.Errors)),
		zap.Bool("partial_success", result.PartialSuccess),
	)

	// Warn about any errors encountered
	if len(result.Errors) > 0 {
		e.logger.Warn("remote scan encountered errors",
			zap.Int("error_count", len(result.Errors)),
		)
		for i, scanErr := range result.Errors {
			if i < 5 { // Log first 5 errors
				e.logger.Warn("remote scan error", zap.Error(scanErr))
			}
		}
		if len(result.Errors) > 5 {
			e.logger.Warn("additional errors omitted", zap.Int("count", len(result.Errors)-5))
		}
	}

	return result.Files, nil
}

// detectChanges handles Phase 3: Detection
func (e *Engine) detectChanges(ctx context.Context, req *SyncRequest,
	localFiles, remoteFiles, cachedFiles map[string]*cache.FileInfo) (
	decisions []*cache.SyncDecision,
	conflicts []*cache.SyncDecision,
	err error,
) {
	// Use change detector for 3-way merge
	allDecisions, err := e.detector.BatchDetermineSyncActions(req.JobID, localFiles, remoteFiles)
	if err != nil {
		return nil, nil, fmt.Errorf("change detection failed: %w", err)
	}

	// Separate conflicts from executable decisions
	conflicts = make([]*cache.SyncDecision, 0)
	decisions = make([]*cache.SyncDecision, 0)

	for _, decision := range allDecisions {
		if decision.NeedsResolution {
			conflicts = append(conflicts, decision)
		} else {
			decisions = append(decisions, decision)
		}
	}

	// Filter decisions based on sync mode
	decisions = e.filterDecisionsByMode(req.Mode, decisions)

	e.logger.Info("change detection completed",
		zap.Int("total_decisions", len(allDecisions)),
		zap.Int("executable", len(decisions)),
		zap.Int("conflicts", len(conflicts)),
	)

	return decisions, conflicts, nil
}

// filterDecisionsByMode filters decisions based on sync mode
func (e *Engine) filterDecisionsByMode(mode SyncMode, decisions []*cache.SyncDecision) []*cache.SyncDecision {
	filtered := make([]*cache.SyncDecision, 0, len(decisions))

	for _, decision := range decisions {
		include := false

		switch decision.Action {
		case cache.ActionUpload:
			include = mode.AllowsUpload()
		case cache.ActionDownload:
			include = mode.AllowsDownload()
		case cache.ActionDeleteLocal:
			include = mode.AllowsDownload() // Only delete local if we can sync from remote
		case cache.ActionDeleteRemote:
			include = mode.AllowsUpload() // Only delete remote if we can sync to remote
		default:
			include = false
		}

		if include {
			filtered = append(filtered, decision)
		}
	}

	return filtered
}

// executeActions handles Phase 4: Execution
func (e *Engine) executeActions(ctx context.Context, req *SyncRequest,
	decisions []*cache.SyncDecision, smbClient *smb.SMBClient, job *database.SyncJob) ([]*SyncAction, error) {

	// Create progress callback
	progressFn := func(progress *SyncProgress) {
		e.reportProgress(req, progress)
	}

	// Execute using executor
	actions, err := e.executor.Execute(ctx, decisions, smbClient, progressFn)
	if err != nil {
		return nil, fmt.Errorf("execution failed: %w", err)
	}

	return actions, nil
}

// finalizeSync handles Phase 5: Finalization
func (e *Engine) finalizeSync(ctx context.Context, req *SyncRequest, result *SyncResult, job *database.SyncJob) error {
	// Update cache for successful actions
	if !req.DryRun {
		if err := e.updateCacheFromActions(req.JobID, result.Actions); err != nil {
			return fmt.Errorf("failed to update cache: %w", err)
		}
	}

	// Record sync history
	history := &database.SyncHistory{
		JobID:            req.JobID,
		Timestamp:        result.StartTime,
		FilesSynced:      result.FilesUploaded + result.FilesDownloaded,
		FilesFailed:      result.FilesError,
		BytesTransferred: result.BytesTransferred,
		Duration:         int(result.Duration.Seconds()),
		Status:           string(result.Status),
		ErrorSummary:     formatErrorSummary(result.Errors),
	}

	if err := e.db.InsertSyncHistory(history); err != nil {
		return fmt.Errorf("failed to insert sync history: %w", err)
	}

	// Update job status
	var finalStatus string
	switch result.Status {
	case SyncStatusSuccess:
		finalStatus = "idle"
	case SyncStatusPartial:
		finalStatus = "idle" // Still idle but with some errors
	case SyncStatusFailed:
		finalStatus = "error"
	}

	if err := e.db.UpdateJobStatus(req.JobID, finalStatus); err != nil {
		return fmt.Errorf("failed to update job status: %w", err)
	}

	// Update job last run time
	if err := e.db.UpdateJobLastRun(req.JobID, result.EndTime); err != nil {
		return fmt.Errorf("failed to update job last run: %w", err)
	}

	e.logger.Info("finalization completed")
	return nil
}

// updateCacheFromActions updates cache based on successful actions
func (e *Engine) updateCacheFromActions(jobID int64, actions []*SyncAction) error {
	updates := make(map[string]*cache.FileInfo)
	remotePaths := make(map[string]string)

	for _, action := range actions {
		if action.Status != ActionStatusSuccess {
			continue
		}

		updates[action.FilePath] = &cache.FileInfo{
			Path:  action.FilePath,
			Size:  action.Size,
			MTime: timeNow(), // Current time after sync
			Hash:  "",        // Hash will be computed on next scan if needed
		}
		remotePaths[action.FilePath] = action.RemotePath
	}

	if len(updates) > 0 {
		return e.cache.UpdateCacheBatch(jobID, updates, remotePaths)
	}

	return nil
}

// reportProgress reports progress to callback if provided
func (e *Engine) reportProgress(req *SyncRequest, progress *SyncProgress) {
	if req.ProgressCallback != nil {
		req.ProgressCallback(progress)
	}
}

// IsSyncing returns whether a job is currently syncing
func (e *Engine) IsSyncing(jobID int64) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	_, exists := e.syncing[jobID]
	return exists
}

// CancelSync attempts to cancel a running sync
func (e *Engine) CancelSync(jobID int64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cancel, exists := e.syncing[jobID]
	if !exists {
		return ErrSyncNotFound
	}

	cancel()
	return nil
}

// Close releases all resources
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.closed {
		return nil
	}

	// Cancel all running syncs
	for jobID, cancel := range e.syncing {
		e.logger.Info("cancelling sync on close", zap.Int64("job_id", jobID))
		cancel()
	}

	e.closed = true
	return nil
}

// Helper functions

// parseCredentialID parses server and share from credential ID
// Expected format: "server:share"
func parseCredentialID(credID string) (server, share string) {
	for i := 0; i < len(credID); i++ {
		if credID[i] == ':' {
			return credID[:i], credID[i+1:]
		}
	}
	return credID, ""
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
