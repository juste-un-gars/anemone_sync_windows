package scanner

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"

	"go.uber.org/zap"
)

// Hasher computes file hashes using chunked reading for memory efficiency
type Hasher struct {
	algorithm  string      // Hash algorithm (currently only "sha256")
	bufferSize int         // Buffer size for chunked reading (in bytes)
	logger     *zap.Logger // Logger for progress and errors
}

// HashResult contains the result of a hash computation
type HashResult struct {
	Hash     string        // Hex-encoded hash
	Size     int64         // File size in bytes
	Duration time.Duration // Time taken to compute hash
	Err      error         // Error if any
}

// NewHasher creates a new Hasher instance
// bufferSizeMB is the buffer size in megabytes (typically 4MB)
func NewHasher(algorithm string, bufferSizeMB int, logger *zap.Logger) *Hasher {
	if algorithm == "" {
		algorithm = "sha256"
	}
	if bufferSizeMB <= 0 {
		bufferSizeMB = 4 // Default 4MB
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Hasher{
		algorithm:  algorithm,
		bufferSize: bufferSizeMB * 1024 * 1024, // Convert MB to bytes
		logger:     logger.With(zap.String("component", "hasher")),
	}
}

// ComputeHash computes the hash of a file at the given path
// Uses chunked reading to handle large files efficiently without loading entire file into memory
func (h *Hasher) ComputeHash(path string) (*HashResult, error) {
	start := time.Now()

	result := &HashResult{}

	// Open file for reading
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			result.Err = WrapError(ErrFileNotFound, "open file for hashing %s", path)
		} else if os.IsPermission(err) {
			result.Err = WrapError(ErrAccessDenied, "open file for hashing %s", path)
		} else {
			result.Err = WrapError(ErrReadFailed, "open file for hashing %s", path)
		}
		return result, result.Err
	}
	defer file.Close()

	// Get file size
	info, err := file.Stat()
	if err != nil {
		result.Err = WrapError(ErrReadFailed, "stat file for hashing %s", path)
		return result, result.Err
	}
	result.Size = info.Size()

	// Compute hash based on algorithm
	var hashBytes []byte
	switch h.algorithm {
	case "sha256":
		hashBytes, err = h.computeSHA256(file)
	default:
		result.Err = fmt.Errorf("unsupported hash algorithm: %s", h.algorithm)
		return result, result.Err
	}

	if err != nil {
		result.Err = WrapError(ErrHashFailed, "compute %s hash for %s", h.algorithm, path)
		return result, result.Err
	}

	// Convert hash to hex string
	result.Hash = hex.EncodeToString(hashBytes)
	result.Duration = time.Since(start)

	h.logger.Debug("hash computed",
		zap.String("path", path),
		zap.String("hash", result.Hash),
		zap.Int64("size", result.Size),
		zap.Duration("duration", result.Duration))

	return result, nil
}

// computeSHA256 computes SHA256 hash using chunked reading
func (h *Hasher) computeSHA256(reader io.Reader) ([]byte, error) {
	hasher := sha256.New()
	buffer := make([]byte, h.bufferSize)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			_, writeErr := hasher.Write(buffer[:n])
			if writeErr != nil {
				return nil, writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return hasher.Sum(nil), nil
}

// ComputeHashHex is a convenience method that returns only the hex hash string
func (h *Hasher) ComputeHashHex(path string) (string, error) {
	result, err := h.ComputeHash(path)
	if err != nil {
		return "", err
	}
	return result.Hash, nil
}

// VerifyHash verifies that a file's hash matches the expected hash
func (h *Hasher) VerifyHash(path string, expectedHash string) (bool, error) {
	result, err := h.ComputeHash(path)
	if err != nil {
		return false, err
	}
	return result.Hash == expectedHash, nil
}
