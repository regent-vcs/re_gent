package store

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"lukechampine.com/blake3"
)

// WriteBlob writes content to the object store and returns its hash.
// Content-addressed: identical content produces identical hash.
func (s *Store) WriteBlob(content []byte) (Hash, error) {
	h := blake3.Sum256(content)
	hashStr := hex.EncodeToString(h[:])

	// Shard by first 2 hex chars for filesystem efficiency
	dir := filepath.Join(s.Root, "objects", hashStr[:2])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create shard directory: %w", err)
	}

	objPath := filepath.Join(dir, hashStr)

	// If object already exists, we're done (content-addressed deduplication)
	if _, err := os.Stat(objPath); err == nil {
		return Hash(hashStr), nil
	}

	// Write atomically: temp file + rename
	if err := atomicWriteFile(objPath, content); err != nil {
		return "", fmt.Errorf("write blob: %w", err)
	}

	return Hash(hashStr), nil
}

// ReadBlob reads a blob from the object store by hash
func (s *Store) ReadBlob(h Hash) ([]byte, error) {
	if len(h) < 2 {
		return nil, fmt.Errorf("invalid hash: too short")
	}

	objPath := filepath.Join(s.Root, "objects", string(h[:2]), string(h))
	content, err := os.ReadFile(objPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("blob %s not found", h)
		}
		return nil, fmt.Errorf("read blob %s: %w", h, err)
	}

	return content, nil
}

// atomicWriteFile writes content to path atomically using temp file + rename
func atomicWriteFile(path string, content []byte) error {
	return atomicWriteFileWithReplace(path, content, false)
}

func atomicWriteFileReplacing(path string, content []byte) error {
	return atomicWriteFileWithReplace(path, content, true)
}

func atomicWriteFileWithReplace(path string, content []byte, replaceExisting bool) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on error
	defer func() {
		if tmpFile != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(content); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		return fmt.Errorf("sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	tmpFile = nil // Mark as closed to prevent cleanup

	// Rename is atomic on POSIX filesystems
	if err := os.Rename(tmpPath, path); err != nil {
		if !replaceExisting || !errors.Is(err, os.ErrExist) && !os.IsExist(err) && !strings.Contains(strings.ToLower(err.Error()), "access is denied") {
			return fmt.Errorf("rename temp file: %w", err)
		}
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("replace existing file: %w", removeErr)
		}
		if err := os.Rename(tmpPath, path); err != nil {
			return fmt.Errorf("rename temp file after replace: %w", err)
		}
	}

	// Set permissions after rename
	if err := os.Chmod(path, 0o444); err != nil {
		// Non-fatal: object is written, just not read-only
		_ = err
	}

	return nil
}

// ObjectExists checks if a blob exists without reading it
func (s *Store) ObjectExists(h Hash) bool {
	if len(h) < 2 {
		return false
	}
	objPath := filepath.Join(s.Root, "objects", string(h[:2]), string(h))
	_, err := os.Stat(objPath)
	return err == nil
}

// WalkObjects walks all objects in the store
func (s *Store) WalkObjects(fn func(Hash) error) error {
	objectsDir := filepath.Join(s.Root, "objects")
	return filepath.WalkDir(objectsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Extract hash from path: objects/ab/abcdef... -> abcdef...
		rel, err := filepath.Rel(objectsDir, path)
		if err != nil || len(rel) < 3 {
			return nil
		}
		// Reconstruct full hash: ab/abcdef... -> ababcdef...
		hash := Hash(filepath.Base(path))
		return fn(hash)
	})
}
