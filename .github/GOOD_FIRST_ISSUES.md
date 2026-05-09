# Good First Issues to Create

Copy-paste these into GitHub Issues, one at a time, with the `good first issue` and `enhancement` labels.

---

## Issue 1: Add `--format=json` flag to `rgt sessions` command

**Labels:** `good first issue`, `enhancement`, `cli`

**Description:**

The `rgt sessions` command currently only outputs human-readable text. We should add a `--format=json` flag to enable programmatic use (scripts, CI/CD, integrations).

**Acceptance Criteria:**
- [ ] Add `--format` flag to `rgt sessions` command accepting `text` (default) or `json`
- [ ] JSON output includes all session data: `session_id`, `step_count`, `last_activity`, `agent_id`
- [ ] JSON output is valid and parseable
- [ ] Update command help text to document the new flag
- [ ] Add test case covering JSON output format

**Example Output:**
```json
{
  "sessions": [
    {
      "session_id": "claude-20260502-143021",
      "step_count": 3,
      "last_activity": "2026-05-02T14:30:21Z",
      "agent_id": "claude-code"
    }
  ]
}
```

**Files to Edit:**
- `internal/cli/sessions.go` — Add flag and JSON formatting logic
- `internal/cli/sessions_test.go` — Add test case

**Helpful Context:**
- Look at other commands (like `rgt log`) for examples of output formatting
- See [BRAND.md](../BRAND.md) for CLI conventions
- Follow the existing command structure in `internal/cli/`

**Estimated Time:** 1-2 hours

---

## Issue 2: Improve error message when `.regent/` doesn't exist

**Labels:** `good first issue`, `enhancement`, `ux`

**Description:**

When running `rgt` commands outside a re_gent repository, the error message is generic. We should provide a helpful message that guides users to run `rgt init`.

**Current Behavior:**
```
Error: .regent/ directory not found
```

**Desired Behavior:**
```
Error: not a re_gent repository (or any parent directory)

Hint: Initialize re_gent in this directory by running:
    rgt init
```

**Acceptance Criteria:**
- [ ] Update error message to match desired behavior
- [ ] Error should be shown for all commands that require `.regent/` (log, sessions, show, blame, status)
- [ ] `rgt init` and `rgt version` should NOT show this error (they don't require a repo)
- [ ] Message should be consistent across all commands
- [ ] Add test case verifying the improved error message

**Files to Edit:**
- `internal/store/store.go` or `internal/cli/root.go` — Centralize the error message
- Tests for commands that require a repo

**Helpful Context:**
- Git's similar error: `fatal: not a git repository (or any of the parent directories): .git`
- The error check likely happens in the store initialization
- See [BRAND.md](../BRAND.md) § Error Messages for tone guidance

**Estimated Time:** 1-2 hours

---

## Issue 3: Add example scenario: "Debugging a Bad Refactor"

**Labels:** `good first issue`, `documentation`, `examples`

**Description:**

Create a realistic example showing how to use `rgt blame` and `rgt log` to debug when an AI agent refactored code incorrectly.

**What to Create:**

A new directory `examples/bad-refactor/` containing:

1. **README.md** — Step-by-step walkthrough:
   - Setup: What the "bad refactor" broke
   - How to use `rgt log` to find the refactor step
   - How to use `rgt blame` to see which prompt changed a specific line
   - How to use `rgt show` to see the full context
   - Conclusion: What you learned

2. **Sample files** to demonstrate:
   - Before: `app.py` (working version)
   - After: `app.py` (broken after agent refactor)
   - Sample `.regent/` snapshot (or instructions to generate it)

3. **Script** (optional): `setup.sh` that:
   - Initializes the example repo
   - Creates the "bad refactor" scenario
   - Pre-populates some steps

**Acceptance Criteria:**
- [ ] Example can be run in < 5 minutes
- [ ] README is clear and beginner-friendly
- [ ] Demonstrates at least 3 re_gent commands (`log`, `blame`, `show`)
- [ ] Shows a realistic scenario (not toy code)
- [ ] Add link to this example from main README's "Examples" section (create if doesn't exist)

**Files to Create:**
- `examples/bad-refactor/README.md`
- `examples/bad-refactor/app.py` (or similar)
- `examples/bad-refactor/setup.sh` (optional)

**Helpful Context:**
- Look at the `demo/` directory for inspiration
- Think about common agent mistakes: over-abstraction, removing needed code, breaking tests
- See [BRAND.md](../BRAND.md) for writing tone

**Estimated Time:** 2-3 hours

---

## How to Claim an Issue

1. Comment on the issue: "I'd like to work on this!"
2. Wait for a maintainer to assign it to you
3. Fork the repo and create a branch
4. Follow [CONTRIBUTING.md](../CONTRIBUTING.md) guidelines
5. Open a PR when ready

**Questions?** Ask in the issue comments or open a [Discussion](https://github.com/regent-vcs/regent/discussions).
