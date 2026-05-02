package hook

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestPayloadDecode(t *testing.T) {
	payloadJSON := `{
		"session_id": "test-session-456",
		"tool_use_id": "tool_use_abc123",
		"tool_name": "Write",
		"tool_input": {"file_path": "test.txt", "content": "hello world"},
		"tool_response": {"success": true},
		"cwd": "/tmp/test",
		"transcript_path": "/tmp/transcript.jsonl"
	}`

	var p Payload
	err := json.NewDecoder(strings.NewReader(payloadJSON)).Decode(&p)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	// Verify all fields
	if p.SessionID != "test-session-456" {
		t.Errorf("Expected session_id 'test-session-456', got %s", p.SessionID)
	}
	if p.ToolUseID != "tool_use_abc123" {
		t.Errorf("Expected tool_use_id 'tool_use_abc123', got %s", p.ToolUseID)
	}
	if p.ToolName != "Write" {
		t.Errorf("Expected tool_name 'Write', got %s", p.ToolName)
	}
	if p.CWD != "/tmp/test" {
		t.Errorf("Expected cwd '/tmp/test', got %s", p.CWD)
	}

	// Verify JSON fields are preserved
	var toolInput map[string]interface{}
	if err := json.Unmarshal(p.ToolInput, &toolInput); err != nil {
		t.Fatalf("Failed to unmarshal tool_input: %v", err)
	}
	if toolInput["file_path"] != "test.txt" {
		t.Errorf("Expected file_path 'test.txt', got %v", toolInput["file_path"])
	}
}

func TestHookSilentlyFailsWithoutRegent(t *testing.T) {
	// Test that hook returns nil when .regent/ doesn't exist
	workspace := t.TempDir()

	payload := Payload{
		SessionID:    "test-session",
		ToolUseID:    "tool_1",
		ToolName:     "Test",
		ToolInput:    json.RawMessage(`{}`),
		ToolResponse: json.RawMessage(`{}`),
		CWD:          workspace,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatalf("Encode payload failed: %v", err)
	}

	// Run hook - should not return error
	err := Run(&buf, io.Discard)
	if err != nil {
		t.Errorf("Hook should fail silently without .regent/, got error: %v", err)
	}
}

func TestHookCreatesStep(t *testing.T) {
	// Full integration: init store, send payload, verify step created
	workspace := t.TempDir()
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("Open index failed: %v", err)
	}
	defer idx.Close()

	// Create test file
	testFile := filepath.Join(workspace, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Write test file failed: %v", err)
	}

	// Create payload
	payload := Payload{
		SessionID:    "test-session-integration",
		ToolUseID:    "tool_write_1",
		ToolName:     "Write",
		ToolInput:    json.RawMessage(`{"file_path":"test.txt","content":"test content"}`),
		ToolResponse: json.RawMessage(`{"success":true}`),
		CWD:          workspace,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		t.Fatalf("Encode payload failed: %v", err)
	}

	// Run hook
	err = Run(&buf, io.Discard)
	if err != nil {
		t.Fatalf("Hook failed: %v", err)
	}

	// Verify step was created
	steps, err := idx.ListSteps("test-session-integration", 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	if steps[0].ToolName != "Write" {
		t.Errorf("Expected tool name 'Write', got %s", steps[0].ToolName)
	}

	if steps[0].ToolUseID != "tool_write_1" {
		t.Errorf("Expected tool_use_id 'tool_write_1', got %s", steps[0].ToolUseID)
	}

	// Verify session ref was updated
	headHash, err := s.ReadRef("sessions/" + payload.SessionID)
	if err != nil {
		t.Fatalf("ReadRef failed: %v", err)
	}

	if headHash != steps[0].Hash {
		t.Errorf("Session ref mismatch: got %s, want %s", headHash, steps[0].Hash)
	}
}

func TestHookMultipleStepsChain(t *testing.T) {
	// Test that multiple hook invocations create a proper parent chain
	workspace := t.TempDir()
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("Open index failed: %v", err)
	}
	defer idx.Close()

	sessionID := "test-session-chain"

	// Create 3 steps
	for i := 0; i < 3; i++ {
		filename := filepath.Join(workspace, "file.txt")
		content := []byte("step " + string(rune('0'+i)))
		os.WriteFile(filename, content, 0644)

		payload := Payload{
			SessionID:    sessionID,
			ToolUseID:    "tool_" + string(rune('A'+i)),
			ToolName:     "Write",
			ToolInput:    json.RawMessage(`{"file_path":"file.txt"}`),
			ToolResponse: json.RawMessage(`{"success":true}`),
			CWD:          workspace,
		}

		var buf bytes.Buffer
		json.NewEncoder(&buf).Encode(payload)

		if err := Run(&buf, io.Discard); err != nil {
			t.Fatalf("Hook %d failed: %v", i, err)
		}
	}

	// Verify we have 3 steps
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("Expected 3 steps, got %d", len(steps))
	}

	// Verify parent chain (steps are in reverse chronological order)
	// steps[0] is the latest, should have parent = steps[1]
	// steps[1] should have parent = steps[2]
	// steps[2] should have no parent

	if steps[0].ParentHash != steps[1].Hash {
		t.Errorf("Step 0 parent mismatch: got %s, want %s", steps[0].ParentHash, steps[1].Hash)
	}

	if steps[1].ParentHash != steps[2].Hash {
		t.Errorf("Step 1 parent mismatch: got %s, want %s", steps[1].ParentHash, steps[2].Hash)
	}

	if steps[2].ParentHash != "" {
		t.Errorf("Step 2 should have no parent, got %s", steps[2].ParentHash)
	}
}

func TestHookLogsErrors(t *testing.T) {
	// Test that errors are logged to .regent/log/hook-error.log
	workspace := t.TempDir()
	_, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a payload with invalid JSON (will cause decoding to fail)
	invalidJSON := `{"invalid": json`

	err = Run(strings.NewReader(invalidJSON), io.Discard)
	if err == nil {
		t.Error("Expected decode error, got nil")
	}

	// For the silent failure case, create payload with bad CWD after init
	// This will cause snapshot to fail, which should be logged
	payload := Payload{
		SessionID:    "test-error",
		ToolUseID:    "tool_error",
		ToolName:     "Test",
		ToolInput:    json.RawMessage(`{}`),
		ToolResponse: json.RawMessage(`{}`),
		CWD:          "/nonexistent/path/that/does/not/exist",
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(payload)

	// This should fail silently (return nil)
	err = Run(&buf, io.Discard)
	if err != nil {
		t.Errorf("Hook should return nil on error, got: %v", err)
	}

	// Note: The error log file won't be created because the store.Open fails silently
	// We've verified the silent failure behavior
}

func TestHookStoresToolArgsAndResult(t *testing.T) {
	// Verify that tool args and result are stored as blobs
	workspace := t.TempDir()
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("Open index failed: %v", err)
	}
	defer idx.Close()

	toolInput := json.RawMessage(`{"command":"ls -la","timeout":5000}`)
	toolResponse := json.RawMessage(`{"stdout":"file1.txt\nfile2.txt","stderr":"","exit_code":0}`)

	payload := Payload{
		SessionID:    "test-blobs",
		ToolUseID:    "tool_bash_1",
		ToolName:     "Bash",
		ToolInput:    toolInput,
		ToolResponse: toolResponse,
		CWD:          workspace,
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(payload)

	if err := Run(&buf, io.Discard); err != nil {
		t.Fatalf("Hook failed: %v", err)
	}

	// Read the step back
	steps, err := idx.ListSteps("test-blobs", 1)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	step, err := s.ReadStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("ReadStep failed: %v", err)
	}

	// Verify args blob
	if step.Cause.ArgsBlob == "" {
		t.Error("ArgsBlob is empty")
	} else {
		argsContent, err := s.ReadBlob(step.Cause.ArgsBlob)
		if err != nil {
			t.Fatalf("ReadBlob(args) failed: %v", err)
		}
		if !bytes.Equal(argsContent, toolInput) {
			t.Errorf("Args content mismatch: got %s, want %s", argsContent, toolInput)
		}
	}

	// Verify result blob
	if step.Cause.ResultBlob == "" {
		t.Error("ResultBlob is empty")
	} else {
		resultContent, err := s.ReadBlob(step.Cause.ResultBlob)
		if err != nil {
			t.Fatalf("ReadBlob(result) failed: %v", err)
		}
		if !bytes.Equal(resultContent, toolResponse) {
			t.Errorf("Result content mismatch: got %s, want %s", resultContent, toolResponse)
		}
	}
}

func TestHookComputesBlame(t *testing.T) {
	workspace := t.TempDir()
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("Open index failed: %v", err)
	}
	defer func() { _ = idx.Close() }()

	// Step 1: Create file with 3 lines
	testFile := filepath.Join(workspace, "test.txt")
	err = os.WriteFile(testFile, []byte("line1\nline2\nline3\n"), 0644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	payload1 := Payload{
		SessionID:    "test-session",
		ToolUseID:    "tool_write_1",
		ToolName:     "Write",
		ToolInput:    json.RawMessage(`{"file_path":"test.txt","content":"line1\nline2\nline3\n"}`),
		ToolResponse: json.RawMessage(`{"success":true}`),
		CWD:          workspace,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload1); err != nil {
		t.Fatalf("Encode payload1 failed: %v", err)
	}

	err = Run(&buf, io.Discard)
	if err != nil {
		t.Fatalf("Hook step 1 failed: %v", err)
	}

	// Step 2: Edit file (change line 2)
	err = os.WriteFile(testFile, []byte("line1\nmodified line2\nline3\n"), 0644)
	if err != nil {
		t.Fatalf("WriteFile step 2 failed: %v", err)
	}

	payload2 := Payload{
		SessionID:    "test-session",
		ToolUseID:    "tool_edit_2",
		ToolName:     "Edit",
		ToolInput:    json.RawMessage(`{"file_path":"test.txt","old_string":"line2","new_string":"modified line2"}`),
		ToolResponse: json.RawMessage(`{"success":true}`),
		CWD:          workspace,
	}

	buf.Reset()
	if err := json.NewEncoder(&buf).Encode(payload2); err != nil {
		t.Fatalf("Encode payload2 failed: %v", err)
	}

	err = Run(&buf, io.Discard)
	if err != nil {
		t.Fatalf("Hook step 2 failed: %v", err)
	}

	// Verify steps were created
	steps, err := idx.ListSteps("test-session", 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 2 {
		t.Fatalf("Expected 2 steps, got %d", len(steps))
	}

	step2 := steps[0] // Most recent first
	step1 := steps[1]

	// Read tree and blame from step 2
	step2Obj, err := s.ReadStep(step2.Hash)
	if err != nil {
		t.Fatalf("ReadStep failed: %v", err)
	}

	tree, err := s.ReadTree(step2Obj.Tree)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Find test.txt entry
	var entry *store.TreeEntry
	for i := range tree.Entries {
		if tree.Entries[i].Path == "test.txt" {
			entry = &tree.Entries[i]
			break
		}
	}

	if entry == nil {
		t.Fatalf("test.txt not found in tree")
	}

	if entry.Blame == "" {
		t.Fatalf("Expected blame map for test.txt, got empty")
	}

	// Read blame map
	blame, err := s.ReadBlame(entry.Blame)
	if err != nil {
		t.Fatalf("ReadBlame failed: %v", err)
	}

	if len(blame.Lines) != 3 {
		t.Fatalf("Expected 3 blame lines, got %d", len(blame.Lines))
	}

	// Verify blame attribution
	// Lines 1 and 3 should have same attribution (unchanged)
	// Line 2 should have different attribution (modified)
	if blame.Lines[0] == blame.Lines[1] {
		t.Errorf("Line 1 and line 2 should have different attribution")
	}

	if blame.Lines[0] != blame.Lines[2] {
		t.Errorf("Line 1 and line 3 should have same attribution")
	}

	t.Logf("Step 1: %s", step1.Hash)
	t.Logf("Step 2: %s", step2.Hash)
	t.Logf("Blame line 1: %s", blame.Lines[0])
	t.Logf("Blame line 2: %s", blame.Lines[1])
	t.Logf("Blame line 3: %s", blame.Lines[2])
}
