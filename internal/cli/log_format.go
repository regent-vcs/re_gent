package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/index"
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
	StepInfo index.StepInfo
	Files    []string
	Args     json.RawMessage
	Result   json.RawMessage
	Duration time.Duration
	Messages []json.RawMessage // Conversation transcript
}

// LogFormatter formats steps for output
type LogFormatter interface {
	Format(steps []EnrichedStep, sessionID string, w io.Writer) error
}

// DefaultFormatter produces timeline view with arrows
type DefaultFormatter struct {
	NoColor bool
}

func (f *DefaultFormatter) Format(steps []EnrichedStep, sessionID string, w io.Writer) error {
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

	for i, step := range steps {
		// Bullet and basic info
		fmt.Fprintf(w, "%s %s  %s  %s",
			style.Brand("*"),
			step.StepInfo.ToolName,
			style.Hash(string(step.StepInfo.Hash[:8])),
			style.Timestamp(step.StepInfo.Timestamp.Format("15:04:05")))

		// Duration if available
		if step.Duration > 0 {
			fmt.Fprintf(w, "  %s",
				style.DimText(fmt.Sprintf("(%s)", formatDuration(step.Duration))))
		}
		fmt.Fprintln(w)

		// Files affected
		for _, file := range step.Files {
			fmt.Fprintf(w, "%s %s\n", style.DimText("│"), file)
		}

		// Tool arguments preview (truncated)
		if len(step.Args) > 0 && string(step.Args) != "null" {
			preview := truncateJSON(step.Args, 80)
			fmt.Fprintf(w, "%s %s\n",
				style.DimText("│ args:"),
				style.DimText(preview))
		}

		// Connector to next step
		if i < len(steps)-1 {
			fmt.Fprintln(w, style.DimText("│"))
		}
	}

	return nil
}

// OnelineFormatter produces compact one-line-per-step output
type OnelineFormatter struct{}

func (f *OnelineFormatter) Format(steps []EnrichedStep, sessionID string, w io.Writer) error {
	for _, step := range steps {
		summary := getSummary(step)
		fmt.Fprintf(w, "%s %s %s\n",
			string(step.StepInfo.Hash[:8]),
			step.StepInfo.ToolName,
			summary)
	}
	return nil
}

// JSONFormatter produces machine-readable JSON output
type JSONFormatter struct{}

type jsonStep struct {
	Hash      string          `json:"hash"`
	Parent    string          `json:"parent,omitempty"`
	Timestamp string          `json:"timestamp"`
	Tool      string          `json:"tool"`
	ToolUseID string          `json:"tool_use_id"`
	Files     []string        `json:"files,omitempty"`
	Args      json.RawMessage `json:"args,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	Duration  float64         `json:"duration_seconds,omitempty"`
}

func (f *JSONFormatter) Format(steps []EnrichedStep, sessionID string, w io.Writer) error {
	output := struct {
		SessionID string     `json:"session_id"`
		Steps     []jsonStep `json:"steps"`
	}{
		SessionID: sessionID,
		Steps:     make([]jsonStep, len(steps)),
	}

	for i, step := range steps {
		output.Steps[i] = jsonStep{
			Hash:      string(step.StepInfo.Hash),
			Parent:    string(step.StepInfo.ParentHash),
			Timestamp: step.StepInfo.Timestamp.Format(time.RFC3339),
			Tool:      step.StepInfo.ToolName,
			ToolUseID: step.StepInfo.ToolUseID,
			Files:     step.Files,
			Args:      step.Args,
			Result:    step.Result,
			Duration:  step.Duration.Seconds(),
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// StatFormatter shows file statistics
type StatFormatter struct{}

func (f *StatFormatter) Format(steps []EnrichedStep, sessionID string, w io.Writer) error {
	fmt.Fprintf(w, "%s %s %s\n\n",
		style.Label("Session:"),
		style.Hash(sessionID),
		style.DimText(fmt.Sprintf("(%d steps)", len(steps))))

	for _, step := range steps {
		fmt.Fprintf(w, "%s  %s  %s\n",
			style.Hash(string(step.StepInfo.Hash[:8])),
			step.StepInfo.ToolName,
			style.Timestamp(step.StepInfo.Timestamp.Format("15:04:05")))

		if len(step.Files) > 0 {
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

func truncateJSON(data json.RawMessage, maxLen int) string {
	s := string(data)
	// Remove newlines and extra spaces
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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
