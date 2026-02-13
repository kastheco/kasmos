//! Review tab rendering.
//!
//! Renders the list of work packages awaiting operator review, with a detail
//! pane showing review information for the selected item.

use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, List, ListItem, Paragraph};
use ratatui::Frame;

use crate::types::WPState;

use super::super::app::App;

/// Render the Review tab into the given area.
pub fn render_review(app: &App, frame: &mut Frame, area: Rect) {
    let for_review: Vec<_> = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::ForReview)
        .collect();

    if for_review.is_empty() {
        let block = Block::default().borders(Borders::ALL).title(" Review ");
        let inner = block.inner(area);
        frame.render_widget(block, area);

        frame.render_widget(
            Paragraph::new(Line::from(Span::styled(
                "No work packages pending review",
                Style::default().fg(Color::DarkGray),
            ))),
            inner,
        );
        return;
    }

    // Split: list on left, detail on right
    let chunks = Layout::default()
        .direction(Direction::Horizontal)
        .constraints([Constraint::Percentage(40), Constraint::Percentage(60)])
        .split(area);

    // Review list
    let items: Vec<ListItem> = for_review
        .iter()
        .enumerate()
        .map(|(i, wp)| {
            let mut spans = vec![
                Span::styled("◉", Style::default().fg(Color::Magenta)),
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

            let style = if i == app.review.selected_index {
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD)
            } else {
                Style::default()
            };

            ListItem::new(Line::from(spans)).style(style)
        })
        .collect();

    let list = List::new(items).block(
        Block::default()
            .borders(Borders::ALL)
            .title(" Pending Review "),
    );
    frame.render_widget(list, chunks[0]);

    // Detail pane for selected review item
    let detail_block = Block::default()
        .borders(Borders::ALL)
        .title(" Review Detail ");
    let detail_inner = detail_block.inner(chunks[1]);
    frame.render_widget(detail_block, chunks[1]);

    if let Some(wp) = for_review.get(app.review.selected_index) {
        let mut lines = vec![
            Line::from(vec![
                Span::styled("ID: ", Style::default().fg(Color::Cyan)),
                Span::raw(&wp.id),
            ]),
            Line::from(vec![
                Span::styled("Title: ", Style::default().fg(Color::Cyan)),
                Span::raw(&wp.title),
            ]),
            Line::from(vec![
                Span::styled("Wave: ", Style::default().fg(Color::Cyan)),
                Span::raw(wp.wave.to_string()),
            ]),
            Line::from(vec![
                Span::styled("Failures: ", Style::default().fg(Color::Cyan)),
                if wp.failure_count > 0 {
                    Span::styled(
                        wp.failure_count.to_string(),
                        Style::default().fg(Color::Red),
                    )
                } else {
                    Span::raw("0")
                },
            ]),
        ];

        if !wp.dependencies.is_empty() {
            lines.push(Line::from(vec![
                Span::styled("Deps: ", Style::default().fg(Color::Cyan)),
                Span::raw(wp.dependencies.join(", ")),
            ]));
        }

        if let Some(path) = &wp.worktree_path {
            lines.push(Line::from(vec![
                Span::styled("Worktree: ", Style::default().fg(Color::Cyan)),
                Span::raw(path.display().to_string()),
            ]));
        }

        // Review pane status
        let has_active_review = app.review.active_review_panes.contains(&wp.id);

        lines.push(Line::default());
        if has_active_review {
            lines.push(Line::from(Span::styled(
                "Review agent running (check floating pane)",
                Style::default().fg(Color::Green),
            )));
        }

        lines.push(Line::default());
        lines.push(Line::from(vec![
            Span::styled("Keys: ", Style::default().fg(Color::DarkGray)),
            if has_active_review {
                Span::styled(
                    "Space=review(running)",
                    Style::default().fg(Color::DarkGray),
                )
            } else {
                Span::styled("Space=review", Style::default().fg(Color::Magenta))
            },
            Span::styled(
                "  a=approve  r=reject  R=reject+relaunch",
                Style::default().fg(Color::DarkGray),
            ),
        ]));

        frame.render_widget(Paragraph::new(lines), detail_inner);
    }
}
