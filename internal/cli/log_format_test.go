package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// ---- pure helper tests ----

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"milliseconds", 500 * time.Millisecond, "500ms"},
		{"seconds", 5 * time.Second, "5s"},
		{"minutes_seconds", 2*time.Minute + 30*time.Second, "2m30s"},
		{"zero", 0, "0ms"},
		{"exact_second", 1 * time.Second, "1s"},
		{"exact_minute", 1 * time.Minute, "1m0s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatFileStat(t *testing.T) {
	tests := []struct {
		name string
		fd   FileDiff
		want string
	}{
		{"added", FileDiff{Status: "added", Additions: 10}, ""}, // has DimText
		{"modified", FileDiff{Status: "modified", Additions: 3, Deletions: 2}, ""},
		{"deleted", FileDiff{Status: "deleted", Deletions: 5}, ""},
		{"binary", FileDiff{Status: "modified", IsBinary: true}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFileStat(tt.fd)
			// Style wraps output; verify non-empty and contains expected indicators
			if got == "" {
				t.Errorf("formatFileStat(%+v) returned empty string", tt.fd)
			}
			if tt.fd.IsBinary && !strings.Contains(got, "binary") {
				t.Errorf("formatFileStat() = %q, want binary indicator", got)
			}
			if tt.fd.Status == "added" && !strings.Contains(got, "+") {
				t.Errorf("formatFileStat() = %q, want + prefix", got)
			}
			if tt.fd.Status == "deleted" && !strings.Contains(got, "-") {
				t.Errorf("formatFileStat() = %q, want - prefix", got)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "1234567890", 10, "1234567890"},
		{"long", "1234567890123", 10, "1234567..."},
		{"empty", "", 10, ""},
		{"maxLen_3", "abcd", 3, "..."}, // maxLen==3 => s[:0] + "..."
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.s, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestGetSummary(t *testing.T) {
	ts := time.Date(2026, 5, 2, 14, 30, 21, 0, time.UTC)
	tests := []struct {
		name string
		step EnrichedStep
		want string
	}{
		{
			name: "files_present",
			step: EnrichedStep{
				StepInfo: index.StepInfo{Hash: "aaaa", ToolName: "Write", Timestamp: ts},
				Files:    []string{"src/main.go"},
			},
			want: "src/main.go",
		},
		{
			name: "bash_with_command",
			step: EnrichedStep{
				StepInfo: index.StepInfo{Hash: "bbbb", ToolName: "Bash", Timestamp: ts},
				Args:     json.RawMessage(`{"command":"go build ./..."}`),
			},
			want: "go build ./...",
		},
		{
			name: "bash_no_args",
			step: EnrichedStep{
				StepInfo: index.StepInfo{Hash: "cccc", ToolName: "Bash", Timestamp: ts},
			},
			want: "",
		},
		{
			name: "no_files_no_bash",
			step: EnrichedStep{
				StepInfo: index.StepInfo{Hash: "dddd", ToolName: "Read", Timestamp: ts},
			},
			want: "",
		},
		{
			name: "empty_step",
			step: EnrichedStep{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSummary(tt.step)
			if got != tt.want {
				t.Errorf("getSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStepToolLabel(t *testing.T) {
	tests := []struct {
		name string
		step EnrichedStep
		want string
	}{
		{"single_cause", EnrichedStep{StepInfo: index.StepInfo{ToolName: "Write"}, Causes: []EnrichedCause{{}}}, "Write"},
		{"multi_cause", EnrichedStep{StepInfo: index.StepInfo{ToolName: "Bash"}, Causes: []EnrichedCause{{}, {}, {}}}, "Bash +2"},
		{"zero_causes", EnrichedStep{StepInfo: index.StepInfo{ToolName: "Read"}}, "Read"},
		{"nil_causes", EnrichedStep{StepInfo: index.StepInfo{ToolName: "Edit"}, Causes: nil}, "Edit"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stepToolLabel(tt.step)
			if got != tt.want {
				t.Errorf("stepToolLabel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrintWarnings(t *testing.T) {
	// Empty warnings produce no output
	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		printWarnings(&buf, nil)
		if buf.Len() != 0 {
			t.Errorf("printWarnings with nil produced output: %q", buf.String())
		}
	})

	t.Run("with_warnings", func(t *testing.T) {
		var buf bytes.Buffer
		printWarnings(&buf, []string{"warning one", "warning two"})
		out := buf.String()
		if !strings.Contains(out, "warning one") {
			t.Errorf("output missing first warning: %q", out)
		}
		if !strings.Contains(out, "warning two") {
			t.Errorf("output missing second warning: %q", out)
		}
	})
}

// ---- helper for building test steps ----

func makeTestStep(hash, toolName string, files []string, args json.RawMessage, ts time.Time) EnrichedStep {
	return EnrichedStep{
		StepInfo: index.StepInfo{Hash: store.Hash(hash), ToolName: toolName, Timestamp: ts},
		Files:    files,
		Args:     args,
	}
}

// ---- formatter tests ----

func TestDefaultFormatter_Format(t *testing.T) {
	ts := time.Date(2026, 5, 2, 14, 30, 21, 0, time.UTC)
	ts2 := ts.Add(-30 * time.Second)

	t.Run("empty_steps", func(t *testing.T) {
		var buf bytes.Buffer
		f := &DefaultFormatter{}
		if err := f.Format([]EnrichedStep{}, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})

	t.Run("basic_with_files", func(t *testing.T) {
		var buf bytes.Buffer
		f := &DefaultFormatter{}
		steps := []EnrichedStep{
			makeTestStep("aaaaaaaabbbbbbbb", "Write", []string{"main.go"}, json.RawMessage(`{"file_path":"main.go"}`), ts),
			makeTestStep("ccccccccdddddddd", "Bash", nil, json.RawMessage(`{"command":"ls"}`), ts2),
		}
		steps[0].FileDiffs = []FileDiff{{Path: "main.go", Status: "added", Additions: 10}}
		steps[0].Duration = 30 * time.Second

		if err := f.Format(steps, "sess-1", false, true, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "sess-1") {
			t.Errorf("output missing session id: %s", out)
		}
		if !strings.Contains(out, "Write") {
			t.Errorf("output missing tool name: %s", out)
		}
		if !strings.Contains(out, "main.go") {
			t.Errorf("output missing file path: %s", out)
		}
	})

	t.Run("conversation_mode", func(t *testing.T) {
		var buf bytes.Buffer
		f := &DefaultFormatter{}
		steps := []EnrichedStep{
			makeTestStep("eeeeeeeef", "Write", []string{"f.txt"}, nil, ts),
		}
		if err := f.Format(steps, "sess-1", true, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		// Conversation mode should produce some output (even if no messages)
		if buf.Len() == 0 {
			t.Error("expected non-empty output for conversation mode")
		}
	})

	t.Run("graph_prefix", func(t *testing.T) {
		var buf bytes.Buffer
		f := &DefaultFormatter{}
		steps := []EnrichedStep{
			{StepInfo: index.StepInfo{Hash: "ffffaaaabbbb", ToolName: "Read", Timestamp: ts}, GraphPrefix: "* "},
		}
		if err := f.Format(steps, "sess-2", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "*") {
			t.Errorf("output missing graph prefix: %s", out)
		}
	})
}

func TestOnelineFormatter_Format(t *testing.T) {
	ts := time.Date(2026, 5, 2, 14, 30, 21, 0, time.UTC)

	t.Run("basic", func(t *testing.T) {
		var buf bytes.Buffer
		f := &OnelineFormatter{}
		steps := []EnrichedStep{
			makeTestStep("aaaabbbb", "Write", []string{"f.go"}, nil, ts),
			makeTestStep("ccccdddd", "Read", nil, nil, ts),
		}
		if err := f.Format(steps, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
		}
		if !strings.Contains(lines[0], "Write") {
			t.Errorf("line 0 missing tool: %q", lines[0])
		}
		if !strings.Contains(lines[1], "Read") {
			t.Errorf("line 1 missing tool: %q", lines[1])
		}
	})

	t.Run("with_file_stats", func(t *testing.T) {
		var buf bytes.Buffer
		f := &OnelineFormatter{}
		steps := []EnrichedStep{
			{
				StepInfo: index.StepInfo{Hash: "aaaabbbb", ToolName: "Edit", Timestamp: ts},
				FileDiffs: []FileDiff{
					{Status: "modified", Additions: 5, Deletions: 3},
					{Status: "modified", Additions: 2, Deletions: 1},
				},
			},
		}
		if err := f.Format(steps, "sess-1", false, true, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "+7 -4") {
			t.Errorf("output missing combined stat: %q", out)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		f := &OnelineFormatter{}
		if err := f.Format([]EnrichedStep{}, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("expected empty output, got %q", buf.String())
		}
	})
}

func TestJSONFormatter_Format(t *testing.T) {
	ts := time.Date(2026, 5, 2, 14, 30, 21, 0, time.UTC)

	t.Run("basic", func(t *testing.T) {
		var buf bytes.Buffer
		f := &JSONFormatter{}
		steps := []EnrichedStep{
			makeTestStep("aaaabbbbccccdddd", "Write", []string{"f.go"}, json.RawMessage(`{"file_path":"f.go"}`), ts),
		}
		if err := f.Format(steps, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}

		var out struct {
			SessionID string `json:"session_id"`
			Steps     []struct {
				Hash string `json:"hash"`
				Tool string `json:"tool"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("output not valid JSON: %v\n%s", err, buf.String())
		}
		if out.SessionID != "sess-1" {
			t.Errorf("session_id = %q, want sess-1", out.SessionID)
		}
		if len(out.Steps) != 1 {
			t.Fatalf("expected 1 step, got %d", len(out.Steps))
		}
		if out.Steps[0].Hash != "aaaabbbbccccdddd" {
			t.Errorf("hash = %q, want aaaabbbbccccdddd", out.Steps[0].Hash)
		}
		if out.Steps[0].Tool != "Write" {
			t.Errorf("tool = %q, want Write", out.Steps[0].Tool)
		}
	})

	t.Run("with_files_and_conversation", func(t *testing.T) {
		var buf bytes.Buffer
		f := &JSONFormatter{}
		steps := []EnrichedStep{
			{
				StepInfo:  index.StepInfo{Hash: "aaaabbbb", ToolName: "Edit", Timestamp: ts},
				FileDiffs: []FileDiff{{Path: "x.go", Status: "modified", Additions: 1}},
				Messages:  []json.RawMessage{json.RawMessage(`{"type":"user"}`)},
			},
		}
		if err := f.Format(steps, "sess-x", true, true, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		var out struct {
			Steps []struct {
				FileDiffs []FileDiff        `json:"file_diffs"`
				Messages  []json.RawMessage `json:"messages"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		if len(out.Steps[0].FileDiffs) != 1 {
			t.Errorf("expected 1 file diff, got %d", len(out.Steps[0].FileDiffs))
		}
		if len(out.Steps[0].Messages) != 1 {
			t.Errorf("expected 1 message, got %d", len(out.Steps[0].Messages))
		}
	})

	t.Run("with_causes", func(t *testing.T) {
		var buf bytes.Buffer
		f := &JSONFormatter{}
		steps := []EnrichedStep{
			{
				StepInfo: index.StepInfo{Hash: "aaaa", ToolName: "MultiTool", Timestamp: ts},
				Causes: []EnrichedCause{
					{Cause: store.Cause{ToolName: "Write", ToolUseID: "t1"}},
					{Cause: store.Cause{ToolName: "Bash", ToolUseID: "t2"}},
				},
			},
		}
		if err := f.Format(steps, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		var out struct {
			Steps []struct {
				Causes []struct {
					Tool      string `json:"tool"`
					ToolUseID string `json:"tool_use_id"`
				} `json:"causes"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		if len(out.Steps[0].Causes) != 2 {
			t.Errorf("expected 2 causes, got %d", len(out.Steps[0].Causes))
		}
	})

	t.Run("with_warnings", func(t *testing.T) {
		var buf bytes.Buffer
		f := &JSONFormatter{}
		steps := []EnrichedStep{
			{
				StepInfo: index.StepInfo{Hash: "wwww", ToolName: "Bash", Timestamp: ts},
				Warnings: []string{"disk full"},
			},
		}
		if err := f.Format(steps, "s", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		var out struct {
			Steps []struct {
				Warnings []string `json:"warnings"`
			} `json:"steps"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		if len(out.Steps[0].Warnings) != 1 || out.Steps[0].Warnings[0] != "disk full" {
			t.Errorf("warnings not preserved: %v", out.Steps[0].Warnings)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		f := &JSONFormatter{}
		if err := f.Format([]EnrichedStep{}, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		var out struct {
			Steps []interface{} `json:"steps"`
		}
		if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
			t.Fatalf("output not valid JSON: %v", err)
		}
		if len(out.Steps) != 0 {
			t.Errorf("expected empty steps, got %d", len(out.Steps))
		}
	})
}

func TestStatFormatter_Format(t *testing.T) {
	ts := time.Date(2026, 5, 2, 14, 30, 21, 0, time.UTC)

	t.Run("basic", func(t *testing.T) {
		var buf bytes.Buffer
		f := &StatFormatter{}
		steps := []EnrichedStep{
			makeTestStep("aaaabbbb", "Write", []string{"f.go"}, nil, ts),
		}
		if err := f.Format(steps, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "sess-1") {
			t.Errorf("output missing session id: %s", out)
		}
		if !strings.Contains(out, "f.go") {
			t.Errorf("output missing file: %s", out)
		}
	})

	t.Run("with_file_diffs", func(t *testing.T) {
		var buf bytes.Buffer
		f := &StatFormatter{}
		steps := []EnrichedStep{
			{
				StepInfo: index.StepInfo{Hash: "aaaabbbb", ToolName: "Edit", Timestamp: ts},
				Files:    []string{"main.go"},
				FileDiffs: []FileDiff{
					{Path: "main.go", Status: "modified", Additions: 3, Deletions: 1},
				},
			},
		}
		if err := f.Format(steps, "sess-1", false, true, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "main.go") {
			t.Errorf("output missing file path: %s", out)
		}
	})

	t.Run("bash_with_command", func(t *testing.T) {
		var buf bytes.Buffer
		f := &StatFormatter{}
		steps := []EnrichedStep{
			{
				StepInfo: index.StepInfo{Hash: "bbbbcccc", ToolName: "Bash", Timestamp: ts},
				Args:     json.RawMessage(`{"command":"echo hello"}`),
			},
		}
		if err := f.Format(steps, "sess-1", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "command") {
			t.Errorf("output missing command indicator: %s", out)
		}
	})

	t.Run("empty", func(t *testing.T) {
		var buf bytes.Buffer
		f := &StatFormatter{}
		if err := f.Format([]EnrichedStep{}, "s", false, false, &buf); err != nil {
			t.Fatalf("Format() returned error: %v", err)
		}
		// Should still have session header
		out := buf.String()
		if !strings.Contains(out, "s") {
			t.Errorf("output missing session id: %s", out)
		}
	})
}
