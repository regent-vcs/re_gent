# Regent — POC Implementation Plan

> **Regent** is a content-addressed version control system for AI agent activity. It captures what an agent did, why, and lets you blame, log, and rewind across sessions.
>
> **CLI**: `rgt`
> **Language**: Go (1.22+)
> **Status**: POC

---

## 1. Goal of the POC

Prove three things end-to-end on real Claude Code activity:

1. **Storage + blame algorithm.** A content-addressed object store (blobs, trees, steps, blame maps) that handles realistic agent workloads with bounded storage growth and sub-10ms blame queries.
2. **Concurrent sessions on a shared workspace.** Two or more Claude Code sessions writing to the same directory produce correct, non-conflicting per-session DAG branches without losing data or corrupting refs.
3. **Conversation state staging.** The originating prompt and assistant turn for each step are lifted from Claude Code's JSONL into Regent's own object store, so `/compact` and `/clear` can't destroy historical lineage.

Out of scope for v0: branching/checkout, worktrees, conflict resolution UI, multi-tool adapters (Cursor, Cline, etc.), remote sync, performance tuning beyond "doesn't fall over."

---

## 2. Conceptual recap

The model has four primitive object types (the first three immutable and content-addressed):

- **Blob** — raw bytes (file content, message body, JSON payload). Identified by `blake3(content)`.
- **Tree** — `{ path → blob_hash, blame_hash }` map describing the workspace at one moment. Itself a blob.
- **Step** — the equivalent of a git commit: `{ parent, tree, transcript, cause, session_id, ... }`. Itself a blob.
- **Ref** — the only mutable object: a named pointer (`session_A`, `HEAD`) holding a step hash.

Each Step has a `parent` pointing to the previous step *in the same session*. The DAG emerges from those pointers. Sessions live as their own refs (`refs/sessions/<session_id>`) so concurrent sessions branch naturally from a common ancestor without colliding.

Workspace on disk is **shared across sessions** by default. The DAG records what each session *thought was happening*; conflict detection happens on a same-file-touched basis at write time. Worktrees are an opt-in escape hatch (post-v0).

---

## 3. Project structure

Idiomatic Go layout:

```
regent/
├── cmd/
│   └── rgt/
│       └── main.go             # CLI entry point (cobra)
├── internal/
│   ├── store/
│   │   ├── blob.go             # blob read/write + atomic file ops
│   │   ├── tree.go             # tree marshal/unmarshal
│   │   ├── step.go             # step marshal/unmarshal
│   │   ├── transcript.go       # transcript chain object
│   │   ├── blame.go            # per-line provenance map
│   │   ├── refs.go             # ref CAS with lock files
│   │   └── store.go            # Store struct, init/open
│   ├── snapshot/
│   │   └── snapshot.go         # walk workspace, build tree
│   ├── diff/
│   │   └── diff.go             # line diff for blame computation
│   ├── jsonl/
│   │   └── jsonl.go            # Claude Code transcript reader
│   ├── hook/
│   │   ├── hook.go             # PostToolUse entry point
│   │   └── payload.go          # hook payload schema
│   ├── index/
│   │   └── index.go            # SQLite metadata index
│   ├── ignore/
│   │   └── ignore.go           # .regentignore parser
│   └── cli/
│       ├── init.go
│       ├── log.go
│       ├── blame.go
│       ├── rewind.go
│       ├── status.go
│       └── cat.go              # debug: dump any object by hash
├── go.mod
├── go.sum
├── README.md
├── CLAUDE.md
└── POC.md (this file)
```

---

## 4. Storage layout on disk

Inside the user's project root:

```
<project>/
├── .regent/
│   ├── objects/
│   │   ├── ab/
│   │   │   └── abcdef0123...    # content-addressed blobs (sharded by first 2 hex chars)
│   │   └── ...
│   ├── refs/
│   │   └── sessions/
│   │       ├── <session_id>     # plain text file containing one hash + newline
│   │       └── ...
│   ├── index.db                 # SQLite — derived index, rebuildable from objects
│   ├── config.toml              # per-project config
│   └── HEAD                     # current default session pointer (optional)
└── .regentignore                # patterns to skip when snapshotting (top-level)
```

**Why both `objects/` and `index.db`:** the object store is the source of truth (immutable, simple, content-addressed). SQLite is a derived index for fast queries (`log` filtered by session, `blame` lookups by path). If the index is ever corrupted or out of sync, `rgt reindex` rebuilds it from `objects/`.

**Default `.regentignore`:**

```
node_modules/
.git/
.regent/
__pycache__/
*.pyc
.venv/
venv/
target/
dist/
build/
.next/
.cache/
```

---

## 5. Data model

All structs live in `internal/store/`. JSON is the wire format for v0 (human-debuggable, simple). Switch to a binary format (CBOR, MessagePack, or custom) only if profiling demands it.

```go
package store

type Hash string

// Blob: just bytes. No struct — handled directly in blob.go.

type TreeEntry struct {
    Path  string `json:"path"`
    Blob  Hash   `json:"blob"`
    Blame Hash   `json:"blame,omitempty"` // per-line provenance for this file
    Mode  uint32 `json:"mode,omitempty"`  // unix mode (executable bit matters)
}

type Tree struct {
    Entries []TreeEntry `json:"entries"` // sorted by Path for deterministic hashing
}

type Cause struct {
    ToolUseID  string `json:"tool_use_id"`
    ToolName   string `json:"tool_name"`
    ArgsBlob   Hash   `json:"args_blob,omitempty"`
    ResultBlob Hash   `json:"result_blob,omitempty"`
}

type Effect struct {
    Kind       string `json:"kind"`        // "http_call", "db_write", "process_exec", ...
    Descriptor string `json:"descriptor"`  // human-readable summary
}

type Step struct {
    Parent          Hash     `json:"parent,omitempty"`
    SecondaryParent Hash     `json:"secondary_parent,omitempty"` // for sub-agent merges
    Tree            Hash     `json:"tree"`
    Transcript      Hash     `json:"transcript,omitempty"`
    Config          Hash     `json:"config,omitempty"` // system prompt + tools + memory hash
    Cause           Cause    `json:"cause"`
    SessionID       string   `json:"session_id"`
    AgentID         string   `json:"agent_id,omitempty"`
    TimestampNanos  int64    `json:"ts"`
    Effects         []Effect `json:"effects,omitempty"`
}

// Transcript: a chain object. To reconstruct the full conversation at a step,
// walk Prev backward, collecting NewMessages, then reverse and dereference.
type Transcript struct {
    Prev         Hash   `json:"prev,omitempty"`
    NewMessages  []Hash `json:"new_messages"` // each is a blob hash of a canonical JSON message
}

// BlameMap: per-line provenance for a file at one tree.
// Index i corresponds to line i+1 of the file at this step.
type BlameMap struct {
    Lines []Hash `json:"lines"` // each entry = step hash that introduced or last modified this line
}
```

**Hashing**: BLAKE3, 256-bit output, hex-encoded. Use `lukechampine.com/blake3` (pure Go, fast, no CGO).

**JSON canonicalization**: marshal with sorted keys (Go's `encoding/json` already sorts struct fields by declaration order; we sort `Tree.Entries` by `Path` ourselves before marshaling).

---

## 6. SQLite index schema

The index is purely derived. It exists to make `log`, `blame`, and "files touched in session X" fast without scanning the object store.

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS steps (
    id          TEXT PRIMARY KEY,        -- step hash
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
    origin        TEXT NOT NULL,        -- "claude_code" for now
    started_at    INTEGER NOT NULL,
    last_seen_at  INTEGER NOT NULL,
    head_step_id  TEXT
);
```

Use `modernc.org/sqlite` (pure Go, no CGO required for distribution).

---

## 7. Algorithms

### 7.1 Snapshot (workspace → tree)

```go
// internal/snapshot/snapshot.go

func Snapshot(s *store.Store, root string, ig *ignore.Matcher) (store.Hash, error) {
    var entries []store.TreeEntry

    err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        rel, _ := filepath.Rel(root, p)
        if rel == "." {
            return nil
        }
        if ig.Match(rel, d.IsDir()) {
            if d.IsDir() {
                return fs.SkipDir
            }
            return nil
        }
        if d.IsDir() {
            return nil
        }
        info, err := d.Info()
        if err != nil {
            return err
        }
        // skip symlinks for v0 (simplifies semantics)
        if info.Mode()&os.ModeSymlink != 0 {
            return nil
        }
        content, err := os.ReadFile(p)
        if err != nil {
            return err
        }
        h, err := s.WriteBlob(content)
        if err != nil {
            return err
        }
        entries = append(entries, store.TreeEntry{
            Path: filepath.ToSlash(rel),
            Blob: h,
            Mode: uint32(info.Mode().Perm()),
        })
        return nil
    })
    if err != nil {
        return "", err
    }

    tree := &store.Tree{Entries: entries}
    return s.WriteTree(tree)
}
```

**Performance notes for the POC**:
- Skip files larger than 10 MB by default (config-tunable). Agents almost never edit large binaries; this prevents pathological cases.
- The `.regentignore` matcher must support gitignore-compatible patterns. Use `github.com/sabhiram/go-gitignore`.
- Don't optimize incremental snapshots in v0. Re-walk the whole tree each step. Content addressing makes it cheap (only changed blobs cost real I/O).

### 7.2 Blame (annotated-blob algorithm)

The key trick: compute and store the per-line provenance **at write time**, not at query time. Each tree entry references both the file content blob and a parallel `BlameMap` blob.

```go
// internal/store/blame.go

import "github.com/sergi/go-diff/diffmatchpatch"

// ComputeBlame builds a new BlameMap for a file given its previous content+blame
// and its new content. Lines unchanged inherit their old attribution; new or
// modified lines are stamped with currentStep.
func ComputeBlame(oldContent, newContent []byte, oldBlame *BlameMap, currentStep Hash) *BlameMap {
    oldLines := splitLines(oldContent)
    newLines := splitLines(newContent)

    // Use Myers diff via go-diff or implement directly.
    // For v0 use the simpler difflib-style opcodes from go-diff.
    ops := lineDiffOpcodes(oldLines, newLines)

    newBlame := &BlameMap{Lines: make([]Hash, 0, len(newLines))}
    for _, op := range ops {
        switch op.Tag {
        case "equal":
            // preserved lines keep their attribution
            for k := op.I1; k < op.I2; k++ {
                if oldBlame != nil && k < len(oldBlame.Lines) {
                    newBlame.Lines = append(newBlame.Lines, oldBlame.Lines[k])
                } else {
                    newBlame.Lines = append(newBlame.Lines, currentStep)
                }
            }
        case "insert", "replace":
            // new or modified lines belong to the current step
            for k := op.J1; k < op.J2; k++ {
                _ = k
                newBlame.Lines = append(newBlame.Lines, currentStep)
            }
        case "delete":
            // deleted lines contribute nothing to the new blame
        }
    }
    return newBlame
}

type opcode struct {
    Tag                string
    I1, I2, J1, J2     int
}

func lineDiffOpcodes(a, b []string) []opcode {
    // Implement using Myers via diffmatchpatch:
    // 1. Encode each line as a single rune (line-mode trick)
    // 2. Run dmp.DiffMain on encoded strings
    // 3. Convert diff results to opcode list
    // See: https://github.com/sergi/go-diff#line-mode
    // ... (omitted for brevity — ~40 lines)
}
```

**Storage cost analysis** (worth measuring during POC): for a 1000-line file edited once (10 lines changed), the new BlameMap is ~32 KB (1000 × 32-byte hex hashes). With dedupe across runs (most lines share the same hash), we can later optimize by run-length-encoding the BlameMap or using shorter step IDs. For v0, accept the ~32 KB-per-touch overhead and see if it actually hurts.

**Blame query**:

```go
// CLI: rgt blame <path>[:<line>]
func Blame(s *store.Store, idx *index.DB, sessionID, path string, line int) ([]BlameAnswer, error) {
    // 1. Find the latest step in this session that touched `path`.
    headStepID, err := idx.SessionHead(sessionID)
    if err != nil {
        return nil, err
    }
    headStep, err := s.ReadStep(headStepID)
    if err != nil {
        return nil, err
    }
    headTree, err := s.ReadTree(headStep.Tree)
    if err != nil {
        return nil, err
    }
    var entry *store.TreeEntry
    for i := range headTree.Entries {
        if headTree.Entries[i].Path == path {
            entry = &headTree.Entries[i]
            break
        }
    }
    if entry == nil {
        return nil, fmt.Errorf("file not in head tree: %s", path)
    }
    if entry.Blame == "" {
        return nil, fmt.Errorf("no blame map for %s (file added before blame tracking)", path)
    }

    blame, err := s.ReadBlame(entry.Blame)
    if err != nil {
        return nil, err
    }

    // 2. For requested line(s), look up the step hash and resolve to cause.
    if line > 0 {
        if line > len(blame.Lines) {
            return nil, fmt.Errorf("line out of range")
        }
        return resolveOne(s, blame.Lines[line-1], line), nil
    }
    // whole-file blame
    return resolveAll(s, blame.Lines), nil
}

type BlameAnswer struct {
    Line       int
    StepID     store.Hash
    ToolName   string
    ToolUseID  string
    Timestamp  int64
    SessionID  string
}
```

### 7.3 Rewind

For v0, rewind is **per-session**, **scoped to files this session touched**, and **conversation-only by default** (with `--files` to also restore disk).

```go
// CLI: rgt rewind <step> [--files] [--conversation-only]
func Rewind(s *store.Store, idx *index.DB, sessionID string, target store.Hash, restoreFiles bool) error {
    // 1. Verify target is an ancestor of the current session head (no time travel forward).
    head, err := idx.SessionHead(sessionID)
    if err != nil {
        return err
    }
    if !idx.IsAncestor(target, head) {
        return fmt.Errorf("target step %s is not an ancestor of session head %s", target, head)
    }

    // 2. Compute the set of files this session touched between target and head.
    touched, err := idx.FilesTouchedBetween(sessionID, target, head)
    if err != nil {
        return err
    }

    if restoreFiles {
        targetStep, err := s.ReadStep(target)
        if err != nil {
            return err
        }
        targetTree, err := s.ReadTree(targetStep.Tree)
        if err != nil {
            return err
        }
        if err := restoreFilesFromTree(s, targetTree, touched); err != nil {
            return err
        }
    }

    // 3. Move the session ref. Use CAS to detect concurrent updates.
    if err := s.UpdateRef("sessions/"+sessionID, head, target); err != nil {
        return err
    }

    // 4. Tell the user what happened (and how to restart their agent if conversation
    //    needs to be restored — see section 9 on the agent-resume problem).
    fmt.Printf("Rewound session %s from %s to %s\n", sessionID, head[:8], target[:8])
    if restoreFiles {
        fmt.Printf("Restored %d file(s) to their state at target.\n", len(touched))
    }
    return nil
}

func restoreFilesFromTree(s *store.Store, tree *store.Tree, scope map[string]bool) error {
    byPath := make(map[string]store.Hash, len(tree.Entries))
    for _, e := range tree.Entries {
        byPath[e.Path] = e.Blob
    }
    for path := range scope {
        h, exists := byPath[path]
        if !exists {
            // file did not exist at target — delete from disk if present
            _ = os.Remove(path)
            continue
        }
        content, err := s.ReadBlob(h)
        if err != nil {
            return err
        }
        if err := atomicWriteFile(path, content); err != nil {
            return err
        }
    }
    return nil
}
```

**Conflict case**: if a file in `touched` was *also* modified by another session between `target` and now, surface a conflict and let the user decide (`--force` to overwrite, `--skip <path>` to leave it alone). The POC can ship with conflict detection but require explicit `--force` to proceed; full resolution UI is post-v0.

### 7.4 Transcript chain

Each step references a `Transcript` blob. The transcript is a linked list: each node holds the new messages added since the previous step + a hash pointer to the previous transcript node.

```go
// internal/store/transcript.go

func WriteTranscript(s *Store, prev Hash, newMessages []Hash) (Hash, error) {
    t := &Transcript{Prev: prev, NewMessages: newMessages}
    data, err := json.Marshal(t)
    if err != nil {
        return "", err
    }
    return s.WriteBlob(data)
}

// ReconstructTranscript walks the chain and returns all messages in order.
func ReconstructTranscript(s *Store, head Hash) ([]json.RawMessage, error) {
    var batches [][]Hash
    cur := head
    for cur != "" {
        data, err := s.ReadBlob(cur)
        if err != nil {
            return nil, err
        }
        var t Transcript
        if err := json.Unmarshal(data, &t); err != nil {
            return nil, err
        }
        batches = append(batches, t.NewMessages)
        cur = t.Prev
    }
    // batches are in reverse-chronological order; flatten back
    var msgs []json.RawMessage
    for i := len(batches) - 1; i >= 0; i-- {
        for _, h := range batches[i] {
            data, err := s.ReadBlob(h)
            if err != nil {
                return nil, err
            }
            msgs = append(msgs, json.RawMessage(data))
        }
    }
    return msgs, nil
}
```

### 7.5 Concurrency-safe ref CAS

The blob/tree/step writes are inherently safe (write-once, content-addressed; identical content from two writers produces identical bytes — last-rename-wins is correct). The contested resource is **refs**. Use a lock-file CAS pattern, same as git:

```go
// internal/store/refs.go

var ErrRefConflict = errors.New("ref was modified concurrently; retry")

func (s *Store) UpdateRef(name string, expectedOld Hash, newHash Hash) error {
    refPath := filepath.Join(s.Root, "refs", name)
    if err := os.MkdirAll(filepath.Dir(refPath), 0o755); err != nil {
        return err
    }
    lockPath := refPath + ".lock"

    // O_CREATE|O_EXCL is atomic across POSIX filesystems.
    fd, err := syscall.Open(lockPath, syscall.O_CREAT|syscall.O_EXCL|syscall.O_WRONLY, 0o644)
    if err != nil {
        if errors.Is(err, syscall.EEXIST) {
            return ErrRefConflict
        }
        return err
    }
    defer func() {
        syscall.Close(fd)
        os.Remove(lockPath)
    }()

    current, err := s.ReadRef(name)
    if err != nil && !errors.Is(err, fs.ErrNotExist) {
        return err
    }
    if current != expectedOld {
        return ErrRefConflict
    }

    return atomicWriteFile(refPath, []byte(string(newHash)+"\n"))
}

// Caller responsibility: retry on ErrRefConflict with backoff.
func (s *Store) UpdateRefWithRetry(name string, expectedOld, newHash Hash, maxAttempts int) error {
    backoff := 5 * time.Millisecond
    for i := 0; i < maxAttempts; i++ {
        err := s.UpdateRef(name, expectedOld, newHash)
        if err == nil {
            return nil
        }
        if !errors.Is(err, ErrRefConflict) {
            return err
        }
        time.Sleep(backoff + time.Duration(rand.Int63n(int64(backoff))))
        backoff *= 2
        if backoff > 100*time.Millisecond {
            backoff = 100 * time.Millisecond
        }
    }
    return ErrRefConflict
}
```

For SQLite, rely on built-in WAL-mode concurrency. Wrap the step insert + step_files insert in a single transaction so the index never sees a partial step.

---

## 8. Hook integration (Claude Code)

Wire `rgt hook` as a `PostToolUse` hook in Claude Code's settings. The hook receives a JSON payload over stdin (per Claude Code's hook protocol), processes it, and exits.

**Hook entry point**:

```go
// internal/hook/hook.go

type Payload struct {
    SessionID    string          `json:"session_id"`
    ToolUseID    string          `json:"tool_use_id"`
    ToolName     string          `json:"tool_name"`
    ToolInput    json.RawMessage `json:"tool_input"`
    ToolResponse json.RawMessage `json:"tool_response"`
    CWD          string          `json:"cwd"`
    TranscriptPath string        `json:"transcript_path"` // path to the JSONL
}

func Run(stdin io.Reader, stdout io.Writer) error {
    var p Payload
    if err := json.NewDecoder(stdin).Decode(&p); err != nil {
        return fmt.Errorf("decode payload: %w", err)
    }

    s, err := store.Open(filepath.Join(p.CWD, ".regent"))
    if err != nil {
        // Repo not initialized in this project — silently no-op.
        // Don't fail the agent.
        return nil
    }
    idx, err := index.Open(s)
    if err != nil {
        return err
    }

    // 1. Snapshot the workspace.
    treeHash, err := snapshot.Snapshot(s, p.CWD, ignore.Default(p.CWD))
    if err != nil {
        return fmt.Errorf("snapshot: %w", err)
    }

    // 2. Compute blame for files that changed since the parent step.
    parentHash, _ := s.ReadRef("sessions/" + p.SessionID)
    treeHash, err = computeBlameForChanges(s, parentHash, treeHash, currentStepIDPlaceholder)
    if err != nil {
        return err
    }
    // (The blame computation needs the current step's hash, but the step's hash
    //  depends on the tree's hash. Two-pass: write tree without blame, build step,
    //  then update tree entries with blame maps stamped with the step ID. Or:
    //  use a deterministic placeholder ID = hash of (tree + cause + parent) so
    //  blame can be computed before the step exists. POC: two-pass for clarity.)

    // 3. Stage the conversation slice.
    transcriptHash, err := stageConversation(s, idx, p)
    if err != nil {
        return fmt.Errorf("stage conversation: %w", err)
    }

    // 4. Build and write the step.
    argsHash, _ := s.WriteBlob(p.ToolInput)
    resultHash, _ := s.WriteBlob(p.ToolResponse)
    step := &store.Step{
        Parent:         parentHash,
        Tree:           treeHash,
        Transcript:     transcriptHash,
        Cause:          store.Cause{
            ToolUseID:  p.ToolUseID,
            ToolName:   p.ToolName,
            ArgsBlob:   argsHash,
            ResultBlob: resultHash,
        },
        SessionID:      p.SessionID,
        TimestampNanos: time.Now().UnixNano(),
    }
    stepHash, err := s.WriteStep(step)
    if err != nil {
        return err
    }

    // 5. CAS the session ref forward.
    if err := s.UpdateRefWithRetry("sessions/"+p.SessionID, parentHash, stepHash, 8); err != nil {
        return err
    }

    // 6. Update the SQLite index.
    if err := idx.IndexStep(stepHash, step, treeChangedFiles); err != nil {
        return err
    }

    return nil
}
```

**Key choices**:
- **Fail silently if `.regent/` doesn't exist.** The hook fires for every tool call in every project; if Regent isn't initialized, do nothing. Never break the user's agent.
- **Don't emit anything to stdout** unless required by Claude Code's hook protocol. Log errors to a file under `.regent/log/` for debugging.
- **Two-pass step write** for blame: the blame map references the step that wrote it, but the step's hash depends on the tree (which contains blame map hashes). Either (a) compute step ID without blame, write everything, or (b) use `(parent + cause + workspace_tree_no_blame)` as the step's stable ID and compute blame referencing that ID. The POC will use (a) — simpler.

**Conversation staging algorithm**:

```go
func stageConversation(s *store.Store, idx *index.DB, p Payload) (store.Hash, error) {
    // Find the last message hash we processed for this session.
    lastMsgID, prevTranscript, err := idx.SessionLastProcessedMessage(p.SessionID)
    if err != nil {
        return "", err
    }

    // Read the JSONL and extract everything between (lastMsgID, p.ToolUseID].
    newMsgs, err := jsonl.ExtractRange(p.TranscriptPath, lastMsgID, p.ToolUseID)
    if err != nil {
        return "", err
    }

    var msgHashes []store.Hash
    for _, m := range newMsgs {
        canonical, _ := canonicalJSON(m)
        h, err := s.WriteBlob(canonical)
        if err != nil {
            return "", err
        }
        msgHashes = append(msgHashes, h)
    }

    transcriptHash, err := store.WriteTranscript(s, prevTranscript, msgHashes)
    if err != nil {
        return "", err
    }

    // Update the index so the next hook knows where to resume.
    lastIngestedID := messageIDOf(newMsgs[len(newMsgs)-1])
    if err := idx.UpdateSessionLastProcessed(p.SessionID, lastIngestedID, transcriptHash); err != nil {
        return "", err
    }
    return transcriptHash, nil
}
```

**Edge cases for transcript staging**:
- **First hook for a session**: `lastMsgID` is empty. Capture all messages from the start of the JSONL up to the current `tool_use_id`. This becomes the baseline.
- **`/compact` happened**: messages we previously ingested may no longer be in the JSONL. Detect by: if `lastMsgID` is provided but not found in the JSONL, log a warning and snapshot from the beginning of whatever's in the JSONL now. Our previously-stored blobs are still intact in `objects/` — only the live JSONL changed.
- **`/clear` happened**: same as compact, except more aggressive. Same fallback.

---

## 9. CLI commands (v0)

Built with `github.com/spf13/cobra`.

| Command | Purpose |
|---|---|
| `rgt init` | Create `.regent/` in the current directory; install hook config snippet for Claude Code |
| `rgt hook` | Internal: invoked by Claude Code's PostToolUse hook (reads payload from stdin) |
| `rgt status` | Show current session ID, head step, files dirty since last step |
| `rgt log [--session ID] [-n N]` | Show steps in reverse-chronological order with tool name and short cause |
| `rgt show <step>` | Dump full step record + the cause's tool args/result |
| `rgt blame <path>[:<line>]` | Per-line provenance |
| `rgt rewind <step> [--files] [--force]` | Move session ref back; optionally restore files |
| `rgt cat <hash>` | Debug: dump any object's contents (auto-detects type) |
| `rgt reindex` | Rebuild SQLite index from the object store |
| `rgt sessions` | List sessions, with last-seen timestamp and head |

**A note on the agent-resume problem** (deferred from v0): when you rewind, the *files* go back, but the live Claude Code agent's transcript doesn't. The agent will see "current" files but think it's at the latest step. v0 prints a warning telling the user to start a fresh session if they want a clean resume. v1 will explore programmatic session reset (writing the rewound transcript back to the JSONL, then asking the user to `/clear` and resume — or whatever Claude Code exposes by then).

---

## 10. Testing strategy

Three test harnesses, one per POC pillar.

### 10.1 Synthetic workload (storage + blame)

```go
// test/storage_workload_test.go

// Generates N synthetic tool calls against a real repo and measures:
//   - total bytes written to .regent/objects/
//   - per-step write latency (p50, p99)
//   - blame query latency for 100 random (file, line) pairs
//   - dedupe ratio
//
// Workload: clone a real medium-sized repo, run 1000 synthetic Edits
// (random small changes), assert storage growth is bounded.
```

**Pass criteria**:
- 1000 single-line edits against a 1000-file repo: total `.regent/` size < 5 × repo size.
- p99 step write < 100ms (snapshot + tree + step + index).
- p99 blame query < 10ms.

### 10.2 JSONL replay (conversation staging)

```go
// test/jsonl_replay_test.go

// Records a real Claude Code session's JSONL into a fixture.
// Replays it message-by-message, invoking the hook entry point after each
// tool_use, and asserts:
//   - reconstructing the transcript at any step produces messages identical
//     to the JSONL slice up to that point
//   - after simulating /compact (rewriting the JSONL to a summary),
//     historical reconstructions still work
```

### 10.3 Concurrent sessions (race conditions)

```go
// test/concurrency_test.go

// Spawns N goroutines, each invoking the hook with a different session_id
// against the same .regent/. Each goroutine fires M tool calls in a tight loop.
// After completion, asserts:
//   - total step count == N * M (no losses)
//   - each session's ref points to a step whose lineage contains exactly
//     M steps for that session
//   - no orphan refs, no .lock files left over
//   - object store has no duplicate or partial blobs
```

Then a same-session race test: spawn N goroutines all with the same session_id. The CAS retry logic should serialize them into a clean linear chain of N×M steps.

---

## 11. Phased implementation plan

Rough effort estimates assume one focused engineer working in Go.

### Phase 1 — Object store skeleton (3–4 days)

- `internal/store/`: blob, tree, step (no blame yet), refs with CAS.
- `internal/snapshot/`: workspace walker with `.regentignore`.
- `internal/index/`: SQLite schema + step insert/query.
- `cmd/rgt/`: `init`, `cat`, `status` (minimal), `log`.
- **Acceptance**: can manually invoke a snapshot, see a step in the log, dump the tree.

### Phase 2 — Hook integration (2–3 days)

- `internal/hook/`: payload schema + entry point.
- `internal/jsonl/`: reader for Claude Code's JSONL format.
- Wire up `rgt init` to print the hook config snippet for the user to paste.
- **Acceptance**: run a real Claude Code session, see steps appearing in `rgt log` with correct tool names and tool_use_ids.

### Phase 3 — Blame algorithm (3–4 days)

- `internal/diff/`: line diff.
- `internal/store/blame.go`: BlameMap compute + persist.
- Update snapshot/hook flow to compute blame for changed files.
- `cmd/rgt/`: `blame` command.
- **Acceptance**: pick a line of code in a file the agent wrote during the test session, run `rgt blame`, verify it returns the correct step + tool call + originating prompt.

### Phase 4 — Transcript staging (2–3 days)

- `internal/store/transcript.go`: chain object + reconstruct.
- Conversation extraction in the hook.
- `rgt show <step>` to display the step + reconstructed conversation slice.
- **Acceptance**: replay test passes; `/compact` test passes.

### Phase 5 — Rewind (2 days)

- Per-session, scoped-files rewind.
- Conflict detection (refuse without `--force` if other sessions touched the same files).
- **Acceptance**: rewind a session, verify files restore correctly, verify other sessions' refs untouched.

### Phase 6 — Concurrency hardening (2–3 days)

- Run all three test harnesses under `-race`.
- Fix any ref CAS retry edge cases.
- Stress test with 10 concurrent fake sessions.
- **Acceptance**: concurrency test passes 100 runs in a row with no lost steps, no corruption.

**Total**: roughly 14–19 focused days for v0.

---

## 12. Dependencies (go.mod)

```
module github.com/regent-vcs/regent

go 1.22

require (
    github.com/spf13/cobra v1.8.0
    lukechampine.com/blake3 v1.2.1
    github.com/sergi/go-diff v1.3.1
    github.com/sabhiram/go-gitignore v0.0.0-20210923224102-525f6e181f06
    modernc.org/sqlite v1.28.0
    github.com/BurntSushi/toml v1.3.2
)
```

All of these compile without CGO, so `rgt` ships as a single static binary.

---

## 13. Out of scope for v0 (with notes for later)

- **Branch / checkout commands**: the data model already supports them (multiple refs in `refs/sessions/`). UI/CLI work only.
- **Worktrees**: opt-in escape hatch for true parallelism. Requires a `worktrees/` directory under `.regent/` and per-worktree CWD tracking.
- **Conflict resolution UI**: v0 detects conflicts and warns, requires `--force`. v1 needs a 3-way diff or interactive picker.
- **Sub-agent (Task tool) lineage**: the Step schema has `SecondaryParent` for merge points; the hook needs to detect sub-agent fan-out/fan-in and populate it.
- **Adapters for other tools** (Cursor, Cline, Continue, Aider): per-tool adapter packages that produce the same `Payload` from each tool's hook system.
- **Conversation rewind into the live agent**: write rewound transcript back into the JSONL + Claude Code session reset.
- **Garbage collection / packing**: orphaned objects after rewinds pile up. Eventually need a `rgt gc` analog of `git gc`.
- **Remote / sync**: pushing/pulling sessions between machines or devs. The content-addressed model accommodates it natively (git's wire protocol is a good reference).

---

## 14. The first concrete next step

1. `mkdir -p cmd/rgt internal/store internal/snapshot internal/hook internal/index`
2. `go mod init github.com/regent-vcs/regent`
3. Implement `internal/store/blob.go` (write_blob + atomic write + read_blob) — it's ~80 lines and unblocks everything else.
4. Write a test that writes 1000 random blobs and verifies dedupe + integrity.
5. Move on to `tree.go`.

If anything in this doc is ambiguous when you hit it, treat the **algorithms section** (§7) as authoritative and update the rest to match.
