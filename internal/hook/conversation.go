package hook

import (
	"encoding/json"
	"fmt"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/jsonl"
	"github.com/regent-vcs/regent/internal/store"
)

// stageConversation extracts messages since last hook and writes them to the transcript chain
func stageConversation(s *store.Store, idx *index.DB, p Payload) (store.Hash, error) {
	// If no transcript path provided, skip staging (non-Claude Code hooks)
	if p.TranscriptPath == "" {
		return "", nil
	}

	// Find last processed message for this session
	lastMsgID, prevTranscript, err := idx.SessionLastProcessedMessage(p.SessionID)
	if err != nil {
		return "", fmt.Errorf("get last processed message: %w", err)
	}

	// Extract messages from JSONL: (lastMsgID, p.ToolUseID]
	newMsgs, err := jsonl.ExtractRange(p.TranscriptPath, lastMsgID, p.ToolUseID)
	if err != nil {
		return "", fmt.Errorf("extract messages: %w", err)
	}

	// No new messages (shouldn't happen, but handle gracefully)
	if len(newMsgs) == 0 {
		return prevTranscript, nil
	}

	// Write each message as a blob
	var msgHashes []store.Hash
	for _, msg := range newMsgs {
		// Canonicalize JSON (stable formatting for deduplication)
		canonical, err := canonicalJSON(msg)
		if err != nil {
			return "", fmt.Errorf("canonicalize message: %w", err)
		}

		h, err := s.WriteBlob(canonical)
		if err != nil {
			return "", fmt.Errorf("write message blob: %w", err)
		}
		msgHashes = append(msgHashes, h)
	}

	// Write transcript node (links to previous)
	transcriptHash, err := s.WriteTranscript(prevTranscript, msgHashes)
	if err != nil {
		return "", fmt.Errorf("write transcript: %w", err)
	}

	// Update index with last processed message
	lastIngestedID := jsonl.MessageID(newMsgs[len(newMsgs)-1])
	if err := idx.UpdateSessionLastProcessed(p.SessionID, lastIngestedID, transcriptHash); err != nil {
		return "", fmt.Errorf("update index: %w", err)
	}

	return transcriptHash, nil
}

// canonicalJSON re-encodes JSON in stable format (compact, sorted keys)
func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	var obj interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		// If JSON is malformed, just return the raw bytes
		// This can happen if JSONL is being written while we read it
		return raw, nil
	}
	return json.Marshal(obj) // Go's json.Marshal produces stable output
}
