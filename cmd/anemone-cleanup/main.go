//go:build windows
// +build windows

// anemone-cleanup is a standalone tool to clean up corrupted Cloud Files
// placeholders left behind by AnemoneSync. It repairs sync root metadata,
// detaches the CldFlt minifilter, deletes placeholder files, and reattaches.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// cldapi.dll syscalls
var (
	cldapi = windows.NewLazySystemDLL("cldapi.dll")

	procCfGetPlatformInfo       = cldapi.NewProc("CfGetPlatformInfo")
	procCfRegisterSyncRoot      = cldapi.NewProc("CfRegisterSyncRoot")
	procCfUnregisterSyncRoot    = cldapi.NewProc("CfUnregisterSyncRoot")
	procCfGetSyncRootInfoByPath = cldapi.NewProc("CfGetSyncRootInfoByPath")
)

const S_OK = 0

// CF_SYNC_ROOT_INFO_CLASS
const (
	CF_SYNC_ROOT_INFO_BASIC = 0
)

// CF_REGISTER_FLAGS
const (
	CF_REGISTER_FLAG_UPDATE                               = 0x00000001
	CF_REGISTER_FLAG_DISABLE_ON_DEMAND_POPULATION_ON_ROOT = 0x00000002
	CF_REGISTER_FLAG_MARK_IN_SYNC_ON_ROOT                 = 0x00000004
)

// File attributes for cloud files detection
const (
	FILE_ATTRIBUTE_OFFLINE               = 0x00001000
	FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS = 0x00400000
)

// Structures for Cloud Files API
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

type CF_HYDRATION_POLICY struct {
	Primary  uint16
	Modifier uint16
}

type CF_POPULATION_POLICY struct {
	Primary  uint16
	Modifier uint16
}

type CF_SYNC_POLICIES struct {
	StructSize            uint32
	Hydration             CF_HYDRATION_POLICY
	Population            CF_POPULATION_POLICY
	InSync                uint32
	HardLink              uint32
	PlaceholderManagement uint32
}

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
	IntegrationId  [16]byte
}

// Sync root status from diagnose
type syncRootStatus int

const (
	statusNotRegistered syncRootStatus = iota
	statusRegistered
	statusCorrupt
	statusUnknownError
)

// parseArgs handles flags in any position (before or after the path argument).
func parseArgs(args []string) (doDelete, doDryRun, doHelp bool, path string) {
	for _, arg := range args {
		switch arg {
		case "--delete", "-delete":
			doDelete = true
		case "--dry-run", "-dry-run":
			doDryRun = true
		case "--help", "-help", "-h":
			doHelp = true
		default:
			if !strings.HasPrefix(arg, "-") {
				path = arg
			}
		}
	}
	return
}

func main() {
	doDelete, doDryRun, doHelp, targetPath := parseArgs(os.Args[1:])

	if doHelp || targetPath == "" {
		printUsage()
		os.Exit(0)
	}
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		fmt.Printf("ERROR: Invalid path: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== AnemoneSync Cleanup Tool ===")
	fmt.Println()

	// Check cldapi.dll
	if err := cldapi.Load(); err != nil {
		fmt.Printf("ERROR: cldapi.dll not available: %v\n", err)
		fmt.Println("Cloud Files API requires Windows 10 version 1709 or later.")
		os.Exit(1)
	}

	// Diagnostic mode (default, or explicit --dry-run)
	if !doDelete {
		fmt.Printf("[1/2] Checking sync root: %s\n", absPath)
		status := diagnose(absPath)
		printStatus(status)

		fmt.Println()
		fmt.Println("[2/2] Scanning placeholder files...")
		placeholders, normal := scanFiles(absPath)
		fmt.Printf("      Found %d placeholder files\n", placeholders)
		fmt.Printf("      Found %d normal files (would be preserved)\n", normal)

		fmt.Println()
		fmt.Println("=== Diagnostic complete ===")
		fmt.Println("Run with --delete to clean up placeholder files.")
		return
	}

	// Delete mode
	if doDryRun {
		fmt.Println("*** DRY-RUN MODE: No changes will be made ***")
		fmt.Println()
	}

	// Check admin rights (needed for fltmc)
	if !isAdmin() {
		fmt.Println("WARNING: Not running as Administrator.")
		fmt.Println("         fltmc detach/attach requires elevated privileges.")
		fmt.Println("         Re-run this tool as Administrator.")
		os.Exit(1)
	}

	volume := extractVolume(absPath)

	// Step 1: Diagnose
	fmt.Printf("[1/7] Checking sync root: %s\n", absPath)
	status := diagnose(absPath)
	printStatus(status)
	fmt.Println()

	// Step 2: Repair if corrupt
	if status == statusCorrupt {
		fmt.Println("[2/7] Repairing sync root metadata...")
		if doDryRun {
			fmt.Println("      [DRY-RUN] Would re-register then unregister sync root")
		} else {
			repairSyncRoot(absPath)
		}
	} else if status == statusRegistered {
		fmt.Println("[2/7] Sync root not corrupt, skipping repair")
	} else {
		fmt.Println("[2/7] No sync root to repair")
	}
	fmt.Println()

	// Step 3: Unregister if registered (skip if repair already unregistered)
	if status == statusRegistered {
		fmt.Println("[3/7] Unregistering sync root...")
		if doDryRun {
			fmt.Println("      [DRY-RUN] Would unregister sync root")
		} else {
			unregisterSyncRoot(absPath)
		}
	} else if status == statusCorrupt {
		fmt.Println("[3/7] Already unregistered by repair step")
	} else {
		fmt.Println("[3/7] No sync root to unregister")
	}
	fmt.Println()

	// Step 4: Detach minifilter
	fmt.Printf("[4/7] Detaching CldFlt minifilter from %s\n", volume)
	if doDryRun {
		fmt.Printf("      [DRY-RUN] Would run: fltmc detach CldFlt %s\n", volume)
	} else {
		detachMinifilter(volume)
		// Ensure reattach even on panic/early exit
		defer func() {
			fmt.Printf("\n[7/7] Reattaching CldFlt minifilter to %s\n", volume)
			attachMinifilter(volume)
		}()
	}
	fmt.Println()

	// Step 5: Delete placeholders
	fmt.Println("[5/7] Scanning and deleting placeholder files...")
	deleted, preserved, errors := deletePlaceholders(absPath, doDryRun)
	fmt.Printf("      Deleted %d placeholder files, preserved %d normal files", deleted, preserved)
	if errors > 0 {
		fmt.Printf(", %d errors", errors)
	}
	fmt.Println()
	fmt.Println()

	// Step 6: Clean empty dirs
	fmt.Println("[6/7] Cleaning empty directories...")
	removedDirs := cleanEmptyDirs(absPath, doDryRun)
	fmt.Printf("      Removed %d empty directories\n", removedDirs)

	if doDryRun {
		fmt.Println()
		fmt.Printf("[7/7] [DRY-RUN] Would reattach CldFlt minifilter to %s\n", volume)
		fmt.Println()
		fmt.Println("=== Dry-run complete (no changes made) ===")
	} else {
		// Step 7 handled by defer above
		fmt.Println()
		fmt.Println("=== Cleanup complete! ===")
	}
}

func printUsage() {
	fmt.Println("AnemoneSync Cleanup Tool")
	fmt.Println()
	fmt.Println("Cleans up corrupted Cloud Files placeholders left by AnemoneSync.")
	fmt.Println("Useful when placeholder files become undeletable due to corrupt metadata.")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  anemone-cleanup.exe <path>              Diagnostic only (no changes)")
	fmt.Println("  anemone-cleanup.exe <path> --delete     Delete placeholder files")
	fmt.Println("  anemone-cleanup.exe <path> --delete --dry-run  Show what would be done")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --delete    Repair sync root, detach minifilter, delete placeholders")
	fmt.Println("  --dry-run   Show what would be done without making changes")
	fmt.Println("  --help      Show this help message")
	fmt.Println()
	fmt.Println("NOTE: --delete requires running as Administrator (for fltmc).")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  anemone-cleanup.exe D:\\Anemone\\Backup")
	fmt.Println("  anemone-cleanup.exe D:\\Anemone\\Backup --delete")
	fmt.Println("  anemone-cleanup.exe D:\\Anemone\\Backup --delete --dry-run")
}

// diagnose checks the sync root status at the given path.
func diagnose(path string) syncRootStatus {
	if err := procCfGetSyncRootInfoByPath.Find(); err != nil {
		fmt.Printf("      CfGetSyncRootInfoByPath not available: %v\n", err)
		return statusUnknownError
	}

	pathPtr, _ := windows.UTF16PtrFromString(path)

	var bufferSize uint32
	hr, _, _ := procCfGetSyncRootInfoByPath.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(CF_SYNC_ROOT_INFO_BASIC),
		0,
		0,
		uintptr(unsafe.Pointer(&bufferSize)),
	)

	switch {
	case hr == 0x8007019A: // ERROR_NOT_A_CLOUD_FILE
		return statusNotRegistered
	case hr == 0x8007018A: // ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT
		return statusNotRegistered
	case hr == 0x800700EA && bufferSize > 0: // ERROR_MORE_DATA = sync root exists
		return statusRegistered
	case hr == S_OK:
		return statusRegistered
	default:
		fmt.Printf("      HRESULT: 0x%08X\n", hr)
		decodeHRESULT(uint32(hr))
		// Corrupt metadata often returns specific error codes
		if hr == 0x80070186 || hr == 0x8007001F {
			return statusCorrupt
		}
		return statusCorrupt
	}
}

func printStatus(status syncRootStatus) {
	switch status {
	case statusNotRegistered:
		fmt.Println("      Status: NOT REGISTERED (no sync root found)")
	case statusRegistered:
		fmt.Println("      Status: REGISTERED (sync root active)")
	case statusCorrupt:
		fmt.Println("      Status: CORRUPT METADATA (needs repair)")
	case statusUnknownError:
		fmt.Println("      Status: UNKNOWN ERROR")
	}
}

// scanFiles counts placeholder vs normal files without modifying anything.
func scanFiles(rootPath string) (placeholders, normal int) {
	walkErrors := 0
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			walkErrors++
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if isPlaceholder(info) {
			placeholders++
		} else {
			normal++
		}
		return nil
	})
	if walkErrors > 0 {
		fmt.Printf("      (%d entries could not be accessed - corrupt metadata may block enumeration)\n", walkErrors)
	}
	return
}

// isPlaceholder checks if a file has Cloud Files attributes.
func isPlaceholder(info os.FileInfo) bool {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return false
	}
	attrs := uint32(data.FileAttributes)
	return attrs&FILE_ATTRIBUTE_OFFLINE != 0 || attrs&FILE_ATTRIBUTE_RECALL_ON_DATA_ACCESS != 0
}

// repairSyncRoot re-registers the sync root (with UPDATE flag) to fix
// corrupt metadata, then immediately unregisters it.
func repairSyncRoot(path string) {
	if err := procCfRegisterSyncRoot.Find(); err != nil {
		fmt.Printf("      ERROR: CfRegisterSyncRoot not available: %v\n", err)
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
		Hydration:             CF_HYDRATION_POLICY{Primary: 2, Modifier: 0x0004},
		Population:            CF_POPULATION_POLICY{Primary: 2, Modifier: 0},
		InSync:                0x00FFFFFF,
		HardLink:              0,
		PlaceholderManagement: 0x00000007,
	}
	policies.StructSize = uint32(unsafe.Sizeof(policies))

	flags := uintptr(CF_REGISTER_FLAG_UPDATE | CF_REGISTER_FLAG_DISABLE_ON_DEMAND_POPULATION_ON_ROOT | CF_REGISTER_FLAG_MARK_IN_SYNC_ON_ROOT)

	hr, _, lastErr := procCfRegisterSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&reg)),
		uintptr(unsafe.Pointer(&policies)),
		flags,
	)

	if hr == S_OK {
		fmt.Println("      Re-registered sync root: OK")
	} else {
		fmt.Printf("      Re-register FAILED: HRESULT 0x%08X\n", hr)
		decodeHRESULT(uint32(hr))
		if lastErr != nil && lastErr != syscall.Errno(0) {
			fmt.Printf("      LastError: %v\n", lastErr)
		}
		return
	}

	// Now unregister
	unregisterSyncRoot(path)
}

// unregisterSyncRoot removes the sync root registration.
func unregisterSyncRoot(path string) {
	if err := procCfUnregisterSyncRoot.Find(); err != nil {
		fmt.Printf("      ERROR: CfUnregisterSyncRoot not available: %v\n", err)
		return
	}

	pathPtr, _ := windows.UTF16PtrFromString(path)
	hr, _, lastErr := procCfUnregisterSyncRoot.Call(
		uintptr(unsafe.Pointer(pathPtr)),
	)

	if hr == S_OK {
		fmt.Println("      Unregistered sync root: OK")
	} else {
		fmt.Printf("      Unregister FAILED: HRESULT 0x%08X\n", hr)
		decodeHRESULT(uint32(hr))
		if lastErr != nil && lastErr != syscall.Errno(0) {
			fmt.Printf("      LastError: %v\n", lastErr)
		}
	}
}

// extractVolume extracts the volume root from a path (e.g., "D:\foo" -> "D:").
func extractVolume(path string) string {
	vol := filepath.VolumeName(path)
	if vol == "" {
		// Fallback: take first 2 chars if it looks like a drive letter
		if len(path) >= 2 && path[1] == ':' {
			return path[:2]
		}
		return "C:"
	}
	return vol
}

// detachMinifilter runs fltmc detach CldFlt <volume>.
func detachMinifilter(volume string) {
	cmd := exec.Command("fltmc", "detach", "CldFlt", volume)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))
		// Not an error if already detached
		if strings.Contains(outStr, "0x801f0014") || strings.Contains(strings.ToLower(outStr), "not attached") {
			fmt.Println("      Already detached: OK")
			return
		}
		fmt.Printf("      WARNING: fltmc detach failed: %v\n", err)
		if len(outStr) > 0 {
			fmt.Printf("      Output: %s\n", outStr)
		}
		return
	}
	fmt.Println("      OK")
}

// attachMinifilter runs fltmc attach CldFlt <volume>.
func attachMinifilter(volume string) {
	cmd := exec.Command("fltmc", "attach", "CldFlt", volume)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outStr := strings.TrimSpace(string(output))
		// Not an error if already attached
		if strings.Contains(outStr, "0x801f0012") || strings.Contains(strings.ToLower(outStr), "already attached") {
			fmt.Println("      Already attached: OK")
			return
		}
		fmt.Printf("      WARNING: fltmc attach failed: %v\n", err)
		if len(outStr) > 0 {
			fmt.Printf("      Output: %s\n", outStr)
		}
		return
	}
	fmt.Println("      OK")
}

// deletePlaceholders walks the path and deletes only placeholder files
// (those with OFFLINE or RECALL_ON_DATA_ACCESS attributes).
func deletePlaceholders(rootPath string, dryRun bool) (deleted, preserved, errors int) {
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("      [SKIP] %s: %v\n", path, err)
			errors++
			return nil
		}
		if info.IsDir() {
			return nil
		}

		if !isPlaceholder(info) {
			preserved++
			return nil
		}

		if dryRun {
			fmt.Printf("      [DRY-RUN] Would delete: %s\n", path)
			deleted++
			return nil
		}

		// Remove read-only attribute if set before deleting
		pathPtr, _ := windows.UTF16PtrFromString(path)
		attrs, attrErr := windows.GetFileAttributes(pathPtr)
		if attrErr == nil && attrs&windows.FILE_ATTRIBUTE_READONLY != 0 {
			windows.SetFileAttributes(pathPtr, attrs&^windows.FILE_ATTRIBUTE_READONLY)
		}

		if err := os.Remove(path); err != nil {
			fmt.Printf("      [ERROR] %s: %v\n", path, err)
			errors++
			return nil
		}
		deleted++
		return nil
	})
	return
}

// cleanEmptyDirs removes empty directories bottom-up.
func cleanEmptyDirs(rootPath string, dryRun bool) int {
	removed := 0

	// Collect all directories first, then process bottom-up
	var dirs []string
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() && path != rootPath {
			dirs = append(dirs, path)
		}
		return nil
	})

	// Process in reverse order (deepest first)
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		if len(entries) == 0 {
			if dryRun {
				fmt.Printf("      [DRY-RUN] Would remove empty dir: %s\n", dir)
				removed++
			} else {
				if err := os.Remove(dir); err == nil {
					removed++
				}
			}
		}
	}
	return removed
}

// isAdmin checks if the current process has administrator privileges.
func isAdmin() bool {
	cmd := exec.Command("net", "session")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	err := cmd.Run()
	return err == nil
}

func decodeHRESULT(hr uint32) {
	switch hr {
	case 0x80070002:
		fmt.Println("      = ERROR_FILE_NOT_FOUND")
	case 0x80070003:
		fmt.Println("      = ERROR_PATH_NOT_FOUND")
	case 0x80070005:
		fmt.Println("      = ERROR_ACCESS_DENIED")
	case 0x8007001F:
		fmt.Println("      = ERROR_GEN_FAILURE (device not functioning)")
	case 0x80070057:
		fmt.Println("      = E_INVALIDARG")
	case 0x80070186:
		fmt.Println("      = ERROR_CLOUD_FILE_METADATA_CORRUPT")
	case 0x8007018A:
		fmt.Println("      = ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT")
	case 0x8007018B:
		fmt.Println("      = ERROR_CLOUD_FILE_IN_USE")
	case 0x8007019A:
		fmt.Println("      = ERROR_NOT_A_CLOUD_FILE")
	case 0x800701A4:
		fmt.Println("      = ERROR_CLOUD_FILE_PROVIDER_NOT_RUNNING")
	default:
		if hr&0xFFFF0000 == 0x80070000 {
			fmt.Printf("      = Win32 Error %d\n", hr&0xFFFF)
		}
	}
}
