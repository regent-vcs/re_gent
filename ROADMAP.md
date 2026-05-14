# re_gent Roadmap to v1.0.0-beta

> **Current Status**: Phase 3 (Advanced Features) — Core functionality complete, building toward production stability

This roadmap outlines the path from our current state to a production-ready v1.0.0-beta release. All phases are tracked via [GitHub milestones](https://github.com/regent-vcs/regent/milestones) and issues are labeled by phase for easy filtering.

---

## Vision: What is v1.0.0-beta?

A production-ready version control system for AI agents that:

- ✅ **Captures agent activity transparently** with zero manual intervention
- ✅ **Provides git-level auditability** through `log`, `blame`, and `show` commands
- 🚧 **Enables time-travel** via non-destructive `rewind` and `fork` operations
- 🚧 **Scales to real-world codebases** (10k+ files, hundreds of sessions)
- 🚧 **Works with multiple AI tools** (Claude Code and Codex complete; Cursor, Cline, Continue planned)
- 📋 **Supports collaboration** (session sharing, merge strategies)

**Target Date**: Q3 2026

---

## Current State (as of May 2026)

**✅ Complete:**
- ✅ Content-addressed object store (BLAKE3, dedupe, atomic writes)
- ✅ SQLite query index with sub-10ms lookups
- ✅ Per-session DAG with concurrent session support
- ✅ Hook integration (Claude Code and Codex per-turn capture)
- ✅ Conversation tracking (survives `/compact` and `/clear`)
- ✅ Core commands: `init`, `log`, `sessions`, `status`, `show`, `blame`
- ✅ `.regentignore` support (gitignore-compatible)
- ✅ CAS refs with retry logic (concurrency-safe)
- ✅ VSCode extension with inline blame

**🚧 In Progress:**
- 🚧 `rgt fork` — Create new sessions from any step
- 🚧 `rgt rewind` — Non-destructive time-travel
- 🚧 Performance optimization for large repos

**📋 Not Started:**
- 📋 Additional adapters (Cursor, Cline, Continue)
- 📋 Session sharing / remote sync
- 📋 Garbage collection
- 📋 Merge strategies for concurrent edits

---

## Phase Breakdown

### ✅ Phase 1: Core Storage (Complete)

**Goal**: Prove the storage model works with content-addressed blobs, trees, and steps.

**Deliverables:**
- [x] Object store implementation (blob, tree, step)
- [x] BLAKE3 hashing with automatic deduplication
- [x] Workspace snapshot algorithm
- [x] SQLite index for fast queries
- [x] Ref management with CAS
- [x] `.regentignore` support

**Outcome**: Storage layer is production-quality. Handles concurrent writes without corruption.

---

### ✅ Phase 2: Hook Integration (Complete)

**Goal**: Capture Claude Code and Codex activity transparently with zero manual commits.

**Deliverables:**
- [x] Claude Code `UserPromptSubmit`, `PostToolBatch`, and `Stop` hook implementation
- [x] Codex `SessionStart`, `UserPromptSubmit`, `PostToolUse`, and `Stop` hook implementation
- [x] Conversation staging and transcript archival
- [x] Session tracking and management
- [x] `rgt log` with filtering (`--session`, `-n`, `--json`, `--graph`)
- [x] `rgt blame` with per-line provenance
- [x] `rgt show` for full step context

**Outcome**: Daily use by contributors. Core functionality is feature-complete.

---

### 🚧 Phase 3: Advanced Features (In Progress — May 2026)

**Goal**: Enable time-travel and exploration workflows.

**Deliverables:**
- [ ] `rgt fork <step>` — Create new session from any step ([#12](https://github.com/regent-vcs/regent/issues/12))
  - New session branches from specified step
  - Conversation state initialized from fork point
  - Session metadata tracks fork ancestry
- [ ] `rgt rewind <step>` — Non-destructive time-travel ([#11](https://github.com/regent-vcs/regent/issues/11))
  - Per-session rewind (doesn't affect other sessions)
  - Files-only mode (`--files`) vs conversation-only
  - Conflict detection when other sessions touched same files
  - CAS safety: abort if session has moved forward since last read
- [ ] Performance optimization ([#15](https://github.com/regent-vcs/regent/issues/15))
  - Incremental snapshots (track mtime, skip unchanged files)
  - Lazy blame computation (compute on first query, not on write)
  - Parallel tree walks for large repos
  - Benchmark suite: target <500ms for 10k-file snapshot

**Success Criteria:**
- `rgt fork` creates independent session with correct ancestry
- `rgt rewind` restores files + conversation to target step
- 10k-file repo: snapshot <500ms, blame query <10ms (p99)
- No data loss or corruption under concurrent load

**Issues:**
- [#12: Implement `rgt fork` command](https://github.com/regent-vcs/regent/issues/12)
- [#11: Implement `rgt rewind` command](https://github.com/regent-vcs/regent/issues/11)
- [#15: Performance optimization for large repos](https://github.com/regent-vcs/regent/issues/15)

---

### 📋 Phase 4: Additional Tool Support (Planned)

**Goal**: Extend the shared capture adapter model beyond Claude Code and Codex.

**Deliverables:**
- [ ] Adapter architecture using the shared capture event model
- [ ] Cursor adapter ([#20](https://github.com/regent-vcs/regent/issues/20))
  - Hook integration for Cursor's `.cursorrules`
  - Conversation extraction from Cursor's state
- [ ] Cline adapter ([#21](https://github.com/regent-vcs/regent/issues/21))
  - MCP-based capture (if Cline exposes it)
  - Fallback: filesystem watcher + log parsing
- [ ] Continue adapter ([#22](https://github.com/regent-vcs/regent/issues/22))
  - Extension hook for VS Code Continue plugin
- [ ] Claude Agent SDK adapter ([#23](https://github.com/regent-vcs/regent/issues/23))
  - Python decorator: `@regent.track`
  - Automatic step creation on tool use
- [ ] Tool detection (auto-detect which tool is running)
- [ ] Origin registry/mapping for additional adapters

**Success Criteria:**
- All four adapters capture activity to the same `.regent/`
- `rgt log --all` shows unified history across tools
- No session ID collisions between tools
- Origin metadata allows filtering by tool

**Issues:**
- [#20: Cursor adapter](https://github.com/regent-vcs/regent/issues/20)
- [#21: Cline adapter](https://github.com/regent-vcs/regent/issues/21)
- [#22: Continue adapter](https://github.com/regent-vcs/regent/issues/22)
- [#23: Claude Agent SDK adapter](https://github.com/regent-vcs/regent/issues/23)

---

### 📋 Phase 5: Collaboration (Planned — Q3 2026)

**Goal**: Share sessions between devs and merge concurrent work.

**Deliverables:**
- [ ] `rgt push <remote>` — Push session to remote store ([#30](https://github.com/regent-vcs/regent/issues/30))
  - S3-compatible backends (Cloudflare R2, MinIO, AWS S3)
  - Content-addressed objects sync (only missing blobs transferred)
  - Ref updates with CAS safety
- [ ] `rgt pull <session>` — Fetch remote session ([#31](https://github.com/regent-vcs/regent/issues/31))
  - Download missing objects
  - Merge session DAG into local `.regent/`
- [ ] `rgt merge <session>` — Merge two sessions ([#32](https://github.com/regent-vcs/regent/issues/32))
  - Three-way merge (common ancestor, two tips)
  - Conflict detection (same file, different edits)
  - Interactive resolution (choose left, right, or manual edit)
- [ ] Remote configuration (`rgt remote add <name> <url>`)
- [ ] Session sharing UX (generate shareable links)

**Success Criteria:**
- Two devs push sessions to same remote without corruption
- `rgt pull` fetches only missing objects (dedupe works)
- `rgt merge` detects conflicts and offers resolution UI
- Merge commits have two parents (secondary_parent field)

**Issues:**
- [#30: Implement `rgt push`](https://github.com/regent-vcs/regent/issues/30)
- [#31: Implement `rgt pull`](https://github.com/regent-vcs/regent/issues/31)
- [#32: Implement `rgt merge`](https://github.com/regent-vcs/regent/issues/32)

---

### 📋 Phase 6: Production Hardening (Planned — Q3 2026)

**Goal**: v1.0.0-beta release with enterprise-ready reliability.

**Deliverables:**
- [ ] `rgt gc` — Garbage collection ([#40](https://github.com/regent-vcs/regent/issues/40))
  - Mark-and-sweep: reachable from refs + grace period
  - Orphan detection (unreachable objects)
  - `rgt reflog` — Track ref movements (undo `rgt rewind`)
- [ ] Error recovery ([#41](https://github.com/regent-vcs/regent/issues/41))
  - Corrupt object detection (hash mismatch)
  - `rgt fsck` — Verify object store integrity
  - `rgt reindex` — Rebuild SQLite from objects
- [ ] Observability ([#42](https://github.com/regent-vcs/regent/issues/42))
  - Structured logging (JSON for aggregation)
  - Metrics: step write latency, blame query latency, storage size
  - `rgt stats` — Summary dashboard (sessions, steps, storage)
- [ ] Security audit ([#43](https://github.com/regent-vcs/regent/issues/43))
  - Dependency audit (Dependabot)
  - Path traversal prevention (`.regentignore` bypass)
  - Remote auth (signed URLs, OAuth)
- [ ] Documentation ([#44](https://github.com/regent-vcs/regent/issues/44))
  - Migration guide (from POC to v1.0)
  - API reference (for adapter authors)
  - Troubleshooting guide (common issues)
  - Video walkthrough (5-minute intro)

**Success Criteria:**
- `rgt gc` reclaims space from abandoned branches
- `rgt fsck` detects and reports corruption
- Security audit passes (no critical CVEs)
- Documentation covers 90% of user questions (measured by closed issues)

**Issues:**
- [#40: Implement `rgt gc`](https://github.com/regent-vcs/regent/issues/40)
- [#41: Error recovery and `rgt fsck`](https://github.com/regent-vcs/regent/issues/41)
- [#42: Observability and metrics](https://github.com/regent-vcs/regent/issues/42)
- [#43: Security audit](https://github.com/regent-vcs/regent/issues/43)
- [#44: Documentation overhaul](https://github.com/regent-vcs/regent/issues/44)

---

## v1.0.0-beta Definition

A v1.0.0-beta release means:

**Feature Complete:**
- ✅ Core commands: `init`, `log`, `blame`, `show`, `sessions`, `status`
- ✅ Time-travel: `rewind`, `fork`
- ✅ Collaboration: `push`, `pull`, `merge`
- ✅ Maintenance: `gc`, `fsck`, `reindex`
- ✅ Multi-tool: Claude Code + Codex + Cursor + Cline + Continue

**Production Ready:**
- ✅ No known data-loss bugs
- ✅ Handles 10k+ file repos without performance degradation
- ✅ Concurrent sessions work correctly (tested under `-race`)
- ✅ Documentation covers installation, usage, troubleshooting
- ✅ Security audit passed
- ✅ CI/CD: automated tests, linting, release builds

**Stability Promise:**
- ✅ Breaking changes will be rare and well-documented
- ✅ Migration path provided for any storage format changes
- ✅ Public APIs (CLI flags, JSON output) are stable

---

## How to Contribute

re_gent is built in public. Contributions are welcome at every phase!

### 🟢 Good First Issues

Start here if you're new to the project:

- [See all issues labeled `good first issue`](https://github.com/regent-vcs/regent/labels/good%20first%20issue)
- [.github/GOOD_FIRST_ISSUES.md](.github/GOOD_FIRST_ISSUES.md) — Curated list with context

### 🔍 Find Work by Phase

Issues are tagged by phase for easy filtering:

- [`phase-3`](https://github.com/regent-vcs/regent/labels/phase-3) — Advanced features (current focus)
- [`phase-4`](https://github.com/regent-vcs/regent/labels/phase-4) — Multi-tool support
- [`phase-5`](https://github.com/regent-vcs/regent/labels/phase-5) — Collaboration
- [`phase-6`](https://github.com/regent-vcs/regent/labels/phase-6) — Production hardening

### 💡 Propose a Feature

Not on the roadmap yet? Open an issue with:

1. **Problem**: What pain point does this solve?
2. **Proposal**: How should it work? (CLI examples, API sketches)
3. **Alternatives**: What other solutions did you consider?
4. **Impact**: Who benefits? How common is this use case?

We prioritize features that:
- Solve real user pain (not hypothetical edge cases)
- Fit the project's scope (agent version control, not general VCS)
- Have clear success criteria (testable, measurable)

### 📚 Resources for Contributors

- [CONTRIBUTING.md](CONTRIBUTING.md) — Full contribution guide
- [QUICK_START.md](.github/QUICK_START.md) — 5-minute setup
- [CLAUDE.md](CLAUDE.md) — Architecture and vocabulary
- [POC.md](POC.md) — Technical specification
- [TESTING.md](TESTING.md) — Testing strategy

---

## FAQ

### Why not just use git?

Git tracks *commits* (manual, developer-authored). re_gent tracks *agent activity* (automatic, tool-generated). They serve different purposes and complement each other.

Key differences:
- Git doesn't capture conversation context (the *why* behind each change)
- Git requires manual staging/committing (agent tools don't expose this)
- Git's merge model assumes human conflict resolution (agents need automatic strategies)

See [README.md](README.md#regent-vs-git) for full comparison.

### When will multi-tool support land?

Claude Code and Codex CLI are supported by the shared capture engine. Additional agent adapters can follow the same `origin`, canonical `session_id`, and `turn_id` event contract.

### Will v1.0 be backward-compatible?

Yes. Post-v1.0, storage format changes will include migration tools (`rgt migrate`). Pre-v1.0, breaking changes are announced in release notes with migration guides.

### Can I use re_gent in production today?

**Depends.** Core functionality (init, log, blame) is production-quality. Advanced features (rewind, fork, push/pull) are not yet stable. Use at your own risk for critical workflows.

### How do I report bugs?

Open an issue with:
1. `rgt version` output
2. Steps to reproduce
3. Expected vs actual behavior
4. Relevant `rgt log` or `rgt show` output

For security issues, email [security@regent-vcs.dev](mailto:security@regent-vcs.dev) (not public issues).

---

## Staying Updated

- **Milestones**: https://github.com/regent-vcs/regent/milestones
- **Discussions**: https://github.com/regent-vcs/regent/discussions
- **Releases**: https://github.com/regent-vcs/regent/releases
- **Twitter**: [@regent_vcs](https://twitter.com/regent_vcs) (announcements only, no spam)

---

<div align="center">
  <p><strong>Built in public. Contributions welcome.</strong></p>
  <p>
    <a href="https://github.com/regent-vcs/regent/discussions">Discussions</a> •
    <a href="https://github.com/regent-vcs/regent/issues">Issues</a> •
    <a href="CONTRIBUTING.md">Contributing</a>
  </p>
</div>
