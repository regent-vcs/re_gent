package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/regent-vcs/regent/internal/conversation"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
)

// LogFormat represents different output formats
type LogFormat string

const (
	FormatDefault LogFormat = "default" // Timeline view
	FormatOneline LogFormat = "oneline" // Compact
	FormatJSON    LogFormat = "json"    // Machine readable
	FormatStat    LogFormat = "stat"    // With file stats
)

// EnrichedStep contains a step with all its related data
type EnrichedStep struct {
	StepInfo    index.StepInfo
	Causes      []EnrichedCause
	Files       []string
	FileDiffs   []FileDiff // Actual file changes (parent → current)
	Args        json.RawMessage
	Result      json.RawMessage
	Duration    time.Duration
	Messages    []json.RawMessage // Conversation transcript
	GraphPrefix string            // ASCII graph line prefix (if graph enabled)
}

type EnrichedCause struct {
	Cause  store.Cause
	Args   json.RawMessage
	Result json.RawMessage
}

// FileDiff represents a file change between steps
type FileDiff struct {
	Path      string
	Status    string // "added", "modified", "deleted"
	Additions int
	Deletions int
	IsBinary  bool
}

// LogFormatter formats steps for output
type LogFormatter interface {
	Format(steps []EnrichedStep, sessionID string, showConversation bool, showFiles bool, w io.Writer) error
}

// DefaultFormatter produces timeline view with arrows
type DefaultFormatter struct {
	NoColor bool
}

func (f *DefaultFormatter) Format(steps []EnrichedStep, sessionID string, showConversation bool, showFiles bool, w io.Writer) error {
	if len(steps) == 0 {
		return nil
	}

	// Calculate total elapsed time
	var totalElapsed time.Duration
	if len(steps) > 0 {
		totalElapsed = steps[0].StepInfo.Timestamp.Sub(steps[len(steps)-1].StepInfo.Timestamp)
	}

	// Session header
	fmt.Fprintf(w, "%s %s %s\n\n",
		style.Label("Session:"),
		style.Hash(sessionID),
		style.DimText(fmt.Sprintf("(%d steps, %s elapsed)", len(steps), formatDuration(totalElapsed))))

	// When showing conversation, reverse order (oldest first, like chat)
	// Also force graph rendering for conversation mode
	displayOrder := steps
	if showConversation {
		displayOrder = make([]EnrichedStep, len(steps))
		for i := range steps {
			displayOrder[i] = steps[len(steps)-1-i]
		}
	}

	for i, step := range displayOrder {
		// In conversation mode, pass step hash to conversation formatter
		if showConversation && len(step.Messages) > 0 {
			// Show graph prefix
			graphPrefix := step.GraphPrefix
			if graphPrefix == "" {
				graphPrefix = style.DimText("* ")
			}

			conv, _ := conversation.ExtractConversation(step.Messages)
			timestamp := step.StepInfo.Timestamp.Format("15:04:05")
			formatted := conversation.FormatConversationWithHash(conv, graphPrefix, style.Hash(string(step.StepInfo.Hash[:8])), timestamp)
			if formatted != "" {
				fmt.Fprint(w, formatted)
				fmt.Fprintln(w) // Blank line after conversation
			}
			continue
		}

		// Non-conversation mode: show traditional format
		// Show graph prefix
		graphPrefix := step.GraphPrefix
		if graphPrefix != "" {
			fmt.Fprint(w, graphPrefix)
		}

		// Show step hash and timestamp
		fmt.Fprintf(w, "%s %s  %s",
			style.Label(stepToolLabel(step)),
			style.Hash(string(step.StepInfo.Hash[:8])),
			style.Timestamp(step.StepInfo.Timestamp.Format("15:04:05")))

		if step.Duration > 0 {
			fmt.Fprintf(w, "  %s", style.DimText(fmt.Sprintf("(%s)", formatDuration(step.Duration))))
		}
		fmt.Fprintln(w)

		// Show what the tool did (command, file, etc.) - only if NOT in conversation mode
		if !showConversation {
			// Show what the tool did (command, file, etc.)
			if len(step.Args) > 0 && string(step.Args) != "null" {
				var args map[string]interface{}
				if json.Unmarshal(step.Args, &args) == nil {
					if cmd, ok := args["command"].(string); ok {
						fmt.Fprintf(w, "  %s\n", truncate(cmd, 90))
					} else if filePath, ok := args["file_path"].(string); ok {
						fmt.Fprintf(w, "  %s\n", filePath)
					}
				}
			}
		}

		// OUTPUT: Show file changes if available
		// Filter to only files touched by this tool (not entire tree diff)
		if showFiles && len(step.FileDiffs) > 0 && len(step.Files) > 0 {
			// Create map of files this tool touched
			touchedFiles := make(map[string]bool)
			for _, f := range step.Files {
				touchedFiles[f] = true
			}

			// Filter FileDiffs to only touched files
			var relevantDiffs []FileDiff
			for _, fd := range step.FileDiffs {
				if touchedFiles[fd.Path] {
					relevantDiffs = append(relevantDiffs, fd)
				}
			}

			// Only show section if there are relevant diffs
			if len(relevantDiffs) > 0 {
				fmt.Fprintln(w)
				for _, fd := range relevantDiffs {
					fmt.Fprintf(w, "  %s  %s\n", fd.Path, formatFileStat(fd))
				}
			}
		}

		// Separator between steps
		if i < len(steps)-1 {
			fmt.Fprintln(w)
		}
	}

	return nil
}

// OnelineFormatter produces compact one-line-per-step output
type OnelineFormatter struct{}

func (f *OnelineFormatter) Format(steps []EnrichedStep, sessionID string, showConversation bool, showFiles bool, w io.Writer) error {
	for _, step := range steps {
		summary := getSummary(step)

		// Build line
		line := fmt.Sprintf("%s %s %s",
			string(step.StepInfo.Hash[:8]),
			stepToolLabel(step),
			summary)

		// Append file stats if requested
		if showFiles && len(step.FileDiffs) > 0 {
			var totalAdd, totalDel int
			for _, fd := range step.FileDiffs {
				totalAdd += fd.Additions
				totalDel += fd.Deletions
			}
			line += fmt.Sprintf(" (+%d -%d)", totalAdd, totalDel)
		}

		fmt.Fprintln(w, line)
	}
	return nil
}

// JSONFormatter produces machine-readable JSON output
type JSONFormatter struct{}

type jsonStep struct {
	Hash      string            `json:"hash"`
	Parent    string            `json:"parent,omitempty"`
	Timestamp string            `json:"timestamp"`
	Origin    string            `json:"origin,omitempty"`
	TurnID    string            `json:"turn_id,omitempty"`
	Tool      string            `json:"tool"`
	ToolUseID string            `json:"tool_use_id"`
	Causes    []jsonCause       `json:"causes,omitempty"`
	Files     []string          `json:"files,omitempty"`
	FileDiffs []FileDiff        `json:"file_diffs,omitempty"`
	Args      json.RawMessage   `json:"args,omitempty"`
	Result    json.RawMessage   `json:"result,omitempty"`
	Duration  float64           `json:"duration_seconds,omitempty"`
	Messages  []json.RawMessage `json:"messages,omitempty"`
}

type jsonCause struct {
	Tool      string          `json:"tool"`
	ToolUseID string          `json:"tool_use_id"`
	Args      json.RawMessage `json:"args,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
}

func (f *JSONFormatter) Format(steps []EnrichedStep, sessionID string, showConversation bool, showFiles bool, w io.Writer) error {
	output := struct {
		SessionID string     `json:"session_id"`
		Steps     []jsonStep `json:"steps"`
	}{
		SessionID: sessionID,
		Steps:     make([]jsonStep, len(steps)),
	}

	for i, step := range steps {
		js := jsonStep{
			Hash:      string(step.StepInfo.Hash),
			Parent:    string(step.StepInfo.ParentHash),
			Timestamp: step.StepInfo.Timestamp.Format(time.RFC3339),
			Origin:    step.StepInfo.Origin,
			TurnID:    step.StepInfo.TurnID,
			Tool:      step.StepInfo.ToolName,
			ToolUseID: step.StepInfo.ToolUseID,
			Files:     step.Files,
			Args:      step.Args,
			Result:    step.Result,
			Duration:  step.Duration.Seconds(),
		}

		if showFiles {
			js.FileDiffs = step.FileDiffs
		}

		if showConversation {
			js.Messages = step.Messages
		}
		if len(step.Causes) > 0 {
			js.Causes = make([]jsonCause, 0, len(step.Causes))
			for _, cause := range step.Causes {
				js.Causes = append(js.Causes, jsonCause{
					Tool:      cause.Cause.ToolName,
					ToolUseID: cause.Cause.ToolUseID,
					Args:      cause.Args,
					Result:    cause.Result,
				})
			}
		}

		output.Steps[i] = js
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// StatFormatter shows file statistics
type StatFormatter struct{}

func (f *StatFormatter) Format(steps []EnrichedStep, sessionID string, showConversation bool, showFiles bool, w io.Writer) error {
	fmt.Fprintf(w, "%s %s %s\n\n",
		style.Label("Session:"),
		style.Hash(sessionID),
		style.DimText(fmt.Sprintf("(%d steps)", len(steps))))

	for _, step := range steps {
		fmt.Fprintf(w, "%s  %s  %s\n",
			style.Hash(string(step.StepInfo.Hash[:8])),
			stepToolLabel(step),
			style.Timestamp(step.StepInfo.Timestamp.Format("15:04:05")))

		// Show file diffs with stats if --files flag
		if showFiles && len(step.FileDiffs) > 0 {
			for _, fd := range step.FileDiffs {
				fmt.Fprintf(w, " %s  %s\n", fd.Path, formatFileStat(fd))
			}
		} else if len(step.Files) > 0 {
			// Backward compat: show files from tool args
			for _, file := range step.Files {
				fmt.Fprintf(w, " %s\n", file)
			}
		} else {
			// Show command or summary for non-file operations
			if step.StepInfo.ToolName == "Bash" && len(step.Args) > 0 {
				var args map[string]interface{}
				if json.Unmarshal(step.Args, &args) == nil {
					if cmd, ok := args["command"].(string); ok {
						fmt.Fprintf(w, " %s\n", style.DimText(fmt.Sprintf("(command: %s)", truncate(cmd, 60))))
					}
				}
			}
		}

		// Show conversation if --conversation flag
		if showConversation && len(step.Messages) > 0 {
			fmt.Fprintln(w)
			formatted := FormatMessagesHumanReadable(step.Messages, " ")
			fmt.Fprint(w, formatted)
		}

		fmt.Fprintln(w)
	}

	return nil
}

// Helper functions

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}

func formatFileStat(fd FileDiff) string {
	if fd.IsBinary {
		return style.DimText("(binary)")
	}
	if fd.Status == "added" {
		return style.DimText(fmt.Sprintf("+%d", fd.Additions))
	}
	if fd.Status == "deleted" {
		return style.DimText(fmt.Sprintf("-%d", fd.Deletions))
	}
	return style.DimText(fmt.Sprintf("+%d -%d", fd.Additions, fd.Deletions))
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func getSummary(step EnrichedStep) string {
	// For file operations, show the primary file
	if len(step.Files) > 0 {
		return step.Files[0]
	}

	// For Bash, show the command
	if step.StepInfo.ToolName == "Bash" && len(step.Args) > 0 {
		var args map[string]interface{}
		if json.Unmarshal(step.Args, &args) == nil {
			if cmd, ok := args["command"].(string); ok {
				return truncate(cmd, 60)
			}
		}
	}

	return ""
}

func stepToolLabel(step EnrichedStep) string {
	if len(step.Causes) <= 1 {
		return step.StepInfo.ToolName
	}
	return fmt.Sprintf("%s +%d", step.StepInfo.ToolName, len(step.Causes)-1)
}
