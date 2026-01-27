//go:build windows
// +build windows

// Package cloudfiles provides callback handlers and data source adapters.
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

// Callback handlers

func (p *CloudFilesProvider) handleFetchData(info *FetchDataInfo) error {
	p.mu.RLock()
	hydration := p.hydration
	p.mu.RUnlock()

	if hydration == nil {
		p.logger.Error("fetch data requested but no data source configured",
			zap.String("file", info.FilePath),
		)
		return fmt.Errorf("no data source configured")
	}

	p.logger.Debug("handling fetch data request",
		zap.String("file", info.FilePath),
		zap.Int64("offset", info.RequiredOffset),
		zap.Int64("length", info.RequiredLength),
	)

	ctx := context.Background() // TODO: Use cancellable context
	return hydration.HandleFetchData(ctx, info)
}

func (p *CloudFilesProvider) handleCancelFetch(filePath string) {
	p.logger.Info("fetch cancel requested",
		zap.String("file", filePath),
	)

	p.mu.RLock()
	hydration := p.hydration
	p.mu.RUnlock()

	if hydration != nil {
		// Extract relative path from full path
		relativePath := filePath
		localPath := p.localPath
		if len(localPath) < len(filePath) && filePath[:len(localPath)] == localPath {
			relativePath = filePath[len(localPath)+1:]
		}
		hydration.CancelHydrationByPath(relativePath)
	}
}

func (p *CloudFilesProvider) handleNotifyDelete(filePath string, isDirectory bool) bool {
	p.logger.Debug("file deleted",
		zap.String("file", filePath),
		zap.Bool("is_directory", isDirectory),
	)

	// Allow deletion
	return true
}

func (p *CloudFilesProvider) handleNotifyRename(sourcePath, targetPath string, isDirectory bool) bool {
	p.logger.Debug("file renamed",
		zap.String("source", sourcePath),
		zap.String("target", targetPath),
		zap.Bool("is_directory", isDirectory),
	)

	// Allow rename
	return true
}

// Data source adapters

// dataSourceAdapter adapts DataSource to DataProvider interface.
type dataSourceAdapter struct {
	source     DataSource
	remotePath string
}

func (a *dataSourceAdapter) GetFileReader(ctx context.Context, relativePath string, offset int64) (io.ReadCloser, error) {
	// Convert relative path to remote path
	remotePath := relativePath
	if a.remotePath != "" {
		remotePath = filepath.Join(a.remotePath, relativePath)
	}
	// Normalize to forward slashes for SMB
	remotePath = strings.ReplaceAll(remotePath, "\\", "/")

	return a.source.GetFileReader(ctx, remotePath, offset)
}

// SMBDataSource implements DataSource for SMB shares.
type SMBDataSource struct {
	client         SMBClient
	sharePath      string
	manifestReader ManifestReader
	logger         *zap.Logger
}

// SMBClient interface for SMB operations.
type SMBClient interface {
	ReadFile(path string) ([]byte, error)
	Download(remotePath, localPath string) error
	GetReader(remotePath string) (io.ReadCloser, error)
}

// ManifestReader interface for reading manifests.
type ManifestReader interface {
	ReadManifest(ctx context.Context, sharePath string) (*ManifestResult, error)
}

// ManifestResult represents the result of reading a manifest.
type ManifestResult struct {
	Files    []ManifestFileEntry
	Found    bool
	Duration time.Duration
}

// NewSMBDataSource creates a new SMB data source.
func NewSMBDataSource(client SMBClient, sharePath string, logger *zap.Logger) *SMBDataSource {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &SMBDataSource{
		client:    client,
		sharePath: sharePath,
		logger:    logger,
	}
}

// GetFileReader implements DataSource.
func (s *SMBDataSource) GetFileReader(ctx context.Context, remotePath string, offset int64) (io.ReadCloser, error) {
	// For now, use GetReader if available
	reader, err := s.client.GetReader(remotePath)
	if err != nil {
		return nil, err
	}

	// Skip to offset if needed
	if offset > 0 {
		// Read and discard bytes until offset
		// This is inefficient but works for now
		// TODO: Implement proper seeking in SMB client
		buf := make([]byte, 8192)
		remaining := offset
		for remaining > 0 {
			toRead := int64(len(buf))
			if toRead > remaining {
				toRead = remaining
			}
			n, err := reader.Read(buf[:toRead])
			if err != nil {
				reader.Close()
				return nil, fmt.Errorf("failed to seek to offset: %w", err)
			}
			remaining -= int64(n)
		}
	}

	return reader, nil
}

// ListFiles implements DataSource.
func (s *SMBDataSource) ListFiles(ctx context.Context) ([]RemoteFileInfo, error) {
	// TODO: Implement listing files from SMB
	// For now, this should be called with manifest data
	return nil, fmt.Errorf("ListFiles not implemented - use manifest instead")
}
