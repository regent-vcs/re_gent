package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/regent-vcs/regent/internal/capture"
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
		ToolResponse json.RawMessage `json:"tool_response"`
	} `json:"tool_calls"`
	SessionID      string `json:"session_id"`
	TurnID         string `json:"turn_id"`
	CWD            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
	Model          string `json:"model"`
	PermissionMode string `json:"permission_mode"`
}

func runToolBatchHook(cmd *cobra.Command, args []string) error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	var payload toolBatchPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return logHookPayloadError(raw, fmt.Errorf("parse payload: %w", err))
	}

	cwd, err := hookCWD(payload.CWD)
	if err != nil {
		return err
	}

	return withHookRecorder(cwd, func(recorder *capture.Recorder) error {
		meta := capture.SessionMetadata{
			SessionID:      payload.SessionID,
			Origin:         capture.OriginClaudeCode,
			Model:          payload.Model,
			PermissionMode: payload.PermissionMode,
			TranscriptPath: payload.TranscriptPath,
		}

		for _, toolCall := range payload.ToolCalls {
			if err := recorder.RecordToolUse(capture.ToolUse{
				SessionMetadata: meta,
				TurnID:          payload.TurnID,
				ToolName:        toolCall.ToolName,
				ToolUseID:       toolCall.ToolUseID,
				ToolInput:       toolCall.ToolInput,
				ToolResponse:    toolCall.ToolResponse,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}
