---
description: Show which re_gent step last modified each line of a file. Use when investigating file provenance, understanding change history, or debugging.
allowed-tools: Bash(rgt blame *)
argument-hint: "<file> [line]"
---

Display per-line provenance showing which step introduced or last modified each line.

Run blame on a file:
```bash
rgt blame $ARGUMENTS
```

## Examples

Blame entire file:
```bash
rgt blame src/main.go
```

Blame specific line:
```bash
rgt blame src/main.go:42
```