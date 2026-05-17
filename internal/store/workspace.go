package store

import (
	"errors"
	"os"
	"path/filepath"
)

// ErrNotRegentRepository is returned when no .regent directory exists in the
// working tree.
var ErrNotRegentRepository = errors.New(`not a re_gent repository (or any parent directory)

Hint: Initialize re_gent in this directory by running:
    rgt init`)

// FindRegentDir walks from start toward the filesystem root and returns the
// path to the first .regent directory found.
func FindRegentDir(start string) (string, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(abs, ".regent")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.IsDir() {
			return candidate, nil
		}

		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}

	return "", ErrNotRegentRepository
}

// OpenFromDir locates .regent from start and opens the store.
func OpenFromDir(start string) (*Store, error) {
	regentDir, err := FindRegentDir(start)
	if err != nil {
		return nil, err
	}
	return Open(regentDir)
}
