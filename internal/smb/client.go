package smb

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

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
			c.logger.Warn("failed to close connection", zap.Error(err))
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

// Download downloads a file from the SMB share to local filesystem
// remotePath is relative to the share root (e.g., "folder/file.txt")
// localPath is the absolute local path where the file will be saved
func (c *SMBClient) Download(remotePath, localPath string) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("downloading file",
		zap.String("remote", remotePath),
		zap.String("local", localPath))

	// Open remote file for reading
	remoteFile, err := fs.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	// Get remote file info for logging
	remoteInfo, err := remoteFile.Stat()
	if err != nil {
		c.logger.Warn("failed to get remote file info", zap.Error(err))
	}

	// Create local directory if needed
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory %s: %w", localDir, err)
	}

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	// Copy data from remote to local
	written, err := io.Copy(localFile, remoteFile)
	if err != nil {
		// Try to clean up incomplete file
		os.Remove(localPath)
		return fmt.Errorf("failed to copy data: %w", err)
	}

	c.logger.Info("file downloaded successfully",
		zap.String("remote", remotePath),
		zap.String("local", localPath),
		zap.Int64("bytes", written),
		zap.Int64("size", remoteInfo.Size()))

	return nil
}

// Upload uploads a file from local filesystem to the SMB share
// localPath is the absolute local path to the file
// remotePath is relative to the share root (e.g., "folder/file.txt")
func (c *SMBClient) Upload(localPath, remotePath string) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("uploading file",
		zap.String("local", localPath),
		zap.String("remote", remotePath))

	// Open local file for reading
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	// Get local file info
	localInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get local file info: %w", err)
	}

	// Check if it's a regular file
	if !localInfo.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", localPath)
	}

	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		// Try to create directory (ignore error if already exists)
		_ = fs.MkdirAll(remoteDir, 0755)
	}

	// Create remote file
	remoteFile, err := fs.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	// Copy data from local to remote
	written, err := io.Copy(remoteFile, localFile)
	if err != nil {
		// Try to clean up incomplete remote file
		fs.Remove(remotePath)
		return fmt.Errorf("failed to copy data: %w", err)
	}

	c.logger.Info("file uploaded successfully",
		zap.String("local", localPath),
		zap.String("remote", remotePath),
		zap.Int64("bytes", written),
		zap.Int64("size", localInfo.Size()))

	return nil
}

// RemoteFileInfo contains metadata about a remote file or directory
type RemoteFileInfo struct {
	Name    string    // File or directory name
	Path    string    // Full path relative to share root
	Size    int64     // Size in bytes (0 for directories)
	ModTime time.Time // Last modification time
	IsDir   bool      // True if this is a directory
}

// ListRemote lists files and directories in the specified remote path
// remotePath is relative to the share root (e.g., "folder" or "" for root)
// Returns a slice of RemoteFileInfo for all entries in the directory
func (c *SMBClient) ListRemote(remotePath string) ([]RemoteFileInfo, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("listing remote directory",
		zap.String("remote", remotePath))

	// Use "." for root if path is empty
	if remotePath == "" {
		remotePath = "."
	}

	// Read directory entries
	entries, err := fs.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory %s: %w", remotePath, err)
	}

	// Convert to RemoteFileInfo slice
	result := make([]RemoteFileInfo, 0, len(entries))
	for _, info := range entries {
		// Build full path
		fullPath := remotePath
		if remotePath == "." {
			fullPath = info.Name()
		} else {
			fullPath = filepath.Join(remotePath, info.Name())
		}

		result = append(result, RemoteFileInfo{
			Name:    info.Name(),
			Path:    fullPath,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
		})
	}

	c.logger.Info("remote directory listed successfully",
		zap.String("remote", remotePath),
		zap.Int("count", len(result)))

	return result, nil
}

// GetMetadata retrieves metadata for a specific remote file or directory
// remotePath is relative to the share root (e.g., "folder/file.txt")
// Returns RemoteFileInfo with metadata about the file/directory
func (c *SMBClient) GetMetadata(remotePath string) (*RemoteFileInfo, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("getting remote file metadata",
		zap.String("remote", remotePath))

	// Stat the file/directory
	info, err := fs.Stat(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata for %s: %w", remotePath, err)
	}

	result := &RemoteFileInfo{
		Name:    info.Name(),
		Path:    remotePath,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}

	c.logger.Debug("metadata retrieved successfully",
		zap.String("remote", remotePath),
		zap.Int64("size", result.Size),
		zap.Bool("isDir", result.IsDir))

	return result, nil
}

// Delete removes a file from the remote SMB share
// remotePath is relative to the share root (e.g., "folder/file.txt")
// Note: This only removes files, not directories (use RemoveAll for directories)
func (c *SMBClient) Delete(remotePath string) error {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("deleting remote file",
		zap.String("remote", remotePath))

	// Remove the file
	if err := fs.Remove(remotePath); err != nil {
		return fmt.Errorf("failed to delete %s: %w", remotePath, err)
	}

	c.logger.Info("remote file deleted successfully",
		zap.String("remote", remotePath))

	return nil
}
