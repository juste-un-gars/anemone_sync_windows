//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API.
// This file implements a CGO bridge that avoids Go scheduler issues with Windows callbacks.
package cloudfiles

/*
#include "cfapi_bridge.h"
*/
import "C"

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"go.uber.org/zap"
)

// BridgeManager manages the CGO bridge for Cloud Files callbacks.
// It processes callbacks from a dedicated OS thread to avoid Go scheduler issues.
type BridgeManager struct {
	mu sync.RWMutex

	// Configuration
	syncRootPath string
	logger       *zap.Logger

	// Connection state
	connectionKey C.int64_t
	connected     bool

	// Handlers
	handlers BridgeHandlers

	// Worker goroutine
	stopChan chan struct{}
	doneChan chan struct{}
	running  bool
}

// BridgeHandlers contains callback handlers for Cloud Files events.
type BridgeHandlers struct {
	// OnFetchData is called when a placeholder needs hydration.
	// The handler should transfer data using the provided keys.
	OnFetchData func(req *BridgeFetchDataRequest) error

	// OnCancelFetch is called when a hydration was cancelled.
	OnCancelFetch func(filePath string)

	// OnNotifyDelete is called when a file is being deleted.
	// Return true to allow the delete, false to block it.
	OnNotifyDelete func(filePath string, isDirectory bool) bool

	// OnNotifyRename is called when a file is being renamed.
	// Return true to allow the rename, false to block it.
	OnNotifyRename func(sourcePath, targetPath string, isDirectory bool) bool
}

// BridgeFetchDataRequest contains information about a hydration request.
type BridgeFetchDataRequest struct {
	ConnectionKey  int64
	TransferKey    int64
	FilePath       string // Full normalized path
	FileSize       int64
	RequiredOffset int64
	RequiredLength int64
}

// BridgeConfig contains configuration for the bridge manager.
type BridgeConfig struct {
	SyncRootPath string
	Logger       *zap.Logger
}

// bridgeInitialized tracks global bridge initialization
var (
	bridgeInitMu   sync.Mutex
	bridgeInitDone bool
)

// initBridge initializes the C bridge (call once per process).
func initBridge() error {
	bridgeInitMu.Lock()
	defer bridgeInitMu.Unlock()

	if bridgeInitDone {
		return nil
	}

	result := C.CfapiBridgeInit()
	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("failed to initialize CFAPI bridge: error %d", result)
	}

	bridgeInitDone = true
	return nil
}

// NewBridgeManager creates a new bridge manager.
func NewBridgeManager(config BridgeConfig) (*BridgeManager, error) {
	if config.SyncRootPath == "" {
		return nil, fmt.Errorf("sync root path is required")
	}

	absPath, err := filepath.Abs(config.SyncRootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid sync root path: %w", err)
	}

	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}

	// Initialize the bridge if not done
	if err := initBridge(); err != nil {
		return nil, err
	}

	return &BridgeManager{
		syncRootPath: absPath,
		logger:       config.Logger,
		stopChan:     make(chan struct{}),
		doneChan:     make(chan struct{}),
	}, nil
}

// SetHandlers sets the callback handlers.
func (b *BridgeManager) SetHandlers(handlers BridgeHandlers) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = handlers
}

// Connect connects to the sync root.
func (b *BridgeManager) Connect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.connected {
		return nil
	}

	// Convert path to wide string
	pathPtr, err := syscall.UTF16PtrFromString(b.syncRootPath)
	if err != nil {
		return fmt.Errorf("invalid sync root path: %w", err)
	}

	var connKey C.int64_t
	result := C.CfapiBridgeConnect(
		(*C.wchar_t)(unsafe.Pointer(pathPtr)),
		nil, // callback context not used
		&connKey,
	)

	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("failed to connect to sync root: error %d", result)
	}

	b.connectionKey = connKey
	b.connected = true

	b.logger.Info("connected to sync root via CGO bridge",
		zap.String("path", b.syncRootPath),
		zap.Int64("connection_key", int64(connKey)),
	)

	return nil
}

// Disconnect disconnects from the sync root.
func (b *BridgeManager) Disconnect() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.connected {
		return nil
	}

	result := C.CfapiBridgeDisconnect(b.connectionKey)
	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("failed to disconnect from sync root: error %d", result)
	}

	b.connected = false
	b.connectionKey = 0

	b.logger.Info("disconnected from sync root")

	return nil
}

// Start starts the worker goroutine that processes callbacks.
func (b *BridgeManager) Start(ctx context.Context) error {
	b.mu.Lock()
	if b.running {
		b.mu.Unlock()
		return nil
	}

	// Connect if not already connected
	if !b.connected {
		b.mu.Unlock()
		if err := b.Connect(); err != nil {
			return err
		}
		b.mu.Lock()
	}

	b.running = true
	b.stopChan = make(chan struct{})
	b.doneChan = make(chan struct{})
	b.mu.Unlock()

	// Start worker goroutine
	go b.processLoop(ctx)

	b.logger.Info("bridge worker started")

	return nil
}

// Stop stops the worker goroutine.
func (b *BridgeManager) Stop() {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return
	}
	b.mu.Unlock()

	// Signal stop
	close(b.stopChan)

	// Wait for worker to finish
	<-b.doneChan

	b.mu.Lock()
	b.running = false
	b.mu.Unlock()

	b.logger.Info("bridge worker stopped")
}

// IsConnected returns whether the bridge is connected.
func (b *BridgeManager) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.connected
}

// IsRunning returns whether the worker is running.
func (b *BridgeManager) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// GetQueueCount returns the number of pending requests.
func (b *BridgeManager) GetQueueCount() int {
	return int(C.CfapiBridgeGetQueueCount())
}

// processLoop is the main worker loop that processes callbacks.
// It runs on a dedicated OS thread to avoid Go scheduler issues.
func (b *BridgeManager) processLoop(ctx context.Context) {
	// Lock this goroutine to a specific OS thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	defer close(b.doneChan)

	b.logger.Debug("bridge process loop started on dedicated OS thread")

	for {
		select {
		case <-ctx.Done():
			b.logger.Debug("bridge process loop stopping: context cancelled")
			return
		case <-b.stopChan:
			b.logger.Debug("bridge process loop stopping: stop signal received")
			return
		default:
		}

		// Wait for a request with timeout (allows checking stop channel)
		result := C.CfapiBridgeWaitForRequest(100) // 100ms timeout
		if result == C.CFAPI_BRIDGE_ERROR_TIMEOUT {
			continue // Check stop channel and wait again
		}
		if result != C.CFAPI_BRIDGE_OK {
			b.logger.Error("error waiting for request", zap.Int("result", int(result)))
			continue
		}

		// Poll for request
		var req C.CfapiBridgeRequest
		result = C.CfapiBridgePollRequest(&req)
		if result == C.CFAPI_BRIDGE_ERROR_QUEUE_EMPTY {
			continue // Race condition, queue became empty
		}
		if result != C.CFAPI_BRIDGE_OK {
			b.logger.Error("error polling request", zap.Int("result", int(result)))
			continue
		}

		// Dispatch based on type
		b.dispatchRequest(&req)
	}
}

// dispatchRequest handles a single callback request.
func (b *BridgeManager) dispatchRequest(req *C.CfapiBridgeRequest) {
	b.mu.RLock()
	handlers := b.handlers
	b.mu.RUnlock()

	switch req._type {
	case C.CFAPI_CALLBACK_FETCH_DATA:
		b.handleFetchData(req, handlers.OnFetchData)

	case C.CFAPI_CALLBACK_CANCEL_FETCH_DATA:
		b.handleCancelFetch(req, handlers.OnCancelFetch)

	case C.CFAPI_CALLBACK_NOTIFY_DELETE:
		b.handleNotifyDelete(req, handlers.OnNotifyDelete)

	case C.CFAPI_CALLBACK_NOTIFY_RENAME:
		b.handleNotifyRename(req, handlers.OnNotifyRename)

	default:
		b.logger.Warn("unknown callback type", zap.Int32("type", int32(req._type)))
	}
}

// handleFetchData handles a FETCH_DATA callback.
func (b *BridgeManager) handleFetchData(req *C.CfapiBridgeRequest, handler func(*BridgeFetchDataRequest) error) {
	filePath := wcharToString((*uint16)(unsafe.Pointer(&req.filePath[0])))

	b.logger.Debug("handling FETCH_DATA",
		zap.String("path", filePath),
		zap.Int64("file_size", int64(req.fileSize)),
		zap.Int64("offset", int64(req.requiredOffset)),
		zap.Int64("length", int64(req.requiredLength)),
	)

	if handler == nil {
		b.logger.Warn("no FETCH_DATA handler registered, reporting error")
		C.CfapiBridgeTransferError(req.connectionKey, req.transferKey, C.int32_t(E_FAIL))
		return
	}

	fetchReq := &BridgeFetchDataRequest{
		ConnectionKey:  int64(req.connectionKey),
		TransferKey:    int64(req.transferKey),
		FilePath:       filePath,
		FileSize:       int64(req.fileSize),
		RequiredOffset: int64(req.requiredOffset),
		RequiredLength: int64(req.requiredLength),
	}

	if err := handler(fetchReq); err != nil {
		b.logger.Error("FETCH_DATA handler failed",
			zap.String("path", filePath),
			zap.Error(err),
		)
		C.CfapiBridgeTransferError(req.connectionKey, req.transferKey, C.int32_t(E_FAIL))
	}
}

// handleCancelFetch handles a CANCEL_FETCH_DATA callback.
func (b *BridgeManager) handleCancelFetch(req *C.CfapiBridgeRequest, handler func(string)) {
	filePath := wcharToString((*uint16)(unsafe.Pointer(&req.filePath[0])))

	b.logger.Debug("handling CANCEL_FETCH_DATA", zap.String("path", filePath))

	if handler != nil {
		handler(filePath)
	}
}

// handleNotifyDelete handles a NOTIFY_DELETE callback.
func (b *BridgeManager) handleNotifyDelete(req *C.CfapiBridgeRequest, handler func(string, bool) bool) {
	filePath := wcharToString((*uint16)(unsafe.Pointer(&req.filePath[0])))
	isDir := req.isDirectory != 0

	b.logger.Debug("handling NOTIFY_DELETE",
		zap.String("path", filePath),
		zap.Bool("is_directory", isDir),
	)

	// For notifications, we just inform the handler - can't block the operation
	if handler != nil {
		handler(filePath, isDir)
	}
}

// handleNotifyRename handles a NOTIFY_RENAME callback.
func (b *BridgeManager) handleNotifyRename(req *C.CfapiBridgeRequest, handler func(string, string, bool) bool) {
	sourcePath := wcharToString((*uint16)(unsafe.Pointer(&req.filePath[0])))
	targetPath := wcharToString((*uint16)(unsafe.Pointer(&req.targetPath[0])))
	isDir := req.isDirectory != 0

	b.logger.Debug("handling NOTIFY_RENAME",
		zap.String("source", sourcePath),
		zap.String("target", targetPath),
		zap.Bool("is_directory", isDir),
	)

	// For notifications, we just inform the handler - can't block the operation
	if handler != nil {
		handler(sourcePath, targetPath, isDir)
	}
}

// wcharToString converts a null-terminated wide string to a Go string.
func wcharToString(ptr *uint16) string {
	if ptr == nil {
		return ""
	}

	// Find the length
	length := 0
	for p := ptr; *p != 0; p = (*uint16)(unsafe.Pointer(uintptr(unsafe.Pointer(p)) + 2)) {
		length++
	}

	if length == 0 {
		return ""
	}

	// Convert to string
	slice := unsafe.Slice(ptr, length)
	return syscall.UTF16ToString(slice)
}

// TransferData sends data for a hydration request.
func (b *BridgeManager) TransferData(connectionKey, transferKey int64, data []byte, offset int64) error {
	if len(data) == 0 {
		return nil
	}

	result := C.CfapiBridgeTransferData(
		C.int64_t(connectionKey),
		C.int64_t(transferKey),
		unsafe.Pointer(&data[0]),
		C.int64_t(len(data)),
		C.int64_t(offset),
	)

	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("transfer data failed: error %d", result)
	}

	return nil
}

// TransferComplete completes a hydration request.
func (b *BridgeManager) TransferComplete(connectionKey, transferKey int64) error {
	result := C.CfapiBridgeTransferComplete(
		C.int64_t(connectionKey),
		C.int64_t(transferKey),
	)

	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("transfer complete failed: error %d", result)
	}

	return nil
}

// TransferError reports an error for a hydration request.
func (b *BridgeManager) TransferError(connectionKey, transferKey int64, hresult int32) error {
	result := C.CfapiBridgeTransferError(
		C.int64_t(connectionKey),
		C.int64_t(transferKey),
		C.int32_t(hresult),
	)

	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("transfer error failed: error %d", result)
	}

	return nil
}

// ReportProgress reports progress during hydration.
func (b *BridgeManager) ReportProgress(connectionKey, transferKey int64, total, completed int64) error {
	result := C.CfapiBridgeReportProgress(
		C.int64_t(connectionKey),
		C.int64_t(transferKey),
		C.int64_t(total),
		C.int64_t(completed),
	)

	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("report progress failed: error %d", result)
	}

	return nil
}

// Close stops the worker and disconnects from the sync root.
func (b *BridgeManager) Close() error {
	b.Stop()
	return b.Disconnect()
}
