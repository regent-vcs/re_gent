package codex

import "time"

// Session is the normalized Codex session metadata used by the adapter.
type Session struct {
	SessionID  string
	ProjectCWD string
	Originator string
	SourceFile string
}

// Turn is the normalized unit of Codex work that may become one re_gent step.
type Turn struct {
	TurnID      string
	SessionID   string
	StartedAt   time.Time
	CompletedAt time.Time
	Messages    []Message
	ToolCalls   []ToolCall
	Warnings    []string
}

// Message is the normalized conversation message preserved in transcripts.
type Message struct {
	ID        string
	Role      string
	Text      string
	Timestamp time.Time
	Raw       []byte
}

// ToolCall is a normalized tool invocation/result pair inside one Codex turn.
type ToolCall struct {
	CallID         string
	ToolName       string
	Input          []byte
	Output         []byte
	StartedAt      time.Time
	CompletedAt    time.Time
	RawCall        []byte
	RawOutput      []byte
	SupportsReplay bool
}

// Checkpoint is persisted in .regent/adapters/codex/state.json.
type Checkpoint struct {
	SourceFile  string    `json:"source_file"`
	ByteOffset  int64     `json:"byte_offset"`
	LastEventTS time.Time `json:"last_event_ts"`
	SessionID   string    `json:"session_id"`
}

// AdapterState is the on-disk runtime state for Codex sidecar progress.
type AdapterState struct {
	ProjectRoot string                `json:"project_root"`
	Checkpoints map[string]Checkpoint `json:"checkpoints"`
}
