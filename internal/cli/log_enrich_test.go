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

// ---- extractFilesFromToolArgs tests ----

func TestExtractFilesFromToolArgs_Write(t *testing.T) {
	args := json.RawMessage(`{"file_path": "src/main.go"}`)
	files := extractFilesFromToolArgs("Write", args)
	if len(files) != 1 || files[0] != "src/main.go" {
		t.Errorf("extractFilesFromToolArgs(Write) = %v, want [src/main.go]", files)
	}
}

func TestExtractFilesFromToolArgs_Edit(t *testing.T) {
	args := json.RawMessage(`{"file_path": "/absolute/path/file.go"}`)
	files := extractFilesFromToolArgs("Edit", args)
	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d: %v", len(files), files)
	}
}

func TestExtractFilesFromToolArgs_Read(t *testing.T) {
	args := json.RawMessage(`{"file_path": "README.md"}`)
	files := extractFilesFromToolArgs("Read", args)
	if len(files) != 1 || files[0] != "README.md" {
		t.Errorf("extractFilesFromToolArgs(Read) = %v, want [README.md]", files)
	}
}

func TestExtractFilesFromToolArgs_Bash(t *testing.T) {
	args := json.RawMessage(`{"command": "rm -rf /tmp"}`)
	files := extractFilesFromToolArgs("Bash", args)
	if len(files) != 0 {
		t.Errorf("extractFilesFromToolArgs(Bash) should return empty, got %v", files)
	}
}

func TestExtractFilesFromToolArgs_Unknown(t *testing.T) {
	args := json.RawMessage(`{"file_path": "a.txt", "path": "b.txt", "filename": "c.txt"}`)
	files := extractFilesFromToolArgs("UnknownTool", args)
	if len(files) != 3 {
		t.Errorf("extractFilesFromToolArgs(UnknownTool) = %v, want 3 files", files)
	}
}

func TestExtractFilesFromToolArgs_EmptyArgs(t *testing.T) {
	files := extractFilesFromToolArgs("Write", json.RawMessage{})
	if len(files) != 0 {
		t.Errorf("empty args should return empty, got %v", files)
	}
}

func TestExtractFilesFromToolArgs_NullArgs(t *testing.T) {
	files := extractFilesFromToolArgs("Write", json.RawMessage("null"))
	if len(files) != 0 {
		t.Errorf("null args should return empty, got %v", files)
	}
}

func TestExtractFilesFromToolArgs_InvalidJSON(t *testing.T) {
	files := extractFilesFromToolArgs("Write", json.RawMessage("not json"))
	if len(files) != 0 {
		t.Errorf("invalid JSON args should return empty, got %v", files)
	}
}

func TestExtractFilesFromToolArgs_WildcardPaths(t *testing.T) {
	args := json.RawMessage(`{"files": ["a.go", "b.go", "c.go"]}`)
	files := extractFilesFromToolArgs("Glob", args)
	if len(files) != 3 {
		t.Errorf("extractFilesFromToolArgs(Glob) = %v, want 3 files", files)
	}
}

// ---- extractPathFields tests ----

func TestExtractPathFields_AllKeys(t *testing.T) {
	argsMap := map[string]interface{}{
		"file_path": "/foo/bar.go",
		"path":      "baz.go",
		"filename":  "qux.go",
	}
	files := extractPathFields(argsMap)
	if len(files) != 3 {
		t.Errorf("expected 3 files, got %d: %v", len(files), files)
	}
}

func TestExtractPathFields_FilesSlice(t *testing.T) {
	argsMap := map[string]interface{}{
		"files": []interface{}{"a.go", "b.go"},
	}
	files := extractPathFields(argsMap)
	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestExtractPathFields_NoPaths(t *testing.T) {
	argsMap := map[string]interface{}{
		"command": "ls",
		"key":     "value",
	}
	files := extractPathFields(argsMap)
	if len(files) != 0 {
		t.Errorf("expected 0 files, got %d: %v", len(files), files)
	}
}

func TestExtractPathFields_EmptyStrings(t *testing.T) {
	argsMap := map[string]interface{}{
		"file_path": "",
		"path":      "",
	}
	files := extractPathFields(argsMap)
	if len(files) != 0 {
		t.Errorf("empty path strings should be skipped, got %v", files)
	}
}

// ---- makeRelativePath tests ----

func TestMakeRelativePath_AlreadyRelative(t *testing.T) {
	got := makeRelativePath("src/main.go")
	if got != "src/main.go" {
		t.Errorf("makeRelativePath() = %q, want %q", got, "src/main.go")
	}
}

func TestMakeRelativePath_Empty(t *testing.T) {
	got := makeRelativePath("")
	if got != "" {
		t.Errorf("makeRelativePath() = %q, want empty", got)
	}
}

func TestMakeRelativePath_UnderCwd(t *testing.T) {
	got := makeRelativePath("/some/absolute/path")
	if got == "" {
		t.Errorf("makeRelativePath() returned empty for absolute path")
	}
}

func TestMakeRelativePath_WindowsStyle(t *testing.T) {
	got := makeRelativePath(`C:\Users\test\file.go`)
	if !strings.Contains(got, "file.go") {
		t.Errorf("makeRelativePath() = %q", got)
	}
}
