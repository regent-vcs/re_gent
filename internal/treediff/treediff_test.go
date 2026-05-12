package treediff

import (
	"bytes"
	"testing"

	"github.com/regent-vcs/regent/internal/store"
)

func TestIsBinaryContent(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		{"empty", []byte{}, false},
		{"text", []byte("hello world"), false},
		{"binary with null", []byte{0x00, 0x01, 0x02}, true},
		{"text with newlines", []byte("line1\nline2\n"), false},
		{"binary in middle", []byte("text\x00more"), true},
		{"large text without null", bytes.Repeat([]byte("text"), 3000), false},
		{"large binary with null", append(bytes.Repeat([]byte("x"), 3000), 0x00), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinaryContent(tt.content)
			if result != tt.expected {
				t.Errorf("isBinaryContent(%q) = %v, want %v", tt.content, result, tt.expected)
			}
		})
	}
}

func TestCompareTreesForDiff_AddedFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create initial empty step (no parent)
	emptyTree := &store.Tree{Entries: []store.TreeEntry{}}
	emptyTreeHash, _ := s.WriteTree(emptyTree)
	step0 := &store.Step{
		SessionID: "test",
		Tree:      emptyTreeHash,
		Cause:     store.Cause{ToolName: "Init", ToolUseID: "tool_0"},
	}
	step0Hash, _ := s.WriteStep(step0)

	// Create step with one file added
	blob1, _ := s.WriteBlob([]byte("line1\nline2\nline3\n"))
	tree1 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: blob1, Mode: 0o644},
		},
	}
	tree1Hash, _ := s.WriteTree(tree1)
	step1 := &store.Step{
		Parent:    step0Hash,
		SessionID: "test",
		Tree:      tree1Hash,
		Cause:     store.Cause{ToolName: "Write", ToolUseID: "tool_1"},
	}
	step1Hash, _ := s.WriteStep(step1)

	// Compare
	diffs, err := CompareTreesForDiff(s, step0Hash, step1Hash)
	if err != nil {
		t.Fatalf("CompareTreesForDiff failed: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}

	diff := diffs[0]
	if diff.Path != "test.txt" {
		t.Errorf("Path: got %s, want test.txt", diff.Path)
	}
	if diff.Status != "added" {
		t.Errorf("Status: got %s, want added", diff.Status)
	}
	if diff.Additions != 3 {
		t.Errorf("Additions: got %d, want 3", diff.Additions)
	}
	if diff.Deletions != 0 {
		t.Errorf("Deletions: got %d, want 0", diff.Deletions)
	}
	if diff.IsBinary {
		t.Error("IsBinary should be false")
	}
}

func TestCompareTreesForDiff_ModifiedFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Step 0: Create file
	blob0, _ := s.WriteBlob([]byte("line1\nline2\nline3\n"))
	tree0 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: blob0, Mode: 0o644},
		},
	}
	tree0Hash, _ := s.WriteTree(tree0)
	step0 := &store.Step{
		SessionID: "test",
		Tree:      tree0Hash,
		Cause:     store.Cause{ToolName: "Write", ToolUseID: "tool_0"},
	}
	step0Hash, _ := s.WriteStep(step0)

	// Step 1: Modify file (change line 2, add line 4)
	blob1, _ := s.WriteBlob([]byte("line1\nmodified line2\nline3\nline4\n"))
	tree1 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: blob1, Mode: 0o644},
		},
	}
	tree1Hash, _ := s.WriteTree(tree1)
	step1 := &store.Step{
		Parent:    step0Hash,
		SessionID: "test",
		Tree:      tree1Hash,
		Cause:     store.Cause{ToolName: "Edit", ToolUseID: "tool_1"},
	}
	step1Hash, _ := s.WriteStep(step1)

	// Compare
	diffs, err := CompareTreesForDiff(s, step0Hash, step1Hash)
	if err != nil {
		t.Fatalf("CompareTreesForDiff failed: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}

	diff := diffs[0]
	if diff.Status != "modified" {
		t.Errorf("Status: got %s, want modified", diff.Status)
	}
	if diff.Additions != 2 {
		t.Errorf("Additions: got %d, want 2 (modified line + new line)", diff.Additions)
	}
	if diff.Deletions != 1 {
		t.Errorf("Deletions: got %d, want 1 (replaced line)", diff.Deletions)
	}
}

func TestCompareTreesForDiff_DeletedFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Step 0: File exists
	blob0, _ := s.WriteBlob([]byte("line1\nline2\n"))
	tree0 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: blob0, Mode: 0o644},
		},
	}
	tree0Hash, _ := s.WriteTree(tree0)
	step0 := &store.Step{
		SessionID: "test",
		Tree:      tree0Hash,
		Cause:     store.Cause{ToolName: "Write", ToolUseID: "tool_0"},
	}
	step0Hash, _ := s.WriteStep(step0)

	// Step 1: File deleted (empty tree)
	tree1 := &store.Tree{Entries: []store.TreeEntry{}}
	tree1Hash, _ := s.WriteTree(tree1)
	step1 := &store.Step{
		Parent:    step0Hash,
		SessionID: "test",
		Tree:      tree1Hash,
		Cause:     store.Cause{ToolName: "Bash", ToolUseID: "tool_1"},
	}
	step1Hash, _ := s.WriteStep(step1)

	// Compare
	diffs, err := CompareTreesForDiff(s, step0Hash, step1Hash)
	if err != nil {
		t.Fatalf("CompareTreesForDiff failed: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}

	diff := diffs[0]
	if diff.Status != "deleted" {
		t.Errorf("Status: got %s, want deleted", diff.Status)
	}
	if diff.Deletions != 2 {
		t.Errorf("Deletions: got %d, want 2", diff.Deletions)
	}
	if diff.Additions != 0 {
		t.Errorf("Additions: got %d, want 0", diff.Additions)
	}
}

func TestCompareTreesForDiff_NoParent(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create first step (no parent)
	blob, _ := s.WriteBlob([]byte("content"))
	tree := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "init.txt", Blob: blob, Mode: 0o644},
		},
	}
	treeHash, _ := s.WriteTree(tree)
	step := &store.Step{
		SessionID: "test",
		Tree:      treeHash,
		Cause:     store.Cause{ToolName: "Write", ToolUseID: "tool_1"},
	}
	stepHash, _ := s.WriteStep(step)

	// Compare with empty parent
	diffs, err := CompareTreesForDiff(s, "", stepHash)
	if err != nil {
		t.Fatalf("CompareTreesForDiff failed: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Status != "added" {
		t.Errorf("Status: got %s, want added", diffs[0].Status)
	}
}

func TestCompareTreesForDiff_UnchangedFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create same blob twice (deduplication)
	blob, _ := s.WriteBlob([]byte("unchanged"))
	tree := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "unchanged.txt", Blob: blob, Mode: 0o644},
		},
	}
	treeHash, _ := s.WriteTree(tree)

	step0 := &store.Step{
		SessionID: "test",
		Tree:      treeHash,
		Cause:     store.Cause{ToolName: "Write", ToolUseID: "tool_0"},
	}
	step0Hash, _ := s.WriteStep(step0)

	step1 := &store.Step{
		Parent:    step0Hash,
		SessionID: "test",
		Tree:      treeHash, // Same tree!
		Cause:     store.Cause{ToolName: "Bash", ToolUseID: "tool_1"},
	}
	step1Hash, _ := s.WriteStep(step1)

	// Compare
	diffs, err := CompareTreesForDiff(s, step0Hash, step1Hash)
	if err != nil {
		t.Fatalf("CompareTreesForDiff failed: %v", err)
	}

	if len(diffs) != 0 {
		t.Errorf("Expected 0 diffs for unchanged file, got %d", len(diffs))
	}
}

func TestCompareTreesForDiff_BinaryFile(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Add binary file (with null bytes)
	binaryBlob, _ := s.WriteBlob([]byte{0xFF, 0x00, 0xAB, 0xCD})
	tree := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "binary.bin", Blob: binaryBlob, Mode: 0o644},
		},
	}
	treeHash, _ := s.WriteTree(tree)

	step := &store.Step{
		SessionID: "test",
		Tree:      treeHash,
		Cause:     store.Cause{ToolName: "Write", ToolUseID: "tool_1"},
	}
	stepHash, _ := s.WriteStep(step)

	// Compare with no parent
	diffs, err := CompareTreesForDiff(s, "", stepHash)
	if err != nil {
		t.Fatalf("CompareTreesForDiff failed: %v", err)
	}

	if len(diffs) != 1 {
		t.Fatalf("Expected 1 diff, got %d", len(diffs))
	}

	diff := diffs[0]
	if !diff.IsBinary {
		t.Error("Binary file should be detected as binary")
	}
	if diff.Additions != 0 || diff.Deletions != 0 {
		t.Errorf("Binary file should have 0 additions/deletions, got +%d -%d", diff.Additions, diff.Deletions)
	}
}

func TestComputeLineStats(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	oldBlob, _ := s.WriteBlob([]byte("line1\nline2\n"))
	newBlob, _ := s.WriteBlob([]byte("line1\nmodified\nline3\n"))

	additions, deletions, isBinary, err := computeLineStats(s, oldBlob, newBlob)
	if err != nil {
		t.Fatalf("computeLineStats failed: %v", err)
	}

	if isBinary {
		t.Error("Should not be binary")
	}

	if additions != 2 {
		t.Errorf("Additions: got %d, want 2", additions)
	}

	if deletions != 1 {
		t.Errorf("Deletions: got %d, want 1", deletions)
	}
}
