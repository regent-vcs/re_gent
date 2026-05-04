# Test File for re_gent Blame Extension

This file was created to test the VS Code extension.

## Purpose

The re_gent extension shows inline blame for files that have been modified through Claude Code.

This line was added by Claude Code via the Write tool.
Another line here.
And a third line for good measure.

When you open this file in Cursor with the extension installed, you should see:
- Inline annotations on each line showing the step hash, tool name, and time ago
- Hover tooltips with full step metadata
- The step in the Timeline view

## Testing

1. Open this file in Cursor
2. Look for inline blame: `a1b2c3d • Write • 1m ago`
3. Hover over any line to see full details
4. Check the Timeline view in the SCM panel

