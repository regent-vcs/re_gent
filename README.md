# Regent

[![CI](https://github.com/regent-vcs/regent/workflows/CI/badge.svg)](https://github.com/regent-vcs/regent/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/regent-vcs/regent)](go.mod)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**Version control for AI agent activity.**

Regent is a content-addressed version control system designed specifically for AI agents. It captures what an agent did, why, and lets you blame, log, and rewind across sessions.

## Status: POC Phase 1 Complete ✅

Phase 1 (Object Store Skeleton) is implemented and tested:
- ✅ Content-addressed blob storage with BLAKE3 hashing
- ✅ Tree and step objects with deterministic serialization
- ✅ CAS-based ref updates for concurrency safety
- ✅ Workspace snapshot with `.regentignore` support
- ✅ SQLite derived index for fast queries
- ✅ Basic CLI: `init`, `status`, `log`, `sessions`, `cat`

## Demo

<!-- GIF demo will be added here showing: rgt init → rgt status → rgt log -->
*Coming soon: Screencast of rgt in action*

## Installation

### From Source (Current)

```bash
# Build from source
git clone https://github.com/regent-vcs/regent
cd regent
go build -o rgt ./cmd/rgt

# Or install directly
go install github.com/regent-vcs/regent/cmd/rgt@latest
```

### Coming Soon
- Homebrew: `brew install regent-vcs/tap/rgt`
- Prebuilt binaries: GitHub Releases

## Quick Start

```bash
# Initialize regent in your project
cd your-project
rgt init

# When prompted: Enable automatic tracking in Claude Code? [Y/n]
# Press Y (or just Enter for default yes)

# That's it! Now use Claude Code normally.
# Every tool call is automatically tracked.

# View your session history
rgt log

# List all sessions
rgt sessions

# View status
rgt status

# Inspect a specific step
rgt cat <step-hash>
```

## How It Works

Regent stores agent activity in a `.regent/` directory inside your project:

```
.regent/
├── objects/        # Content-addressed blobs (trees, steps, files)
├── refs/           # Mutable pointers to steps (per-session heads)
│   └── sessions/
├── index.db        # SQLite derived index for fast queries
└── config.toml
```

Every tool call creates a **Step** (like a git commit):
- Parent step hash
- Tree hash (workspace snapshot)
- Cause (tool name, args, result)
- Session ID and timestamp

## Regent vs Git

| Feature | Git | Regent |
|---------|-----|--------|
| Tracks code changes | ✅ | ✅ |
| Tracks agent activity | ❌ | ✅ |
| Per-line blame | ✅ (manual) | ✅ (automatic, with prompt) |
| Rewind/time-travel | ✅ (`reset --hard`) | ✅ (per-session, non-destructive) |
| Conversation history | ❌ | ✅ (content-addressed transcripts) |
| Concurrent sessions | ⚠️ (conflicts) | ✅ (separate branches per session) |
| **Purpose** | Developer version control | AI agent activity tracking |

**Regent complements git, it doesn't replace it.** Use git for your codebase, Regent for agent activity.

## Architecture Highlights

- **Content-addressed storage**: BLAKE3 hashing with automatic deduplication
- **Immutable objects**: Blobs, trees, and steps are write-once
- **Mutable refs**: Per-session pointers using CAS (compare-and-swap) for concurrency
- **SQLite index**: Rebuildable from object store, optimized for queries
- **Per-session DAG**: Each agent session maintains its own branch

## Testing

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run specific test
go test -v ./internal/store -run TestBlob
```

Current test coverage:
- Blob deduplication and integrity
- Tree determinism
- Snapshot with ignore patterns
- End-to-end step creation
- Multi-step chains

## Roadmap

### Phase 2: Hook Integration (Next)
- Claude Code PostToolUse hook
- JSONL transcript reader
- Automatic step capture during agent sessions

### Phase 3: Blame Algorithm
- Per-line provenance tracking
- Myers diff for efficient blame computation
- `rgt blame <path>[:<line>]` command

### Phase 4: Transcript Staging
- Conversation capture from JSONL
- Resilient to `/compact` and `/clear`
- `rgt show <step>` with conversation reconstruction

### Phase 5: Rewind
- Move session ref backward
- Optional file restoration
- Conflict detection

### Phase 6: Concurrency Hardening
- Stress tests (1000+ concurrent steps)
- Performance validation (sub-10ms blame, sub-100ms writes)
- Race condition fixes

See [POC.md](POC.md) for the complete implementation plan.

## Commands

| Command | Description |
|---------|-------------|
| `rgt init` | Initialize .regent/ in current directory |
| `rgt status` | Show sessions and current state |
| `rgt log [--session ID] [-n N]` | Display step history |
| `rgt sessions` | List all sessions |
| `rgt cat <hash>` | Dump any object by hash (debug) |

More commands coming in phases 2-6:
- `rgt hook` (Phase 2): Hook entry point for Claude Code
- `rgt blame <path>` (Phase 3): Per-line provenance
- `rgt show <step>` (Phase 4): Step + conversation
- `rgt rewind <step>` (Phase 5): Time travel

## Development

```bash
# Project structure
regent/
├── cmd/rgt/              # CLI entry point
├── internal/
│   ├── store/           # Core object storage
│   ├── snapshot/        # Workspace → tree conversion
│   ├── index/           # SQLite indexing
│   ├── ignore/          # .regentignore parser
│   └── cli/             # Command implementations
└── test/                # Integration tests
```

Built with:
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [blake3](https://lukechampine.com/blake3) - Hashing
- [go-diff](https://github.com/sergi/go-diff) - Myers diff (Phase 3)
- [go-gitignore](https://github.com/sabhiram/go-gitignore) - Pattern matching
- [modernc.org/sqlite](https://modernc.org/sqlite) - Pure Go SQLite

## License

Apache 2.0

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing requirements, and PR process.

This is a POC implementation. See [POC.md](POC.md) for the complete technical specification.

Issues and PRs welcome at: https://github.com/regent-vcs/regent
