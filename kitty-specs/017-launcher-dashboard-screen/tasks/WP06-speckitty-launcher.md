---
work_package_id: WP06
title: Spec-Kitty Launcher Actions
lane: done
dependencies:
- WP03
subtasks:
- 'f action transitions to dashboard and opens spec-kitty feature creation flow'
- 'p action with multiple features shows feature picker then starts plan flow'
- 'p action with one feature starts plan flow directly'
- 'p action with no features shows message suggesting f first'
- 'Handle missing spec-kitty binary gracefully (error message, stay on launcher)'
- 'Reuse existing newdialog.go flows where possible'
- 'Tests for f/p routing, feature picker, missing spec-kitty error'
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

# Work Package Prompt: WP06 - Spec-Kitty Launcher Actions

## Mission

Implement launcher `f` and `p` actions by reusing existing spec-kitty flows from
the new-task dialog path.

## Scope
### Files to Create / Modify

```text
internal/tui/update.go
internal/tui/newdialog.go
internal/tui/launcher.go
```

### Technical References

- `internal/tui/newdialog.go`
- `internal/task/speckitty.go`
- `kitty-specs/` directory layout
- `kitty-specs/017-launcher-dashboard-screen/plan.md`

## Implementation

Route launcher keys to existing spec-kitty creation/plan logic.

Requirements:
- `f`: hide launcher, transition to dashboard context, open feature creation flow
- `p`:
  - multiple features -> show feature picker, then launch plan flow
  - one feature -> start plan flow immediately
  - zero features -> show message to run `f` first
- Missing `spec-kitty` binary: show error and stay on launcher
- Reuse `NewFormModel` and existing new-dialog plumbing where possible
- Keep behavior consistent with existing spec-kitty integration and key handling

## Verification

- `go test ./internal/tui -run SpecKitty`
- `go test ./internal/task -run SpecKitty`
- `go test ./...`
- Manual check: launcher `f` and `p` follow expected branch logic for 0/1/many features
