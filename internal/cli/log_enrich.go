package cli

import (
	"encoding/json"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
)

// enrichSteps adds files, args, results, and duration to each step
func enrichSteps(s *store.Store, steps []index.StepInfo) ([]EnrichedStep, error) {
	if len(steps) == 0 {
		return []EnrichedStep{}, nil
	}

	enriched := make([]EnrichedStep, len(steps))

	for i, stepInfo := range steps {
		// Read the full step to get cause details
		step, err := s.ReadStep(stepInfo.Hash)
		if err != nil {
			// If we can't read the step, just include basic info
			enriched[i] = EnrichedStep{
				StepInfo: stepInfo,
			}
			continue
		}

		// Fetch files from tree
		files := []string{}
		if stepInfo.TreeHash != "" {
			tree, err := s.ReadTree(stepInfo.TreeHash)
			if err == nil {
				files = extractPrimaryFiles(tree, stepInfo.ToolName)
			}
		}

		// Fetch tool args and results
		var args, result json.RawMessage
		if step.Cause.ArgsBlob != "" {
			argsData, err := s.ReadBlob(step.Cause.ArgsBlob)
			if err == nil {
				args = json.RawMessage(argsData)
			}
		}
		if step.Cause.ResultBlob != "" {
			resultData, err := s.ReadBlob(step.Cause.ResultBlob)
			if err == nil {
				result = json.RawMessage(resultData)
			}
		}

		// Calculate duration (time since previous step)
		var duration time.Duration
		if i < len(steps)-1 {
			duration = stepInfo.Timestamp.Sub(steps[i+1].Timestamp)
		}

		enriched[i] = EnrichedStep{
			StepInfo: stepInfo,
			Files:    files,
			Args:     args,
			Result:   result,
			Duration: duration,
		}
	}

	return enriched, nil
}

// extractPrimaryFiles gets the most relevant files from a tree based on tool type
func extractPrimaryFiles(tree *store.Tree, toolName string) []string {
	if tree == nil || len(tree.Entries) == 0 {
		return []string{}
	}

	// For now, just return files (limit to first 3 to keep output clean)
	files := []string{}
	for _, entry := range tree.Entries {
		files = append(files, entry.Path)
		if len(files) >= 3 {
			break
		}
	}

	return files
}
