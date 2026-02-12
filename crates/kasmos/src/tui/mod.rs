//! TUI module — interactive terminal interface for kasmos orchestration.
//!
//! Provides a ratatui-based TUI that displays a kanban dashboard, review
//! workflow, and log viewer inside the Zellij controller pane. The TUI
//! receives state updates via a `tokio::sync::watch` channel and sends
//! commands via the existing `mpsc<EngineAction>` channel.

pub mod app;
pub mod event;
pub mod keybindings;
// tabs/ and widgets/ will be added in later WPs

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
fn setup_terminal() -> anyhow::Result<Terminal<CrosstermBackend<Stdout>>> {
    enable_raw_mode()?;
    let mut stdout = std::io::stdout();
    execute!(stdout, EnterAlternateScreen, EnableMouseCapture)?;
    let backend = CrosstermBackend::new(stdout);
    Terminal::new(backend).context("Failed to create terminal")
}

/// Restore the terminal to its original state (disable raw mode, leave alternate screen).
fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<Stdout>>) -> anyhow::Result<()> {
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
fn install_panic_hook() {
    let original_hook = std::panic::take_hook();
    std::panic::set_hook(Box::new(move |panic_info| {
        // Best-effort terminal restoration — ignore errors since we're panicking
        let _ = disable_raw_mode();
        let _ = execute!(
            std::io::stdout(),
            LeaveAlternateScreen,
            DisableMouseCapture
        );
        original_hook(panic_info);
    }));
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

        if app.should_quit {
            break;
        }
    }

    restore_terminal(&mut terminal)?;
    Ok(())
}
