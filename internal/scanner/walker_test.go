package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWalker_BasicWalk(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create test structure
	h.CreateTestFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"))
	h.CreateTestFile(filepath.Join(tempDir, "file2.txt"), []byte("content2"))
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	h.CreateTestFile(filepath.Join(tempDir, "subdir", "file3.txt"), []byte("content3"))

	excluder := NewExcluder(h.GetTestLogger(false))
	walker := NewWalker(excluder, h.GetTestLogger(false))

	// Walk and count files
	fileCount := 0
	err := walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		fileCount++
		return nil
	})

	h.AssertNoError(err, "walk directory")
	h.AssertEqual(3, fileCount, "file count")

	stats := walker.GetStatistics()
	h.AssertEqual(3, stats.TotalFiles, "total files in stats")
	h.AssertEqual(1, stats.TotalDirs, "total dirs in stats (subdir)")
}

func TestWalker_ExcludedDirectory(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create test structure with excluded directory
	h.CreateTestFile(filepath.Join(tempDir, "file1.txt"), []byte("content1"))
	os.Mkdir(filepath.Join(tempDir, "node_modules"), 0755)
	h.CreateTestFile(filepath.Join(tempDir, "node_modules", "package.json"), []byte("{}"))
	h.CreateTestFile(filepath.Join(tempDir, "node_modules", "index.js"), []byte("code"))

	excluder := NewExcluder(h.GetTestLogger(false))
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	err := excluder.LoadDefaultExclusions(configPath)
	h.AssertNoError(err, "load exclusions")

	walker := NewWalker(excluder, h.GetTestLogger(false))

	// Walk and collect paths
	var foundPaths []string
	err = walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		foundPaths = append(foundPaths, path)
		return nil
	})

	h.AssertNoError(err, "walk directory")

	// Should only find file1.txt, node_modules should be excluded
	h.AssertEqual(1, len(foundPaths), "should only find 1 file")

	// Verify node_modules was excluded
	stats := walker.GetStatistics()
	if stats.ExcludedDirs == 0 {
		t.Error("node_modules directory should be excluded")
	}
}

func TestWalker_NestedDirectories(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create nested structure
	h.CreateNestedStructure(tempDir, 5, 2) // 5 levels, 2 files per level

	excluder := NewExcluder(h.GetTestLogger(false))
	walker := NewWalker(excluder, h.GetTestLogger(false))

	fileCount := 0
	err := walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		fileCount++
		return nil
	})

	h.AssertNoError(err, "walk nested directories")

	// Should find 2 files per level × 5 levels = 10 files
	h.AssertEqual(10, fileCount, "files in nested structure")
}

func TestWalker_ExcludedFiles(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create files with different extensions
	h.CreateTestFile(filepath.Join(tempDir, "document.txt"), []byte("text"))
	h.CreateTestFile(filepath.Join(tempDir, "temporary.tmp"), []byte("temp"))
	h.CreateTestFile(filepath.Join(tempDir, "backup.bak"), []byte("backup"))
	h.CreateTestFile(filepath.Join(tempDir, ".DS_Store"), []byte("meta"))

	excluder := NewExcluder(h.GetTestLogger(false))
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	err := excluder.LoadDefaultExclusions(configPath)
	h.AssertNoError(err, "load exclusions")

	walker := NewWalker(excluder, h.GetTestLogger(false))

	var foundFiles []string
	err = walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		foundFiles = append(foundFiles, filepath.Base(path))
		return nil
	})

	h.AssertNoError(err, "walk directory")

	// Should only find document.txt
	h.AssertEqual(1, len(foundFiles), "should only find non-excluded files")
	h.AssertEqual("document.txt", foundFiles[0], "found file name")

	stats := walker.GetStatistics()
	if stats.ExcludedFiles == 0 {
		t.Error("should have excluded some files")
	}
}

func TestWalker_EmptyDirectory(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Don't create any files

	excluder := NewExcluder(h.GetTestLogger(false))
	walker := NewWalker(excluder, h.GetTestLogger(false))

	fileCount := 0
	err := walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		fileCount++
		return nil
	})

	h.AssertNoError(err, "walk empty directory")
	h.AssertEqual(0, fileCount, "should find no files in empty directory")
}

func TestWalker_NonexistentDirectory(t *testing.T) {
	h := NewTestHelpers(t)

	excluder := NewExcluder(h.GetTestLogger(false))
	walker := NewWalker(excluder, h.GetTestLogger(false))

	err := walker.Walk(1, "/nonexistent/directory", func(path string, metadata *FileMetadata) error {
		return nil
	})

	// Should return error for nonexistent directory
	h.AssertError(err, "should error on nonexistent directory")
}

func TestWalker_Statistics(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create mixed structure
	h.CreateTestFile(filepath.Join(tempDir, "file1.txt"), []byte("1"))
	h.CreateTestFile(filepath.Join(tempDir, "file2.txt"), []byte("2"))
	h.CreateTestFile(filepath.Join(tempDir, "skip.tmp"), []byte("temp"))
	os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	h.CreateTestFile(filepath.Join(tempDir, "subdir", "file3.txt"), []byte("3"))
	os.Mkdir(filepath.Join(tempDir, "node_modules"), 0755)
	h.CreateTestFile(filepath.Join(tempDir, "node_modules", "pkg.js"), []byte("js"))

	excluder := NewExcluder(h.GetTestLogger(false))
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	excluder.LoadDefaultExclusions(configPath)

	walker := NewWalker(excluder, h.GetTestLogger(false))

	walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		return nil
	})

	stats := walker.GetStatistics()

	// Should have found 3 regular files (file1, file2, file3)
	h.AssertEqual(3, stats.TotalFiles, "total files")

	// Should have excluded 1 file (skip.tmp) and 1 dir (node_modules)
	if stats.ExcludedFiles == 0 {
		t.Error("should have excluded skip.tmp")
	}
	if stats.ExcludedDirs == 0 {
		t.Error("should have excluded node_modules")
	}
}

func TestWalker_WalkFuncError(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	h.CreateTestFile(filepath.Join(tempDir, "file1.txt"), []byte("1"))
	h.CreateTestFile(filepath.Join(tempDir, "file2.txt"), []byte("2"))

	excluder := NewExcluder(h.GetTestLogger(false))
	walker := NewWalker(excluder, h.GetTestLogger(false))

	fileCount := 0
	err := walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		fileCount++
		// Don't return error, walker should continue
		return nil
	})

	h.AssertNoError(err, "walk should complete even with errors in walkFn")
	h.AssertEqual(2, fileCount, "should process all files")
}

func TestWalker_FollowSymlinks(t *testing.T) {
	// Skip on Windows as symlink creation requires admin privileges
	if os.Getenv("GOOS") == "windows" {
		t.Skip("skipping symlink test on Windows")
	}

	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create a file and a symlink to it
	targetFile := h.CreateTestFile(filepath.Join(tempDir, "target.txt"), []byte("content"))
	symlinkPath := filepath.Join(tempDir, "link.txt")

	err := os.Symlink(targetFile, symlinkPath)
	if err != nil {
		t.Skip("cannot create symlink:", err)
	}

	excluder := NewExcluder(h.GetTestLogger(false))
	walker := NewWalker(excluder, h.GetTestLogger(false))

	// By default, symlinks should be skipped
	walker.SetFollowSymlinks(false)

	fileCount := 0
	walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
		fileCount++
		return nil
	})

	// Should only find target.txt, symlink should be skipped
	h.AssertEqual(1, fileCount, "should skip symlinks by default")

	stats := walker.GetStatistics()
	if stats.SymlinksSkipped == 0 {
		t.Error("should have skipped at least one symlink")
	}
}

// --- Benchmarks ---

func BenchmarkWalker_1000Files(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	defer h.Cleanup()

	// Create 1000 files
	h.CreateTestFiles(tempDir, 1000, 1024) // 1KB each

	excluder := NewExcluder(nil)
	walker := NewWalker(excluder, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
			return nil
		})
	}
}

func BenchmarkWalker_WithExclusions(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	defer h.Cleanup()

	// Create mixed files
	h.CreateTestFiles(tempDir, 500, 1024)

	// Create excluded files
	os.Mkdir(filepath.Join(tempDir, "node_modules"), 0755)
	h.CreateTestFiles(filepath.Join(tempDir, "node_modules"), 500, 1024)

	excluder := NewExcluder(nil)
	configPath := filepath.Join("..", "..", "configs", "default_exclusions.json")
	excluder.LoadDefaultExclusions(configPath)

	walker := NewWalker(excluder, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		walker.Walk(1, tempDir, func(path string, metadata *FileMetadata) error {
			return nil
		})
	}
}
