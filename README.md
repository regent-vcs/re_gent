<div align="center">
  <a href="https://github.com/regent-vcs/regent">
    <img
      src="assets/regent-logo-dark.png"
      alt="Regent"
      width="100%"
    />
  </a>
  <br />
  <br />
  <h1>Git for AI Agents</h1>
  <p>
    Version control for AI agent activity. Track what your agent did, which prompt wrote each line, and rewind when things break.
  </p>

[![Star on GitHub](https://img.shields.io/github/stars/regent-vcs/regent?style=for-the-badge&logo=github&color=gold)](https://github.com/regent-vcs/regent)
[![Apache 2.0 License](https://img.shields.io/badge/License-Apache%202.0-blue?style=for-the-badge)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/regent-vcs/regent?style=for-the-badge&logo=go&logoColor=white&color=00ADD8)](go.mod)

[![CI Status](https://img.shields.io/github/actions/workflow/status/regent-vcs/regent/ci.yml?style=for-the-badge&logo=githubactions&logoColor=white)](https://github.com/regent-vcs/regent/actions/workflows/ci.yml)
[![Contributions Welcome](https://img.shields.io/badge/Contributions-Welcome-10b981?style=for-the-badge&logo=github)](CONTRIBUTING.md)
[![Claude Code Compatible](https://img.shields.io/badge/Claude%20Code-Compatible-6366f1?style=for-the-badge&logo=anthropic&logoColor=white)](https://github.com/regent-vcs/regent)

</div>

---

## Demo

<div align="center">
  <img src="assets/demo.gif" alt="Regent tracking Claude Code activity" width="100%"/>
  <p><em>Every tool call is automatically captured. No manual commits needed.</em></p>
</div>

---

## Quick Start

```bash
# Install via Homebrew (macOS/Linux)
brew tap regent-vcs/tap
brew install --HEAD regent

# Or via Go
go install github.com/regent-vcs/regent/cmd/rgt@latest

# Initialize in your project
cd your-project
rgt init

# Work with Claude Code normally (every tool call is tracked)

# See what happened
rgt log
```

That's it. Your agent activity is now auditable.

---

## What You Get

### See what your agent actually did

```bash
$ rgt log

Step a1b2c3d  |  2 min ago  |  Tool: Edit
│ File: src/handler.go
│ Added error handling to request handler
│ + 5 lines, - 2 lines

Step d4e5f6g  |  5 min ago  |  Tool: Write
│ File: tests/handler_test.go
│ Created unit tests for handler
│ + 23 lines

Step f8g9h0i  |  8 min ago  |  Tool: Bash
│ Command: go mod tidy
│ Cleaned up dependencies
```

### Blame: which prompt wrote this line?

```bash
$ rgt blame src/handler.go:42

Line 42: func handleRequest(w http.ResponseWriter, r *http.Request) {

Step:    a1b2c3d4e5f6
Session: claude-20260502-143021
Tool:    Edit
Prompt:  "Add error handling to the request handler"
```

### Track multiple concurrent sessions

```bash
$ rgt sessions

Active Sessions:
claude-20260502-143021  |  3 steps  |  Last: 2 min ago
claude-20260502-091534  |  7 steps  |  Last: 2 hours ago

$ rgt log --session claude-20260502-143021
# Filter history by session
```

### See full context for any change

```bash
$ rgt show a1b2c3d

Step a1b2c3d4e5f6
Parent: d4e5f6g7h8i9
Session: claude-20260502-143021
Time: 2026-05-02 14:30:21

Tool: Edit
File: src/handler.go

Changes:
+ func handleRequest(w http.ResponseWriter, r *http.Request) {
+     if r.Method != "GET" {
+         http.Error(w, "Method not allowed", 405)
+         return
+     }
- func handleRequest(w http.ResponseWriter, r *http.Request) {

Conversation:
User: "Add error handling to reject non-GET requests"
Assistant: "I'll add method validation to the handler..."
```

---

## Why This Exists

**The problem:** AI agents have no version control of their own.

You know this pain:
- *"It was working five minutes ago"*
- *"Why did you change that file?"*
- *"Go back to before the refactor"*
- `/compact` and pray
- Copy-pasting code into a fresh chat

**The solution:** Three primitives that should already exist:

- **`rgt log`** — what did this session do?
- **`rgt blame`** — which prompt wrote this line?
- **`rgt rewind`** — restore to any previous step (coming soon)

We gave agents write access to our codebases. We did not give ourselves git for it. Regent fixes that.

---

## How It Works

Regent stores agent activity in `.regent/` (like `.git/`):

```
.regent/
├── objects/     # Content-addressed blobs (BLAKE3)
├── refs/        # Session pointers (one per agent)
├── index.db     # SQLite query index
└── config.toml
```

Every tool call creates a **Step**:

```go
Step {
  parent:      <previous-step-hash>
  tree:        <workspace-snapshot>
  transcript:  <conversation-delta>
  cause: {
    tool_name: "Edit"
    args:      <what-changed>
    result:    <tool-output>
  }
  session_id:  "claude-20260502-143021"
  timestamp:   "2026-05-02T14:30:21Z"
}
```

Steps form a **DAG**. Each session has its own branch. Common ancestors dedupe. You get git-level auditability for agent activity.

**Technical details:** See [POC.md](POC.md) for the complete specification.

---

## Installation

### Via Homebrew (macOS/Linux)

```bash
brew tap regent-vcs/tap
brew install --HEAD regent  # HEAD install until first stable release
```

> **Note:** Currently requires `--HEAD` flag as we're in active development. Stable bottle releases coming soon!

### Via Go Install

```bash
go install github.com/regent-vcs/regent/cmd/rgt@latest
```

### From Source

```bash
git clone https://github.com/regent-vcs/regent
cd regent
go build -o rgt ./cmd/rgt
sudo mv rgt /usr/local/bin/
```

### Binary Releases

Download pre-built binaries from [GitHub Releases](https://github.com/regent-vcs/regent/releases)

---

## Commands

**Available Now:**

| Command | Description |
|---------|-------------|
| `rgt init` | Initialize `.regent/` in current directory |
| `rgt log` | Show step history (supports `--session`, `-n`, `--since`) |
| `rgt sessions` | List all active sessions |
| `rgt status` | Show current repository state |
| `rgt show <step>` | Display full context for a step |
| `rgt cat <hash>` | Inspect any object by hash (debug) |

**Coming Soon:**

| Command | Status | Description |
|---------|--------|-------------|
| `rgt blame <path>:<line>` | Phase 3 | Per-line provenance with prompt |
| `rgt rewind <step>` | Phase 5 | Non-destructive time-travel |
| `rgt gc` | Phase 6 | Garbage collection |

---

## Features

- **Content-Addressed Storage** — BLAKE3 hashing, automatic deduplication
- **Fast Queries** — SQLite index, sub-10ms lookups
- **Per-Session DAG** — Concurrent agents, no conflicts
- **Conversation Tracking** — Survives `/compact` and `/clear`
- **Hook-Driven** — Transparent Claude Code integration
- **Concurrency-Safe** — CAS refs, ACID transactions
- **Gitignore-Compatible** — `.regentignore` support

---

## Regent vs Git

| | Git | Regent |
|---|---|---|
| **Tracks code** | ✅ | ✅ |
| **Tracks agent activity** | ❌ | ✅ |
| **Blame with prompt** | ❌ | ✅ |
| **Conversation history** | ❌ | ✅ |
| **Concurrent sessions** | ⚠️ conflicts | ✅ separate branches |
| **Purpose** | Developer VCS | Agent audit trail |

**Regent complements git, doesn't replace it.** Use both.

---

## Roadmap

- **Phase 1:** Object store (blob, tree, step, ref) — **COMPLETE**
- **Phase 2:** Hook integration (Claude Code) — **COMPLETE**
- **Phase 3:** Blame algorithm (Myers diff) — **IN PROGRESS**
- **Phase 4:** Transcript capture (JSONL) — **PLANNED**
- **Phase 5:** Rewind (time-travel) — **PLANNED**
- **Phase 6:** Concurrency hardening — **PLANNED**

Check [GitHub Projects](https://github.com/regent-vcs/regent/projects) for current priorities.

---

## Status

**Active Development (POC Stage)**

- ~4.5k LOC Go implementation
- Core functionality works (Phases 1-2 complete)
- Used in production by contributors
- Not yet v1.0 (see roadmap)

**Honest assessment:** Production-quality code at POC-level feature completeness. We're building in public.

---

## Contributing

Contributions are welcome! Regent is built in public and we actively review PRs.

**Quick Start:**
- Read [QUICK_START.md](.github/QUICK_START.md) for a 5-minute setup guide
- Check [good first issues](https://github.com/regent-vcs/regent/labels/good%20first%20issue)
- Full guidelines: [CONTRIBUTING.md](.github/CONTRIBUTING.md)

**Before opening a PR:**
- [ ] Tests pass: `go test ./...` and `go test -race ./...`
- [ ] Linter passes: `golangci-lint run`
- [ ] Code formatted: `go fmt ./...`
- [ ] PR template filled out

**Important files:**
- [CONTRIBUTING.md](.github/CONTRIBUTING.md) — Full contribution guide
- [SECURITY.md](SECURITY.md) — Security policy and reporting
- [CLAUDE.md](CLAUDE.md) — Project context and architecture
- [POC.md](POC.md) — Technical specification

---

## Built With

- [cobra](https://github.com/spf13/cobra) — CLI framework
- [blake3](https://lukechampine.com/blake3) — BLAKE3 hashing
- [go-diff](https://github.com/sergi/go-diff) — Myers diff
- [modernc.org/sqlite](https://modernc.org/sqlite) — Pure Go SQLite

---

## License

[Apache License 2.0](LICENSE)

---

<div align="center">
  <p>
    <sub>Built by <a href="https://github.com/regent-vcs/regent/graphs/contributors">contributors</a></sub>
  </p>
  <p>
    <a href="https://github.com/regent-vcs/regent/discussions">Discussions</a> •
    <a href="https://github.com/regent-vcs/regent/issues">Issues</a> •
    <a href="POC.md">Technical Spec</a>
  </p>
</div>
