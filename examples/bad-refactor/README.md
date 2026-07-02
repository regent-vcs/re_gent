# Debugging a Bad Refactor

This example shows how to use **re_gent** when an AI agent refactors working code and quietly breaks a business rule.

The app is a small subscription billing calculator. It handles enterprise invoices with base fees, seat overages, API overages, discounts, contract minimums, and tax. The original version only bills active seats. The bad refactor accidentally bills every user, including suspended users.

You will use:

- `rgt log` to find the suspicious refactor step
- `rgt blame` to see which step last changed the broken line
- `rgt show` to inspect the prompt and tool context for that step

## Setup

From the repository root:

```bash
./examples/bad-refactor/setup.sh
cd examples/bad-refactor/workspace
```

The setup script creates a throwaway workspace, initializes `.regent/`, and pre-populates three captured steps:

1. An agent creates a working billing calculator.
2. The agent adds a regression test for suspended users.
3. The agent refactors the billing code and introduces the bug.

It should complete in a few seconds.

If you want to run it containerized, run this from the repository root:
```bash
docker build -t rgt-demo -f ./examples/bad-refactor/Dockerfile . \
  && docker run -it rgt-demo
```

## Reproduce The Breakage

Run the regression test:

```bash
python3 -m unittest -q
```

The test fails because suspended users are now included in seat overage billing. The important failure is:

```text
AssertionError: 42000 != 28000
```

The expected extra-seat charge is `$280.00`: 18 active seats minus 10 included seats, multiplied by `$35.00`.

The broken refactor charges `$420.00`: 22 total users minus 10 included seats, multiplied by `$35.00`.

## Find The Refactor Step

Start with the session history:

```bash
rgt log --oneline
```

You should see steps similar to:

```text
<hash> Edit app.py
<hash> Write test_app.py
<hash> Write app.py
```

The newest step is already suspicious: the only code change after the passing regression test was an `Edit app.py` refactor.

For a little more context, run:

```bash
rgt log --files-only
```

That shows which files changed in each step.

## Blame The Broken Line

Find the line that chooses the billable seat count:

```bash
grep -n 'billable_seats' app.py
```

Then blame that line:

```bash
BAD_LINE=$(grep -n 'billable_seats = account\["user_count"\]' app.py | cut -d: -f1)
rgt blame app.py:$BAD_LINE
```

The output points at the step that last wrote the line:

```text
<hash> 2026-... Edit       | <line> |     billable_seats = account["user_count"]
```

That tells you the line came from the refactor step, not from the original working implementation.

## Show The Full Context

Capture the refactor step hash from the log:

```bash
REFACTOR_STEP=$(rgt log --oneline | awk '/Edit app.py/ { print $1; exit }')
rgt show "$REFACTOR_STEP"
```

`rgt show` displays the recorded prompt, tool input, tool result, and assistant response. In this example, the prompt says:

```text
Refactor the billing calculator into smaller helper functions without changing invoice behavior.
```

That is the useful debugging clue. The user asked for a behavior-preserving refactor, but the tool input shows the new helper using:

```python
billable_seats = account["user_count"]
```

The original code used `active_seats`, because suspended users should not be billed.

## Fix

Change the broken line in `app.py`:

```python
billable_seats = account["active_seats"]
```

Then rerun:

```bash
python3 -m unittest -q
```

The regression test should pass again.

## What You Learned

When an AI refactor breaks behavior, you do not have to guess which turn caused it.

- `rgt log` narrows the search to the suspicious step.
- `rgt blame` connects a specific broken line to the step that wrote it.
- `rgt show` explains why the step happened by showing the prompt, tool input, and conversation context.

Git can tell you what changed between commits. re_gent tells you which agent step and prompt produced the line you are debugging.
