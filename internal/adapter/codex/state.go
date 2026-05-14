package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/regent-vcs/regent/internal/store"
)

const (
	defaultPollIntervalSeconds = 2
	adapterStateRelativePath   = "adapters/codex/state.json"
)

func loadState(s *store.Store, projectRoot string) (*AdapterState, error) {
	statePath := filepath.Join(s.Root, adapterStateRelativePath)
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &AdapterState{
				ProjectRoot: projectRoot,
				Checkpoints: map[string]Checkpoint{},
			}, nil
		}
		return nil, fmt.Errorf("read codex adapter state: %w", err)
	}

	var state AdapterState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode codex adapter state: %w", err)
	}
	if state.Checkpoints == nil {
		state.Checkpoints = map[string]Checkpoint{}
	}
	if state.ProjectRoot == "" {
		state.ProjectRoot = projectRoot
	}

	return &state, nil
}

func saveState(s *store.Store, state *AdapterState) error {
	statePath := filepath.Join(s.Root, adapterStateRelativePath)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return fmt.Errorf("create codex adapter state dir: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode codex adapter state: %w", err)
	}

	return os.WriteFile(statePath, data, 0o644)
}
