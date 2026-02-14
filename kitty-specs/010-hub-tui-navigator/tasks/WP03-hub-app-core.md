---
work_package_id: WP03
title: Hub App Core - Event Loop, List View, Navigation
lane: done
dependencies:
- WP01
subtasks:
- T012
- T013
- T014
- T015
- T016
- T017
phase: Phase 2 - Core Hub
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-13T03:53:23Z'
---

# Work Package Prompt: WP03 - Hub App Core - Event Loop, List View, Navigation

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

- Implement the hub's main async event loop with terminal setup/teardown
- Render a feature list with status indicators using ratatui
- Support keyboard navigation: j/k up/down, Enter select, Esc back, Alt+q quit
- Implement periodic refresh (5s interval) using `tokio::task::spawn_blocking` for scanner
- Implement manual refresh with `r` keybinding
- Detect Zellij-absent mode and display read-only warning (FR-017)
- Hub launches with `cargo run -p kasmos` (no args) and is fully interactive
- Keyboard input latency under 50ms (NFR-002)

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-002: Hub Module Structure, AD-007: Feature Status Refresh)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-001, FR-002, FR-009, FR-010, FR-016, FR-017)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md` (HubView, InputMode, FeatureEntry)
- **Research**: `kitty-specs/010-hub-tui-navigator/research.md` (R-004: TUI Event Loop Inside Zellij, R-006: Hub Logging)
- **Quickstart**: `kitty-specs/010-hub-tui-navigator/quickstart.md` (keybinding table)
- **Dependencies**: WP01 (TUI plumbing as `pub`), WP02 (FeatureScanner)

### Key Architectural Decisions

- Hub event loop reuses `tui::setup_terminal()`, `tui::restore_terminal()`, `tui::install_panic_hook()` from WP01
- Hub reuses `tui::event::EventHandler` for crossterm event polling
- `ZELLIJ_SESSION_NAME` env var determines full vs read-only mode (AD-002)
- Periodic refresh uses `tokio::time::interval(Duration::from_secs(5))` with `spawn_blocking` (AD-007)
- Hub does NOT initialize tracing logging (R-006) -- errors shown inline in TUI

## Subtasks & Detailed Guidance

### Subtask T012 - Define HubView, InputMode, App struct

- **Purpose**: Create the hub application state that drives rendering and event handling.
- **Steps**:
  1. Create `crates/kasmos/src/hub/app.rs`
  2. Add `pub mod app;` to `crates/kasmos/src/hub/mod.rs`
  3. Define the following types:

```rust
use super::scanner::FeatureEntry;

/// Which view the hub is currently displaying.
#[derive(Debug, Clone, PartialEq)]
pub enum HubView {
    /// Feature list (main view)
    List,
    /// Expanded feature detail
    Detail { index: usize },
}

/// Input mode for the hub.
#[derive(Debug, Clone)]
pub enum InputMode {
    /// Standard navigation
    Normal,
    /// Typing a new feature name
    NewFeaturePrompt { input: String },
    /// Confirmation modal (e.g., >6 WP warning)
    ConfirmDialog {
        message: String,
        // on_confirm action stored as enum variant, not Box<dyn>
    },
}

/// Hub application state.
pub struct App {
    /// Current feature list from scanner
    pub features: Vec<FeatureEntry>,
    /// Currently highlighted feature index
    pub selected: usize,
    /// Current view
    pub view: HubView,
    /// Current input mode
    pub input_mode: InputMode,
    /// Zellij session name (None = read-only mode)
    pub zellij_session: Option<String>,
    /// Whether the hub should quit
    pub should_quit: bool,
    /// Status message displayed in the status bar
    pub status_message: Option<String>,
}

impl App {
    pub fn new(features: Vec<FeatureEntry>, zellij_session: Option<String>) -> Self {
        Self {
            features,
            selected: 0,
            view: HubView::List,
            input_mode: InputMode::Normal,
            zellij_session,
            should_quit: false,
            status_message: None,
        }
    }

    /// Update the feature list from a fresh scan, preserving selection.
    pub fn update_features(&mut self, features: Vec<FeatureEntry>) {
        // Preserve selection by clamping to new list length
        self.features = features;
        if self.selected >= self.features.len() && !self.features.is_empty() {
            self.selected = self.features.len() - 1;
        }
    }

    pub fn select_next(&mut self) {
        if !self.features.is_empty() {
            self.selected = (self.selected + 1).min(self.features.len() - 1);
        }
    }

    pub fn select_previous(&mut self) {
        self.selected = self.selected.saturating_sub(1);
    }

    pub fn is_read_only(&self) -> bool {
        self.zellij_session.is_none()
    }
}
```

- **Files**: `crates/kasmos/src/hub/app.rs` (new), `crates/kasmos/src/hub/mod.rs` (add module)
- **Parallel?**: No (T013-T017 depend on this)

### Subtask T013 - Implement hub::run() event loop

- **Purpose**: Replace the WP01 placeholder with the real hub event loop.
- **Steps**:
  1. Replace the placeholder in `crates/kasmos/src/hub/mod.rs` with the full event loop
  2. The event loop should:
     a. Read `ZELLIJ_SESSION_NAME` env var
     b. Call `kasmos::tui::install_panic_hook()`
     c. Perform initial scan via `FeatureScanner::scan()`
     d. Call `kasmos::tui::setup_terminal()`
     e. Create `App` with scan results and session name
     f. Create `EventHandler` from `kasmos::tui::event::EventHandler`
     g. Create `tokio::time::interval(Duration::from_secs(5))` for periodic refresh
     h. Loop: draw frame, select on (event, interval tick), handle quit
     i. Call `kasmos::tui::restore_terminal()` on exit

```rust
pub async fn run() -> anyhow::Result<()> {
    // Detect Zellij session
    let zellij_session = std::env::var("ZELLIJ_SESSION_NAME").ok();

    // Install panic hook before entering raw mode
    kasmos::tui::install_panic_hook();

    // Initial scan
    let specs_root = std::path::PathBuf::from("kitty-specs");
    let scanner = scanner::FeatureScanner::new(specs_root.clone());
    let features = scanner.scan();

    // Setup terminal
    let mut terminal = kasmos::tui::setup_terminal()?;

    // Create app state
    let mut app = app::App::new(features, zellij_session);
    let mut event_handler = kasmos::tui::event::EventHandler::new();
    let mut refresh_interval = tokio::time::interval(std::time::Duration::from_secs(5));
    refresh_interval.tick().await; // consume initial tick

    loop {
        terminal.draw(|frame| app.render(frame))?;

        tokio::select! {
            Some(event) = event_handler.next() => {
                keybindings::handle_event(&mut app, event);
            }
            _ = refresh_interval.tick() => {
                let scanner_clone = scanner::FeatureScanner::new(specs_root.clone());
                let features = tokio::task::spawn_blocking(move || {
                    scanner_clone.scan()
                }).await?;
                app.update_features(features);
            }
        }

        if app.should_quit {
            break;
        }
    }

    kasmos::tui::restore_terminal(&mut terminal)?;
    Ok(())
}
```

- **Files**: `crates/kasmos/src/hub/mod.rs`
- **Parallel?**: No (core event loop)
- **Notes**: The event loop structure mirrors `crates/kasmos/src/tui/mod.rs` lines 84-115 but without the watch channel (hub has no engine). The `spawn_blocking` call ensures filesystem I/O doesn't block the event loop (NFR-003).
- **Important (R-006)**: `main.rs` calls `init_logging()` before the match dispatch, which initializes tracing to stderr. This will corrupt the hub TUI's alternate screen. The hub's `run()` must be called BEFORE `init_logging()`, or `init_logging()` must be moved inside the `Some(...)` match arms so it only runs for subcommands. Recommended approach: move `let _ = kasmos::init_logging();` from before the match into each `Some(Commands::...)` arm, and skip it for the `None` (hub) arm.

### Subtask T014 - Implement feature list rendering

- **Purpose**: Render the feature list with status indicators using ratatui widgets.
- **Steps**:
  1. Implement `pub fn render(&self, frame: &mut ratatui::Frame)` on `App`
  2. Layout: header (title), main area (feature list), footer (keybinding hints)
  3. Use `ratatui::widgets::List` with `ListItem` for each feature
  4. Each list item shows: `[NNN] slug-name  [status]`
  5. Status indicators (displayed left-to-right, show the most relevant one per FR-002):
     - `SpecStatus::Empty` -> `[empty spec]` in dim/gray
     - `SpecStatus::Present` + `PlanStatus::Absent` -> `[no plan]` in yellow
     - `SpecStatus::Present` + `PlanStatus::Present` + `TaskProgress::NoTasks` -> `[no tasks]` in yellow
     - `TaskProgress::InProgress { done, total }` -> `[done/total done]` in cyan
     - `TaskProgress::Complete` -> `[complete]` in green with checkmark
     - `OrchestrationStatus::Running` -> `[running]` in bright green (takes precedence over task progress)
  6. Highlight selected item with a different background color
  7. If `app.is_read_only()`, show a warning banner at the top: "Read-only mode -- Zellij not detected"

- **Files**: `crates/kasmos/src/hub/app.rs`
- **Parallel?**: Yes (can proceed once T012 App struct exists)
- **Notes**: Use `ratatui::layout::Layout` with `Constraint::Length` for header/footer and `Constraint::Min` for the list area. Use `ratatui::style::Style` with `Color` for status indicators.

**Example rendering layout**:
```
+------------------------------------------+
| kasmos Hub TUI                    [r]efresh |
|------------------------------------------|
| > [001] my-feature        [2/5 done]    |
|   [002] another-feature   [empty spec]  |
|   [003] third-feature     [running]     |
|   [010] hub-tui-navigator [no tasks]    |
|                                          |
|------------------------------------------|
| j/k:nav  Enter:select  n:new  Alt+q:quit |
+------------------------------------------+
```

### Subtask T015 - Implement keyboard navigation

- **Purpose**: Handle keyboard events for hub navigation.
- **Steps**:
  1. Create `crates/kasmos/src/hub/keybindings.rs`
  2. Add `pub mod keybindings;` to `crates/kasmos/src/hub/mod.rs`
  3. Implement `pub fn handle_event(app: &mut App, event: crossterm::event::Event)` that handles:
     - `KeyCode::Char('j')` or `KeyCode::Down` -> `app.select_next()`
     - `KeyCode::Char('k')` or `KeyCode::Up` -> `app.select_previous()`
     - `KeyCode::Enter` -> enter detail view for selected feature (set `app.view = HubView::Detail { index: app.selected }`)
     - `KeyCode::Esc` -> return to list view (set `app.view = HubView::List`)
     - `KeyCode::Char('r')` -> set a flag for manual refresh (handled in event loop)
     - `KeyCode::Char('n')` -> enter `InputMode::NewFeaturePrompt` (placeholder for WP05)
     - `Alt+q` (`KeyCode::Char('q')` with `KeyModifiers::ALT`) -> `app.should_quit = true`
  4. Only handle keys when `app.input_mode` is `InputMode::Normal`

- **Files**: `crates/kasmos/src/hub/keybindings.rs` (new), `crates/kasmos/src/hub/mod.rs` (add module)
- **Parallel?**: Yes (can proceed once T012 App struct exists)
- **Notes**: Follow the pattern from `crates/kasmos/src/tui/keybindings.rs`. Use `crossterm::event::{Event, KeyCode, KeyEvent, KeyModifiers}`.

### Subtask T016 - Implement periodic refresh timer

- **Purpose**: Keep the feature list up-to-date as agents create files in adjacent panes.
- **Steps**:
  1. In the event loop (T013), the `refresh_interval.tick()` branch already calls `spawn_blocking` with the scanner
  2. Add a `refresh_requested: bool` field to `App` for manual refresh support
  3. In the event loop, check `app.refresh_requested` after handling events -- if true, trigger an immediate scan and reset the flag
  4. After each refresh, call `app.update_features(features)` which preserves selection state
- **Files**: `crates/kasmos/src/hub/mod.rs`, `crates/kasmos/src/hub/app.rs`
- **Parallel?**: No (part of the event loop)
- **Notes**: The 5-second interval is per AD-007. `spawn_blocking` ensures the scan doesn't block the event loop (NFR-003). The manual refresh (`r` key) sets `refresh_requested = true` which is checked in the next loop iteration.

### Subtask T017 - Implement manual refresh and read-only mode detection

- **Purpose**: Allow manual refresh and gracefully handle running outside Zellij.
- **Steps**:
  1. In `keybindings.rs`, the `r` key handler sets `app.refresh_requested = true`
  2. In the event loop, after handling events, check `app.refresh_requested`:
     ```rust
     if app.refresh_requested {
         app.refresh_requested = false;
         let scanner_clone = scanner::FeatureScanner::new(specs_root.clone());
         let features = tokio::task::spawn_blocking(move || scanner_clone.scan()).await?;
         app.update_features(features);
         app.status_message = Some("Refreshed".to_string());
     }
     ```
  3. Read-only mode: `app.is_read_only()` returns `true` when `zellij_session` is `None`
  4. In rendering (T014), show a warning banner when read-only
  5. In keybindings, when read-only mode is active and an action key is pressed, show a status message: "Action unavailable -- not running inside Zellij"
- **Files**: `crates/kasmos/src/hub/mod.rs`, `crates/kasmos/src/hub/app.rs`, `crates/kasmos/src/hub/keybindings.rs`
- **Parallel?**: No (integrates with event loop and keybindings)
- **Notes**: FR-017 requires the hub to detect when it's running outside Zellij and operate in read-only mode. The `ZELLIJ_SESSION_NAME` env var is set by Zellij for all processes running inside a session.

## Test Strategy

- **Build verification**: `cargo build -p kasmos` succeeds
- **Manual testing**: Run `kasmos` (no args) -- hub should display feature list and respond to keyboard
- **Unit tests**: Test `App::update_features()` preserves selection, `select_next()`/`select_previous()` bounds checking
- **Integration test**: Verify hub launches and exits cleanly with Alt+q

## Risks & Mitigations

- **Event loop blocking**: Scanner runs on `spawn_blocking` -- verified by code structure
- **Terminal corruption**: Panic hook from WP01 restores terminal on panic
- **Zellij key conflicts**: Hub uses j/k/Enter/Esc/Alt+q which don't conflict with Zellij defaults (R-004)
- **Large feature lists**: ratatui `List` widget handles scrolling natively

## Review Guidance

- Verify event loop structure matches the pattern from `crates/kasmos/src/tui/mod.rs`
- Verify `spawn_blocking` is used for scanner calls (not blocking the async runtime)
- Verify read-only mode detection works correctly
- Verify selection preservation across refreshes
- Run `cargo build` and test manually with `cargo run -p kasmos`

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T12:00:00Z - release opencode agent - lane=done - Acceptance validation passed
