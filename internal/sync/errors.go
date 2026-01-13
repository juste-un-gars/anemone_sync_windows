package sync

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// Common sync errors
var (
	// Validation errors
	ErrInvalidJobID              = errors.New("invalid job ID")
	ErrInvalidLocalPath          = errors.New("invalid local path")
	ErrInvalidRemotePath         = errors.New("invalid remote path")
	ErrInvalidSyncMode           = errors.New("invalid sync mode")
	ErrInvalidConflictResolution = errors.New("invalid conflict resolution policy")

	// State errors
	ErrSyncInProgress = errors.New("sync already in progress for this job")
	ErrSyncNotFound   = errors.New("sync not found")
	ErrEngineClosed   = errors.New("sync engine is closed")

	// Operation errors
	ErrSyncAborted      = errors.New("sync was aborted")
	ErrContextCancelled = errors.New("context cancelled")
)

// ErrorCategory classifies error types
type ErrorCategory string

const (
	// ErrorCategoryNetwork indicates network-related errors
	ErrorCategoryNetwork ErrorCategory = "network"
	// ErrorCategoryFileSystem indicates filesystem errors
	ErrorCategoryFileSystem ErrorCategory = "filesystem"
	// ErrorCategoryDatabase indicates database errors
	ErrorCategoryDatabase ErrorCategory = "database"
	// ErrorCategorySMB indicates SMB-specific errors
	ErrorCategorySMB ErrorCategory = "smb"
	// ErrorCategoryPermission indicates permission errors
	ErrorCategoryPermission ErrorCategory = "permission"
	// ErrorCategoryUnknown indicates unknown error type
	ErrorCategoryUnknown ErrorCategory = "unknown"
)

// ClassifyError analyzes an error and returns its category and whether it's retryable
func ClassifyError(err error) (ErrorCategory, bool) {
	if err == nil {
		return ErrorCategoryUnknown, false
	}

	// Check for specific error types
	if IsNetworkError(err) {
		return ErrorCategoryNetwork, true // Network errors are generally retryable
	}

	if IsPermissionError(err) {
		return ErrorCategoryPermission, false // Permission errors are not retryable
	}

	if IsFileSystemError(err) {
		return ErrorCategoryFileSystem, IsTransientFileSystemError(err)
	}

	if IsDatabaseError(err) {
		return ErrorCategoryDatabase, true // DB errors may be retryable
	}

	if IsSMBError(err) {
		return ErrorCategorySMB, IsTransientSMBError(err)
	}

	return ErrorCategoryUnknown, false
}

// IsNetworkError returns true if the error is network-related
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check error message patterns
	msg := err.Error()
	networkPatterns := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"timeout",
		"network",
		"dial tcp",
		"no route to host",
		"host is down",
	}

	for _, pattern := range networkPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	return false
}

// IsPermissionError returns true if the error is permission-related
func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types
	if errors.Is(err, os.ErrPermission) {
		return true
	}

	if errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.EPERM) {
		return true
	}

	// Check error message patterns
	msg := err.Error()
	permissionPatterns := []string{
		"permission denied",
		"access denied",
		"access is denied",
		"insufficient permissions",
	}

	for _, pattern := range permissionPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	return false
}

// IsFileSystemError returns true if the error is filesystem-related
func IsFileSystemError(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific error types
	if errors.Is(err, os.ErrNotExist) ||
		errors.Is(err, os.ErrExist) ||
		errors.Is(err, os.ErrInvalid) {
		return true
	}

	// Check error message patterns
	msg := err.Error()
	fsPatterns := []string{
		"file not found",
		"no such file",
		"invalid path",
		"path too long",
		"disk full",
		"no space left",
	}

	for _, pattern := range fsPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	return false
}

// IsTransientFileSystemError returns true if the filesystem error is transient
func IsTransientFileSystemError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	transientPatterns := []string{
		"file is locked",
		"used by another process",
		"resource temporarily unavailable",
		"disk full", // May be temporary on remote
	}

	for _, pattern := range transientPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	// File not found is not retryable (race condition acceptable)
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	return false
}

// IsDatabaseError returns true if the error is database-related
func IsDatabaseError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	dbPatterns := []string{
		"database",
		"sql",
		"sqlite",
		"constraint",
	}

	for _, pattern := range dbPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	return false
}

// IsSMBError returns true if the error is SMB-related
func IsSMBError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	smbPatterns := []string{
		"smb",
		"not connected",
		"server",
		"share",
	}

	for _, pattern := range smbPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	return false
}

// IsTransientSMBError returns true if the SMB error is transient
func IsTransientSMBError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	transientPatterns := []string{
		"server busy",
		"server not responding",
		"timeout",
		"temporarily unavailable",
	}

	for _, pattern := range transientPatterns {
		if contains(msg, pattern) {
			return true
		}
	}

	// Not connected is retryable (can reconnect)
	if contains(msg, "not connected") {
		return true
	}

	return false
}

// IsTransientError returns true if the error is transient and should be retried
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	_, retryable := ClassifyError(err)
	return retryable
}

// WrapSyncError wraps an error with sync context
func WrapSyncError(err error, filePath, operation string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s failed for %s: %w", operation, filePath, err)
}

// NewSyncError creates a new SyncError from an error
func NewSyncError(filePath, operation string, err error, attempt int) *SyncError {
	_, retryable := ClassifyError(err)
	return &SyncError{
		FilePath:  filePath,
		Operation: operation,
		Error:     err,
		Retryable: retryable,
		Timestamp: timeNow(), // Using time.Now() via variable for testability
		Attempt:   attempt,
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive check
	sLower := toLower(s)
	substrLower := toLower(substr)
	return containsSubstring(sLower, substrLower)
}

// toLower converts string to lowercase (simple ASCII version)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

// containsSubstring checks if s contains substr
func containsSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// timeNow is a variable to allow mocking in tests
var timeNow = func() time.Time {
	return time.Now()
}
