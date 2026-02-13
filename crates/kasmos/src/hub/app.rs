//! Hub application state and rendering.
//!
//! Manages the feature list view, selection state, and ratatui rendering
//! for the hub TUI that launches when `kasmos` is invoked with no subcommand.

use ratatui::{
    layout::{Constraint, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph},
    Frame,
};

use super::scanner::{FeatureEntry, OrchestrationStatus, PlanStatus, SpecStatus, TaskProgress};

/// Which view the hub is currently displaying.
#[derive(Debug, Clone, PartialEq)]
pub enum HubView {
    /// Feature list (main view).
    List,
    /// Expanded feature detail.
    Detail { index: usize },
}

/// Input mode for the hub.
#[derive(Debug, Clone)]
pub enum InputMode {
    /// Standard navigation.
    Normal,
    /// Typing a new feature name (placeholder for WP05).
    #[allow(dead_code)]
    NewFeaturePrompt { input: String },
    /// Confirmation modal.
    #[allow(dead_code)]
    ConfirmDialog { message: String },
}

/// Hub application state.
pub struct App {
    /// Current feature list from scanner.
    pub features: Vec<FeatureEntry>,
    /// Currently highlighted feature index.
    pub selected: usize,
    /// Current view.
    pub view: HubView,
    /// Current input mode.
    pub input_mode: InputMode,
    /// Zellij session name (`None` = read-only mode).
    pub zellij_session: Option<String>,
    /// Whether the hub should quit.
    pub should_quit: bool,
    /// Status message displayed in the footer.
    pub status_message: Option<String>,
    /// Whether a manual refresh was requested.
    pub refresh_requested: bool,
    /// List state for ratatui scrolling.
    list_state: ListState,
}

impl App {
    pub fn new(features: Vec<FeatureEntry>, zellij_session: Option<String>) -> Self {
        let mut list_state = ListState::default();
        if !features.is_empty() {
            list_state.select(Some(0));
        }
        Self {
            features,
            selected: 0,
            view: HubView::List,
            input_mode: InputMode::Normal,
            zellij_session,
            should_quit: false,
            status_message: None,
            refresh_requested: false,
            list_state,
        }
    }

    /// Update the feature list from a fresh scan, preserving selection.
    pub fn update_features(&mut self, features: Vec<FeatureEntry>) {
        self.features = features;
        if self.selected >= self.features.len() && !self.features.is_empty() {
            self.selected = self.features.len() - 1;
        }
        if self.features.is_empty() {
            self.list_state.select(None);
        } else {
            self.list_state.select(Some(self.selected));
        }
    }

    pub fn select_next(&mut self) {
        if !self.features.is_empty() {
            self.selected = (self.selected + 1).min(self.features.len() - 1);
            self.list_state.select(Some(self.selected));
        }
    }

    pub fn select_previous(&mut self) {
        self.selected = self.selected.saturating_sub(1);
        self.list_state.select(Some(self.selected));
    }

    pub fn is_read_only(&self) -> bool {
        self.zellij_session.is_none()
    }

    /// Render the hub TUI.
    pub fn render(&mut self, frame: &mut Frame) {
        match self.view {
            HubView::List => self.render_list(frame),
            HubView::Detail { index } => self.render_detail(frame, index),
        }
    }

    fn render_list(&mut self, frame: &mut Frame) {
        let area = frame.area();

        // Layout: optional warning + header + list + footer
        let has_warning = self.is_read_only();
        let constraints = if has_warning {
            vec![
                Constraint::Length(1), // read-only warning
                Constraint::Length(2), // header
                Constraint::Min(1),    // list
                Constraint::Length(1), // footer
            ]
        } else {
            vec![
                Constraint::Length(2), // header
                Constraint::Min(1),    // list
                Constraint::Length(1), // footer
            ]
        };

        let chunks = Layout::vertical(constraints).split(area);
        let mut idx = 0;

        // Read-only warning banner
        if has_warning {
            let warning = Paragraph::new(Line::from(vec![Span::styled(
                " Read-only mode -- Zellij not detected ",
                Style::default()
                    .fg(Color::Black)
                    .bg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            )]));
            frame.render_widget(warning, chunks[idx]);
            idx += 1;
        }

        // Header
        let header = Paragraph::new(Line::from(vec![
            Span::styled(
                " kasmos ",
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::styled("Hub", Style::default().fg(Color::White)),
            Span::raw("  "),
            Span::styled(
                format!("{} features", self.features.len()),
                Style::default().fg(Color::DarkGray),
            ),
        ]))
        .block(Block::default().borders(Borders::BOTTOM));
        frame.render_widget(header, chunks[idx]);
        idx += 1;

        // Feature list
        let list_area = chunks[idx];
        idx += 1;

        let items: Vec<ListItem> = self
            .features
            .iter()
            .map(|f| {
                let status = format_status(f);
                ListItem::new(Line::from(vec![
                    Span::styled(
                        format!(" [{}] ", f.number),
                        Style::default().fg(Color::DarkGray),
                    ),
                    Span::styled(format!("{:<30}", f.slug), Style::default().fg(Color::White)),
                    Span::raw(" "),
                    status,
                ]))
            })
            .collect();

        let list = List::new(items)
            .highlight_style(
                Style::default()
                    .bg(Color::DarkGray)
                    .add_modifier(Modifier::BOLD),
            )
            .highlight_symbol("> ");

        frame.render_stateful_widget(list, list_area, &mut self.list_state);

        // Footer
        let footer_area = chunks[idx];
        let footer_text = if let Some(ref msg) = self.status_message {
            Line::from(vec![Span::styled(
                format!(" {msg}"),
                Style::default().fg(Color::Green),
            )])
        } else {
            Line::from(vec![
                Span::styled(" j/k", Style::default().fg(Color::Cyan)),
                Span::raw(":nav  "),
                Span::styled("Enter", Style::default().fg(Color::Cyan)),
                Span::raw(":detail  "),
                Span::styled("r", Style::default().fg(Color::Cyan)),
                Span::raw(":refresh  "),
                Span::styled("Alt+q", Style::default().fg(Color::Cyan)),
                Span::raw(":quit"),
            ])
        };
        let footer = Paragraph::new(footer_text);
        frame.render_widget(footer, footer_area);
    }

    fn render_detail(&self, frame: &mut Frame, index: usize) {
        let area = frame.area();

        let Some(feature) = self.features.get(index) else {
            // Index out of bounds — fall back to empty
            let msg = Paragraph::new("No feature selected")
                .block(Block::default().borders(Borders::ALL).title(" Detail "));
            frame.render_widget(msg, area);
            return;
        };

        let chunks = Layout::vertical([
            Constraint::Length(3), // title
            Constraint::Min(1),    // details
            Constraint::Length(1), // footer
        ])
        .split(area);

        // Title
        let title = Paragraph::new(Line::from(vec![
            Span::styled(
                format!(" [{}] ", feature.number),
                Style::default().fg(Color::DarkGray),
            ),
            Span::styled(
                &feature.slug,
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
        ]))
        .block(Block::default().borders(Borders::BOTTOM));
        frame.render_widget(title, chunks[0]);

        // Details
        let status_span = format_status(feature);
        let lines = vec![
            Line::from(vec![
                Span::styled("  Spec:          ", Style::default().fg(Color::DarkGray)),
                match &feature.spec_status {
                    SpecStatus::Present => {
                        Span::styled("present", Style::default().fg(Color::Green))
                    }
                    SpecStatus::Empty => Span::styled("empty", Style::default().fg(Color::Red)),
                },
            ]),
            Line::from(vec![
                Span::styled("  Plan:          ", Style::default().fg(Color::DarkGray)),
                match &feature.plan_status {
                    PlanStatus::Present => {
                        Span::styled("present", Style::default().fg(Color::Green))
                    }
                    PlanStatus::Absent => Span::styled("absent", Style::default().fg(Color::Red)),
                },
            ]),
            Line::from(vec![
                Span::styled("  Tasks:         ", Style::default().fg(Color::DarkGray)),
                match &feature.task_progress {
                    TaskProgress::NoTasks => {
                        Span::styled("none", Style::default().fg(Color::DarkGray))
                    }
                    TaskProgress::InProgress { done, total } => Span::styled(
                        format!("{done}/{total} done"),
                        Style::default().fg(Color::Cyan),
                    ),
                    TaskProgress::Complete { total } => Span::styled(
                        format!("all {total} complete"),
                        Style::default().fg(Color::Green),
                    ),
                },
            ]),
            Line::from(vec![
                Span::styled("  Orchestration: ", Style::default().fg(Color::DarkGray)),
                match &feature.orchestration_status {
                    OrchestrationStatus::None => {
                        Span::styled("idle", Style::default().fg(Color::DarkGray))
                    }
                    OrchestrationStatus::Running => {
                        Span::styled("running", Style::default().fg(Color::Green))
                    }
                    OrchestrationStatus::Completed => {
                        Span::styled("completed", Style::default().fg(Color::Yellow))
                    }
                },
            ]),
            Line::from(vec![
                Span::styled("  Status:        ", Style::default().fg(Color::DarkGray)),
                status_span,
            ]),
            Line::raw(""),
            Line::from(vec![Span::styled(
                format!("  Path: {}", feature.feature_dir.display()),
                Style::default().fg(Color::DarkGray),
            )]),
        ];

        let detail = Paragraph::new(lines).block(Block::default().borders(Borders::NONE));
        frame.render_widget(detail, chunks[1]);

        // Footer
        let footer = Paragraph::new(Line::from(vec![
            Span::styled(" Esc", Style::default().fg(Color::Cyan)),
            Span::raw(":back  "),
            Span::styled("Alt+q", Style::default().fg(Color::Cyan)),
            Span::raw(":quit"),
        ]));
        frame.render_widget(footer, chunks[2]);
    }
}

/// Format a single-span status indicator for a feature entry.
fn format_status(f: &FeatureEntry) -> Span<'_> {
    // Orchestration takes precedence
    if f.orchestration_status == OrchestrationStatus::Running {
        return Span::styled("[running]", Style::default().fg(Color::Green));
    }
    if f.orchestration_status == OrchestrationStatus::Completed {
        return Span::styled("[completed]", Style::default().fg(Color::Yellow));
    }

    match (&f.spec_status, &f.plan_status, &f.task_progress) {
        (SpecStatus::Empty, _, _) => {
            Span::styled("[empty spec]", Style::default().fg(Color::DarkGray))
        }
        (SpecStatus::Present, PlanStatus::Absent, _) => {
            Span::styled("[no plan]", Style::default().fg(Color::Yellow))
        }
        (SpecStatus::Present, PlanStatus::Present, TaskProgress::NoTasks) => {
            Span::styled("[no tasks]", Style::default().fg(Color::Yellow))
        }
        (_, _, TaskProgress::InProgress { done, total }) => Span::styled(
            format!("[{done}/{total} done]"),
            Style::default().fg(Color::Cyan),
        ),
        (_, _, TaskProgress::Complete { .. }) => {
            Span::styled("[complete]", Style::default().fg(Color::Green))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    fn dummy_feature(number: &str, slug: &str) -> FeatureEntry {
        FeatureEntry {
            number: number.to_string(),
            slug: slug.to_string(),
            full_slug: format!("{number}-{slug}"),
            spec_status: SpecStatus::Present,
            plan_status: PlanStatus::Present,
            task_progress: TaskProgress::InProgress { done: 1, total: 3 },
            orchestration_status: OrchestrationStatus::None,
            feature_dir: PathBuf::from(format!("kitty-specs/{number}-{slug}")),
        }
    }

    #[test]
    fn select_next_clamps() {
        let features = vec![dummy_feature("001", "a"), dummy_feature("002", "b")];
        let mut app = App::new(features, None);
        assert_eq!(app.selected, 0);
        app.select_next();
        assert_eq!(app.selected, 1);
        app.select_next();
        assert_eq!(app.selected, 1); // clamped
    }

    #[test]
    fn select_previous_saturates() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None);
        app.select_previous();
        assert_eq!(app.selected, 0); // saturated at 0
    }

    #[test]
    fn update_features_preserves_selection() {
        let features = vec![
            dummy_feature("001", "a"),
            dummy_feature("002", "b"),
            dummy_feature("003", "c"),
        ];
        let mut app = App::new(features, None);
        app.selected = 2;
        app.list_state.select(Some(2));

        // Shrink list — selection should clamp
        let smaller = vec![dummy_feature("001", "a")];
        app.update_features(smaller);
        assert_eq!(app.selected, 0);
    }

    #[test]
    fn update_features_empty_list() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None);
        app.update_features(Vec::new());
        assert_eq!(app.list_state.selected(), None);
    }

    #[test]
    fn is_read_only() {
        let app = App::new(Vec::new(), None);
        assert!(app.is_read_only());

        let app = App::new(Vec::new(), Some("test-session".to_string()));
        assert!(!app.is_read_only());
    }
}
