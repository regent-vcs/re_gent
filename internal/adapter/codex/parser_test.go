package codex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseRolloutFile_BasicTurn(t *testing.T) {
	tmpDir := t.TempDir()
	rolloutPath := filepath.Join(tmpDir, "rollout.jsonl")
	content := strings.Join([]string{
		`{"timestamp":"2026-05-14T01:00:00Z","type":"session_meta","payload":{"id":"abc123","cwd":"C:\\proj","originator":"Codex Desktop"}}`,
		`{"timestamp":"2026-05-14T01:00:01Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-05-14T01:00:02Z","type":"turn_context","payload":{"turn_id":"turn-1","cwd":"C:\\proj"}}`,
		`{"timestamp":"2026-05-14T01:00:03Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"output_text","text":"fix bug"}]}}`,
		`{"timestamp":"2026-05-14T01:00:04Z","type":"response_item","payload":{"type":"custom_tool_call","name":"apply_patch","call_id":"call-1","input":"*** Begin Patch\n*** Add File: note.txt\n+hello\n*** End Patch\n"}}`,
		`{"timestamp":"2026-05-14T01:00:05Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"call-1","output":"{\"output\":\"ok\"}"}}`,
		`{"timestamp":"2026-05-14T01:00:06Z","type":"event_msg","payload":{"type":"patch_apply_end","call_id":"call-1","stdout":"patched","stderr":"","success":true}}`,
		`{"timestamp":"2026-05-14T01:00:07Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}`,
		`{"timestamp":"2026-05-14T01:00:08Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
	}, "\n")

	if err := os.WriteFile(rolloutPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	parsed, err := parseRolloutFile(rolloutPath, 0)
	if err != nil {
		t.Fatalf("parse rollout: %v", err)
	}

	if parsed.Session.SessionID != "codex:abc123" {
		t.Fatalf("unexpected session id: %s", parsed.Session.SessionID)
	}
	if len(parsed.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(parsed.Turns))
	}

	turn := parsed.Turns[0]
	if turn.TurnID != "turn-1" {
		t.Fatalf("unexpected turn id: %s", turn.TurnID)
	}
	if len(turn.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(turn.ToolCalls))
	}
	if !turn.ToolCalls[0].SupportsReplay {
		t.Fatalf("expected apply_patch to be replayable")
	}
	if len(turn.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(turn.Messages))
	}
	if !strings.Contains(string(turn.ToolCalls[0].Output), "patch_apply_end") {
		t.Fatalf("expected patch_apply_end metadata to be merged into output")
	}
}

func TestParseRolloutFile_BootstrapFromOffsetWithLargeSessionMeta(t *testing.T) {
	tmpDir := t.TempDir()
	rolloutPath := filepath.Join(tmpDir, "rollout.jsonl")
	largeText := strings.Repeat("x", 128*1024)
	content := strings.Join([]string{
		`{"timestamp":"2026-05-14T01:00:00Z","type":"session_meta","payload":{"id":"abc123","cwd":"C:\\proj","originator":"Codex Desktop","base_instructions":{"text":"` + largeText + `"}}}`,
		`{"timestamp":"2026-05-14T01:00:01Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-05-14T01:00:02Z","type":"turn_context","payload":{"turn_id":"turn-1","cwd":"C:\\proj"}}`,
		`{"timestamp":"2026-05-14T01:00:03Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}}`,
		`{"timestamp":"2026-05-14T01:00:04Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
	}, "\n")

	if err := os.WriteFile(rolloutPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	offset := int64(strings.Index(content, `{"timestamp":"2026-05-14T01:00:01Z"`))
	if offset <= 0 {
		t.Fatalf("failed to compute offset")
	}

	parsed, err := parseRolloutFile(rolloutPath, offset)
	if err != nil {
		t.Fatalf("parse rollout from offset: %v", err)
	}

	if parsed.Session.SessionID != "codex:abc123" {
		t.Fatalf("unexpected session id: %s", parsed.Session.SessionID)
	}
	if parsed.Session.ProjectCWD != filepath.Clean(`C:\proj`) {
		t.Fatalf("unexpected project cwd: %s", parsed.Session.ProjectCWD)
	}
	if len(parsed.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(parsed.Turns))
	}
}

func TestParseRolloutFile_WithUTF8BOM(t *testing.T) {
	tmpDir := t.TempDir()
	rolloutPath := filepath.Join(tmpDir, "rollout.jsonl")
	content := "\uFEFF" + strings.Join([]string{
		`{"timestamp":"2026-05-14T01:00:00Z","type":"session_meta","payload":{"id":"abc123","cwd":"C:\\proj","originator":"Codex Desktop"}}`,
		`{"timestamp":"2026-05-14T01:00:01Z","type":"event_msg","payload":{"type":"task_started","turn_id":"turn-1"}}`,
		`{"timestamp":"2026-05-14T01:00:02Z","type":"event_msg","payload":{"type":"task_complete","turn_id":"turn-1"}}`,
	}, "\n")

	if err := os.WriteFile(rolloutPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	parsed, err := parseRolloutFile(rolloutPath, 0)
	if err != nil {
		t.Fatalf("parse rollout: %v", err)
	}
	if parsed.Session.SessionID != "codex:abc123" {
		t.Fatalf("unexpected session id: %s", parsed.Session.SessionID)
	}
}
