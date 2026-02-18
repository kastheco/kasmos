---
work_package_id: WP04
title: Session Restore Picker
lane: done
dependencies:
- WP03
subtasks:
- 'Restore picker overlay with selection and load'
- 'Show last active session from .kasmos/session.json at top (if exists and PID dead)'
- 'List archived sessions from .kasmos/sessions/ below, reverse chronological'
- 'Each entry shows session ID, timestamp, worker count, task source type'
- 'Up/down navigation, Enter to select and load'
- 'On select load session, restore workers, transition to dashboard'
- 'Handle no-sessions gracefully (message + stay on launcher)'
- 'Handle corrupt session gracefully (error message, skip entry)'
- 'Esc returns to launcher'
- 'Tests for render with sessions, render empty, selection + load'
phase: Wave 3 - Extended Features
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-18T00:00:00Z'
  lane: planned
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP04 - Session Restore Picker

## Mission

Implement launcher `r` action as a restore picker that highlights last active
session first, shows archived sessions below, and loads the selected session.

## Scope
### Files to Create / Modify

```text
internal/tui/model.go
internal/tui/update.go
internal/tui/launcher.go or internal/tui/restore.go
internal/tui/commands.go
```

### Technical References

- `internal/tui/history.go`
- `internal/persist/session.go`
- `internal/persist/schema.go`
- `kitty-specs/017-launcher-dashboard-screen/plan.md`

## Implementation

Build a restore picker overlay with selection and loading behavior.

Requirements:
- Add restore picker state fields to `Model`
- Show active `.kasmos/session.json` entry first when PID is dead
- Show archived `.kasmos/sessions/` entries below in reverse chronological order
- Render per-entry metadata: session id, timestamp, worker count, task source
- Key handling: up/down to move, enter to restore, esc to return launcher
- On successful selection: load session, restore workers, set `showLauncher=false`, show dashboard
- No sessions: show clear message and remain in launcher context
- Corrupt session: show error, skip bad entry, continue running

## Verification

- `go test ./internal/tui -run Restore`
- `go test ./internal/persist -run Session`
- `go test ./...`
- Manual check: launcher `r` opens picker, esc returns to launcher, enter loads valid session
