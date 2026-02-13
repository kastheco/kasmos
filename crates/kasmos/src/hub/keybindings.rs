//! Hub keybinding handlers.
//!
//! Maps keyboard events to hub application state mutations.

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use kasmos::tui::event::TuiEvent;

use super::app::{App, HubView, InputMode};

/// Handle a TUI event for the hub.
pub fn handle_event(app: &mut App, event: TuiEvent) {
    let TuiEvent::Key(key) = event else {
        return;
    };

    // Only handle keys in Normal mode.
    if !matches!(app.input_mode, InputMode::Normal) {
        return;
    }

    // Clear status message on any keypress.
    app.status_message = None;

    match &app.view {
        HubView::List => handle_list_key(app, key),
        HubView::Detail { .. } => handle_detail_key(app, key),
    }
}

fn handle_list_key(app: &mut App, key: KeyEvent) {
    match key.code {
        // Quit
        KeyCode::Char('q') if key.modifiers.contains(KeyModifiers::ALT) => {
            app.should_quit = true;
        }

        // Navigation
        KeyCode::Char('j') | KeyCode::Down => app.select_next(),
        KeyCode::Char('k') | KeyCode::Up => app.select_previous(),

        // Enter detail view
        KeyCode::Enter => {
            if !app.features.is_empty() {
                app.view = HubView::Detail {
                    index: app.selected,
                };
            }
        }

        // Manual refresh
        KeyCode::Char('r') => {
            app.refresh_requested = true;
            app.status_message = Some("Refreshing...".to_string());
        }

        // New feature prompt (placeholder for WP05)
        KeyCode::Char('n') => {
            if app.is_read_only() {
                app.status_message =
                    Some("Action unavailable -- not running inside Zellij".to_string());
            } else {
                app.input_mode = InputMode::NewFeaturePrompt {
                    input: String::new(),
                };
            }
        }

        _ => {}
    }
}

fn handle_detail_key(app: &mut App, key: KeyEvent) {
    match key.code {
        // Quit
        KeyCode::Char('q') if key.modifiers.contains(KeyModifiers::ALT) => {
            app.should_quit = true;
        }

        // Back to list
        KeyCode::Esc => {
            app.view = HubView::List;
        }

        _ => {}
    }
}
