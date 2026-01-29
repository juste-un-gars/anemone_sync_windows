package database

import (
	"database/sql"
	"fmt"
	"time"
)

// --- Sync Jobs CRUD ---

// GetSyncJob retrieves a sync job by ID
func (db *DB) GetSyncJob(jobID int64) (*SyncJob, error) {
	var job SyncJob
	var lastRun, nextRun sql.NullInt64
	var triggerParams, conflictRes, networkCond sql.NullString
	var createdAt, updatedAt int64

	err := db.conn.QueryRow(`
		SELECT id, name, local_path, remote_path, server_credential_id,
			   sync_mode, trigger_mode, trigger_params, conflict_resolution,
			   network_conditions, enabled, last_run, next_run,
			   created_at, updated_at
		FROM sync_jobs
		WHERE id = ?
	`, jobID).Scan(
		&job.ID, &job.Name, &job.LocalPath, &job.RemotePath, &job.ServerCredentialID,
		&job.SyncMode, &job.TriggerMode, &triggerParams, &conflictRes,
		&networkCond, &job.Enabled, &lastRun, &nextRun,
		&createdAt, &updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Job not found
		}
		return nil, fmt.Errorf("get sync job: %w", err)
	}

	if triggerParams.Valid {
		job.TriggerParams = triggerParams.String
	}
	if conflictRes.Valid {
		job.ConflictResolution = conflictRes.String
	}
	if networkCond.Valid {
		job.NetworkConditions = networkCond.String
	}
	if lastRun.Valid {
		t := time.Unix(lastRun.Int64, 0)
		job.LastRun = &t
	}
	if nextRun.Valid {
		t := time.Unix(nextRun.Int64, 0)
		job.NextRun = &t
	}
	job.CreatedAt = time.Unix(createdAt, 0)
	job.UpdatedAt = time.Unix(updatedAt, 0)

	return &job, nil
}

// GetAllSyncJobs retrieves all sync jobs
func (db *DB) GetAllSyncJobs() ([]*SyncJob, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, local_path, remote_path, server_credential_id,
			   sync_mode, trigger_mode, trigger_params, conflict_resolution,
			   network_conditions, enabled, last_run, next_run,
			   created_at, updated_at
		FROM sync_jobs
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query sync jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*SyncJob
	for rows.Next() {
		var job SyncJob
		var lastRun, nextRun sql.NullInt64
		var triggerParams, conflictRes, networkCond sql.NullString
		var createdAt, updatedAt int64

		err := rows.Scan(
			&job.ID, &job.Name, &job.LocalPath, &job.RemotePath, &job.ServerCredentialID,
			&job.SyncMode, &job.TriggerMode, &triggerParams, &conflictRes,
			&networkCond, &job.Enabled, &lastRun, &nextRun,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan sync job: %w", err)
		}

		if triggerParams.Valid {
			job.TriggerParams = triggerParams.String
		}
		if conflictRes.Valid {
			job.ConflictResolution = conflictRes.String
		}
		if networkCond.Valid {
			job.NetworkConditions = networkCond.String
		}
		if lastRun.Valid {
			t := time.Unix(lastRun.Int64, 0)
			job.LastRun = &t
		}
		if nextRun.Valid {
			t := time.Unix(nextRun.Int64, 0)
			job.NextRun = &t
		}
		job.CreatedAt = time.Unix(createdAt, 0)
		job.UpdatedAt = time.Unix(updatedAt, 0)

		jobs = append(jobs, &job)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync jobs: %w", err)
	}

	return jobs, nil
}

// CreateSyncJob creates a new sync job
func (db *DB) CreateSyncJob(job *SyncJob) error {
	now := time.Now().Unix()

	var lastRunUnix, nextRunUnix sql.NullInt64
	if job.LastRun != nil {
		lastRunUnix = sql.NullInt64{Int64: job.LastRun.Unix(), Valid: true}
	}
	if job.NextRun != nil {
		nextRunUnix = sql.NullInt64{Int64: job.NextRun.Unix(), Valid: true}
	}

	result, err := db.conn.Exec(`
		INSERT INTO sync_jobs (
			name, local_path, remote_path, server_credential_id,
			sync_mode, trigger_mode, trigger_params, conflict_resolution,
			network_conditions, enabled, last_run, next_run,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		job.Name, job.LocalPath, job.RemotePath, job.ServerCredentialID,
		job.SyncMode, job.TriggerMode, job.TriggerParams, job.ConflictResolution,
		job.NetworkConditions, job.Enabled, lastRunUnix, nextRunUnix,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("insert sync job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	job.ID = id
	job.CreatedAt = time.Unix(now, 0)
	job.UpdatedAt = time.Unix(now, 0)

	return nil
}

// UpdateSyncJob updates an existing sync job
func (db *DB) UpdateSyncJob(job *SyncJob) error {
	now := time.Now().Unix()

	var lastRunUnix, nextRunUnix sql.NullInt64
	if job.LastRun != nil {
		lastRunUnix = sql.NullInt64{Int64: job.LastRun.Unix(), Valid: true}
	}
	if job.NextRun != nil {
		nextRunUnix = sql.NullInt64{Int64: job.NextRun.Unix(), Valid: true}
	}

	result, err := db.conn.Exec(`
		UPDATE sync_jobs SET
			name = ?, local_path = ?, remote_path = ?, server_credential_id = ?,
			sync_mode = ?, trigger_mode = ?, trigger_params = ?, conflict_resolution = ?,
			network_conditions = ?, enabled = ?, last_run = ?, next_run = ?,
			updated_at = ?
		WHERE id = ?
	`,
		job.Name, job.LocalPath, job.RemotePath, job.ServerCredentialID,
		job.SyncMode, job.TriggerMode, job.TriggerParams, job.ConflictResolution,
		job.NetworkConditions, job.Enabled, lastRunUnix, nextRunUnix,
		now, job.ID,
	)
	if err != nil {
		return fmt.Errorf("update sync job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("sync job not found: %d", job.ID)
	}

	job.UpdatedAt = time.Unix(now, 0)
	return nil
}

// DeleteSyncJob deletes a sync job by ID
func (db *DB) DeleteSyncJob(jobID int64) error {
	result, err := db.conn.Exec(`DELETE FROM sync_jobs WHERE id = ?`, jobID)
	if err != nil {
		return fmt.Errorf("delete sync job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("sync job not found: %d", jobID)
	}

	return nil
}

// UpdateJobStatus updates the status of a sync job
// Note: Status is not a field in SyncJob model, we'll update it via sync_history
// For now, we'll use this to mark jobs as syncing/idle/error via a metadata approach
// or we can just log it. Let's keep it simple and not persist status for now.
func (db *DB) UpdateJobStatus(jobID int64, status string) error {
	// For Phase 4, we'll just log the status
	// In Phase 5+, we could add a status field to sync_jobs table
	// For now, this is a no-op that won't break the engine
	return nil
}

// UpdateJobLastRun updates the last run timestamp of a sync job
func (db *DB) UpdateJobLastRun(jobID int64, lastRun time.Time) error {
	_, err := db.conn.Exec(`
		UPDATE sync_jobs
		SET last_run = ?, updated_at = ?
		WHERE id = ?
	`, lastRun.Unix(), time.Now().Unix(), jobID)

	if err != nil {
		return fmt.Errorf("update job last run: %w", err)
	}

	return nil
}

// --- Sync History ---

// InsertSyncHistory inserts a sync history record
func (db *DB) InsertSyncHistory(history *SyncHistory) error {
	_, err := db.conn.Exec(`
		INSERT INTO sync_history (
			job_id, timestamp, files_synced, files_failed,
			bytes_transferred, duration, status, error_summary, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		history.JobID,
		history.Timestamp.Unix(),
		history.FilesSynced,
		history.FilesFailed,
		history.BytesTransferred,
		history.Duration,
		history.Status,
		history.ErrorSummary,
		time.Now().Unix(),
	)

	if err != nil {
		return fmt.Errorf("insert sync history: %w", err)
	}

	return nil
}

// --- Job Statistics ---

// GetJobStatistics retrieves statistics for a sync job
func (db *DB) GetJobStatistics(jobID int64) (*JobStatistics, error) {
	job, err := db.GetSyncJob(jobID)
	if err != nil {
		return nil, err
	}
	if job == nil {
		return nil, fmt.Errorf("job not found")
	}

	stats := &JobStatistics{
		ID:      job.ID,
		Name:    job.Name,
		Enabled: job.Enabled,
	}

	// Count total files
	err = db.conn.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(size), 0)
		FROM files_state
		WHERE job_id = ?
	`, jobID).Scan(&stats.TotalFiles, &stats.TotalSize)

	if err != nil {
		return nil, fmt.Errorf("get file stats: %w", err)
	}

	// Count files with errors
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM files_state
		WHERE job_id = ? AND sync_status = 'error'
	`, jobID).Scan(&stats.FilesWithErrors)

	if err != nil {
		return nil, fmt.Errorf("get error count: %w", err)
	}

	// Get last sync time
	var lastSyncUnix sql.NullInt64
	err = db.conn.QueryRow(`
		SELECT MAX(last_sync)
		FROM files_state
		WHERE job_id = ?
	`, jobID).Scan(&lastSyncUnix)

	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("get last sync: %w", err)
	}

	if lastSyncUnix.Valid {
		t := time.Unix(lastSyncUnix.Int64, 0)
		stats.LastSyncTime = &t
	}

	// Count queued operations (for future offline queue feature)
	err = db.conn.QueryRow(`
		SELECT COUNT(*)
		FROM offline_queue_items
		WHERE job_id = ?
	`, jobID).Scan(&stats.QueuedOperations)

	if err != nil && err != sql.ErrNoRows {
		// Offline queue table may not exist yet
		stats.QueuedOperations = 0
	}

	return stats, nil
}

// --- Exclusions ---

// GetExclusions retrieves all exclusions for a job (global + job-specific)
func (db *DB) GetExclusions(jobID int64) ([]*Exclusion, error) {
	rows, err := db.conn.Query(`
		SELECT id, type, pattern_or_path, reason, date_added, job_id, created_at
		FROM exclusions
		WHERE type = 'global' OR (type = 'job' AND job_id = ?)
		ORDER BY type DESC, id ASC
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("query exclusions: %w", err)
	}
	defer rows.Close()

	var exclusions []*Exclusion
	for rows.Next() {
		var excl Exclusion
		err := rows.Scan(
			&excl.ID,
			&excl.Type,
			&excl.PatternOrPath,
			&excl.Reason,
			&excl.DateAdded,
			&excl.JobID,
			&excl.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan exclusion: %w", err)
		}
		exclusions = append(exclusions, &excl)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exclusions: %w", err)
	}

	return exclusions, nil
}

// GetIndividualExclusions retrieves individual path exclusions for a job
func (db *DB) GetIndividualExclusions(jobID int64) (map[string]bool, error) {
	rows, err := db.conn.Query(`
		SELECT pattern_or_path
		FROM exclusions
		WHERE type = 'individual' AND job_id = ?
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("query individual exclusions: %w", err)
	}
	defer rows.Close()

	paths := make(map[string]bool)
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, fmt.Errorf("scan individual exclusion: %w", err)
		}
		paths[path] = true
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate individual exclusions: %w", err)
	}

	return paths, nil
}
