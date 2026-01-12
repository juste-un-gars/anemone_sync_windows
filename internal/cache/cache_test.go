package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"go.uber.org/zap"
)

func setupTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()

	// Create temp directory for test database
	tmpDir, err := os.MkdirTemp("", "cache_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	db, err := database.Open(database.Config{
		Path:             dbPath,
		EncryptionKey:    "test-key-12345",
		CreateIfNotExist: true,
	})
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to open database: %v", err)
	}

	// Cleanup function
	cleanup := func() {
		db.Close()
		os.RemoveAll(tmpDir)
	}

	return db, cleanup
}

func TestNewCacheManager(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())
	if cm == nil {
		t.Fatal("expected cache manager but got nil")
	}
	if cm.db != db {
		t.Error("cache manager has wrong database reference")
	}
}

func TestCacheManager_UpdateAndGet(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())

	jobID := int64(1)
	localPath := "/test/file.txt"
	remotePath := "/remote/file.txt"

	fileInfo := &FileInfo{
		Path:  localPath,
		Size:  1024,
		MTime: time.Now().Truncate(time.Second),
		Hash:  "abc123",
	}

	// Update cache
	err := cm.UpdateCache(jobID, localPath, remotePath, fileInfo)
	if err != nil {
		t.Fatalf("failed to update cache: %v", err)
	}

	// Get cached state
	cached, err := cm.GetCachedState(jobID, localPath)
	if err != nil {
		t.Fatalf("failed to get cached state: %v", err)
	}

	if cached == nil {
		t.Fatal("expected cached state but got nil")
	}

	// Verify cached data
	if cached.Size != fileInfo.Size {
		t.Errorf("size: expected %d, got %d", fileInfo.Size, cached.Size)
	}
	if !cached.MTime.Equal(fileInfo.MTime) {
		t.Errorf("mtime: expected %v, got %v", fileInfo.MTime, cached.MTime)
	}
	if cached.Hash != fileInfo.Hash {
		t.Errorf("hash: expected %s, got %s", fileInfo.Hash, cached.Hash)
	}
}

func TestCacheManager_GetNonExistent(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())

	// Try to get non-existent file
	cached, err := cm.GetCachedState(1, "/non/existent/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cached != nil {
		t.Error("expected nil for non-existent file")
	}
}

func TestCacheManager_UpdateCacheBatch(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())

	jobID := int64(1)
	now := time.Now().Truncate(time.Second)

	updates := map[string]*FileInfo{
		"/file1.txt": {Path: "/file1.txt", Size: 100, MTime: now, Hash: "hash1"},
		"/file2.txt": {Path: "/file2.txt", Size: 200, MTime: now, Hash: "hash2"},
		"/file3.txt": {Path: "/file3.txt", Size: 300, MTime: now, Hash: "hash3"},
	}

	remotePaths := map[string]string{
		"/file1.txt": "/remote/file1.txt",
		"/file2.txt": "/remote/file2.txt",
		"/file3.txt": "/remote/file3.txt",
	}

	// Batch update
	err := cm.UpdateCacheBatch(jobID, updates, remotePaths)
	if err != nil {
		t.Fatalf("failed to batch update: %v", err)
	}

	// Verify all files were cached
	for localPath, expectedInfo := range updates {
		cached, err := cm.GetCachedState(jobID, localPath)
		if err != nil {
			t.Fatalf("failed to get cached state for %s: %v", localPath, err)
		}
		if cached == nil {
			t.Errorf("expected cached state for %s but got nil", localPath)
			continue
		}
		if cached.Size != expectedInfo.Size {
			t.Errorf("%s: size mismatch: expected %d, got %d", localPath, expectedInfo.Size, cached.Size)
		}
	}
}

func TestCacheManager_RemoveFromCache(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())

	jobID := int64(1)
	localPath := "/test/file.txt"
	remotePath := "/remote/file.txt"

	fileInfo := &FileInfo{
		Path:  localPath,
		Size:  1024,
		MTime: time.Now().Truncate(time.Second),
		Hash:  "abc123",
	}

	// Add to cache
	err := cm.UpdateCache(jobID, localPath, remotePath, fileInfo)
	if err != nil {
		t.Fatalf("failed to update cache: %v", err)
	}

	// Verify it exists
	cached, err := cm.GetCachedState(jobID, localPath)
	if err != nil || cached == nil {
		t.Fatal("file should be in cache")
	}

	// Remove from cache
	err = cm.RemoveFromCache(jobID, localPath)
	if err != nil {
		t.Fatalf("failed to remove from cache: %v", err)
	}

	// Verify it's gone
	cached, err = cm.GetCachedState(jobID, localPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cached != nil {
		t.Error("file should not be in cache after removal")
	}
}

func TestCacheManager_DetectLocalChange(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())

	jobID := int64(1)
	localPath := "/test/file.txt"
	remotePath := "/remote/file.txt"
	now := time.Now().Truncate(time.Second)

	tests := []struct {
		name        string
		cached      *FileInfo
		current     *FileInfo
		expectedType ChangeType
	}{
		{
			name:         "new file",
			cached:       nil,
			current:      &FileInfo{Path: localPath, Size: 100, MTime: now, Hash: "hash1"},
			expectedType: ChangeTypeNew,
		},
		{
			name:         "deleted file",
			cached:       &FileInfo{Path: localPath, Size: 100, MTime: now, Hash: "hash1"},
			current:      nil,
			expectedType: ChangeTypeDeleted,
		},
		{
			name:         "modified file (size)",
			cached:       &FileInfo{Path: localPath, Size: 100, MTime: now, Hash: "hash1"},
			current:      &FileInfo{Path: localPath, Size: 200, MTime: now, Hash: "hash2"},
			expectedType: ChangeTypeModified,
		},
		{
			name:         "modified file (mtime)",
			cached:       &FileInfo{Path: localPath, Size: 100, MTime: now, Hash: "hash1"},
			current:      &FileInfo{Path: localPath, Size: 100, MTime: now.Add(time.Hour), Hash: "hash1"},
			expectedType: ChangeTypeModified,
		},
		{
			name:         "unchanged file",
			cached:       &FileInfo{Path: localPath, Size: 100, MTime: now, Hash: "hash1"},
			current:      &FileInfo{Path: localPath, Size: 100, MTime: now, Hash: "hash1"},
			expectedType: ChangeTypeNone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup cached state
			if tt.cached != nil {
				err := cm.UpdateCache(jobID, localPath, remotePath, tt.cached)
				if err != nil {
					t.Fatalf("failed to setup cache: %v", err)
				}
			} else {
				// Ensure no cached state
				cm.RemoveFromCache(jobID, localPath)
			}

			// Detect change
			change, err := cm.DetectLocalChange(jobID, localPath, tt.current)
			if err != nil {
				t.Fatalf("failed to detect change: %v", err)
			}

			if change.Type != tt.expectedType {
				t.Errorf("expected change type %s, got %s", tt.expectedType, change.Type)
			}

			// Cleanup
			cm.RemoveFromCache(jobID, localPath)
		})
	}
}

func TestCacheManager_GetAllCachedFiles(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	cm := NewCacheManager(db, zap.NewNop())

	jobID := int64(1)
	now := time.Now().Truncate(time.Second)

	// Add multiple files
	files := map[string]*FileInfo{
		"/file1.txt": {Path: "/file1.txt", Size: 100, MTime: now, Hash: "hash1"},
		"/file2.txt": {Path: "/file2.txt", Size: 200, MTime: now, Hash: "hash2"},
		"/file3.txt": {Path: "/file3.txt", Size: 300, MTime: now, Hash: "hash3"},
	}

	remotePaths := make(map[string]string)
	for path := range files {
		remotePaths[path] = path
	}

	err := cm.UpdateCacheBatch(jobID, files, remotePaths)
	if err != nil {
		t.Fatalf("failed to batch update: %v", err)
	}

	// Get all cached files
	cached, err := cm.GetAllCachedFiles(jobID)
	if err != nil {
		t.Fatalf("failed to get all cached files: %v", err)
	}

	if len(cached) != len(files) {
		t.Errorf("expected %d files, got %d", len(files), len(cached))
	}

	for path, expectedInfo := range files {
		cachedInfo, ok := cached[path]
		if !ok {
			t.Errorf("missing cached file: %s", path)
			continue
		}
		if cachedInfo.Size != expectedInfo.Size {
			t.Errorf("%s: size mismatch: expected %d, got %d", path, expectedInfo.Size, cachedInfo.Size)
		}
	}
}
