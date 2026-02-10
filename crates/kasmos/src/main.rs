//! kasmos — Zellij-based agent orchestrator.

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};

mod attach;
mod feature_arg;
mod launch;
mod list_specs;
mod report;
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
  kasmos launch <feature>             Launch wave-gated orchestration
  kasmos launch <feature> --mode continuous
                                      Launch without wave gates
  kasmos status [feature]             Check WP progress
  kasmos attach <feature>             Attach to Zellij session
  kasmos stop [feature]               Gracefully stop orchestration

\x1b[1mTypical Workflow:\x1b[0m
  1. kasmos                           See what features are available
  2. kasmos launch 001-my-feature     Start orchestration (wave-gated)
  3. kasmos status                    Monitor progress
  4. kasmos attach 001-my-feature     Jump into the Zellij session
  5. kasmos stop                      Stop when done"
)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// Launch orchestration for a feature
    Launch {
        /// Feature directory path
        feature: String,
        /// Progression mode: continuous or wave-gated
        #[arg(long, default_value = "wave-gated")]
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
        None => {
            list_specs::run().context("Failed to list specs")?;
        }
        Some(Commands::Launch { feature, mode }) => {
            launch::run(&feature, &mode)
                .await
                .context("Launch failed")?;
        }
        Some(Commands::Status { feature }) => {
            status::run(feature.as_deref()).context("Status failed")?;
        }
        Some(Commands::Attach { feature }) => {
            attach::run(&feature).await.context("Attach failed")?;
        }
        Some(Commands::Stop { feature }) => {
            stop::run(feature.as_deref()).await.context("Stop failed")?;
        }
    }

    Ok(())
}
