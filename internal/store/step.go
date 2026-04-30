package store

import (
	"encoding/json"
	"fmt"
)

// Cause describes what triggered this step
type Cause struct {
	ToolUseID  string `json:"tool_use_id"`
	ToolName   string `json:"tool_name"`
	ArgsBlob   Hash   `json:"args_blob,omitempty"`
	ResultBlob Hash   `json:"result_blob,omitempty"`
}

// Effect describes side effects of the step
type Effect struct {
	Kind       string `json:"kind"`        // "http_call", "db_write", "process_exec", ...
	Descriptor string `json:"descriptor"`  // human-readable summary
}

// Step is the equivalent of a git commit
type Step struct {
	Parent          Hash     `json:"parent,omitempty"`
	SecondaryParent Hash     `json:"secondary_parent,omitempty"` // for sub-agent merges
	Tree            Hash     `json:"tree"`
	Transcript      Hash     `json:"transcript,omitempty"`
	Config          Hash     `json:"config,omitempty"` // system prompt + tools + memory hash
	Cause           Cause    `json:"cause"`
	SessionID       string   `json:"session_id"`
	AgentID         string   `json:"agent_id,omitempty"`
	TimestampNanos  int64    `json:"ts"`
	Effects         []Effect `json:"effects,omitempty"`
}

// WriteStep writes a step to the object store
func (s *Store) WriteStep(step *Step) (Hash, error) {
	data, err := json.Marshal(step)
	if err != nil {
		return "", fmt.Errorf("marshal step: %w", err)
	}

	return s.WriteBlob(data)
}

// ReadStep reads a step from the object store
func (s *Store) ReadStep(h Hash) (*Step, error) {
	data, err := s.ReadBlob(h)
	if err != nil {
		return nil, err
	}

	var step Step
	if err := json.Unmarshal(data, &step); err != nil {
		return nil, fmt.Errorf("unmarshal step %s: %w", h, err)
	}

	return &step, nil
}

// WalkAncestors walks the step's ancestor chain, calling fn for each step
// Stops when fn returns an error or when a step has no parent
func (s *Store) WalkAncestors(h Hash, fn func(*Step) error) error {
	for h != "" {
		step, err := s.ReadStep(h)
		if err != nil {
			return err
		}

		if err := fn(step); err != nil {
			return err
		}

		h = step.Parent
	}
	return nil
}
