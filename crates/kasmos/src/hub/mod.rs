//! Hub TUI module — interactive project command center.
//!
//! Provides a ratatui-based TUI for browsing feature specs, launching
//! OpenCode agent panes, and starting implementation sessions.

pub mod app;
pub mod keybindings;
pub mod scanner;

use std::path::PathBuf;
use std::time::Duration;

use kasmos::tui as tui_plumbing;
use kasmos::tui::event::EventHandler;

/// Run the hub TUI.
///
/// This is the entry point when `kasmos` is invoked with no subcommand.
/// Sets up the terminal, performs an initial scan of `kitty-specs/`, and
/// runs an async event loop with periodic refresh.
pub async fn run() -> anyhow::Result<()> {
    // Detect Zellij session (None = read-only mode).
    let zellij_session = std::env::var("ZELLIJ_SESSION_NAME").ok();

    // Install panic hook before entering raw mode.
    tui_plumbing::install_panic_hook();

    // Initial scan.
    let specs_root = PathBuf::from("kitty-specs");
    let scanner = scanner::FeatureScanner::new(specs_root.clone());
    let features = scanner.scan();

    // Setup terminal.
    let mut terminal = tui_plumbing::setup_terminal()?;

    // Create app state.
    let mut app = app::App::new(features, zellij_session);
    let mut event_handler = EventHandler::new();
    let mut refresh_interval = tokio::time::interval(Duration::from_secs(5));
    refresh_interval.tick().await; // consume initial tick

    loop {
        terminal.draw(|frame| app.render(frame))?;

        tokio::select! {
            Some(event) = event_handler.next() => {
                keybindings::handle_event(&mut app, event);
            }
            _ = refresh_interval.tick() => {
                let scanner_clone = scanner::FeatureScanner::new(specs_root.clone());
                let features = tokio::task::spawn_blocking(move || {
                    scanner_clone.scan()
                }).await?;
                app.update_features(features);
            }
        }

        // Manual refresh requested via 'r' key.
        if app.refresh_requested {
            app.refresh_requested = false;
            let scanner_clone = scanner::FeatureScanner::new(specs_root.clone());
            let features = tokio::task::spawn_blocking(move || scanner_clone.scan()).await?;
            app.update_features(features);
            app.status_message = Some("Refreshed".to_string());
        }

        if app.should_quit {
            break;
        }
    }

    tui_plumbing::restore_terminal(&mut terminal)?;
    Ok(())
}
