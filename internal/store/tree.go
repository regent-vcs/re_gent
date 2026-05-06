package store

import (
	"encoding/json"
	"fmt"
	"sort"
)

// TreeEntry represents a file in a tree
type TreeEntry struct {
	Path string `json:"path"`
	Blob Hash   `json:"blob"`
	Mode uint32 `json:"mode,omitempty"` // unix mode (executable bit matters)
}

// Tree represents the workspace at one moment
type Tree struct {
	Entries []TreeEntry `json:"entries"` // sorted by Path for deterministic hashing
}

// WriteTree writes a tree to the object store
func (s *Store) WriteTree(tree *Tree) (Hash, error) {
	// Sort entries by path for deterministic hashing
	sort.Slice(tree.Entries, func(i, j int) bool {
		return tree.Entries[i].Path < tree.Entries[j].Path
	})

	data, err := json.Marshal(tree)
	if err != nil {
		return "", fmt.Errorf("marshal tree: %w", err)
	}

	return s.WriteBlob(data)
}

// ReadTree reads a tree from the object store
func (s *Store) ReadTree(h Hash) (*Tree, error) {
	data, err := s.ReadBlob(h)
	if err != nil {
		return nil, err
	}

	var tree Tree
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("unmarshal tree %s: %w", h, err)
	}

	return &tree, nil
}

// FindEntry finds an entry by path in the tree
func (t *Tree) FindEntry(path string) *TreeEntry {
	for i := range t.Entries {
		if t.Entries[i].Path == path {
			return &t.Entries[i]
		}
	}
	return nil
}
