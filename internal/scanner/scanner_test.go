package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juste-un-gars/anemone_sync_windows/internal/config"
)

func TestScanner_FirstScan(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	// Create test files
	h.CreateTestFiles(tempDir, 10, 1024) // 10 files, 1KB each

	// Create job
	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	// Create scanner
	cfg := &config.Config{
		Paths: config.PathsConfig{
			ConfigDir: tempDir,
		},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// Perform first scan
	result, err := scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})

	h.AssertNoError(err, "first scan")

	// Verify results
	h.AssertEqual(10, result.TotalFiles, "total files")
	h.AssertEqual(10, len(result.NewFiles), "new files")
	h.AssertEqual(0, len(result.ModifiedFiles), "modified files")
	h.AssertEqual(0, len(result.UnchangedFiles), "unchanged files")
	h.AssertEqual(0, len(result.DeletedFiles), "deleted files")

	// Verify database
	states, err := db.GetAllFileStates(jobID)
	h.AssertNoError(err, "get file states")
	h.AssertEqual(10, len(states), "file states in DB")

	// Verify hashes are not empty
	for _, state := range states {
		if state.Hash == "" {
			t.Error("hash should not be empty")
		}
	}
}

func TestScanner_NoChanges(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	// Create test files
	h.CreateTestFiles(tempDir, 10, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// First scan
	_, err = scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "first scan")

	// Second scan immediately
	result, err := scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "second scan")

	// Should find no new or modified files
	h.AssertEqual(0, len(result.NewFiles), "new files on rescan")
	h.AssertEqual(0, len(result.ModifiedFiles), "modified files on rescan")
	h.AssertEqual(10, len(result.UnchangedFiles), "unchanged files")

	// Verify high skip rate (should not hash unchanged files)
	skipRate := float64(result.SkippedFiles) / float64(result.TotalFiles) * 100
	t.Logf("Skip rate: %.1f%%", skipRate)
}

func TestScanner_ModifiedFiles(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	// Create files
	files := h.CreateTestFiles(tempDir, 10, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// First scan
	_, err = scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "first scan")

	// Modify 3 files
	for i := 0; i < 3; i++ {
		os.WriteFile(files[i], []byte("modified content"), 0644)
	}

	// Second scan
	result, err := scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "second scan")

	// Should detect 3 modified files
	h.AssertEqual(3, len(result.ModifiedFiles), "modified files detected")
	h.AssertEqual(7, len(result.UnchangedFiles), "unchanged files")
}

func TestScanner_DeletedFiles(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	// Create files
	files := h.CreateTestFiles(tempDir, 10, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// First scan
	_, err = scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "first scan")

	// Delete 2 files
	os.Remove(files[0])
	os.Remove(files[1])

	// Second scan
	result, err := scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "second scan")

	// Should detect 2 deleted files
	h.AssertEqual(2, len(result.DeletedFiles), "deleted files detected")
	h.AssertEqual(8, result.TotalFiles, "remaining files")
}

func TestScanner_WithExclusions(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	// Create regular files
	h.CreateTestFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"))
	h.CreateTestFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"))

	// Create excluded files
	h.CreateTestFile(filepath.Join(tempDir, "temp.tmp"), []byte("temp"))
	os.Mkdir(filepath.Join(tempDir, "node_modules"), 0755)
	h.CreateTestFile(filepath.Join(tempDir, "node_modules", "pkg.js"), []byte("js"))

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// Scan
	result, err := scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})
	h.AssertNoError(err, "scan with exclusions")

	// Should only find 2 regular files
	h.AssertEqual(2, result.TotalFiles, "total files (excluded not counted)")

	// Verify walk stats show exclusions
	if result.WalkStats.ExcludedFiles == 0 {
		t.Error("should have excluded temp.tmp")
	}
	if result.WalkStats.ExcludedDirs == 0 {
		t.Error("should have excluded node_modules/")
	}
}

func TestScanner_ContextCancellation(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	// Create many files
	h.CreateTestFiles(tempDir, 100, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Scan should be interrupted
	_, err = scanner.Scan(ctx, ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})

	// Should error due to context cancellation
	if err == nil {
		t.Error("expected error from context cancellation")
	}
}

func TestScanner_ConcurrentScansBlocked(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()
	db := h.SetupTestDB()

	h.CreateTestFiles(tempDir, 10, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, err := NewScanner(cfg, db, h.GetTestLogger(false))
	h.AssertNoError(err, "create scanner")
	defer scanner.Close()

	// Start first scan
	go func() {
		scanner.Scan(context.Background(), ScanRequest{
			JobID:      jobID,
			BasePath:   tempDir,
			RemoteBase: "\\\\server\\share",
		})
	}()

	// Wait a bit to ensure first scan started
	time.Sleep(10 * time.Millisecond)

	// Try to start second scan (should be blocked)
	_, err = scanner.Scan(context.Background(), ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	})

	// Should error because scan already in progress
	h.AssertError(err, "concurrent scan should be blocked")
}

// --- Benchmarks ---

func BenchmarkScanner_1000Files(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	defer h.Cleanup()
	db := h.SetupTestDB()

	h.CreateTestFiles(tempDir, 1000, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, _ := NewScanner(cfg, db, nil)
	defer scanner.Close()

	req := ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.Scan(context.Background(), req)
	}
}

func BenchmarkScanner_ReScan(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	defer h.Cleanup()
	db := h.SetupTestDB()

	h.CreateTestFiles(tempDir, 1000, 1024)

	jobID := h.CreateTestJob(db, tempDir, "\\\\server\\share")

	cfg := &config.Config{
		Paths: config.PathsConfig{ConfigDir: tempDir},
		Sync: config.SyncConfig{
			Performance: config.PerformanceConfig{
				HashAlgorithm: "sha256",
				BufferSizeMB:  4,
			},
		},
	}

	scanner, _ := NewScanner(cfg, db, nil)
	defer scanner.Close()

	req := ScanRequest{
		JobID:      jobID,
		BasePath:   tempDir,
		RemoteBase: "\\\\server\\share",
	}

	// First scan to populate database
	scanner.Scan(context.Background(), req)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		scanner.Scan(context.Background(), req)
	}
}
