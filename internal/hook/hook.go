package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
)

// gitCommitSHAPattern captures the abbreviated SHA git prints on the commit line. The
// branch portion varies and can contain spaces, so it is matched loosely: a normal commit
// is "[main c4e2117]", a first commit is "[master (root-commit) 24832e2]", a detached head
// is "[detached HEAD c4e2117]". [^\]]* stays inside the first bracket group, and the
// leftmost match wins, so a later "[... abc1234]" in the commit message body cannot be
// mistaken for the SHA.
var gitCommitSHAPattern = regexp.MustCompile(`\[[^\]]* ([0-9a-f]{7,40})\]`)

// gitCommitEffect returns a git_commit Effect if the bash call was a git commit and the
// response contains a SHA, otherwise returns nil. The SHA is the join key for LLMTrace's
// /api/attribution endpoint, linking agent-authored commits to their runtime cost impact.
func gitCommitEffect(toolName string, toolInput, toolResponse json.RawMessage) *store.Effect {
	if toolName != "Bash" {
		return nil
	}
	var args struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(toolInput, &args); err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(args.Command)
	if !strings.Contains(trimmed, "git commit") {
		return nil
	}
	var resp struct {
		Output string `json:"output"`
	}
	// tool response may be plain text or JSON with an "output" field
	if err := json.Unmarshal(toolResponse, &resp); err != nil {
		resp.Output = string(toolResponse)
	}
	m := gitCommitSHAPattern.FindStringSubmatch(resp.Output)
	if len(m) < 2 {
		return nil
	}
	return &store.Effect{Kind: "git_commit", Descriptor: m[1]}
}

// Run is the main hook entry point, invoked by Claude Code after each tool use.
// It reads a Payload from stdin, captures workspace state, and creates a step.
func Run(stdin io.Reader, stdout io.Writer) error {
	// 1. Decode payload from stdin
	var p Payload
	if err := json.NewDecoder(stdin).Decode(&p); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	// 2. Filter out rgt commands to avoid self-referential logs
	if shouldSkipStep(&p) {
		return nil
	}

	// 3. Open store (fail silently if .regent/ doesn't exist)
	regentDir := filepath.Join(p.CWD, ".regent")
	s, err := store.Open(regentDir)
	if err != nil {
		// Not initialized - silently no-op (don't break the agent)
		return nil
	}

	// 3. Open index
	idx, err := index.Open(s)
	if err != nil {
		return logError(s, fmt.Errorf("open index: %w", err))
	}
	defer func() { _ = idx.Close() }()

	// 4. Snapshot workspace (without blame yet)
	ig := ignore.Default(p.CWD)
	treeHash, err := snapshot.Snapshot(s, p.CWD, ig)
	if err != nil {
		return logError(s, fmt.Errorf("snapshot: %w", err))
	}

	// 5. Get parent step (if any)
	parentHash, _ := s.ReadRef("sessions/" + p.SessionID)

	// 6. Write tool args and result as blobs (needed for pre-step hash)
	argsHash, err := s.WriteBlob(p.ToolInput)
	if err != nil {
		return logError(s, fmt.Errorf("write args blob: %w", err))
	}

	resultHash, err := s.WriteBlob(p.ToolResponse)
	if err != nil {
		return logError(s, fmt.Errorf("write result blob: %w", err))
	}

	// 7-8. Blame computation removed (now done in message_hook.go with separate storage)
	// Old blame-in-tree approach disabled

	// 9. Build step object
	step := &store.Step{
		Parent:         parentHash,
		Tree:           treeHash,
		SessionID:      p.SessionID,
		TimestampNanos: time.Now().UnixNano(),
		Cause: store.Cause{
			ToolUseID:  p.ToolUseID,
			ToolName:   p.ToolName,
			ArgsBlob:   argsHash,
			ResultBlob: resultHash,
		},
	}
	if effect := gitCommitEffect(p.ToolName, p.ToolInput, p.ToolResponse); effect != nil {
		step.Effects = append(step.Effects, *effect)
	}

	// 10. Write step
	stepHash, err := s.WriteStep(step)
	if err != nil {
		return logError(s, fmt.Errorf("write step: %w", err))
	}

	// 11. Blame hash replacement removed (no longer needed with separate storage)

	// 12. Read tree for indexing
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return logError(s, fmt.Errorf("read tree: %w", err))
	}

	// 13. CAS update session ref (with retry) - SOURCE OF TRUTH
	// Refs must be updated first to maintain consistency. If this fails, nothing is committed.
	if err := s.UpdateRefWithRetry("sessions/"+p.SessionID, parentHash, stepHash, 8); err != nil {
		return logError(s, fmt.Errorf("update ref: %w", err))
	}

	// 14. Index the step (best effort for this legacy hook path).
	// If indexing fails, recorded workspace state remains recoverable from refs/objects,
	// but CLI visibility still depends on index repair.
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		// Log error but don't fail hook - refs/objects are source of truth
		_ = logError(s, fmt.Errorf("index step (non-fatal): %w", err))
		// Continue - refs are updated, that's what matters
	}

	// Success - exit silently
	return nil
}

// logError writes errors to .regent/log/hook-error.log instead of stdout/stderr
// Returns nil so the hook doesn't break the agent
func logError(s *store.Store, err error) error {
	logPath := filepath.Join(s.Root, "log", "hook-error.log")
	f, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		// Can't even log - give up silently
		return nil
	}
	defer func() { _ = f.Close() }()

	timestamp := time.Now().Format(time.RFC3339)
	_, _ = fmt.Fprintf(f, "[%s] %v\n", timestamp, err)

	// Return nil so hook exits cleanly and doesn't break the agent
	return nil
}

// shouldSkipStep returns true if this tool call should not create a step.
// Currently filters out rgt commands run via Bash tool to avoid self-referential logs.
func shouldSkipStep(p *Payload) bool {
	if p.ToolName != "Bash" {
		return false
	}

	var args map[string]interface{}
	if err := json.Unmarshal(p.ToolInput, &args); err != nil {
		return false // Parse error - don't skip
	}

	cmd, ok := args["command"].(string)
	if !ok {
		return false
	}

	trimmed := strings.TrimSpace(cmd)
	return trimmed == "rgt" || strings.HasPrefix(trimmed, "rgt ")
}
