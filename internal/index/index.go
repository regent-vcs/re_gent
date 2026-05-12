package index

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/store"
	_ "modernc.org/sqlite"
)

var ErrSessionHasNoSteps = errors.New("session has no steps")

// DB wraps the SQLite index
type DB struct {
	db *sql.DB
}

// Open opens the SQLite index (creates if doesn't exist)
func Open(s *store.Store) (*DB, error) {
	dbPath := filepath.Join(s.Root, "index.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure SQLite for concurrency
	if _, err := db.Exec("PRAGMA journal_mode = WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	// Create schema
	if err := createSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &DB{db: db}, nil
}

func createSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS steps (
		id          TEXT PRIMARY KEY,
		parent_id   TEXT,
		session_id  TEXT NOT NULL,
		origin      TEXT NOT NULL DEFAULT 'claude_code',
		turn_id     TEXT,
		agent_id    TEXT,
		ts_nanos    INTEGER NOT NULL,
		tool_name   TEXT NOT NULL,
		tool_use_id TEXT NOT NULL,
		tree_hash   TEXT NOT NULL,
		transcript_hash TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_steps_session ON steps(session_id, ts_nanos);
	CREATE INDEX IF NOT EXISTS idx_steps_parent ON steps(parent_id);
	CREATE INDEX IF NOT EXISTS idx_steps_tool_use ON steps(tool_use_id);

	CREATE TABLE IF NOT EXISTS step_files (
		step_id   TEXT NOT NULL,
		path      TEXT NOT NULL,
		blob_hash TEXT NOT NULL,
		PRIMARY KEY (step_id, path)
	);
	CREATE INDEX IF NOT EXISTS idx_step_files_path ON step_files(path);

	CREATE TABLE IF NOT EXISTS sessions (
		id            TEXT PRIMARY KEY,
		origin        TEXT NOT NULL,
		started_at    INTEGER NOT NULL,
		last_seen_at  INTEGER NOT NULL,
		head_step_id  TEXT,
		model         TEXT,
		permission_mode TEXT,
		transcript_path TEXT,
		forked_from_session TEXT,
		forked_from_step    TEXT,
		fork_detected_at    INTEGER
	);

	CREATE TABLE IF NOT EXISTS session_transcript (
		session_id           TEXT PRIMARY KEY,
		last_message_id      TEXT NOT NULL,
		last_transcript_hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS messages (
		id              TEXT PRIMARY KEY,
		session_id      TEXT NOT NULL,
		step_id         TEXT,
		turn_id         TEXT,
		seq_num         INTEGER NOT NULL,
		timestamp       INTEGER NOT NULL,
		processed_at    INTEGER,
		message_type    TEXT NOT NULL,
		content_text    TEXT,
		tool_name       TEXT,
		tool_use_id     TEXT,
		tool_input      TEXT,
		tool_output     TEXT,
		FOREIGN KEY (step_id) REFERENCES steps(id)
	);
	CREATE INDEX IF NOT EXISTS idx_messages_session_seq ON messages(session_id, seq_num);
	CREATE INDEX IF NOT EXISTS idx_messages_step ON messages(step_id);
	CREATE INDEX IF NOT EXISTS idx_messages_tool_use ON messages(tool_use_id);

	CREATE TABLE IF NOT EXISTS tool_uses (
		session_id  TEXT NOT NULL,
		turn_id     TEXT NOT NULL,
		tool_use_id TEXT NOT NULL,
		PRIMARY KEY (session_id, turn_id, tool_use_id)
	);

	CREATE TABLE IF NOT EXISTS jsonl_snapshots (
		session_id      TEXT NOT NULL,
		captured_at     INTEGER NOT NULL,
		jsonl_blob      TEXT NOT NULL,
		PRIMARY KEY (session_id, captured_at)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// Migrate existing tables (add columns if missing)
	return migrateSchema(db)
}

// migrateSchema adds new columns to existing tables
func migrateSchema(db *sql.DB) error {
	migrations := map[string][]string{
		"sessions": {
			`ALTER TABLE sessions ADD COLUMN forked_from_session TEXT`,
			`ALTER TABLE sessions ADD COLUMN forked_from_step TEXT`,
			`ALTER TABLE sessions ADD COLUMN fork_detected_at INTEGER`,
			`ALTER TABLE sessions ADD COLUMN model TEXT`,
			`ALTER TABLE sessions ADD COLUMN permission_mode TEXT`,
			`ALTER TABLE sessions ADD COLUMN transcript_path TEXT`,
		},
		"steps": {
			`ALTER TABLE steps ADD COLUMN origin TEXT NOT NULL DEFAULT 'claude_code'`,
			`ALTER TABLE steps ADD COLUMN turn_id TEXT`,
		},
		"messages": {
			`ALTER TABLE messages ADD COLUMN turn_id TEXT`,
			`ALTER TABLE messages ADD COLUMN processed_at INTEGER`,
		},
	}

	for table, stmts := range migrations {
		for _, stmt := range stmts {
			column := migrationColumn(stmt)
			exists, err := columnExists(db, table, column)
			if err != nil {
				return fmt.Errorf("check column %s.%s: %w", table, column, err)
			}
			if exists {
				continue
			}
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("migration failed: %s: %w", stmt, err)
			}
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_steps_turn ON steps(session_id, turn_id)`,
		`CREATE INDEX IF NOT EXISTS idx_messages_session_turn ON messages(session_id, turn_id)`,
	}
	for _, stmt := range indexes {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("migration failed: %s: %w", stmt, err)
		}
	}

	if _, err := db.Exec(`
		INSERT OR IGNORE INTO tool_uses (session_id, turn_id, tool_use_id)
		SELECT session_id, COALESCE(turn_id, ''), tool_use_id
		FROM messages
		WHERE message_type = 'tool_call'
		  AND tool_use_id IS NOT NULL
		  AND tool_use_id != ''
	`); err != nil {
		return fmt.Errorf("backfill tool uses: %w", err)
	}

	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info(?)
		WHERE name=?
	`, table, column).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func migrationColumn(stmt string) string {
	fields := strings.Fields(stmt)
	for i, field := range fields {
		if strings.EqualFold(field, "COLUMN") && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// Close closes the database connection
func (idx *DB) Close() error {
	return idx.db.Close()
}

const defaultOrigin = "claude_code"

func originOrDefault(origin string) string {
	if origin == "" {
		return defaultOrigin
	}
	return origin
}

// SessionUpdate holds optional metadata observed from an agent host.
type SessionUpdate struct {
	ID             string
	Origin         string
	HeadStepID     store.Hash
	Model          string
	PermissionMode string
	TranscriptPath string
}

// UpsertSession records a session before or after a step exists.
func (idx *DB) UpsertSession(update SessionUpdate) error {
	if update.ID == "" {
		return fmt.Errorf("session id is required")
	}

	now := time.Now().UnixNano()
	_, err := idx.db.Exec(`
		INSERT INTO sessions (id, origin, started_at, last_seen_at, head_step_id,
		                      model, permission_mode, transcript_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			origin = COALESCE(NULLIF(?, ''), origin),
			last_seen_at = ?,
			head_step_id = COALESCE(NULLIF(?, ''), head_step_id),
			model = COALESCE(NULLIF(?, ''), model),
			permission_mode = COALESCE(NULLIF(?, ''), permission_mode),
			transcript_path = COALESCE(NULLIF(?, ''), transcript_path)
	`, update.ID, originOrDefault(update.Origin), now, now, nullString(string(update.HeadStepID)),
		nullString(update.Model), nullString(update.PermissionMode), nullString(update.TranscriptPath),
		update.Origin, now, string(update.HeadStepID), update.Model, update.PermissionMode, update.TranscriptPath)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}

	return nil
}

// RenameSession moves index rows from an older session ID to a canonical ID.
func (idx *DB) RenameSession(oldID, newID, origin string) (bool, error) {
	if oldID == "" {
		return false, fmt.Errorf("old session id is required")
	}
	if newID == "" {
		return false, fmt.Errorf("new session id is required")
	}
	if oldID == newID {
		return false, nil
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	changed := false
	oldSessionExists, err := sessionExists(tx, oldID)
	if err != nil {
		return false, fmt.Errorf("check old session: %w", err)
	}
	newSessionExists, err := sessionExists(tx, newID)
	if err != nil {
		return false, fmt.Errorf("check new session: %w", err)
	}

	if oldSessionExists && !newSessionExists {
		result, err := tx.Exec(`
			UPDATE sessions
			SET id = ?, origin = COALESCE(NULLIF(?, ''), origin)
			WHERE id = ?
		`, newID, origin, oldID)
		if err != nil {
			return false, fmt.Errorf("rename session row: %w", err)
		}
		changed = rowsChanged(result) || changed
	} else if oldSessionExists {
		_, err := tx.Exec(`
			UPDATE sessions
			SET
				origin = COALESCE(NULLIF(origin, ''), NULLIF(?, ''), origin),
				started_at = MIN(started_at, (SELECT started_at FROM sessions WHERE id = ?)),
				last_seen_at = MAX(last_seen_at, (SELECT last_seen_at FROM sessions WHERE id = ?)),
				head_step_id = COALESCE(head_step_id, (SELECT head_step_id FROM sessions WHERE id = ?)),
				model = COALESCE(NULLIF(model, ''), (SELECT model FROM sessions WHERE id = ?)),
				permission_mode = COALESCE(NULLIF(permission_mode, ''), (SELECT permission_mode FROM sessions WHERE id = ?)),
				transcript_path = COALESCE(NULLIF(transcript_path, ''), (SELECT transcript_path FROM sessions WHERE id = ?)),
				forked_from_session = COALESCE(forked_from_session, (SELECT forked_from_session FROM sessions WHERE id = ?)),
				forked_from_step = COALESCE(forked_from_step, (SELECT forked_from_step FROM sessions WHERE id = ?)),
				fork_detected_at = COALESCE(fork_detected_at, (SELECT fork_detected_at FROM sessions WHERE id = ?))
			WHERE id = ?
		`, origin, oldID, oldID, oldID, oldID, oldID, oldID, oldID, oldID, oldID, newID)
		if err != nil {
			return false, fmt.Errorf("merge session row: %w", err)
		}
		result, err := tx.Exec(`DELETE FROM sessions WHERE id = ?`, oldID)
		if err != nil {
			return false, fmt.Errorf("delete old session row: %w", err)
		}
		changed = rowsChanged(result) || changed
	}

	for _, stmt := range []string{
		`UPDATE steps SET session_id = ? WHERE session_id = ?`,
		`UPDATE messages SET session_id = ? WHERE session_id = ?`,
		`UPDATE sessions SET forked_from_session = ? WHERE forked_from_session = ?`,
	} {
		result, err := tx.Exec(stmt, newID, oldID)
		if err != nil {
			return false, fmt.Errorf("rename session references: %w", err)
		}
		changed = rowsChanged(result) || changed
	}

	result, err := tx.Exec(`
		INSERT OR IGNORE INTO tool_uses (session_id, turn_id, tool_use_id)
		SELECT ?, turn_id, tool_use_id
		FROM tool_uses
		WHERE session_id = ?
	`, newID, oldID)
	if err != nil {
		return false, fmt.Errorf("copy tool uses: %w", err)
	}
	changed = rowsChanged(result) || changed
	result, err = tx.Exec(`DELETE FROM tool_uses WHERE session_id = ?`, oldID)
	if err != nil {
		return false, fmt.Errorf("delete old tool uses: %w", err)
	}
	changed = rowsChanged(result) || changed

	result, err = tx.Exec(`
		INSERT OR IGNORE INTO session_transcript (session_id, last_message_id, last_transcript_hash)
		SELECT ?, last_message_id, last_transcript_hash
		FROM session_transcript
		WHERE session_id = ?
	`, newID, oldID)
	if err != nil {
		return false, fmt.Errorf("copy session transcript: %w", err)
	}
	changed = rowsChanged(result) || changed
	result, err = tx.Exec(`DELETE FROM session_transcript WHERE session_id = ?`, oldID)
	if err != nil {
		return false, fmt.Errorf("delete old session transcript: %w", err)
	}
	changed = rowsChanged(result) || changed

	result, err = tx.Exec(`
		INSERT OR IGNORE INTO jsonl_snapshots (session_id, captured_at, jsonl_blob)
		SELECT ?, captured_at, jsonl_blob
		FROM jsonl_snapshots
		WHERE session_id = ?
	`, newID, oldID)
	if err != nil {
		return false, fmt.Errorf("copy jsonl snapshots: %w", err)
	}
	changed = rowsChanged(result) || changed
	result, err = tx.Exec(`DELETE FROM jsonl_snapshots WHERE session_id = ?`, oldID)
	if err != nil {
		return false, fmt.Errorf("delete old jsonl snapshots: %w", err)
	}
	changed = rowsChanged(result) || changed

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return changed, nil
}

func sessionExists(tx *sql.Tx, sessionID string) (bool, error) {
	var count int
	err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ?`, sessionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func rowsChanged(result sql.Result) bool {
	rows, err := result.RowsAffected()
	return err == nil && rows > 0
}

// IndexStep indexes a step and its files
func (idx *DB) IndexStep(stepHash store.Hash, step *store.Step, tree *store.Tree) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	primaryCause := step.PrimaryCause()

	// Insert step
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO steps
		(id, parent_id, session_id, origin, turn_id, agent_id, ts_nanos, tool_name, tool_use_id, tree_hash, transcript_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		stepHash,
		step.Parent,
		step.SessionID,
		originOrDefault(step.Origin),
		nullString(step.TurnID),
		step.AgentID,
		step.TimestampNanos,
		primaryCause.ToolName,
		primaryCause.ToolUseID,
		step.Tree,
		step.Transcript,
	)
	if err != nil {
		return fmt.Errorf("insert step: %w", err)
	}

	// Insert file entries
	for _, entry := range tree.Entries {
		_, err = tx.Exec(`
			INSERT OR REPLACE INTO step_files (step_id, path, blob_hash)
			VALUES (?, ?, ?)
		`, stepHash, entry.Path, entry.Blob)
		if err != nil {
			return fmt.Errorf("insert step file: %w", err)
		}
	}

	// Detect fork: if parent is from different session, this is a fork
	var forkedFromSession string
	var forkedFromStep store.Hash
	if step.Parent != "" {
		var parentSessionID string
		err := tx.QueryRow(`
			SELECT session_id FROM steps WHERE id = ?
		`, step.Parent).Scan(&parentSessionID)

		if err == nil && parentSessionID != step.SessionID {
			// This is a fork!
			forkedFromSession = parentSessionID
			forkedFromStep = step.Parent
		}
	}

	// Update session record
	now := time.Now().UnixNano()
	if forkedFromSession != "" {
		// First step in a forked session
		_, err = tx.Exec(`
			INSERT INTO sessions (id, origin, started_at, last_seen_at, head_step_id,
			                     forked_from_session, forked_from_step, fork_detected_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				last_seen_at = ?,
				head_step_id = ?,
				origin = COALESCE(NULLIF(?, ''), origin),
				forked_from_session = COALESCE(forked_from_session, ?),
				forked_from_step = COALESCE(forked_from_step, ?),
				fork_detected_at = COALESCE(fork_detected_at, ?)
		`, step.SessionID, originOrDefault(step.Origin), now, now, stepHash,
			forkedFromSession, forkedFromStep, now,
			now, stepHash, step.Origin,
			forkedFromSession, forkedFromStep, now)
	} else {
		// Normal session continuation or new session
		_, err = tx.Exec(`
			INSERT INTO sessions (id, origin, started_at, last_seen_at, head_step_id)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				last_seen_at = ?,
				head_step_id = ?,
				origin = COALESCE(NULLIF(?, ''), origin)
		`, step.SessionID, originOrDefault(step.Origin), now, now, stepHash, now, stepHash, step.Origin)
	}
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return tx.Commit()
}

// SessionHead returns the head step hash for a session
func (idx *DB) SessionHead(sessionID string) (store.Hash, error) {
	var headHash sql.NullString
	err := idx.db.QueryRow("SELECT head_step_id FROM sessions WHERE id = ?", sessionID).Scan(&headHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("session not found: %s", sessionID)
		}
		return "", err
	}
	if !headHash.Valid || headHash.String == "" {
		return "", fmt.Errorf("%w: %s", ErrSessionHasNoSteps, sessionID)
	}
	return store.Hash(headHash.String), nil
}

// StepForTurn returns the indexed step for a captured turn.
func (idx *DB) StepForTurn(sessionID, turnID string) (store.Hash, bool, error) {
	if sessionID == "" {
		return "", false, fmt.Errorf("session id is required")
	}
	if turnID == "" {
		return "", false, fmt.Errorf("turn id is required")
	}

	var stepHash string
	err := idx.db.QueryRow(`
		SELECT id FROM steps
		WHERE session_id = ? AND turn_id = ?
		ORDER BY ts_nanos DESC
		LIMIT 1
	`, sessionID, turnID).Scan(&stepHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", false, nil
		}
		return "", false, err
	}
	return store.Hash(stepHash), true, nil
}

// StepInfo holds displayable info about a step
type StepInfo struct {
	Hash           store.Hash
	ParentHash     store.Hash
	SessionID      string
	Origin         string
	TurnID         string
	Timestamp      time.Time
	ToolName       string
	ToolUseID      string
	TreeHash       store.Hash
	TranscriptHash store.Hash
	ArgsBlob       store.Hash
	ResultBlob     store.Hash
}

// ListSteps returns recent steps for a session (newest first)
func (idx *DB) ListSteps(sessionID string, limit int) ([]StepInfo, error) {
	query := `
		SELECT id, parent_id, session_id, origin, turn_id, ts_nanos, tool_name, tool_use_id,
		       tree_hash, transcript_hash
		FROM steps
		WHERE session_id = ?
		ORDER BY ts_nanos DESC
		LIMIT ?
	`

	rows, err := idx.db.Query(query, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var steps []StepInfo
	for rows.Next() {
		var s StepInfo
		var parentHash, turnID sql.NullString
		var transcriptHash sql.NullString
		var tsNanos int64

		err := rows.Scan(&s.Hash, &parentHash, &s.SessionID, &s.Origin, &turnID, &tsNanos, &s.ToolName, &s.ToolUseID,
			&s.TreeHash, &transcriptHash)
		if err != nil {
			return nil, err
		}

		if parentHash.Valid {
			s.ParentHash = store.Hash(parentHash.String)
		}
		if transcriptHash.Valid {
			s.TranscriptHash = store.Hash(transcriptHash.String)
		}
		if turnID.Valid {
			s.TurnID = turnID.String
		}
		s.Timestamp = time.Unix(0, tsNanos)

		steps = append(steps, s)
	}

	return steps, rows.Err()
}

// ListAllSessions returns all sessions
func (idx *DB) ListAllSessions() ([]SessionInfo, error) {
	rows, err := idx.db.Query(`
		SELECT id, origin, started_at, last_seen_at, head_step_id,
		       model, permission_mode, transcript_path,
		       forked_from_session, forked_from_step, fork_detected_at
		FROM sessions
		ORDER BY last_seen_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var sessions []SessionInfo
	for rows.Next() {
		var s SessionInfo
		var startedAt, lastSeenAt int64
		var headStepID, model, permissionMode, transcriptPath sql.NullString
		var forkedFromSession, forkedFromStep sql.NullString
		var forkDetectedAt sql.NullInt64

		err := rows.Scan(&s.ID, &s.Origin, &startedAt, &lastSeenAt, &headStepID,
			&model, &permissionMode, &transcriptPath,
			&forkedFromSession, &forkedFromStep, &forkDetectedAt)
		if err != nil {
			return nil, err
		}

		s.StartedAt = time.Unix(0, startedAt)
		s.LastSeenAt = time.Unix(0, lastSeenAt)
		if headStepID.Valid {
			s.HeadStepID = store.Hash(headStepID.String)
		}
		if model.Valid {
			s.Model = model.String
		}
		if permissionMode.Valid {
			s.PermissionMode = permissionMode.String
		}
		if transcriptPath.Valid {
			s.TranscriptPath = transcriptPath.String
		}
		if forkedFromSession.Valid {
			s.ForkedFromSession = forkedFromSession.String
		}
		if forkedFromStep.Valid {
			s.ForkedFromStep = store.Hash(forkedFromStep.String)
		}
		if forkDetectedAt.Valid {
			t := time.Unix(0, forkDetectedAt.Int64)
			s.ForkDetectedAt = &t
		}

		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// ListHeadedSessions returns sessions that have at least one indexed step.
func (idx *DB) ListHeadedSessions() ([]SessionInfo, error) {
	sessions, err := idx.ListAllSessions()
	if err != nil {
		return nil, err
	}

	headed := make([]SessionInfo, 0, len(sessions))
	for _, session := range sessions {
		if session.HeadStepID != "" {
			headed = append(headed, session)
		}
	}
	return headed, nil
}

// SessionInfo holds displayable session info
type SessionInfo struct {
	ID                string
	Origin            string
	StartedAt         time.Time
	LastSeenAt        time.Time
	HeadStepID        store.Hash
	Model             string
	PermissionMode    string
	TranscriptPath    string
	ForkedFromSession string
	ForkedFromStep    store.Hash
	ForkDetectedAt    *time.Time // pointer for nullable
}

// SessionLastProcessedMessage returns the last message ID and transcript hash for a session
// Returns ("", "", nil) if session has no transcript history yet
func (idx *DB) SessionLastProcessedMessage(sessionID string) (string, store.Hash, error) {
	var lastMsgID string
	var lastTranscript string

	err := idx.db.QueryRow(`
		SELECT last_message_id, last_transcript_hash
		FROM session_transcript
		WHERE session_id = ?
	`, sessionID).Scan(&lastMsgID, &lastTranscript)

	if err == sql.ErrNoRows {
		return "", "", nil // New session
	}
	if err != nil {
		return "", "", err
	}

	return lastMsgID, store.Hash(lastTranscript), nil
}

// UpdateSessionLastProcessed records the last processed message for a session
func (idx *DB) UpdateSessionLastProcessed(sessionID, lastMsgID string, transcriptHash store.Hash) error {
	_, err := idx.db.Exec(`
		INSERT OR REPLACE INTO session_transcript (session_id, last_message_id, last_transcript_hash)
		VALUES (?, ?, ?)
	`, sessionID, lastMsgID, string(transcriptHash))
	return err
}
