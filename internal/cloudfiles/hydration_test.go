//go:build windows
// +build windows

package cloudfiles

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"time"
)

// mockDataProvider implements DataProvider for testing
type mockDataProvider struct {
	mu       sync.RWMutex
	files    map[string][]byte
	readDelay time.Duration
}

func newMockDataProvider() *mockDataProvider {
	return &mockDataProvider{
		files: make(map[string][]byte),
	}
}

func (m *mockDataProvider) AddFile(path string, content []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = content
}

func (m *mockDataProvider) SetReadDelay(d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readDelay = d
}

func (m *mockDataProvider) GetFileReader(ctx context.Context, relativePath string, offset int64) (io.ReadCloser, error) {
	m.mu.RLock()
	content, ok := m.files[relativePath]
	delay := m.readDelay
	m.mu.RUnlock()

	if !ok {
		return nil, io.EOF
	}

	// Simulate network delay
	if delay > 0 {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}

	if offset > 0 {
		if offset >= int64(len(content)) {
			content = []byte{}
		} else {
			content = content[offset:]
		}
	}

	return &delayedReader{
		reader:  bytes.NewReader(content),
		delay:   delay,
		ctx:     ctx,
	}, nil
}

// delayedReader adds delay to reads for testing cancellation
type delayedReader struct {
	reader *bytes.Reader
	delay  time.Duration
	ctx    context.Context
}

func (r *delayedReader) Read(p []byte) (n int, err error) {
	if r.delay > 0 {
		select {
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		case <-time.After(r.delay / 10): // Small delay per chunk
		}
	}
	return r.reader.Read(p)
}

func (r *delayedReader) Close() error {
	return nil
}

func TestNewHydrationHandler(t *testing.T) {
	provider := newMockDataProvider()

	// Create a minimal sync root manager for testing
	config := SyncRootConfig{
		Path:         t.TempDir(),
		ProviderName: "TestProvider",
	}
	syncRoot, err := NewSyncRootManager(config)
	if err != nil {
		t.Fatalf("Failed to create sync root manager: %v", err)
	}

	handler := NewHydrationHandler(syncRoot, provider, nil)
	if handler == nil {
		t.Fatal("HydrationHandler should not be nil")
	}

	if handler.chunkSize != 1024*1024 {
		t.Errorf("Expected chunk size 1MB, got %d", handler.chunkSize)
	}
}

func TestHydrationHandlerSetChunkSize(t *testing.T) {
	provider := newMockDataProvider()
	config := SyncRootConfig{
		Path:         t.TempDir(),
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)
	handler := NewHydrationHandler(syncRoot, provider, nil)

	// Set valid chunk size
	handler.SetChunkSize(512 * 1024)
	if handler.chunkSize != 512*1024 {
		t.Errorf("Expected chunk size 512KB, got %d", handler.chunkSize)
	}

	// Invalid chunk size should be ignored
	handler.SetChunkSize(0)
	if handler.chunkSize != 512*1024 {
		t.Error("Chunk size should not change for invalid value")
	}

	handler.SetChunkSize(-1)
	if handler.chunkSize != 512*1024 {
		t.Error("Chunk size should not change for negative value")
	}
}

func TestHydrationHandlerGetActiveHydrations(t *testing.T) {
	provider := newMockDataProvider()
	config := SyncRootConfig{
		Path:         t.TempDir(),
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)
	handler := NewHydrationHandler(syncRoot, provider, nil)

	// Initially no active hydrations
	active := handler.GetActiveHydrations()
	if len(active) != 0 {
		t.Errorf("Expected 0 active hydrations, got %d", len(active))
	}
}

func TestHydrationHandlerCancelNonExistent(t *testing.T) {
	provider := newMockDataProvider()
	config := SyncRootConfig{
		Path:         t.TempDir(),
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)
	handler := NewHydrationHandler(syncRoot, provider, nil)

	// Cancelling non-existent hydration should not panic
	handler.CancelHydration(CF_TRANSFER_KEY(12345))
	handler.CancelHydrationByPath("nonexistent/file.txt")
}

func TestHydrationStatus(t *testing.T) {
	status := HydrationStatus{
		FilePath:         "test/file.txt",
		TotalBytes:       1000,
		BytesTransferred: 500,
	}

	if status.FilePath != "test/file.txt" {
		t.Errorf("FilePath mismatch: got %s", status.FilePath)
	}
	if status.TotalBytes != 1000 {
		t.Errorf("TotalBytes mismatch: got %d", status.TotalBytes)
	}
	if status.BytesTransferred != 500 {
		t.Errorf("BytesTransferred mismatch: got %d", status.BytesTransferred)
	}
}

func TestActiveHydrationTracking(t *testing.T) {
	provider := newMockDataProvider()
	provider.AddFile("test.txt", make([]byte, 1024*1024*5)) // 5MB file
	provider.SetReadDelay(100 * time.Millisecond) // Add delay to make hydration take time

	config := SyncRootConfig{
		Path:         t.TempDir(),
		ProviderName: "TestProvider",
	}
	syncRoot, _ := NewSyncRootManager(config)
	handler := NewHydrationHandler(syncRoot, provider, nil)
	handler.SetChunkSize(64 * 1024) // 64KB chunks for more iterations

	// Start hydration in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		info := &FetchDataInfo{
			ConnectionKey:  1,
			TransferKey:    100,
			FilePath:       syncRoot.Path() + "\\test.txt",
			FileSize:       5 * 1024 * 1024,
			RequiredOffset: 0,
			RequiredLength: 0,
		}
		// This will fail due to no actual CF API, but we're testing the tracking
		_ = handler.HandleFetchData(ctx, info)
	}()

	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)

	// Check active hydrations (might have some or might have finished/failed)
	active := handler.GetActiveHydrations()
	t.Logf("Active hydrations during test: %d", len(active))

	// Cancel and wait
	cancel()
	wg.Wait()

	// After cancellation, should have no active hydrations
	active = handler.GetActiveHydrations()
	if len(active) != 0 {
		t.Errorf("Expected 0 active hydrations after cancel, got %d", len(active))
	}
}
