// Package smb provides file operations for the SMB client.
package smb

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

// RemoteFileInfo contains metadata about a remote file or directory
type RemoteFileInfo struct {
	Name    string    // File or directory name
	Path    string    // Full path relative to share root
	Size    int64     // Size in bytes (0 for directories)
	ModTime time.Time // Last modification time
	IsDir   bool      // True if this is a directory
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

// ReadFile reads a file from the SMB share and returns its content.
// remotePath is relative to the share root (e.g., ".anemone/manifest.json")
// Returns the file content as bytes or an error.
func (c *SMBClient) ReadFile(remotePath string) ([]byte, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("reading remote file",
		zap.String("remote", remotePath))

	// Open remote file for reading
	remoteFile, err := fs.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	// Read all content
	data, err := io.ReadAll(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file %s: %w", remotePath, err)
	}

	c.logger.Debug("file read successfully",
		zap.String("remote", remotePath),
		zap.Int("bytes", len(data)))

	return data, nil
}

// OpenFile opens a remote file and returns an io.ReadCloser for streaming reads.
// The caller is responsible for closing the reader.
// remotePath is relative to the share root (e.g., "folder/file.txt")
func (c *SMBClient) OpenFile(remotePath string) (io.ReadCloser, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("not connected to SMB server")
	}
	fs := c.fs
	c.mu.RUnlock()

	c.logger.Debug("opening remote file for streaming",
		zap.String("remote", remotePath))

	// Open remote file for reading
	remoteFile, err := fs.Open(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}

	return remoteFile, nil
}

// UploadTempSuffix is the suffix used for temporary upload files (atomic upload)
const UploadTempSuffix = ".anemone-uploading"

// Upload uploads a file from local filesystem to the SMB share
// localPath is the absolute local path to the file
// remotePath is relative to the share root (e.g., "folder/file.txt")
// Uses atomic upload: writes to .anemone-uploading file first, then renames
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

	// Check if it's a directory (we can't upload directories this way)
	// Note: Cloud Files placeholders are reparse points but can be read normally
	if localInfo.IsDir() {
		return fmt.Errorf("cannot upload directory: %s", localPath)
	}

	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		// Try to create directory (ignore error if already exists)
		_ = fs.MkdirAll(remoteDir, 0755)
	}

	// Use atomic upload: write to temp file first, then rename
	tempPath := remotePath + UploadTempSuffix

	// Create temp remote file
	remoteFile, err := fs.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", tempPath, err)
	}

	// Copy data from local to remote
	written, err := io.Copy(remoteFile, localFile)
	remoteFile.Close() // Close before rename

	if err != nil {
		// Try to clean up incomplete temp file (may fail if connection lost)
		fs.Remove(tempPath)
		return fmt.Errorf("failed to copy data: %w", err)
	}

	// Remove existing file if present (rename won't overwrite on SMB)
	fs.Remove(remotePath)

	// Rename temp file to final name (atomic operation)
	if err := fs.Rename(tempPath, remotePath); err != nil {
		// Try to clean up temp file
		fs.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file to %s: %w", remotePath, err)
	}

	c.logger.Info("file uploaded successfully",
		zap.String("local", localPath),
		zap.String("remote", remotePath),
		zap.Int64("bytes", written),
		zap.Int64("size", localInfo.Size()))

	return nil
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
