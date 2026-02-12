# Data Model: TUI Widget Crate Adoption

**Feature**: 005-tui-widget-crate-adoption
**Date**: 2026-02-12

This document describes the entity and state changes to the kasmos TUI data model resulting from each crate adoption. No changes to the core domain model (`types.rs`) are needed — all changes are in TUI-layer state (`tui/app.rs`).

## Entities Removed

### LogEntry (removed by WP02 — tui-logger)

```rust
// REMOVED — replaced by tui-logger's internal buffer
pub struct LogEntry {
    pub timestamp: SystemTime,
    pub level: LogLevel,
    pub wp_id: Option<String>,
    pub message: String,
}
```

### LogLevel (removed by WP02 — tui-logger)

```rust
// REMOVED — tui-logger uses log::LevelFilter / tracing levels natively
pub enum LogLevel {
    Info,
    Warn,
    Error,
}
```

### LogsState (removed by WP02 — tui-logger)

```rust
// REMOVED — replaced by TuiWidgetState from tui-logger
pub struct LogsState {
    pub entries: Vec<LogEntry>,
    pub filter: String,
    pub filter_active: bool,
    pub scroll_offset: usize,
    pub auto_scroll: bool,
}
```

## Entities Added

### DashboardViewMode (added by WP05 — tui-nodes)

```rust
/// Which view is active in the Dashboard tab.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum DashboardViewMode {
    /// Standard kanban lane view (Planned / Doing / ForReview / Done).
    #[default]
    Kanban,
    /// Directed graph showing WP dependency relationships.
    DependencyGraph,
}
```

**Lifecycle**: Toggled by `v` key in Dashboard tab. Defaults to `Kanban`. No persistence across sessions.

### ConfirmAction (added by WP03 — tui-popup)

```rust
/// A pending confirmation dialog action.
#[derive(Debug, Clone)]
pub enum ConfirmAction {
    /// Force-advance a work package past its current state.
    ForceAdvance { wp_id: String },
    /// Abort the entire orchestration run.
    AbortRun,
}
```

**Lifecycle**: Created when operator triggers a destructive action. Consumed on `y` (execute action) or dismissed on `n`/`Esc`. Stored as `Option<ConfirmAction>` on `App`.

**Methods**:
- `title(&self) -> &str` — Dialog title (e.g., "Confirm Force Advance")
- `description(&self) -> String` — Dialog body text describing the action

## Entities Modified

### App (modified across WP01–WP05)

```rust
pub struct App {
    // Unchanged fields:
    pub run: OrchestrationRun,
    pub active_tab: Tab,
    pub notifications: Vec<Notification>,
    pub dashboard: DashboardState,
    pub review: ReviewState,
    pub action_tx: mpsc::Sender<EngineAction>,
    pub should_quit: bool,
    notification_counter: u64,
    review_policy_executor: ReviewPolicyExecutor,

    // REMOVED (WP02):
    // pub logs: LogsState,

    // ADDED (WP02 — tui-logger):
    /// tui-logger widget state (target selection, scroll, page mode).
    pub logger_state: tui_logger::TuiWidgetState,

    // ADDED (WP03 — tui-popup):
    /// Currently pending confirmation dialog, if any.
    pub pending_confirm: Option<ConfirmAction>,

    // ADDED (WP04 — throbber-widgets-tui):
    // (ThrobberState lives in DashboardState, not App directly)
}
```

### DashboardState (modified by WP04 and WP05)

```rust
pub struct DashboardState {
    // Unchanged:
    pub focused_lane: usize,
    pub selected_index: usize,
    pub scroll_offsets: [usize; 4],

    // ADDED (WP04 — throbber-widgets-tui):
    /// Shared spinner state for Active WP indicators.
    /// Ticked on App::on_tick() every 250ms.
    pub throbber_state: throbber_widgets_tui::ThrobberState,

    // ADDED (WP05 — tui-nodes):
    /// Current Dashboard sub-view mode (Kanban vs DependencyGraph).
    pub view_mode: DashboardViewMode,
}
```

## State Transitions

### ConfirmAction lifecycle

```
None ──(destructive action triggered)──→ Some(ConfirmAction)
Some(ConfirmAction) ──(user presses 'y')──→ execute action → None
Some(ConfirmAction) ──(user presses 'n' or Esc)──→ None
```

### DashboardViewMode transitions

```
Kanban ──(press 'v')──→ DependencyGraph
DependencyGraph ──(press 'v')──→ Kanban
```

### ThrobberState (internal)

```
frame_index: 0 ──(on_tick / calc_next)──→ 1 ──→ 2 ──→ ... ──→ N ──→ 0 (wraps)
```

Frame count depends on the selected throbber set (e.g., `BRAILLE_SIX` has 6 frames → 1.5s full rotation at 250ms tick).

## Validation Rules

- `pending_confirm` must be `None` when the popup is not visible (no stale confirmation state)
- `ThrobberState` is ticked unconditionally on every 250ms tick regardless of active tab (keeps animation smooth when switching back to Dashboard)
- `DashboardViewMode` does not affect any non-Dashboard state
- `logger_state` (TuiWidgetState) is initialized once at App creation and persists for the session lifetime

## Dependency Graph Data Flow

The dependency graph is derived (not stored separately) from `OrchestrationRun`:

```
OrchestrationRun.work_packages
    ├── wp.id → NodeLayout (label, position, state-based color)
    ├── wp.dependencies → Connection (directed edge from dep → wp)
    └── wp.state → Style (color-coded node border/fill)
```

The graph is rebuilt on each render frame from the current `run` snapshot. No separate graph state is cached (WP count is small enough that rebuilding is negligible).
