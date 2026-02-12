# Architecture Plan: Wire TUI Mode into kasmos CLI

## Overview

This is an integration plan — no new architectural patterns are introduced. We're connecting existing, tested subsystems that were built but never plumbed together.

## Current Architecture (relevant parts)

```
                     ┌──────────────┐
                     │  start.rs    │
                     │  (CLI entry) │
                     └──────┬───────┘
                            │ creates
                            ▼
┌───────────────┐    ┌──────────────┐    ┌──────────────┐
│ CommandReader │───▶│  WaveEngine  │◀───│ Completion   │
│ (FIFO)       │    │              │    │ Detector     │
└───────────────┘    └──────┬───────┘    └──────────────┘
                            │ state in
                            ▼
                     Arc<RwLock<OrchestrationRun>>
                            │
                            ▼
                     zellij attach (blocking)
```

## Target Architecture

```
                     ┌──────────────┐
                     │  start.rs    │
                     │  --tui flag  │
                     └──────┬───────┘
                            │ creates
                            ▼
┌───────────────┐    ┌──────────────┐    ┌──────────────┐
│ CommandReader │───▶│  WaveEngine  │◀───│ Completion   │
│ (FIFO)       │    │ +watch_tx    │    │ Detector     │
└───────────────┘    └──────┬───────┘    └──────────────┘
                            │ broadcasts via
                            │ watch::Sender<OrchestrationRun>
                            ▼
                     ┌──────────────┐
                     │  tui::run()  │───▶ action_tx ───▶ engine
                     │  (ratatui)   │◀─── watch_rx
                     └──────────────┘
                            │
                     OR (if no --tui)
                            │
                     zellij attach (blocking, unchanged)
```

## Change Inventory

### 1. CLI Flag Addition (`main.rs`)

Add `--tui` boolean flag to the `Start` command variant. Pass it through to `start::run()`.

**Scope**: ~5 lines in `main.rs`.

### 2. Watch Channel Creation (`start.rs`)

Create `watch::channel(initial_run.clone())` after building the `OrchestrationRun`. Pass `watch_tx` to `WaveEngine::new()`. Pass `watch_rx` to `tui::run()`.

**Decision**: The watch channel is created unconditionally (even without `--tui`) so that future consumers (status endpoint, external monitors) can also subscribe. Cost is negligible (one `watch::channel` allocation).

**Scope**: ~15 lines in `start.rs`.

### 3. Engine Watch Broadcast (`engine.rs`)

Add `watch_tx: Option<watch::Sender<OrchestrationRun>>` field to `WaveEngine`. Add a `broadcast_state()` helper that clones the run and sends via `watch_tx`. Call `broadcast_state()` at the end of:
- `handle_completion()`
- `handle_action()`
- `launch_eligible_wps_and_notify()`
- `advance_wave()`

**Scope**: ~30 lines in `engine.rs`.

### 4. TUI Spawn (`start.rs`)

When `--tui` is set:
- Spawn `tui::run(watch_rx, engine_action_tx.clone())` as a tokio task
- Instead of `zellij attach` (interactive blocking), `zellij attach --create-background` was already done — just skip the interactive attach and instead `tui_handle.await`
- On TUI exit (user presses `q`), the orchestration continues running (detached Zellij session persists)

When `--tui` is NOT set:
- Existing behavior unchanged: run `zellij attach` interactively

**Scope**: ~20 lines in `start.rs`.

### 5. Dashboard Kanban Rendering (`tui/app.rs`)

Replace the placeholder in `render()` for `Tab::Dashboard` with a real 4-column kanban layout:
- Partition `self.run.work_packages` by state into 4 lanes
- Render each lane as a `List` widget inside a `Block` with title
- Highlight the selected item based on `self.dashboard.focused_lane` + `selected_index`
- Show WP card info: ID, title, state badge (colored), wave, elapsed time

Wire up dashboard keybindings in `keybindings.rs`:
- `j`/`k`: move `selected_index` within lane (clamped to lane length)
- `h`/`l`: move `focused_lane` (0-3), reset `selected_index` to 0
- Action keys dispatch to `action_tx`

**Scope**: ~150 lines across `app.rs` and `keybindings.rs`.

### 6. Review Tab Rendering (`tui/app.rs`)

Replace the Review placeholder with a split layout:
- Left: list of WPs where `state == ForReview`, with `j`/`k` navigation
- Right: detail pane showing selected WP's title, wave, time in review, review feedback (if any from persisted results)
- `a` sends `EngineAction::Approve(wp_id)`, `r` sends `EngineAction::Reject { wp_id, relaunch: true }`

Wire up review keybindings in `keybindings.rs`.

**Scope**: ~100 lines across `app.rs` and `keybindings.rs`.

### 7. Notification Diffing (`tui/app.rs`)

In `update_state()`, compare old vs new WP states:
- WP entered ForReview → push `NotificationKind::ReviewPending`
- WP entered Failed → push `NotificationKind::Failure`
- WP left ForReview/Failed → auto-dismiss matching notification

Wire `n` key to cycle through notifications and jump to relevant tab + WP.

**Scope**: ~60 lines in `app.rs`, ~10 lines in `keybindings.rs`.

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| `watch_tx.send()` blocks engine if TUI is slow | Engine latency | `watch::send()` is non-blocking by design (overwrites) |
| TUI panic corrupts terminal | Bad UX | Panic hook already installed in `tui/mod.rs` |
| `engine_action_tx` shared between FIFO handler and TUI | Data race | `mpsc::Sender` is `Clone` + thread-safe, no issue |
| Breaking existing non-TUI flow | Regression | `--tui` is opt-in, default path unchanged |

## Testing Strategy

- Existing `cargo test` must pass (no regressions)
- Add unit test: `WaveEngine` with watch channel → verify broadcast after completion
- Add unit test: Dashboard renders WPs in correct lanes (using `TestBackend`)
- Add unit test: Review tab renders ForReview WPs with keybinding dispatch
- Add unit test: Notification diffing produces correct add/dismiss events
- Manual smoke test: `kasmos start <feature> --tui` shows live-updating kanban
