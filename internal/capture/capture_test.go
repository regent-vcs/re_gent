package capture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/store"
)

func TestRecorder_CodexTurnCreatesOneStep(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	recorder, ok, err := Open(root)
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	if !ok {
		t.Fatal("expected initialized recorder")
	}
	defer func() { _ = recorder.Close() }()

	meta := SessionMetadata{
		SessionID:      "codex-session",
		Origin:         OriginCodexCLI,
		Model:          "gpt-5.5",
		PermissionMode: "bypassPermissions",
	}
	sessionID := canonicalSessionID(OriginCodexCLI, meta.SessionID)

	if err := recorder.UpsertSession(meta); err != nil {
		t.Fatalf("upsert session: %v", err)
	}
	if err := recorder.RecordUserPrompt(UserPrompt{
		SessionMetadata: meta,
		TurnID:          "turn-1",
		Prompt:          "write hello.txt",
	}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		TurnID:          "turn-1",
		ToolName:        "Bash",
		ToolUseID:       "call_1",
		ToolInput:       json.RawMessage(`{"command":"printf hello > hello.txt"}`),
		ToolResponse:    json.RawMessage(`"ok"`),
	}); err != nil {
		t.Fatalf("record tool: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{
		SessionMetadata:      meta,
		TurnID:               "turn-1",
		LastAssistantMessage: "done",
	}); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	steps, err := recorder.Index.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Origin != OriginCodexCLI {
		t.Fatalf("origin = %q, want %q", steps[0].Origin, OriginCodexCLI)
	}
	if steps[0].TurnID != "turn-1" {
		t.Fatalf("turn id = %q, want turn-1", steps[0].TurnID)
	}

	step, err := recorder.Store.ReadStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("read step: %v", err)
	}
	if len(step.Causes) != 1 {
		t.Fatalf("expected 1 cause, got %d", len(step.Causes))
	}
	if step.Causes[0].ToolName != "Bash" {
		t.Fatalf("tool name = %q, want Bash", step.Causes[0].ToolName)
	}

	messages, err := recorder.Index.GetMessagesForStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected 4 linked messages, got %d", len(messages))
	}
}

func TestRecorder_NoToolTurnIsProcessedWithoutStep(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	recorder, ok, err := Open(root)
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	if !ok {
		t.Fatal("expected initialized recorder")
	}
	defer func() { _ = recorder.Close() }()

	meta := SessionMetadata{SessionID: "codex-session", Origin: OriginCodexCLI}
	sessionID := canonicalSessionID(OriginCodexCLI, meta.SessionID)
	if err := recorder.RecordUserPrompt(UserPrompt{
		SessionMetadata: meta,
		TurnID:          "turn-2",
		Prompt:          "say ok",
	}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{
		SessionMetadata:      meta,
		TurnID:               "turn-2",
		LastAssistantMessage: "ok",
	}); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	steps, err := recorder.Index.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 0 {
		t.Fatalf("expected no steps, got %d", len(steps))
	}

	pending, err := recorder.Index.GetPendingMessages(sessionID, "turn-2")
	if err != nil {
		t.Fatalf("pending messages: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected no pending messages, got %d", len(pending))
	}
}

func TestRecorder_TurnIsolationAndMultiToolCauses(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	recorder, ok, err := Open(root)
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	if !ok {
		t.Fatal("expected initialized recorder")
	}
	defer func() { _ = recorder.Close() }()

	meta := SessionMetadata{SessionID: "codex-session", Origin: OriginCodexCLI}
	sessionID := canonicalSessionID(OriginCodexCLI, meta.SessionID)

	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, TurnID: "turn-no-tool", Prompt: "say ok"}); err != nil {
		t.Fatalf("record no-tool prompt: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{SessionMetadata: meta, TurnID: "turn-no-tool", LastAssistantMessage: "ok"}); err != nil {
		t.Fatalf("finalize no-tool turn: %v", err)
	}

	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, TurnID: "turn-tools", Prompt: "write files"}); err != nil {
		t.Fatalf("record tool prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "one.txt"), []byte("one\n"), 0o644); err != nil {
		t.Fatalf("write one: %v", err)
	}
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		TurnID:          "turn-tools",
		ToolName:        "Write",
		ToolUseID:       "tool-1",
		ToolInput:       json.RawMessage(`{"file_path":"one.txt","content":"one\n"}`),
		ToolResponse:    json.RawMessage(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("record first tool: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "two.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatalf("write two: %v", err)
	}
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		TurnID:          "turn-tools",
		ToolName:        "Write",
		ToolUseID:       "tool-2",
		ToolInput:       json.RawMessage(`{"file_path":"two.txt","content":"two\n"}`),
		ToolResponse:    json.RawMessage(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("record second tool: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{SessionMetadata: meta, TurnID: "turn-tools", LastAssistantMessage: "done"}); err != nil {
		t.Fatalf("finalize tool turn: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{SessionMetadata: meta, TurnID: "turn-tools", LastAssistantMessage: "done again"}); err != nil {
		t.Fatalf("retry finalize tool turn: %v", err)
	}

	steps, err := recorder.Index.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected only the tool turn to create one step, got %d", len(steps))
	}

	step, err := recorder.Store.ReadStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("read step: %v", err)
	}
	if len(step.Causes) != 2 {
		t.Fatalf("expected two causes, got %d", len(step.Causes))
	}
	if step.Causes[0].ToolUseID != "tool-1" || step.Causes[1].ToolUseID != "tool-2" {
		t.Fatalf("causes out of order: %#v", step.Causes)
	}

	linked, err := recorder.Index.GetMessagesForStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("linked messages: %v", err)
	}
	if len(linked) != 6 {
		t.Fatalf("expected 6 linked messages after retry, got %d", len(linked))
	}

	pendingNoTool, err := recorder.Index.GetPendingMessages(sessionID, "turn-no-tool")
	if err != nil {
		t.Fatalf("pending no-tool messages: %v", err)
	}
	if len(pendingNoTool) != 0 {
		t.Fatalf("expected no pending no-tool messages, got %d", len(pendingNoTool))
	}
}

func TestRecorder_MissingCodexTurnIDRejected(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	recorder, ok, err := Open(root)
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	if !ok {
		t.Fatal("expected initialized recorder")
	}
	defer func() { _ = recorder.Close() }()

	err = recorder.RecordUserPrompt(UserPrompt{
		SessionMetadata: SessionMetadata{SessionID: "codex-session", Origin: OriginCodexCLI},
		Prompt:          "missing turn",
	})
	if err == nil {
		t.Fatal("expected missing turn id to be rejected")
	}
}

func TestCanonicalSessionID_IdempotentAndEscapesPathSeparators(t *testing.T) {
	sessionID := canonicalSessionID(OriginCodexCLI, "session/with/slash")
	if sessionID != "codex_cli:session%2Fwith%2Fslash" {
		t.Fatalf("canonical session id = %q", sessionID)
	}
	if again := canonicalSessionID(OriginCodexCLI, sessionID); again != sessionID {
		t.Fatalf("canonicalization should be idempotent: %q", again)
	}
}

func TestRecordToolUse_SelfCommandsDoNotCreateToolCauses(t *testing.T) {
	root := t.TempDir()
	if _, err := store.Init(root); err != nil {
		t.Fatalf("init store: %v", err)
	}

	recorder, ok, err := Open(root)
	if err != nil {
		t.Fatalf("open recorder: %v", err)
	}
	if !ok {
		t.Fatal("expected initialized recorder")
	}
	defer func() { _ = recorder.Close() }()

	meta := SessionMetadata{SessionID: "codex-session", Origin: OriginCodexCLI}
	sessionID := canonicalSessionID(OriginCodexCLI, meta.SessionID)
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		TurnID:          "turn-3",
		ToolName:        "Bash",
		ToolUseID:       "call_rgt",
		ToolInput:       json.RawMessage(`{"command":"rgt log"}`),
		ToolResponse:    json.RawMessage(`"ok"`),
	}); err != nil {
		t.Fatalf("record tool: %v", err)
	}

	pending, err := recorder.Index.GetPendingMessages(sessionID, "turn-3")
	if err != nil {
		t.Fatalf("pending messages: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected self-noise to be skipped, got %d messages", len(pending))
	}
}

func TestComputeAndWriteBlame_ReturnsParentReadError(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	blobHash, err := s.WriteBlob([]byte("hello\n"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	treeHash, err := s.WriteTree(&store.Tree{Entries: []store.TreeEntry{{Path: "hello.txt", Blob: blobHash}}})
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}

	err = computeAndWriteBlame(s, store.Hash(strings.Repeat("a", 64)), store.Hash(strings.Repeat("b", 64)), treeHash)
	if err == nil {
		t.Fatal("expected missing parent step to be reported")
	}
	if !strings.Contains(err.Error(), "read parent step") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestComputeAndWriteBlame_DoesNotInventUnchangedBlame(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	blobHash, err := s.WriteBlob([]byte("hello\n"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	treeHash, err := s.WriteTree(&store.Tree{Entries: []store.TreeEntry{{Path: "hello.txt", Blob: blobHash}}})
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	parentHash, err := s.WriteStep(&store.Step{
		Tree:           treeHash,
		SessionID:      "claude_code:session",
		TimestampNanos: 1,
	})
	if err != nil {
		t.Fatalf("write parent step: %v", err)
	}
	currentHash := store.Hash(strings.Repeat("c", 64))

	err = computeAndWriteBlame(s, parentHash, currentHash, treeHash)
	if err == nil {
		t.Fatal("expected missing unchanged parent blame to be reported")
	}
	if !strings.Contains(err.Error(), "read parent blame for unchanged hello.txt") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := s.ReadBlameForFile(currentHash, "hello.txt"); err == nil {
		t.Fatal("unchanged file without parent blame should not get invented current-step blame")
	}
}
