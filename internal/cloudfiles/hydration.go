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
	"golang.org/x/sys/windows"
)

// HydrationHandler manages the hydration (download) of placeholder files.
type HydrationHandler struct {
	syncRoot     *SyncRootManager
	dataProvider DataProvider
	chunkSize    int64
	logger       *zap.Logger

	mu               sync.RWMutex
	activeHydrations map[CF_TRANSFER_KEY]*activeHydration
}

// activeHydration tracks an in-progress hydration operation.
type activeHydration struct {
	cancel       context.CancelFunc
	filePath     string
	totalBytes   int64
	bytesTransferred int64
}

// DataProvider provides data for hydrating placeholder files.
type DataProvider interface {
	// GetFileReader returns a reader for the file at the given relative path.
	// The reader should be positioned at the given offset.
	GetFileReader(ctx context.Context, relativePath string, offset int64) (io.ReadCloser, error)
}

// NewHydrationHandler creates a new hydration handler.
func NewHydrationHandler(syncRoot *SyncRootManager, provider DataProvider, logger *zap.Logger) *HydrationHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HydrationHandler{
		syncRoot:         syncRoot,
		dataProvider:     provider,
		chunkSize:        1024 * 1024, // 1MB chunks
		logger:           logger,
		activeHydrations: make(map[CF_TRANSFER_KEY]*activeHydration),
	}
}

// SetChunkSize sets the chunk size for data transfer.
func (h *HydrationHandler) SetChunkSize(size int64) {
	if size > 0 {
		h.chunkSize = size
	}
}

// handleFetchDataCallback is the callback function for SyncRootManager.
// It converts FetchDataCallback signature to HandleFetchData call.
func (h *HydrationHandler) handleFetchDataCallback(info *FetchDataInfo) error {
	return h.HandleFetchData(context.Background(), info)
}

// HandleFetchData handles a fetch data callback from Windows.
// This is called when a user opens a placeholder file.
func (h *HydrationHandler) HandleFetchData(ctx context.Context, info *FetchDataInfo) error {
	// Create cancellable context
	ctx, cancel := context.WithCancel(ctx)

	// Get relative path from full path
	relativePath := info.FilePath
	if len(h.syncRoot.Path()) < len(relativePath) {
		relativePath = relativePath[len(h.syncRoot.Path())+1:]
	}

	// Track this hydration
	hydration := &activeHydration{
		cancel:     cancel,
		filePath:   relativePath,
		totalBytes: info.FileSize,
	}
	h.mu.Lock()
	h.activeHydrations[info.TransferKey] = hydration
	h.mu.Unlock()

	// Cleanup on exit
	defer func() {
		h.mu.Lock()
		delete(h.activeHydrations, info.TransferKey)
		h.mu.Unlock()
		cancel()
	}()

	h.logger.Info("starting hydration",
		zap.String("file", relativePath),
		zap.Int64("offset", info.RequiredOffset),
		zap.Int64("size", info.FileSize),
	)

	// Get reader from data provider
	reader, err := h.dataProvider.GetFileReader(ctx, relativePath, info.RequiredOffset)
	if err != nil {
		h.logger.Error("failed to get file reader",
			zap.String("file", relativePath),
			zap.Error(err),
		)
		return fmt.Errorf("failed to get file reader: %w", err)
	}
	defer reader.Close()

	// Transfer data in chunks
	offset := info.RequiredOffset
	remaining := info.RequiredLength
	if remaining <= 0 {
		remaining = info.FileSize - offset
	}

	buffer := make([]byte, h.chunkSize)
	transferred := int64(0)

	for remaining > 0 {
		select {
		case <-ctx.Done():
			h.logger.Info("hydration cancelled",
				zap.String("file", relativePath),
				zap.Int64("transferred", transferred),
			)
			return ctx.Err()
		default:
		}

		// Determine chunk size
		toRead := h.chunkSize
		if toRead > remaining {
			toRead = remaining
		}

		// Read data
		n, err := io.ReadFull(reader, buffer[:toRead])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			h.logger.Error("failed to read data",
				zap.String("file", relativePath),
				zap.Error(err),
			)
			return fmt.Errorf("failed to read data: %w", err)
		}
		if n == 0 {
			break
		}

		// Transfer to Windows
		if err := TransferData(info.ConnectionKey, info.TransferKey, buffer[:n], offset); err != nil {
			h.logger.Error("failed to transfer data",
				zap.String("file", relativePath),
				zap.Error(err),
			)
			return fmt.Errorf("failed to transfer data: %w", err)
		}

		offset += int64(n)
		remaining -= int64(n)
		transferred += int64(n)

		// Update tracking
		h.mu.Lock()
		if active, ok := h.activeHydrations[info.TransferKey]; ok {
			active.bytesTransferred = transferred
		}
		h.mu.Unlock()

		// Report progress to Windows (shows in Explorer)
		h.reportProgress(info.ConnectionKey, info.TransferKey, info.FileSize, offset)
	}

	h.logger.Info("hydration complete",
		zap.String("file", relativePath),
		zap.Int64("bytes", transferred),
	)

	return nil
}

// CancelHydration cancels an active hydration.
func (h *HydrationHandler) CancelHydration(transferKey CF_TRANSFER_KEY) {
	h.mu.RLock()
	active, ok := h.activeHydrations[transferKey]
	h.mu.RUnlock()

	if ok && active != nil {
		h.logger.Info("cancelling hydration",
			zap.String("file", active.filePath),
			zap.Int64("transferred", active.bytesTransferred),
		)
		active.cancel()
	}
}

// CancelHydrationByPath cancels an active hydration by file path.
func (h *HydrationHandler) CancelHydrationByPath(filePath string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, active := range h.activeHydrations {
		if active.filePath == filePath {
			h.logger.Info("cancelling hydration by path",
				zap.String("file", filePath),
			)
			active.cancel()
			return
		}
	}
}

// GetActiveHydrations returns information about active hydrations.
func (h *HydrationHandler) GetActiveHydrations() []HydrationStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]HydrationStatus, 0, len(h.activeHydrations))
	for _, active := range h.activeHydrations {
		result = append(result, HydrationStatus{
			FilePath:         active.filePath,
			TotalBytes:       active.totalBytes,
			BytesTransferred: active.bytesTransferred,
		})
	}
	return result
}

// HydrationStatus represents the status of an active hydration.
type HydrationStatus struct {
	FilePath         string
	TotalBytes       int64
	BytesTransferred int64
}

// reportProgress reports hydration progress to Windows.
func (h *HydrationHandler) reportProgress(connectionKey CF_CONNECTION_KEY, transferKey CF_TRANSFER_KEY, total, completed int64) {
	// Use ReportProviderProgress if available
	_ = ReportProviderProgress(connectionKey, transferKey, total, completed)
}

// HydrateFile manually hydrates a placeholder file (downloads content).
func (h *HydrationHandler) HydrateFile(ctx context.Context, relativePath string) error {
	fullPath := h.syncRoot.Path() + "\\" + relativePath

	// Open the file
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fullPath),
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer windows.CloseHandle(handle)

	// Get file size
	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fileInfo); err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fileSize := int64(fileInfo.FileSizeHigh)<<32 | int64(fileInfo.FileSizeLow)

	// Request hydration
	return HydratePlaceholder(handle, 0, fileSize, 0)
}

// DehydrateFile dehydrates a hydrated file (removes local content, keeps placeholder).
func (h *HydrationHandler) DehydrateFile(ctx context.Context, relativePath string) error {
	fullPath := h.syncRoot.Path() + "\\" + relativePath

	// Open the file
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fullPath),
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer windows.CloseHandle(handle)

	// Get file size
	var fileInfo windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &fileInfo); err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	fileSize := int64(fileInfo.FileSizeHigh)<<32 | int64(fileInfo.FileSizeLow)

	// Request dehydration
	return DehydratePlaceholder(handle, 0, fileSize, 0)
}

// SetPinned sets whether a file should always be available offline.
func (h *HydrationHandler) SetPinned(relativePath string, pinned bool) error {
	fullPath := h.syncRoot.Path() + "\\" + relativePath

	// Open the file
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(fullPath),
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer windows.CloseHandle(handle)

	pinState := CF_PIN_STATE_UNPINNED
	if pinned {
		pinState = CF_PIN_STATE_PINNED
	}

	return SetPinState(handle, pinState, 0)
}
