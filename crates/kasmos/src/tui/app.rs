//! TUI application state.
//!
//! Contains the root `App` struct and supporting types that hold all UI state:
//! active tab, per-tab scroll/selection state, notifications, and the latest
//! orchestration run snapshot from the engine.

use crate::command_handlers::EngineAction;
use crate::review::{
    ReviewAutomationPolicy, ReviewFailureSeverity, ReviewFailureType, ReviewPolicyExecutor,
};
use crate::types::{OrchestrationRun, WPState, WorkPackage};
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, List, ListItem, Paragraph, Tabs};
use ratatui::Frame;
use std::collections::HashMap;
use std::time::{Instant, SystemTime, UNIX_EPOCH};
use tokio::sync::mpsc;

use super::event::TuiEvent;
use super::keybindings;

// ---------------------------------------------------------------------------
// Confirmation dialog types
// ---------------------------------------------------------------------------

/// A pending confirmation dialog action.
#[derive(Debug, Clone)]
pub enum ConfirmAction {
    /// Force-advance a work package past its current state.
    ForceAdvance { wp_id: String },
    /// Abort the entire orchestration run.
    AbortRun,
}

impl ConfirmAction {
    /// Dialog title for the confirmation popup.
    pub fn title(&self) -> &str {
        match self {
            ConfirmAction::ForceAdvance { .. } => "Confirm Force Advance",
            ConfirmAction::AbortRun => "Confirm Abort",
        }
    }

    /// Dialog body text describing the action and consequences.
    pub fn description(&self) -> String {
        match self {
            ConfirmAction::ForceAdvance { wp_id } => {
                format!(
                    "Force-advance {} past its current state?\n\
                     This skips remaining work. Press [y] to confirm, [n] to cancel.",
                    wp_id
                )
            }
            ConfirmAction::AbortRun => "Abort the entire orchestration run?\n\
                 All active work packages will be stopped. Press [y] to confirm, [n] to cancel."
                .to_string(),
        }
    }
}

// ---------------------------------------------------------------------------
// Lane helpers
// ---------------------------------------------------------------------------

/// Lane names for the kanban board.
const LANE_NAMES: [&str; 4] = ["Planned", "Doing", "Review", "Done"];

/// Map a `WPState` to a kanban lane index.
///
/// - Lane 0 "Planned": Pending, Paused
/// - Lane 1 "Doing": Active
/// - Lane 2 "Review": ForReview
/// - Lane 3 "Done": Completed, Failed
pub(super) fn wp_lane(state: WPState) -> usize {
    match state {
        WPState::Pending | WPState::Paused => 0,
        WPState::Active => 1,
        WPState::ForReview => 2,
        WPState::Completed | WPState::Failed => 3,
    }
}

// ---------------------------------------------------------------------------
// Tab enum
// ---------------------------------------------------------------------------

/// The available tabs in the TUI.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Tab {
    Dashboard,
    Review,
    Logs,
}

impl Tab {
    /// Return the tab index (0-based) for rendering.
    pub fn index(self) -> usize {
        match self {
            Tab::Dashboard => 0,
            Tab::Review => 1,
            Tab::Logs => 2,
        }
    }

    /// All tab titles for the tab bar.
    pub fn titles() -> Vec<&'static str> {
        vec!["[1] Dashboard", "[2] Review", "[3] Logs"]
    }
}

// ---------------------------------------------------------------------------
// Notification types
// ---------------------------------------------------------------------------

/// The kind of attention a notification represents.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum NotificationKind {
    /// A work package reached the `for_review` lane.
    ReviewPending,
    /// A work package entered the `Failed` state.
    Failure,
    /// An agent signaled it needs operator input.
    InputNeeded,
}

/// An attention item surfaced in the persistent notification bar.
#[derive(Debug, Clone)]
pub struct Notification {
    /// Unique notification ID (monotonically increasing).
    pub id: u64,
    /// What kind of attention is needed.
    pub kind: NotificationKind,
    /// The work package this notification refers to.
    pub wp_id: String,
    /// Optional message (populated for InputNeeded with the agent's question).
    pub message: Option<String>,
    /// Failure type for review-automation failures.
    pub failure_type: Option<ReviewFailureType>,
    /// Failure severity for review-automation failures.
    pub severity: Option<ReviewFailureSeverity>,
    /// When this notification was created.
    pub created_at: Instant,
}

// ---------------------------------------------------------------------------
// Per-tab state structs
// ---------------------------------------------------------------------------

/// Which view is active in the Dashboard tab.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
pub enum DashboardViewMode {
    /// Standard kanban lane view (Planned / Doing / ForReview / Done).
    #[default]
    Kanban,
    /// Directed graph showing WP dependency relationships.
    DependencyGraph,
}

/// UI state for the Dashboard tab.
#[derive(Debug, Default)]
pub struct DashboardState {
    /// Which lane column is focused (0=planned, 1=doing, 2=for_review, 3=done).
    pub focused_lane: usize,
    /// Selected WP index within the focused lane.
    pub selected_index: usize,
    /// Vertical scroll offset per lane.
    pub scroll_offsets: [usize; 4],
    /// Current Dashboard sub-view mode (Kanban vs DependencyGraph).
    pub view_mode: DashboardViewMode,
}

/// UI state for the Review tab.
#[derive(Debug, Default)]
pub struct ReviewState {
    /// Index of the selected review item in the for_review list.
    pub selected_index: usize,
    /// Scroll offset for the review detail pane.
    pub detail_scroll: usize,
}

/// A single log entry in the orchestration log.
#[derive(Debug, Clone)]
pub struct LogEntry {
    /// When the event occurred.
    pub timestamp: SystemTime,
    /// Severity level.
    pub level: LogLevel,
    /// Associated work package ID, if any.
    pub wp_id: Option<String>,
    /// Human-readable message.
    pub message: String,
}

/// Log severity level.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LogLevel {
    Info,
    Warn,
    Error,
}

/// UI state for the Logs tab.
#[derive(Debug)]
pub struct LogsState {
    /// All log entries.
    pub entries: Vec<LogEntry>,
    /// Active filter text (empty = show all).
    pub filter: String,
    /// Whether the filter input field is active.
    pub filter_active: bool,
    /// Scroll offset into the (filtered) entries list.
    pub scroll_offset: usize,
    /// Whether to auto-scroll to the bottom on new entries.
    pub auto_scroll: bool,
}

impl Default for LogsState {
    fn default() -> Self {
        Self {
            entries: Vec::new(),
            filter: String::new(),
            filter_active: false,
            scroll_offset: 0,
            auto_scroll: true,
        }
    }
}

// ---------------------------------------------------------------------------
// App (root TUI state)
// ---------------------------------------------------------------------------

/// Root application state for the TUI.
///
/// Holds the latest orchestration run snapshot, per-tab UI state, notifications,
/// and the channel for sending commands back to the engine.
pub struct App {
    /// Latest orchestration state from the engine (via watch channel).
    pub run: OrchestrationRun,
    /// Currently active tab.
    pub active_tab: Tab,
    /// Active notifications requiring operator attention.
    pub notifications: Vec<Notification>,
    /// Dashboard tab UI state.
    pub dashboard: DashboardState,
    /// Review tab UI state.
    pub review: ReviewState,
    /// Logs tab UI state.
    pub logs: LogsState,
    /// Currently pending confirmation dialog, if any.
    pub pending_confirm: Option<ConfirmAction>,
    /// Channel to send commands to the engine.
    pub action_tx: mpsc::Sender<EngineAction>,
    /// Exit flag — when true, the event loop breaks.
    pub should_quit: bool,
    /// Monotonically increasing counter for notification IDs.
    notification_counter: u64,
    /// Executor for `for_review` policy decisions.
    review_policy_executor: ReviewPolicyExecutor,
}

impl App {
    /// Create a new App with the initial orchestration run and action channel.
    pub fn new(run: OrchestrationRun, action_tx: mpsc::Sender<EngineAction>) -> Self {
        Self {
            run,
            active_tab: Tab::Dashboard,
            notifications: Vec::new(),
            dashboard: DashboardState::default(),
            review: ReviewState::default(),
            logs: LogsState::default(),
            pending_confirm: None,
            action_tx,
            should_quit: false,
            notification_counter: 0,
            review_policy_executor: ReviewPolicyExecutor::new(ReviewAutomationPolicy::default()),
        }
    }

    /// Set the review automation policy used at `for_review` transitions.
    pub fn set_review_policy(&mut self, policy: ReviewAutomationPolicy) {
        self.review_policy_executor = ReviewPolicyExecutor::new(policy);
    }

    /// Record a typed review automation failure in both notifications and logs.
    pub fn record_review_failure(
        &mut self,
        wp_id: impl Into<String>,
        failure_type: ReviewFailureType,
        message: impl Into<String>,
    ) {
        let wp_id = wp_id.into();
        let message = message.into();
        let notification_id = self.next_notification_id();

        self.notifications.push(Notification {
            id: notification_id,
            kind: NotificationKind::Failure,
            wp_id: wp_id.clone(),
            message: Some(message.clone()),
            failure_type: Some(failure_type),
            severity: Some(ReviewFailureSeverity::Error),
            created_at: Instant::now(),
        });

        self.logs.entries.push(LogEntry {
            timestamp: SystemTime::now(),
            level: LogLevel::Error,
            wp_id: Some(wp_id.clone()),
            message: format!("review_failure {:?}: {}", failure_type, message),
        });

        if self.logs.entries.len() > 10_000 {
            let overflow = self.logs.entries.len() - 10_000;
            self.logs.entries.drain(..overflow);
            if !self.logs.auto_scroll {
                self.logs.scroll_offset = self.logs.scroll_offset.saturating_sub(overflow);
            }
        }
    }

    /// Handle a terminal event (key, mouse, resize).
    pub fn handle_event(&mut self, event: TuiEvent) {
        match event {
            TuiEvent::Key(key) => keybindings::handle_key(self, key),
            TuiEvent::Mouse(_mouse) => {
                // Mouse handling will be added in later WPs
            }
            TuiEvent::Resize(_w, _h) => {
                // ratatui handles resize automatically via terminal.draw()
            }
        }
    }

    /// Update the orchestration run snapshot from the engine.
    ///
    /// Diffs old vs new WP states to add/remove notifications, then captures
    /// log entries and replaces the run snapshot.
    pub fn update_state(&mut self, new_run: OrchestrationRun) {
        // --- Notification diffing: compare old and new WP states ---
        let old_states: HashMap<String, WPState> = self
            .run
            .work_packages
            .iter()
            .map(|wp| (wp.id.clone(), wp.state))
            .collect();

        for wp in &new_run.work_packages {
            let old_state = old_states.get(&wp.id).copied();

            // Add notifications for transitions INTO attention states
            if old_state != Some(WPState::ForReview) && wp.state == WPState::ForReview {
                let id = self.next_notification_id();
                self.notifications.push(Notification {
                    id,
                    kind: NotificationKind::ReviewPending,
                    wp_id: wp.id.clone(),
                    message: Some(format!("{} is ready for review", wp.title)),
                    failure_type: None,
                    severity: None,
                    created_at: Instant::now(),
                });
            }

            if old_state != Some(WPState::Failed) && wp.state == WPState::Failed {
                let id = self.next_notification_id();
                self.notifications.push(Notification {
                    id,
                    kind: NotificationKind::Failure,
                    wp_id: wp.id.clone(),
                    message: Some(format!("{} has failed", wp.title)),
                    failure_type: None,
                    severity: None,
                    created_at: Instant::now(),
                });
            }

            // Auto-dismiss notifications for transitions OUT of attention states
            if old_state == Some(WPState::ForReview) && wp.state != WPState::ForReview {
                self.notifications
                    .retain(|n| !(n.kind == NotificationKind::ReviewPending && n.wp_id == wp.id));
            }

            if old_state == Some(WPState::Failed) && wp.state != WPState::Failed {
                self.notifications
                    .retain(|n| !(n.kind == NotificationKind::Failure && n.wp_id == wp.id));
            }
        }

        self.capture_state_logs(&new_run);
        self.run = new_run;

        if self.logs.entries.len() > 10_000 {
            let overflow = self.logs.entries.len() - 10_000;
            self.logs.entries.drain(..overflow);
            if !self.logs.auto_scroll {
                self.logs.scroll_offset = self.logs.scroll_offset.saturating_sub(overflow);
            }
        }
    }

    /// Periodic tick handler for elapsed time display updates.
    ///
    /// Called every 250ms from the event loop.
    pub fn on_tick(&mut self) {
        // Placeholder — elapsed time formatting will use this in WP03
    }

    fn capture_state_logs(&mut self, new_run: &OrchestrationRun) {
        let old_states: HashMap<&str, WPState> = self
            .run
            .work_packages
            .iter()
            .map(|wp| (wp.id.as_str(), wp.state))
            .collect();

        for wp in &new_run.work_packages {
            let old_state = old_states.get(wp.id.as_str()).copied();
            if old_state == Some(wp.state) {
                continue;
            }

            let from = old_state
                .map(|state| format!("{state:?}"))
                .unwrap_or_else(|| "(new)".to_string());

            let level = match wp.state {
                WPState::Failed => LogLevel::Error,
                WPState::ForReview => LogLevel::Warn,
                _ => LogLevel::Info,
            };

            self.logs.entries.push(LogEntry {
                timestamp: SystemTime::now(),
                level,
                wp_id: Some(wp.id.clone()),
                message: format!("{from} -> {:?}", wp.state),
            });

            if old_state != Some(WPState::ForReview) && wp.state == WPState::ForReview {
                let decision = self.review_policy_executor.on_for_review_transition();
                self.logs.entries.push(LogEntry {
                    timestamp: SystemTime::now(),
                    level: LogLevel::Info,
                    wp_id: Some(wp.id.clone()),
                    message: format!(
                        "review_policy {:?}: run_automation={}, auto_mark_done={}",
                        self.review_policy_executor.policy(),
                        decision.run_automation,
                        decision.auto_mark_done
                    ),
                });
            }
        }

        if new_run.state != self.run.state {
            self.logs.entries.push(LogEntry {
                timestamp: SystemTime::now(),
                level: LogLevel::Info,
                wp_id: None,
                message: format!("Run state: {:?} -> {:?}", self.run.state, new_run.state),
            });
        }
    }

    fn filtered_log_entries(&self) -> Vec<&LogEntry> {
        if self.logs.filter.is_empty() {
            return self.logs.entries.iter().collect();
        }

        let needle = self.logs.filter.to_ascii_lowercase();
        self.logs
            .entries
            .iter()
            .filter(|entry| {
                entry.message.to_ascii_lowercase().contains(&needle)
                    || entry
                        .wp_id
                        .as_deref()
                        .unwrap_or_default()
                        .to_ascii_lowercase()
                        .contains(&needle)
            })
            .collect()
    }

    fn format_timestamp(timestamp: SystemTime) -> String {
        match timestamp.duration_since(UNIX_EPOCH) {
            Ok(duration) => {
                let total = duration.as_secs() % 86_400;
                let hours = total / 3_600;
                let minutes = (total % 3_600) / 60;
                let seconds = total % 60;
                format!("{hours:02}:{minutes:02}:{seconds:02}")
            }
            Err(_) => "00:00:00".to_string(),
        }
    }

    /// Render the Review tab: left pane (review queue list) + right pane (detail).
    fn render_review(&self, frame: &mut Frame, area: Rect) {
        let review_wps: Vec<&WorkPackage> = self
            .run
            .work_packages
            .iter()
            .filter(|wp| wp.state == WPState::ForReview)
            .collect();

        // Empty state
        if review_wps.is_empty() {
            let body = Paragraph::new(Line::from(Span::styled(
                "No work packages awaiting review",
                Style::default().fg(Color::DarkGray),
            )))
            .block(Block::default().borders(Borders::ALL).title(" Review "));
            frame.render_widget(body, area);
            return;
        }

        // Split horizontally: 40% list, 60% detail
        let panes = Layout::default()
            .direction(Direction::Horizontal)
            .constraints([Constraint::Percentage(40), Constraint::Percentage(60)])
            .split(area);

        // --- Left pane: review queue list ---
        let selected = self
            .review
            .selected_index
            .min(review_wps.len().saturating_sub(1));
        let items: Vec<ListItem> = review_wps
            .iter()
            .enumerate()
            .map(|(i, wp)| {
                let style = if i == selected {
                    Style::default().add_modifier(Modifier::REVERSED)
                } else {
                    Style::default()
                };
                ListItem::new(Line::from(vec![
                    Span::styled(&wp.id, Style::default().fg(Color::Cyan)),
                    Span::raw(" "),
                    Span::raw(&wp.title),
                ]))
                .style(style)
            })
            .collect();

        let list_title = format!(" Review Queue ({}) ", review_wps.len());
        let list = List::new(items).block(Block::default().borders(Borders::ALL).title(list_title));
        frame.render_widget(list, panes[0]);

        // --- Right pane: detail for selected WP ---
        let wp = review_wps[selected];

        let mut detail_lines: Vec<Line> = Vec::new();

        // ID and title
        detail_lines.push(Line::from(vec![
            Span::styled("ID:    ", Style::default().fg(Color::DarkGray)),
            Span::styled(
                &wp.id,
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
        ]));
        detail_lines.push(Line::from(vec![
            Span::styled("Title: ", Style::default().fg(Color::DarkGray)),
            Span::raw(&wp.title),
        ]));

        // Wave assignment
        detail_lines.push(Line::from(vec![
            Span::styled("Wave:  ", Style::default().fg(Color::DarkGray)),
            Span::raw(format!("{}", wp.wave)),
        ]));

        // Time in review (approximate with elapsed since started_at)
        let time_str = wp
            .started_at
            .and_then(|started| started.elapsed().ok())
            .map(|elapsed| {
                let secs = elapsed.as_secs();
                let mins = secs / 60;
                let hrs = mins / 60;
                if hrs > 0 {
                    format!("{}h{}m", hrs, mins % 60)
                } else if mins > 0 {
                    format!("{}m{}s", mins, secs % 60)
                } else {
                    format!("{}s", secs)
                }
            })
            .unwrap_or_else(|| "\u{2014}".to_string());
        detail_lines.push(Line::from(vec![
            Span::styled("Time:  ", Style::default().fg(Color::DarkGray)),
            Span::raw(time_str),
        ]));

        // Dependencies
        let deps_str = if wp.dependencies.is_empty() {
            "(none)".to_string()
        } else {
            wp.dependencies.join(", ")
        };
        detail_lines.push(Line::from(vec![
            Span::styled("Deps:  ", Style::default().fg(Color::DarkGray)),
            Span::raw(deps_str),
        ]));

        // State badge
        detail_lines.push(Line::from(vec![
            Span::styled("State: ", Style::default().fg(Color::DarkGray)),
            Span::styled(
                format!("{:?}", wp.state),
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
        ]));

        // Spacer
        detail_lines.push(Line::from(""));

        // Action hints
        detail_lines.push(Line::from(vec![
            Span::styled(
                "[a]",
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::raw(" Approve  "),
            Span::styled(
                "[r]",
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            ),
            Span::raw(" Reject + Relaunch"),
        ]));

        let detail_block = Block::default().borders(Borders::ALL).title(" Details ");
        let detail = Paragraph::new(detail_lines).block(detail_block);
        frame.render_widget(detail, panes[1]);
    }

    /// Render the notification bar between the tab bar and body.
    fn render_notification_bar(&self, frame: &mut Frame, area: Rect) {
        let review_count = self
            .notifications
            .iter()
            .filter(|n| n.kind == NotificationKind::ReviewPending)
            .count();
        let failure_count = self
            .notifications
            .iter()
            .filter(|n| n.kind == NotificationKind::Failure)
            .count();

        let mut spans: Vec<Span> = vec![Span::raw("  ")];

        if review_count > 0 {
            spans.push(Span::styled(
                format!("{} review", review_count),
                Style::default()
                    .fg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ));
        }

        if review_count > 0 && failure_count > 0 {
            spans.push(Span::raw(" | "));
        }

        if failure_count > 0 {
            spans.push(Span::styled(
                format!("{} failed", failure_count),
                Style::default().fg(Color::Red).add_modifier(Modifier::BOLD),
            ));
        }

        spans.push(Span::styled(
            "  [n] jump",
            Style::default().fg(Color::DarkGray),
        ));

        let bar = Paragraph::new(Line::from(spans))
            .style(Style::default().bg(Color::DarkGray).fg(Color::White));
        frame.render_widget(bar, area);
    }

    fn render_logs(&self, frame: &mut Frame, area: Rect) {
        let block = Block::default().borders(Borders::ALL).title(" Logs ");
        let inner = block.inner(area);
        frame.render_widget(block, area);

        if inner.height == 0 {
            return;
        }

        let filtered = self.filtered_log_entries();
        let reserve = if self.logs.filter_active { 2 } else { 1 };
        let list_height = usize::from(inner.height.saturating_sub(reserve));
        let max_top = filtered.len().saturating_sub(list_height);
        let top = if self.logs.auto_scroll {
            max_top
        } else {
            self.logs.scroll_offset.min(max_top)
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
                    LogLevel::Info => Span::styled("INFO", Style::default().fg(Color::DarkGray)),
                    LogLevel::Warn => Span::styled("WARN", Style::default().fg(Color::Yellow)),
                    LogLevel::Error => Span::styled(
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
                        Self::format_timestamp(entry.timestamp),
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

        let paused_text = if self.logs.auto_scroll {
            "AUTO-SCROLL"
        } else {
            "PAUSED - press G to resume"
        };
        let status_style = if self.logs.auto_scroll {
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
                    format!("Filter: {}", self.logs.filter),
                    Style::default().fg(Color::DarkGray),
                ),
            ])),
            status_area,
        );

        if self.logs.filter_active {
            let filter_area = Rect {
                x: inner.x,
                y: inner.y + inner.height.saturating_sub(1),
                width: inner.width,
                height: 1,
            };
            frame.render_widget(
                Paragraph::new(Line::from(vec![
                    Span::styled("/", Style::default().fg(Color::Yellow)),
                    Span::raw(&self.logs.filter),
                    Span::styled("_", Style::default().fg(Color::Yellow)),
                ])),
                filter_area,
            );
        }
    }

    /// Allocate a new unique notification ID.
    pub fn next_notification_id(&mut self) -> u64 {
        self.notification_counter += 1;
        self.notification_counter
    }

    /// Get WP references in a given lane (for navigation bounds and rendering).
    pub fn wps_in_lane(&self, lane: usize) -> Vec<&WorkPackage> {
        self.run
            .work_packages
            .iter()
            .filter(|wp| wp_lane(wp.state) == lane)
            .collect()
    }

    /// Render the Dashboard into the given area.
    ///
    /// Dispatches between the kanban board view and the dependency graph view
    /// based on `DashboardViewMode`.
    fn render_dashboard(&self, frame: &mut Frame, area: Rect) {
        match self.dashboard.view_mode {
            DashboardViewMode::Kanban => self.render_dashboard_kanban(frame, area),
            DashboardViewMode::DependencyGraph => {
                self.render_dashboard_graph(frame, area);
            }
        }
    }

    /// Render the dependency graph view in the Dashboard tab.
    fn render_dashboard_graph(&self, frame: &mut Frame, area: Rect) {
        // Split area into graph body + hint bar at bottom.
        let vert_chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([Constraint::Min(0), Constraint::Length(1)])
            .split(area);

        let graph_area = vert_chunks[0];
        let hint_area = vert_chunks[1];

        // Render the graph
        super::widgets::dependency_graph::render_dependency_graph(&self.run, frame, graph_area);

        // Hint bar
        let hint = Paragraph::new(Line::from(Span::styled(
            "[v] Switch to Kanban view",
            Style::default().fg(Color::DarkGray),
        )));
        frame.render_widget(hint, hint_area);
    }

    /// Render the Dashboard kanban board into the given area.
    fn render_dashboard_kanban(&self, frame: &mut Frame, area: Rect) {
        // Split area into kanban columns + action hint bar at the bottom.
        let vert_chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([Constraint::Min(0), Constraint::Length(1)])
            .split(area);

        let kanban_area = vert_chunks[0];
        let hint_area = vert_chunks[1];

        // 4 equal columns for the lanes.
        let columns = Layout::default()
            .direction(Direction::Horizontal)
            .constraints([
                Constraint::Percentage(25),
                Constraint::Percentage(25),
                Constraint::Percentage(25),
                Constraint::Percentage(25),
            ])
            .split(kanban_area);

        // Partition WPs into lanes.
        let lanes: [Vec<&WorkPackage>; 4] = [
            self.wps_in_lane(0),
            self.wps_in_lane(1),
            self.wps_in_lane(2),
            self.wps_in_lane(3),
        ];

        for (lane_idx, (lane_wps, col_area)) in lanes.iter().zip(columns.iter()).enumerate() {
            let is_focused = lane_idx == self.dashboard.focused_lane;
            let title = format!(" {} ({}) ", LANE_NAMES[lane_idx], lane_wps.len());

            let border_style = if is_focused {
                Style::default().fg(Color::Yellow)
            } else {
                Style::default()
            };

            let block = Block::default()
                .borders(Borders::ALL)
                .title(title)
                .border_style(border_style);

            if lane_wps.is_empty() {
                let items = vec![ListItem::new(Line::from(Span::styled(
                    "(empty)",
                    Style::default().fg(Color::DarkGray),
                )))];
                let list = List::new(items).block(block);
                frame.render_widget(list, *col_area);
            } else {
                let items: Vec<ListItem> = lane_wps
                    .iter()
                    .enumerate()
                    .map(|(i, wp)| {
                        let state_color = match wp.state {
                            WPState::Pending => Color::DarkGray,
                            WPState::Active => Color::Green,
                            WPState::Paused => Color::Yellow,
                            WPState::ForReview => Color::Cyan,
                            WPState::Completed => Color::Blue,
                            WPState::Failed => Color::Red,
                        };

                        let mut spans = vec![
                            Span::styled(
                                wp.id.to_string(),
                                if wp.state == WPState::Failed {
                                    Style::default()
                                        .fg(state_color)
                                        .add_modifier(Modifier::BOLD)
                                } else {
                                    Style::default().fg(state_color)
                                },
                            ),
                            Span::raw(" "),
                            Span::raw(&wp.title),
                        ];

                        // Show elapsed time for Active WPs with started_at.
                        if wp.state == WPState::Active
                            && let Some(started) = wp.started_at
                            && let Ok(elapsed) = started.elapsed()
                        {
                            let secs = elapsed.as_secs();
                            let mins = secs / 60;
                            let hrs = mins / 60;
                            let elapsed_str = if hrs > 0 {
                                format!(" ({}h{}m)", hrs, mins % 60)
                            } else if mins > 0 {
                                format!(" ({}m{}s)", mins, secs % 60)
                            } else {
                                format!(" ({}s)", secs)
                            };
                            spans.push(Span::styled(
                                elapsed_str,
                                Style::default().fg(Color::DarkGray),
                            ));
                        }

                        let mut style = Style::default();
                        if is_focused && i == self.dashboard.selected_index {
                            style = style.add_modifier(Modifier::REVERSED);
                        }

                        ListItem::new(Line::from(spans)).style(style)
                    })
                    .collect();

                let list = List::new(items).block(block);
                frame.render_widget(list, *col_area);
            }
        }

        // Action hint bar for the currently selected WP.
        let mut hint_text = self.dashboard_action_hints();
        if !hint_text.is_empty() {
            hint_text.push_str("  ");
        }
        hint_text.push_str("[v] Graph view");
        let hint = Paragraph::new(Line::from(Span::styled(
            hint_text,
            Style::default().fg(Color::DarkGray),
        )));
        frame.render_widget(hint, hint_area);
    }

    /// Build the action hint string for the currently selected WP.
    fn dashboard_action_hints(&self) -> String {
        let lane_wps = self.wps_in_lane(self.dashboard.focused_lane);
        let Some(wp) = lane_wps.get(self.dashboard.selected_index) else {
            return String::new();
        };

        let mut hints = Vec::new();
        match wp.state {
            WPState::Failed => {
                hints.push("[R]estart");
                hints.push("[T] Retry");
                hints.push("[F]orce-advance");
            }
            WPState::Paused => {
                hints.push("[R]estart");
                hints.push("[P] Resume");
            }
            WPState::Active => {
                hints.push("[P]ause");
            }
            _ => {}
        }

        if self.run.mode == crate::types::ProgressionMode::WaveGated
            && self.run.state == crate::types::RunState::Paused
        {
            hints.push("[A]dvance wave");
        }

        hints.join("  ")
    }

    /// Render the entire TUI frame.
    ///
    /// Layout:
    /// - Tab header bar at top
    /// - Optional notification bar (when notifications exist)
    /// - Body: tab-specific content
    pub fn render(&self, frame: &mut Frame) {
        let area = frame.area();

        let has_notifications = !self.notifications.is_empty();
        let constraints = if has_notifications {
            vec![
                Constraint::Length(3), // tab bar
                Constraint::Length(1), // notification bar
                Constraint::Min(0),    // body
            ]
        } else {
            vec![
                Constraint::Length(3), // tab bar
                Constraint::Min(0),    // body
            ]
        };

        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints(constraints)
            .split(area);

        // Tab bar
        let titles: Vec<Line> = Tab::titles()
            .iter()
            .map(|t| Line::from(Span::raw(*t)))
            .collect();

        let tabs = Tabs::new(titles)
            .block(Block::default().borders(Borders::ALL).title(" kasmos "))
            .select(self.active_tab.index())
            .style(Style::default().fg(Color::White))
            .highlight_style(
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            );

        frame.render_widget(tabs, chunks[0]);

        // Notification bar (when present)
        let body_area = if has_notifications {
            self.render_notification_bar(frame, chunks[1]);
            chunks[2]
        } else {
            chunks[1]
        };

        // Body — tab-specific content
        match self.active_tab {
            Tab::Dashboard => {
                self.render_dashboard(frame, body_area);
            }
            Tab::Review => {
                self.render_review(frame, body_area);
            }
            Tab::Logs => {
                self.render_logs(frame, body_area);
            }
        }

        // Confirmation popup overlay (renders on top of everything)
        if let Some(ref action) = self.pending_confirm {
            let popup = tui_popup::Popup::new(action.description())
                .title(action.title())
                .style(Style::default().fg(Color::White).bg(Color::Red));
            frame.render_widget(popup, frame.area());
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::review::ReviewAutomationPolicy;
    use crate::types::{ProgressionMode, RunState, Wave, WaveState, WorkPackage};
    use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};
    use ratatui::backend::TestBackend;
    use ratatui::Terminal;

    fn create_test_run(wp_count: usize) -> OrchestrationRun {
        let mut work_packages = Vec::with_capacity(wp_count);
        for i in 0..wp_count {
            work_packages.push(WorkPackage {
                id: format!("WP{:02}", i + 1),
                title: format!("Work package {}", i + 1),
                state: WPState::Active,
                dependencies: vec![],
                wave: i / 5,
                pane_id: None,
                pane_name: format!("wp{:02}", i + 1),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            });
        }

        OrchestrationRun {
            id: "run-1".to_string(),
            feature: "feature".to_string(),
            feature_dir: std::path::PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages,
            waves: vec![Wave {
                index: 0,
                wp_ids: vec!["WP01".to_string()],
                state: WaveState::Active,
            }],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::WaveGated,
        }
    }

    #[test]
    fn test_review_policy_mode_selection_and_auto_mark_done_path() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(1), tx);

        app.set_review_policy(ReviewAutomationPolicy::ManualOnly);
        let mut to_for_review = app.run.clone();
        to_for_review.work_packages[0].state = WPState::ForReview;
        app.update_state(to_for_review);
        assert!(app.logs.entries.iter().any(|entry| {
            entry
                .message
                .contains("review_policy ManualOnly: run_automation=false, auto_mark_done=false")
        }));

        app.set_review_policy(ReviewAutomationPolicy::AutoAndMarkDone);
        let mut active_again = app.run.clone();
        active_again.work_packages[0].state = WPState::Active;
        app.update_state(active_again.clone());
        active_again.work_packages[0].state = WPState::ForReview;
        app.update_state(active_again);
        assert!(app.logs.entries.iter().any(|entry| {
            entry
                .message
                .contains("review_policy AutoAndMarkDone: run_automation=true, auto_mark_done=true")
        }));
    }

    #[test]
    fn test_review_failure_surfaces_notification_and_log_entry() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(1), tx);

        app.record_review_failure(
            "WP01",
            ReviewFailureType::Timeout,
            "review command exceeded timeout",
        );

        assert_eq!(app.notifications.len(), 1);
        let notification = &app.notifications[0];
        assert_eq!(notification.kind, NotificationKind::Failure);
        assert_eq!(notification.wp_id, "WP01");
        assert_eq!(notification.failure_type, Some(ReviewFailureType::Timeout));
        assert_eq!(notification.severity, Some(ReviewFailureSeverity::Error));

        assert!(app.logs.entries.iter().any(|entry| {
            entry.level == LogLevel::Error
                && entry.wp_id.as_deref() == Some("WP01")
                && entry.message.contains("review_failure Timeout")
        }));
    }

    #[test]
    fn test_resize_reflow_render_does_not_panic() {
        let (tx, _rx) = mpsc::channel(4);
        let app = App::new(create_test_run(12), tx);

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("initial draw");

        terminal.backend_mut().resize(80, 20);
        terminal
            .draw(|frame| app.render(frame))
            .expect("draw after resize");

        let resized = terminal.size().expect("size after resize");
        assert_eq!(resized.width, 80);
        assert_eq!(resized.height, 20);
    }

    #[test]
    fn test_keyboard_only_flow_without_mouse_events() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('2'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.active_tab, Tab::Review);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('3'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.active_tab, Tab::Logs);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('/'),
            KeyModifiers::NONE,
        )));
        assert!(app.logs.filter_active);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Esc,
            KeyModifiers::NONE,
        )));
        assert!(!app.logs.filter_active);
    }

    #[test]
    fn test_event_loop_hot_paths_stay_non_blocking_under_load() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(50), tx);

        let mut worst_key = std::time::Duration::ZERO;
        for _ in 0..500 {
            let start = Instant::now();
            app.handle_event(TuiEvent::Key(KeyEvent::new(
                KeyCode::Char('j'),
                KeyModifiers::NONE,
            )));
            let elapsed = start.elapsed();
            if elapsed > worst_key {
                worst_key = elapsed;
            }
        }

        let mut worst_update = std::time::Duration::ZERO;
        for i in 0..200 {
            let mut updated = app.run.clone();
            let idx = i % updated.work_packages.len();
            updated.work_packages[idx].state = if i % 2 == 0 {
                WPState::ForReview
            } else {
                WPState::Active
            };

            let start = Instant::now();
            app.update_state(updated);
            let elapsed = start.elapsed();
            if elapsed > worst_update {
                worst_update = elapsed;
            }
        }

        assert!(
            worst_key <= std::time::Duration::from_millis(25),
            "key handling exceeded 25ms: {:?}",
            worst_key
        );
        assert!(
            worst_update <= std::time::Duration::from_millis(25),
            "state update exceeded 25ms: {:?}",
            worst_update
        );
    }

    /// Helper to create a run with WPs in various states for dashboard tests.
    fn create_mixed_state_run() -> OrchestrationRun {
        let states = [
            ("WP01", "Setup CI", WPState::Pending),
            ("WP02", "Build pipeline", WPState::Active),
            ("WP03", "Write tests", WPState::Paused),
            ("WP04", "Code review", WPState::ForReview),
            ("WP05", "Deploy staging", WPState::Completed),
            ("WP06", "Fix flaky test", WPState::Failed),
        ];

        let work_packages: Vec<WorkPackage> = states
            .iter()
            .map(|(id, title, state)| WorkPackage {
                id: id.to_string(),
                title: title.to_string(),
                state: *state,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: id.to_lowercase(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            })
            .collect();

        OrchestrationRun {
            id: "run-mixed".to_string(),
            feature: "feature".to_string(),
            feature_dir: std::path::PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages,
            waves: vec![Wave {
                index: 0,
                wp_ids: vec![
                    "WP01".into(),
                    "WP02".into(),
                    "WP03".into(),
                    "WP04".into(),
                    "WP05".into(),
                    "WP06".into(),
                ],
                state: WaveState::Active,
            }],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[test]
    fn test_dashboard_renders_wps_in_correct_lanes() {
        let (tx, _rx) = mpsc::channel(4);
        let app = App::new(create_mixed_state_run(), tx);

        // Verify lane partitioning via wps_in_lane.
        let planned = app.wps_in_lane(0);
        assert_eq!(planned.len(), 2); // WP01 (Pending) + WP03 (Paused)
        assert!(planned.iter().any(|wp| wp.id == "WP01"));
        assert!(planned.iter().any(|wp| wp.id == "WP03"));

        let doing = app.wps_in_lane(1);
        assert_eq!(doing.len(), 1); // WP02 (Active)
        assert_eq!(doing[0].id, "WP02");

        let review = app.wps_in_lane(2);
        assert_eq!(review.len(), 1); // WP04 (ForReview)
        assert_eq!(review[0].id, "WP04");

        let done = app.wps_in_lane(3);
        assert_eq!(done.len(), 2); // WP05 (Completed) + WP06 (Failed)
        assert!(done.iter().any(|wp| wp.id == "WP05"));
        assert!(done.iter().any(|wp| wp.id == "WP06"));

        // Render to a TestBackend and verify no panic + WP IDs appear in output.
        let backend = TestBackend::new(120, 30);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("dashboard render");

        let buf = terminal.backend().buffer().clone();
        let mut rendered = String::new();
        for y in 0..buf.area.height {
            for x in 0..buf.area.width {
                rendered.push_str(buf[(x, y)].symbol());
            }
        }

        // Verify WP IDs appear in the rendered buffer.
        assert!(
            rendered.contains("WP01"),
            "WP01 should appear in rendered output"
        );
        assert!(
            rendered.contains("WP02"),
            "WP02 should appear in rendered output"
        );
        assert!(
            rendered.contains("WP04"),
            "WP04 should appear in rendered output"
        );
        assert!(
            rendered.contains("WP05"),
            "WP05 should appear in rendered output"
        );
        assert!(
            rendered.contains("WP06"),
            "WP06 should appear in rendered output"
        );

        // Verify lane headers appear.
        assert!(
            rendered.contains("Planned"),
            "Planned lane header should appear"
        );
        assert!(
            rendered.contains("Doing"),
            "Doing lane header should appear"
        );
        assert!(
            rendered.contains("Review"),
            "Review lane header should appear"
        );
        assert!(rendered.contains("Done"), "Done lane header should appear");
    }

    #[test]
    fn test_dashboard_navigation_bounds() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_mixed_state_run(), tx);

        // Start at lane 0, index 0.
        assert_eq!(app.dashboard.focused_lane, 0);
        assert_eq!(app.dashboard.selected_index, 0);

        // j moves down within lane 0 (has 2 items: WP01, WP03).
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('j'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.selected_index, 1);

        // j again should clamp at 1 (lane 0 has 2 items, max index = 1).
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('j'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.selected_index, 1);

        // k moves up.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('k'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.selected_index, 0);

        // k again should clamp at 0 (saturating_sub).
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('k'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.selected_index, 0);

        // l moves to lane 1, resets selected_index.
        app.dashboard.selected_index = 1; // set to non-zero first
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('l'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 1);
        assert_eq!(app.dashboard.selected_index, 0);

        // l twice more to reach lane 3.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('l'),
            KeyModifiers::NONE,
        )));
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('l'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 3);

        // l again should clamp at 3.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('l'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 3);

        // h moves back to lane 2.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('h'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 2);
        assert_eq!(app.dashboard.selected_index, 0);

        // h three times to reach lane 0, then one more to verify clamping.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('h'),
            KeyModifiers::NONE,
        )));
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('h'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 0);
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('h'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 0);
    }

    #[test]
    fn test_action_keys_dispatch_correct_engine_action() {
        let (tx, mut rx) = mpsc::channel(16);
        let mut app = App::new(create_mixed_state_run(), tx);

        // Navigate to lane 3 (Done) which has WP05 (Completed) and WP06 (Failed).
        app.dashboard.focused_lane = 3;
        app.dashboard.selected_index = 0;

        // WP05 is Completed — T (Retry) should NOT dispatch (not valid for Completed).
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('T'),
            KeyModifiers::NONE,
        )));
        assert!(
            rx.try_recv().is_err(),
            "Retry should not dispatch for Completed WP"
        );

        // Select WP06 (Failed) — index 1 in the Done lane.
        app.dashboard.selected_index = 1;

        // T (Retry) should dispatch for Failed WP.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('T'),
            KeyModifiers::NONE,
        )));
        match rx.try_recv() {
            Ok(EngineAction::Retry(wp_id)) => assert_eq!(wp_id, "WP06"),
            other => panic!("Expected Retry(WP06), got {:?}", other),
        }

        // R (Restart) should dispatch for Failed WP.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('R'),
            KeyModifiers::NONE,
        )));
        match rx.try_recv() {
            Ok(EngineAction::Restart(wp_id)) => assert_eq!(wp_id, "WP06"),
            other => panic!("Expected Restart(WP06), got {:?}", other),
        }

        // F (ForceAdvance) should set pending_confirm (not dispatch immediately).
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('F'),
            KeyModifiers::NONE,
        )));
        assert!(
            rx.try_recv().is_err(),
            "ForceAdvance should NOT dispatch immediately — confirmation required"
        );
        assert!(
            app.pending_confirm.is_some(),
            "pending_confirm should be set after pressing F on Failed WP"
        );
        match &app.pending_confirm {
            Some(ConfirmAction::ForceAdvance { wp_id }) => assert_eq!(wp_id, "WP06"),
            other => panic!("Expected ForceAdvance(WP06) confirm, got {:?}", other),
        }

        // Confirm with 'y' — now the action dispatches.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('y'),
            KeyModifiers::NONE,
        )));
        match rx.try_recv() {
            Ok(EngineAction::ForceAdvance(wp_id)) => assert_eq!(wp_id, "WP06"),
            other => panic!("Expected ForceAdvance(WP06), got {:?}", other),
        }
        assert!(
            app.pending_confirm.is_none(),
            "pending_confirm should be cleared after confirmation"
        );

        // Navigate to lane 1 (Doing) — WP02 is Active.
        app.dashboard.focused_lane = 1;
        app.dashboard.selected_index = 0;

        // P (Pause) should dispatch for Active WP.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('P'),
            KeyModifiers::NONE,
        )));
        match rx.try_recv() {
            Ok(EngineAction::Pause(wp_id)) => assert_eq!(wp_id, "WP02"),
            other => panic!("Expected Pause(WP02), got {:?}", other),
        }

        // Navigate to lane 0 (Planned) — WP03 is Paused (index 1).
        app.dashboard.focused_lane = 0;
        app.dashboard.selected_index = 1; // WP03 (Paused)

        // P (Resume) should dispatch for Paused WP.
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('P'),
            KeyModifiers::NONE,
        )));
        match rx.try_recv() {
            Ok(EngineAction::Resume(wp_id)) => assert_eq!(wp_id, "WP03"),
            other => panic!("Expected Resume(WP03), got {:?}", other),
        }
    }

    #[test]
    fn test_review_tab_renders_for_review_wps() {
        let (tx, _rx) = mpsc::channel(4);
        let mut run = create_test_run(5);
        run.work_packages[0].state = WPState::ForReview;
        run.work_packages[2].state = WPState::ForReview;
        let mut app = App::new(run, tx);
        app.active_tab = Tab::Review;

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("render review tab with ForReview WPs");
    }

    #[test]
    fn test_review_tab_renders_empty_state() {
        let (tx, _rx) = mpsc::channel(4);
        let run = create_test_run(3); // all Active, none ForReview
        let mut app = App::new(run, tx);
        app.active_tab = Tab::Review;

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("render review tab empty state");
    }

    #[test]
    fn test_notification_diffing_adds_and_removes() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(2), tx);

        // WP01 transitions Active → ForReview → should add ReviewPending notification
        let mut to_review = app.run.clone();
        to_review.work_packages[0].state = WPState::ForReview;
        app.update_state(to_review);

        assert_eq!(app.notifications.len(), 1);
        assert_eq!(app.notifications[0].kind, NotificationKind::ReviewPending);
        assert_eq!(app.notifications[0].wp_id, "WP01");

        // WP01 transitions ForReview → Completed → should remove ReviewPending notification
        let mut to_completed = app.run.clone();
        to_completed.work_packages[0].state = WPState::Completed;
        app.update_state(to_completed);

        assert!(
            app.notifications.is_empty(),
            "ReviewPending notification should be auto-dismissed"
        );
    }

    #[test]
    fn test_notification_diffing_failure_add_and_remove() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(1), tx);

        // Active → Failed → should add Failure notification
        let mut to_failed = app.run.clone();
        to_failed.work_packages[0].state = WPState::Failed;
        app.update_state(to_failed);

        assert_eq!(app.notifications.len(), 1);
        assert_eq!(app.notifications[0].kind, NotificationKind::Failure);
        assert_eq!(app.notifications[0].wp_id, "WP01");

        // Failed → Active (retry) → should remove Failure notification
        let mut to_active = app.run.clone();
        to_active.work_packages[0].state = WPState::Active;
        app.update_state(to_active);

        assert!(
            app.notifications.is_empty(),
            "Failure notification should be auto-dismissed on retry"
        );
    }

    #[test]
    fn test_approve_key_sends_action() {
        let (tx, mut rx) = mpsc::channel(4);
        let mut run = create_test_run(2);
        run.work_packages[0].state = WPState::ForReview;
        let mut app = App::new(run, tx);
        app.active_tab = Tab::Review;
        app.review.selected_index = 0;

        keybindings::handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('a'), KeyModifiers::NONE),
        );

        match rx.try_recv() {
            Ok(EngineAction::Approve(wp_id)) => assert_eq!(wp_id, "WP01"),
            other => panic!("Expected Approve(WP01), got {:?}", other),
        }
    }

    #[test]
    fn test_reject_key_sends_action() {
        let (tx, mut rx) = mpsc::channel(4);
        let mut run = create_test_run(2);
        run.work_packages[0].state = WPState::ForReview;
        let mut app = App::new(run, tx);
        app.active_tab = Tab::Review;
        app.review.selected_index = 0;

        keybindings::handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('r'), KeyModifiers::NONE),
        );

        match rx.try_recv() {
            Ok(EngineAction::Reject { wp_id, relaunch }) => {
                assert_eq!(wp_id, "WP01");
                assert!(relaunch);
            }
            other => panic!(
                "Expected Reject {{ wp_id: WP01, relaunch: true }}, got {:?}",
                other
            ),
        }
    }

    #[test]
    fn test_notification_jump_switches_tab() {
        let (tx, _rx) = mpsc::channel(4);
        let mut run = create_test_run(3);
        run.work_packages[1].state = WPState::ForReview;
        let mut app = App::new(run, tx);

        // Manually add a ReviewPending notification
        let notif_id = app.next_notification_id();
        app.notifications.push(Notification {
            id: notif_id,
            kind: NotificationKind::ReviewPending,
            wp_id: "WP02".to_string(),
            message: None,
            failure_type: None,
            severity: None,
            created_at: Instant::now(),
        });

        // Start on Dashboard
        app.active_tab = Tab::Dashboard;

        // Press 'n' to jump
        keybindings::handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('n'), KeyModifiers::NONE),
        );

        assert_eq!(app.active_tab, Tab::Review);
        // WP02 is the only ForReview WP, at index 0
        assert_eq!(app.review.selected_index, 0);
    }

    #[test]
    fn test_notification_bar_renders_with_notifications() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);

        // Trigger notification diffing: WP02 → Failed
        let mut new_run = app.run.clone();
        new_run.work_packages[1].state = WPState::Failed;
        app.update_state(new_run);

        assert!(!app.notifications.is_empty());

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("render with notification bar");
    }
}
