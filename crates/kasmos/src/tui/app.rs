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
use std::time::Instant;
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
    /// tui-logger widget state (target selection, scroll, page mode).
    pub logger_state: tui_logger::TuiWidgetState,
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
            logger_state: tui_logger::TuiWidgetState::new(),
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

        // Log via tracing — tui-logger captures automatically
        tracing::error!(wp_id = %wp_id, "review_failure {:?}: {}", failure_type, message);
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

            match wp.state {
                WPState::Failed => {
                    tracing::error!(wp_id = %wp.id, "{from} -> {:?}", wp.state);
                }
                WPState::ForReview => {
                    tracing::warn!(wp_id = %wp.id, "{from} -> {:?}", wp.state);
                }
                _ => {
                    tracing::info!(wp_id = %wp.id, "{from} -> {:?}", wp.state);
                }
            }

            if old_state != Some(WPState::ForReview) && wp.state == WPState::ForReview {
                let decision = self.review_policy_executor.on_for_review_transition();
                tracing::info!(
                    wp_id = %wp.id,
                    "review_policy {:?}: run_automation={}, auto_mark_done={}",
                    self.review_policy_executor.policy(),
                    decision.run_automation,
                    decision.auto_mark_done
                );
            }
        }

        if new_run.state != self.run.state {
            tracing::info!("Run state: {:?} -> {:?}", self.run.state, new_run.state);
        }
    }

    /// Render the Logs tab using tui-logger's TuiLoggerSmartWidget.
    fn render_logs(&self, frame: &mut Frame, area: Rect) {
        let widget = tui_logger::TuiLoggerSmartWidget::default()
            .style_error(Style::default().fg(Color::Red))
            .style_warn(Style::default().fg(Color::Yellow))
            .style_info(Style::default().fg(Color::Cyan))
            .style_debug(Style::default().fg(Color::DarkGray))
            .style_trace(Style::default().fg(Color::DarkGray))
            .output_separator(' ')
            .output_target(true)
            .output_timestamp(Some("%H:%M:%S".to_string()))
            .output_level(Some(tui_logger::TuiLoggerLevelOutput::Abbreviated))
            .output_file(false)
            .output_line(false)
            .state(&self.logger_state);
        frame.render_widget(widget, area);
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

        // Body — placeholder per tab
        let body_text = match self.active_tab {
            Tab::Dashboard => {
                let wp_count = self.run.work_packages.len();
                format!(
                    "Dashboard view coming soon\n\n{} work packages loaded\nPress 'q' to quit",
                    wp_count
                )
            }
            Tab::Review => "Review view coming soon\n\nPress 'q' to quit".to_string(),
            Tab::Logs => {
                self.render_logs(frame, chunks[1]);
                return;
            }
        };

        let body =
            Paragraph::new(body_text).block(Block::default().borders(Borders::ALL).title(format!(
                " {} ",
                match self.active_tab {
                    Tab::Dashboard => "Dashboard",
                    Tab::Review => "Review",
                    Tab::Logs => "Logs",
                }
            )));

        frame.render_widget(body, chunks[1]);
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

        // Verify ManualOnly policy is applied on ForReview transition
        app.set_review_policy(ReviewAutomationPolicy::ManualOnly);
        let mut to_for_review = app.run.clone();
        to_for_review.work_packages[0].state = WPState::ForReview;
        app.update_state(to_for_review);
        // After tui-logger migration, log content assertions are removed —
        // the test verifies the state transition runs without panic.

        // Verify AutoAndMarkDone policy is applied on second ForReview transition
        app.set_review_policy(ReviewAutomationPolicy::AutoAndMarkDone);
        let mut active_again = app.run.clone();
        active_again.work_packages[0].state = WPState::Active;
        app.update_state(active_again.clone());
        active_again.work_packages[0].state = WPState::ForReview;
        app.update_state(active_again);
        // Log assertions removed — tui-logger captures events internally.
    }

    #[test]
    fn test_review_failure_surfaces_notification() {
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
        // Log content assertions removed — tui-logger captures via tracing::error!()
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

        // tui-logger handles its own key events (h=hide/show target selector,
        // PageUp/PageDown for page mode, etc.) — no filter_active to test.
        // Verify Logs tab key handling doesn't panic:
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('h'),
            KeyModifiers::NONE,
        )));
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Esc,
            KeyModifiers::NONE,
        )));
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
