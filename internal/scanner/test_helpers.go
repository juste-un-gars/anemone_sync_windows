package scanner

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/database"
	"go.uber.org/zap"
)

// TestHelpers provides utilities for scanner tests
type TestHelpers struct {
	t       *testing.T
	tempDir string
	db      *database.DB
}

// NewTestHelpers creates a new test helper instance
func NewTestHelpers(t *testing.T) *TestHelpers {
	t.Helper()
	return &TestHelpers{
		t: t,
	}
}

// CreateTempDir creates a temporary directory for tests
func (h *TestHelpers) CreateTempDir() string {
	h.t.Helper()

	tempDir, err := os.MkdirTemp("", "scanner_test_*")
	if err != nil {
		h.t.Fatalf("failed to create temp dir: %v", err)
	}

	h.tempDir = tempDir
	h.t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	return tempDir
}

// CreateTestFile creates a test file with specified content
func (h *TestHelpers) CreateTestFile(path string, content []byte) string {
	h.t.Helper()

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	// Write file
	if err := os.WriteFile(path, content, 0644); err != nil {
		h.t.Fatalf("failed to write file %s: %v", path, err)
	}

	return path
}

// CreateTestFileWithSize creates a test file with random content of specified size
func (h *TestHelpers) CreateTestFileWithSize(path string, size int64) string {
	h.t.Helper()

	// Create parent directories
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	// Create and write file
	file, err := os.Create(path)
	if err != nil {
		h.t.Fatalf("failed to create file %s: %v", path, err)
	}
	defer file.Close()

	// Write random data
	written, err := io.CopyN(file, rand.Reader, size)
	if err != nil {
		h.t.Fatalf("failed to write random data: %v", err)
	}
	if written != size {
		h.t.Fatalf("wrote %d bytes, expected %d", written, size)
	}

	return path
}

// CreateTestFiles creates multiple test files
func (h *TestHelpers) CreateTestFiles(baseDir string, count int, sizePerFile int64) []string {
	h.t.Helper()

	paths := make([]string, count)
	for i := 0; i < count; i++ {
		path := filepath.Join(baseDir, fmt.Sprintf("file_%04d.txt", i))
		paths[i] = h.CreateTestFileWithSize(path, sizePerFile)
	}

	return paths
}

// CreateNestedStructure creates a nested directory structure for testing
func (h *TestHelpers) CreateNestedStructure(baseDir string, depth int, filesPerLevel int) {
	h.t.Helper()

	h.createNestedLevel(baseDir, depth, filesPerLevel, 0)
}

func (h *TestHelpers) createNestedLevel(dir string, maxDepth, filesPerLevel, currentDepth int) {
	if currentDepth >= maxDepth {
		return
	}

	// Create files at this level
	for i := 0; i < filesPerLevel; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file_level%d_%d.txt", currentDepth, i))
		h.CreateTestFile(path, []byte(fmt.Sprintf("content level %d file %d", currentDepth, i)))
	}

	// Create subdirectory and recurse
	subdir := filepath.Join(dir, fmt.Sprintf("level%d", currentDepth+1))
	if err := os.MkdirAll(subdir, 0755); err != nil {
		h.t.Fatalf("failed to create subdir: %v", err)
	}
	h.createNestedLevel(subdir, maxDepth, filesPerLevel, currentDepth+1)
}

// ComputeFileSHA256 computes SHA256 hash of a file (for verification)
func (h *TestHelpers) ComputeFileSHA256(path string) string {
	h.t.Helper()

	file, err := os.Open(path)
	if err != nil {
		h.t.Fatalf("failed to open file %s: %v", path, err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		h.t.Fatalf("failed to hash file %s: %v", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil))
}

// SetupTestDB creates an in-memory SQLite database for testing
func (h *TestHelpers) SetupTestDB() *database.DB {
	h.t.Helper()

	// Create temp directory for database
	tempDir := h.CreateTempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.Open(database.Config{
		Path:             dbPath,
		EncryptionKey:    "test_key_1234567890123456", // 32 chars for testing
		CreateIfNotExist: true,
	})
	if err != nil {
		h.t.Fatalf("failed to open test database: %v", err)
	}

	h.db = db
	h.t.Cleanup(func() {
		db.Close()
	})

	return db
}

// CreateTestJob creates a test sync job in the database
func (h *TestHelpers) CreateTestJob(db *database.DB, localPath, remotePath string) int64 {
	h.t.Helper()

	now := time.Now().Unix()
	result, err := db.Conn().Exec(`
		INSERT INTO sync_jobs (
			name, local_path, remote_path, server_credential_id,
			sync_mode, trigger_mode, enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "Test Job", localPath, remotePath, "test_cred", "mirror", "manual", true, now, now)

	if err != nil {
		h.t.Fatalf("failed to create test job: %v", err)
	}

	jobID, err := result.LastInsertId()
	if err != nil {
		h.t.Fatalf("failed to get job ID: %v", err)
	}

	return jobID
}

// GetTestLogger creates a test logger (no-op or console)
func (h *TestHelpers) GetTestLogger(verbose bool) *zap.Logger {
	h.t.Helper()

	if verbose {
		logger, err := zap.NewDevelopment()
		if err != nil {
			h.t.Fatalf("failed to create logger: %v", err)
		}
		return logger
	}

	return zap.NewNop()
}

// AssertFileExists asserts that a file exists
func (h *TestHelpers) AssertFileExists(path string) {
	h.t.Helper()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		h.t.Errorf("file does not exist: %s", path)
	}
}

// AssertFileNotExists asserts that a file does not exist
func (h *TestHelpers) AssertFileNotExists(path string) {
	h.t.Helper()

	if _, err := os.Stat(path); err == nil {
		h.t.Errorf("file exists but should not: %s", path)
	}
}

// AssertNoError asserts that error is nil
func (h *TestHelpers) AssertNoError(err error, msgAndArgs ...interface{}) {
	h.t.Helper()

	if err != nil {
		msg := "unexpected error"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError asserts that error is not nil
func (h *TestHelpers) AssertError(err error, msgAndArgs ...interface{}) {
	h.t.Helper()

	if err == nil {
		msg := "expected error but got nil"
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...)
		}
		h.t.Fatal(msg)
	}
}

// AssertEqual asserts that two values are equal
func (h *TestHelpers) AssertEqual(expected, actual interface{}, msgAndArgs ...interface{}) {
	h.t.Helper()

	if expected != actual {
		msg := fmt.Sprintf("expected %v, got %v", expected, actual)
		if len(msgAndArgs) > 0 {
			msg = fmt.Sprintf(msgAndArgs[0].(string), msgAndArgs[1:]...) + ": " + msg
		}
		h.t.Error(msg)
	}
}

// Cleanup cleans up test resources
func (h *TestHelpers) Cleanup() {
	if h.db != nil {
		h.db.Close()
	}
	if h.tempDir != "" {
		os.RemoveAll(h.tempDir)
	}
}
