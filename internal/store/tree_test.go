package store

import (
	"testing"
)

func TestTreeDeterminism(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create two identical trees with entries in different orders
	blob1, _ := s.WriteBlob([]byte("content1"))
	blob2, _ := s.WriteBlob([]byte("content2"))
	blob3, _ := s.WriteBlob([]byte("content3"))

	tree1 := &Tree{
		Entries: []TreeEntry{
			{Path: "file1.txt", Blob: blob1, Mode: 0o644},
			{Path: "file2.txt", Blob: blob2, Mode: 0o644},
			{Path: "file3.txt", Blob: blob3, Mode: 0o644},
		},
	}

	tree2 := &Tree{
		Entries: []TreeEntry{
			{Path: "file3.txt", Blob: blob3, Mode: 0o644}, // Different order
			{Path: "file1.txt", Blob: blob1, Mode: 0o644},
			{Path: "file2.txt", Blob: blob2, Mode: 0o644},
		},
	}

	hash1, err := s.WriteTree(tree1)
	if err != nil {
		t.Fatalf("WriteTree failed: %v", err)
	}

	hash2, err := s.WriteTree(tree2)
	if err != nil {
		t.Fatalf("WriteTree failed: %v", err)
	}

	// Hashes should be identical (deterministic ordering)
	if hash1 != hash2 {
		t.Errorf("Expected identical tree hashes, got %s and %s", hash1, hash2)
	}

	// Read back and verify
	readTree, err := s.ReadTree(hash1)
	if err != nil {
		t.Fatalf("ReadTree failed: %v", err)
	}

	if len(readTree.Entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(readTree.Entries))
	}

	// Verify entries are sorted
	if readTree.Entries[0].Path != "file1.txt" ||
		readTree.Entries[1].Path != "file2.txt" ||
		readTree.Entries[2].Path != "file3.txt" {
		t.Error("Tree entries not sorted correctly")
	}
}

func TestTreeFindEntry(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	blob1, _ := s.WriteBlob([]byte("content1"))
	blob2, _ := s.WriteBlob([]byte("content2"))

	tree := &Tree{
		Entries: []TreeEntry{
			{Path: "dir/file1.txt", Blob: blob1},
			{Path: "dir/file2.txt", Blob: blob2},
		},
	}

	// Test FindEntry
	entry := tree.FindEntry("dir/file1.txt")
	if entry == nil {
		t.Error("Expected to find entry, got nil")
	} else if entry.Blob != blob1 {
		t.Errorf("Wrong blob: got %s, want %s", entry.Blob, blob1)
	}

	// Test non-existent path
	entry = tree.FindEntry("nonexistent.txt")
	if entry != nil {
		t.Error("Expected nil for non-existent path")
	}
}
