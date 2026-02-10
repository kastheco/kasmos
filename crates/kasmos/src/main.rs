//! kasmos — Zellij-based agent orchestrator.

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};

mod attach;
mod launch;
mod report;
mod status;
mod stop;

#[derive(Parser)]
#[command(name = "kasmos", version, about = "Zellij agent orchestrator")]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Launch orchestration for a feature
    Launch {
        /// Feature directory path
        feature: String,
        /// Progression mode: continuous or wave-gated
        #[arg(long, default_value = "continuous")]
        mode: String,
    },
    /// Show orchestration status
    Status {
        /// Feature directory (optional, auto-detects from .kasmos/)
        feature: Option<String>,
    },
    /// Attach to an existing orchestration session
    Attach {
        /// Feature directory path
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
        Commands::Launch { feature, mode } => {
            launch::run(&feature, &mode)
                .await
                .context("Launch failed")?;
        }
        Commands::Status { feature } => {
            status::run(feature.as_deref()).context("Status failed")?;
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
