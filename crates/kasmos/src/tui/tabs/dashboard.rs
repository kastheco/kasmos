//! Dashboard tab rendering.
//!
//! Renders either a kanban-style board with four lanes (Planned, Doing,
//! For Review, Done) or a WP dependency graph visualization, toggled by `v`.
//! Both views include a progress summary bar, and the kanban view has
//! responsive column layout and failure badges.

use ratatui::Frame;
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Gauge, List, ListItem, Paragraph};

use crate::tui::widgets::dependency_graph::render_dependency_graph;
use crate::types::{WPState, WorkPackage};

use super::super::app::{App, DashboardViewMode, state_to_lane};

// ---------------------------------------------------------------------------
// Column mode (responsive layout)
// ---------------------------------------------------------------------------

/// Determines how many kanban columns to render based on terminal width.
#[derive(Debug, Clone, Copy)]
enum ColumnMode {
    /// >= 100 cols: all 4 lanes visible.
    Full,
    /// 60..100 cols: 2 lanes visible (focused pair).
    Compact,
    /// < 60 cols: 1 lane visible (focused lane only).
    Single,
}

impl ColumnMode {
    fn from_width(width: u16) -> Self {
        if width >= 100 {
            Self::Full
        } else if width >= 60 {
            Self::Compact
        } else {
            Self::Single
        }
    }
}

// ---------------------------------------------------------------------------
// Lane helpers
// ---------------------------------------------------------------------------

/// The four kanban lane definitions.
const LANE_TITLES: [&str; 4] = ["Planned", "Doing", "For Review", "Done"];

/// Partition work packages into 4 lanes by state.
fn partition_lanes(work_packages: &[WorkPackage]) -> [Vec<&WorkPackage>; 4] {
    let mut lanes: [Vec<&WorkPackage>; 4] = Default::default();
    for wp in work_packages {
        let lane = state_to_lane(wp.state);
        lanes[lane].push(wp);
    }
    lanes
}

/// Format a WP item with state badge and optional failure count.
fn format_wp_item(wp: &WorkPackage, is_selected: bool) -> ListItem<'static> {
    let state_badge = match wp.state {
        WPState::Pending => Span::styled("○", Style::default().fg(Color::DarkGray)),
        WPState::Active => Span::styled("●", Style::default().fg(Color::Yellow)),
        WPState::Completed => Span::styled("✓", Style::default().fg(Color::Green)),
        WPState::Failed => Span::styled(
            "✗",
            Style::default().fg(Color::Red).add_modifier(Modifier::BOLD),
        ),
        WPState::Paused => Span::styled("⏸", Style::default().fg(Color::Blue)),
        WPState::ForReview => Span::styled("◉", Style::default().fg(Color::Magenta)),
    };

    let mut spans = vec![
        state_badge,
        Span::raw(" "),
        Span::raw(format!("{}: {}", wp.id, wp.title)),
    ];

    // Failure badge (FR-022)
    if wp.failure_count > 0 {
        spans.push(Span::raw(" "));
        spans.push(Span::styled(
            format!("[x{}]", wp.failure_count),
            Style::default().fg(Color::Red).add_modifier(Modifier::BOLD),
        ));
    }

    let style = if is_selected {
        Style::default()
            .fg(Color::Yellow)
            .add_modifier(Modifier::BOLD)
    } else {
        Style::default()
    };

    ListItem::new(Line::from(spans)).style(style)
}

// ---------------------------------------------------------------------------
// Progress summary bar (FR-020)
// ---------------------------------------------------------------------------

/// Render the progress summary bar above the kanban lanes.
fn render_progress_bar(app: &App, frame: &mut Frame, area: Rect) {
    let total = app.run.work_packages.len();
    if total == 0 {
        return;
    }

    let completed = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::Completed)
        .count();
    let active = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::Active)
        .count();
    let failed = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::Failed)
        .count();
    let review = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::ForReview)
        .count();
    let pending = total - completed - active - failed - review;

    let pct = (completed as f64 / total as f64 * 100.0) as u16;

    // Split: gauge left, counts right
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(50), Constraint::Percentage(50)])
        .split(area);

    let gauge = Gauge::default()
        .block(Block::default())
        .gauge_style(
            Style::default()
                .fg(Color::Green)
                .add_modifier(Modifier::BOLD),
        )
        .percent(pct)
        .label(format!("{completed}/{total} complete ({pct}%)"));

    frame.render_widget(gauge, chunks[0]);

    // Wave progress
    let current_wave = app
        .run
        .waves
        .iter()
        .position(|w| {
            w.state == crate::types::WaveState::Active
                || w.state == crate::types::WaveState::Pending
        })
        .unwrap_or(app.run.waves.len());
    let total_waves = app.run.waves.len();

    let counts_line = Line::from(vec![
        Span::styled(
            format!("P:{pending} "),
            Style::default().fg(Color::DarkGray),
        ),
        Span::styled(format!("A:{active} "), Style::default().fg(Color::Yellow)),
        Span::styled(format!("R:{review} "), Style::default().fg(Color::Magenta)),
        Span::styled(format!("F:{failed} "), Style::default().fg(Color::Red)),
        Span::styled(format!("D:{completed} "), Style::default().fg(Color::Green)),
        Span::styled(
            format!("W:{current_wave}/{total_waves}"),
            Style::default().fg(Color::Cyan),
        ),
    ]);

    frame.render_widget(Paragraph::new(counts_line), chunks[1]);
}

// ---------------------------------------------------------------------------
// Kanban lane rendering with scrolling
// ---------------------------------------------------------------------------

/// Render a single kanban lane with scroll support.
fn render_lane(
    app: &App,
    frame: &mut Frame,
    area: Rect,
    lane_index: usize,
    wps: &[&WorkPackage],
    is_focused: bool,
) {
    let title = format!(" {} ({}) ", LANE_TITLES[lane_index], wps.len());

    let border_style = if is_focused {
        Style::default()
            .fg(Color::Yellow)
            .add_modifier(Modifier::BOLD)
    } else {
        Style::default().fg(Color::DarkGray)
    };

    let block = Block::default()
        .borders(Borders::ALL)
        .title(title)
        .border_style(border_style);

    let inner = block.inner(area);
    frame.render_widget(block, area);

    if wps.is_empty() {
        let empty = Paragraph::new(Span::styled(
            "(empty)",
            Style::default().fg(Color::DarkGray),
        ));
        frame.render_widget(empty, inner);
        return;
    }

    let visible_height = inner.height as usize;
    // Update last known lane height for scroll offset calculations in keybinding handlers
    if is_focused {
        app.dashboard.last_lane_height.set(visible_height);
    }
    let scroll_offset = app.dashboard.scroll_offsets[lane_index];
    let selected = if is_focused {
        app.dashboard.selected_index
    } else {
        usize::MAX // No selection highlight for unfocused lanes
    };

    // Build items with scroll offset applied
    let items: Vec<ListItem> = wps
        .iter()
        .enumerate()
        .skip(scroll_offset)
        .take(visible_height)
        .map(|(i, wp)| format_wp_item(wp, i == selected))
        .collect();

    let list = List::new(items);
    frame.render_widget(list, inner);
}

// ---------------------------------------------------------------------------
// Public render entry point
// ---------------------------------------------------------------------------

/// Render the entire Dashboard tab into the given area.
pub fn render_dashboard(app: &App, frame: &mut Frame, area: Rect) {
    // Split: progress bar (1 row) + view mode indicator (1 row) + body
    let vertical = Layout::default()
        .direction(Direction::Vertical)
        .constraints([
            Constraint::Length(1),
            Constraint::Length(1),
            Constraint::Min(0),
        ])
        .split(area);

    // Progress summary bar (FR-020)
    render_progress_bar(app, frame, vertical[0]);

    // View mode indicator
    let mode_label = match app.dashboard.view_mode {
        DashboardViewMode::Kanban => " Kanban  [v] switch to graph",
        DashboardViewMode::DependencyGraph => " Graph   [v] switch to kanban",
    };
    let mode_bar = Paragraph::new(mode_label).style(Style::default().fg(Color::DarkGray));
    frame.render_widget(mode_bar, vertical[1]);

    // Body: kanban or dependency graph
    match app.dashboard.view_mode {
        DashboardViewMode::Kanban => render_kanban(app, frame, vertical[2]),
        DashboardViewMode::DependencyGraph => {
            render_dependency_graph(&app.run, frame, vertical[2]);
        }
    }
}

/// Render the kanban board body.
fn render_kanban(app: &App, frame: &mut Frame, area: Rect) {
    let lanes = partition_lanes(&app.run.work_packages);

    // Responsive column layout (FR-021)
    let column_mode = ColumnMode::from_width(area.width);

    match column_mode {
        ColumnMode::Full => {
            // All 4 lanes
            let cols = Layout::default()
                .direction(Direction::Horizontal)
                .constraints([
                    Constraint::Percentage(25),
                    Constraint::Percentage(25),
                    Constraint::Percentage(25),
                    Constraint::Percentage(25),
                ])
                .split(area);

            for (i, col) in cols.iter().enumerate() {
                render_lane(
                    app,
                    frame,
                    *col,
                    i,
                    &lanes[i],
                    i == app.dashboard.focused_lane,
                );
            }
        }
        ColumnMode::Compact => {
            // Show 2 lanes: focused lane and one neighbor
            let (left_lane, right_lane) = compact_lane_pair(app.dashboard.focused_lane);
            let cols = Layout::default()
                .direction(Direction::Horizontal)
                .constraints([Constraint::Percentage(50), Constraint::Percentage(50)])
                .split(area);

            render_lane(
                app,
                frame,
                cols[0],
                left_lane,
                &lanes[left_lane],
                left_lane == app.dashboard.focused_lane,
            );
            render_lane(
                app,
                frame,
                cols[1],
                right_lane,
                &lanes[right_lane],
                right_lane == app.dashboard.focused_lane,
            );
        }
        ColumnMode::Single => {
            // Show only the focused lane
            let lane = app.dashboard.focused_lane;
            render_lane(app, frame, area, lane, &lanes[lane], true);
        }
    }
}

/// Select which two lanes to show in compact (2-column) mode.
///
/// Keeps the focused lane visible and pairs it with an adjacent lane.
fn compact_lane_pair(focused: usize) -> (usize, usize) {
    match focused {
        0 => (0, 1),
        1 => (0, 1),
        2 => (2, 3),
        3 => (2, 3),
        _ => (0, 1),
    }
}
