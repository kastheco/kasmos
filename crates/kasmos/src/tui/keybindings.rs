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
use crate::types::{ProgressionMode, RunState, WPState, WorkPackage};

use super::app::{App, ConfirmAction, Tab};

/// Handle a key event by dispatching to global or tab-specific handlers.
pub fn handle_key(app: &mut App, key: KeyEvent) {
    // --- Popup confirmation interception (highest priority) ---
    if let Some(action) = app.pending_confirm.take() {
        match key.code {
            KeyCode::Char('y') | KeyCode::Char('Y') => {
                // Execute the confirmed action
                match action {
                    ConfirmAction::ForceAdvance { wp_id } => {
                        let _ = app.action_tx.try_send(EngineAction::ForceAdvance(wp_id));
                    }
                    ConfirmAction::AbortRun => {
                        let _ = app.action_tx.try_send(EngineAction::Abort);
                    }
                }
            }
            KeyCode::Char('n') | KeyCode::Char('N') | KeyCode::Esc => {
                // Dismissed — action already taken out, nothing to do
            }
            _ => {
                // Swallow key but keep popup visible
                app.pending_confirm = Some(action);
            }
        }
        return;
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
            // Notification jump — will be implemented in WP05
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
    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            // Move down in lane — will be implemented in WP03
        }
        KeyCode::Char('k') | KeyCode::Up => {
            // Move up in lane — will be implemented in WP03
        }
        KeyCode::Char('h') | KeyCode::Left => {
            // Move to left lane — will be implemented in WP03
        }
        KeyCode::Char('l') | KeyCode::Right => {
            // Move to right lane — will be implemented in WP03
        }
        KeyCode::Char('A') => {
            if app.run.mode == ProgressionMode::WaveGated && app.run.state == RunState::Paused {
                let _ = app.action_tx.try_send(EngineAction::Advance);
            }
        }
        KeyCode::Char('F') => {
            if let Some(wp) = selected_wp(app)
                && wp.state == WPState::Failed
            {
                app.pending_confirm = Some(ConfirmAction::ForceAdvance {
                    wp_id: wp.id.clone(),
                });
            }
        }
        // Action keys will be filled in WP04
        _ => {}
    }
}

/// Get the currently selected work package in the dashboard, if any.
///
/// Maps the focused lane index to WP state categories and returns the WP
/// at the current selection index within that lane.
fn selected_wp(app: &App) -> Option<&WorkPackage> {
    let lane_wps: Vec<&WorkPackage> = app
        .run
        .work_packages
        .iter()
        .filter(|wp| match app.dashboard.focused_lane {
            0 => wp.state == WPState::Pending || wp.state == WPState::Failed,
            1 => wp.state == WPState::Active || wp.state == WPState::Paused,
            2 => wp.state == WPState::ForReview,
            3 => wp.state == WPState::Completed,
            _ => false,
        })
        .collect();

    lane_wps.get(app.dashboard.selected_index).copied()
}

/// Handle keys specific to the Review tab.
fn handle_review_key(_app: &mut App, key: KeyEvent) {
    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            // Next review item — will be implemented in WP06
        }
        KeyCode::Char('k') | KeyCode::Up => {
            // Previous review item — will be implemented in WP06
        }
        // Approve/reject/request-changes keys will be filled in WP06
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

    /// Create a test run with a Failed WP in lane 0 (planned/failed lane).
    fn create_test_run_with_failed_wp() -> OrchestrationRun {
        OrchestrationRun {
            id: "run-1".to_string(),
            feature: "feature".to_string(),
            feature_dir: PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages: vec![WorkPackage {
                id: "WP01".to_string(),
                title: "WP01".to_string(),
                state: WPState::Failed,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 1,
            }],
            waves: vec![Wave {
                index: 0,
                wp_ids: vec!["WP01".to_string()],
                state: WaveState::Active,
            }],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[test]
    fn test_force_advance_on_failed_wp_sets_pending_confirm() {
        let (tx, _rx) = mpsc::channel(4);
        let run = create_test_run_with_failed_wp();
        let mut app = App::new(run, tx);
        // Lane 0 = planned/failed, selected_index 0 = WP01 (Failed)
        app.active_tab = Tab::Dashboard;
        app.dashboard.focused_lane = 0;
        app.dashboard.selected_index = 0;

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('F'), KeyModifiers::NONE),
        );

        assert!(app.pending_confirm.is_some());
        match &app.pending_confirm {
            Some(ConfirmAction::ForceAdvance { wp_id }) => {
                assert_eq!(wp_id, "WP01");
            }
            other => panic!("Expected ForceAdvance, got {:?}", other),
        }
    }

    #[test]
    fn test_confirm_y_dispatches_force_advance_and_clears_popup() {
        let (tx, mut rx) = mpsc::channel(4);
        let run = create_test_run_with_failed_wp();
        let mut app = App::new(run, tx);
        app.pending_confirm = Some(ConfirmAction::ForceAdvance {
            wp_id: "WP01".to_string(),
        });

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('y'), KeyModifiers::NONE),
        );

        assert!(app.pending_confirm.is_none());
        match rx.try_recv() {
            Ok(EngineAction::ForceAdvance(wp_id)) => assert_eq!(wp_id, "WP01"),
            other => panic!("Expected ForceAdvance action, got {:?}", other),
        }
    }

    #[test]
    fn test_confirm_n_dismisses_popup_without_dispatching() {
        let (tx, mut rx) = mpsc::channel(4);
        let run = create_test_run_with_failed_wp();
        let mut app = App::new(run, tx);
        app.pending_confirm = Some(ConfirmAction::ForceAdvance {
            wp_id: "WP01".to_string(),
        });

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('n'), KeyModifiers::NONE),
        );

        assert!(app.pending_confirm.is_none());
        assert!(rx.try_recv().is_err(), "No action should be dispatched on dismiss");
    }

    #[test]
    fn test_confirm_esc_dismisses_popup_without_dispatching() {
        let (tx, mut rx) = mpsc::channel(4);
        let run = create_test_run_with_failed_wp();
        let mut app = App::new(run, tx);
        app.pending_confirm = Some(ConfirmAction::ForceAdvance {
            wp_id: "WP01".to_string(),
        });

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Esc, KeyModifiers::NONE),
        );

        assert!(app.pending_confirm.is_none());
        assert!(rx.try_recv().is_err(), "No action should be dispatched on Esc");
    }

    #[test]
    fn test_popup_swallows_quit_key() {
        let (tx, _rx) = mpsc::channel(4);
        let run = create_test_run_with_failed_wp();
        let mut app = App::new(run, tx);
        app.pending_confirm = Some(ConfirmAction::ForceAdvance {
            wp_id: "WP01".to_string(),
        });

        // Press 'q' while popup is visible — should NOT quit
        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('q'), KeyModifiers::NONE),
        );

        assert!(!app.should_quit, "Popup must swallow quit key");
        assert!(app.pending_confirm.is_some(), "Popup must remain visible");
    }

    #[test]
    fn test_force_advance_ignored_on_non_failed_wp() {
        let (tx, _rx) = mpsc::channel(4);
        // Default test run has WP01 in Pending state
        let run = create_test_run(ProgressionMode::Continuous, RunState::Running);
        let mut app = App::new(run, tx);
        app.active_tab = Tab::Dashboard;
        app.dashboard.focused_lane = 0;
        app.dashboard.selected_index = 0;

        handle_key(
            &mut app,
            KeyEvent::new(KeyCode::Char('F'), KeyModifiers::NONE),
        );

        assert!(
            app.pending_confirm.is_none(),
            "F key should be ignored on non-Failed WP"
        );
    }
}
