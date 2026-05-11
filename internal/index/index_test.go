package index

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/store"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("store.Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	// Verify we can query the database
	var count int
	err = idx.db.QueryRow("SELECT COUNT(*) FROM steps").Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 steps initially, got %d", count)
	}
}

func TestIndexStep_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	// Create a simple step
	blobHash, _ := s.WriteBlob([]byte("test content"))
	tree := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "test.txt", Blob: blobHash, Mode: 0o644},
		},
	}
	treeHash, _ := s.WriteTree(tree)

	step := &store.Step{
		Parent:         "",
		SessionID:      "test-session",
		AgentID:        "agent-1",
		TimestampNanos: time.Now().UnixNano(),
		Tree:           treeHash,
		Cause: store.Cause{
			ToolName:  "Write",
			ToolUseID: "tool_1",
		},
	}

	stepHash, _ := s.WriteStep(step)

	// Index the step
	err = idx.IndexStep(stepHash, step, tree)
	if err != nil {
		t.Fatalf("IndexStep failed: %v", err)
	}

	// Verify step was indexed
	steps, err := idx.ListSteps("test-session", 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	if steps[0].Hash != stepHash {
		t.Errorf("Step hash mismatch: got %s, want %s", steps[0].Hash, stepHash)
	}

	if steps[0].ToolName != "Write" {
		t.Errorf("Tool name mismatch: got %s, want Write", steps[0].ToolName)
	}

	// Verify file was indexed
	var fileCount int
	err = idx.db.QueryRow("SELECT COUNT(*) FROM step_files WHERE step_id = ?", stepHash).Scan(&fileCount)
	if err != nil {
		t.Fatalf("Query step_files failed: %v", err)
	}

	if fileCount != 1 {
		t.Errorf("Expected 1 file in step_files, got %d", fileCount)
	}
}

func TestIndexStep_ParentChain(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	sessionID := "test-session-chain"

	// Create 3 steps with parent relationships
	var prevHash store.Hash
	for i := 0; i < 3; i++ {
		blobHash, _ := s.WriteBlob([]byte("content"))
		tree := &store.Tree{
			Entries: []store.TreeEntry{
				{Path: "file.txt", Blob: blobHash, Mode: 0o644},
			},
		}
		treeHash, _ := s.WriteTree(tree)

		step := &store.Step{
			Parent:         prevHash,
			SessionID:      sessionID,
			TimestampNanos: time.Now().UnixNano() + int64(i*1000), // Ensure different timestamps
			Tree:           treeHash,
			Cause: store.Cause{
				ToolName:  "Edit",
				ToolUseID: "tool_" + string(rune('A'+i)),
			},
		}

		stepHash, _ := s.WriteStep(step)
		if err := idx.IndexStep(stepHash, step, tree); err != nil {
			t.Fatalf("IndexStep %d failed: %v", i, err)
		}

		prevHash = stepHash
	}

	// List steps (newest first)
	steps, err := idx.ListSteps(sessionID, 10)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("Expected 3 steps, got %d", len(steps))
	}

	// Verify parent relationships (steps are in reverse chronological order)
	if steps[0].ParentHash != steps[1].Hash {
		t.Errorf("Step 0 parent mismatch: got %s, want %s", steps[0].ParentHash, steps[1].Hash)
	}

	if steps[1].ParentHash != steps[2].Hash {
		t.Errorf("Step 1 parent mismatch: got %s, want %s", steps[1].ParentHash, steps[2].Hash)
	}

	if steps[2].ParentHash != "" {
		t.Errorf("Step 2 should have no parent, got %s", steps[2].ParentHash)
	}
}

func TestSessionHead(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	sessionID := "test-session-head"

	// Try to get head of non-existent session
	_, err = idx.SessionHead(sessionID)
	if err == nil {
		t.Error("Expected error for non-existent session")
	}

	// Create a step
	blobHash, _ := s.WriteBlob([]byte("content"))
	tree := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "file.txt", Blob: blobHash, Mode: 0o644},
		},
	}
	treeHash, _ := s.WriteTree(tree)

	step := &store.Step{
		SessionID:      sessionID,
		TimestampNanos: time.Now().UnixNano(),
		Tree:           treeHash,
		Cause: store.Cause{
			ToolName:  "Write",
			ToolUseID: "tool_1",
		},
	}

	stepHash, _ := s.WriteStep(step)
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("IndexStep failed: %v", err)
	}

	// Get head
	head, err := idx.SessionHead(sessionID)
	if err != nil {
		t.Fatalf("SessionHead failed: %v", err)
	}

	if head != stepHash {
		t.Errorf("Head mismatch: got %s, want %s", head, stepHash)
	}
}

func TestListAllSessions(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	// Initially no sessions
	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions initially, got %d", len(sessions))
	}

	// Create steps in different sessions
	sessionIDs := []string{"session-1", "session-2", "session-3"}
	for _, sessionID := range sessionIDs {
		blobHash, _ := s.WriteBlob([]byte("content"))
		tree := &store.Tree{
			Entries: []store.TreeEntry{
				{Path: "file.txt", Blob: blobHash, Mode: 0o644},
			},
		}
		treeHash, _ := s.WriteTree(tree)

		step := &store.Step{
			SessionID:      sessionID,
			TimestampNanos: time.Now().UnixNano(),
			Tree:           treeHash,
			Cause: store.Cause{
				ToolName:  "Write",
				ToolUseID: "tool_1",
			},
		}

		stepHash, _ := s.WriteStep(step)
		if err := idx.IndexStep(stepHash, step, tree); err != nil {
			t.Fatalf("IndexStep failed: %v", err)
		}
	}

	// List all sessions
	sessions, err = idx.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}

	if len(sessions) != 3 {
		t.Fatalf("Expected 3 sessions, got %d", len(sessions))
	}

	// Verify sessions are ordered by last_seen_at DESC
	for i := 0; i < len(sessions)-1; i++ {
		if sessions[i].LastSeenAt.Before(sessions[i+1].LastSeenAt) {
			t.Error("Sessions not ordered by LastSeenAt DESC")
		}
	}
}

func TestForkDetection(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	// Create step in session-1
	blobHash1, _ := s.WriteBlob([]byte("content1"))
	tree1 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "file.txt", Blob: blobHash1, Mode: 0o644},
		},
	}
	treeHash1, _ := s.WriteTree(tree1)

	step1 := &store.Step{
		SessionID:      "session-1",
		TimestampNanos: time.Now().UnixNano(),
		Tree:           treeHash1,
		Cause: store.Cause{
			ToolName:  "Write",
			ToolUseID: "tool_1",
		},
	}

	step1Hash, _ := s.WriteStep(step1)
	if err := idx.IndexStep(step1Hash, step1, tree1); err != nil {
		t.Fatalf("IndexStep 1 failed: %v", err)
	}

	// Create step in session-2 with parent from session-1 (fork)
	blobHash2, _ := s.WriteBlob([]byte("content2"))
	tree2 := &store.Tree{
		Entries: []store.TreeEntry{
			{Path: "file.txt", Blob: blobHash2, Mode: 0o644},
		},
	}
	treeHash2, _ := s.WriteTree(tree2)

	step2 := &store.Step{
		Parent:         step1Hash, // Parent from different session!
		SessionID:      "session-2",
		TimestampNanos: time.Now().UnixNano(),
		Tree:           treeHash2,
		Cause: store.Cause{
			ToolName:  "Edit",
			ToolUseID: "tool_2",
		},
	}

	step2Hash, _ := s.WriteStep(step2)
	if err := idx.IndexStep(step2Hash, step2, tree2); err != nil {
		t.Fatalf("IndexStep 2 failed: %v", err)
	}

	// Verify fork was detected
	sessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("ListAllSessions failed: %v", err)
	}

	var session2 *SessionInfo
	for i := range sessions {
		if sessions[i].ID == "session-2" {
			session2 = &sessions[i]
			break
		}
	}

	if session2 == nil {
		t.Fatal("session-2 not found")
	}

	if session2.ForkedFromSession != "session-1" {
		t.Errorf("ForkedFromSession: got %s, want session-1", session2.ForkedFromSession)
	}

	if session2.ForkedFromStep != step1Hash {
		t.Errorf("ForkedFromStep: got %s, want %s", session2.ForkedFromStep, step1Hash)
	}

	if session2.ForkDetectedAt == nil {
		t.Error("ForkDetectedAt should not be nil")
	}
}

func TestSessionLastProcessedMessage(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	sessionID := "test-session"

	// New session should return empty
	lastMsgID, lastTranscript, err := idx.SessionLastProcessedMessage(sessionID)
	if err != nil {
		t.Fatalf("SessionLastProcessedMessage failed: %v", err)
	}
	if lastMsgID != "" {
		t.Errorf("Expected empty lastMsgID, got %s", lastMsgID)
	}
	if lastTranscript != "" {
		t.Errorf("Expected empty lastTranscript, got %s", lastTranscript)
	}

	// Update last processed
	testMsgID := "msg_abc123"
	testTranscriptHash := store.Hash("transcript_hash_xyz")
	err = idx.UpdateSessionLastProcessed(sessionID, testMsgID, testTranscriptHash)
	if err != nil {
		t.Fatalf("UpdateSessionLastProcessed failed: %v", err)
	}

	// Verify it was saved
	lastMsgID, lastTranscript, err = idx.SessionLastProcessedMessage(sessionID)
	if err != nil {
		t.Fatalf("SessionLastProcessedMessage failed: %v", err)
	}
	if lastMsgID != testMsgID {
		t.Errorf("lastMsgID: got %s, want %s", lastMsgID, testMsgID)
	}
	if lastTranscript != testTranscriptHash {
		t.Errorf("lastTranscript: got %s, want %s", lastTranscript, testTranscriptHash)
	}
}

func TestListSteps_Limit(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	sessionID := "test-session-limit"

	// Create 10 steps
	for i := 0; i < 10; i++ {
		blobHash, _ := s.WriteBlob([]byte("content"))
		tree := &store.Tree{
			Entries: []store.TreeEntry{
				{Path: "file.txt", Blob: blobHash, Mode: 0o644},
			},
		}
		treeHash, _ := s.WriteTree(tree)

		step := &store.Step{
			SessionID:      sessionID,
			TimestampNanos: time.Now().UnixNano() + int64(i*1000),
			Tree:           treeHash,
			Cause: store.Cause{
				ToolName:  "Write",
				ToolUseID: "tool_" + string(rune('0'+i)),
			},
		}

		stepHash, _ := s.WriteStep(step)
		if err := idx.IndexStep(stepHash, step, tree); err != nil {
			t.Fatalf("IndexStep %d failed: %v", i, err)
		}
	}

	// List with limit 5
	steps, err := idx.ListSteps(sessionID, 5)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(steps) != 5 {
		t.Errorf("Expected 5 steps, got %d", len(steps))
	}

	// List all
	stepsAll, err := idx.ListSteps(sessionID, 100)
	if err != nil {
		t.Fatalf("ListSteps failed: %v", err)
	}

	if len(stepsAll) != 10 {
		t.Errorf("Expected 10 steps, got %d", len(stepsAll))
	}
}

func TestInsertMessage(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	msg := Message{
		ID:          "msg_123",
		SessionID:   "test-session",
		StepID:      "step_abc",
		SeqNum:      0,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "user",
		ContentText: "Hello, world!",
	}

	err = idx.InsertMessage(msg)
	if err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Verify message was inserted
	var count int
	err = idx.db.QueryRow("SELECT COUNT(*) FROM messages WHERE id = ?", msg.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 message, got %d", count)
	}
}

func TestGetNextMessageSeq(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	sessionID := "test-session"

	// First message should be seq 0
	seq, err := idx.GetNextMessageSeq(sessionID)
	if err != nil {
		t.Fatalf("GetNextMessageSeq failed: %v", err)
	}
	if seq != 0 {
		t.Errorf("Expected seq 0, got %d", seq)
	}

	// Insert a message
	msg := Message{
		ID:          "msg_1",
		SessionID:   sessionID,
		SeqNum:      0,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "user",
		ContentText: "Test",
	}
	if err := idx.InsertMessage(msg); err != nil {
		t.Fatalf("InsertMessage failed: %v", err)
	}

	// Next seq should be 1
	seq, err = idx.GetNextMessageSeq(sessionID)
	if err != nil {
		t.Fatalf("GetNextMessageSeq failed: %v", err)
	}
	if seq != 1 {
		t.Errorf("Expected seq 1, got %d", seq)
	}
}

func TestGetMessagesForStep(t *testing.T) {
	tmpDir := t.TempDir()
	s, err := store.Init(tmpDir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer idx.Close()

	stepID := store.Hash("step_abc")

	// Insert messages linked to step
	messages := []Message{
		{
			ID:          "msg_1",
			SessionID:   "test-session",
			StepID:      string(stepID),
			SeqNum:      0,
			Timestamp:   time.Now().UnixNano(),
			MessageType: "user",
			ContentText: "First message",
		},
		{
			ID:          "msg_2",
			SessionID:   "test-session",
			StepID:      string(stepID),
			SeqNum:      1,
			Timestamp:   time.Now().UnixNano(),
			MessageType: "assistant",
			ContentText: "Second message",
		},
	}

	for _, msg := range messages {
		if err := idx.InsertMessage(msg); err != nil {
			t.Fatalf("InsertMessage failed: %v", err)
		}
	}

	// Get messages for step
	retrieved, err := idx.GetMessagesForStep(stepID)
	if err != nil {
		t.Fatalf("GetMessagesForStep failed: %v", err)
	}

	if len(retrieved) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(retrieved))
	}

	// Verify order (by seq_num)
	if retrieved[0].SeqNum != 0 {
		t.Errorf("First message seq: got %d, want 0", retrieved[0].SeqNum)
	}
	if retrieved[1].SeqNum != 1 {
		t.Errorf("Second message seq: got %d, want 1", retrieved[1].SeqNum)
	}
}

func TestOpen_MigratesLegacySchema(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(s.Root, "index.db"))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := db.Exec(legacySchema); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("open migrated index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	for _, column := range []struct {
		table string
		name  string
	}{
		{"steps", "origin"},
		{"steps", "turn_id"},
		{"messages", "turn_id"},
		{"messages", "processed_at"},
		{"sessions", "model"},
		{"sessions", "permission_mode"},
		{"sessions", "transcript_path"},
	} {
		exists, err := columnExists(idx.db, column.table, column.name)
		if err != nil {
			t.Fatalf("check column %s.%s: %v", column.table, column.name, err)
		}
		if !exists {
			t.Fatalf("expected migrated column %s.%s", column.table, column.name)
		}
	}

	if err := idx.AppendMessage(Message{
		ID:          "msg-1",
		SessionID:   "codex_cli:session",
		TurnID:      "turn-1",
		Timestamp:   time.Now().UnixNano(),
		MessageType: "user",
		ContentText: "hello",
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}

	blobHash, err := s.WriteBlob([]byte("hello\n"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "hello.txt", Blob: blobHash}}}
	treeHash, err := s.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	step := &store.Step{
		Tree:           treeHash,
		Causes:         []store.Cause{{ToolName: "Write", ToolUseID: "tool-1"}},
		SessionID:      "codex_cli:session",
		Origin:         "codex_cli",
		TurnID:         "turn-1",
		TimestampNanos: time.Now().UnixNano(),
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
	if len(steps) != 1 || steps[0].Origin != "codex_cli" || steps[0].TurnID != "turn-1" {
		t.Fatalf("unexpected migrated step: %#v", steps)
	}
}

func TestOpen_MigratesPreForkLegacySchema(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}

	db, err := sql.Open("sqlite", filepath.Join(s.Root, "index.db"))
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := db.Exec(preForkLegacySchema); err != nil {
		t.Fatalf("create pre-fork legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close raw db: %v", err)
	}

	idx, err := Open(s)
	if err != nil {
		t.Fatalf("open migrated index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	for _, column := range []struct {
		table string
		name  string
	}{
		{"steps", "origin"},
		{"steps", "turn_id"},
		{"messages", "turn_id"},
		{"messages", "processed_at"},
		{"sessions", "model"},
		{"sessions", "permission_mode"},
		{"sessions", "transcript_path"},
		{"sessions", "forked_from_session"},
		{"sessions", "forked_from_step"},
		{"sessions", "fork_detected_at"},
	} {
		exists, err := columnExists(idx.db, column.table, column.name)
		if err != nil {
			t.Fatalf("check column %s.%s: %v", column.table, column.name, err)
		}
		if !exists {
			t.Fatalf("expected migrated column %s.%s", column.table, column.name)
		}
	}

	if err := idx.UpsertSession(SessionUpdate{
		ID:             "codex_cli:session",
		Origin:         "codex_cli",
		Model:          "gpt-5.5",
		PermissionMode: "bypassPermissions",
		TranscriptPath: "/tmp/session.jsonl",
	}); err != nil {
		t.Fatalf("upsert migrated session: %v", err)
	}
}

func TestSessions_NoHeadSessionsAreNotDefaultable(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	idx, err := Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	if err := idx.UpsertSession(SessionUpdate{ID: "codex_cli:no-head", Origin: "codex_cli"}); err != nil {
		t.Fatalf("upsert no-head session: %v", err)
	}

	allSessions, err := idx.ListAllSessions()
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(allSessions) != 1 {
		t.Fatalf("expected stored no-head session, got %d", len(allSessions))
	}

	headedSessions, err := idx.ListHeadedSessions()
	if err != nil {
		t.Fatalf("list headed sessions: %v", err)
	}
	if len(headedSessions) != 0 {
		t.Fatalf("no-head sessions should not be defaultable: %#v", headedSessions)
	}

	if _, err := idx.SessionHead("codex_cli:no-head"); !errors.Is(err, ErrSessionHasNoSteps) {
		t.Fatalf("SessionHead error = %v, want ErrSessionHasNoSteps", err)
	}
}

func TestRenameSession_MovesLegacyRowsToCanonicalID(t *testing.T) {
	root := t.TempDir()
	s, err := store.Init(root)
	if err != nil {
		t.Fatalf("init store: %v", err)
	}
	idx, err := Open(s)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	defer func() { _ = idx.Close() }()

	oldID := "claude-legacy"
	newID := "claude_code:claude-legacy"
	if err := idx.UpsertSession(SessionUpdate{ID: oldID, Origin: "claude_code"}); err != nil {
		t.Fatalf("upsert old session: %v", err)
	}
	if err := idx.AppendMessage(Message{
		ID:          "msg-1",
		SessionID:   oldID,
		Timestamp:   time.Now().UnixNano(),
		MessageType: "user",
		ContentText: "hello",
	}); err != nil {
		t.Fatalf("append message: %v", err)
	}

	blobHash, err := s.WriteBlob([]byte("hello\n"))
	if err != nil {
		t.Fatalf("write blob: %v", err)
	}
	tree := &store.Tree{Entries: []store.TreeEntry{{Path: "hello.txt", Blob: blobHash}}}
	treeHash, err := s.WriteTree(tree)
	if err != nil {
		t.Fatalf("write tree: %v", err)
	}
	step := &store.Step{
		Tree:           treeHash,
		Cause:          store.Cause{ToolName: "Write", ToolUseID: "tool-1"},
		SessionID:      oldID,
		Origin:         "claude_code",
		TimestampNanos: time.Now().UnixNano(),
	}
	stepHash, err := s.WriteStep(step)
	if err != nil {
		t.Fatalf("write step: %v", err)
	}
	if err := idx.IndexStep(stepHash, step, tree); err != nil {
		t.Fatalf("index step: %v", err)
	}
	if err := idx.InsertJSONLSnapshot(oldID, 1, blobHash); err != nil {
		t.Fatalf("insert jsonl snapshot: %v", err)
	}
	if err := idx.UpdateSessionLastProcessed(oldID, "msg-1", blobHash); err != nil {
		t.Fatalf("update transcript state: %v", err)
	}

	changed, err := idx.RenameSession(oldID, newID, "claude_code")
	if err != nil {
		t.Fatalf("rename session: %v", err)
	}
	if !changed {
		t.Fatal("expected rows to be renamed")
	}

	steps, err := idx.ListSteps(newID, 10)
	if err != nil {
		t.Fatalf("list renamed steps: %v", err)
	}
	if len(steps) != 1 || steps[0].Hash != stepHash {
		t.Fatalf("unexpected renamed steps: %#v", steps)
	}
	messages, err := idx.GetAllPendingMessages(newID)
	if err != nil {
		t.Fatalf("get renamed messages: %v", err)
	}
	if len(messages) != 1 || messages[0].SessionID != newID {
		t.Fatalf("unexpected renamed messages: %#v", messages)
	}
	if oldMessages, err := idx.GetAllPendingMessages(oldID); err != nil || len(oldMessages) != 0 {
		t.Fatalf("old messages remain: messages=%#v err=%v", oldMessages, err)
	}
	if _, _, err := idx.SessionLastProcessedMessage(newID); err != nil {
		t.Fatalf("renamed transcript state missing: %v", err)
	}
}

const legacySchema = `
CREATE TABLE steps (
	id          TEXT PRIMARY KEY,
	parent_id   TEXT,
	session_id  TEXT NOT NULL,
	agent_id    TEXT,
	ts_nanos    INTEGER NOT NULL,
	tool_name   TEXT NOT NULL,
	tool_use_id TEXT NOT NULL,
	tree_hash   TEXT NOT NULL,
	transcript_hash TEXT
);
CREATE TABLE step_files (
	step_id   TEXT NOT NULL,
	path      TEXT NOT NULL,
	blob_hash TEXT NOT NULL,
	PRIMARY KEY (step_id, path)
);
CREATE TABLE sessions (
	id            TEXT PRIMARY KEY,
	origin        TEXT NOT NULL,
	started_at    INTEGER NOT NULL,
	last_seen_at  INTEGER NOT NULL,
	head_step_id  TEXT,
	forked_from_session TEXT,
	forked_from_step    TEXT,
	fork_detected_at    INTEGER
);
CREATE TABLE session_transcript (
	session_id           TEXT PRIMARY KEY,
	last_message_id      TEXT NOT NULL,
	last_transcript_hash TEXT NOT NULL
);
CREATE TABLE messages (
	id              TEXT PRIMARY KEY,
	session_id      TEXT NOT NULL,
	step_id         TEXT,
	seq_num         INTEGER NOT NULL,
	timestamp       INTEGER NOT NULL,
	message_type    TEXT NOT NULL,
	content_text    TEXT,
	tool_name       TEXT,
	tool_use_id     TEXT,
	tool_input      TEXT,
	tool_output     TEXT
);
CREATE TABLE jsonl_snapshots (
	session_id      TEXT NOT NULL,
	captured_at     INTEGER NOT NULL,
	jsonl_blob      TEXT NOT NULL,
	PRIMARY KEY (session_id, captured_at)
);
`

const preForkLegacySchema = `
CREATE TABLE steps (
	id          TEXT PRIMARY KEY,
	parent_id   TEXT,
	session_id  TEXT NOT NULL,
	agent_id    TEXT,
	ts_nanos    INTEGER NOT NULL,
	tool_name   TEXT NOT NULL,
	tool_use_id TEXT NOT NULL,
	tree_hash   TEXT NOT NULL,
	transcript_hash TEXT
);
CREATE TABLE step_files (
	step_id   TEXT NOT NULL,
	path      TEXT NOT NULL,
	blob_hash TEXT NOT NULL,
	PRIMARY KEY (step_id, path)
);
CREATE TABLE sessions (
	id            TEXT PRIMARY KEY,
	origin        TEXT NOT NULL,
	started_at    INTEGER NOT NULL,
	last_seen_at  INTEGER NOT NULL,
	head_step_id  TEXT
);
CREATE TABLE session_transcript (
	session_id           TEXT PRIMARY KEY,
	last_message_id      TEXT NOT NULL,
	last_transcript_hash TEXT NOT NULL
);
CREATE TABLE messages (
	id              TEXT PRIMARY KEY,
	session_id      TEXT NOT NULL,
	step_id         TEXT,
	seq_num         INTEGER NOT NULL,
	timestamp       INTEGER NOT NULL,
	message_type    TEXT NOT NULL,
	content_text    TEXT,
	tool_name       TEXT,
	tool_use_id     TEXT,
	tool_input      TEXT,
	tool_output     TEXT
);
CREATE TABLE jsonl_snapshots (
	session_id      TEXT NOT NULL,
	captured_at     INTEGER NOT NULL,
	jsonl_blob      TEXT NOT NULL,
	PRIMARY KEY (session_id, captured_at)
);
`
