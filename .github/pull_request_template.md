## Description

<!-- What does this PR do and why? -->

## Related Issue

Closes #

<!-- Link to related issues. Use "Closes #123" to auto-close when merged -->

---

## Type of Change

- [ ] Bug fix (non-breaking change that fixes an issue)
- [ ] New feature (non-breaking change that adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to change)
- [ ] Refactoring (code change that neither fixes a bug nor adds a feature)
- [ ] Documentation update
- [ ] Phase implementation (from POC.md)

---

## Testing Done

### Automated Tests
- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes (no race conditions)
- [ ] `golangci-lint run` passes (no new warnings)
- [ ] Added unit tests for new code
- [ ] Added integration tests (if applicable)

### Manual Testing
<!-- Describe what you tested manually and the results -->

**Test environment**:
- OS: <!-- macOS/Linux/Windows -->
- Go version: <!-- run: go version -->
- Regent version: <!-- run: ./rgt version -->

**Steps tested**:
1. 
2. 
3. 

**Expected behavior**:
<!-- What should happen -->

**Actual behavior**:
<!-- What actually happened -->

---

## Phase Implementation

<!-- Only fill out if this PR implements a phase from POC.md -->

- [ ] Implements Phase: ___ (specify number and name)
- [ ] Matches POC.md specification
- [ ] All acceptance tests from POC.md pass
- [ ] Architecture decisions documented in CLAUDE.md (if changed)

---

## Breaking Changes

- [ ] No breaking changes
- [ ] Contains breaking changes (describe below)

<!-- If breaking changes exist, describe:
1. What breaks
2. Why the break is necessary
3. Migration path for users
4. Deprecation timeline (if applicable)
-->

---

## Documentation

- [ ] README.md updated (if user-facing change)
- [ ] CLAUDE.md updated (if architecture changed)
- [ ] POC.md updated (if implementation differs from spec)
- [ ] docs/ updated (if commands or APIs changed)
- [ ] CHANGELOG.md updated (for user-facing changes)
- [ ] Code comments added (only for non-obvious logic)

---

## Code Quality

- [ ] Code follows Go conventions (`go fmt` applied)
- [ ] Imports organized with `goimports`
- [ ] No commented-out code left in
- [ ] No debug print statements left in
- [ ] Error messages are clear and actionable
- [ ] Variable and function names are descriptive

---

## Security Considerations

- [ ] No sensitive data logged (API keys, tokens, etc.)
- [ ] No SQL injection risks (if touching SQLite queries)
- [ ] No path traversal vulnerabilities (if handling file paths)
- [ ] No race conditions introduced (verified with `-race`)
- [ ] Dependencies reviewed (if adding new packages)

<!-- If security-relevant, explain the threat model and mitigations -->

---

## Performance Impact

- [ ] No performance impact expected
- [ ] Performance tested and acceptable
- [ ] Performance regression (explain why acceptable below)

<!-- If performance-relevant:
1. Benchmark results (go test -bench)
2. Profile results (pprof output)
3. Resource usage (memory, disk, CPU)
-->

---

## Screenshots (if applicable)

<!-- For CLI output changes, include before/after screenshots or terminal output -->

**Before**:
```
# paste output here
```

**After**:
```
# paste output here
```

---

## Checklist

- [ ] I have read [CONTRIBUTING.md](../CONTRIBUTING.md)
- [ ] I have read [CLAUDE.md](../CLAUDE.md) and [POC.md](../POC.md)
- [ ] My code follows the project's coding standards
- [ ] I have performed a self-review of my code
- [ ] I have commented my code only where necessary
- [ ] My changes generate no new warnings
- [ ] I have added tests that prove my fix/feature works
- [ ] New and existing tests pass locally
- [ ] Any dependent changes have been merged

---

## Additional Context

<!-- Add any other context, design decisions, or implementation notes -->

---

## Reviewer Notes

<!-- Optional: Call out specific areas for reviewers to focus on -->
- [ ] Please focus on: 
- [ ] Known issues/TODOs: 
- [ ] Follow-up work needed: 