package cli

import (
	"encoding/json"
	"os"
	"strings"
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

		// Extract files from tool arguments (what the tool actually touched)
		files := extractFilesFromToolArgs(stepInfo.ToolName, args)

		// Fetch conversation transcript
		var messages []json.RawMessage
		if step.Transcript != "" {
			transcriptMsgs, err := s.ReconstructTranscript(step.Transcript)
			if err == nil {
				messages = transcriptMsgs
			}
			// Silently skip if transcript unavailable (don't fail the whole log)
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
			Messages: messages,
		}
	}

	return enriched, nil
}

// extractPrimaryFiles gets the most relevant files from a tree based on tool type
func extractPrimaryFiles(tree *store.Tree, toolName string) []string {
	// Don't show files from tree snapshot - it's misleading
	// The tree contains ALL files in workspace, not just what the tool touched
	// TODO: Compute diff against parent tree to show actual changes
	return []string{}
}

// extractFilesFromToolArgs extracts file paths from tool arguments
// This shows what the tool actually operated on, not all files in the workspace
func extractFilesFromToolArgs(toolName string, args json.RawMessage) []string {
	if len(args) == 0 || string(args) == "null" {
		return []string{}
	}

	var argsMap map[string]interface{}
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return []string{}
	}

	files := []string{}

	switch toolName {
	case "Write", "Edit", "Read":
		// These tools have a file_path argument
		if filePath, ok := argsMap["file_path"].(string); ok && filePath != "" {
			// Make path relative to current directory if it's absolute
			files = append(files, makeRelativePath(filePath))
		}
	case "Bash":
		// Bash doesn't directly specify files, leave empty
		// Could potentially parse from command, but that's fragile
	default:
		// Unknown tool, try file_path as fallback
		if filePath, ok := argsMap["file_path"].(string); ok && filePath != "" {
			files = append(files, makeRelativePath(filePath))
		}
	}

	return files
}

// makeRelativePath converts absolute paths to relative paths from cwd
func makeRelativePath(path string) string {
	// If path doesn't start with /, it's already relative
	if len(path) == 0 || path[0] != '/' {
		return path
	}

	// Try to get cwd
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}

	// If path is under cwd, make it relative
	if strings.HasPrefix(path, cwd+"/") {
		return strings.TrimPrefix(path, cwd+"/")
	}

	// Otherwise return as-is
	return path
}
