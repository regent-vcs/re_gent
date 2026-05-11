---
description: View the re_gent activity log for the default or selected session. The default view shows the conversation timeline and tool calls; file summaries are available with file flags.
allowed-tools: Bash(rgt log *)
argument-hint: "[session-id] [flags]"
---

Display the re_gent activity log showing captured steps, conversation context, and tool calls.

By default, `rgt log` shows the conversation timeline for the most recent session with captured steps. Use `--files-only` for file-change summaries.

Run the log command:
```bash
rgt log $ARGUMENTS
```

## Common usage

Show recent conversation:
```bash
rgt log
```

Show only conversation:
```bash
rgt log --conversation-only
```

Show only file changes:
```bash
rgt log --files-only
```

Show step lineage as graph (like git log --graph):
```bash
rgt log --graph
```

Show more history:
```bash
rgt log --limit 50
```

Show specific session:
```bash
rgt log <session-id>
```
