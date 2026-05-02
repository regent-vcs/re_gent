package jsonl

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createTestJSONL(t *testing.T, lines []string) string {
	t.Helper()
	tmpdir := t.TempDir()
	jsonlPath := filepath.Join(tmpdir, "test.jsonl")
	f, err := os.Create(jsonlPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, line := range lines {
		f.WriteString(line + "\n")
	}

	return jsonlPath
}

func TestExtractRange_FromBeginning(t *testing.T) {
	// Write test JSONL
	tmpfile := createTestJSONL(t, []string{
		`{"id":"msg1","type":"user","content":"first"}`,
		`{"id":"msg2","type":"assistant","content":"ok"}`,
		`{"tool_use_id":"tool1","type":"tool_use","name":"Write"}`,
	})

	msgs, err := ExtractRange(tmpfile, "", "tool1")
	if err != nil {
		t.Fatalf("ExtractRange: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(msgs))
	}
}

func TestExtractRange_FromMiddle(t *testing.T) {
	tmpfile := createTestJSONL(t, []string{
		`{"id":"msg1","type":"user","content":"first"}`,
		`{"tool_use_id":"tool1","type":"tool_use"}`,
		`{"id":"msg2","type":"user","content":"second"}`,
		`{"tool_use_id":"tool2","type":"tool_use"}`,
	})

	// Extract from after tool1 up to tool2
	msgs, err := ExtractRange(tmpfile, "tool1", "tool2")
	if err != nil {
		t.Fatalf("ExtractRange: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages (msg2 + tool2), got %d", len(msgs))
	}
}

func TestExtractRange_AfterCompact(t *testing.T) {
	// Simulate /compact: old messages gone
	tmpfile := createTestJSONL(t, []string{
		`{"id":"msg3","type":"user","content":"after compact"}`,
		`{"tool_use_id":"tool3","type":"tool_use"}`,
	})

	// Try to resume from msg1 (doesn't exist)
	msgs, err := ExtractRange(tmpfile, "msg1", "tool3")
	if err != nil {
		t.Fatalf("ExtractRange: %v", err)
	}

	// Should gracefully start from beginning of current JSONL
	if len(msgs) != 2 {
		t.Fatalf("Expected 2 messages (graceful fallback), got %d", len(msgs))
	}
}

func TestMessageID(t *testing.T) {
	tests := []struct {
		name string
		msg  string
		want string
	}{
		{
			name: "id field",
			msg:  `{"id":"msg123","type":"user"}`,
			want: "msg123",
		},
		{
			name: "tool_use_id field",
			msg:  `{"tool_use_id":"tool456","type":"tool_use"}`,
			want: "tool456",
		},
		{
			name: "both fields (id takes precedence)",
			msg:  `{"id":"msg789","tool_use_id":"tool999","type":"assistant"}`,
			want: "msg789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MessageID(json.RawMessage(tt.msg))
			if got != tt.want {
				t.Errorf("MessageID() = %q, want %q", got, tt.want)
			}
		})
	}
}
