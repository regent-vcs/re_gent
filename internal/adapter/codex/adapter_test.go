package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestRunImport_BasicApplyPatchTurn(t *testing.T) {
	projectRoot := t.TempDir()
	codexHome := filepath.Join(t.TempDir(), ".codex")
	rolloutDir := filepath.Join(codexHome, "sessions", "2026", "05", "14")
	if err := os.MkdirAll(rolloutDir, 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}

	s, err := store.Init(projectRoot)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	_ = s

	quotedProjectRoot, err := json.Marshal(filepath.Clean(projectRoot))
	if err != nil {
		t.Fatalf("marshal project root: %v", err)
	}

	rolloutPath := filepath.Join(rolloutDir, "rollout.jsonl")
	content := "" +
		"{\"timestamp\":\"2026-05-14T01:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"abc123\",\"cwd\":" + string(quotedProjectRoot) + ",\"originator\":\"Codex Desktop\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:01Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"turn_id\":\"turn-1\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:02Z\",\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"turn-1\",\"cwd\":" + string(quotedProjectRoot) + "}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:03Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"done\"}]}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:04Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call\",\"name\":\"apply_patch\",\"call_id\":\"call-1\",\"input\":\"*** Begin Patch\\n*** Add File: note.txt\\n+hello\\n*** End Patch\\n\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:05Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"patch_apply_end\",\"call_id\":\"call-1\",\"turn_id\":\"turn-1\",\"stdout\":\"patched\",\"stderr\":\"\",\"success\":true}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:06Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call_output\",\"call_id\":\"call-1\",\"output\":\"{\\\"output\\\":\\\"ok\\\"}\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:07Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\",\"turn_id\":\"turn-1\"}}\n"
	if err := os.WriteFile(rolloutPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	if err := RunImport(Options{
		ProjectRoot: projectRoot,
		CodexHome:   codexHome,
		ChangesOnly: true,
	}); err != nil {
		t.Fatalf("run import: %v", err)
	}

	s, err = store.Open(filepath.Join(projectRoot, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].ID != "codex:abc123" {
		t.Fatalf("unexpected session id: %s", sessions[0].ID)
	}

	steps, err := idx.ListSteps("codex:abc123", 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	step, err := s.ReadStep(steps[0].Hash)
	if err != nil {
		t.Fatalf("read step: %v", err)
	}
	tree, err := s.ReadTree(step.Tree)
	if err != nil {
		t.Fatalf("read tree: %v", err)
	}
	entry := tree.FindEntry("note.txt")
	if entry == nil {
		t.Fatalf("expected note.txt in tree")
	}
	contentBytes, err := s.ReadBlob(entry.Blob)
	if err != nil {
		t.Fatalf("read blob: %v", err)
	}
	if string(contentBytes) != "hello" {
		t.Fatalf("unexpected note.txt content: %q", string(contentBytes))
	}

	if err := RunImport(Options{
		ProjectRoot: projectRoot,
		CodexHome:   codexHome,
		ChangesOnly: true,
	}); err != nil {
		t.Fatalf("run import second time: %v", err)
	}

	stepsAfter, err := idx.ListSteps("codex:abc123", 10)
	if err != nil {
		t.Fatalf("list steps after rerun: %v", err)
	}
	if len(stepsAfter) != 1 {
		t.Fatalf("expected 1 step after rerun, got %d", len(stepsAfter))
	}
}

func TestClassifyImportTurn_ReadOnlyShellCommand(t *testing.T) {
	turn := Turn{
		TurnID: "turn-readonly",
		ToolCalls: []ToolCall{
			{
				ToolName: "shell_command",
				Input:    []byte(`{"command":"Get-Content README.md"}`),
			},
		},
	}

	if got := classifyImportTurn(turn); got != importTurnReadOnly {
		t.Fatalf("expected read-only turn classification, got %v", got)
	}
}

func TestRunOnceWithRuntime_WatchWarnsOnConcurrentSessions(t *testing.T) {
	projectRoot := t.TempDir()
	codexHome := filepath.Join(t.TempDir(), ".codex")
	rolloutDir := filepath.Join(codexHome, "sessions", "2026", "05", "14")
	if err := os.MkdirAll(rolloutDir, 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}

	if _, err := store.Init(projectRoot); err != nil {
		t.Fatalf("init store: %v", err)
	}

	quotedProjectRoot, err := json.Marshal(filepath.Clean(projectRoot))
	if err != nil {
		t.Fatalf("marshal project root: %v", err)
	}

	rolloutA := "" +
		"{\"timestamp\":\"2026-05-14T01:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"session-a\",\"cwd\":" + string(quotedProjectRoot) + ",\"originator\":\"Codex Desktop\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:01Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"turn_id\":\"turn-a\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:02Z\",\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"turn-a\",\"cwd\":" + string(quotedProjectRoot) + "}}\n"
	rolloutB := "" +
		"{\"timestamp\":\"2026-05-14T01:00:03Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"session-b\",\"cwd\":" + string(quotedProjectRoot) + ",\"originator\":\"Codex Desktop\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:04Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"turn_id\":\"turn-b\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:05Z\",\"type\":\"turn_context\",\"payload\":{\"turn_id\":\"turn-b\",\"cwd\":" + string(quotedProjectRoot) + "}}\n"
	if err := os.WriteFile(filepath.Join(rolloutDir, "session-a.jsonl"), []byte(rolloutA), 0o644); err != nil {
		t.Fatalf("write rollout A: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rolloutDir, "session-b.jsonl"), []byte(rolloutB), 0o644); err != nil {
		t.Fatalf("write rollout B: %v", err)
	}

	var warnings []string
	opts := normalizeOptions(Options{
		ProjectRoot: projectRoot,
		CodexHome:   codexHome,
		WatchMode:   true,
		ChangesOnly: true,
		Stderr: func(msg string) {
			warnings = append(warnings, msg)
		},
	})

	runtime := &watchRuntime{}
	if err := runOnceWithRuntime(opts, runtime); err != nil {
		t.Fatalf("first runOnceWithRuntime: %v", err)
	}

	if !containsWarning(warnings, "multiple active Codex sessions detected") {
		t.Fatalf("expected concurrent session warning, got %v", warnings)
	}
	if !containsWarning(warnings, "codex:session-a, codex:session-b") {
		t.Fatalf("expected warning to list both sessions, got %v", warnings)
	}

	warnings = nil
	if err := runOnceWithRuntime(opts, runtime); err != nil {
		t.Fatalf("second runOnceWithRuntime: %v", err)
	}
	if containsWarning(warnings, "multiple active Codex sessions detected") {
		t.Fatalf("expected duplicate concurrent warning to be suppressed, got %v", warnings)
	}
}

func TestRunImport_BlockedBaselineSkipsLaterReplayableTurns(t *testing.T) {
	projectRoot := t.TempDir()
	codexHome := filepath.Join(t.TempDir(), ".codex")
	rolloutDir := filepath.Join(codexHome, "sessions", "2026", "05", "14")
	if err := os.MkdirAll(rolloutDir, 0o755); err != nil {
		t.Fatalf("mkdir rollout dir: %v", err)
	}

	if _, err := store.Init(projectRoot); err != nil {
		t.Fatalf("init store: %v", err)
	}

	quotedProjectRoot, err := json.Marshal(filepath.Clean(projectRoot))
	if err != nil {
		t.Fatalf("marshal project root: %v", err)
	}

	rollout := "" +
		"{\"timestamp\":\"2026-05-14T01:00:00Z\",\"type\":\"session_meta\",\"payload\":{\"id\":\"blocked\",\"cwd\":" + string(quotedProjectRoot) + ",\"originator\":\"Codex Desktop\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:01Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"turn_id\":\"turn-1\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:02Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"function_call\",\"name\":\"shell_command\",\"call_id\":\"call-1\",\"arguments\":\"{\\\"command\\\":\\\"Set-Content note.txt 'hello'\\\"}\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:03Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"function_call_output\",\"call_id\":\"call-1\",\"output\":\"done\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:04Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\",\"turn_id\":\"turn-1\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:05Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_started\",\"turn_id\":\"turn-2\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:06Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call\",\"name\":\"apply_patch\",\"call_id\":\"call-2\",\"input\":\"*** Begin Patch\\n*** Add File: note.txt\\n+hello\\n*** End Patch\\n\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:07Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"patch_apply_end\",\"call_id\":\"call-2\",\"turn_id\":\"turn-2\",\"stdout\":\"patched\",\"stderr\":\"\",\"success\":true}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:08Z\",\"type\":\"response_item\",\"payload\":{\"type\":\"custom_tool_call_output\",\"call_id\":\"call-2\",\"output\":\"{\\\"output\\\":\\\"ok\\\"}\"}}\n" +
		"{\"timestamp\":\"2026-05-14T01:00:09Z\",\"type\":\"event_msg\",\"payload\":{\"type\":\"task_complete\",\"turn_id\":\"turn-2\"}}\n"
	if err := os.WriteFile(filepath.Join(rolloutDir, "blocked.jsonl"), []byte(rollout), 0o644); err != nil {
		t.Fatalf("write rollout: %v", err)
	}

	var warnings []string
	if err := RunImport(Options{
		ProjectRoot: projectRoot,
		CodexHome:   codexHome,
		ChangesOnly: true,
		Stderr: func(msg string) {
			warnings = append(warnings, msg)
		},
	}); err != nil {
		t.Fatalf("run import: %v", err)
	}

	s, err := store.Open(filepath.Join(projectRoot, ".regent"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected blocked import to create no sessions, got %d", len(sessions))
	}

	if !containsWarning(warnings, "workspace changes cannot be reconstructed from rollout history") {
		t.Fatalf("expected blocked-baseline warning, got %v", warnings)
	}
	if !containsWarning(warnings, "earlier non-replayable turns left the session baseline unknown") {
		t.Fatalf("expected later-turn skip warning, got %v", warnings)
	}
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
