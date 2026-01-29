//go:build windows
// +build windows

// cloudfiles_debug is a diagnostic tool for Cloud Files API issues.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	cldapi = windows.NewLazySystemDLL("cldapi.dll")

	procCfGetPlatformInfo        = cldapi.NewProc("CfGetPlatformInfo")
	procCfUnregisterSyncRoot     = cldapi.NewProc("CfUnregisterSyncRoot")
	procCfGetSyncRootInfoByPath  = cldapi.NewProc("CfGetSyncRootInfoByPath")
)

const S_OK = 0

// CF_SYNC_ROOT_INFO_CLASS
const (
	CF_SYNC_ROOT_INFO_BASIC      = 0
	CF_SYNC_ROOT_INFO_STANDARD   = 1
	CF_SYNC_ROOT_INFO_PROVIDER   = 2
)

type CF_PLATFORM_INFO struct {
	StructSize     uint32
	BuildNumber    uint32
	RevisionNumber uint32
	IntegrationId  [16]byte // GUID
}

func main() {
	pathFlag := flag.String("path", "", "Path to check/unregister")
	unregisterFlag := flag.Bool("unregister", false, "Unregister the sync root")
	forceFlag := flag.Bool("force", false, "Force unregister even if not found")
	flag.Parse()

	fmt.Println("=== Cloud Files API Diagnostic Tool ===")
	fmt.Println()

	// Check if cldapi.dll is available
	if err := cldapi.Load(); err != nil {
		fmt.Printf("ERROR: cldapi.dll not available: %v\n", err)
		fmt.Println("Cloud Files API requires Windows 10 version 1709 or later.")
		os.Exit(1)
	}
	fmt.Println("[OK] cldapi.dll loaded")

	// Get platform info
	if err := procCfGetPlatformInfo.Find(); err == nil {
		info := CF_PLATFORM_INFO{StructSize: uint32(unsafe.Sizeof(CF_PLATFORM_INFO{}))}
		hr, _, _ := procCfGetPlatformInfo.Call(uintptr(unsafe.Pointer(&info)))
		if hr == S_OK {
			fmt.Printf("[OK] Platform: Build %d, Revision %d\n", info.BuildNumber, info.RevisionNumber)
		} else {
			fmt.Printf("[WARN] CfGetPlatformInfo failed: HRESULT 0x%08X\n", hr)
		}
	}

	if *pathFlag == "" {
		fmt.Println("\nUsage: cloudfiles_debug -path <directory> [-unregister] [-force]")
		fmt.Println("\nExamples:")
		fmt.Println("  cloudfiles_debug -path D:\\test_anemone")
		fmt.Println("  cloudfiles_debug -path D:\\test_anemone -unregister")
		os.Exit(0)
	}

	// Normalize path
	absPath, err := filepath.Abs(*pathFlag)
	if err != nil {
		fmt.Printf("ERROR: Invalid path: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nTarget path: %s\n", absPath)

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("[WARN] Path does not exist")
		} else {
			fmt.Printf("[ERROR] Cannot access path: %v\n", err)
			fmt.Println("This may indicate the sync root is blocking access.")
		}
	} else {
		if info.IsDir() {
			fmt.Println("[OK] Path exists and is a directory")
		} else {
			fmt.Println("[WARN] Path exists but is a file, not a directory")
		}
	}

	// Check sync root info
	fmt.Println("\n--- Checking Sync Root Status ---")
	checkSyncRootInfo(absPath)

	// Unregister if requested
	if *unregisterFlag {
		fmt.Println("\n--- Unregistering Sync Root ---")
		unregisterSyncRoot(absPath, *forceFlag)
	}
}

func checkSyncRootInfo(path string) {
	if err := procCfGetSyncRootInfoByPath.Find(); err != nil {
		fmt.Printf("[WARN] CfGetSyncRootInfoByPath not available: %v\n", err)
		return
	}

	pathPtr, _ := windows.UTF16PtrFromString(path)

	// First call to get required buffer size
	var bufferSize uint32
	hr, _, _ := procCfGetSyncRootInfoByPath.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(CF_SYNC_ROOT_INFO_BASIC),
		0,
		0,
		uintptr(unsafe.Pointer(&bufferSize)),
	)

	// HRESULT_FROM_WIN32(ERROR_MORE_DATA) = 0x800700EA
	// HRESULT_FROM_WIN32(ERROR_NOT_A_CLOUD_FILE) = 0x8007019A (not a cloud file)
	// HRESULT_FROM_WIN32(ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT) = 0x8007018A

	if hr == 0x8007019A {
		fmt.Println("[INFO] Path is NOT registered as a sync root (ERROR_NOT_A_CLOUD_FILE)")
		return
	}

	if hr == 0x8007018A {
		fmt.Println("[INFO] Path is NOT under a sync root (ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT)")
		return
	}

	if hr == 0x800700EA && bufferSize > 0 {
		fmt.Printf("[INFO] Path IS a sync root! (buffer size: %d bytes)\n", bufferSize)

		// Allocate buffer and get info
		buffer := make([]byte, bufferSize)
		hr, _, _ = procCfGetSyncRootInfoByPath.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			uintptr(CF_SYNC_ROOT_INFO_BASIC),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(bufferSize),
			uintptr(unsafe.Pointer(&bufferSize)),
		)
		if hr == S_OK {
			fmt.Println("[OK] Successfully retrieved sync root info")
			// The buffer contains CF_SYNC_ROOT_BASIC_INFO structure
			// SyncRootFileId is at offset 0 (int64)
			syncRootFileId := *(*int64)(unsafe.Pointer(&buffer[0]))
			fmt.Printf("     SyncRootFileId: %d\n", syncRootFileId)
		}
		return
	}

	if hr == S_OK {
		fmt.Println("[INFO] Path may be a sync root (unexpected S_OK with no buffer)")
		return
	}

	// Decode error
	fmt.Printf("[INFO] CfGetSyncRootInfoByPath returned HRESULT 0x%08X\n", hr)
	decodeHRESULT(uint32(hr))
}

func unregisterSyncRoot(path string, force bool) {
	if err := procCfUnregisterSyncRoot.Find(); err != nil {
		fmt.Printf("[ERROR] CfUnregisterSyncRoot not available: %v\n", err)
		return
	}

	pathPtr, _ := windows.UTF16PtrFromString(path)

	fmt.Printf("Calling CfUnregisterSyncRoot(%s)...\n", path)
	hr, _, lastErr := procCfUnregisterSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
	)

	if hr == S_OK {
		fmt.Println("[OK] Sync root unregistered successfully!")
		return
	}

	fmt.Printf("[ERROR] CfUnregisterSyncRoot failed: HRESULT 0x%08X\n", hr)
	decodeHRESULT(uint32(hr))

	if lastErr != nil && lastErr != syscall.Errno(0) {
		fmt.Printf("        LastError: %v\n", lastErr)
	}
}

func decodeHRESULT(hr uint32) {
	// Common HRESULT values for Cloud Files API
	switch hr {
	case 0x80070002:
		fmt.Println("        = ERROR_FILE_NOT_FOUND (The system cannot find the file specified)")
	case 0x80070003:
		fmt.Println("        = ERROR_PATH_NOT_FOUND (The system cannot find the path specified)")
	case 0x80070005:
		fmt.Println("        = ERROR_ACCESS_DENIED (Access is denied)")
	case 0x8007001F:
		fmt.Println("        = ERROR_GEN_FAILURE (A device attached to the system is not functioning)")
	case 0x80070057:
		fmt.Println("        = E_INVALIDARG (One or more arguments are invalid)")
	case 0x800700B7:
		fmt.Println("        = ERROR_ALREADY_EXISTS (Cannot create a file when that file already exists)")
	case 0x800700EA:
		fmt.Println("        = ERROR_MORE_DATA (More data is available)")
	case 0x8007018A:
		fmt.Println("        = ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT")
	case 0x8007018B:
		fmt.Println("        = ERROR_CLOUD_FILE_IN_USE (The cloud file is in use)")
	case 0x8007018C:
		fmt.Println("        = ERROR_CLOUD_FILE_PINNED (The cloud file is pinned)")
	case 0x8007018D:
		fmt.Println("        = ERROR_CLOUD_FILE_REQUEST_ABORTED")
	case 0x8007018E:
		fmt.Println("        = ERROR_CLOUD_FILE_PROPERTY_BLOB_TOO_LARGE")
	case 0x80070190:
		fmt.Println("        = ERROR_CLOUD_FILE_ACCESS_DENIED")
	case 0x80070191:
		fmt.Println("        = ERROR_CLOUD_FILE_INCOMPATIBLE_HARDLINKS")
	case 0x80070192:
		fmt.Println("        = ERROR_CLOUD_FILE_PROPERTY_LOCK_CONFLICT")
	case 0x80070193:
		fmt.Println("        = ERROR_CLOUD_FILE_REQUEST_CANCELED")
	case 0x8007019A:
		fmt.Println("        = ERROR_NOT_A_CLOUD_FILE")
	case 0x8007019B:
		fmt.Println("        = ERROR_CLOUD_FILE_NOT_IN_SYNC")
	case 0x8007019C:
		fmt.Println("        = ERROR_CLOUD_FILE_ALREADY_CONNECTED")
	case 0x8007019D:
		fmt.Println("        = ERROR_CLOUD_FILE_NOT_SUPPORTED")
	case 0x8007019E:
		fmt.Println("        = ERROR_CLOUD_FILE_INVALID_REQUEST")
	case 0x8007019F:
		fmt.Println("        = ERROR_CLOUD_FILE_READ_ONLY_VOLUME")
	case 0x800701A0:
		fmt.Println("        = ERROR_CLOUD_FILE_CONNECTED_PROVIDER_NOT_FOUND")
	case 0x800701A1:
		fmt.Println("        = ERROR_CLOUD_FILE_VALIDATION_FAILED")
	case 0x800701A4:
		fmt.Println("        = ERROR_CLOUD_FILE_PROVIDER_NOT_RUNNING (Provider not connected)")
	case 0x800701A7:
		fmt.Println("        = ERROR_CLOUD_FILE_DEHYDRATION_DISALLOWED")
	default:
		// Try to decode as Win32 error
		if hr&0xFFFF0000 == 0x80070000 {
			win32Err := hr & 0xFFFF
			fmt.Printf("        = Win32 Error %d\n", win32Err)
		}
	}
}
