# re_gent Brand Guidelines

> *"Version control for AI agents"* — but make it vibe.

This document is serious about the brand, but not about itself. If you're touching anything user-facing (CLI output, docs, logos, error messages), read this first.

---

## Mission Statement

re_gent gives developers **git-level control** over AI agent activity. We're building the missing version control layer that should have existed the moment agents started writing code.

Our vibe: **professional tools with personality**. We're not enterprise-boring, but we're not startup-cutesy either. Think: `git` meets modern CLI aesthetics (like `gh`, `ripgrep`, `exa`).

---

## Core Principles

1. **Clarity over cleverness** — Error messages should help, not puzzle.
2. **Respect the user's time** — Fast defaults, escape hatches for edge cases.
3. **Powerful, not patronizing** — Users are developers. Trust them.
4. **Memorable, not gimmicky** — `rgt` is short because developers type it hundreds of times, not because we're being cute.
5. **Brand Name - !important** - 'It's re_gent, not Regent, not regent.'

---

## Visual Identity

### Brand Name: re_gent

The brand name is **re_gent** — always lowercase with an underscore.

**Correct:**
- re_gent
- `re_gent` (in monospace/code)

**Incorrect:**
- Regent
- regent  
- REGENT
- Re_gent
- re-gent

**In CLI output:** The brand name appears in purple (re_gent Purple #9B59D0 / ANSI 141) for "brand moments" — occasions where we're identifying ourselves:
- `rgt version` output
- `rgt init` header
- Introduction text in help/wizards

**In documentation:** Use **re_gent** in bold when first introducing the tool. Subsequent references can be plain text or refer to the command `rgt`.

---

### Logo

**Primary mark**: The crown icon 👑 (U+1F451)

- Used in: Documentation headers, social media, stickers (eventually)
- **Not used in**: CLI output (emoji in terminals = encoding hell), error messages, logs

**Typography logo**: `REGENT` in monospace, all caps
```
 ██████  ███████  ██████  ███████ ██   ██ ████████ 
 ██   ██ ██      ██       ██      ███  ██    ██    
 ██████  █████   ██   ███ █████   ██ █ ██    ██    
 ██   ██ ██      ██    ██ ██      ██  ███    ██    
 ██   ██ ███████  ██████  ███████ ██   ██    ██    
```

- Used in: ASCII art contexts, welcome banners (not in version output anymore - we use "re_gent" there)
- Font: Any clean monospace (JetBrains Mono, Fira Code, SF Mono)

### Colors

**Primary palette** (CLI + terminal output):

```
re_gent Purple:  #9B59D0  ANSI 141  (primary brand color, used for brand moments)
Royal Blue:     #5B7FFF  ANSI 69   (labels, structure)
Emerald:        #10B981  ANSI 42   (success states)
Amber:          #F59E0B  ANSI 214  (warnings)
Rose:           #EF4444  ANSI 196  (errors, destructive actions)
Slate:          #64748B  ANSI 103  (de-emphasized text, metadata)
```

**ANSI 256 Color Implementation:**

We use ANSI 256-color codes instead of RGB for better terminal compatibility:

```go
Purple256 = "\033[38;5;141m"  // re_gent Purple
Blue256   = "\033[38;5;69m"   // Royal Blue
Green256  = "\033[38;5;42m"   // Emerald
Amber256  = "\033[38;5;214m"  // Amber
Red256    = "\033[38;5;196m"  // Rose
Gray256   = "\033[38;5;103m"  // Slate
```

**Dark mode defaults** (assume dark terminals):
- Text: `#E2E8F0` (light slate)
- Background: user's terminal background (don't override)
- Never use pure white (#FFFFFF) or pure black (#000000) — too harsh

**Light mode fallback**:
- Text: `#1E293B` (dark slate)
- Same palette, but test contrast ratios (WCAG AA minimum)

### CLI Styling Rules

**Color usage hierarchy**:
1. **No color by default for data** — Hashes, timestamps, file paths should work in pipes
2. **Color for UI chrome** — Headings, labels, prompts get color
3. **Semantic color** — Green = success, red = error, amber = warning, purple = re_gent-specific highlights
4. **Respect `NO_COLOR`** — If `NO_COLOR` env var is set, strip all ANSI codes

**Typography**:
- **Bold** for emphasis (headings, key terms)
- *Italic* for secondary info (timestamps, metadata) — but sparingly, not all terminals support it
- `Monospace inline` for code, hashes, paths — but use actual monospace rendering, not backticks
- Dim/faint for de-emphasized content (parent hashes, internal IDs)

**Output structure**:
```
# Good - scannable, color on labels
Session: a1b2c3d4
Status:  ✓ Active
Steps:   23

# Bad - color everywhere, hard to parse
Session: a1b2c3d4
Status: ✓ Active
Steps: 23
```

---

### CLI Output Examples

Here are concrete examples of how each command looks with our git-inspired styling:

#### `rgt version`
```
re_gent version 0.1.0 (commit: abc1234)
```
Colors: "re_gent" in purple (brand moment), rest plain

#### `rgt log`
```
Session: a1b2c3d4 (12 steps)

ef4a89c2  2026-05-01 14:23:45  Edit
  parent: 8c9a02f1
  tool_use_id: toolu_abc123

8c9a02f1  2026-05-01 14:19:32  Read
  parent: 3f72bac4
  tool_use_id: toolu_def456
```
Colors:
- "Session:" label in blue
- "(12 steps)" dimmed
- Hashes plain (pipeable)
- Timestamps dimmed
- Tool names plain
- Tree items (parent, tool_use_id) dimmed

#### `rgt status`
```
re_gent repository: /path/to/project/.regent
Sessions: 3

Session: abc12345
  Origin:     claude-code-v1.2.0
  Started:    2026-05-01 10:00:00
  Last seen:  2026-05-01 14:23:45
  Head:       ef4a89c2
```
Colors:
- "re_gent" in purple
- Labels ("repository:", "Sessions:", "Session:", "Origin:", etc.) in blue
- Values plain
- Timestamps dimmed

#### `rgt blame <file>`
```
ef4a89c2 2026-05-01 14:23:45 Edit      │    1 │ package main
ef4a89c2 2026-05-01 14:23:45 Edit      │    2 │
8c9a02f1 2026-05-01 14:19:32 Read      │    3 │ import (
```
Colors:
- Hashes plain (pipeable)
- Timestamps dimmed
- Tool names plain
- Column dividers (│) dimmed
- Line content plain

#### `rgt show <hash>`
```
Step: ef4a89c2deadbeef
Time: 2026-05-01 14:23:45
Tool: Edit
Tool Use ID: toolu_abc123
Parent: 8c9a02f1

═══ Tool Arguments ═══
{
  "file_path": "/path/to/file.go",
  "content": "..."
}
```
Colors:
- Labels ("Step:", "Time:", "Tool:", etc.) in blue
- Hash values plain
- Timestamps dimmed
- Section headers ("═══ Tool Arguments ═══") dimmed
- JSON content plain

#### `rgt init` (interactive)
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  re_gent - Version Control for AI Agent Activity
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

━━━ Step 1/3: Initialize Repository

  ✓ Created .regent/ directory
  ✓ Initialized object store
  ✓ Created SQLite index

━━━ Step 2/3: Configure Claude Code Hook

re_gent captures step history automatically via Claude Code hooks.

This will configure .claude/settings.json to run 'rgt hook'
after each tool use (Write, Edit, Bash, etc.).

Enable automatic tracking? [Y/n]:

  ✓ Hook configured in .claude/settings.json
  ✓ Steps will be captured automatically

━━━ Step 3/3: Install Claude Skills

Claude skills let you use re_gent commands with slash syntax:

  /regent-log [limit]      Show step history
  /regent-blame <file>     Show line provenance
  /regent-show <step>      Show step details
  /regent-rewind <step>    Rewind to a step

Install skills? [Y/n]:

  ✓ Skills installed in .claude/skills/
  ✓ Use /regent-log, /regent-blame, etc. in Claude

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  ✓ Initialization Complete!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Next steps:
  • Start a Claude Code session in this directory
  • Make some changes (the hook will capture them)
  • Run: rgt log
  • Run: rgt blame <file>
  • Use: /regent-log, /regent-blame in Claude

Repository: /path/to/project/.regent
```
Colors:
- "re_gent" in purple (brand moments)
- Dividers (━━━) dimmed
- Step headers ("Step 1/3:") bold
- Checkmarks (✓) green
- Warnings (⚠) amber
- "[Y/n]" in prompts dimmed
- "Repository:" label in blue
- Body text plain

---

## Language & Tone

### Voice

**We sound like**: A senior engineer pairing with you. Helpful, not condescending. Direct, not terse.

**We do not sound like**: 
- Corporate help desk ("We apologize for the inconvenience")
- Startup hype machine ("Revolutionizing your workflow!")
- Snarky CLI tools (looking at you, `sl` and `fuck`)

### Writing Style

**Error messages**:
```
# Good
Error: Session a1b2c3d4 not found.

Try:
  rgt sessions          List all sessions
  rgt log --all         Show steps across all sessions

# Bad
Error: Session does not exist. Please check your input and try again.

# Also bad
💀 RIP that session, chief. Try `rgt sessions`?
```

**Help text**:
- Imperative mood: "Show step history" not "Shows step history"
- One-line summaries, expanded detail in `--help`
- Examples over abstract descriptions

**Changelog / release notes**:
- Start with "Added", "Fixed", "Changed", "Removed"
- Link to issues/PRs
- "Breaking:" prefix for breaking changes
- Emoji are fine here (🎉 for big features, 🐛 for bug fixes, ⚠️ for breaking changes)

### Terminology

**Consistent vocabulary** (these are load-bearing terms):

| Use this | Not this |
|----------|----------|
| Step | Commit, snapshot, checkpoint |
| Session | Run, instance, process |
| Transcript | Conversation, chat log, messages |
| Cause | Trigger, reason, source |
| Tree | Snapshot, filesystem, directory |
| Blob | File, object, content |
| Ref | Pointer, branch, head |
| Rewind | Reset, undo, rollback |

**Capitalization**:
- re_gent (brand name, always lowercase with underscore)
- `rgt` (command, always lowercase monospace)
- Step, Tree, Blob (capitalized when referring to the data structure, lowercase when generic)

---

## CLI Command Style

### Naming

- **Short, common operations**: `log`, `status`, `init`, `blame`, `show`
- **Longer, less frequent**: `sessions`, `rewind`, `reindex`
- **Never**: `initialize`, `display-history`, `show-status`

### Flag conventions

- Single-letter short flags for common usage: `-n`, `-s`, `-a`
- Full words for long flags: `--session`, `--all`, `--verbose`
- **No abbreviations** in long flags: `--session` not `--sess`
- Boolean flags default to false: `--verbose` (not `--no-verbose`)

### Subcommands

Flat structure, avoid nesting:
```
# Good
rgt session list
rgt session new

# Bad
rgt session --list
rgt sessions list-all
```

### Output formats

**Default**: Human-readable, colored, formatted for TTY

**Scripting**: JSON via `--json`, one record per line (NDJSON for streams)

**Porcelain vs plumbing**: 
- Porcelain commands (default) can change format between versions
- Plumbing commands (`cat`, `hash-object`) have stable output for scripts

---

## Documentation Style

### README
- Lead with *what* and *why* before *how*
- Animated GIF demo in the first screenful
- Installation before usage
- Link to detailed docs, don't inline them

### CLAUDE.md
- Source of truth for architecture
- Vocabulary section is load-bearing — keep it updated
- Open questions are okay; document what we *don't* know

### Code comments
- **Rare by default** — code should be self-documenting
- When present: *why*, never *what*
- Format: `// Why: <reason>` or `// SAFETY: <invariant>`

### Commit messages
```
# Good
feat: add --session filter to rgt log

Allows filtering by session ID. Refs #42.

# Bad
updated log command
```

Follow [Conventional Commits](https://www.conventionalcommits.org/):
- `feat:` new feature
- `fix:` bug fix
- `docs:` documentation
- `test:` test changes
- `chore:` tooling, deps

---

## Asset Checklist

When adding/updating any user-facing asset:

- [ ] Follows color palette (if using color)
- [ ] Works in both dark and light terminals (if CLI)
- [ ] Tested with `NO_COLOR=1` (if CLI)
- [ ] Uses consistent terminology from this doc
- [ ] Tone matches examples (helpful senior engineer, not corporate/snarky)
- [ ] File paths, hashes, data are color-free for pipeability (if CLI output)
- [ ] Respects the logo usage rules (crown emoji not in CLI output)

---

## Evolution

This document is canonical until it isn't. If you find the guidelines blocking good work:

1. Propose the change in an issue
2. Link to the specific guideline that's the problem
3. Show the before/after

Brand consistency is important, but not more important than building a tool people love.

---

## Examples in the Wild

**Excellent CLI experiences to learn from**:
- `gh` — GitHub's CLI, great color usage and help text
- `ripgrep` — fast, respects `NO_COLOR`, excellent error messages
- `exa` — modern `ls`, beautiful color palette
- `delta` — git diff tool, shows how to do syntax highlighting right
- `bat` — `cat` clone, great theme support

**What we're NOT**:
- `cowsay` — too gimmicky
- `lolcat` — color for color's sake
- `sl` — joke commands (we're a serious tool with personality, not a toy)

---

**Last updated**: 2026-04-30  
**Maintained by**: @shayliv (brand czar 👑)

Questions? Open an issue tagged `brand`.
