---
name: log
description: Show re_gent step history for the default or selected session
---

# Log Skill

Shows the re_gent version control log for the default or selected session. The default view is a conversation timeline with captured tool calls; use file flags when you need file summaries.

## Usage

```
/log
```

Optional flags:
- `/log --conversation-only` - Show only conversation (hide files)
- `/log --files-only` - Show only files (hide conversation)  
- `/log --graph` - Show step lineage as ASCII graph
- `/log --limit 50` - Show more steps (default 20)
- `/log <session-id>` - Show log for specific session

## What it shows

By default, the log displays:
- **Conversation timeline**: User prompts, assistant responses, and tool invocations
- **Timestamps**: When each step occurred
- **Tool details**: Which tools were used and their key parameters

Use `--files-only` when you want file paths and line statistics instead of the conversation timeline.

## Examples

Show recent steps with default view:
```
/log
```

Show only the conversation thread:
```
/log --conversation-only
```

Show only file changes:
```
/log --files-only
```

Show step lineage as a graph (like git log --graph):
```
/log --graph
```

Show more history:
```
/log --limit 100
```

## Implementation

This skill runs `rgt log`. Without an explicit session, re_gent uses the most recent session with captured steps in the `.regent/` directory.

If you need to see a different session's log, you can specify the session ID explicitly:
```
/log <session-id>
```

You can list all sessions with:
```
rgt sessions
```

## Output

The default log shows conversation turns in chat order for the selected session. File-focused output is available with `--files-only`.

1. **Conversation context** (if available):
   - User: Your prompt
   - Assistant: My response  
   - Tools: Tool calls with parameters

2. **Tool execution metadata**:
   - Tool name (Read, Write, Edit, Bash, etc.)
   - Step hash (first 8 chars)
   - Timestamp
   - Duration

3. **File changes** (with `--files-only`):
   - File paths modified
   - Line statistics (+/-) 
   - Binary file indicators

## Tips

- Use `/log --graph` to visualize how your work branched when working in multiple concurrent sessions
- Use `/log --conversation-only` to review what you asked me to do
- Use `/log --files-only` to see a clean summary of all file changes
- Combine with `--limit` to see more history: `/log --graph --limit 50`
