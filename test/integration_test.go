package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
)

// TestEndToEndStep tests creating a complete step: snapshot → step → index
func TestEndToEndStep(t *testing.T) {
	workspace := t.TempDir()

	// Initialize store
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Initialize index
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("Open index failed: %v", err)
	}
	defer idx.Close()

	// Create test files
	os.WriteFile(filepath.Join(workspace, "test.txt"), []byte("hello regent"), 0o644)

	// Snapshot
	ig := ignore.Default(workspace)
	treeHash, err := snapshot.Snapshot(s, workspace, ig)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Create step
	sessionID := "test-session-123"
	step := &store.Step{
		Parent:    "",
		Tree:      treeHash,
		SessionID: sessionID,
		Cause: store.Cause{
			ToolUseID: "tool_use_1",
			ToolName:  "Write",
		},
		TimestampNanos: time.Now().UnixNano(),
	}

	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("WriteStep failed: %v", err)
	}

	// Index the step
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("IndexStep failed: %v", err)
	}

	// Update session ref
	if err := s.UpdateRef("sessions/"+sessionID, "", stepHash); err != nil {
		t.Fatalf("UpdateRef failed: %v", err)
	}

	// Verify we can query it back
	headHash, err := idx.SessionHead(sessionID)
	if err != nil {
		t.Fatalf("SessionHead failed: %v", err)
	}

	if headHash != stepHash {
		t.Errorf("SessionHead returned %s, want %s", headHash, stepHash)
	}

	// Verify log
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	if steps[0].ToolName != "Write" {
		t.Errorf("Expected tool name 'Write', got %s", steps[0].ToolName)
	}

	// Verify sessions list
	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}

	if sessions[0].ID != sessionID {
		t.Errorf("Expected session ID %s, got %s", sessionID, sessions[0].ID)
	}

	t.Logf("✓ Step created: %s", stepHash[:8])
	t.Logf("✓ Session indexed: %s", sessionID)
	t.Logf("✓ Files captured: %d", len(tree.Entries))
}

// TestMultipleSteps tests creating a chain of steps
func TestMultipleSteps(t *testing.T) {
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

	sessionID := "test-session-multi"
	ig := ignore.Default(workspace)

	var prevHash store.Hash

	// Create 5 steps
	for i := 0; i < 5; i++ {
		// Modify file
		content := []byte("step " + string(rune('0'+i)))
		os.WriteFile(filepath.Join(workspace, "file.txt"), content, 0o644)

		// Snapshot
		treeHash, err := snapshot.Snapshot(s, workspace, ig)
		if err != nil {
			t.Fatalf("Snapshot %d failed: %v", i, err)
		}

		// Create step
		step := &store.Step{
			Parent:         prevHash,
			Tree:           treeHash,
			SessionID:      sessionID,
			Cause:          store.Cause{ToolUseID: "tool_" + string(rune('0'+i)), ToolName: "Edit"},
			TimestampNanos: time.Now().UnixNano(),
		}

		stepHash, err := s.WriteStep(step)
		if err != nil {
			t.Fatalf("WriteStep %d failed: %v", i, err)
		}

		tree, _ := s.ReadTree(treeHash)
		if err := idx.IndexStep(stepHash, step, tree); err != nil {
			t.Fatalf("IndexStep %d failed: %v", i, err)
		}

		// Update ref (with retry for concurrency safety)
		if err := s.UpdateRefWithRetry("sessions/"+sessionID, prevHash, stepHash, 3); err != nil {
			t.Fatalf("UpdateRef %d failed: %v", i, err)
		}

		prevHash = stepHash
		time.Sleep(1 * time.Millisecond) // Small delay for timestamp uniqueness
	}

	// Verify chain
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 5 {
		t.Fatalf("Expected 5 steps, got %d", len(steps))
	}

	// Steps should be in reverse chronological order
	for i, step := range steps {
		expectedToolUse := "tool_" + string(rune('4'-i))
		if step.ToolUseID != expectedToolUse {
			t.Errorf("Step %d: expected tool_use %s, got %s", i, expectedToolUse, step.ToolUseID)
		}
	}

	// Verify parent chain
	head := steps[0].Hash
	count := 0
	err = s.WalkAncestors(head, func(step *store.Step) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("WalkAncestors failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 ancestors, got %d", count)
	}

	t.Logf("✓ Created chain of 5 steps")
	t.Logf("✓ Head: %s", head[:8])
}
