package capture

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/snapshot"
	"github.com/regent-vcs/regent/internal/store"
)

const (
	OriginClaudeCode = "claude_code"
	OriginCodexCLI   = "codex_cli"

	maxRefUpdateAttempts = 8
)

var messageCounter atomic.Uint64

type Recorder struct {
	Store *store.Store
	Index *index.DB
	CWD   string
}

type turnScope struct {
	id       string
	allTurns bool
}

type SessionMetadata struct {
	SessionID      string
	Origin         string
	Model          string
	PermissionMode string
	TranscriptPath string
	externalID     string
}

type UserPrompt struct {
	SessionMetadata
	TurnID string
	Prompt string
}

type AssistantResponse struct {
	SessionMetadata
	TurnID               string
	LastAssistantMessage string
}

type ToolUse struct {
	SessionMetadata
	TurnID       string
	ToolName     string
	ToolUseID    string
	ToolInput    json.RawMessage
	ToolResponse json.RawMessage
}

func Open(cwd string) (*Recorder, bool, error) {
	if cwd == "" {
		return nil, false, fmt.Errorf("cwd is required")
	}

	s, err := store.Open(filepath.Join(cwd, ".regent"))
	if err != nil {
		if strings.Contains(err.Error(), "regent store not found") {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("open store: %w", err)
	}

	idx, err := index.Open(s)
	if err != nil {
		return nil, false, fmt.Errorf("open index: %w", err)
	}

	return &Recorder{Store: s, Index: idx, CWD: cwd}, true, nil
}

func (r *Recorder) Close() error {
	if r == nil || r.Index == nil {
		return nil
	}
	return r.Index.Close()
}

func (r *Recorder) UpsertSession(meta SessionMetadata) error {
	session, err := normalizeSession(meta)
	if err != nil {
		return err
	}
	if err := r.adoptLegacySession(session); err != nil {
		return err
	}

	return r.upsertNormalizedSession(session)
}

func (r *Recorder) upsertNormalizedSession(session SessionMetadata) error {
	return r.Index.UpsertSession(index.SessionUpdate{
		ID:             session.SessionID,
		Origin:         session.Origin,
		Model:          session.Model,
		PermissionMode: session.PermissionMode,
		TranscriptPath: session.TranscriptPath,
	})
}

func (r *Recorder) RecordUserPrompt(event UserPrompt) error {
	session, scope, err := normalizeTurnEvent(event.SessionMetadata, event.TurnID)
	if err != nil {
		return err
	}
	if err := r.adoptLegacySession(session); err != nil {
		return err
	}
	if err := r.upsertNormalizedSession(session); err != nil {
		return err
	}

	return r.Index.AppendMessage(index.Message{
		ID:          newMessageID("user"),
		SessionID:   session.SessionID,
		TurnID:      scope.id,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "user",
		ContentText: event.Prompt,
	})
}

func (r *Recorder) RecordToolUse(event ToolUse) error {
	if isRegentCommand(event.ToolName, event.ToolInput) {
		return nil
	}
	session, scope, err := normalizeTurnEvent(event.SessionMetadata, event.TurnID)
	if err != nil {
		return err
	}
	if event.ToolName == "" {
		return fmt.Errorf("tool name is required")
	}
	if event.ToolUseID == "" {
		return fmt.Errorf("tool use id is required")
	}
	if len(event.ToolInput) == 0 || !json.Valid(event.ToolInput) {
		return fmt.Errorf("tool input must be valid JSON")
	}
	if len(event.ToolResponse) > 0 && !json.Valid(event.ToolResponse) {
		return fmt.Errorf("tool response must be valid JSON")
	}
	if err := r.adoptLegacySession(session); err != nil {
		return err
	}
	if err := r.upsertNormalizedSession(session); err != nil {
		return err
	}

	inputHash, err := r.Store.WriteBlob(event.ToolInput)
	if err != nil {
		return fmt.Errorf("store tool input: %w", err)
	}

	var outputHash store.Hash
	if len(event.ToolResponse) > 0 {
		outputHash, err = r.Store.WriteBlob(event.ToolResponse)
		if err != nil {
			return fmt.Errorf("store tool output: %w", err)
		}
	}

	now := time.Now().UnixNano()
	call := index.Message{
		ID:          newMessageID("call"),
		SessionID:   session.SessionID,
		TurnID:      scope.id,
		Timestamp:   now,
		MessageType: "tool_call",
		ToolName:    event.ToolName,
		ToolUseID:   event.ToolUseID,
		ToolInput:   string(inputHash),
	}

	result := index.Message{
		ID:          newMessageID("result"),
		SessionID:   session.SessionID,
		TurnID:      scope.id,
		Timestamp:   now + 1,
		MessageType: "tool_result",
		ToolName:    event.ToolName,
		ToolUseID:   event.ToolUseID,
		ToolOutput:  string(outputHash),
	}
	if _, err := r.Index.AppendToolUseMessages(call, result); err != nil {
		return fmt.Errorf("insert tool use messages: %w", err)
	}

	return nil
}

func (r *Recorder) RecordAssistantAndFinalize(event AssistantResponse) error {
	session, scope, err := normalizeTurnEvent(event.SessionMetadata, event.TurnID)
	if err != nil {
		return err
	}
	if err := r.adoptLegacySession(session); err != nil {
		return err
	}
	if err := r.upsertNormalizedSession(session); err != nil {
		return err
	}

	if !scope.allTurns {
		existingStep, ok, err := r.existingStepForTurn(session.SessionID, scope.id)
		if err != nil {
			return err
		}
		if ok {
			if err := r.indexAndLinkStep(existingStep, session.SessionID, scope); err != nil {
				logHookError(r.Store, fmt.Sprintf("recover existing turn %s: %v", scope.id, err))
			}
			if session.TranscriptPath != "" {
				if err := r.ArchiveTranscript(session.SessionID, session.TranscriptPath); err != nil {
					logHookError(r.Store, fmt.Sprintf("archive transcript: %v", err))
				}
			}
			return nil
		}
	}

	now := time.Now().UnixNano()
	if err := r.Index.AppendMessage(index.Message{
		ID:          newMessageID("assistant"),
		SessionID:   session.SessionID,
		TurnID:      scope.id,
		Timestamp:   now,
		MessageType: "assistant",
		ContentText: event.LastAssistantMessage,
	}); err != nil {
		return fmt.Errorf("insert assistant message: %w", err)
	}

	stepHash, err := r.createStepForTurn(session, scope)
	if err != nil {
		return err
	}
	if stepHash == "" {
		rows, err := r.markPendingMessagesProcessed(session.SessionID, scope, now)
		if err != nil {
			return fmt.Errorf("mark messages processed: %w", err)
		}
		if rows == 0 {
			logDebug(r.Store, fmt.Sprintf("no pending messages marked for session %s turn %q", session.SessionID, scope.id))
		}
	}

	if session.TranscriptPath != "" {
		if err := r.ArchiveTranscript(session.SessionID, session.TranscriptPath); err != nil {
			logHookError(r.Store, fmt.Sprintf("archive transcript: %v", err))
		}
	}

	return nil
}

func (r *Recorder) ArchiveTranscript(sessionID, transcriptPath string) error {
	data, err := os.ReadFile(transcriptPath)
	if err != nil {
		return fmt.Errorf("read transcript: %w", err)
	}

	blobHash, err := r.Store.WriteBlob(data)
	if err != nil {
		return fmt.Errorf("write transcript blob: %w", err)
	}

	return r.Index.InsertJSONLSnapshot(sessionID, time.Now().UnixNano(), blobHash)
}

func (r *Recorder) adoptLegacySession(session SessionMetadata) error {
	if session.externalID == "" || session.externalID == session.SessionID {
		return nil
	}

	legacyHead, err := r.Store.ReadRef("sessions/" + session.externalID)
	legacyRefExists := err == nil
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("read legacy session ref: %w", err)
		}
	}

	if legacyRefExists {
		canonicalHead, err := r.Store.ReadRef("sessions/" + session.SessionID)
		switch {
		case err == nil:
			if canonicalHead != legacyHead {
				logHookError(r.Store, fmt.Sprintf("archiving divergent legacy session ref %s -> %s", session.externalID, session.SessionID))
				if archiveErr := r.archiveLegacySessionRef(session.externalID, legacyHead); archiveErr != nil {
					return archiveErr
				}
			}
		case errors.Is(err, fs.ErrNotExist):
			if err := r.Store.UpdateRef("sessions/"+session.SessionID, "", legacyHead); err != nil && !errors.Is(err, store.ErrRefConflict) {
				return fmt.Errorf("write canonical session ref: %w", err)
			}
			logDebug(r.Store, fmt.Sprintf("adopted legacy session ref %s -> %s", session.externalID, session.SessionID))
		default:
			return fmt.Errorf("read canonical session ref: %w", err)
		}

		if err := r.Store.DeleteRef("sessions/"+session.externalID, legacyHead); err != nil && !errors.Is(err, store.ErrRefConflict) {
			return fmt.Errorf("delete legacy session ref: %w", err)
		}
	}

	changed, err := r.Index.RenameSession(session.externalID, session.SessionID, session.Origin)
	if err != nil {
		return fmt.Errorf("adopt legacy session index: %w", err)
	}
	if changed {
		logDebug(r.Store, fmt.Sprintf("adopted legacy session index %s -> %s", session.externalID, session.SessionID))
	}
	return nil
}

func (r *Recorder) archiveLegacySessionRef(externalID string, head store.Hash) error {
	archiveRef := "legacy-sessions/" + externalID
	if err := r.Store.UpdateRef(archiveRef, "", head); err != nil {
		if !errors.Is(err, store.ErrRefConflict) {
			return fmt.Errorf("archive legacy session ref: %w", err)
		}
		archivedHead, readErr := r.Store.ReadRef(archiveRef)
		if readErr != nil {
			return fmt.Errorf("read archived legacy session ref: %w", readErr)
		}
		if archivedHead != head {
			return fmt.Errorf("archive legacy session ref: %w", store.ErrRefConflict)
		}
	}
	return nil
}

func (r *Recorder) createStepForTurn(session SessionMetadata, scope turnScope) (store.Hash, error) {
	sessionID := session.SessionID
	for attempt := 0; attempt < maxRefUpdateAttempts; attempt++ {
		parentHash, err := r.Store.ReadRef("sessions/" + sessionID)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("read session ref: %w", err)
		}

		messages, err := r.pendingMessages(sessionID, scope)
		if err != nil {
			return "", fmt.Errorf("get pending messages: %w", err)
		}

		causes := causesFromMessages(messages)
		if len(causes) == 0 {
			return "", nil
		}

		treeHash, err := snapshotWorkspace(r.Store, r.CWD)
		if err != nil {
			return "", fmt.Errorf("snapshot workspace: %w", err)
		}

		step := &store.Step{
			Parent:         parentHash,
			Tree:           treeHash,
			Cause:          causes[0],
			Causes:         causes,
			SessionID:      sessionID,
			Origin:         session.Origin,
			TurnID:         scope.id,
			TimestampNanos: time.Now().UnixNano(),
		}

		stepHash, err := r.Store.WriteStep(step)
		if err != nil {
			return "", fmt.Errorf("write step: %w", err)
		}

		if err := computeAndWriteBlame(r.Store, parentHash, stepHash, treeHash); err != nil {
			logHookError(r.Store, fmt.Sprintf("blame step %s: %v", stepHash, err))
		}

		if err := r.Store.UpdateRef("sessions/"+sessionID, parentHash, stepHash); err != nil {
			if errors.Is(err, store.ErrRefConflict) {
				time.Sleep(refUpdateBackoff(attempt))
				continue
			}
			return "", fmt.Errorf("update ref: %w", err)
		}

		if err := r.indexAndLinkStep(stepHash, sessionID, scope); err != nil {
			logHookError(r.Store, fmt.Sprintf("index/link step %s: %v", stepHash, err))
		}

		return stepHash, nil
	}

	return "", fmt.Errorf("update ref: %w", store.ErrRefConflict)
}

func refUpdateBackoff(attempt int) time.Duration {
	backoff := 5 * time.Millisecond
	for i := 0; i < attempt; i++ {
		backoff *= 2
		if backoff >= 100*time.Millisecond {
			return 100 * time.Millisecond
		}
	}
	return backoff
}

func (r *Recorder) existingStepForTurn(sessionID, turnID string) (store.Hash, bool, error) {
	stepHash, ok, err := r.Index.StepForTurn(sessionID, turnID)
	if err != nil {
		return "", false, fmt.Errorf("lookup indexed turn: %w", err)
	}
	if ok {
		return stepHash, true, nil
	}

	headHash, err := r.Store.ReadRef("sessions/" + sessionID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read session ref: %w", err)
	}

	headStep, err := r.Store.ReadStep(headHash)
	if err != nil {
		return "", false, fmt.Errorf("read head step: %w", err)
	}
	if headStep.SessionID == sessionID && headStep.TurnID == turnID {
		return headHash, true, nil
	}
	return "", false, nil
}

func (r *Recorder) indexAndLinkStep(stepHash store.Hash, sessionID string, scope turnScope) error {
	step, err := r.Store.ReadStep(stepHash)
	if err != nil {
		return fmt.Errorf("read step: %w", err)
	}
	tree, err := r.Store.ReadTree(step.Tree)
	if err != nil {
		return fmt.Errorf("read tree: %w", err)
	}
	if err := r.Index.IndexStep(stepHash, step, tree); err != nil {
		return fmt.Errorf("index step: %w", err)
	}

	rows, err := r.linkPendingMessages(sessionID, scope, stepHash, time.Now().UnixNano())
	if err != nil {
		return fmt.Errorf("link messages: %w", err)
	}
	if rows == 0 {
		logDebug(r.Store, fmt.Sprintf("no pending messages linked for step %s session %s turn %q", stepHash, sessionID, scope.id))
	}
	return nil
}

func (r *Recorder) pendingMessages(sessionID string, scope turnScope) ([]index.Message, error) {
	if scope.allTurns {
		return r.Index.GetAllPendingMessages(sessionID)
	}
	return r.Index.GetPendingMessages(sessionID, scope.id)
}

func (r *Recorder) linkPendingMessages(sessionID string, scope turnScope, stepHash store.Hash, processedAt int64) (int64, error) {
	if scope.allTurns {
		return r.Index.LinkAllPendingMessagesToStep(sessionID, stepHash, processedAt)
	}
	return r.Index.LinkPendingMessagesToStep(sessionID, scope.id, stepHash, processedAt)
}

func (r *Recorder) markPendingMessagesProcessed(sessionID string, scope turnScope, processedAt int64) (int64, error) {
	if scope.allTurns {
		return r.Index.MarkAllPendingMessagesProcessed(sessionID, processedAt)
	}
	return r.Index.MarkPendingMessagesProcessed(sessionID, scope.id, processedAt)
}

func causesFromMessages(messages []index.Message) []store.Cause {
	var causes []store.Cause
	for _, msg := range messages {
		if msg.MessageType != "tool_call" || msg.ToolInput == "" {
			continue
		}

		cause := store.Cause{
			ToolUseID: msg.ToolUseID,
			ToolName:  msg.ToolName,
			ArgsBlob:  store.Hash(msg.ToolInput),
		}
		for _, resultMsg := range messages {
			if resultMsg.MessageType == "tool_result" && resultMsg.ToolUseID == msg.ToolUseID {
				cause.ResultBlob = store.Hash(resultMsg.ToolOutput)
				break
			}
		}
		causes = append(causes, cause)
	}
	return causes
}

func snapshotWorkspace(s *store.Store, cwd string) (store.Hash, error) {
	return snapshot.Snapshot(s, cwd, ignore.Default(cwd))
}

func computeAndWriteBlame(s *store.Store, parentHash, currentStepHash, treeHash store.Hash) error {
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return fmt.Errorf("read current tree %s: %w", treeHash, err)
	}

	parentEntries := map[string]store.TreeEntry{}
	if parentHash != "" {
		parentStep, err := s.ReadStep(parentHash)
		if err != nil {
			return fmt.Errorf("read parent step %s: %w", parentHash, err)
		}
		parentTree, err := s.ReadTree(parentStep.Tree)
		if err != nil {
			return fmt.Errorf("read parent tree %s: %w", parentStep.Tree, err)
		}
		for _, entry := range parentTree.Entries {
			parentEntries[entry.Path] = entry
		}
	}

	var problems []error
	for _, entry := range tree.Entries {
		parentEntry, hasParentEntry := parentEntries[entry.Path]

		if hasParentEntry && parentEntry.Blob == entry.Blob {
			oldBlame, err := s.ReadBlameForFile(parentHash, entry.Path)
			if err == nil {
				if err := s.WriteBlameForFile(currentStepHash, entry.Path, oldBlame); err != nil {
					problems = append(problems, fmt.Errorf("copy blame for %s: %w", entry.Path, err))
				}
				continue
			}
			problems = append(problems, fmt.Errorf("read parent blame for unchanged %s: %w", entry.Path, err))
			continue
		}

		var oldContent []byte
		var oldBlame *store.BlameMap
		if hasParentEntry {
			oldContent, err = s.ReadBlob(parentEntry.Blob)
			if err != nil {
				problems = append(problems, fmt.Errorf("read parent blob for %s: %w", entry.Path, err))
				continue
			}
			oldBlame, err = s.ReadBlameForFile(parentHash, parentEntry.Path)
			if err != nil {
				problems = append(problems, fmt.Errorf("read parent blame for %s: %w", entry.Path, err))
			}
		}

		newContent, err := s.ReadBlob(entry.Blob)
		if err != nil {
			problems = append(problems, fmt.Errorf("read current blob for %s: %w", entry.Path, err))
			continue
		}

		newBlame := store.ComputeBlame(oldContent, newContent, oldBlame, currentStepHash)
		if err := s.WriteBlameForFile(currentStepHash, entry.Path, newBlame); err != nil {
			problems = append(problems, fmt.Errorf("write blame for %s: %w", entry.Path, err))
		}
	}

	return errors.Join(problems...)
}

func isRegentCommand(toolName string, input json.RawMessage) bool {
	if !strings.EqualFold(toolName, "Bash") && !strings.EqualFold(toolName, "functions.exec_command") {
		return false
	}

	var args map[string]interface{}
	if err := json.Unmarshal(input, &args); err != nil {
		return false
	}

	command, _ := args["command"].(string)
	if command == "" {
		command, _ = args["cmd"].(string)
	}
	return startsWithRegentCommand(command)
}

func startsWithRegentCommand(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	for len(fields) > 0 && strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "=") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return false
	}

	first := strings.TrimPrefix(filepath.Base(fields[0]), "./")
	if first == "rgt" || first == "regent" {
		return true
	}

	return len(fields) >= 3 && fields[0] == "go" && fields[1] == "run" && strings.Contains(fields[2], "cmd/rgt")
}

func normalizeTurnEvent(meta SessionMetadata, turnID string) (SessionMetadata, turnScope, error) {
	session, err := normalizeSession(meta)
	if err != nil {
		return SessionMetadata{}, turnScope{}, err
	}

	scope, err := normalizeTurnScope(session.Origin, turnID)
	if err != nil {
		return SessionMetadata{}, turnScope{}, err
	}
	return session, scope, nil
}

func normalizeSession(meta SessionMetadata) (SessionMetadata, error) {
	origin := meta.Origin
	if origin == "" {
		origin = OriginClaudeCode
	}
	if !isSafeOrigin(origin) {
		return SessionMetadata{}, fmt.Errorf("invalid origin %q", origin)
	}
	externalID := strings.TrimSpace(meta.SessionID)
	if externalID == "" {
		return SessionMetadata{}, fmt.Errorf("session id is required")
	}

	meta.Origin = origin
	meta.SessionID = canonicalSessionID(origin, externalID)
	meta.externalID = externalID
	return meta, nil
}

func canonicalSessionID(origin, externalID string) string {
	return origin + ":" + url.QueryEscape(externalID)
}

func normalizeTurnScope(origin, turnID string) (turnScope, error) {
	if turnID != "" {
		return turnScope{id: turnID}, nil
	}
	if origin == OriginClaudeCode {
		return turnScope{allTurns: true}, nil
	}
	return turnScope{}, fmt.Errorf("turn id is required")
}

func isSafeOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	for _, r := range origin {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func newMessageID(kind string) string {
	return fmt.Sprintf("msg_%d_%d_%s", time.Now().UnixNano(), messageCounter.Add(1), kind)
}

func logDebug(s *store.Store, msg string) {
	logPath := filepath.Join(s.Root, "log", "hook-debug.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), msg)
}

func logHookError(s *store.Store, msg string) {
	logPath := filepath.Join(s.Root, "log", "hook-error.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o755)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	fmt.Fprintf(f, "[%s] %s\n", time.Now().Format(time.RFC3339), msg)
}
