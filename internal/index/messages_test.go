package index

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/regent-vcs/regent/internal/store"
)

func TestAppendToolUseMessages_IsIdempotentUnderConcurrency(t *testing.T) {
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

	const sessionID = "codex_cli:session"
	const turnID = "turn-1"
	const toolUseID = "tool-1"

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	inserted := make(chan bool, 16)
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			now := time.Now().UnixNano()
			ok, err := idx.AppendToolUseMessages(Message{
				ID:          fmt.Sprintf("call-%d", i),
				SessionID:   sessionID,
				TurnID:      turnID,
				Timestamp:   now,
				MessageType: "tool_call",
				ToolName:    "Write",
				ToolUseID:   toolUseID,
				ToolInput:   "args",
			}, Message{
				ID:          fmt.Sprintf("result-%d", i),
				SessionID:   sessionID,
				TurnID:      turnID,
				Timestamp:   now + 1,
				MessageType: "tool_result",
				ToolName:    "Write",
				ToolUseID:   toolUseID,
				ToolOutput:  "result",
			})
			if err != nil {
				errs <- err
				return
			}
			inserted <- ok
		}(i)
	}
	wg.Wait()
	close(errs)
	close(inserted)

	for err := range errs {
		t.Fatalf("append tool use messages: %v", err)
	}
	insertedCount := 0
	for ok := range inserted {
		if ok {
			insertedCount++
		}
	}
	if insertedCount != 1 {
		t.Fatalf("inserted count = %d, want 1", insertedCount)
	}

	messages, err := idx.GetPendingMessages(sessionID, turnID)
	if err != nil {
		t.Fatalf("get pending messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want call/result pair", len(messages))
	}
	if messages[0].MessageType != "tool_call" || messages[1].MessageType != "tool_result" {
		t.Fatalf("unexpected messages: %#v", messages)
	}
}

func TestAppendToolUseMessages_RejectsConflictingDuplicate(t *testing.T) {
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

	call := Message{
		ID:          "call-1",
		SessionID:   "codex_cli:session",
		TurnID:      "turn-1",
		Timestamp:   time.Now().UnixNano(),
		MessageType: "tool_call",
		ToolName:    "Write",
		ToolUseID:   "tool-1",
		ToolInput:   "args-1",
	}
	result := Message{
		ID:          "result-1",
		SessionID:   call.SessionID,
		TurnID:      call.TurnID,
		Timestamp:   call.Timestamp + 1,
		MessageType: "tool_result",
		ToolName:    call.ToolName,
		ToolUseID:   call.ToolUseID,
		ToolOutput:  "result-1",
	}
	ok, err := idx.AppendToolUseMessages(call, result)
	if err != nil {
		t.Fatalf("append initial tool use: %v", err)
	}
	if !ok {
		t.Fatal("expected initial tool use to insert")
	}

	conflictingCall := call
	conflictingCall.ID = "call-2"
	conflictingCall.ToolInput = "args-2"
	conflictingResult := result
	conflictingResult.ID = "result-2"

	ok, err = idx.AppendToolUseMessages(conflictingCall, conflictingResult)
	if err == nil {
		t.Fatal("expected conflicting duplicate to fail")
	}
	if ok {
		t.Fatal("conflicting duplicate should not report insertion")
	}
	if !strings.Contains(err.Error(), "existing tool_call payload differs") {
		t.Fatalf("unexpected duplicate error: %v", err)
	}

	messages, err := idx.GetPendingMessages(call.SessionID, call.TurnID)
	if err != nil {
		t.Fatalf("get pending messages: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("conflicting duplicate inserted messages: %#v", messages)
	}
}
