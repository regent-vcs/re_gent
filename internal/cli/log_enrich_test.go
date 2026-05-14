package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

func TestRawJSONForOutput_PreservesValidJSON(t *testing.T) {
	raw := rawJSONForOutput([]byte(`{"ok":true}`))
	if string(raw) != `{"ok":true}` {
		t.Fatalf("rawJSONForOutput() = %s", raw)
	}
}

func TestRawJSONForOutput_WrapsLegacyPlainText(t *testing.T) {
	raw := rawJSONForOutput([]byte(`ok`))
	if string(raw) != `"ok"` {
		t.Fatalf("rawJSONForOutput() = %s, want JSON string", raw)
	}

	var decoded string
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("wrapped output should be valid JSON: %v", err)
	}
	if decoded != "ok" {
		t.Fatalf("decoded output = %q", decoded)
	}
}

func TestEnrichSteps_ReportsMissingToolBlobWarning(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	idx, err := index.Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	fileBlob, err := s.WriteBlob([]byte("hello\n"))
	if err != nil {
		t.Fatalf("write file blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "hello.txt", Blob: fileBlob}}}
	treeHash, err := s.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	step := &store.Step{
		Tree:           treeHash,
		SessionID:      "codex_cli:session",
		Origin:         "codex_cli",
		TurnID:         "turn-1",
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-1", ArgsBlob: "aa"},
		TimestampNanos: 1,
	}
	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("index step: %v", err)
	}

	steps, err := idx.ListSteps("codex_cli:session", 10)
	if err != nil {
		t.Fatalf("list steps: %v", err)
	}
	enriched, err := enrichSteps(s, steps, false, false)
	if err != nil {
		t.Fatalf("enrich steps: %v", err)
	}
	if len(enriched) != 1 {
		t.Fatalf("enriched steps = %d, want 1", len(enriched))
	}
	if len(enriched[0].Warnings) != 1 || !strings.Contains(enriched[0].Warnings[0], "read tool args blob") {
		t.Fatalf("expected missing args blob warning, got %#v", enriched[0].Warnings)
	}
}
