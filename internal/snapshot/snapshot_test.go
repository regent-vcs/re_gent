package snapshot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/store"
)

func TestSnapshotBasic(t *testing.T) {
	// Create test workspace
	workspace := t.TempDir()

	// Create .regent in workspace
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create test files
	os.WriteFile(filepath.Join(workspace, "file1.txt"), []byte("content1"), 0o644)
	os.WriteFile(filepath.Join(workspace, "file2.txt"), []byte("content2"), 0o644)
	os.MkdirAll(filepath.Join(workspace, "subdir"), 0o755)
	os.WriteFile(filepath.Join(workspace, "subdir", "file3.txt"), []byte("content3"), 0o644)

	// Snapshot
	ig := ignore.Default(workspace)
	treeHash, err := Snapshot(s, workspace, ig)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	// Read tree
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Verify we captured 3 files (.regent should be ignored)
	if len(tree.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(tree.Entries))
		for _, e := range tree.Entries {
			t.Logf("  Entry: %s", e.Path)
		}
	}

	// Verify paths use forward slashes
	hasFile1 := false
	hasFile2 := false
	hasSubdirFile3 := false

	for _, entry := range tree.Entries {
		switch entry.Path {
		case "file1.txt":
			hasFile1 = true
		case "file2.txt":
			hasFile2 = true
		case "subdir/file3.txt":
			hasSubdirFile3 = true
		}
	}

	if !hasFile1 || !hasFile2 || !hasSubdirFile3 {
		t.Errorf("Missing expected files: file1=%v, file2=%v, subdir/file3=%v",
			hasFile1, hasFile2, hasSubdirFile3)
	}

	// Verify content is correct
	entry := tree.FindEntry("file1.txt")
	if entry == nil {
		t.Fatal("file1.txt not found in tree")
	}
	content, err := s.ReadBlob(entry.Blob)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}
	if string(content) != "content1" {
		t.Errorf("Wrong content: got %q, want %q", content, "content1")
	}
}

func TestSnapshotIgnorePatterns(t *testing.T) {
	workspace := t.TempDir()
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create various files, some that should be ignored
	os.WriteFile(filepath.Join(workspace, "keep.txt"), []byte("keep"), 0o644)
	os.MkdirAll(filepath.Join(workspace, "node_modules"), 0o755)
	os.WriteFile(filepath.Join(workspace, "node_modules", "package.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(workspace, ".git"), 0o755)
	os.WriteFile(filepath.Join(workspace, ".git", "config"), []byte("config"), 0o644)

	ig := ignore.Default(workspace)
	treeHash, err := Snapshot(s, workspace, ig)
	if err != nil {
		t.Fatalf("Snapshot failed: %v", err)
	}

	tree, err := s.ReadTree(treeHash)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	// Should only have keep.txt (node_modules, .git, .regent should be ignored)
	if len(tree.Entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(tree.Entries))
		for _, e := range tree.Entries {
			t.Logf("  Entry: %s", e.Path)
		}
	}

	if tree.Entries[0].Path != "keep.txt" {
		t.Errorf("Expected keep.txt, got %s", tree.Entries[0].Path)
	}
}

func TestSnapshotDeterminism(t *testing.T) {
	workspace := t.TempDir()
	s, err := store.Init(workspace)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create files
	os.WriteFile(filepath.Join(workspace, "a.txt"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(workspace, "b.txt"), []byte("b"), 0o644)
	os.WriteFile(filepath.Join(workspace, "c.txt"), []byte("c"), 0o644)

	ig := ignore.Default(workspace)

	// Snapshot twice
	hash1, err := Snapshot(s, workspace, ig)
	if err != nil {
		t.Fatalf("Snapshot 1 failed: %v", err)
	}

	hash2, err := Snapshot(s, workspace, ig)
	if err != nil {
		t.Fatalf("Snapshot 2 failed: %v", err)
	}

	// Hashes should be identical (deterministic)
	if hash1 != hash2 {
		t.Errorf("Expected identical snapshot hashes, got %s and %s", hash1, hash2)
	}
}
