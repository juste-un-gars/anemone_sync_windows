//go:build windows
// +build windows

#include "cfapi_bridge.h"
#include <stdio.h>
#include <string.h>
#include <stdarg.h>
#include <time.h>

// --- Debug logging ---
static int g_debugLogging = 1; // Enable verbose debug logging

static void DebugLog(const char* format, ...) {
    if (!g_debugLogging) return;

    // Get timestamp
    time_t now = time(NULL);
    struct tm* t = localtime(&now);

    fprintf(stderr, "[CFAPI %02d:%02d:%02d] ", t->tm_hour, t->tm_min, t->tm_sec);

    va_list args;
    va_start(args, format);
    vfprintf(stderr, format, args);
    va_end(args);

    fprintf(stderr, "\n");
    fflush(stderr);
}

static void DebugLogW(const wchar_t* prefix, const wchar_t* path) {
    if (!g_debugLogging) return;

    time_t now = time(NULL);
    struct tm* t = localtime(&now);

    fprintf(stderr, "[CFAPI %02d:%02d:%02d] %ls: %ls\n", t->tm_hour, t->tm_min, t->tm_sec, prefix, path ? path : L"(null)");
    fflush(stderr);
}

// --- Cloud Files API definitions (from cfapi.h) ---
// We define these ourselves to avoid SDK dependency

typedef LONGLONG CF_CONNECTION_KEY;
typedef LONGLONG CF_TRANSFER_KEY;

typedef enum {
    CF_CALLBACK_TYPE_FETCH_DATA = 0,
    CF_CALLBACK_TYPE_VALIDATE_DATA = 1,
    CF_CALLBACK_TYPE_CANCEL_FETCH_DATA = 2,
    CF_CALLBACK_TYPE_FETCH_PLACEHOLDERS = 3,
    CF_CALLBACK_TYPE_CANCEL_FETCH_PLACEHOLDERS = 4,
    CF_CALLBACK_TYPE_NOTIFY_FILE_OPEN_COMPLETION = 5,
    CF_CALLBACK_TYPE_NOTIFY_FILE_CLOSE_COMPLETION = 6,
    CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE = 7,
    CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE_COMPLETION = 8,
    CF_CALLBACK_TYPE_NOTIFY_DELETE = 9,
    CF_CALLBACK_TYPE_NOTIFY_DELETE_COMPLETION = 10,
    CF_CALLBACK_TYPE_NOTIFY_RENAME = 11,
    CF_CALLBACK_TYPE_NOTIFY_RENAME_COMPLETION = 12,
    CF_CALLBACK_TYPE_NONE = 0xFFFFFFFF
} CF_CALLBACK_TYPE;

typedef enum {
    CF_CONNECT_FLAG_NONE = 0x00000000,
    CF_CONNECT_FLAG_REQUIRE_PROCESS_INFO = 0x00000002,
    CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH = 0x00000004,
    CF_CONNECT_FLAG_BLOCK_SELF_IMPLICIT_HYDRATION = 0x00000008
} CF_CONNECT_FLAGS;

typedef enum {
    CF_OPERATION_TYPE_TRANSFER_DATA = 0,
    CF_OPERATION_TYPE_RETRIEVE_DATA = 1,
    CF_OPERATION_TYPE_ACK_DATA = 2,
    CF_OPERATION_TYPE_RESTART_HYDRATION = 3,
    CF_OPERATION_TYPE_TRANSFER_PLACEHOLDERS = 4,
    CF_OPERATION_TYPE_ACK_DEHYDRATE = 5,
    CF_OPERATION_TYPE_ACK_DELETE = 6,
    CF_OPERATION_TYPE_ACK_RENAME = 7
} CF_OPERATION_TYPE;

typedef enum {
    CF_CALLBACK_DELETE_FLAG_IS_DIRECTORY = 0x00000001,
    CF_CALLBACK_DELETE_FLAG_IS_UNDELETE = 0x00000002
} CF_CALLBACK_DELETE_FLAGS;

typedef enum {
    CF_CALLBACK_RENAME_FLAG_IS_DIRECTORY = 0x00000001,
    CF_CALLBACK_RENAME_FLAG_SOURCE_IN_SCOPE = 0x00000002,
    CF_CALLBACK_RENAME_FLAG_TARGET_IN_SCOPE = 0x00000004
} CF_CALLBACK_RENAME_FLAGS;

// Callback info structure
typedef struct {
    DWORD StructSize;
    CF_CONNECTION_KEY ConnectionKey;
    LPVOID CallbackContext;
    LPCWSTR VolumeGuidName;
    LPCWSTR VolumeDosName;
    DWORD VolumeSerialNumber;
    LONGLONG SyncRootFileId;
    LPVOID SyncRootIdentity;
    DWORD SyncRootIdentityLength;
    LONGLONG FileId;
    LONGLONG FileSize;
    LPVOID FileIdentity;
    DWORD FileIdentityLength;
    LPCWSTR NormalizedPath;
    CF_TRANSFER_KEY TransferKey;
    BYTE PriorityHint;
    BYTE Reserved[3];
    LPVOID CorrelationVector;
    LPVOID ProcessInfo;
    LONGLONG RequestKey;
} CF_CALLBACK_INFO;

// Callback parameter structures
typedef struct {
    DWORD Flags;
    LONGLONG RequiredFileOffset;
    LONGLONG RequiredLength;
    LONGLONG OptionalFileOffset;
    LONGLONG OptionalLength;
    LONGLONG LastDehydrationTime;
    DWORD LastDehydrationReason;
} CF_CALLBACK_PARAMETERS_FETCHDATA;

typedef struct {
    DWORD Flags;
} CF_CALLBACK_PARAMETERS_DELETE;

typedef struct {
    DWORD Flags;
    LPCWSTR TargetPath;
} CF_CALLBACK_PARAMETERS_RENAME;

typedef struct {
    DWORD ParamSize;
    union {
        CF_CALLBACK_PARAMETERS_FETCHDATA FetchData;
        CF_CALLBACK_PARAMETERS_DELETE Delete;
        CF_CALLBACK_PARAMETERS_RENAME Rename;
        BYTE Reserved[64];
    };
} CF_CALLBACK_PARAMETERS;

// Callback function type
typedef void (CALLBACK *CF_CALLBACK)(
    const CF_CALLBACK_INFO* CallbackInfo,
    const CF_CALLBACK_PARAMETERS* CallbackParameters
);

// Callback registration
typedef struct {
    CF_CALLBACK_TYPE Type;
    CF_CALLBACK Callback;
} CF_CALLBACK_REGISTRATION;

// Operation info
typedef struct {
    DWORD StructSize;
    CF_OPERATION_TYPE Type;
    CF_CONNECTION_KEY ConnectionKey;
    CF_TRANSFER_KEY TransferKey;
    LPVOID CorrelationVector;
    LPVOID SyncStatus;
    LONGLONG RequestKey;
} CF_OPERATION_INFO;

// Operation parameters for transfer data
typedef struct {
    LARGE_INTEGER Offset;
    LARGE_INTEGER Length;
    PVOID Buffer;
    HRESULT CompletionStatus;
} CF_OPERATION_TRANSFER_DATA_PARAMS;

// Operation parameters for ack data
typedef struct {
    DWORD Flags;
    HRESULT CompletionStatus;
    LARGE_INTEGER Offset;
    LARGE_INTEGER Length;
} CF_OPERATION_ACK_DATA_PARAMS;

typedef struct {
    DWORD ParamSize;
    union {
        CF_OPERATION_TRANSFER_DATA_PARAMS TransferData;
        CF_OPERATION_ACK_DATA_PARAMS AckData;
        BYTE Reserved[128];
    };
} CF_OPERATION_PARAMETERS;

// --- Internal State ---

// Request queue (circular buffer)
static CfapiBridgeRequest g_requestQueue[CFAPI_BRIDGE_MAX_QUEUE_SIZE];
static int g_queueHead = 0;
static int g_queueTail = 0;
static int g_queueCount = 0;

// Synchronization
static CRITICAL_SECTION g_queueCS;
static HANDLE g_newRequestEvent = NULL;
static int g_initialized = 0;

// Cloud Files API function pointers (loaded dynamically)
typedef HRESULT (WINAPI *PFN_CfConnectSyncRoot)(
    LPCWSTR SyncRootPath,
    const CF_CALLBACK_REGISTRATION* CallbackTable,
    LPCVOID CallbackContext,
    CF_CONNECT_FLAGS ConnectFlags,
    CF_CONNECTION_KEY* ConnectionKey
);

typedef HRESULT (WINAPI *PFN_CfDisconnectSyncRoot)(
    CF_CONNECTION_KEY ConnectionKey
);

typedef HRESULT (WINAPI *PFN_CfExecute)(
    const CF_OPERATION_INFO* OpInfo,
    CF_OPERATION_PARAMETERS* OpParams
);

typedef HRESULT (WINAPI *PFN_CfReportProviderProgress)(
    CF_CONNECTION_KEY ConnectionKey,
    CF_TRANSFER_KEY TransferKey,
    LARGE_INTEGER Total,
    LARGE_INTEGER Completed
);

static HMODULE g_cldapiModule = NULL;
static PFN_CfConnectSyncRoot g_pfnCfConnectSyncRoot = NULL;
static PFN_CfDisconnectSyncRoot g_pfnCfDisconnectSyncRoot = NULL;
static PFN_CfExecute g_pfnCfExecute = NULL;
static PFN_CfReportProviderProgress g_pfnCfReportProviderProgress = NULL;

// --- Internal Functions ---

// Enqueue a request (thread-safe)
static int EnqueueRequest(const CfapiBridgeRequest* request) {
    EnterCriticalSection(&g_queueCS);

    if (g_queueCount >= CFAPI_BRIDGE_MAX_QUEUE_SIZE) {
        LeaveCriticalSection(&g_queueCS);
        return CFAPI_BRIDGE_ERROR_QUEUE_FULL;
    }

    memcpy(&g_requestQueue[g_queueTail], request, sizeof(CfapiBridgeRequest));
    g_queueTail = (g_queueTail + 1) % CFAPI_BRIDGE_MAX_QUEUE_SIZE;
    g_queueCount++;

    LeaveCriticalSection(&g_queueCS);

    // Signal that a new request is available
    SetEvent(g_newRequestEvent);

    return CFAPI_BRIDGE_OK;
}

// Dequeue a request (thread-safe)
static int DequeueRequest(CfapiBridgeRequest* request) {
    EnterCriticalSection(&g_queueCS);

    if (g_queueCount == 0) {
        LeaveCriticalSection(&g_queueCS);
        return CFAPI_BRIDGE_ERROR_QUEUE_EMPTY;
    }

    memcpy(request, &g_requestQueue[g_queueHead], sizeof(CfapiBridgeRequest));
    g_queueHead = (g_queueHead + 1) % CFAPI_BRIDGE_MAX_QUEUE_SIZE;
    g_queueCount--;

    LeaveCriticalSection(&g_queueCS);

    return CFAPI_BRIDGE_OK;
}

// --- Callback Handlers (called by Windows on filter thread) ---

// Helper: Print callback info details
static void PrintCallbackInfo(const char* callbackName, const CF_CALLBACK_INFO* info) {
    DebugLog("=== %s CALLBACK ===", callbackName);
    DebugLog("  ConnectionKey: %lld", (long long)info->ConnectionKey);
    DebugLog("  TransferKey: %lld", (long long)info->TransferKey);
    DebugLog("  FileId: %lld", (long long)info->FileId);
    DebugLog("  FileSize: %lld", (long long)info->FileSize);
    DebugLog("  SyncRootFileId: %lld", (long long)info->SyncRootFileId);
    DebugLog("  FileIdentityLength: %u", info->FileIdentityLength);
    DebugLogW(L"  NormalizedPath", info->NormalizedPath);
    DebugLogW(L"  VolumeDosName", info->VolumeDosName);
}

// FETCH_DATA callback - file needs hydration
static void CALLBACK OnFetchDataCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    PrintCallbackInfo("FETCH_DATA", callbackInfo);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized!");
        return;
    }

    CfapiBridgeRequest req;
    memset(&req, 0, sizeof(req));

    req.type = CFAPI_CALLBACK_FETCH_DATA;
    req.connectionKey = (int64_t)callbackInfo->ConnectionKey;
    req.transferKey = (int64_t)callbackInfo->TransferKey;
    req.fileSize = callbackInfo->FileSize;
    req.isDirectory = 0;

    // Copy normalized path
    if (callbackInfo->NormalizedPath) {
        wcsncpy(req.filePath, callbackInfo->NormalizedPath, CFAPI_BRIDGE_MAX_PATH - 1);
        req.filePath[CFAPI_BRIDGE_MAX_PATH - 1] = L'\0';
    }

    // Extract fetch parameters
    if (callbackParameters && callbackParameters->ParamSize >= sizeof(DWORD) + sizeof(CF_CALLBACK_PARAMETERS_FETCHDATA)) {
        req.requiredOffset = callbackParameters->FetchData.RequiredFileOffset;
        req.requiredLength = callbackParameters->FetchData.RequiredLength;
        DebugLog("  FetchData: offset=%lld, length=%lld",
                 (long long)req.requiredOffset, (long long)req.requiredLength);
    }

    // Enqueue for Go to process
    int result = EnqueueRequest(&req);
    if (result != CFAPI_BRIDGE_OK) {
        DebugLog("ERROR: Queue full, dropping FETCH_DATA request!");
    } else {
        DebugLog("FETCH_DATA enqueued OK, queue count=%d", g_queueCount);
    }
}

// CANCEL_FETCH_DATA callback - hydration was cancelled
static void CALLBACK OnCancelFetchDataCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("CANCEL_FETCH_DATA", callbackInfo);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized!");
        return;
    }

    CfapiBridgeRequest req;
    memset(&req, 0, sizeof(req));

    req.type = CFAPI_CALLBACK_CANCEL_FETCH_DATA;
    req.connectionKey = (int64_t)callbackInfo->ConnectionKey;
    req.transferKey = (int64_t)callbackInfo->TransferKey;

    if (callbackInfo->NormalizedPath) {
        wcsncpy(req.filePath, callbackInfo->NormalizedPath, CFAPI_BRIDGE_MAX_PATH - 1);
        req.filePath[CFAPI_BRIDGE_MAX_PATH - 1] = L'\0';
    }

    EnqueueRequest(&req);
    DebugLog("CANCEL_FETCH_DATA enqueued");
}

// NOTIFY_DELETE callback - file is being deleted
static void CALLBACK OnNotifyDeleteCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    PrintCallbackInfo("NOTIFY_DELETE", callbackInfo);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized!");
        return;
    }

    CfapiBridgeRequest req;
    memset(&req, 0, sizeof(req));

    req.type = CFAPI_CALLBACK_NOTIFY_DELETE;
    req.connectionKey = (int64_t)callbackInfo->ConnectionKey;
    req.transferKey = (int64_t)callbackInfo->TransferKey;

    if (callbackInfo->NormalizedPath) {
        wcsncpy(req.filePath, callbackInfo->NormalizedPath, CFAPI_BRIDGE_MAX_PATH - 1);
        req.filePath[CFAPI_BRIDGE_MAX_PATH - 1] = L'\0';
    }

    // Check if directory from parameters
    if (callbackParameters && callbackParameters->ParamSize >= sizeof(DWORD) + sizeof(CF_CALLBACK_PARAMETERS_DELETE)) {
        req.isDirectory = (callbackParameters->Delete.Flags & CF_CALLBACK_DELETE_FLAG_IS_DIRECTORY) ? 1 : 0;
        DebugLog("  IsDirectory: %d", req.isDirectory);
    }

    EnqueueRequest(&req);
    DebugLog("NOTIFY_DELETE enqueued");
}

// Debounce for FETCH_PLACEHOLDERS - track last call time per path
static DWORD g_lastFetchPlaceholdersTime = 0;
static wchar_t g_lastFetchPlaceholdersPath[CFAPI_BRIDGE_MAX_PATH] = {0};

// FETCH_PLACEHOLDERS callback - Windows wants us to populate a directory
// CRITICAL: Must ALWAYS respond to this callback, never return without acknowledging!
static void CALLBACK OnFetchPlaceholdersCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("FETCH_PLACEHOLDERS", callbackInfo);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized!");
        // Still need to respond even on error!
    }

    DebugLog("FETCH_PLACEHOLDERS: Acknowledging with TRANSFER_PLACEHOLDERS...");

    // ALWAYS acknowledge the callback - Windows will freeze if we don't respond!
    int result = CfapiBridgeAckFetchPlaceholders(
        (int64_t)callbackInfo->ConnectionKey,
        (int64_t)callbackInfo->TransferKey
    );

    DebugLog("FETCH_PLACEHOLDERS ack result: %d", result);
}

// CANCEL_FETCH_PLACEHOLDERS callback
static void CALLBACK OnCancelFetchPlaceholdersCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("CANCEL_FETCH_PLACEHOLDERS", callbackInfo);
}

// NOTIFY_FILE_OPEN_COMPLETION callback - file was opened
static void CALLBACK OnNotifyFileOpenCompletionCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("FILE_OPEN_COMPLETION", callbackInfo);
    // Info only - no action needed
}

// NOTIFY_FILE_CLOSE_COMPLETION callback - file was closed
static void CALLBACK OnNotifyFileCloseCompletionCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("FILE_CLOSE_COMPLETION", callbackInfo);
    // Info only - no action needed
}

// NOTIFY_DEHYDRATE callback - file is being dehydrated
static void CALLBACK OnNotifyDehydrateCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("NOTIFY_DEHYDRATE", callbackInfo);
    // Info only - no action needed
}

// NOTIFY_DEHYDRATE_COMPLETION callback - file dehydration completed
static void CALLBACK OnNotifyDehydrateCompletionCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("NOTIFY_DEHYDRATE_COMPLETION", callbackInfo);
    // Info only - no action needed
}

// NOTIFY_RENAME callback - file is being renamed
static void CALLBACK OnNotifyRenameCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    PrintCallbackInfo("NOTIFY_RENAME", callbackInfo);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized!");
        return;
    }

    CfapiBridgeRequest req;
    memset(&req, 0, sizeof(req));

    req.type = CFAPI_CALLBACK_NOTIFY_RENAME;
    req.connectionKey = (int64_t)callbackInfo->ConnectionKey;
    req.transferKey = (int64_t)callbackInfo->TransferKey;

    // Source path
    if (callbackInfo->NormalizedPath) {
        wcsncpy(req.filePath, callbackInfo->NormalizedPath, CFAPI_BRIDGE_MAX_PATH - 1);
        req.filePath[CFAPI_BRIDGE_MAX_PATH - 1] = L'\0';
    }

    // Target path from parameters
    if (callbackParameters && callbackParameters->ParamSize >= sizeof(DWORD) + sizeof(CF_CALLBACK_PARAMETERS_RENAME)) {
        if (callbackParameters->Rename.TargetPath) {
            wcsncpy(req.targetPath, callbackParameters->Rename.TargetPath, CFAPI_BRIDGE_MAX_PATH - 1);
            req.targetPath[CFAPI_BRIDGE_MAX_PATH - 1] = L'\0';
            DebugLogW(L"  TargetPath", req.targetPath);
        }
        req.isDirectory = (callbackParameters->Rename.Flags & CF_CALLBACK_RENAME_FLAG_IS_DIRECTORY) ? 1 : 0;
        DebugLog("  IsDirectory: %d", req.isDirectory);
    }

    EnqueueRequest(&req);
    DebugLog("NOTIFY_RENAME enqueued");
}

// VALIDATE_DATA callback - Windows wants to validate data before allowing access
static void CALLBACK OnValidateDataCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("VALIDATE_DATA", callbackInfo);
    DebugLog("VALIDATE_DATA: Acknowledging validation...");

    // Acknowledge validation - required to not block access!
    // We need to call CfExecute with ACK_DATA to acknowledge
    if (g_pfnCfExecute) {
        CF_OPERATION_INFO opInfo;
        memset(&opInfo, 0, sizeof(opInfo));
        opInfo.StructSize = sizeof(CF_OPERATION_INFO);
        opInfo.Type = CF_OPERATION_TYPE_ACK_DATA;
        opInfo.ConnectionKey = callbackInfo->ConnectionKey;
        opInfo.TransferKey = callbackInfo->TransferKey;

        CF_OPERATION_PARAMETERS opParams;
        memset(&opParams, 0, sizeof(opParams));
        opParams.ParamSize = sizeof(CF_OPERATION_PARAMETERS);
        opParams.AckData.Flags = 0;
        opParams.AckData.CompletionStatus = S_OK;

        HRESULT hr = g_pfnCfExecute(&opInfo, &opParams);
        DebugLog("VALIDATE_DATA ack result: HRESULT=0x%08lX", hr);
    }
}

// NOTIFY_DELETE_COMPLETION callback - file deletion completed
static void CALLBACK OnNotifyDeleteCompletionCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("NOTIFY_DELETE_COMPLETION", callbackInfo);
    // Info only - no action needed
}

// NOTIFY_RENAME_COMPLETION callback - file rename completed
static void CALLBACK OnNotifyRenameCompletionCallback(
    const CF_CALLBACK_INFO* callbackInfo,
    const CF_CALLBACK_PARAMETERS* callbackParameters
) {
    (void)callbackParameters;
    PrintCallbackInfo("NOTIFY_RENAME_COMPLETION", callbackInfo);
    // Info only - no action needed
}

// --- Public API ---

int32_t CfapiBridgeInit(void) {
    DebugLog("CfapiBridgeInit called");

    if (g_initialized) {
        DebugLog("Already initialized");
        return CFAPI_BRIDGE_OK;
    }

    // Load cldapi.dll
    g_cldapiModule = LoadLibraryW(L"cldapi.dll");
    if (!g_cldapiModule) {
        DebugLog("ERROR: Failed to load cldapi.dll (error=%lu)", GetLastError());
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }
    DebugLog("cldapi.dll loaded OK");

    // Get function pointers
    g_pfnCfConnectSyncRoot = (PFN_CfConnectSyncRoot)GetProcAddress(g_cldapiModule, "CfConnectSyncRoot");
    g_pfnCfDisconnectSyncRoot = (PFN_CfDisconnectSyncRoot)GetProcAddress(g_cldapiModule, "CfDisconnectSyncRoot");
    g_pfnCfExecute = (PFN_CfExecute)GetProcAddress(g_cldapiModule, "CfExecute");
    g_pfnCfReportProviderProgress = (PFN_CfReportProviderProgress)GetProcAddress(g_cldapiModule, "CfReportProviderProgress");

    DebugLog("Function pointers: CfConnectSyncRoot=%p, CfDisconnectSyncRoot=%p, CfExecute=%p",
             (void*)g_pfnCfConnectSyncRoot, (void*)g_pfnCfDisconnectSyncRoot, (void*)g_pfnCfExecute);

    if (!g_pfnCfConnectSyncRoot || !g_pfnCfDisconnectSyncRoot || !g_pfnCfExecute) {
        DebugLog("ERROR: Failed to get function pointers");
        FreeLibrary(g_cldapiModule);
        g_cldapiModule = NULL;
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    // Initialize critical section
    InitializeCriticalSection(&g_queueCS);

    // Create event for signaling new requests
    g_newRequestEvent = CreateEventW(NULL, FALSE, FALSE, NULL);
    if (!g_newRequestEvent) {
        DebugLog("ERROR: Failed to create event (error=%lu)", GetLastError());
        DeleteCriticalSection(&g_queueCS);
        FreeLibrary(g_cldapiModule);
        g_cldapiModule = NULL;
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    // Reset queue
    g_queueHead = 0;
    g_queueTail = 0;
    g_queueCount = 0;

    g_initialized = 1;
    DebugLog("CfapiBridgeInit SUCCESS");
    return CFAPI_BRIDGE_OK;
}

void CfapiBridgeCleanup(void) {
    if (!g_initialized) return;

    g_initialized = 0;

    if (g_newRequestEvent) {
        CloseHandle(g_newRequestEvent);
        g_newRequestEvent = NULL;
    }

    DeleteCriticalSection(&g_queueCS);

    if (g_cldapiModule) {
        FreeLibrary(g_cldapiModule);
        g_cldapiModule = NULL;
    }

    g_pfnCfConnectSyncRoot = NULL;
    g_pfnCfDisconnectSyncRoot = NULL;
    g_pfnCfExecute = NULL;
    g_pfnCfReportProviderProgress = NULL;
}

int32_t CfapiBridgeConnect(
    const wchar_t* syncRootPath,
    void* callbackContext,
    int64_t* connectionKey
) {
    (void)callbackContext; // unused for now

    DebugLogW(L"CfapiBridgeConnect", syncRootPath);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized");
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    if (!syncRootPath || !connectionKey) {
        DebugLog("ERROR: Invalid parameters");
        return CFAPI_BRIDGE_ERROR_INVALID_PARAM;
    }

    // Build callback registration table - ALL callbacks for debugging
    // We need to find which callback is blocking directory enumeration
    CF_CALLBACK_REGISTRATION callbacks[16];
    int idx = 0;

    // FETCH_DATA (0) - for hydration
    callbacks[idx].Type = CF_CALLBACK_TYPE_FETCH_DATA;
    callbacks[idx].Callback = OnFetchDataCallback;
    DebugLog("  [%d] FETCH_DATA", idx);
    idx++;

    // VALIDATE_DATA (1) - might be required for directory access!
    callbacks[idx].Type = CF_CALLBACK_TYPE_VALIDATE_DATA;
    callbacks[idx].Callback = OnValidateDataCallback;
    DebugLog("  [%d] VALIDATE_DATA", idx);
    idx++;

    // CANCEL_FETCH_DATA (2)
    callbacks[idx].Type = CF_CALLBACK_TYPE_CANCEL_FETCH_DATA;
    callbacks[idx].Callback = OnCancelFetchDataCallback;
    DebugLog("  [%d] CANCEL_FETCH_DATA", idx);
    idx++;

    // NOTE: FETCH_PLACEHOLDERS and CANCEL_FETCH_PLACEHOLDERS are NOT registered
    // because we use CF_POPULATION_POLICY_ALWAYS_FULL which means:
    // "Provider pre-populates placeholders, Windows should not call FETCH_PLACEHOLDERS"
    DebugLog("  [SKIP] FETCH_PLACEHOLDERS (using ALWAYS_FULL policy)");
    DebugLog("  [SKIP] CANCEL_FETCH_PLACEHOLDERS (using ALWAYS_FULL policy)");

    // NOTIFY callbacks
    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_FILE_OPEN_COMPLETION;
    callbacks[idx].Callback = OnNotifyFileOpenCompletionCallback;
    DebugLog("  [%d] NOTIFY_FILE_OPEN_COMPLETION", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_FILE_CLOSE_COMPLETION;
    callbacks[idx].Callback = OnNotifyFileCloseCompletionCallback;
    DebugLog("  [%d] NOTIFY_FILE_CLOSE_COMPLETION", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE;
    callbacks[idx].Callback = OnNotifyDehydrateCallback;
    DebugLog("  [%d] NOTIFY_DEHYDRATE", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_DEHYDRATE_COMPLETION;
    callbacks[idx].Callback = OnNotifyDehydrateCompletionCallback;
    DebugLog("  [%d] NOTIFY_DEHYDRATE_COMPLETION", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_DELETE;
    callbacks[idx].Callback = OnNotifyDeleteCallback;
    DebugLog("  [%d] NOTIFY_DELETE", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_DELETE_COMPLETION;
    callbacks[idx].Callback = OnNotifyDeleteCompletionCallback;
    DebugLog("  [%d] NOTIFY_DELETE_COMPLETION", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_RENAME;
    callbacks[idx].Callback = OnNotifyRenameCallback;
    DebugLog("  [%d] NOTIFY_RENAME", idx);
    idx++;

    callbacks[idx].Type = CF_CALLBACK_TYPE_NOTIFY_RENAME_COMPLETION;
    callbacks[idx].Callback = OnNotifyRenameCompletionCallback;
    DebugLog("  [%d] NOTIFY_RENAME_COMPLETION", idx);
    idx++;

    // Terminator
    callbacks[idx].Type = CF_CALLBACK_TYPE_NONE;
    callbacks[idx].Callback = NULL;

    DebugLog("Calling CfConnectSyncRoot with %d callbacks (ALL for debugging)...", idx);

    // Use BOTH flags like CloudMirror sample:
    // - CF_CONNECT_FLAG_REQUIRE_PROCESS_INFO: get process info in callbacks
    // - CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH: get full path in callbacks
    CF_CONNECT_FLAGS connectFlags = CF_CONNECT_FLAG_REQUIRE_PROCESS_INFO | CF_CONNECT_FLAG_REQUIRE_FULL_FILE_PATH;

    CF_CONNECTION_KEY connKey;
    HRESULT hr = g_pfnCfConnectSyncRoot(
        syncRootPath,
        callbacks,
        NULL,
        connectFlags,
        &connKey
    );

    if (FAILED(hr)) {
        DebugLog("ERROR: CfConnectSyncRoot FAILED: HRESULT=0x%08lX", hr);
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    DebugLog("CfConnectSyncRoot SUCCESS, connectionKey=%lld", (long long)connKey);

    *connectionKey = (int64_t)connKey;
    return CFAPI_BRIDGE_OK;
}

int32_t CfapiBridgeDisconnect(int64_t connectionKey) {
    DebugLog("CfapiBridgeDisconnect: connectionKey=%lld", (long long)connectionKey);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized");
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    HRESULT hr = g_pfnCfDisconnectSyncRoot((CF_CONNECTION_KEY)connectionKey);
    if (FAILED(hr)) {
        DebugLog("ERROR: CfDisconnectSyncRoot FAILED: HRESULT=0x%08lX", hr);
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    DebugLog("CfDisconnectSyncRoot SUCCESS");
    return CFAPI_BRIDGE_OK;
}

int32_t CfapiBridgeWaitForRequest(uint32_t timeoutMs) {
    if (!g_initialized) {
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    // Check if already have requests
    EnterCriticalSection(&g_queueCS);
    int hasRequests = (g_queueCount > 0);
    LeaveCriticalSection(&g_queueCS);

    if (hasRequests) {
        return CFAPI_BRIDGE_OK;
    }

    // Wait for new request
    DWORD result = WaitForSingleObject(g_newRequestEvent, timeoutMs);
    if (result == WAIT_OBJECT_0) {
        return CFAPI_BRIDGE_OK;
    } else if (result == WAIT_TIMEOUT) {
        return CFAPI_BRIDGE_ERROR_TIMEOUT;
    }

    return CFAPI_BRIDGE_ERROR_API_FAILED;
}

int32_t CfapiBridgePollRequest(CfapiBridgeRequest* request) {
    if (!g_initialized) {
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    if (!request) {
        return CFAPI_BRIDGE_ERROR_INVALID_PARAM;
    }

    return DequeueRequest(request);
}

int32_t CfapiBridgeTransferData(
    int64_t connectionKey,
    int64_t transferKey,
    const void* buffer,
    int64_t bufferLength,
    int64_t offset
) {
    DebugLog("CfapiBridgeTransferData: connKey=%lld, transKey=%lld, len=%lld, offset=%lld",
             (long long)connectionKey, (long long)transferKey,
             (long long)bufferLength, (long long)offset);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized");
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    if (!buffer || bufferLength <= 0) {
        DebugLog("ERROR: Invalid buffer parameters");
        return CFAPI_BRIDGE_ERROR_INVALID_PARAM;
    }

    CF_OPERATION_INFO opInfo;
    memset(&opInfo, 0, sizeof(opInfo));
    opInfo.StructSize = sizeof(CF_OPERATION_INFO);
    opInfo.Type = CF_OPERATION_TYPE_TRANSFER_DATA;
    opInfo.ConnectionKey = (CF_CONNECTION_KEY)connectionKey;
    opInfo.TransferKey = (CF_TRANSFER_KEY)transferKey;

    CF_OPERATION_PARAMETERS opParams;
    memset(&opParams, 0, sizeof(opParams));
    opParams.ParamSize = sizeof(CF_OPERATION_PARAMETERS);
    opParams.TransferData.CompletionStatus = S_OK;
    opParams.TransferData.Buffer = (PVOID)buffer;
    opParams.TransferData.Offset.QuadPart = offset;
    opParams.TransferData.Length.QuadPart = bufferLength;

    HRESULT hr = g_pfnCfExecute(&opInfo, &opParams);
    if (FAILED(hr)) {
        DebugLog("ERROR: CfExecute (TransferData) FAILED: HRESULT=0x%08lX", hr);
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    DebugLog("TransferData SUCCESS");
    return CFAPI_BRIDGE_OK;
}

int32_t CfapiBridgeTransferComplete(
    int64_t connectionKey,
    int64_t transferKey
) {
    DebugLog("CfapiBridgeTransferComplete: connKey=%lld, transKey=%lld",
             (long long)connectionKey, (long long)transferKey);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized");
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    CF_OPERATION_INFO opInfo;
    memset(&opInfo, 0, sizeof(opInfo));
    opInfo.StructSize = sizeof(CF_OPERATION_INFO);
    opInfo.Type = CF_OPERATION_TYPE_ACK_DATA;
    opInfo.ConnectionKey = (CF_CONNECTION_KEY)connectionKey;
    opInfo.TransferKey = (CF_TRANSFER_KEY)transferKey;

    CF_OPERATION_PARAMETERS opParams;
    memset(&opParams, 0, sizeof(opParams));
    opParams.ParamSize = sizeof(CF_OPERATION_PARAMETERS);
    opParams.AckData.CompletionStatus = S_OK;

    HRESULT hr = g_pfnCfExecute(&opInfo, &opParams);
    if (FAILED(hr)) {
        DebugLog("ERROR: CfExecute (AckData) FAILED: HRESULT=0x%08lX", hr);
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    DebugLog("TransferComplete SUCCESS");
    return CFAPI_BRIDGE_OK;
}

int32_t CfapiBridgeTransferError(
    int64_t connectionKey,
    int64_t transferKey,
    int32_t hresult
) {
    DebugLog("CfapiBridgeTransferError: connKey=%lld, transKey=%lld, hr=0x%08X",
             (long long)connectionKey, (long long)transferKey, hresult);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized");
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    CF_OPERATION_INFO opInfo;
    memset(&opInfo, 0, sizeof(opInfo));
    opInfo.StructSize = sizeof(CF_OPERATION_INFO);
    opInfo.Type = CF_OPERATION_TYPE_TRANSFER_DATA;
    opInfo.ConnectionKey = (CF_CONNECTION_KEY)connectionKey;
    opInfo.TransferKey = (CF_TRANSFER_KEY)transferKey;

    CF_OPERATION_PARAMETERS opParams;
    memset(&opParams, 0, sizeof(opParams));
    opParams.ParamSize = sizeof(CF_OPERATION_PARAMETERS);
    opParams.TransferData.CompletionStatus = hresult;
    opParams.TransferData.Buffer = NULL;
    opParams.TransferData.Offset.QuadPart = 0;
    opParams.TransferData.Length.QuadPart = 0;

    HRESULT hr = g_pfnCfExecute(&opInfo, &opParams);
    if (FAILED(hr)) {
        DebugLog("ERROR: CfExecute (TransferError) FAILED: HRESULT=0x%08lX", hr);
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    DebugLog("TransferError sent OK");
    return CFAPI_BRIDGE_OK;
}

int32_t CfapiBridgeReportProgress(
    int64_t connectionKey,
    int64_t transferKey,
    int64_t total,
    int64_t completed
) {
    if (!g_initialized) {
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    if (!g_pfnCfReportProviderProgress) {
        // Function not available on older Windows - silently succeed
        return CFAPI_BRIDGE_OK;
    }

    LARGE_INTEGER totalLI, completedLI;
    totalLI.QuadPart = total;
    completedLI.QuadPart = completed;

    HRESULT hr = g_pfnCfReportProviderProgress(
        (CF_CONNECTION_KEY)connectionKey,
        (CF_TRANSFER_KEY)transferKey,
        totalLI,
        completedLI
    );

    if (FAILED(hr)) {
        // Don't treat progress reporting failure as fatal
        return CFAPI_BRIDGE_OK;
    }

    return CFAPI_BRIDGE_OK;
}

int32_t CfapiBridgeIsInitialized(void) {
    return g_initialized;
}

int32_t CfapiBridgeGetQueueCount(void) {
    if (!g_initialized) return 0;

    EnterCriticalSection(&g_queueCS);
    int count = g_queueCount;
    LeaveCriticalSection(&g_queueCS);

    return count;
}

// Simple structure for TRANSFER_PLACEHOLDERS params
typedef struct {
    DWORD Flags;
    HRESULT CompletionStatus;
    LARGE_INTEGER PlaceholderTotalCount;
} CF_OPERATION_TRANSFER_PLACEHOLDERS_PARAMS_SIMPLE;

int32_t CfapiBridgeAckFetchPlaceholders(
    int64_t connectionKey,
    int64_t transferKey
) {
    DebugLog("CfapiBridgeAckFetchPlaceholders: connKey=%lld, transKey=%lld",
             (long long)connectionKey, (long long)transferKey);

    if (!g_initialized) {
        DebugLog("ERROR: Bridge not initialized");
        return CFAPI_BRIDGE_ERROR_NOT_INITIALIZED;
    }

    CF_OPERATION_INFO opInfo;
    memset(&opInfo, 0, sizeof(opInfo));
    opInfo.StructSize = sizeof(CF_OPERATION_INFO);
    opInfo.Type = CF_OPERATION_TYPE_TRANSFER_PLACEHOLDERS;
    opInfo.ConnectionKey = (CF_CONNECTION_KEY)connectionKey;
    opInfo.TransferKey = (CF_TRANSFER_KEY)transferKey;

    CF_OPERATION_PARAMETERS opParams;
    memset(&opParams, 0, sizeof(opParams));
    opParams.ParamSize = sizeof(CF_OPERATION_PARAMETERS);

    // Use the simple params embedded in the union
    CF_OPERATION_TRANSFER_PLACEHOLDERS_PARAMS_SIMPLE* tpParams =
        (CF_OPERATION_TRANSFER_PLACEHOLDERS_PARAMS_SIMPLE*)&opParams.Reserved[0];
    // NOTE: Do NOT use DISABLE_ON_DEMAND_POPULATION (0x00000001) - it causes freeze on subsequent access!
    // With Flags=0, Windows will call FETCH_PLACEHOLDERS each time the folder is accessed.
    tpParams->Flags = 0x00000000; // No special flags - allow repeated callbacks
    tpParams->CompletionStatus = S_OK;
    tpParams->PlaceholderTotalCount.QuadPart = 0;

    DebugLog("  Calling CfExecute(TRANSFER_PLACEHOLDERS) with Flags=0x%08X (no DISABLE_ON_DEMAND)", tpParams->Flags);

    HRESULT hr = g_pfnCfExecute(&opInfo, &opParams);

    if (FAILED(hr)) {
        DebugLog("ERROR: CfExecute (AckFetchPlaceholders) FAILED: HRESULT=0x%08lX", hr);
        return CFAPI_BRIDGE_ERROR_API_FAILED;
    }

    DebugLog("AckFetchPlaceholders SUCCESS");
    return CFAPI_BRIDGE_OK;
}
