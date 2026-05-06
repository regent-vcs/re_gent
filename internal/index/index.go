package index

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/store"
	_ "modernc.org/sqlite"
)

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
		seq_num         INTEGER NOT NULL,
		timestamp       INTEGER NOT NULL,
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
	// Check if fork columns exist
	var count int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM pragma_table_info('sessions')
		WHERE name='forked_from_session'
	`).Scan(&count)

	if err != nil {
		return fmt.Errorf("check schema: %w", err)
	}

	if count == 0 {
		// Add fork tracking columns
		migrations := []string{
			`ALTER TABLE sessions ADD COLUMN forked_from_session TEXT`,
			`ALTER TABLE sessions ADD COLUMN forked_from_step TEXT`,
			`ALTER TABLE sessions ADD COLUMN fork_detected_at INTEGER`,
		}

		for _, migration := range migrations {
			if _, err := db.Exec(migration); err != nil {
				return fmt.Errorf("migration failed: %s: %w", migration, err)
			}
		}
	}

	return nil
}

// Close closes the database connection
func (idx *DB) Close() error {
	return idx.db.Close()
}

// IndexStep indexes a step and its files
func (idx *DB) IndexStep(stepHash store.Hash, step *store.Step, tree *store.Tree) error {
	tx, err := idx.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Insert step
	_, err = tx.Exec(`
		INSERT OR REPLACE INTO steps
		(id, parent_id, session_id, agent_id, ts_nanos, tool_name, tool_use_id, tree_hash, transcript_hash)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		stepHash,
		step.Parent,
		step.SessionID,
		step.AgentID,
		step.TimestampNanos,
		step.Cause.ToolName,
		step.Cause.ToolUseID,
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
			VALUES (?, 'claude_code', ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				last_seen_at = ?,
				head_step_id = ?,
				forked_from_session = COALESCE(forked_from_session, ?),
				forked_from_step = COALESCE(forked_from_step, ?),
				fork_detected_at = COALESCE(fork_detected_at, ?)
		`, step.SessionID, now, now, stepHash,
			forkedFromSession, forkedFromStep, now,
			now, stepHash,
			forkedFromSession, forkedFromStep, now)
	} else {
		// Normal session continuation or new session
		_, err = tx.Exec(`
			INSERT INTO sessions (id, origin, started_at, last_seen_at, head_step_id)
			VALUES (?, 'claude_code', ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				last_seen_at = ?,
				head_step_id = ?
		`, step.SessionID, now, now, stepHash, now, stepHash)
	}
	if err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	return tx.Commit()
}

// SessionHead returns the head step hash for a session
func (idx *DB) SessionHead(sessionID string) (store.Hash, error) {
	var headHash string
	err := idx.db.QueryRow("SELECT head_step_id FROM sessions WHERE id = ?", sessionID).Scan(&headHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("session not found: %s", sessionID)
		}
		return "", err
	}
	return store.Hash(headHash), nil
}

// StepInfo holds displayable info about a step
type StepInfo struct {
	Hash           store.Hash
	ParentHash     store.Hash
	SessionID      string
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
		SELECT id, parent_id, session_id, ts_nanos, tool_name, tool_use_id,
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
		var parentHash sql.NullString
		var transcriptHash sql.NullString
		var tsNanos int64

		err := rows.Scan(&s.Hash, &parentHash, &s.SessionID, &tsNanos, &s.ToolName, &s.ToolUseID,
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
		s.Timestamp = time.Unix(0, tsNanos)

		steps = append(steps, s)
	}

	return steps, rows.Err()
}

// ListAllSessions returns all sessions
func (idx *DB) ListAllSessions() ([]SessionInfo, error) {
	rows, err := idx.db.Query(`
		SELECT id, origin, started_at, last_seen_at, head_step_id,
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
		var headStepID sql.NullString
		var forkedFromSession, forkedFromStep sql.NullString
		var forkDetectedAt sql.NullInt64

		err := rows.Scan(&s.ID, &s.Origin, &startedAt, &lastSeenAt, &headStepID,
			&forkedFromSession, &forkedFromStep, &forkDetectedAt)
		if err != nil {
			return nil, err
		}

		s.StartedAt = time.Unix(0, startedAt)
		s.LastSeenAt = time.Unix(0, lastSeenAt)
		if headStepID.Valid {
			s.HeadStepID = store.Hash(headStepID.String)
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

// SessionInfo holds displayable session info
type SessionInfo struct {
	ID                string
	Origin            string
	StartedAt         time.Time
	LastSeenAt        time.Time
	HeadStepID        store.Hash
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
