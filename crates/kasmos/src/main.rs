//! kasmos -- MCP agent swarm orchestrator.

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};

mod feature_arg;
mod list_specs;
mod status;

// Legacy TUI modules -- preserved behind feature gate per FR-024.
// These are not wired into the current CLI surface but kept compilable.
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod attach;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod cmd;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod hub;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod report;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod sendmsg;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod start;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod stop;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod tui_cmd;
#[cfg(feature = "tui")]
#[allow(dead_code)]
mod tui_preview;

#[derive(Parser)]
#[command(
    name = "kasmos",
    version,
    about = "MCP agent swarm orchestrator",
    after_help = "\
\x1b[1mQuick Start:\x1b[0m
  kasmos 011                            Launch orchestration for spec prefix 011
  kasmos serve                          Run MCP server (stdio transport)
  kasmos setup                          Validate environment and generate configs
  kasmos list                           List available feature specs
  kasmos status [feature]               Check WP progress

\x1b[1mTypical Workflow:\x1b[0m
  1. kasmos setup                       Validate environment
  2. kasmos 011                          Launch orchestration session
  3. kasmos status 011                   Monitor progress
  4. kasmos serve                        Run as MCP server (spawned by manager agent)"
)]
struct Cli {
    /// Feature spec prefix (e.g., "011") - launches orchestration session
    spec_prefix: Option<String>,

    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// Run MCP server (stdio transport, spawned by manager agent)
    Serve,
    /// Validate environment and generate default configs
    Setup,
    /// List available feature specs
    List,
    /// Show orchestration status for a feature
    Status {
        /// Feature directory (optional, auto-detects)
        feature: Option<String>,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Some(Commands::Serve) => {
            let _ = kasmos::init_logging(false);
            kasmos::serve::run().await.context("MCP serve failed")?;
        }
        Some(Commands::Setup) => {
            let _ = kasmos::init_logging(false);
            kasmos::setup::run().await.context("Setup failed")?;
        }
        Some(Commands::List) => {
            let _ = kasmos::init_logging(false);
            list_specs::run().context("Failed to list specs")?;
        }
        Some(Commands::Status { feature }) => {
            let _ = kasmos::init_logging(false);
            status::run(feature.as_deref()).context("Status failed")?;
        }
        None => {
            if let Some(ref prefix) = cli.spec_prefix {
                let _ = kasmos::init_logging(false);
                kasmos::launch::run(Some(prefix)).await.context("Launch failed")?;
            } else {
                let _ = kasmos::init_logging(false);
                kasmos::launch::run(None).await.context("Launch failed")?;
            }
        }
    }

    Ok(())
}
