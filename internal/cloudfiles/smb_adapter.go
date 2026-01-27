//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API.
package cloudfiles

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SMBClientAdapter adapts an SMB client to the DataSource interface.
type SMBClientAdapter struct {
	client    SMBFileClient
	sharePath string // Base path within the share
	logger    *zap.Logger
}

// SMBFileClient defines the SMB operations needed for cloud files.
type SMBFileClient interface {
	// OpenFile opens a remote file for streaming reads.
	OpenFile(remotePath string) (io.ReadCloser, error)
	// ReadFile reads the entire file content.
	ReadFile(remotePath string) ([]byte, error)
	// ListRemote lists files in a directory.
	ListRemote(remotePath string) ([]SMBRemoteFileInfo, error)
	// IsConnected returns true if connected to the SMB server.
	IsConnected() bool
}

// SMBRemoteFileInfo represents file info from SMB listing.
type SMBRemoteFileInfo struct {
	Name    string
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// NewSMBClientAdapter creates a new SMB client adapter.
func NewSMBClientAdapter(client SMBFileClient, sharePath string, logger *zap.Logger) *SMBClientAdapter {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SMBClientAdapter{
		client:    client,
		sharePath: strings.TrimPrefix(sharePath, "/"),
		logger:    logger,
	}
}

// GetFileReader implements DataSource.
func (a *SMBClientAdapter) GetFileReader(ctx context.Context, relativePath string, offset int64) (io.ReadCloser, error) {
	// Build full remote path
	remotePath := relativePath
	if a.sharePath != "" {
		remotePath = filepath.Join(a.sharePath, relativePath)
	}
	// Normalize to forward slashes for SMB
	remotePath = strings.ReplaceAll(remotePath, "\\", "/")

	a.logger.Debug("opening file for hydration",
		zap.String("remote_path", remotePath),
		zap.Int64("offset", offset),
	)

	// Open the file
	reader, err := a.client.OpenFile(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file: %w", err)
	}

	// Handle offset by wrapping in an offset reader
	if offset > 0 {
		return &offsetReader{
			reader: reader,
			offset: offset,
		}, nil
	}

	return reader, nil
}

// ListFiles implements DataSource.
func (a *SMBClientAdapter) ListFiles(ctx context.Context) ([]RemoteFileInfo, error) {
	a.logger.Debug("listing remote files",
		zap.String("share_path", a.sharePath),
	)

	// List files recursively
	var allFiles []RemoteFileInfo
	err := a.listRecursive(ctx, a.sharePath, &allFiles)
	if err != nil {
		return nil, err
	}

	a.logger.Info("listed remote files",
		zap.Int("count", len(allFiles)),
	)

	return allFiles, nil
}

// listRecursive lists files recursively from the given path.
func (a *SMBClientAdapter) listRecursive(ctx context.Context, path string, files *[]RemoteFileInfo) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// List current directory
	entries, err := a.client.ListRemote(path)
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", path, err)
	}

	for _, entry := range entries {
		// Skip . and ..
		if entry.Name == "." || entry.Name == ".." {
			continue
		}

		// Skip hidden/system files
		if strings.HasPrefix(entry.Name, ".") {
			continue
		}

		fullPath := entry.Path
		if path != "" && path != "." {
			fullPath = path + "/" + entry.Name
		} else {
			fullPath = entry.Name
		}

		if entry.IsDir {
			// Recurse into directory
			if err := a.listRecursive(ctx, fullPath, files); err != nil {
				return err
			}
		} else {
			// Calculate relative path
			relativePath := fullPath
			if a.sharePath != "" && strings.HasPrefix(relativePath, a.sharePath) {
				relativePath = strings.TrimPrefix(relativePath, a.sharePath)
				relativePath = strings.TrimPrefix(relativePath, "/")
			}

			*files = append(*files, RemoteFileInfo{
				Path:        relativePath,
				Size:        entry.Size,
				ModTime:     entry.ModTime,
				IsDirectory: false,
			})
		}
	}

	return nil
}

// offsetReader wraps a reader and skips to the given offset.
type offsetReader struct {
	reader       io.ReadCloser
	offset       int64
	offsetDone   bool
}

func (r *offsetReader) Read(p []byte) (n int, err error) {
	// Skip to offset on first read
	if !r.offsetDone && r.offset > 0 {
		buf := make([]byte, 8192)
		remaining := r.offset
		for remaining > 0 {
			toRead := int64(len(buf))
			if toRead > remaining {
				toRead = remaining
			}
			n, err := r.reader.Read(buf[:toRead])
			if err != nil {
				return 0, fmt.Errorf("failed to seek to offset: %w", err)
			}
			remaining -= int64(n)
		}
		r.offsetDone = true
	}

	return r.reader.Read(p)
}

func (r *offsetReader) Close() error {
	return r.reader.Close()
}

// ManifestDataSource wraps a manifest and provides file listing.
type ManifestDataSource struct {
	files       []ManifestFileEntry
	smbAdapter  *SMBClientAdapter
}

// NewManifestDataSource creates a data source from manifest entries.
func NewManifestDataSource(files []ManifestFileEntry, smbAdapter *SMBClientAdapter) *ManifestDataSource {
	return &ManifestDataSource{
		files:      files,
		smbAdapter: smbAdapter,
	}
}

// GetFileReader implements DataSource by delegating to SMB adapter.
func (m *ManifestDataSource) GetFileReader(ctx context.Context, relativePath string, offset int64) (io.ReadCloser, error) {
	if m.smbAdapter == nil {
		return nil, fmt.Errorf("no SMB adapter configured")
	}
	return m.smbAdapter.GetFileReader(ctx, relativePath, offset)
}

// ListFiles implements DataSource by returning manifest files.
func (m *ManifestDataSource) ListFiles(ctx context.Context) ([]RemoteFileInfo, error) {
	return FromManifestFiles(m.files), nil
}
