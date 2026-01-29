//go:build windows
// +build windows

// Package cloudfiles provides Windows Cloud Files API bindings.
// This file contains placeholder creation and hydration functions.
package cloudfiles

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CreatePlaceholders creates placeholder files in the sync root.
func CreatePlaceholders(basePath string, placeholders []CF_PLACEHOLDER_CREATE_INFO) error {
	if err := procCfCreatePlaceholders.Find(); err != nil {
		return fmt.Errorf("CfCreatePlaceholders not available: %w", err)
	}

	if len(placeholders) == 0 {
		return nil
	}

	pathPtr, err := windows.UTF16PtrFromString(basePath)
	if err != nil {
		return fmt.Errorf("invalid base path: %w", err)
	}

	var entriesProcessed uint32

	hr, _, _ := procCfCreatePlaceholders.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&placeholders[0])),
		uintptr(len(placeholders)),
		0, // CF_CREATE_FLAGS - none
		uintptr(unsafe.Pointer(&entriesProcessed)),
	)

	// 0x800700B7 = HRESULT_FROM_WIN32(ERROR_ALREADY_EXISTS) - not an error if file exists
	const HRESULT_ALREADY_EXISTS = 0x800700B7

	if hr != S_OK && hr != HRESULT_ALREADY_EXISTS {
		return fmt.Errorf("CfCreatePlaceholders failed: HRESULT 0x%08X (processed %d/%d)", hr, entriesProcessed, len(placeholders))
	}

	return nil
}

// HydratePlaceholder hydrates a placeholder file (downloads content).
func HydratePlaceholder(fileHandle windows.Handle, startingOffset, length int64, flags uint32) error {
	if err := procCfHydratePlaceholder.Find(); err != nil {
		return fmt.Errorf("CfHydratePlaceholder not available: %w", err)
	}

	type LARGE_INTEGER struct {
		QuadPart int64
	}

	offset := LARGE_INTEGER{QuadPart: startingOffset}
	size := LARGE_INTEGER{QuadPart: length}

	hr, _, _ := procCfHydratePlaceholder.Call(
		uintptr(fileHandle),
		uintptr(unsafe.Pointer(&offset)),
		uintptr(unsafe.Pointer(&size)),
		uintptr(flags),
		0, // Overlapped - NULL for synchronous
	)

	if hr != S_OK {
		return fmt.Errorf("CfHydratePlaceholder failed: HRESULT 0x%08X", hr)
	}

	return nil
}

// DehydratePlaceholder dehydrates a placeholder file (removes local content).
func DehydratePlaceholder(fileHandle windows.Handle, startingOffset, length int64, flags uint32) error {
	if err := procCfDehydratePlaceholder.Find(); err != nil {
		return fmt.Errorf("CfDehydratePlaceholder not available: %w", err)
	}

	type LARGE_INTEGER struct {
		QuadPart int64
	}

	offset := LARGE_INTEGER{QuadPart: startingOffset}
	size := LARGE_INTEGER{QuadPart: length}

	hr, _, _ := procCfDehydratePlaceholder.Call(
		uintptr(fileHandle),
		uintptr(unsafe.Pointer(&offset)),
		uintptr(unsafe.Pointer(&size)),
		uintptr(flags),
		0, // Overlapped - NULL for synchronous
	)

	if hr != S_OK {
		return fmt.Errorf("CfDehydratePlaceholder failed: HRESULT 0x%08X (%s)", hr, decodeHRESULT(uint32(hr)))
	}

	return nil
}

// GetPlaceholderState returns the placeholder state of a file.
func GetPlaceholderState(fileAttributes uint32, reparseTag uint32) CF_PLACEHOLDER_STATE {
	if err := procCfGetPlaceholderStateFromAttributeTag.Find(); err != nil {
		return CF_PLACEHOLDER_STATE_INVALID
	}

	state, _, _ := procCfGetPlaceholderStateFromAttributeTag.Call(
		uintptr(fileAttributes),
		uintptr(reparseTag),
	)

	return CF_PLACEHOLDER_STATE(state)
}
