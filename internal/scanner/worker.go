package scanner

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// ProcessFunc is the function signature for processing jobs
type ProcessFunc func(job interface{}) (interface{}, error)

// WorkerPool manages a pool of worker goroutines for parallel processing
type WorkerPool struct {
	workerCount int
	jobQueue    chan interface{}
	resultQueue chan WorkResult
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	processFn   ProcessFunc
	logger      *zap.Logger
	started     bool
	mu          sync.Mutex
}

// WorkResult contains the result of a processed job
type WorkResult struct {
	Job    interface{} // Original job
	Result interface{} // Processing result
	Err    error       // Error if processing failed
}

// NewWorkerPool creates a new WorkerPool instance
// workerCount: number of parallel workers (typically 4)
// queueSize: size of job queue buffer (0 = unbuffered)
func NewWorkerPool(workerCount int, queueSize int, processFn ProcessFunc, logger *zap.Logger) *WorkerPool {
	if workerCount <= 0 {
		workerCount = 4 // Default to 4 workers
	}
	if queueSize < 0 {
		queueSize = 0
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		workerCount: workerCount,
		jobQueue:    make(chan interface{}, queueSize),
		resultQueue: make(chan WorkResult, workerCount*2), // Buffer results
		ctx:         ctx,
		cancel:      cancel,
		processFn:   processFn,
		logger:      logger.With(zap.String("component", "worker_pool")),
		started:     false,
	}
}

// Start starts the worker pool
// Must be called before submitting jobs
func (w *WorkerPool) Start() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.started {
		return fmt.Errorf("worker pool already started")
	}

	w.logger.Info("starting worker pool",
		zap.Int("worker_count", w.workerCount))

	// Start worker goroutines
	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go w.worker(i)
	}

	w.started = true
	return nil
}

// worker is the main worker goroutine
func (w *WorkerPool) worker(id int) {
	defer w.wg.Done()

	w.logger.Debug("worker started",
		zap.Int("worker_id", id))

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Debug("worker stopping (context canceled)",
				zap.Int("worker_id", id))
			return

		case job, ok := <-w.jobQueue:
			if !ok {
				w.logger.Debug("worker stopping (job queue closed)",
					zap.Int("worker_id", id))
				return
			}

			// Process the job
			result, err := w.processFn(job)

			// Send result
			select {
			case w.resultQueue <- WorkResult{
				Job:    job,
				Result: result,
				Err:    err,
			}:
				// Result sent successfully
			case <-w.ctx.Done():
				// Context canceled, stop
				return
			}
		}
	}
}

// Submit submits a job to the worker pool
// Returns error if pool is not started or context is canceled
func (w *WorkerPool) Submit(job interface{}) error {
	w.mu.Lock()
	started := w.started
	w.mu.Unlock()

	if !started {
		return fmt.Errorf("worker pool not started")
	}

	select {
	case w.jobQueue <- job:
		return nil
	case <-w.ctx.Done():
		return WrapError(ErrContextCanceled, "submit job")
	}
}

// SubmitBatch submits multiple jobs at once
// More efficient than multiple Submit calls
func (w *WorkerPool) SubmitBatch(jobs []interface{}) error {
	for _, job := range jobs {
		if err := w.Submit(job); err != nil {
			return err
		}
	}
	return nil
}

// Results returns the result channel
// Caller should read from this channel to collect results
func (w *WorkerPool) Results() <-chan WorkResult {
	return w.resultQueue
}

// Close closes the job queue and waits for all workers to finish
// Should be called after all jobs have been submitted
func (w *WorkerPool) Close() {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	w.logger.Info("closing worker pool")

	// Close job queue to signal workers to finish
	close(w.jobQueue)

	// Wait for all workers to finish
	w.wg.Wait()

	// Close result queue
	close(w.resultQueue)

	w.logger.Info("worker pool closed")
}

// Cancel cancels the worker pool context
// Workers will stop after finishing their current job
func (w *WorkerPool) Cancel() {
	w.logger.Info("canceling worker pool")
	w.cancel()
}

// Wait waits for all workers to finish
// Useful when you want to wait without closing the pool
func (w *WorkerPool) Wait() {
	w.wg.Wait()
}

// WorkerCount returns the number of workers in the pool
func (w *WorkerPool) WorkerCount() int {
	return w.workerCount
}

// IsStarted returns whether the pool has been started
func (w *WorkerPool) IsStarted() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.started
}
