//go:build windows
// +build windows

// Package cloudfiles provides Windows Cloud Files API bindings.
// This file contains data transfer and state management operations.
package cloudfiles

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

	hr, _, _ := procCfSetInSyncState.Call(
		uintptr(fileHandle),
		uintptr(inSyncState),
		0, // IN_SYNC_FLAGS
		usnPtr,
	)

	if hr != S_OK {
		return fmt.Errorf("CfSetInSyncState failed: HRESULT 0x%08X", hr)
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

// TransferData transfers data to a placeholder during hydration callback.
func TransferData(connectionKey CF_CONNECTION_KEY, transferKey CF_TRANSFER_KEY, buffer []byte, offset int64) error {
	opInfo := &CF_OPERATION_INFO{
		StructSize:    uint32(unsafe.Sizeof(CF_OPERATION_INFO{})),
		Type:          CF_OPERATION_TYPE_TRANSFER_DATA,
		ConnectionKey: connectionKey,
		TransferKey:   transferKey,
	}

	params := &CF_OPERATION_TRANSFER_DATA_PARAMS{
		ParamSize:        uint32(unsafe.Sizeof(CF_OPERATION_TRANSFER_DATA_PARAMS{})),
		CompletionStatus: S_OK,
		Buffer:           unsafe.Pointer(&buffer[0]),
		Offset:           offset,
		Length:           int64(len(buffer)),
	}

	// Copy params to CF_OPERATION_PARAMETERS
	opParams := &CF_OPERATION_PARAMETERS{
		ParamSize: params.ParamSize,
	}
	*(*CF_OPERATION_TRANSFER_DATA_PARAMS)(unsafe.Pointer(&opParams.Data[0])) = *params

	return Execute(opInfo, opParams)
}

// CF_OPERATION_ACK_DATA_PARAMS for ACK_DATA operation.
type CF_OPERATION_ACK_DATA_PARAMS struct {
	ParamSize        uint32
	Flags            uint32
	CompletionStatus int32
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
