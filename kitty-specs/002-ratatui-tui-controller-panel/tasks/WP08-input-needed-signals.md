---
work_package_id: WP08
title: Input-Needed Signal Detection
lane: "done"
dependencies:
- WP05
base_branch: 002-ratatui-tui-controller-panel-WP05
base_commit: 34b410edfa6ebd5f11dbc8ff370b1e39570055cd
created_at: '2026-02-11T10:33:06.323164+00:00'
subtasks:
- T039
- T040
- T041
- T042
phase: Phase 3 - Advanced
assignee: 'unknown'
agent: "reviewer"
shell_pid: "3312537"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP08 – Input-Needed Signal Detection

## Objectives & Success Criteria

- TUI detects `.input-needed` marker files in WP worktrees on each tick (~1s)
- InputNeeded notifications appear in the notification bar with the agent's message
- Activating an InputNeeded notification focuses/zooms the agent's Zellij pane
- Notifications auto-clear when the agent removes the marker file
- FR-017, FR-018, FR-019: Detect signals, surface notifications, focus pane, auto-clear

**Implementation command**: `spec-kitty implement WP08 --base WP05`

## Context & Constraints

- **Research R7**: Agent writes `.input-needed` marker in WP worktree, content is the question/message
- **File path**: `{worktree_path}/.input-needed` — worktree_path from `WorkPackage.worktree_path`
- **SessionManager**: `zoom_pane(wp_id)` method for focus/zoom — from `crates/kasmos/src/session.rs`
- **App** holds `Arc<SessionManager>` (from data-model.md) — or alternatively, send a focus command via action_tx
- **Notification bar** from WP05 already handles InputNeeded kind
- **Spec clarification**: Operator sees message in TUI, then zooms pane to interact directly with agent

## Subtasks & Detailed Guidance

### Subtask T039 – Implement `.input-needed` marker file polling

**Purpose**: On each tick, check all active WP worktree paths for the `.input-needed` marker file.

**Steps**:
1. In `App::on_tick()`, add input-needed scanning:
   ```rust
   fn check_input_needed(&mut self) {
       for wp in &self.run.work_packages {
           if wp.state != WPState::Active {
               continue;  // Only check active WPs
           }
           let Some(worktree) = &wp.worktree_path else { continue };
           let marker_path = worktree.join(".input-needed");

           if marker_path.exists() {
               // Read agent's message from the file
               let message = std::fs::read_to_string(&marker_path).ok();

               // Add notification if not already present
               if !self.notifications.iter().any(|n| {
                   n.kind == NotificationKind::InputNeeded && n.wp_id == wp.id
               }) {
                   self.add_notification(NotificationKind::InputNeeded, &wp.id, message);
               }
           }
       }
   }
   ```

2. Call `check_input_needed()` from `on_tick()` — this runs every ~250ms (tick interval from WP01). Consider throttling to every 4th tick (~1s) to avoid excessive filesystem polling:
   ```rust
   pub fn on_tick(&mut self) {
       self.tick_count += 1;
       if self.tick_count % 4 == 0 {
           self.check_input_needed();
       }
   }
   ```

3. Add `tick_count: u64` to App state

**Files**: `crates/kasmos/src/tui/app.rs` (~30 lines)

**Notes**: File reads are cheap (single stat + small file read). Polling 50 WPs every second is negligible overhead.

### Subtask T040 – Surface InputNeeded notifications with agent's message

**Purpose**: InputNeeded notifications should display the agent's question in the notification bar and detail view.

**Steps**:
1. The `add_notification()` method already supports `message: Option<String>`. For InputNeeded, the message is the file content.

2. In the notification bar rendering (WP05), when InputNeeded notifications exist and have messages, show a truncated preview:
   ```
   [1 input needed: WP03 — "Which auth provider should I use?"]
   ```

3. When the operator presses `n` and lands on an InputNeeded notification, display the full message in a popup or status line before focusing the pane.

**Files**: `crates/kasmos/src/tui/widgets/notification_bar.rs` (~10 lines)

### Subtask T041 – Implement focus/zoom action for InputNeeded

**Purpose**: When the operator activates an InputNeeded notification, focus and zoom the agent's Zellij pane so they can interact directly.

**Steps**:
1. The App needs access to SessionManager for focus/zoom. Two approaches:
   - **Option A**: App holds `Arc<dyn SessionController>` and calls `zoom_pane()` directly
   - **Option B**: Send a new `EngineAction::FocusPane(wp_id)` and let the engine handle it

   **Prefer Option A** — focus/zoom is a UI concern, not an engine concern. The SessionManager already implements this.

2. Add to App:
   ```rust
   pub session: Option<Arc<dyn SessionController>>,
   ```

3. In notification jump handler (from WP05), for InputNeeded:
   ```rust
   NotificationKind::InputNeeded => {
       if let Some(session) = &app.session {
           let wp_id = notif.wp_id.clone();
           // Spawn a task to zoom — don't block the event loop
           let session = session.clone();
           tokio::spawn(async move {
               if let Err(e) = session.focus_and_zoom(&wp_id).await {
                   tracing::error!("Failed to zoom pane for {}: {}", wp_id, e);
               }
           });
       }
   }
   ```

4. Pass SessionManager to App during construction in launch.rs (update T011 in WP02 if needed)

**Files**: `crates/kasmos/src/tui/app.rs` (~5 lines), `crates/kasmos/src/tui/keybindings.rs` (~15 lines), `crates/kasmos/src/launch.rs` (~3 lines)

**Notes**: `SessionController` is an async trait (`async_trait`). The `tokio::spawn` ensures the zoom operation doesn't block the TUI event loop.

### Subtask T042 – Auto-clear InputNeeded notifications on marker removal

**Purpose**: When the agent resumes work and deletes its `.input-needed` marker, the notification should disappear.

**Steps**:
1. In `check_input_needed()`, also check for marker removal:
   ```rust
   // After checking for new markers, check for removed ones
   self.notifications.retain(|n| {
       if n.kind != NotificationKind::InputNeeded {
           return true;
       }
       let wp = self.run.work_packages.iter().find(|w| w.id == n.wp_id);
       let Some(wp) = wp else { return false };
       let Some(worktree) = &wp.worktree_path else { return false };
       worktree.join(".input-needed").exists()
   });
   ```

2. This runs on each input-needed poll cycle (~1s), so the notification clears within 1s of marker removal

**Files**: `crates/kasmos/src/tui/app.rs` (~10 lines)

## Risks & Mitigations

- **worktree_path may be None**: Some WPs (e.g., those not yet launched) won't have worktree paths. Skip them.
- **SessionManager not wired**: The `NoOpSessionCtrl` placeholder currently exists in launch.rs. If real SessionManager isn't wired yet, focus/zoom will silently succeed (NoOp). The TUI code should work regardless — just won't actually zoom.
- **Race condition**: Agent deletes marker while TUI is reading it. `read_to_string` returns error → treat as no marker (already handled by `.ok()`).

## Review Guidance

- Verify creating `.input-needed` in a WP worktree produces a notification within 2s
- Verify the notification shows the file's content as the message
- Verify activating the notification calls zoom_pane (check logs or mock)
- Verify deleting `.input-needed` clears the notification within 2s

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T10:33:06Z – coder – shell_pid=2982053 – lane=doing – Assigned agent via workflow command
- 2026-02-11T12:21:37Z – coder – shell_pid=2982053 – lane=for_review – WP08 input-needed signal detection complete: polling, notification bar, keybinding, status line
- 2026-02-11T12:22:02Z – reviewer – shell_pid=3312537 – lane=doing – Started review via workflow command
- 2026-02-11T12:24:02Z – reviewer – shell_pid=3312537 – lane=done – Review passed via swarm
