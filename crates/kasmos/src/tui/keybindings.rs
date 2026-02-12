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
use crate::types::{ProgressionMode, RunState};

use super::app::{App, Tab};

/// Handle a key event by dispatching to global or tab-specific handlers.
pub fn handle_key(app: &mut App, key: KeyEvent) {
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
        // Action keys will be filled in WP04
        _ => {}
    }
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
///
/// Translates crossterm key events to tui-logger widget events and
/// forwards them via `TuiWidgetState::transition()`.
fn handle_logs_key(app: &mut App, key: KeyEvent) {
    let event = match key.code {
        KeyCode::Char('h') => Some(tui_logger::TuiWidgetEvent::HideKey),
        KeyCode::Char('f') => Some(tui_logger::TuiWidgetEvent::FocusKey),
        KeyCode::Char(' ') => Some(tui_logger::TuiWidgetEvent::SpaceKey),
        KeyCode::Up => Some(tui_logger::TuiWidgetEvent::UpKey),
        KeyCode::Down => Some(tui_logger::TuiWidgetEvent::DownKey),
        KeyCode::Left => Some(tui_logger::TuiWidgetEvent::LeftKey),
        KeyCode::Right => Some(tui_logger::TuiWidgetEvent::RightKey),
        KeyCode::Char('+') => Some(tui_logger::TuiWidgetEvent::PlusKey),
        KeyCode::Char('-') => Some(tui_logger::TuiWidgetEvent::MinusKey),
        KeyCode::PageUp => Some(tui_logger::TuiWidgetEvent::PrevPageKey),
        KeyCode::PageDown => Some(tui_logger::TuiWidgetEvent::NextPageKey),
        KeyCode::Esc => Some(tui_logger::TuiWidgetEvent::EscapeKey),
        _ => None,
    };
    if let Some(evt) = event {
        app.logger_state.transition(evt);
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
