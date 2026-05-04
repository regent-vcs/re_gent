package treediff

import (
	"bytes"
	"fmt"

	"github.com/regent-vcs/regent/internal/diff"
	"github.com/regent-vcs/regent/internal/store"
)

// FileDiff represents a file change between two steps
type FileDiff struct {
	Path      string
	Status    string // "added", "modified", "deleted"
	OldBlob   store.Hash
	NewBlob   store.Hash
	Additions int // lines added
	Deletions int // lines deleted
	IsBinary  bool
}

// CompareTreesForDiff computes file changes between parent and current step
func CompareTreesForDiff(s *store.Store, parentStepHash, currentStepHash store.Hash) ([]FileDiff, error) {
	// Read current step and tree
	currentStep, err := s.ReadStep(currentStepHash)
	if err != nil {
		return nil, fmt.Errorf("read current step: %w", err)
	}

	currentTree, err := s.ReadTree(currentStep.Tree)
	if err != nil {
		return nil, fmt.Errorf("read current tree: %w", err)
	}

	// Read parent tree (if exists)
	var parentTree *store.Tree
	if parentStepHash != "" {
		parentStep, err := s.ReadStep(parentStepHash)
		if err == nil {
			parentTree, _ = s.ReadTree(parentStep.Tree)
		}
	}

	// Build map of parent entries for fast lookup
	parentEntries := make(map[string]*store.TreeEntry)
	if parentTree != nil {
		for i := range parentTree.Entries {
			parentEntries[parentTree.Entries[i].Path] = &parentTree.Entries[i]
		}
	}

	// Track which parent files we've seen
	seenParentPaths := make(map[string]bool)

	// Collect file diffs
	var diffs []FileDiff

	// Check current tree entries
	for i := range currentTree.Entries {
		entry := &currentTree.Entries[i]
		parentEntry := parentEntries[entry.Path]

		if parentEntry != nil {
			seenParentPaths[entry.Path] = true

			// Skip if file unchanged (same blob hash)
			if parentEntry.Blob == entry.Blob {
				continue
			}

			// File modified
			additions, deletions, isBinary, err := computeLineStats(s, parentEntry.Blob, entry.Blob)
			if err != nil {
				// If we can't compute stats, still record the change
				diffs = append(diffs, FileDiff{
					Path:    entry.Path,
					Status:  "modified",
					OldBlob: parentEntry.Blob,
					NewBlob: entry.Blob,
				})
				continue
			}

			diffs = append(diffs, FileDiff{
				Path:      entry.Path,
				Status:    "modified",
				OldBlob:   parentEntry.Blob,
				NewBlob:   entry.Blob,
				Additions: additions,
				Deletions: deletions,
				IsBinary:  isBinary,
			})
		} else {
			// File added
			additions, deletions, isBinary, err := computeLineStats(s, "", entry.Blob)
			if err != nil {
				diffs = append(diffs, FileDiff{
					Path:    entry.Path,
					Status:  "added",
					NewBlob: entry.Blob,
				})
				continue
			}

			diffs = append(diffs, FileDiff{
				Path:      entry.Path,
				Status:    "added",
				NewBlob:   entry.Blob,
				Additions: additions,
				Deletions: deletions,
				IsBinary:  isBinary,
			})
		}
	}

	// Check for deleted files (in parent but not in current)
	if parentTree != nil {
		for i := range parentTree.Entries {
			entry := &parentTree.Entries[i]
			if !seenParentPaths[entry.Path] {
				// File deleted
				additions, deletions, isBinary, err := computeLineStats(s, entry.Blob, "")
				if err != nil {
					diffs = append(diffs, FileDiff{
						Path:    entry.Path,
						Status:  "deleted",
						OldBlob: entry.Blob,
					})
					continue
				}

				diffs = append(diffs, FileDiff{
					Path:      entry.Path,
					Status:    "deleted",
					OldBlob:   entry.Blob,
					Additions: additions,
					Deletions: deletions,
					IsBinary:  isBinary,
				})
			}
		}
	}

	return diffs, nil
}

// computeLineStats computes line additions and deletions using diff.LineDiff
func computeLineStats(s *store.Store, oldBlobHash, newBlobHash store.Hash) (additions, deletions int, isBinary bool, err error) {
	var oldContent, newContent []byte

	if oldBlobHash != "" {
		oldContent, err = s.ReadBlob(oldBlobHash)
		if err != nil {
			return 0, 0, false, fmt.Errorf("read old blob: %w", err)
		}
	}

	if newBlobHash != "" {
		newContent, err = s.ReadBlob(newBlobHash)
		if err != nil {
			return 0, 0, false, fmt.Errorf("read new blob: %w", err)
		}
	}

	// Check for binary content (null bytes)
	if isBinaryContent(oldContent) || isBinaryContent(newContent) {
		return 0, 0, true, nil
	}

	// Compute line diff
	opcodes := diff.LineDiff(oldContent, newContent)

	// Sum line changes from opcodes
	for _, op := range opcodes {
		switch op.Tag {
		case "insert":
			additions += op.J2 - op.J1
		case "delete":
			deletions += op.I2 - op.I1
		case "replace":
			additions += op.J2 - op.J1
			deletions += op.I2 - op.I1
		}
	}

	return additions, deletions, false, nil
}

// isBinaryContent checks if content contains null bytes (common binary indicator)
func isBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	// Check first 8KB for null bytes
	limit := len(content)
	if limit > 8192 {
		limit = 8192
	}
	return bytes.IndexByte(content[:limit], 0) != -1
}
