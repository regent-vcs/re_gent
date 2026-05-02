package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveShortHash finds a full hash matching the prefix
// Returns error if no match or multiple matches found
func (s *Store) ResolveShortHash(prefix string) (Hash, error) {
	if len(prefix) < 4 {
		return "", fmt.Errorf("hash prefix too short (need at least 4 chars)")
	}

	// Object stored at objects/aa/aabbccdd...
	subdir := prefix[:2]
	subdirPath := filepath.Join(s.Root, "objects", subdir)

	entries, err := os.ReadDir(subdirPath)
	if err != nil {
		return "", fmt.Errorf("read objects/%s: %w", subdir, err)
	}

	var matches []string
	for _, e := range entries {
		// e.Name() is the full hash (without subdir prefix in filename)
		fullHash := e.Name()
		if strings.HasPrefix(fullHash, prefix) {
			matches = append(matches, fullHash)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no object found matching %s", prefix)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous hash prefix %s (matches %d objects)", prefix, len(matches))
	}

	return Hash(matches[0]), nil
}
