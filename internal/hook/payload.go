package hook

import "encoding/json"

// Payload is the JSON structure Claude Code sends to PostToolUse hooks via stdin
type Payload struct {
	SessionID      string          `json:"session_id"`
	ToolUseID      string          `json:"tool_use_id"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolResponse   json.RawMessage `json:"tool_response"`
	CWD            string          `json:"cwd"`
	TranscriptPath string          `json:"transcript_path"`
}
