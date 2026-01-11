package database

import "time"

// SyncJob représente un job de synchronisation
type SyncJob struct {
	ID                   int64     `json:"id"`
	Name                 string    `json:"name"`
	LocalPath            string    `json:"local_path"`
	RemotePath           string    `json:"remote_path"`
	ServerCredentialID   string    `json:"server_credential_id"`
	SyncMode             string    `json:"sync_mode"` // mirror, upload, download, mirror_priority
	TriggerMode          string    `json:"trigger_mode"` // realtime, interval, scheduled, manual
	TriggerParams        string    `json:"trigger_params,omitempty"` // JSON
	ConflictResolution   string    `json:"conflict_resolution,omitempty"` // recent, local, remote, both, ask
	NetworkConditions    string    `json:"network_conditions,omitempty"` // JSON
	Enabled              bool      `json:"enabled"`
	LastRun              *time.Time `json:"last_run,omitempty"`
	NextRun              *time.Time `json:"next_run,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// FileState représente l'état d'un fichier synchronisé
type FileState struct {
	ID           int64   `json:"id"`
	JobID        int64   `json:"job_id"`
	LocalPath    string  `json:"local_path"`
	RemotePath   string  `json:"remote_path"`
	Size         int64   `json:"size"`
	MTime        int64   `json:"mtime"` // Unix timestamp de modification
	Hash         string  `json:"hash,omitempty"` // SHA256 (empty if not computed)
	LastSync     *int64  `json:"last_sync,omitempty"` // Unix timestamp
	SyncStatus   string  `json:"sync_status"` // idle, syncing, error, queued
	ErrorMessage *string `json:"error_message,omitempty"`
	CreatedAt    int64   `json:"created_at"` // Unix timestamp
	UpdatedAt    int64   `json:"updated_at"` // Unix timestamp
}

// Exclusion représente une règle d'exclusion
type Exclusion struct {
	ID            int64     `json:"id"`
	Type          string    `json:"type"` // global, job, individual
	PatternOrPath string    `json:"pattern_or_path"`
	Reason        string    `json:"reason,omitempty"`
	DateAdded     time.Time `json:"date_added"`
	JobID         *int64    `json:"job_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// SyncHistory représente une entrée d'historique de synchronisation
type SyncHistory struct {
	ID               int64     `json:"id"`
	JobID            int64     `json:"job_id"`
	Timestamp        time.Time `json:"timestamp"`
	FilesSynced      int       `json:"files_synced"`
	FilesFailed      int       `json:"files_failed"`
	BytesTransferred int64     `json:"bytes_transferred"`
	Duration         int       `json:"duration"` // En secondes
	Status           string    `json:"status"` // success, partial, failed
	ErrorSummary     string    `json:"error_summary,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// SMBServer représente un serveur SMB configuré
type SMBServer struct {
	ID                     int64      `json:"id"`
	Name                   string     `json:"name"`
	Host                   string     `json:"host"`
	Port                   int        `json:"port"`
	Share                  string     `json:"share"`
	Domain                 string     `json:"domain,omitempty"`
	CredentialID           string     `json:"credential_id"` // ID dans le keystore
	SMBVersion             string     `json:"smb_version,omitempty"`
	LastConnectionTest     *time.Time `json:"last_connection_test,omitempty"`
	LastConnectionStatus   string     `json:"last_connection_status,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

// OfflineQueueItem représente un élément dans la file d'attente hors-ligne
type OfflineQueueItem struct {
	ID         int64     `json:"id"`
	JobID      int64     `json:"job_id"`
	FilePath   string    `json:"file_path"`
	Operation  string    `json:"operation"` // upload, download, delete
	Priority   int       `json:"priority"`
	RetryCount int       `json:"retry_count"`
	LastError  string    `json:"last_error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// JobStatistics représente les statistiques d'un job
type JobStatistics struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Enabled          bool       `json:"enabled"`
	TotalFiles       int        `json:"total_files"`
	TotalSize        int64      `json:"total_size"`
	LastSyncTime     *time.Time `json:"last_sync_time,omitempty"`
	FilesWithErrors  int        `json:"files_with_errors"`
	QueuedOperations int        `json:"queued_operations"`
}
