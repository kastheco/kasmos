---
work_package_id: WP03
title: Launcher-Dashboard Integration
lane: planned
dependencies:
- WP01
- WP02
subtasks:
- Add showLauncher bool and config *config.Config to Model struct
- NewModel() accepts config parameter, sets showLauncher based on whether CLI args were provided
- View() checks showLauncher first, renders launcher or dashboard
- Launcher key handling in Update(): dispatch menu actions
- n -> set showLauncher=false, open spawn dialog (yolo mode)
- f -> set showLauncher=false, start spec-kitty feature creation
- p -> set showLauncher=false, start spec-kitty plan flow
- h -> open history overlay (Esc returns to launcher)
- r -> open restore picker (WP04 implements, stub for now)
- s -> open settings view (WP05 implements, stub for now)
- q -> quit
- CLI bypass: when args provided OR --attach OR --daemon, showLauncher starts as false
- Esc from history/restore/settings returns to launcher (not dashboard)
- main.go changes: load config, detect bare invocation, pass to NewModel
- Tests: launcher shown on bare invocation, skipped with args, menu key routing
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
