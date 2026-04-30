package hook

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
)

// Run is the main hook entry point, invoked by Claude Code after each tool use.
// It reads a Payload from stdin, captures workspace state, and creates a step.
func Run(stdin io.Reader, stdout io.Writer) error {
	// 1. Decode payload from stdin
	var p Payload
	if err := json.NewDecoder(stdin).Decode(&p); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	// 2. Open store (fail silently if .regent/ doesn't exist)
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

	// 4. Snapshot workspace
	ig := ignore.Default(p.CWD)
	treeHash, err := snapshot.Snapshot(s, p.CWD, ig)
	if err != nil {
		return logError(s, fmt.Errorf("snapshot: %w", err))
	}

	// 5. Get parent step (if any)
	parentHash, _ := s.ReadRef("sessions/" + p.SessionID)

	// 6. Write tool args and result as blobs
	argsHash, err := s.WriteBlob(p.ToolInput)
	if err != nil {
		return logError(s, fmt.Errorf("write args blob: %w", err))
	}

	resultHash, err := s.WriteBlob(p.ToolResponse)
	if err != nil {
		return logError(s, fmt.Errorf("write result blob: %w", err))
	}

	// 7. Build step
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

	// 8. Write step
	stepHash, err := s.WriteStep(step)
	if err != nil {
		return logError(s, fmt.Errorf("write step: %w", err))
	}

	// 9. Read tree for indexing
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return logError(s, fmt.Errorf("read tree: %w", err))
	}

	// 10. Index the step
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		return logError(s, fmt.Errorf("index step: %w", err))
	}

	// 11. CAS update session ref (with retry)
	if err := s.UpdateRefWithRetry("sessions/"+p.SessionID, parentHash, stepHash, 8); err != nil {
		return logError(s, fmt.Errorf("update ref: %w", err))
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
