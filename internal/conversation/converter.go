package conversation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/regent-vcs/regent/internal/style"
)

// ClaudeCodeMessage represents the envelope format from Claude Code JSONL
type ClaudeCodeMessage struct {
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message,omitempty"`
	IsMeta  bool            `json:"isMeta,omitempty"`
}

// ClaudeAPIMessage represents the actual Claude API message format
type ClaudeAPIMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// ConversationMessage is the normalized format for display
type ConversationMessage struct {
	Role    string   // "user" or "assistant"
	Text    []string // Text blocks
	ToolUse []ToolUse
}

// ToolUse represents a tool invocation in the conversation
type ToolUse struct {
	ID    string
	Name  string
	Input map[string]interface{}
}

// ExtractConversation converts Claude Code JSONL messages to normalized conversation
// This is agent-agnostic: it extracts the actual Claude API messages from whatever
// wrapper format the agent uses (Claude Code, Cursor, etc.)
func ExtractConversation(rawMessages []json.RawMessage) ([]ConversationMessage, error) {
	var result []ConversationMessage

	for _, raw := range rawMessages {
		// Parse the Claude Code envelope
		var envelope ClaudeCodeMessage
		if err := json.Unmarshal(raw, &envelope); err != nil {
			// Skip unparseable messages
			continue
		}

		// Filter: only process user and assistant messages, skip metadata
		if envelope.Type != "user" && envelope.Type != "assistant" {
			continue
		}
		if envelope.IsMeta {
			continue
		}

		// Extract the nested Claude API message
		if len(envelope.Message) == 0 {
			continue
		}

		var apiMsg ClaudeAPIMessage
		if err := json.Unmarshal(envelope.Message, &apiMsg); err != nil {
			continue
		}

		// Parse content (can be string or array of content blocks)
		msg, err := parseContent(apiMsg.Role, apiMsg.Content)
		if err != nil {
			continue
		}

		// Skip empty messages
		if len(msg.Text) == 0 && len(msg.ToolUse) == 0 {
			continue
		}

		result = append(result, msg)
	}

	return result, nil
}

// parseContent handles both string content and content block arrays
func parseContent(role string, content json.RawMessage) (ConversationMessage, error) {
	msg := ConversationMessage{Role: role}

	// Try parsing as string first
	var textContent string
	if err := json.Unmarshal(content, &textContent); err == nil {
		if textContent != "" {
			msg.Text = []string{textContent}
		}
		return msg, nil
	}

	// Try parsing as array of content blocks
	var blocks []map[string]interface{}
	if err := json.Unmarshal(content, &blocks); err != nil {
		return msg, fmt.Errorf("content is neither string nor array")
	}

	for _, block := range blocks {
		blockType, _ := block["type"].(string)

		switch blockType {
		case "text":
			if text, ok := block["text"].(string); ok && text != "" {
				msg.Text = append(msg.Text, text)
			}

		case "thinking":
			// Skip thinking blocks - internal to Claude

		case "tool_use":
			// Tool invocation
			toolUse := ToolUse{
				ID:   getString(block, "id"),
				Name: getString(block, "name"),
			}
			if input, ok := block["input"].(map[string]interface{}); ok {
				toolUse.Input = input
			}
			msg.ToolUse = append(msg.ToolUse, toolUse)

		case "tool_result":
			// Tool results in user messages (part of API conversation)
			// Don't show these - they're system messages, not user input
			// Just skip them
		}
	}

	return msg, nil
}

// FormatConversation formats full conversation including user, assistant, and tool calls
// Returns empty string if no messages found
func FormatConversation(messages []ConversationMessage, indent string) string {
	return formatConversationInternal(messages, indent, "", "", "")
}

// FormatConversationWithHash formats conversation with step hash and graph prefix on each line
func FormatConversationWithHash(messages []ConversationMessage, graphPrefix, stepHash, timestamp string) string {
	return formatConversationInternal(messages, "", graphPrefix, stepHash, timestamp)
}

func formatConversationInternal(messages []ConversationMessage, indent, graphPrefix, stepHash, timestamp string) string {
	if len(messages) == 0 {
		return ""
	}

	var output strings.Builder

	// Show hash at the top with timestamp (both muted)
	if stepHash != "" {
		subtitle := style.DimText(stepHash)
		if timestamp != "" {
			subtitle += style.DimText(" • " + timestamp)
		}
		output.WriteString(graphPrefix + subtitle + "\n")
	}

	for i, msg := range messages {
		if msg.Role == "user" {
			// User prompt - base level indent
			for _, text := range msg.Text {
				trimmed := strings.TrimSpace(text)
				if trimmed != "" {
					if stepHash != "" {
						output.WriteString("  Human: " + formatText(text, "         ", 90) + "\n")
					} else {
						output.WriteString(indent + "Human: " + formatText(text, indent+"       ", 90) + "\n")
					}
				}
			}
			// Add flow indicator if there's a next message
			if i < len(messages)-1 {
				if stepHash != "" {
					output.WriteString("    ↓\n")
				} else {
					output.WriteString(indent + "  ↓\n")
				}
			}
		} else if msg.Role == "assistant" {
			// Assistant response - indented one more level
			for _, text := range msg.Text {
				trimmed := strings.TrimSpace(text)
				if trimmed != "" {
					if stepHash != "" {
						output.WriteString("    Agent: " + formatText(text, "           ", 90) + "\n")
					} else {
						output.WriteString(indent + "Agent: " + formatText(text, indent+"       ", 90) + "\n")
					}
				}
			}

			// Show tool calls - indented even further
			if len(msg.ToolUse) > 0 {
				for j, tool := range msg.ToolUse {
					prefix := "├─"
					if j == len(msg.ToolUse)-1 {
						prefix = "└─"
					}
					if stepHash != "" {
						output.WriteString("      " + prefix + " " + formatToolUse(tool, "         ") + "\n")
					} else {
						output.WriteString(indent + "  " + prefix + " " + formatToolUse(tool, indent+"     ") + "\n")
					}
				}
			}

			// Add blank line between conversation turns
			if i < len(messages)-1 {
				output.WriteString("\n")
			}
		}
	}

	return output.String()
}

// formatText wraps and indents long text
func formatText(text string, indent string, maxLen int) string {
	text = strings.TrimSpace(text)
	if len(text) <= maxLen {
		return text
	}
	return wrapLine(text, maxLen, indent)
}

// formatToolUse formats a tool invocation with key arguments
func formatToolUse(tool ToolUse, indent string) string {
	var parts []string
	parts = append(parts, tool.Name)

	// Add key arguments
	if len(tool.Input) > 0 {
		var args []string
		for key, val := range tool.Input {
			if shouldShowArg(key) {
				valStr := fmt.Sprintf("%v", val)
				// Truncate long values
				if len(valStr) > 60 {
					valStr = valStr[:57] + "..."
				}
				args = append(args, fmt.Sprintf("%s: %s", key, valStr))
			}
		}
		if len(args) > 0 {
			parts = append(parts, "("+strings.Join(args, ", ")+")")
		}
	}

	return strings.Join(parts, " ")
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func shouldShowArg(key string) bool {
	important := map[string]bool{
		"file_path":   true,
		"command":     true,
		"description": true,
		"prompt":      true,
		"query":       true,
		"path":        true,
		"content":     false, // Too verbose
	}
	return important[key]
}

func wrapLine(line string, maxLen int, indent string) string {
	if len(line) <= maxLen {
		return line
	}

	words := strings.Fields(line)
	var result strings.Builder
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)
		if i == 0 {
			result.WriteString(word)
			lineLen = wordLen
		} else if lineLen+1+wordLen <= maxLen {
			result.WriteString(" " + word)
			lineLen += 1 + wordLen
		} else {
			result.WriteString("\n" + indent + word)
			lineLen = wordLen
		}
	}

	return result.String()
}
