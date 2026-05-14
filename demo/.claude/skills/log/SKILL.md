---
description: View the re_gent activity log for the default or selected session. The default view shows the conversation timeline and tool calls; file summaries are available with file flags.
allowed-tools: Bash(rgt log *)
argument-hint: "[session-id] [flags]"
---

Display the re_gent activity log.

By default, `rgt log` shows the conversation timeline for the most recent session with captured steps. Use `--files-only` for file-change summaries.

Run:
```bash
rgt log $ARGUMENTS
```

Common usage:
```bash
rgt log
rgt log --conversation-only
rgt log --files-only
rgt log --graph
rgt log --limit 50
```