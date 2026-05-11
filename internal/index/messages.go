package index

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/regent-vcs/regent/internal/store"
)

// Message represents a discrete conversation message
type Message struct {
	ID          string
	SessionID   string
	StepID      string // NULL for orphan messages
	TurnID      string
	SeqNum      int
	Timestamp   int64
	ProcessedAt int64
	MessageType string // 'user', 'assistant', 'tool_call', 'tool_result'
	ContentText string
	ToolName    string
	ToolUseID   string
	ToolInput   string // JSON blob hash
	ToolOutput  string // JSON blob hash
}

// InsertMessage stores a message in the database
func (idx *DB) InsertMessage(msg Message) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.Exec(`
		INSERT INTO messages (id, session_id, step_id, turn_id, seq_num, timestamp, processed_at, message_type,
		                      content_text, tool_name, tool_use_id, tool_input, tool_output)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.SessionID, nullString(msg.StepID), nullString(msg.TurnID), msg.SeqNum, msg.Timestamp,
		nullInt64(msg.ProcessedAt), msg.MessageType,
		nullString(msg.ContentText), nullString(msg.ToolName), nullString(msg.ToolUseID),
		nullString(msg.ToolInput), nullString(msg.ToolOutput))

	if err != nil {
		return err
	}

	return tx.Commit()
}

// AppendMessage assigns the next session sequence number and stores a message.
func (idx *DB) AppendMessage(msg Message) error {
	if msg.SessionID == "" {
		return fmt.Errorf("session id is required")
	}

	_, err := idx.db.Exec(`
		INSERT INTO messages (id, session_id, step_id, turn_id, seq_num, timestamp, processed_at, message_type,
		                      content_text, tool_name, tool_use_id, tool_input, tool_output)
		SELECT ?, ?, ?, ?, COALESCE(MAX(seq_num), -1) + 1, ?, ?, ?, ?, ?, ?, ?, ?
		FROM messages
		WHERE session_id = ?
	`, msg.ID, msg.SessionID, nullString(msg.StepID), nullString(msg.TurnID), msg.Timestamp,
		nullInt64(msg.ProcessedAt), msg.MessageType,
		nullString(msg.ContentText), nullString(msg.ToolName), nullString(msg.ToolUseID),
		nullString(msg.ToolInput), nullString(msg.ToolOutput), msg.SessionID)
	return err
}

// GetNextMessageSeq returns the next sequence number for a session
func (idx *DB) GetNextMessageSeq(sessionID string) (int, error) {
	var maxSeq sql.NullInt64
	err := idx.db.QueryRow(`
		SELECT MAX(seq_num) FROM messages WHERE session_id = ?
	`, sessionID).Scan(&maxSeq)

	if err != nil {
		return 0, err
	}

	if !maxSeq.Valid {
		return 0, nil // First message
	}

	return int(maxSeq.Int64) + 1, nil
}

// GetMessagesForStep returns all messages linked to a step
func (idx *DB) GetMessagesForStep(stepID store.Hash) ([]Message, error) {
	rows, err := idx.db.Query(`
		SELECT id, session_id, step_id, turn_id, seq_num, timestamp, processed_at, message_type,
		       content_text, tool_name, tool_use_id, tool_input, tool_output
		FROM messages
		WHERE step_id = ?
		ORDER BY seq_num ASC
	`, stepID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var msg Message
		var stepID, turnID, contentText, toolName, toolUseID, toolInput, toolOutput sql.NullString
		var processedAt sql.NullInt64

		err := rows.Scan(&msg.ID, &msg.SessionID, &stepID, &turnID, &msg.SeqNum, &msg.Timestamp,
			&processedAt,
			&msg.MessageType, &contentText, &toolName, &toolUseID, &toolInput, &toolOutput)
		if err != nil {
			return nil, err
		}

		if stepID.Valid {
			msg.StepID = stepID.String
		}
		if turnID.Valid {
			msg.TurnID = turnID.String
		}
		if processedAt.Valid {
			msg.ProcessedAt = processedAt.Int64
		}
		if contentText.Valid {
			msg.ContentText = contentText.String
		}
		if toolName.Valid {
			msg.ToolName = toolName.String
		}
		if toolUseID.Valid {
			msg.ToolUseID = toolUseID.String
		}
		if toolInput.Valid {
			msg.ToolInput = toolInput.String
		}
		if toolOutput.Valid {
			msg.ToolOutput = toolOutput.String
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// ToolUseExists reports whether a tool call was already recorded.
func (idx *DB) ToolUseExists(sessionID, turnID, toolUseID string, allTurns bool) (bool, error) {
	if sessionID == "" {
		return false, fmt.Errorf("session id is required")
	}
	if toolUseID == "" {
		return false, fmt.Errorf("tool use id is required")
	}
	if !allTurns && turnID == "" {
		return false, fmt.Errorf("turn id is required")
	}

	query := `
		SELECT COUNT(*)
		FROM messages
		WHERE session_id = ?
		  AND message_type = 'tool_call'
		  AND tool_use_id = ?
	`
	args := []interface{}{sessionID, toolUseID}
	if !allTurns {
		query += ` AND turn_id = ?`
		args = append(args, turnID)
	}

	var count int
	if err := idx.db.QueryRow(query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetOrphanMessages returns all messages in a session that aren't linked to a step yet
func (idx *DB) GetOrphanMessages(sessionID string) ([]Message, error) {
	return idx.GetAllPendingMessages(sessionID)
}

// GetAllPendingMessages returns unprocessed messages for a session across turns.
func (idx *DB) GetAllPendingMessages(sessionID string) ([]Message, error) {
	return idx.getPendingMessages(sessionID, "", true)
}

// GetPendingMessages returns unprocessed messages for one explicit turn.
func (idx *DB) GetPendingMessages(sessionID, turnID string) ([]Message, error) {
	return idx.getPendingMessages(sessionID, turnID, false)
}

func (idx *DB) getPendingMessages(sessionID, turnID string, allTurns bool) ([]Message, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session id is required")
	}
	if !allTurns && turnID == "" {
		return nil, fmt.Errorf("turn id is required")
	}

	where := `WHERE session_id = ? AND step_id IS NULL AND processed_at IS NULL`
	args := []interface{}{sessionID}
	if !allTurns {
		where += ` AND turn_id = ?`
		args = append(args, turnID)
	}

	rows, err := idx.db.Query(`
		SELECT id, session_id, step_id, turn_id, seq_num, timestamp, processed_at, message_type,
		       content_text, tool_name, tool_use_id, tool_input, tool_output
		FROM messages
		`+where+`
		ORDER BY seq_num ASC
	`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var msg Message
		var stepID, turnID, contentText, toolName, toolUseID, toolInput, toolOutput sql.NullString
		var processedAt sql.NullInt64

		err := rows.Scan(&msg.ID, &msg.SessionID, &stepID, &turnID, &msg.SeqNum, &msg.Timestamp,
			&processedAt,
			&msg.MessageType, &contentText, &toolName, &toolUseID, &toolInput, &toolOutput)
		if err != nil {
			return nil, err
		}

		if stepID.Valid {
			msg.StepID = stepID.String
		}
		if turnID.Valid {
			msg.TurnID = turnID.String
		}
		if processedAt.Valid {
			msg.ProcessedAt = processedAt.Int64
		}
		if contentText.Valid {
			msg.ContentText = contentText.String
		}
		if toolName.Valid {
			msg.ToolName = toolName.String
		}
		if toolUseID.Valid {
			msg.ToolUseID = toolUseID.String
		}
		if toolInput.Valid {
			msg.ToolInput = toolInput.String
		}
		if toolOutput.Valid {
			msg.ToolOutput = toolOutput.String
		}

		messages = append(messages, msg)
	}

	return messages, rows.Err()
}

// LinkMessagesToStep updates orphan messages (step_id IS NULL) to link them to a step
func (idx *DB) LinkMessagesToStep(sessionID string, stepID store.Hash) error {
	_, err := idx.LinkAllPendingMessagesToStep(sessionID, stepID, 0)
	return err
}

// LinkAllPendingMessagesToStep links all pending messages to a step and marks them processed.
func (idx *DB) LinkAllPendingMessagesToStep(sessionID string, stepID store.Hash, processedAt int64) (int64, error) {
	return idx.linkPendingMessagesToStep(sessionID, "", stepID, processedAt, true)
}

// LinkPendingMessagesToStep links pending messages for one turn to a step and marks them processed.
func (idx *DB) LinkPendingMessagesToStep(sessionID, turnID string, stepID store.Hash, processedAt int64) (int64, error) {
	return idx.linkPendingMessagesToStep(sessionID, turnID, stepID, processedAt, false)
}

func (idx *DB) linkPendingMessagesToStep(sessionID, turnID string, stepID store.Hash, processedAt int64, allTurns bool) (int64, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}
	if stepID == "" {
		return 0, fmt.Errorf("step id is required")
	}
	if !allTurns && turnID == "" {
		return 0, fmt.Errorf("turn id is required")
	}
	if processedAt == 0 {
		processedAt = timeNow()
	}
	query := `
		UPDATE messages
		SET step_id = ?, processed_at = ?
		WHERE session_id = ? AND step_id IS NULL AND processed_at IS NULL
	`
	args := []interface{}{stepID, processedAt, sessionID}
	if !allTurns {
		query += ` AND turn_id = ?`
		args = append(args, turnID)
	}
	result, err := idx.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// MarkAllPendingMessagesProcessed marks all pending messages as consumed without creating a step.
func (idx *DB) MarkAllPendingMessagesProcessed(sessionID string, processedAt int64) (int64, error) {
	return idx.markPendingMessagesProcessed(sessionID, "", processedAt, true)
}

// MarkPendingMessagesProcessed marks a no-tool turn as consumed without creating a step.
func (idx *DB) MarkPendingMessagesProcessed(sessionID, turnID string, processedAt int64) (int64, error) {
	return idx.markPendingMessagesProcessed(sessionID, turnID, processedAt, false)
}

func (idx *DB) markPendingMessagesProcessed(sessionID, turnID string, processedAt int64, allTurns bool) (int64, error) {
	if sessionID == "" {
		return 0, fmt.Errorf("session id is required")
	}
	if !allTurns && turnID == "" {
		return 0, fmt.Errorf("turn id is required")
	}
	if processedAt == 0 {
		processedAt = timeNow()
	}
	query := `
		UPDATE messages
		SET processed_at = ?
		WHERE session_id = ? AND step_id IS NULL AND processed_at IS NULL
	`
	args := []interface{}{processedAt, sessionID}
	if !allTurns {
		query += ` AND turn_id = ?`
		args = append(args, turnID)
	}

	result, err := idx.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// InsertJSONLSnapshot stores a JSONL archive snapshot
func (idx *DB) InsertJSONLSnapshot(sessionID string, capturedAt int64, blobHash store.Hash) error {
	_, err := idx.db.Exec(`
		INSERT INTO jsonl_snapshots (session_id, captured_at, jsonl_blob)
		VALUES (?, ?, ?)
	`, sessionID, capturedAt, blobHash)

	return err
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt64(n int64) sql.NullInt64 {
	if n == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: n, Valid: true}
}

func timeNow() int64 {
	return time.Now().UnixNano()
}
