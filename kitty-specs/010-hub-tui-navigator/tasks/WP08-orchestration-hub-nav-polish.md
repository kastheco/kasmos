---
work_package_id: WP08
title: Orchestration TUI Hub Nav & Edge Cases & Polish
lane: "done"
dependencies:
- WP03
subtasks:
- T038
- T039
- T040
- T041
- T042
- T043
- T044
phase: Phase 5 - Polish
assignee: coder-wp08
agent: "reviewer-wp08"
shell_pid: "305200"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-13T03:53:23Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-13T08:53:00Z'
  lane: for_review
  agent: coder-wp08
  shell_pid: ''
  action: 'Implementation completed - All subtasks (T038-T044) implemented and tested: Alt+h keybinding in orchestration TUI, hub tab detection with navigate_to_hub(), edge case handling (no kitty-specs/, read-only mode without Zellij, no WP files, narrow terminal <60x10), and polish (status bar with refresh time, help footer, visual distinction for complete/running features with color coding). All 262 library tests + 92 binary tests passing including 88 hub-specific tests. Build successful with no errors.'
---

# Work Package Prompt: WP08 - Orchestration TUI Hub Nav & Edge Cases & Polish

## Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** -- Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Markdown Formatting
Wrap HTML/XML tags in backticks: `` `<div>` ``, `` `<script>` ``
Use language identifiers in code blocks: ````rust`, ````bash`

---

## Objectives & Success Criteria

- Add Alt+h keybinding to orchestration TUI that opens/switches to hub tab (AD-005)
- Handle all edge cases: no kitty-specs/, no Zellij, narrow terminal, no WPs
- Polish hub UX: status bar, help footer, visual distinction for complete features
- Operators can navigate from orchestration TUI to hub within 2 keystrokes (SC-008)
- Hub renders correctly at 80x24 minimum terminal size (NFR-004)
- All edge cases display appropriate messages

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-005: Orchestration TUI -> Hub Navigation)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-015, FR-017, User Story 7, Edge Cases)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md` (HubView navigation diagram)
- **Dependencies**: WP03 (hub app core), WP06 (agent pane launch), WP07 (implementation launch)
- **Key source files**:
  - `crates/kasmos/src/tui/keybindings.rs` -- orchestration TUI keybindings (add Alt+h)
  - `crates/kasmos/src/tui/app.rs` -- orchestration TUI app (add hub action)

### Key Architectural Decisions

- **AD-005**: Alt+h in orchestration TUI queries tab names for "hub" or "kasmos-hub". If found, go-to. If not, new-tab with `kasmos`.
- Inside-session Zellij commands (same as hub actions -- no `--session` flag)

## Subtasks & Detailed Guidance

### Subtask T038 - Add Alt+h keybinding to orchestration TUI

- **Purpose**: Allow operators to navigate from orchestration TUI to the hub.
- **Steps**:
  1. Open `crates/kasmos/src/tui/keybindings.rs`
  2. Add handling for `Alt+h` (`KeyCode::Char('h')` with `KeyModifiers::ALT`):
     ```rust
     KeyCode::Char('h') if key_event.modifiers.contains(KeyModifiers::ALT) => {
         // Open or switch to hub tab
         app.open_hub_requested = true;
     }
     ```
  3. Add `pub open_hub_requested: bool` field to the orchestration `App` struct in `crates/kasmos/src/tui/app.rs`
  4. In the orchestration TUI event loop (`crates/kasmos/src/tui/mod.rs`), after handling events:
     ```rust
     if app.open_hub_requested {
         app.open_hub_requested = false;
         // Spawn async task to handle hub navigation
         tokio::spawn(async {
             if let Err(e) = hub_navigation().await {
                 tracing::warn!("Failed to open hub: {}", e);
             }
         });
     }
     ```

- **Files**: `crates/kasmos/src/tui/keybindings.rs`, `crates/kasmos/src/tui/app.rs`, `crates/kasmos/src/tui/mod.rs`
- **Parallel?**: Yes (independent of T040-T044)
- **Notes**: The orchestration TUI runs inside a Zellij pane, so it uses the same inside-session Zellij commands as the hub. Alt+h is unlikely to conflict with Zellij defaults (Zellij uses Ctrl+p for pane mode, Ctrl+t for tab mode).

### Subtask T039 - Implement hub tab detection

- **Purpose**: Detect whether a hub tab already exists and navigate to it or create one.
- **Steps**:
  1. Create a helper function (can live in `crates/kasmos/src/hub/actions.rs` or a shared location):
     ```rust
     pub async fn open_or_switch_to_hub() -> anyhow::Result<()> {
         let tab_names = query_tab_names().await.unwrap_or_default();

         // Check for existing hub tab
         let hub_tab = tab_names.iter().find(|name| {
             let lower = name.to_lowercase();
             lower == "hub" || lower == "kasmos-hub" || lower.contains("kasmos hub")
         });

         if let Some(tab_name) = hub_tab {
             // Switch to existing hub tab
             go_to_tab(tab_name).await?;
         } else {
             // Create new hub tab
             open_new_tab("hub", "kasmos", &[]).await?;
         }

         Ok(())
     }
     ```
  2. Wire this into the orchestration TUI's hub navigation (T038)

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes (can proceed alongside T038)
- **Notes**: The tab name matching is case-insensitive and checks multiple patterns to be robust. The new tab runs bare `kasmos` which launches the hub TUI (from WP01).

### Subtask T040 - Handle no kitty-specs/ edge case

- **Purpose**: Display a helpful message when `kitty-specs/` doesn't exist.
- **Steps**:
  1. In `crates/kasmos/src/hub/app.rs`, in the `render()` method:
     - If `self.features.is_empty()` and the `kitty-specs/` directory doesn't exist:
       - Display centered message: "No kitty-specs/ directory found"
       - Below: "Create one with: mkdir kitty-specs"
       - Or: "Run spec-kitty init to set up your project"
  2. In `crates/kasmos/src/hub/mod.rs`, check if `kitty-specs/` exists before scanning:
     - If not, still launch the hub but with an empty feature list and a flag indicating the directory is missing

- **Files**: `crates/kasmos/src/hub/app.rs`, `crates/kasmos/src/hub/mod.rs`
- **Parallel?**: Yes (independent edge case)
- **Notes**: The hub should still be interactive (Alt+q to quit, n to create new feature). The missing directory message should be visually distinct (e.g., yellow warning color).

### Subtask T041 - Handle no Zellij session edge case

- **Purpose**: Operate in read-only mode when running outside Zellij.
- **Steps**:
  1. This is partially implemented in WP03 (T017) -- the `is_read_only()` check
  2. Enhance the read-only mode:
     - Show a persistent warning banner at the top: "Read-only mode -- not running inside Zellij. Actions requiring panes/tabs are unavailable."
     - When any action key is pressed, show status message: "Cannot launch panes outside Zellij. Run kasmos inside a Zellij session."
     - Disable action-related keybindings (Enter on actions, n for new feature)
     - Keep navigation working (j/k, detail view, refresh)
  3. The warning banner should be styled with `Color::Yellow` background

- **Files**: `crates/kasmos/src/hub/app.rs`, `crates/kasmos/src/hub/keybindings.rs`
- **Parallel?**: Yes (independent edge case)

### Subtask T042 - Handle no WP files edge case

- **Purpose**: Show error when Start is attempted on a feature with no WP files.
- **Steps**:
  1. In action resolution (WP05), `StartContinuous`/`StartWaveGated` are only offered when `TaskProgress::InProgress`
  2. Add an explicit check: if the operator somehow triggers Start on a feature with `TaskProgress::NoTasks`:
     - Show status message: "No work packages found. Run 'Generate Tasks' first."
  3. In the detail view, when a feature has no tasks:
     - Show message: "No work packages. Generate tasks to begin implementation."
     - Highlight the "Generate Tasks" action if available

- **Files**: `crates/kasmos/src/hub/app.rs`, `crates/kasmos/src/hub/keybindings.rs`
- **Parallel?**: Yes (independent edge case)

### Subtask T043 - Handle narrow terminal edge case

- **Purpose**: Gracefully handle terminals smaller than 80x24.
- **Steps**:
  1. In `crates/kasmos/src/hub/app.rs`, at the start of `render()`:
     ```rust
     let size = frame.area();
     if size.width < 80 || size.height < 24 {
         // Render minimal "terminal too small" message
         let msg = Paragraph::new("Terminal too small. Minimum: 80x24")
             .style(Style::default().fg(Color::Red))
             .alignment(Alignment::Center);
         frame.render_widget(msg, size);
         return;
     }
     ```
  2. For side-by-side pane splits: if the terminal width is less than 160 columns (80 for hub + 80 for agent pane), consider warning the operator or falling back to opening the agent in a new tab instead of a side pane
  3. Add a check before `open_pane_right()`:
     ```rust
     let (cols, _rows) = crossterm::terminal::size()?;
     if cols < 160 {
         // Fall back to new tab instead of side pane
         open_new_tab(&pane_name, command, args).await?;
     } else {
         open_pane_right(&pane_name, command, args, cwd).await?;
     }
     ```

- **Files**: `crates/kasmos/src/hub/app.rs`, `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes (independent edge case)
- **Notes**: NFR-004 requires graceful handling at 80x24 minimum. The "terminal too small" message is a last resort -- the hub should render a minimal view at 80x24.

### Subtask T044 - Polish: status bar, help footer, visual distinction

- **Purpose**: Add finishing touches to the hub UX.
- **Steps**:
  1. **Status bar** (bottom of screen, above help footer):
     - Show: current view name, feature count, last refresh time
     - Example: "List View | 12 features | Last refresh: 3s ago"
     - Show status messages (from action dispatch) that auto-clear after 5 seconds
  2. **Help footer** (very bottom):
     - List view: `j/k:nav  Enter:select  n:new  r:refresh  Alt+q:quit`
     - Detail view: `j/k:nav  Enter:action  Esc:back  r:refresh  Alt+q:quit`
     - Read-only mode: `j/k:nav  Enter:details  r:refresh  Alt+q:quit  (read-only)`
  3. **Visual distinction for complete features**:
     - Features with `TaskProgress::Complete` should be dimmed (lower contrast) or marked with a checkmark
     - Use `Style::default().fg(Color::DarkGray)` for dimming
     - Or prefix with a checkmark character: `[001] my-feature  [complete]`
  4. **Running features**:
     - Features with `OrchestrationStatus::Running` should have a bright indicator
     - Use `Style::default().fg(Color::Green).add_modifier(Modifier::BOLD)` for the running indicator
  5. Add `last_refresh: std::time::Instant` field to `App` for "last refresh" display

- **Files**: `crates/kasmos/src/hub/app.rs`
- **Parallel?**: No (integrates with all previous rendering work)
- **Notes**: The status bar and help footer use `ratatui::layout::Layout` with `Constraint::Length(1)` for each. Status messages should be stored with a timestamp and auto-cleared after 5 seconds in the `on_tick()` handler.

## Test Strategy

- **Manual testing**:
  - Alt+h from orchestration TUI opens hub tab
  - Alt+h again switches to existing hub tab (no duplicate)
  - Hub renders "No kitty-specs/" when directory is missing
  - Hub shows read-only warning outside Zellij
  - Hub shows "terminal too small" at very small sizes
  - Complete features are visually dimmed
  - Status bar shows correct information
- **Unit tests**: Test `open_or_switch_to_hub()` tab name matching logic

## Risks & Mitigations

- **Alt+h conflict with Zellij**: Unlikely -- Zellij uses Ctrl-based shortcuts. Verify on target platform.
- **Terminal size detection**: `crossterm::terminal::size()` may return incorrect values in some edge cases. Mitigation: use `frame.area()` which is always accurate during rendering.
- **Status message timing**: Use `Instant::elapsed()` for auto-clear timing, not wall clock.

## Review Guidance

- Verify Alt+h works in orchestration TUI without Zellij key conflicts
- Verify hub tab detection handles case-insensitive matching
- Verify all edge cases display appropriate messages
- Verify minimum terminal size handling (80x24)
- Verify visual distinction for complete and running features
- Verify status bar and help footer are accurate for each view

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T09:08:50Z – reviewer-wp08 – shell_pid=305200 – lane=doing – Started review via workflow command
- 2026-02-13T09:13:57Z – reviewer-wp08 – shell_pid=305200 – lane=done – Review passed: Fixed critical double-init panic (try_init revert), hub tab name casing, detector variable naming. All 354 tests pass. T038-T044 fully implemented.
