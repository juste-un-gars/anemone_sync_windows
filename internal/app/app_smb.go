package app

import (
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// --- SMB Connections Management ---

// GetSMBConnections returns all configured SMB connections.
func (a *App) GetSMBConnections() []*SMBConnection {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.smbConnections
}

// GetSMBConnection returns an SMB connection by ID.
func (a *App) GetSMBConnection(id int64) *SMBConnection {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, conn := range a.smbConnections {
		if conn.ID == id {
			return conn
		}
	}
	return nil
}

// AddSMBConnection adds a new SMB connection.
func (a *App) AddSMBConnection(conn *SMBConnection) error {
	// Convert to DB server and save
	dbServer := convertAppConnectionToDBServer(conn)

	if a.db != nil {
		if err := a.db.CreateSMBServer(dbServer); err != nil {
			return err
		}
		conn.ID = dbServer.ID
		conn.CredentialID = dbServer.CredentialID
	}

	a.mu.Lock()
	a.smbConnections = append(a.smbConnections, conn)
	a.mu.Unlock()

	a.logger.Info("Added SMB connection", zap.String("name", conn.Name), zap.Int64("id", conn.ID))
	return nil
}

// UpdateSMBConnection updates an existing SMB connection.
func (a *App) UpdateSMBConnection(conn *SMBConnection) error {
	// Persist to database
	if a.db != nil {
		dbServer := convertAppConnectionToDBServer(conn)
		if err := a.db.UpdateSMBServer(dbServer); err != nil {
			return err
		}
		conn.CredentialID = dbServer.CredentialID
	}

	a.mu.Lock()
	for i, c := range a.smbConnections {
		if c.ID == conn.ID {
			a.smbConnections[i] = conn
			a.mu.Unlock()
			a.logger.Info("Updated SMB connection", zap.String("name", conn.Name), zap.Int64("id", conn.ID))
			return nil
		}
	}
	a.mu.Unlock()

	return errSMBConnectionNotFound
}

// DeleteSMBConnection removes an SMB connection.
func (a *App) DeleteSMBConnection(id int64) error {
	// Get connection to delete credentials from keyring
	var conn *SMBConnection
	a.mu.RLock()
	for _, c := range a.smbConnections {
		if c.ID == id {
			conn = c
			break
		}
	}
	a.mu.RUnlock()

	if conn == nil {
		return errSMBConnectionNotFound
	}

	// Delete credentials from keyring
	if a.credMgr != nil {
		if err := a.credMgr.Delete(conn.Host); err != nil {
			a.logger.Warn("Failed to delete credentials from keyring", zap.Error(err))
		}
	}

	// Delete from database
	if a.db != nil {
		if err := a.db.DeleteSMBServer(id); err != nil {
			return err
		}
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i, c := range a.smbConnections {
		if c.ID == id {
			a.smbConnections = append(a.smbConnections[:i], a.smbConnections[i+1:]...)
			a.logger.Info("Deleted SMB connection", zap.Int64("id", id))
			return nil
		}
	}

	return errSMBConnectionNotFound
}

// SaveSMBCredential saves SMB credentials to the keyring.
func (a *App) SaveSMBCredential(host, username, password, domain string, port int) error {
	if a.credMgr == nil {
		return nil
	}

	creds := &smb.Credentials{
		Server:   host,
		Username: username,
		Password: password,
		Domain:   domain,
		Port:     port,
	}
	return a.credMgr.Save(creds)
}

// LoadSMBCredential loads SMB credentials from the keyring.
func (a *App) LoadSMBCredential(host string) (*smb.Credentials, error) {
	if a.credMgr == nil {
		return nil, nil
	}
	return a.credMgr.Load(host)
}

// TestSMBConnection tests an SMB connection by listing shares.
func (a *App) TestSMBConnection(host, username, password, domain string, port int) error {
	_, err := smb.ListSharesOnServer(host, port, username, password, domain, a.logger.Named("smb-test"))
	return err
}

// ListSMBShares lists available shares on an SMB server.
func (a *App) ListSMBShares(host, username, password, domain string, port int) ([]string, error) {
	return smb.ListSharesOnServer(host, port, username, password, domain, a.logger.Named("smb-browse"))
}

// ListSMBSharesFromConnection lists shares using a saved SMB connection.
func (a *App) ListSMBSharesFromConnection(connID int64) ([]string, error) {
	conn := a.GetSMBConnection(connID)
	if conn == nil {
		return nil, errSMBConnectionNotFound
	}

	// Load credentials from keyring
	creds, err := a.LoadSMBCredential(conn.Host)
	if err != nil {
		return nil, err
	}
	if creds == nil {
		return nil, &appError{msg: "credentials not found for this connection"}
	}

	return smb.ListSharesOnServer(conn.Host, conn.Port, creds.Username, creds.Password, creds.Domain, a.logger.Named("smb-browse"))
}

// ListRemoteFolders lists folders in a path on an SMB share.
func (a *App) ListRemoteFolders(connID int64, share, remotePath string) ([]string, error) {
	conn := a.GetSMBConnection(connID)
	if conn == nil {
		return nil, errSMBConnectionNotFound
	}

	// Load credentials from keyring
	creds, err := a.LoadSMBCredential(conn.Host)
	if err != nil {
		return nil, err
	}
	if creds == nil {
		return nil, &appError{msg: "credentials not found for this connection"}
	}

	// Create SMB client config
	cfg := &smb.ClientConfig{
		Server:   conn.Host,
		Share:    share,
		Port:     conn.Port,
		Username: creds.Username,
		Password: creds.Password,
		Domain:   creds.Domain,
	}

	// Create SMB client and connect
	client, err := smb.NewSMBClient(cfg, a.logger.Named("smb-browse"))
	if err != nil {
		return nil, err
	}
	if err := client.Connect(); err != nil {
		return nil, err
	}
	defer client.Disconnect()

	// List directory contents
	entries, err := client.ListRemote(remotePath)
	if err != nil {
		return nil, err
	}

	// Filter to only directories
	folders := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir {
			folders = append(folders, entry.Name)
		}
	}

	return folders, nil
}
