//go:build windows
// +build windows

package cloudfiles

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewCloudFilesProvider(t *testing.T) {
	tempDir := t.TempDir()

	config := ProviderConfig{
		LocalPath:    tempDir,
		RemotePath:   "\\\\server\\share",
		ProviderName: "TestProvider",
	}

	provider, err := NewCloudFilesProvider(config)
	if err != nil {
		t.Fatalf("NewCloudFilesProvider failed: %v", err)
	}

	if provider == nil {
		t.Fatal("provider should not be nil")
	}

	if provider.localPath != tempDir {
		t.Errorf("localPath mismatch: got %s, want %s", provider.localPath, tempDir)
	}

	if provider.providerName != "TestProvider" {
		t.Errorf("providerName mismatch: got %s, want %s", provider.providerName, "TestProvider")
	}

	if provider.IsInitialized() {
		t.Error("provider should not be initialized initially")
	}
}

func TestNewCloudFilesProviderDefaults(t *testing.T) {
	tempDir := t.TempDir()

	config := ProviderConfig{
		LocalPath: tempDir,
	}

	provider, err := NewCloudFilesProvider(config)
	if err != nil {
		t.Fatalf("NewCloudFilesProvider failed: %v", err)
	}

	if provider.providerName != "AnemoneSync" {
		t.Errorf("Expected default provider name 'AnemoneSync', got '%s'", provider.providerName)
	}
}

func TestNewCloudFilesProviderEmptyPath(t *testing.T) {
	config := ProviderConfig{
		LocalPath: "",
	}

	_, err := NewCloudFilesProvider(config)
	if err == nil {
		t.Error("Expected error for empty local path")
	}
}

// Mock data source for testing
type mockDataSource struct {
	files map[string][]byte
}

func newMockDataSource() *mockDataSource {
	return &mockDataSource{
		files: make(map[string][]byte),
	}
}

func (m *mockDataSource) AddFile(path string, content []byte) {
	m.files[path] = content
}

func (m *mockDataSource) GetFileReader(ctx context.Context, remotePath string, offset int64) (io.ReadCloser, error) {
	content, ok := m.files[remotePath]
	if !ok {
		return nil, os.ErrNotExist
	}

	if offset > 0 {
		if offset >= int64(len(content)) {
			content = []byte{}
		} else {
			content = content[offset:]
		}
	}

	return io.NopCloser(strings.NewReader(string(content))), nil
}

func (m *mockDataSource) ListFiles(ctx context.Context) ([]RemoteFileInfo, error) {
	var files []RemoteFileInfo
	for path, content := range m.files {
		files = append(files, RemoteFileInfo{
			Path:    path,
			Size:    int64(len(content)),
			ModTime: time.Now(),
		})
	}
	return files, nil
}

func TestCloudFilesProviderSetDataSource(t *testing.T) {
	tempDir := t.TempDir()

	config := ProviderConfig{
		LocalPath: tempDir,
	}

	provider, err := NewCloudFilesProvider(config)
	if err != nil {
		t.Fatalf("NewCloudFilesProvider failed: %v", err)
	}

	// Initially no hydration handler
	if provider.hydration != nil {
		t.Error("hydration should be nil before setting data source")
	}

	// Set data source
	mockSource := newMockDataSource()
	mockSource.AddFile("test.txt", []byte("hello world"))
	provider.SetDataSource(mockSource)

	// Now should have hydration handler
	if provider.hydration == nil {
		t.Error("hydration should not be nil after setting data source")
	}
}

func TestCloudFilesProviderClose(t *testing.T) {
	tempDir := t.TempDir()

	config := ProviderConfig{
		LocalPath: tempDir,
	}

	provider, err := NewCloudFilesProvider(config)
	if err != nil {
		t.Fatalf("NewCloudFilesProvider failed: %v", err)
	}

	// Close should work even if not initialized
	if err := provider.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// Integration test - requires admin privileges and actual Cloud Files API
func TestCloudFilesProviderIntegration(t *testing.T) {
	if !IsAvailable() {
		t.Skip("Cloud Files API not available")
	}

	if os.Getenv("RUN_CLOUDFILES_INTEGRATION") != "1" {
		t.Skip("Skipping integration test. Set RUN_CLOUDFILES_INTEGRATION=1 to run")
	}

	tempDir := t.TempDir()
	t.Logf("Using temp dir: %s", tempDir)

	config := ProviderConfig{
		LocalPath:    tempDir,
		ProviderName: "AnemoneSync-Test",
	}

	provider, err := NewCloudFilesProvider(config)
	if err != nil {
		t.Fatalf("NewCloudFilesProvider failed: %v", err)
	}

	// Set up mock data source
	mockSource := newMockDataSource()
	mockSource.AddFile("file1.txt", []byte("content of file 1"))
	mockSource.AddFile("dir/file2.txt", []byte("content of file 2"))
	provider.SetDataSource(mockSource)

	// Initialize
	ctx := context.Background()
	if err := provider.Initialize(ctx); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	if !provider.IsInitialized() {
		t.Error("provider should be initialized")
	}

	// Sync placeholders
	files := []RemoteFileInfo{
		{Path: "file1.txt", Size: 17, ModTime: time.Now()},
		{Path: "dir/file2.txt", Size: 17, ModTime: time.Now()},
	}

	if err := provider.SyncPlaceholders(ctx, files); err != nil {
		t.Errorf("SyncPlaceholders failed: %v", err)
	}

	// Check placeholder state
	state, err := provider.GetPlaceholderState("file1.txt")
	if err != nil {
		t.Errorf("GetPlaceholderState failed: %v", err)
	}
	t.Logf("file1.txt state: exists=%v, placeholder=%v, hydrated=%v",
		state.Exists, state.IsPlaceholder, state.IsHydrated)

	// Clean up
	if err := provider.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Unregister to clean up completely
	if err := provider.Unregister(); err != nil {
		t.Errorf("Unregister failed: %v", err)
	}
}

func TestFromManifestFilesIntegration(t *testing.T) {
	manifest := []ManifestFileEntry{
		{Path: "documents/report.pdf", Size: 1024000, MTime: 1700000000, Hash: "sha256:abc123"},
		{Path: "images/photo.jpg", Size: 2048000, MTime: 1700000001, Hash: "sha256:def456"},
		{Path: "data/config.json", Size: 512, MTime: 1700000002, Hash: "sha256:ghi789"},
	}

	files := FromManifestFiles(manifest)

	if len(files) != 3 {
		t.Fatalf("Expected 3 files, got %d", len(files))
	}

	// Check first file
	if files[0].Path != "documents/report.pdf" {
		t.Errorf("Expected path 'documents/report.pdf', got '%s'", files[0].Path)
	}
	if files[0].Size != 1024000 {
		t.Errorf("Expected size 1024000, got %d", files[0].Size)
	}
	if files[0].IsDirectory {
		t.Error("File should not be marked as directory")
	}
}

func TestSMBDataSource(t *testing.T) {
	// Test NewSMBDataSource
	source := NewSMBDataSource(nil, "\\\\server\\share", nil)

	if source == nil {
		t.Fatal("SMBDataSource should not be nil")
	}

	if source.sharePath != "\\\\server\\share" {
		t.Errorf("sharePath mismatch: got %s", source.sharePath)
	}
}
