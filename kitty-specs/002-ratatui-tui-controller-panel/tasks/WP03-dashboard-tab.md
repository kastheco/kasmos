---
work_package_id: WP03
title: Dashboard Tab — Kanban Board
lane: "done"
dependencies:
- WP01
subtasks:
- T012
- T013
- T014
- T015
- T016
- T017
phase: Phase 2 - Core Views
assignee: ''
agent: 'claude-sonnet-4-5'
shell_pid: ''
review_status: pending
reviewed_by: ''
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-11T04:03:00Z'
  lane: for_review
  agent: claude-sonnet-4-5
  shell_pid: ''
  action: WP03 implementation complete - dashboard kanban board with 4 lanes, tab header, and wp_card widget
---

# Work Package Prompt: WP03 – Dashboard Tab — Kanban Board

## Objectives & Success Criteria

- Dashboard tab renders all WPs grouped into 4 kanban lanes: Planned, Doing, For Review, Done
- Each WP displays as a card with id, title, state badge, wave number, and elapsed time
- Vim-style navigation works: `h`/`l` between lanes, `j`/`k` within a lane
- Wave separators and overall progress summary are visible
- Tab header bar shows all 3 tabs with active tab highlighted

**Implementation command**: `spec-kitty implement WP03 --base WP02`

## Context & Constraints

- **Plan**: `kitty-specs/002-ratatui-tui-controller-panel/plan.md` — kanban layout, keybindings, contextual actions table
- **Data model**: `data-model.md` — DashboardState (focused_lane, selected_index, scroll_offsets)
- **App struct** from WP01: `crates/kasmos/src/tui/app.rs` — App, Tab, DashboardState already defined
- **WPState mapping**: Pending→planned, Active→doing, Paused→doing (badge), Failed→doing (badge), ForReview→for_review, Completed→done
- **OrchestrationRun fields**: `work_packages: Vec<WorkPackage>`, `waves: Vec<Wave>`, `mode: ProgressionMode`

## Subtasks & Detailed Guidance

### Subtask T012 – Create `tui/tabs/mod.rs` — tab rendering dispatch and header bar

**Purpose**: Establish the tab framework that dispatches rendering to the active tab and shows a header bar for tab switching.

**Steps**:
1. Create `crates/kasmos/src/tui/tabs/mod.rs`:
   ```rust
   pub mod dashboard;
   // review and logs added in WP06/WP07

   use ratatui::prelude::*;
   use ratatui::widgets::{Block, Borders, Tabs};

   pub fn render_tab_header(frame: &mut Frame, area: Rect, active: &super::app::Tab) {
       let titles = vec!["1:Dashboard", "2:Review", "3:Logs"];
       let selected = match active {
           super::app::Tab::Dashboard => 0,
           super::app::Tab::Review => 1,
           super::app::Tab::Logs => 2,
       };
       let tabs = Tabs::new(titles)
           .select(selected)
           .highlight_style(Style::default().fg(Color::Yellow).add_modifier(Modifier::BOLD))
           .divider(" | ");
       frame.render_widget(tabs, area);
   }
   ```

2. Add `pub mod tabs;` to `tui/mod.rs`

3. Update `App::render()` to split the frame into header area (1 line) + body area, call `render_tab_header()` for the header, then dispatch body rendering based on `active_tab`

**Files**: `crates/kasmos/src/tui/tabs/mod.rs` (new, ~30 lines), `crates/kasmos/src/tui/mod.rs` (edit)

### Subtask T013 – Create `tui/tabs/dashboard.rs` — 4-column kanban layout

**Purpose**: Render the kanban board with 4 vertical columns for each lane.

**Steps**:
1. Create `crates/kasmos/src/tui/tabs/dashboard.rs` with a `render()` function:
   ```rust
   pub fn render(frame: &mut Frame, area: Rect, app: &App) {
       // Split area into 4 equal columns
       let columns = Layout::horizontal([
           Constraint::Percentage(25),
           Constraint::Percentage(25),
           Constraint::Percentage(25),
           Constraint::Percentage(25),
       ]).split(area);

       let lanes = group_by_lane(&app.run.work_packages);

       for (i, (title, wps)) in lanes.iter().enumerate() {
           let is_focused = app.dashboard.focused_lane == i;
           let block = Block::default()
               .title(format!(" {} ({}) ", title, wps.len()))
               .borders(Borders::ALL)
               .border_style(if is_focused {
                   Style::default().fg(Color::Yellow)
               } else {
                   Style::default().fg(Color::DarkGray)
               });

           // Render WP cards within the column
           // ... (delegates to wp_card widget)
       }
   }
   ```

2. Each column shows its lane title and WP count in the border title

3. The focused lane has a highlighted border (yellow), unfocused lanes are dim

4. Within each column, render WP cards vertically stacked, scrollable

**Files**: `crates/kasmos/src/tui/tabs/dashboard.rs` (new, ~80 lines)

### Subtask T014 – Create `tui/widgets/wp_card.rs` — WP card widget

**Purpose**: Reusable widget that renders a single WP as a compact card showing essential info.

**Steps**:
1. Create `crates/kasmos/src/tui/widgets/mod.rs`:
   ```rust
   pub mod wp_card;
   // notification_bar and action_buttons added in WP04/WP05
   ```

2. Create `crates/kasmos/src/tui/widgets/wp_card.rs`:
   ```rust
   pub fn render_wp_card(frame: &mut Frame, area: Rect, wp: &WorkPackage, selected: bool) {
       let state_badge = match wp.state {
           WPState::Pending => ("PND", Color::DarkGray),
           WPState::Active => ("RUN", Color::Green),
           WPState::Paused => ("PSE", Color::Yellow),
           WPState::Failed => ("ERR", Color::Red),
           WPState::ForReview => ("REV", Color::Cyan),
           WPState::Completed => ("DON", Color::Blue),
       };

       // Layout: [badge] WP01 — Title  (W1, 2m30s)
       let line = Line::from(vec![
           Span::styled(format!("[{}]", state_badge.0), Style::default().fg(state_badge.1)),
           Span::raw(format!(" {} — {}", wp.id, wp.title)),
       ]);

       let style = if selected {
           Style::default().bg(Color::DarkGray).add_modifier(Modifier::BOLD)
       } else {
           Style::default()
       };

       frame.render_widget(Paragraph::new(line).style(style), area);
   }
   ```

3. Add `pub mod widgets;` to `tui/mod.rs`

4. Show elapsed time since `started_at` (if active/paused) and wave number

**Files**: `crates/kasmos/src/tui/widgets/wp_card.rs` (new, ~50 lines), `crates/kasmos/src/tui/widgets/mod.rs` (new)

### Subtask T015 – Implement WP-to-lane grouping logic

**Purpose**: Partition the work_packages vector into 4 lane groups for dashboard rendering.

**Steps**:
1. In `dashboard.rs`, implement `group_by_lane()`:
   ```rust
   fn group_by_lane(wps: &[WorkPackage]) -> [(&str, Vec<&WorkPackage>); 4] {
       let mut planned = vec![];
       let mut doing = vec![];
       let mut for_review = vec![];
       let mut done = vec![];

       for wp in wps {
           match wp.state {
               WPState::Pending => planned.push(wp),
               WPState::Active | WPState::Paused | WPState::Failed => doing.push(wp),
               WPState::ForReview => for_review.push(wp),
               WPState::Completed => done.push(wp),
           }
       }

       [
           ("Planned", planned),
           ("Doing", doing),
           ("For Review", for_review),
           ("Done", done),
       ]
   }
   ```

2. Sort WPs within each lane by wave number, then by id

**Files**: `crates/kasmos/src/tui/tabs/dashboard.rs` (~20 lines added)

**Notes**: Failed and Paused WPs stay in the "Doing" lane but show a state badge (ERR/PSE) so the operator knows their actual status.

### Subtask T016 – Implement dashboard navigation

**Purpose**: Enable vim-style keyboard navigation across lanes and within lanes.

**Steps**:
1. In `tui/keybindings.rs`, fill in the `handle_dashboard_key()` stubs:
   ```rust
   fn handle_dashboard_key(app: &mut App, key: KeyEvent) {
       let lane_counts = get_lane_counts(&app.run.work_packages);

       match key.code {
           KeyCode::Char('j') | KeyCode::Down => {
               let max = lane_counts[app.dashboard.focused_lane].saturating_sub(1);
               if app.dashboard.selected_index < max {
                   app.dashboard.selected_index += 1;
               }
           }
           KeyCode::Char('k') | KeyCode::Up => {
               app.dashboard.selected_index = app.dashboard.selected_index.saturating_sub(1);
           }
           KeyCode::Char('h') | KeyCode::Left => {
               if app.dashboard.focused_lane > 0 {
                   app.dashboard.focused_lane -= 1;
                   app.dashboard.selected_index = 0;
               }
           }
           KeyCode::Char('l') | KeyCode::Right => {
               if app.dashboard.focused_lane < 3 {
                   app.dashboard.focused_lane += 1;
                   app.dashboard.selected_index = 0;
               }
           }
           _ => {}
       }
   }
   ```

2. When switching lanes, reset `selected_index` to 0 and clamp to lane size

3. Handle scroll: when `selected_index` exceeds visible area height, adjust `scroll_offsets[lane]`

**Files**: `crates/kasmos/src/tui/keybindings.rs` (~30 lines)

### Subtask T017 – Render wave separators and progress summary

**Purpose**: Show wave groupings within lanes and an overall progress bar.

**Steps**:
1. In dashboard rendering, insert wave separator lines between WPs of different waves:
   ```
   ── Wave 1 ──────
   [RUN] WP01 — Core Types
   [RUN] WP02 — Spec Parser
   ── Wave 2 ──────
   [PND] WP03 — KDL Layouts
   ```

2. At the bottom of the dashboard (or in a status line), show progress:
   ```
   Progress: 5/11 WPs complete | Wave 2/6 | Mode: wave-gated
   ```

3. Use `app.run.waves` and `app.run.mode` for data

**Files**: `crates/kasmos/src/tui/tabs/dashboard.rs` (~25 lines)

## Risks & Mitigations

- **Empty lanes**: Must render cleanly with placeholder text ("No work packages") rather than collapsing
- **Terminal width**: 4 equal columns may be too narrow on small terminals. Consider a minimum width check and fallback to 2-column layout if terminal is <80 cols
- **Selected index out of bounds**: When state updates change lane membership, the selected_index may exceed the new lane size. Clamp it in `App::update_state()`

## Review Guidance

- Verify all 4 lanes render with correct WP grouping
- Test with 0, 1, and 10+ WPs to verify scrolling and layout
- Verify navigation wraps/clamps correctly at lane boundaries
- Check wave separator rendering matches wave assignments from OrchestrationRun

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T04:02:59Z – claude-sonnet-4-5 – lane=for_review – Implementation complete: Dashboard tab with 4-column kanban board, vim navigation, wave separators, and progress summary. All tests pass.
