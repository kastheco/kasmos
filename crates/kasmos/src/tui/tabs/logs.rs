//! Logs tab rendering.
//!
//! Renders the orchestration log viewer with filter support, auto-scroll,
//! and timestamp-prefixed entries.

use ratatui::layout::Rect;
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Paragraph};
use ratatui::Frame;

use super::super::app::App;

/// Render the Logs tab into the given area.
pub fn render_logs(app: &App, frame: &mut Frame, area: Rect) {
    let block = Block::default().borders(Borders::ALL).title(" Logs ");
    let inner = block.inner(area);
    frame.render_widget(block, area);

    if inner.height == 0 {
        return;
    }

    let filtered = app.filtered_log_entries();
    let reserve = if app.logs.filter_active { 2 } else { 1 };
    let list_height = usize::from(inner.height.saturating_sub(reserve));
    let max_top = filtered.len().saturating_sub(list_height);
    let top = if app.logs.auto_scroll {
        max_top
    } else {
        app.logs.scroll_offset.min(max_top)
    };

    let end = if list_height == 0 {
        top
    } else {
        (top + list_height).min(filtered.len())
    };

    let mut lines = Vec::new();
    if filtered.is_empty() {
        lines.push(Line::from(Span::styled(
            "No log entries",
            Style::default().fg(Color::DarkGray),
        )));
    } else {
        for entry in &filtered[top..end] {
            let level = match entry.level {
                crate::tui::app::LogLevel::Info => {
                    Span::styled("INFO", Style::default().fg(Color::DarkGray))
                }
                crate::tui::app::LogLevel::Warn => {
                    Span::styled("WARN", Style::default().fg(Color::Yellow))
                }
                crate::tui::app::LogLevel::Error => Span::styled(
                    "ERR ",
                    Style::default().fg(Color::Red).add_modifier(Modifier::BOLD),
                ),
            };

            let wp_prefix = entry
                .wp_id
                .as_ref()
                .map(|wp_id| format!("[{wp_id}] "))
                .unwrap_or_default();

            lines.push(Line::from(vec![
                Span::styled(
                    App::format_timestamp(entry.timestamp),
                    Style::default().fg(Color::DarkGray),
                ),
                Span::raw(" "),
                level,
                Span::raw(format!(" {wp_prefix}{}", entry.message)),
            ]));
        }
    }

    let list_area = Rect {
        x: inner.x,
        y: inner.y,
        width: inner.width,
        height: inner.height.saturating_sub(reserve),
    };
    frame.render_widget(Paragraph::new(lines), list_area);

    let paused_text = if app.logs.auto_scroll {
        "AUTO-SCROLL"
    } else {
        "PAUSED - press G to resume"
    };
    let status_style = if app.logs.auto_scroll {
        Style::default().fg(Color::DarkGray)
    } else {
        Style::default().fg(Color::Yellow)
    };

    let status_area = Rect {
        x: inner.x,
        y: inner.y + inner.height.saturating_sub(reserve),
        width: inner.width,
        height: 1,
    };

    frame.render_widget(
        Paragraph::new(Line::from(vec![
            Span::styled(paused_text, status_style),
            Span::raw("  "),
            Span::styled(
                format!("Filter: {}", app.logs.filter),
                Style::default().fg(Color::DarkGray),
            ),
        ])),
        status_area,
    );

    if app.logs.filter_active {
        let filter_area = Rect {
            x: inner.x,
            y: inner.y + inner.height.saturating_sub(1),
            width: inner.width,
            height: 1,
        };
        frame.render_widget(
            Paragraph::new(Line::from(vec![
                Span::styled("/", Style::default().fg(Color::Yellow)),
                Span::raw(&app.logs.filter),
                Span::styled("_", Style::default().fg(Color::Yellow)),
            ])),
            filter_area,
        );
    }
}
