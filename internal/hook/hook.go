package hook

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/store"
	"lukechampine.com/blake3"
)

// Run is the main hook entry point, invoked by Claude Code after each tool use.
// NOTE: This hook is DEPRECATED - steps are now created by the Stop hook (one step per conversation turn).
// This is kept for backward compatibility but does nothing.
func Run(stdin io.Reader, stdout io.Writer) error {
	// TODO: Remove this hook entirely once we confirm Stop hook works
	return nil
}

// logError writes errors to .regent/log/hook-error.log instead of stdout/stderr
// Returns nil so the hook doesn't break the agent
func logError(s *store.Store, err error) error {
	logPath := filepath.Join(s.Root, "log", "hook-error.log")
	f, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		// Can't even log - give up silently
		return nil
	}
	defer func() { _ = f.Close() }()

	timestamp := time.Now().Format(time.RFC3339)
	_, _ = fmt.Fprintf(f, "[%s] %v\n", timestamp, err)

	// Return nil so hook exits cleanly and doesn't break the agent
	return nil
}

// computePreStepHash creates a deterministic hash before step exists
// Used as currentStep reference in blame computation
func computePreStepHash(parent, tree, argsBlob, resultBlob store.Hash, p Payload) store.Hash {
	h := blake3.New(32, nil)
	h.Write([]byte(parent))
	h.Write([]byte(tree))
	h.Write([]byte(argsBlob))
	h.Write([]byte(resultBlob))
	h.Write([]byte(p.SessionID))
	h.Write([]byte(p.ToolUseID))
	return store.Hash(hex.EncodeToString(h.Sum(nil)))
}

// updateBlameWithRealStepHash replaces pre-step hash with real step hash in all blame maps
func updateBlameWithRealStepHash(s *store.Store, treeHash, preStepHash, realStepHash store.Hash) (store.Hash, error) {
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return "", err
	}

	modified := false
	for i := range tree.Entries {
		entry := &tree.Entries[i]

		if entry.Blame == "" {
			continue
		}

		// Read blame map
		blameMap, err := s.ReadBlame(entry.Blame)
		if err != nil {
			continue // Skip if can't read
		}

		// Replace pre-step hash with real step hash
		changed := false
		for j := range blameMap.Lines {
			if blameMap.Lines[j] == preStepHash {
				blameMap.Lines[j] = realStepHash
				changed = true
			}
		}

		if changed {
			// Write updated blame map
			newBlameHash, err := s.WriteBlame(blameMap)
			if err != nil {
				return "", err
			}
			entry.Blame = newBlameHash
			modified = true
		}
	}

	if !modified {
		return treeHash, nil
	}

	// Write updated tree
	return s.WriteTree(tree)
}

// computeBlameForChanges computes blame for files that changed since parent
// Returns new tree hash with blame map references populated
func computeBlameForChanges(s *store.Store, parentHash, treeHash, currentStep store.Hash) (store.Hash, error) {
	// Read current tree
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return "", fmt.Errorf("read tree: %w", err)
	}

	// Read parent tree (if exists)
	var parentTree *store.Tree
	if parentHash != "" {
		parentStep, err := s.ReadStep(parentHash)
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

	// Compute blame for each file
	modified := false
	for i := range tree.Entries {
		entry := &tree.Entries[i]
		parentEntry := parentEntries[entry.Path]

		// Skip if file unchanged (same blob hash)
		if parentEntry != nil && parentEntry.Blob == entry.Blob {
			// Inherit parent blame
			if parentEntry.Blame != "" && entry.Blame != parentEntry.Blame {
				entry.Blame = parentEntry.Blame
				modified = true
			}
			continue
		}

		// File is new or modified - compute blame
		newContent, err := s.ReadBlob(entry.Blob)
		if err != nil {
			return "", fmt.Errorf("read blob %s: %w", entry.Blob, err)
		}

		var oldContent []byte
		var oldBlame *store.BlameMap
		if parentEntry != nil {
			// Modified file - get old content and blame
			oldContent, _ = s.ReadBlob(parentEntry.Blob)
			if parentEntry.Blame != "" {
				oldBlame, _ = s.ReadBlame(parentEntry.Blame)
			}
		}
		// else: new file, oldContent and oldBlame stay nil

		// Compute new blame
		newBlame := store.ComputeBlame(oldContent, newContent, oldBlame, currentStep)

		// Write blame map to store
		blameHash, err := s.WriteBlame(newBlame)
		if err != nil {
			return "", fmt.Errorf("write blame: %w", err)
		}

		entry.Blame = blameHash
		modified = true
	}

	if !modified {
		// No changes, return original tree hash
		return treeHash, nil
	}

	// Write updated tree with blame references
	return s.WriteTree(tree)
}

// shouldSkipStep returns true if this tool call should not create a step.
// Currently filters out rgt commands run via Bash tool to avoid self-referential logs.
func shouldSkipStep(p *Payload) bool {
	if p.ToolName != "Bash" {
		return false
	}

	var args map[string]interface{}
	if err := json.Unmarshal(p.ToolInput, &args); err != nil {
		return false // Parse error - don't skip
	}

	cmd, ok := args["command"].(string)
	if !ok {
		return false
	}

	trimmed := strings.TrimSpace(cmd)
	return trimmed == "rgt" || strings.HasPrefix(trimmed, "rgt ")
}
