//! Hub application state and rendering.
//!
//! Manages the feature list view, selection state, and ratatui rendering
//! for the hub TUI that launches when `kasmos` is invoked with no subcommand.

use std::path::PathBuf;

use ratatui::{
    layout::{Constraint, Layout},
    style::{Color, Modifier, Style},
    text::{Line, Span},
    widgets::{Block, Borders, List, ListItem, ListState, Paragraph, Row, Table, TableState},
    Frame,
};

use super::scanner::{
    FeatureDetail, FeatureEntry, OrchestrationStatus, PlanStatus, SpecStatus, TaskProgress,
};

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
    /// Typing a new feature name.
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
    /// Whether the kitty-specs/ directory exists (WP08 T040).
    pub specs_dir_exists: bool,
    /// Loaded detail for the current feature (lazy, set on Enter).
    pub detail: Option<FeatureDetail>,
    /// Currently selected WP row in detail view.
    pub detail_selected: usize,
    /// Last refresh time (WP08 T044).
    pub last_refresh: std::time::Instant,
    /// Root path for kitty-specs/ (WP05).
    pub specs_root: PathBuf,
    /// List state for ratatui scrolling.
    list_state: ListState,
    /// Table state for detail view WP scrolling.
    detail_table_state: TableState,
}

impl App {
    pub fn new(
        features: Vec<FeatureEntry>,
        zellij_session: Option<String>,
        specs_dir_exists: bool,
    ) -> Self {
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
            specs_dir_exists,
            detail: None,
            detail_selected: 0,
            last_refresh: std::time::Instant::now(),
            specs_root: PathBuf::from("kitty-specs"),
            list_state,
            detail_table_state: TableState::default(),
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

    pub fn select_next_wp(&mut self) {
        if let Some(ref detail) = self.detail {
            if !detail.work_packages.is_empty() {
                self.detail_selected =
                    (self.detail_selected + 1).min(detail.work_packages.len() - 1);
                self.detail_table_state.select(Some(self.detail_selected));
            }
        }
    }

    pub fn select_previous_wp(&mut self) {
        self.detail_selected = self.detail_selected.saturating_sub(1);
        self.detail_table_state.select(Some(self.detail_selected));
    }

    pub fn is_read_only(&self) -> bool {
        self.zellij_session.is_none()
    }

    /// Render the hub TUI.
    pub fn render(&mut self, frame: &mut Frame) {
        let size = frame.area();
        // Handle narrow terminal (WP08 T043)
        if size.width < 60 || size.height < 10 {
            let msg = Paragraph::new("Terminal too small\nMinimum: 60x10")
                .style(Style::default().fg(Color::Red))
                .alignment(ratatui::layout::Alignment::Center);
            frame.render_widget(msg, size);
            return;
        }

        match self.view {
            HubView::List => self.render_list(frame),
            HubView::Detail { index } => self.render_detail(frame, index),
        }

        // Overlay the new-feature prompt if active (WP05).
        if let InputMode::NewFeaturePrompt { ref input } = self.input_mode {
            self.render_new_feature_prompt(frame, input.clone());
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

        // Check if kitty-specs/ directory is missing (WP08 T040)
        if self.features.is_empty() && !self.specs_dir_exists {
            let msg = Paragraph::new(vec![
                Line::from(""),
                Line::from(""),
                Line::from(Span::styled(
                    "No kitty-specs/ directory found",
                    Style::default()
                        .fg(Color::Yellow)
                        .add_modifier(Modifier::BOLD),
                )),
                Line::from(""),
                Line::from(Span::styled(
                    "Initialize with: mkdir kitty-specs",
                    Style::default().fg(Color::DarkGray),
                )),
            ])
            .alignment(ratatui::layout::Alignment::Center);
            frame.render_widget(msg, list_area);
        } else {
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
        }

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

    fn render_detail(&mut self, frame: &mut Frame, index: usize) {
        let area = frame.area();

        let Some(feature) = self.features.get(index) else {
            // Index out of bounds — fall back to empty
            let msg = Paragraph::new("No feature selected")
                .block(Block::default().borders(Borders::ALL).title(" Detail "));
            frame.render_widget(msg, area);
            return;
        };

        let chunks = Layout::vertical([
            Constraint::Length(3), // header
            Constraint::Min(1),    // WP table
            Constraint::Length(1), // footer
        ])
        .split(area);

        // Header: feature name + overall status
        let status_span = format_status(feature);
        let header = Paragraph::new(Line::from(vec![
            Span::styled(
                format!(" [{}] ", feature.number),
                Style::default().fg(Color::DarkGray),
            ),
            Span::styled(
                feature.slug.clone(),
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::raw("  "),
            status_span,
        ]))
        .block(Block::default().borders(Borders::BOTTOM));
        frame.render_widget(header, chunks[0]);

        // WP table
        let table_area = chunks[1];

        if let Some(ref detail) = self.detail {
            if detail.work_packages.is_empty() {
                let msg = Paragraph::new("  No work packages found")
                    .style(Style::default().fg(Color::DarkGray));
                frame.render_widget(msg, table_area);
            } else {
                let header_row = Row::new(vec![
                    Span::styled("ID", Style::default().add_modifier(Modifier::BOLD)),
                    Span::styled("Title", Style::default().add_modifier(Modifier::BOLD)),
                    Span::styled("Lane", Style::default().add_modifier(Modifier::BOLD)),
                    Span::styled(
                        "Dependencies",
                        Style::default().add_modifier(Modifier::BOLD),
                    ),
                ])
                .style(Style::default().fg(Color::White))
                .bottom_margin(1);

                let rows: Vec<Row> = detail
                    .work_packages
                    .iter()
                    .map(|wp| {
                        let lane_color = match wp.lane.as_str() {
                            "done" => Color::Green,
                            "doing" => Color::Yellow,
                            "for_review" => Color::Blue,
                            _ => Color::DarkGray, // planned or unknown
                        };
                        let deps = if wp.dependencies.is_empty() {
                            "-".to_string()
                        } else {
                            wp.dependencies.join(", ")
                        };
                        Row::new(vec![
                            Span::styled(wp.id.clone(), Style::default().fg(Color::White)),
                            Span::styled(wp.title.clone(), Style::default().fg(Color::White)),
                            Span::styled(wp.lane.clone(), Style::default().fg(lane_color)),
                            Span::styled(deps, Style::default().fg(Color::DarkGray)),
                        ])
                    })
                    .collect();

                let widths = [
                    Constraint::Length(8),
                    Constraint::Min(20),
                    Constraint::Length(12),
                    Constraint::Min(15),
                ];

                let table = Table::new(rows, widths)
                    .header(header_row)
                    .highlight_style(
                        Style::default()
                            .bg(Color::DarkGray)
                            .add_modifier(Modifier::BOLD),
                    )
                    .highlight_symbol("> ")
                    .block(Block::default().borders(Borders::NONE));

                self.detail_table_state.select(Some(self.detail_selected));
                frame.render_stateful_widget(table, table_area, &mut self.detail_table_state);
            }
        } else {
            let msg = Paragraph::new("  No work packages found")
                .style(Style::default().fg(Color::DarkGray));
            frame.render_widget(msg, table_area);
        }

        // Footer
        let footer = Paragraph::new(Line::from(vec![
            Span::styled(" j/k", Style::default().fg(Color::Cyan)),
            Span::raw(":nav  "),
            Span::styled("Esc", Style::default().fg(Color::Cyan)),
            Span::raw(":back  "),
            Span::styled("r", Style::default().fg(Color::Cyan)),
            Span::raw(":refresh  "),
            Span::styled("Alt+q", Style::default().fg(Color::Cyan)),
            Span::raw(":quit"),
        ]));
        frame.render_widget(footer, chunks[2]);
    }

    /// Render the new-feature prompt overlay at the bottom of the screen (WP05).
    fn render_new_feature_prompt(&self, frame: &mut Frame, input: String) {
        let area = frame.area();
        // Place the prompt in the last row of the terminal.
        let prompt_area = ratatui::layout::Rect {
            x: area.x,
            y: area.y + area.height.saturating_sub(1),
            width: area.width,
            height: 1,
        };

        let prompt = Paragraph::new(Line::from(vec![
            Span::styled(
                " New feature: ",
                Style::default()
                    .fg(Color::White)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::styled(&input, Style::default().fg(Color::Cyan)),
            Span::styled(
                "_",
                Style::default()
                    .fg(Color::White)
                    .add_modifier(Modifier::SLOW_BLINK),
            ),
            Span::styled(
                "  [Enter]:create [Esc]:cancel",
                Style::default().fg(Color::DarkGray),
            ),
        ]))
        .style(Style::default().bg(Color::DarkGray));
        frame.render_widget(prompt, prompt_area);
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
        let mut app = App::new(features, None, true);
        assert_eq!(app.selected, 0);
        app.select_next();
        assert_eq!(app.selected, 1);
        app.select_next();
        assert_eq!(app.selected, 1); // clamped
    }

    #[test]
    fn select_previous_saturates() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
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
        let mut app = App::new(features, None, true);
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
        let mut app = App::new(features, None, true);
        app.update_features(Vec::new());
        assert_eq!(app.list_state.selected(), None);
    }

    #[test]
    fn is_read_only() {
        let app = App::new(Vec::new(), None, true);
        assert!(app.is_read_only());

        let app = App::new(Vec::new(), Some("test-session".to_string()), true);
        assert!(!app.is_read_only());
    }

    #[test]
    fn render_list_empty_does_not_panic() {
        let mut app = App::new(Vec::new(), None, true);
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn render_list_with_features_does_not_panic() {
        let features = vec![
            dummy_feature("001", "alpha"),
            dummy_feature("002", "bravo"),
            dummy_feature("003", "charlie"),
        ];
        let mut app = App::new(features, Some("test-session".to_string()), true);
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn render_read_only_banner_does_not_panic() {
        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, None, true); // read-only
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn render_detail_view_does_not_panic() {
        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn render_detail_view_out_of_bounds_does_not_panic() {
        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 99 };
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn render_with_status_message_does_not_panic() {
        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, None, true);
        app.status_message = Some("Refreshed".to_string());
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn format_status_precedence() {
        // Running takes precedence over everything
        let running = FeatureEntry {
            number: "001".into(),
            slug: "x".into(),
            full_slug: "001-x".into(),
            spec_status: SpecStatus::Empty,
            plan_status: PlanStatus::Absent,
            task_progress: TaskProgress::NoTasks,
            orchestration_status: OrchestrationStatus::Running,
            feature_dir: PathBuf::from("/tmp"),
        };
        let span = format_status(&running);
        assert!(span.content.contains("running"));

        // Empty spec
        let empty = FeatureEntry {
            orchestration_status: OrchestrationStatus::None,
            ..running.clone()
        };
        let span = format_status(&empty);
        assert!(span.content.contains("empty spec"));

        // No plan
        let no_plan = FeatureEntry {
            spec_status: SpecStatus::Present,
            plan_status: PlanStatus::Absent,
            ..empty.clone()
        };
        let span = format_status(&no_plan);
        assert!(span.content.contains("no plan"));

        // No tasks
        let no_tasks = FeatureEntry {
            plan_status: PlanStatus::Present,
            ..no_plan.clone()
        };
        let span = format_status(&no_tasks);
        assert!(span.content.contains("no tasks"));

        // In progress
        let in_progress = FeatureEntry {
            task_progress: TaskProgress::InProgress { done: 2, total: 5 },
            ..no_tasks.clone()
        };
        let span = format_status(&in_progress);
        assert!(span.content.contains("2/5 done"));

        // Complete
        let complete = FeatureEntry {
            task_progress: TaskProgress::Complete { total: 5 },
            ..no_tasks.clone()
        };
        let span = format_status(&complete);
        assert!(span.content.contains("complete"));
    }

    #[test]
    fn render_detail_with_wp_data_does_not_panic() {
        use crate::hub::scanner::WPSummary;

        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };
        app.detail = Some(FeatureDetail {
            feature: dummy_feature("001", "alpha"),
            work_packages: vec![
                WPSummary {
                    id: "WP01".into(),
                    title: "Setup".into(),
                    lane: "done".into(),
                    dependencies: vec![],
                },
                WPSummary {
                    id: "WP02".into(),
                    title: "Implementation".into(),
                    lane: "doing".into(),
                    dependencies: vec!["WP01".into()],
                },
                WPSummary {
                    id: "WP03".into(),
                    title: "Review".into(),
                    lane: "for_review".into(),
                    dependencies: vec!["WP01".into(), "WP02".into()],
                },
            ],
        });

        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn render_detail_with_empty_wps_does_not_panic() {
        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };
        app.detail = Some(FeatureDetail {
            feature: dummy_feature("001", "alpha"),
            work_packages: vec![],
        });

        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }

    #[test]
    fn select_next_wp_clamps() {
        use crate::hub::scanner::WPSummary;

        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        app.detail = Some(FeatureDetail {
            feature: dummy_feature("001", "a"),
            work_packages: vec![
                WPSummary {
                    id: "WP01".into(),
                    title: "A".into(),
                    lane: "done".into(),
                    dependencies: vec![],
                },
                WPSummary {
                    id: "WP02".into(),
                    title: "B".into(),
                    lane: "doing".into(),
                    dependencies: vec![],
                },
            ],
        });

        assert_eq!(app.detail_selected, 0);
        app.select_next_wp();
        assert_eq!(app.detail_selected, 1);
        app.select_next_wp();
        assert_eq!(app.detail_selected, 1); // clamped
    }

    #[test]
    fn select_previous_wp_saturates() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        app.detail = Some(FeatureDetail {
            feature: dummy_feature("001", "a"),
            work_packages: vec![],
        });

        app.select_previous_wp();
        assert_eq!(app.detail_selected, 0); // saturated
    }

    #[test]
    fn render_new_feature_prompt_does_not_panic() {
        let features = vec![dummy_feature("001", "alpha")];
        let mut app = App::new(features, Some("session".to_string()), true);
        app.input_mode = InputMode::NewFeaturePrompt {
            input: "my-feature".to_string(),
        };
        let backend = ratatui::backend::TestBackend::new(80, 24);
        let mut terminal = ratatui::Terminal::new(backend).unwrap();
        terminal.draw(|frame| app.render(frame)).unwrap();
    }
}
