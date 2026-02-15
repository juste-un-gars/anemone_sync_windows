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

	procCfGetPlatformInfo       = cldapi.NewProc("CfGetPlatformInfo")
	procCfRegisterSyncRoot      = cldapi.NewProc("CfRegisterSyncRoot")
	procCfUnregisterSyncRoot    = cldapi.NewProc("CfUnregisterSyncRoot")
	procCfGetSyncRootInfoByPath = cldapi.NewProc("CfGetSyncRootInfoByPath")
	procCfRevertPlaceholder     = cldapi.NewProc("CfRevertPlaceholder")
	procCfOpenFileWithOplock    = cldapi.NewProc("CfOpenFileWithOplock")
	procCfCloseHandle           = cldapi.NewProc("CfCloseHandle")
)

const S_OK = 0

// CF_SYNC_ROOT_INFO_CLASS
const (
	CF_SYNC_ROOT_INFO_BASIC    = 0
	CF_SYNC_ROOT_INFO_STANDARD = 1
	CF_SYNC_ROOT_INFO_PROVIDER = 2
)

// CF_REGISTER_FLAGS
const (
	CF_REGISTER_FLAG_UPDATE                               = 0x00000001
	CF_REGISTER_FLAG_DISABLE_ON_DEMAND_POPULATION_ON_ROOT = 0x00000002
	CF_REGISTER_FLAG_MARK_IN_SYNC_ON_ROOT                 = 0x00000004
)

// CF_OPEN_FILE_FLAGS
const (
	CF_OPEN_FILE_FLAG_NONE          = 0x00000000
	CF_OPEN_FILE_FLAG_EXCLUSIVE     = 0x00000001
	CF_OPEN_FILE_FLAG_WRITE_ACCESS  = 0x00000002
	CF_OPEN_FILE_FLAG_DELETE_ACCESS = 0x00000004
	CF_OPEN_FILE_FLAG_FOREGROUND    = 0x00000008
)

// CF_REVERT_FLAGS
const (
	CF_REVERT_FLAG_NONE = 0x00000000
)

// Cloud Files reparse tag
const IO_REPARSE_TAG_CLOUD = 0x9000001A

// File attributes
const (
	FILE_ATTRIBUTE_REPARSE_POINT       = 0x00000400
	FILE_ATTRIBUTE_OFFLINE             = 0x00001000
	FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS = 0x00400000
)

// GUID for AnemoneSync provider
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// CF_HYDRATION_POLICY
type CF_HYDRATION_POLICY struct {
	Primary  uint16
	Modifier uint16
}

// CF_POPULATION_POLICY
type CF_POPULATION_POLICY struct {
	Primary  uint16
	Modifier uint16
}

// CF_SYNC_POLICIES
type CF_SYNC_POLICIES struct {
	StructSize            uint32
	Hydration             CF_HYDRATION_POLICY
	Population            CF_POPULATION_POLICY
	InSync                uint32
	HardLink              uint32
	PlaceholderManagement uint32
}

// CF_SYNC_REGISTRATION
type CF_SYNC_REGISTRATION struct {
	StructSize             uint32
	ProviderName           *uint16
	ProviderVersion        *uint16
	SyncRootIdentity       unsafe.Pointer
	SyncRootIdentityLength uint32
	FileIdentity           unsafe.Pointer
	FileIdentityLength     uint32
	ProviderId             GUID
}

type CF_PLATFORM_INFO struct {
	StructSize     uint32
	BuildNumber    uint32
	RevisionNumber uint32
	IntegrationId  [16]byte // GUID
}

func main() {
	pathFlag := flag.String("path", "", "Path to check/unregister")
	unregisterFlag := flag.Bool("unregister", false, "Unregister the sync root")
	repairFlag := flag.Bool("repair", false, "Re-register then unregister (fixes corrupt metadata)")
	cleanFlag := flag.Bool("clean", false, "Remove cloud file reparse points from all files")
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

	// Repair: re-register then unregister
	if *repairFlag {
		fmt.Println("\n--- REPAIR: Re-registering to fix corrupt metadata ---")
		repairSyncRoot(absPath)
	}

	// Unregister if requested
	if *unregisterFlag {
		fmt.Println("\n--- Unregistering Sync Root ---")
		unregisterSyncRoot(absPath, *forceFlag)
	}

	// Clean: remove reparse points from files
	if *cleanFlag {
		fmt.Println("\n--- CLEAN: Removing cloud file reparse points ---")
		cleanCloudFiles(absPath)
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

func repairSyncRoot(path string) {
	if err := procCfRegisterSyncRoot.Find(); err != nil {
		fmt.Printf("[ERROR] CfRegisterSyncRoot not available: %v\n", err)
		return
	}

	pathPtr, _ := windows.UTF16PtrFromString(path)
	namePtr, _ := windows.UTF16PtrFromString("AnemoneSync")
	versionPtr, _ := windows.UTF16PtrFromString("1.0.0")

	reg := CF_SYNC_REGISTRATION{
		ProviderName:    namePtr,
		ProviderVersion: versionPtr,
		ProviderId: GUID{
			Data1: 0xA4E30000,
			Data2: 0x5059,
			Data3: 0x4E43,
			Data4: [8]byte{0x41, 0x4E, 0x45, 0x4D, 0x4F, 0x4E, 0x45, 0x00},
		},
	}
	reg.StructSize = uint32(unsafe.Sizeof(reg))

	policies := CF_SYNC_POLICIES{
		Hydration:  CF_HYDRATION_POLICY{Primary: 2, Modifier: 0x0004}, // FULL + AUTO_DEHYDRATION
		Population: CF_POPULATION_POLICY{Primary: 2, Modifier: 0},     // ALWAYS_FULL
		InSync:     0x00FFFFFF,                                         // TRACK_ALL
		HardLink:   0,
		PlaceholderManagement: 0x00000007, // All unrestricted
	}
	policies.StructSize = uint32(unsafe.Sizeof(policies))

	flags := uintptr(CF_REGISTER_FLAG_UPDATE | CF_REGISTER_FLAG_DISABLE_ON_DEMAND_POPULATION_ON_ROOT | CF_REGISTER_FLAG_MARK_IN_SYNC_ON_ROOT)

	fmt.Printf("Step 1: CfRegisterSyncRoot(%s) with UPDATE flag...\n", path)
	hr, _, lastErr := procCfRegisterSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&reg)),
		uintptr(unsafe.Pointer(&policies)),
		flags,
	)

	if hr == S_OK {
		fmt.Println("[OK] Re-registration successful! Metadata should be repaired.")
	} else {
		fmt.Printf("[ERROR] CfRegisterSyncRoot failed: HRESULT 0x%08X\n", hr)
		decodeHRESULT(uint32(hr))
		if lastErr != nil && lastErr != syscall.Errno(0) {
			fmt.Printf("        LastError: %v\n", lastErr)
		}
		fmt.Println("\nRepair failed. Try running as Administrator.")
		return
	}

	// Now unregister
	fmt.Println("\nStep 2: Unregistering repaired sync root...")
	unregisterSyncRoot(path, false)
}

func cleanCloudFiles(rootPath string) {
	// Walk the directory and try to remove reparse points
	count := 0
	errors := 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("  [SKIP] %s: %v\n", path, err)
			errors++
			return nil // Continue walking
		}

		if info.IsDir() {
			return nil
		}

		// Check if file has cloud file attributes
		attrs := uint32(info.Sys().(*syscall.Win32FileAttributeData).FileAttributes)
		if attrs&FILE_ATTRIBUTE_OFFLINE != 0 || attrs&FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS != 0 {
			fmt.Printf("  Cleaning: %s (attrs: 0x%08X)\n", path, attrs)

			// Try to remove reparse point via DeviceIoControl
			if removeCloudReparse(path) {
				count++
			} else {
				errors++
			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("[ERROR] Walk failed: %v\n", err)
	}

	fmt.Printf("\n[DONE] Cleaned %d files, %d errors\n", count, errors)
}

func removeCloudReparse(filePath string) bool {
	pathPtr, _ := windows.UTF16PtrFromString(filePath)

	// Try opening with CfOpenFileWithOplock first (uses Cloud Files API)
	if procCfOpenFileWithOplock.Find() == nil {
		var protectedHandle uintptr
		flags := uintptr(CF_OPEN_FILE_FLAG_EXCLUSIVE | CF_OPEN_FILE_FLAG_WRITE_ACCESS | CF_OPEN_FILE_FLAG_DELETE_ACCESS)
		hr, _, _ := procCfOpenFileWithOplock.Call(
			uintptr(unsafe.Pointer(pathPtr)),
			flags,
			uintptr(unsafe.Pointer(&protectedHandle)),
		)
		if hr == S_OK && protectedHandle != 0 {
			// Try to revert placeholder
			if procCfRevertPlaceholder.Find() == nil {
				hr2, _, _ := procCfRevertPlaceholder.Call(
					protectedHandle,
					uintptr(CF_REVERT_FLAG_NONE),
					0, // overlapped
				)
				procCfCloseHandle.Call(protectedHandle)
				if hr2 == S_OK {
					fmt.Printf("    [OK] Reverted via CfRevertPlaceholder\n")
					return true
				}
				fmt.Printf("    [WARN] CfRevertPlaceholder failed: 0x%08X\n", hr2)
			}
			procCfCloseHandle.Call(protectedHandle)
		}
	}

	// Fallback: Try to delete the reparse point directly via FSCTL_DELETE_REPARSE_POINT
	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_WRITE,
		0,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		fmt.Printf("    [ERROR] CreateFile failed: %v\n", err)
		return false
	}
	defer windows.CloseHandle(handle)

	// REPARSE_GUID_DATA_BUFFER_HEADER_SIZE for delete = 8 bytes (ReparseTag + ReparseDataLength)
	type REPARSE_DATA_BUFFER struct {
		ReparseTag        uint32
		ReparseDataLength uint16
		Reserved          uint16
	}

	buf := REPARSE_DATA_BUFFER{
		ReparseTag:        IO_REPARSE_TAG_CLOUD,
		ReparseDataLength: 0,
		Reserved:          0,
	}

	var bytesReturned uint32
	const FSCTL_DELETE_REPARSE_POINT = 0x000900AC

	err = windows.DeviceIoControl(
		handle,
		FSCTL_DELETE_REPARSE_POINT,
		(*byte)(unsafe.Pointer(&buf)),
		uint32(unsafe.Sizeof(buf)),
		nil,
		0,
		&bytesReturned,
		nil,
	)
	if err != nil {
		fmt.Printf("    [ERROR] FSCTL_DELETE_REPARSE_POINT failed: %v\n", err)
		return false
	}

	fmt.Printf("    [OK] Reparse point removed\n")

	// Now remove OFFLINE and RECALL attributes
	newAttrs, _ := windows.GetFileAttributes(pathPtr)
	newAttrs &^= FILE_ATTRIBUTE_OFFLINE
	newAttrs &^= FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS
	windows.SetFileAttributes(pathPtr, newAttrs)

	return true
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
