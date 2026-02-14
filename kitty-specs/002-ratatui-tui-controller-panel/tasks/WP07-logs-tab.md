---
work_package_id: WP07
title: Logs Tab
lane: "done"
dependencies:
- WP01
subtasks:
- T034
- T035
- T036
- T037
- T038
phase: Phase 2 - Core Views
assignee: claude-sonnet-4-5
agent: "reviewer"
shell_pid: 'unknown'
review_status: "has_feedback"
reviewed_by: "reviewer"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-11T03:58:00Z'
  lane: for_review
  agent: claude-sonnet-4-5
  shell_pid: ''
  action: WP07 implementation complete - all subtasks done, committed to branch 002-ratatui-tui-controller-panel-WP07
---

# Work Package Prompt: WP07 – Logs Tab

## Objectives & Success Criteria

- Scrollable log viewer renders orchestration events with timestamps and level badges
- State transitions automatically generate log entries
- `/` activates text filter, `Esc` exits filter mode
- Auto-scroll follows new entries; manual scroll up pauses auto-scroll; `G` resumes
- Log levels are color-coded: Info (dim), Warn (yellow), Error (red)
- FR-014: Scrollable, filterable log view of orchestration events

**Implementation command**: `spec-kitty implement WP07 --base WP02`

## Context & Constraints

- **LogsState** from data-model.md: entries, filter, filter_active, scroll_offset, auto_scroll
- **LogEntry**: timestamp, level, wp_id (optional), message
- **LogLevel**: Info, Warn, Error
- **State source**: Log entries derived from OrchestrationRun diffs on each watch update

## Subtasks & Detailed Guidance

### Subtask T034 – Create `tui/tabs/logs.rs` — scrollable log list

**Purpose**: Render a scrollable list of log entries with timestamps and level indicators.

**Steps**:
1. Create `crates/kasmos/src/tui/tabs/logs.rs`:
   ```rust
   pub fn render(frame: &mut Frame, area: Rect, app: &App) {
       let block = Block::default().title(" Logs ").borders(Borders::ALL);
       let inner = block.inner(area);
       frame.render_widget(block, area);

       let entries = if app.logs.filter.is_empty() {
           &app.logs.entries
       } else {
           // Filtered view — computed from entries matching filter
       };

       // Calculate visible window based on scroll_offset
       let visible_height = inner.height as usize;
       let start = app.logs.scroll_offset;
       let end = (start + visible_height).min(entries.len());

       for (i, entry) in entries[start..end].iter().enumerate() {
           let y = inner.y + i as u16;
           let time_str = format_timestamp(entry.timestamp);
           let level_span = match entry.level {
               LogLevel::Info => Span::styled("INFO", Style::default().fg(Color::DarkGray)),
               LogLevel::Warn => Span::styled("WARN", Style::default().fg(Color::Yellow)),
               LogLevel::Error => Span::styled("ERR ", Style::default().fg(Color::Red)),
           };
           let wp_prefix = entry.wp_id.as_deref().map(|id| format!("[{}] ", id)).unwrap_or_default();
           let line = Line::from(vec![
               Span::styled(time_str, Style::default().fg(Color::DarkGray)),
               Span::raw(" "),
               level_span,
               Span::raw(format!(" {}{}", wp_prefix, entry.message)),
           ]);
           frame.render_widget(Paragraph::new(line), Rect::new(inner.x, y, inner.width, 1));
       }

       // Render filter bar at bottom if filter_active
       if app.logs.filter_active {
           render_filter_bar(frame, area, &app.logs.filter);
       }
   }
   ```

2. Add `pub mod logs;` to `tui/tabs/mod.rs`

**Files**: `crates/kasmos/src/tui/tabs/logs.rs` (new, ~80 lines)

### Subtask T035 – Implement log capture from state diffs

**Purpose**: Convert OrchestrationRun state changes into human-readable log entries.

**Steps**:
1. In `App::update_state()`, after diffing for notifications, also generate log entries:
   ```rust
   fn generate_log_entries(&mut self, new_run: &OrchestrationRun) {
       for new_wp in &new_run.work_packages {
           let old_wp = self.run.work_packages.iter().find(|w| w.id == new_wp.id);
           let old_state = old_wp.map(|w| w.state);

           if Some(new_wp.state) != old_state {
               let level = match new_wp.state {
                   WPState::Failed => LogLevel::Error,
                   WPState::ForReview => LogLevel::Warn,
                   _ => LogLevel::Info,
               };
               self.logs.entries.push(LogEntry {
                   timestamp: SystemTime::now(),
                   level,
                   wp_id: Some(new_wp.id.clone()),
                   message: format!("{} → {:?}", old_state.map(|s| format!("{:?}", s)).unwrap_or("(new)".into()), new_wp.state),
               });
           }
       }

       // Run state changes
       if new_run.state != self.run.state {
           self.logs.entries.push(LogEntry {
               timestamp: SystemTime::now(),
               level: LogLevel::Info,
               wp_id: None,
               message: format!("Run state: {:?} → {:?}", self.run.state, new_run.state),
           });
       }

       // Cap at 10,000 entries
       if self.logs.entries.len() > 10_000 {
           self.logs.entries.drain(..self.logs.entries.len() - 10_000);
       }
   }
   ```

2. Call `generate_log_entries()` from `update_state()` before updating `self.run`

**Files**: `crates/kasmos/src/tui/app.rs` (~30 lines)

### Subtask T036 – Implement text filter

**Purpose**: `/` activates a filter input bar at the bottom. Typing filters visible entries in real-time.

**Steps**:
1. In `handle_logs_key()`:
   ```rust
   KeyCode::Char('/') => {
       app.logs.filter_active = true;
       app.logs.filter.clear();
   }
   ```

2. When `filter_active`, all keys go to filter input:
   ```rust
   if app.logs.filter_active {
       match key.code {
           KeyCode::Esc => {
               app.logs.filter_active = false;
               app.logs.filter.clear();
           }
           KeyCode::Enter => {
               app.logs.filter_active = false;
               // Keep filter applied
           }
           KeyCode::Backspace => { app.logs.filter.pop(); }
           KeyCode::Char(c) => { app.logs.filter.push(c); }
           _ => {}
       }
       return;
   }
   ```

3. Render filter bar at bottom of logs area:
   ```
   Filter: wp03█
   ```

4. In render, filter entries by checking if `entry.message` or `entry.wp_id` contains `filter` (case-insensitive)

**Files**: `crates/kasmos/src/tui/keybindings.rs` (~20 lines), `crates/kasmos/src/tui/tabs/logs.rs` (~15 lines)

### Subtask T037 – Implement auto-scroll

**Purpose**: New log entries automatically scroll into view unless the operator has manually scrolled up.

**Steps**:
1. In `App::update_state()`, after adding log entries:
   ```rust
   if self.logs.auto_scroll {
       self.logs.scroll_offset = self.logs.entries.len().saturating_sub(visible_height);
   }
   ```

2. Manual scroll up disables auto-scroll:
   ```rust
   KeyCode::Char('k') | KeyCode::Up => {
       app.logs.scroll_offset = app.logs.scroll_offset.saturating_sub(1);
       app.logs.auto_scroll = false;
   }
   ```

3. `G` (shift+g) resumes auto-scroll:
   ```rust
   KeyCode::Char('G') => {
       app.logs.auto_scroll = true;
       app.logs.scroll_offset = app.logs.entries.len().saturating_sub(visible_height);
   }
   ```

4. Show indicator when auto-scroll is paused: `[PAUSED — press G to resume]`

**Files**: `crates/kasmos/src/tui/keybindings.rs` (~10 lines), `crates/kasmos/src/tui/tabs/logs.rs` (~10 lines)

### Subtask T038 – Apply log level styling

**Purpose**: Visual distinction by log level makes it easy to spot errors and warnings.

**Steps**:
1. Color scheme (already in T034 render code):
   - Info: `Color::DarkGray` — routine state transitions
   - Warn: `Color::Yellow` — WP entering review, wave pauses
   - Error: `Color::Red` + `Modifier::BOLD` — WP failures, engine errors

2. Optionally add a level filter (toggle showing only errors/warnings) — low priority, can be deferred

**Files**: `crates/kasmos/src/tui/tabs/logs.rs` (~5 lines)

## Risks & Mitigations

- **Unbounded log growth**: Cap at 10,000 entries with FIFO eviction (drain oldest)
- **Filter mode captures all keys**: Must return early from handle_logs_key when filter_active to prevent vim nav from inserting characters
- **visible_height unknown at update_state time**: Store terminal height or compute from last render. Alternative: auto-scroll just sets scroll_offset to max on next render.

## Review Guidance

- Verify state transitions produce log entries with correct levels
- Test filter: type a WP id, verify only matching entries shown
- Test auto-scroll: add entries, verify list scrolls down. Scroll up, verify it pauses. Press G, verify it resumes.
- Check log entries display correctly with long messages (truncation at line width)

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T09:06:07Z – claude-sonnet-4-5 – lane=doing – Moved to doing
- 2026-02-11T10:05:19Z – reviewer – lane=done – Review passed via swarm
