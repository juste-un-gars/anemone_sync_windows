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

// Callback types matching CF_CALLBACK_TYPE
typedef enum {
    CFAPI_CALLBACK_FETCH_DATA = 0,
    CFAPI_CALLBACK_CANCEL_FETCH_DATA = 2,
    CFAPI_CALLBACK_NOTIFY_DELETE = 9,
    CFAPI_CALLBACK_NOTIFY_RENAME = 11,
} CfapiBridgeCallbackType;

// Request structure passed from C to Go
typedef struct {
    int32_t type;                           // CfapiBridgeCallbackType
    int64_t connectionKey;                  // CF_CONNECTION_KEY
    int64_t transferKey;                    // CF_TRANSFER_KEY
    wchar_t filePath[CFAPI_BRIDGE_MAX_PATH]; // Normalized file path
    int64_t fileSize;                       // File size (for FETCH_DATA)
    int64_t requiredOffset;                 // Required offset (for FETCH_DATA)
    int64_t requiredLength;                 // Required length (for FETCH_DATA)
    wchar_t targetPath[CFAPI_BRIDGE_MAX_PATH]; // Target path (for NOTIFY_RENAME)
    int32_t isDirectory;                    // Is this a directory operation
} CfapiBridgeRequest;

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

// Transfer data for a hydration request
// connectionKey: the connection key
// transferKey: the transfer key from the request
// buffer: data buffer to send
// bufferLength: length of data in buffer
// offset: file offset for this data
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeTransferData(
    int64_t connectionKey,
    int64_t transferKey,
    const void* buffer,
    int64_t bufferLength,
    int64_t offset
);

// Complete a hydration request successfully
// connectionKey: the connection key
// transferKey: the transfer key from the request
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeTransferComplete(
    int64_t connectionKey,
    int64_t transferKey
);

// Report an error for a hydration request
// connectionKey: the connection key
// transferKey: the transfer key from the request
// hresult: the error code to report (e.g., E_FAIL)
// Returns CFAPI_BRIDGE_OK on success
int32_t CfapiBridgeTransferError(
    int64_t connectionKey,
    int64_t transferKey,
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

#endif // CFAPI_BRIDGE_H
