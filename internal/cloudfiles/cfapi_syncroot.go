//go:build windows
// +build windows

// Package cloudfiles provides Windows Cloud Files API bindings.
// This file contains sync root registration and connection functions.
package cloudfiles

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// RegisterSyncRoot registers a directory as a sync root.
func RegisterSyncRoot(syncRootPath string, registration *CF_SYNC_REGISTRATION, policies *CF_SYNC_POLICIES, flags CF_REGISTER_FLAGS) error {
	if err := procCfRegisterSyncRoot.Find(); err != nil {
		return fmt.Errorf("CfRegisterSyncRoot not available: %w", err)
	}

	pathPtr, err := windows.UTF16PtrFromString(syncRootPath)
	if err != nil {
		return fmt.Errorf("invalid sync root path: %w", err)
	}

	hr, _, lastErr := procCfRegisterSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(registration)),
		uintptr(unsafe.Pointer(policies)),
		uintptr(flags),
	)

	_ = lastErr // Ignore for now

	if hr != S_OK {
		return fmt.Errorf("CfRegisterSyncRoot failed: HRESULT 0x%08X (%s)", hr, decodeHRESULT(uint32(hr)))
	}

	return nil
}

// decodeHRESULT returns a human-readable description of an HRESULT.
func decodeHRESULT(hr uint32) string {
	switch hr {
	case 0x80070005:
		return "ERROR_ACCESS_DENIED"
	case 0x800700B7:
		return "ERROR_ALREADY_EXISTS"
	case 0x8007018A:
		return "ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT"
	case 0x8007019A:
		return "ERROR_NOT_A_CLOUD_FILE"
	case 0x800701A4:
		return "ERROR_CLOUD_FILE_PROVIDER_NOT_RUNNING"
	default:
		return "unknown"
	}
}

// UnregisterSyncRoot unregisters a sync root.
func UnregisterSyncRoot(syncRootPath string) error {
	if err := procCfUnregisterSyncRoot.Find(); err != nil {
		return fmt.Errorf("CfUnregisterSyncRoot not available: %w", err)
	}

	pathPtr, err := windows.UTF16PtrFromString(syncRootPath)
	if err != nil {
		return fmt.Errorf("invalid sync root path: %w", err)
	}

	hr, _, _ := procCfUnregisterSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
	)

	if hr != S_OK {
		return fmt.Errorf("CfUnregisterSyncRoot failed: HRESULT 0x%08X", hr)
	}

	return nil
}

// SyncRootConnection represents an active connection to a sync root.
type SyncRootConnection struct {
	ConnectionKey   CF_CONNECTION_KEY
	SyncRootPath    string
	CallbackContext unsafe.Pointer
	callbacks       []CF_CALLBACK_REGISTRATION
}

// ConnectSyncRoot connects to a registered sync root to receive callbacks.
func ConnectSyncRoot(syncRootPath string, callbacks []CF_CALLBACK_REGISTRATION, callbackContext unsafe.Pointer, flags CF_CONNECT_FLAGS) (*SyncRootConnection, error) {
	if err := procCfConnectSyncRoot.Find(); err != nil {
		return nil, fmt.Errorf("CfConnectSyncRoot not available: %w", err)
	}

	pathPtr, err := windows.UTF16PtrFromString(syncRootPath)
	if err != nil {
		return nil, fmt.Errorf("invalid sync root path: %w", err)
	}

	// Ensure callback table ends with CF_CALLBACK_REGISTRATION_END
	callbackTable := append(callbacks, CF_CALLBACK_REGISTRATION_END)

	var connectionKey CF_CONNECTION_KEY

	hr, _, lastErr := procCfConnectSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&callbackTable[0])),
		uintptr(callbackContext),
		uintptr(flags),
		uintptr(unsafe.Pointer(&connectionKey)),
	)

	_ = lastErr // Ignore for now

	if hr != S_OK {
		return nil, fmt.Errorf("CfConnectSyncRoot failed: HRESULT 0x%08X (%s)", hr, decodeHRESULT(uint32(hr)))
	}

	return &SyncRootConnection{
		ConnectionKey:   connectionKey,
		SyncRootPath:    syncRootPath,
		CallbackContext: callbackContext,
		callbacks:       callbackTable,
	}, nil
}

// Disconnect disconnects from the sync root.
func (c *SyncRootConnection) Disconnect() error {
	if err := procCfDisconnectSyncRoot.Find(); err != nil {
		return fmt.Errorf("CfDisconnectSyncRoot not available: %w", err)
	}

	hr, _, _ := procCfDisconnectSyncRoot.Call(
		uintptr(c.ConnectionKey),
	)

	if hr != S_OK {
		return fmt.Errorf("CfDisconnectSyncRoot failed: HRESULT 0x%08X", hr)
	}

	return nil
}
