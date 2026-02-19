---
description: Open the Spec Kitty dashboard in your browser.
---

> **Skill Required**: Before executing this command, load the **spec-kitty** skill (`.opencode/skills/spec-kitty/SKILL.md`). If a Skill loading tool is available, use it with name `spec-kitty`. Otherwise, read the skill file directly. The skill provides essential context about the spec-kitty workflow, CLI commands, worktree management, and the `spec-kitty agent` programmatic API.

## Dashboard Access

This command launches the Spec Kitty dashboard in your browser using the spec-kitty CLI.

## What to do

Simply run the `spec-kitty dashboard` command to:
- Start the dashboard if it's not already running
- Open it in your default web browser
- Display the dashboard URL

If you need to stop the dashboard, you can use `spec-kitty dashboard --kill`.

## Implementation

Execute the following terminal command:

```bash
spec-kitty dashboard
```

## Additional Options

- To specify a preferred port: `spec-kitty dashboard --port 8080`
- To stop the dashboard: `spec-kitty dashboard --kill`

## Success Criteria

- User sees the dashboard URL clearly displayed
- Browser opens automatically to the dashboard
- If browser doesn't open, user gets clear instructions
- Error messages are helpful and actionable
