package app

import (
	"os"
	"path/filepath"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// --- Type Conversions ---

// convertDBJobToAppJob converts a database SyncJob to an app SyncJob.
func convertDBJobToAppJob(dbJob *database.SyncJob) *SyncJob {
	// Use TriggerParams for exact schedule if available, otherwise fall back to TriggerMode
	// Determine trigger mode from DB
	triggerMode := SyncTriggerMode(dbJob.TriggerParams)
	if triggerMode == "" {
		triggerMode = parseTriggerModeFromDB(dbJob.TriggerMode)
	}

	// Parse job options from network_conditions JSON
	opts := ParseJobOptions(dbJob.NetworkConditions)

	job := &SyncJob{
		ID:                 dbJob.ID,
		Name:               dbJob.Name,
		LocalPath:          dbJob.LocalPath,
		Mode:               parseDBSyncMode(dbJob.SyncMode),
		ConflictResolution: dbJob.ConflictResolution,
		Enabled:            dbJob.Enabled,
		TriggerMode:        triggerMode,
		LastStatus:         JobStatusIdle,
		// Options from JSON
		SyncOnStartup:     opts.SyncOnStartup,
		FilesOnDemand:     opts.FilesOnDemand,
		AutoDehydrateDays: opts.AutoDehydrateDays,
		TrustSource:       opts.TrustSource,
		FirstSyncDone:     opts.FirstSyncDone,
	}

	// Parse remote path into components (format: \\host\share\path)
	// This sets RemoteHost, RemoteShare, and RemotePath (subfolder only)
	parseRemotePath(dbJob.RemotePath, job)

	if dbJob.LastRun != nil {
		job.LastSync = *dbJob.LastRun
	}
	if dbJob.NextRun != nil {
		job.NextSync = *dbJob.NextRun
	}

	return job
}

// convertAppJobToDBJob converts an app SyncJob to a database SyncJob.
func convertAppJobToDBJob(job *SyncJob) *database.SyncJob {
	// Serialize job options to JSON
	opts := &JobOptions{
		SyncOnStartup:     job.SyncOnStartup,
		FilesOnDemand:     job.FilesOnDemand,
		AutoDehydrateDays: job.AutoDehydrateDays,
		TrustSource:       job.TrustSource,
		FirstSyncDone:     job.FirstSyncDone,
	}

	dbJob := &database.SyncJob{
		ID:                 job.ID,
		Name:               job.Name,
		LocalPath:          job.LocalPath,
		RemotePath:         job.FullRemotePath(),
		ServerCredentialID: job.RemoteHost + "_" + job.Username,
		SyncMode:           string(job.Mode),
		TriggerMode:        convertTriggerModeForDB(job.TriggerMode),
		TriggerParams:      string(job.TriggerMode), // Store exact trigger mode
		ConflictResolution: job.ConflictResolution,
		NetworkConditions:  opts.ToJSON(), // Store options as JSON
		Enabled:            job.Enabled,
	}

	if !job.LastSync.IsZero() {
		dbJob.LastRun = &job.LastSync
	}
	if !job.NextSync.IsZero() {
		dbJob.NextRun = &job.NextSync
	}

	return dbJob
}

// parseRemotePath parses a UNC path into host, share, and path components.
func parseRemotePath(remotePath string, job *SyncJob) {
	// Format: \\host\share\path or //host/share/path
	path := remotePath

	// Remove leading slashes
	for len(path) > 0 && (path[0] == '/' || path[0] == '\\') {
		path = path[1:]
	}

	// Split by / or \
	parts := splitPath(path)

	if len(parts) >= 1 {
		job.RemoteHost = parts[0]
	}
	if len(parts) >= 2 {
		job.RemoteShare = parts[1]
	}
	if len(parts) >= 3 {
		// RemotePath is the subfolder within the share (not the full UNC path)
		job.RemotePath = filepath.Join(parts[2:]...)
	} else {
		job.RemotePath = "" // No subfolder, root of share
	}
}

// splitPath splits a path by / or \.
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' || c == '\\' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

// parseDBSyncMode converts a database sync mode string to SyncMode.
func parseDBSyncMode(mode string) syncpkg.SyncMode {
	switch mode {
	case "upload":
		return syncpkg.SyncModeUpload
	case "download":
		return syncpkg.SyncModeDownload
	case "mirror_priority":
		return syncpkg.SyncModeMirrorPriority
	default:
		return syncpkg.SyncModeMirror
	}
}

// parseTriggerModeFromDB converts DB trigger mode to app SyncTriggerMode.
func parseTriggerModeFromDB(trigger string) SyncTriggerMode {
	switch trigger {
	case "interval":
		return SyncTrigger15Min
	case "realtime":
		return SyncTriggerRealtime
	case "scheduled":
		return SyncTrigger1Hour
	default:
		return SyncTriggerManual
	}
}

// convertTriggerModeForDB converts app SyncTriggerMode to DB trigger mode.
func convertTriggerModeForDB(mode SyncTriggerMode) string {
	switch mode {
	case SyncTrigger5Min, SyncTrigger15Min, SyncTrigger30Min:
		return "interval"
	case SyncTrigger1Hour:
		return "scheduled"
	case SyncTriggerRealtime:
		return "realtime"
	default:
		return "manual"
	}
}

// --- SMB Connection Type Conversions ---

// convertDBServerToAppConnection converts a database SMBServer to an app SMBConnection.
func convertDBServerToAppConnection(dbServer *database.SMBServer) *SMBConnection {
	return &SMBConnection{
		ID:           dbServer.ID,
		Name:         dbServer.Name,
		Host:         dbServer.Host,
		Port:         dbServer.Port,
		Username:     dbServer.Username,
		Domain:       dbServer.Domain,
		CredentialID: dbServer.CredentialID,
		SMBVersion:   dbServer.SMBVersion,
	}
}

// convertAppConnectionToDBServer converts an app SMBConnection to a database SMBServer.
func convertAppConnectionToDBServer(conn *SMBConnection) *database.SMBServer {
	return &database.SMBServer{
		ID:           conn.ID,
		Name:         conn.Name,
		Host:         conn.Host,
		Port:         conn.Port,
		Username:     conn.Username,
		Domain:       conn.Domain,
		CredentialID: conn.CredentialID,
		SMBVersion:   conn.SMBVersion,
	}
}

// --- Size Calculation ---

// CalculateFolderSize calculates the total size and file count of a folder.
// Returns size in bytes and file count. Skips directories and handles errors gracefully.
func CalculateFolderSize(folderPath string) (size int64, fileCount int, err error) {
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// Skip files/dirs we can't access
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
			fileCount++
		}
		return nil
	})
	return size, fileCount, err
}

// UpdateJobSizes calculates and updates size information for all jobs.
// This is called periodically in the background.
func (a *App) UpdateJobSizes() {
	a.mu.RLock()
	jobs := make([]*SyncJob, len(a.syncJobs))
	copy(jobs, a.syncJobs)
	a.mu.RUnlock()

	for _, job := range jobs {
		// Check if folder exists
		if _, err := os.Stat(job.LocalPath); os.IsNotExist(err) {
			continue
		}

		// Calculate size
		size, count, err := CalculateFolderSize(job.LocalPath)
		if err != nil {
			a.logger.Debug("Failed to calculate folder size",
				zap.Int64("job_id", job.ID),
				zap.String("path", job.LocalPath),
				zap.Error(err),
			)
			continue
		}

		// Update job (need to find the actual job in the slice)
		a.mu.Lock()
		for _, j := range a.syncJobs {
			if j.ID == job.ID {
				j.LocalSize = size
				j.LocalFileCount = count
				j.SizeUpdatedAt = time.Now()
				break
			}
		}
		a.mu.Unlock()
	}
}

// startSizeUpdater starts a background goroutine to periodically update job sizes.
func (a *App) startSizeUpdater() {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()

		// Initial calculation after a short delay
		select {
		case <-time.After(5 * time.Second):
			a.UpdateJobSizes()
		case <-a.ctx.Done():
			return
		}

		// Update every 30 seconds
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.UpdateJobSizes()
			case <-a.ctx.Done():
				return
			}
		}
	}()
}
