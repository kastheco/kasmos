//! Hub keybinding handlers.
//!
//! Maps keyboard events to hub application state mutations.

use crossterm::event::{KeyCode, KeyEvent, KeyModifiers};

use kasmos::tui::event::TuiEvent;

use super::actions::{self, HubAction};
use super::app::{App, HubView, InputMode};
use super::scanner;

/// Handle a TUI event for the hub.
///
/// Returns `Some(action)` when an action should be dispatched asynchronously
/// by the event loop. Returns `None` for navigation-only events.
pub fn handle_event(app: &mut App, event: TuiEvent) -> Option<HubAction> {
    let TuiEvent::Key(key) = event else {
        return None;
    };

    // Dispatch based on input mode.
    match &app.input_mode {
        InputMode::NewFeaturePrompt { .. } => {
            handle_new_feature_prompt_key(app, key);
            return None;
        }
        InputMode::ConfirmDialog { .. } => {
            return handle_confirm_dialog_key(app, key);
        }
        InputMode::Normal => {}
    }

    // Clear status message on any keypress in Normal mode.
    app.status_message = None;

    match &app.view {
        HubView::List => handle_list_key(app, key),
        HubView::Detail { .. } => handle_detail_key(app, key),
    }
}

fn handle_list_key(app: &mut App, key: KeyEvent) -> Option<HubAction> {
    match key.code {
        // Quit
        KeyCode::Char('q') if key.modifiers.contains(KeyModifiers::ALT) => {
            app.should_quit = true;
        }

        // Navigation
        KeyCode::Char('j') | KeyCode::Down => app.select_next(),
        KeyCode::Char('k') | KeyCode::Up => app.select_previous(),

        // Enter: open detail view OR dispatch primary action
        KeyCode::Enter if key.modifiers.contains(KeyModifiers::SHIFT) => {
            // Shift+Enter: continuous start (secondary mode)
            if app.outside_zellij() {
                app.status_message =
                    Some("Requires Zellij -- run kasmos inside a Zellij session".to_string());
            } else if !app.features.is_empty() {
                let entry = &app.features[app.selected];
                let hub_actions = actions::resolve_actions(entry);
                if hub_actions
                    .iter()
                    .any(|a| matches!(a, HubAction::StartContinuous { .. }))
                {
                    return hub_actions
                        .into_iter()
                        .find(|a| matches!(a, HubAction::StartContinuous { .. }));
                } else {
                    app.status_message =
                        Some("Continuous start not available for this feature".to_string());
                }
            }
        }
        KeyCode::Enter => {
            if !app.features.is_empty() {
                let detail = scanner::load_detail(&app.features[app.selected]);
                app.detail = Some(detail);
                app.detail_selected = 0;
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

        // New feature prompt (WP05) — works without Zellij (just creates a directory).
        KeyCode::Char('n') => {
            app.input_mode = InputMode::NewFeaturePrompt {
                input: String::new(),
            };
        }

        _ => {}
    }
    None
}

fn handle_detail_key(app: &mut App, key: KeyEvent) -> Option<HubAction> {
    match key.code {
        // Quit
        KeyCode::Char('q') if key.modifiers.contains(KeyModifiers::ALT) => {
            app.should_quit = true;
        }

        // Back to list (preserves list selection, clears detail)
        KeyCode::Esc => {
            app.view = HubView::List;
            app.detail = None;
            app.detail_selected = 0;
        }

        // WP row navigation
        KeyCode::Char('j') | KeyCode::Down => app.select_next_wp(),
        KeyCode::Char('k') | KeyCode::Up => app.select_previous_wp(),

        // Enter: dispatch primary action for this feature (WP06 T030)
        KeyCode::Enter if key.modifiers.contains(KeyModifiers::SHIFT) => {
            // Shift+Enter: continuous start (secondary mode)
            if app.outside_zellij() {
                app.status_message =
                    Some("Requires Zellij -- run kasmos inside a Zellij session".to_string());
            } else if let HubView::Detail { index } = app.view {
                if let Some(entry) = app.features.get(index) {
                    let hub_actions = actions::resolve_actions(entry);
                    if let Some(action) = hub_actions
                        .into_iter()
                        .find(|a| matches!(a, HubAction::StartContinuous { .. }))
                    {
                        return Some(action);
                    } else {
                        app.status_message =
                            Some("Continuous start not available for this feature".to_string());
                    }
                }
            }
        }
        KeyCode::Enter => {
            // Enter in detail: dispatch the primary non-ViewDetails action
            if app.outside_zellij() {
                app.status_message =
                    Some("Requires Zellij -- run kasmos inside a Zellij session".to_string());
            } else if let HubView::Detail { index } = app.view {
                if let Some(entry) = app.features.get(index) {
                    let hub_actions = actions::resolve_actions(entry);
                    // Find the first non-ViewDetails action
                    let primary = hub_actions
                        .into_iter()
                        .find(|a| !matches!(a, HubAction::ViewDetails));
                    if let Some(action) = primary {
                        return Some(action);
                    }
                }
            }
        }

        // Manual refresh
        KeyCode::Char('r') => {
            app.refresh_requested = true;
            app.status_message = Some("Refreshing...".to_string());
        }

        _ => {}
    }
    None
}

/// Handle keys for ConfirmDialog input mode (WP07 T034).
///
/// - `y`/`Y`: Switch to continuous, dispatch StartContinuous
/// - `n`/`Enter`: Continue with wave-gated (default), dispatch the pending action
/// - `Esc`: Cancel, return to Normal
fn handle_confirm_dialog_key(app: &mut App, key: KeyEvent) -> Option<HubAction> {
    match key.code {
        KeyCode::Esc => {
            app.input_mode = InputMode::Normal;
            app.pending_action = None;
            None
        }
        KeyCode::Char('y') | KeyCode::Char('Y') => {
            app.input_mode = InputMode::Normal;
            // User chose continuous instead of wave-gated.
            let action = app.pending_action.take();
            if let Some(HubAction::StartWaveGated { feature_slug }) = action {
                Some(HubAction::StartContinuous { feature_slug })
            } else {
                action
            }
        }
        KeyCode::Char('n') | KeyCode::Char('N') | KeyCode::Enter => {
            app.input_mode = InputMode::Normal;
            // User confirmed wave-gated mode (default).
            app.pending_action.take()
        }
        _ => None,
    }
}

/// Handle keys for NewFeaturePrompt input mode (WP05 T024).
fn handle_new_feature_prompt_key(app: &mut App, key: KeyEvent) {
    let InputMode::NewFeaturePrompt { ref mut input } = app.input_mode else {
        return;
    };

    match key.code {
        // Cancel prompt
        KeyCode::Esc => {
            app.input_mode = InputMode::Normal;
        }

        // Finalize: create feature directory
        KeyCode::Enter => {
            let slug = actions::slugify(input);
            if slug.is_empty() {
                app.status_message = Some("Feature name cannot be empty".to_string());
                app.input_mode = InputMode::Normal;
                return;
            }

            // Auto-assign feature number
            let number = actions::next_feature_number(&app.features);
            let full_slug = format!("{number}-{slug}");
            let feature_dir = app.specs_root.join(&full_slug);

            // Create directory
            match std::fs::create_dir_all(&feature_dir) {
                Ok(_) => {
                    app.status_message = Some(format!("Created {full_slug}"));
                    app.refresh_requested = true;
                }
                Err(e) => {
                    app.status_message = Some(format!("Failed to create directory: {e}"));
                }
            }

            app.input_mode = InputMode::Normal;
        }

        // Backspace: remove last character
        KeyCode::Backspace => {
            input.pop();
        }

        // Character input: append to buffer
        KeyCode::Char(c) => {
            input.push(c);
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
        let mut app = App::new(features, None, true);
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
        let mut app = App::new(features, None, true);

        handle_event(&mut app, key(KeyCode::Down));
        assert_eq!(app.selected, 1);

        handle_event(&mut app, key(KeyCode::Up));
        assert_eq!(app.selected, 0);
    }

    #[test]
    fn enter_opens_detail() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);

        handle_event(&mut app, key(KeyCode::Enter));
        assert_eq!(app.view, HubView::Detail { index: 0 });
    }

    #[test]
    fn enter_does_nothing_on_empty_list() {
        let mut app = App::new(vec![], None, true);
        handle_event(&mut app, key(KeyCode::Enter));
        assert_eq!(app.view, HubView::List);
    }

    #[test]
    fn esc_returns_to_list() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };

        handle_event(&mut app, key(KeyCode::Esc));
        assert_eq!(app.view, HubView::List);
    }

    #[test]
    fn alt_q_quits() {
        let mut app = App::new(vec![], None, true);
        assert!(!app.should_quit);

        handle_event(&mut app, alt_key(KeyCode::Char('q')));
        assert!(app.should_quit);
    }

    #[test]
    fn alt_q_quits_from_detail() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };

        handle_event(&mut app, alt_key(KeyCode::Char('q')));
        assert!(app.should_quit);
    }

    #[test]
    fn r_triggers_refresh() {
        let mut app = App::new(vec![], None, true);
        assert!(!app.refresh_requested);

        handle_event(&mut app, key(KeyCode::Char('r')));
        assert!(app.refresh_requested);
        assert!(app.status_message.is_some());
    }

    #[test]
    fn n_key_works_without_zellij() {
        let mut app = App::new(vec![], None, true); // no zellij
        handle_event(&mut app, key(KeyCode::Char('n')));
        // New feature prompt opens regardless of Zellij — it only creates a directory.
        assert!(matches!(app.input_mode, InputMode::NewFeaturePrompt { .. }));
    }

    #[test]
    fn n_key_with_zellij_enters_prompt() {
        let mut app = App::new(vec![], Some("session".to_string()), true);
        handle_event(&mut app, key(KeyCode::Char('n')));
        assert!(matches!(app.input_mode, InputMode::NewFeaturePrompt { .. }));
    }

    #[test]
    fn keys_ignored_in_non_normal_mode() {
        let features = vec![dummy_feature("001", "a"), dummy_feature("002", "b")];
        let mut app = App::new(features, Some("s".to_string()), true);
        app.input_mode = InputMode::NewFeaturePrompt {
            input: String::new(),
        };

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert_eq!(app.selected, 0); // not moved
    }

    #[test]
    fn status_message_cleared_on_keypress() {
        let mut app = App::new(vec![], None, true);
        app.status_message = Some("old message".to_string());

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert!(app.status_message.is_none());
    }

    // -- Detail view keybinding tests --

    #[test]
    fn enter_loads_detail() {
        let tmp = tempfile::tempdir().unwrap();
        let feature_dir = tmp.path().join("001-alpha");
        std::fs::create_dir_all(feature_dir.join("tasks")).unwrap();
        std::fs::write(
            feature_dir.join("tasks/WP01-setup.md"),
            "---\nwork_package_id: WP01\ntitle: \"Setup\"\nlane: done\n---\n# Setup",
        )
        .unwrap();

        let feature = FeatureEntry {
            number: "001".into(),
            slug: "alpha".into(),
            full_slug: "001-alpha".into(),
            spec_status: SpecStatus::Present,
            plan_status: PlanStatus::Present,
            task_progress: TaskProgress::InProgress { done: 1, total: 1 },
            orchestration_status: OrchestrationStatus::None,
            feature_dir,
        };
        let mut app = App::new(vec![feature], None, true);

        handle_event(&mut app, key(KeyCode::Enter));

        assert_eq!(app.view, HubView::Detail { index: 0 });
        assert!(app.detail.is_some());
        let detail = app.detail.as_ref().unwrap();
        assert_eq!(detail.work_packages.len(), 1);
        assert_eq!(detail.work_packages[0].id, "WP01");
        assert_eq!(app.detail_selected, 0);
    }

    #[test]
    fn esc_clears_detail_and_returns_to_list() {
        let features = vec![dummy_feature("001", "a"), dummy_feature("002", "b")];
        let mut app = App::new(features, None, true);
        app.selected = 1;
        app.view = HubView::Detail { index: 1 };
        app.detail = Some(crate::hub::scanner::FeatureDetail {
            work_packages: vec![],
        });

        handle_event(&mut app, key(KeyCode::Esc));

        assert_eq!(app.view, HubView::List);
        assert!(app.detail.is_none());
        assert_eq!(app.selected, 1); // preserved
    }

    #[test]
    fn j_k_in_detail_view_navigates_wps() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };
        app.detail = Some(crate::hub::scanner::FeatureDetail {
            work_packages: vec![
                crate::hub::scanner::WPSummary {
                    id: "WP01".into(),
                    title: "A".into(),
                    lane: "done".into(),
                    wave: None,
                    dependencies: vec![],
                },
                crate::hub::scanner::WPSummary {
                    id: "WP02".into(),
                    title: "B".into(),
                    lane: "doing".into(),
                    wave: None,
                    dependencies: vec![],
                },
                crate::hub::scanner::WPSummary {
                    id: "WP03".into(),
                    title: "C".into(),
                    lane: "planned".into(),
                    wave: None,
                    dependencies: vec![],
                },
            ],
        });

        assert_eq!(app.detail_selected, 0);

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert_eq!(app.detail_selected, 1);

        handle_event(&mut app, key(KeyCode::Char('j')));
        assert_eq!(app.detail_selected, 2);

        handle_event(&mut app, key(KeyCode::Char('k')));
        assert_eq!(app.detail_selected, 1);
    }

    #[test]
    fn r_in_detail_view_triggers_refresh() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        app.view = HubView::Detail { index: 0 };

        handle_event(&mut app, key(KeyCode::Char('r')));
        assert!(app.refresh_requested);
    }

    // -- WP06/WP07: Action dispatch and mode selection tests --

    fn shift_enter() -> TuiEvent {
        TuiEvent::Key(KeyEvent::new(KeyCode::Enter, KeyModifiers::SHIFT))
    }

    fn make_startable_feature(number: &str, slug: &str, total: usize) -> FeatureEntry {
        FeatureEntry {
            number: number.to_string(),
            slug: slug.to_string(),
            full_slug: format!("{number}-{slug}"),
            spec_status: SpecStatus::Present,
            plan_status: PlanStatus::Present,
            task_progress: TaskProgress::InProgress { done: 1, total },
            orchestration_status: OrchestrationStatus::None,
            feature_dir: PathBuf::from(format!("kitty-specs/{number}-{slug}")),
        }
    }

    #[test]
    fn navigation_returns_none() {
        let features = vec![dummy_feature("001", "a"), dummy_feature("002", "b")];
        let mut app = App::new(features, None, true);
        let result = handle_event(&mut app, key(KeyCode::Char('j')));
        assert!(result.is_none());
    }

    #[test]
    fn enter_in_list_returns_none_opens_detail() {
        let features = vec![dummy_feature("001", "a")];
        let mut app = App::new(features, None, true);
        let result = handle_event(&mut app, key(KeyCode::Enter));
        assert!(result.is_none());
        assert_eq!(app.view, HubView::Detail { index: 0 });
    }

    #[test]
    fn shift_enter_in_list_returns_continuous_when_available() {
        let features = vec![make_startable_feature("001", "alpha", 3)];
        let mut app = App::new(features, Some("session".to_string()), true);
        let result = handle_event(&mut app, shift_enter());
        assert!(matches!(result, Some(HubAction::StartContinuous { .. })));
    }

    #[test]
    fn shift_enter_outside_zellij_returns_none() {
        let features = vec![make_startable_feature("001", "alpha", 3)];
        let mut app = App::new(features, None, true); // no zellij
        let result = handle_event(&mut app, shift_enter());
        assert!(result.is_none());
        assert!(app.status_message.as_ref().unwrap().contains("Zellij"));
    }

    #[test]
    fn enter_detail_dispatches_primary_action() {
        let features = vec![make_startable_feature("001", "alpha", 3)];
        let mut app = App::new(features, Some("session".to_string()), true);
        app.view = HubView::Detail { index: 0 };
        app.detail = Some(crate::hub::scanner::FeatureDetail {
            work_packages: vec![],
        });

        let result = handle_event(&mut app, key(KeyCode::Enter));
        // Primary action for in-progress feature is StartWaveGated (default)
        assert!(matches!(result, Some(HubAction::StartWaveGated { .. })));
    }

    #[test]
    fn enter_detail_with_many_wps_dispatches_directly() {
        let features = vec![make_startable_feature("001", "alpha", 8)]; // >6 WPs
        let mut app = App::new(features, Some("session".to_string()), true);
        app.view = HubView::Detail { index: 0 };
        app.detail = Some(crate::hub::scanner::FeatureDetail {
            work_packages: vec![],
        });

        let result = handle_event(&mut app, key(KeyCode::Enter));
        // Wave-gated is already the safe default — no confirm dialog needed
        assert!(matches!(result, Some(HubAction::StartWaveGated { .. })));
    }

    #[test]
    fn confirm_dialog_y_switches_to_continuous() {
        let mut app = App::new(vec![], Some("s".to_string()), true);
        app.input_mode = InputMode::ConfirmDialog {
            message: "test".to_string(),
        };
        app.pending_action = Some(HubAction::StartWaveGated {
            feature_slug: "001-alpha".to_string(),
        });

        let result = handle_event(&mut app, key(KeyCode::Char('y')));
        assert!(matches!(
            result,
            Some(HubAction::StartContinuous { ref feature_slug }) if feature_slug == "001-alpha"
        ));
        assert!(matches!(app.input_mode, InputMode::Normal));
    }

    #[test]
    fn confirm_dialog_n_continues_with_wave_gated() {
        let mut app = App::new(vec![], Some("s".to_string()), true);
        app.input_mode = InputMode::ConfirmDialog {
            message: "test".to_string(),
        };
        app.pending_action = Some(HubAction::StartWaveGated {
            feature_slug: "001-alpha".to_string(),
        });

        let result = handle_event(&mut app, key(KeyCode::Char('n')));
        assert!(matches!(
            result,
            Some(HubAction::StartWaveGated { ref feature_slug }) if feature_slug == "001-alpha"
        ));
    }

    #[test]
    fn confirm_dialog_enter_continues_with_wave_gated() {
        let mut app = App::new(vec![], Some("s".to_string()), true);
        app.input_mode = InputMode::ConfirmDialog {
            message: "test".to_string(),
        };
        app.pending_action = Some(HubAction::StartWaveGated {
            feature_slug: "001-alpha".to_string(),
        });

        let result = handle_event(&mut app, key(KeyCode::Enter));
        assert!(matches!(result, Some(HubAction::StartWaveGated { .. })));
    }

    #[test]
    fn confirm_dialog_esc_cancels() {
        let mut app = App::new(vec![], Some("s".to_string()), true);
        app.input_mode = InputMode::ConfirmDialog {
            message: "test".to_string(),
        };
        app.pending_action = Some(HubAction::StartWaveGated {
            feature_slug: "001-alpha".to_string(),
        });

        let result = handle_event(&mut app, key(KeyCode::Esc));
        assert!(result.is_none());
        assert!(matches!(app.input_mode, InputMode::Normal));
        assert!(app.pending_action.is_none());
    }

    #[test]
    fn enter_detail_outside_zellij_shows_warning() {
        let features = vec![make_startable_feature("001", "alpha", 3)];
        let mut app = App::new(features, None, true); // no zellij
        app.view = HubView::Detail { index: 0 };
        app.detail = Some(crate::hub::scanner::FeatureDetail {
            work_packages: vec![],
        });

        let result = handle_event(&mut app, key(KeyCode::Enter));
        assert!(result.is_none());
        assert!(app.status_message.as_ref().unwrap().contains("Zellij"));
    }
}
