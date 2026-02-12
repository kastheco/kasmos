//! TUI application state.
//!
//! Contains the root `App` struct and supporting types that hold all UI state:
//! active tab, per-tab scroll/selection state, notifications, and the latest
//! orchestration run snapshot from the engine.

use crate::command_handlers::EngineAction;
use crate::review::{
    ReviewAutomationPolicy, ReviewFailureSeverity, ReviewFailureType, ReviewPolicyExecutor,
};
use crate::types::{OrchestrationRun, WPState};
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Paragraph, Tabs};
use ratatui::Frame;
use std::collections::HashMap;
use std::time::{Instant, SystemTime, UNIX_EPOCH};
use tokio::sync::mpsc;

use super::event::TuiEvent;
use super::keybindings;

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
// Confirmation dialog
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
// Per-tab state structs
// ---------------------------------------------------------------------------

/// UI state for the Dashboard tab.
#[derive(Debug)]
pub struct DashboardState {
    /// Which lane column is focused (0=planned, 1=doing, 2=for_review, 3=done).
    pub focused_lane: usize,
    /// Selected WP index within the focused lane.
    pub selected_index: usize,
    /// Vertical scroll offset per lane.
    pub scroll_offsets: [usize; 4],
}

impl Default for DashboardState {
    fn default() -> Self {
        Self {
            focused_lane: 0,
            selected_index: 0,
            scroll_offsets: [0; 4],
        }
    }
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
    /// Called when the watch channel signals a new state. Notification diffing
    /// will be added in WP05.
    pub fn update_state(&mut self, new_run: OrchestrationRun) {
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
    #[allow(dead_code)]
    pub fn next_notification_id(&mut self) -> u64 {
        self.notification_counter += 1;
        self.notification_counter
    }

    /// Render the entire TUI frame.
    ///
    /// Layout:
    /// - Tab header bar at top
    /// - Body: tab-specific content (placeholder for now)
    pub fn render(&self, frame: &mut Frame) {
        let area = frame.area();

        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3), // tab bar
                Constraint::Min(0),    // body
            ])
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

        // Body — per-tab content
        match self.active_tab {
            Tab::Logs => {
                self.render_logs(frame, chunks[1]);
            }
            _ => {
                let body_text = match self.active_tab {
                    Tab::Dashboard => {
                        let wp_count = self.run.work_packages.len();
                        format!(
                            "Dashboard view coming soon\n\n{} work packages loaded\nPress 'q' to quit",
                            wp_count
                        )
                    }
                    Tab::Review => "Review view coming soon\n\nPress 'q' to quit".to_string(),
                    Tab::Logs => unreachable!(),
                };

                let body = Paragraph::new(body_text).block(
                    Block::default().borders(Borders::ALL).title(format!(
                        " {} ",
                        match self.active_tab {
                            Tab::Dashboard => "Dashboard",
                            Tab::Review => "Review",
                            Tab::Logs => "Logs",
                        }
                    )),
                );

                frame.render_widget(body, chunks[1]);
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
}
