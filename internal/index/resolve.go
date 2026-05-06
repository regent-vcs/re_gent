package index

import (
	"fmt"
	"strings"

	"github.com/regent-vcs/regent/internal/store"
)

// ResolveStepHash resolves a short hash prefix to a full step hash
// Supports git-style abbreviated hashes (minimum 7 characters)
func (idx *DB) ResolveStepHash(prefix string) (store.Hash, error) {
	if len(prefix) < 7 {
		return "", fmt.Errorf("hash prefix must be at least 7 characters")
	}

	// Query steps with matching prefix
	query := `
		SELECT id FROM steps
		WHERE id LIKE ? || '%'
		LIMIT 2
	`

	rows, err := idx.db.Query(query, prefix)
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

	var matches []string
	for rows.Next() {
		var hash string
		if err := rows.Scan(&hash); err != nil {
			return "", err
		}
		matches = append(matches, hash)
	}

	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no step found matching prefix: %s", prefix)
	}

	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous hash prefix %s (matches %d steps)", prefix, len(matches))
	}

	return store.Hash(matches[0]), nil
}

// NormalizeStepHash converts a short or full hash to full hash
// Returns the input unchanged if already full length, otherwise resolves it
func (idx *DB) NormalizeStepHash(hash string) (store.Hash, error) {
	hash = strings.TrimSpace(hash)

	// Full hash (64 hex chars for BLAKE3)
	if len(hash) == 64 {
		return store.Hash(hash), nil
	}

	// Short hash - resolve it
	return idx.ResolveStepHash(hash)
}
