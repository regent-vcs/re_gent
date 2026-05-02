package jsonl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

// Message represents a Claude Code transcript message
type Message struct {
	ID        string          `json:"id,omitempty"`
	Type      string          `json:"type"`    // "user", "assistant", "tool_use", "tool_result"
	Content   json.RawMessage `json:"content"` // Varies by type
	ToolUseID string          `json:"tool_use_id,omitempty"`
}

// ExtractRange reads messages from JSONL between (afterID, upToToolUseID]
// Returns messages in chronological order
// If afterID is empty, starts from beginning
// If afterID is not found (e.g., after /compact), starts from beginning with warning
func ExtractRange(jsonlPath string, afterID string, upToToolUseID string) ([]json.RawMessage, error) {
	f, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("open JSONL: %w", err)
	}
	defer f.Close()

	var messages []json.RawMessage
	var foundAfter bool
	if afterID == "" {
		foundAfter = true // Start from beginning
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse enough to get ID and tool_use_id
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip malformed lines (happens during writes)
			continue
		}

		// If we're looking for afterID and haven't found it yet
		if !foundAfter {
			if msg.ID == afterID || msg.ToolUseID == afterID {
				foundAfter = true
			}
			continue
		}

		// Collect this message
		messages = append(messages, json.RawMessage(line))

		// Stop if we reached the target tool use
		if msg.ToolUseID == upToToolUseID {
			break
		}
		if msg.ID == upToToolUseID {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan JSONL: %w", err)
	}

	// If afterID was provided but never found (probably /compact happened),
	// we need to re-scan from the beginning
	if afterID != "" && !foundAfter {
		// Recursively call with empty afterID to start from beginning
		return ExtractRange(jsonlPath, "", upToToolUseID)
	}

	return messages, nil
}

// MessageID extracts the ID from a message (handles different formats)
func MessageID(msg json.RawMessage) string {
	var m Message
	if err := json.Unmarshal(msg, &m); err != nil {
		return ""
	}
	if m.ID != "" {
		return m.ID
	}
	if m.ToolUseID != "" {
		return m.ToolUseID
	}
	return ""
}
