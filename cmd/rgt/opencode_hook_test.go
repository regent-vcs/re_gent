package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestRunOpenCodeHook_CapturesTurn(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"SessionStart","session_id":"ses_abc123","cwd":%q}`, root))
	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"UserPromptSubmit","session_id":"ses_abc123","cwd":%q,"model":"claude-sonnet-4-6-20250514"}`, root))
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("world\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"PostToolUse","session_id":"ses_abc123","cwd":%q,"tool_name":"write","tool_use_id":"call_001","tool_input":{"file_path":"hello.txt","content":"world\n"},"tool_response":{"title":"Wrote hello.txt","output":"ok","metadata":{}}}`, root))
	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"Stop","session_id":"ses_abc123","cwd":%q}`, root))

	s, err := store.Open(filepath.Join(root, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessionID := "opencode:" + url.PathEscape("ses_abc123")
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
	if steps[0].Origin != "opencode" || steps[0].ToolName != "write" {
		t.Fatalf("unexpected step metadata: %#v", steps[0])
	}

	messages, err := idx.GetMessagesForStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 linked messages (user, tool_call, tool_result, assistant), got %d", len(messages))
	}
}

func TestRunOpenCodeHook_NoTurnID(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"PostToolUse","session_id":"ses_xyz","cwd":%q,"tool_name":"bash","tool_use_id":"call_002","tool_input":{"command":"ls"},"tool_response":{"output":"file.txt"}}`, root))
	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"Stop","session_id":"ses_xyz","cwd":%q}`, root))

	s, err := store.Open(filepath.Join(root, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessionID := "opencode:" + url.PathEscape("ses_xyz")
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step (allTurns mode), got %d", len(steps))
	}
}

func TestRunOpenCodeHook_UnsupportedEventLogs(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"unknown_event","session_id":"ses_abc","cwd":%q}`, root))

	data, err := os.ReadFile(filepath.Join(root, ".regent", "log", "hook-error.log"))
	if err != nil {
		t.Fatalf("expected hook error log: %v", err)
	}
	if !strings.Contains(string(data), "unsupported OpenCode hook event") {
		t.Fatalf("unexpected hook error log: %s", data)
	}
}

func TestRunOpenCodeHook_NoStoreIsNoop(t *testing.T) {
	root := t.TempDir()

	runOpenCodePayload(t, fmt.Sprintf(`{"hook_event_name":"SessionStart","session_id":"ses_abc","cwd":%q}`, root))

	if _, err := os.Stat(filepath.Join(root, ".regent")); !os.IsNotExist(err) {
		t.Fatalf("hook should not create .regent, stat err=%v", err)
	}
}

func TestRunOpenCodeHook_InvalidPayloadNoStoreIsNoop(t *testing.T) {
	root := t.TempDir()
	chdir(t, root)

	runOpenCodePayload(t, `{not-json`)

	if _, err := os.Stat(filepath.Join(root, ".regent")); !os.IsNotExist(err) {
		t.Fatalf("invalid hook payload should not create .regent, stat err=%v", err)
	}
}

func TestRunOpenCodeHook_InvalidPayloadLogsWhenStoreExists(t *testing.T) {
	root := t.TempDir()
	chdir(t, root)
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runOpenCodePayload(t, `{not-json`)

	data, err := os.ReadFile(filepath.Join(root, ".regent", "log", "hook-error.log"))
	if err != nil {
		t.Fatalf("expected hook error log: %v", err)
	}
	if !strings.Contains(string(data), "parse payload") {
		t.Fatalf("unexpected hook error log: %s", data)
	}
}

func runOpenCodePayload(t *testing.T, payload string) {
	t.Helper()
	runWithStdin(t, payload, func() error {
		return runOpenCodeHook(nil, nil)
	})
}
