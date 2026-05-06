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
		return handleUserPrompt(idx, s, payload)
	case "assistant":
		return handleAssistantResponse(idx, s, payload)
	default:
		return fmt.Errorf("unknown hook type: %s", hookType)
	}
}

func handleUserPrompt(idx *index.DB, s *store.Store, payload []byte) error {
	var p userPromptPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		logDebug(s, fmt.Sprintf("UserPrompt: parse error: %v", err))
		return fmt.Errorf("parse payload: %w", err)
	}

	logDebug(s, fmt.Sprintf("UserPrompt: session=%s prompt=%s", p.SessionID, truncateString(p.Prompt, 50)))

	// Get next sequence number for session
	seqNum, err := idx.GetNextMessageSeq(p.SessionID)
	if err != nil {
		logDebug(s, fmt.Sprintf("UserPrompt: seq error: %v", err))
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
		logDebug(s, fmt.Sprintf("UserPrompt: insert error: %v", err))
		return fmt.Errorf("insert message: %w", err)
	}

	logDebug(s, "UserPrompt: success")
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

func truncateString(str string, max int) string {
	if len(str) <= max {
		return str
	}
	return str[:max] + "..."
}

func logDebug(s *store.Store, msg string) {
	logPath := filepath.Join(s.Root, "log", "hook-debug.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	timestamp := time.Now().Format("2006-01-02T15:04:05-07:00")
	fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
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

	// 1. Snapshot workspace (tree without blame)
	treeHash, err := snapshotWorkspace(s, cwd)
	if err != nil {
		return fmt.Errorf("snapshot workspace: %w", err)
	}

	// 2. Write step with clean tree (no circular dependency!)
	step := &store.Step{
		Parent:         parentHash,
		Tree:           treeHash,
		Causes:         causes,
		SessionID:      sessionID,
		TimestampNanos: time.Now().UnixNano(),
	}

	if len(causes) > 0 {
		step.Cause = causes[0] // Backward compat
	}

	stepHash, err := s.WriteStep(step)
	if err != nil {
		return fmt.Errorf("write step: %w", err)
	}

	logDebug(s, fmt.Sprintf("Created step: %s", stepHash[:16]))

	// 3. NOW compute blame using the real step hash
	if err := computeAndWriteBlame(s, parentHash, stepHash, treeHash); err != nil {
		logDebug(s, fmt.Sprintf("Compute blame error: %v", err))
		// Don't fail the hook if blame computation fails
	}

	// 4. Update session ref
	if err := s.UpdateRefWithRetry("sessions/"+sessionID, parentHash, stepHash, 8); err != nil {
		return fmt.Errorf("update ref: %w", err)
	}

	// 5. Index step
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return fmt.Errorf("read tree: %w", err)
	}

	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		logDebug(s, fmt.Sprintf("Index step error: %v", err))
	}

	// 6. Link messages to step
	if err := idx.LinkMessagesToStep(sessionID, stepHash); err != nil {
		logDebug(s, fmt.Sprintf("Link messages error: %v", err))
	}

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

// computeAndWriteBlame computes blame for all files and writes to separate storage
func computeAndWriteBlame(s *store.Store, parentHash, currentStepHash, treeHash store.Hash) error {
	// Read current tree
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return err
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
	for _, entry := range tree.Entries {
		// Find file in parent tree
		var parentEntry *store.TreeEntry
		if parentTree != nil {
			for i := range parentTree.Entries {
				if parentTree.Entries[i].Path == entry.Path {
					parentEntry = &parentTree.Entries[i]
					break
				}
			}
		}

		// If file unchanged (same blob hash), inherit parent blame
		if parentEntry != nil && parentEntry.Blob == entry.Blob {
			// Copy blame from parent step to current step
			oldBlame, err := s.ReadBlameForFile(parentHash, entry.Path)
			if err == nil {
				if err := s.WriteBlameForFile(currentStepHash, entry.Path, oldBlame); err != nil {
					logDebug(s, fmt.Sprintf("Failed to copy blame for %s: %v", entry.Path, err))
				}
				continue
			}
			// Parent blame doesn't exist - need to create initial blame
			// Fall through to compute blame for this file
		}

		// File is new or modified - compute blame
		var oldContent []byte
		var oldBlame *store.BlameMap

		if parentEntry != nil {
			oldContent, _ = s.ReadBlob(parentEntry.Blob)
			oldBlame, _ = s.ReadBlameForFile(parentHash, parentEntry.Path)
		}

		newContent, err := s.ReadBlob(entry.Blob)
		if err != nil {
			continue
		}

		// Compute blame using the REAL step hash (no placeholder!)
		newBlame := store.ComputeBlame(oldContent, newContent, oldBlame, currentStepHash)

		// Write blame to separate storage
		if err := s.WriteBlameForFile(currentStepHash, entry.Path, newBlame); err != nil {
			logDebug(s, fmt.Sprintf("Failed to write blame for %s: %v", entry.Path, err))
		}
	}

	return nil
}

