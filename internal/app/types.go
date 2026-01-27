package app

import (
	"encoding/json"
	"time"

	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
)

// JobOptions contains job options stored as JSON in network_conditions field.
type JobOptions struct {
	SyncOnStartup bool `json:"sync_on_startup,omitempty"` // Sync immediately when app starts via autostart
	// Files On Demand (Cloud Files API)
	FilesOnDemand     bool `json:"files_on_demand,omitempty"`     // Enable placeholder files
	AutoDehydrateDays int  `json:"auto_dehydrate_days,omitempty"` // Auto-dehydrate files not accessed for X days (0 = disabled)
	// Trust source for conflict resolution
	TrustSource    string `json:"trust_source,omitempty"`    // "ask", "server", "local", "recent"
	FirstSyncDone  bool   `json:"first_sync_done,omitempty"` // True after first sync wizard is completed
}

// ToJSON serializes JobOptions to JSON string.
func (o *JobOptions) ToJSON() string {
	data, err := json.Marshal(o)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// ParseJobOptions parses JobOptions from JSON string.
func ParseJobOptions(jsonStr string) *JobOptions {
	opts := &JobOptions{}
	if jsonStr == "" {
		return opts
	}
	json.Unmarshal([]byte(jsonStr), opts)
	return opts
}

// SyncTriggerMode defines when sync is triggered.
type SyncTriggerMode string

const (
	SyncTriggerManual   SyncTriggerMode = "manual"   // Manual sync only
	SyncTrigger5Min     SyncTriggerMode = "5m"       // Every 5 minutes
	SyncTrigger15Min    SyncTriggerMode = "15m"      // Every 15 minutes
	SyncTrigger30Min    SyncTriggerMode = "30m"      // Every 30 minutes
	SyncTrigger1Hour    SyncTriggerMode = "1h"       // Every hour
	SyncTriggerRealtime SyncTriggerMode = "realtime" // Realtime (local watcher + remote check every 5min)
)

// SyncJob represents a configured sync job for the UI.
type SyncJob struct {
	ID                 int64
	Name               string
	LocalPath          string
	SMBConnectionID    int64  // Reference to SMBConnection
	RemoteHost         string // SMB server address (from SMBConnection)
	RemoteShare        string // Share name (from SMBConnection)
	RemotePath         string // Path within share (job-specific subfolder)
	Username           string // Username (from SMBConnection)
	Mode               syncpkg.SyncMode
	ConflictResolution string
	Enabled            bool
	LastSync           time.Time
	LastStatus         JobStatus
	NextSync           time.Time
	// Sync trigger mode: "manual", "5m", "15m", "30m", "1h", "realtime"
	TriggerMode   SyncTriggerMode
	SyncOnStartup bool // Sync immediately when app starts via autostart
	// Files On Demand (Cloud Files API)
	FilesOnDemand     bool // Enable placeholder files (download on demand)
	AutoDehydrateDays int  // Auto-dehydrate files not accessed for X days (0 = disabled)
	// Trust source for conflict resolution
	TrustSource   string // "ask", "server", "local", "recent"
	FirstSyncDone bool   // True after first sync wizard is completed
	// Size information (calculated periodically, not persisted)
	LocalSize      int64 // Total size of local folder in bytes
	LocalFileCount int   // Number of files in local folder
	SizeUpdatedAt  time.Time
}

// JobStatus represents the status of a sync job.
type JobStatus string

const (
	JobStatusIdle      JobStatus = "idle"
	JobStatusSyncing   JobStatus = "syncing"
	JobStatusSuccess   JobStatus = "success"
	JobStatusPartial   JobStatus = "partial"
	JobStatusFailed    JobStatus = "failed"
	JobStatusDisabled  JobStatus = "disabled"
)

// String returns the display string for JobStatus.
func (s JobStatus) String() string {
	switch s {
	case JobStatusIdle:
		return "Idle"
	case JobStatusSyncing:
		return "Syncing..."
	case JobStatusSuccess:
		return "Success"
	case JobStatusPartial:
		return "Partial"
	case JobStatusFailed:
		return "Failed"
	case JobStatusDisabled:
		return "Disabled"
	default:
		return string(s)
	}
}

// Icon returns a status indicator character.
func (s JobStatus) Icon() string {
	switch s {
	case JobStatusIdle:
		return "-"
	case JobStatusSyncing:
		return "~"
	case JobStatusSuccess:
		return "+"
	case JobStatusPartial:
		return "!"
	case JobStatusFailed:
		return "X"
	case JobStatusDisabled:
		return "O"
	default:
		return "?"
	}
}

// FullRemotePath returns the complete SMB path.
func (j *SyncJob) FullRemotePath() string {
	if j.RemotePath == "" || j.RemotePath == "/" {
		return "\\\\" + j.RemoteHost + "\\" + j.RemoteShare
	}
	return "\\\\" + j.RemoteHost + "\\" + j.RemoteShare + "\\" + j.RemotePath
}

// SMBConnection represents a configured SMB server connection.
// Passwords are stored securely in the Windows Credential Manager, not in the DB.
// Note: Share is not stored here - it's selected when creating a sync job.
type SMBConnection struct {
	ID           int64
	Name         string // Display name for the connection
	Host         string // Server IP or FQDN
	Port         int    // Default 445
	Domain       string // Optional domain
	Username     string // Username for authentication
	CredentialID string // Reference to Windows Credential Manager
	SMBVersion   string // "2.0", "2.1", "3.0", "3.1.1"
}

// DisplayName returns a formatted display name for the connection.
func (c *SMBConnection) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.Host
}

// AppSettings holds application-wide settings.
type AppSettings struct {
	AutoStart            bool
	NotificationsEnabled bool
	LogLevel             string
	SyncInterval         string
}

// DefaultAppSettings returns default settings.
func DefaultAppSettings() *AppSettings {
	return &AppSettings{
		AutoStart:            false,
		NotificationsEnabled: true,
		LogLevel:             "Info",
		SyncInterval:         "15 minutes",
	}
}
