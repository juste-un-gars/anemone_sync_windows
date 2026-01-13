package sync

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"go.uber.org/zap"
)

func TestNewWorkerPool(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	pool := NewWorkerPool(4, executor, zap.NewNop())

	if pool == nil {
		t.Fatal("expected pool to be created")
	}

	if pool.numWorkers != 4 {
		t.Errorf("expected 4 workers, got %d", pool.numWorkers)
	}

	if pool.started {
		t.Error("pool should not be started initially")
	}
}

func TestNewWorkerPool_DefaultWorkers(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	pool := NewWorkerPool(0, executor, zap.NewNop())

	if pool.numWorkers != 4 {
		t.Errorf("expected default 4 workers, got %d", pool.numWorkers)
	}
}

func TestWorkerPoolStartStop(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	pool := NewWorkerPool(2, executor, zap.NewNop())

	ctx := context.Background()

	// Start pool
	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}

	if !pool.started {
		t.Error("pool should be marked as started")
	}

	// Try to start again (should fail)
	err = pool.Start(ctx)
	if err == nil {
		t.Error("expected error when starting already started pool")
	}

	// Stop pool
	pool.Stop()

	if !pool.stopped {
		t.Error("pool should be marked as stopped")
	}
}

func TestWorkerPoolProcessJob(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	executor.SetRetryPolicy(NoRetryPolicy()) // No retries for faster test
	pool := NewWorkerPool(2, executor, zap.NewNop())

	ctx := context.Background()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}
	defer pool.Stop()

	// Create a simple job (no real SMB operation)
	decision := &cache.SyncDecision{
		LocalPath:  "test.txt",
		RemotePath: "test.txt",
		Action:     cache.ActionNone, // No action - will succeed quickly
	}

	job := &SyncJob{
		ID:        1,
		Decision:  decision,
		SMBClient: nil,
	}

	// Submit job
	submitted := pool.Submit(ctx, job)
	if !submitted {
		t.Error("failed to submit job")
	}

	// Close jobs and wait for result
	pool.Stop()

	// Check stats
	stats := pool.GetStats()
	if stats.JobsSubmitted != 1 {
		t.Errorf("expected 1 job submitted, got %d", stats.JobsSubmitted)
	}
}

func TestWorkerPoolMultipleJobs(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	executor.SetRetryPolicy(NoRetryPolicy())
	pool := NewWorkerPool(3, executor, zap.NewNop())

	ctx := context.Background()

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}

	numJobs := 10
	results := make(map[int]*SyncJobResult)
	resultsMu := sync.Mutex{}

	// Use WaitGroup to ensure collector finishes
	var collectorWg sync.WaitGroup
	collectorWg.Add(1)

	// Collect results in goroutine
	go func() {
		defer collectorWg.Done()
		for result := range pool.Results() {
			resultsMu.Lock()
			results[result.JobID] = result
			resultsMu.Unlock()
		}
	}()

	// Submit jobs
	for i := 0; i < numJobs; i++ {
		decision := &cache.SyncDecision{
			LocalPath:  fmt.Sprintf("file%d.txt", i),
			RemotePath: fmt.Sprintf("file%d.txt", i),
			Action:     cache.ActionNone,
		}

		job := &SyncJob{
			ID:        i,
			Decision:  decision,
			SMBClient: nil,
		}

		submitted := pool.Submit(ctx, job)
		if !submitted {
			t.Errorf("failed to submit job %d", i)
		}
	}

	// Stop pool and wait for collector to finish
	pool.Stop()
	collectorWg.Wait()

	// Check results
	resultsMu.Lock()
	if len(results) != numJobs {
		t.Errorf("expected %d results, got %d", numJobs, len(results))
	}
	resultsMu.Unlock()

	// Check stats
	stats := pool.GetStats()
	if stats.JobsSubmitted != int64(numJobs) {
		t.Errorf("expected %d jobs submitted, got %d", numJobs, stats.JobsSubmitted)
	}
	if stats.JobsCompleted != int64(numJobs) {
		t.Errorf("expected %d jobs completed, got %d", numJobs, stats.JobsCompleted)
	}
}

func TestWorkerPoolContextCancellation(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	executor.SetRetryPolicy(NoRetryPolicy())
	pool := NewWorkerPool(2, executor, zap.NewNop())

	ctx, cancel := context.WithCancel(context.Background())

	err := pool.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start pool: %v", err)
	}

	// Submit a job
	decision := &cache.SyncDecision{
		LocalPath:  "test.txt",
		RemotePath: "test.txt",
		Action:     cache.ActionNone,
	}

	job := &SyncJob{
		ID:        1,
		Decision:  decision,
		SMBClient: nil,
	}

	submitted := pool.Submit(ctx, job)
	if !submitted {
		t.Error("failed to submit job")
	}

	// Cancel context
	cancel()

	// Try to submit another job (should fail)
	submitted = pool.Submit(ctx, job)
	if submitted {
		t.Error("expected job submission to fail after context cancellation")
	}

	pool.Stop()
}

func TestWorkerPoolStats(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	pool := NewWorkerPool(2, executor, zap.NewNop())

	// Initial stats
	stats := pool.GetStats()
	if stats.JobsSubmitted != 0 {
		t.Error("expected 0 jobs submitted initially")
	}
	if stats.NumWorkers != 2 {
		t.Errorf("expected 2 workers, got %d", stats.NumWorkers)
	}
}

func TestExecuteParallel(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	executor.SetRetryPolicy(NoRetryPolicy())

	// Create test decisions
	decisions := make([]*cache.SyncDecision, 10)
	for i := 0; i < 10; i++ {
		decisions[i] = &cache.SyncDecision{
			LocalPath:  fmt.Sprintf("file%d.txt", i),
			RemotePath: fmt.Sprintf("file%d.txt", i),
			Action:     cache.ActionNone,
		}
	}

	ctx := context.Background()

	// Execute in parallel
	actions, err := ExecuteParallel(
		ctx,
		decisions,
		nil, // No SMB client needed for ActionNone
		executor,
		3, // 3 workers
		nil,
		zap.NewNop(),
	)

	if err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	if len(actions) != 10 {
		t.Errorf("expected 10 actions, got %d", len(actions))
	}

	// Check all actions are present
	for i, action := range actions {
		if action == nil {
			t.Errorf("action %d is nil", i)
		}
	}
}

func TestExecuteParallelWithProgress(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	executor.SetRetryPolicy(NoRetryPolicy())

	decisions := make([]*cache.SyncDecision, 20)
	for i := 0; i < 20; i++ {
		decisions[i] = &cache.SyncDecision{
			LocalPath:  fmt.Sprintf("file%d.txt", i),
			RemotePath: fmt.Sprintf("file%d.txt", i),
			Action:     cache.ActionNone,
		}
	}

	ctx := context.Background()

	var progressCalls int32
	progressFn := func(p *SyncProgress) {
		atomic.AddInt32(&progressCalls, 1)
		if p.FilesTotal != 20 {
			t.Errorf("expected 20 total files in progress, got %d", p.FilesTotal)
		}
	}

	actions, err := ExecuteParallel(
		ctx,
		decisions,
		nil,
		executor,
		4,
		progressFn,
		zap.NewNop(),
	)

	if err != nil {
		t.Fatalf("parallel execution failed: %v", err)
	}

	if len(actions) != 20 {
		t.Errorf("expected 20 actions, got %d", len(actions))
	}

	// Progress callback should have been called
	if atomic.LoadInt32(&progressCalls) == 0 {
		t.Error("expected progress callback to be called")
	}
}

func TestExecuteParallelEmpty(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())

	decisions := []*cache.SyncDecision{}
	ctx := context.Background()

	actions, err := ExecuteParallel(
		ctx,
		decisions,
		nil,
		executor,
		2,
		nil,
		zap.NewNop(),
	)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty decisions, got %d", len(actions))
	}
}

func TestExecuteParallelCancellation(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())
	executor.SetRetryPolicy(NoRetryPolicy())

	// Create many decisions
	decisions := make([]*cache.SyncDecision, 100)
	for i := 0; i < 100; i++ {
		decisions[i] = &cache.SyncDecision{
			LocalPath:  fmt.Sprintf("file%d.txt", i),
			RemotePath: fmt.Sprintf("file%d.txt", i),
			Action:     cache.ActionNone,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	actions, err := ExecuteParallel(
		ctx,
		decisions,
		nil,
		executor,
		2,
		nil,
		zap.NewNop(),
	)

	// With ActionNone being very fast, jobs might complete before cancellation
	// This is valid behavior - check that we either get an error OR all jobs completed
	if err != nil {
		t.Logf("got expected cancellation error: %v", err)
		// Some actions might have completed before cancellation
		t.Logf("completed %d/%d actions before cancellation", len(actions), len(decisions))
	} else {
		// All jobs completed before timeout - this is also valid
		if len(actions) != len(decisions) {
			t.Errorf("no error but incomplete results: got %d/%d", len(actions), len(decisions))
		}
		t.Logf("all %d actions completed before timeout", len(actions))
	}
}

func TestSetParallelMode(t *testing.T) {
	executor := NewExecutor(4, zap.NewNop())

	// Initially sequential
	if executor.numWorkers != 0 {
		t.Errorf("expected 0 workers initially, got %d", executor.numWorkers)
	}

	// Enable parallel mode
	executor.SetParallelMode(4)
	if executor.numWorkers != 4 {
		t.Errorf("expected 4 workers after SetParallelMode, got %d", executor.numWorkers)
	}

	// Disable parallel mode
	executor.SetParallelMode(0)
	if executor.numWorkers != 0 {
		t.Errorf("expected 0 workers after disabling, got %d", executor.numWorkers)
	}

	// Negative value should be clamped to 0
	executor.SetParallelMode(-5)
	if executor.numWorkers != 0 {
		t.Errorf("expected 0 workers for negative value, got %d", executor.numWorkers)
	}
}
