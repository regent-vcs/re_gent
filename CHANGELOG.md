# Changelog

All notable changes to re_gent will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-05-14

### Added
- OpenCode integration with interactive agent selection during `rgt init`
- Idempotent `rgt init` with existing hook detection (safe to re-run)

### Changed
- README rewritten for stable release
- Cleaned up CLI output and error messages

### Fixed
- Linting errors in init command
- GoReleaser config pointing to incorrect repository name

## [0.2.0] - 2026-05-10

### Added
- Codex CLI capture parity (full hook integration with `SessionStart`, `UserPromptSubmit`, `PostToolUse`, `Stop`)
- Enhanced `rgt log` with full conversation display, graph view, and improved UX
- Workspace snapshotting and blame computation in message hook
- VSCode extension section in README
- Comprehensive unit test suite for internal packages
- Discord community link
- Roadmap and issue templates for phases

### Fixed
- Restored PostToolUse hook with blame computation
- Homebrew tap push using dedicated token
- Codex parity review blockers

## [0.1.2] - 2026-05-04

### Added
- Homebrew installation support (`brew tap regent-vcs/tap && brew install regent`)

### Fixed
- GoReleaser action upgraded to v6 for config v2 support

## [0.1.1-beta] - 2026-05-02

### Added
- Initial beta release
- Core object store implementation (blobs, trees, steps)
- Content-addressed storage with BLAKE3 hashing
- SQLite-based index for fast queries
- CLI commands: `init`, `log`, `status`, `cat`, `version`
- Claude Code integration via PostToolUse hook
- Session tracking with DAG-based step lineage
- Automatic workspace snapshotting
- Transcript chain for conversation history
- Basic blame tracking

---

## Links

- [1.0.0](https://github.com/regent-vcs/regent/compare/v0.2.0...v1.0.0)
- [0.2.0](https://github.com/regent-vcs/regent/compare/v0.1.2...v0.2.0)
- [0.1.2](https://github.com/regent-vcs/regent/compare/v0.1.1-beta...v0.1.2)
- [0.1.1-beta](https://github.com/regent-vcs/regent/releases/tag/v0.1.1-beta)
