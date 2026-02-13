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

use anyhow::Context;
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

/// Ensure the hub is running inside a Zellij session.
///
/// If `ZELLIJ_SESSION_NAME` is not set, creates (or attaches to) a
/// `kasmos-hub` session that runs `kasmos` inside it. The outer process
/// then exits after Zellij detaches, and the inner `kasmos` invocation
/// picks up `ZELLIJ_SESSION_NAME` automatically.
///
/// Returns `Ok(None)` when Zellij was spawned (caller should exit),
/// or `Ok(Some(session_name))` when already inside Zellij.
async fn ensure_zellij_session() -> anyhow::Result<Option<String>> {
    if let Ok(session) = std::env::var("ZELLIJ_SESSION_NAME") {
        return Ok(Some(session));
    }

    let session_name = "kasmos-hub";

    // Resolve the kasmos binary path so re-exec works regardless of cwd.
    let kasmos_bin = std::env::current_exe()
        .unwrap_or_else(|_| std::path::PathBuf::from("kasmos"));

    // Check if a kasmos-hub session already exists.
    let existing = tokio::process::Command::new("zellij")
        .args(["list-sessions", "--short", "--no-formatting"])
        .output()
        .await
        .ok()
        .and_then(|o| if o.status.success() { Some(o) } else { None })
        .map(|o| String::from_utf8_lossy(&o.stdout).to_string())
        .unwrap_or_default();

    let session_exists = existing.lines().any(|l| l.trim() == session_name);

    if session_exists {
        // Attach to the existing session (foreground).
        let status = tokio::process::Command::new("zellij")
            .args(["attach", session_name])
            .stdin(std::process::Stdio::inherit())
            .stdout(std::process::Stdio::inherit())
            .stderr(std::process::Stdio::inherit())
            .status()
            .await
            .context("Failed to attach to Zellij session")?;

        if !status.success() {
            anyhow::bail!("Zellij attach exited with: {status}");
        }
    } else {
        // Create a new session running kasmos inside it.
        let status = tokio::process::Command::new("zellij")
            .args([
                "--session",
                session_name,
                "--",
                &kasmos_bin.display().to_string(),
            ])
            .stdin(std::process::Stdio::inherit())
            .stdout(std::process::Stdio::inherit())
            .stderr(std::process::Stdio::inherit())
            .status()
            .await
            .context("Failed to create Zellij session")?;

        if !status.success() {
            anyhow::bail!("Zellij session exited with: {status}");
        }
    }

    // Zellij has exited (user detached or quit). Outer process should exit.
    Ok(None)
}

/// Run the hub TUI.
///
/// This is the entry point when `kasmos` is invoked with no subcommand.
/// If not inside Zellij, spawns a session first. Then sets up the terminal,
/// performs an initial scan of `kitty-specs/`, and runs an async event loop
/// with periodic refresh.
pub async fn run() -> anyhow::Result<()> {
    // Ensure we're inside Zellij. If not, create/attach and re-exec.
    let zellij_session = match ensure_zellij_session().await? {
        Some(session) => Some(session),
        None => return Ok(()), // Zellij was spawned; outer process exits cleanly.
    };

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
                        app.status_message = Some("Action unavailable -- not running inside Zellij".to_string());
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
