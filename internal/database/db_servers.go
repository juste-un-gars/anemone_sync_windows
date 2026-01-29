package database

import (
	"database/sql"
	"fmt"
	"time"
)

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
