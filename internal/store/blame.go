package store

import (
	"encoding/json"
	"fmt"

	"github.com/regent-vcs/regent/internal/diff"
)

// BlameMap tracks per-line provenance for a file
type BlameMap struct {
	Lines []Hash `json:"lines"` // one step hash per line
}

// ReadBlame reads a blame map from the object store
func (s *Store) ReadBlame(h Hash) (*BlameMap, error) {
	data, err := s.ReadBlob(h)
	if err != nil {
		return nil, err
	}

	var bm BlameMap
	if err := json.Unmarshal(data, &bm); err != nil {
		return nil, fmt.Errorf("decode blame map: %w", err)
	}

	return &bm, nil
}

// WriteBlame writes a blame map to the object store
func (s *Store) WriteBlame(bm *BlameMap) (Hash, error) {
	data, err := json.Marshal(bm)
	if err != nil {
		return "", fmt.Errorf("encode blame map: %w", err)
	}
	return s.WriteBlob(data)
}

// ComputeBlame builds a new blame map given old/new content and old blame
// Lines unchanged inherit old attribution; modified lines get currentStep
func ComputeBlame(oldContent, newContent []byte, oldBlame *BlameMap, currentStep Hash) *BlameMap {
	ops := diff.LineDiff(oldContent, newContent)

	newBlame := &BlameMap{Lines: make([]Hash, 0)}

	for _, op := range ops {
		switch op.Tag {
		case "equal":
			// Preserved lines keep their old attribution
			for i := op.I1; i < op.I2; i++ {
				if oldBlame != nil && i < len(oldBlame.Lines) {
					newBlame.Lines = append(newBlame.Lines, oldBlame.Lines[i])
				} else {
					// File existed but had no blame (pre-Phase 3)
					newBlame.Lines = append(newBlame.Lines, currentStep)
				}
			}

		case "insert", "replace":
			// New or modified lines belong to currentStep
			for j := op.J1; j < op.J2; j++ {
				newBlame.Lines = append(newBlame.Lines, currentStep)
			}

		case "delete":
			// Deleted lines contribute nothing to new blame
		}
	}

	return newBlame
}
