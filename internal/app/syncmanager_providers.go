// Package app provides Cloud Files provider management for the sync manager.
package app

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/cloudfiles"
	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	syncpkg "github.com/juste-un-gars/anemone_sync_windows/internal/sync"
	"go.uber.org/zap"
)

// smbClientWrapper wraps smb.SMBClient to implement cloudfiles.SMBFileClient.
type smbClientWrapper struct {
	client *smb.SMBClient
}

func (w *smbClientWrapper) OpenFile(remotePath string) (io.ReadCloser, error) {
	return w.client.OpenFile(remotePath)
}

func (w *smbClientWrapper) ReadFile(remotePath string) ([]byte, error) {
	return w.client.ReadFile(remotePath)
}

func (w *smbClientWrapper) ListRemote(remotePath string) ([]cloudfiles.SMBRemoteFileInfo, error) {
	files, err := w.client.ListRemote(remotePath)
	if err != nil {
		return nil, err
	}

	result := make([]cloudfiles.SMBRemoteFileInfo, len(files))
	for i, f := range files {
		result[i] = cloudfiles.SMBRemoteFileInfo{
			Name:    f.Name,
			Path:    f.Path,
			Size:    f.Size,
			ModTime: f.ModTime,
			IsDir:   f.IsDir,
		}
	}
	return result, nil
}

func (w *smbClientWrapper) IsConnected() bool {
	return w.client.IsConnected()
}

// getOrCreateProvider gets or creates a CloudFilesProvider for the given job.
func (m *SyncManager) getOrCreateProvider(job *SyncJob) (*cloudfiles.CloudFilesProvider, error) {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()

	// Check if provider already exists
	if provider, exists := m.providers[job.ID]; exists {
		return provider, nil
	}

	// Check if Cloud Files API is available
	if !cloudfiles.IsAvailable() {
		return nil, fmt.Errorf("Cloud Files API not available (requires Windows 10 1709+)")
	}

	// Normalize path to Windows format (backslashes)
	localPathWin := filepath.FromSlash(job.LocalPath)

	m.logger.Info("Creating Cloud Files provider",
		zap.String("job", job.Name),
		zap.String("local_path", localPathWin),
	)

	// Create provider config
	config := cloudfiles.ProviderConfig{
		LocalPath:    localPathWin,
		RemotePath:   job.RemotePath, // Relative path within share
		ProviderName: "AnemoneSync",
		Logger:       m.logger.Named("cloudfiles"),
		UseCGOBridge: true, // Enable CGO bridge for proper hydration callbacks
	}

	// Create provider
	provider, err := cloudfiles.NewCloudFilesProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	// Set up SMB data source for hydration
	dataSource, err := m.createSMBDataSource(job)
	if err != nil {
		m.logger.Warn("Failed to create SMB data source for hydration",
			zap.Error(err),
		)
		// Continue without hydration support - placeholders will still work
	} else {
		provider.SetDataSource(dataSource)
	}

	// Initialize the provider (register sync root + connect)
	if err := provider.Initialize(m.ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize provider: %w", err)
	}

	// Configure auto-dehydration if enabled
	if job.AutoDehydrateDays > 0 {
		policy := cloudfiles.DehydrationPolicy{
			Enabled:    true,
			MaxAgeDays: job.AutoDehydrateDays,
		}
		provider.SetDehydrationPolicy(policy)
		if err := provider.StartAutoDehydration(m.ctx); err != nil {
			m.logger.Warn("Failed to start auto-dehydration",
				zap.Error(err),
			)
		}
	}

	// Store provider
	m.providers[job.ID] = provider

	// IMPORTANT: Populate placeholders immediately so the folder is browsable.
	// Without this, Windows can't display the folder contents.
	go m.populatePlaceholdersAsync(provider, job)

	return provider, nil
}

// populatePlaceholdersAsync populates placeholders in the background.
// This is called after provider initialization to make the folder browsable.
// It tries the manifest first (instant), then falls back to full SMB scan.
func (m *SyncManager) populatePlaceholdersAsync(provider *cloudfiles.CloudFilesProvider, job *SyncJob) {
	m.logger.Info("Populating placeholders for Cloud Files provider",
		zap.String("job", job.Name),
		zap.String("local_path", job.LocalPath),
	)

	// Try manifest first (much faster than full SMB scan)
	remoteFiles, err := m.populateFromManifest(job)
	if err != nil {
		m.logger.Info("Manifest not available, falling back to SMB scan",
			zap.String("reason", err.Error()),
		)
		// Fallback to full SMB scan
		remoteFiles, err = m.populateFromSMBScan(job)
		if err != nil {
			m.logger.Error("Failed to list remote files for placeholder population",
				zap.Error(err),
			)
			return
		}
	}

	m.logger.Info("Creating placeholders from remote file list",
		zap.Int("file_count", len(remoteFiles)),
	)

	// Create placeholders
	if err := provider.SyncPlaceholders(m.ctx, remoteFiles); err != nil {
		m.logger.Error("Failed to create placeholders",
			zap.Error(err),
		)
		return
	}

	m.logger.Info("Placeholders created successfully",
		zap.Int("count", len(remoteFiles)),
	)
}

// populateFromManifest reads the Anemone Server manifest and converts it to RemoteFileInfo.
func (m *SyncManager) populateFromManifest(job *SyncJob) ([]cloudfiles.RemoteFileInfo, error) {
	smbClient, err := smb.NewSMBClientFromKeyring(job.RemoteHost, job.RemoteShare, m.logger.Named("smb"))
	if err != nil {
		return nil, fmt.Errorf("failed to create SMB client: %w", err)
	}
	defer smbClient.Disconnect()

	if err := smbClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	manifestReader := syncpkg.NewManifestReader(smbClient, m.logger.Named("manifest"))
	result := manifestReader.ReadManifest(m.ctx, job.RemotePath)

	if result.Error != nil {
		return nil, fmt.Errorf("manifest error: %w", result.Error)
	}
	if !result.Found {
		return nil, fmt.Errorf("no manifest found")
	}

	// Convert manifest files to cloudfiles.RemoteFileInfo
	// Manifest paths are relative to share root; strip RemotePath prefix if set
	manifestFiles := make([]cloudfiles.ManifestFileEntry, 0, len(result.Manifest.Files))
	for _, f := range result.Manifest.Files {
		relPath := f.Path
		if job.RemotePath != "" {
			prefix := strings.TrimPrefix(job.RemotePath, "/")
			if strings.HasPrefix(relPath, prefix+"/") {
				relPath = relPath[len(prefix)+1:]
			} else {
				continue // File not under our remote path
			}
		}
		manifestFiles = append(manifestFiles, cloudfiles.ManifestFileEntry{
			Path:  relPath,
			Size:  f.Size,
			MTime: f.MTime,
			Hash:  f.Hash,
		})
	}

	remoteFiles := cloudfiles.FromManifestFiles(manifestFiles)

	m.logger.Info("Placeholders from manifest",
		zap.Int("file_count", len(remoteFiles)),
		zap.Duration("manifest_read", result.Duration),
	)

	return remoteFiles, nil
}

// populateFromSMBScan does a full recursive SMB scan to list all remote files.
func (m *SyncManager) populateFromSMBScan(job *SyncJob) ([]cloudfiles.RemoteFileInfo, error) {
	dataSource, err := m.createSMBDataSource(job)
	if err != nil {
		return nil, fmt.Errorf("failed to create SMB data source: %w", err)
	}

	remoteFiles, err := dataSource.ListFiles(m.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote files: %w", err)
	}

	return remoteFiles, nil
}

// createSMBDataSource creates a reconnectable SMB data source for hydration.
func (m *SyncManager) createSMBDataSource(job *SyncJob) (cloudfiles.DataSource, error) {
	// Create initial SMB client to verify connectivity
	smbClient, err := smb.NewSMBClientFromKeyring(job.RemoteHost, job.RemoteShare, m.logger.Named("smb"))
	if err != nil {
		return nil, fmt.Errorf("failed to create SMB client: %w", err)
	}

	if err := smbClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	// Wrap in reconnectable adapter that handles dropped connections
	reconnectable := &reconnectableSMBDataSource{
		host:       job.RemoteHost,
		share:      job.RemoteShare,
		remotePath: job.RemotePath,
		client:     smbClient,
		logger:     m.logger.Named("smb_hydration"),
	}

	return reconnectable, nil
}

// reconnectableSMBDataSource wraps an SMB client with auto-reconnection.
// When the connection drops (EOF, reset, timeout), it creates a fresh connection.
type reconnectableSMBDataSource struct {
	host       string
	share      string
	remotePath string
	client     *smb.SMBClient
	logger     *zap.Logger
}

func (r *reconnectableSMBDataSource) reconnect() error {
	// Close old connection (ignore errors)
	if r.client != nil {
		r.client.Disconnect()
	}

	r.logger.Info("reconnecting SMB for hydration",
		zap.String("host", r.host),
		zap.String("share", r.share),
	)

	newClient, err := smb.NewSMBClientFromKeyring(r.host, r.share, r.logger)
	if err != nil {
		return fmt.Errorf("failed to create SMB client: %w", err)
	}
	if err := newClient.Connect(); err != nil {
		return fmt.Errorf("failed to reconnect: %w", err)
	}

	r.client = newClient
	return nil
}

func (r *reconnectableSMBDataSource) GetFileReader(ctx context.Context, relativePath string, offset int64) (io.ReadCloser, error) {
	wrapper := &smbClientWrapper{client: r.client}
	adapter := cloudfiles.NewSMBClientAdapter(wrapper, r.remotePath, r.logger)

	reader, err := adapter.GetFileReader(ctx, relativePath, offset)
	if err != nil {
		// Try reconnecting once on connection error
		if reconnErr := r.reconnect(); reconnErr != nil {
			return nil, fmt.Errorf("connection lost and reconnect failed: %w (original: %v)", reconnErr, err)
		}
		// Retry with new connection
		wrapper = &smbClientWrapper{client: r.client}
		adapter = cloudfiles.NewSMBClientAdapter(wrapper, r.remotePath, r.logger)
		return adapter.GetFileReader(ctx, relativePath, offset)
	}
	return reader, nil
}

func (r *reconnectableSMBDataSource) ListFiles(ctx context.Context) ([]cloudfiles.RemoteFileInfo, error) {
	wrapper := &smbClientWrapper{client: r.client}
	adapter := cloudfiles.NewSMBClientAdapter(wrapper, r.remotePath, r.logger)
	return adapter.ListFiles(ctx)
}

// createPlaceholderCallback creates a callback for creating placeholders.
func (m *SyncManager) createPlaceholderCallback(provider *cloudfiles.CloudFilesProvider, job *SyncJob) syncpkg.PlaceholderCallback {
	return func(files []syncpkg.PlaceholderFileInfo) (int, error) {
		// Convert to cloudfiles.RemoteFileInfo
		remoteFiles := make([]cloudfiles.RemoteFileInfo, len(files))
		for i, f := range files {
			remoteFiles[i] = cloudfiles.RemoteFileInfo{
				Path:        f.RelativePath,
				Size:        f.Size,
				ModTime:     time.Unix(f.ModTime, 0),
				IsDirectory: false,
			}
		}

		// Create placeholders
		if err := provider.SyncPlaceholders(m.ctx, remoteFiles); err != nil {
			return 0, err
		}

		return len(files), nil
	}
}

// CloseProvider closes and removes the provider for a job.
func (m *SyncManager) CloseProvider(jobID int64) error {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()

	provider, exists := m.providers[jobID]
	if !exists {
		return nil
	}

	m.logger.Info("Closing Cloud Files provider", zap.Int64("job_id", jobID))

	// Stop auto-dehydration
	provider.StopAutoDehydration()

	// Close provider (disconnect but keep sync root registered)
	if err := provider.Close(); err != nil {
		return err
	}

	delete(m.providers, jobID)
	return nil
}

// ReconnectProvider reconnects a Cloud Files provider for a job.
// This is used at startup to reconnect providers for jobs that have FilesOnDemand enabled.
func (m *SyncManager) ReconnectProvider(job *SyncJob) error {
	if !job.FilesOnDemand {
		return nil
	}

	// getOrCreateProvider will connect if not already connected
	_, err := m.getOrCreateProvider(job)
	return err
}

// UnregisterProvider completely unregisters the sync root for a job.
// This removes all placeholders and the sync root registration from Windows.
// Use this when disabling Files On Demand for a job.
func (m *SyncManager) UnregisterProvider(jobID int64) error {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()

	provider, exists := m.providers[jobID]
	if !exists {
		return nil
	}

	m.logger.Info("Unregistering Cloud Files provider", zap.Int64("job_id", jobID))

	// Stop auto-dehydration
	provider.StopAutoDehydration()

	// Unregister (this removes sync root and all placeholders)
	if err := provider.Unregister(); err != nil {
		return err
	}

	delete(m.providers, jobID)
	return nil
}

// UnregisterSyncRootByPath unregisters a sync root directly by path.
// This is useful when the provider is not in memory but the sync root is still registered.
func (m *SyncManager) UnregisterSyncRootByPath(localPath string) error {
	// Normalize path to Windows format (backslashes)
	localPathWin := filepath.FromSlash(localPath)
	m.logger.Info("Unregistering sync root by path", zap.String("path", localPathWin))
	return cloudfiles.UnregisterSyncRoot(localPathWin)
}

// closeAllProviders closes all Cloud Files providers.
func (m *SyncManager) closeAllProviders() {
	m.providersMu.Lock()
	defer m.providersMu.Unlock()

	for jobID, provider := range m.providers {
		m.logger.Info("Closing Cloud Files provider", zap.Int64("job_id", jobID))
		provider.StopAutoDehydration()
		provider.Close()
	}
	m.providers = make(map[int64]*cloudfiles.CloudFilesProvider)
}

// GetProvider returns the Cloud Files provider for a job, or nil if not found.
func (m *SyncManager) GetProvider(jobID int64) *cloudfiles.CloudFilesProvider {
	m.providersMu.RLock()
	defer m.providersMu.RUnlock()
	return m.providers[jobID]
}
