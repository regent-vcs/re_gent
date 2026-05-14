package test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/cli"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestCodexCLIImportAndInspect(t *testing.T) {
	workspace := t.TempDir()
	codexHome := filepath.Join(t.TempDir(), ".codex")
	rolloutDir := filepath.Join(codexHome, "sessions", "2026", "05", "14")
	if err := os.MkdirAll(rolloutDir, 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}

	if _, err := store.Init(workspace); err != nil {
		t.Fatalf("init store: %v", err)
	}

	quotedWorkspace, err := json.Marshal(filepath.Clean(workspace))
	if err != nil {
		t.Fatalf("marshal workspace: %v", err)
	}

	rollout := "" +
		"{\"timestamp\":\"2026-05-14T01:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"smoke-session\",\"cwd\":" + string(quotedWorkspace) + ",\"originator\":\"Codex Desktop\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:01Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"turn_id\":\"turn-1\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:02Z\",\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"turn-1\",\"cwd\":" + string(quotedWorkspace) + "}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:03Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"done\"}]}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:04Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call\",\"name\":\"apply_patch\",\"call_id\":\"call-1\",\"input\":\"*** Begin Patch\\n*** Add File: note.txt\\n+hello\\n*** End Patch\\n\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:05Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"patch_apply_end\",\"call_id\":\"call-1\",\"turn_id\":\"turn-1\",\"stdout\":\"patched\",\"stderr\":\"\",\"success\":true}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:06Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call_output\",\"call_id\":\"call-1\",\"output\":\"{\\\"output\\\":\\\"ok\\\"}\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:07Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\",\"turn_id\":\"turn-1\"}}\n"
	if err := os.WriteFile(filepath.Join(rolloutDir, "rollout-smoke.jsonl"), []byte(rollout), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	restoreWD := mustChdir(t, workspace)
	defer restoreWD()

	codexCmd := cli.CodexCmd()
	codexCmd.SetArgs([]string{"import", "--project", workspace, "--codex-home", codexHome})
	if err := codexCmd.Execute(); err != nil {
		t.Fatalf("execute codex import: %v", err)
	}

	s, err := store.Open(filepath.Join(workspace, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "codex:smoke-session" {
		t.Fatalf("unexpected session id: %s", sessions[0].ID)
	}

	steps, err := idx.ListSteps("codex:smoke-session", 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	sessionsOutput, err := captureStdout(t, func() error {
		cmd := cli.SessionsCmd()
		cmd.SetArgs([]string{})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("execute sessions: %v", err)
	}
	if !strings.Contains(sessionsOutput, "codex:smoke-session") {
		t.Fatalf("sessions output missing session id: %s", sessionsOutput)
	}
	if !strings.Contains(strings.ToLower(sessionsOutput), "codex") {
		t.Fatalf("sessions output missing codex origin: %s", sessionsOutput)
	}

	logOutput, err := captureStdout(t, func() error {
		cmd := cli.LogCmd()
		cmd.SetArgs([]string{"--files-only", "codex:smoke-session"})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("execute log: %v", err)
	}
	if !strings.Contains(logOutput, "note.txt") {
		t.Fatalf("log output missing touched file: %s", logOutput)
	}
	if !strings.Contains(logOutput, "+1") {
		t.Fatalf("log output missing diff stat: %s", logOutput)
	}

	showOutput, err := captureStdout(t, func() error {
		cmd := cli.ShowCmd()
		cmd.SetArgs([]string{string(steps[0].Hash[:8])})
		return cmd.Execute()
	})
	if err != nil {
		t.Fatalf("execute show: %v", err)
	}
	for _, want := range []string{"apply_patch", "call-1", "note.txt", "\"output\": \"ok\"", "done"} {
		if !strings.Contains(showOutput, want) {
			t.Fatalf("show output missing %q: %s", want, showOutput)
		}
	}
}

func mustChdir(t *testing.T, dir string) func() {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to %s: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd to %s: %v", previous, err)
		}
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}

	os.Stdout = writer
	defer func() {
		os.Stdout = oldStdout
	}()

	runErr := fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close stdout writer: %v", err)
	}
	output, readErr := io.ReadAll(reader)
	if readErr != nil {
		t.Fatalf("read stdout: %v", readErr)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close stdout reader: %v", err)
	}

	return string(output), runErr
}
