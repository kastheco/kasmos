# Work Packages: Wire TUI Mode into kasmos CLI

**Inputs**: spec.md, plan.md from `kitty-specs/005-002-ratatui-tui-controller-panel/`
**Prerequisites**: All 002 WPs complete (TUI foundation code exists). `cargo build` succeeds.

**Organization**: 14 subtasks (`T001`-`T014`) in 4 work packages (`WP01`-`WP04`).

---

## Work Package WP01: CLI Flag + Watch Channel Plumbing (Priority: P0)

**Goal**: Add `--tui` flag to CLI, create watch channel in `start.rs`, plumb `watch_tx` into engine, and broadcast state after every mutation.
**Independent Test**: `cargo build` succeeds. `WaveEngine` with watch channel broadcasts state changes (unit test). `kasmos start --help` shows `--tui` flag.
**Estimated Size**: ~80 lines changed

### Included Subtasks

- [ ] T001 Add `--tui` boolean flag to `Commands::Start` in `main.rs`, pass to `start::run()`
- [ ] T002 Create `watch::channel(run.clone())` in `start.rs` after `OrchestrationRun` construction
- [ ] T003 Add `watch_tx: Option<watch::Sender<OrchestrationRun>>` to `WaveEngine`, add `with_watch_tx()` builder method
- [ ] T004 Add `broadcast_state()` helper to `WaveEngine` — clones run, sends via `watch_tx`. Call after `handle_completion()`, `handle_action()`, `launch_eligible_wps_and_notify()`, and initial launch.
- [ ] T005 Add unit test: engine with watch channel verifies receiver gets updated state after completion event

### Implementation Notes

- `watch::Sender::send()` never blocks — it overwrites the current value. Safe to call from the engine's hot path.
- `watch_tx` is `Option` so existing tests that don't need it aren't affected.
- The `start.rs` signature changes from `run(feature, mode)` to `run(feature, mode, tui: bool)`.

### Dependencies
- None (starting package)

---

## Work Package WP02: TUI Spawn + Lifecycle in start.rs (Priority: P0)

**Goal**: When `--tui` is set, spawn `tui::run()` as a tokio task instead of interactive `zellij attach`. Ensure clean lifecycle: TUI exit doesn't kill the engine, engine completion is surfaced in TUI.
**Independent Test**: With `--tui`, kasmos starts without attaching interactively. Without `--tui`, existing behavior unchanged. TUI task panic doesn't crash the process.
**Estimated Size**: ~40 lines changed

### Included Subtasks

- [ ] T006 When `tui == true`: clone `engine_action_tx`, spawn `tui::run(watch_rx, engine_action_tx_clone)` as a tokio task. Await the TUI handle instead of `zellij attach`.
- [ ] T007 When `tui == false`: existing `zellij attach` flow unchanged (pass-through).
- [ ] T008 Handle TUI task result: if `tui::run()` returns `Err`, log the error and print a user-visible message. If it returns `Ok`, exit cleanly.

### Implementation Notes

- The Zellij session is still created in background mode (`--create-background`) regardless of `--tui`. The TUI replaces only the interactive attach step.
- `engine_action_tx` must be cloned because both the FIFO command bridge and the TUI need a sender.
- The TUI gets its own clone of `engine_action_tx`, not the one already moved into `CommandHandler`.

### Dependencies
- Depends on WP01 (watch channel must exist)

---

## Work Package WP03: Dashboard Kanban + Action Keybindings (Priority: P1)

**Goal**: Replace Dashboard placeholder with real 4-column kanban rendering. Wire up vim navigation and action key dispatch.
**Independent Test**: Dashboard renders WPs in correct lanes (TestBackend). h/j/k/l navigation updates focused lane and selected index. Action keys send correct EngineAction.
**Estimated Size**: ~200 lines changed

### Included Subtasks

- [ ] T009 Implement `render_dashboard()` in `app.rs`: partition WPs by state into 4 lanes (Pending→Planned, Active→Doing, ForReview, Completed→Done). Render as 4 `List` widgets in a horizontal `Layout` with `Constraint::Percentage(25)` each. Each item: `"{wp.id} {wp.title}"` with colored state badge. Highlight selected item.
- [ ] T010 Wire dashboard navigation in `keybindings.rs`: `j`/`k` increment/decrement `selected_index` (clamped to lane length). `h`/`l` move `focused_lane` between 0-3, reset `selected_index` to 0. Show wave separator lines between wave groups within each lane.
- [ ] T011 Wire action keybindings in `keybindings.rs` for Dashboard tab: `R` → Restart, `P` → Pause (if Active) or Resume (if Paused), `F` → ForceAdvance, `T` → Retry, `A` → Advance wave (wave-gated only). Each sends the corresponding `EngineAction` via `action_tx.try_send()`. Only dispatch if the action is valid for the selected WP's current state.
- [ ] T012 Add unit tests: (a) `render_dashboard` with TestBackend verifies 4 lanes rendered. (b) Navigation updates state correctly. (c) Action keys produce correct EngineAction on channel.

### Implementation Notes

- State-to-lane mapping: `Pending|Paused → Planned`, `Active → Doing`, `ForReview → For Review`, `Completed|Failed → Done` (Failed shown with red badge in Done lane, matching 002 spec's 4-lane design).
- Elapsed time: `wp.started_at.map(|t| SystemTime::now().duration_since(t))` formatted as `Xm Ys`.
- Action validity table (from 002 spec WP04): Active→{Pause}, Paused→{Resume,Restart}, Failed→{Restart,Retry,ForceAdvance}, ForReview→{Approve,Reject}, Pending→{} (no actions).

### Dependencies
- Depends on WP01 (watch channel for live updates)

---

## Work Package WP04: Review Tab + Notification Diffing (Priority: P1)

**Goal**: Replace Review tab placeholder with real review queue rendering. Implement notification diffing in `update_state()` and notification jump keybinding.
**Independent Test**: Review tab shows ForReview WPs with approve/reject keys. Notification bar updates on state transitions. `n` key jumps to notified WP.
**Estimated Size**: ~180 lines changed

### Included Subtasks

- [ ] T013 Implement `render_review()` in `app.rs`: filter WPs where `state == ForReview`. Left pane: scrollable list with `j`/`k` navigation. Right pane: selected WP detail (title, wave, time in review, dependencies). Bottom: action hints `[a] Approve  [r] Reject+Relaunch`. Wire `a` → `EngineAction::Approve(wp_id)`, `r` → `EngineAction::Reject { wp_id, relaunch: true }` in `keybindings.rs`.
- [ ] T014 Implement notification diffing in `update_state()`: compare old vs new WP states. WP entered ForReview → push `NotificationKind::ReviewPending`. WP entered Failed → push `NotificationKind::Failure`. WP left ForReview (approved/rejected) → auto-dismiss. WP left Failed (restarted/retried) → auto-dismiss. Render notification bar at top of frame (between tab bar and body) when notifications are non-empty.
- [ ] T015 Wire `n` keybinding: cycle through `self.notifications`, switch `active_tab` to the relevant tab (ReviewPending → Review, Failure → Dashboard), set selection to the notified WP's position in its lane/list.
- [ ] T016 Add unit tests: (a) Review tab renders ForReview WPs. (b) Approve key sends correct action. (c) Notification diffing: state change to ForReview adds notification, state change to Completed removes it. (d) `n` key cycles and switches tabs.

### Dependencies
- Depends on WP01 (watch channel)
- Can run in parallel with WP03

---

## Dependency & Execution Summary

```
Wave 1 (Plumbing):     WP01 ──┐
                       WP02 ──┤ (WP02 depends on WP01)
                              │
Wave 2 (UI):           WP03 ──┤ depends WP01 (parallel with WP04)
                       WP04 ──┘ depends WP01 (parallel with WP03)
```

**Critical path**: WP01 → WP02 (TUI is launchable with placeholder tabs)
**Parallelization**: WP03 and WP04 can run concurrently after WP01.

**MVP**: WP01 + WP02 = TUI launches and shows live-updating state (placeholder tabs, but functional event loop). WP03 + WP04 = full interactive experience.

---

## Subtask Index

| Subtask | Summary | WP | Priority |
|---------|---------|-----|----------|
| T001 | Add `--tui` flag to CLI | WP01 | P0 |
| T002 | Create watch channel in `start.rs` | WP01 | P0 |
| T003 | Add `watch_tx` field to `WaveEngine` | WP01 | P0 |
| T004 | Add `broadcast_state()` to engine, call after mutations | WP01 | P0 |
| T005 | Unit test: engine watch broadcast | WP01 | P0 |
| T006 | Spawn `tui::run()` when `--tui` set | WP02 | P0 |
| T007 | Preserve existing non-TUI attach flow | WP02 | P0 |
| T008 | Handle TUI task error/exit | WP02 | P0 |
| T009 | Dashboard kanban rendering (4 lanes) | WP03 | P1 |
| T010 | Dashboard vim navigation (h/j/k/l) | WP03 | P1 |
| T011 | Dashboard action keybindings (R/P/F/T/A) | WP03 | P1 |
| T012 | Dashboard unit tests | WP03 | P1 |
| T013 | Review tab rendering (queue + detail split) | WP04 | P1 |
| T014 | Notification diffing in `update_state()` | WP04 | P1 |
| T015 | Notification jump keybinding (`n`) | WP04 | P1 |
| T016 | Review + notification unit tests | WP04 | P1 |
