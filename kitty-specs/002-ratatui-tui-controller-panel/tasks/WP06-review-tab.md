---
work_package_id: WP06
title: Review Tab
lane: "done"
dependencies:
- WP02
base_branch: 002-ratatui-tui-controller-panel-WP02
base_commit: a1c4a7e3bc679aa136bf5ea2a44b0e9bfe44ceee
created_at: '2026-02-11T10:33:05.930673+00:00'
subtasks:
- T028
- T029
- T030
- T031
- T032
- T033
- T057
- T058
- T059
- T060
phase: Phase 3 - Advanced
assignee: 'unknown'
agent: "reviewer"
shell_pid: "3448159"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP06 – Review Tab

## Objectives & Success Criteria

- Review tab lists all WPs in `ForReview` state with title, time in review, and wave
- Split layout: review queue list (left) + detail pane (right) showing review context
- Approve (`a`), Reject (`r`), and Request Changes (`c`) actions work correctly
- Reject prompts for auto-relaunch vs hold mode
- Review context from WP task file is displayed in the detail pane
- Automated tiered review can be triggered in `slash` mode or `prompt` mode from the review runner
- Default prompt-mode reviewer uses model `openai/gpt-5.3-codex` with high reasoning
- SC-007: Review workflow completes without leaving the TUI

**Implementation command**: `spec-kitty implement WP06 --base WP03`

## Context & Constraints

- **EngineAction variants**: `Approve(String)`, `Reject { wp_id, relaunch }` — added in WP02
- **WPState::ForReview** — added in WP02, transition Active→ForReview→Completed/Active/Pending
- **FR-010**: Auto-pause WPs at for_review, surface in Review tab with review context
- **FR-011**: Approve, reject (configurable relaunch/hold), request changes
- **FR-020**: Re-review action for request-changes flow
- **FR-021**: Support `slash` and `prompt` review trigger modes
- **FR-022**: Prompt-mode review is model-agnostic with default `openai/gpt-5.3-codex` + high reasoning
- **FR-025**: Surface automated review failures as notifications/log entries
- **Spec clarification**: "Request changes" keeps WP in for_review without relaunching — operator manually edits files, then re-triggers review

## Subtasks & Detailed Guidance

### Subtask T028 – Create `tui/tabs/review.rs` — split layout

**Purpose**: Render the review tab with a list pane on the left and a detail pane on the right.

**Steps**:
1. Create `crates/kasmos/src/tui/tabs/review.rs`:
   ```rust
   pub fn render(frame: &mut Frame, area: Rect, app: &App) {
       let chunks = Layout::horizontal([
           Constraint::Percentage(40),  // Review queue
           Constraint::Percentage(60),  // Detail pane
       ]).split(area);

       render_review_list(frame, chunks[0], app);
       render_review_detail(frame, chunks[1], app);
   }
   ```

2. Add `pub mod review;` to `tui/tabs/mod.rs`

3. Wire rendering dispatch in `App::render()` for `Tab::Review`

**Files**: `crates/kasmos/src/tui/tabs/review.rs` (new, ~100 lines)

### Subtask T029 – List ForReview WPs

**Purpose**: Show all WPs awaiting review with selection highlighting.

**Steps**:
1. Filter `app.run.work_packages` to those with `state == WPState::ForReview`

2. Render as a selectable list:
   ```
   Review Queue (2)
   ────────────────
   ▸ WP03 — KDL Layouts        [15m in review]
     WP05 — Session Manager     [3m in review]
   ```

3. Use `app.review.selected_index` for cursor position

4. Handle j/k navigation in `handle_review_key()`:
   ```rust
   fn handle_review_key(app: &mut App, key: KeyEvent) {
       let review_count = app.run.work_packages.iter()
           .filter(|wp| wp.state == WPState::ForReview).count();
       match key.code {
           KeyCode::Char('j') | KeyCode::Down => {
               if app.review.selected_index < review_count.saturating_sub(1) {
                   app.review.selected_index += 1;
               }
           }
           KeyCode::Char('k') | KeyCode::Up => {
               app.review.selected_index = app.review.selected_index.saturating_sub(1);
           }
           // Action keys handled in T030-T032
       }
   }
   ```

5. Show empty state when no WPs are in review: "No work packages awaiting review"

**Files**: `crates/kasmos/src/tui/tabs/review.rs` (~40 lines), `crates/kasmos/src/tui/keybindings.rs` (~10 lines)

### Subtask T030 – Implement approve action

**Purpose**: Pressing `a` on a selected review item approves the WP (moves to Completed).

**Steps**:
1. In `handle_review_key()`:
   ```rust
   KeyCode::Char('a') => {
       if let Some(wp) = get_selected_review_wp(app) {
           let _ = app.action_tx.try_send(EngineAction::Approve(wp.id.clone()));
       }
   }
   ```

2. The engine handles `Approve` by transitioning ForReview→Completed and launching dependents

3. The watch channel update will remove the WP from the review list automatically

**Files**: `crates/kasmos/src/tui/keybindings.rs` (~5 lines)

### Subtask T031 – Implement reject action with relaunch/hold choice

**Purpose**: Pressing `r` prompts the operator to choose between auto-relaunch (send back to agent immediately) and hold (manual restart later).

**Steps**:
1. In `handle_review_key()`:
   ```rust
   KeyCode::Char('r') => {
       if get_selected_review_wp(app).is_some() {
           app.review.reject_prompt_active = true;
       }
   }
   ```

2. Add to ReviewState:
   ```rust
   pub reject_prompt_active: bool,
   ```

3. When `reject_prompt_active`, render inline prompt:
   ```
   Reject WP03?  [r] Relaunch  [h] Hold  [Esc] Cancel
   ```

4. Handle sub-keys:
   ```rust
   if app.review.reject_prompt_active {
       match key.code {
           KeyCode::Char('r') => {
               if let Some(wp) = get_selected_review_wp(app) {
                   let _ = app.action_tx.try_send(EngineAction::Reject {
                       wp_id: wp.id.clone(), relaunch: true
                   });
               }
               app.review.reject_prompt_active = false;
           }
           KeyCode::Char('h') => {
               if let Some(wp) = get_selected_review_wp(app) {
                   let _ = app.action_tx.try_send(EngineAction::Reject {
                       wp_id: wp.id.clone(), relaunch: false
                   });
               }
               app.review.reject_prompt_active = false;
           }
           KeyCode::Esc => { app.review.reject_prompt_active = false; }
           _ => {}
       }
       return;
   }
   ```

**Files**: `crates/kasmos/src/tui/app.rs` (~2 lines), `crates/kasmos/src/tui/keybindings.rs` (~25 lines), `crates/kasmos/src/tui/tabs/review.rs` (~10 lines)

### Subtask T032 – Implement request-changes action

**Purpose**: Pressing `c` keeps the WP in ForReview and marks it for manual edits, showing a re-review option.

**Steps**:
1. This is a TUI-only concept — no EngineAction needed. The WP stays in ForReview.

2. Add to App or ReviewState:
   ```rust
   pub changes_requested: HashSet<String>,  // WP IDs with pending manual changes
   ```

3. In `handle_review_key()`:
   ```rust
   KeyCode::Char('c') => {
       if let Some(wp) = get_selected_review_wp(app) {
           app.review.changes_requested.insert(wp.id.clone());
       }
   }
   ```

4. For WPs in `changes_requested`, show different UI:
   ```
   WP03 — KDL Layouts  [changes requested — edit files, then re-review]
   ```

5. Add a re-review key (`v`) that clears the `changes_requested` flag. This is TUI-only state — pressing `v` simply removes the WP from `changes_requested` so it re-appears as a normal review item. The operator then re-reviews manually (approve/reject). No backend action or spec-kitty re-run is triggered automatically.

**Files**: `crates/kasmos/src/tui/app.rs` (~3 lines), `crates/kasmos/src/tui/keybindings.rs` (~10 lines), `crates/kasmos/src/tui/tabs/review.rs` (~15 lines)

### Subtask T033 – Display review feedback context

**Purpose**: The detail pane shows review context from the WP's task file so the operator has information to make review decisions.

**Steps**:
1. In the detail pane, display:
   - WP title and summary
   - Wave number and dependencies
   - Time in ForReview state
   - Review feedback section from the WP task file (if any)

2. Read the WP's task file to extract review context:
   ```rust
   fn load_review_context(wp: &WorkPackage) -> Option<String> {
       let task_file = wp.prompt_path.as_ref()?;
       let content = std::fs::read_to_string(task_file).ok()?;
       // Extract the "## Review Feedback" section
       let start = content.find("## Review Feedback")?;
       let end = content[start..].find("\n## ").map(|i| start + i).unwrap_or(content.len());
       Some(content[start..end].to_string())
   }
   ```

3. Cache the review context — don't re-read the file on every render. Refresh on tick (~1s) or on selection change.

4. Render as scrollable text in the detail pane, respecting `app.review.detail_scroll`

**Files**: `crates/kasmos/src/tui/tabs/review.rs` (~40 lines)

**Notes**: File reads should not happen on every frame — cache the result and refresh periodically or on selection change.

### Subtask T057 – Add ReviewRunner service with configurable trigger mode

**Purpose**: Centralize review automation policy and dispatch so review flow is not hardcoded in UI handlers.

**Steps**:
1. Add review automation config in kasmos config layer:
   - `mode`: `slash` or `prompt`
   - `slash_command`: default `/kas:verify`
   - `fallback_to_prompt`: bool
   - `model`: default `openai/gpt-5.3-codex`
   - `reasoning`: default `high`
   - `policy`: default `auto_then_manual_approve`
2. Create `ReviewRunner` service that accepts `(wp_id, worktree_path, pane_id)` and returns `ReviewResult`.
3. Invoke runner when WP enters `ForReview` and when operator triggers re-review.

### Subtask T058 – Implement slash mode command injection

**Purpose**: Reuse existing `/kas:verify` or `/kas:review` plugin flow with minimal operator typing.

**Steps**:
1. Add `SessionManager::write_chars_to_pane()` (or equivalent) wrapper around Zellij `write-chars` action.
2. In slash mode, inject configured command + newline into reviewer pane for the WP.
3. Capture command dispatch success/failure and persist to `ReviewResult`.

### Subtask T059 – Implement prompt mode tiered review via opencode

**Purpose**: Provide a portable fallback when slash commands/plugins are unavailable.

**Steps**:
1. Define built-in tiered review prompt (static checks -> reality checks -> simplifier summary).
2. Run reviewer via opencode with configured model/reasoning (default `openai/gpt-5.3-codex`, `high`).
3. Parse output into normalized `ReviewResult` fields (`Pass`/`Fail`/`Error`, summary, findings).

### Subtask T060 – Persist and render ReviewResult details

**Purpose**: Make review outcomes durable and visible across restarts/status/report commands.

**Steps**:
1. Persist latest `ReviewResult` per WP in run state or adjacent persistence file.
2. Show result metadata in Review tab detail pane (mode, status, summary, key findings, timestamp).
3. Emit notification/log entries on review errors/timeouts/parse failures.

## Risks & Mitigations

- **File I/O in render path**: Cache review context to avoid blocking the render loop. Read once on selection change, store in ReviewState.
- **Empty review queue**: When all reviews are handled, show a clear "No reviews pending" message.
- **Reject prompt overlapping**: If reject prompt is active, all other keys should be suppressed until resolved.
- **Slash command unavailable**: Detect command failure quickly and fallback to prompt mode when configured.

## Review Guidance

- Verify approve transitions WP from ForReview to Completed
- Verify reject with relaunch triggers WP re-execution (watch for state change back to Active)
- Verify reject with hold moves WP to Pending (idle until manual restart)
- Verify request-changes keeps WP in ForReview with visual indicator
- Check review context loads from task file and displays in detail pane
- Verify slash mode injects configured command into the correct reviewer pane
- Verify prompt mode defaults to `openai/gpt-5.3-codex` and high reasoning when unset

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T10:33:06Z – coder – shell_pid=2981989 – lane=doing – Assigned agent via workflow command
- 2026-02-11T13:22:26Z – coder – shell_pid=2981989 – lane=for_review – Submitted for review via swarm
- 2026-02-11T13:22:33Z – reviewer – shell_pid=3448159 – lane=doing – Started review via workflow command
- 2026-02-11T13:24:24Z – reviewer – shell_pid=3448159 – lane=done – Review passed via swarm
