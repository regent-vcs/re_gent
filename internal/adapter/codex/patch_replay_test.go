package codex

import (
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/store"
)

func TestApplyPatchToEntries_AddFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	result, err := applyPatchToEntries(s, tmpDir, map[string]store.TreeEntry{}, "*** Begin Patch\n*** Add File: hello.txt\n+hello\n*** End Patch\n")
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	entry, ok := result["hello.txt"]
	if !ok {
		t.Fatalf("expected hello.txt to be present")
	}

	content, err := s.ReadBlob(entry.Blob)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(content) != "hello" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestApplyPatchToEntries_UpdateFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	baseBlob, err := s.WriteBlob([]byte("one\ntwo\nthree"))
	if err != nil {
		t.Fatalf("write base blob: %v", err)
	}
	base := map[string]store.TreeEntry{
		"hello.txt": {Path: "hello.txt", Blob: baseBlob, Mode: 0o644},
	}

	patch := strings.Join([]string{
		"*** Begin Patch",
		"*** Update File: hello.txt",
		"@@ -1,3 +1,3 @@",
		" one",
		"-two",
		"+TWO",
		" three",
		"*** End Patch",
		"",
	}, "\n")

	result, err := applyPatchToEntries(s, tmpDir, base, patch)
	if err != nil {
		t.Fatalf("apply patch: %v", err)
	}

	entry, ok := result["hello.txt"]
	if !ok {
		t.Fatalf("expected hello.txt to be present")
	}

	content, err := s.ReadBlob(entry.Blob)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(content) != "one\nTWO\nthree" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}
