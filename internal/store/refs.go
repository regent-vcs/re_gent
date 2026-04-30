package store

import (
	"errors"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var ErrRefConflict = errors.New("ref was modified concurrently; retry")

// ReadRef reads a ref and returns its hash
func (s *Store) ReadRef(name string) (Hash, error) {
	refPath := filepath.Join(s.Root, "refs", name)
	data, err := os.ReadFile(refPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fs.ErrNotExist
		}
		return "", fmt.Errorf("read ref %s: %w", name, err)
	}

	hashStr := strings.TrimSpace(string(data))
	return Hash(hashStr), nil
}

// UpdateRef updates a ref using CAS (compare-and-swap) with lock files
// expectedOld can be "" for a new ref
func (s *Store) UpdateRef(name string, expectedOld Hash, newHash Hash) error {
	refPath := filepath.Join(s.Root, "refs", name)
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		return err
	}
	lockPath := refPath + ".lock"

	// O_CREATE|O_EXCL is atomic across POSIX filesystems
	fd, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_EXCL|syscall.O_WRONLY, 0o644)
	if err != nil {
		if errors.Is(err, syscall.EEXIST) {
			return ErrRefConflict
		}
		return fmt.Errorf("acquire lock: %w", err)
	}

	// Cleanup lock file when done
	defer func() {
		syscall.Close(fd)
		os.Remove(lockPath)
	}()

	// Read current value
	current, err := s.ReadRef(name)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	// Check CAS condition
	if current != expectedOld {
		return ErrRefConflict
	}

	// Write new value
	newContent := string(newHash) + "\n"
	return atomicWriteFile(refPath, []byte(newContent))
}

// UpdateRefWithRetry updates a ref with exponential backoff retry on conflicts
func (s *Store) UpdateRefWithRetry(name string, expectedOld, newHash Hash, maxAttempts int) error {
	backoff := 5 * time.Millisecond
	for i := 0; i < maxAttempts; i++ {
		err := s.UpdateRef(name, expectedOld, newHash)
		if err == nil {
			return nil
		}
		if !errors.Is(err, ErrRefConflict) {
			return err
		}

		// Exponential backoff with jitter
		jitter := time.Duration(rand.Int63n(int64(backoff)))
		time.Sleep(backoff + jitter)
		backoff *= 2
		if backoff > 100*time.Millisecond {
			backoff = 100 * time.Millisecond
		}

		// Re-read current value for next attempt
		expectedOld, _ = s.ReadRef(name)
	}
	return ErrRefConflict
}

// ListRefs lists all refs in a directory (e.g., "sessions")
func (s *Store) ListRefs(dir string) (map[string]Hash, error) {
	refsDir := filepath.Join(s.Root, "refs", dir)
	refs := make(map[string]Hash)

	err := filepath.WalkDir(refsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		// Skip lock files
		if strings.HasSuffix(path, ".lock") {
			return nil
		}

		rel, _ := filepath.Rel(refsDir, path)
		hash, err := s.ReadRef(filepath.Join(dir, rel))
		if err != nil {
			return err
		}
		refs[rel] = hash
		return nil
	})

	return refs, err
}
