---
description: Rewind workspace and conversation to a previous re_gent step. Non-destructive with automatic backup. Use when recovering from mistakes or exploring alternative paths.
allowed-tools: Bash(rgt rewind *)
argument-hint: "<step-hash>"
disable-model-invocation: true
---

⚠️ **Warning**: This will restore files and conversation state to the specified step.

Automatic backup is created at `.regent/backups/` before rewinding.

Rewind to a step:
```bash
rgt rewind $ARGUMENTS
```

The step hash can be shortened (first 7+ characters).

After rewinding, the current conversation transcript will be saved and the workspace files will match the target step's state.
