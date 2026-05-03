package store

import (
	"encoding/json"
	"fmt"
)

// Transcript is a linked-list node in the conversation chain
type Transcript struct {
	Prev        Hash   `json:"prev"`         // Previous transcript hash (empty for first)
	NewMessages []Hash `json:"new_messages"` // Message blob hashes added at this step
}

// WriteTranscript writes a transcript node to the object store
func (s *Store) WriteTranscript(prev Hash, newMessages []Hash) (Hash, error) {
	t := &Transcript{
		Prev:        prev,
		NewMessages: newMessages,
	}
	data, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("marshal transcript: %w", err)
	}
	return s.WriteBlob(data)
}

// ReadTranscript reads a transcript node from the object store
func (s *Store) ReadTranscript(h Hash) (*Transcript, error) {
	data, err := s.ReadBlob(h)
	if err != nil {
		return nil, err
	}
	var t Transcript
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("decode transcript: %w", err)
	}
	return &t, nil
}

// ReconstructTranscript walks the chain and returns all messages in chronological order
func (s *Store) ReconstructTranscript(head Hash) ([]json.RawMessage, error) {
	if head == "" {
		return []json.RawMessage{}, nil
	}

	// Walk backward collecting batches
	var batches [][]Hash
	cur := head
	for cur != "" {
		t, err := s.ReadTranscript(cur)
		if err != nil {
			return nil, fmt.Errorf("read transcript %s: %w", cur, err)
		}
		batches = append(batches, t.NewMessages)
		cur = t.Prev
	}

	// Reverse and dereference
	var msgs []json.RawMessage
	for i := len(batches) - 1; i >= 0; i-- {
		for _, h := range batches[i] {
			data, err := s.ReadBlob(h)
			if err != nil {
				return nil, fmt.Errorf("read message blob %s: %w", h, err)
			}
			msgs = append(msgs, json.RawMessage(data))
		}
	}

	return msgs, nil
}
