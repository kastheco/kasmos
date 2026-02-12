# Feature Specification: Wire TUI Mode into kasmos CLI

**Feature Branch**: `008-wire-tui-mode`
**Created**: 2026-02-12
**Status**: Draft
**Predecessor**: `002-ratatui-tui-controller-panel` (TUI foundation built but never wired)

## Problem Statement

The ratatui TUI controller panel (spec 002) was implemented across 10 work packages (WP01-WP10, all marked done), producing:
- A complete `tui/` module with async event loop, terminal lifecycle, and panic hooks
- An `App` struct with 3 tabs (Dashboard, Review, Logs), notification system, keybindings
- `tui::run()` function accepting `watch::Receiver<OrchestrationRun>` + `mpsc::Sender<EngineAction>`
- Comprehensive tests (resize, keyboard-only, hot-path latency, review policy)

**However, none of this code is reachable by the user.** Three critical gaps prevent it:

1. **No CLI activation path** -- `kasmos start` always runs `zellij attach` (line 656 of `start.rs`). There is no `--tui` flag or `kasmos tui` subcommand.
2. **No watch channel in the engine** -- `WaveEngine` does not hold a `watch::Sender<OrchestrationRun>`. The `tui::run()` function requires a `watch::Receiver<OrchestrationRun>` but the engine never creates or broadcasts to one.
3. **Placeholder tab rendering** -- Dashboard renders `"Dashboard view coming soon"` and Review renders `"Review view coming soon"`. Only the Logs tab has real rendering. Keybinding handlers for Dashboard (j/k/h/l navigation) and Review (j/k, approve/reject) are empty stubs with `// will be implemented in WPxx` comments.

## Scope

Wire the existing TUI infrastructure into a launchable end-to-end flow. This is a **completion/integration** spec, not a new feature build.

### In Scope

- CLI flag (`--tui`) on `kasmos start` to opt into TUI mode instead of bare `zellij attach`
- `watch` channel creation in `start.rs`, plumbed through `WaveEngine`
- Engine broadcasts `OrchestrationRun` snapshots after every state mutation
- TUI task spawned alongside engine, receiving watch + action channels
- Dashboard tab: real kanban rendering (4-lane layout, WP cards, vim navigation)
- Review tab: real review queue rendering (list + detail split, approve/reject keybindings)
- Dashboard action keybindings (R/P/F/T/A dispatching `EngineAction`)
- Notification bar diffing (detect state changes, surface review/failure/input-needed)

### Out of Scope

- Mouse click target detection (WP09, already has stub)
- `.input-needed` marker file polling (WP08, separate feature)
- ReviewRunner slash/prompt execution (WP06 T057-T060, already implemented)
- New feature work beyond what was specced in 002

## Requirements

### Functional Requirements

- **FR-W01**: `kasmos start <feature> --tui` MUST launch the TUI in the controller pane's terminal instead of running bare `zellij attach`.
- **FR-W02**: Without `--tui`, behavior MUST remain identical to current (attach to Zellij session directly).
- **FR-W03**: `WaveEngine` MUST accept a `watch::Sender<OrchestrationRun>` and broadcast the current `OrchestrationRun` snapshot after every call to `handle_completion()` and `handle_action()`.
- **FR-W04**: The Dashboard tab MUST render WPs in a 4-column kanban layout (Planned, Doing, For Review, Done) using ratatui widgets.
- **FR-W05**: Dashboard MUST support vim-style navigation: `h`/`l` between lanes, `j`/`k` within a lane.
- **FR-W06**: Dashboard MUST show selected-WP context: state badge, wave, elapsed time, dependencies.
- **FR-W07**: Dashboard MUST render contextual action hints for the selected WP (matching WP04 action-state table from 002 spec).
- **FR-W08**: Action keybindings (`R` restart, `P` pause/resume, `F` force-advance, `T` retry, `A` advance wave) MUST dispatch the corresponding `EngineAction` via the action channel.
- **FR-W09**: The Review tab MUST render a split layout: review queue list (left) + detail pane (right) for WPs in `ForReview` state.
- **FR-W10**: Review tab MUST support `a` (approve), `r` (reject+relaunch), and `j`/`k` navigation.
- **FR-W11**: The notification bar MUST diff previous vs current `OrchestrationRun` on each watch update and surface/dismiss notifications for state changes (ForReview, Failed).
- **FR-W12**: The notification jump keybinding (`n`) MUST cycle through active notifications, switching to the relevant tab and focusing the WP.

### Non-Functional Requirements

- **NFR-W01**: Watch channel broadcast MUST NOT add more than 1ms overhead to engine state mutations (clone + send).
- **NFR-W02**: TUI startup MUST NOT delay Zellij session creation or agent pane spawning.
- **NFR-W03**: If the TUI task panics or exits early, the engine and agents MUST continue running unaffected.

## Acceptance Criteria

1. `kasmos start 001-test --tui` launches, shows the ratatui TUI with a populated Dashboard kanban.
2. WPs move between lanes in real-time as agents complete work (watch channel → TUI).
3. Operator can navigate the kanban with h/j/k/l, select a failed WP, press `T` to retry, and see it move back to Doing.
4. Review tab shows WPs in ForReview with approve/reject actions that work end-to-end.
5. `kasmos start 001-test` (no `--tui`) still works exactly as before.
6. `cargo test` passes with no regressions.
