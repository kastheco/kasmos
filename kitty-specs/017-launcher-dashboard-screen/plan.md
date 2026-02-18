# Implementation Plan: Launcher Dashboard Screen

**Branch**: `017-launcher-dashboard-screen` | **Date**: 2026-02-18 | **Spec**: `kitty-specs/017-launcher-dashboard-screen/spec.md`
**Input**: Feature specification from `kitty-specs/017-launcher-dashboard-screen/spec.md`

## Summary

kasmos currently drops directly into the worker dashboard on bare invocation. This feature
adds a LazyVim-style launcher screen shown when kasmos is run with no arguments -- centered
ASCII art branding with a menu of actions (new task, create feature spec, create plan,
view history, restore session, settings, quit). CLI arguments, `--attach`, and `--daemon`
bypass the launcher entirely.

**Technical approach**: The launcher is a view state within the existing bubbletea Model,
not a separate tea.Program. A `showLauncher` bool gates whether View() renders the launcher
or the worker dashboard. Menu actions either transition to the dashboard (spawning/restoring)
or open sub-views (settings, history) that Esc returns from. A new `internal/config/` package
handles TOML-based persistent configuration for per-agent-role model settings and default
task source.

## Technical Context

**Language/Version**: Go 1.23+
**Primary Dependencies**: bubbletea v2, lipgloss v2, bubbles, huh, cobra, pelletier/go-toml/v2 (new)
**Storage**: `.kasmos/config.toml` (new, TOML), `.kasmos/session.json` (existing, JSON)
**Testing**: `go test ./...`, table-driven tests, mock WorkerBackend
**Existing infrastructure**:
- `internal/tui/history.go` - history overlay (lists archived sessions, view-only)
- `internal/tui/newdialog.go` - `n` key picker (spec-kitty/gsd/yolo), feature/plan forms
- `internal/tui/overlays.go` - spawn/continue/batch/quit dialog rendering
- `internal/persist/session.go` - `LoadSessionFromPath()`, `ListArchived()`
- `internal/task/source.go` - `AutoDetect()`, `AutoDetectSpecKitty()`, `AutoDetectGSD()`

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| Go 1.23+ | PASS | No new language features required. |
| bubbletea/lipgloss/bubbles | PASS | Launcher uses existing component stack. |
| OpenCode sole agent harness | PASS | No change to worker spawning. |
| `go test ./...` for testing | PASS | All WPs include test requirements. |
| TUI never blocks Update loop | PASS | Config load is synchronous but fast (single file read). |
| Linux primary, macOS secondary | PASS | TOML + file paths are cross-platform. |
| Single binary distribution | PASS | go-toml/v2 compiles in statically. |
| No secrets in persistence | PASS | config.toml stores model names, not credentials. |

No violations.

## Key Design Decisions

### 1. Launcher is a view state, not a separate Program

The launcher is controlled by `m.showLauncher` within the existing Model. This avoids
the complexity of chaining multiple tea.Programs and preserves access to the backend,
persister, and all existing infrastructure. View() checks `showLauncher` first, before
any other rendering.

### 2. Esc returns to launcher from sub-views only

Settings, history, and restore picker are "sub-views" of the launcher. Pressing Esc
returns to the launcher. Once a session-creating action fires (new task, restore session,
spec-kitty flow), the user transitions to the dashboard and the launcher is behind them.
No "back to launcher" from an active session.

### 3. Config is TOML, sessions stay JSON

`.kasmos/config.toml` is user-facing (hand-editable, supports comments). Session
persistence remains JSON (machine-managed, no human editing expected). The config
package is independent of the persist package.

### 4. Config loaded once at startup, saved on change

Config is loaded during `NewModel()` (or before, in main.go). Settings changes save
immediately. The config struct is passed by pointer to the Model so spawn commands
can read per-role settings.

### 5. Restore picker extends existing history infrastructure

`persist.ListArchived()` already returns archived sessions. The restore picker adds
selection + load capability on top, reusing the same data source. The active session
(`.kasmos/session.json`) is shown first if it exists and the PID is dead.

### 6. ASCII branding uses the existing gradient palette

The "kasmos" ASCII art uses a multi-line block font rendered with lipgloss gradient
from hot pink (#F25D94) to purple (#7D56F4), matching the existing color system in
`internal/tui/styles.go`. The gradient is applied per-line using gamut (already a dep).

## Project Structure

### New Files

```
internal/
  config/
    config.go           # Config struct, Load(), Save(), defaults, TOML serialization
    config_test.go      # Table-driven tests for load/save/defaults/corrupt handling

internal/tui/
  launcher.go           # Launcher view rendering, ASCII art, menu items
```

### Modified Files

```
cmd/kasmos/main.go      # Detect bare invocation, pass launcher flag to Model
internal/tui/model.go   # Add showLauncher state, config pointer, launcher transitions
internal/tui/update.go  # Launcher key handling, action dispatch, Esc routing
internal/tui/keys.go    # Launcher-specific key bindings (or reuse existing)
internal/tui/history.go # Extend for restore-from-launcher flow (selection + load)
internal/tui/styles.go  # ASCII art styles, launcher menu styles
internal/tui/commands.go # Restore session command
go.mod                  # Add pelletier/go-toml/v2
```

## Implementation Waves

### Wave 1: Foundation (2 WPs, parallelizable)

Config package and launcher screen can be built independently. No dependency between them.

**Dependencies**: None (builds on existing codebase)

**Deliverables**:
- `internal/config/` - TOML config load/save with defaults
- `internal/tui/launcher.go` - ASCII branding + menu rendering

**Acceptance**: Config loads/saves/defaults work. Launcher renders centered branding + menu.

### Wave 2: Integration (1 WP)

Wire the launcher into the main TUI lifecycle. CLI bypass logic. Transition routing.

**Dependencies**: Wave 1 (both WP01 and WP02)

**Deliverables**:
- Launcher shown on bare invocation, skipped with args/attach/daemon
- Menu actions route to correct destinations
- Esc from sub-views returns to launcher

**Acceptance**: `kasmos` shows launcher. `kasmos <path>` skips it. Menu keys work.

### Wave 3: Extended Features (3 WPs, parallelizable)

Restore picker, settings view, and spec-kitty integration are independent of each other.
All depend on Wave 2 (launcher must be integrated).

**Dependencies**: Wave 2 (WP03). WP05 also depends on WP01 (config package).

**Deliverables**:
- Session restore picker with selection and load
- Settings view with per-role config editing
- Spec-kitty feature/plan creation from launcher

**Acceptance**: All 5 user stories from the spec pass. Full round-trip from launcher
to dashboard and back (for sub-views).

## Work Package Decomposition

6 WPs across 3 waves. Each WP is independently implementable by a coder agent
in a single session. WP files are in `kitty-specs/017-launcher-dashboard-screen/tasks/`.

### Wave 1: Foundation (2 WPs)

| WP | Title | Dependencies | User Stories | Key Deliverables |
|----|-------|-------------|--------------|------------------|
| WP01 | Config Package | none | US3 (partial) | internal/config/ (TOML load/save, defaults, per-role agent config) |
| WP02 | Launcher Screen View | none | US1 (partial) | internal/tui/launcher.go (ASCII art, menu, centered layout) |

### Wave 2: Integration (1 WP)

| WP | Title | Dependencies | User Stories | Key Deliverables |
|----|-------|-------------|--------------|------------------|
| WP03 | Launcher-Dashboard Integration | WP01, WP02 | US1, US5 | Wire launcher into Model, CLI bypass, transitions, Esc routing |

### Wave 3: Extended Features (3 WPs)

| WP | Title | Dependencies | User Stories | Key Deliverables |
|----|-------|-------------|--------------|------------------|
| WP04 | Session Restore Picker | WP03 | US2 | Restore picker UI, session selection, load into dashboard |
| WP05 | Settings View | WP01, WP03 | US3 | Settings overlay, per-role editing, save to TOML, wire into spawn |
| WP06 | Spec-Kitty Launcher Actions | WP03 | US4 | f/p menu handlers, feature picker, graceful missing-tool handling |

### Dependency Graph

```
WP01 (config) ──────────────┬──→ WP03 (integration) ──┬──→ WP04 (restore)
                             │                         ├──→ WP06 (spec-kitty)
WP02 (launcher view) ───────┘                         │
                                                      └──→ WP05 (settings)
WP01 (config) ────────────────────────────────────────────→ WP05 (settings)
```

**Parallelism opportunities**:
- WP01 and WP02 can run in parallel (no dependency)
- WP04, WP05, WP06 can run in parallel after WP03 completes (WP05 also needs WP01)

## Risk Register

| Risk | Impact | Mitigation |
|------|--------|------------|
| go-toml/v2 adds binary size bloat | Larger binary | go-toml/v2 is ~2MB compiled. Acceptable for config parsing. |
| ASCII art looks bad on narrow terminals | Poor first impression | Minimum 80-col width already enforced. Test at 80x24. |
| Launcher adds startup latency | Slower to first useful screen | Config load is a single file read. Launcher renders in one frame. |
| Settings changes not reflected in running workers | Confusion | Settings apply to NEW spawns only. Document this in settings view. |
| Restore picker shows stale/corrupt sessions | Failed restore | Validate session JSON on load. Show error and stay on picker if corrupt. |

## Reference Documents

- **Spec**: `kitty-specs/017-launcher-dashboard-screen/spec.md`
- **016 Plan**: `kitty-specs/016-kasmos-agent-orchestrator/plan.md`
- **Layout specification**: `design-artifacts/tui-layout-spec.md`
- **Style specification**: `design-artifacts/tui-styles.md`
- **Constitution**: `.kittify/memory/constitution.md`
- **Architecture**: `.kittify/memory/architecture.md`
