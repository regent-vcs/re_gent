package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
)

// Options configure Codex sidecar import/watch behavior.
type Options struct {
	ProjectRoot  string
	CodexHome    string
	PollInterval time.Duration
	ChangesOnly  bool
	WatchMode    bool
	Stderr       func(string)
}

type parsedRollout struct {
	path       string
	checkpoint Checkpoint
	parsed     *parsedFile
}

type watchRuntime struct {
	lastConcurrentWarningKey string
}

// RunImport executes a one-shot Codex import for the target project.
func RunImport(opts Options) error {
	return runOnce(opts)
}

// RunWatch executes the same import loop continuously with polling.
func RunWatch(opts Options) error {
	opts = normalizeOptions(opts)
	runtime := &watchRuntime{}
	for {
		if err := runOnceWithRuntime(opts, runtime); err != nil {
			return err
		}
		time.Sleep(opts.PollInterval)
	}
}

func normalizeOptions(opts Options) Options {
	if opts.ProjectRoot == "" {
		opts.ProjectRoot = "."
	}
	if opts.CodexHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			opts.CodexHome = filepath.Join(home, ".codex")
		}
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultPollIntervalSeconds * time.Second
	}
	if opts.Stderr == nil {
		opts.Stderr = func(msg string) {
			fmt.Fprintln(os.Stderr, msg)
		}
	}
	return opts
}

func runOnce(opts Options) error {
	opts = normalizeOptions(opts)
	return runOnceWithRuntime(opts, nil)
}

func runOnceWithRuntime(opts Options, runtime *watchRuntime) error {
	projectRoot, err := filepath.Abs(opts.ProjectRoot)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}

	s, err := store.Open(filepath.Join(projectRoot, ".regent"))
	if err != nil {
		return err
	}

	idx, err := index.Open(s)
	if err != nil {
		return err
	}
	defer func() { _ = idx.Close() }()

	state, err := loadState(s, projectRoot)
	if err != nil {
		return err
	}

	rollouts, err := findProjectRollouts(opts.CodexHome, projectRoot)
	if err != nil {
		return err
	}

	var parsedRollouts []parsedRollout
	for _, rolloutPath := range rollouts {
		cp := state.Checkpoints[rolloutPath]
		parsed, parseErr := parseRolloutFile(rolloutPath, cp.ByteOffset)
		if parseErr != nil {
			return parseErr
		}
		if parsed.Session.SessionID == "" || !sameProject(parsed.Session.ProjectCWD, projectRoot) {
			continue
		}

		parsedRollouts = append(parsedRollouts, parsedRollout{
			path:       rolloutPath,
			checkpoint: cp,
			parsed:     parsed,
		})
	}

	if opts.WatchMode {
		maybeWarnConcurrentSessions(projectRoot, parsedRollouts, opts, runtime)
	}

	for _, rollout := range parsedRollouts {
		if processErr := processParsedFile(s, idx, projectRoot, rollout.parsed, opts); processErr != nil {
			return processErr
		}

		lastTS := rollout.checkpoint.LastEventTS
		if len(rollout.parsed.Turns) > 0 {
			lastTS = rollout.parsed.Turns[len(rollout.parsed.Turns)-1].CompletedAt
			if lastTS.IsZero() {
				lastTS = rollout.parsed.Turns[len(rollout.parsed.Turns)-1].StartedAt
			}
		}
		state.Checkpoints[rollout.path] = Checkpoint{
			SourceFile:  rollout.path,
			ByteOffset:  rollout.parsed.Offset,
			LastEventTS: lastTS,
			SessionID:   rollout.parsed.Session.SessionID,
		}
	}

	return saveState(s, state)
}

func maybeWarnConcurrentSessions(projectRoot string, rollouts []parsedRollout, opts Options, runtime *watchRuntime) {
	activeSessions := overlappingSessionIDs(rollouts, time.Now().UTC())
	if len(activeSessions) < 2 {
		if runtime != nil {
			runtime.lastConcurrentWarningKey = ""
		}
		return
	}

	key := strings.Join(activeSessions, "\x00")
	if runtime != nil && runtime.lastConcurrentWarningKey == key {
		return
	}
	if runtime != nil {
		runtime.lastConcurrentWarningKey = key
	}

	opts.Stderr(fmt.Sprintf(
		"warning: multiple active Codex sessions detected for project %s (%s); attribution may be ambiguous",
		projectRoot,
		strings.Join(activeSessions, ", "),
	))
}

type sessionWindow struct {
	sessionID string
	startedAt time.Time
	endedAt   time.Time
}

func overlappingSessionIDs(rollouts []parsedRollout, now time.Time) []string {
	var windows []sessionWindow
	for _, rollout := range rollouts {
		sessionID := rollout.parsed.Session.SessionID
		if sessionID == "" {
			continue
		}
		for _, turn := range rollout.parsed.Turns {
			if turn.StartedAt.IsZero() {
				continue
			}

			end := turn.CompletedAt
			if end.IsZero() {
				end = now
			}
			if !end.After(turn.StartedAt) {
				end = turn.StartedAt.Add(time.Nanosecond)
			}

			windows = append(windows, sessionWindow{
				sessionID: sessionID,
				startedAt: turn.StartedAt,
				endedAt:   end,
			})
		}
	}

	active := map[string]struct{}{}
	for i := 0; i < len(windows); i++ {
		for j := i + 1; j < len(windows); j++ {
			if windows[i].sessionID == windows[j].sessionID {
				continue
			}
			if windowsOverlap(windows[i], windows[j]) {
				active[windows[i].sessionID] = struct{}{}
				active[windows[j].sessionID] = struct{}{}
			}
		}
	}

	ids := make([]string, 0, len(active))
	for sessionID := range active {
		ids = append(ids, sessionID)
	}
	sort.Strings(ids)
	return ids
}

func windowsOverlap(left, right sessionWindow) bool {
	if left.startedAt.IsZero() || right.startedAt.IsZero() {
		return false
	}
	return left.startedAt.Before(right.endedAt) && right.startedAt.Before(left.endedAt)
}

func processParsedFile(s *store.Store, idx *index.DB, projectRoot string, parsed *parsedFile, opts Options) error {
	baselineKnown := true
	for _, turn := range parsed.Turns {
		if turn.CompletedAt.IsZero() {
			if !opts.WatchMode {
				opts.Stderr(fmt.Sprintf("warning: skipping incomplete Codex turn %s in %s", turn.TurnID, filepath.Base(parsed.Session.SourceFile)))
			}
			continue
		}

		if !opts.WatchMode {
			switch classifyImportTurn(turn) {
			case importTurnReadOnly:
				continue
			case importTurnBlocked:
				if baselineKnown {
					opts.Stderr(fmt.Sprintf("warning: skipping Codex turn %s in %s because its workspace changes cannot be reconstructed from rollout history; later turns in this session will also be skipped during import", turn.TurnID, filepath.Base(parsed.Session.SourceFile)))
				}
				baselineKnown = false
				continue
			case importTurnReplayable:
				if !baselineKnown {
					opts.Stderr(fmt.Sprintf("warning: skipping Codex turn %s in %s because earlier non-replayable turns left the session baseline unknown; use `rgt codex watch` for live capture", turn.TurnID, filepath.Base(parsed.Session.SourceFile)))
					continue
				}
			}
		}

		created, err := processTurn(s, idx, projectRoot, parsed.Session, turn, opts)
		if err != nil {
			return fmt.Errorf("process turn %s: %w", turn.TurnID, err)
		}

		if !created && len(turn.Warnings) > 0 {
			for _, warning := range turn.Warnings {
				opts.Stderr("warning: " + warning)
			}
		}
	}

	return nil
}

func processTurn(s *store.Store, idx *index.DB, projectRoot string, sess Session, turn Turn, opts Options) (bool, error) {
	sessionID := sess.SessionID
	parentHash, _ := s.ReadSessionRef(sessionID)

	existingHead, err := idx.SessionHead(sessionID)
	if err == nil && existingHead != "" && parentHash == "" {
		parentHash = existingHead
	}

	prevTranscript := store.Hash("")
	if _, transcriptHash, err := idx.SessionLastProcessedMessage(sessionID); err == nil {
		prevTranscript = transcriptHash
	}

	if alreadyImported(s, parentHash, turn.TurnID) {
		return false, nil
	}

	treeHash, replayed, err := materializeTurnTree(s, projectRoot, parentHash, turn, opts)
	if err != nil {
		return false, err
	}

	if opts.ChangesOnly && parentHash != "" {
		parentStep, parentErr := s.ReadStep(parentHash)
		if parentErr == nil && parentStep.Tree == treeHash {
			return false, nil
		}
	}

	if opts.ChangesOnly && parentHash == "" && !replayed {
		// First step with only a live snapshot but no file changes is still allowed.
	}

	transcriptHash, lastMessageID, err := writeTurnTranscript(s, turn, prevTranscript)
	if err != nil {
		return false, err
	}

	causes, err := writeTurnCauses(s, turn)
	if err != nil {
		return false, err
	}
	if len(causes) == 0 {
		return false, nil
	}

	step := &store.Step{
		Parent:         parentHash,
		Tree:           treeHash,
		Transcript:     transcriptHash,
		Cause:          causes[0],
		Causes:         causes,
		SessionID:      sessionID,
		Origin:         "codex",
		TurnID:         turn.TurnID,
		AgentID:        turn.TurnID,
		TimestampNanos: turn.CompletedAt.UnixNano(),
	}

	stepHash, err := s.WriteStep(step)
	if err != nil {
		return false, fmt.Errorf("write step: %w", err)
	}

	if err := computeAndWriteBlameForStep(s, parentHash, stepHash, treeHash); err != nil {
		opts.Stderr(fmt.Sprintf("warning: Codex blame generation failed for step %s: %v", stepHash[:8], err))
	}

	if err := s.UpdateSessionRefWithRetry(sessionID, parentHash, stepHash, 8); err != nil {
		return false, fmt.Errorf("update session ref: %w", err)
	}

	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return false, fmt.Errorf("read tree: %w", err)
	}
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		return false, fmt.Errorf("index step: %w", err)
	}

	for i, msg := range turn.Messages {
		seqNum, seqErr := idx.GetNextMessageSeq(sessionID)
		if seqErr != nil {
			return false, seqErr
		}
		if err := idx.InsertMessage(index.Message{
			ID:          fmt.Sprintf("%s-msg-%d", turn.TurnID, i),
			SessionID:   sessionID,
			StepID:      string(stepHash),
			TurnID:      turn.TurnID,
			SeqNum:      seqNum,
			Timestamp:   msg.Timestamp.UnixNano(),
			MessageType: msg.Role,
			ContentText: msg.Text,
		}); err != nil && !isDuplicateMessageErr(err) {
			return false, err
		}
	}

	for i, tool := range turn.ToolCalls {
		seqNum, seqErr := idx.GetNextMessageSeq(sessionID)
		if seqErr != nil {
			return false, seqErr
		}
		argsHash, _ := s.WriteBlob(tool.Input)
		if err := idx.InsertMessage(index.Message{
			ID:          fmt.Sprintf("%s-tool-call-%d", turn.TurnID, i),
			SessionID:   sessionID,
			StepID:      string(stepHash),
			TurnID:      turn.TurnID,
			SeqNum:      seqNum,
			Timestamp:   tool.StartedAt.UnixNano(),
			MessageType: "tool_call",
			ToolName:    tool.ToolName,
			ToolUseID:   tool.CallID,
			ToolInput:   string(argsHash),
		}); err != nil && !isDuplicateMessageErr(err) {
			return false, err
		}

		seqNum, seqErr = idx.GetNextMessageSeq(sessionID)
		if seqErr != nil {
			return false, seqErr
		}
		resultHash, _ := s.WriteBlob(tool.Output)
		if err := idx.InsertMessage(index.Message{
			ID:          fmt.Sprintf("%s-tool-result-%d", turn.TurnID, i),
			SessionID:   sessionID,
			StepID:      string(stepHash),
			TurnID:      turn.TurnID,
			SeqNum:      seqNum,
			Timestamp:   tool.CompletedAt.UnixNano(),
			MessageType: "tool_result",
			ToolName:    tool.ToolName,
			ToolUseID:   tool.CallID,
			ToolOutput:  string(resultHash),
		}); err != nil && !isDuplicateMessageErr(err) {
			return false, err
		}
	}

	if lastMessageID != "" && transcriptHash != "" {
		if err := idx.UpdateSessionLastProcessed(sessionID, lastMessageID, transcriptHash); err != nil {
			return false, err
		}
	}

	return true, nil
}

func materializeTurnTree(s *store.Store, projectRoot string, parentHash store.Hash, turn Turn, opts Options) (store.Hash, bool, error) {
	if canReplayTurn(turn) {
		treeHash, err := replayTurnTree(s, projectRoot, parentHash, turn)
		return treeHash, true, err
	}

	if !opts.WatchMode {
		return "", false, fmt.Errorf("turn %s cannot be replayed from Codex history; rerun with `rgt codex watch` for live capture", turn.TurnID)
	}

	ig := ignore.Default(projectRoot)
	treeHash, err := snapshot.Snapshot(s, projectRoot, ig)
	return treeHash, false, err
}

func canReplayTurn(turn Turn) bool {
	if len(turn.ToolCalls) == 0 {
		return false
	}
	for _, tool := range turn.ToolCalls {
		if !tool.SupportsReplay {
			return false
		}
	}
	return true
}

type importTurnClass int

const (
	importTurnReadOnly importTurnClass = iota
	importTurnReplayable
	importTurnBlocked
)

func classifyImportTurn(turn Turn) importTurnClass {
	if canReplayTurn(turn) {
		return importTurnReplayable
	}
	if len(turn.ToolCalls) == 0 {
		return importTurnReadOnly
	}
	for _, tool := range turn.ToolCalls {
		if !isReadOnlyToolCall(tool) {
			return importTurnBlocked
		}
	}
	return importTurnReadOnly
}

func isReadOnlyToolCall(tool ToolCall) bool {
	switch strings.ToLower(strings.TrimSpace(tool.ToolName)) {
	case "read_thread_terminal", "load_workspace_dependencies",
		"list_mcp_resources", "list_mcp_resource_templates", "read_mcp_resource",
		"view_image", "open", "find", "click", "screenshot",
		"search_query", "image_query", "weather", "time", "sports", "finance",
		"spawn_agent", "wait_agent", "send_input", "close_agent", "resume_agent":
		return true
	case "shell_command":
		return isReadOnlyShellCommand(tool.Input)
	default:
		return false
	}
}

func isReadOnlyShellCommand(raw []byte) bool {
	var payload struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	cmd := strings.TrimSpace(strings.ToLower(payload.Command))
	if cmd == "" {
		return false
	}

	allowedPrefixes := []string{
		"get-content", "get-childitem", "select-string", "rg ", "rg.exe ", "where ",
		"get-command", "test-path", "resolve-path", "pwd", "get-location",
		"git status", "git diff", "git show", "git log", "git rev-parse", "git branch",
		"ls", "dir", "cat ", "type ", "echo ", "findstr ", "& ",
	}
	allowedExact := map[string]struct{}{
		"ls":           {},
		"dir":          {},
		"pwd":          {},
		"get-location": {},
	}
	for exact := range allowedExact {
		if cmd == exact {
			return true
		}
	}
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cmd, prefix) {
			if strings.HasPrefix(cmd, "& ") {
				return strings.Contains(cmd, " -help") || strings.HasSuffix(cmd, " -help")
			}
			return true
		}
	}
	return false
}

func replayTurnTree(s *store.Store, projectRoot string, parentHash store.Hash, turn Turn) (store.Hash, error) {
	baseEntries := map[string]store.TreeEntry{}
	if parentHash != "" {
		parentStep, err := s.ReadStep(parentHash)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil {
			parentTree, err := s.ReadTree(parentStep.Tree)
			if err != nil {
				return "", err
			}
			for _, entry := range parentTree.Entries {
				baseEntries[entry.Path] = entry
			}
		}
	}

	for _, tool := range turn.ToolCalls {
		if !strings.EqualFold(tool.ToolName, "apply_patch") {
			return "", fmt.Errorf("unsupported replay tool %s", tool.ToolName)
		}

		var patchText string
		var payload struct {
			Input string `json:"input"`
		}
		if err := json.Unmarshal(tool.Input, &payload); err == nil && payload.Input != "" {
			patchText = payload.Input
		}
		if strings.TrimSpace(patchText) == "" {
			patchText = strings.TrimSpace(string(tool.Input))
		}
		if strings.TrimSpace(patchText) == "" {
			return "", fmt.Errorf("empty apply_patch payload for call %s", tool.CallID)
		}

		nextEntries, err := applyPatchToEntries(s, projectRoot, baseEntries, patchText)
		if err != nil {
			return "", err
		}
		baseEntries = nextEntries
	}

	entries := make([]store.TreeEntry, 0, len(baseEntries))
	for _, entry := range baseEntries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	return s.WriteTree(&store.Tree{Entries: entries})
}

func writeTurnTranscript(s *store.Store, turn Turn, prev store.Hash) (store.Hash, string, error) {
	if len(turn.Messages) == 0 {
		return prev, "", nil
	}

	messageHashes := make([]store.Hash, 0, len(turn.Messages))
	lastMessageID := ""
	for _, msg := range turn.Messages {
		if len(msg.Raw) == 0 {
			envelope := map[string]interface{}{
				"type": msg.Role,
				"message": map[string]interface{}{
					"role":    msg.Role,
					"content": msg.Text,
				},
			}
			raw, err := json.Marshal(envelope)
			if err != nil {
				return "", "", err
			}
			msg.Raw = raw
		}
		hash, err := s.WriteBlob(msg.Raw)
		if err != nil {
			return "", "", err
		}
		messageHashes = append(messageHashes, hash)
		lastMessageID = msg.ID
	}

	hash, err := s.WriteTranscript(prev, messageHashes)
	if err != nil {
		return "", "", err
	}
	return hash, lastMessageID, nil
}

func writeTurnCauses(s *store.Store, turn Turn) ([]store.Cause, error) {
	var causes []store.Cause
	for _, tool := range turn.ToolCalls {
		if len(tool.Input) == 0 && len(tool.Output) == 0 {
			continue
		}
		argsHash, err := s.WriteBlob(tool.Input)
		if err != nil {
			return nil, err
		}
		resultHash, err := s.WriteBlob(tool.Output)
		if err != nil {
			return nil, err
		}
		causes = append(causes, store.Cause{
			ToolUseID:  tool.CallID,
			ToolName:   tool.ToolName,
			ArgsBlob:   argsHash,
			ResultBlob: resultHash,
		})
	}
	return causes, nil
}

func findProjectRollouts(codexHome, _ string) ([]string, error) {
	sessionsRoot := filepath.Join(codexHome, "sessions")
	var rollouts []string
	err := filepath.WalkDir(sessionsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(d.Name()), ".jsonl") {
			rollouts = append(rollouts, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan Codex sessions: %w", err)
	}

	sort.Strings(rollouts)
	return rollouts, nil
}

func sameProject(sessionCWD, projectRoot string) bool {
	if sessionCWD == "" {
		return false
	}
	left := filepath.Clean(sessionCWD)
	right := filepath.Clean(projectRoot)
	if strings.EqualFold(filepath.VolumeName(left), filepath.VolumeName(right)) {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func alreadyImported(s *store.Store, headHash store.Hash, turnID string) bool {
	for headHash != "" && turnID != "" {
		step, err := s.ReadStep(headHash)
		if err != nil {
			return false
		}
		if step.AgentID == turnID || step.TurnID == turnID {
			return true
		}
		headHash = step.Parent
	}
	return false
}

func isDuplicateMessageErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed: messages.id")
}
