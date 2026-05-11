---
description: View re_gent activity log with conversation, file changes, and step history. Use when reviewing session history or understanding what happened in previous steps.
allowed-tools: Bash(rgt log *)
argument-hint: "[session-id] [flags]"
---

Display the re_gent activity log.

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
