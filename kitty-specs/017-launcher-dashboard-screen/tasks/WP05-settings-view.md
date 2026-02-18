---
work_package_id: WP05
title: Settings View
lane: done
dependencies:
- WP01
- WP03
subtasks:
- 'Settings overlay/view accessible from launcher only'
- 'Display 4 agent roles (planner, coder, reviewer, release) with current model + reasoning'
- 'Default task source selector (spec-kitty, gsd, yolo)'
- 'Navigate between roles with up/down'
- 'Edit model name (text input) and reasoning level cycling'
- 'Save on exit (Esc saves and returns to launcher)'
- 'Wire config into worker spawn to look up role config'
- 'Handle missing config gracefully (create with defaults)'
- 'Tests for display settings, edit + save round-trip, defaults on missing config'
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

# Work Package Prompt: WP05 - Settings View

## Mission

Implement launcher `s` action as a settings view for per-role agent model/reasoning
config and default task source, persisted to `.kasmos/config.toml`.

## Scope
### Files to Create / Modify

```text
internal/tui/settings.go
internal/tui/model.go
internal/tui/update.go
internal/tui/commands.go
internal/worker/backend.go
```

### Technical References

- `internal/config/config.go`
- `internal/tui/overlays.go`
- `internal/worker/backend.go`
- `kitty-specs/017-launcher-dashboard-screen/plan.md`

## Implementation

Add a launcher-only settings overlay and wire config into spawn behavior.

Requirements:
- Add settings view state + rendering (`internal/tui/settings.go`)
- Show planner/coder/reviewer/release with current `model` and `reasoning`
- Add default task source selector (`spec-kitty`, `gsd`, `yolo`)
- Up/down navigation across editable rows
- Model editing via text input
- Reasoning cycle values: `default`, `low`, `medium`, `high`
- Esc saves config and returns to launcher
- Spawn commands read role config and pass model/reasoning to backend spawn
- Missing config file is handled by creating defaults
- Update `SpawnConfig` in `internal/worker/backend.go` if required

## Verification

- `go test ./internal/tui -run Settings`
- `go test ./internal/config -run Test`
- `go test ./internal/worker -run Spawn`
- `go test ./...`
- Manual check: edit values, esc to save, restart app, confirm values persist
