//! `kasmos tui-ctrl <feature>` — standalone TUI process for the Zellij controller pane.
//!
//! Polls `state.json` for orchestration state updates and sends commands
//! (like `advance`) via the FIFO pipe. Designed to run inside a Zellij pane
//! as the controller view when there are no agent panes to display.

use anyhow::{Context, Result};
use std::path::{Path, PathBuf};
use std::time::Duration;
use tokio::sync::mpsc;

use kasmos::command_handlers::EngineAction;
use kasmos::persistence::StatePersister;
use kasmos::tui;

/// Run the TUI as a standalone process connected to a running orchestration
/// via file-based IPC (state.json + cmd.pipe FIFO).
pub async fn run(feature: &str) -> Result<()> {
    let feature_dir = crate::feature_arg::resolve_feature_dir(feature)?;

    let mut config = kasmos::Config::default();
    let config_path = feature_dir.join(".kasmos/config.toml");
    if config_path.exists() {
        config
            .load_from_file(&config_path)
            .context("Failed to load config")?;
    }
    config.load_from_env().ok();

    let kasmos_dir = feature_dir.join(&config.kasmos_dir);
    let fifo_path = kasmos_dir.join("cmd.pipe");

    // Load initial state — retry briefly since the engine may still be writing
    // state.json when the Zellij pane launches.
    let persister = StatePersister::new(&kasmos_dir);
    let initial_run = {
        let mut attempts = 0;
        loop {
            match persister.load() {
                Ok(Some(run)) => break run,
                Ok(None) if attempts < 20 => {
                    attempts += 1;
                    tokio::time::sleep(Duration::from_millis(250)).await;
                }
                Ok(None) => {
                    anyhow::bail!("No orchestration state found after 5s — is kasmos running?");
                }
                Err(e) if attempts < 20 => {
                    attempts += 1;
                    tracing::debug!("State load attempt {} failed: {}", attempts, e);
                    tokio::time::sleep(Duration::from_millis(250)).await;
                }
                Err(e) => {
                    return Err(e).context("Failed to load orchestration state");
                }
            }
        }
    };

    // watch channel: state poller → TUI
    let (watch_tx, watch_rx) = tokio::sync::watch::channel(initial_run);

    // action channel: TUI keybindings → FIFO writer
    let (action_tx, mut action_rx) = mpsc::channel::<EngineAction>(64);

    // Task 1: Poll state.json and push updates through the watch channel
    let poll_kasmos_dir = kasmos_dir.clone();
    let _poller = tokio::spawn(async move {
        let persister = StatePersister::new(&poll_kasmos_dir);
        let mut last_modified = file_modified(&poll_kasmos_dir.join("state.json"));
        loop {
            tokio::time::sleep(Duration::from_millis(500)).await;
            let current_modified = file_modified(&poll_kasmos_dir.join("state.json"));
            if current_modified != last_modified {
                last_modified = current_modified;
                match persister.load() {
                    Ok(Some(run)) => {
                        if watch_tx.send(run).is_err() {
                            break; // TUI closed
                        }
                    }
                    Ok(None) => {} // file disappeared, ignore
                    Err(e) => {
                        tracing::warn!("Failed to reload state: {}", e);
                    }
                }
            }
        }
    });

    // Task 2: Forward EngineAction commands to the FIFO
    let _fifo_writer = tokio::spawn(async move {
        while let Some(action) = action_rx.recv().await {
            let line = match &action {
                EngineAction::Advance => "advance".to_string(),
                EngineAction::Abort => "abort".to_string(),
                EngineAction::Finalize => "finalize".to_string(),
                EngineAction::Retry(wp_id) => format!("retry {wp_id}"),
                EngineAction::Restart(wp_id) => format!("restart {wp_id}"),
                EngineAction::Pause(wp_id) => format!("pause {wp_id}"),
                EngineAction::Resume(wp_id) => format!("resume {wp_id}"),
                EngineAction::ForceAdvance(wp_id) => format!("force-advance {wp_id}"),
                EngineAction::Approve(wp_id) => format!("approve {wp_id}"),
                EngineAction::Reject { wp_id, relaunch } => {
                    if *relaunch {
                        format!("reject {wp_id}")
                    } else {
                        format!("reject {wp_id} --no-relaunch")
                    }
                }
            };
            if let Err(e) = write_fifo(&fifo_path, &format!("{line}\n")) {
                tracing::error!("Failed to send command via FIFO: {}", e);
            }
        }
    });

    // Run the TUI event loop (blocks until user quits)
    tui::run(watch_rx, action_tx).await?;

    Ok(())
}

/// Get the modification time of a file as a comparable value.
fn file_modified(path: &PathBuf) -> Option<std::time::SystemTime> {
    std::fs::metadata(path)
        .ok()
        .and_then(|m| m.modified().ok())
}

/// Write a command line to the FIFO pipe (non-blocking open, blocking write).
fn write_fifo(fifo_path: &Path, command: &str) -> Result<()> {
    use nix::errno::Errno;
    use nix::fcntl::{open, OFlag};
    use nix::sys::stat::Mode;
    use std::io::Write;

    let fd = match open(
        fifo_path,
        OFlag::O_WRONLY | OFlag::O_NONBLOCK,
        Mode::empty(),
    ) {
        Ok(fd) => fd,
        Err(Errno::ENXIO) => {
            anyhow::bail!("No active command reader on FIFO");
        }
        Err(e) => {
            anyhow::bail!("Failed to open FIFO: {}", e);
        }
    };

    // Convert OwnedFd to File via From<OwnedFd>
    let mut file = std::fs::File::from(fd);
    file.write_all(command.as_bytes())
        .context("Failed to write to FIFO")?;
    file.flush().context("Failed to flush FIFO")?;
    Ok(())
}
