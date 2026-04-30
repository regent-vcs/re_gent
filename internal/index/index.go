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
		db.Close()
		return nil, fmt.Errorf("set journal mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous = NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set synchronous: %w", err)
	}

	// Create schema
	if err := createSchema(db); err != nil {
		db.Close()
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
		step_id    TEXT NOT NULL,
		path       TEXT NOT NULL,
		blob_hash  TEXT NOT NULL,
		blame_hash TEXT,
		PRIMARY KEY (step_id, path)
	);
	CREATE INDEX IF NOT EXISTS idx_step_files_path ON step_files(path);

	CREATE TABLE IF NOT EXISTS sessions (
		id            TEXT PRIMARY KEY,
		origin        TEXT NOT NULL,
		started_at    INTEGER NOT NULL,
		last_seen_at  INTEGER NOT NULL,
		head_step_id  TEXT
	);
	`

	_, err := db.Exec(schema)
	return err
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
			INSERT OR REPLACE INTO step_files (step_id, path, blob_hash, blame_hash)
			VALUES (?, ?, ?, ?)
		`, stepHash, entry.Path, entry.Blob, entry.Blame)
		if err != nil {
			return fmt.Errorf("insert step file: %w", err)
		}
	}

	// Update session record
	now := time.Now().UnixNano()
	_, err = tx.Exec(`
		INSERT INTO sessions (id, origin, started_at, last_seen_at, head_step_id)
		VALUES (?, 'claude_code', ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			last_seen_at = ?,
			head_step_id = ?
	`, step.SessionID, now, now, stepHash, now, stepHash)
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
	Hash       store.Hash
	ParentHash store.Hash
	SessionID  string
	Timestamp  time.Time
	ToolName   string
	ToolUseID  string
}

// ListSteps returns recent steps for a session (newest first)
func (idx *DB) ListSteps(sessionID string, limit int) ([]StepInfo, error) {
	query := `
		SELECT id, parent_id, session_id, ts_nanos, tool_name, tool_use_id
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
		var tsNanos int64

		err := rows.Scan(&s.Hash, &parentHash, &s.SessionID, &tsNanos, &s.ToolName, &s.ToolUseID)
		if err != nil {
			return nil, err
		}

		if parentHash.Valid {
			s.ParentHash = store.Hash(parentHash.String)
		}
		s.Timestamp = time.Unix(0, tsNanos)

		steps = append(steps, s)
	}

	return steps, rows.Err()
}

// ListAllSessions returns all sessions
func (idx *DB) ListAllSessions() ([]SessionInfo, error) {
	rows, err := idx.db.Query(`
		SELECT id, origin, started_at, last_seen_at, head_step_id
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

		err := rows.Scan(&s.ID, &s.Origin, &startedAt, &lastSeenAt, &headStepID)
		if err != nil {
			return nil, err
		}

		s.StartedAt = time.Unix(0, startedAt)
		s.LastSeenAt = time.Unix(0, lastSeenAt)
		if headStepID.Valid {
			s.HeadStepID = store.Hash(headStepID.String)
		}

		sessions = append(sessions, s)
	}

	return sessions, rows.Err()
}

// SessionInfo holds displayable session info
type SessionInfo struct {
	ID         string
	Origin     string
	StartedAt  time.Time
	LastSeenAt time.Time
	HeadStepID store.Hash
}
