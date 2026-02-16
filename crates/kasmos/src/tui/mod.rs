//! TUI module — interactive terminal interface for kasmos orchestration.
//!
//! Provides a ratatui-based TUI that displays a kanban dashboard, review
//! workflow, and log viewer inside the Zellij controller pane. The TUI
//! receives state updates via a `tokio::sync::watch` channel and sends
//! commands via the existing `mpsc<EngineAction>` channel.

pub mod app;
pub mod event;
pub mod keybindings;
pub mod tabs;
pub(crate) mod widgets;

use std::io::Stdout;
use std::time::Duration;

use anyhow::Context;
use crossterm::event::{DisableMouseCapture, EnableMouseCapture};
use crossterm::execute;
use crossterm::terminal::{
    EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode,
};
use ratatui::Terminal;
use ratatui::backend::CrosstermBackend;
use tokio::sync::{mpsc, watch};

use crate::command_handlers::EngineAction;
use crate::types::OrchestrationRun;

use self::app::App;
use self::event::EventHandler;

/// Set up the terminal for TUI rendering (raw mode, alternate screen, mouse capture).
pub fn setup_terminal() -> anyhow::Result<Terminal<CrosstermBackend<Stdout>>> {
    enable_raw_mode()?;
    let mut stdout = std::io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    Terminal::new(backend).context("Failed to create terminal")
}

/// Restore the terminal to its original state (disable raw mode, leave alternate screen).
pub fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<Stdout>>) -> anyhow::Result<()> {
    disable_raw_mode()?;
    execute!(
        terminal.backend_mut(),
        LeaveAlternateScreen,
        DisableMouseCapture
    )?;
    terminal.show_cursor()?;
    Ok(())
}

/// Install a panic hook that restores the terminal before the default panic handler runs.
///
/// This prevents terminal corruption when panics occur while raw mode is active.
pub fn install_panic_hook() {
    let original_hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |panic_info| {
        // Best-effort terminal restoration — ignore errors since we're panicking
        let _ = disable_raw_mode();
        let _ = execute!(std::io::stdout(), LeaveAlternateScreen, DisableMouseCapture);
        original_hook(panic_info);
    }));
}

/// Navigate to an existing Hub tab or create one running `kasmos`.
///
/// Queries Zellij tab names for a "Hub" tab. If found, switches to it;
/// otherwise creates a new tab named "Hub" running the `kasmos` binary.
async fn navigate_to_hub() -> anyhow::Result<()> {
    // Query existing tab names
    let output = tokio::process::Command::new("zellij")
        .args(["action", "query-tab-names"])
        .output()
        .await?;

    if output.status.success() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        let has_hub = stdout.lines().any(|l| {
            let trimmed = l.trim().to_lowercase();
            trimmed == "hub" || trimmed == "kasmos-hub"
        });
        if has_hub {
            let switch = tokio::process::Command::new("zellij")
                .args(["action", "go-to-tab-name", "Hub"])
                .output()
                .await?;
            if switch.status.success() {
                return Ok(());
            }
        }
    }

    // No hub tab found — create one running kasmos (which launches the hub TUI)
    let create = tokio::process::Command::new("zellij")
        .args(["action", "new-tab", "--name", "Hub"])
        .output()
        .await?;
    if !create.status.success() {
        anyhow::bail!(
            "Failed to create Hub tab: {}",
            String::from_utf8_lossy(&create.stderr)
        );
    }

    // Run kasmos in the new (now-focused) tab
    let run = tokio::process::Command::new("zellij")
        .args(["run", "--", "kasmos"])
        .output()
        .await?;
    if !run.status.success() {
        anyhow::bail!(
            "Failed to run kasmos in Hub tab: {}",
            String::from_utf8_lossy(&run.stderr)
        );
    }

    Ok(())
}

/// Launch a review agent pane in a new Zellij pane for the given WP.
///
/// Creates a floating pane running `ocx oc -- --agent reviewer --prompt "/kas.review <wp_id>"`
/// in the WP's worktree directory.
async fn launch_review_pane(
    wp_id: &str,
    worktree_path: Option<&std::path::Path>,
) -> anyhow::Result<()> {
    let pane_name = format!("review-{}", wp_id.to_lowercase());
    let prompt = format!("/kas.review {}", wp_id);

    // Load profile from config.
    let profile = {
        let mut cfg = crate::config::Config::default();
        let _ = cfg.load_from_env();
        cfg.opencode_profile
    };
    let profile_flag = match &profile {
        Some(p) => format!(
            " -p {}",
            shell_escape::escape(std::borrow::Cow::Borrowed(p))
        ),
        None => String::new(),
    };

    let mut args = vec![
        "run".to_string(),
        "--name".to_string(),
        pane_name,
        "--floating".to_string(),
        "--".to_string(),
        "bash".to_string(),
        "-c".to_string(),
    ];

    // Build the command that runs inside the pane.
    // If we have a worktree path, cd into it first.
    let inner_cmd = if let Some(wt) = worktree_path {
        format!(
            "cd {} && ocx oc{profile_flag} -- --agent reviewer --prompt \"{}\"",
            shell_escape::escape(std::borrow::Cow::Borrowed(wt.to_str().unwrap_or("."))),
            prompt
        )
    } else {
        format!(
            "ocx oc{profile_flag} -- --agent reviewer --prompt \"{}\"",
            prompt
        )
    };
    args.push(inner_cmd);

    let result = tokio::process::Command::new("zellij")
        .args(&args)
        .output()
        .await?;

    if !result.status.success() {
        anyhow::bail!(
            "zellij run failed: {}",
            String::from_utf8_lossy(&result.stderr)
        );
    }

    tracing::info!(wp_id = %wp_id, "Review agent pane launched");
    Ok(())
}

/// Run the TUI event loop.
///
/// This is the main entry point for the TUI. It sets up the terminal, creates
/// the `App` state, and runs the async event loop that:
/// 1. Processes crossterm terminal events (keys, mouse, resize)
/// 2. Receives engine state updates via the watch channel
/// 3. Ticks periodically for elapsed time display updates
///
/// The function returns when the user quits (presses `q`).
///
/// # Arguments
///
/// * `watch_rx` — Receives `OrchestrationRun` snapshots from the engine
/// * `action_tx` — Sends `EngineAction` commands to the engine
pub async fn run(
    mut watch_rx: watch::Receiver<OrchestrationRun>,
    action_tx: mpsc::Sender<EngineAction>,
) -> anyhow::Result<()> {
    // Initialize tui-logger BEFORE setting up the tracing subscriber.
    tui_logger::init_logger(log::LevelFilter::Trace)
        .map_err(|e| anyhow::anyhow!("tui-logger init failed: {e}"))?;
    tui_logger::set_default_level(log::LevelFilter::Trace);
    crate::init_logging(true)?;

    install_panic_hook();
    let mut terminal = setup_terminal()?;
    let mut app = App::new(watch_rx.borrow().clone(), action_tx);
    let mut event_handler = EventHandler::new();

    loop {
        terminal.draw(|frame| app.render(frame))?;

        tokio::select! {
            Some(event) = event_handler.next() => {
                app.handle_event(event);
            }
            Ok(()) = watch_rx.changed() => {
                app.update_state(watch_rx.borrow().clone());
            }
            _ = tokio::time::sleep(Duration::from_millis(250)) => {
                tui_logger::move_events();
                app.on_tick();
            }
        }

        // Handle hub navigation request (WP08 T038/T039)
        if app.open_hub_requested {
            app.open_hub_requested = false;
            tokio::spawn(async {
                if let Err(e) = navigate_to_hub().await {
                    tracing::warn!("Failed to open hub: {}", e);
                }
            });
        }

        // Handle review agent launch request
        if let Some((wp_id, worktree_path)) = app.launch_review_request.take() {
            tokio::spawn(async move {
                if let Err(e) = launch_review_pane(&wp_id, worktree_path.as_deref()).await {
                    tracing::warn!(wp_id = %wp_id, "Failed to launch review pane: {}", e);
                }
            });
        }

        if app.should_quit {
            break;
        }
    }

    restore_terminal(&mut terminal)?;
    Ok(())
}
