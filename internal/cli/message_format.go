package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/regent-vcs/regent/internal/conversation"
	"github.com/regent-vcs/regent/internal/style"
)

// FormatMessagesHumanReadable converts raw message JSON into readable conversation format
func FormatMessagesHumanReadable(messages []json.RawMessage, indent string) string {
	if len(messages) == 0 {
		return style.DimText(indent + "(no conversation)")
	}

	// Extract actual conversation from agent wrapper format (Claude Code, etc.)
	conv, err := conversation.ExtractConversation(messages)
	if err != nil || len(conv) == 0 {
		return style.DimText(indent + fmt.Sprintf("(no conversation extracted from %d events)\n", len(messages)))
	}

	// Format conversation for display
	return conversation.FormatConversation(conv, indent)
}

// PLACEHOLDER - will implement proper parsing once we understand the format
func FormatMessagesHumanReadable_OLD(messages []json.RawMessage, indent string) string {
	if len(messages) == 0 {
		return style.DimText(indent + "(no conversation)")
	}

	var output strings.Builder
	var currentSection string

	for _, msgRaw := range messages {
		// Parse message to extract type and content
		var msg map[string]interface{}
		if err := json.Unmarshal(msgRaw, &msg); err != nil {
			// Skip unparseable messages silently
			continue
		}

		msgType, _ := msg["type"].(string)

		// Handle different Claude Code message types
		switch msgType {
		case "user_message", "user":
			// User input
			if currentSection != "user" {
				if currentSection != "" {
					output.WriteString("\n")
				}
				output.WriteString(indent + "User:\n")
				currentSection = "user"
			}

			if text, ok := msg["text"].(string); ok {
				formatted := formatTextContent(text, indent+"  ")
				output.WriteString(formatted + "\n")
			}

		case "assistant_message", "assistant":
			// Assistant response
			if currentSection != "assistant" {
				if currentSection != "" {
					output.WriteString("\n")
				}
				output.WriteString(indent + "Assistant:\n")
				currentSection = "assistant"
			}

			if text, ok := msg["text"].(string); ok {
				formatted := formatTextContent(text, indent+"  ")
				output.WriteString(formatted + "\n")
			}

		case "text":
			// Simple text content
			if text, ok := msg["text"].(string); ok {
				formatted := formatTextContent(text, indent+"  ")
				output.WriteString(formatted + "\n")
			}

		case "tool_use":
			// Tool invocation
			toolName, _ := msg["name"].(string)
			toolUseID, _ := msg["id"].(string)
			output.WriteString(indent + style.DimText(fmt.Sprintf("  [Tool Use: %s (%s)]", toolName, toolUseID[:8])) + "\n")

			// Show key tool arguments
			if input, ok := msg["input"].(map[string]interface{}); ok {
				for key, val := range input {
					// Only show important args, truncate long values
					if shouldShowArg(key) {
						valStr := fmt.Sprintf("%v", val)
						if len(valStr) > 60 {
							valStr = valStr[:57] + "..."
						}
						output.WriteString(indent + style.DimText(fmt.Sprintf("    %s: %s", key, valStr)) + "\n")
					}
				}
			}

		case "tool_result":
			// Tool execution result
			toolUseID, _ := msg["tool_use_id"].(string)
			isError, _ := msg["is_error"].(bool)

			status := "success"
			if isError {
				status = "error"
			}

			output.WriteString(indent + style.DimText(fmt.Sprintf("  [Tool Result: %s (%s)]", status, toolUseID[:8])) + "\n")

			// Show result content (truncated)
			if content, ok := msg["content"].(string); ok {
				lines := strings.Split(content, "\n")
				maxLines := 3
				for i, line := range lines {
					if i >= maxLines {
						output.WriteString(indent + style.DimText(fmt.Sprintf("    ... (%d more lines)", len(lines)-maxLines)) + "\n")
						break
					}
					if len(line) > 80 {
						line = line[:77] + "..."
					}
					output.WriteString(indent + style.DimText("    "+line) + "\n")
				}
			}

		default:
			// Handle Claude API content blocks format
			if content, ok := msg["content"].([]interface{}); ok {
				for _, block := range content {
					if blockMap, ok := block.(map[string]interface{}); ok {
						blockType, _ := blockMap["type"].(string)

						switch blockType {
						case "text":
							if text, ok := blockMap["text"].(string); ok {
								formatted := formatTextContent(text, indent+"  ")
								output.WriteString(formatted + "\n")
							}

						case "tool_use":
							toolName, _ := blockMap["name"].(string)
							toolUseID, _ := blockMap["id"].(string)
							output.WriteString(indent + style.DimText(fmt.Sprintf("  [Tool Use: %s (%s)]", toolName, toolUseID[:8])) + "\n")

							if input, ok := blockMap["input"].(map[string]interface{}); ok {
								for key, val := range input {
									if shouldShowArg(key) {
										valStr := fmt.Sprintf("%v", val)
										if len(valStr) > 60 {
											valStr = valStr[:57] + "..."
										}
										output.WriteString(indent + style.DimText(fmt.Sprintf("    %s: %s", key, valStr)) + "\n")
									}
								}
							}

						case "tool_result":
							toolUseID, _ := blockMap["tool_use_id"].(string)
							isError, _ := blockMap["is_error"].(bool)

							status := "success"
							if isError {
								status = "error"
							}

							output.WriteString(indent + style.DimText(fmt.Sprintf("  [Tool Result: %s (%s)]", status, toolUseID[:8])) + "\n")
						}
					}
				}
			} else if text, ok := msg["content"].(string); ok {
				// Simple string content
				formatted := formatTextContent(text, indent+"  ")
				output.WriteString(formatted + "\n")
			}
		}
	}

	return output.String()
}

// formatTextContent formats text content with proper wrapping and indentation
func formatTextContent(text, indent string) string {
	if text == "" {
		return ""
	}

	// Split by lines
	lines := strings.Split(text, "\n")
	var formatted strings.Builder

	for _, line := range lines {
		// Preserve empty lines
		if strings.TrimSpace(line) == "" {
			formatted.WriteString("\n")
			continue
		}

		// Wrap long lines
		if len(line) > 100 {
			words := strings.Fields(line)
			currentLine := indent
			for _, word := range words {
				if len(currentLine)+len(word)+1 > 100 {
					formatted.WriteString(strings.TrimRight(currentLine, " ") + "\n")
					currentLine = indent + word + " "
				} else {
					currentLine += word + " "
				}
			}
			if len(currentLine) > len(indent) {
				formatted.WriteString(strings.TrimRight(currentLine, " ") + "\n")
			}
		} else {
			formatted.WriteString(indent + line + "\n")
		}
	}

	return strings.TrimRight(formatted.String(), "\n")
}

// shouldShowArg determines if a tool argument should be displayed
func shouldShowArg(key string) bool {
	// Show these important argument keys
	important := map[string]bool{
		"file_path":   true,
		"command":     true,
		"description": true,
		"prompt":      true,
		"query":       true,
		"path":        true,
	}
	return important[key]
}
