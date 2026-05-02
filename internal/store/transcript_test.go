package store

import (
	"encoding/json"
	"testing"
)

func TestTranscriptChain_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)

	msgs, err := s.ReconstructTranscript("")
	if err != nil {
		t.Fatalf("Reconstruct empty: %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(msgs))
	}
}

func TestTranscriptChain_SingleNode(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)

	// Write message blobs
	msg1Hash, _ := s.WriteBlob([]byte(`{"type":"user","content":"hello"}`))

	// Write transcript node
	tHash, err := s.WriteTranscript("", []Hash{msg1Hash})
	if err != nil {
		t.Fatalf("WriteTranscript: %v", err)
	}

	// Reconstruct
	msgs, err := s.ReconstructTranscript(tHash)
	if err != nil {
		t.Fatalf("Reconstruct: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}
}

func TestTranscriptChain_MultipleNodes(t *testing.T) {
	tmpDir := t.TempDir()
	s, _ := Init(tmpDir)

	// Node 1: user message
	msg1, _ := s.WriteBlob([]byte(`{"type":"user","content":"step 1"}`))
	t1, _ := s.WriteTranscript("", []Hash{msg1})

	// Node 2: assistant + tool use
	msg2, _ := s.WriteBlob([]byte(`{"type":"assistant","content":"ok"}`))
	msg3, _ := s.WriteBlob([]byte(`{"type":"tool_use","name":"Write"}`))
	t2, _ := s.WriteTranscript(t1, []Hash{msg2, msg3})

	// Reconstruct from head
	msgs, err := s.ReconstructTranscript(t2)
	if err != nil {
		t.Fatalf("Reconstruct: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(msgs))
	}

	// Verify order (chronological)
	var m1, m2, m3 map[string]interface{}
	json.Unmarshal(msgs[0], &m1)
	json.Unmarshal(msgs[1], &m2)
	json.Unmarshal(msgs[2], &m3)

	if m1["content"] != "step 1" {
		t.Errorf("Message 1 out of order")
	}
	if m2["content"] != "ok" {
		t.Errorf("Message 2 out of order")
	}
	if m3["name"] != "Write" {
		t.Errorf("Message 3 out of order")
	}
}
