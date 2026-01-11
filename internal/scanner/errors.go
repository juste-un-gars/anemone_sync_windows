// Package scanner provides file scanning, hashing, and change detection for AnemoneSync.
package scanner

import (
	"errors"
	"fmt"
	"time"
)

// Error categories for classification
const (
	ErrorCategoryFS      = "filesystem" // File access, permissions
	ErrorCategoryHash    = "hash"       // Hash computation
	ErrorCategoryDB      = "database"   // Database operations
	ErrorCategoryExclude = "exclusion"  // Pattern matching
	ErrorCategoryWorker  = "worker"     // Worker pool operations
	ErrorCategoryUnknown = "unknown"    // Unclassified errors
)

// Common error types
var (
	// Filesystem errors
	ErrAccessDenied = errors.New("access denied")
	ErrFileNotFound = errors.New("file not found")
	ErrIsDirectory  = errors.New("is a directory")
	ErrInvalidPath  = errors.New("invalid path")

	// Hash computation errors
	ErrHashFailed = errors.New("hash computation failed")
	ErrReadFailed = errors.New("file read failed")

	// Database errors
	ErrDBUpdate = errors.New("database update failed")
	ErrDBQuery  = errors.New("database query failed")

	// Worker pool errors
	ErrWorkerPoolClosed = errors.New("worker pool is closed")
	ErrJobQueueFull     = errors.New("job queue is full")
	ErrContextCanceled  = errors.New("context canceled")

	// Exclusion errors
	ErrInvalidPattern = errors.New("invalid exclusion pattern")

	// Scanner errors
	ErrTooManyErrors = errors.New("too many errors, scan aborted")
	ErrScanAborted   = errors.New("scan aborted")
)

// ScanError represents an error during scanning with context
type ScanError struct {
	Category  string    // Error category (filesystem, hash, database, etc.)
	Path      string    // File path where error occurred
	Operation string    // Operation being performed (read, hash, stat, db_update, etc.)
	Err       error     // Underlying error
	Timestamp time.Time // When error occurred
	Retryable bool      // Whether operation can be retried
}

// Error implements the error interface
func (e *ScanError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s error during %s for %s: %v", e.Category, e.Operation, e.Path, e.Err)
	}
	return fmt.Sprintf("%s error during %s: %v", e.Category, e.Operation, e.Err)
}

// Unwrap returns the underlying error for errors.Is/As
func (e *ScanError) Unwrap() error {
	return e.Err
}

// NewScanError creates a new ScanError with automatic categorization
func NewScanError(path, operation string, err error) *ScanError {
	category, retryable := categorizeError(err)
	return &ScanError{
		Category:  category,
		Path:      path,
		Operation: operation,
		Err:       err,
		Timestamp: time.Now(),
		Retryable: retryable,
	}
}

// categorizeError determines the category and retryability of an error
func categorizeError(err error) (category string, retryable bool) {
	switch {
	case errors.Is(err, ErrAccessDenied):
		return ErrorCategoryFS, false
	case errors.Is(err, ErrFileNotFound):
		return ErrorCategoryFS, false
	case errors.Is(err, ErrIsDirectory):
		return ErrorCategoryFS, false
	case errors.Is(err, ErrInvalidPath):
		return ErrorCategoryFS, false
	case errors.Is(err, ErrHashFailed):
		return ErrorCategoryHash, true
	case errors.Is(err, ErrReadFailed):
		return ErrorCategoryHash, true
	case errors.Is(err, ErrDBUpdate):
		return ErrorCategoryDB, true
	case errors.Is(err, ErrDBQuery):
		return ErrorCategoryDB, true
	case errors.Is(err, ErrWorkerPoolClosed):
		return ErrorCategoryWorker, false
	case errors.Is(err, ErrJobQueueFull):
		return ErrorCategoryWorker, true
	case errors.Is(err, ErrContextCanceled):
		return ErrorCategoryWorker, false
	case errors.Is(err, ErrInvalidPattern):
		return ErrorCategoryExclude, false
	case errors.Is(err, ErrTooManyErrors):
		return ErrorCategoryUnknown, false
	case errors.Is(err, ErrScanAborted):
		return ErrorCategoryUnknown, false
	default:
		return ErrorCategoryUnknown, true
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	msg := fmt.Sprintf(format, args...)
	return fmt.Errorf("%s: %w", msg, err)
}
