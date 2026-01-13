package sync

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestDefaultRetryPolicy(t *testing.T) {
	policy := DefaultRetryPolicy(zap.NewNop())

	if policy.MaxRetries != 3 {
		t.Errorf("expected MaxRetries=3, got %d", policy.MaxRetries)
	}
	if policy.InitialDelay != 1*time.Second {
		t.Errorf("expected InitialDelay=1s, got %v", policy.InitialDelay)
	}
	if policy.Multiplier != 2.0 {
		t.Errorf("expected Multiplier=2.0, got %f", policy.Multiplier)
	}
	if !policy.OnlyRetryableErrors {
		t.Error("expected OnlyRetryableErrors=true")
	}
}

func TestAggressiveRetryPolicy(t *testing.T) {
	policy := AggressiveRetryPolicy(zap.NewNop())

	if policy.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", policy.MaxRetries)
	}
	if policy.InitialDelay != 500*time.Millisecond {
		t.Errorf("expected InitialDelay=500ms, got %v", policy.InitialDelay)
	}
}

func TestNoRetryPolicy(t *testing.T) {
	policy := NoRetryPolicy()

	if policy.MaxRetries != 0 {
		t.Errorf("expected MaxRetries=0, got %d", policy.MaxRetries)
	}
}

func TestRetrySuccess(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:          3,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: false,
		Logger:              zap.NewNop(),
	}

	attempts := 0
	fn := func() error {
		attempts++
		return nil // Success immediately
	}

	err := policy.Retry(context.Background(), "test-op", fn)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestRetrySuccessAfterFailures(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:          3,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}

	attempts := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			// Fail with retryable error (network error)
			return syscall.ECONNREFUSED
		}
		return nil // Success on third attempt
	}

	err := policy.Retry(context.Background(), "test-op", fn)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestRetryMaxRetriesExceeded(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:          2,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}

	attempts := 0
	expectedErr := syscall.ECONNREFUSED
	fn := func() error {
		attempts++
		return expectedErr
	}

	err := policy.Retry(context.Background(), "test-op", fn)

	if err == nil {
		t.Error("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error to wrap %v, got %v", expectedErr, err)
	}

	// Should be MaxRetries + 1 (initial attempt + retries)
	expectedAttempts := policy.MaxRetries + 1
	if attempts != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, attempts)
	}
}

func TestRetryNonRetryableError(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:          3,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}

	attempts := 0
	nonRetryableErr := syscall.EACCES // Permission denied - not retryable
	fn := func() error {
		attempts++
		return nonRetryableErr
	}

	err := policy.Retry(context.Background(), "test-op", fn)

	if err == nil {
		t.Error("expected error, got nil")
	}

	// Should only attempt once (no retries for non-retryable errors)
	if attempts != 1 {
		t.Errorf("expected 1 attempt for non-retryable error, got %d", attempts)
	}
}

func TestRetryContextCancellation(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:          5,
		InitialDelay:        100 * time.Millisecond,
		MaxDelay:            1 * time.Second,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}

	ctx, cancel := context.WithCancel(context.Background())

	attempts := 0
	fn := func() error {
		attempts++
		if attempts == 2 {
			// Cancel context during second attempt
			cancel()
		}
		return syscall.ECONNREFUSED // Retryable error
	}

	err := policy.Retry(ctx, "test-op", fn)

	if err == nil {
		t.Error("expected error due to context cancellation")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}

	// Should have attempted twice before cancellation
	if attempts != 2 {
		t.Errorf("expected 2 attempts before cancellation, got %d", attempts)
	}
}

func TestCalculateDelay(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   5,
		InitialDelay: 1 * time.Second,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
		Jitter:       0, // No jitter for predictable testing
		Logger:       zap.NewNop(),
	}

	tests := []struct {
		attempt      int
		expectedMin  time.Duration
		expectedMax  time.Duration
		description  string
	}{
		{1, 1 * time.Second, 1 * time.Second, "first retry"},
		{2, 2 * time.Second, 2 * time.Second, "second retry"},
		{3, 4 * time.Second, 4 * time.Second, "third retry"},
		{4, 8 * time.Second, 8 * time.Second, "fourth retry"},
		{5, 16 * time.Second, 16 * time.Second, "fifth retry"},
		{6, 32 * time.Second, 32 * time.Second, "sixth retry"},
		{10, 60 * time.Second, 60 * time.Second, "capped at max delay"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			delay := policy.calculateDelay(tt.attempt)

			if delay < tt.expectedMin || delay > tt.expectedMax {
				t.Errorf("attempt %d: expected delay between %v and %v, got %v",
					tt.attempt, tt.expectedMin, tt.expectedMax, delay)
			}
		})
	}
}

func TestCalculateDelayWithJitter(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.5, // 50% jitter
		Logger:       zap.NewNop(),
	}

	// Calculate delay multiple times to ensure jitter varies
	delays := make([]time.Duration, 10)
	for i := 0; i < 10; i++ {
		delays[i] = policy.calculateDelay(2)
	}

	// Check that at least some delays are different (jitter is working)
	allSame := true
	for i := 1; i < len(delays); i++ {
		if delays[i] != delays[0] {
			allSame = false
			break
		}
	}

	if allSame {
		t.Error("all delays are the same, jitter not working")
	}

	// Check that delays are within expected range
	// For attempt 2: base delay is 2s, with 50% jitter, range is [1s, 2s]
	minDelay := 1 * time.Second
	maxDelay := 2 * time.Second

	for i, delay := range delays {
		if delay < minDelay || delay > maxDelay {
			t.Errorf("delay %d: %v out of range [%v, %v]", i, delay, minDelay, maxDelay)
		}
	}
}

func TestRetryWithCallback(t *testing.T) {
	policy := &RetryPolicy{
		MaxRetries:          3,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}

	attempts := 0
	callbackCalls := 0
	fn := func() error {
		attempts++
		if attempts < 3 {
			return syscall.ECONNREFUSED // Retryable
		}
		return nil // Success on third attempt
	}

	callback := func(ctx RetryContext) {
		callbackCalls++

		if ctx.Attempt < 2 {
			t.Errorf("callback should only be called on retry (attempt >= 2), got attempt %d", ctx.Attempt)
		}

		if ctx.TotalRetries != policy.MaxRetries {
			t.Errorf("expected TotalRetries=%d, got %d", policy.MaxRetries, ctx.TotalRetries)
		}

		if ctx.LastError == nil {
			t.Error("LastError should not be nil")
		}
	}

	err := policy.RetryWithCallback(context.Background(), "test-op", fn, callback)

	if err != nil {
		t.Errorf("expected success, got error: %v", err)
	}

	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}

	// Callback should be called on attempt 2 and 3 (not on first attempt)
	expectedCallbacks := 2
	if callbackCalls != expectedCallbacks {
		t.Errorf("expected %d callback calls, got %d", expectedCallbacks, callbackCalls)
	}
}

func TestIsRetryableError(t *testing.T) {
	policy := &RetryPolicy{
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}

	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "network error (retryable)",
			err:       syscall.ECONNREFUSED,
			retryable: true,
		},
		{
			name:      "permission error (not retryable)",
			err:       syscall.EACCES,
			retryable: false,
		},
		{
			name:      "generic error",
			err:       fmt.Errorf("some error"),
			retryable: false,
		},
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := policy.IsRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("expected %v, got %v", tt.retryable, result)
			}
		})
	}
}

func TestRetryShouldRespectOnlyRetryableErrors(t *testing.T) {
	// Policy that retries all errors
	policyRetryAll := &RetryPolicy{
		MaxRetries:          2,
		InitialDelay:        10 * time.Millisecond,
		MaxDelay:            100 * time.Millisecond,
		Multiplier:          2.0,
		Jitter:              0,
		OnlyRetryableErrors: false, // Retry all errors
		Logger:              zap.NewNop(),
	}

	attempts := 0
	nonRetryableErr := syscall.EACCES
	fn := func() error {
		attempts++
		return nonRetryableErr
	}

	err := policyRetryAll.Retry(context.Background(), "test-op", fn)

	if err == nil {
		t.Error("expected error, got nil")
	}

	// Should retry even non-retryable errors
	expectedAttempts := policyRetryAll.MaxRetries + 1
	if attempts != expectedAttempts {
		t.Errorf("expected %d attempts, got %d", expectedAttempts, attempts)
	}
}
