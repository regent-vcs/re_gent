package style

import (
	"strings"
	"testing"
)

func TestSetNoColor(t *testing.T) {
	// Restore state at end of test
	origNoColor := noColor
	defer func() { noColor = origNoColor }()

	SetNoColor(true)
	if !noColor {
		t.Error("SetNoColor(true) did not set noColor to true")
	}

	// In noColor mode, styling should be disabled (return text unchanged)
	txt := BoldText("hello")
	if txt != "hello" {
		t.Errorf("expected plain 'hello' when noColor is true, got %q", txt)
	}

	SetNoColor(false)
	if noColor {
		t.Error("SetNoColor(false) did not set noColor to false")
	}

	// In color mode, styling should be enabled
	txt = BoldText("hello")
	if !strings.Contains(txt, "\033[1m") {
		t.Errorf("expected styled text when noColor is false, got %q", txt)
	}
}

func TestToolName(t *testing.T) {
	origNoColor := noColor
	defer func() { noColor = origNoColor }()
	SetNoColor(false)

	tests := []struct {
		name     string
		expected string
	}{
		{"Edit", Amber256},
		{"Write", Green256},
		{"Bash", Blue256},
		{"Read", Blue256},
		{"Unknown", Blue256},
	}

	for _, tt := range tests {
		got := ToolName(tt.name)
		if !strings.Contains(got, tt.expected) {
			t.Errorf("ToolName(%q) = %q, expected color code %q", tt.name, got, tt.expected)
		}
		if !strings.Contains(got, tt.name) {
			t.Errorf("ToolName(%q) = %q, expected it to contain tool name", tt.name, got)
		}
	}

	// Empty tool name should return empty
	if got := ToolName(""); got != "" {
		t.Errorf("ToolName(\"\") = %q, expected empty", got)
	}
}

func TestFilePath(t *testing.T) {
	origNoColor := noColor
	defer func() { noColor = origNoColor }()
	SetNoColor(false)

	got := FilePath("src/main.go")
	if !strings.Contains(got, Underline) {
		t.Errorf("FilePath() = %q, expected it to contain Underline code", got)
	}
	if !strings.Contains(got, "src/main.go") {
		t.Errorf("FilePath() = %q, expected it to contain file path", got)
	}
}

func TestAddition(t *testing.T) {
	origNoColor := noColor
	defer func() { noColor = origNoColor }()
	SetNoColor(false)

	got := Addition("+10")
	if !strings.Contains(got, Green256) {
		t.Errorf("Addition() = %q, expected green color", got)
	}
}

func TestDeletion(t *testing.T) {
	origNoColor := noColor
	defer func() { noColor = origNoColor }()
	SetNoColor(false)

	got := Deletion("-5")
	if !strings.Contains(got, Red256) {
		t.Errorf("Deletion() = %q, expected red color", got)
	}
}

func TestBoldHash(t *testing.T) {
	origNoColor := noColor
	defer func() { noColor = origNoColor }()
	SetNoColor(false)

	got := BoldHash("abc123ef")
	if !strings.Contains(got, Bold) {
		t.Errorf("BoldHash() = %q, expected bold code", got)
	}
}
