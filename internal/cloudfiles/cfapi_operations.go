//go:build windows
// +build windows

// Package cloudfiles provides Windows Cloud Files API bindings.
// This file contains data transfer and state management operations.
package cloudfiles

/*
#include "cfapi_bridge.h"
*/
import "C"
import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Execute executes a placeholder operation (transfer data, ack, etc.).
func Execute(opInfo *CF_OPERATION_INFO, opParams *CF_OPERATION_PARAMETERS) error {
	if err := procCfExecute.Find(); err != nil {
		return fmt.Errorf("CfExecute not available: %w", err)
	}

	hr, _, _ := procCfExecute.Call(
		uintptr(unsafe.Pointer(opInfo)),
		uintptr(unsafe.Pointer(opParams)),
	)

	if hr != S_OK {
		return fmt.Errorf("CfExecute failed: HRESULT 0x%08X", hr)
	}

	return nil
}

// SetInSyncState sets the in-sync state of a placeholder.
func SetInSyncState(fileHandle windows.Handle, inSyncState uint32, usn *int64) error {
	if err := procCfSetInSyncState.Find(); err != nil {
		return fmt.Errorf("CfSetInSyncState not available: %w", err)
	}

	var usnPtr uintptr
	if usn != nil {
		usnPtr = uintptr(unsafe.Pointer(usn))
	}

	fmt.Printf("[DEBUG CfSetInSyncState] handle=%v, inSyncState=%d, usnPtr=%v\n",
		fileHandle, inSyncState, usnPtr)

	hr, _, lastErr := procCfSetInSyncState.Call(
		uintptr(fileHandle),
		uintptr(inSyncState),
		0, // IN_SYNC_FLAGS
		usnPtr,
	)

	fmt.Printf("[DEBUG CfSetInSyncState] HRESULT=0x%08X, lastErr=%v\n", hr, lastErr)

	if hr != S_OK {
		return fmt.Errorf("CfSetInSyncState failed: HRESULT 0x%08X (%s)", hr, decodeHRESULT(uint32(hr)))
	}

	return nil
}

// CF_PIN_STATE represents the pin state of a placeholder.
type CF_PIN_STATE uint32

const (
	CF_PIN_STATE_UNSPECIFIED CF_PIN_STATE = 0
	CF_PIN_STATE_PINNED      CF_PIN_STATE = 1
	CF_PIN_STATE_UNPINNED    CF_PIN_STATE = 2
	CF_PIN_STATE_EXCLUDED    CF_PIN_STATE = 3
	CF_PIN_STATE_INHERIT     CF_PIN_STATE = 4
)

// SetPinState sets the pin state of a placeholder.
func SetPinState(fileHandle windows.Handle, pinState CF_PIN_STATE, flags uint32) error {
	if err := procCfSetPinState.Find(); err != nil {
		return fmt.Errorf("CfSetPinState not available: %w", err)
	}

	hr, _, _ := procCfSetPinState.Call(
		uintptr(fileHandle),
		uintptr(pinState),
		uintptr(flags),
		0, // Overlapped - NULL for synchronous
	)

	if hr != S_OK {
		return fmt.Errorf("CfSetPinState failed: HRESULT 0x%08X", hr)
	}

	return nil
}

// TransferData flag constants
const (
	// CF_OPERATION_TRANSFER_DATA_FLAG_MARK_IN_SYNC marks the file as in-sync after transfer.
	// This should be set on the LAST chunk of a hydration operation.
	CF_OPERATION_TRANSFER_DATA_FLAG_MARK_IN_SYNC = 0x00000001
)

// TransferData transfers data to a placeholder during hydration callback.
// Uses the CGO bridge for proper execution in callback context.
// requestKey is required for async operations to identify the callback context.
// isLastChunk should be true for the final chunk to mark the file as in-sync.
func TransferData(connectionKey CF_CONNECTION_KEY, transferKey CF_TRANSFER_KEY, requestKey int64, buffer []byte, offset int64, isLastChunk bool) error {
	if len(buffer) == 0 {
		return nil
	}

	flags := int32(0)
	if isLastChunk {
		flags = CF_OPERATION_TRANSFER_DATA_FLAG_MARK_IN_SYNC
	}

	// Use the C bridge function which executes in the proper context
	result := C.CfapiBridgeTransferData(
		C.int64_t(connectionKey),
		C.int64_t(transferKey),
		C.int64_t(requestKey),
		unsafe.Pointer(&buffer[0]),
		C.int64_t(len(buffer)),
		C.int64_t(offset),
		C.int32_t(flags),
	)

	if result != C.CFAPI_BRIDGE_OK {
		return fmt.Errorf("CfExecute failed: HRESULT 0x%08X", result)
	}

	return nil
}

// CF_OPERATION_ACK_DATA_PARAMS for ACK_DATA operation.
// IMPORTANT: Field alignment must match Windows x64 ABI.
type CF_OPERATION_ACK_DATA_PARAMS struct {
	ParamSize        uint32
	Flags            uint32
	CompletionStatus int32
	_                uint32 // padding for 8-byte alignment of Offset
	Offset           int64
	Length           int64
}

// AckData acknowledges that data transfer is complete.
// This should be called after all data has been transferred for a hydration request.
func AckData(connectionKey CF_CONNECTION_KEY, transferKey CF_TRANSFER_KEY, completionStatus int32) error {
	opInfo := &CF_OPERATION_INFO{
		StructSize:    uint32(unsafe.Sizeof(CF_OPERATION_INFO{})),
		Type:          CF_OPERATION_TYPE_ACK_DATA,
		ConnectionKey: connectionKey,
		TransferKey:   transferKey,
	}

	params := &CF_OPERATION_ACK_DATA_PARAMS{
		ParamSize:        uint32(unsafe.Sizeof(CF_OPERATION_ACK_DATA_PARAMS{})),
		Flags:            0,
		CompletionStatus: completionStatus,
	}

	opParams := &CF_OPERATION_PARAMETERS{
		ParamSize: params.ParamSize,
	}
	*(*CF_OPERATION_ACK_DATA_PARAMS)(unsafe.Pointer(&opParams.Data[0])) = *params

	return Execute(opInfo, opParams)
}

// ReportProviderProgress reports progress during hydration.
// This makes the progress visible in Windows Explorer's progress indicator.
func ReportProviderProgress(connectionKey CF_CONNECTION_KEY, transferKey CF_TRANSFER_KEY, total, completed int64) error {
	if err := procCfReportProviderProgress.Find(); err != nil {
		// Function not available on older Windows versions - silently ignore
		return nil
	}

	type LARGE_INTEGER struct {
		QuadPart int64
	}

	totalLI := LARGE_INTEGER{QuadPart: total}
	completedLI := LARGE_INTEGER{QuadPart: completed}

	hr, _, _ := procCfReportProviderProgress.Call(
		uintptr(connectionKey),
		uintptr(transferKey),
		uintptr(unsafe.Pointer(&totalLI)),
		uintptr(unsafe.Pointer(&completedLI)),
	)

	if hr != S_OK {
		return fmt.Errorf("CfReportProviderProgress failed: HRESULT 0x%08X", hr)
	}

	return nil
}

// CF_OPEN_FILE_FLAGS specifies permissions when opening a file with oplock.
type CF_OPEN_FILE_FLAGS uint32

const (
	CF_OPEN_FILE_FLAG_NONE          CF_OPEN_FILE_FLAGS = 0x00000000
	CF_OPEN_FILE_FLAG_EXCLUSIVE     CF_OPEN_FILE_FLAGS = 0x00000001 // Share-none handle with RH oplock
	CF_OPEN_FILE_FLAG_WRITE_ACCESS  CF_OPEN_FILE_FLAGS = 0x00000002 // Request write access
	CF_OPEN_FILE_FLAG_DELETE_ACCESS CF_OPEN_FILE_FLAGS = 0x00000004 // Request delete access
	CF_OPEN_FILE_FLAG_FOREGROUND    CF_OPEN_FILE_FLAGS = 0x00000008 // Don't request oplock (foreground app)
)

// OpenFileWithOplock opens a file with a proper oplock for safe cloud file operations.
// This is required for operations like dehydration that need exclusive access.
// The returned handle MUST be closed with CloseHandle (not windows.CloseHandle).
func OpenFileWithOplock(filePath string, flags CF_OPEN_FILE_FLAGS) (windows.Handle, error) {
	if err := procCfOpenFileWithOplock.Find(); err != nil {
		return 0, fmt.Errorf("CfOpenFileWithOplock not available: %w", err)
	}

	pathPtr, err := windows.UTF16PtrFromString(filePath)
	if err != nil {
		return 0, fmt.Errorf("invalid file path: %w", err)
	}

	var handle windows.Handle

	hr, _, lastErr := procCfOpenFileWithOplock.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(flags),
		uintptr(unsafe.Pointer(&handle)),
	)

	fmt.Printf("[DEBUG CfOpenFileWithOplock] path=%s, flags=0x%X, hr=0x%08X, handle=%v, lastErr=%v\n",
		filePath, flags, hr, handle, lastErr)

	if hr != S_OK {
		return 0, fmt.Errorf("CfOpenFileWithOplock failed: HRESULT 0x%08X (%s)", hr, decodeHRESULT(uint32(hr)))
	}

	return handle, nil
}

// CloseHandle closes a handle opened with OpenFileWithOplock.
// MUST be used instead of windows.CloseHandle for oplock handles.
func CloseHandle(handle windows.Handle) error {
	if err := procCfCloseHandle.Find(); err != nil {
		return fmt.Errorf("CfCloseHandle not available: %w", err)
	}

	hr, _, _ := procCfCloseHandle.Call(uintptr(handle))

	if hr != S_OK {
		return fmt.Errorf("CfCloseHandle failed: HRESULT 0x%08X", hr)
	}

	return nil
}

// GetWin32HandleFromProtectedHandle converts a protected handle to a Win32 handle.
// The protected handle from CfOpenFileWithOplock cannot be used with regular Win32 APIs.
// This function returns a Win32 handle that CAN be used with Win32 APIs like CfDehydratePlaceholder.
// Note: The returned handle is only valid while the protected handle is valid.
func GetWin32HandleFromProtectedHandle(protectedHandle windows.Handle) (windows.Handle, error) {
	if err := procCfGetWin32HandleFromProtectedHandle.Find(); err != nil {
		return 0, fmt.Errorf("CfGetWin32HandleFromProtectedHandle not available: %w", err)
	}

	win32Handle, _, _ := procCfGetWin32HandleFromProtectedHandle.Call(uintptr(protectedHandle))

	if win32Handle == 0 || win32Handle == ^uintptr(0) { // INVALID_HANDLE_VALUE
		return 0, fmt.Errorf("CfGetWin32HandleFromProtectedHandle returned invalid handle")
	}

	return windows.Handle(win32Handle), nil
}

// ReferenceProtectedHandle increments the reference count of a protected handle.
// Returns a Win32 handle that can be used with non-CfApi Win32 APIs.
// The caller MUST call ReleaseProtectedHandle when done.
func ReferenceProtectedHandle(protectedHandle windows.Handle) (windows.Handle, error) {
	if err := procCfReferenceProtectedHandle.Find(); err != nil {
		return 0, fmt.Errorf("CfReferenceProtectedHandle not available: %w", err)
	}

	win32Handle, _, _ := procCfReferenceProtectedHandle.Call(uintptr(protectedHandle))

	if win32Handle == 0 || win32Handle == ^uintptr(0) { // INVALID_HANDLE_VALUE
		return 0, fmt.Errorf("CfReferenceProtectedHandle returned invalid handle")
	}

	return windows.Handle(win32Handle), nil
}

// ReleaseProtectedHandle decrements the reference count of a protected handle.
// Must be called after using a handle obtained from ReferenceProtectedHandle.
func ReleaseProtectedHandle(protectedHandle windows.Handle) {
	if err := procCfReleaseProtectedHandle.Find(); err != nil {
		return
	}
	procCfReleaseProtectedHandle.Call(uintptr(protectedHandle))
}

// CF_UPDATE_FLAGS for CfUpdatePlaceholder
type CF_UPDATE_FLAGS uint32

const (
	CF_UPDATE_FLAG_NONE                        CF_UPDATE_FLAGS = 0x00000000
	CF_UPDATE_FLAG_VERIFY_IN_SYNC              CF_UPDATE_FLAGS = 0x00000001
	CF_UPDATE_FLAG_MARK_IN_SYNC                CF_UPDATE_FLAGS = 0x00000002 // Mark as in-sync after update
	CF_UPDATE_FLAG_DEHYDRATE                   CF_UPDATE_FLAGS = 0x00000004
	CF_UPDATE_FLAG_ENABLE_ON_DEMAND_POPULATION CF_UPDATE_FLAGS = 0x00000008
	CF_UPDATE_FLAG_DISABLE_ON_DEMAND_POPULATION CF_UPDATE_FLAGS = 0x00000010
	CF_UPDATE_FLAG_REMOVE_FILE_IDENTITY        CF_UPDATE_FLAGS = 0x00000020
	CF_UPDATE_FLAG_CLEAR_IN_SYNC               CF_UPDATE_FLAGS = 0x00000040
	CF_UPDATE_FLAG_REMOVE_PROPERTY             CF_UPDATE_FLAGS = 0x00000080
	CF_UPDATE_FLAG_PASSTHROUGH_FS_METADATA     CF_UPDATE_FLAGS = 0x00000100
	CF_UPDATE_FLAG_ALWAYS_FULL                 CF_UPDATE_FLAGS = 0x00000200
	CF_UPDATE_FLAG_ALLOW_PARTIAL               CF_UPDATE_FLAGS = 0x00000400
)

// UpdatePlaceholder updates a placeholder file with optional metadata and flags.
// Use CF_UPDATE_FLAG_MARK_IN_SYNC to mark the file as in-sync after hydration.
func UpdatePlaceholder(fileHandle windows.Handle, flags CF_UPDATE_FLAGS) error {
	if err := procCfUpdatePlaceholder.Find(); err != nil {
		return fmt.Errorf("CfUpdatePlaceholder not available: %w", err)
	}

	fmt.Printf("[DEBUG CfUpdatePlaceholder] handle=%v, flags=0x%X\n", fileHandle, flags)

	// CfUpdatePlaceholder signature:
	// HRESULT CfUpdatePlaceholder(
	//   HANDLE FileHandle,
	//   const CF_FS_METADATA *FsMetadata,        // NULL = no change
	//   LPCVOID FileIdentity,                    // NULL = no change
	//   DWORD FileIdentityLength,
	//   const CF_FILE_RANGE *DehydrateRangeArray, // NULL = no dehydrate ranges
	//   DWORD DehydrateRangeCount,
	//   CF_UPDATE_FLAGS UpdateFlags,
	//   USN *UpdateUsn,                          // NULL = don't return USN
	//   LPOVERLAPPED Overlapped                  // NULL = synchronous
	// )
	hr, _, lastErr := procCfUpdatePlaceholder.Call(
		uintptr(fileHandle),
		0, // FsMetadata - NULL
		0, // FileIdentity - NULL
		0, // FileIdentityLength
		0, // DehydrateRangeArray - NULL
		0, // DehydrateRangeCount
		uintptr(flags),
		0, // UpdateUsn - NULL
		0, // Overlapped - NULL (synchronous)
	)

	fmt.Printf("[DEBUG CfUpdatePlaceholder] HRESULT=0x%08X, lastErr=%v\n", hr, lastErr)

	if hr != S_OK {
		return fmt.Errorf("CfUpdatePlaceholder failed: HRESULT 0x%08X (%s)", hr, decodeHRESULT(uint32(hr)))
	}

	return nil
}
