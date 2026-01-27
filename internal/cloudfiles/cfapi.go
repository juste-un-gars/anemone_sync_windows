//go:build windows
// +build windows

// Package cloudfiles provides Go bindings for the Windows Cloud Files API (cldapi.dll).
// This file contains the DLL loading, core functions, and helper utilities.
package cloudfiles

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// CldApi.dll - Cloud Files API
	cldapi = windows.NewLazySystemDLL("cldapi.dll")

	// Core functions
	procCfRegisterSyncRoot   = cldapi.NewProc("CfRegisterSyncRoot")
	procCfUnregisterSyncRoot = cldapi.NewProc("CfUnregisterSyncRoot")
	procCfConnectSyncRoot    = cldapi.NewProc("CfConnectSyncRoot")
	procCfDisconnectSyncRoot = cldapi.NewProc("CfDisconnectSyncRoot")

	// Placeholder functions
	procCfCreatePlaceholders   = cldapi.NewProc("CfCreatePlaceholders")
	procCfUpdatePlaceholder    = cldapi.NewProc("CfUpdatePlaceholder")
	procCfConvertToPlaceholder = cldapi.NewProc("CfConvertToPlaceholder")
	procCfRevertPlaceholder    = cldapi.NewProc("CfRevertPlaceholder")

	// Hydration functions
	procCfHydratePlaceholder   = cldapi.NewProc("CfHydratePlaceholder")
	procCfDehydratePlaceholder = cldapi.NewProc("CfDehydratePlaceholder")

	// Data transfer
	procCfExecute            = cldapi.NewProc("CfExecute")
	procCfGetTransferKey     = cldapi.NewProc("CfGetTransferKey")
	procCfReleaseTransferKey = cldapi.NewProc("CfReleaseTransferKey")

	// State query
	procCfGetPlaceholderStateFromAttributeTag = cldapi.NewProc("CfGetPlaceholderStateFromAttributeTag")
	procCfGetPlaceholderStateFromFileInfo     = cldapi.NewProc("CfGetPlaceholderStateFromFileInfo")
	procCfGetPlaceholderInfo                  = cldapi.NewProc("CfGetPlaceholderInfo")

	// Sync status
	procCfReportSyncStatus       = cldapi.NewProc("CfReportSyncStatus")
	procCfReportProviderProgress = cldapi.NewProc("CfReportProviderProgress")
	procCfSetInSyncState         = cldapi.NewProc("CfSetInSyncState")
	procCfSetPinState            = cldapi.NewProc("CfSetPinState")
	procCfGetPlatformInfo        = cldapi.NewProc("CfGetPlatformInfo")
)

// HRESULT error codes
const (
	S_OK                                      = 0x00000000
	E_INVALIDARG                              = 0x80070057
	HRESULT_FROM_WIN32_ERROR_ALREADY_EXISTS   = 0x800700B7
)

// IsAvailable checks if the Cloud Files API is available on this system.
func IsAvailable() bool {
	err := cldapi.Load()
	if err != nil {
		return false
	}
	err = procCfRegisterSyncRoot.Find()
	return err == nil
}

// CF_PLATFORM_INFO contains information about the Cloud Files platform.
type CF_PLATFORM_INFO struct {
	StructSize     uint32
	BuildNumber    uint32
	RevisionNumber uint32
	IntegrationId  GUID
}

// GetPlatformInfo returns information about the Cloud Files platform.
func GetPlatformInfo() (*CF_PLATFORM_INFO, error) {
	if err := procCfGetPlatformInfo.Find(); err != nil {
		return nil, fmt.Errorf("CfGetPlatformInfo not available: %w", err)
	}

	info := &CF_PLATFORM_INFO{
		StructSize: uint32(unsafe.Sizeof(CF_PLATFORM_INFO{})),
	}

	hr, _, _ := procCfGetPlatformInfo.Call(
		uintptr(unsafe.Pointer(info)),
	)

	if hr != S_OK {
		return nil, fmt.Errorf("CfGetPlatformInfo failed: HRESULT 0x%08X", hr)
	}

	return info, nil
}

// HRESULTError wraps an HRESULT error code.
type HRESULTError struct {
	Code    uint32
	Message string
}

func (e *HRESULTError) Error() string {
	return fmt.Sprintf("%s (HRESULT 0x%08X)", e.Message, e.Code)
}

// NewHRESULTError creates a new HRESULT error.
func NewHRESULTError(hr uintptr, message string) error {
	if hr == S_OK {
		return nil
	}
	return &HRESULTError{
		Code:    uint32(hr),
		Message: message,
	}
}

// GetLastError returns the last Windows error as a Go error.
func GetLastError() error {
	err := syscall.GetLastError()
	if err == nil {
		return nil
	}
	return err
}
