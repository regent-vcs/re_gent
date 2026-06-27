package store

import (
	"strings"
	"testing"
)

func TestResolveShortHash_Success(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	content := []byte("test blob for short hash resolution")
	hash, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Resolve with 4-char prefix (minimum)
	prefix := string(hash[:4])
	resolved, err := s.ResolveShortHash(prefix)
	if err != nil {
		t.Fatalf("ResolveShortHash failed: %v", err)
	}
	if resolved != hash {
		t.Errorf("Expected %s, got %s", hash, resolved)
	}

	// Resolve with 8-char prefix
	prefix8 := string(hash[:8])
	resolved8, err := s.ResolveShortHash(prefix8)
	if err != nil {
		t.Fatalf("ResolveShortHash with 8-char prefix failed: %v", err)
	}
	if resolved8 != hash {
		t.Errorf("Expected %s, got %s", hash, resolved8)
	}
}

func TestResolveShortHash_TooShort(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = s.ResolveShortHash("093") // 3 chars
	if err == nil {
		t.Error("Expected error for 3-char prefix, got nil")
	}
	if !strings.Contains(err.Error(), "too short") {
		t.Errorf("Expected 'too short' error, got: %v", err)
	}
}

func TestResolveShortHash_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = s.ResolveShortHash("dead")
	if err == nil {
		t.Error("Expected error for non-matching prefix, got nil")
	}
}

func TestResolveShortHash_NonExistentShard(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = s.ResolveShortHash("ffde")
	if err == nil {
		t.Error("Expected error for non-existent shard, got nil")
	}
}

func TestReadBlob_ShortHash(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	content := []byte("test content for ReadBlob short hash")
	fullHash, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Read back with 4-char short hash (minimum)
	shortHash := Hash(fullHash[:4])
	readContent, err := s.ReadBlob(shortHash)
	if err != nil {
		t.Fatalf("ReadBlob with short hash failed: %v", err)
	}
	if string(readContent) != string(content) {
		t.Errorf("Content mismatch: got %q, want %q", readContent, content)
	}

	// Read back with full hash (should still work)
	readContentFull, err := s.ReadBlob(fullHash)
	if err != nil {
		t.Fatalf("ReadBlob with full hash failed: %v", err)
	}
	if string(readContentFull) != string(content) {
		t.Errorf("Content mismatch with full hash: got %q, want %q", readContentFull, content)
	}
}

func TestReadBlob_ShortHashNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = s.ReadBlob("dead")
	if err == nil {
		t.Error("Expected error for non-matching short hash, got nil")
	}
}

func TestReadBlob_TooShort(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = s.ReadBlob("0") // 1 char, below minimum for h[:2] indexing
	if err == nil {
		t.Error("Expected error for <2 char hash, got nil")
	}
}
