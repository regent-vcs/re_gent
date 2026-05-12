package conversation

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExtractConversation_SimpleUserMessage(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{
			"type": "user",
			"message": {
				"role": "user",
				"content": "Hello, Claude!"
			}
		}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if msgs[0].Role != "user" {
		t.Errorf("Expected role 'user', got %s", msgs[0].Role)
	}

	if len(msgs[0].Text) != 1 || msgs[0].Text[0] != "Hello, Claude!" {
		t.Errorf("Expected text 'Hello, Claude!', got %v", msgs[0].Text)
	}
}

func TestExtractConversation_AssistantWithTextBlocks(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{
			"type": "assistant",
			"message": {
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me help you with that."},
					{"type": "text", "text": "Here's what I found."}
				]
			}
		}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if msgs[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %s", msgs[0].Role)
	}

	if len(msgs[0].Text) != 2 {
		t.Fatalf("Expected 2 text blocks, got %d", len(msgs[0].Text))
	}

	expectedTexts := []string{"Let me help you with that.", "Here's what I found."}
	for i, expected := range expectedTexts {
		if msgs[0].Text[i] != expected {
			t.Errorf("Text block %d: got %s, want %s", i, msgs[0].Text[i], expected)
		}
	}
}

func TestExtractConversation_ToolUse(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{
			"type": "assistant",
			"message": {
				"role": "assistant",
				"content": [
					{"type": "text", "text": "Let me read that file."},
					{
						"type": "tool_use",
						"id": "toolu_abc123",
						"name": "Read",
						"input": {
							"file_path": "test.txt"
						}
					}
				]
			}
		}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if len(msgs[0].ToolUse) != 1 {
		t.Fatalf("Expected 1 tool use, got %d", len(msgs[0].ToolUse))
	}

	tool := msgs[0].ToolUse[0]
	if tool.ID != "toolu_abc123" {
		t.Errorf("Tool ID: got %s, want toolu_abc123", tool.ID)
	}
	if tool.Name != "Read" {
		t.Errorf("Tool name: got %s, want Read", tool.Name)
	}

	filePath, ok := tool.Input["file_path"].(string)
	if !ok || filePath != "test.txt" {
		t.Errorf("Tool input file_path: got %v, want test.txt", tool.Input["file_path"])
	}
}

func TestExtractConversation_SkipsThinkingBlocks(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{
			"type": "assistant",
			"message": {
				"role": "assistant",
				"content": [
					{"type": "thinking", "text": "Internal reasoning..."},
					{"type": "text", "text": "Here's my answer."}
				]
			}
		}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	// Should only have the text block, not the thinking block
	if len(msgs[0].Text) != 1 {
		t.Fatalf("Expected 1 text block, got %d", len(msgs[0].Text))
	}

	if msgs[0].Text[0] != "Here's my answer." {
		t.Errorf("Got unexpected text: %s", msgs[0].Text[0])
	}
}

func TestExtractConversation_SkipsMetaMessages(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{
			"type": "user",
			"isMeta": true,
			"message": {
				"role": "user",
				"content": "Meta info"
			}
		}`),
		json.RawMessage(`{
			"type": "user",
			"message": {
				"role": "user",
				"content": "Real user message"
			}
		}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message (meta should be skipped), got %d", len(msgs))
	}

	if msgs[0].Text[0] != "Real user message" {
		t.Errorf("Got wrong message: %s", msgs[0].Text[0])
	}
}

func TestExtractConversation_SkipsNonUserAssistantTypes(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{"type": "system", "message": {"role": "system", "content": "System prompt"}}`),
		json.RawMessage(`{"type": "tool_result", "message": {"role": "user", "content": "Tool output"}}`),
		json.RawMessage(`{"type": "user", "message": {"role": "user", "content": "User message"}}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message (only user message), got %d", len(msgs))
	}

	if msgs[0].Text[0] != "User message" {
		t.Errorf("Got wrong message: %s", msgs[0].Text[0])
	}
}

func TestExtractConversation_SkipsEmptyMessages(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{"type": "user", "message": {"role": "user", "content": ""}}`),
		json.RawMessage(`{"type": "assistant", "message": {"role": "assistant", "content": []}}`),
		json.RawMessage(`{"type": "user", "message": {"role": "user", "content": "Real message"}}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation failed: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message (empty ones should be skipped), got %d", len(msgs))
	}

	if msgs[0].Text[0] != "Real message" {
		t.Errorf("Got wrong message: %s", msgs[0].Text[0])
	}
}

func TestExtractConversation_HandlesInvalidJSON(t *testing.T) {
	rawMessages := []json.RawMessage{
		json.RawMessage(`{invalid json`),
		json.RawMessage(`{"type": "user", "message": {"role": "user", "content": "Valid message"}}`),
	}

	msgs, err := ExtractConversation(rawMessages)
	if err != nil {
		t.Fatalf("ExtractConversation should not fail on invalid JSON, got: %v", err)
	}

	// Should skip invalid and return valid
	if len(msgs) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(msgs))
	}

	if msgs[0].Text[0] != "Valid message" {
		t.Errorf("Got wrong message: %s", msgs[0].Text[0])
	}
}

func TestFormatConversation_Simple(t *testing.T) {
	msgs := []ConversationMessage{
		{
			Role: "user",
			Text: []string{"Hello, Claude!"},
		},
		{
			Role: "assistant",
			Text: []string{"Hello! How can I help you?"},
		},
	}

	output := FormatConversation(msgs, "")
	if output == "" {
		t.Error("Expected non-empty output")
	}

	// Check that it contains both the user and assistant labels
	if !strings.Contains(output, "Human:") {
		t.Error("Output should contain 'Human:'")
	}
	if !strings.Contains(output, "Agent:") {
		t.Error("Output should contain 'Agent:'")
	}
}

func TestFormatConversation_WithToolUse(t *testing.T) {
	msgs := []ConversationMessage{
		{
			Role: "assistant",
			Text: []string{"Let me read that file."},
			ToolUse: []ToolUse{
				{
					ID:   "toolu_123",
					Name: "Read",
					Input: map[string]interface{}{
						"file_path": "test.txt",
					},
				},
			},
		},
	}

	output := FormatConversation(msgs, "")
	if !strings.Contains(output, "Read") {
		t.Error("Output should contain tool name 'Read'")
	}
	if !strings.Contains(output, "file_path") {
		t.Error("Output should contain 'file_path' argument")
	}
}

func TestFormatConversation_Empty(t *testing.T) {
	msgs := []ConversationMessage{}
	output := FormatConversation(msgs, "")
	if output != "" {
		t.Errorf("Expected empty output for empty messages, got: %s", output)
	}
}

func TestShouldShowArg(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{"file_path", true},
		{"command", true},
		{"description", true},
		{"prompt", true},
		{"query", true},
		{"path", true},
		{"content", false}, // Too verbose
		{"random_key", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := shouldShowArg(tt.key)
			if got != tt.want {
				t.Errorf("shouldShowArg(%s) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestParseContent_StringContent(t *testing.T) {
	content := json.RawMessage(`"Hello, world!"`)
	msg, err := parseContent("user", content)
	if err != nil {
		t.Fatalf("parseContent failed: %v", err)
	}

	if msg.Role != "user" {
		t.Errorf("Role: got %s, want user", msg.Role)
	}

	if len(msg.Text) != 1 || msg.Text[0] != "Hello, world!" {
		t.Errorf("Text: got %v, want ['Hello, world!']", msg.Text)
	}
}

func TestParseContent_EmptyString(t *testing.T) {
	content := json.RawMessage(`""`)
	msg, err := parseContent("user", content)
	if err != nil {
		t.Fatalf("parseContent failed: %v", err)
	}

	if len(msg.Text) != 0 {
		t.Errorf("Expected no text for empty string, got %v", msg.Text)
	}
}

func TestFormatToolUse_TruncatesLongValues(t *testing.T) {
	tool := ToolUse{
		Name: "Write",
		Input: map[string]interface{}{
			"file_path": "test.txt",
			"content":   strings.Repeat("x", 100), // Long value (but won't show due to shouldShowArg)
		},
	}

	output := formatToolUse(tool, "")
	if !strings.Contains(output, "Write") {
		t.Error("Output should contain tool name")
	}
	if !strings.Contains(output, "file_path") {
		t.Error("Output should contain file_path")
	}
	// content should not appear because shouldShowArg returns false for it
	if strings.Contains(output, "content:") {
		t.Error("Output should not contain 'content' argument")
	}
}

func TestWrapLine(t *testing.T) {
	text := "This is a very long line that should be wrapped at some point when it exceeds the maximum length"
	wrapped := wrapLine(text, 40, "  ")

	// Should contain newline
	if !strings.Contains(wrapped, "\n") {
		t.Error("Long line should be wrapped")
	}

	// Each line should be <= 40 chars (approximately, word boundaries matter)
	lines := strings.Split(wrapped, "\n")
	for i, line := range lines {
		if i > 0 {
			// Subsequent lines should start with indent
			if !strings.HasPrefix(line, "  ") {
				t.Errorf("Line %d should be indented: %s", i, line)
			}
		}
	}
}

func TestWrapLine_ShortLine(t *testing.T) {
	text := "Short line"
	wrapped := wrapLine(text, 40, "  ")

	if wrapped != text {
		t.Errorf("Short line should not be wrapped: got %s", wrapped)
	}
}
