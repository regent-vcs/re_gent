package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type rawEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type sessionMetaPayload struct {
	ID         string `json:"id"`
	CWD        string `json:"cwd"`
	Originator string `json:"originator"`
}

type turnContextPayload struct {
	TurnID string `json:"turn_id"`
	CWD    string `json:"cwd"`
}

type taskStartedPayload struct {
	TurnID string `json:"turn_id"`
}

type taskCompletePayload struct {
	TurnID string `json:"turn_id"`
}

type patchApplyEndPayload struct {
	CallID  string                      `json:"call_id"`
	TurnID  string                      `json:"turn_id"`
	Stdout  string                      `json:"stdout"`
	Stderr  string                      `json:"stderr"`
	Success bool                        `json:"success"`
	Changes map[string]patchApplyChange `json:"changes"`
}

type patchApplyChange struct {
	Type        string  `json:"type"`
	UnifiedDiff string  `json:"unified_diff"`
	MovePath    *string `json:"move_path"`
}

type responseItemPayload struct {
	Type      string          `json:"type"`
	Role      string          `json:"role"`
	Name      string          `json:"name"`
	CallID    string          `json:"call_id"`
	Arguments string          `json:"arguments"`
	Output    string          `json:"output"`
	Content   json.RawMessage `json:"content"`
	Input     string          `json:"input"`
}

type responseMessageContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type eventMessagePayload struct {
	Message string `json:"message"`
}

type parsedFile struct {
	Session Session
	Turns   []Turn
	Offset  int64
}

type turnBuilder struct {
	turn        Turn
	startOffset int64
}

func parseRolloutFile(path string, fromOffset int64) (*parsedFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open rollout file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if fromOffset > 0 {
		if _, err := file.Seek(fromOffset, io.SeekStart); err != nil {
			return nil, fmt.Errorf("seek rollout file: %w", err)
		}
	}

	reader := bufio.NewReader(file)
	builders := map[string]*turnBuilder{}
	callToTurn := map[string]string{}
	patchMetadata := map[string]patchApplyEndPayload{}
	lastTurnID := ""
	var session Session
	var warnings []string
	currentOffset := fromOffset

	for {
		lineStart := currentOffset
		line, err := reader.ReadBytes('\n')
		nextOffset := currentOffset + int64(len(line))
		if len(line) == 0 && err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read rollout line: %w", err)
		}

		trimmed := normalizeJSONLLine(line)
		if len(trimmed) == 0 {
			currentOffset = nextOffset
			if err == io.EOF {
				break
			}
			continue
		}

		var env rawEnvelope
		if unmarshalErr := json.Unmarshal(trimmed, &env); unmarshalErr != nil {
			warnings = append(warnings, fmt.Sprintf("skip truncated or invalid JSONL line in %s: %v", path, unmarshalErr))
			if err == io.EOF {
				break
			}
			currentOffset = nextOffset
			continue
		}

		ts, tsErr := time.Parse(time.RFC3339Nano, env.Timestamp)
		if tsErr != nil {
			ts = time.Time{}
		}

		switch env.Type {
		case "session_meta":
			var payload sessionMetaPayload
			if json.Unmarshal(env.Payload, &payload) == nil {
				session = Session{
					SessionID:  logicalSessionID(payload.ID),
					ProjectCWD: filepath.Clean(payload.CWD),
					Originator: payload.Originator,
					SourceFile: path,
				}
			}

		case "turn_context":
			var payload turnContextPayload
			if json.Unmarshal(env.Payload, &payload) == nil {
				builder := ensureTurnBuilder(builders, payload.TurnID, session.SessionID, lineStart)
				if builder.turn.StartedAt.IsZero() {
					builder.turn.StartedAt = ts
				}
				lastTurnID = payload.TurnID
				if payload.CWD != "" && session.ProjectCWD == "" {
					session.ProjectCWD = filepath.Clean(payload.CWD)
				}
			}

		case "event_msg":
			lastTurnID = handleEventMessage(env.Payload, ts, builders, callToTurn, patchMetadata, lastTurnID, lineStart, &warnings)

		case "response_item":
			updatedTurnID := handleResponseItem(env.Payload, ts, builders, callToTurn, patchMetadata, lastTurnID, lineStart, &warnings)
			if updatedTurnID != "" {
				lastTurnID = updatedTurnID
			}
		}

		currentOffset = nextOffset

		if err == io.EOF {
			break
		}
	}

	var turns []Turn
	safeOffset := currentOffset
	for _, builder := range builders {
		if len(builder.turn.Warnings) == 0 && len(warnings) > 0 {
			builder.turn.Warnings = append(builder.turn.Warnings, warnings...)
		}
		if builder.turn.TurnID == "" {
			continue
		}
		if builder.turn.CompletedAt.IsZero() && builder.startOffset < safeOffset {
			safeOffset = builder.startOffset
		}
		turns = append(turns, builder.turn)
	}

	sort.Slice(turns, func(i, j int) bool {
		if turns[i].StartedAt.Equal(turns[j].StartedAt) {
			return turns[i].TurnID < turns[j].TurnID
		}
		return turns[i].StartedAt.Before(turns[j].StartedAt)
	})

	if (session.SessionID == "" || session.ProjectCWD == "") && fromOffset > 0 {
		meta, metaErr := readSessionBootstrap(path)
		if metaErr == nil {
			if session.SessionID == "" {
				session.SessionID = meta.SessionID
			}
			if session.ProjectCWD == "" {
				session.ProjectCWD = meta.ProjectCWD
			}
			if session.Originator == "" {
				session.Originator = meta.Originator
			}
			session.SourceFile = path
		}
	}

	return &parsedFile{
		Session: session,
		Turns:   turns,
		Offset:  safeOffset,
	}, nil
}

func handleEventMessage(payload json.RawMessage, ts time.Time, builders map[string]*turnBuilder, callToTurn map[string]string, patchMetadata map[string]patchApplyEndPayload, lastTurnID string, lineStart int64, warningSink *[]string) string {
	var meta struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(payload, &meta) != nil {
		return lastTurnID
	}

	switch meta.Type {
	case "task_started":
		var data taskStartedPayload
		if json.Unmarshal(payload, &data) == nil {
			builder := ensureTurnBuilder(builders, data.TurnID, "", lineStart)
			if builder.turn.StartedAt.IsZero() {
				builder.turn.StartedAt = ts
			}
			return data.TurnID
		}
	case "task_complete":
		var data taskCompletePayload
		if json.Unmarshal(payload, &data) == nil {
			builder := ensureTurnBuilder(builders, data.TurnID, "", lineStart)
			builder.turn.CompletedAt = ts
			return data.TurnID
		}
	case "user_message":
		var data eventMessagePayload
		if json.Unmarshal(payload, &data) == nil && lastTurnID != "" {
			builder := ensureTurnBuilder(builders, lastTurnID, "", lineStart)
			if !hasRoleMessage(builder.turn.Messages, "user") {
				builder.turn.Messages = append(builder.turn.Messages, Message{
					ID:        fmt.Sprintf("%s:user:%d", lastTurnID, len(builder.turn.Messages)),
					Role:      "user",
					Text:      data.Message,
					Timestamp: ts,
					Raw:       append([]byte(nil), payload...),
				})
			}
		}
	case "agent_message":
		var data eventMessagePayload
		if json.Unmarshal(payload, &data) == nil && lastTurnID != "" {
			builder := ensureTurnBuilder(builders, lastTurnID, "", lineStart)
			if !hasRoleMessage(builder.turn.Messages, "assistant") {
				builder.turn.Messages = append(builder.turn.Messages, Message{
					ID:        fmt.Sprintf("%s:assistant:%d", lastTurnID, len(builder.turn.Messages)),
					Role:      "assistant",
					Text:      data.Message,
					Timestamp: ts,
					Raw:       append([]byte(nil), payload...),
				})
			}
		}
	case "patch_apply_end":
		var data patchApplyEndPayload
		if json.Unmarshal(payload, &data) == nil {
			turnID, ok := callToTurn[data.CallID]
			if !ok && data.TurnID != "" {
				turnID = data.TurnID
				ok = true
			}
			if !ok {
				patchMetadata[data.CallID] = data
				return lastTurnID
			}
			builder := ensureTurnBuilder(builders, turnID, "", lineStart)
			patchMetadata[data.CallID] = data
			for i := range builder.turn.ToolCalls {
				if builder.turn.ToolCalls[i].CallID != data.CallID {
					continue
				}

				merged, mergeErr := mergePatchApplyMetadata(builder.turn.ToolCalls[i].Output, data)
				if mergeErr != nil {
					*warningSink = append(*warningSink, fmt.Sprintf("failed to merge patch_apply_end metadata for call %s: %v", data.CallID, mergeErr))
					return turnID
				}

				builder.turn.ToolCalls[i].Output = merged
				return turnID
			}
			if data.TurnID != "" {
				return data.TurnID
			}
		}
	}

	return lastTurnID
}

func handleResponseItem(payload json.RawMessage, ts time.Time, builders map[string]*turnBuilder, callToTurn map[string]string, patchMetadata map[string]patchApplyEndPayload, lastTurnID string, lineStart int64, warningSink *[]string) string {
	var item responseItemPayload
	if json.Unmarshal(payload, &item) != nil {
		return lastTurnID
	}

	switch item.Type {
	case "function_call", "custom_tool_call":
		if lastTurnID == "" {
			return lastTurnID
		}

		builder := ensureTurnBuilder(builders, lastTurnID, "", lineStart)
		toolName := item.Name
		input := item.Arguments
		if item.Type == "custom_tool_call" {
			input = item.Input
		}
		tool := ToolCall{
			CallID:         item.CallID,
			ToolName:       toolName,
			Input:          []byte(input),
			StartedAt:      ts,
			RawCall:        append([]byte(nil), payload...),
			SupportsReplay: strings.EqualFold(toolName, "apply_patch"),
		}
		builder.turn.ToolCalls = append(builder.turn.ToolCalls, tool)
		callToTurn[item.CallID] = lastTurnID
		return lastTurnID

	case "function_call_output", "custom_tool_call_output":
		turnID, ok := callToTurn[item.CallID]
		if !ok {
			return lastTurnID
		}
		builder := ensureTurnBuilder(builders, turnID, "", lineStart)
		for i := range builder.turn.ToolCalls {
			if builder.turn.ToolCalls[i].CallID != item.CallID {
				continue
			}

			builder.turn.ToolCalls[i].Output = []byte(item.Output)
			builder.turn.ToolCalls[i].CompletedAt = ts
			builder.turn.ToolCalls[i].RawOutput = append([]byte(nil), payload...)
			if metadata, ok := patchMetadata[item.CallID]; ok {
				merged, mergeErr := mergePatchApplyMetadata(builder.turn.ToolCalls[i].Output, metadata)
				if mergeErr != nil {
					*warningSink = append(*warningSink, fmt.Sprintf("failed to merge patch_apply_end metadata for call %s: %v", item.CallID, mergeErr))
				} else {
					builder.turn.ToolCalls[i].Output = merged
				}
			}
			return turnID
		}
		*warningSink = append(*warningSink, fmt.Sprintf("missing tool call for output %s", item.CallID))

	case "message":
		if lastTurnID == "" {
			return lastTurnID
		}

		if item.Role != "assistant" && item.Role != "user" {
			return lastTurnID
		}

		text := extractResponseMessageText(item.Content)
		if text == "" {
			return lastTurnID
		}

		builder := ensureTurnBuilder(builders, lastTurnID, "", lineStart)
		builder.turn.Messages = append(builder.turn.Messages, Message{
			ID:        fmt.Sprintf("%s:%s:%d", lastTurnID, item.Role, len(builder.turn.Messages)),
			Role:      item.Role,
			Text:      text,
			Timestamp: ts,
			Raw:       append([]byte(nil), payload...),
		})
		return lastTurnID
	}

	return lastTurnID
}

func ensureTurnBuilder(builders map[string]*turnBuilder, turnID, sessionID string, lineStart int64) *turnBuilder {
	builder, ok := builders[turnID]
	if !ok {
		builder = &turnBuilder{
			turn: Turn{
				TurnID:    turnID,
				SessionID: sessionID,
			},
			startOffset: lineStart,
		}
		builders[turnID] = builder
	}
	if sessionID != "" && builder.turn.SessionID == "" {
		builder.turn.SessionID = sessionID
	}
	return builder
}

func readSessionBootstrap(path string) (Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = file.Close() }()

	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadBytes('\n')
		trimmed := normalizeJSONLLine(line)
		if len(trimmed) == 0 {
			if err == io.EOF {
				break
			}
			if err != nil {
				return Session{}, err
			}
			continue
		}
		var env rawEnvelope
		if err := json.Unmarshal(trimmed, &env); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		if env.Type != "session_meta" {
			if err == io.EOF {
				break
			}
			continue
		}

		var payload sessionMetaPayload
		if err := json.Unmarshal(env.Payload, &payload); err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		return Session{
			SessionID:  logicalSessionID(payload.ID),
			ProjectCWD: filepath.Clean(payload.CWD),
			Originator: payload.Originator,
			SourceFile: path,
		}, nil
	}
	return Session{}, fmt.Errorf("session_meta not found in %s", path)
}

func extractResponseMessageText(content json.RawMessage) string {
	var blocks []responseMessageContentItem
	if err := json.Unmarshal(content, &blocks); err == nil {
		var parts []string
		for _, block := range blocks {
			switch block.Type {
			case "output_text", "input_text", "text":
			default:
				continue
			}
			if strings.TrimSpace(block.Text) != "" {
				parts = append(parts, block.Text)
			}
		}
		return strings.TrimSpace(strings.Join(parts, "\n\n"))
	}

	var asString string
	if err := json.Unmarshal(content, &asString); err == nil {
		return strings.TrimSpace(asString)
	}

	return ""
}

func logicalSessionID(rawID string) string {
	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return ""
	}
	return "codex:" + rawID
}

func hasRoleMessage(messages []Message, role string) bool {
	for _, msg := range messages {
		if msg.Role == role && strings.TrimSpace(msg.Text) != "" {
			return true
		}
	}
	return false
}

func mergePatchApplyMetadata(existing []byte, payload patchApplyEndPayload) ([]byte, error) {
	var doc map[string]interface{}
	if len(existing) == 0 {
		doc = map[string]interface{}{}
	} else if err := json.Unmarshal(existing, &doc); err != nil {
		doc = map[string]interface{}{
			"output": strings.TrimSpace(string(existing)),
		}
	}

	doc["patch_apply_end"] = map[string]interface{}{
		"call_id": payload.CallID,
		"turn_id": payload.TurnID,
		"stdout":  payload.Stdout,
		"stderr":  payload.Stderr,
		"success": payload.Success,
		"changes": payload.Changes,
	}

	return json.Marshal(doc)
}

func normalizeJSONLLine(line []byte) []byte {
	trimmed := bytes.TrimSpace(line)
	return bytes.TrimPrefix(trimmed, []byte{0xEF, 0xBB, 0xBF})
}
