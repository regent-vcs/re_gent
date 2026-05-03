# Quick Start for Contributors

Fast-track guide to contributing to Regent. For full details, see [CONTRIBUTING.md](CONTRIBUTING.md).

---

## Setup (5 minutes)

```bash
# Clone and setup
git clone https://github.com/regent-vcs/regent.git
cd regent
go mod download
go build -o rgt ./cmd/rgt

# Verify it works
./rgt version
go test ./...
```

---

## Make Your First Contribution

### 1. Find Something to Work On

**New to the project?** Look for issues labeled:
- [`good first issue`](https://github.com/regent-vcs/regent/labels/good%20first%20issue)
- [`help wanted`](https://github.com/regent-vcs/regent/labels/help%20wanted)
- [`documentation`](https://github.com/regent-vcs/regent/labels/documentation)

**Want something meatier?** Check:
- Phase implementations in [POC.md](../POC.md)
- Open feature requests

### 2. Create Your Branch

```bash
git checkout develop
git pull origin develop
git checkout -b feature/your-feature-name
```

### 3. Make Changes

```bash
# Edit code
vim internal/store/blob.go

# Run tests frequently
go test ./internal/store

# Format code
go fmt ./...

# Check linting
golangci-lint run
```

### 4. Commit

```bash
git add <files>
git commit -m "feat: add blob deduplication"
```

**Commit message format**: `type: description`
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `test:` tests
- `refactor:` refactoring

### 5. Push and Create PR

```bash
git push origin feature/your-feature-name
gh pr create --base develop --fill
```

### 6. Wait for Review

- CI must pass (tests, lint, build)
- Address review feedback
- Maintainer will merge when ready

---

## Common Commands

```bash
# Run all tests
go test ./...

# Run tests with race detector
go test -race ./...

# Run specific test
go test -run TestBlobStore ./internal/store

# Run linter
golangci-lint run

# Format code
go fmt ./...

# Build binary
go build -o rgt ./cmd/rgt

# Test your changes
./rgt init
./rgt status
./rgt log
```

---

## Before Opening a PR

**Checklist**:
- [ ] Tests pass: `go test ./...`
- [ ] Race detector passes: `go test -race ./...`
- [ ] Linter passes: `golangci-lint run`
- [ ] Code formatted: `go fmt ./...`
- [ ] Commits follow style: `type: description`
- [ ] PR template filled out

---

## Common Patterns

### Adding a New Command

```go
// cmd/rgt/commands/mycommand.go
package commands

import "github.com/spf13/cobra"

func NewMyCommandCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "mycommand",
        Short: "Does something useful",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Your logic here
            return nil
        },
    }
}

// Register in cmd/rgt/main.go
rootCmd.AddCommand(commands.NewMyCommandCmd())
```

### Writing a Table-Driven Test

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"empty", "", ""},
        {"hello", "hello", "HELLO"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := MyFunction(tt.input)
            if got != tt.expected {
                t.Errorf("got %q, want %q", got, tt.expected)
            }
        })
    }
}
```

### Using the Object Store

```go
// Write a blob
hash, err := blobStore.Write([]byte("content"))
if err != nil {
    return fmt.Errorf("write blob: %w", err)
}

// Read a blob
content, err := blobStore.Read(hash)
if err != nil {
    return fmt.Errorf("read blob: %w", err)
}
```

---

## Getting Help

- **Stuck?** Comment on the issue or open a discussion
- **Bug?** Check if it's already reported in [Issues](https://github.com/regent-vcs/regent/issues)
- **Question?** Ask in [Discussions](https://github.com/regent-vcs/regent/discussions)

---

## Important Files to Read

1. **[CLAUDE.md](../CLAUDE.md)** - Project context and vocabulary
2. **[POC.md](../POC.md)** - Implementation specification
3. **[CONTRIBUTING.md](CONTRIBUTING.md)** - Full contribution guide
4. **[README.md](../README.md)** - User-facing documentation

---

## Debugging Tips

```bash
# Run tests with verbose output
go test -v ./internal/store

# Run tests continuously
go test ./... -count=1  # disable caching

# Profile memory usage
go test -memprofile=mem.out ./internal/store
go tool pprof mem.out

# Check for races
go test -race ./...

# Use delve debugger
dlv test ./internal/store -- -test.run TestBlobStore
```

---

## Code Review Etiquette

**As an author**:
- Respond to all comments (even just "Done" or "Agreed")
- Don't take feedback personally
- Ask clarifying questions if unclear
- Mark conversations as resolved after addressing

**As a reviewer**:
- Be constructive and specific
- Explain the "why" behind suggestions
- Approve small improvements rather than demanding perfection
- Praise good solutions

---

## Need More Detail?

See the full [CONTRIBUTING.md](CONTRIBUTING.md) for:
- Architecture principles
- Testing strategies
- Security guidelines
- Release process
- Design decision workflow

---

_Happy contributing! 🎉_