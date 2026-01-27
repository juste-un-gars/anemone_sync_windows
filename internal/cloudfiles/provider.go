//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API.
package cloudfiles

import (
	"context"
	"fmt"
	"io"
	"sync"

	"go.uber.org/zap"
)

// CloudFilesProvider manages the Files On Demand functionality for a sync job.
// It handles the sync root lifecycle, placeholder creation, and hydration.
type CloudFilesProvider struct {
	mu sync.RWMutex

	// Configuration
	localPath    string // Local folder path (sync root)
	remotePath   string // Remote SMB path (for hydration)
	providerName string
	useCGOBridge bool   // Use CGO bridge for callbacks

	// Components
	syncRoot     *SyncRootManager
	placeholders *PlaceholderManager
	hydration    *HydrationHandler
	dehydration  *DehydrationManager
	logger       *zap.Logger

	// Data source for hydration
	dataSource DataSource

	// Context for bridge
	ctx    context.Context
	cancel context.CancelFunc

	// State
	initialized bool
}

// DataSource provides remote file data for hydration.
type DataSource interface {
	// GetFileReader returns a reader for the remote file.
	GetFileReader(ctx context.Context, remotePath string, offset int64) (io.ReadCloser, error)
	// ListFiles returns all files from the remote source.
	ListFiles(ctx context.Context) ([]RemoteFileInfo, error)
}

// ProviderConfig contains configuration for CloudFilesProvider.
type ProviderConfig struct {
	LocalPath    string // Local folder to sync
	RemotePath   string // Remote SMB path
	ProviderName string // Provider name for Windows (default: "AnemoneSync")
	Logger       *zap.Logger
	UseCGOBridge bool // Use CGO bridge for callbacks (recommended for proper hydration)
}

// NewCloudFilesProvider creates a new CloudFilesProvider.
func NewCloudFilesProvider(config ProviderConfig) (*CloudFilesProvider, error) {
	if config.LocalPath == "" {
		return nil, fmt.Errorf("local path is required")
	}
	if config.ProviderName == "" {
		config.ProviderName = "AnemoneSync"
	}
	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}

	// Create sync root manager
	syncRootConfig := SyncRootConfig{
		Path:         config.LocalPath,
		ProviderName: config.ProviderName,
		ProviderID:   DefaultProviderID(),
		UseCGOBridge: config.UseCGOBridge,
	}

	syncRoot, err := NewSyncRootManager(syncRootConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync root manager: %w", err)
	}

	provider := &CloudFilesProvider{
		localPath:    config.LocalPath,
		remotePath:   config.RemotePath,
		providerName: config.ProviderName,
		useCGOBridge: config.UseCGOBridge,
		syncRoot:     syncRoot,
		placeholders: NewPlaceholderManager(syncRoot),
		logger:       config.Logger,
	}

	return provider, nil
}

// SetDataSource sets the data source for hydration.
func (p *CloudFilesProvider) SetDataSource(source DataSource) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.dataSource = source

	// Create hydration handler with adapter
	if source != nil {
		adapter := &dataSourceAdapter{source: source, remotePath: p.remotePath}
		p.hydration = NewHydrationHandler(p.syncRoot, adapter, p.logger)

		// If already initialized with bridge, update the callback
		if p.initialized && p.syncRoot.IsUsingBridge() {
			p.syncRoot.SetFetchDataCallback(p.hydration.handleFetchDataCallback)
		}
	}
}

// Initialize registers the sync root and connects to receive callbacks.
func (p *CloudFilesProvider) Initialize(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.initialized {
		return nil
	}

	p.logger.Info("initializing cloud files provider",
		zap.String("local_path", p.localPath),
		zap.String("remote_path", p.remotePath),
		zap.Bool("use_cgo_bridge", p.useCGOBridge),
	)

	// Register sync root
	if err := p.syncRoot.Register(); err != nil {
		return fmt.Errorf("failed to register sync root: %w", err)
	}

	// Connect using CGO bridge if enabled
	if p.useCGOBridge {
		// Create context for bridge
		p.ctx, p.cancel = context.WithCancel(ctx)

		// Set up hydration callback before connecting
		if p.hydration != nil {
			p.syncRoot.SetFetchDataCallback(p.hydration.handleFetchDataCallback)
		}

		if err := p.syncRoot.ConnectWithBridge(p.ctx, p.logger); err != nil {
			p.logger.Error("failed to connect with CGO bridge, falling back to passive mode",
				zap.Error(err),
			)
			// Continue without bridge - placeholders will still work
		} else {
			p.logger.Info("sync root connected with CGO bridge (active callback mode)")
		}
	} else {
		// NOTE: Without CGO bridge, we don't connect to callbacks.
		// Connecting with windows.NewCallback causes Windows to block folder access
		// due to Go scheduler issues.
		p.logger.Info("sync root registered (passive mode - no callbacks)")
	}

	p.initialized = true
	p.logger.Info("cloud files provider initialized successfully")

	return nil
}

// SyncPlaceholders syncs placeholders with the remote file list.
// It creates new placeholders and removes ones that no longer exist remotely.
func (p *CloudFilesProvider) SyncPlaceholders(ctx context.Context, remoteFiles []RemoteFileInfo) error {
	p.mu.RLock()
	if !p.initialized {
		p.mu.RUnlock()
		return fmt.Errorf("provider not initialized")
	}
	p.mu.RUnlock()

	p.logger.Info("syncing placeholders",
		zap.Int("remote_file_count", len(remoteFiles)),
	)

	// Create placeholders for all remote files
	if err := p.placeholders.CreatePlaceholders(remoteFiles); err != nil {
		return fmt.Errorf("failed to create placeholders: %w", err)
	}

	// TODO: Remove placeholders that no longer exist remotely
	// This requires scanning the local directory and comparing

	p.logger.Info("placeholders synced successfully",
		zap.Int("placeholder_count", len(remoteFiles)),
	)

	return nil
}

// SyncFromManifest syncs placeholders using manifest file entries.
func (p *CloudFilesProvider) SyncFromManifest(ctx context.Context, manifestFiles []ManifestFileEntry) error {
	remoteFiles := FromManifestFiles(manifestFiles)
	return p.SyncPlaceholders(ctx, remoteFiles)
}

// GetPlaceholderState returns the state of a file.
func (p *CloudFilesProvider) GetPlaceholderState(relativePath string) (PlaceholderFileState, error) {
	return p.placeholders.GetPlaceholderState(relativePath)
}

// HydrateFile manually hydrates a placeholder file.
func (p *CloudFilesProvider) HydrateFile(ctx context.Context, relativePath string) error {
	p.mu.RLock()
	hydration := p.hydration
	p.mu.RUnlock()

	if hydration == nil {
		return fmt.Errorf("no data source configured")
	}

	return hydration.HydrateFile(ctx, relativePath)
}

// SetPinned sets whether a file should always be available offline.
func (p *CloudFilesProvider) SetPinned(relativePath string, pinned bool) error {
	p.mu.RLock()
	hydration := p.hydration
	p.mu.RUnlock()

	if hydration == nil {
		return fmt.Errorf("no data source configured")
	}

	return hydration.SetPinned(relativePath, pinned)
}

// Close disconnects from the sync root.
// Note: This does NOT unregister the sync root - placeholders remain visible.
func (p *CloudFilesProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.initialized {
		return nil
	}

	p.logger.Info("closing cloud files provider")

	// Cancel bridge context if using CGO bridge
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}

	if err := p.syncRoot.Close(); err != nil {
		return err
	}

	p.initialized = false
	return nil
}

// Unregister completely removes the sync root and all placeholders.
func (p *CloudFilesProvider) Unregister() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.logger.Info("unregistering cloud files provider")

	if err := p.syncRoot.Unregister(); err != nil {
		return err
	}

	p.initialized = false
	return nil
}

// IsInitialized returns whether the provider is initialized.
func (p *CloudFilesProvider) IsInitialized() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.initialized
}

// GetActiveHydrations returns information about active hydrations.
func (p *CloudFilesProvider) GetActiveHydrations() []HydrationStatus {
	p.mu.RLock()
	hydration := p.hydration
	p.mu.RUnlock()

	if hydration == nil {
		return nil
	}
	return hydration.GetActiveHydrations()
}

// CancelAllHydrations cancels all active hydrations.
func (p *CloudFilesProvider) CancelAllHydrations() {
	p.mu.RLock()
	hydration := p.hydration
	p.mu.RUnlock()

	if hydration == nil {
		return
	}

	for _, status := range hydration.GetActiveHydrations() {
		hydration.CancelHydrationByPath(status.FilePath)
	}
}

// SetDehydrationPolicy sets the dehydration policy.
func (p *CloudFilesProvider) SetDehydrationPolicy(policy DehydrationPolicy) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dehydration == nil {
		p.dehydration = NewDehydrationManager(p.syncRoot, policy, p.logger)
	} else {
		p.dehydration.SetPolicy(policy)
	}
}

// GetDehydrationPolicy returns the current dehydration policy.
func (p *CloudFilesProvider) GetDehydrationPolicy() DehydrationPolicy {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.dehydration == nil {
		return DefaultDehydrationPolicy()
	}
	return p.dehydration.GetPolicy()
}

// StartAutoDehydration starts automatic dehydration based on the policy.
func (p *CloudFilesProvider) StartAutoDehydration(ctx context.Context) error {
	p.mu.Lock()
	if p.dehydration == nil {
		p.dehydration = NewDehydrationManager(p.syncRoot, DefaultDehydrationPolicy(), p.logger)
	}
	dehydration := p.dehydration
	p.mu.Unlock()

	return dehydration.Start(ctx)
}

// StopAutoDehydration stops automatic dehydration.
func (p *CloudFilesProvider) StopAutoDehydration() {
	p.mu.RLock()
	dehydration := p.dehydration
	p.mu.RUnlock()

	if dehydration != nil {
		dehydration.Stop()
	}
}

// DehydrateFile dehydrates a single file (frees its disk space).
func (p *CloudFilesProvider) DehydrateFile(ctx context.Context, relativePath string) error {
	p.mu.Lock()
	if p.dehydration == nil {
		p.dehydration = NewDehydrationManager(p.syncRoot, DefaultDehydrationPolicy(), p.logger)
	}
	dehydration := p.dehydration
	p.mu.Unlock()

	return dehydration.DehydrateFile(ctx, relativePath)
}

// DehydrateAll dehydrates all eligible files based on the policy.
func (p *CloudFilesProvider) DehydrateAll(ctx context.Context) (int, int64, error) {
	p.mu.Lock()
	if p.dehydration == nil {
		p.dehydration = NewDehydrationManager(p.syncRoot, DefaultDehydrationPolicy(), p.logger)
	}
	dehydration := p.dehydration
	p.mu.Unlock()

	return dehydration.DehydrateAll(ctx)
}

// GetSpaceUsage returns information about disk space usage.
func (p *CloudFilesProvider) GetSpaceUsage(ctx context.Context) (SpaceUsage, error) {
	p.mu.Lock()
	if p.dehydration == nil {
		p.dehydration = NewDehydrationManager(p.syncRoot, DefaultDehydrationPolicy(), p.logger)
	}
	dehydration := p.dehydration
	p.mu.Unlock()

	return dehydration.GetSpaceUsage(ctx)
}

// GetDehydrationStats returns dehydration statistics.
func (p *CloudFilesProvider) GetDehydrationStats() DehydrationStats {
	p.mu.RLock()
	dehydration := p.dehydration
	p.mu.RUnlock()

	if dehydration == nil {
		return DehydrationStats{}
	}
	return dehydration.GetStats()
}

