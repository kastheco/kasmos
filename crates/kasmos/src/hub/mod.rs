//! Hub TUI module — interactive project command center.
//!
//! Provides a ratatui-based TUI for browsing feature specs, launching
//! OpenCode agent panes, and starting implementation sessions.

pub mod actions;
pub mod app;
pub mod keybindings;
pub mod scanner;

use std::path::PathBuf;
use std::time::Duration;

use kasmos::tui as tui_plumbing;
use kasmos::tui::event::EventHandler;

/// Refresh the detail view if the user is currently viewing one.
///
/// Called after `update_features()` to keep the detail data in sync.
fn refresh_detail_if_active(app: &mut app::App) {
    if let app::HubView::Detail { index } = app.view {
        if let Some(feature) = app.features.get(index) {
            let detail = scanner::load_detail(feature);
            app.detail = Some(detail);
            // Clamp WP selection if the list shrank.
            if let Some(ref d) = app.detail {
                if app.detail_selected >= d.work_packages.len() && !d.work_packages.is_empty() {
                    app.detail_selected = d.work_packages.len() - 1;
                }
            }
        }
    }
}

/// Run the hub TUI.
///
/// This is the entry point when `kasmos` is invoked with no subcommand.
/// The hub runs standalone — browsing, detail views, and refresh all work
/// without Zellij. Actions that need Zellij (pane/tab operations) are gated
/// by `is_read_only()` and show a clear message when unavailable.
pub async fn run() -> anyhow::Result<()> {
    // Detect Zellij session (None = actions requiring panes/tabs are unavailable).
    let zellij_session = std::env::var("ZELLIJ_SESSION_NAME").ok();

    // Install panic hook before entering raw mode.
    tui_plumbing::install_panic_hook();

    // Initial scan.
    let specs_root = PathBuf::from("kitty-specs");
    let specs_dir_exists = specs_root.is_dir();
    let scanner = scanner::FeatureScanner::new(specs_root.clone());
    let features = scanner.scan();

    // Setup terminal.
    let mut terminal = tui_plumbing::setup_terminal()?;

    // Create app state.
    let mut app = app::App::new(features, zellij_session, specs_dir_exists);
    let mut event_handler = EventHandler::new();
    let mut refresh_interval = tokio::time::interval(Duration::from_secs(5));
    refresh_interval.tick().await; // consume initial tick

    loop {
        terminal.draw(|frame| app.render(frame))?;

        tokio::select! {
            Some(event) = event_handler.next() => {
                if let Some(action) = keybindings::handle_event(&mut app, event) {
                    if app.is_read_only() {
                        app.status_message = Some("Requires Zellij -- run kasmos inside a Zellij session to launch panes/tabs".to_string());
                    } else {
                        let label = action.label().to_string();
                        tokio::spawn(async move {
                            if let Err(e) = actions::dispatch_action(&action).await {
                                tracing::warn!("Action dispatch failed: {}", e);
                            }
                        });
                        app.status_message = Some(format!("Launched: {label}"));
                    }
                }
            }
            _ = refresh_interval.tick() => {
                let scanner_clone = scanner::FeatureScanner::new(specs_root.clone());
                let features = tokio::task::spawn_blocking(move || {
                    scanner_clone.scan()
                }).await?;
                app.update_features(features);
                refresh_detail_if_active(&mut app);
            }
        }

        // Manual refresh requested via 'r' key.
        if app.refresh_requested {
            app.refresh_requested = false;
            let scanner_clone = scanner::FeatureScanner::new(specs_root.clone());
            let features = tokio::task::spawn_blocking(move || scanner_clone.scan()).await?;
            app.update_features(features);
            refresh_detail_if_active(&mut app);
            app.status_message = Some("Refreshed".to_string());
        }

        if app.should_quit {
            break;
        }
    }

    tui_plumbing::restore_terminal(&mut terminal)?;
    Ok(())
}
