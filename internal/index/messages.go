package index

import (
	"database/sql"

	"github.com/regent-vcs/regent/internal/store"
)

// Message represents a discrete conversation message
type Message struct {
	ID          string
	SessionID   string
	StepID      string // NULL for orphan messages
	SeqNum      int
	Timestamp   int64
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
		INSERT INTO messages (id, session_id, step_id, seq_num, timestamp, message_type,
		                      content_text, tool_name, tool_use_id, tool_input, tool_output)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, msg.ID, msg.SessionID, nullString(msg.StepID), msg.SeqNum, msg.Timestamp, msg.MessageType,
		nullString(msg.ContentText), nullString(msg.ToolName), nullString(msg.ToolUseID),
		nullString(msg.ToolInput), nullString(msg.ToolOutput))

	if err != nil {
		return err
	}

	return tx.Commit()
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
		SELECT id, session_id, step_id, seq_num, timestamp, message_type,
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
		var stepID, contentText, toolName, toolUseID, toolInput, toolOutput sql.NullString

		err := rows.Scan(&msg.ID, &msg.SessionID, &stepID, &msg.SeqNum, &msg.Timestamp,
			&msg.MessageType, &contentText, &toolName, &toolUseID, &toolInput, &toolOutput)
		if err != nil {
			return nil, err
		}

		if stepID.Valid {
			msg.StepID = stepID.String
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

// GetOrphanMessages returns all messages in a session that aren't linked to a step yet
func (idx *DB) GetOrphanMessages(sessionID string) ([]Message, error) {
	rows, err := idx.db.Query(`
		SELECT id, session_id, step_id, seq_num, timestamp, message_type,
		       content_text, tool_name, tool_use_id, tool_input, tool_output
		FROM messages
		WHERE session_id = ? AND step_id IS NULL
		ORDER BY seq_num ASC
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var messages []Message
	for rows.Next() {
		var msg Message
		var stepID, contentText, toolName, toolUseID, toolInput, toolOutput sql.NullString

		err := rows.Scan(&msg.ID, &msg.SessionID, &stepID, &msg.SeqNum, &msg.Timestamp,
			&msg.MessageType, &contentText, &toolName, &toolUseID, &toolInput, &toolOutput)
		if err != nil {
			return nil, err
		}

		if stepID.Valid {
			msg.StepID = stepID.String
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
	_, err := idx.db.Exec(`
		UPDATE messages
		SET step_id = ?
		WHERE session_id = ? AND step_id IS NULL
	`, stepID, sessionID)

	return err
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
