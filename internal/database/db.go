package database

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

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
		&state.Hash,
		&state.LastSync,
		&state.SyncStatus,
		&state.ErrorMessage,
		&state.CreatedAt,
		&state.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("file state not found for job %d path %s", jobID, localPath)
	}
	if err != nil {
		return nil, fmt.Errorf("query file state: %w", err)
	}

	return &state, nil
}

// UpsertFileState inserts or updates a file state
func (db *DB) UpsertFileState(state *FileState) error {
	_, err := db.conn.Exec(`
		INSERT INTO files_state (job_id, local_path, remote_path, size, mtime, hash, sync_status)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(job_id, local_path)
		DO UPDATE SET
			remote_path = excluded.remote_path,
			size = excluded.size,
			mtime = excluded.mtime,
			hash = excluded.hash,
			sync_status = excluded.sync_status,
			updated_at = CURRENT_TIMESTAMP
	`, state.JobID, state.LocalPath, state.RemotePath, state.Size, state.MTime, state.Hash, state.SyncStatus)

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
		stmt, err := tx.Prepare(`
			INSERT INTO files_state (job_id, local_path, remote_path, size, mtime, hash, sync_status)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(job_id, local_path)
			DO UPDATE SET
				remote_path = excluded.remote_path,
				size = excluded.size,
				mtime = excluded.mtime,
				hash = excluded.hash,
				sync_status = excluded.sync_status,
				updated_at = CURRENT_TIMESTAMP
		`)
		if err != nil {
			return fmt.Errorf("prepare statement: %w", err)
		}
		defer stmt.Close()

		for _, state := range states {
			_, err := stmt.Exec(state.JobID, state.LocalPath, state.RemotePath, state.Size, state.MTime, state.Hash, state.SyncStatus)
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
		err := rows.Scan(
			&state.ID,
			&state.JobID,
			&state.LocalPath,
			&state.RemotePath,
			&state.Size,
			&state.MTime,
			&state.Hash,
			&state.LastSync,
			&state.SyncStatus,
			&state.ErrorMessage,
			&state.CreatedAt,
			&state.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan file state: %w", err)
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
