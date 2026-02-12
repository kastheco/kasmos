---
work_package_id: "WP03"
subtasks:
  - "T013"
  - "T014"
  - "T015"
  - "T016"
  - "T017"
title: "Adopt tui-popup — Confirmation Dialog Widget"
phase: "Phase 2 - Crate Adoptions"
lane: "done"
assignee: "coder"
agent: "reviewer"
shell_pid: "1333019"
review_status: "approved"
reviewed_by: "kas"
dependencies: ["WP01"]
history:
  - timestamp: "2026-02-12T00:00:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
  - timestamp: "2026-02-12T12:30:28Z"
    lane: "doing"
    agent: "coder"
    shell_pid: ""
    action: "Implementation started"
  - timestamp: "2026-02-12T12:40:00Z"
    lane: "done"
    agent: "reviewer"
    shell_pid: "1333019"
    action: "Review approved, accepted"
---

# Work Package Prompt: WP03 – Adopt tui-popup — Confirmation Dialog Widget

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
spec-kitty implement WP03 --base WP01
```

Depends on WP01 (ratatui-macros). New code should use macro syntax.

---

## Objectives & Success Criteria

1. Add `tui-popup` 0.7.x as a dependency.
2. Introduce `ConfirmAction` enum that models pending destructive action confirmations.
3. Add confirmation dialog flow: destructive actions (`F` ForceAdvance) now set `pending_confirm` instead of dispatching immediately.
4. Render centered, auto-sized popup with styled border and title when `pending_confirm.is_some()` (FR-005, FR-006).
5. Handle `y`/`n`/`Esc` to confirm or dismiss the popup (FR-007).
6. Popup auto-recenters on terminal resize (US2-AC4 — handled automatically by tui-popup).
7. **SC-003**: No manual coordinate calculation in application code.
8. **SC-005/SC-006**: `cargo test` and `cargo clippy` pass with zero regressions.

## Context & Constraints

- **Research R-4** (research.md): tui-popup API — `Popup::new(content).title(title).style(style)`, auto-centers in `frame.area()`.
- **data-model.md**: ConfirmAction enum definition, lifecycle (None → Some → None on y/n/Esc).
- **Spec FR-005**: Replace hand-rolled dialog with tui-popup Popup widget.
- **Edge case**: Popup must clamp to terminal dimensions. tui-popup handles this automatically via `frame.area()`. Keep action descriptions concise (1-2 lines).
- **Currently**: The `F` key in `keybindings.rs` dispatches `EngineAction::ForceAdvance` immediately (no confirmation). This must change.

### Files in scope

| File | Changes |
|------|---------|
| `crates/kasmos/Cargo.toml` | Add tui-popup dependency |
| `crates/kasmos/src/tui/app.rs` | Add ConfirmAction, pending_confirm, popup rendering |
| `crates/kasmos/src/tui/keybindings.rs` | Confirmation flow for destructive actions, popup key interception |

---

## Subtasks & Detailed Guidance

### Subtask T013 – Add `tui-popup` dependency to Cargo.toml

- **Purpose**: Introduce the popup widget crate.
- **Steps**:
  1. Add to `crates/kasmos/Cargo.toml` `[dependencies]`:
     ```toml
     tui-popup = "0.7"
     ```
  2. Run `cargo check -p kasmos` to verify resolution.
- **Files**: `crates/kasmos/Cargo.toml`
- **Notes**: tui-popup 0.7.x depends on `ratatui-core` and `ratatui-widgets` (ratatui 0.30 modular crates).

### Subtask T014 – Add `ConfirmAction` enum to `app.rs`

- **Purpose**: Model pending confirmation dialog actions as a typed enum.
- **Steps**:
  1. Add the enum near the other TUI state types in `crates/kasmos/src/tui/app.rs`:
     ```rust
     /// A pending confirmation dialog action.
     #[derive(Debug, Clone)]
     pub enum ConfirmAction {
         /// Force-advance a work package past its current state.
         ForceAdvance { wp_id: String },
         /// Abort the entire orchestration run.
         AbortRun,
     }
     
     impl ConfirmAction {
         /// Dialog title for the confirmation popup.
         pub fn title(&self) -> &str {
             match self {
                 ConfirmAction::ForceAdvance { .. } => "Confirm Force Advance",
                 ConfirmAction::AbortRun => "Confirm Abort",
             }
         }
     
         /// Dialog body text describing the action and consequences.
         pub fn description(&self) -> String {
             match self {
                 ConfirmAction::ForceAdvance { wp_id } => {
                     format!(
                         "Force-advance {} past its current state?\n\
                          This skips remaining work. Press [y] to confirm, [n] to cancel.",
                         wp_id
                     )
                 }
                 ConfirmAction::AbortRun => {
                     "Abort the entire orchestration run?\n\
                      All active work packages will be stopped. Press [y] to confirm, [n] to cancel."
                         .to_string()
                 }
             }
         }
     }
     ```
- **Files**: `crates/kasmos/src/tui/app.rs`

### Subtask T015 – Add `pending_confirm` field to `App` struct

- **Purpose**: Track whether a confirmation dialog is currently visible.
- **Steps**:
  1. Add field to `App` struct:
     ```rust
     /// Currently pending confirmation dialog, if any.
     pub pending_confirm: Option<ConfirmAction>,
     ```
  2. Initialize in `App::new()`:
     ```rust
     pending_confirm: None,
     ```
- **Files**: `crates/kasmos/src/tui/app.rs`

### Subtask T016 – Add popup rendering overlay in `App::render()`

- **Purpose**: Render a centered confirmation popup on top of the regular TUI content when `pending_confirm` is active.
- **Steps**:
  1. At the **end** of `App::render()`, after the tab body rendering, add:
     ```rust
     // Confirmation popup overlay (renders on top of everything)
     if let Some(ref action) = self.pending_confirm {
         let popup = tui_popup::Popup::new(action.description())
             .title(action.title())
             .style(Style::default().fg(Color::White).bg(Color::Red));
         frame.render_widget(popup, frame.area());
     }
     ```
  2. The popup renders **after** the body content, so it overlays on top. tui-popup auto-centers within the provided area.
  3. Use macro syntax for any text construction (from WP01).
- **Files**: `crates/kasmos/src/tui/app.rs`
- **Parallel?**: Yes — rendering changes (app.rs) can be done in parallel with keybinding changes (keybindings.rs, T017).
- **Notes**:
  - `tui_popup::Popup::new()` accepts `impl Into<Text>`. A string with `\n` will produce multi-line content.
  - Check the exact import path — it may be `tui_popup::Popup` or `tui_popup::popup::Popup`.
  - The `.style()` sets the popup body style. The border and title styling may have separate methods — check the crate API.

### Subtask T017 – Route destructive actions through confirmation flow + popup key handling

- **Purpose**: Change destructive action keys to set `pending_confirm` instead of dispatching immediately. Add popup-active key interception.
- **Steps**:
  1. In `crates/kasmos/src/tui/keybindings.rs`, add popup interception at the **top** of `handle_key()`, before global keys:
     ```rust
     pub fn handle_key(app: &mut App, key: KeyEvent) {
         // --- Popup confirmation interception (highest priority) ---
         if let Some(ref action) = app.pending_confirm.clone() {
             match key.code {
                 KeyCode::Char('y') | KeyCode::Char('Y') => {
                     // Execute the confirmed action
                     match action {
                         ConfirmAction::ForceAdvance { wp_id } => {
                             let _ = app.action_tx.try_send(EngineAction::ForceAdvance(wp_id));
                         }
                         ConfirmAction::AbortRun => {
                             // TODO: Add AbortRun engine action if not yet implemented
                         }
                     }
                     app.pending_confirm = None;
                     return;
                 }
                 KeyCode::Char('n') | KeyCode::Char('N') | KeyCode::Esc => {
                     app.pending_confirm = None;
                     return;
                 }
                 _ => return, // Swallow all other keys while popup is visible
             }
         }
         
         // --- Global keys (existing code below) ---
         match key.code {
             // ... existing global handlers ...
         }
     }
     ```
  2. Change the `F` key handler in `handle_dashboard_key()` to set `pending_confirm` instead of dispatching:
     ```rust
     KeyCode::Char('F') => {
         if let Some(wp) = selected_wp(app) {
             if wp.state == WPState::Failed {
                 app.pending_confirm = Some(ConfirmAction::ForceAdvance {
                     wp_id: wp.id.clone(),
                 });
             }
         }
     }
     ```
  3. Import `ConfirmAction` in keybindings.rs:
     ```rust
     use super::app::{App, ConfirmAction, Tab};
     ```
  4. Ensure the popup blocks ALL key events while visible — the `_ => return` in the interception block handles this.

- **Files**: `crates/kasmos/src/tui/keybindings.rs`
- **Parallel?**: Yes — different file from T016.
- **Notes**:
  - The existing test `test_action_keys_dispatch_correct_engine_action` (app.rs line ~1433) sends `KeyCode::Char('F')` and expects an immediate `EngineAction::ForceAdvance`. This test must be updated to:
    1. Press `F` → verify `pending_confirm` is `Some(ForceAdvance)`
    2. Press `y` → verify `EngineAction::ForceAdvance` is dispatched
    3. Verify `pending_confirm` is `None` after dismissal
  - Also add a test for dismissal: Press `F`, then `n` → verify no action dispatched, `pending_confirm` is `None`.

---

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Existing ForceAdvance tests break | Certain | T017 updates them to confirm flow |
| tui-popup API slightly different from R-4 | Low | Verify API in crate docs before implementing |
| Popup content exceeds terminal | Very Low | Keep descriptions to 1-2 lines; tui-popup truncates |
| Clone of `pending_confirm` in key handler | Low | ConfirmAction is small — Clone is cheap |

## Review Guidance

- **Confirm flow**: Press `F` on a Failed WP → popup appears → `y` dispatches, `n` dismisses. Verify both paths.
- **Popup blocks keys**: While popup is visible, `q` (quit), tab keys, navigation keys should all be swallowed.
- **Visual check**: Popup should be centered, have a red background, visible title, multi-line description.
- **No manual centering**: `grep -rn 'centered.*Rect\|Clear' crates/kasmos/src/tui/` should show zero results (SC-003).
- **Tests pass**: `cargo test -p kasmos` — especially the updated ForceAdvance test.

## Activity Log

- 2026-02-12T00:00:00Z – system – lane=planned – Prompt created.
- 2026-02-12T12:30:28Z – claude – lane=doing – Implementation complete, moving to doing for review
- 2026-02-12T12:30:49Z – coder – lane=doing – Reassign agent to coder for review-cycle compatibility
- 2026-02-12T12:31:19Z – coder – lane=for_review – Submitted for review via swarm
- 2026-02-12T12:31:19Z – reviewer – shell_pid=1302302 – lane=doing – Started review via workflow command
- 2026-02-12T12:35:15Z – coder – shell_pid=1302302 – lane=doing – Feedback addressed, resubmitting
- 2026-02-12T12:35:29Z – coder – shell_pid=1302302 – lane=for_review – Submitted for review via swarm
- 2026-02-12T12:35:29Z – reviewer – shell_pid=1333019 – lane=doing – Started review via workflow command
- 2026-02-12T12:40:00Z – reviewer – shell_pid=1333019 – lane=done – Review approved, accepted
