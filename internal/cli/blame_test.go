package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// captureStdout captures os.Stdout during the execution of fn.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// ---- displayBlameLine tests ----

func TestDisplayBlameLine_ValidStep(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Write a tree with a file so we can write a proper step
	blobHash, _ := s.WriteBlob([]byte("line1\nline2\n"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "test.txt", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)

	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "test-session",
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-1"},
		TimestampNanos: 1000000000, // 1 second in nanos
	}
	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = displayBlameLine(s, stepHash, 1, "hello world")
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	if err != nil {
		t.Fatalf("displayBlameLine: %v", err)
	}
	out := buf.String()
	// Output should contain the step hash prefix (8 chars), timestamp, tool name, and line content
	if !strings.Contains(out, "Write") {
		t.Errorf("output missing tool name: %q", out)
	}
	if !strings.Contains(out, "hello world") {
		t.Errorf("output missing line content: %q", out)
	}
	if !strings.Contains(out, "   1 ") {
		t.Errorf("output missing line number: %q", out)
	}
}

func TestDisplayBlameLine_MissingStep(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = displayBlameLine(s, "nonexistent", 1, "content")
	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)
	os.Stdout = old

	// Should NOT return error (graceful degradation)
	if err != nil {
		t.Fatalf("displayBlameLine should not error on missing step, got %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "(pre-blame)") {
		t.Errorf("missing step output should contain (pre-blame): %q", out)
	}
	if !strings.Contains(out, "(unknown)") {
		t.Errorf("missing step output should contain (unknown): %q", out)
	}
}

// ---- Blame command tests ----

func TestBlameCmd_NoRegentDir(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	cmd := BlameCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"nonexistent.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing .regent directory")
	}
}

func TestBlameCmd_FileNotFound(t *testing.T) {
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

	// Write a session with a head step (required for blame to resolve the session)
	writeIndexedSessionStep(t, s, idx, "sess-1", "", "agent", time.Now())
	idx.Close()

	cmd := BlameCmd()
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"nonexistent.txt"})

	err = cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBlameCmd_InvalidLineNumber(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	store.Init(root)

	cmd := BlameCmd()
	cmd.SetArgs([]string{"file.txt:abc"}) // invalid line number

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid line number")
	}
	if !strings.Contains(err.Error(), "invalid line number") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBlameCmd_EmptyFile(t *testing.T) {
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

	// Create empty file blob
	blobHash, err := s.WriteBlob([]byte(""))
	if err != nil {
		t.Fatalf("write empty blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "empty.txt", Blob: blobHash}}}
	treeHash, err := s.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "sess-1",
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "t1"},
		TimestampNanos: 1,
	}
	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("index step: %v", err)
	}

	// Write empty blame map
	blameMap := &store.BlameMap{Lines: []store.Hash{}}
	if err := s.WriteBlameForFile(stepHash, "empty.txt", blameMap); err != nil {
		t.Fatalf("write blame map: %v", err)
	}
	idx.Close()

	cmd := BlameCmd()
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"empty.txt"})

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("blame cmd: %v", cmdErr)
	}
	if !strings.Contains(out, "empty") {
		t.Errorf("expected empty file message, got: %q", out)
	}
}

func TestBlameCmd_ValidBlame(t *testing.T) {
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

	// Create file content
	content := []byte("line one\nline two\nline three\n")
	blobHash, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "test.txt", Blob: blobHash}}}
	treeHash, err := s.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "sess-1",
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "t1"},
		TimestampNanos: 1000000000,
	}
	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("index step: %v", err)
	}

	// Write blame map: all 3 lines attributed to this step
	blameMap := &store.BlameMap{Lines: []store.Hash{stepHash, stepHash, stepHash}}
	if err := s.WriteBlameForFile(stepHash, "test.txt", blameMap); err != nil {
		t.Fatalf("write blame map: %v", err)
	}
	idx.Close()

	cmd := BlameCmd()
	var errBuf bytes.Buffer
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"test.txt"})

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("blame cmd: %v\nstderr: %s", cmdErr, errBuf.String())
	}
	if !strings.Contains(out, "line one") {
		t.Errorf("output missing first line: %q", out)
	}
	if !strings.Contains(out, "line two") {
		t.Errorf("output missing second line: %q", out)
	}
	if !strings.Contains(out, "line three") {
		t.Errorf("output missing third line: %q", out)
	}
	if !strings.Contains(out, "Write") {
		t.Errorf("output missing tool name: %q", out)
	}
}

func TestBlameCmd_SingleLine(t *testing.T) {
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

	content := []byte("a\nb\nc\n")
	blobHash, _ := s.WriteBlob(content)
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f.txt", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "sess-1",
		Cause:          store.Cause{ToolName: "Edit", ToolUseID: "t2"},
		TimestampNanos: 1000000000,
	}
	stepHash, _ := s.WriteStep(step)
	idx.IndexStep(stepHash, step, tree)
	blameMap := &store.BlameMap{Lines: []store.Hash{stepHash, stepHash, stepHash}}
	s.WriteBlameForFile(stepHash, "f.txt", blameMap)
	idx.Close()

	// Request only line 2
	cmd := BlameCmd()
	cmd.SetArgs([]string{"f.txt:2"})

	var cmdErr error
	out := captureStdout(func() {
		cmdErr = cmd.Execute()
	})
	if cmdErr != nil {
		t.Fatalf("blame cmd: %v", cmdErr)
	}
	if !strings.Contains(out, "b") && !strings.Contains(out, "   2 ") {
		t.Errorf("expected line 2 content, got: %q", out)
	}
}

func TestBlameCmd_LineOutOfRange(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	s, _ := store.Init(root)
	idx, _ := index.Open(s)

	blobHash, _ := s.WriteBlob([]byte("a\n"))
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "f.txt", Blob: blobHash}}}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{Tree: treeHash, SessionID: "s", Cause: store.Cause{ToolName: "W"}, TimestampNanos: 1}
	stepHash, _ := s.WriteStep(step)
	idx.IndexStep(stepHash, step, tree)
	blameMap := &store.BlameMap{Lines: []store.Hash{stepHash}}
	s.WriteBlameForFile(stepHash, "f.txt", blameMap)
	idx.Close()

	cmd := BlameCmd()
	cmd.SetArgs([]string{"f.txt:5"}) // only 1 line exists

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for out-of-range line")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBlameCmd_NoSessions(t *testing.T) {
	root := t.TempDir()
	withWorkingDir(t, root)

	store.Init(root)

	cmd := BlameCmd()
	cmd.SetArgs([]string{"nonexistent.txt"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error with no sessions")
	}
	if !strings.Contains(err.Error(), "no sessions") {
		t.Errorf("unexpected error: %v", err)
	}
}
