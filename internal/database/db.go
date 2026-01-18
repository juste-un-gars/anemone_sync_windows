package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mutecomm/go-sqlcipher/v4"
)

//go:embed schema.sql
var schemaSQL string

// DB représente la connexion à la base de données
type DB struct {
	conn *sql.DB
	path string
}

// Config contient la configuration de la base de données
type Config struct {
	Path           string
	EncryptionKey  string // Clé de chiffrement SQLCipher (récupérée depuis keystore)
	CreateIfNotExist bool
}

// Open ouvre ou crée la base de données SQLite chiffrée
func Open(cfg Config) (*DB, error) {
	// Créer le répertoire parent si nécessaire
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("impossible de créer le répertoire de la base de données: %w", err)
	}

	// Vérifier si la base de données existe
	dbExists := fileExists(cfg.Path)

	// Connection string avec SQLCipher
	// Format: file:path?_pragma_key=ENCRYPTION_KEY&_pragma_cipher_page_size=4096
	connStr := fmt.Sprintf("file:%s?_pragma_key=%s&_pragma_cipher_page_size=4096",
		cfg.Path, cfg.EncryptionKey)

	// Ouvrir la connexion
	conn, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("erreur ouverture base de données: %w", err)
	}

	// Tester la connexion
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("impossible de se connecter à la base de données: %w", err)
	}

	db := &DB{
		conn: conn,
		path: cfg.Path,
	}

	// Si la base de données n'existe pas ou est nouvelle, initialiser le schéma
	if !dbExists || cfg.CreateIfNotExist {
		if err := db.initSchema(); err != nil {
			db.Close()
			return nil, fmt.Errorf("erreur initialisation schéma: %w", err)
		}
	}

	// Vérifier la version du schéma
	if err := db.checkSchemaVersion(); err != nil {
		db.Close()
		return nil, fmt.Errorf("erreur vérification version schéma: %w", err)
	}

	// Clean up corrupted cache entries (absolute paths from bug)
	if err := db.cleanupCorruptedCacheEntries(); err != nil {
		// Log but don't fail - this is a cleanup operation
		fmt.Printf("Warning: failed to cleanup corrupted cache entries: %v\n", err)
	}

	return db, nil
}

// Close ferme la connexion à la base de données
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Conn retourne la connexion SQL sous-jacente
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// initSchema initialise le schéma de la base de données
func (db *DB) initSchema() error {
	// Exécuter le script SQL de création du schéma
	if _, err := db.conn.Exec(schemaSQL); err != nil {
		return fmt.Errorf("erreur exécution schéma SQL: %w", err)
	}

	return nil
}

// checkSchemaVersion vérifie la version du schéma de la base de données
func (db *DB) checkSchemaVersion() error {
	var version string
	err := db.conn.QueryRow("SELECT value FROM db_metadata WHERE key = 'schema_version'").Scan(&version)
	if err != nil {
		return fmt.Errorf("impossible de lire la version du schéma: %w", err)
	}

	// TODO: Implémenter la migration de schéma si nécessaire
	// Pour l'instant, nous vérifions juste que la version existe
	if version == "" {
		return fmt.Errorf("version du schéma invalide")
	}

	return nil
}

// cleanupCorruptedCacheEntries removes files_state entries with absolute Windows paths.
// This fixes a bug where paths like "D:\data\file.txt" were stored instead of "data/file.txt".
func (db *DB) cleanupCorruptedCacheEntries() error {
	// Delete entries where local_path contains Windows drive letter (e.g., "C:\", "D:\")
	// These are corrupted entries from a previous bug
	result, err := db.conn.Exec(`
		DELETE FROM files_state
		WHERE local_path LIKE '%:\%'
		   OR local_path LIKE '%:/%'
	`)
	if err != nil {
		return fmt.Errorf("failed to cleanup corrupted cache entries: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("Cleaned up %d corrupted cache entries with absolute paths\n", rowsAffected)
	}

	return nil
}

// fileExists vérifie si un fichier existe
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Transaction exécute une fonction dans une transaction
func (db *DB) Transaction(fn func(*sql.Tx) error) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("impossible de démarrer la transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("erreur transaction: %v, erreur rollback: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("impossible de valider la transaction: %w", err)
	}

	return nil
}

// HealthCheck vérifie que la base de données est accessible
func (db *DB) HealthCheck() error {
	return db.conn.Ping()
}

// GetMetadata récupère une métadonnée de la base de données
func (db *DB) GetMetadata(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM db_metadata WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("métadonnée '%s' non trouvée", key)
	}
	if err != nil {
		return "", fmt.Errorf("erreur lecture métadonnée: %w", err)
	}
	return value, nil
}

// SetMetadata définit une métadonnée dans la base de données
func (db *DB) SetMetadata(key, value string) error {
	_, err := db.conn.Exec(`
		INSERT INTO db_metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("erreur écriture métadonnée: %w", err)
	}
	return nil
}

// --- Scanner-specific methods ---

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
	_, err := db.conn.Exec(`
		INSERT INTO files_state (job_id, local_path, remote_path, size, mtime, hash, sync_status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id, local_path)
		DO UPDATE SET
			remote_path = excluded.remote_path,
			size = excluded.size,
			mtime = excluded.mtime,
			hash = excluded.hash,
			sync_status = excluded.sync_status,
			updated_at = excluded.updated_at
	`, state.JobID, state.LocalPath, state.RemotePath, state.Size, state.MTime, state.Hash, state.SyncStatus, now, now)

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
			INSERT INTO files_state (job_id, local_path, remote_path, size, mtime, hash, sync_status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(job_id, local_path)
			DO UPDATE SET
				remote_path = excluded.remote_path,
				size = excluded.size,
				mtime = excluded.mtime,
				hash = excluded.hash,
				sync_status = excluded.sync_status,
				updated_at = excluded.updated_at
		`)
		if err != nil {
			return fmt.Errorf("prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, state := range states {
			_, err := stmt.Exec(state.JobID, state.LocalPath, state.RemotePath, state.Size, state.MTime, state.Hash, state.SyncStatus, now, now)
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

// --- Sync Jobs CRUD ---

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

// --- App Config ---

// GetAppConfig retrieves an app config value
func (db *DB) GetAppConfig(key string) (string, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM app_config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Return empty string if not found
	}
	if err != nil {
		return "", fmt.Errorf("get app config: %w", err)
	}
	return value, nil
}

// SetAppConfig sets an app config value
func (db *DB) SetAppConfig(key, value, valueType string) error {
	now := time.Now().Unix()
	_, err := db.conn.Exec(`
		INSERT INTO app_config (key, value, value_type, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET
			value = excluded.value,
			value_type = excluded.value_type,
			updated_at = excluded.updated_at
	`, key, value, valueType, now)
	if err != nil {
		return fmt.Errorf("set app config: %w", err)
	}
	return nil
}

// GetAllAppConfig retrieves all app config values
func (db *DB) GetAllAppConfig() (map[string]string, error) {
	rows, err := db.conn.Query(`SELECT key, value FROM app_config`)
	if err != nil {
		return nil, fmt.Errorf("query app config: %w", err)
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan app config: %w", err)
		}
		config[key] = value
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate app config: %w", err)
	}

	return config, nil
}

// --- SMB Servers CRUD ---

// GetAllSMBServers retrieves all SMB server configurations
func (db *DB) GetAllSMBServers() ([]*SMBServer, error) {
	rows, err := db.conn.Query(`
		SELECT id, name, host, port, username, domain, credential_id,
			   smb_version, last_connection_test, last_connection_status,
			   created_at, updated_at
		FROM smb_servers
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query smb servers: %w", err)
	}
	defer rows.Close()

	var servers []*SMBServer
	for rows.Next() {
		var s SMBServer
		var domain, smbVersion, connStatus sql.NullString
		var lastConnTest sql.NullInt64
		var createdAt, updatedAt int64

		err := rows.Scan(
			&s.ID, &s.Name, &s.Host, &s.Port, &s.Username,
			&domain, &s.CredentialID, &smbVersion, &lastConnTest, &connStatus,
			&createdAt, &updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan smb server: %w", err)
		}

		if domain.Valid {
			s.Domain = domain.String
		}
		if smbVersion.Valid {
			s.SMBVersion = smbVersion.String
		}
		if connStatus.Valid {
			s.LastConnectionStatus = connStatus.String
		}
		if lastConnTest.Valid {
			t := time.Unix(lastConnTest.Int64, 0)
			s.LastConnectionTest = &t
		}
		s.CreatedAt = time.Unix(createdAt, 0)
		s.UpdatedAt = time.Unix(updatedAt, 0)

		servers = append(servers, &s)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate smb servers: %w", err)
	}

	return servers, nil
}

// GetSMBServer retrieves a single SMB server by ID
func (db *DB) GetSMBServer(id int64) (*SMBServer, error) {
	var s SMBServer
	var domain, smbVersion, connStatus sql.NullString
	var lastConnTest sql.NullInt64
	var createdAt, updatedAt int64

	err := db.conn.QueryRow(`
		SELECT id, name, host, port, username, domain, credential_id,
			   smb_version, last_connection_test, last_connection_status,
			   created_at, updated_at
		FROM smb_servers
		WHERE id = ?
	`, id).Scan(
		&s.ID, &s.Name, &s.Host, &s.Port, &s.Username,
		&domain, &s.CredentialID, &smbVersion, &lastConnTest, &connStatus,
		&createdAt, &updatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get smb server: %w", err)
	}

	if domain.Valid {
		s.Domain = domain.String
	}
	if smbVersion.Valid {
		s.SMBVersion = smbVersion.String
	}
	if connStatus.Valid {
		s.LastConnectionStatus = connStatus.String
	}
	if lastConnTest.Valid {
		t := time.Unix(lastConnTest.Int64, 0)
		s.LastConnectionTest = &t
	}
	s.CreatedAt = time.Unix(createdAt, 0)
	s.UpdatedAt = time.Unix(updatedAt, 0)

	return &s, nil
}

// CreateSMBServer creates a new SMB server configuration
func (db *DB) CreateSMBServer(server *SMBServer) error {
	now := time.Now().Unix()

	// Generate credential_id from host only
	server.CredentialID = server.Host

	result, err := db.conn.Exec(`
		INSERT INTO smb_servers (
			name, host, port, username, domain, credential_id,
			smb_version, created_at, updated_at
		) VALUES (?, ?, ?, ?, NULLIF(?, ''), ?, NULLIF(?, ''), ?, ?)
	`,
		server.Name, server.Host, server.Port, server.Username,
		server.Domain, server.CredentialID, server.SMBVersion, now, now,
	)
	if err != nil {
		return fmt.Errorf("insert smb server: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("get last insert id: %w", err)
	}

	server.ID = id
	server.CreatedAt = time.Unix(now, 0)
	server.UpdatedAt = time.Unix(now, 0)

	return nil
}

// UpdateSMBServer updates an existing SMB server configuration
func (db *DB) UpdateSMBServer(server *SMBServer) error {
	now := time.Now().Unix()

	// Update credential_id from host only
	server.CredentialID = server.Host

	result, err := db.conn.Exec(`
		UPDATE smb_servers SET
			name = ?, host = ?, port = ?, username = ?,
			domain = NULLIF(?, ''), credential_id = ?, smb_version = NULLIF(?, ''), updated_at = ?
		WHERE id = ?
	`,
		server.Name, server.Host, server.Port, server.Username,
		server.Domain, server.CredentialID, server.SMBVersion, now, server.ID,
	)
	if err != nil {
		return fmt.Errorf("update smb server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("smb server not found: %d", server.ID)
	}

	server.UpdatedAt = time.Unix(now, 0)
	return nil
}

// DeleteSMBServer deletes an SMB server configuration by ID
func (db *DB) DeleteSMBServer(id int64) error {
	result, err := db.conn.Exec(`DELETE FROM smb_servers WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete smb server: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("smb server not found: %d", id)
	}

	return nil
}

// UpdateSMBServerConnectionStatus updates the connection test status
func (db *DB) UpdateSMBServerConnectionStatus(id int64, status string) error {
	now := time.Now().Unix()
	_, err := db.conn.Exec(`
		UPDATE smb_servers SET
			last_connection_test = ?, last_connection_status = ?, updated_at = ?
		WHERE id = ?
	`, now, status, now, id)
	if err != nil {
		return fmt.Errorf("update connection status: %w", err)
	}
	return nil
}
