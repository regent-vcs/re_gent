package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/regent-vcs/regent/internal/capture"
	"github.com/spf13/cobra"
)

func CodexHookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "codex-hook",
		Short:  "Internal: Codex CLI hook",
		RunE:   runCodexHook,
		Hidden: true,
	}
}

type codexHookPayload struct {
	SessionID            string          `json:"session_id"`
	TurnID               string          `json:"turn_id"`
	TranscriptPath       string          `json:"transcript_path"`
	CWD                  string          `json:"cwd"`
	HookEventName        string          `json:"hook_event_name"`
	Model                string          `json:"model"`
	PermissionMode       string          `json:"permission_mode"`
	Prompt               string          `json:"prompt"`
	LastAssistantMessage string          `json:"last_assistant_message"`
	ToolName             string          `json:"tool_name"`
	ToolInput            json.RawMessage `json:"tool_input"`
	ToolUseID            string          `json:"tool_use_id"`
	ToolResponse         json.RawMessage `json:"tool_response"`
}

func runCodexHook(cmd *cobra.Command, args []string) error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	var payload codexHookPayload
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
			Origin:         capture.OriginCodexCLI,
			Model:          payload.Model,
			PermissionMode: payload.PermissionMode,
			TranscriptPath: payload.TranscriptPath,
		}

		switch normalizeHookEventName(payload.HookEventName) {
		case "sessionstart":
			return recorder.UpsertSession(meta)
		case "userpromptsubmit":
			return recorder.RecordUserPrompt(capture.UserPrompt{
				SessionMetadata: meta,
				TurnID:          payload.TurnID,
				Prompt:          payload.Prompt,
			})
		case "posttooluse":
			return recorder.RecordToolUse(capture.ToolUse{
				SessionMetadata: meta,
				TurnID:          payload.TurnID,
				ToolName:        payload.ToolName,
				ToolUseID:       payload.ToolUseID,
				ToolInput:       payload.ToolInput,
				ToolResponse:    payload.ToolResponse,
			})
		case "stop":
			return recorder.RecordAssistantAndFinalize(capture.AssistantResponse{
				SessionMetadata:      meta,
				TurnID:               payload.TurnID,
				LastAssistantMessage: payload.LastAssistantMessage,
			})
		default:
			return fmt.Errorf("unsupported Codex hook event: %s", payload.HookEventName)
		}
	})
}

func normalizeHookEventName(name string) string {
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, "-", "")
	return strings.ToLower(name)
}
