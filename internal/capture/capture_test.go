package capture

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/index"
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

func TestRecorder_DuplicateToolUseIsIdempotent(t *testing.T) {
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
	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, TurnID: "turn-dup", Prompt: "write file"}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "dup.txt"), []byte("dup\n"), 0o644); err != nil {
		t.Fatalf("write dup: %v", err)
	}
	tool := ToolUse{
		SessionMetadata: meta,
		TurnID:          "turn-dup",
		ToolName:        "Write",
		ToolUseID:       "tool-dup",
		ToolInput:       json.RawMessage(`{"file_path":"dup.txt","content":"dup\n"}`),
		ToolResponse:    json.RawMessage(`{"ok":true}`),
	}
	if err := recorder.RecordToolUse(tool); err != nil {
		t.Fatalf("record tool: %v", err)
	}
	if err := recorder.RecordToolUse(tool); err != nil {
		t.Fatalf("record duplicate tool: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{SessionMetadata: meta, TurnID: "turn-dup", LastAssistantMessage: "done"}); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	steps, err := recorder.Index.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
	step, err := recorder.Store.ReadStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("read step: %v", err)
	}
	if len(step.Causes) != 1 {
		t.Fatalf("expected duplicate tool delivery to produce one cause, got %d", len(step.Causes))
	}
	messages, err := recorder.Index.GetMessagesForStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("linked messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected user/call/result/assistant messages, got %d", len(messages))
	}
}

func TestRecorder_RecoversHeadStepMissingIndexAndLinks(t *testing.T) {
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
	turnID := "turn-recover"
	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, TurnID: turnID, Prompt: "write file"}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "recover.txt"), []byte("recover\n"), 0o644); err != nil {
		t.Fatalf("write recover: %v", err)
	}
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		TurnID:          turnID,
		ToolName:        "Write",
		ToolUseID:       "tool-recover",
		ToolInput:       json.RawMessage(`{"file_path":"recover.txt","content":"recover\n"}`),
		ToolResponse:    json.RawMessage(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("record tool: %v", err)
	}
	if err := recorder.Index.AppendMessage(index.Message{
		ID:          "assistant-recover",
		SessionID:   sessionID,
		TurnID:      turnID,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "assistant",
		ContentText: "done",
	}); err != nil {
		t.Fatalf("append assistant: %v", err)
	}

	treeHash, err := snapshotWorkspace(recorder.Store, root)
	if err != nil {
		t.Fatalf("snapshot workspace: %v", err)
	}
	step := &store.Step{
		Tree:           treeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-recover"},
		Causes:         []store.Cause{{ToolName: "Write", ToolUseID: "tool-recover"}},
		SessionID:      sessionID,
		Origin:         OriginCodexCLI,
		TurnID:         turnID,
		TimestampNanos: time.Now().UnixNano(),
	}
	stepHash, err := recorder.Store.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}
	if err := recorder.Store.UpdateSessionRef(sessionID, "", stepHash); err != nil {
		t.Fatalf("write ref: %v", err)
	}

	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{SessionMetadata: meta, TurnID: turnID, LastAssistantMessage: "done retry"}); err != nil {
		t.Fatalf("recover finalize: %v", err)
	}

	steps, err := recorder.Index.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 || steps[0].Hash != stepHash {
		t.Fatalf("expected recovered step %s, got %#v", stepHash, steps)
	}
	messages, err := recorder.Index.GetMessagesForStep(stepHash)
	if err != nil {
		t.Fatalf("linked messages: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("expected existing pending messages linked to recovered step, got %d", len(messages))
	}
}

func TestRecorder_RetriesRefConflictAgainstLatestHead(t *testing.T) {
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
	turnID := "turn-retry"
	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, TurnID: turnID, Prompt: "write file"}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "retry.txt"), []byte("retry\n"), 0o644); err != nil {
		t.Fatalf("write retry: %v", err)
	}
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		TurnID:          turnID,
		ToolName:        "Write",
		ToolUseID:       "tool-retry",
		ToolInput:       json.RawMessage(`{"file_path":"retry.txt","content":"retry\n"}`),
		ToolResponse:    json.RawMessage(`{"ok":true}`),
	}); err != nil {
		t.Fatalf("record tool: %v", err)
	}

	refPath := filepath.Join(root, ".regent", "refs", filepath.FromSlash(store.SessionRefPath(sessionID)))
	lockPath := refPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
		t.Fatalf("mkdir refs: %v", err)
	}
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("create lock: %v", err)
	}
	defer func() {
		_ = lockFile.Close()
		_ = os.Remove(lockPath)
	}()

	done := make(chan error, 1)
	go func() {
		done <- recorder.RecordAssistantAndFinalize(AssistantResponse{
			SessionMetadata:      meta,
			TurnID:               turnID,
			LastAssistantMessage: "done",
		})
	}()

	time.Sleep(20 * time.Millisecond)
	treeHash, err := snapshotWorkspace(recorder.Store, root)
	if err != nil {
		t.Fatalf("snapshot workspace: %v", err)
	}
	competingStep := &store.Step{
		Tree:           treeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-competing"},
		Causes:         []store.Cause{{ToolName: "Write", ToolUseID: "tool-competing"}},
		SessionID:      sessionID,
		Origin:         OriginCodexCLI,
		TurnID:         "turn-competing",
		TimestampNanos: time.Now().UnixNano(),
	}
	competingHash, err := recorder.Store.WriteStep(competingStep)
	if err != nil {
		t.Fatalf("write competing step: %v", err)
	}
	if err := os.WriteFile(refPath, []byte(string(competingHash)+"\n"), 0o644); err != nil {
		t.Fatalf("write competing ref: %v", err)
	}
	if err := lockFile.Close(); err != nil {
		t.Fatalf("close lock: %v", err)
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatalf("remove lock: %v", err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("finalize after conflict: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for finalize")
	}

	headHash, err := recorder.Store.ReadSessionRef(sessionID)
	if err != nil {
		t.Fatalf("read head ref: %v", err)
	}
	headStep, err := recorder.Store.ReadStep(headHash)
	if err != nil {
		t.Fatalf("read head step: %v", err)
	}
	if headStep.TurnID != turnID {
		t.Fatalf("head turn = %q, want %q", headStep.TurnID, turnID)
	}
	if headStep.Parent != competingHash {
		t.Fatalf("retried step parent = %s, want latest head %s", headStep.Parent, competingHash)
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

func TestCanonicalSessionID_EscapesRawIDWithoutPrefixCollision(t *testing.T) {
	sessionID := canonicalSessionID(OriginCodexCLI, "session/with/slash")
	if sessionID != "codex_cli:session%2Fwith%2Fslash" {
		t.Fatalf("canonical session id = %q", sessionID)
	}
	prefixedRawID := canonicalSessionID(OriginCodexCLI, "codex_cli:abc")
	if prefixedRawID != "codex_cli:codex_cli%3Aabc" {
		t.Fatalf("prefixed raw id should not collide with canonical id: %q", prefixedRawID)
	}
}

func TestRecorder_AdoptsLegacyRawSessionID(t *testing.T) {
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

	legacySessionID := "claude-legacy"
	legacyBlob, err := recorder.Store.WriteBlob([]byte("legacy\n"))
	if err != nil {
		t.Fatalf("write legacy blob: %v", err)
	}
	legacyTree := &store.Tree{Entries: []store.TreeEntry{{Path: "legacy.txt", Blob: legacyBlob}}}
	legacyTreeHash, err := recorder.Store.WriteTree(legacyTree)
	if err != nil {
		t.Fatalf("write legacy tree: %v", err)
	}
	legacyStep := &store.Step{
		Tree:           legacyTreeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "legacy-tool"},
		SessionID:      legacySessionID,
		TimestampNanos: 1,
	}
	legacyStepHash, err := recorder.Store.WriteStep(legacyStep)
	if err != nil {
		t.Fatalf("write legacy step: %v", err)
	}
	if err := recorder.Index.IndexStep(legacyStepHash, legacyStep, legacyTree); err != nil {
		t.Fatalf("index legacy step: %v", err)
	}
	if err := recorder.Store.UpdateSessionRef(legacySessionID, "", legacyStepHash); err != nil {
		t.Fatalf("write legacy ref: %v", err)
	}

	meta := SessionMetadata{SessionID: legacySessionID, Origin: OriginClaudeCode}
	canonicalID := canonicalSessionID(OriginClaudeCode, legacySessionID)
	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, Prompt: "continue"}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "next.txt"), []byte("next\n"), 0o644); err != nil {
		t.Fatalf("write next file: %v", err)
	}
	if err := recorder.RecordToolUse(ToolUse{
		SessionMetadata: meta,
		ToolName:        "Write",
		ToolUseID:       "next-tool",
		ToolInput:       json.RawMessage(`{"file_path":"next.txt","content":"next\n"}`),
		ToolResponse:    json.RawMessage(`"ok"`),
	}); err != nil {
		t.Fatalf("record tool: %v", err)
	}
	if err := recorder.RecordAssistantAndFinalize(AssistantResponse{SessionMetadata: meta, LastAssistantMessage: "done"}); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	steps, err := recorder.Index.ListSteps(canonicalID, 10)
	if err != nil {
		t.Fatalf("list canonical steps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected legacy and new steps under canonical id, got %d", len(steps))
	}
	if steps[0].ParentHash != legacyStepHash {
		t.Fatalf("new step parent = %s, want legacy step %s", steps[0].ParentHash, legacyStepHash)
	}
	if _, err := recorder.Store.ReadSessionRef(canonicalID); err != nil {
		t.Fatalf("canonical ref was not adopted: %v", err)
	}
	if _, err := recorder.Store.ReadSessionRef(legacySessionID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("legacy raw ref should be removed after adoption, err=%v", err)
	}
}

func TestRecorder_DivergentLegacyRawSessionIDIsNotMerged(t *testing.T) {
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

	legacySessionID := "claude-legacy"
	canonicalID := canonicalSessionID(OriginClaudeCode, legacySessionID)

	legacyBlob, err := recorder.Store.WriteBlob([]byte("legacy\n"))
	if err != nil {
		t.Fatalf("write legacy blob: %v", err)
	}
	legacyTree := &store.Tree{Entries: []store.TreeEntry{{Path: "legacy.txt", Blob: legacyBlob}}}
	legacyTreeHash, err := recorder.Store.WriteTree(legacyTree)
	if err != nil {
		t.Fatalf("write legacy tree: %v", err)
	}
	legacyStep := &store.Step{
		Tree:           legacyTreeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "legacy-tool"},
		SessionID:      legacySessionID,
		TimestampNanos: 1,
	}
	legacyStepHash, err := recorder.Store.WriteStep(legacyStep)
	if err != nil {
		t.Fatalf("write legacy step: %v", err)
	}
	if err := recorder.Index.IndexStep(legacyStepHash, legacyStep, legacyTree); err != nil {
		t.Fatalf("index legacy step: %v", err)
	}
	if err := recorder.Store.UpdateSessionRef(legacySessionID, "", legacyStepHash); err != nil {
		t.Fatalf("write legacy ref: %v", err)
	}

	canonicalBlob, err := recorder.Store.WriteBlob([]byte("canonical\n"))
	if err != nil {
		t.Fatalf("write canonical blob: %v", err)
	}
	canonicalTree := &store.Tree{Entries: []store.TreeEntry{{Path: "canonical.txt", Blob: canonicalBlob}}}
	canonicalTreeHash, err := recorder.Store.WriteTree(canonicalTree)
	if err != nil {
		t.Fatalf("write canonical tree: %v", err)
	}
	canonicalStep := &store.Step{
		Tree:           canonicalTreeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "canonical-tool"},
		SessionID:      canonicalID,
		Origin:         OriginClaudeCode,
		TimestampNanos: 2,
	}
	canonicalStepHash, err := recorder.Store.WriteStep(canonicalStep)
	if err != nil {
		t.Fatalf("write canonical step: %v", err)
	}
	if err := recorder.Index.IndexStep(canonicalStepHash, canonicalStep, canonicalTree); err != nil {
		t.Fatalf("index canonical step: %v", err)
	}
	if err := recorder.Store.UpdateSessionRef(canonicalID, "", canonicalStepHash); err != nil {
		t.Fatalf("write canonical ref: %v", err)
	}

	meta := SessionMetadata{SessionID: legacySessionID, Origin: OriginClaudeCode}
	if err := recorder.RecordUserPrompt(UserPrompt{SessionMetadata: meta, Prompt: "continue"}); err != nil {
		t.Fatalf("record prompt: %v", err)
	}

	canonicalSteps, err := recorder.Index.ListSteps(canonicalID, 10)
	if err != nil {
		t.Fatalf("list canonical steps: %v", err)
	}
	if len(canonicalSteps) != 1 || canonicalSteps[0].Hash != canonicalStepHash {
		t.Fatalf("legacy rows merged into divergent canonical session: %#v", canonicalSteps)
	}
	legacySteps, err := recorder.Index.ListSteps(legacySessionID, 10)
	if err != nil {
		t.Fatalf("list legacy steps: %v", err)
	}
	if len(legacySteps) != 1 || legacySteps[0].Hash != legacyStepHash {
		t.Fatalf("legacy rows not preserved separately: %#v", legacySteps)
	}
	if _, err := recorder.Store.ReadRef("legacy-sessions/" + legacySessionID); err != nil {
		t.Fatalf("legacy ref was not archived: %v", err)
	}
	if _, err := recorder.Store.ReadSessionRef(legacySessionID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("legacy raw ref should be removed after archive, err=%v", err)
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

func TestExistingStepForTurnWalksSessionAncestry(t *testing.T) {
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

	sessionID := canonicalSessionID(OriginCodexCLI, "codex-session")
	blob, err := recorder.Store.WriteBlob([]byte("content\n"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "file.txt", Blob: blob}}}
	treeHash, err := recorder.Store.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	oldStep := &store.Step{
		Tree:           treeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-1"},
		SessionID:      sessionID,
		Origin:         OriginCodexCLI,
		TurnID:         "turn-1",
		TimestampNanos: 1,
	}
	oldHash, err := recorder.Store.WriteStep(oldStep)
	if err != nil {
		t.Fatalf("write old step: %v", err)
	}
	newStep := &store.Step{
		Parent:         oldHash,
		Tree:           treeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-2"},
		SessionID:      sessionID,
		Origin:         OriginCodexCLI,
		TurnID:         "turn-2",
		TimestampNanos: 2,
	}
	newHash, err := recorder.Store.WriteStep(newStep)
	if err != nil {
		t.Fatalf("write new step: %v", err)
	}
	if err := recorder.Store.UpdateSessionRef(sessionID, "", newHash); err != nil {
		t.Fatalf("write session ref: %v", err)
	}

	stepHash, ok, err := recorder.existingStepForTurn(sessionID, "turn-1")
	if err != nil {
		t.Fatalf("find existing step: %v", err)
	}
	if !ok || stepHash != oldHash {
		t.Fatalf("existing step = %s ok=%v, want %s", stepHash, ok, oldHash)
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
