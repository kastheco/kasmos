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
  kasmos new [description]              Create a new feature specification
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

#[derive(Debug, Subcommand)]
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
    /// Create a new feature specification
    New {
        /// Initial feature description (optional, can be multiple words)
        #[arg(trailing_var_arg = true)]
        description: Vec<String>,
    },
}

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Some(Commands::Serve) => {
            if let Err(err) = kasmos::init_logging(false) {
                eprintln!("Warning: logging init failed: {err}");
            }
            kasmos::serve::run().await.context("MCP serve failed")?;
        }
        Some(Commands::Setup) => {
            if let Err(err) = kasmos::init_logging(false) {
                eprintln!("Warning: logging init failed: {err}");
            }
            kasmos::setup::run().await.context("Setup failed")?;
        }
        Some(Commands::List) => {
            if let Err(err) = kasmos::init_logging(false) {
                eprintln!("Warning: logging init failed: {err}");
            }
            list_specs::run().context("Failed to list specs")?;
        }
        Some(Commands::Status { feature }) => {
            if let Err(err) = kasmos::init_logging(false) {
                eprintln!("Warning: logging init failed: {err}");
            }
            status::run(feature.as_deref()).context("Status failed")?;
        }
        Some(Commands::New { description }) => {
            if let Err(err) = kasmos::init_logging(false) {
                eprintln!("Warning: logging init failed: {err}");
            }
            let desc = if description.is_empty() {
                None
            } else {
                Some(description.join(" "))
            };
            let code = kasmos::new::run(desc.as_deref())
                .context("New feature spec failed")?;
            std::process::exit(code);
        }
        None => {
            if let Err(err) = kasmos::init_logging(false) {
                eprintln!("Warning: logging init failed: {err}");
            }
            if let Some(ref prefix) = cli.spec_prefix {
                kasmos::launch::run(Some(prefix))
                    .await
                    .context("Launch failed")?;
            } else {
                kasmos::launch::run(None).await.context("Launch failed")?;
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use clap::Parser;

    #[test]
    fn new_command_parses_without_description() {
        let cli = Cli::try_parse_from(["kasmos", "new"]).unwrap();
        match cli.command {
            Some(Commands::New { description }) => {
                assert!(
                    description.is_empty(),
                    "description should be empty when no args given"
                );
            }
            other => panic!("expected Commands::New, got {other:?}"),
        }
    }

    #[test]
    fn new_command_parses_quoted_description() {
        let cli = Cli::try_parse_from(["kasmos", "new", "add dark mode toggle"]).unwrap();
        match cli.command {
            Some(Commands::New { description }) => {
                assert!(
                    !description.is_empty(),
                    "description should have words"
                );
                // A single quoted string arrives as one element
                assert!(
                    description.contains(&"add dark mode toggle".to_string()),
                    "description should contain the full phrase, got: {description:?}"
                );
            }
            other => panic!("expected Commands::New, got {other:?}"),
        }
    }

    #[test]
    fn new_command_parses_unquoted_trailing_words() {
        let cli = Cli::try_parse_from(["kasmos", "new", "add", "dark", "mode"]).unwrap();
        match cli.command {
            Some(Commands::New { description }) => {
                let joined = description.join(" ");
                assert_eq!(
                    joined, "add dark mode",
                    "trailing words should join to 'add dark mode'"
                );
            }
            other => panic!("expected Commands::New, got {other:?}"),
        }
    }
}
