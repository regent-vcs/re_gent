package store

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBlobDeduplication(t *testing.T) {
	// Create temporary store
	tmpDir := t.TempDir()
	regentDir := filepath.Join(tmpDir, ".regent")
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Write same content twice
	content := []byte("hello, regent!")
	hash1, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	hash2, err := s.WriteBlob(content)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}

	// Hashes should be identical (deduplication)
	if hash1 != hash2 {
		t.Errorf("Expected identical hashes, got %s and %s", hash1, hash2)
	}

	// Read back content
	readContent, err := s.ReadBlob(hash1)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}

	if !bytes.Equal(content, readContent) {
		t.Errorf("Content mismatch: got %q, want %q", readContent, content)
	}

	// Verify only one copy exists on disk
	objPath := filepath.Join(regentDir, "objects", string(hash1[:2]), string(hash1))
	if _, err := os.Stat(objPath); err != nil {
		t.Errorf("Object file should exist at %s", objPath)
	}
}

func TestBlobIntegrity(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	testCases := [][]byte{
		[]byte(""),                    // empty
		[]byte("a"),                   // single byte
		[]byte("hello, world!"),       // small
		bytes.Repeat([]byte("x"), 1024*1024), // 1 MB
	}

	for i, content := range testCases {
		hash, err := s.WriteBlob(content)
		if err != nil {
			t.Errorf("Case %d: WriteBlob failed: %v", i, err)
			continue
		}

		readContent, err := s.ReadBlob(hash)
		if err != nil {
			t.Errorf("Case %d: ReadBlob failed: %v", i, err)
			continue
		}

		if !bytes.Equal(content, readContent) {
			t.Errorf("Case %d: Content mismatch (length: want %d, got %d)",
				i, len(content), len(readContent))
		}
	}
}

func TestBlobNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Try to read non-existent blob
	_, err = s.ReadBlob("0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Error("Expected error for non-existent blob, got nil")
	}
}
