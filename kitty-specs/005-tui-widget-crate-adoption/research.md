# Research: TUI Widget Crate Adoption

**Feature**: 005-tui-widget-crate-adoption
**Date**: 2026-02-12

## R-1: ratatui Version Compatibility

**Decision**: All 5 crates use their latest ratatui 0.30 versions.

**Rationale**: Research revealed that ALL latest versions of the adopted crates require ratatui 0.30, not just tui-nodes as originally assumed:
- `tui-logger 0.18.1` → `ratatui ^0.30`
- `tui-popup 0.7.2` → `ratatui-core ^0.1` + `ratatui-widgets ^0.3` (ratatui 0.30 modularized crates)
- `throbber-widgets-tui 0.10.0` → `ratatui ^0.30.0`
- `ratatui-macros 0.7.0` → `ratatui-core ^0.1.0` + `ratatui-widgets ^0.3.0`
- `tui-nodes 0.10.0` → `ratatui ^0.30.0`

Since feature 006 (dependency upgrade to ratatui 0.30) is confirmed to merge before feature 005 starts, no fallback to older crate versions is needed.

**Alternatives considered**: Using older crate versions compatible with ratatui 0.29 (e.g., throbber-widgets-tui 0.8.0, ratatui-macros 0.6.0). Rejected because feature 006 lands first, making this unnecessary complexity.

## R-2: tui-logger Tracing Integration Pattern

**Decision**: Use `TuiTracingSubscriberLayer` with a conditional `Registry`-based subscriber.

**Rationale**: The current `logging.rs` uses `tracing_subscriber::fmt().init()` which sets a global default subscriber and cannot be composed. tui-logger provides `TuiTracingSubscriberLayer` (via the `tracing-support` feature) which implements `tracing_subscriber::Layer`.

**Integration pattern**:
```rust
// In logging.rs (TUI mode):
use tracing_subscriber::{Registry, layer::SubscriberExt, util::SubscriberInitExt};
use tui_logger::TuiTracingSubscriberLayer;

tui_logger::init_logger(log::LevelFilter::Trace)?;
tui_logger::set_default_level(log::LevelFilter::Trace);

Registry::default()
    .with(TuiTracingSubscriberLayer)
    .init();

// In logging.rs (headless mode):
use tracing_subscriber::{Registry, layer::SubscriberExt, util::SubscriberInitExt, fmt};

Registry::default()
    .with(fmt::layer().with_target(true).with_file(true).with_line_number(true))
    .with(EnvFilter::try_from_default_env().unwrap_or_else(|_| EnvFilter::new("kasmos=info")))
    .init();
```

**Key requirements**:
- `tui_logger::init_logger()` MUST be called before the tracing subscriber is initialized
- `tui_logger::move_events()` should be called periodically (on each render tick) to move events from the hot buffer to the display buffer — this is already aligned with the 250ms tick
- The `tracing-support` feature of `tui-logger` must be enabled in `Cargo.toml`

**Alternatives considered**: (A) Running fmt + tui-logger simultaneously — rejected because stderr output is invisible during TUI's alternate screen. (C) fmt to log file + tui-logger — rejected as unnecessary scope for a developer tool.

## R-3: tui-logger Widget API

**Decision**: Use `TuiLoggerSmartWidget` with `TuiWidgetState` for the Logs tab.

**API surface**:
- `TuiWidgetState::new()` — creates shared widget state
- `TuiLoggerSmartWidget::default().style(...).state(&widget_state)` — builds the widget
- `widget_state.transition(TuiWidgetEvent::...)` — forwards key events

**Built-in key commands** (delegated in Logs tab):
| Key | Action |
|-----|--------|
| h | Toggle target selector visible/hidden |
| f | Toggle focus on selected target |
| UP/DOWN | Select target in target selector |
| LEFT/RIGHT | Adjust displayed log level |
| -/+ | Adjust captured log level |
| PageUp | Enter page mode, scroll up |
| PageDown | Scroll down (page mode only) |
| Escape | Exit page mode |
| Space | Toggle hiding of off-level targets |

**Impact on existing code**: The entire `LogsState` struct, `LogEntry`, `LogLevel`, `filtered_log_entries()`, `format_timestamp()`, `render_logs()`, and the 10k-entry cap logic in `update_state()` and `record_review_failure()` are removed. The `record_review_failure()` method switches to using `tracing::error!()` which tui-logger captures automatically.

## R-4: tui-popup Widget API

**Decision**: Use `Popup::new(content).title(title).style(style)` for confirmation dialogs.

**API surface**:
- `Popup::new("content text").title("Confirm Action").style(Style::new().white().on_blue())`
- Render: `frame.render_widget(popup, frame.area())` — auto-centers in the provided area
- For multi-line content: Pass `Text` or `Paragraph` wrapped in `KnownSizeWrapper`
- `PopupState` is available for movable popups but not needed for confirmation dialogs

**Integration pattern**:
```rust
// In app.rs:
pub struct App {
    // ...
    pub pending_confirm: Option<ConfirmAction>,
}

// In render():
if let Some(action) = &self.pending_confirm {
    let popup = Popup::new(action.description())
        .title(action.title())
        .style(Style::new().white().on_red());
    frame.render_widget(popup, frame.area());
}
```

**Key handling**: When `pending_confirm.is_some()`, intercept `y`/`n`/`Esc` before tab-specific handlers.

## R-5: throbber-widgets-tui Widget API

**Decision**: Use single shared `ThrobberState` ticked on `App::on_tick()`.

**API surface**:
- `ThrobberState::default()` — creates animation state (tracks current frame index)
- `throbber_state.calc_next()` — advances to next frame (call per tick)
- `Throbber::default().label("label").throbber_style(style).throbber_set(SET)` — builds widget
- `frame.render_stateful_widget(throbber, area, &mut throbber_state)` — renders

**Available throbber sets** (constants): `BRAILLE_DOUBLE`, `BRAILLE_SIX`, `BRAILLE_SIX_DOUBLE`, `CLOCK`, `DOT`, `DOUBLE_DOT`, `HORIZONTAL_BLOCK`, `VERTICAL_BLOCK`, `ASCII`, `BOX_DRAWING`, `OGHAM`, and more. Recommend `BRAILLE_SIX` or `DOT` for a clean look.

**Integration**: One `ThrobberState` in `DashboardState`. In `on_tick()`, call `self.dashboard.throbber_state.calc_next()`. During dashboard rendering, for each Active WP, render a `Throbber` widget in the WP row using the shared state. Non-active WPs render a static `Span` badge instead.

## R-6: ratatui-macros Migration Patterns

**Decision**: Adopt `ratatui-macros 0.7.0` and migrate all TUI source files.

**Migration patterns** (current → new):

Layout construction:
```rust
// Before:
let chunks = Layout::default()
    .direction(Direction::Vertical)
    .constraints([Constraint::Length(3), Constraint::Min(0)])
    .split(area);

// After:
let [tab_bar, body] = vertical![==3, *=0].areas(area);
```

Text construction:
```rust
// Before:
Line::from(vec![
    Span::styled("INFO", Style::default().fg(Color::DarkGray)),
    Span::raw(" "),
    Span::raw(message),
])

// After:
line![span!(Color::DarkGray; "INFO"), " ", message]
```

Tab titles:
```rust
// Before:
let titles: Vec<Line> = Tab::titles().iter().map(|t| Line::from(Span::raw(*t))).collect();

// After:
let titles: Vec<Line> = Tab::titles().iter().map(|t| line![*t]).collect();
```

**Scope**: Files to migrate: `app.rs`, `keybindings.rs`, `mod.rs`, and any new widget files added by subsequent WPs.

## R-7: tui-nodes Graph Widget API

**Decision**: Use `NodeGraph` with `NodeLayout` and `Connection` to render WP dependencies.

**API surface**:
- `NodeGraph::new()` — creates empty graph
- `NodeLayout` — per-node configuration (position, label, styling)
- `Connection` — directed edge between two nodes
- `LineType` — edge rendering style (straight, curved)

**Integration pattern**:
```rust
// Build graph from OrchestrationRun:
fn build_dependency_graph(run: &OrchestrationRun) -> NodeGraph {
    let mut graph = NodeGraph::new();
    for wp in &run.work_packages {
        let style = state_to_style(wp.state);
        graph.add_node(NodeLayout::new(&wp.id, &wp.title, style));
        for dep in &wp.dependencies {
            graph.add_connection(Connection::new(dep, &wp.id));
        }
    }
    graph
}

fn state_to_style(state: WPState) -> Style {
    match state {
        WPState::Pending => Style::default().fg(Color::DarkGray),
        WPState::Active => Style::default().fg(Color::Yellow),
        WPState::Completed => Style::default().fg(Color::Green),
        WPState::Failed => Style::default().fg(Color::Red),
        WPState::ForReview => Style::default().fg(Color::Magenta),
        WPState::Paused => Style::default().fg(Color::Blue),
    }
}
```

**Edge cases**: Cycle detection must be handled before passing to tui-nodes (spec edge case). Render a warning banner if cycles are detected. For 20+ WPs, rely on tui-nodes' built-in layout algorithm and add scrolling if the graph exceeds the viewport.

**Note**: tui-nodes has very low documentation coverage (6.67%). Implementers should reference the crate's examples and source code directly. The API is small (4 types) so this is manageable.
