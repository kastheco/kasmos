//! kasmos — Zellij-based agent orchestrator.

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};

mod attach;
mod cmd;
mod feature_arg;
mod hub;
mod list_specs;
mod report;
mod start;
mod status;
mod stop;
mod tui_cmd;
mod tui_preview;

#[derive(Parser)]
#[command(
    name = "kasmos",
    version,
    about = "Zellij agent orchestrator",
    after_help = "\
\x1b[1mQuick Start:\x1b[0m
  kasmos                              Launch hub TUI (project navigator)
  kasmos start <feature>              Start orchestration (TUI dashboard)
  kasmos start <feature> --no-tui     Start without TUI (direct Zellij attach)
  kasmos start <feature> --mode wave-gated
                                       Start with wave gates (default: continuous)
  kasmos status [feature]             Check WP progress
  kasmos cmd status                   Send controller command via FIFO
  kasmos cmd focus WP02               Focus a work package pane
  kasmos attach <feature>             Attach to Zellij session
  kasmos stop [feature]               Gracefully stop orchestration

\x1b[1mTypical Workflow:\x1b[0m
  1. kasmos                           Open the hub TUI
  2. Select a feature and start       Hub launches orchestration in new tab
  3. kasmos cmd status                Query live orchestration state
  4. Alt+h in orchestration TUI       Switch back to hub
  5. kasmos stop                      Stop when done"
)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// List available feature specs
    List,
    /// Start orchestration for a feature and attach to the Zellij session
    Start {
        /// Feature spec ID or prefix (e.g. "002" or "002-ratatui-tui-controller-panel")
        feature: String,
        /// Progression mode: continuous or wave-gated
        #[arg(long, default_value = "wave-gated")]
        mode: String,
        /// Launch the interactive TUI dashboard instead of attaching to Zellij
        #[arg(long)]
        tui: bool,
    },
    /// Show orchestration status
    Status {
        /// Feature directory (optional, auto-detects from .kasmos/)
        feature: Option<String>,
    },
    /// Send a controller command to a running orchestration via FIFO
    Cmd {
        /// Feature directory (optional, auto-detects from current directory)
        #[arg(long)]
        feature: Option<String>,

        #[command(subcommand)]
        command: cmd::FifoCommand,
    },
    /// Attach to an existing orchestration session
    Attach {
        /// Feature spec ID or prefix
        feature: String,
    },
    /// Stop a running orchestration
    Stop {
        /// Feature directory (optional, auto-detects from .kasmos/)
        feature: Option<String>,
    },
    /// Launch the TUI dashboard inside a Zellij controller pane
    #[command(hide = true)]
    TuiCtrl {
        /// Feature spec ID or prefix
        feature: String,
    },
    /// Launch the TUI with animated mock data (no orchestration)
    TuiPreview {
        /// Number of simulated work packages (minimum: 1)
        #[arg(long, default_value_t = 12, value_parser = clap::builder::RangedU64ValueParser::<usize>::new().range(1..))]
        count: usize,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        // No subcommand: launch hub TUI (no logging init — hub manages its own terminal)
        None => hub::run().await.context("Hub TUI failed")?,

        Some(Commands::List) => {
            let _ = kasmos::init_logging(false);
            list_specs::run().context("Failed to list specs")?;
        }
        Some(Commands::Start { feature, mode, tui }) => {
            let _ = kasmos::init_logging(false);
            start::run(&feature, &mode, tui)
                .await
                .context("Start failed")?;
        }
        Some(Commands::Status { feature }) => {
            let _ = kasmos::init_logging(false);
            status::run(feature.as_deref()).context("Status failed")?;
        }
        Some(Commands::Cmd { feature, command }) => {
            let _ = kasmos::init_logging(false);
            cmd::run(feature.as_deref(), command).context("Command send failed")?;
        }
        Some(Commands::Attach { feature }) => {
            let _ = kasmos::init_logging(false);
            attach::run(&feature).await.context("Attach failed")?;
        }
        Some(Commands::Stop { feature }) => {
            let _ = kasmos::init_logging(false);
            stop::run(feature.as_deref()).await.context("Stop failed")?;
        }
        Some(Commands::TuiCtrl { feature }) => {
            tui_cmd::run(&feature).await.context("TUI failed")?;
        }
        Some(Commands::TuiPreview { count }) => {
            tui_preview::run(count).await.context("TUI preview failed")?;
        }
    }

    Ok(())
}
