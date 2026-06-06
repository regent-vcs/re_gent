#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
WORKDIR="${1:-$SCRIPT_DIR/workspace}"

if ! command -v rgt >/dev/null 2>&1; then
  echo "error: rgt is not installed or is not on PATH" >&2
  echo "install it first, then rerun this script" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 is required for this example" >&2
  exit 1
fi

rm -rf "$WORKDIR"
mkdir -p "$WORKDIR"

(
  cd "$WORKDIR"
  rgt init --skip-hook --skip-skills --agent codex >/dev/null
)

EXAMPLE_DIR="$SCRIPT_DIR" WORKDIR="$WORKDIR" python3 - <<'PY'
import json
import os
import subprocess
import sys
from pathlib import Path


example_dir = Path(os.environ["EXAMPLE_DIR"])
workdir = Path(os.environ["WORKDIR"])
session_id = "bad-refactor-demo"


def send_hook(payload: dict) -> None:
    subprocess.run(
        ["rgt", "codex-hook"],
        cwd=workdir,
        input=json.dumps(payload).encode("utf-8"),
        check=True,
    )


def record_write_turn(
    *,
    turn_id: str,
    prompt: str,
    source_name: str,
    target_name: str,
    tool_name: str,
    tool_use_id: str,
    assistant_message: str,
) -> None:
    source = example_dir / source_name
    target = workdir / target_name
    content = source.read_text(encoding="utf-8")

    send_hook(
        {
            "hook_event_name": "user-prompt-submit",
            "session_id": session_id,
            "turn_id": turn_id,
            "cwd": str(workdir),
            "prompt": prompt,
        }
    )

    target.write_text(content, encoding="utf-8")

    send_hook(
        {
            "hook_event_name": "PostToolUse",
            "session_id": session_id,
            "turn_id": turn_id,
            "cwd": str(workdir),
            "tool_name": tool_name,
            "tool_use_id": tool_use_id,
            "tool_input": {
                "file_path": target_name,
                "content": content,
            },
            "tool_response": {
                "ok": True,
                "message": f"{target_name} updated",
            },
        }
    )

    send_hook(
        {
            "hook_event_name": "stop",
            "session_id": session_id,
            "turn_id": turn_id,
            "cwd": str(workdir),
            "last_assistant_message": assistant_message,
        }
    )


send_hook(
    {
        "hook_event_name": "session_start",
        "session_id": session_id,
        "cwd": str(workdir),
        "model": "example-agent",
    }
)

record_write_turn(
    turn_id="turn-1",
    prompt="Create the subscription billing calculator for enterprise invoices.",
    source_name="app_before.py",
    target_name="app.py",
    tool_name="Write",
    tool_use_id="tool-write-billing-app",
    assistant_message="Built the initial billing calculator.",
)

record_write_turn(
    turn_id="turn-2",
    prompt="Add a regression test for enterprise accounts with suspended users.",
    source_name="regression_test.py",
    target_name="test_app.py",
    tool_name="Write",
    tool_use_id="tool-write-regression-test",
    assistant_message="Added a unittest regression test for suspended seats.",
)

passing = subprocess.run(
    [sys.executable, "-m", "unittest", "-q"],
    cwd=workdir,
    text=True,
    capture_output=True,
)
if passing.returncode != 0:
    print("The working version failed before the refactor:", file=sys.stderr)
    print(passing.stdout, file=sys.stderr)
    print(passing.stderr, file=sys.stderr)
    sys.exit(passing.returncode)

record_write_turn(
    turn_id="turn-3",
    prompt=(
        "Refactor the billing calculator into smaller helper functions without "
        "changing invoice behavior."
    ),
    source_name="app.py",
    target_name="app.py",
    tool_name="Edit",
    tool_use_id="tool-refactor-billing-app",
    assistant_message=(
        "Refactored invoice calculation into helper functions while preserving behavior."
    ),
)
PY

BAD_LINE="$(grep -n 'billable_seats = account\["user_count"\]' "$WORKDIR/app.py" | cut -d: -f1)"

cat <<EOF
Bad-refactor example ready.

Workspace: $WORKDIR

Try it:
  cd "$WORKDIR"
  python3 -m unittest -q
  rgt log --oneline
  rgt blame app.py:$BAD_LINE
  REFACTOR_STEP=\$(rgt log --oneline | awk '/Edit app.py/ { print \$1; exit }')
  rgt show "\$REFACTOR_STEP"
EOF
