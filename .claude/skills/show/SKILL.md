---
description: Show detailed context for a re_gent step including tool arguments, result, and conversation. Use when investigating what happened in a specific step.
allowed-tools: Bash(rgt show *)
argument-hint: "<step-hash>"
---

Display full details for a step including:
- Tool call (name, arguments)
- Tool result
- Conversation messages

Show step details:
```bash
rgt show $ARGUMENTS
```

The step hash can be shortened (first 7+ characters).
