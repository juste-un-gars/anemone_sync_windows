// Package sync provides synchronization engine components.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cache"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// ManifestPath is the path to the manifest file in the share.
const ManifestPath = ".anemone/manifest.json"

// ManifestFile represents a single file entry in the Anemone manifest.
type ManifestFile struct {
	Path  string `json:"path"`  // Relative path from share root
	Size  int64  `json:"size"`  // File size in bytes
	MTime int64  `json:"mtime"` // Modification time (Unix timestamp)
	Hash  string `json:"hash"`  // SHA256 hash with "sha256:" prefix
}

// Manifest represents the Anemone Server manifest structure.
type Manifest struct {
	Version     int            `json:"version"`
	GeneratedAt string         `json:"generated_at"` // ISO 8601 timestamp
	ShareName   string         `json:"share_name"`
	ShareType   string         `json:"share_type"` // "backup" or "data"
	Username    string         `json:"username"`
	FileCount   int            `json:"file_count"`
	TotalSize   int64          `json:"total_size"`
	Files       []ManifestFile `json:"files"`
}

// ParsedTime returns the parsed generation time.
func (m *Manifest) ParsedTime() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, m.GeneratedAt)
}

// ManifestReader reads and parses Anemone manifests from SMB shares.
type ManifestReader struct {
	client *smb.SMBClient
	logger *zap.Logger
}

// NewManifestReader creates a new manifest reader.
func NewManifestReader(client *smb.SMBClient, logger *zap.Logger) *ManifestReader {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &ManifestReader{
		client: client,
		logger: logger,
	}
}

// ManifestResult contains the result of reading a manifest.
type ManifestResult struct {
	Manifest *Manifest
	Found    bool          // True if manifest was found
	Duration time.Duration // Time to read and parse
	Error    error         // Error if any (nil if Found is false and no error)
}

// ReadManifest attempts to read the Anemone manifest from the share.
// Returns ManifestResult with Found=false if manifest doesn't exist (not an error).
func (mr *ManifestReader) ReadManifest(ctx context.Context, sharePath string) *ManifestResult {
	startTime := time.Now()
	result := &ManifestResult{}

	// Build path to manifest (relative to share root)
	manifestPath := ManifestPath
	if sharePath != "" && sharePath != "." {
		// If sharePath is a subdirectory, prepend it
		sharePath = strings.TrimPrefix(sharePath, "/")
		sharePath = strings.TrimPrefix(sharePath, "\\")
		if sharePath != "" && sharePath != "." {
			manifestPath = sharePath + "/" + ManifestPath
		}
	}

	mr.logger.Debug("attempting to read manifest",
		zap.String("path", manifestPath),
	)

	// Check context cancellation
	select {
	case <-ctx.Done():
		result.Error = ctx.Err()
		result.Duration = time.Since(startTime)
		return result
	default:
	}

	// Try to read manifest file
	data, err := mr.client.ReadFile(manifestPath)
	if err != nil {
		// Check if it's a "file not found" error
		if isNotFoundError(err) {
			mr.logger.Info("manifest not found, will use SMB scan",
				zap.String("path", manifestPath),
			)
			result.Found = false
			result.Duration = time.Since(startTime)
			return result
		}
		// Other error
		result.Error = fmt.Errorf("failed to read manifest: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Parse JSON
	manifest := &Manifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		result.Error = fmt.Errorf("failed to parse manifest JSON: %w", err)
		result.Duration = time.Since(startTime)
		return result
	}

	result.Manifest = manifest
	result.Found = true
	result.Duration = time.Since(startTime)

	mr.logger.Info("manifest loaded successfully",
		zap.String("share", manifest.ShareName),
		zap.Int("file_count", manifest.FileCount),
		zap.Int64("total_size", manifest.TotalSize),
		zap.Duration("duration", result.Duration),
	)

	return result
}

// ToFileInfoMap converts manifest files to a map of cache.FileInfo.
// This allows direct use with the existing change detection system.
func (m *Manifest) ToFileInfoMap() map[string]*cache.FileInfo {
	files := make(map[string]*cache.FileInfo, len(m.Files))

	for _, f := range m.Files {
		// Normalize path (use forward slashes)
		path := strings.ReplaceAll(f.Path, "\\", "/")

		// Extract hash without prefix
		hash := f.Hash
		if strings.HasPrefix(hash, "sha256:") {
			hash = strings.TrimPrefix(hash, "sha256:")
		}

		files[path] = &cache.FileInfo{
			Path:  path,
			Size:  f.Size,
			MTime: time.Unix(f.MTime, 0),
			Hash:  hash,
		}
	}

	return files
}

// isNotFoundError checks if the error indicates file not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "no such file") ||
		strings.Contains(errStr, "does not exist") ||
		strings.Contains(errStr, "cannot find") ||
		strings.Contains(errStr, "STATUS_OBJECT_NAME_NOT_FOUND") ||
		strings.Contains(errStr, "STATUS_OBJECT_PATH_NOT_FOUND")
}
