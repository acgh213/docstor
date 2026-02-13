package attachments

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Storage interface for file storage backends
type Storage interface {
	Store(tenantID string, sha256Hash string, r io.Reader) (storageKey string, err error)
	Retrieve(storageKey string) (io.ReadCloser, error)
	Delete(storageKey string) error
}

// LocalStorage stores files on local disk
type LocalStorage struct {
	BasePath string
}

// NewLocalStorage creates a new local storage backend
func NewLocalStorage(basePath string) (*LocalStorage, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("create storage dir: %w", err)
	}
	return &LocalStorage{BasePath: basePath}, nil
}

// Store saves a file to local disk
// Files are stored at: basePath/tenantID/ab/sha256hash
func (s *LocalStorage) Store(tenantID, sha256Hash string, r io.Reader) (string, error) {
	// Use first 2 chars of hash as subdirectory to avoid too many files in one dir
	subdir := sha256Hash[:2]
	dir := filepath.Join(s.BasePath, tenantID, subdir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create subdir: %w", err)
	}

	storageKey := filepath.Join(tenantID, subdir, sha256Hash)
	fullPath := filepath.Join(s.BasePath, storageKey)

	// Check if file already exists (deduplication)
	if _, err := os.Stat(fullPath); err == nil {
		return storageKey, nil
	}

	// Write to temp file first, then rename for atomicity
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		os.Remove(tmpPath) // Clean up on error
	}()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close file: %w", err)
	}

	if err := os.Rename(tmpPath, fullPath); err != nil {
		return "", fmt.Errorf("rename file: %w", err)
	}

	return storageKey, nil
}

// Retrieve opens a file from local disk
func (s *LocalStorage) Retrieve(storageKey string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.BasePath, storageKey)
	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	return f, nil
}

// Delete removes a file from local disk
func (s *LocalStorage) Delete(storageKey string) error {
	fullPath := filepath.Join(s.BasePath, storageKey)
	return os.Remove(fullPath)
}

// ComputeSHA256 calculates SHA256 hash while reading and returns both hash and content.
// maxSize limits how much data to read (0 = unlimited). Returns error if limit exceeded.
func ComputeSHA256(r io.Reader) (hash string, content []byte, err error) {
	h := sha256.New()
	data, err := io.ReadAll(io.TeeReader(r, h))
	if err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(h.Sum(nil)), data, nil
}

// ComputeSHA256Streaming hashes a file without buffering the entire content in memory.
// It writes to a temp file while hashing, then returns the hash and temp file path.
// Caller is responsible for removing the temp file.
func ComputeSHA256Streaming(r io.Reader, tmpDir string) (hash string, tmpPath string, size int64, err error) {
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", "", 0, fmt.Errorf("create tmp dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpDir, ".upload-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath = tmpFile.Name()

	h := sha256.New()
	w := io.MultiWriter(tmpFile, h)

	size, err = io.Copy(w, r)
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return "", "", 0, fmt.Errorf("copy to temp: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), tmpPath, size, nil
}
