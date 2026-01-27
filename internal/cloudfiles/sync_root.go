//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API.
package cloudfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// SyncRootManager manages the lifecycle of a Cloud Files sync root.
// It handles registration, connection, placeholder creation, and callbacks.
type SyncRootManager struct {
	mu sync.RWMutex

	// Configuration
	path            string // Local folder path (sync root)
	providerName    string
	providerVersion string
	providerID      GUID
	useCGOBridge    bool

	// State
	registered bool
	connection *SyncRootConnection

	// CGO Bridge (alternative to windows.NewCallback)
	bridgeManager *BridgeManager

	// Callbacks
	fetchDataCallback    FetchDataCallback
	cancelFetchCallback  CancelFetchCallback
	notifyDeleteCallback NotifyDeleteCallback
	notifyRenameCallback NotifyRenameCallback
}

// FetchDataCallback is called when a placeholder needs to be hydrated.
type FetchDataCallback func(info *FetchDataInfo) error

// FetchDataInfo contains information about a fetch data request.
type FetchDataInfo struct {
	ConnectionKey CF_CONNECTION_KEY
	TransferKey   CF_TRANSFER_KEY
	FilePath      string // Full path to the file
	FileSize      int64
	RequiredOffset int64
	RequiredLength int64
}

// CancelFetchCallback is called when a fetch operation should be cancelled.
type CancelFetchCallback func(filePath string)

// NotifyDeleteCallback is called when a file is being deleted.
type NotifyDeleteCallback func(filePath string, isDirectory bool) bool

// NotifyRenameCallback is called when a file is being renamed.
type NotifyRenameCallback func(sourcePath, targetPath string, isDirectory bool) bool

// SyncRootConfig contains configuration for creating a sync root.
type SyncRootConfig struct {
	Path            string // Local folder path
	ProviderName    string // e.g., "AnemoneSync"
	ProviderVersion string // e.g., "1.0.0"
	ProviderID      GUID   // Unique identifier for the provider
	UseCGOBridge    bool   // Use CGO bridge for callbacks (recommended)
}

// DefaultProviderID returns a default GUID for AnemoneSync.
// GUID: {A4E30000-5059-4E43-414E-454D4F4E4500}
func DefaultProviderID() GUID {
	return GUID{
		Data1: 0xA4E30000,
		Data2: 0x5059,
		Data3: 0x4E43,
		Data4: [8]byte{0x41, 0x4E, 0x45, 0x4D, 0x4F, 0x4E, 0x45, 0x00},
	}
}

// NewSyncRootManager creates a new sync root manager.
func NewSyncRootManager(config SyncRootConfig) (*SyncRootManager, error) {
	if config.Path == "" {
		return nil, fmt.Errorf("sync root path is required")
	}
	if config.ProviderName == "" {
		config.ProviderName = "AnemoneSync"
	}
	if config.ProviderVersion == "" {
		config.ProviderVersion = "1.0.0"
	}

	// Ensure path is absolute
	absPath, err := filepath.Abs(config.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	return &SyncRootManager{
		path:            absPath,
		providerName:    config.ProviderName,
		providerVersion: config.ProviderVersion,
		providerID:      config.ProviderID,
		useCGOBridge:    config.UseCGOBridge,
	}, nil
}

// Path returns the sync root path.
func (m *SyncRootManager) Path() string {
	return m.path
}

// IsRegistered returns whether the sync root is registered.
func (m *SyncRootManager) IsRegistered() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.registered
}

// IsConnected returns whether the sync root is connected.
func (m *SyncRootManager) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connection != nil
}

// Register registers the folder as a sync root.
// The folder must exist before calling this function.
func (m *SyncRootManager) Register() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered {
		return nil // Already registered
	}

	// Ensure the directory exists
	if err := os.MkdirAll(m.path, 0755); err != nil {
		return fmt.Errorf("failed to create sync root directory: %w", err)
	}

	// Create registration info
	registration := NewSyncRegistration(m.providerName, m.providerVersion)
	registration.ProviderId = m.providerID

	// Create policies (Files On Demand style)
	policies := NewDefaultSyncPolicies()

	// Register the sync root
	flags := CF_REGISTER_FLAG_NONE
	if err := RegisterSyncRoot(m.path, registration, policies, flags); err != nil {
		// Check if already registered (not an error)
		if isAlreadyExistsError(err) {
			m.registered = true
			return nil
		}
		return fmt.Errorf("failed to register sync root: %w", err)
	}

	m.registered = true
	return nil
}

// Unregister unregisters the sync root.
func (m *SyncRootManager) Unregister() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.registered {
		return nil // Not registered
	}

	// Disconnect first if connected
	if m.connection != nil {
		if err := m.connection.Disconnect(); err != nil {
			// Log but continue with unregister
		}
		m.connection = nil
	}

	if err := UnregisterSyncRoot(m.path); err != nil {
		return fmt.Errorf("failed to unregister sync root: %w", err)
	}

	m.registered = false
	return nil
}

// Connect connects to the sync root to receive callbacks.
// Must be called after Register().
func (m *SyncRootManager) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.connection != nil {
		return nil // Already connected
	}

	if !m.registered {
		return fmt.Errorf("sync root must be registered before connecting")
	}

	// Build callback table
	callbacks := m.buildCallbackTable()

	// Connect
	conn, err := ConnectSyncRoot(m.path, callbacks, unsafe.Pointer(m), CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH)
	if err != nil {
		return fmt.Errorf("failed to connect to sync root: %w", err)
	}

	m.connection = conn
	return nil
}

// Disconnect disconnects from the sync root.
func (m *SyncRootManager) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Disconnect bridge if using CGO bridge
	if m.bridgeManager != nil {
		m.bridgeManager.Close()
		m.bridgeManager = nil
	}

	if m.connection == nil {
		return nil
	}

	err := m.connection.Disconnect()
	m.connection = nil
	return err
}

// ConnectWithBridge connects using the CGO bridge for callbacks.
// This avoids Go scheduler issues with Windows callbacks.
// Must be called after Register().
func (m *SyncRootManager) ConnectWithBridge(ctx context.Context, logger *zap.Logger) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.bridgeManager != nil {
		return nil // Already connected
	}

	if !m.registered {
		return fmt.Errorf("sync root must be registered before connecting")
	}

	// Create bridge manager
	bridge, err := NewBridgeManager(BridgeConfig{
		SyncRootPath: m.path,
		Logger:       logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create bridge manager: %w", err)
	}

	// Set up handlers that forward to our callbacks
	bridge.SetHandlers(BridgeHandlers{
		OnFetchData: func(req *BridgeFetchDataRequest) error {
			m.mu.RLock()
			cb := m.fetchDataCallback
			m.mu.RUnlock()

			if cb == nil {
				return fmt.Errorf("no fetch data callback registered")
			}

			info := &FetchDataInfo{
				ConnectionKey:  CF_CONNECTION_KEY(req.ConnectionKey),
				TransferKey:    CF_TRANSFER_KEY(req.TransferKey),
				FilePath:       req.FilePath,
				FileSize:       req.FileSize,
				RequiredOffset: req.RequiredOffset,
				RequiredLength: req.RequiredLength,
			}

			return cb(info)
		},
		OnCancelFetch: func(filePath string) {
			m.mu.RLock()
			cb := m.cancelFetchCallback
			m.mu.RUnlock()

			if cb != nil {
				cb(filePath)
			}
		},
		OnNotifyDelete: func(filePath string, isDir bool) bool {
			m.mu.RLock()
			cb := m.notifyDeleteCallback
			m.mu.RUnlock()

			if cb != nil {
				return cb(filePath, isDir)
			}
			return true
		},
		OnNotifyRename: func(src, dst string, isDir bool) bool {
			m.mu.RLock()
			cb := m.notifyRenameCallback
			m.mu.RUnlock()

			if cb != nil {
				return cb(src, dst, isDir)
			}
			return true
		},
	})

	// Start the bridge
	if err := bridge.Start(ctx); err != nil {
		return fmt.Errorf("failed to start bridge: %w", err)
	}

	m.bridgeManager = bridge
	return nil
}

// GetBridgeManager returns the bridge manager (if using CGO bridge).
func (m *SyncRootManager) GetBridgeManager() *BridgeManager {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bridgeManager
}

// IsUsingBridge returns whether the CGO bridge is in use.
func (m *SyncRootManager) IsUsingBridge() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.bridgeManager != nil
}

// SetFetchDataCallback sets the callback for hydration requests.
func (m *SyncRootManager) SetFetchDataCallback(cb FetchDataCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fetchDataCallback = cb
}

// SetCancelFetchCallback sets the callback for cancelled fetch requests.
func (m *SyncRootManager) SetCancelFetchCallback(cb CancelFetchCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cancelFetchCallback = cb
}

// SetNotifyDeleteCallback sets the callback for delete notifications.
func (m *SyncRootManager) SetNotifyDeleteCallback(cb NotifyDeleteCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyDeleteCallback = cb
}

// SetNotifyRenameCallback sets the callback for rename notifications.
func (m *SyncRootManager) SetNotifyRenameCallback(cb NotifyRenameCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyRenameCallback = cb
}

// Close disconnects and unregisters the sync root.
func (m *SyncRootManager) Close() error {
	if err := m.Disconnect(); err != nil {
		return err
	}
	// Note: we don't unregister on close - that's a separate operation
	// because unregistering removes all placeholders
	return nil
}

// buildCallbackTable creates the callback registration table.
func (m *SyncRootManager) buildCallbackTable() []CF_CALLBACK_REGISTRATION {
	var callbacks []CF_CALLBACK_REGISTRATION

	// FETCH_DATA - Required for hydration
	callbacks = append(callbacks, CF_CALLBACK_REGISTRATION{
		Type:     CF_CALLBACK_TYPE_FETCH_DATA,
		Callback: windows.NewCallback(m.onFetchData),
	})

	// CANCEL_FETCH_DATA - Handle cancellation
	callbacks = append(callbacks, CF_CALLBACK_REGISTRATION{
		Type:     CF_CALLBACK_TYPE_CANCEL_FETCH_DATA,
		Callback: windows.NewCallback(m.onCancelFetchData),
	})

	// NOTIFY_DELETE - File deletion notification
	callbacks = append(callbacks, CF_CALLBACK_REGISTRATION{
		Type:     CF_CALLBACK_TYPE_NOTIFY_DELETE,
		Callback: windows.NewCallback(m.onNotifyDelete),
	})

	// NOTIFY_RENAME - File rename notification
	callbacks = append(callbacks, CF_CALLBACK_REGISTRATION{
		Type:     CF_CALLBACK_TYPE_NOTIFY_RENAME,
		Callback: windows.NewCallback(m.onNotifyRename),
	})

	return callbacks
}

// Callback handlers

func (m *SyncRootManager) onFetchData(info *CF_CALLBACK_INFO, params *CF_CALLBACK_PARAMETERS) uintptr {
	filePath := ""
	if info.NormalizedPath != nil {
		filePath = windows.UTF16PtrToString(info.NormalizedPath)
	}
	fmt.Printf("[DEBUG CloudFiles] onFetchData CALLED: path=%s, fileSize=%d\n", filePath, info.FileSize)

	m.mu.RLock()
	cb := m.fetchDataCallback
	m.mu.RUnlock()

	if cb == nil {
		fmt.Printf("[DEBUG CloudFiles] onFetchData: NO CALLBACK SET, returning 0\n")
		return 0
	}

	// Extract parameters
	fetchParams := (*CF_CALLBACK_FETCH_DATA_PARAMS)(unsafe.Pointer(&params.Data[0]))

	fetchInfo := &FetchDataInfo{
		ConnectionKey:  info.ConnectionKey,
		TransferKey:    info.TransferKey,
		FilePath:       filePath,
		FileSize:       info.FileSize,
		RequiredOffset: fetchParams.RequiredFileOffset,
		RequiredLength: fetchParams.RequiredLength,
	}

	// Call user callback
	if err := cb(fetchInfo); err != nil {
		// Report failure
		m.reportFetchFailure(info.ConnectionKey, info.TransferKey)
	}

	return 0
}

func (m *SyncRootManager) onCancelFetchData(info *CF_CALLBACK_INFO, params *CF_CALLBACK_PARAMETERS) uintptr {
	filePath := ""
	if info.NormalizedPath != nil {
		filePath = windows.UTF16PtrToString(info.NormalizedPath)
	}
	fmt.Printf("[DEBUG CloudFiles] onCancelFetchData CALLED: path=%s\n", filePath)

	m.mu.RLock()
	cb := m.cancelFetchCallback
	m.mu.RUnlock()

	if cb == nil {
		return 0
	}

	cb(filePath)
	return 0
}

func (m *SyncRootManager) onNotifyDelete(info *CF_CALLBACK_INFO, params *CF_CALLBACK_PARAMETERS) uintptr {
	filePath := ""
	if info.NormalizedPath != nil {
		filePath = windows.UTF16PtrToString(info.NormalizedPath)
	}
	fmt.Printf("[DEBUG CloudFiles] onNotifyDelete CALLED: path=%s\n", filePath)

	m.mu.RLock()
	cb := m.notifyDeleteCallback
	m.mu.RUnlock()

	if cb == nil {
		return 0
	}

	// TODO: Determine if directory from params
	cb(filePath, false)
	return 0
}

func (m *SyncRootManager) onNotifyRename(info *CF_CALLBACK_INFO, params *CF_CALLBACK_PARAMETERS) uintptr {
	sourcePath := ""
	if info.NormalizedPath != nil {
		sourcePath = windows.UTF16PtrToString(info.NormalizedPath)
	}
	fmt.Printf("[DEBUG CloudFiles] onNotifyRename CALLED: path=%s\n", sourcePath)

	m.mu.RLock()
	cb := m.notifyRenameCallback
	m.mu.RUnlock()

	if cb == nil {
		return 0
	}

	// TODO: Extract target path from params
	targetPath := ""

	cb(sourcePath, targetPath, false)
	return 0
}

func (m *SyncRootManager) reportFetchFailure(connectionKey CF_CONNECTION_KEY, transferKey CF_TRANSFER_KEY) {
	// Report failure to Windows
	opInfo := &CF_OPERATION_INFO{
		StructSize:    uint32(unsafe.Sizeof(CF_OPERATION_INFO{})),
		Type:          CF_OPERATION_TYPE_TRANSFER_DATA,
		ConnectionKey: connectionKey,
		TransferKey:   transferKey,
	}

	params := &CF_OPERATION_TRANSFER_DATA_PARAMS{
		ParamSize:        uint32(unsafe.Sizeof(CF_OPERATION_TRANSFER_DATA_PARAMS{})),
		CompletionStatus: E_FAIL,
	}

	opParams := &CF_OPERATION_PARAMETERS{
		ParamSize: params.ParamSize,
	}
	*(*CF_OPERATION_TRANSFER_DATA_PARAMS)(unsafe.Pointer(&opParams.Data[0])) = *params

	Execute(opInfo, opParams)
}

// CF_CALLBACK_FETCH_DATA_PARAMS for extracting fetch parameters.
type CF_CALLBACK_FETCH_DATA_PARAMS struct {
	Flags              uint32
	RequiredFileOffset int64
	RequiredLength     int64
	OptionalFileOffset int64
	OptionalLength     int64
}

// Error codes (HRESULT as int32)
const (
	E_FAIL int32 = -2147467259 // 0x80004005
)

// isAlreadyExistsError checks if the error indicates already exists.
func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	if hErr, ok := err.(*HRESULTError); ok {
		return hErr.Code == uint32(HRESULT_FROM_WIN32_ERROR_ALREADY_EXISTS)
	}
	return false
}
