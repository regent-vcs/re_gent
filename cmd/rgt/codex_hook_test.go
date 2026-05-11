package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestRunCodexHook_CapturesTurn(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runCodexPayload(t, fmt.Sprintf(`{"hook_event_name":"session_start","session_id":"codex-session","cwd":%q,"model":"gpt-5.5"}`, root))
	runCodexPayload(t, fmt.Sprintf(`{"hook_event_name":"user-prompt-submit","session_id":"codex-session","turn_id":"turn-1","cwd":%q,"prompt":"write file"}`, root))
	if err := os.WriteFile(filepath.Join(root, "codex.txt"), []byte("ok\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runCodexPayload(t, fmt.Sprintf(`{"hook_event_name":"PostToolUse","session_id":"codex-session","turn_id":"turn-1","cwd":%q,"tool_name":"Write","tool_use_id":"tool-1","tool_input":{"file_path":"codex.txt","content":"ok\n"},"tool_response":{"ok":true}}`, root))
	runCodexPayload(t, fmt.Sprintf(`{"hook_event_name":"stop","session_id":"codex-session","turn_id":"turn-1","cwd":%q,"last_assistant_message":"done"}`, root))

	s, err := store.Open(filepath.Join(root, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessionID := "codex_cli:" + url.PathEscape("codex-session")
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
	if steps[0].Origin != "codex_cli" || steps[0].TurnID != "turn-1" || steps[0].ToolName != "Write" {
		t.Fatalf("unexpected step metadata: %#v", steps[0])
	}

	messages, err := idx.GetMessagesForStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 linked messages, got %d", len(messages))
	}
}

func TestRunCodexHook_UnsupportedEventLogsAndDoesNotFail(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runCodexPayload(t, fmt.Sprintf(`{"hook_event_name":"unknown_event","session_id":"codex-session","cwd":%q}`, root))

	data, err := os.ReadFile(filepath.Join(root, ".regent", "log", "hook-error.log"))
	if err != nil {
		t.Fatalf("expected hook error log: %v", err)
	}
	if !strings.Contains(string(data), "unsupported Codex hook event") {
		t.Fatalf("unexpected hook error log: %s", data)
	}
}

func TestRunCodexHook_NoStoreIsNoop(t *testing.T) {
	root := t.TempDir()

	runCodexPayload(t, fmt.Sprintf(`{"hook_event_name":"session_start","session_id":"codex-session","cwd":%q}`, root))

	if _, err := os.Stat(filepath.Join(root, ".regent")); !os.IsNotExist(err) {
		t.Fatalf("hook should not create .regent, stat err=%v", err)
	}
}

func TestRunCodexHook_InvalidPayloadNoStoreIsNoop(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	runCodexPayload(t, `{not-json`)

	if _, err := os.Stat(filepath.Join(root, ".regent")); !os.IsNotExist(err) {
		t.Fatalf("invalid hook payload should not create .regent, stat err=%v", err)
	}
}

func TestRunCodexHook_InvalidPayloadLogsWhenStoreExists(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runCodexPayload(t, `{not-json`)

	data, err := os.ReadFile(filepath.Join(root, ".regent", "log", "hook-error.log"))
	if err != nil {
		t.Fatalf("expected hook error log: %v", err)
	}
	if !strings.Contains(string(data), "parse payload") {
		t.Fatalf("unexpected hook error log: %s", data)
	}
}

func TestRunToolBatchHook_MarshalsStringResponseAsJSON(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runToolBatchPayload(t, fmt.Sprintf(`{"session_id":"claude-session","cwd":%q,"tool_calls":[{"tool_name":"Bash","tool_use_id":"tool-1","tool_input":{"command":"printf ok"},"tool_response":"ok"}]}`, root))

	s, err := store.Open(filepath.Join(root, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessionID := "claude_code:" + url.PathEscape("claude-session")
	messages, err := idx.GetAllPendingMessages(sessionID)
	if err != nil {
		t.Fatalf("pending messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected call/result messages, got %d", len(messages))
	}

	resultData, err := s.ReadBlob(store.Hash(messages[1].ToolOutput))
	if err != nil {
		t.Fatalf("read tool output: %v", err)
	}
	if string(resultData) != `"ok"` {
		t.Fatalf("tool response blob = %s, want JSON string", resultData)
	}
}

func TestRunToolBatchHook_PreservesStructuredResponse(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	runToolBatchPayload(t, fmt.Sprintf(`{"session_id":"claude-session","cwd":%q,"tool_calls":[{"tool_name":"Read","tool_use_id":"tool-1","tool_input":{"file_path":"notes.txt"},"tool_response":[{"type":"text","text":"ok"}]}]}`, root))

	s, err := store.Open(filepath.Join(root, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessionID := "claude_code:" + url.PathEscape("claude-session")
	messages, err := idx.GetAllPendingMessages(sessionID)
	if err != nil {
		t.Fatalf("pending messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected call/result messages, got %d", len(messages))
	}

	resultData, err := s.ReadBlob(store.Hash(messages[1].ToolOutput))
	if err != nil {
		t.Fatalf("read tool output: %v", err)
	}
	if string(resultData) != `[{"type":"text","text":"ok"}]` {
		t.Fatalf("tool response blob = %s", resultData)
	}
}

func runCodexPayload(t *testing.T, payload string) {
	t.Helper()
	runWithStdin(t, payload, func() error {
		return runCodexHook(nil, nil)
	})
}

func runToolBatchPayload(t *testing.T, payload string) {
	t.Helper()
	runWithStdin(t, payload, func() error {
		return runToolBatchHook(nil, nil)
	})
}

func runWithStdin(t *testing.T, payload string, fn func() error) {
	t.Helper()

	oldStdin := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := io.WriteString(writer, payload); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	os.Stdin = reader
	defer func() {
		os.Stdin = oldStdin
		_ = reader.Close()
	}()

	if err := fn(); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
}
