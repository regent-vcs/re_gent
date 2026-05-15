# Codex Desktop Integration

This document describes the current Codex Desktop integration in `re_gent`.

## Status

The current integration is a local sidecar adapter.

- `re_gent` does not embed into Codex Desktop itself.
- `re_gent` does not depend on a public Codex hook API.
- The adapter reads local Codex rollout JSONL files from `~/.codex/sessions/**/*.jsonl`.
- The adapter is currently best described as a PoC-quality integration path with production-friendly guardrails.

This is the lowest-risk way to support Codex today because Codex session JSONL already contains enough local event history to reconstruct useful agent steps.

## What It Captures

The adapter currently parses these Codex event families:

- `session_meta`
- `turn_context`
- `event_msg.task_started`
- `event_msg.task_complete`
- `event_msg.patch_apply_end`
- `event_msg.user_message`
- `event_msg.agent_message`
- `response_item.message`
- `response_item.function_call`
- `response_item.function_call_output`
- `response_item.custom_tool_call`
- `response_item.custom_tool_call_output`

It intentionally ignores or de-prioritizes:

- `reasoning`
- token-count and rate-limit events
- encrypted or non-user-facing reasoning payloads

## Data Model

Each Codex session is normalized into a `re_gent` session with this ID format:

```text
codex:<session-id>
```

This keeps Codex sessions separate from Claude Code and other future adapters.

Within `re_gent`, the adapter imports:

- one Codex session into one logical `re_gent` session branch
- one completed Codex turn into at most one `re_gent` step
- all tool calls from that turn into ordered `Step.Causes`

Turn closure is defined by `task_complete`.

## Project Matching

The adapter aggregates Codex history by project root.

- The Codex session `cwd` must match the target `--project` path after path normalization.
- Matching is exact for V1.
- The adapter does not currently follow extra writable directories outside the main project root.

If two different rollout files have the same normalized `cwd`, they can both be imported into the same `.regent/` repository as separate `codex:<session>` histories.

## Commands

Initialize `re_gent` in the target project first:

```bash
rgt init
```

Then use one of these Codex adapter commands.

### Historical Import

```bash
rgt codex import --project /absolute/path/to/project
```

Optional flags:

- `--codex-home <path>`: override the default Codex home directory
- `--changes-only=false`: record completed turns even if the tree hash did not change

Typical examples:

```bash
rgt codex import --project .
rgt codex import --project /repo/app --codex-home ~/.codex
```

On Windows:

```powershell
rgt codex import --project C:\work\repo --codex-home $env:USERPROFILE\.codex
```

If you are building `rgt` from source locally on Windows before using the Codex adapter, load the repo's PowerShell dev environment first:

```powershell
. .\scripts\dev-env.ps1
go build -o .\bin\rgt.exe .\cmd\rgt
.\bin\rgt.exe codex import --project C:\work\repo --codex-home $env:USERPROFILE\.codex
```

### Live Watch Mode

```bash
rgt codex watch --project /absolute/path/to/project --poll 2s
```

Optional flags:

- `--codex-home <path>`
- `--poll <duration>`
- `--changes-only=false`

Typical examples:

```bash
rgt codex watch --project .
rgt codex watch --project /repo/app --poll 5s
```

`watch` is the preferred mode when Codex is actively editing the workspace.

## Import Semantics

By default, the adapter runs in `changes-only=true` mode.

That means:

- a completed turn only produces a step if the resulting tree differs from the previous step in that session
- read-only turns are skipped
- the tool call history is still preserved for steps that are written

Message extraction rules:

- prefer `response_item.message`
- fall back to `event_msg.user_message` and `event_msg.agent_message` only when needed

Tool call handling rules:

- `apply_patch` is preserved as a tool cause
- `custom_tool_call_output` and `function_call_output` are preserved as tool results
- `patch_apply_end` metadata is merged into the stored result blob when available

## Replay vs Snapshot Behavior

The adapter treats historical import and live watch differently.

### Historical Import

Historical import tries to reconstruct workspace history from rollout data alone.

Currently:

- `apply_patch` turns are replayed from patch hunks
- read-only turns are skipped
- non-replayable mutating turns are not guessed

If a historical turn changed the workspace in a way that cannot be replayed from the JSONL history, the adapter:

- skips that turn
- marks the session baseline as unknown for later historical turns
- skips later replayable turns from that same session during `import`
- emits a warning telling you to use `rgt codex watch` for live capture

This is deliberate. The adapter fails safely instead of inventing file history.

### Live Watch

`watch` can still capture turns that are not historically replayable.

When replay is not possible in watch mode, the adapter snapshots the live workspace tree at turn completion. This makes watch mode more forgiving for shell-based edits and other mutations that do not provide a replayable patch stream.

## Runtime State

The adapter stores checkpoints here:

```text
.regent/adapters/codex/state.json
```

This file stores runtime progress such as:

- source rollout file path
- byte offset
- last event timestamp
- logical session ID

This is a checkpoint file, not the source of truth. The source of truth is still the Codex rollout JSONL.

## Concurrency Behavior

The adapter supports multiple Codex sessions for the same project, but V1 keeps attribution conservative.

- each Codex session is recorded separately
- no automatic merge or conflict resolution is attempted
- `watch` emits a warning when multiple active Codex sessions overlap on the same project

That warning means step attribution may become approximate if multiple sessions edit the same workspace during overlapping turn windows.

## Limitations

Current known limitations:

- The adapter depends on Codex rollout JSONL, which appears to be an internal persistence format rather than a stable public API.
- Exact project matching is based on normalized `cwd`.
- V1 only tracks the main project root, not arbitrary external writable directories.
- Historical import can only replay changes it can prove from rollout history.
- Concurrent sessions on the same project can make exact per-turn attribution fuzzy.
- The adapter reads JSONL only. It does not depend on Codex SQLite internals.

## Why Sidecar Instead of Native Codex Hooks

At the moment, the Codex integration is intentionally implemented as a sidecar because:

- the local session JSONL already contains enough structure for useful provenance capture
- a sidecar keeps Codex Desktop itself untouched
- the risk of breakage is isolated inside `internal/adapter/codex`

If Codex exposes a stable public hook or event API in the future, `re_gent` can add a more native integration path later.

## Troubleshooting

### No Codex sessions are imported

Check:

- the project was initialized with `rgt init`
- the Codex session `cwd` exactly matches the target project path after normalization
- you pointed `--codex-home` at the correct Codex home directory

### Import says the session baseline is unknown

That means `import` encountered a mutating turn that could not be replayed from history.

Use:

```bash
rgt codex watch --project /absolute/path/to/project
```

### I see a warning about multiple active Codex sessions

That warning is informational. `re_gent` still records each session separately, but project-level attribution may be less precise if the sessions overlap in time and both modify the same workspace.

### Where do the imported sessions appear?

Use:

```bash
rgt sessions
rgt log codex:<session-id>
rgt show <step-hash>
```

Imported sessions are indexed with origin `codex`.

## Implementation Notes

Relevant implementation entry points:

- `internal/cli/codex.go`
- `internal/adapter/codex/`
- `internal/store/session_refs.go`

For command-level behavior, also see:

- `README.md`
- `docs/FAQ.md`
- `scripts/dev-env.ps1`
