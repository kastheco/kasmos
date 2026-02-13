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

/// Build a hub KDL layout that runs kasmos, inheriting the user's
/// `default_tab_template` from their Zellij default layout.
///
/// Reads `~/.config/zellij/layouts/default.kdl` and extracts the
/// `default_tab_template` block. If the file doesn't exist or can't be
/// parsed, falls back to a minimal template with `status-bar`.
fn build_hub_layout() -> anyhow::Result<(String, std::path::PathBuf)> {
    let kasmos_bin = std::env::current_exe()
        .unwrap_or_else(|_| std::path::PathBuf::from("kasmos"));

    // Try to read the user's default layout for their tab template.
    let tab_template = read_user_tab_template().unwrap_or_else(|| {
        // Fallback: minimal template with status-bar.
        "    default_tab_template {\n        children\n        pane size=1 borderless=true {\n            plugin location=\"status-bar\"\n        }\n    }".to_string()
    });

    let layout = format!(
        "layout {{\n{tab_template}\n    tab name=\"Hub\" {{\n        pane command=\"{}\" close_on_exit=true\n    }}\n}}\n",
        kasmos_bin.display()
    );

    let layout_dir = std::env::temp_dir().join("kasmos");
    std::fs::create_dir_all(&layout_dir)
        .context("Failed to create temp layout directory")?;
    let layout_path = layout_dir.join("hub-layout.kdl");
    std::fs::write(&layout_path, &layout)
        .context("Failed to write hub layout file")?;

    Ok((layout, layout_path))
}

/// Read the user's `default_tab_template` from their Zellij default layout.
///
/// Returns the raw text block if found, or None.
/// Public within the crate so `main.rs` can reuse it for the start bootstrap.
pub(crate) fn read_user_tab_template_text() -> Option<String> {
    read_user_tab_template()
}

/// Read the user's `default_tab_template` from their Zellij default layout.
///
/// Returns the raw text block if found, or None.
fn read_user_tab_template() -> Option<String> {
    let home = std::env::var("HOME").ok()?;
    let default_layout = std::path::PathBuf::from(home)
        .join(".config/zellij/layouts/default.kdl");
    let content = std::fs::read_to_string(&default_layout).ok()?;

    // Extract the default_tab_template block (simple brace-matching).
    let start = content.find("default_tab_template")?;
    let block_start = content[start..].find('{')? + start;
    let mut depth = 0;
    let mut block_end = block_start;
    for (i, ch) in content[block_start..].char_indices() {
        match ch {
            '{' => depth += 1,
            '}' => {
                depth -= 1;
                if depth == 0 {
                    block_end = block_start + i + 1;
                    break;
                }
            }
            _ => {}
        }
    }

    if block_end > block_start {
        Some(format!("    {}", &content[start..block_end]))
    } else {
        None
    }
}

/// Create or attach to the `kasmos-hub` Zellij session.
///
/// If the session exists, attaches to it. Otherwise creates it with a
/// layout that inherits the user's default tab template and runs kasmos.
///
/// Returns `Ok(true)` when Zellij was launched (caller should exit),
/// or `Ok(false)` if already inside Zellij (proceed normally).
async fn ensure_zellij_session() -> anyhow::Result<bool> {
    if std::env::var("ZELLIJ_SESSION_NAME").is_ok() {
        return Ok(false);
    }

    let session_name = "kasmos-hub";

    // Check if session already exists.
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
        let status = tokio::process::Command::new("zellij")
            .args(["attach", session_name])
            .stdin(std::process::Stdio::inherit())
            .stdout(std::process::Stdio::inherit())
            .stderr(std::process::Stdio::inherit())
            .status()
            .await
            .context("Failed to attach to kasmos-hub session")?;

        if !status.success() {
            anyhow::bail!("Zellij attach exited with: {status}");
        }
    } else {
        let (_layout_content, layout_path) = build_hub_layout()?;

        let status = tokio::process::Command::new("zellij")
            .args([
                "--layout",
                &layout_path.display().to_string(),
                "attach",
                session_name,
                "--create",
            ])
            .stdin(std::process::Stdio::inherit())
            .stdout(std::process::Stdio::inherit())
            .stderr(std::process::Stdio::inherit())
            .status()
            .await
            .context("Failed to create kasmos-hub session")?;

        if !status.success() {
            anyhow::bail!("Zellij session exited with: {status}");
        }
    }

    Ok(true)
}

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
/// If not inside Zellij, creates a `kasmos-hub` session using a layout
/// that inherits the user's default tab template (status bar, etc.) and
/// runs kasmos inside it. The outer process exits after Zellij detaches.
pub async fn run() -> anyhow::Result<()> {
    // If not inside Zellij, bootstrap into a session first.
    if ensure_zellij_session().await? {
        return Ok(()); // Zellij was spawned and has exited; outer process done.
    }

    // We're inside Zellij — proceed with the hub TUI.
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
                    if app.outside_zellij() {
                        app.status_message = Some("Requires Zellij -- run kasmos inside a Zellij session".to_string());
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
