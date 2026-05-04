package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/spf13/cobra"
)

func MessageHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "message-hook [user|assistant]",
		Short:  "Internal: Claude Code hook for capturing messages",
		Args:   cobra.ExactArgs(1),
		RunE:   runMessageHook,
		Hidden: true,
	}
}

type userPromptPayload struct {
	Prompt         string `json:"prompt"`
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
}

type stopPayload struct {
	LastAssistantMessage string `json:"last_assistant_message"`
	SessionID            string `json:"session_id"`
	TranscriptPath       string `json:"transcript_path"`
	CWD                  string `json:"cwd"`
}

func runMessageHook(cmd *cobra.Command, args []string) error {
	hookType := args[0]

	// Read payload from stdin
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	// Parse payload to get CWD
	var cwdPayload struct {
		CWD string `json:"cwd"`
	}
	if err := json.Unmarshal(payload, &cwdPayload); err != nil {
		return fmt.Errorf("parse cwd: %w", err)
	}

	// Open store from project directory
	regentDir := filepath.Join(cwdPayload.CWD, ".regent")
	s, err := store.Open(regentDir)
	if err != nil {
		// Not initialized, silently exit
		return nil
	}

	// Open index
	idx, err := index.Open(s)
	if err != nil {
		return fmt.Errorf("open index: %w", err)
	}
	defer func() { _ = idx.Close() }()

	switch hookType {
	case "user":
		return handleUserPrompt(idx, payload)
	case "assistant":
		return handleAssistantResponse(idx, s, payload)
	default:
		return fmt.Errorf("unknown hook type: %s", hookType)
	}
}

func handleUserPrompt(idx *index.DB, payload []byte) error {
	var p userPromptPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Get next sequence number for session
	seqNum, err := idx.GetNextMessageSeq(p.SessionID)
	if err != nil {
		return fmt.Errorf("get sequence: %w", err)
	}

	// Store user message
	msg := index.Message{
		ID:          generateMessageID(),
		SessionID:   p.SessionID,
		SeqNum:      seqNum,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "user",
		ContentText: p.Prompt,
	}

	if err := idx.InsertMessage(msg); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	return nil
}

func handleAssistantResponse(idx *index.DB, s *store.Store, payload []byte) error {
	var p stopPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Get next sequence number for session
	seqNum, err := idx.GetNextMessageSeq(p.SessionID)
	if err != nil {
		return fmt.Errorf("get sequence: %w", err)
	}

	// Store assistant message
	msg := index.Message{
		ID:          generateMessageID(),
		SessionID:   p.SessionID,
		SeqNum:      seqNum,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "assistant",
		ContentText: p.LastAssistantMessage,
	}

	if err := idx.InsertMessage(msg); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	// NOW CREATE THE STEP (one step per conversation turn)
	if err := createStepForTurn(s, idx, p.SessionID, p.CWD); err != nil {
		return fmt.Errorf("create step: %w", err)
	}

	// Archive JSONL snapshot
	if p.TranscriptPath != "" {
		if err := archiveJSONL(idx, s, p.SessionID, p.TranscriptPath); err != nil {
			// Don't fail the hook if archiving fails
			fmt.Fprintf(os.Stderr, "warning: failed to archive JSONL: %v\n", err)
		}
	}

	return nil
}

func archiveJSONL(idx *index.DB, s *store.Store, sessionID, transcriptPath string) error {
	// Read JSONL file
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		return fmt.Errorf("read transcript: %w", err)
	}

	// Store as blob
	blobHash, err := s.WriteBlob(data)
	if err != nil {
		return fmt.Errorf("write blob: %w", err)
	}

	// Record snapshot
	return idx.InsertJSONLSnapshot(sessionID, time.Now().UnixNano(), blobHash)
}

func generateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// createStepForTurn creates one step for the entire conversation turn
// This captures all tools executed in this turn (multi-tool support)
func createStepForTurn(s *store.Store, idx *index.DB, sessionID, cwd string) error {
	// Get parent step
	parentHash, _ := s.ReadRef("sessions/" + sessionID)

	// Get all orphan messages (not yet linked to a step)
	orphanMessages, err := idx.GetOrphanMessages(sessionID)
	if err != nil {
		return fmt.Errorf("get orphan messages: %w", err)
	}

	// Extract tool calls from messages
	var causes []store.Cause
	for _, msg := range orphanMessages {
		if msg.MessageType == "tool_call" && msg.ToolInput != "" {
			// Read tool input blob
			inputData, err := s.ReadBlob(store.Hash(msg.ToolInput))
			if err != nil {
				continue
			}
			argsHash, _ := s.WriteBlob(inputData)

			// Find corresponding tool_result
			var resultHash store.Hash
			for _, resultMsg := range orphanMessages {
				if resultMsg.MessageType == "tool_result" && resultMsg.ToolUseID == msg.ToolUseID {
					if resultMsg.ToolOutput != "" {
						outputData, _ := s.ReadBlob(store.Hash(resultMsg.ToolOutput))
						resultHash, _ = s.WriteBlob(outputData)
					}
					break
				}
			}

			causes = append(causes, store.Cause{
				ToolUseID:  msg.ToolUseID,
				ToolName:   msg.ToolName,
				ArgsBlob:   argsHash,
				ResultBlob: resultHash,
			})
		}
	}

	// If no tools, don't create a step (conversation-only turn)
	if len(causes) == 0 {
		return nil
	}

	// Snapshot workspace
	treeHash, err := snapshotWorkspace(s, cwd)
	if err != nil {
		return fmt.Errorf("snapshot workspace: %w", err)
	}

	// Compute blame for modified files
	treeHash, err = computeBlameForTree(s, parentHash, treeHash)
	if err != nil {
		return fmt.Errorf("compute blame: %w", err)
	}

	// Create step with multiple causes
	step := &store.Step{
		Parent:         parentHash,
		Tree:           treeHash,
		Causes:         causes,
		SessionID:      sessionID,
		TimestampNanos: time.Now().UnixNano(),
	}

	// For backward compat, also set single Cause field to first tool
	if len(causes) > 0 {
		step.Cause = causes[0]
	}

	stepHash, err := s.WriteStep(step)
	if err != nil {
		return fmt.Errorf("write step: %w", err)
	}

	// Update session ref with CAS retry
	if err := s.UpdateRefWithRetry("sessions/"+sessionID, parentHash, stepHash, 8); err != nil {
		return fmt.Errorf("update ref: %w", err)
	}

	// Read tree for indexing
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return fmt.Errorf("read tree: %w", err)
	}

	// Index the step (best effort)
	_ = idx.IndexStep(stepHash, step, tree)

	// Link all orphan messages to this step
	_ = idx.LinkMessagesToStep(sessionID, stepHash)

	return nil
}

func snapshotWorkspace(s *store.Store, cwd string) (store.Hash, error) {
	// Load ignore patterns
	ig, err := loadIgnorePatterns(cwd)
	if err != nil {
		// If can't load ignore patterns, use empty matcher
		ig = &ignore.Matcher{}
	}

	// Snapshot using the existing snapshot package
	return snapshot.Snapshot(s, cwd, ig)
}

func loadIgnorePatterns(root string) (*ignore.Matcher, error) {
	// Use the Default matcher which loads .regentignore or uses default patterns
	return ignore.Default(root), nil
}

func computeBlameForTree(s *store.Store, parentHash, treeHash store.Hash) (store.Hash, error) {
	// Read current tree
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return "", err
	}

	// Read parent tree if exists
	var parentTree *store.Tree
	if parentHash != "" {
		parentStep, err := s.ReadStep(parentHash)
		if err == nil {
			parentTree, _ = s.ReadTree(parentStep.Tree)
		}
	}

	// Compute blame for each file
	for i := range tree.Entries {
		entry := &tree.Entries[i]

		// Find file in parent tree
		var parentEntry *store.TreeEntry
		if parentTree != nil {
			for j := range parentTree.Entries {
				if parentTree.Entries[j].Path == entry.Path {
					parentEntry = &parentTree.Entries[j]
					break
				}
			}
		}

		// If file unchanged (same blob hash), inherit parent blame
		if parentEntry != nil && parentEntry.Blob == entry.Blob && parentEntry.Blame != "" {
			entry.Blame = parentEntry.Blame
			continue
		}

		// File is new or modified - compute blame
		var oldContent []byte
		var oldBlame *store.BlameMap

		if parentEntry != nil {
			// Read old content
			oldContent, _ = s.ReadBlob(parentEntry.Blob)
			// Read old blame if exists
			if parentEntry.Blame != "" {
				oldBlame, _ = s.ReadBlame(parentEntry.Blame)
			}
		}

		// Read new content
		newContent, err := s.ReadBlob(entry.Blob)
		if err != nil {
			continue // Skip if can't read
		}

		// Compute blame (use a placeholder hash since we don't have the step yet)
		// We'll use empty hash for now - this is a simplification
		// The proper way would be to use a pre-step hash, but that's complex
		newBlame := store.ComputeBlame(oldContent, newContent, oldBlame, treeHash)

		// Write blame map
		blameHash, err := s.WriteBlame(newBlame)
		if err != nil {
			continue // Skip if can't write
		}

		entry.Blame = blameHash
	}

	// Write updated tree with blame hashes
	return s.WriteTree(tree)
}
