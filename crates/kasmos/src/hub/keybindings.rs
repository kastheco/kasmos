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

#[cfg(test)]
mod tests {
    use super::*;
    use crate::hub::scanner::{
        FeatureEntry, OrchestrationStatus, PlanStatus, SpecStatus, TaskProgress,
    };
    use std::path::PathBuf;

    fn dummy_feature(number: &str, slug: &str) -> FeatureEntry {
        FeatureEntry {
            number: number.to_string(),
            slug: slug.to_string(),
            full_slug: format!("{number}-{slug}"),
            spec_status: SpecStatus::Present,
            plan_status: PlanStatus::Present,
            task_progress: TaskProgress::InProgress { done: 1, total: 3 },
            orchestration_status: OrchestrationStatus::None,
            feature_dir: PathBuf::from(format!("kitty-specs/{number}-{slug}")),
        }
    }

    fn key(code: KeyCode) -> TuiEvent {
        TuiEvent::Key(KeyEvent::new(code, KeyModifiers::NONE))
    }

    fn alt_key(code: KeyCode) -> TuiEvent {
        TuiEvent::Key(KeyEvent::new(code, KeyModifiers::ALT))
    }

    #[test]
    fn j_k_navigation() {
        let features = vec![
            dummy_feature("001", "a"),
            dummy_feature("002", "b"),
            dummy_feature("003", "c"),
        ];
        let mut app = App::new(features, None);
        assert_eq!(app.selected, 0);

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert_eq!(app.selected, 1);

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert_eq!(app.selected, 2);

        handle_event(&mut app, key(KeyCode::Char('k')));
        assert_eq!(app.selected, 1);
    }

    #[test]
    fn arrow_key_navigation() {
        let features = vec![dummy_feature("001", "a"), dummy_feature("002", "b")];
        let mut app = App::new(features, None);

        handle_event(&mut app, key(KeyCode::Down));
        assert_eq!(app.selected, 1);

        handle_event(&mut app, key(KeyCode::Up));
        assert_eq!(app.selected, 0);
    }

    #[test]
    fn enter_opens_detail() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None);

        handle_event(&mut app, key(KeyCode::Enter));
        assert_eq!(app.view, HubView::Detail { index: 0 });
    }

    #[test]
    fn enter_does_nothing_on_empty_list() {
        let mut app = App::new(vec![], None);
        handle_event(&mut app, key(KeyCode::Enter));
        assert_eq!(app.view, HubView::List);
    }

    #[test]
    fn esc_returns_to_list() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None);
        app.view = HubView::Detail { index: 0 };

        handle_event(&mut app, key(KeyCode::Esc));
        assert_eq!(app.view, HubView::List);
    }

    #[test]
    fn alt_q_quits() {
        let mut app = App::new(vec![], None);
        assert!(!app.should_quit);

        handle_event(&mut app, alt_key(KeyCode::Char('q')));
        assert!(app.should_quit);
    }

    #[test]
    fn alt_q_quits_from_detail() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None);
        app.view = HubView::Detail { index: 0 };

        handle_event(&mut app, alt_key(KeyCode::Char('q')));
        assert!(app.should_quit);
    }

    #[test]
    fn r_triggers_refresh() {
        let mut app = App::new(vec![], None);
        assert!(!app.refresh_requested);

        handle_event(&mut app, key(KeyCode::Char('r')));
        assert!(app.refresh_requested);
        assert!(app.status_message.is_some());
    }

    #[test]
    fn n_key_read_only_shows_warning() {
        let mut app = App::new(vec![], None); // read-only
        handle_event(&mut app, key(KeyCode::Char('n')));
        assert!(matches!(app.input_mode, InputMode::Normal));
        assert!(app.status_message.as_ref().unwrap().contains("unavailable"));
    }

    #[test]
    fn n_key_with_zellij_enters_prompt() {
        let mut app = App::new(vec![], Some("session".to_string()));
        handle_event(&mut app, key(KeyCode::Char('n')));
        assert!(matches!(app.input_mode, InputMode::NewFeaturePrompt { .. }));
    }

    #[test]
    fn keys_ignored_in_non_normal_mode() {
        let features = vec![dummy_feature("001", "a"), dummy_feature("002", "b")];
        let mut app = App::new(features, Some("s".to_string()));
        app.input_mode = InputMode::NewFeaturePrompt {
            input: String::new(),
        };

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert_eq!(app.selected, 0); // not moved
    }

    #[test]
    fn status_message_cleared_on_keypress() {
        let mut app = App::new(vec![], None);
        app.status_message = Some("old message".to_string());

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert!(app.status_message.is_none());
    }
}
