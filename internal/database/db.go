// Package database provides SQLite persistence with SQLCipher encryption.
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

// DB represents the database connection.
type DB struct {
	conn *sql.DB
	path string
}

// Config contains database configuration.
type Config struct {
	Path             string
	EncryptionKey    string // SQLCipher encryption key (from keystore)
	CreateIfNotExist bool
}

// Open opens or creates an encrypted SQLite database.
func Open(cfg Config) (*DB, error) {
	// Create parent directory if needed
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Check if database exists
	dbExists := fileExists(cfg.Path)

	// Connection string with SQLCipher
	connStr := fmt.Sprintf("file:%s?_pragma_key=%s&_pragma_cipher_page_size=4096",
		cfg.Path, cfg.EncryptionKey)

	// Open connection
	conn, err := sql.Open("sqlite3", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db := &DB{
		conn: conn,
		path: cfg.Path,
	}

	// Initialize schema if new database
	if !dbExists || cfg.CreateIfNotExist {
		if err := db.initSchema(); err != nil {
			db.Close()
			return nil, fmt.Errorf("failed to initialize schema: %w", err)
		}
	}

	// Check schema version
	if err := db.checkSchemaVersion(); err != nil {
		db.Close()
		return nil, fmt.Errorf("schema version check failed: %w", err)
	}

	// Clean up corrupted cache entries (absolute paths from bug)
	if err := db.cleanupCorruptedCacheEntries(); err != nil {
		// Log but don't fail - this is a cleanup operation
		fmt.Printf("Warning: failed to cleanup corrupted cache entries: %v\n", err)
	}

	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// Conn returns the underlying SQL connection.
func (db *DB) Conn() *sql.DB {
	return db.conn
}

// initSchema initializes the database schema.
func (db *DB) initSchema() error {
	if _, err := db.conn.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema SQL: %w", err)
	}
	return nil
}

// checkSchemaVersion verifies the database schema version.
func (db *DB) checkSchemaVersion() error {
	var version string
	err := db.conn.QueryRow("SELECT value FROM db_metadata WHERE key = 'schema_version'").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to read schema version: %w", err)
	}

	// TODO: Implement schema migration if needed
	if version == "" {
		return fmt.Errorf("invalid schema version")
	}

	return nil
}

// cleanupCorruptedCacheEntries removes files_state entries with absolute Windows paths.
// This fixes a bug where paths like "D:\data\file.txt" were stored instead of "data/file.txt".
func (db *DB) cleanupCorruptedCacheEntries() error {
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

// fileExists checks if a file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// Transaction executes a function within a transaction.
func (db *DB) Transaction(fn func(*sql.Tx) error) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("transaction error: %v, rollback error: %w", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// HealthCheck verifies the database is accessible.
func (db *DB) HealthCheck() error {
	return db.conn.Ping()
}

// GetMetadata retrieves a metadata value from the database.
func (db *DB) GetMetadata(key string) (string, error) {
	var value string
	err := db.conn.QueryRow("SELECT value FROM db_metadata WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("metadata '%s' not found", key)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read metadata: %w", err)
	}
	return value, nil
}

// SetMetadata sets a metadata value in the database.
func (db *DB) SetMetadata(key, value string) error {
	_, err := db.conn.Exec(`
		INSERT INTO db_metadata (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	if err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	return nil
}
