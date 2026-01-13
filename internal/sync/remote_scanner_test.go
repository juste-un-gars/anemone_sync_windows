package sync

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/smb"
	"go.uber.org/zap"
)

// mockSMBClient is a mock implementation of SMBClient for testing
type mockSMBClient struct {
	files        map[string][]smb.RemoteFileInfo // Maps directory path to its contents
	listErrors   map[string]error                 // Maps directory path to error to return
	listCallCount int
}

func newMockSMBClient() *mockSMBClient {
	return &mockSMBClient{
		files:      make(map[string][]smb.RemoteFileInfo),
		listErrors: make(map[string]error),
	}
}

func (m *mockSMBClient) addFile(dir, name string, size int64) {
	path := dir + "/" + name
	entry := smb.RemoteFileInfo{
		Path:    path,
		Name:    name,
		Size:    size,
		ModTime: time.Now(),
		IsDir:   false,
	}
	m.files[dir] = append(m.files[dir], entry)
}

func (m *mockSMBClient) addDir(parentDir, name string) {
	path := parentDir + "/" + name
	entry := smb.RemoteFileInfo{
		Path:    path,
		Name:    name,
		Size:    0,
		ModTime: time.Now(),
		IsDir:   true,
	}
	m.files[parentDir] = append(m.files[parentDir], entry)

	// Initialize the directory entry in the map
	if _, exists := m.files[path]; !exists {
		m.files[path] = make([]smb.RemoteFileInfo, 0)
	}
}

func (m *mockSMBClient) setListError(dir string, err error) {
	m.listErrors[dir] = err
}

func (m *mockSMBClient) ListRemote(path string) ([]smb.RemoteFileInfo, error) {
	m.listCallCount++

	// Check for error
	if err, exists := m.listErrors[path]; exists {
		return nil, err
	}

	// Return files
	files, exists := m.files[path]
	if !exists {
		return []smb.RemoteFileInfo{}, nil
	}

	return files, nil
}

func TestNewRemoteScanner(t *testing.T) {
	client := newMockSMBClient()
	logger := zap.NewNop()

	scanner := NewRemoteScanner(client, logger, nil)

	if scanner == nil {
		t.Fatal("expected scanner to be created")
	}
	if scanner.client == nil {
		t.Error("client not set correctly")
	}
	if scanner.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestNewRemoteScannerNilLogger(t *testing.T) {
	client := newMockSMBClient()

	scanner := NewRemoteScanner(client, nil, nil)

	if scanner.logger == nil {
		t.Error("logger should default to nop logger")
	}
}

func TestRemoteScannerBasic(t *testing.T) {
	mock := newMockSMBClient()
	mock.addFile("/share/docs", "file1.txt", 100)
	mock.addFile("/share/docs", "file2.txt", 200)

	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	result, err := scanner.Scan(context.Background(), "/share/docs")

	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if result.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", result.TotalFiles)
	}

	if result.TotalDirs != 1 {
		t.Errorf("expected 1 dir scanned, got %d", result.TotalDirs)
	}

	if len(result.Files) != 2 {
		t.Errorf("expected 2 files in map, got %d", len(result.Files))
	}

	// Check file contents
	if file, exists := result.Files["file1.txt"]; !exists {
		t.Error("file1.txt not found")
	} else {
		if file.Size != 100 {
			t.Errorf("file1.txt: expected size 100, got %d", file.Size)
		}
	}

	if file, exists := result.Files["file2.txt"]; !exists {
		t.Error("file2.txt not found")
	} else {
		if file.Size != 200 {
			t.Errorf("file2.txt: expected size 200, got %d", file.Size)
		}
	}
}

func TestRemoteScannerRecursive(t *testing.T) {
	mock := newMockSMBClient()

	// Root level
	mock.addFile("/share", "root.txt", 100)
	mock.addDir("/share", "subdir1")
	mock.addDir("/share", "subdir2")

	// Subdir1
	mock.addFile("/share/subdir1", "file1.txt", 200)
	mock.addFile("/share/subdir1", "file2.txt", 300)
	mock.addDir("/share/subdir1", "nested")

	// Nested
	mock.addFile("/share/subdir1/nested", "deep.txt", 400)

	// Subdir2
	mock.addFile("/share/subdir2", "file3.txt", 500)

	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	result, err := scanner.Scan(context.Background(), "/share")

	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if result.TotalFiles != 5 {
		t.Errorf("expected 5 files, got %d", result.TotalFiles)
	}

	if result.TotalDirs != 4 { // /share, /share/subdir1, /share/subdir1/nested, /share/subdir2
		t.Errorf("expected 4 dirs scanned, got %d", result.TotalDirs)
	}

	if len(result.Files) != 5 {
		t.Errorf("expected 5 files in map, got %d", len(result.Files))
	}

	// Check nested file
	if file, exists := result.Files["subdir1/nested/deep.txt"]; !exists {
		t.Error("subdir1/nested/deep.txt not found")
	} else {
		if file.Size != 400 {
			t.Errorf("deep.txt: expected size 400, got %d", file.Size)
		}
	}

	// Check total bytes
	expectedBytes := int64(100 + 200 + 300 + 400 + 500)
	if result.TotalBytes != expectedBytes {
		t.Errorf("expected %d total bytes, got %d", expectedBytes, result.TotalBytes)
	}
}

func TestRemoteScannerEmptyDirectory(t *testing.T) {
	mock := newMockSMBClient()
	// Don't add any files

	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	result, err := scanner.Scan(context.Background(), "/share/empty")

	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if result.TotalFiles != 0 {
		t.Errorf("expected 0 files, got %d", result.TotalFiles)
	}

	if len(result.Files) != 0 {
		t.Errorf("expected empty file map, got %d files", len(result.Files))
	}
}

func TestRemoteScannerWithErrors(t *testing.T) {
	mock := newMockSMBClient()

	// Root level
	mock.addFile("/share", "root.txt", 100)
	mock.addDir("/share", "accessible")
	mock.addDir("/share", "forbidden")

	// Accessible dir
	mock.addFile("/share/accessible", "file1.txt", 200)

	// Forbidden dir - will error
	mock.setListError("/share/forbidden", fmt.Errorf("permission denied"))

	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	result, err := scanner.Scan(context.Background(), "/share")

	// Should not fail completely
	if err != nil {
		t.Fatalf("scan should succeed with partial results, got error: %v", err)
	}

	if result.TotalFiles != 2 { // root.txt and file1.txt
		t.Errorf("expected 2 files, got %d", result.TotalFiles)
	}

	if len(result.Errors) == 0 {
		t.Error("expected errors to be recorded")
	}

	if !result.PartialSuccess {
		t.Error("expected PartialSuccess to be true")
	}
}

func TestRemoteScannerContextCancellation(t *testing.T) {
	mock := newMockSMBClient()

	// Add many files to make cancellation possible
	for i := 0; i < 100; i++ {
		mock.addFile("/share", fmt.Sprintf("file%d.txt", i), 100)
	}

	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	// Cancel immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := scanner.Scan(ctx, "/share")

	if err == nil {
		t.Error("expected error due to context cancellation")
	}

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", err)
	}

	// Result may be nil or incomplete
	if result != nil {
		t.Logf("partial result: %d files found before cancellation", len(result.Files))
	}
}

func TestRemoteScannerProgressCallback(t *testing.T) {
	mock := newMockSMBClient()

	// Add enough files to trigger progress callbacks
	for i := 0; i < 150; i++ { // More than 100 to trigger callback
		mock.addFile("/share", fmt.Sprintf("file%d.txt", i), 100)
	}

	progressCalls := 0
	callback := func(progress RemoteScanProgress) {
		progressCalls++

		if progress.FilesFound < 0 {
			t.Error("FilesFound should not be negative")
		}
		if progress.DirsScanned < 0 {
			t.Error("DirsScanned should not be negative")
		}
		if progress.BytesDiscovered < 0 {
			t.Error("BytesDiscovered should not be negative")
		}
	}

	scanner := NewRemoteScanner(mock, zap.NewNop(), callback)

	result, err := scanner.Scan(context.Background(), "/share")

	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if progressCalls == 0 {
		t.Error("expected progress callback to be called")
	}

	if result.TotalFiles != 150 {
		t.Errorf("expected 150 files, got %d", result.TotalFiles)
	}
}

func TestRemoteScannerGetStats(t *testing.T) {
	mock := newMockSMBClient()
	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	stats := scanner.GetStats()

	if stats.FilesFound != 0 {
		t.Error("initial FilesFound should be 0")
	}
	if stats.DirsScanned != 0 {
		t.Error("initial DirsScanned should be 0")
	}
	if stats.BytesDiscovered != 0 {
		t.Error("initial BytesDiscovered should be 0")
	}
}

func TestRemoteScannerResultDuration(t *testing.T) {
	mock := newMockSMBClient()
	mock.addFile("/share", "file.txt", 100)

	scanner := NewRemoteScanner(mock, zap.NewNop(), nil)

	result, err := scanner.Scan(context.Background(), "/share")

	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	// Duration should be >= 0 (can be 0 for very fast scans)
	if result.Duration < 0 {
		t.Error("duration should not be negative")
	}

	t.Logf("scan duration: %v", result.Duration)
}
