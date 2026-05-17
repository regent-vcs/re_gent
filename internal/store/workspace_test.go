package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRegentDir_NotFound(t *testing.T) {
	root := t.TempDir()

	_, err := FindRegentDir(root)
	if !errors.Is(err, ErrNotRegentRepository) {
		t.Fatalf("expected ErrNotRegentRepository, got %v", err)
	}
}

func TestFindRegentDir_InParent(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatal(err)
	}

	nested := filepath.Join(root, "pkg", "sub")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := FindRegentDir(nested)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".regent")
	if got != want {
		t.Fatalf("FindRegentDir() = %q, want %q", got, want)
	}
}
