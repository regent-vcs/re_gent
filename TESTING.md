# re_gent Testing Guide

Quick checks for the `rgt` CLI and agent hook integrations.

## Prerequisites

```bash
go build -o rgt ./cmd/rgt
```

Use the built binary in examples below:

```bash
export RGT=/path/to/re_gent/rgt
```

## Basic CLI

```bash
tmp=$(mktemp -d)
cd "$tmp"
"$RGT" init --agent both

"$RGT" status
"$RGT" sessions
```

Expected results:

- `.regent/` exists.
- `.claude/settings.json` contains `UserPromptSubmit`, `Stop`, and `PostToolBatch`.
- `.codex/config.toml` contains `SessionStart`, `UserPromptSubmit`, `PostToolUse`, and `Stop`.
- `rgt sessions` reports no sessions until an agent hook fires.

Codex may ask you to trust the project and hook commands the first time it loads the project-local config.

## Manual Claude Turn

This exercises the current Claude per-turn flow without starting Claude Code.

```bash
cd "$tmp"
echo 'hello' > hello.txt

printf '{"session_id":"claude-manual","cwd":"%s","prompt":"create hello.txt"}' "$PWD" \
  | "$RGT" message-hook user

cat > /tmp/claude-tool-batch.json <<EOF
{
  "session_id": "claude-manual",
  "cwd": "$PWD",
  "tool_calls": [
    {
      "tool_name": "Write",
      "tool_use_id": "tool_1",
      "tool_input": {"file_path":"hello.txt","content":"hello"},
      "tool_response": "ok"
    }
  ]
}
EOF
"$RGT" tool-batch-hook < /tmp/claude-tool-batch.json

printf '{"session_id":"claude-manual","cwd":"%s","last_assistant_message":"done"}' "$PWD" \
  | "$RGT" message-hook assistant

"$RGT" log --session claude_code:claude-manual
"$RGT" sessions
```

Expected result: one step is created for the turn, with `origin: claude_code`.

## Manual Codex Turn

This exercises the Codex hook adapter without starting Codex.

```bash
cd "$tmp"
echo 'codex' > codex.txt

printf '{"hook_event_name":"SessionStart","session_id":"codex-manual","cwd":"%s","model":"gpt-5.5"}' "$PWD" \
  | "$RGT" codex-hook

printf '{"hook_event_name":"UserPromptSubmit","session_id":"codex-manual","turn_id":"turn-1","cwd":"%s","prompt":"create codex.txt"}' "$PWD" \
  | "$RGT" codex-hook

cat > /tmp/codex-post-tool.json <<EOF
{
  "hook_event_name": "PostToolUse",
  "session_id": "codex-manual",
  "turn_id": "turn-1",
  "cwd": "$PWD",
  "tool_name": "Bash",
  "tool_use_id": "call_1",
  "tool_input": {"command":"printf codex > codex.txt"},
  "tool_response": "ok"
}
EOF
"$RGT" codex-hook < /tmp/codex-post-tool.json

printf '{"hook_event_name":"Stop","session_id":"codex-manual","turn_id":"turn-1","cwd":"%s","last_assistant_message":"done"}' "$PWD" \
  | "$RGT" codex-hook

"$RGT" log --session codex_cli:codex-manual --json
HASH=$("$RGT" log --session codex_cli:codex-manual --oneline | awk 'NR==1 {print $1}')
"$RGT" show "$HASH"
```

Expected result: one step is created for the turn, with `origin: codex_cli` and `turn_id: turn-1`.

## No-Tool Turn

```bash
printf '{"hook_event_name":"UserPromptSubmit","session_id":"codex-manual","turn_id":"turn-2","cwd":"%s","prompt":"say ok"}' "$PWD" \
  | "$RGT" codex-hook
printf '{"hook_event_name":"Stop","session_id":"codex-manual","turn_id":"turn-2","cwd":"%s","last_assistant_message":"ok"}' "$PWD" \
  | "$RGT" codex-hook

"$RGT" log --session codex_cli:codex-manual
```

Expected result: no new step is created, and the no-tool messages do not attach to later tool-using turns.

## Full Verification

```bash
go test ./...
```

Before opening a PR, also run any available lint or race checks used by the project.
