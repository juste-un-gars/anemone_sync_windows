// Package cloudfiles provides Go bindings for the Windows Cloud Files API (cfapi.dll).
// This enables "Files On Demand" functionality similar to OneDrive.
//
// Requirements:
//   - Windows 10 version 1709+ (Fall Creators Update)
//   - NTFS volumes only
//
// Reference:
//   - https://learn.microsoft.com/en-us/windows/win32/cfapi/cloud-files-api-portal
//   - https://github.com/microsoft/Windows-classic-samples/tree/main/Samples/CloudMirror
package cloudfiles

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// GUID represents a Windows GUID structure.
type GUID struct {
	Data1 uint32
	Data2 uint16
	Data3 uint16
	Data4 [8]byte
}

// CF_CONNECTION_KEY is an opaque handle returned by CfConnectSyncRoot.
type CF_CONNECTION_KEY int64

// CF_TRANSFER_KEY is used to identify a specific file operation.
type CF_TRANSFER_KEY int64

// --- Hydration Policy ---

// CF_HYDRATION_POLICY_PRIMARY defines how placeholder files should be hydrated.
type CF_HYDRATION_POLICY_PRIMARY uint16

const (
	CF_HYDRATION_POLICY_PARTIAL     CF_HYDRATION_POLICY_PRIMARY = 0 // Partial hydration, no background completion
	CF_HYDRATION_POLICY_PROGRESSIVE CF_HYDRATION_POLICY_PRIMARY = 1 // Progressive hydration with background completion
	CF_HYDRATION_POLICY_FULL        CF_HYDRATION_POLICY_PRIMARY = 2 // Full hydration before completing IO
	CF_HYDRATION_POLICY_ALWAYS_FULL CF_HYDRATION_POLICY_PRIMARY = 3 // Always fully hydrated, no placeholders
)

// CF_HYDRATION_POLICY_MODIFIER are optional modifiers for hydration policy.
type CF_HYDRATION_POLICY_MODIFIER uint16

const (
	CF_HYDRATION_POLICY_MODIFIER_NONE                       CF_HYDRATION_POLICY_MODIFIER = 0x0000
	CF_HYDRATION_POLICY_MODIFIER_VALIDATION_REQUIRED        CF_HYDRATION_POLICY_MODIFIER = 0x0001
	CF_HYDRATION_POLICY_MODIFIER_STREAMING_ALLOWED          CF_HYDRATION_POLICY_MODIFIER = 0x0002
	CF_HYDRATION_POLICY_MODIFIER_AUTO_DEHYDRATION_ALLOWED   CF_HYDRATION_POLICY_MODIFIER = 0x0004
	CF_HYDRATION_POLICY_MODIFIER_ALLOW_FULL_RESTART_HYDRATION CF_HYDRATION_POLICY_MODIFIER = 0x0008
)

// CF_HYDRATION_POLICY combines primary policy and modifiers.
type CF_HYDRATION_POLICY struct {
	Primary  CF_HYDRATION_POLICY_PRIMARY
	Modifier CF_HYDRATION_POLICY_MODIFIER
}

// --- Population Policy ---

// CF_POPULATION_POLICY_PRIMARY defines how placeholder namespace is created.
type CF_POPULATION_POLICY_PRIMARY uint16

const (
	CF_POPULATION_POLICY_PARTIAL     CF_POPULATION_POLICY_PRIMARY = 0 // On-demand population
	CF_POPULATION_POLICY_FULL        CF_POPULATION_POLICY_PRIMARY = 1 // Request all entries before completing
	CF_POPULATION_POLICY_ALWAYS_FULL CF_POPULATION_POLICY_PRIMARY = 2 // Full namespace always available
)

// CF_POPULATION_POLICY_MODIFIER are optional modifiers for population policy.
type CF_POPULATION_POLICY_MODIFIER uint16

const (
	CF_POPULATION_POLICY_MODIFIER_NONE CF_POPULATION_POLICY_MODIFIER = 0x0000
)

// CF_POPULATION_POLICY combines primary policy and modifiers.
type CF_POPULATION_POLICY struct {
	Primary  CF_POPULATION_POLICY_PRIMARY
	Modifier CF_POPULATION_POLICY_MODIFIER
}

// --- InSync Policy ---

// CF_INSYNC_POLICY defines when in-sync state should be cleared.
type CF_INSYNC_POLICY uint32

const (
	CF_INSYNC_POLICY_NONE                           CF_INSYNC_POLICY = 0x00000000
	CF_INSYNC_POLICY_TRACK_FILE_CREATION_TIME       CF_INSYNC_POLICY = 0x00000001
	CF_INSYNC_POLICY_TRACK_FILE_READONLY_ATTRIBUTE  CF_INSYNC_POLICY = 0x00000002
	CF_INSYNC_POLICY_TRACK_FILE_HIDDEN_ATTRIBUTE    CF_INSYNC_POLICY = 0x00000004
	CF_INSYNC_POLICY_TRACK_FILE_SYSTEM_ATTRIBUTE    CF_INSYNC_POLICY = 0x00000008
	CF_INSYNC_POLICY_TRACK_DIRECTORY_CREATION_TIME  CF_INSYNC_POLICY = 0x00000010
	CF_INSYNC_POLICY_TRACK_DIRECTORY_READONLY_ATTRIBUTE CF_INSYNC_POLICY = 0x00000020
	CF_INSYNC_POLICY_TRACK_DIRECTORY_HIDDEN_ATTRIBUTE   CF_INSYNC_POLICY = 0x00000040
	CF_INSYNC_POLICY_TRACK_DIRECTORY_SYSTEM_ATTRIBUTE   CF_INSYNC_POLICY = 0x00000080
	CF_INSYNC_POLICY_TRACK_FILE_LAST_WRITE_TIME         CF_INSYNC_POLICY = 0x00000100
	CF_INSYNC_POLICY_TRACK_DIRECTORY_LAST_WRITE_TIME    CF_INSYNC_POLICY = 0x00000200
	CF_INSYNC_POLICY_TRACK_FILE_ALL                     CF_INSYNC_POLICY = 0x0055550F
	CF_INSYNC_POLICY_TRACK_DIRECTORY_ALL                CF_INSYNC_POLICY = 0x00AAAAF0
	CF_INSYNC_POLICY_TRACK_ALL                          CF_INSYNC_POLICY = 0x00FFFFFF
	CF_INSYNC_POLICY_PRESERVE_INSYNC_FOR_SYNC_ENGINE    CF_INSYNC_POLICY = 0x80000000
)

// --- HardLink Policy ---

// CF_HARDLINK_POLICY defines hard link support on placeholders.
type CF_HARDLINK_POLICY uint32

const (
	CF_HARDLINK_POLICY_NONE    CF_HARDLINK_POLICY = 0x00000000
	CF_HARDLINK_POLICY_ALLOWED CF_HARDLINK_POLICY = 0x00000001
)

// --- Placeholder Management Policy ---

// CF_PLACEHOLDER_MANAGEMENT_POLICY controls non-provider placeholder operations.
type CF_PLACEHOLDER_MANAGEMENT_POLICY uint32

const (
	CF_PLACEHOLDER_MANAGEMENT_POLICY_DEFAULT              CF_PLACEHOLDER_MANAGEMENT_POLICY = 0x00000000
	CF_PLACEHOLDER_MANAGEMENT_POLICY_CREATE_UNRESTRICTED  CF_PLACEHOLDER_MANAGEMENT_POLICY = 0x00000001
	CF_PLACEHOLDER_MANAGEMENT_POLICY_CONVERT_UNRESTRICTED CF_PLACEHOLDER_MANAGEMENT_POLICY = 0x00000002
	CF_PLACEHOLDER_MANAGEMENT_POLICY_UPDATE_UNRESTRICTED  CF_PLACEHOLDER_MANAGEMENT_POLICY = 0x00000004
)

// --- Sync Policies ---

// CF_SYNC_POLICIES contains all sync policies for a sync root.
type CF_SYNC_POLICIES struct {
	StructSize            uint32
	Hydration             CF_HYDRATION_POLICY
	Population            CF_POPULATION_POLICY
	InSync                CF_INSYNC_POLICY
	HardLink              CF_HARDLINK_POLICY
	PlaceholderManagement CF_PLACEHOLDER_MANAGEMENT_POLICY
}

// NewDefaultSyncPolicies creates default sync policies for Files On Demand.
func NewDefaultSyncPolicies() *CF_SYNC_POLICIES {
	policies := &CF_SYNC_POLICIES{
		Hydration: CF_HYDRATION_POLICY{
			Primary:  CF_HYDRATION_POLICY_FULL,
			Modifier: CF_HYDRATION_POLICY_MODIFIER_AUTO_DEHYDRATION_ALLOWED,
		},
		Population: CF_POPULATION_POLICY{
			// Use ALWAYS_FULL - provider pre-populates placeholders, no FETCH_PLACEHOLDERS callback needed
			// With this policy, we should NOT register FETCH_PLACEHOLDERS callback
			Primary:  CF_POPULATION_POLICY_ALWAYS_FULL,
			Modifier: CF_POPULATION_POLICY_MODIFIER_NONE,
		},
		InSync:   CF_INSYNC_POLICY_TRACK_ALL,
		HardLink: CF_HARDLINK_POLICY_NONE,
		// Allow non-provider processes to create/convert/update placeholders
		// This prevents file creation from being blocked in the sync root
		PlaceholderManagement: CF_PLACEHOLDER_MANAGEMENT_POLICY_CREATE_UNRESTRICTED |
			CF_PLACEHOLDER_MANAGEMENT_POLICY_CONVERT_UNRESTRICTED |
			CF_PLACEHOLDER_MANAGEMENT_POLICY_UPDATE_UNRESTRICTED,
	}
	policies.StructSize = uint32(unsafe.Sizeof(*policies))
	return policies
}

// --- Sync Registration ---

// CF_SYNC_REGISTRATION contains information about the sync provider.
type CF_SYNC_REGISTRATION struct {
	StructSize             uint32
	ProviderName           *uint16 // LPCWSTR
	ProviderVersion        *uint16 // LPCWSTR
	SyncRootIdentity       unsafe.Pointer
	SyncRootIdentityLength uint32
	FileIdentity           unsafe.Pointer
	FileIdentityLength     uint32
	ProviderId             GUID
}

// NewSyncRegistration creates a new sync registration with the given provider info.
func NewSyncRegistration(providerName, providerVersion string) *CF_SYNC_REGISTRATION {
	namePtr, _ := windows.UTF16PtrFromString(providerName)
	versionPtr, _ := windows.UTF16PtrFromString(providerVersion)

	reg := &CF_SYNC_REGISTRATION{
		ProviderName:    namePtr,
		ProviderVersion: versionPtr,
	}
	reg.StructSize = uint32(unsafe.Sizeof(*reg))
	return reg
}

// --- Register Flags ---

// CF_REGISTER_FLAGS are flags for CfRegisterSyncRoot.
type CF_REGISTER_FLAGS uint32

const (
	CF_REGISTER_FLAG_NONE                                  CF_REGISTER_FLAGS = 0x00000000
	CF_REGISTER_FLAG_UPDATE                                CF_REGISTER_FLAGS = 0x00000001
	CF_REGISTER_FLAG_DISABLE_ON_DEMAND_POPULATION_ON_ROOT  CF_REGISTER_FLAGS = 0x00000002
	CF_REGISTER_FLAG_MARK_IN_SYNC_ON_ROOT                  CF_REGISTER_FLAGS = 0x00000004
)

// --- Connect Flags ---

// CF_CONNECT_FLAGS are flags for CfConnectSyncRoot.
type CF_CONNECT_FLAGS uint32

const (
	CF_CONNECT_FLAG_NONE                       CF_CONNECT_FLAGS = 0x00000000
	CF_CONNECT_FLAG_REQUIRE_PROCESS_INFO       CF_CONNECT_FLAGS = 0x00000002
	CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH     CF_CONNECT_FLAGS = 0x00000004
	CF_CONNECT_FLAG_BLOCK_SELF_IMPLICIT_HYDRATION CF_CONNECT_FLAGS = 0x00000008
)

// --- Callback Types ---

// CF_CALLBACK_TYPE identifies the type of callback.
type CF_CALLBACK_TYPE uint32

const (
	CF_CALLBACK_TYPE_FETCH_DATA               CF_CALLBACK_TYPE = 0
	CF_CALLBACK_TYPE_VALIDATE_DATA            CF_CALLBACK_TYPE = 1
	CF_CALLBACK_TYPE_CANCEL_FETCH_DATA        CF_CALLBACK_TYPE = 2
	CF_CALLBACK_TYPE_FETCH_PLACEHOLDERS       CF_CALLBACK_TYPE = 3
	CF_CALLBACK_TYPE_CANCEL_FETCH_PLACEHOLDERS CF_CALLBACK_TYPE = 4
	CF_CALLBACK_TYPE_NOTIFY_FILE_OPEN_COMPLETION    CF_CALLBACK_TYPE = 5
	CF_CALLBACK_TYPE_NOTIFY_FILE_CLOSE_COMPLETION   CF_CALLBACK_TYPE = 6
	CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE               CF_CALLBACK_TYPE = 7
	CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE_COMPLETION    CF_CALLBACK_TYPE = 8
	CF_CALLBACK_TYPE_NOTIFY_DELETE                  CF_CALLBACK_TYPE = 9
	CF_CALLBACK_TYPE_NOTIFY_DELETE_COMPLETION       CF_CALLBACK_TYPE = 10
	CF_CALLBACK_TYPE_NOTIFY_RENAME                  CF_CALLBACK_TYPE = 11
	CF_CALLBACK_TYPE_NOTIFY_RENAME_COMPLETION       CF_CALLBACK_TYPE = 12
	CF_CALLBACK_TYPE_NONE                           CF_CALLBACK_TYPE = 0xFFFFFFFF
)

// CF_CALLBACK_INFO contains information passed to callbacks.
type CF_CALLBACK_INFO struct {
	StructSize          uint32
	ConnectionKey       CF_CONNECTION_KEY
	CallbackContext     unsafe.Pointer
	VolumeGuidName      *uint16 // LPCWSTR
	VolumeDosName       *uint16 // LPCWSTR
	VolumeSerialNumber  uint32
	SyncRootFileId      int64
	SyncRootIdentity    unsafe.Pointer
	SyncRootIdentityLength uint32
	FileId              int64
	FileSize            int64
	FileIdentity        unsafe.Pointer
	FileIdentityLength  uint32
	NormalizedPath      *uint16 // LPCWSTR
	TransferKey         CF_TRANSFER_KEY
	PriorityHint        byte
	_                   [3]byte // padding
	CorrelationVector   unsafe.Pointer
	ProcessInfo         unsafe.Pointer
	RequestKey          int64
}

// CF_CALLBACK_PARAMETERS contains parameters specific to each callback type.
type CF_CALLBACK_PARAMETERS struct {
	ParamSize uint32
	// Union of different parameter types - we'll use the largest
	Data [64]byte
}

// CF_CALLBACK is the callback function signature.
type CF_CALLBACK func(callbackInfo *CF_CALLBACK_INFO, callbackParameters *CF_CALLBACK_PARAMETERS)

// CF_CALLBACK_REGISTRATION registers a callback for a specific type.
type CF_CALLBACK_REGISTRATION struct {
	Type     CF_CALLBACK_TYPE
	Callback uintptr // CF_CALLBACK
}

// CF_CALLBACK_REGISTRATION_END marks the end of callback registration array.
var CF_CALLBACK_REGISTRATION_END = CF_CALLBACK_REGISTRATION{
	Type:     CF_CALLBACK_TYPE_NONE,
	Callback: 0,
}

// --- Placeholder State ---

// CF_PLACEHOLDER_STATE represents the state of a placeholder file.
type CF_PLACEHOLDER_STATE uint32

const (
	CF_PLACEHOLDER_STATE_NO_STATES         CF_PLACEHOLDER_STATE = 0x00000000
	CF_PLACEHOLDER_STATE_PLACEHOLDER       CF_PLACEHOLDER_STATE = 0x00000001
	CF_PLACEHOLDER_STATE_SYNC_ROOT         CF_PLACEHOLDER_STATE = 0x00000002
	CF_PLACEHOLDER_STATE_ESSENTIAL_PROP_PRESENT CF_PLACEHOLDER_STATE = 0x00000004
	CF_PLACEHOLDER_STATE_IN_SYNC           CF_PLACEHOLDER_STATE = 0x00000008
	CF_PLACEHOLDER_STATE_PARTIAL           CF_PLACEHOLDER_STATE = 0x00000010
	CF_PLACEHOLDER_STATE_PARTIALLY_ON_DISK CF_PLACEHOLDER_STATE = 0x00000020
	CF_PLACEHOLDER_STATE_INVALID           CF_PLACEHOLDER_STATE = 0xFFFFFFFF
)

// --- Placeholder Create Info ---

// CF_PLACEHOLDER_CREATE_INFO contains information for creating a placeholder.
type CF_PLACEHOLDER_CREATE_INFO struct {
	RelativeFileName     *uint16 // LPCWSTR
	FsMetadata           CF_FS_METADATA
	FileIdentity         unsafe.Pointer
	FileIdentityLength   uint32
	Flags                CF_PLACEHOLDER_CREATE_FLAGS
	Result               int32 // HRESULT
	CreateUsn            int64
}

// CF_FS_METADATA contains file system metadata for a placeholder.
type CF_FS_METADATA struct {
	FileSize         int64
	BasicInfo        FILE_BASIC_INFO
}

// FILE_BASIC_INFO contains basic file information.
type FILE_BASIC_INFO struct {
	CreationTime   int64 // FILETIME as int64
	LastAccessTime int64
	LastWriteTime  int64
	ChangeTime     int64
	FileAttributes uint32
	_              uint32 // padding
}

// CF_PLACEHOLDER_CREATE_FLAGS are flags for creating placeholders.
type CF_PLACEHOLDER_CREATE_FLAGS uint32

const (
	CF_PLACEHOLDER_CREATE_FLAG_NONE                    CF_PLACEHOLDER_CREATE_FLAGS = 0x00000000
	CF_PLACEHOLDER_CREATE_FLAG_DISABLE_ON_DEMAND_POPULATION CF_PLACEHOLDER_CREATE_FLAGS = 0x00000001
	CF_PLACEHOLDER_CREATE_FLAG_MARK_IN_SYNC            CF_PLACEHOLDER_CREATE_FLAGS = 0x00000002
	CF_PLACEHOLDER_CREATE_FLAG_SUPERSEDE               CF_PLACEHOLDER_CREATE_FLAGS = 0x00000004
	CF_PLACEHOLDER_CREATE_FLAG_ALWAYS_FULL             CF_PLACEHOLDER_CREATE_FLAGS = 0x00000008
)

// --- Operation Info ---

// CF_OPERATION_INFO contains information for placeholder operations.
type CF_OPERATION_INFO struct {
	StructSize       uint32
	Type             CF_OPERATION_TYPE
	ConnectionKey    CF_CONNECTION_KEY
	TransferKey      CF_TRANSFER_KEY
	CorrelationVector unsafe.Pointer
	SyncStatus       unsafe.Pointer
	RequestKey       int64
}

// CF_OPERATION_TYPE identifies the type of operation.
type CF_OPERATION_TYPE uint32

const (
	CF_OPERATION_TYPE_TRANSFER_DATA        CF_OPERATION_TYPE = 0
	CF_OPERATION_TYPE_RETRIEVE_DATA        CF_OPERATION_TYPE = 1
	CF_OPERATION_TYPE_ACK_DATA             CF_OPERATION_TYPE = 2
	CF_OPERATION_TYPE_RESTART_HYDRATION    CF_OPERATION_TYPE = 3
	CF_OPERATION_TYPE_TRANSFER_PLACEHOLDERS CF_OPERATION_TYPE = 4
	CF_OPERATION_TYPE_ACK_DEHYDRATE        CF_OPERATION_TYPE = 5
	CF_OPERATION_TYPE_ACK_DELETE           CF_OPERATION_TYPE = 6
	CF_OPERATION_TYPE_ACK_RENAME           CF_OPERATION_TYPE = 7
)

// --- Operation Parameters ---

// CF_OPERATION_PARAMETERS contains parameters for operations.
type CF_OPERATION_PARAMETERS struct {
	ParamSize uint32
	// Union - using largest size
	Data [128]byte
}

// CF_OPERATION_TRANSFER_DATA_PARAMS for TRANSFER_DATA operation.
type CF_OPERATION_TRANSFER_DATA_PARAMS struct {
	ParamSize      uint32
	Flags          uint32
	CompletionStatus int32
	Buffer         unsafe.Pointer
	Offset         int64
	Length         int64
}
