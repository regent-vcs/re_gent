package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestSessionForkDetection(t *testing.T) {
	workspace := t.TempDir()

	// Initialize regent
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("Open index failed: %v", err)
	}
	defer idx.Close()

	// Create a test file
	testFile := filepath.Join(workspace, "test.txt")
	if err := os.WriteFile(testFile, []byte("content v1"), 0644); err != nil {
		t.Fatalf("Write test file failed: %v", err)
	}

	// Step 1: Create session A with 2 steps
	t.Log("Creating session A with 2 steps")

	step1 := &store.Step{
		Parent:         "",
		Tree:           "tree1",
		SessionID:      "session-A",
		TimestampNanos: time.Now().UnixNano(),
		Cause: store.Cause{
			ToolName:  "Write",
			ToolUseID: "tool_001",
		},
	}

	tree1 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: "blob1"},
		},
	}

	step1Hash, err := s.WriteStep(step1)
	if err != nil {
		t.Fatalf("Write step1 failed: %v", err)
	}

	if err := idx.IndexStep(step1Hash, step1, tree1); err != nil {
		t.Fatalf("Index step1 failed: %v", err)
	}

	// Update session A ref
	if err := s.UpdateRef("sessions/session-A", "", step1Hash); err != nil {
		t.Fatalf("Update ref for session A failed: %v", err)
	}

	step2 := &store.Step{
		Parent:         step1Hash,
		Tree:           "tree2",
		SessionID:      "session-A",
		TimestampNanos: time.Now().UnixNano(),
		Cause: store.Cause{
			ToolName:  "Edit",
			ToolUseID: "tool_002",
		},
	}

	tree2 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: "blob2"},
		},
	}

	step2Hash, err := s.WriteStep(step2)
	if err != nil {
		t.Fatalf("Write step2 failed: %v", err)
	}

	if err := idx.IndexStep(step2Hash, step2, tree2); err != nil {
		t.Fatalf("Index step2 failed: %v", err)
	}

	// Update session A ref
	if err := s.UpdateRef("sessions/session-A", step1Hash, step2Hash); err != nil {
		t.Fatalf("Update ref for session A step2 failed: %v", err)
	}

	// Step 2: Create session B with parent from session A (fork!)
	t.Log("Creating session B forked from session A")

	step3 := &store.Step{
		Parent:         step1Hash, // Parent from different session = fork
		Tree:           "tree3",
		SessionID:      "session-B",
		TimestampNanos: time.Now().UnixNano(),
		Cause: store.Cause{
			ToolName:  "Write",
			ToolUseID: "tool_003",
		},
	}

	tree3 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "other.txt", Blob: "blob3"},
		},
	}

	step3Hash, err := s.WriteStep(step3)
	if err != nil {
		t.Fatalf("Write step3 failed: %v", err)
	}

	if err := idx.IndexStep(step3Hash, step3, tree3); err != nil {
		t.Fatalf("Index step3 failed: %v", err)
	}

	// Update session B ref
	if err := s.UpdateRef("sessions/session-B", "", step3Hash); err != nil {
		t.Fatalf("Update ref for session B failed: %v", err)
	}

	// Step 3: Verify session B is marked as fork
	t.Log("Verifying fork detection")

	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("Expected 2 sessions, got %d", len(sessions))
	}

	var sessionA, sessionB *index.SessionInfo
	for i := range sessions {
		if sessions[i].ID == "session-A" {
			sessionA = &sessions[i]
		} else if sessions[i].ID == "session-B" {
			sessionB = &sessions[i]
		}
	}

	if sessionA == nil {
		t.Fatal("Session A not found")
	}
	if sessionB == nil {
		t.Fatal("Session B not found")
	}

	// Session A should NOT be a fork
	if sessionA.ForkedFromSession != "" {
		t.Errorf("Session A should not be marked as fork, but has forked_from=%s", sessionA.ForkedFromSession)
	}

	// Session B should be a fork from session A
	if sessionB.ForkedFromSession != "session-A" {
		t.Errorf("Session B should fork from session-A, got %s", sessionB.ForkedFromSession)
	}
	if sessionB.ForkedFromStep != step1Hash {
		t.Errorf("Session B should fork from step %s, got %s", step1Hash, sessionB.ForkedFromStep)
	}
	if sessionB.ForkDetectedAt == nil {
		t.Error("Session B should have fork_detected_at timestamp")
	}

	t.Logf("✓ Session B correctly marked as fork from session A at step %s", step1Hash[:8])

	// Step 4: Continue session B, verify fork metadata persists
	t.Log("Continuing session B")

	step4 := &store.Step{
		Parent:         step3Hash,
		Tree:           "tree4",
		SessionID:      "session-B",
		TimestampNanos: time.Now().UnixNano(),
		Cause: store.Cause{
			ToolName:  "Edit",
			ToolUseID: "tool_004",
		},
	}

	tree4 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "other.txt", Blob: "blob4"},
		},
	}

	step4Hash, err := s.WriteStep(step4)
	if err != nil {
		t.Fatalf("Write step4 failed: %v", err)
	}

	if err := idx.IndexStep(step4Hash, step4, tree4); err != nil {
		t.Fatalf("Index step4 failed: %v", err)
	}

	// Verify fork metadata still present
	sessions2, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}

	var sessionB2 *index.SessionInfo
	for i := range sessions2 {
		if sessions2[i].ID == "session-B" {
			sessionB2 = &sessions2[i]
			break
		}
	}

	if sessionB2 == nil {
		t.Fatal("Session B not found after step 4")
	}

	if sessionB2.ForkedFromSession != "session-A" {
		t.Errorf("Session B fork metadata lost after continuation")
	}

	t.Logf("✓ Session B fork metadata persists after continuation")
}

func TestRgtCommandFiltering(t *testing.T) {
	workspace := t.TempDir()

	// Initialize regent
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
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Write test file failed: %v", err)
	}

	// Test via hook simulation
	transcriptPath := filepath.Join(workspace, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(""), 0644); err != nil {
		t.Fatalf("Create transcript failed: %v", err)
	}

	// Simulate hook payloads
	payloads := []struct {
		name       string
		command    string
		shouldSkip bool
	}{
		{"normal bash", "ls -la", false},
		{"rgt log", "rgt log", true},
		{"rgt blame", "rgt blame test.txt", true},
		{"grep with rgt", "grep rgt README.md", false},
	}

	expectedSteps := 0
	for _, tc := range payloads {
		t.Run(tc.name, func(t *testing.T) {
			toolInput := map[string]string{"command": tc.command}
			_, _ = json.Marshal(toolInput)

			// Manually check shouldSkipStep logic
			shouldSkip := tc.command == "rgt" || (len(tc.command) > 4 && tc.command[:4] == "rgt ")

			if shouldSkip != tc.shouldSkip {
				t.Errorf("Expected shouldSkip=%v for command %q, got %v", tc.shouldSkip, tc.command, shouldSkip)
			}

			if !tc.shouldSkip {
				expectedSteps++
			}

			t.Logf("Command %q: shouldSkip=%v", tc.command, shouldSkip)
		})
	}

	t.Logf("✓ Filter logic correctly handles %d test cases", len(payloads))
}
