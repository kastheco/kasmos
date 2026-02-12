//! kasmos — Zellij-based agent orchestrator.

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};

mod attach;
mod cmd;
mod feature_arg;
mod list_specs;
mod report;
mod start;
mod status;
mod stop;

#[derive(Parser)]
#[command(
    name = "kasmos",
    version,
    about = "Zellij agent orchestrator",
    after_help = "\
\x1b[1mQuick Start:\x1b[0m
  kasmos                              List available features
  kasmos start <feature>              Start orchestration (wave-gated)
  kasmos start <feature> --mode continuous
                                       Start without wave gates
  kasmos status [feature]             Check WP progress
  kasmos cmd status                   Send controller command via FIFO
  kasmos cmd focus WP02               Focus a work package pane
  kasmos attach <feature>             Attach to Zellij session
  kasmos stop [feature]               Gracefully stop orchestration

\x1b[1mTypical Workflow:\x1b[0m
  1. kasmos                           See what features are available
  2. kasmos start 001-my-feature      Start orchestration (wave-gated)
  3. kasmos cmd status                Query live orchestration state
  4. kasmos attach 001-my-feature     Reattach to the Zellij session
  5. kasmos stop                      Stop when done"
)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
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
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    let _ = kasmos::init_logging();

    match cli.command {
        Commands::List => {
            list_specs::run().context("Failed to list specs")?;
        }
        Commands::Start { feature, mode, tui } => {
            start::run(&feature, &mode, tui)
                .await
                .context("Start failed")?;
        }
        Commands::Status { feature } => {
            status::run(feature.as_deref()).context("Status failed")?;
        }
        Commands::Cmd { feature, command } => {
            cmd::run(feature.as_deref(), command).context("Command send failed")?;
        }
        Commands::Attach { feature } => {
            attach::run(&feature).await.context("Attach failed")?;
        }
        Commands::Stop { feature } => {
            stop::run(feature.as_deref()).await.context("Stop failed")?;
        }
    }

    Ok(())
}
