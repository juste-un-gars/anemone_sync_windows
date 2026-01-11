package scanner

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool_BasicOperation(t *testing.T) {
	h := NewTestHelpers(t)

	// Simple processor that doubles the input
	processFunc := func(job interface{}) (interface{}, error) {
		n := job.(int)
		return n * 2, nil
	}

	pool := NewWorkerPool(4, 10, processFunc, h.GetTestLogger(false))
	err := pool.Start()
	h.AssertNoError(err, "start pool")

	// Submit 10 jobs
	for i := 0; i < 10; i++ {
		err := pool.Submit(i)
		h.AssertNoError(err, "submit job %d", i)
	}

	// Close job queue to signal completion
	go func() {
		pool.Close()
	}()

	// Collect results
	results := make(map[int]int)
	for result := range pool.Results() {
		h.AssertNoError(result.Err, "process job")
		input := result.Job.(int)
		output := result.Result.(int)
		results[input] = output
	}

	// Verify all jobs processed
	h.AssertEqual(10, len(results), "result count")

	// Verify correctness
	for i := 0; i < 10; i++ {
		expected := i * 2
		if results[i] != expected {
			t.Errorf("job %d: expected %d, got %d", i, expected, results[i])
		}
	}
}

func TestWorkerPool_ConcurrentProcessing(t *testing.T) {
	h := NewTestHelpers(t)

	var processed atomic.Int32

	processFunc := func(job interface{}) (interface{}, error) {
		// Simulate some work
		time.Sleep(1 * time.Millisecond)
		processed.Add(1)
		return job, nil
	}

	pool := NewWorkerPool(4, 20, processFunc, h.GetTestLogger(false))
	err := pool.Start()
	h.AssertNoError(err, "start pool")

	// Submit 100 jobs in goroutine
	jobCount := 100
	go func() {
		for i := 0; i < jobCount; i++ {
			err := pool.Submit(i)
			if err != nil {
				t.Errorf("submit job %d: %v", i, err)
			}
		}
		// Close job queue after all jobs submitted
		pool.Close()
	}()

	// Wait for all results
	resultCount := 0
	for range pool.Results() {
		resultCount++
	}

	h.AssertEqual(jobCount, resultCount, "all jobs should be processed")
	h.AssertEqual(int32(jobCount), processed.Load(), "processed count")
}

func TestWorkerPool_ErrorHandling(t *testing.T) {
	h := NewTestHelpers(t)

	// Processor that errors on even numbers
	processFunc := func(job interface{}) (interface{}, error) {
		n := job.(int)
		if n%2 == 0 {
			return nil, fmt.Errorf("error on even number: %d", n)
		}
		return n, nil
	}

	pool := NewWorkerPool(4, 10, processFunc, h.GetTestLogger(false))
	err := pool.Start()
	h.AssertNoError(err, "start pool")

	// Submit 10 jobs
	for i := 0; i < 10; i++ {
		pool.Submit(i)
	}

	// Close job queue in goroutine
	go func() {
		pool.Close()
	}()

	// Collect results and count errors
	successCount := 0
	errorCount := 0
	for result := range pool.Results() {
		if result.Err != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	// Should have 5 successes (odd numbers) and 5 errors (even numbers)
	h.AssertEqual(5, successCount, "success count")
	h.AssertEqual(5, errorCount, "error count")
}

func TestWorkerPool_Cancel(t *testing.T) {
	h := NewTestHelpers(t)

	var started atomic.Int32
	var completed atomic.Int32

	processFunc := func(job interface{}) (interface{}, error) {
		started.Add(1)
		time.Sleep(100 * time.Millisecond) // Longer processing
		completed.Add(1)
		return job, nil
	}

	pool := NewWorkerPool(2, 10, processFunc, h.GetTestLogger(false))
	err := pool.Start()
	h.AssertNoError(err, "start pool")

	// Submit jobs
	for i := 0; i < 10; i++ {
		pool.Submit(i)
	}

	// Wait a bit for some jobs to start
	time.Sleep(50 * time.Millisecond)

	// Cancel the pool
	pool.Cancel()

	// Close job queue in goroutine
	go func() {
		pool.Close()
	}()

	// Drain results
	for range pool.Results() {
	}

	// Some jobs should have started but not all completed
	if started.Load() == 0 {
		t.Error("no jobs started")
	}

	// Not all jobs should have completed (due to cancel)
	t.Logf("Started: %d, Completed: %d", started.Load(), completed.Load())
}

func TestWorkerPool_DoubleStart(t *testing.T) {
	h := NewTestHelpers(t)

	processFunc := func(job interface{}) (interface{}, error) {
		return job, nil
	}

	pool := NewWorkerPool(4, 10, processFunc, h.GetTestLogger(false))

	err := pool.Start()
	h.AssertNoError(err, "first start")

	// Second start should error
	err = pool.Start()
	h.AssertError(err, "second start should error")

	pool.Close()
}

func TestWorkerPool_SubmitBeforeStart(t *testing.T) {
	h := NewTestHelpers(t)

	processFunc := func(job interface{}) (interface{}, error) {
		return job, nil
	}

	pool := NewWorkerPool(4, 10, processFunc, h.GetTestLogger(false))

	// Try to submit before starting
	err := pool.Submit(1)
	h.AssertError(err, "submit before start should error")

	pool.Close()
}

func TestWorkerPool_WorkerCount(t *testing.T) {
	h := NewTestHelpers(t)

	processFunc := func(job interface{}) (interface{}, error) {
		return job, nil
	}

	// Test different worker counts
	workerCounts := []int{1, 2, 4, 8, 16}

	for _, count := range workerCounts {
		pool := NewWorkerPool(count, 10, processFunc, h.GetTestLogger(false))
		h.AssertEqual(count, pool.WorkerCount(), "worker count")
		pool.Close()
	}
}

func TestWorkerPool_IsStarted(t *testing.T) {
	h := NewTestHelpers(t)

	processFunc := func(job interface{}) (interface{}, error) {
		return job, nil
	}

	pool := NewWorkerPool(4, 10, processFunc, h.GetTestLogger(false))

	if pool.IsStarted() {
		t.Error("pool should not be started initially")
	}

	err := pool.Start()
	h.AssertNoError(err, "start pool")

	if !pool.IsStarted() {
		t.Error("pool should be started after Start()")
	}

	pool.Close()
}

func TestWorkerPool_SubmitBatch(t *testing.T) {
	h := NewTestHelpers(t)

	processFunc := func(job interface{}) (interface{}, error) {
		return job, nil
	}

	pool := NewWorkerPool(4, 100, processFunc, h.GetTestLogger(false))
	err := pool.Start()
	h.AssertNoError(err, "start pool")

	// Submit batch
	jobs := make([]interface{}, 50)
	for i := 0; i < 50; i++ {
		jobs[i] = i
	}

	err = pool.SubmitBatch(jobs)
	h.AssertNoError(err, "submit batch")

	// Close job queue in goroutine
	go func() {
		pool.Close()
	}()

	// Count results
	resultCount := 0
	for range pool.Results() {
		resultCount++
	}

	h.AssertEqual(50, resultCount, "all batch jobs should be processed")
}

func TestWorkerPool_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	h := NewTestHelpers(t)

	var processed atomic.Int32

	processFunc := func(job interface{}) (interface{}, error) {
		// Simulate variable work
		n := job.(int)
		if n%10 == 0 {
			time.Sleep(1 * time.Millisecond)
		}
		processed.Add(1)
		return n, nil
	}

	pool := NewWorkerPool(8, 1000, processFunc, h.GetTestLogger(false))
	err := pool.Start()
	h.AssertNoError(err, "start pool")

	// Submit 10000 jobs
	jobCount := 10000
	go func() {
		for i := 0; i < jobCount; i++ {
			pool.Submit(i)
		}
		pool.Close()
	}()

	// Collect results
	resultCount := 0
	for range pool.Results() {
		resultCount++
	}

	h.AssertEqual(jobCount, resultCount, "all stress test jobs processed")
}

// --- Benchmarks ---

func BenchmarkWorkerPool_Throughput(b *testing.B) {
	processFunc := func(job interface{}) (interface{}, error) {
		// Minimal work
		return job, nil
	}

	pool := NewWorkerPool(4, 1000, processFunc, nil)
	pool.Start()
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(i)
	}

	// Wait for completion
	pool.Close()
	for range pool.Results() {
	}
}

func BenchmarkWorkerPool_DifferentWorkerCounts(b *testing.B) {
	processFunc := func(job interface{}) (interface{}, error) {
		time.Sleep(100 * time.Microsecond)
		return job, nil
	}

	workerCounts := []int{1, 2, 4, 8, 16}

	for _, count := range workerCounts {
		b.Run(fmt.Sprintf("Workers%d", count), func(b *testing.B) {
			pool := NewWorkerPool(count, 100, processFunc, nil)
			pool.Start()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				pool.Submit(i)
			}

			pool.Close()
			for range pool.Results() {
			}
		})
	}
}

func BenchmarkWorkerPool_ConcurrentSubmit(b *testing.B) {
	processFunc := func(job interface{}) (interface{}, error) {
		return job, nil
	}

	pool := NewWorkerPool(4, 1000, processFunc, nil)
	pool.Start()
	defer pool.Close()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			pool.Submit(i)
			i++
		}
	})

	pool.Close()
	for range pool.Results() {
	}
}

func BenchmarkWorkerPool_WithErrors(b *testing.B) {
	processFunc := func(job interface{}) (interface{}, error) {
		n := job.(int)
		if n%10 == 0 {
			return nil, fmt.Errorf("error on job %d", n)
		}
		return n, nil
	}

	pool := NewWorkerPool(4, 1000, processFunc, nil)
	pool.Start()
	defer pool.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pool.Submit(i)
	}

	pool.Close()
	for range pool.Results() {
	}
}
