# Changelog

All notable changes to re_gent will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Comprehensive test suite with integration and acceptance tests
- Session fork detection and metadata tracking
- `rgt` command filtering to prevent recursive tracking
- CI/CD pipeline with linting and testing

### Changed
- Improved hook payload handling
- Enhanced index queries for session lineage

### Fixed
- Code formatting issues in transcript and test files

## [0.1.1-beta] - 2026-05-02

### Added
- Initial beta release
- Core object store implementation (blobs, trees, steps)
- Content-addressed storage with BLAKE3 hashing
- SQLite-based index for fast queries
- Basic CLI commands: `init`, `log`, `status`, `cat`, `version`
- Claude Code integration via PostToolUse hook
- Session tracking with DAG-based step lineage
- Automatic workspace snapshotting
- Transcript chain for conversation history
- Basic blame tracking

### Documentation
- Complete project documentation (CLAUDE.md, POC.md)
- Testing guide
- Contributing guidelines
- Code of conduct

---

## Links

- [Unreleased](https://github.com/regent-vcs/re_gent/compare/v0.1.1-beta...HEAD)
- [0.1.1-beta](https://github.com/regent-vcs/re_gent/releases/tag/v0.1.1-beta)
