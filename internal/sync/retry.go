package sync

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"go.uber.org/zap"
)

// RetryPolicy defines the retry behavior for failed operations
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts
	MaxRetries int

	// InitialDelay is the delay before the first retry
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries
	MaxDelay time.Duration

	// Multiplier is the backoff multiplier (e.g., 2.0 for exponential)
	Multiplier float64

	// Jitter adds randomness to delay (0.0 to 1.0)
	// 0.0 = no jitter, 1.0 = full jitter (delay can be 0 to calculated value)
	Jitter float64

	// OnlyRetryableErrors if true, only retries errors classified as retryable
	OnlyRetryableErrors bool

	// Logger for retry events
	Logger *zap.Logger
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy(logger *zap.Logger) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:          3,
		InitialDelay:        1 * time.Second,
		MaxDelay:            30 * time.Second,
		Multiplier:          2.0,
		Jitter:              0.3,
		OnlyRetryableErrors: true,
		Logger:              logger,
	}
}

// AggressiveRetryPolicy returns a retry policy with more attempts
func AggressiveRetryPolicy(logger *zap.Logger) *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:          5,
		InitialDelay:        500 * time.Millisecond,
		MaxDelay:            60 * time.Second,
		Multiplier:          2.0,
		Jitter:              0.2,
		OnlyRetryableErrors: true,
		Logger:              logger,
	}
}

// NoRetryPolicy returns a policy that never retries
func NoRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxRetries:          0,
		InitialDelay:        0,
		MaxDelay:            0,
		Multiplier:          1.0,
		Jitter:              0,
		OnlyRetryableErrors: true,
		Logger:              zap.NewNop(),
	}
}

// RetryableFunc is a function that can be retried
type RetryableFunc func() error

// RetryContext is information about the current retry attempt
type RetryContext struct {
	Attempt      int           // Current attempt number (1-based)
	TotalRetries int           // Total retries configured
	LastError    error         // Error from previous attempt
	Delay        time.Duration // Delay before this attempt
}

// Retry executes a function with retry logic
func (p *RetryPolicy) Retry(ctx context.Context, operation string, fn RetryableFunc) error {
	if p.Logger == nil {
		p.Logger = zap.NewNop()
	}

	attempt := 0

	for {
		attempt++

		// Execute the operation
		err := fn()
		if err == nil {
			// Success
			if attempt > 1 {
				p.Logger.Info("operation succeeded after retries",
					zap.String("operation", operation),
					zap.Int("attempts", attempt),
				)
			}
			return nil
		}

		// Check if we should retry
		if !p.shouldRetry(attempt, err) {
			if attempt > 1 {
				p.Logger.Error("operation failed after retries",
					zap.String("operation", operation),
					zap.Int("attempts", attempt),
					zap.Error(err),
				)
			}
			return fmt.Errorf("operation failed after %d attempts: %w", attempt, err)
		}

		// Calculate delay with backoff and jitter
		delay := p.calculateDelay(attempt)

		p.Logger.Warn("operation failed, retrying",
			zap.String("operation", operation),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", p.MaxRetries),
			zap.Duration("delay", delay),
			zap.Error(err),
		)

		// Wait before retry (with context cancellation support)
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry aborted: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
}

// shouldRetry determines if an operation should be retried
func (p *RetryPolicy) shouldRetry(attempt int, err error) bool {
	// Check if we've exceeded max retries
	if attempt > p.MaxRetries {
		return false
	}

	// Check if error is retryable
	if p.OnlyRetryableErrors {
		_, retryable := ClassifyError(err)
		if !retryable {
			p.Logger.Debug("error not retryable",
				zap.Int("attempt", attempt),
				zap.Error(err),
			)
			return false
		}
	}

	return true
}

// calculateDelay calculates the delay for the given attempt with exponential backoff and jitter
func (p *RetryPolicy) calculateDelay(attempt int) time.Duration {
	// Calculate exponential backoff
	// delay = InitialDelay * (Multiplier ^ (attempt - 1))
	exponent := float64(attempt - 1)
	delay := float64(p.InitialDelay) * math.Pow(p.Multiplier, exponent)

	// Cap at max delay
	if delay > float64(p.MaxDelay) {
		delay = float64(p.MaxDelay)
	}

	// Apply jitter
	if p.Jitter > 0 {
		jitterAmount := delay * p.Jitter
		// Random value between [delay - jitter, delay]
		jitterOffset := rand.Float64() * jitterAmount
		delay = delay - jitterOffset
	}

	return time.Duration(delay)
}

// RetryWithCallback executes a function with retry logic and calls a callback on each attempt
func (p *RetryPolicy) RetryWithCallback(
	ctx context.Context,
	operation string,
	fn RetryableFunc,
	callback func(RetryContext),
) error {
	if p.Logger == nil {
		p.Logger = zap.NewNop()
	}

	var lastErr error
	attempt := 0

	for {
		attempt++

		// Call callback before attempt
		if callback != nil && attempt > 1 {
			delay := p.calculateDelay(attempt)
			callback(RetryContext{
				Attempt:      attempt,
				TotalRetries: p.MaxRetries,
				LastError:    lastErr,
				Delay:        delay,
			})
		}

		// Execute the operation
		err := fn()
		if err == nil {
			// Success
			if attempt > 1 {
				p.Logger.Info("operation succeeded after retries",
					zap.String("operation", operation),
					zap.Int("attempts", attempt),
				)
			}
			return nil
		}

		lastErr = err

		// Check if we should retry
		if !p.shouldRetry(attempt, err) {
			if attempt > 1 {
				p.Logger.Error("operation failed after retries",
					zap.String("operation", operation),
					zap.Int("attempts", attempt),
					zap.Error(err),
				)
			}
			return fmt.Errorf("operation failed after %d attempts: %w", attempt, err)
		}

		// Calculate delay with backoff and jitter
		delay := p.calculateDelay(attempt)

		p.Logger.Warn("operation failed, retrying",
			zap.String("operation", operation),
			zap.Int("attempt", attempt),
			zap.Int("max_retries", p.MaxRetries),
			zap.Duration("delay", delay),
			zap.Error(err),
		)

		// Wait before retry (with context cancellation support)
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry aborted: %w", ctx.Err())
		case <-time.After(delay):
			// Continue to next attempt
		}
	}
}

// IsRetryableError returns true if the error is retryable according to this policy
func (p *RetryPolicy) IsRetryableError(err error) bool {
	if !p.OnlyRetryableErrors {
		return true
	}
	_, retryable := ClassifyError(err)
	return retryable
}
