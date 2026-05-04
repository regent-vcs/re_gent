---
name: log
description: Show Regent step history for the current session
---

# Log Skill

Shows the Regent version control log for the current Claude Code session, displaying the full history of tool calls, file changes, and conversation.

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
- **Full conversation**: User prompts, assistant responses, and tool invocations
- **File changes**: Which files were modified with line counts (+additions, -deletions)
- **Timestamps**: When each step occurred
- **Tool details**: Which tools were used and their key parameters

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

This skill runs `rgt log` with the current session ID. The session ID is automatically detected from the most recent Regent session in the `.regent/` directory.

If you need to see a different session's log, you can specify the session ID explicitly:
```
/log <session-id>
```

You can list all sessions with:
```
rgt sessions
```

## Output

The log shows steps in reverse-chronological order (newest first), with each step including:

1. **Conversation context** (if available):
   - User: Your prompt
   - Assistant: My response  
   - Tools: Tool calls with parameters

2. **Tool execution**:
   - Tool name (Read, Write, Edit, Bash, etc.)
   - Step hash (first 8 chars)
   - Timestamp
   - Duration

3. **File changes** (if any):
   - File paths modified
   - Line statistics (+/-) 
   - Binary file indicators

## Tips

- Use `/log --graph` to visualize how your work branched when working in multiple concurrent sessions
- Use `/log --conversation-only` to review what you asked me to do
- Use `/log --files-only` to see a clean summary of all file changes
- Combine with `--limit` to see more history: `/log --graph --limit 50`