package diff

import (
	"testing"
)

func TestLineDiff_NoChanges(t *testing.T) {
	old := []byte("line1\nline2\nline3\n")
	new := []byte("line1\nline2\nline3\n")
	ops := LineDiff(old, new)

	if len(ops) != 1 {
		t.Fatalf("Expected 1 op, got %d", len(ops))
	}
	if ops[0].Tag != "equal" {
		t.Errorf("Expected 'equal' op, got %s", ops[0].Tag)
	}
	if ops[0].I2-ops[0].I1 != 3 {
		t.Errorf("Expected 3 equal lines, got %d", ops[0].I2-ops[0].I1)
	}
}

func TestLineDiff_Insert(t *testing.T) {
	old := []byte("line1\nline3\n")
	new := []byte("line1\nline2\nline3\n")
	ops := LineDiff(old, new)

	// Should have: equal(line1), insert(line2), equal(line3)
	if len(ops) != 3 {
		t.Fatalf("Expected 3 ops, got %d", len(ops))
	}

	if ops[0].Tag != "equal" {
		t.Errorf("Op 0: expected 'equal', got %s", ops[0].Tag)
	}
	if ops[1].Tag != "insert" {
		t.Errorf("Op 1: expected 'insert', got %s", ops[1].Tag)
	}
	if ops[2].Tag != "equal" {
		t.Errorf("Op 2: expected 'equal', got %s", ops[2].Tag)
	}

	// Verify insert range
	if ops[1].J2-ops[1].J1 != 1 {
		t.Errorf("Expected 1 inserted line, got %d", ops[1].J2-ops[1].J1)
	}
}

func TestLineDiff_Delete(t *testing.T) {
	old := []byte("line1\nline2\nline3\n")
	new := []byte("line1\nline3\n")
	ops := LineDiff(old, new)

	// Should have: equal(line1), delete(line2), equal(line3)
	if len(ops) != 3 {
		t.Fatalf("Expected 3 ops, got %d", len(ops))
	}

	if ops[1].Tag != "delete" {
		t.Errorf("Op 1: expected 'delete', got %s", ops[1].Tag)
	}

	// Verify delete range
	if ops[1].I2-ops[1].I1 != 1 {
		t.Errorf("Expected 1 deleted line, got %d", ops[1].I2-ops[1].I1)
	}
}

func TestLineDiff_Replace(t *testing.T) {
	old := []byte("line1\nold line\nline3\n")
	new := []byte("line1\nnew line\nline3\n")
	ops := LineDiff(old, new)

	// Should have: equal, replace, equal
	if len(ops) != 3 {
		t.Fatalf("Expected 3 ops, got %d", len(ops))
	}

	if ops[1].Tag != "replace" {
		t.Errorf("Op 1: expected 'replace', got %s", ops[1].Tag)
	}

	// Verify replace range
	if ops[1].I2-ops[1].I1 != 1 {
		t.Errorf("Expected 1 old line in replace, got %d", ops[1].I2-ops[1].I1)
	}
	if ops[1].J2-ops[1].J1 != 1 {
		t.Errorf("Expected 1 new line in replace, got %d", ops[1].J2-ops[1].J1)
	}
}

func TestLineDiff_EmptyFiles(t *testing.T) {
	old := []byte("")
	new := []byte("")
	ops := LineDiff(old, new)

	if len(ops) != 0 {
		t.Errorf("Expected 0 ops for empty files, got %d", len(ops))
	}
}

func TestLineDiff_AddToEmpty(t *testing.T) {
	old := []byte("")
	new := []byte("line1\nline2\n")
	ops := LineDiff(old, new)

	if len(ops) != 1 {
		t.Fatalf("Expected 1 op, got %d", len(ops))
	}

	if ops[0].Tag != "insert" {
		t.Errorf("Expected 'insert', got %s", ops[0].Tag)
	}

	if ops[0].J2-ops[0].J1 != 2 {
		t.Errorf("Expected 2 inserted lines, got %d", ops[0].J2-ops[0].J1)
	}
}

func TestLineDiff_DeleteAll(t *testing.T) {
	old := []byte("line1\nline2\n")
	new := []byte("")
	ops := LineDiff(old, new)

	if len(ops) != 1 {
		t.Fatalf("Expected 1 op, got %d", len(ops))
	}

	if ops[0].Tag != "delete" {
		t.Errorf("Expected 'delete', got %s", ops[0].Tag)
	}

	if ops[0].I2-ops[0].I1 != 2 {
		t.Errorf("Expected 2 deleted lines, got %d", ops[0].I2-ops[0].I1)
	}
}

func TestLineDiff_MultipleChanges(t *testing.T) {
	old := []byte("line1\nline2\nline3\nline4\n")
	new := []byte("line1\nmodified2\nline3\nline5\n")
	ops := LineDiff(old, new)

	// Should detect two separate replacements
	foundReplace := false
	for _, op := range ops {
		if op.Tag == "replace" {
			foundReplace = true
		}
	}

	if !foundReplace {
		t.Errorf("Expected at least one 'replace' operation")
	}
}
