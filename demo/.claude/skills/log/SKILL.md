---
description: View re_gent activity log with full conversation, file changes, and step history. Shows what you asked, how I responded, which tools were used, and what files changed. Use when reviewing session history or understanding what happened in previous steps.
allowed-tools: Bash(rgt log *)
argument-hint: "[session-id] [flags]"
---

Display the re_gent activity log showing captured steps, full conversation (user + assistant + tools), and file changes.

By default shows both conversation and file changes for the most recent session.

Run the log command:
```bash
rgt log $ARGUMENTS
```

## Common usage

Show recent steps (conversation + files):
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