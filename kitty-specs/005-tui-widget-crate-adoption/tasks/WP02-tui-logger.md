---
work_package_id: "WP02"
subtasks:
  - "T006"
  - "T007"
  - "T008"
  - "T009"
  - "T010"
  - "T011"
  - "T012"
title: "Adopt tui-logger — Replace Hand-Rolled Log Viewer"
phase: "Phase 2 - Crate Adoptions"
lane: "done"
assignee: ""
agent: "reviewer"
shell_pid: "1313335"
review_status: "approved"
reviewed_by: "kas"
dependencies: ["WP01"]
history:
  - timestamp: "2026-02-12T00:00:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP02 – Adopt tui-logger — Replace Hand-Rolled Log Viewer

## ⚠️ IMPORTANT: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** – Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP02 --base WP01
```

Depends on WP01 (ratatui-macros). New code should use macro syntax.

---

## Objectives & Success Criteria

1. Replace the entire hand-rolled log system (`LogEntry`, `LogLevel`, `LogsState`, `render_logs()`, `filtered_log_entries()`, `format_timestamp()`, 10k-entry cap, filter mechanism) with `tui-logger`'s `TuiLoggerSmartWidget`.
2. Integrate tui-logger with the existing tracing subscriber stack via `TuiTracingSubscriberLayer` (FR-002).
3. Expose per-target log level toggling and page-mode scrollback in the Logs tab (FR-003, FR-004).
4. Refactor `logging.rs` to a conditional Registry-based subscriber (AD-1): TUI mode vs headless mode.
5. **SC-007**: Reduce Logs tab implementation by ≥100 LOC.
6. **SC-005/SC-006**: `cargo test` and `cargo clippy` pass with zero regressions.

## Context & Constraints

- **Architecture Decision AD-1** (plan.md): Conditional tracing subscriber with `init_logging(tui_mode: bool)`.
- **Architecture Decision AD-6** (plan.md): tui-logger key commands delegated via `TuiWidgetState::transition()`.
- **Research R-2** (research.md): Full integration pattern with code examples for Registry + TuiTracingSubscriberLayer.
- **Research R-3** (research.md): TuiLoggerSmartWidget API, built-in keybinding table, impact on existing code.
- **data-model.md**: Entities removed (LogEntry, LogLevel, LogsState), entities modified (App gains `logger_state`).
- **Constitution**: `tui_logger::move_events()` is O(buffer) per tick — no render loop impact.

### Files in scope

| File | Changes |
|------|---------|
| `crates/kasmos/Cargo.toml` | Add tui-logger dependency |
| `crates/kasmos/src/logging.rs` | Major refactor — conditional subscriber |
| `crates/kasmos/src/main.rs` | Update `init_logging()` call |
| `crates/kasmos/src/tui/mod.rs` | Call `tui_logger::init_logger()`, update `init_logging()` |
| `crates/kasmos/src/tui/app.rs` | Remove log types, add TuiWidgetState, replace render_logs |
| `crates/kasmos/src/tui/keybindings.rs` | Replace handle_logs_key |

---

## Subtasks & Detailed Guidance

### Subtask T006 – Add `tui-logger` dependency to Cargo.toml

- **Purpose**: Introduce tui-logger with tracing-support feature.
- **Steps**:
  1. Add to `crates/kasmos/Cargo.toml` `[dependencies]`:
     ```toml
     tui-logger = { version = "0.18", features = ["tracing-support"] }
     ```
  2. The `tracing-support` feature enables `TuiTracingSubscriberLayer` for integration with the tracing ecosystem.
  3. Run `cargo check -p kasmos` to verify dependency resolution.
- **Files**: `crates/kasmos/Cargo.toml`
- **Notes**: tui-logger 0.18.x requires ratatui ^0.30 (confirmed in R-1).

### Subtask T007 – Refactor `logging.rs` for conditional subscriber

- **Purpose**: Replace `fmt().init()` with a Registry-based approach that supports TUI mode (tui-logger) and headless mode (fmt to stderr).
- **Steps**:
  1. Update `init_logging()` signature to accept a mode parameter:
     ```rust
     pub fn init_logging(tui_mode: bool) -> Result<()> {
     ```
  2. Implement headless mode (existing behavior, restructured):
     ```rust
     if !tui_mode {
         let filter = EnvFilter::try_from_default_env()
             .unwrap_or_else(|_| EnvFilter::new("kasmos=info"));
         Registry::default()
             .with(fmt::layer()
                 .with_target(true)
                 .with_file(true)
                 .with_line_number(true))
             .with(filter)
             .init();
         return Ok(());
     }
     ```
  3. Implement TUI mode:
     ```rust
     // TUI mode: route tracing events to tui-logger widget
     tui_logger::init_logger(log::LevelFilter::Trace)
         .map_err(|e| crate::error::Error::Config(format!("tui-logger init failed: {e}")))?;
     tui_logger::set_default_level(log::LevelFilter::Trace);
     
     Registry::default()
         .with(tui_logger::tracing_subscriber_layer())
         .init();
     ```
  4. Add required imports:
     ```rust
     use tracing_subscriber::{Registry, layer::SubscriberExt, util::SubscriberInitExt};
     ```
  5. Remove the old `fmt().init()` code entirely.
  6. Update the doc comment and examples to reflect the new `tui_mode` parameter.
- **Files**: `crates/kasmos/src/logging.rs`
- **Parallel?**: Yes — can proceed concurrently with T009 (different file).
- **Notes**:
  - `tui_logger::init_logger()` MUST be called before the tracing subscriber is initialized.
  - The `log` crate may need to be added as a dependency for `log::LevelFilter`. Check if tui-logger re-exports it.
  - The existing logging test (`test_init_logging_succeeds`) will need updating for the new signature.

### Subtask T008 – Wire up TUI-mode logging in `mod.rs` and `main.rs`

- **Purpose**: Ensure the correct logging mode is activated based on the user's command.
- **Steps**:
  1. In `crates/kasmos/src/main.rs`, update the `init_logging()` call:
     ```rust
     // Currently: let _ = kasmos::init_logging();
     // Change to: let _ = kasmos::init_logging(false);
     // (main.rs always starts in headless mode — TUI init happens in tui::run)
     ```
     BUT: If `--tui` mode is used, the subscriber must be set up in TUI mode. Two approaches:
     - **Option A**: Pass `tui: bool` through to `init_logging()` call in main.
     - **Option B**: Delay `init_logging()` — call headless in main, but for TUI mode, don't call it in main at all and instead call it in `tui::run()`.
     
     **Recommended**: Option B — In `main.rs` for the `Start` command with `tui: true`, skip `init_logging()` and let `tui::run()` handle it. For all other commands, call `init_logging(false)`.

  2. In `crates/kasmos/src/tui/mod.rs`, update `run()`:
     ```rust
     pub async fn run(...) -> anyhow::Result<()> {
         // Initialize tui-logger BEFORE setting up subscriber
         crate::init_logging(true)?;
         
         install_panic_hook();
         let mut terminal = setup_terminal()?;
         // ...
     ```
  3. Add `tui_logger::move_events()` to the tick handler in the event loop:
     ```rust
     _ = tokio::time::sleep(Duration::from_millis(250)) => {
         tui_logger::move_events();  // Transfer log events to display buffer
         app.on_tick();
     }
     ```
- **Files**: `crates/kasmos/src/main.rs`, `crates/kasmos/src/tui/mod.rs`
- **Notes**: The `init_logging` function is likely exported from `crates/kasmos/src/lib.rs`. Verify the public API path.

### Subtask T009 – Remove hand-rolled log types and state from `app.rs`

- **Purpose**: Delete the old log system types and replace the `logs` field with `logger_state`.
- **Steps**:
  1. **Remove** these types from `crates/kasmos/src/tui/app.rs`:
     - `LogEntry` struct (lines ~142-153)
     - `LogLevel` enum (lines ~156-161)
     - `LogsState` struct (lines ~164-188)
  2. **Remove** `logs: LogsState` from the `App` struct (line ~210).
  3. **Add** `logger_state: tui_logger::TuiWidgetState` to the `App` struct:
     ```rust
     /// tui-logger widget state (target selection, scroll, page mode).
     pub logger_state: tui_logger::TuiWidgetState,
     ```
  4. **Update** `App::new()` — replace `logs: LogsState::default()` with:
     ```rust
     logger_state: tui_logger::TuiWidgetState::new(),
     ```
  5. **Remove** `filtered_log_entries()` method (lines ~424-443).
  6. **Remove** `format_timestamp()` method (lines ~445-456).
  7. **Remove** `render_logs()` method (lines ~652-766) — this is replaced by T010.
  8. **Remove** the `use std::time::{..., SystemTime, UNIX_EPOCH}` import if `SystemTime`/`UNIX_EPOCH` are no longer needed.
- **Files**: `crates/kasmos/src/tui/app.rs`
- **Parallel?**: Yes — can start concurrently with T007 (different file).
- **Notes**: This will cause temporary compilation errors until T010, T011, and T012 are also completed. Implement T009-T012 as a single coherent pass.

### Subtask T010 – Add TuiLoggerSmartWidget rendering in the Logs tab

- **Purpose**: Replace the removed `render_logs()` with `TuiLoggerSmartWidget`.
- **Steps**:
  1. Add a new `render_logs()` method to `App`:
     ```rust
     fn render_logs(&self, frame: &mut Frame, area: Rect) {
         let widget = tui_logger::TuiLoggerSmartWidget::default()
             .style_error(Style::default().fg(Color::Red))
             .style_warn(Style::default().fg(Color::Yellow))
             .style_info(Style::default().fg(Color::Cyan))
             .style_debug(Style::default().fg(Color::DarkGray))
             .style_trace(Style::default().fg(Color::DarkGray))
             .output_separator(' ')
             .output_target(true)
             .output_timestamp(Some("%H:%M:%S".to_string()))
             .output_level(Some(tui_logger::TuiLoggerLevelOutput::Abbreviated))
             .state(&self.logger_state);
         frame.render_widget(widget, area);
     }
     ```
     Note: `render_widget` requires the widget to take `&TuiWidgetState`. Check the actual tui-logger API — some versions require `&mut TuiWidgetState`. If so, `render_logs` must take `&mut self` and the `render()` dispatch must be adjusted.
  2. The `render()` method's `Tab::Logs` arm already calls `self.render_logs(frame, body_area)` — no dispatch change needed.
  3. Use macro syntax for any new text/layout construction (since WP01 is already merged).
- **Files**: `crates/kasmos/src/tui/app.rs`
- **Notes**: The TuiLoggerSmartWidget includes a built-in target selector panel (toggled by `h` key) and page mode scrollback (PageUp/PageDown). No additional UI code needed.

### Subtask T011 – Replace `handle_logs_key()` with tui-logger key event delegation

- **Purpose**: Forward key events to the tui-logger widget instead of the hand-rolled scroll/filter logic.
- **Steps**:
  1. Replace the entire `handle_logs_key()` function in `crates/kasmos/src/tui/keybindings.rs`:
     ```rust
     fn handle_logs_key(app: &mut App, key: KeyEvent) {
         // Translate crossterm key events to tui-logger widget events
         let event = match key.code {
             KeyCode::Char('h') => Some(tui_logger::TuiWidgetEvent::HideKey),
             KeyCode::Char('f') => Some(tui_logger::TuiWidgetEvent::FocusKey),
             KeyCode::Char(' ') => Some(tui_logger::TuiWidgetEvent::SpaceKey),
             KeyCode::Up => Some(tui_logger::TuiWidgetEvent::UpKey),
             KeyCode::Down => Some(tui_logger::TuiWidgetEvent::DownKey),
             KeyCode::Left => Some(tui_logger::TuiWidgetEvent::LeftKey),
             KeyCode::Right => Some(tui_logger::TuiWidgetEvent::RightKey),
             KeyCode::Char('+') => Some(tui_logger::TuiWidgetEvent::PlusKey),
             KeyCode::Char('-') => Some(tui_logger::TuiWidgetEvent::MinusKey),
             KeyCode::PageUp => Some(tui_logger::TuiWidgetEvent::PrevPageKey),
             KeyCode::PageDown => Some(tui_logger::TuiWidgetEvent::NextPageKey),
             KeyCode::Esc => Some(tui_logger::TuiWidgetEvent::EscapeKey),
             _ => None,
         };
         if let Some(evt) = event {
             app.logger_state.transition(evt);
         }
     }
     ```
  2. **Remove** all references to `app.logs` in keybindings.rs:
     - The old handler accessed `app.logs.filter_active`, `app.logs.auto_scroll`, `app.logs.scroll_offset`, `app.logs.filter`
     - All of these are removed — tui-logger manages its own state internally.
  3. Remove the `use crate::types::WorkPackage;` import if it's no longer needed in keybindings.rs.
- **Files**: `crates/kasmos/src/tui/keybindings.rs`
- **Parallel?**: Yes — different file from T009/T010.
- **Notes**:
  - Check the exact `TuiWidgetEvent` variant names in tui-logger 0.18.x — they may differ slightly. Use IDE autocompletion or check docs.
  - The `h` key for tui-logger target selector conflicts with the Dashboard's `h` (move lane left), but they're in different tabs so no conflict.
  - The old `/` (filter) key handler is removed entirely — tui-logger's target selector replaces the custom filter.

### Subtask T012 – Update all tests referencing removed log types and state

- **Purpose**: Fix compilation errors in tests that reference `LogEntry`, `LogLevel`, `LogsState`, or `app.logs`.
- **Steps**:
  1. In `crates/kasmos/src/tui/app.rs` tests:
     - **`test_review_policy_mode_selection_and_auto_mark_done_path`** (line ~1060): References `app.logs.entries`. This test verifies review policy logging. After migration, review policy events are logged via `tracing::info!()` and captured by tui-logger. The assertion must change from checking `app.logs.entries` to either:
       - (a) Using `tui_logger::TuiLoggerWidget` to read back events (complex)
       - (b) Removing the log content assertion and testing only the notification/state behavior
       - **Recommended**: Option (b) — the test's primary purpose is verifying review policy decisions, not log content. Remove the `app.logs.entries.iter().any(...)` assertions.
     
     - **`test_review_failure_surfaces_notification_and_log_entry`** (line ~1088): Same pattern — remove the `app.logs.entries.iter().any(...)` assertion. Keep the notification assertions (those are unchanged).
     
     - **`test_keyboard_only_flow_without_mouse_events`** (line ~1134): References `app.logs.filter_active`. The assertion `assert!(app.logs.filter_active)` after pressing `/` must be removed because the `/` key no longer activates a filter — tui-logger handles filtering internally. Remove the `/` key press test and the `filter_active` assertions.
     
     - **`test_event_loop_hot_paths_stay_non_blocking_under_load`** (line ~1164): References `app.logs.entries` indirectly via `update_state()`. This should still work since `update_state()` is being refactored to use tracing macros, but verify it compiles.

  2. In `crates/kasmos/src/tui/keybindings.rs` tests:
     - Verify no tests reference the old log state. (Current tests are Dashboard-focused, so no changes expected.)

  3. In `crates/kasmos/src/logging.rs` tests:
     - Update `test_init_logging_succeeds` to call `init_logging(false)` (headless mode).

  4. Refactor `capture_state_logs()` in app.rs to use tracing instead of pushing to `self.logs.entries`:
     ```rust
     fn capture_state_logs(&mut self, new_run: &OrchestrationRun) {
         let old_states: HashMap<&str, WPState> = self.run.work_packages.iter()
             .map(|wp| (wp.id.as_str(), wp.state)).collect();
         
         for wp in &new_run.work_packages {
             let old_state = old_states.get(wp.id.as_str()).copied();
             if old_state == Some(wp.state) { continue; }
             
             let from = old_state.map(|s| format!("{s:?}")).unwrap_or_else(|| "(new)".to_string());
             match wp.state {
                 WPState::Failed => tracing::error!(wp_id = %wp.id, "{from} -> {:?}", wp.state),
                 WPState::ForReview => tracing::warn!(wp_id = %wp.id, "{from} -> {:?}", wp.state),
                 _ => tracing::info!(wp_id = %wp.id, "{from} -> {:?}", wp.state),
             }
             
             if old_state != Some(WPState::ForReview) && wp.state == WPState::ForReview {
                 let decision = self.review_policy_executor.on_for_review_transition();
                 tracing::info!(wp_id = %wp.id,
                     "review_policy {:?}: run_automation={}, auto_mark_done={}",
                     self.review_policy_executor.policy(), decision.run_automation, decision.auto_mark_done);
             }
         }
         
         if new_run.state != self.run.state {
             tracing::info!("Run state: {:?} -> {:?}", self.run.state, new_run.state);
         }
     }
     ```
  5. Refactor `record_review_failure()` — remove `self.logs.entries.push(...)` and 10k cap:
     ```rust
     pub fn record_review_failure(&mut self, wp_id: impl Into<String>, failure_type: ReviewFailureType, message: impl Into<String>) {
         let wp_id = wp_id.into();
         let message = message.into();
         let notification_id = self.next_notification_id();
         
         self.notifications.push(Notification { /* ... same as before ... */ });
         
         // Log via tracing — tui-logger captures automatically
         tracing::error!(wp_id = %wp_id, "review_failure {:?}: {}", failure_type, message);
     }
     ```
  6. Remove the 10k-entry cap logic from `update_state()` (lines ~351-357).
- **Files**: `crates/kasmos/src/tui/app.rs`, `crates/kasmos/src/logging.rs`
- **Notes**: This is the most complex subtask — touches many test functions and the core logging integration. Take care to maintain test coverage for notification behavior even as log assertions are removed.

---

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Tracing subscriber single-init constraint in tests | High | Gate `init_logging()` calls in tests or use `#[serial]` |
| tui-logger API differs from research assumptions | Low | Check crate docs/source for exact method signatures |
| `TuiWidgetState` requires `&mut` for rendering | Medium | If so, change `render_logs(&self)` to `render_logs(&mut self)` |
| Test breakage from removed log assertions | Certain | T012 handles this — remove log content assertions, keep behavioral ones |

## Review Guidance

- **LOC reduction**: Count removed lines (LogEntry, LogLevel, LogsState, render_logs, filtered_log_entries, format_timestamp, cap logic) vs added lines. Must be ≥100 LOC net reduction (SC-007).
- **Tracing integration**: Verify `init_logger()` is called before subscriber init. Verify `move_events()` is in the tick handler.
- **Key delegation**: Verify all 12 tui-logger key commands are mapped in `handle_logs_key()`.
- **No stale references**: `grep -rn 'LogsState\|LogEntry\|LogLevel' crates/kasmos/src/` should return zero results.
- **Test pass**: `cargo test -p kasmos` with zero failures.

## Activity Log

- 2026-02-12T00:00:00Z – system – lane=planned – Prompt created.
- 2026-02-12T12:31:34Z – claude-opus-4-6 – lane=doing – Implementation complete
- 2026-02-12T12:32:32Z – claude-opus-4-6 – lane=for_review – Submitted for review
- 2026-02-12T12:32:39Z – reviewer – shell_pid=1313335 – lane=doing – Started review via workflow command
- 2026-02-12T12:34:35Z – reviewer – shell_pid=1313335 – lane=done – Review passed: All spec requirements met — tui-logger correctly replaces hand-rolled log viewer with 219 LOC net reduction, correct init ordering, all 12 key commands mapped, zero test failures, zero new clippy warnings, no stale type references.
