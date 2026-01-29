// Package app provides Cloud Files provider management for the sync manager.
package app

import (
	"fmt"
	"io"
	"path/filepath"
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
func (m *SyncManager) populatePlaceholdersAsync(provider *cloudfiles.CloudFilesProvider, job *SyncJob) {
	m.logger.Info("Populating placeholders for Cloud Files provider",
		zap.String("job", job.Name),
		zap.String("local_path", job.LocalPath),
	)

	// Try to list files from SMB and create placeholders
	dataSource, err := m.createSMBDataSource(job)
	if err != nil {
		m.logger.Error("Failed to create SMB data source for placeholder population",
			zap.Error(err),
		)
		return
	}

	// List files from remote
	remoteFiles, err := dataSource.ListFiles(m.ctx)
	if err != nil {
		m.logger.Error("Failed to list remote files for placeholder population",
			zap.Error(err),
		)
		return
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

// createSMBDataSource creates an SMB data source for hydration.
func (m *SyncManager) createSMBDataSource(job *SyncJob) (cloudfiles.DataSource, error) {
	// Create SMB client for this job
	smbClient, err := smb.NewSMBClientFromKeyring(job.RemoteHost, job.RemoteShare, m.logger.Named("smb"))
	if err != nil {
		return nil, fmt.Errorf("failed to create SMB client: %w", err)
	}

	// Connect to SMB server
	if err := smbClient.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	// Create adapter
	adapter := cloudfiles.NewSMBClientAdapter(
		&smbClientWrapper{client: smbClient},
		job.RemotePath,
		m.logger.Named("smb_adapter"),
	)

	return adapter, nil
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
