//! Centralized keybinding definitions for the TUI.
//!
//! Maps keyboard events to application state mutations. Global keys (quit,
//! tab switching) are handled first; remaining keys are delegated to the
//! active tab's handler.
//!
//! Keybinding logic is kept thin — actual state mutations call methods on
//! `App` or its sub-state structs.

use crossterm::event::{KeyCode, KeyEvent};

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
fn handle_dashboard_key(_app: &mut App, key: KeyEvent) {
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
fn handle_logs_key(_app: &mut App, key: KeyEvent) {
    match key.code {
        KeyCode::Char('j') | KeyCode::Down => {
            // Scroll down — will be implemented in WP07
        }
        KeyCode::Char('k') | KeyCode::Up => {
            // Scroll up — will be implemented in WP07
        }
        KeyCode::Char('/') => {
            // Activate filter — will be implemented in WP07
        }
        _ => {}
    }
}
