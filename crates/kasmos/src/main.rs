//! kasmos — Zellij-based agent orchestrator.

use anyhow::{Context, Result};
use clap::{Parser, Subcommand};

mod attach;
mod cmd;
mod feature_arg;
mod hub;
mod list_specs;
#[allow(dead_code)]
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
  kasmos start <feature> --mode continuous
                                       Start in continuous mode (default: wave-gated)
  kasmos status [feature]             Check WP progress
  kasmos cmd status                   Send controller command via FIFO
  kasmos attach <feature>             Attach to Zellij session
  kasmos stop [feature]               Gracefully stop orchestration

\x1b[1mTypical Workflow:\x1b[0m
  1. kasmos                           Open the hub TUI
  2. Select a feature and start       Hub launches orchestration in new tab
  3. Alt+h in orchestration TUI       Switch back to hub
  4. kasmos stop                      Stop when done"
)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

#[derive(Subcommand)]
enum Commands {
    /// List available feature specs
    List,
    /// Start orchestration for a feature (TUI dashboard in Zellij)
    Start {
        /// Feature spec ID or prefix (e.g. "002" or "002-ratatui-tui-controller-panel")
        feature: String,
        /// Progression mode: wave-gated (default) or continuous
        #[arg(long, default_value = "wave-gated")]
        mode: String,
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
        Some(Commands::Start { feature, mode }) => {
            // Outside Zellij: bootstrap into a Zellij session so the user
            // gets their status bars, then re-invoke inside that session.
            if std::env::var("ZELLIJ_SESSION_NAME").is_err() {
                bootstrap_start_in_zellij(&feature, &mode)
                    .await
                    .context("Failed to bootstrap Zellij session for TUI")?;
            } else {
                // Inside Zellij: run engine + TUI directly.
                start::run(&feature, &mode)
                    .await
                    .context("Start failed")?;
            }
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

/// Bootstrap `kasmos start` inside a Zellij session with the user's tab template.
///
/// Creates a temporary layout that runs `kasmos start <feature> --mode <mode>`
/// inside a Zellij session named `kasmos-start-<feature>`, inheriting the
/// user's `default_tab_template` for status bars.
async fn bootstrap_start_in_zellij(feature: &str, mode: &str) -> Result<()> {
    let kasmos_bin = std::env::current_exe()
        .unwrap_or_else(|_| std::path::PathBuf::from("kasmos"));

    let tab_template = kasmos::LayoutGenerator::tab_template_kdl_string();

    let layout = format!(
        "layout {{\n{tab_template}\n    tab name=\"kasmos\" {{\n        pane command=\"{}\" close_on_exit=true {{\n            args \"start\" \"{}\" \"--mode\" \"{}\"\n        }}\n    }}\n}}\n",
        kasmos_bin.display(),
        feature,
        mode,
    );

    let layout_dir = std::env::temp_dir().join("kasmos");
    std::fs::create_dir_all(&layout_dir)
        .context("Failed to create temp layout directory")?;
    let layout_path = layout_dir.join("start-layout.kdl");
    std::fs::write(&layout_path, &layout)
        .context("Failed to write start layout file")?;

    // Use a session name based on feature so re-invocations reuse/replace it.
    let session_name = format!("kasmos-start-{}", feature);

    // Kill any existing session with this name first.
    let _ = tokio::process::Command::new("zellij")
        .args(["kill-session", &session_name])
        .output()
        .await;
    let _ = tokio::process::Command::new("zellij")
        .args(["delete-session", &session_name])
        .output()
        .await;

    let status = tokio::process::Command::new("zellij")
        .args([
            "--layout",
            &layout_path.display().to_string(),
            "attach",
            &session_name,
            "--create",
        ])
        .stdin(std::process::Stdio::inherit())
        .stdout(std::process::Stdio::inherit())
        .stderr(std::process::Stdio::inherit())
        .status()
        .await
        .context("Failed to launch Zellij session")?;

    if !status.success() {
        anyhow::bail!("Zellij session exited with: {status}");
    }

    Ok(())
}
