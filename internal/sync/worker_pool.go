package sync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// WorkerPool manages parallel execution of sync actions
type WorkerPool struct {
	numWorkers int
	logger     *zap.Logger
	executor   *Executor

	// Channels
	jobs    chan *SyncJob
	results chan *SyncJobResult

	// State
	mu       sync.RWMutex
	started  bool
	stopped  bool
	wg       sync.WaitGroup
	cancels  []context.CancelFunc

	// Statistics (atomic)
	jobsSubmitted  int64
	jobsCompleted  int64
	jobsSucceeded  int64
	jobsFailed     int64
	bytesProcessed int64
}

// SyncJob represents a sync job to be executed
type SyncJob struct {
	ID        int                  // Job index
	Decision  *cache.SyncDecision  // Sync decision to execute
	SMBClient *smb.SMBClient       // SMB client for operations
}

// SyncJobResult contains the result of a sync job
type SyncJobResult struct {
	JobID  int         // Job ID
	Action *SyncAction // Action result
	Error  error       // Error if any
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(numWorkers int, executor *Executor, logger *zap.Logger) *WorkerPool {
	if numWorkers <= 0 {
		numWorkers = 4 // Default to 4 workers
	}

	if logger == nil {
		logger = zap.NewNop()
	}

	if executor == nil {
		panic("executor cannot be nil")
	}

	return &WorkerPool{
		numWorkers: numWorkers,
		logger:     logger,
		executor:   executor,
		jobs:       make(chan *SyncJob, numWorkers*2), // Buffer for smoother flow
		results:    make(chan *SyncJobResult, numWorkers*2),
		cancels:    make([]context.CancelFunc, 0),
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start(ctx context.Context) error {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	if wp.started {
		return fmt.Errorf("worker pool already started")
	}

	if wp.stopped {
		return fmt.Errorf("worker pool has been stopped")
	}

	wp.started = true

	wp.logger.Info("starting worker pool", zap.Int("workers", wp.numWorkers))

	// Start workers
	for i := 0; i < wp.numWorkers; i++ {
		workerCtx, cancel := context.WithCancel(ctx)
		wp.cancels = append(wp.cancels, cancel)

		wp.wg.Add(1)
		go wp.worker(workerCtx, i)
	}

	return nil
}

// Stop stops the worker pool and waits for all workers to finish
func (wp *WorkerPool) Stop() {
	wp.mu.Lock()

	if !wp.started || wp.stopped {
		wp.mu.Unlock()
		return
	}

	wp.stopped = true

	// Close jobs channel to signal no more jobs
	close(wp.jobs)

	wp.mu.Unlock()

	// Wait for all workers to finish processing
	wp.wg.Wait()

	// Close results channel (all workers are done, no more results coming)
	close(wp.results)

	// Cancel all worker contexts (cleanup)
	for _, cancel := range wp.cancels {
		cancel()
	}

	wp.logger.Info("worker pool stopped",
		zap.Int64("jobs_submitted", atomic.LoadInt64(&wp.jobsSubmitted)),
		zap.Int64("jobs_completed", atomic.LoadInt64(&wp.jobsCompleted)),
		zap.Int64("jobs_succeeded", atomic.LoadInt64(&wp.jobsSucceeded)),
		zap.Int64("jobs_failed", atomic.LoadInt64(&wp.jobsFailed)),
		zap.Int64("bytes_processed", atomic.LoadInt64(&wp.bytesProcessed)),
	)
}

// Submit submits a job to the worker pool
// Returns false if the pool is stopped or the context is cancelled
func (wp *WorkerPool) Submit(ctx context.Context, job *SyncJob) bool {
	// Check if context is already cancelled before attempting submission
	select {
	case <-ctx.Done():
		return false
	default:
	}

	wp.mu.RLock()
	if wp.stopped {
		wp.mu.RUnlock()
		return false
	}
	wp.mu.RUnlock()

	atomic.AddInt64(&wp.jobsSubmitted, 1)

	select {
	case wp.jobs <- job:
		return true
	case <-ctx.Done():
		return false
	}
}

// Results returns the results channel for collecting job results
func (wp *WorkerPool) Results() <-chan *SyncJobResult {
	return wp.results
}

// GetStats returns current worker pool statistics
func (wp *WorkerPool) GetStats() WorkerPoolStats {
	return WorkerPoolStats{
		JobsSubmitted:  atomic.LoadInt64(&wp.jobsSubmitted),
		JobsCompleted:  atomic.LoadInt64(&wp.jobsCompleted),
		JobsSucceeded:  atomic.LoadInt64(&wp.jobsSucceeded),
		JobsFailed:     atomic.LoadInt64(&wp.jobsFailed),
		BytesProcessed: atomic.LoadInt64(&wp.bytesProcessed),
		NumWorkers:     wp.numWorkers,
	}
}

// WorkerPoolStats contains worker pool statistics
type WorkerPoolStats struct {
	JobsSubmitted  int64
	JobsCompleted  int64
	JobsSucceeded  int64
	JobsFailed     int64
	BytesProcessed int64
	NumWorkers     int
}

// worker is the main worker goroutine
func (wp *WorkerPool) worker(ctx context.Context, workerID int) {
	defer wp.wg.Done()

	wp.logger.Debug("worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-ctx.Done():
			wp.logger.Debug("worker cancelled", zap.Int("worker_id", workerID))
			return

		case job, ok := <-wp.jobs:
			if !ok {
				// Jobs channel closed, exit
				wp.logger.Debug("worker finished (jobs closed)", zap.Int("worker_id", workerID))
				return
			}

			// Process job
			result := wp.processJob(ctx, workerID, job)

			// Always try to send result (don't lose results due to context cancellation)
			wp.results <- result
		}
	}
}

// processJob processes a single job
func (wp *WorkerPool) processJob(ctx context.Context, workerID int, job *SyncJob) *SyncJobResult {
	wp.logger.Debug("processing job",
		zap.Int("worker_id", workerID),
		zap.Int("job_id", job.ID),
		zap.String("action", string(job.Decision.Action)),
		zap.String("path", job.Decision.LocalPath),
	)

	// Execute the action
	action, err := wp.executor.executeAction(ctx, job.Decision, job.SMBClient)

	// Update statistics
	atomic.AddInt64(&wp.jobsCompleted, 1)
	if err != nil {
		atomic.AddInt64(&wp.jobsFailed, 1)
	} else {
		atomic.AddInt64(&wp.jobsSucceeded, 1)
		atomic.AddInt64(&wp.bytesProcessed, action.BytesTransferred)
	}

	return &SyncJobResult{
		JobID:  job.ID,
		Action: action,
		Error:  err,
	}
}

// ExecuteParallel executes a batch of decisions in parallel using the worker pool
func ExecuteParallel(
	ctx context.Context,
	decisions []*cache.SyncDecision,
	smbClient *smb.SMBClient,
	executor *Executor,
	numWorkers int,
	progressFn ProgressCallback,
	logger *zap.Logger,
) ([]*SyncAction, error) {

	if len(decisions) == 0 {
		return []*SyncAction{}, nil
	}

	// Create worker pool
	pool := NewWorkerPool(numWorkers, executor, logger)

	// Start worker pool
	if err := pool.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start worker pool: %w", err)
	}
	defer pool.Stop()

	// Launch result collector goroutine
	actions := make([]*SyncAction, len(decisions))
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)

	go func() {
		defer collectorWg.Done()

		completed := 0
		var bytesTransferred int64

		for result := range pool.Results() {
			// Store action
			if result.JobID >= 0 && result.JobID < len(actions) {
				actions[result.JobID] = result.Action
			}

			completed++
			if result.Action != nil {
				bytesTransferred += result.Action.BytesTransferred
			}

			// Report progress
			if progressFn != nil {
				progressFn(&SyncProgress{
					Phase:            "executing",
					FilesProcessed:   completed,
					FilesTotal:       len(decisions),
					BytesTransferred: bytesTransferred,
					Percentage:       35 + float64(completed)/float64(len(decisions))*60, // 35-95%
				})
			}

			// Log errors
			if result.Error != nil {
				logger.Error("job failed",
					zap.Int("job_id", result.JobID),
					zap.Error(result.Error),
				)
			}
		}
	}()

	// Submit all jobs
	for i, decision := range decisions {
		job := &SyncJob{
			ID:        i,
			Decision:  decision,
			SMBClient: smbClient,
		}

		if !pool.Submit(ctx, job) {
			// Context cancelled or pool stopped
			pool.Stop()
			collectorWg.Wait()
			return actions, ctx.Err()
		}
	}

	// Stop pool (closes jobs channel, waits for workers, closes results channel)
	pool.Stop()

	// Wait for collector to finish
	collectorWg.Wait()

	stats := pool.GetStats()
	logger.Info("parallel execution completed",
		zap.Int("total_jobs", len(decisions)),
		zap.Int64("succeeded", stats.JobsSucceeded),
		zap.Int64("failed", stats.JobsFailed),
		zap.Int64("bytes", stats.BytesProcessed),
	)

	return actions, nil
}
