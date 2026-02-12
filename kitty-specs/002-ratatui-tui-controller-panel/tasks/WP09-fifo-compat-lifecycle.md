---
work_package_id: WP09
title: FIFO Compatibility & Terminal Lifecycle
lane: "done"
dependencies:
- WP01
base_branch: 002-ratatui-tui-controller-panel-WP01
base_commit: 0562b7a1d497f31478d3be6c173e0fea36354825
created_at: '2026-02-11T13:39:14.781941+00:00'
subtasks:
- T043
- T044
- T045
- T046
- T047
- T048
phase: Phase 3 - Advanced
assignee: 'unknown'
agent: "reviewer"
shell_pid: "3542700"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP09 – FIFO Compatibility & Terminal Lifecycle

## Objectives & Success Criteria

- FIFO commands (restart, advance, status, etc.) work while TUI is running
- FIFO-triggered state changes appear in TUI via watch channel
- No data races when TUI and FIFO send commands concurrently
- Terminal resize reflows layout gracefully
- Mouse clicks work on tabs and WP cards
- Orchestration termination shows final summary
- Empty state (no active run) displays guidance
- FR-012, FR-013, FR-015, FR-016: FIFO compat, real-time FIFO reflection, resize handling, keyboard-only operation

**Implementation command**: `spec-kitty implement WP09 --base WP07`

## Context & Constraints

- **FIFO pipeline**: CommandReader → ControllerCommand → CommandHandler → EngineAction → engine (via action_rx)
- **TUI pipeline**: crossterm events → keybindings → EngineAction → engine (via action_tx, same channel)
- **Both produce to same `action_rx`**: mpsc::Sender is Clone, no additional sync needed
- **Watch channel**: Engine broadcasts after every mutation — FIFO-triggered changes automatically visible
- **Edge cases from spec**: Terminal resize, 50+ WPs, unexpected run termination, malformed FIFO input, no mouse support

## Subtasks & Detailed Guidance

### Subtask T043 – Verify FIFO commands produce visible state changes

**Purpose**: Confirm that commands sent via the FIFO pipe (`.kasmos/cmd.pipe`) result in state changes visible in the TUI.

**Steps**:
1. This is primarily a verification task. The architecture already ensures this:
   - FIFO command → CommandHandler → EngineAction → engine mutates state → watch_tx.send() → TUI receives via watch_rx

2. Add a log entry when a FIFO command is received (if not already logged by CommandReader):
   ```rust
   // In App::update_state(), log external commands
   // This happens automatically since all state changes generate log entries (WP07)
   ```

3. Verify by testing: while TUI runs, write `restart WP01` to the FIFO and confirm the TUI dashboard shows the state change and the logs tab shows the event.

4. Ensure the `status` FIFO command doesn't interfere — it currently prints to stdout which would corrupt the TUI. Either:
   - Suppress stdout for status when TUI is active (redirect to log)
   - Or have CommandHandler detect TUI mode and skip stdout output

**Files**: `crates/kasmos/src/command_handlers.rs` (~5 lines — guard status output when TUI active)

**Notes**: The `Status` command in `command_handlers.rs` calls `println!()` which writes to the raw terminal. This WILL corrupt the TUI display. Add a `tui_active: bool` flag to CommandHandler and suppress print output when true.

### Subtask T044 – Handle concurrent TUI + FIFO input

**Purpose**: Ensure no data races or panics when both TUI and FIFO send EngineActions simultaneously.

**Steps**:
1. `tokio::sync::mpsc::Sender` is `Clone` and thread-safe. Multiple senders can call `send()` concurrently without races.

2. The engine processes actions sequentially from `action_rx` — ordering is first-come-first-served.

3. Verify: No `Mutex` or additional locking is needed. The existing channel architecture handles this.

4. Edge case: If the TUI sends `Pause(WP01)` and FIFO simultaneously sends `Restart(WP01)`, the engine processes whichever arrives first. The second may fail the state machine check (e.g., can't Restart an Active WP). This is correct behavior — log the error and continue.

5. Ensure EngineAction handler errors are logged but don't crash the engine:
   ```rust
   // In engine event loop:
   if let Err(e) = self.handle_action(action).await {
       tracing::warn!("Action failed: {}", e);
   }
   ```

**Files**: `crates/kasmos/src/engine.rs` (~3 lines — ensure error logging, not panic)

### Subtask T045 – Implement terminal resize handling

**Purpose**: When the terminal is resized (window resize, Zellij pane resize), the TUI layout must reflow.

**Steps**:
1. In `App::handle_event()`, handle Resize events:
   ```rust
   TuiEvent::Resize(width, height) => {
       self.terminal_size = (width, height);
       // ratatui handles resize automatically on next draw()
       // Just need to clamp scroll offsets and selection indices
       self.clamp_dashboard_indices();
       self.clamp_review_indices();
       self.clamp_logs_scroll();
   }
   ```

2. Add `terminal_size: (u16, u16)` to App state

3. In render functions, use the frame's area (which reflects current terminal size) for all layout calculations. ratatui's `Layout` already handles this — no manual calculation needed.

4. Edge case: Very small terminals (<40 cols or <10 rows). Show a "Terminal too small" message instead of attempting to render the full layout:
   ```rust
   if frame.area().width < 40 || frame.area().height < 10 {
       frame.render_widget(
           Paragraph::new("Terminal too small. Resize to at least 40x10."),
           frame.area()
       );
       return;
   }
   ```

**Files**: `crates/kasmos/src/tui/app.rs` (~20 lines)

### Subtask T046 – Implement mouse support

**Purpose**: Enable mouse interactions as a complement to keyboard navigation.

**Steps**:
1. Mouse support is enabled by `EnableMouseCapture` in terminal setup (WP01).

2. In `App::handle_event()`, handle mouse events:
   ```rust
   TuiEvent::Mouse(mouse) => {
       match mouse.kind {
           MouseEventKind::Down(MouseButton::Left) => {
               let x = mouse.column;
               let y = mouse.row;
               // Check if click is in tab header area
               if y == 0 {
                   self.handle_tab_click(x);
               }
               // Check if click is in a WP card
               // Requires tracking rendered widget positions
           }
           MouseEventKind::ScrollUp => {
               // Scroll up in current view
               self.handle_scroll(-3);
           }
           MouseEventKind::ScrollDown => {
               self.handle_scroll(3);
           }
           _ => {}
       }
   }
   ```

3. Tab header click detection: Calculate tab boundaries based on tab title widths. Store tab header positions after rendering.

4. WP card click detection: This requires tracking where each WP card was rendered. Store a `Vec<(Rect, String)>` mapping rendered areas to WP IDs, updated on each render.

5. Scroll wheel: In Dashboard, scroll the focused lane. In Logs, scroll the log list. In Review, scroll the detail pane.

**Files**: `crates/kasmos/src/tui/app.rs` (~40 lines)

**Notes**: Mouse support should be best-effort. If position tracking is too complex initially, just support scroll wheel and tab header clicks. FR-016 requires keyboard-only to be fully functional.

### Subtask T047 – Handle orchestration termination

**Purpose**: When the engine finishes (Completed, Failed, or Aborted), show a final summary and allow the operator to quit.

**Steps**:
1. Detect terminal states in `App::update_state()`:
   ```rust
   match new_run.state {
       RunState::Completed | RunState::Failed | RunState::Aborted => {
           self.run_finished = true;
           self.logs.entries.push(LogEntry {
               timestamp: SystemTime::now(),
               level: if new_run.state == RunState::Failed { LogLevel::Error } else { LogLevel::Info },
               wp_id: None,
               message: format!("Orchestration {:?}", new_run.state),
           });
       }
       _ => {}
   }
   ```

2. Add `run_finished: bool` to App

3. When `run_finished`, render a status line at the bottom:
   ```
   Orchestration complete (5/5 WPs done). Press [q] to exit.
   ```
   Or for failures:
   ```
   Orchestration failed (3/5 WPs done, 2 failed). Press [q] to exit or [R] to restart failed WPs.
   ```

4. The TUI continues running (operator can inspect final state) until they press `q`

**Files**: `crates/kasmos/src/tui/app.rs` (~20 lines), rendering in dashboard (~10 lines)

### Subtask T048 – Implement empty/no-run state

**Purpose**: When the TUI starts with no active orchestration or the orchestration hasn't begun, show helpful guidance.

**Steps**:
1. Check if `app.run.work_packages.is_empty()` or `app.run.state == RunState::Initializing`

2. Render a centered guidance message:
   ```
   ┌─────────────────────────────────┐
   │     kasmos — no active run      │
   │                                 │
   │  Waiting for orchestration...   │
   │                                 │
   │  Start with:                    │
   │  kasmos launch <feature>        │
   └─────────────────────────────────┘
   ```

3. This should replace the normal tab rendering — no kanban board for an empty state

**Files**: `crates/kasmos/src/tui/app.rs` (~15 lines)

## Risks & Mitigations

- **Status command corrupts TUI**: The FIFO `status` command currently prints to stdout. Must suppress when TUI is active — add a guard in CommandHandler.
- **Mouse click position tracking**: Complex to implement fully. Start with tab header clicks and scroll wheel only. WP card clicks can be added later.
- **Orchestration termination timing**: The TUI might receive the final state update before the engine fully shuts down. The TUI should handle the watch channel closing gracefully (returns None from changed()).

## Review Guidance

- Test: Send FIFO commands while TUI is running. Verify state changes appear in both dashboard and logs.
- Test: Resize terminal while TUI is running. Verify layout reflows without crash or corruption.
- Test: Mouse scroll in logs tab. Verify scroll works.
- Test: Run to completion. Verify final summary shows and TUI remains interactive.
- Test: Start TUI before engine launches first WP. Verify empty state guidance renders.

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T13:39:14Z – coder – shell_pid=3488876 – lane=doing – Assigned agent via workflow command
- 2026-02-11T13:55:42Z – coder – shell_pid=3488876 – lane=for_review – Submitted for review via swarm
- 2026-02-11T13:55:42Z – reviewer – shell_pid=3542700 – lane=doing – Started review via workflow command
- 2026-02-11T13:57:36Z – reviewer – shell_pid=3542700 – lane=done – Review passed via swarm
