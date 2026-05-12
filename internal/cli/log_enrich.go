package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/treediff"
)

// enrichSteps adds files, args, results, duration, and optionally graph rendering to each step
func enrichSteps(s *store.Store, steps []index.StepInfo, computeFileDiffs bool, renderGraph bool) ([]EnrichedStep, error) {
	if len(steps) == 0 {
		return []EnrichedStep{}, nil
	}

	// Open index for reading messages
	idx, err := index.Open(s)
	if err != nil {
		return nil, err
	}
	defer func() { _ = idx.Close() }()

	enriched := make([]EnrichedStep, len(steps))

	// Render graph if requested
	var graphPrefixes []string
	var graphWarning string
	if renderGraph {
		var err error
		graphPrefixes, err = RenderGraph(steps, s)
		if err != nil {
			graphWarning = fmt.Sprintf("render graph: %v", err)
			graphPrefixes = nil
		}
	}

	for i, stepInfo := range steps {
		var warnings []string
		if i == 0 && graphWarning != "" {
			warnings = append(warnings, graphWarning)
		}

		// Read the full step to get cause details
		step, err := s.ReadStep(stepInfo.Hash)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read step %s: %v", stepInfo.Hash, err))
			enriched[i] = EnrichedStep{
				StepInfo: stepInfo,
				Warnings: warnings,
			}
			continue
		}

		causes := step.Causes
		if len(causes) == 0 && step.Cause.ToolName != "" {
			causes = []store.Cause{step.Cause}
		}

		enrichedCauses := make([]EnrichedCause, 0, len(causes))
		filesByPath := map[string]bool{}
		var args, result json.RawMessage
		for i, cause := range causes {
			var causeArgs, causeResult json.RawMessage
			if cause.ArgsBlob != "" {
				argsData, err := s.ReadBlob(cause.ArgsBlob)
				if err == nil {
					causeArgs = rawJSONForOutput(argsData)
				} else {
					warnings = append(warnings, fmt.Sprintf("read tool args blob %s: %v", cause.ArgsBlob, err))
				}
			}
			if cause.ResultBlob != "" {
				resultData, err := s.ReadBlob(cause.ResultBlob)
				if err == nil {
					causeResult = rawJSONForOutput(resultData)
				} else {
					warnings = append(warnings, fmt.Sprintf("read tool result blob %s: %v", cause.ResultBlob, err))
				}
			}
			if i == 0 {
				args = causeArgs
				result = causeResult
			}
			for _, file := range extractFilesFromToolArgs(cause.ToolName, causeArgs) {
				filesByPath[file] = true
			}
			enrichedCauses = append(enrichedCauses, EnrichedCause{
				Cause:  cause,
				Args:   causeArgs,
				Result: causeResult,
			})
		}

		files := make([]string, 0, len(filesByPath))
		for file := range filesByPath {
			files = append(files, file)
		}
		sort.Strings(files)

		// Fetch conversation messages from database
		var messages []json.RawMessage
		dbMessages, err := idx.GetMessagesForStep(stepInfo.Hash)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("read indexed messages: %v", err))
		} else if len(dbMessages) > 0 {
			// Group messages: collect tool_use blocks and embed them in assistant message
			var currentContent []map[string]interface{}
			var currentRole string

			for _, msg := range dbMessages {
				if msg.MessageType == "user" {
					// User message - emit directly
					envelope := map[string]interface{}{
						"type": "user",
						"message": map[string]interface{}{
							"role":    "user",
							"content": msg.ContentText,
						},
					}
					msgJSON, _ := json.Marshal(envelope)
					messages = append(messages, msgJSON)

				} else if msg.MessageType == "assistant" {
					// Assistant message - collect content
					currentRole = "assistant"
					if msg.ContentText != "" {
						currentContent = append(currentContent, map[string]interface{}{
							"type": "text",
							"text": msg.ContentText,
						})
					}

				} else if msg.MessageType == "tool_call" {
					// Tool call - add to content blocks
					if msg.ToolInput != "" {
						inputData, err := s.ReadBlob(store.Hash(msg.ToolInput))
						if err != nil {
							warnings = append(warnings, fmt.Sprintf("read tool input blob %s: %v", msg.ToolInput, err))
							continue
						}
						var inputValue interface{}
						if err := json.Unmarshal(rawJSONForOutput(inputData), &inputValue); err != nil {
							warnings = append(warnings, fmt.Sprintf("parse tool input blob %s: %v", msg.ToolInput, err))
							continue
						}

						currentContent = append(currentContent, map[string]interface{}{
							"type":  "tool_use",
							"id":    msg.ToolUseID,
							"name":  msg.ToolName,
							"input": inputValue,
						})
					}
				}
				// tool_result messages are skipped (they're system responses)
			}

			// Emit assistant message with all content blocks
			if currentRole == "assistant" && len(currentContent) > 0 {
				envelope := map[string]interface{}{
					"type": "assistant",
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": currentContent,
					},
				}
				msgJSON, _ := json.Marshal(envelope)
				messages = append(messages, msgJSON)
			}
		} else if step.Transcript != "" {
			// Fallback to old transcript chain for backward compatibility
			transcriptMsgs, err := s.ReconstructTranscript(step.Transcript)
			if err == nil {
				messages = transcriptMsgs
			} else {
				warnings = append(warnings, fmt.Sprintf("reconstruct transcript %s: %v", step.Transcript, err))
			}
		}

		// Calculate duration (time since previous step)
		var duration time.Duration
		if i < len(steps)-1 {
			duration = stepInfo.Timestamp.Sub(steps[i+1].Timestamp)
		}

		// Compute file diffs if requested
		var fileDiffs []FileDiff
		if computeFileDiffs {
			diffs, err := treediff.CompareTreesForDiff(s, stepInfo.ParentHash, stepInfo.Hash)
			if err == nil {
				// Convert treediff.FileDiff to cli.FileDiff
				fileDiffs = make([]FileDiff, len(diffs))
				for j, d := range diffs {
					fileDiffs[j] = FileDiff{
						Path:      d.Path,
						Status:    d.Status,
						Additions: d.Additions,
						Deletions: d.Deletions,
						IsBinary:  d.IsBinary,
					}
				}
			} else {
				warnings = append(warnings, fmt.Sprintf("compute file diff: %v", err))
			}
		}

		// Add graph prefix if available
		var graphPrefix string
		if renderGraph && i < len(graphPrefixes) {
			graphPrefix = graphPrefixes[i]
		}

		enriched[i] = EnrichedStep{
			StepInfo:    stepInfo,
			Causes:      enrichedCauses,
			Files:       files,
			FileDiffs:   fileDiffs,
			Args:        args,
			Result:      result,
			Duration:    duration,
			Messages:    messages,
			GraphPrefix: graphPrefix,
			Warnings:    warnings,
		}
	}

	return enriched, nil
}

func rawJSONForOutput(data []byte) json.RawMessage {
	if json.Valid(data) {
		return json.RawMessage(data)
	}
	encoded, err := json.Marshal(string(data))
	if err != nil {
		return json.RawMessage(`null`)
	}
	return json.RawMessage(encoded)
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
		files = append(files, extractPathFields(argsMap)...)
	}

	return files
}

func extractPathFields(argsMap map[string]interface{}) []string {
	var files []string
	for _, key := range []string{"file_path", "path", "filename"} {
		if path, ok := argsMap[key].(string); ok && path != "" {
			files = append(files, makeRelativePath(path))
		}
	}
	if paths, ok := argsMap["files"].([]interface{}); ok {
		for _, raw := range paths {
			if path, ok := raw.(string); ok && path != "" {
				files = append(files, makeRelativePath(path))
			}
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
