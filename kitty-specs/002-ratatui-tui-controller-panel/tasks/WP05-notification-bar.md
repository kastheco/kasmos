---
work_package_id: WP05
title: Notification Bar
lane: "done"
dependencies:
- WP01
subtasks:
- T023
- T024
- T025
- T026
- T027
phase: Phase 2 - Core Views
assignee: 'unknown'
agent: "reviewer"
shell_pid: "2949930"
review_status: "has_feedback"
reviewed_by: "kas"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-11T03:57:00Z'
  lane: for_review
  agent: claude-sonnet-4-5
  shell_pid: ''
  action: WP05 implementation complete, moved to for_review
---

# Work Package Prompt: WP05 – Notification Bar

## Objectives & Success Criteria

- Persistent notification bar renders across all tabs at the top of the TUI (below tab header)
- Notifications auto-generated when WPs enter Failed, ForReview, or InputNeeded states
- Visual distinction between notification types (color-coded)
- `n` key cycles through notifications, jumping to the relevant tab and WP
- Notifications auto-dismiss when the triggering condition resolves

**Implementation command**: `spec-kitty implement WP05 --base WP02`

## Context & Constraints

- **Notification types**: ReviewPending (cyan), Failure (red), InputNeeded (yellow) — from data-model.md
- **App.notifications**: `Vec<Notification>` with id, kind, wp_id, message, created_at
- **State diffing**: Compare previous OrchestrationRun with new one on each watch update
- **FR-007**: Persistent notification bar across all tabs with counts and identifiers
- **FR-008**: Three attention types visually distinguished
- **FR-009**: Keybinding to jump from notification to relevant WP

## Subtasks & Detailed Guidance

### Subtask T023 – Create `tui/widgets/notification_bar.rs`

**Purpose**: Render a 1-line notification strip at the top of the frame, visible across all tabs.

**Steps**:
1. Create `crates/kasmos/src/tui/widgets/notification_bar.rs`:
   ```rust
   pub fn render_notification_bar(frame: &mut Frame, area: Rect, notifications: &[Notification]) {
       if notifications.is_empty() {
           let idle = Paragraph::new(" No alerts")
               .style(Style::default().fg(Color::DarkGray));
           frame.render_widget(idle, area);
           return;
       }

       let review_count = notifications.iter().filter(|n| matches!(n.kind, NotificationKind::ReviewPending)).count();
       let fail_count = notifications.iter().filter(|n| matches!(n.kind, NotificationKind::Failure)).count();
       let input_count = notifications.iter().filter(|n| matches!(n.kind, NotificationKind::InputNeeded)).count();

       let mut spans = vec![];
       if review_count > 0 {
           spans.push(Span::styled(format!(" {} review ", review_count), Style::default().fg(Color::Black).bg(Color::Cyan)));
           spans.push(Span::raw(" "));
       }
       if fail_count > 0 {
           spans.push(Span::styled(format!(" {} failed ", fail_count), Style::default().fg(Color::White).bg(Color::Red)));
           spans.push(Span::raw(" "));
       }
       if input_count > 0 {
           spans.push(Span::styled(format!(" {} input needed ", input_count), Style::default().fg(Color::Black).bg(Color::Yellow)));
       }
       spans.push(Span::styled("  [n] next", Style::default().fg(Color::DarkGray)));

       frame.render_widget(Paragraph::new(Line::from(spans)), area);
   }
   ```

2. Add `pub mod notification_bar;` to `tui/widgets/mod.rs`

3. Update `App::render()` layout to allocate 1 line for notification bar between tab header and body

**Files**: `crates/kasmos/src/tui/widgets/notification_bar.rs` (new, ~50 lines)

### Subtask T024 – Implement notification diffing on state update

**Purpose**: When a new OrchestrationRun snapshot arrives via watch channel, detect which WPs have entered attention-requiring states.

**Steps**:
1. In `App::update_state()`, before replacing `self.run`:
   ```rust
   pub fn update_state(&mut self, new_run: OrchestrationRun) {
       // Diff for new notifications
       for new_wp in &new_run.work_packages {
           let old_wp = self.run.work_packages.iter().find(|w| w.id == new_wp.id);
           let old_state = old_wp.map(|w| w.state);

           // New failure
           if new_wp.state == WPState::Failed && old_state != Some(WPState::Failed) {
               self.add_notification(NotificationKind::Failure, &new_wp.id, None);
           }
           // New review
           if new_wp.state == WPState::ForReview && old_state != Some(WPState::ForReview) {
               self.add_notification(NotificationKind::ReviewPending, &new_wp.id, None);
           }
       }

       // Auto-dismiss resolved notifications
       self.dismiss_resolved(&new_run);

       self.run = new_run;
   }
   ```

2. Use `notification_counter` (auto-incrementing u64) for unique notification IDs

**Files**: `crates/kasmos/src/tui/app.rs` (~30 lines)

### Subtask T025 – Render notification counts with visual distinction

**Purpose**: Each notification type has distinct visual styling so the operator can quickly identify what needs attention.

**Steps**:
1. Styling from T023 already covers the basics. Extend with WP identifiers when count is small (<=3):
   ```
   [1 review: WP03] [2 failed: WP05 WP07] [n] next
   ```

2. When count exceeds 3, show just the count:
   ```
   [1 review] [5 failed] [n] next
   ```

**Files**: `crates/kasmos/src/tui/widgets/notification_bar.rs` (~15 lines)

### Subtask T026 – Implement notification jump (`n` key)

**Purpose**: Pressing `n` cycles through active notifications, switching to the relevant tab and focusing the WP.

**Steps**:
1. Track `notification_cursor: usize` on App

2. In keybindings, handle `n`:
   ```rust
   KeyCode::Char('n') => {
       if !app.notifications.is_empty() {
           app.notification_cursor = (app.notification_cursor + 1) % app.notifications.len();
           let notif = &app.notifications[app.notification_cursor];
           match notif.kind {
               NotificationKind::Failure => {
                   app.active_tab = Tab::Dashboard;
                   // Focus the failed WP in the dashboard
                   app.focus_wp_in_dashboard(&notif.wp_id);
               }
               NotificationKind::ReviewPending => {
                   app.active_tab = Tab::Review;
                   app.focus_wp_in_review(&notif.wp_id);
               }
               NotificationKind::InputNeeded => {
                   // Focus/zoom handled in WP08
               }
           }
       }
   }
   ```

3. Implement `App::focus_wp_in_dashboard(wp_id)` — find the WP's lane and index, update `dashboard.focused_lane` and `dashboard.selected_index`

**Files**: `crates/kasmos/src/tui/keybindings.rs` (~15 lines), `crates/kasmos/src/tui/app.rs` (~20 lines)

### Subtask T027 – Auto-dismiss notifications when resolved

**Purpose**: Remove stale notifications when the WP is no longer in the triggering state.

**Steps**:
1. Implement `App::dismiss_resolved()`:
   ```rust
   fn dismiss_resolved(&mut self, new_run: &OrchestrationRun) {
       self.notifications.retain(|notif| {
           let wp = new_run.work_packages.iter().find(|w| w.id == notif.wp_id);
           match (&notif.kind, wp.map(|w| w.state)) {
               (NotificationKind::Failure, Some(state)) => state == WPState::Failed,
               (NotificationKind::ReviewPending, Some(state)) => state == WPState::ForReview,
               (NotificationKind::InputNeeded, _) => true, // Managed separately via file polling (WP08)
               (_, None) => false, // WP no longer exists
           }
       });
   }
   ```

2. Call `dismiss_resolved()` from `update_state()` before updating `self.run`

**Files**: `crates/kasmos/src/tui/app.rs` (~15 lines)

## Risks & Mitigations

- **Notification spam**: Rapid state transitions (e.g., WP fails, restarts, fails again) could create duplicate notifications. Use wp_id + kind as dedup key — don't add if one already exists for the same wp+kind.
- **Cursor out of bounds**: When notifications are dismissed, `notification_cursor` may exceed `notifications.len()`. Reset to 0 when list shrinks.

## Review Guidance

- Verify notification bar shows correct counts when WPs fail or enter review
- Test `n` key navigates to correct tab and focuses the right WP
- Verify auto-dismiss: restart a failed WP and confirm the failure notification disappears
- Check styling distinction between review (cyan), failure (red), input-needed (yellow)

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T07:14:39Z – coder – shell_pid=1444681 – lane=doing – Started review via workflow command
- 2026-02-11T07:29:13Z – coder – shell_pid=1444681 – lane=planned – Tiered review NEEDS_CHANGES via krdone
- 2026-02-11T07:30:15Z – coder – shell_pid=1444681 – lane=doing – Started implementation via workflow command
- 2026-02-11T08:06:35Z – coder – shell_pid=1444681 – lane=for_review – Ready for review: Complete notification bar implementation with state diffing, auto-dismiss, and 'n' key navigation
- 2026-02-11T08:34:32Z – coder – shell_pid=1444681 – lane=planned – Tiered review BLOCKED via krdone
- 2026-02-11T10:08:42Z – coder – shell_pid=2910234 – lane=doing – Started implementation via workflow command
- 2026-02-11T10:21:39Z – coder – shell_pid=2910234 – lane=for_review – Submitted for review via swarm
- 2026-02-11T10:21:40Z – reviewer – shell_pid=2949930 – lane=doing – Started review via workflow command
- 2026-02-11T10:24:41Z – reviewer – shell_pid=2949930 – lane=for_review – Moved to for_review
- 2026-02-11T10:26:39Z – reviewer – shell_pid=2949930 – lane=done – Moved to done
