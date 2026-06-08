package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// ---- pure helper tests ----

func TestPrintRawJSON_ValidJSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRawJSON([]byte(`{"key": "value"}`))
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, `"key"`) || !strings.Contains(out, `"value"`) {
		t.Errorf("printRawJSON() output missing expected keys: %q", out)
	}
	// Verify it's pretty-printed (has indentation)
	if !strings.Contains(out, "\n") {
		t.Errorf("expected multi-line pretty-printed JSON: %q", out)
	}
}

func TestPrintRawJSON_NonJSON(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRawJSON([]byte("plain text"))
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if out != "plain text\n" {
		t.Errorf("printRawJSON() = %q, want plain text output", out)
	}
}

func TestPrintRawJSON_EmptyBytes(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printRawJSON([]byte{})
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if out != "\n" {
		t.Errorf("printRawJSON() = %q, want newline", out)
	}
}

// ---- printBlob tests ----

func TestPrintBlob_ValidBlob(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	hash, err := s.WriteBlob([]byte(`{"tool": "Write"}`))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printBlob(s, hash)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "Write") {
		t.Errorf("printBlob() output missing content: %q", out)
	}
}

func TestPrintBlob_EmptyHash(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printBlob(s, "")
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "(none)") {
		t.Errorf("printBlob(empty) should show (none): %q", out)
	}
}

func TestPrintBlob_MissingHash(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printBlob(s, "nonexistent_hash")
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "error reading blob") {
		t.Errorf("printBlob(missing) should show error: %q", out)
	}
}

// ---- printStepMetadata tests ----

func TestPrintStepMetadata_Basic(t *testing.T) {
	step := &store.Step{
		SessionID:      "sess-1",
		Origin:         "claude_code",
		TurnID:         "turn-5",
		Parent:         "parent_hash_xxxx",
		TimestampNanos: 1000000000, // 1 second
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStepMetadata("full_step_hash_12chars", step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "sess-1") {
		t.Errorf("output missing session: %q", out)
	}
	if !strings.Contains(out, "claude_code") {
		t.Errorf("output missing origin: %q", out)
	}
	if !strings.Contains(out, "turn-5") {
		t.Errorf("output missing turn: %q", out)
	}
	if !strings.Contains(out, "parent_hash_xxxx") {
		t.Errorf("output missing parent: %q", out)
	}
}

func TestPrintStepMetadata_Minimal(t *testing.T) {
	step := &store.Step{
		SessionID:      "minimal-sess",
		TimestampNanos: 0,
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStepMetadata("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "minimal-sess") {
		t.Errorf("output missing session: %q", out)
	}
	// No parent/origin/turn should not appear
	if strings.Contains(out, "Parent:") {
		t.Errorf("output should not have Parent: for step without parent: %q", out)
	}
	if strings.Contains(out, "Origin:") {
		t.Errorf("output should not have Origin: when empty: %q", out)
	}
}

// ---- printStepCauses tests ----

func TestPrintStepCauses_Single(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	argsHash, _ := s.WriteBlob([]byte(`{"file_path":"main.go"}`))
	resultHash, _ := s.WriteBlob([]byte(`"success"`))

	step := &store.Step{
		Cause: store.Cause{ToolName: "Write", ToolUseID: "tool-1", ArgsBlob: argsHash, ResultBlob: resultHash},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStepCauses(s, step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "Write") {
		t.Errorf("output missing tool name: %q", out)
	}
	if !strings.Contains(out, "main.go") {
		t.Errorf("output missing args content: %q", out)
	}
}

func TestPrintStepCauses_Multiple(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	argsHash, _ := s.WriteBlob([]byte(`{}`))
	resultHash, _ := s.WriteBlob([]byte(`"ok"`))

	step := &store.Step{
		Causes: []store.Cause{
			{ToolName: "Write", ToolUseID: "t1", ArgsBlob: argsHash, ResultBlob: resultHash},
			{ToolName: "Bash", ToolUseID: "t2", ArgsBlob: argsHash, ResultBlob: resultHash},
		},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStepCauses(s, step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	// Should show "Tool 1" and "Tool 2"
	if !strings.Contains(out, "Tool 1") {
		t.Errorf("output missing 'Tool 1': %q", out)
	}
	if !strings.Contains(out, "Tool 2") {
		t.Errorf("output missing 'Tool 2': %q", out)
	}
}

func TestPrintStepCauses_Empty(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	step := &store.Step{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printStepCauses(s, step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	// Should not panic, and produce minimal output (no tool section)
	out := buf.String()
	_ = out // ok if empty or minimal
}

// ---- printIndexedMessage tests ----

func TestPrintIndexedMessage_User(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	msg := index.Message{MessageType: "user", ContentText: "Hello world"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printIndexedMessage(s, msg)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "Hello world") {
		t.Errorf("output missing message text: %q", out)
	}
}

func TestPrintIndexedMessage_Assistant(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	msg := index.Message{MessageType: "assistant", ContentText: "I will help"}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printIndexedMessage(s, msg)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "I will help") {
		t.Errorf("output missing assistant text: %q", out)
	}
}

func TestPrintIndexedMessage_ToolCall(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	inputHash, _ := s.WriteBlob([]byte(`{"command":"ls"}`))
	msg := index.Message{
		MessageType: "tool_call",
		ToolName:    "Bash",
		ToolUseID:   "tool-123",
		ToolInput:   string(inputHash),
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printIndexedMessage(s, msg)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "Bash") {
		t.Errorf("output missing tool name: %q", out)
	}
	if !strings.Contains(out, "tool-123") {
		t.Errorf("output missing tool use id: %q", out)
	}
}

func TestPrintIndexedMessage_ToolResult(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	outputHash, _ := s.WriteBlob([]byte(`{"result":"done"}`))
	msg := index.Message{
		MessageType: "tool_result",
		ToolName:    "Bash",
		ToolUseID:   "tool-456",
		ToolOutput:  string(outputHash),
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printIndexedMessage(s, msg)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	out := buf.String()
	if !strings.Contains(out, "Bash") {
		t.Errorf("output missing tool name: %q", out)
	}
}

// ---- Show command tests ----

func TestShowCmd_InvalidHash(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	store.Init(root)

	cmd := ShowCmd()
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent hash")
	}
}

func TestShowCmd_ValidStep(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}

	// Write a simple step
	blobHash, _ := s.WriteBlob([]byte("content"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "show-test-sess",
		Origin:         "claude_code",
		TurnID:         "turn-1",
		TimestampNanos: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).UnixNano(),
		Cause:          store.Cause{ToolName: "Read", ToolUseID: "t1"},
	}
	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("index step: %v", err)
	}
	idx.Close()

	cmd := ShowCmd()
	cmd.SetArgs([]string{string(stepHash)})

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("show cmd: %v", cmdErr)
	}
	if !strings.Contains(out, "show-test-sess") {
		t.Errorf("output missing session: %q", out)
	}
	if !strings.Contains(out, "Read") {
		t.Errorf("output missing tool: %q", out)
	}
}

func TestShowCmd_ShortHash(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}

	blobHash, _ := s.WriteBlob([]byte("x"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "x", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "short-test",
		Cause:          store.Cause{ToolName: "Edit", ToolUseID: "e1"},
		TimestampNanos: 1,
	}
	fullHash, _ := s.WriteStep(step)
	idx.IndexStep(fullHash, step, tree)
	idx.Close()

	// Try with short hash (first 8 chars)
	shortHash := string(fullHash)[:8]
	cmd := ShowCmd()
	cmd.SetArgs([]string{shortHash})

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("show cmd with short hash %q: %v", shortHash, cmdErr)
	}
	if !strings.Contains(out, "short-test") {
		t.Errorf("output missing session: %q", out)
	}
}

func TestShowCmd_WithMessages(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}

	blobHash, _ := s.WriteBlob([]byte("content"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f.txt", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "msg-sess",
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "w1"},
		TimestampNanos: 1,
	}
	stepHash, _ := s.WriteStep(step)
	idx.IndexStep(stepHash, step, tree)

	// Insert a message associated with this step
	msg := index.Message{
		ID:          "msg-1",
		SessionID:   "msg-sess",
		StepID:      string(stepHash),
		TurnID:      "turn-1",
		SeqNum:      0,
		Timestamp:   1,
		ProcessedAt: 1,
		MessageType: "user",
		ContentText: "Write a file",
	}
	if err := idx.InsertMessage(msg); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	idx.Close()

	cmd := ShowCmd()
	cmd.SetArgs([]string{string(stepHash)})

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("show cmd: %v", cmdErr)
	}
	if !strings.Contains(out, "Write a file") {
		t.Errorf("output missing conversation: %q", out)
	}
}

// ---- printStepConversation tests ----

func TestPrintStepConversation_NoTranscript(t *testing.T) {
	root := t.TempDir()
	s, _ := store.Init(root)
	idx, _ := index.Open(s)
	defer idx.Close()

	step := &store.Step{Transcript: ""}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printStepConversation(s, idx, "hash", step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	if err != nil {
		t.Fatalf("printStepConversation: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "no conversation") {
		t.Errorf("output should indicate no conversation: %q", out)
	}
}

func TestPrintStepConversation_IndexedMessages(t *testing.T) {
	root := t.TempDir()
	s, _ := store.Init(root)
	idx, _ := index.Open(s)
	defer idx.Close()

	// Insert a message for a step
	msg := index.Message{
		ID:          "m1",
		SessionID:   "s",
		StepID:      "aaaa",
		TurnID:      "t1",
		SeqNum:      0,
		Timestamp:   1,
		ProcessedAt: 1,
		MessageType: "user",
		ContentText: "help",
	}
	if err := idx.InsertMessage(msg); err != nil {
		t.Fatalf("insert message: %v", err)
	}

	step := &store.Step{}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Use a dummy hash that matches the step_id in the message
	err := printStepConversation(s, idx, "aaaa", step)
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	if err != nil {
		t.Fatalf("printStepConversation: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "help") {
		t.Errorf("output missing message text: %q", out)
	}
}
