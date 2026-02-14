---
work_package_id: WP04
title: Action Buttons & WP Control Dispatch
lane: "done"
dependencies:
- WP02
base_branch: 002-ratatui-tui-controller-panel-WP02
base_commit: a1c4a7e3bc679aa136bf5ea2a44b0e9bfe44ceee
created_at: '2026-02-11T10:33:05.534900+00:00'
subtasks:
- T018
- T019
- T020
- T021
- T022
phase: Phase 2 - Core Views
assignee: 'unknown'
agent: "reviewer"
shell_pid: "3253933"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP04 – Action Buttons & WP Control Dispatch

## Objectives & Success Criteria

- Contextual action buttons render below the selected WP in the dashboard
- Only valid actions for the WP's current state are shown (per plan table)
- Keybinds dispatch the correct `EngineAction` via `action_tx`
- Wave advance button visible at wave boundary in wave-gated mode
- Destructive actions (Force-Advance, Abort) require confirmation

**Implementation command**: `spec-kitty implement WP04 --base WP03`

## Context & Constraints

- **Keybindings** from plan: `R`=Restart, `P`=Pause/Resume, `F`=Force-Advance, `T`=Retry, `A`=Advance Wave
- **EngineAction variants**: Restart(String), Pause(String), Resume(String), ForceAdvance(String), Retry(String), Advance, Abort, Approve(String), Reject{wp_id, relaunch}
- **State-to-actions map** from plan.md:
  - Pending: (none)
  - Active: Pause
  - Paused: Resume
  - Failed: Restart, Retry, Force-Advance
  - ForReview: (handled in Review tab, not dashboard)
  - Completed: (none)
- **App has**: `action_tx: mpsc::Sender<EngineAction>` for sending commands

## Subtasks & Detailed Guidance

### Subtask T018 – Create `tui/widgets/action_buttons.rs`

**Purpose**: Render a horizontal bar of action buttons for the currently selected WP.

**Steps**:
1. Create `crates/kasmos/src/tui/widgets/action_buttons.rs`:
   ```rust
   pub struct ActionButton {
       pub label: String,
       pub key: char,
       pub style: Style,
   }

   pub fn render_action_bar(frame: &mut Frame, area: Rect, buttons: &[ActionButton]) {
       let spans: Vec<Span> = buttons.iter()
           .flat_map(|b| vec![
               Span::styled(format!("[{}]", b.key), Style::default().fg(Color::Yellow)),
               Span::styled(format!(" {} ", b.label), b.style),
               Span::raw("  "),
           ])
           .collect();
       frame.render_widget(Paragraph::new(Line::from(spans)), area);
   }
   ```

2. Add `pub mod action_buttons;` to `tui/widgets/mod.rs`

3. Render the action bar below the dashboard kanban area (allocate 1-2 lines at bottom)

**Files**: `crates/kasmos/src/tui/widgets/action_buttons.rs` (new, ~40 lines)

### Subtask T019 – Implement state-based action filtering

**Purpose**: Determine which actions are valid for the currently selected WP based on its state.

**Steps**:
1. Implement `get_actions_for_state()`:
   ```rust
   pub fn get_actions_for_state(state: WPState) -> Vec<ActionButton> {
       match state {
           WPState::Active => vec![
               ActionButton { label: "Pause".into(), key: 'P', style: Style::default().fg(Color::Yellow) },
           ],
           WPState::Paused => vec![
               ActionButton { label: "Resume".into(), key: 'P', style: Style::default().fg(Color::Green) },
           ],
           WPState::Failed => vec![
               ActionButton { label: "Restart".into(), key: 'R', style: Style::default().fg(Color::Green) },
               ActionButton { label: "Retry".into(), key: 'T', style: Style::default().fg(Color::Cyan) },
               ActionButton { label: "Force-Advance".into(), key: 'F', style: Style::default().fg(Color::Red) },
           ],
           _ => vec![], // Pending, Completed, ForReview — no dashboard actions
       }
   }
   ```

2. Call this from the dashboard render function to populate the action bar based on the selected WP's state

**Files**: `crates/kasmos/src/tui/widgets/action_buttons.rs` (~20 lines)

### Subtask T020 – Wire action key dispatch

**Purpose**: When the operator presses an action key, construct the correct EngineAction and send it.

**Steps**:
1. In `tui/keybindings.rs`, extend `handle_dashboard_key()`:
   ```rust
   KeyCode::Char('R') => {
       if let Some(wp) = get_selected_wp(app) {
           if wp.state == WPState::Failed {
               let _ = app.action_tx.try_send(EngineAction::Restart(wp.id.clone()));
           }
       }
   }
   KeyCode::Char('P') => {
       if let Some(wp) = get_selected_wp(app) {
           match wp.state {
               WPState::Active => { let _ = app.action_tx.try_send(EngineAction::Pause(wp.id.clone())); }
               WPState::Paused => { let _ = app.action_tx.try_send(EngineAction::Resume(wp.id.clone())); }
               _ => {}
           }
       }
   }
   KeyCode::Char('F') => {
       if let Some(wp) = get_selected_wp(app) {
           if wp.state == WPState::Failed {
               app.pending_confirmation = Some(EngineAction::ForceAdvance(wp.id.clone()));
           }
       }
   }
   KeyCode::Char('T') => {
       if let Some(wp) = get_selected_wp(app) {
           if wp.state == WPState::Failed {
               let _ = app.action_tx.try_send(EngineAction::Retry(wp.id.clone()));
           }
       }
   }
   ```

2. Add helper `get_selected_wp(app: &App) -> Option<&WorkPackage>` that returns the WP at the current lane + index

3. Use `try_send` to avoid blocking the TUI event loop if the channel is full

**Files**: `crates/kasmos/src/tui/keybindings.rs` (~40 lines)

### Subtask T021 – Implement wave advance UI

**Purpose**: Show a global "Advance Wave" button when the orchestration is in wave-gated mode and paused at a wave boundary.

**Steps**:
1. Detect wave boundary pause: `app.run.state == RunState::Paused && app.run.mode == ProgressionMode::WaveGated`

2. When at a wave boundary, render an "Advance Wave" button in a prominent position (below action bar or in a status line):
   ```
   ⏸ Wave 2 complete — [A] Advance to Wave 3
   ```

3. Handle `KeyCode::Char('A')`:
   ```rust
   KeyCode::Char('A') => {
       if app.run.state == RunState::Paused && app.run.mode == ProgressionMode::WaveGated {
           let _ = app.action_tx.try_send(EngineAction::Advance);
       }
   }
   ```

**Files**: `crates/kasmos/src/tui/tabs/dashboard.rs` (~15 lines), `crates/kasmos/src/tui/keybindings.rs` (~5 lines)

### Subtask T022 – Add confirmation dialog for destructive actions

**Purpose**: Prevent accidental Force-Advance or Abort by requiring explicit confirmation.

**Steps**:
1. Add to App state:
   ```rust
   pub pending_confirmation: Option<EngineAction>,
   ```

2. When `pending_confirmation` is `Some`, render an overlay/inline prompt:
   ```
   Force-advance WP03? This skips review. [y] Yes  [n] No
   ```

3. Handle confirmation keys:
   ```rust
   if app.pending_confirmation.is_some() {
       match key.code {
           KeyCode::Char('y') => {
               if let Some(action) = app.pending_confirmation.take() {
                   let _ = app.action_tx.try_send(action);
               }
           }
           KeyCode::Char('n') | KeyCode::Esc => {
               app.pending_confirmation = None;
           }
           _ => {} // ignore other keys while confirming
       }
       return; // don't process other keybindings while dialog is active
   }
   ```

4. Also use confirmation for Abort (Ctrl+C or dedicated key)

**Files**: `crates/kasmos/src/tui/app.rs` (~3 lines), `crates/kasmos/src/tui/keybindings.rs` (~20 lines), `crates/kasmos/src/tui/tabs/dashboard.rs` (~15 lines for rendering)

## Risks & Mitigations

- **Channel full**: `try_send` returns `Err` if channel is full. Log the error but don't block the TUI. The mpsc channel should have sufficient capacity (default unbounded or at least 64).
- **Race between action send and state update**: After sending an action, the TUI state won't update until the engine processes it and broadcasts via watch. The button may still show briefly. This is expected and acceptable.

## Review Guidance

- Verify action buttons change based on WP state selection
- Test that Force-Advance shows confirmation dialog
- Verify wave advance only appears when paused at wave boundary
- Check that invalid actions are not dispatchable (e.g., can't Restart an Active WP)

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T10:33:05Z – coder – shell_pid=2981946 – lane=doing – Assigned agent via workflow command
- 2026-02-11T11:58:57Z – coder – shell_pid=2981946 – lane=for_review – Submitted for review via swarm
- 2026-02-11T11:58:57Z – reviewer – shell_pid=3253933 – lane=doing – Started review via workflow command
- 2026-02-11T12:00:56Z – reviewer – shell_pid=3253933 – lane=done – Review passed via swarm
