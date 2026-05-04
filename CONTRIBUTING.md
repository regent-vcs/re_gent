# Contributing to re_gent

Thanks for your interest in contributing to re_gent! This document outlines the development process, testing requirements, and PR guidelines.

---

## Quick Links

- [Code of Conduct](CODE_OF_CONDUCT.md)
- [Brand Guidelines](BRAND.md) — **Read this if touching anything user-facing**
- [Architecture Overview](CLAUDE.md)
- [Implementation Plan](POC.md)
- [Testing Strategy](TESTING.md)

---

## Before You Start

1. **Check existing issues** — Someone might already be working on it
2. **Open an issue first** for large changes — Discuss the approach before writing code
3. **Read [BRAND.md](BRAND.md)** if you're touching CLI output, docs, or error messages

---

## Development Setup

### Prerequisites

- **Go 1.22+** — `go version` to check
- **Git** — for version control (meta, we know)
- **Make** (optional) — for convenience commands

### Clone and Build

```bash
git clone https://github.com/regent-vcs/regent
cd regent
go mod download
go build -o rgt ./cmd/rgt
./rgt --version
```

### Run Tests

```bash
# All tests
go test ./...

# With race detector
go test -race ./...

# Specific package
go test ./internal/store -v

# With coverage
go test -cover ./...
```

See [TESTING.md](TESTING.md) for the full testing strategy and phase-specific test requirements.

---

## Project Structure

```
regent/
├── cmd/rgt/              # CLI entry point
├── internal/
│   ├── store/           # Core object storage (blobs, trees, steps)
│   ├── snapshot/        # Workspace → tree conversion
│   ├── index/           # SQLite derived index
│   ├── ignore/          # .regentignore parser
│   ├── cli/             # Command implementations
│   └── hook/            # Claude Code integration
├── test/                # Integration tests
└── docs/                # Additional documentation
```

**Golden rule**: Code in `internal/` is not public API. It can change between versions without notice.

---

## Coding Standards

### Go Style

- Follow [Effective Go](https://go.dev/doc/effective_go)
- Run `gofmt` before committing (CI will check)
- Use `golangci-lint` for linting (config in `.golangci.yml`)

### Comments

**Rare by default.** Only comment when the *why* is non-obvious:

```go
// Good - explains a non-obvious constraint
// SAFETY: Must hold store.mu during ref CAS to prevent concurrent updates
func (s *Store) UpdateRef(name string, old, new Hash) error { ... }

// Bad - repeats what the code says
// UpdateRef updates a ref
func (s *Store) UpdateRef(name string, old, new Hash) error { ... }
```

### Error Handling

- Wrap errors with context: `fmt.Errorf("snapshot workspace: %w", err)`
- User-facing errors should follow [BRAND.md](BRAND.md) tone
- Internal errors can be terse

### Tests

- Test files live next to source: `store.go` → `store_test.go`
- Use table-driven tests for multiple cases
- Test names: `TestFunctionName_Scenario` (e.g., `TestBlobWrite_Deduplication`)

---

## CLI Development

### Adding a New Command

1. Create command file: `internal/cli/yourcommand.go`
2. Register in `internal/cli/root.go`
3. Follow naming conventions from [BRAND.md](BRAND.md)
4. Add tests in `internal/cli/yourcommand_test.go`

### CLI Output Guidelines

**Required reading**: [BRAND.md § CLI Styling Rules](BRAND.md#cli-styling-rules)

Key points:
- Respect `NO_COLOR` environment variable
- Use color for UI chrome, not data
- Test output in both dark and light terminals
- Provide `--json` for scripting

Example:
```go
// Good - data is uncolored, labels are colored
fmt.Printf("%s: %s\n", 
    aurora.Bold("Session"), 
    sessionID)

// Bad - everything is colored
fmt.Printf("%s: %s\n", 
    aurora.Blue("Session"), 
    aurora.Green(sessionID))
```

---

## Pull Request Process

### Before Opening a PR

- [ ] Tests pass locally (`go test ./...`)
- [ ] Code is formatted (`gofmt -w .`)
- [ ] Lint passes (`golangci-lint run`)
- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
- [ ] If touching user-facing output, checked against [BRAND.md](BRAND.md)

### PR Title Format

```
feat: add session filtering to rgt log
fix: race condition in ref CAS
docs: clarify blame algorithm in POC.md
test: add coverage for tree determinism
```

### PR Description Template

```markdown
## What

Brief description of the change.

## Why

Why is this change needed? Link to issue if applicable.

## Testing

How did you test this? Include commands run, edge cases checked.

## Checklist

- [ ] Tests added/updated
- [ ] Documentation updated (if needed)
- [ ] Follows [BRAND.md](BRAND.md) (if user-facing)
- [ ] No breaking changes (or marked with `BREAKING:` in title)
```

### Review Process

1. **Automated checks** — CI must pass (tests, lint, format)
2. **Maintainer review** — One approval required
3. **Brand check** (if applicable) — @shayliv reviews user-facing changes
4. **Merge** — Squash merge preferred for feature PRs

---

## Issue Labels

- `bug` — Something broken
- `enhancement` — New feature or improvement
- `docs` — Documentation changes
- `good-first-issue` — Beginner-friendly
- `brand` — Requires brand guideline review
- `breaking` — Breaking change in next release
- `phase-N` — Tied to a specific POC phase

---

## Roadmap & Phases

re_gent is being built in phases per [POC.md](POC.md):

- **Phase 1**: Object store skeleton ✅ **(Complete)**
- **Phase 2**: Hook integration ✅ **(Complete)**
- **Phase 3**: Blame algorithm (In Progress)
- **Phase 4**: Transcript staging
- **Phase 5**: Rewind
- **Phase 6**: Concurrency hardening

Check the [GitHub Projects board](https://github.com/regent-vcs/regent/projects) for current priorities.

---

## Architecture Decisions

For significant changes to the data model, algorithms, or CLI structure:

1. Open an issue tagged `architecture`
2. Reference relevant sections from [CLAUDE.md](CLAUDE.md) or [POC.md](POC.md)
3. Discuss tradeoffs before implementation
4. Update docs after the decision

**Current open design questions** are listed in [CLAUDE.md § Open Design Questions](CLAUDE.md#open-design-questions).

---

## Code of Conduct

We follow the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). Be kind, assume good faith, and remember we're all here because we care about making agent-driven development better.

---

## Getting Help

- **Questions about contributing?** Open a discussion or ping in the issue
- **Found a bug?** Open an issue with reproduction steps
- **Want to pair on a feature?** Tag the issue with `pairing-welcome`

---

## License

By contributing, you agree that your contributions will be licensed under the Apache 2.0 License. See [LICENSE](LICENSE) for details.

---

**Thanks for contributing!** 🚀

Every PR, issue, and discussion helps make re_gent better for developers wrestling with agent-generated code.
