package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// captureStdout is defined in blame_test.go (same package)

// ---- validateConsistency tests ----

func TestValidateConsistency_NoRefs(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer idx.Close()

	out := captureStdout(func() {
		err = validateConsistency(s, idx)
	})
	if err != nil {
		t.Fatalf("validateConsistency: %v", err)
	}
	if !strings.Contains(out, "All session refs match") {
		t.Errorf("expected success message, got: %q", out)
	}
}

func TestValidateConsistency_Mismatch(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}

	// Write a session with head step in the index
	sessionID := "test-session"
	writeIndexedSessionStep(t, s, idx, sessionID, "", "agent", time.Now())

	// Now write a DIFFERENT step to the ref file, creating a mismatch
	fakeHash := store.Hash("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")
	refPath := filepath.Join(s.Root, "refs", "sessions", sessionID)
	if err := os.WriteFile(refPath, []byte(fakeHash+"\n"), 0644); err != nil {
		idx.Close()
		t.Fatalf("write ref: %v", err)
	}

	// Close index before capturing to avoid file lock
	idx.Close()

	out := captureStdout(func() {
		_ = validateConsistency(s, idx)
	})

	if !strings.Contains(out, "Consistency issues") {
		t.Errorf("expected mismatch message, got: %q", out)
	}
}

func TestValidateConsistency_Match(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}

	// Write a proper session with ref and index in sync
	sessionID := "sync-session"
	writeIndexedSessionStep(t, s, idx, sessionID, "", "agent", time.Now())
	idx.Close()

	// Reopen for clean read
	idx, err = index.Open(s)
	if err != nil {
		t.Fatalf("reopen index: %v", err)
	}
	defer idx.Close()

	out := captureStdout(func() {
		err = validateConsistency(s, idx)
	})
	if err != nil {
		t.Fatalf("validateConsistency should succeed when in sync: %v", err)
	}
	if !strings.Contains(out, "All session refs match") {
		t.Errorf("expected success message, got: %q", out)
	}
}

// ---- Status command tests ----

func TestStatusCmd_NoSessions(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	_, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	cmd := StatusCmd()

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("status cmd: %v", cmdErr)
	}
	if !strings.Contains(out, "No sessions") {
		t.Errorf("expected 'No sessions' for empty repo: %q", out)
	}
}

func TestStatusCmd_WithSessions(t *testing.T) {
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

	sessionID := "test-session-1"
	writeIndexedSessionStep(t, s, idx, sessionID, "", "claude-code", time.Now())
	idx.Close()

	cmd := StatusCmd()

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("status cmd: %v", cmdErr)
	}
	if !strings.Contains(out, sessionID) {
		t.Errorf("output missing session ID: %q", out)
	}
	if !strings.Contains(out, "Sessions:") {
		t.Errorf("output missing Sessions count: %q", out)
	}
	if !strings.Contains(out, "Consistency:") {
		t.Errorf("output missing Consistency section: %q", out)
	}
}

func TestStatusCmd_NoRegentDir(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	cmd := StatusCmd()
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing .regent directory")
	}
}

func TestStatusCmd_MultipleSessions(t *testing.T) {
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

	writeIndexedSessionStep(t, s, idx, "sess-A", "", "agent-A", time.Now())
	writeIndexedSessionStep(t, s, idx, "sess-B", "", "agent-B", time.Now())
	idx.Close()

	cmd := StatusCmd()
	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("status cmd: %v", cmdErr)
	}

	// Should show both sessions
	if !strings.Contains(out, "sess-A") {
		t.Errorf("output missing sess-A: %q", out)
	}
	if !strings.Contains(out, "sess-B") {
		t.Errorf("output missing sess-B: %q", out)
	}
	// Should show count
	if !strings.Contains(out, "Sessions:") {
		t.Errorf("output missing Sessions count: %q", out)
	}
}
