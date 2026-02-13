//! Centralized keybinding definitions for the TUI.
//!
//! Maps keyboard events to application state mutations. Overlay keys are
//! intercepted first (help > detail), then global keys (quit, tab switching),
//! then tab-specific keys.
//!
//! Keybinding logic is kept thin — actual state mutations call methods on
//! `App` or its sub-state structs.

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use crate::command_handlers::EngineAction;
use crate::types::{ProgressionMode, RunState, WPState};

use super::app::{state_to_lane, App, Tab};

/// Handle a key event by dispatching to overlay, global, or tab-specific handlers.
pub fn handle_key(app: &mut App, key: KeyEvent) {
    // Overlay priority: help > detail > normal
    // When an overlay is active, intercept keys before anything else.

    if app.show_help {
        match key.code {
            KeyCode::Char('?') | KeyCode::Esc => {
                app.show_help = false;
            }
            _ => {
                // Swallow all other keys while help is visible
            }
        }
        return;
    }

    if app.detail_wp_id.is_some() {
        match key.code {
            KeyCode::Esc => {
                app.detail_wp_id = None;
            }
            _ => {
                // Swallow all other keys while detail is visible
            }
        }
        return;
    }

    // Global keys (work in all tabs)
    match key.code {
        KeyCode::Char('q') if key.modifiers.contains(KeyModifiers::ALT) => {
            app.should_quit = true;
            return;
        }
        KeyCode::Char('?') => {
            app.show_help = true;
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
            // Notification cycling (FR-023)
            if !app.notifications.is_empty() {
                app.notification_cycle_index =
                    (app.notification_cycle_index + 1) % app.notifications.len();
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

/// Count WPs in a given lane.
fn lane_count(app: &App, lane: usize) -> usize {
    app.run
        .work_packages
        .iter()
        .filter(|wp| state_to_lane(wp.state) == lane)
        .count()
}

/// Get the WP at a given lane and index position.
fn wp_at_lane_index(app: &App, lane: usize, index: usize) -> Option<String> {
    app.run
        .work_packages
        .iter()
        .filter(|wp| state_to_lane(wp.state) == lane)
        .nth(index)
        .map(|wp| wp.id.clone())
}

/// Handle keys specific to the Dashboard tab.
fn handle_dashboard_key(app: &mut App, key: KeyEvent) {
    let focused = app.dashboard.focused_lane;
    let count = lane_count(app, focused);

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            // Move down in lane (FR-018)
            if count > 0 {
                app.dashboard.selected_index =
                    (app.dashboard.selected_index + 1).min(count.saturating_sub(1));
                app.dashboard.ensure_selected_visible();
            }
        }
        KeyCode::Char('k') | KeyCode::Up => {
            // Move up in lane (FR-018)
            app.dashboard.selected_index = app.dashboard.selected_index.saturating_sub(1);
            app.dashboard.ensure_selected_visible();
        }
        KeyCode::Char('h') | KeyCode::Left => {
            // Move to left lane — reset selection and scroll to top
            if app.dashboard.focused_lane > 0 {
                app.dashboard.focused_lane -= 1;
                app.dashboard.selected_index = 0;
                app.dashboard.scroll_offsets[app.dashboard.focused_lane] = 0;
            }
        }
        KeyCode::Char('l') | KeyCode::Right => {
            // Move to right lane — reset selection and scroll to top
            if app.dashboard.focused_lane < 3 {
                app.dashboard.focused_lane += 1;
                app.dashboard.selected_index = 0;
                app.dashboard.scroll_offsets[app.dashboard.focused_lane] = 0;
            }
        }
        KeyCode::Enter => {
            // WP detail popup (FR-019)
            if let Some(wp_id) = wp_at_lane_index(app, focused, app.dashboard.selected_index) {
                app.detail_wp_id = Some(wp_id);
            }
        }
        KeyCode::Char('A') => {
            if app.run.mode == ProgressionMode::WaveGated && app.run.state == RunState::Paused {
                let _ = app.action_tx.try_send(EngineAction::Advance);
            }
        }
        _ => {}
    }
}

/// Handle keys specific to the Review tab.
fn handle_review_key(app: &mut App, key: KeyEvent) {
    let review_count = app
        .run
        .work_packages
        .iter()
        .filter(|wp| wp.state == WPState::ForReview)
        .count();

    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            if review_count > 0 {
                app.review.selected_index =
                    (app.review.selected_index + 1).min(review_count.saturating_sub(1));
            }
        }
        KeyCode::Char('k') | KeyCode::Up => {
            app.review.selected_index = app.review.selected_index.saturating_sub(1);
        }
        KeyCode::Char('a') => {
            // Approve — dispatch action for the selected review WP
            if let Some(wp) = app
                .run
                .work_packages
                .iter()
                .filter(|wp| wp.state == WPState::ForReview)
                .nth(app.review.selected_index)
            {
                let _ = app.action_tx.try_send(EngineAction::Approve(wp.id.clone()));
            }
        }
        KeyCode::Char('r') => {
            // Reject — hold (ForReview → Pending)
            if let Some(wp) = app
                .run
                .work_packages
                .iter()
                .filter(|wp| wp.state == WPState::ForReview)
                .nth(app.review.selected_index)
            {
                let _ = app.action_tx.try_send(EngineAction::Reject {
                    wp_id: wp.id.clone(),
                    relaunch: false,
                });
            }
        }
        KeyCode::Char('R') => {
            // Reject + relaunch (ForReview → Active)
            if let Some(wp) = app
                .run
                .work_packages
                .iter()
                .filter(|wp| wp.state == WPState::ForReview)
                .nth(app.review.selected_index)
            {
                let _ = app.action_tx.try_send(EngineAction::Reject {
                    wp_id: wp.id.clone(),
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

    #[test]
    fn test_help_overlay_intercepts_keys() {
        let (tx, _rx) = mpsc::channel(4);
        let run = create_test_run(ProgressionMode::Continuous, RunState::Running);
        let mut app = App::new(run, tx);

        app.show_help = true;

        // Tab switch should be swallowed
        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('2'), KeyModifiers::NONE),
        );
        assert_eq!(app.active_tab, Tab::Dashboard);
        assert!(app.show_help);

        // ? dismisses
        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('?'), KeyModifiers::NONE),
        );
        assert!(!app.show_help);
    }

    #[test]
    fn test_detail_popup_intercepts_keys() {
        let (tx, _rx) = mpsc::channel(4);
        let run = create_test_run(ProgressionMode::Continuous, RunState::Running);
        let mut app = App::new(run, tx);

        app.detail_wp_id = Some("WP01".to_string());

        // Tab switch should be swallowed
        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('2'), KeyModifiers::NONE),
        );
        assert_eq!(app.active_tab, Tab::Dashboard);

        // Esc dismisses
        handle_key(&mut app, KeyEvent::new(KeyCode::Esc, KeyModifiers::NONE));
        assert!(app.detail_wp_id.is_none());
    }

    #[test]
    fn test_notification_cycling_wraps() {
        let (tx, _rx) = mpsc::channel(4);
        let run = create_test_run(ProgressionMode::Continuous, RunState::Running);
        let mut app = App::new(run, tx);

        // Add 2 notifications
        for i in 0..2 {
            let id = app.next_notification_id();
            app.notifications.push(super::super::app::Notification {
                id,
                kind: super::super::app::NotificationKind::Failure,
                wp_id: format!("WP{:02}", i + 1),
                message: None,
                failure_type: None,
                severity: None,
                created_at: std::time::Instant::now(),
            });
        }

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('n'), KeyModifiers::NONE),
        );
        assert_eq!(app.notification_cycle_index, 1);

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('n'), KeyModifiers::NONE),
        );
        assert_eq!(app.notification_cycle_index, 0); // wraps
    }
}
