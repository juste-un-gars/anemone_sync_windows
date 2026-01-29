package database

import (
	"database/sql"
	"fmt"
	"time"
)

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
