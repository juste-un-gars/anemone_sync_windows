//go:build windows
// +build windows

#ifndef CFAPI_BRIDGE_H
#define CFAPI_BRIDGE_H

#include <windows.h>
#include <stdint.h>

// Maximum queue size for callback requests
#define CFAPI_BRIDGE_MAX_QUEUE_SIZE 64

// Maximum path length
#define CFAPI_BRIDGE_MAX_PATH 520

// Maximum chunk size for data transfer (1 MB)
#define CFAPI_BRIDGE_MAX_CHUNK_SIZE (1024 * 1024)

// Callback types matching CF_CALLBACK_TYPE
typedef enum {
    CFAPI_CALLBACK_FETCH_DATA = 0,
    CFAPI_CALLBACK_CANCEL_FETCH_DATA = 2,
    CFAPI_CALLBACK_NOTIFY_DELETE = 9,
    CFAPI_CALLBACK_NOTIFY_RENAME = 11,
} CfapiBridgeCallbackType;

// Request structure passed from C to Go (for non-FETCH_DATA callbacks)
typedef struct {
    int32_t type;                           // CfapiBridgeCallbackType
    int64_t connectionKey;                  // CF_CONNECTION_KEY
    int64_t transferKey;                    // CF_TRANSFER_KEY
    int64_t requestKey;                     // CF_REQUEST_KEY (required for CfExecute)
    wchar_t filePath[CFAPI_BRIDGE_MAX_PATH]; // Normalized file path
    int64_t fileSize;                       // File size (for FETCH_DATA)
    int64_t requiredOffset;                 // Required offset (for FETCH_DATA)
    int64_t requiredLength;                 // Required length (for FETCH_DATA)
    wchar_t targetPath[CFAPI_BRIDGE_MAX_PATH]; // Target path (for NOTIFY_RENAME)
    int32_t isDirectory;                    // Is this a directory operation
    void* completionEvent;                  // Event to signal when transfer is done (for sync callbacks)
} CfapiBridgeRequest;

// Response from Go for FETCH_DATA - contains a chunk of data
typedef struct {
    int32_t errorCode;                      // 0 = success, negative = error
    int64_t dataLength;                     // Length of data in buffer
    uint8_t data[CFAPI_BRIDGE_MAX_CHUNK_SIZE]; // Data buffer
} CfapiBridgeFetchResponse;

// Shared data request structure for thread-safe data fetching
// C posts a request, Go fills in the data, C reads the data.
// This avoids calling Go from the Windows callback thread.
typedef struct {
    // Request fields (written by C, read by Go)
    wchar_t filePath[CFAPI_BRIDGE_MAX_PATH];
    int64_t offset;
    int64_t maxLength;

    // Synchronization (created by C, used by both)
    void* requestReadyEvent;    // C sets this when request is ready
    void* dataReadyEvent;       // Go sets this when data is ready

    // Response fields (written by Go, read by C)
    int32_t errorCode;
    int64_t dataLength;
    uint8_t data[CFAPI_BRIDGE_MAX_CHUNK_SIZE];
} CfapiBridgeSharedFetchRequest;

// Initialize the shared fetch request (call once)
int32_t CfapiBridgeInitSharedFetch(void);

// Cleanup the shared fetch request
void CfapiBridgeCleanupSharedFetch(void);

// C-side: Wait for Go to fill the shared buffer (called from callback thread)
// Returns 0 on success, negative on error/timeout
int32_t CfapiBridgeWaitForData(uint32_t timeoutMs);

// Go-side: Get the current pending request
// Returns pointer to shared request, or NULL if no request pending
CfapiBridgeSharedFetchRequest* CfapiBridgeGetPendingRequest(void);

// Go-side: Signal that data is ready
void CfapiBridgeSignalDataReady(void);

// Result codes
typedef enum {
    CFAPI_BRIDGE_OK = 0,
    CFAPI_BRIDGE_ERROR_NOT_INITIALIZED = -1,
    CFAPI_BRIDGE_ERROR_QUEUE_FULL = -2,
    CFAPI_BRIDGE_ERROR_QUEUE_EMPTY = -3,
    CFAPI_BRIDGE_ERROR_TIMEOUT = -4,
    CFAPI_BRIDGE_ERROR_API_FAILED = -5,
    CFAPI_BRIDGE_ERROR_INVALID_PARAM = -6,
} CfapiBridgeResult;

// Initialize the bridge (call once at startup)
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeInit(void);

// Cleanup the bridge (call at shutdown)
void CfapiBridgeCleanup(void);

// Connect to a sync root with C callbacks
// syncRootPath: path to the sync root (wide string)
// callbackContext: opaque pointer passed back in requests
// connectionKey: output - connection key for later operations
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeConnect(
    const wchar_t* syncRootPath,
    void* callbackContext,
    int64_t* connectionKey
);

// Disconnect from a sync root
// connectionKey: the connection key from CfapiBridgeConnect
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeDisconnect(int64_t connectionKey);

// Wait for a request to be available
// timeoutMs: timeout in milliseconds (0 = no wait, INFINITE = forever)
// Returns CFAPI_BRIDGE_OK if a request is available, CFAPI_BRIDGE_ERROR_TIMEOUT otherwise
int32_t CfapiBridgeWaitForRequest(uint32_t timeoutMs);

// Poll for a request (non-blocking)
// request: output - the request data
// Returns CFAPI_BRIDGE_OK if a request was retrieved, CFAPI_BRIDGE_ERROR_QUEUE_EMPTY if none
int32_t CfapiBridgePollRequest(CfapiBridgeRequest* request);

// Transfer data flags
#define CF_OPERATION_TRANSFER_DATA_FLAG_MARK_IN_SYNC 0x00000001

// Transfer data for a hydration request
// connectionKey: the connection key
// transferKey: the transfer key from the request
// requestKey: the request key from the callback (required for async operations)
// buffer: data buffer to send
// bufferLength: length of data in buffer
// offset: file offset for this data
// flags: 0 for intermediate chunks, CF_OPERATION_TRANSFER_DATA_FLAG_MARK_IN_SYNC for last chunk
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeTransferData(
    int64_t connectionKey,
    int64_t transferKey,
    int64_t requestKey,
    const void* buffer,
    int64_t bufferLength,
    int64_t offset,
    int32_t flags
);

// Complete a hydration request successfully
// connectionKey: the connection key
// transferKey: the transfer key from the request
// requestKey: the request key from the callback
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeTransferComplete(
    int64_t connectionKey,
    int64_t transferKey,
    int64_t requestKey
);

// Report an error for a hydration request
// connectionKey: the connection key
// transferKey: the transfer key from the request
// requestKey: the request key from the callback
// hresult: the error code to report (e.g., E_FAIL)
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeTransferError(
    int64_t connectionKey,
    int64_t transferKey,
    int64_t requestKey,
    int32_t hresult
);

// Report progress during hydration
// connectionKey: the connection key
// transferKey: the transfer key from the request
// total: total bytes to transfer
// completed: bytes transferred so far
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeReportProgress(
    int64_t connectionKey,
    int64_t transferKey,
    int64_t total,
    int64_t completed
);

// Check if the bridge is initialized
// Returns 1 if initialized, 0 otherwise
int32_t CfapiBridgeIsInitialized(void);

// Get the number of pending requests in the queue
int32_t CfapiBridgeGetQueueCount(void);

// Acknowledge FETCH_PLACEHOLDERS callback (tell Windows we're done populating)
// connectionKey: the connection key
// transferKey: the transfer key from the callback
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeAckFetchPlaceholders(
    int64_t connectionKey,
    int64_t transferKey
);

// Signal that a FETCH_DATA transfer is complete
// This must be called after all data has been transferred to unblock the callback
// completionEvent: the event handle from CfapiBridgeRequest
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeSignalTransferComplete(void* completionEvent);

// ============================================================================
// NEW ARCHITECTURE: Direct callback execution
// ============================================================================
// Instead of enqueueing FETCH_DATA and having Go call CfExecute from another
// thread (which causes ERROR_CLOUD_FILE_NOT_UNDER_SYNC_ROOT), the callback
// now calls Go directly to fetch data chunks, then C calls CfExecute from
// within the callback context.
//
// This is implemented via CGO export: Go exports GoFetchDataChunk which C calls.
// ============================================================================

// Go-exported function prototype (implemented in cfapi_bridge.go via //export)
// C calls this function from within the FETCH_DATA callback to get data.
// Returns: 0 on success with data in response, negative error code on failure.
// When dataLength is 0 with errorCode 0, it means EOF (no more data).
// NOTE: No 'const' because CGO generates non-const parameters
extern int32_t GoFetchDataChunk(
    wchar_t* normalizedPath,        // File path (e.g., "\sync_root\file.txt")
    int64_t offset,                 // Offset to read from
    int64_t maxLength,              // Maximum bytes to read
    CfapiBridgeFetchResponse* response  // Output: data chunk
);

// Go-exported function to report progress (optional)
extern void GoReportHydrationProgress(
    wchar_t* normalizedPath,
    int64_t total,
    int64_t completed
);

#endif // CFAPI_BRIDGE_H
