package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/spf13/cobra"
)

func ToolBatchHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "tool-batch-hook",
		Short:  "Internal: Claude Code PostToolBatch hook",
		RunE:   runToolBatchHook,
		Hidden: true,
	}
}

type toolBatchPayload struct {
	ToolCalls []struct {
		ToolName     string          `json:"tool_name"`
		ToolInput    json.RawMessage `json:"tool_input"`
		ToolUseID    string          `json:"tool_use_id"`
		ToolResponse string          `json:"tool_response"`
	} `json:"tool_calls"`
	SessionID string `json:"session_id"`
	CWD       string `json:"cwd"`
}

func runToolBatchHook(cmd *cobra.Command, args []string) error {
	// Read payload from stdin
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	var p toolBatchPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Open store from project directory
	regentDir := filepath.Join(p.CWD, ".regent")
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

	// Store each tool call and result
	for _, toolCall := range p.ToolCalls {
		// Store tool input as blob
		inputHash, err := s.WriteBlob(toolCall.ToolInput)
		if err != nil {
			return fmt.Errorf("store tool input: %w", err)
		}

		// Store tool output as blob
		outputData := []byte(toolCall.ToolResponse)
		outputHash, err := s.WriteBlob(outputData)
		if err != nil {
			return fmt.Errorf("store tool output: %w", err)
		}

		// Get sequence numbers
		callSeqNum, err := idx.GetNextMessageSeq(p.SessionID)
		if err != nil {
			return fmt.Errorf("get sequence: %w", err)
		}

		resultSeqNum, err := idx.GetNextMessageSeq(p.SessionID)
		if err != nil {
			return fmt.Errorf("get sequence: %w", err)
		}

		now := time.Now().UnixNano()

		// Store tool_call message
		callMsg := index.Message{
			ID:          fmt.Sprintf("msg_%d_call", now),
			SessionID:   p.SessionID,
			SeqNum:      callSeqNum,
			Timestamp:   now,
			MessageType: "tool_call",
			ToolName:    toolCall.ToolName,
			ToolUseID:   toolCall.ToolUseID,
			ToolInput:   string(inputHash),
		}

		if err := idx.InsertMessage(callMsg); err != nil {
			return fmt.Errorf("insert tool call: %w", err)
		}

		// Store tool_result message
		resultMsg := index.Message{
			ID:          fmt.Sprintf("msg_%d_result", now),
			SessionID:   p.SessionID,
			SeqNum:      resultSeqNum,
			Timestamp:   now + 1, // Slight offset to ensure ordering
			MessageType: "tool_result",
			ToolName:    toolCall.ToolName,
			ToolUseID:   toolCall.ToolUseID,
			ToolOutput:  string(outputHash),
		}

		if err := idx.InsertMessage(resultMsg); err != nil {
			return fmt.Errorf("insert tool result: %w", err)
		}
	}

	return nil
}
