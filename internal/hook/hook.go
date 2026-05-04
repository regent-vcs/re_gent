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

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
	"lukechampine.com/blake3"
)

// Run is the main hook entry point, invoked by Claude Code after each tool use.
// It reads a Payload from stdin, captures workspace state, and creates a step.
func Run(stdin io.Reader, stdout io.Writer) error {
	// 1. Decode payload from stdin
	var p Payload
	if err := json.NewDecoder(stdin).Decode(&p); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	// 2. Filter out rgt commands to avoid self-referential logs
	if shouldSkipStep(&p) {
		return nil
	}

	// 3. Open store (fail silently if .regent/ doesn't exist)
	regentDir := filepath.Join(p.CWD, ".regent")
	s, err := store.Open(regentDir)
	if err != nil {
		// Not initialized - silently no-op (don't break the agent)
		return nil
	}

	// 3. Open index
	idx, err := index.Open(s)
	if err != nil {
		return logError(s, fmt.Errorf("open index: %w", err))
	}
	defer func() { _ = idx.Close() }()

	// 4. Snapshot workspace (without blame yet)
	ig := ignore.Default(p.CWD)
	treeHash, err := snapshot.Snapshot(s, p.CWD, ig)
	if err != nil {
		return logError(s, fmt.Errorf("snapshot: %w", err))
	}

	// 5. Get parent step (if any)
	parentHash, _ := s.ReadRef("sessions/" + p.SessionID)

	// 6. Stage conversation (Phase 4)
	transcriptHash, err := stageConversation(s, idx, p)
	if err != nil {
		return logError(s, fmt.Errorf("stage conversation: %w", err))
	}

	// 7. Write tool args and result as blobs (needed for pre-step hash)
	argsHash, err := s.WriteBlob(p.ToolInput)
	if err != nil {
		return logError(s, fmt.Errorf("write args blob: %w", err))
	}

	resultHash, err := s.WriteBlob(p.ToolResponse)
	if err != nil {
		return logError(s, fmt.Errorf("write result blob: %w", err))
	}

	// 8. Compute deterministic step hash (will be the same after we add blame to tree)
	preStepHash := computePreStepHash(parentHash, treeHash, argsHash, resultHash, p)

	// 9. Compute blame for changed files using pre-step hash
	treeHash, err = computeBlameForChanges(s, parentHash, treeHash, preStepHash)
	if err != nil {
		return logError(s, fmt.Errorf("compute blame: %w", err))
	}

	// 10. Build step object (with transcript)
	stepWithoutTree := &store.Step{
		Parent:         parentHash,
		Tree:           treeHash,
		Transcript:     transcriptHash, // NEW - Phase 4
		SessionID:      p.SessionID,
		TimestampNanos: time.Now().UnixNano(),
		Cause: store.Cause{
			ToolUseID:  p.ToolUseID,
			ToolName:   p.ToolName,
			ArgsBlob:   argsHash,
			ResultBlob: resultHash,
		},
	}

	// 11. Write step (first time - to get step hash)
	stepHash, err := s.WriteStep(stepWithoutTree)
	if err != nil {
		return logError(s, fmt.Errorf("write step: %w", err))
	}

	// 12. Update blame maps to use actual step hash
	// This requires rewriting the step with the updated tree
	if preStepHash != stepHash {
		updatedTreeHash, err := updateBlameWithRealStepHash(s, treeHash, preStepHash, stepHash)
		if err == nil && updatedTreeHash != treeHash {
			// Tree was updated - rewrite step with corrected tree
			stepWithoutTree.Tree = updatedTreeHash
			finalStepHash, err := s.WriteStep(stepWithoutTree)
			if err == nil {
				// Use the final step hash
				stepHash = finalStepHash
				treeHash = updatedTreeHash
			}
		}
	}

	// 13. Read tree for indexing
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return logError(s, fmt.Errorf("read tree: %w", err))
	}

	// 14. CAS update session ref (with retry) - SOURCE OF TRUTH
	// Refs must be updated first to maintain consistency. If this fails, nothing is committed.
	if err := s.UpdateRefWithRetry("sessions/"+p.SessionID, parentHash, stepHash, 8); err != nil {
		return logError(s, fmt.Errorf("update ref: %w", err))
	}

	// 15. Index the step (best effort - derived index)
	// If indexing fails, refs/objects are still consistent and user can run `rgt reindex`.
	if err := idx.IndexStep(stepHash, stepWithoutTree, tree); err != nil {
		// Log error but don't fail hook - refs/objects are source of truth
		_ = logError(s, fmt.Errorf("index step (non-fatal): %w", err))
		// Continue - refs are updated, that's what matters
	}

	// Success - exit silently
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
