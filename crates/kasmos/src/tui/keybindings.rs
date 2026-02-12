//! Centralized keybinding definitions for the TUI.
//!
//! Maps keyboard events to application state mutations. Global keys (quit,
//! tab switching) are handled first; remaining keys are delegated to the
//! active tab's handler.
//!
//! Keybinding logic is kept thin — actual state mutations call methods on
//! `App` or its sub-state structs.

use crossterm::event::{KeyCode, KeyEvent};

use crate::command_handlers::EngineAction;
use crate::types::{ProgressionMode, RunState, WPState};

use super::app::{App, ConfirmAction, DashboardViewMode, Tab};

/// Handle a key event by dispatching to global or tab-specific handlers.
pub fn handle_key(app: &mut App, key: KeyEvent) {
    // --- Popup confirmation interception (highest priority) ---
    if let Some(ref action) = app.pending_confirm.clone() {
        match key.code {
            KeyCode::Char('y') | KeyCode::Char('Y') => {
                // Execute the confirmed action
                match action {
                    ConfirmAction::ForceAdvance { wp_id } => {
                        let _ = app
                            .action_tx
                            .try_send(EngineAction::ForceAdvance(wp_id.clone()));
                    }
                    ConfirmAction::AbortRun => {
                        // TODO: Add AbortRun engine action if not yet implemented
                    }
                }
                app.pending_confirm = None;
                return;
            }
            KeyCode::Char('n') | KeyCode::Char('N') | KeyCode::Esc => {
                app.pending_confirm = None;
                return;
            }
            _ => return, // Swallow all other keys while popup is visible
        }
    }

    // Global keys (work in all tabs)
    match key.code {
        KeyCode::Char('q') => {
            app.should_quit = true;
            return;
        }
        KeyCode::Char('1') => {
            app.active_tab = Tab::Dashboard;
            return;
        }
        KeyCode::Char('2') => {
            app.active_tab = Tab::Review;
            return;
        }
        KeyCode::Char('3') => {
            app.active_tab = Tab::Logs;
            return;
        }
        KeyCode::Char('n') => {
            if !app.notifications.is_empty() {
                let notif = &app.notifications[0];
                match notif.kind {
                    super::app::NotificationKind::ReviewPending => {
                        let wp_id = notif.wp_id.clone();
                        app.active_tab = Tab::Review;
                        // Find the WP's index in the review list
                        let review_wps: Vec<_> = app
                            .run
                            .work_packages
                            .iter()
                            .filter(|wp| wp.state == WPState::ForReview)
                            .collect();
                        if let Some(pos) = review_wps.iter().position(|wp| wp.id == wp_id) {
                            app.review.selected_index = pos;
                        }
                    }
                    super::app::NotificationKind::Failure
                    | super::app::NotificationKind::InputNeeded => {
                        let wp_id = notif.wp_id.clone();
                        app.active_tab = Tab::Dashboard;
                        if let Some(wp) = app.run.work_packages.iter().find(|wp| wp.id == wp_id) {
                            let lane = super::app::wp_lane(wp.state);
                            app.dashboard.focused_lane = lane;
                            let lane_wps = app.wps_in_lane(lane);
                            if let Some(pos) = lane_wps.iter().position(|w| w.id == wp_id) {
                                app.dashboard.selected_index = pos;
                            }
                        }
                    }
                }
            }
            return;
        }
        _ => {}
    }

    // Tab-specific keys
    match app.active_tab {
        Tab::Dashboard => handle_dashboard_key(app, key),
        Tab::Review => handle_review_key(app, key),
        Tab::Logs => handle_logs_key(app, key),
    }
}

/// Handle keys specific to the Dashboard tab.
fn handle_dashboard_key(app: &mut App, key: KeyEvent) {
    // --- View mode toggle (works in both modes) ---
    if key.code == KeyCode::Char('v') {
        app.dashboard.view_mode = match app.dashboard.view_mode {
            DashboardViewMode::Kanban => DashboardViewMode::DependencyGraph,
            DashboardViewMode::DependencyGraph => DashboardViewMode::Kanban,
        };
        return;
    }

    // In graph mode, lane navigation and action keys are inactive
    if app.dashboard.view_mode == DashboardViewMode::DependencyGraph {
        return;
    }

    match key.code {
        // --- Navigation ---
        KeyCode::Char('j') | KeyCode::Down => {
            let lane_len = app.wps_in_lane(app.dashboard.focused_lane).len();
            if lane_len > 0 {
                app.dashboard.selected_index = (app.dashboard.selected_index + 1).min(lane_len - 1);
            }
        }
        KeyCode::Char('k') | KeyCode::Up => {
            app.dashboard.selected_index = app.dashboard.selected_index.saturating_sub(1);
        }
        KeyCode::Char('h') | KeyCode::Left => {
            app.dashboard.focused_lane = app.dashboard.focused_lane.saturating_sub(1);
            app.dashboard.selected_index = 0;
        }
        KeyCode::Char('l') | KeyCode::Right => {
            app.dashboard.focused_lane = (app.dashboard.focused_lane + 1).min(3);
            app.dashboard.selected_index = 0;
        }

        // --- Action keys ---
        KeyCode::Char('A') => {
            if app.run.mode == ProgressionMode::WaveGated && app.run.state == RunState::Paused {
                let _ = app.action_tx.try_send(EngineAction::Advance);
            }
        }
        KeyCode::Char('R') => {
            if let Some((wp_id, state)) = selected_wp(app) {
                match state {
                    WPState::Failed | WPState::Paused => {
                        let _ = app.action_tx.try_send(EngineAction::Restart(wp_id));
                    }
                    _ => {}
                }
            }
        }
        KeyCode::Char('P') => {
            if let Some((wp_id, state)) = selected_wp(app) {
                match state {
                    WPState::Active => {
                        let _ = app.action_tx.try_send(EngineAction::Pause(wp_id));
                    }
                    WPState::Paused => {
                        let _ = app.action_tx.try_send(EngineAction::Resume(wp_id));
                    }
                    _ => {}
                }
            }
        }
        KeyCode::Char('F') => {
            if let Some((wp_id, state)) = selected_wp(app)
                && state == WPState::Failed
            {
                app.pending_confirm = Some(ConfirmAction::ForceAdvance { wp_id });
            }
        }
        KeyCode::Char('T') => {
            if let Some((wp_id, state)) = selected_wp(app) {
                if state == WPState::Failed {
                    let _ = app.action_tx.try_send(EngineAction::Retry(wp_id));
                }
            }
        }
        _ => {}
    }
}

/// Return `(wp_id, state)` for the currently selected WP in the dashboard.
///
/// Avoids cloning the full `WorkPackage` — callers only need the id and state.
fn selected_wp(app: &App) -> Option<(String, WPState)> {
    let lane_wps = app.wps_in_lane(app.dashboard.focused_lane);
    lane_wps
        .get(app.dashboard.selected_index)
        .map(|wp| (wp.id.clone(), wp.state))
}

/// Handle keys specific to the Review tab.
fn handle_review_key(app: &mut App, key: KeyEvent) {
    let review_wps: Vec<String> = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::ForReview)
        .map(|wp| wp.id.clone())
        .collect();

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            if !review_wps.is_empty() {
                app.review.selected_index =
                    (app.review.selected_index + 1).min(review_wps.len() - 1);
            }
        }
        KeyCode::Char('k') | KeyCode::Up => {
            app.review.selected_index = app.review.selected_index.saturating_sub(1);
        }
        KeyCode::Char('a') => {
            // Approve selected WP
            if let Some(wp_id) = review_wps.get(app.review.selected_index) {
                let _ = app.action_tx.try_send(EngineAction::Approve(wp_id.clone()));
            }
        }
        KeyCode::Char('r') => {
            // Reject + relaunch selected WP
            if let Some(wp_id) = review_wps.get(app.review.selected_index) {
                let _ = app.action_tx.try_send(EngineAction::Reject {
                    wp_id: wp_id.clone(),
                    relaunch: true,
                });
            }
        }
        _ => {}
    }
}

/// Handle keys specific to the Logs tab.
fn handle_logs_key(app: &mut App, key: KeyEvent) {
    if app.logs.filter_active {
        match key.code {
            KeyCode::Esc | KeyCode::Enter => {
                app.logs.filter_active = false;
            }
            KeyCode::Backspace => {
                app.logs.filter.pop();
            }
            KeyCode::Char(c) => {
                app.logs.filter.push(c);
            }
            _ => {}
        }
        return;
    }

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            app.logs.auto_scroll = false;
            app.logs.scroll_offset = app.logs.scroll_offset.saturating_add(1);
        }
        KeyCode::Char('k') | KeyCode::Up => {
            app.logs.auto_scroll = false;
            app.logs.scroll_offset = app.logs.scroll_offset.saturating_sub(1);
        }
        KeyCode::Char('G') => {
            app.logs.auto_scroll = true;
        }
        KeyCode::Char('g') => {
            app.logs.auto_scroll = false;
            app.logs.scroll_offset = 0;
        }
        KeyCode::Char('/') => {
            app.logs.filter_active = true;
        }
        _ => {}
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::types::{OrchestrationRun, WPState, Wave, WaveState, WorkPackage};
    use crossterm::event::KeyModifiers;
    use std::path::PathBuf;
    use tokio::sync::mpsc;

    fn create_test_run(mode: ProgressionMode, state: RunState) -> OrchestrationRun {
        OrchestrationRun {
            id: "run-1".to_string(),
            feature: "feature".to_string(),
            feature_dir: PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages: vec![WorkPackage {
                id: "WP01".to_string(),
                title: "WP01".to_string(),
                state: WPState::Pending,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }],
            waves: vec![Wave {
                index: 0,
                wp_ids: vec!["WP01".to_string()],
                state: WaveState::Pending,
            }],
            state,
            started_at: None,
            completed_at: None,
            mode,
        }
    }

    #[test]
    fn test_advance_wave_key_dispatches_in_wave_gated_pause() {
        let (tx, mut rx) = mpsc::channel(4);
        let run = create_test_run(ProgressionMode::WaveGated, RunState::Paused);
        let mut app = App::new(run, tx);

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('A'), KeyModifiers::NONE),
        );

        assert!(matches!(rx.try_recv(), Ok(EngineAction::Advance)));
    }

    #[test]
    fn test_advance_wave_key_ignored_outside_pause_boundary() {
        let (tx, mut rx) = mpsc::channel(4);
        let run = create_test_run(ProgressionMode::Continuous, RunState::Running);
        let mut app = App::new(run, tx);

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('A'), KeyModifiers::NONE),
        );

        assert!(rx.try_recv().is_err());
    }
}
