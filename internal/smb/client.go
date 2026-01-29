package smb

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/hirochachacha/go-smb2"
	"go.uber.org/zap"
)

// SMBClient handles SMB connections and file operations
type SMBClient struct {
	// Connection details
	server string // Server address (e.g., "192.168.1.100" or "server.local")
	share  string // Share name (e.g., "documents")
	port   int    // SMB port (default: 445)

	// Credentials
	username string
	password string
	domain   string

	// SMB2 objects
	conn    net.Conn
	dialer  *smb2.Dialer
	session *smb2.Session
	fs      *smb2.Share

	// State
	mu        sync.RWMutex
	connected bool

	// Logger
	logger *zap.Logger
}

// ClientConfig contains configuration for creating an SMB client
type ClientConfig struct {
	Server   string // Server address
	Share    string // Share name
	Port     int    // SMB port (0 = default 445)
	Username string
	Password string
	Domain   string // Optional domain
}

// NewSMBClient creates a new SMB client instance
func NewSMBClient(cfg *ClientConfig, logger *zap.Logger) (*SMBClient, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if cfg.Server == "" {
		return nil, fmt.Errorf("server cannot be empty")
	}
	if cfg.Share == "" {
		return nil, fmt.Errorf("share cannot be empty")
	}
	if cfg.Username == "" {
		return nil, fmt.Errorf("username cannot be empty")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	port := cfg.Port
	if port == 0 {
		port = 445 // Default SMB port
	}

	return &SMBClient{
		server:   cfg.Server,
		share:    cfg.Share,
		port:     port,
		username: cfg.Username,
		password: cfg.Password,
		domain:   cfg.Domain,
		logger:   logger.With(zap.String("component", "smb")),
	}, nil
}

// Connect establishes a connection to the SMB server
func (c *SMBClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("already connected")
	}

	c.logger.Info("connecting to SMB server",
		zap.String("server", c.server),
		zap.String("share", c.share),
		zap.Int("port", c.port))

	// Connect to server
	addr := fmt.Sprintf("%s:%d", c.server, c.port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	c.conn = conn

	// Create SMB2 dialer
	c.dialer = &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     c.username,
			Password: c.password,
			Domain:   c.domain,
		},
	}

	// Start SMB2 session
	session, err := c.dialer.Dial(conn)
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to create SMB session: %w", err)
	}
	c.session = session

	// Mount share
	fs, err := session.Mount(c.share)
	if err != nil {
		c.session.Logoff()
		c.conn.Close()
		return fmt.Errorf("failed to mount share %s: %w", c.share, err)
	}
	c.fs = fs

	c.connected = true

	c.logger.Info("successfully connected to SMB server",
		zap.String("server", c.server),
		zap.String("share", c.share))

	return nil
}

// Disconnect closes the SMB connection
func (c *SMBClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil // Already disconnected
	}

	c.logger.Info("disconnecting from SMB server",
		zap.String("server", c.server))

	// Unmount share
	if c.fs != nil {
		if err := c.fs.Umount(); err != nil {
			c.logger.Warn("failed to unmount share", zap.Error(err))
		}
		c.fs = nil
	}

	// Logoff session
	if c.session != nil {
		if err := c.session.Logoff(); err != nil {
			c.logger.Warn("failed to logoff session", zap.Error(err))
		}
		c.session = nil
	}

	// Close connection
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			// Ignore "use of closed network connection" - happens after long transfers
			// when server has already closed the connection (timeout)
			if !isClosedConnectionError(err) {
				c.logger.Warn("failed to close connection", zap.Error(err))
			}
		}
		c.conn = nil
	}

	c.connected = false
	c.dialer = nil

	c.logger.Info("disconnected from SMB server")

	return nil
}

// IsConnected returns true if the client is currently connected
func (c *SMBClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// GetServer returns the server address
func (c *SMBClient) GetServer() string {
	return c.server
}

// GetShare returns the share name
func (c *SMBClient) GetShare() string {
	return c.share
}

// NewSMBClientFromKeyring creates a new SMB client using credentials from the system keyring
// server is used to identify the credentials in the keyring
func NewSMBClientFromKeyring(server, share string, logger *zap.Logger) (*SMBClient, error) {
	if server == "" {
		return nil, fmt.Errorf("server cannot be empty")
	}
	if share == "" {
		return nil, fmt.Errorf("share cannot be empty")
	}

	// Create credential manager
	credMgr := NewCredentialManager(logger)

	// Load credentials from keyring (keyed by server only)
	creds, err := credMgr.Load(server)
	if err != nil {
		return nil, fmt.Errorf("failed to load credentials from keyring: %w", err)
	}

	// Create client config from credentials
	cfg := &ClientConfig{
		Server:   creds.Server,
		Share:    share,
		Port:     creds.Port,
		Username: creds.Username,
		Password: creds.Password,
		Domain:   creds.Domain,
	}

	// Create and return client
	return NewSMBClient(cfg, logger)
}

// SaveCredentialsToKeyring saves the client's credentials to the system keyring
// This allows the credentials to be reused later without storing them in config files
func (c *SMBClient) SaveCredentialsToKeyring() error {
	// Create credential manager
	credMgr := NewCredentialManager(c.logger)

	// Create credentials structure (without share - keyed by server only)
	creds := &Credentials{
		Server:   c.server,
		Port:     c.port,
		Username: c.username,
		Password: c.password,
		Domain:   c.domain,
	}

	// Save to keyring
	if err := credMgr.Save(creds); err != nil {
		return fmt.Errorf("failed to save credentials to keyring: %w", err)
	}

	return nil
}

// DeleteCredentialsFromKeyring removes the client's credentials from the system keyring
func (c *SMBClient) DeleteCredentialsFromKeyring() error {
	// Create credential manager
	credMgr := NewCredentialManager(c.logger)

	// Delete from keyring (keyed by server only)
	if err := credMgr.Delete(c.server); err != nil {
		return fmt.Errorf("failed to delete credentials from keyring: %w", err)
	}

	return nil
}

// isClosedConnectionError checks if the error indicates the connection was already closed.
// This is benign during disconnect - happens when server closes connection first (e.g., timeout).
func isClosedConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		strings.Contains(errStr, "connection reset by peer") ||
		strings.Contains(errStr, "broken pipe")
}

// ListSharesOnServer connects to a server and lists available shares
// This is a utility function that doesn't require a full client
func ListSharesOnServer(server string, port int, username, password, domain string, logger *zap.Logger) ([]string, error) {
	if server == "" {
		return nil, fmt.Errorf("server cannot be empty")
	}
	if port == 0 {
		port = 445
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	logger.Debug("listing shares on server",
		zap.String("server", server),
		zap.Int("port", port))

	// Connect to server
	addr := fmt.Sprintf("%s:%d", server, port)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", addr, err)
	}
	defer conn.Close()

	// Create SMB2 dialer
	dialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     username,
			Password: password,
			Domain:   domain,
		},
	}

	// Start SMB2 session
	session, err := dialer.Dial(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create SMB session: %w", err)
	}
	defer session.Logoff()

	// List shares
	shares, err := session.ListSharenames()
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}

	logger.Debug("found shares",
		zap.String("server", server),
		zap.Strings("shares", shares))

	return shares, nil
}
