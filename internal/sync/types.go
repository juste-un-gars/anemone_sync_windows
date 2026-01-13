package sync

import (
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
)

// SyncMode defines the direction of synchronization
type SyncMode string

const (
	// SyncModeMirror performs bidirectional synchronization
	SyncModeMirror SyncMode = "mirror"
	// SyncModeUpload only uploads local changes to remote
	SyncModeUpload SyncMode = "upload"
	// SyncModeDownload only downloads remote changes to local
	SyncModeDownload SyncMode = "download"
	// SyncModeMirrorPriority performs bidirectional sync with priority
	SyncModeMirrorPriority SyncMode = "mirror_priority"
)

// SyncRequest represents a synchronization request
type SyncRequest struct {
	// JobID is the sync job identifier from sync_jobs table
	JobID int64

	// LocalPath is the local base directory path
	LocalPath string

	// RemotePath is the remote base path on SMB server
	RemotePath string

	// Mode defines the sync direction
	Mode SyncMode

	// ConflictResolution defines how to resolve conflicts
	// Values: "recent", "local", "remote", "ask"
	ConflictResolution string

	// DryRun if true, simulates sync without executing actions
	DryRun bool

	// ProgressCallback is called to report progress (optional)
	ProgressCallback ProgressCallback
}

// SyncResult contains the result of a sync operation
type SyncResult struct {
	// JobID is the sync job identifier
	JobID int64

	// Status indicates the overall sync outcome
	Status SyncStatus

	// Timestamps
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration

	// File counts
	TotalFiles      int // Total files examined
	FilesUploaded   int // Files uploaded to remote
	FilesDownloaded int // Files downloaded from remote
	FilesDeleted    int // Files deleted (local or remote)
	FilesSkipped    int // Files skipped (unchanged)
	FilesError      int // Files with errors
	ConflictsFound  int // Conflicts detected

	// Data transfer
	BytesTransferred int64 // Total bytes transferred

	// Details
	Errors    []*SyncError           // Errors encountered
	Conflicts []*cache.SyncDecision  // Unresolved conflicts
	Actions   []*SyncAction          // Actions taken
}

// SyncStatus represents the outcome of a sync
type SyncStatus string

const (
	// SyncStatusSuccess indicates all files synced successfully
	SyncStatusSuccess SyncStatus = "success"
	// SyncStatusPartial indicates some files failed but sync continued
	SyncStatusPartial SyncStatus = "partial"
	// SyncStatusFailed indicates sync failed completely
	SyncStatusFailed SyncStatus = "failed"
)

// SyncAction represents an action taken during sync
type SyncAction struct {
	// FilePath is the local file path
	FilePath string

	// RemotePath is the remote file path
	RemotePath string

	// Action is the sync action taken
	Action cache.SyncAction

	// Status is the result of this action
	Status ActionStatus

	// Size is the file size in bytes
	Size int64

	// BytesTransferred is the actual bytes transferred
	BytesTransferred int64

	// Error if action failed
	Error error

	// Duration of the action
	Duration time.Duration

	// Timestamp when action was executed
	Timestamp time.Time
}

// ActionStatus represents the status of a sync action
type ActionStatus string

const (
	// ActionStatusPending indicates action not yet started
	ActionStatusPending ActionStatus = "pending"
	// ActionStatusExecuting indicates action in progress
	ActionStatusExecuting ActionStatus = "executing"
	// ActionStatusSuccess indicates action completed successfully
	ActionStatusSuccess ActionStatus = "success"
	// ActionStatusFailed indicates action failed
	ActionStatusFailed ActionStatus = "failed"
	// ActionStatusSkipped indicates action was skipped
	ActionStatusSkipped ActionStatus = "skipped"
)

// SyncError represents an error during sync
type SyncError struct {
	// FilePath where error occurred
	FilePath string

	// Operation that failed (e.g., "upload", "download", "delete")
	Operation string

	// Error details
	Error error

	// Retryable indicates if error is transient
	Retryable bool

	// Timestamp when error occurred
	Timestamp time.Time

	// Attempt number (for retries)
	Attempt int
}

// SyncProgress represents progress information
type SyncProgress struct {
	// Phase is the current sync phase
	// Values: "preparation", "scanning", "detecting", "executing", "finalizing"
	Phase string

	// CurrentFile being processed (optional)
	CurrentFile string

	// FilesProcessed so far
	FilesProcessed int

	// FilesTotal expected (may be estimate)
	FilesTotal int

	// BytesTransferred so far
	BytesTransferred int64

	// BytesTotal expected (may be estimate)
	BytesTotal int64

	// CurrentAction being performed
	CurrentAction string

	// Percentage complete (0.0 to 100.0)
	Percentage float64

	// Message for display (optional)
	Message string
}

// ProgressCallback is called to report progress updates
type ProgressCallback func(progress *SyncProgress)

// String returns the string representation of SyncMode
func (m SyncMode) String() string {
	return string(m)
}

// String returns the string representation of SyncStatus
func (s SyncStatus) String() string {
	return string(s)
}

// String returns the string representation of ActionStatus
func (s ActionStatus) String() string {
	return string(s)
}

// IsTerminal returns true if the action status is terminal (success/failed/skipped)
func (s ActionStatus) IsTerminal() bool {
	return s == ActionStatusSuccess || s == ActionStatusFailed || s == ActionStatusSkipped
}

// Validate validates the sync request
func (r *SyncRequest) Validate() error {
	if r.JobID <= 0 {
		return ErrInvalidJobID
	}
	if r.LocalPath == "" {
		return ErrInvalidLocalPath
	}
	if r.RemotePath == "" {
		return ErrInvalidRemotePath
	}
	if !r.Mode.IsValid() {
		return ErrInvalidSyncMode
	}
	if !IsValidConflictResolution(r.ConflictResolution) {
		return ErrInvalidConflictResolution
	}
	return nil
}

// IsValid returns true if the sync mode is valid
func (m SyncMode) IsValid() bool {
	switch m {
	case SyncModeMirror, SyncModeUpload, SyncModeDownload, SyncModeMirrorPriority:
		return true
	default:
		return false
	}
}

// IsValidConflictResolution returns true if the conflict resolution strategy is valid
func IsValidConflictResolution(policy string) bool {
	switch policy {
	case "recent", "local", "remote", "ask":
		return true
	default:
		return false
	}
}

// IsBidirectional returns true if the sync mode is bidirectional
func (m SyncMode) IsBidirectional() bool {
	return m == SyncModeMirror || m == SyncModeMirrorPriority
}

// AllowsUpload returns true if the sync mode allows uploads
func (m SyncMode) AllowsUpload() bool {
	return m == SyncModeMirror || m == SyncModeUpload || m == SyncModeMirrorPriority
}

// AllowsDownload returns true if the sync mode allows downloads
func (m SyncMode) AllowsDownload() bool {
	return m == SyncModeMirror || m == SyncModeDownload || m == SyncModeMirrorPriority
}

// NewSyncResult creates a new SyncResult with initialized fields
func NewSyncResult(jobID int64) *SyncResult {
	return &SyncResult{
		JobID:     jobID,
		StartTime: time.Now(),
		Status:    SyncStatusSuccess, // Default to success, will be updated
		Errors:    make([]*SyncError, 0),
		Conflicts: make([]*cache.SyncDecision, 0),
		Actions:   make([]*SyncAction, 0),
	}
}

// Finalize completes the sync result by setting end time and calculating duration
func (r *SyncResult) Finalize() {
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)

	// Determine overall status
	if r.FilesError > 0 {
		if r.FilesError == r.TotalFiles {
			r.Status = SyncStatusFailed
		} else {
			r.Status = SyncStatusPartial
		}
	} else if r.ConflictsFound > 0 {
		r.Status = SyncStatusPartial
	} else {
		r.Status = SyncStatusSuccess
	}
}

// AddError adds an error to the sync result
func (r *SyncResult) AddError(err *SyncError) {
	r.Errors = append(r.Errors, err)
	r.FilesError++
}

// AddConflict adds a conflict to the sync result
func (r *SyncResult) AddConflict(conflict *cache.SyncDecision) {
	r.Conflicts = append(r.Conflicts, conflict)
	r.ConflictsFound++
}

// AddAction adds an action to the sync result
func (r *SyncResult) AddAction(action *SyncAction) {
	r.Actions = append(r.Actions, action)

	// Update counters based on action
	if action.Status == ActionStatusSuccess {
		switch action.Action {
		case cache.ActionUpload:
			r.FilesUploaded++
			r.BytesTransferred += action.BytesTransferred
		case cache.ActionDownload:
			r.FilesDownloaded++
			r.BytesTransferred += action.BytesTransferred
		case cache.ActionDeleteLocal, cache.ActionDeleteRemote:
			r.FilesDeleted++
		}
	} else if action.Status == ActionStatusSkipped {
		r.FilesSkipped++
	}
}
