# Regent Testing Guide

Quick guide to test the `rgt` CLI and hook integration.

## Prerequisites

```bash
# Build the binary
cd /Users/shay/Projects/regent
go build -o rgt ./cmd/rgt
```

## Test 1: Basic CLI Commands

```bash
# Initialize a test project
cd /tmp
rm -rf regent-test
mkdir regent-test && cd regent-test

# Initialize with automatic hook configuration (press Y)
/Users/shay/Projects/regent/rgt init
# Prompt: Enable automatic tracking in Claude Code? [Y/n]: y
# Expected: "✓ Configured PostToolUse hook in .claude/settings.json"

# Verify hook was configured
cat .claude/settings.json
# Expected: {"hooks":{"PostToolUse":"rgt hook"}}

# Check status
/Users/shay/Projects/regent/rgt status
# Expected: "No sessions recorded yet."

# List sessions
/Users/shay/Projects/regent/rgt sessions
# Expected: "No sessions recorded yet."

# Try log
/Users/shay/Projects/regent/rgt log
# Expected: "No sessions found."
```

## Test 2: Manual Hook Test

Test the hook without Claude Code by piping a test payload:

```bash
cd /tmp/regent-test

# Create a test file
echo "hello from manual test" > test.txt

# Create test payload
cat > /tmp/hook-payload.json << 'EOF'
{
  "session_id": "manual-test-123",
  "tool_use_id": "tool_manual_1",
  "tool_name": "Write",
  "tool_input": {"file_path": "test.txt", "content": "hello from manual test"},
  "tool_response": {"success": true},
  "cwd": "/tmp/regent-test",
  "transcript_path": ""
}
EOF

# Run hook
/Users/shay/Projects/regent/rgt hook < /tmp/hook-payload.json

# Verify step was created
/Users/shay/Projects/regent/rgt log --session manual-test-123
# Expected: Shows 1 step with tool name "Write"

/Users/shay/Projects/regent/rgt sessions
# Expected: Shows session "manual-test-123"

# Inspect the step
HASH=$(/Users/shay/Projects/regent/rgt log --session manual-test-123 | grep -o '^[a-f0-9]\{8\}' | head -1)
/Users/shay/Projects/regent/rgt cat $HASH
# Expected: JSON showing step with parent, tree, cause, etc.
```

## Test 3: Multiple Steps Chain

```bash
cd /tmp/regent-test

# Create 3 steps manually
for i in 1 2 3; do
  echo "step $i" > file$i.txt
  
  cat > /tmp/payload$i.json << EOF
{
  "session_id": "chain-test",
  "tool_use_id": "tool_$i",
  "tool_name": "Write",
  "tool_input": {"file_path": "file$i.txt", "content": "step $i"},
  "tool_response": {"success": true},
  "cwd": "/tmp/regent-test",
  "transcript_path": ""
}
EOF
  
  /Users/shay/Projects/regent/rgt hook < /tmp/payload$i.json
  sleep 0.1  # Small delay for timestamp uniqueness
done

# Verify chain
/Users/shay/Projects/regent/rgt log --session chain-test
# Expected: Shows 3 steps with parent chain

# Verify parent relationships
/Users/shay/Projects/regent/rgt log --session chain-test | grep "parent:"
# Each step (except first) should have a parent
```

## Test 4: Enable Hook in Claude Code

**For the Regent project itself (dogfooding):**

1. Open `/Users/shay/Projects/regent/.claude/settings.json`

2. Add the hook configuration:
   ```json
   {
     "hooks": {
       "PostToolUse": "rgt hook"
     }
   }
   ```

3. Initialize regent in this project:
   ```bash
   cd /Users/shay/Projects/regent
   ./rgt init
   ```

4. Now this conversation should record steps automatically!

5. Verify it's working:
   ```bash
   cd /Users/shay/Projects/regent
   ./rgt log
   # Should show recent steps from this conversation
   
   ./rgt sessions
   # Should show current session
   ```

## Test 5: Real Claude Code Session

In a new project:

```bash
# Create test project
mkdir -p /tmp/test-claude-regent
cd /tmp/test-claude-regent

# Initialize regent (accept hook prompt with Y)
/Users/shay/Projects/regent/rgt init
# Prompt: Enable automatic tracking in Claude Code? [Y/n]: y

# That's it! Hook is already configured.

# Now start a Claude Code session in this directory
# Ask Claude to: "Create a file hello.txt with content 'Hello Regent!'"

# After Claude creates the file, check:
/Users/shay/Projects/regent/rgt log
# Should show a step with tool_name "Write"

/Users/shay/Projects/regent/rgt sessions
# Should show the current Claude session ID
```

## Debugging

**If steps aren't appearing:**

1. Check hook is configured:
   ```bash
   cat .claude/settings.json
   # Should contain: "PostToolUse": "rgt hook"
   ```

2. Check for errors:
   ```bash
   cat .regent/log/hook-error.log
   # Should be empty or show specific errors
   ```

3. Test hook manually:
   ```bash
   echo '{"session_id":"debug","tool_use_id":"t1","tool_name":"Test","tool_input":{},"tool_response":{},"cwd":"'$(pwd)'","transcript_path":""}' | /Users/shay/Projects/regent/rgt hook
   
   /Users/shay/Projects/regent/rgt log --session debug
   ```

4. Verify .regent exists:
   ```bash
   ls -la .regent/
   ```

**If hook seems to hang:**
- Add `-v` for verbose logging (future enhancement)
- Check for blocking operations
- Hook should complete in <100ms

## Expected Output

### Successful `rgt log`:
```
Session: claude-code-abc123 (3 steps)

a1b2c3d4  2026-04-30 12:30:15  Write
    tool_use_id: tool_use_abc123
    parent: 9f8e7d6c

9f8e7d6c  2026-04-30 12:30:10  Edit
    tool_use_id: tool_use_abc122
    parent: 5e4d3c2b

5e4d3c2b  2026-04-30 12:30:05  Bash
    tool_use_id: tool_use_abc121
    parent: 
```

### Successful `rgt sessions`:
```
Total sessions: 1

Session: claude-code-abc123
  Origin:     claude_code
  Started:    2026-04-30 12:30:00
  Last seen:  2026-04-30 12:30:15
  Head:       a1b2c3d4e5f6g7h8
```

### Successful `rgt cat <step-hash>`:
```json
{
  "parent": "9f8e7d6c5b4a3210",
  "tree": "fedcba9876543210",
  "cause": {
    "tool_use_id": "tool_use_abc123",
    "tool_name": "Write",
    "args_blob": "abcdef1234567890",
    "result_blob": "0987654321fedcba"
  },
  "session_id": "claude-code-abc123",
  "ts": 1714483815000000000
}
```

## Running Automated Tests

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run hook tests specifically
go test -v ./internal/hook

# Run integration tests
go test -v ./test
```
