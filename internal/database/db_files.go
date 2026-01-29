package database

import (
	"database/sql"
	"fmt"
	"time"
)

// --- File State Operations ---

// GetFileState retrieves file state from database
func (db *DB) GetFileState(jobID int64, localPath string) (*FileState, error) {
	var state FileState
	var hash, errorMsg sql.NullString
	var lastSync sql.NullInt64

	err := db.conn.QueryRow(`
		SELECT id, job_id, local_path, remote_path, size, mtime, hash,
		       last_sync, sync_status, error_message, created_at, updated_at
		FROM files_state
		WHERE job_id = ? AND local_path = ?
	`, jobID, localPath).Scan(
		&state.ID,
		&state.JobID,
		&state.LocalPath,
		&state.RemotePath,
		&state.Size,
		&state.MTime,
		&hash,
		&lastSync,
		&state.SyncStatus,
		&errorMsg,
		&state.CreatedAt,
		&state.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file state not found for job %d path %s", jobID, localPath)
	}
	if err != nil {
		return nil, fmt.Errorf("query file state: %w", err)
	}

	// Convert sql.Null* types
	state.Hash = hash.String // Empty string if NULL
	if lastSync.Valid {
		state.LastSync = &lastSync.Int64
	}
	if errorMsg.Valid {
		state.ErrorMessage = &errorMsg.String
	}

	return &state, nil
}

// UpsertFileState inserts or updates a file state
func (db *DB) UpsertFileState(state *FileState) error {
	now := time.Now().Unix()

	// Handle optional last_sync
	var lastSync interface{}
	if state.LastSync != nil {
		lastSync = *state.LastSync
	} else {
		lastSync = nil
	}

	_, err := db.conn.Exec(`
		INSERT INTO files_state (job_id, local_path, remote_path, size, mtime, hash, last_sync, sync_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id, local_path)
		DO UPDATE SET
			remote_path = excluded.remote_path,
			size = excluded.size,
			mtime = excluded.mtime,
			hash = excluded.hash,
			last_sync = excluded.last_sync,
			sync_status = excluded.sync_status,
			updated_at = excluded.updated_at
	`, state.JobID, state.LocalPath, state.RemotePath, state.Size, state.MTime, state.Hash, lastSync, state.SyncStatus, now, now)

	if err != nil {
		return fmt.Errorf("upsert file state: %w", err)
	}
	return nil
}

// BulkUpdateFileStates updates multiple file states in a single transaction
func (db *DB) BulkUpdateFileStates(states []*FileState) error {
	if len(states) == 0 {
		return nil
	}

	return db.Transaction(func(tx *sql.Tx) error {
		now := time.Now().Unix()
		stmt, err := tx.Prepare(`
			INSERT INTO files_state (job_id, local_path, remote_path, size, mtime, hash, last_sync, sync_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(job_id, local_path)
			DO UPDATE SET
				remote_path = excluded.remote_path,
				size = excluded.size,
				mtime = excluded.mtime,
				hash = excluded.hash,
				last_sync = excluded.last_sync,
				sync_status = excluded.sync_status,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return fmt.Errorf("prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, state := range states {
			lastSync := now
			if state.LastSync != nil {
				lastSync = *state.LastSync
			}
			_, err := stmt.Exec(state.JobID, state.LocalPath, state.RemotePath, state.Size, state.MTime, state.Hash, lastSync, state.SyncStatus, now, now)
			if err != nil {
				return fmt.Errorf("execute statement for %s: %w", state.LocalPath, err)
			}
		}

		return nil
	})
}

// GetAllFileStates retrieves all file states for a job
func (db *DB) GetAllFileStates(jobID int64) ([]*FileState, error) {
	rows, err := db.conn.Query(`
		SELECT id, job_id, local_path, remote_path, size, mtime, hash,
		       last_sync, sync_status, error_message, created_at, updated_at
		FROM files_state
		WHERE job_id = ?
	`, jobID)
	if err != nil {
		return nil, fmt.Errorf("query file states: %w", err)
	}
	defer rows.Close()

	var states []*FileState
	for rows.Next() {
		var state FileState
		var hash, errorMsg sql.NullString
		var lastSync sql.NullInt64

		err := rows.Scan(
			&state.ID,
			&state.JobID,
			&state.LocalPath,
			&state.RemotePath,
			&state.Size,
			&state.MTime,
			&hash,
			&lastSync,
			&state.SyncStatus,
			&errorMsg,
			&state.CreatedAt,
			&state.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan file state: %w", err)
		}

		// Convert sql.Null* types
		state.Hash = hash.String // Empty string if NULL
		if lastSync.Valid {
			state.LastSync = &lastSync.Int64
		}
		if errorMsg.Valid {
			state.ErrorMessage = &errorMsg.String
		}

		states = append(states, &state)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate file states: %w", err)
	}

	return states, nil
}

// DeleteFileState deletes a file state (for deleted files)
func (db *DB) DeleteFileState(jobID int64, localPath string) error {
	_, err := db.conn.Exec(`
		DELETE FROM files_state
		WHERE job_id = ? AND local_path = ?
	`, jobID, localPath)
	if err != nil {
		return fmt.Errorf("delete file state: %w", err)
	}
	return nil
}

// ClearFilesState removes all file state entries for a job (used for testing)
func (db *DB) ClearFilesState(jobID int64) error {
	_, err := db.conn.Exec(`DELETE FROM files_state WHERE job_id = ?`, jobID)
	if err != nil {
		return fmt.Errorf("clear files state: %w", err)
	}
	return nil
}
