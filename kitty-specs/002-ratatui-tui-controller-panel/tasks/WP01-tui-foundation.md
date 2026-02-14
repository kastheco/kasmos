---
work_package_id: WP01
title: TUI Foundation & Dependencies
lane: done
dependencies: []
base_branch: 002-ratatui-tui-controller-panel
base_commit: 0562b7a1d497f31478d3be6c173e0fea36354825
created_at: '2026-02-11T05:17:37.091781+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
phase: Phase 1 - Foundation
assignee: 'unknown'
agent: 'reviewer'
shell_pid: 'unknown'
review_status: approved
reviewed_by: kas
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP01 – TUI Foundation & Dependencies

## Objectives & Success Criteria

- Add ratatui, crossterm, and futures-util dependencies to the kasmos crate
- Create the `tui/` module skeleton with terminal lifecycle management, event handling, App state, and keybinding framework
- The TUI event loop compiles and runs (renders placeholder content, exits on `q`)
- All existing 198 tests continue to pass
- `cargo build` succeeds with no warnings in new code

**Implementation command**: `spec-kitty implement WP01`

## Context & Constraints

- **Plan**: `kitty-specs/002-ratatui-tui-controller-panel/plan.md` — architecture, channel topology, keybindings
- **Data model**: `kitty-specs/002-ratatui-tui-controller-panel/data-model.md` — App/Tab/Notification/DashboardState/ReviewState/LogsState structs
- **Research**: `kitty-specs/002-ratatui-tui-controller-panel/research.md` — R1 (async integration), R3 (direct state), R4 (terminal lifecycle)
- **Existing crate**: `crates/kasmos/` — Cargo.toml at `crates/kasmos/Cargo.toml`, lib.rs at `crates/kasmos/src/lib.rs`
- **Key decision**: Direct state mutation on App struct (not Elm-style). Event loop uses `tokio::select!` with crossterm EventStream.
- **Key decision**: ratatui + crossterm backend. TUI runs inside Zellij controller pane.

## Subtasks & Detailed Guidance

### Subtask T001 – Add ratatui, crossterm, and futures-util dependencies

**Purpose**: Enable ratatui TUI rendering with crossterm terminal backend and futures stream support for async event handling.

**Steps**:
1. Edit `crates/kasmos/Cargo.toml` to add under `[dependencies]`:
   ```toml
   ratatui = { version = "0.29", features = ["crossterm"] }
   crossterm = "0.28"
   futures-util = "0.3"
   ```
2. Run `cargo check -p kasmos` to verify dependency resolution

**Files**: `crates/kasmos/Cargo.toml`

**Notes**: ratatui re-exports crossterm types via `ratatui::crossterm`, but we also need crossterm directly for `EventStream` which requires the `event-stream` feature. Check if ratatui's crossterm re-export includes this, otherwise add `crossterm = { version = "0.28", features = ["event-stream"] }`.

### Subtask T002 – Create `tui/mod.rs` — terminal lifecycle and event loop

**Purpose**: Establish the TUI entry point with terminal setup/teardown, panic hook, and the async event loop skeleton that will be the core of the TUI.

**Steps**:
1. Create `crates/kasmos/src/tui/mod.rs` with:
   ```rust
   pub mod app;
   pub mod event;
   pub mod keybindings;
   // tabs/ and widgets/ will be added in later WPs
   ```

2. Implement terminal setup function:
   ```rust
   fn setup_terminal() -> Result<Terminal<CrosstermBackend<Stdout>>> {
       enable_raw_mode()?;
       let mut stdout = std::io::stdout();
       execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
       let backend = CrosstermBackend::new(stdout);
       Terminal::new(backend).context("Failed to create terminal")
   }
   ```

3. Implement terminal teardown:
   ```rust
   fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<Stdout>>) -> Result<()> {
       disable_raw_mode()?;
       execute!(terminal.backend_mut(), LeaveAlternateScreen, DisableMouseCapture)?;
       terminal.show_cursor()?;
       Ok(())
   }
   ```

4. Install panic hook that restores terminal:
   ```rust
   fn install_panic_hook() {
       let original_hook = std::panic::take_hook();
       std::panic::set_hook(Box::new(move |panic_info| {
           let _ = disable_raw_mode();
           let _ = execute!(std::io::stdout(), LeaveAlternateScreen, DisableMouseCapture);
           original_hook(panic_info);
       }));
   }
   ```

5. Implement the main TUI run function (async):
   ```rust
   pub async fn run(
       watch_rx: watch::Receiver<OrchestrationRun>,
       action_tx: mpsc::Sender<EngineAction>,
   ) -> Result<()> {
       install_panic_hook();
       let mut terminal = setup_terminal()?;
       let mut app = App::new(watch_rx.borrow().clone(), action_tx);
       let mut event_handler = EventHandler::new();

       loop {
           terminal.draw(|frame| app.render(frame))?;

           tokio::select! {
               Some(event) = event_handler.next() => {
                   app.handle_event(event);
               }
               Ok(()) = watch_rx.changed() => {
                   app.update_state(watch_rx.borrow().clone());
               }
               _ = tokio::time::sleep(Duration::from_millis(250)) => {
                   app.on_tick();
               }
           }

           if app.should_quit {
               break;
           }
       }

       restore_terminal(&mut terminal)?;
       Ok(())
   }
   ```

**Files**: `crates/kasmos/src/tui/mod.rs` (new, ~100 lines)

**Notes**:
- The `watch_rx` and `action_tx` parameters will be wired in WP02 (launch.rs). For now, this function exists but isn't called from main yet.
- Use `tokio::sync::watch` for watch_rx and `tokio::sync::mpsc` for action_tx — both already available via tokio "full" feature.
- The tick interval (250ms) is for elapsed time display updates — not for polling state.

### Subtask T003 – Create `tui/event.rs` — Event enum and EventStream wrapper

**Purpose**: Wrap crossterm's EventStream into a tokio-compatible async event source that produces typed events for the TUI.

**Steps**:
1. Create `crates/kasmos/src/tui/event.rs` with:
   ```rust
   use crossterm::event::{Event as CrosstermEvent, EventStream, KeyEvent, MouseEvent};
   use futures_util::StreamExt;

   pub enum TuiEvent {
       Key(KeyEvent),
       Mouse(MouseEvent),
       Resize(u16, u16),
   }

   pub struct EventHandler {
       stream: EventStream,
   }

   impl EventHandler {
       pub fn new() -> Self {
           Self { stream: EventStream::new() }
       }

       pub async fn next(&mut self) -> Option<TuiEvent> {
           loop {
               match self.stream.next().await? {
                   Ok(CrosstermEvent::Key(key)) => {
                       if key.kind == crossterm::event::KeyEventKind::Press {
                           return Some(TuiEvent::Key(key));
                       }
                   }
                   Ok(CrosstermEvent::Mouse(mouse)) => {
                       return Some(TuiEvent::Mouse(mouse));
                   }
                   Ok(CrosstermEvent::Resize(w, h)) => {
                       return Some(TuiEvent::Resize(w, h));
                   }
                   _ => {} // Ignore FocusGained, FocusLost, Paste
               }
           }
       }
   }
   ```

2. Only emit `KeyEventKind::Press` events (ignore Release/Repeat) to prevent double-firing

**Files**: `crates/kasmos/src/tui/event.rs` (new, ~40 lines)

**Notes**: `crossterm::event::EventStream` requires the `event-stream` feature on crossterm. Verify this is enabled.

### Subtask T004 – Create `tui/app.rs` — App struct and supporting types

**Purpose**: Define the root application state that holds the orchestration run snapshot, per-tab UI state, and notification list.

**Steps**:
1. Create `crates/kasmos/src/tui/app.rs` with all types from data-model.md:

   ```rust
   use crate::{EngineAction, OrchestrationRun};
   use std::time::Instant;
   use tokio::sync::mpsc;

   pub enum Tab {
       Dashboard,
       Review,
       Logs,
   }

   pub struct Notification {
       pub id: u64,
       pub kind: NotificationKind,
       pub wp_id: String,
       pub message: Option<String>,
       pub created_at: Instant,
   }

   pub enum NotificationKind {
       ReviewPending,
       Failure,
       InputNeeded,
   }

   pub struct DashboardState {
       pub focused_lane: usize,
       pub selected_index: usize,
       pub scroll_offsets: [usize; 4],
   }

   pub struct ReviewState {
       pub selected_index: usize,
       pub detail_scroll: usize,
   }

   pub struct LogsState {
       pub entries: Vec<LogEntry>,
       pub filter: String,
       pub filter_active: bool,
       pub scroll_offset: usize,
       pub auto_scroll: bool,
   }

   pub struct LogEntry {
       pub timestamp: std::time::SystemTime,
       pub level: LogLevel,
       pub wp_id: Option<String>,
       pub message: String,
   }

   pub enum LogLevel { Info, Warn, Error }

   pub struct App {
       pub run: OrchestrationRun,
       pub active_tab: Tab,
       pub notifications: Vec<Notification>,
       pub dashboard: DashboardState,
       pub review: ReviewState,
       pub logs: LogsState,
       pub action_tx: mpsc::Sender<EngineAction>,
       pub should_quit: bool,
       notification_counter: u64,
   }
   ```

2. Implement `App::new(run, action_tx)` with default state for each tab

3. Implement `App::handle_event(event: TuiEvent)` — delegate to keybindings for key events, handle resize/mouse

4. Implement `App::update_state(new_run: OrchestrationRun)` — store new run snapshot, will be extended in WP05 for notification diffing

5. Implement `App::on_tick()` — placeholder for elapsed time updates

6. Implement `App::render(frame: &mut Frame)` — for now, render a simple placeholder:
   - Tab header bar at top (highlight active tab)
   - Body: "Dashboard view coming soon" / etc.
   - This will be replaced by real tab rendering in WP03+

**Files**: `crates/kasmos/src/tui/app.rs` (new, ~150 lines)

**Notes**: `OrchestrationRun` must derive `Clone` for watch channel compatibility. Check if it already does (it should via serde). If not, add `#[derive(Clone)]`.

### Subtask T005 – Create `tui/keybindings.rs` — keymap definitions

**Purpose**: Centralized keybinding definitions for tab switching, vim navigation, quit, and action keys.

**Steps**:
1. Create `crates/kasmos/src/tui/keybindings.rs`:
   ```rust
   use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
   use crate::tui::app::{App, Tab};

   pub fn handle_key(app: &mut App, key: KeyEvent) {
       // Global keys (work in all tabs)
       match key.code {
           KeyCode::Char('q') => app.should_quit = true,
           KeyCode::Char('1') => app.active_tab = Tab::Dashboard,
           KeyCode::Char('2') => app.active_tab = Tab::Review,
           KeyCode::Char('3') => app.active_tab = Tab::Logs,
           KeyCode::Char('n') => { /* notification jump — WP05 */ }
           _ => {
               // Tab-specific keys — delegated per active tab
               match app.active_tab {
                   Tab::Dashboard => handle_dashboard_key(app, key),
                   Tab::Review => handle_review_key(app, key),
                   Tab::Logs => handle_logs_key(app, key),
               }
           }
       }
   }

   fn handle_dashboard_key(app: &mut App, key: KeyEvent) {
       match key.code {
           KeyCode::Char('j') | KeyCode::Down => { /* move down in lane — WP03 */ }
           KeyCode::Char('k') | KeyCode::Up => { /* move up in lane — WP03 */ }
           KeyCode::Char('h') | KeyCode::Left => { /* move to left lane — WP03 */ }
           KeyCode::Char('l') | KeyCode::Right => { /* move to right lane — WP03 */ }
           // Action keys filled in WP04
           _ => {}
       }
   }

   fn handle_review_key(app: &mut App, key: KeyEvent) {
       // Review-specific keys filled in WP06
       match key.code {
           KeyCode::Char('j') | KeyCode::Down => { /* next review item */ }
           KeyCode::Char('k') | KeyCode::Up => { /* prev review item */ }
           _ => {}
       }
   }

   fn handle_logs_key(app: &mut App, key: KeyEvent) {
       // Logs-specific keys filled in WP07
       match key.code {
           KeyCode::Char('j') | KeyCode::Down => { /* scroll down */ }
           KeyCode::Char('k') | KeyCode::Up => { /* scroll up */ }
           KeyCode::Char('/') => { /* activate filter — WP07 */ }
           _ => {}
       }
   }
   ```

2. Use stub bodies (comments) for keys that depend on later WPs — they'll be filled in as those WPs are implemented

**Files**: `crates/kasmos/src/tui/keybindings.rs` (new, ~60 lines)

**Notes**: Keep keybinding logic thin — actual state mutations should call methods on App or its sub-state structs. The keybinding module just maps keys to method calls.

## Risks & Mitigations

- **crossterm feature flags**: Ensure `event-stream` feature is enabled for `EventStream`. If ratatui's re-export doesn't include it, add it explicitly on the crossterm dep.
- **Terminal corruption on panic**: The panic hook must run before any other panic handler. Call `std::panic::take_hook()` to chain with the original.
- **futures-util version compatibility**: Use `0.3` which is stable and compatible with tokio 1.x.

## Review Guidance

- Verify `cargo build -p kasmos` compiles with zero warnings in new tui/ code
- Verify `cargo test -p kasmos` — all 198 existing tests pass
- Verify terminal setup/teardown: run the TUI function (even if not wired to main yet), confirm alternate screen is entered and exited cleanly
- Check that the event loop doesn't busy-loop — the `tokio::select!` should block until an event arrives

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T01:31:59Z – unknown – lane=done – Review passed: all 5 subtasks complete, zero new warnings, 208 tests pass, types match data-model, integration seam clean
- 2026-02-11T02:16:20Z – unknown – lane=done – Review passed: good
- 2026-02-11T04:47:09Z – unknown – lane=done – Review passed: finished
