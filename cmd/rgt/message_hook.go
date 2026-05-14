package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/capture"
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

type claudeMessagePayload struct {
	Prompt               string `json:"prompt"`
	LastAssistantMessage string `json:"last_assistant_message"`
	SessionID            string `json:"session_id"`
	TurnID               string `json:"turn_id"`
	TranscriptPath       string `json:"transcript_path"`
	CWD                  string `json:"cwd"`
	Model                string `json:"model"`
	PermissionMode       string `json:"permission_mode"`
}

func runMessageHook(cmd *cobra.Command, args []string) error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read payload: %w", err)
	}

	var payload claudeMessagePayload
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

		switch args[0] {
		case "user":
			return recorder.RecordUserPrompt(capture.UserPrompt{
				SessionMetadata: meta,
				TurnID:          payload.TurnID,
				Prompt:          payload.Prompt,
			})
		case "assistant":
			return recorder.RecordAssistantAndFinalize(capture.AssistantResponse{
				SessionMetadata:      meta,
				TurnID:               payload.TurnID,
				LastAssistantMessage: payload.LastAssistantMessage,
			})
		default:
			return fmt.Errorf("unknown hook type: %s", args[0])
		}
	})
}

func hookCWD(cwd string) (string, error) {
	if cwd != "" {
		return cwd, nil
	}

	workingDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return workingDir, nil
}

func withHookRecorder(cwd string, fn func(*capture.Recorder) error) error {
	recorder, ok, err := capture.Open(cwd)
	if err != nil {
		logHookCommandError(cwd, err)
		return nil
	}
	if !ok {
		return nil
	}
	defer func() { _ = recorder.Close() }()

	if err := fn(recorder); err != nil {
		logHookCommandError(cwd, err)
	}
	return nil
}

func logHookPayloadError(raw []byte, err error) error {
	cwd, cwdErr := hookCWD(cwdFromPayload(raw))
	if cwdErr != nil {
		return nil
	}
	logHookCommandError(cwd, err)
	return nil
}

func cwdFromPayload(raw []byte) string {
	var payload struct {
		CWD string `json:"cwd"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return ""
	}
	return payload.CWD
}

func logHookCommandError(cwd string, err error) {
	if err == nil || cwd == "" {
		return
	}

	regentDir := filepath.Join(cwd, ".regent")
	if _, statErr := os.Stat(regentDir); statErr != nil {
		return
	}

	logPath := filepath.Join(regentDir, "log", "hook-error.log")
	if mkErr := os.MkdirAll(filepath.Dir(logPath), 0o755); mkErr != nil {
		return
	}
	f, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if openErr != nil {
		return
	}
	defer func() { _ = f.Close() }()

	fmt.Fprintf(f, "[%s] %v\n", time.Now().Format(time.RFC3339), err)
}
