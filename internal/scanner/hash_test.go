package scanner

import (
	"fmt"
	"testing"
	"time"
)

func TestHasher_EmptyFile(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create empty file
	emptyFile := h.CreateTestFile(tempDir+"/empty.txt", []byte{})

	// Create hasher
	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	// Compute hash
	result, err := hasher.ComputeHash(emptyFile)
	h.AssertNoError(err, "compute hash for empty file")

	// SHA256 of empty file is well-known
	expectedHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	h.AssertEqual(expectedHash, result.Hash, "hash of empty file")
	h.AssertEqual(int64(0), result.Size, "size of empty file")
}

func TestHasher_SmallFile(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create file with known content
	content := []byte("hello world")
	testFile := h.CreateTestFile(tempDir+"/hello.txt", content)

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	result, err := hasher.ComputeHash(testFile)
	h.AssertNoError(err, "compute hash for small file")

	// SHA256 of "hello world"
	expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	h.AssertEqual(expectedHash, result.Hash, "hash of 'hello world'")
	h.AssertEqual(int64(len(content)), result.Size, "size of small file")
}

func TestHasher_MediumFile(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create 1MB file
	mediumFile := h.CreateTestFileWithSize(tempDir+"/medium.bin", 1*1024*1024)

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	result, err := hasher.ComputeHash(mediumFile)
	h.AssertNoError(err, "compute hash for medium file")

	// Verify hash is not empty and has correct length (64 hex chars)
	if len(result.Hash) != 64 {
		t.Errorf("expected hash length 64, got %d", len(result.Hash))
	}

	h.AssertEqual(int64(1*1024*1024), result.Size, "size of medium file")

	// Verify hash is consistent
	result2, err := hasher.ComputeHash(mediumFile)
	h.AssertNoError(err, "compute hash again")
	h.AssertEqual(result.Hash, result2.Hash, "hash should be consistent")
}

func TestHasher_LargeFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large file test in short mode")
	}

	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create 100MB file
	largeFile := h.CreateTestFileWithSize(tempDir+"/large.bin", 100*1024*1024)

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	start := time.Now()
	result, err := hasher.ComputeHash(largeFile)
	duration := time.Since(start)

	h.AssertNoError(err, "compute hash for large file")
	h.AssertEqual(int64(100*1024*1024), result.Size, "size of large file")

	// Verify performance: should complete in reasonable time (< 3s on SSD)
	if duration > 3*time.Second {
		t.Logf("WARNING: Large file hash took %v (expected < 3s)", duration)
	}

	t.Logf("Hashed 100MB in %v (%.2f MB/s)", duration, 100.0/duration.Seconds())
}

func TestHasher_FileNotFound(t *testing.T) {
	h := NewTestHelpers(t)

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	result, err := hasher.ComputeHash("/nonexistent/file.txt")
	h.AssertError(err, "should error on nonexistent file")

	// Verify error is wrapped correctly
	if result == nil || result.Err == nil {
		t.Error("result should contain error")
	}
}

func TestHasher_ComputeHashHex(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	content := []byte("test")
	testFile := h.CreateTestFile(tempDir+"/test.txt", content)

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	// Test convenience method
	hash, err := hasher.ComputeHashHex(testFile)
	h.AssertNoError(err, "compute hash hex")

	// SHA256 of "test"
	expectedHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	h.AssertEqual(expectedHash, hash, "hash of 'test'")
}

func TestHasher_VerifyHash(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	content := []byte("verify me")
	testFile := h.CreateTestFile(tempDir+"/verify.txt", content)

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	// Compute hash first
	hash, err := hasher.ComputeHashHex(testFile)
	h.AssertNoError(err, "compute initial hash")

	// Verify correct hash
	matches, err := hasher.VerifyHash(testFile, hash)
	h.AssertNoError(err, "verify hash")
	if !matches {
		t.Error("hash should match")
	}

	// Verify incorrect hash
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	matches, err = hasher.VerifyHash(testFile, wrongHash)
	h.AssertNoError(err, "verify wrong hash")
	if matches {
		t.Error("wrong hash should not match")
	}
}

func TestHasher_BufferSizes(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create 10MB file
	testFile := h.CreateTestFileWithSize(tempDir+"/buftest.bin", 10*1024*1024)

	// Test with different buffer sizes
	bufferSizes := []int{1, 2, 4, 8, 16} // MB

	var expectedHash string
	for i, bufSize := range bufferSizes {
		hasher := NewHasher("sha256", bufSize, h.GetTestLogger(false))
		result, err := hasher.ComputeHash(testFile)
		h.AssertNoError(err, "compute hash with %dMB buffer", bufSize)

		if i == 0 {
			expectedHash = result.Hash
		} else {
			// All buffer sizes should produce same hash
			h.AssertEqual(expectedHash, result.Hash,
				"hash should be same regardless of buffer size")
		}
	}
}

func TestHasher_ConcurrentHashing(t *testing.T) {
	h := NewTestHelpers(t)
	tempDir := h.CreateTempDir()

	// Create multiple test files
	files := make([]string, 10)
	for i := 0; i < 10; i++ {
		files[i] = h.CreateTestFileWithSize(fmt.Sprintf("%s/concurrent_%d.bin", tempDir, i), 1*1024*1024)
	}

	hasher := NewHasher("sha256", 4, h.GetTestLogger(false))

	// Hash files concurrently
	done := make(chan bool, 10)
	for _, file := range files {
		go func(f string) {
			_, err := hasher.ComputeHash(f)
			if err != nil {
				t.Errorf("concurrent hash failed: %v", err)
			}
			done <- true
		}(file)
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// --- Benchmarks ---

func BenchmarkHashSmallFile_1KB(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	testFile := h.CreateTestFileWithSize(tempDir+"/bench_1kb.bin", 1*1024)
	defer h.Cleanup()

	hasher := NewHasher("sha256", 4, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hasher.ComputeHash(testFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHashMediumFile_1MB(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	testFile := h.CreateTestFileWithSize(tempDir+"/bench_1mb.bin", 1*1024*1024)
	defer h.Cleanup()

	hasher := NewHasher("sha256", 4, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hasher.ComputeHash(testFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHashLargeFile_100MB(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	testFile := h.CreateTestFileWithSize(tempDir+"/bench_100mb.bin", 100*1024*1024)
	defer h.Cleanup()

	hasher := NewHasher("sha256", 4, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hasher.ComputeHash(testFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkHashWithDifferentBuffers(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	testFile := h.CreateTestFileWithSize(tempDir+"/bench_buffers.bin", 10*1024*1024)
	defer h.Cleanup()

	bufferSizes := []int{1, 2, 4, 8, 16}

	for _, bufSize := range bufferSizes {
		b.Run("Buffer"+string(rune(bufSize))+"MB", func(b *testing.B) {
			hasher := NewHasher("sha256", bufSize, nil)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := hasher.ComputeHash(testFile)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkMemoryUsage tests memory usage during hashing
func BenchmarkMemoryUsage(b *testing.B) {
	h := NewTestHelpers(&testing.T{})
	tempDir := h.CreateTempDir()
	// Create large file to test memory efficiency
	testFile := h.CreateTestFileWithSize(tempDir+"/bench_memory.bin", 100*1024*1024)
	defer h.Cleanup()

	hasher := NewHasher("sha256", 4, nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hasher.ComputeHash(testFile)
		if err != nil {
			b.Fatal(err)
		}
	}
}
