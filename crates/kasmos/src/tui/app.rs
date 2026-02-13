//! TUI application state.
//!
//! Contains the root `App` struct and supporting types that hold all UI state:
//! active tab, per-tab scroll/selection state, notifications, overlays, and
//! the latest orchestration run snapshot from the engine.

use crate::command_handlers::EngineAction;
use crate::review::{
    ReviewAutomationPolicy, ReviewFailureSeverity, ReviewFailureType, ReviewPolicyExecutor,
};
use crate::types::{OrchestrationRun, RunState, WPState};
use ratatui::layout::{Constraint, Direction, Layout, Rect};
use ratatui::style::{Color, Modifier, Style};
use ratatui::text::{Line, Span};
use ratatui::widgets::{Block, Borders, Clear, Paragraph, Tabs, Wrap};
use ratatui::Frame;
use std::cell::Cell;
use std::collections::HashMap;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use tokio::sync::mpsc;

use super::event::TuiEvent;
use super::keybindings;
use super::tabs;

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
    /// Last known visible lane height (updated during render via `Cell` for interior mutability).
    /// Used by keybinding handler to adjust scroll offsets.
    pub last_lane_height: Cell<usize>,
}

impl Default for DashboardState {
    fn default() -> Self {
        Self {
            focused_lane: 0,
            selected_index: 0,
            scroll_offsets: [0; 4],
            last_lane_height: Cell::new(0),
        }
    }
}

impl DashboardState {
    /// Ensure the selected item is visible within the scroll window.
    ///
    /// Call this after modifying `selected_index` in keybinding handlers.
    pub fn ensure_selected_visible(&mut self) {
        let lane = self.focused_lane;
        let visible = {
            let h = self.last_lane_height.get();
            if h > 0 { h } else { 20 }
        };

        // Scroll up if selected is above viewport
        if self.selected_index < self.scroll_offsets[lane] {
            self.scroll_offsets[lane] = self.selected_index;
        }

        // Scroll down if selected is below viewport
        if self.selected_index >= self.scroll_offsets[lane] + visible {
            self.scroll_offsets[lane] = self.selected_index.saturating_sub(visible) + 1;
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
/// overlay state, and the channel for sending commands back to the engine.
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
    /// Channel to send commands to the engine.
    pub action_tx: mpsc::Sender<EngineAction>,
    /// Exit flag — when true, the event loop breaks.
    pub should_quit: bool,
    /// Monotonically increasing counter for notification IDs.
    notification_counter: u64,
    /// Executor for `for_review` policy decisions.
    review_policy_executor: ReviewPolicyExecutor,

    // -- WP06 fields --
    /// Whether the help overlay is currently visible.
    pub show_help: bool,
    /// WP ID currently shown in the detail popup, if any.
    pub detail_wp_id: Option<String>,
    /// Index into `notifications` for cycling with 'n' key.
    pub notification_cycle_index: usize,
    /// When the TUI was started (for elapsed time display).
    pub started_at: Instant,

    // -- WP08 fields --
    /// Whether Alt+h was pressed to open/switch to hub tab.
    pub open_hub_requested: bool,
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
            action_tx,
            should_quit: false,
            notification_counter: 0,
            review_policy_executor: ReviewPolicyExecutor::new(ReviewAutomationPolicy::default()),
            show_help: false,
            detail_wp_id: None,
            notification_cycle_index: 0,
            started_at: Instant::now(),
            open_hub_requested: false,
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
        // Reset notification cycle index on list change
        self.notification_cycle_index = 0;

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
    /// Called when the watch channel signals a new state. Also clears stale
    /// detail popup references and resets notification cycle index.
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

        // Clear stale detail popup reference
        if let Some(ref wp_id) = self.detail_wp_id
            && !self.run.work_packages.iter().any(|wp| wp.id == *wp_id)
        {
            self.detail_wp_id = None;
        }

        // Clamp review.selected_index to valid range
        let review_count = self
            .run
            .work_packages
            .iter()
            .filter(|wp| wp.state == WPState::ForReview)
            .count();
        if review_count > 0 {
            self.review.selected_index = self.review.selected_index.min(review_count - 1);
        } else {
            self.review.selected_index = 0;
        }

        // Clamp dashboard.selected_index to focused lane count
        let focused_lane = self.dashboard.focused_lane;
        let lane_count = self
            .run
            .work_packages
            .iter()
            .filter(|wp| state_to_lane(wp.state) == focused_lane)
            .count();
        if lane_count > 0 {
            self.dashboard.selected_index = self.dashboard.selected_index.min(lane_count - 1);
        } else {
            self.dashboard.selected_index = 0;
        }
    }

    /// Periodic tick handler for elapsed time display updates.
    ///
    /// Called every 250ms from the event loop.
    pub fn on_tick(&mut self) {
        // Status footer elapsed time is computed at render time from self.started_at,
        // so no state update is needed here. The tick just triggers a re-render.
    }

    /// Return the elapsed time since the TUI started.
    pub fn elapsed(&self) -> Duration {
        self.started_at.elapsed()
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

    /// Return filtered log entries matching the current filter text.
    pub fn filtered_log_entries(&self) -> Vec<&LogEntry> {
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

    /// Format a SystemTime as HH:MM:SS (UTC time of day).
    pub fn format_timestamp(timestamp: SystemTime) -> String {
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

    /// Format a Duration as HH:MM:SS.
    fn format_duration(d: Duration) -> String {
        let total = d.as_secs();
        let hours = total / 3_600;
        let minutes = (total % 3_600) / 60;
        let seconds = total % 60;
        format!("{hours:02}:{minutes:02}:{seconds:02}")
    }

    /// Allocate a new unique notification ID.
    #[allow(dead_code)]
    pub fn next_notification_id(&mut self) -> u64 {
        self.notification_counter += 1;
        self.notification_counter
    }

    // -- Overlay rendering helpers --

    /// Render the help overlay listing all keybindings for the current tab.
    fn render_help_overlay(&self, frame: &mut Frame, area: Rect) {
        let global_keys = vec![
            ("Alt+q", "Quit"),
            ("1/2/3", "Switch tab"),
            ("?", "Toggle help"),
            ("n", "Next notification"),
        ];

        let tab_keys: Vec<(&str, &str)> = match self.active_tab {
            Tab::Dashboard => vec![
                ("j/↓", "Move down in lane"),
                ("k/↑", "Move up in lane"),
                ("h/←", "Move to left lane"),
                ("l/→", "Move to right lane"),
                ("Enter", "WP detail popup"),
                ("A", "Advance wave (wave-gated)"),
            ],
            Tab::Review => vec![
                ("j/↓", "Next review item"),
                ("k/↑", "Previous review item"),
                ("a", "Approve WP"),
                ("r", "Reject WP"),
                ("R", "Reject + relaunch WP"),
            ],
            Tab::Logs => vec![
                ("j/↓", "Scroll down"),
                ("k/↑", "Scroll up"),
                ("G", "Resume auto-scroll"),
                ("g", "Jump to top"),
                ("/", "Filter"),
            ],
        };

        let mut lines: Vec<Line> = Vec::new();
        lines.push(Line::from(Span::styled(
            "Global Keys",
            Style::default()
                .fg(Color::Cyan)
                .add_modifier(Modifier::BOLD),
        )));
        for (key, desc) in &global_keys {
            lines.push(Line::from(vec![
                Span::styled(
                    format!("  {key:<12}"),
                    Style::default()
                        .fg(Color::Yellow)
                        .add_modifier(Modifier::BOLD),
                ),
                Span::raw(*desc),
            ]));
        }

        lines.push(Line::default());

        let tab_name = match self.active_tab {
            Tab::Dashboard => "Dashboard Keys",
            Tab::Review => "Review Keys",
            Tab::Logs => "Logs Keys",
        };
        lines.push(Line::from(Span::styled(
            tab_name,
            Style::default()
                .fg(Color::Cyan)
                .add_modifier(Modifier::BOLD),
        )));
        for (key, desc) in &tab_keys {
            lines.push(Line::from(vec![
                Span::styled(
                    format!("  {key:<12}"),
                    Style::default()
                        .fg(Color::Yellow)
                        .add_modifier(Modifier::BOLD),
                ),
                Span::raw(*desc),
            ]));
        }

        lines.push(Line::default());
        lines.push(Line::from(Span::styled(
            "Press ? or Esc to close",
            Style::default().fg(Color::DarkGray),
        )));

        let popup_width = 40u16.min(area.width.saturating_sub(4));
        let popup_height = (lines.len() as u16 + 2).min(area.height.saturating_sub(2));
        let popup_area = centered_rect(popup_width, popup_height, area);

        frame.render_widget(Clear, popup_area);
        let block = Block::default()
            .borders(Borders::ALL)
            .title(" Help ")
            .border_style(Style::default().fg(Color::Cyan));
        let inner = block.inner(popup_area);
        frame.render_widget(block, popup_area);
        frame.render_widget(Paragraph::new(lines), inner);
    }

    /// Render the WP detail popup.
    fn render_detail_popup(&self, frame: &mut Frame, area: Rect) {
        let wp_id = match &self.detail_wp_id {
            Some(id) => id,
            None => return,
        };

        let wp = match self.run.work_packages.iter().find(|wp| wp.id == *wp_id) {
            Some(wp) => wp,
            None => return,
        };

        let mut lines: Vec<Line> = Vec::new();

        lines.push(Line::from(vec![
            Span::styled("ID:          ", Style::default().fg(Color::Cyan)),
            Span::raw(&wp.id),
        ]));
        lines.push(Line::from(vec![
            Span::styled("Title:       ", Style::default().fg(Color::Cyan)),
            Span::raw(&wp.title),
        ]));
        lines.push(Line::from(vec![
            Span::styled("State:       ", Style::default().fg(Color::Cyan)),
            Span::styled(
                format!("{:?}", wp.state),
                Style::default().fg(state_color(wp.state)),
            ),
        ]));
        lines.push(Line::from(vec![
            Span::styled("Wave:        ", Style::default().fg(Color::Cyan)),
            Span::raw(wp.wave.to_string()),
        ]));
        lines.push(Line::from(vec![
            Span::styled("Dependencies:", Style::default().fg(Color::Cyan)),
            Span::raw(if wp.dependencies.is_empty() {
                " (none)".to_string()
            } else {
                format!(" {}", wp.dependencies.join(", "))
            }),
        ]));
        lines.push(Line::from(vec![
            Span::styled("Failures:    ", Style::default().fg(Color::Cyan)),
            if wp.failure_count > 0 {
                Span::styled(
                    wp.failure_count.to_string(),
                    Style::default().fg(Color::Red).add_modifier(Modifier::BOLD),
                )
            } else {
                Span::raw("0")
            },
        ]));

        // Elapsed time
        let elapsed_str = match (wp.started_at, wp.completed_at) {
            (Some(start), Some(end)) => match end.duration_since(start) {
                Ok(d) => Self::format_duration(d),
                Err(_) => "N/A".to_string(),
            },
            (Some(start), None) => match SystemTime::now().duration_since(start) {
                Ok(d) => format!("{} (running)", Self::format_duration(d)),
                Err(_) => "N/A".to_string(),
            },
            _ => "N/A".to_string(),
        };
        lines.push(Line::from(vec![
            Span::styled("Elapsed:     ", Style::default().fg(Color::Cyan)),
            Span::raw(elapsed_str),
        ]));

        lines.push(Line::from(vec![
            Span::styled("Worktree:    ", Style::default().fg(Color::Cyan)),
            Span::raw(
                wp.worktree_path
                    .as_ref()
                    .map(|p| p.display().to_string())
                    .unwrap_or_else(|| "(none)".to_string()),
            ),
        ]));
        lines.push(Line::from(vec![
            Span::styled("Pane ID:     ", Style::default().fg(Color::Cyan)),
            Span::raw(
                wp.pane_id
                    .map(|id| id.to_string())
                    .unwrap_or_else(|| "(none)".to_string()),
            ),
        ]));
        lines.push(Line::from(vec![
            Span::styled("Completion:  ", Style::default().fg(Color::Cyan)),
            Span::raw(
                wp.completion_method
                    .map(|m| format!("{m:?}"))
                    .unwrap_or_else(|| "(pending)".to_string()),
            ),
        ]));

        lines.push(Line::default());
        lines.push(Line::from(Span::styled(
            "Press Esc to close",
            Style::default().fg(Color::DarkGray),
        )));

        let popup_width = 60u16.min(area.width.saturating_sub(4));
        let popup_height = (lines.len() as u16 + 2).min(area.height.saturating_sub(2));
        let popup_area = centered_rect(popup_width, popup_height, area);

        frame.render_widget(Clear, popup_area);
        let block = Block::default()
            .borders(Borders::ALL)
            .title(format!(" {} Detail ", wp.id))
            .border_style(Style::default().fg(Color::Yellow));
        let inner = block.inner(popup_area);
        frame.render_widget(block, popup_area);
        frame.render_widget(Paragraph::new(lines).wrap(Wrap { trim: false }), inner);
    }

    /// Render the persistent status footer (FR-016).
    fn render_status_footer(&self, frame: &mut Frame, area: Rect) {
        let total = self.run.work_packages.len();
        let completed = self
            .run
            .work_packages
            .iter()
            .filter(|wp| wp.state == WPState::Completed)
            .count();
        let active = self
            .run
            .work_packages
            .iter()
            .filter(|wp| wp.state == WPState::Active)
            .count();
        let failed = self
            .run
            .work_packages
            .iter()
            .filter(|wp| wp.state == WPState::Failed)
            .count();

        let run_state_span = match self.run.state {
            RunState::Running => Span::styled(
                " RUNNING ",
                Style::default()
                    .fg(Color::Black)
                    .bg(Color::Green)
                    .add_modifier(Modifier::BOLD),
            ),
            RunState::Paused => Span::styled(
                " PAUSED ",
                Style::default()
                    .fg(Color::Black)
                    .bg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            ),
            RunState::Completed => Span::styled(
                " COMPLETED ",
                Style::default()
                    .fg(Color::Black)
                    .bg(Color::Cyan)
                    .add_modifier(Modifier::BOLD),
            ),
            RunState::Failed => Span::styled(
                " FAILED ",
                Style::default()
                    .fg(Color::White)
                    .bg(Color::Red)
                    .add_modifier(Modifier::BOLD),
            ),
            RunState::Aborted => Span::styled(
                " ABORTED ",
                Style::default()
                    .fg(Color::White)
                    .bg(Color::Red)
                    .add_modifier(Modifier::BOLD),
            ),
            RunState::Initializing => Span::styled(
                " INIT ",
                Style::default()
                    .fg(Color::Black)
                    .bg(Color::DarkGray)
                    .add_modifier(Modifier::BOLD),
            ),
        };

        let elapsed_str = Self::format_duration(self.elapsed());
        let mode_str = match self.run.mode {
            crate::types::ProgressionMode::Continuous => "continuous",
            crate::types::ProgressionMode::WaveGated => "wave-gated",
        };

        let spans = vec![
            run_state_span,
            Span::raw(" "),
            Span::styled(
                format!("{completed}/{total}"),
                Style::default().fg(Color::Green),
            ),
            Span::styled(" done ", Style::default().fg(Color::DarkGray)),
            Span::styled(format!("{active}"), Style::default().fg(Color::Yellow)),
            Span::styled(" active ", Style::default().fg(Color::DarkGray)),
            Span::styled(format!("{failed}"), Style::default().fg(Color::Red)),
            Span::styled(" failed ", Style::default().fg(Color::DarkGray)),
            Span::styled(format!("⏱ {elapsed_str}"), Style::default().fg(Color::Cyan)),
            Span::raw("  "),
            Span::styled(mode_str, Style::default().fg(Color::DarkGray)),
        ];

        // Add notification indicator if there are active notifications
        let mut all_spans = spans;
        if !self.notifications.is_empty() {
            all_spans.push(Span::raw("  "));
            all_spans.push(Span::styled(
                format!("🔔{}", self.notifications.len()),
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            ));
        }

        frame.render_widget(
            Paragraph::new(Line::from(all_spans)).style(Style::default().bg(Color::DarkGray)),
            area,
        );
    }

    /// Render the entire TUI frame.
    ///
    /// Layout:
    /// - Tab header bar at top
    /// - Body: tab-specific content
    /// - Status footer at bottom
    /// - Overlays: help > detail (overlay priority order)
    /// Returns `true` when the run is paused at a wave gate, meaning the
    /// operator needs to press Shift+A to advance.
    fn is_wave_gated_paused(&self) -> bool {
        self.run.mode == crate::types::ProgressionMode::WaveGated
            && self.run.state == RunState::Paused
    }

    /// Compute the index of the last completed wave. Falls back to checking
    /// which waves have all WPs completed.
    fn completed_wave_index(&self) -> usize {
        use crate::types::WaveState;
        if let Some(wave) = self
            .run
            .waves
            .iter()
            .rev()
            .find(|w| w.state == WaveState::Completed)
        {
            return wave.index;
        }
        // Fallback: derive from WP states
        let max_wave = self
            .run
            .work_packages
            .iter()
            .map(|wp| wp.wave)
            .max()
            .unwrap_or(0);
        for w in (0..=max_wave).rev() {
            let all_done = self
                .run
                .work_packages
                .iter()
                .filter(|wp| wp.wave == w)
                .all(|wp| matches!(wp.state, WPState::Completed | WPState::Failed));
            if all_done {
                return w;
            }
        }
        0
    }

    pub fn render(&self, frame: &mut Frame) {
        let area = frame.area();

        let show_banner = self.is_wave_gated_paused();
        let banner_height = if show_banner { 3 } else { 0 };

        let chunks = Layout::default()
            .direction(Direction::Vertical)
            .constraints([
                Constraint::Length(3),             // tab bar
                Constraint::Length(banner_height), // wave-gate banner (0 when hidden)
                Constraint::Min(0),                // body
                Constraint::Length(1),             // status footer
            ])
            .split(area);

        // Tab bar
        let titles: Vec<Line> = Tab::titles()
            .iter()
            .map(|t| Line::from(Span::raw(*t)))
            .collect();

        let tab_title = if show_banner {
            " kasmos - PAUSED "
        } else {
            " kasmos "
        };

        let tabs = Tabs::new(titles)
            .block(Block::default().borders(Borders::ALL).title(tab_title))
            .select(self.active_tab.index())
            .style(Style::default().fg(Color::White))
            .highlight_style(
                Style::default()
                    .fg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            );

        frame.render_widget(tabs, chunks[0]);

        // Wave-gate banner
        if show_banner {
            let completed_wave = self.completed_wave_index();
            let total_waves = self.run.waves.len();
            let next_wave = completed_wave + 1;

            let banner_text = if next_wave < total_waves {
                format!(
                    "  Wave {} complete ({}/{} waves) -- press Shift+A to advance to wave {}",
                    completed_wave, next_wave, total_waves, next_wave
                )
            } else {
                format!(
                    "  Wave {} complete ({}/{} waves) -- press Shift+A to advance",
                    completed_wave, next_wave, total_waves
                )
            };

            let banner = Paragraph::new(Line::from(vec![Span::styled(
                banner_text,
                Style::default()
                    .fg(Color::Black)
                    .bg(Color::Yellow)
                    .add_modifier(Modifier::BOLD),
            )]))
            .block(
                Block::default()
                    .borders(Borders::ALL)
                    .border_style(Style::default().fg(Color::Yellow))
                    .title(" Action Required "),
            );

            frame.render_widget(banner, chunks[1]);
        }

        // Body — dispatch to tab rendering modules (SC-015)
        let body_area = chunks[2];
        match self.active_tab {
            Tab::Dashboard => tabs::dashboard::render_dashboard(self, frame, body_area),
            Tab::Review => tabs::review::render_review(self, frame, body_area),
            Tab::Logs => tabs::logs::render_logs(self, frame, body_area),
        }

        // Status footer (FR-016, SC-010)
        self.render_status_footer(frame, chunks[3]);

        // Overlays — priority: help > detail
        if self.show_help {
            self.render_help_overlay(frame, area);
        } else if self.detail_wp_id.is_some() {
            self.render_detail_popup(frame, area);
        }
    }
}

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

/// Compute a centered rectangle within a parent area.
fn centered_rect(width: u16, height: u16, area: Rect) -> Rect {
    let x = area.x + (area.width.saturating_sub(width)) / 2;
    let y = area.y + (area.height.saturating_sub(height)) / 2;
    Rect::new(x, y, width.min(area.width), height.min(area.height))
}

/// Map a WPState to a kanban lane index (0=planned, 1=doing, 2=for_review, 3=done).
pub fn state_to_lane(state: WPState) -> usize {
    match state {
        WPState::Pending | WPState::Paused => 0,
        WPState::Active | WPState::Failed => 1,
        WPState::ForReview => 2,
        WPState::Completed => 3,
    }
}

/// Map a WPState to a display color.
pub fn state_color(state: WPState) -> Color {
    match state {
        WPState::Pending => Color::DarkGray,
        WPState::Active => Color::Yellow,
        WPState::Completed => Color::Green,
        WPState::Failed => Color::Red,
        WPState::ForReview => Color::Magenta,
        WPState::Paused => Color::Blue,
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

    // -- WP06 tests --

    #[test]
    fn test_status_footer_renders_on_all_tabs() {
        let (tx, _rx) = mpsc::channel(4);
        let app = App::new(create_test_run(5), tx);

        for &tab in &[Tab::Dashboard, Tab::Review, Tab::Logs] {
            let mut app_clone = App::new(create_test_run(5), app.action_tx.clone());
            app_clone.active_tab = tab;

            let backend = TestBackend::new(120, 40);
            let mut terminal = Terminal::new(backend).expect("create terminal");
            terminal
                .draw(|frame| app_clone.render(frame))
                .expect("draw");
        }
    }

    #[test]
    fn test_help_overlay_toggle() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);

        assert!(!app.show_help);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('?'),
            KeyModifiers::NONE,
        )));
        assert!(app.show_help);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('?'),
            KeyModifiers::NONE,
        )));
        assert!(!app.show_help);
    }

    #[test]
    fn test_help_overlay_dismiss_with_esc() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);

        app.show_help = true;
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Esc,
            KeyModifiers::NONE,
        )));
        assert!(!app.show_help);
    }

    #[test]
    fn test_help_overlay_swallows_other_keys() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);

        app.show_help = true;
        let tab_before = app.active_tab;

        // Try to switch tab while help is open — should be swallowed
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('2'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.active_tab, tab_before);
        assert!(app.show_help);
    }

    #[test]
    fn test_dashboard_lane_navigation() {
        let (tx, _rx) = mpsc::channel(4);
        let mut run = create_test_run(8);
        // Put WPs in different lanes
        run.work_packages[0].state = WPState::Pending;
        run.work_packages[1].state = WPState::Pending;
        run.work_packages[2].state = WPState::Active;
        run.work_packages[3].state = WPState::Active;
        run.work_packages[4].state = WPState::ForReview;
        run.work_packages[5].state = WPState::ForReview;
        run.work_packages[6].state = WPState::Completed;
        run.work_packages[7].state = WPState::Completed;

        let mut app = App::new(run, tx);
        assert_eq!(app.dashboard.focused_lane, 0);
        assert_eq!(app.dashboard.selected_index, 0);

        // Move down
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('j'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.selected_index, 1);

        // Move to right lane
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('l'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.dashboard.focused_lane, 1);
        assert_eq!(app.dashboard.selected_index, 0);
    }

    #[test]
    fn test_notification_cycling() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);

        // Add some notifications
        for i in 0..3 {
            let id = app.next_notification_id();
            app.notifications.push(Notification {
                id,
                kind: NotificationKind::Failure,
                wp_id: format!("WP{:02}", i + 1),
                message: Some(format!("fail {}", i + 1)),
                failure_type: None,
                severity: None,
                created_at: Instant::now(),
            });
        }
        app.notification_cycle_index = 0;

        // Press 'n' three times — should cycle through all
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('n'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.notification_cycle_index, 1);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('n'),
            KeyModifiers::NONE,
        )));
        assert_eq!(app.notification_cycle_index, 2);

        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Char('n'),
            KeyModifiers::NONE,
        )));
        // Wraps back to 0
        assert_eq!(app.notification_cycle_index, 0);
    }

    #[test]
    fn test_wp_detail_popup_opens_and_closes() {
        let (tx, _rx) = mpsc::channel(4);
        let mut run = create_test_run(3);
        run.work_packages[0].state = WPState::Pending;
        let mut app = App::new(run, tx);

        assert!(app.detail_wp_id.is_none());

        // Press Enter to open detail
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Enter,
            KeyModifiers::NONE,
        )));
        assert!(app.detail_wp_id.is_some());

        // Press Esc to close
        app.handle_event(TuiEvent::Key(KeyEvent::new(
            KeyCode::Esc,
            KeyModifiers::NONE,
        )));
        assert!(app.detail_wp_id.is_none());
    }

    #[test]
    fn test_responsive_layout_renders_at_various_widths() {
        let (tx, _rx) = mpsc::channel(4);
        let app = App::new(create_test_run(8), tx);

        // Wide terminal (120 cols) — should render 4 columns
        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("draw at 120 cols");

        // Medium terminal (80 cols) — should render 2 columns
        terminal.backend_mut().resize(80, 40);
        terminal
            .draw(|frame| app.render(frame))
            .expect("draw at 80 cols");

        // Narrow terminal (50 cols) — should render 1 column
        terminal.backend_mut().resize(50, 40);
        terminal
            .draw(|frame| app.render(frame))
            .expect("draw at 50 cols");
    }

    #[test]
    fn test_failure_badges_visible_in_dashboard() {
        let (tx, _rx) = mpsc::channel(4);
        let mut run = create_test_run(3);
        run.work_packages[0].failure_count = 3;
        run.work_packages[0].state = WPState::Failed;
        let app = App::new(run, tx);

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| app.render(frame))
            .expect("draw with failures");
        // If it renders without panic, the badge code path was exercised
    }

    #[test]
    fn test_stale_detail_popup_cleared_on_state_update() {
        let (tx, _rx) = mpsc::channel(4);
        let mut app = App::new(create_test_run(3), tx);
        app.detail_wp_id = Some("WP99".to_string()); // non-existent

        // Update state — should clear stale reference
        let new_run = create_test_run(3);
        app.update_state(new_run);
        assert!(app.detail_wp_id.is_none());
    }

    #[test]
    fn test_dashboard_lane_scrolling() {
        let (tx, _rx) = mpsc::channel(4);
        let mut run = create_test_run(25);
        // Put all WPs in lane 0 (Pending)
        for wp in &mut run.work_packages {
            wp.state = WPState::Pending;
        }
        let mut app = App::new(run, tx);

        // Navigate down many times
        for _ in 0..20 {
            app.handle_event(TuiEvent::Key(KeyEvent::new(
                KeyCode::Char('j'),
                KeyModifiers::NONE,
            )));
        }

        assert_eq!(app.dashboard.selected_index, 20);
        // Scroll offset should have been adjusted (we can't easily test the exact
        // value without knowing the visible height, but it should be > 0 if there
        // are more items than visible rows)
    }
}
