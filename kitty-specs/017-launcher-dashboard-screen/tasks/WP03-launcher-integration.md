---
work_package_id: WP03
title: Launcher-Dashboard Integration
lane: planned
dependencies:
- WP01
- WP02
subtasks:
- 'Add showLauncher bool and config pointer to Model struct'
- 'NewModel() accepts config parameter, sets showLauncher based on CLI args'
- 'View() checks showLauncher first, renders launcher or dashboard'
- 'Launcher key handling in Update() dispatches menu actions'
- 'n sets showLauncher=false and opens spawn dialog (yolo mode)'
- 'f sets showLauncher=false and starts spec-kitty feature creation'
- 'p sets showLauncher=false and starts spec-kitty plan flow'
- 'h opens history overlay (Esc returns to launcher)'
- 'r opens restore picker (WP04 implements, stub for now)'
- 's opens settings view (WP05 implements, stub for now)'
- 'q quits'
- 'CLI bypass when args provided or --attach or --daemon'
- 'Esc from history/restore/settings returns to launcher not dashboard'
- 'main.go loads config, detects bare invocation, passes to NewModel'
- 'Tests for launcher shown/hidden, menu key routing'
phase: Wave 2 - Integration
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

# Work Package Prompt: WP03 - Launcher-Dashboard Integration

## Mission

Wire launcher state into the main TUI lifecycle, including startup bypass rules,
launcher key routing, and Esc behavior for launcher sub-views.

## Scope
### Files to Create / Modify

```text
cmd/kasmos/main.go
internal/tui/model.go
internal/tui/update.go
internal/tui/keys.go
```

### Technical References

- `internal/tui/model.go`
- `internal/tui/update.go`
- `cmd/kasmos/main.go`
- `kitty-specs/017-launcher-dashboard-screen/plan.md`

## Implementation

Integrate launcher flow into existing model/update/view wiring.

Requirements:
- Add `showLauncher bool` and `config *config.Config` to `Model`
- Update `NewModel(...)` signature to accept config + launcher startup context
- In `View()`, check `showLauncher` before dashboard rendering
- Add launcher key dispatch in `Update()` for `n/f/p/h/r/s/q`
- Implement transition behavior exactly:
  - `n` -> hide launcher, open spawn dialog in yolo mode
  - `f` -> hide launcher, route to spec-kitty feature creation
  - `p` -> hide launcher, route to spec-kitty plan flow
  - `h` -> open history overlay
  - `r` -> open restore picker stub
  - `s` -> open settings view stub
  - `q` -> quit
- CLI bypass: if positional args exist, or `--attach`, or `--daemon`, start with `showLauncher=false`
- Esc from history/restore/settings returns to launcher, not dashboard
- In `main.go`, load config and pass launch context into `NewModel`

## Verification

- `go test ./internal/tui -run Launcher`
- `go test ./cmd/kasmos -run Launcher`
- `go test ./...`
- Manual checks:
  - bare `kasmos` shows launcher
  - `kasmos <path>`, `kasmos --attach`, and `kasmos --daemon` bypass launcher
