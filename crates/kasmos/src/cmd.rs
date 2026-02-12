//! Send controller commands to a running orchestration via FIFO.

use anyhow::{bail, Context, Result};
use clap::Subcommand;
use std::io::Write;
use std::path::{Path, PathBuf};

/// Supported controller commands that can be sent to `.kasmos/cmd.pipe`.
#[derive(Subcommand, Debug, Clone)]
#[command(disable_help_subcommand = true)]
pub enum FifoCommand {
    /// Display orchestration state table
    Status,
    /// Restart a failed/crashed work package
    Restart {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Pause a running work package
    Pause {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Resume a paused work package
    Resume {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Navigate to work package pane
    Focus {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Focus and zoom pane to full view
    Zoom {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Gracefully shutdown orchestration
    Abort,
    /// Confirm wave advancement (wave-gated mode)
    Advance,
    /// Skip failed WP and unblock dependents
    ForceAdvance {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Re-run a failed work package
    Retry {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Approve a work package in review
    Approve {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Reject a work package in review (relaunch for rework)
    Reject {
        /// Work package ID (example: WP02)
        wp_id: String,
    },
    /// Show command help
    Help,
}

impl FifoCommand {
    fn as_fifo_line(&self) -> String {
        match self {
            Self::Status => "status".to_string(),
            Self::Restart { wp_id } => format!("restart {wp_id}"),
            Self::Pause { wp_id } => format!("pause {wp_id}"),
            Self::Resume { wp_id } => format!("resume {wp_id}"),
            Self::Focus { wp_id } => format!("focus {wp_id}"),
            Self::Zoom { wp_id } => format!("zoom {wp_id}"),
            Self::Abort => "abort".to_string(),
            Self::Advance => "advance".to_string(),
            Self::ForceAdvance { wp_id } => format!("force-advance {wp_id}"),
            Self::Retry { wp_id } => format!("retry {wp_id}"),
            Self::Approve { wp_id } => format!("approve {wp_id}"),
            Self::Reject { wp_id } => format!("reject {wp_id}"),
            Self::Help => "help".to_string(),
        }
    }
}

/// Send a FIFO controller command.
pub fn run(feature: Option<&str>, command: FifoCommand) -> Result<()> {
    let base = resolve_base_dir(feature)?;
    let kasmos_dir = resolve_kasmos_dir(&base)?;
    let fifo_path = kasmos_dir.join("cmd.pipe");

    if !fifo_path.exists() {
        bail!(
            "No command pipe found at {}. Is kasmos currently running?",
            fifo_path.display()
        );
    }

    let metadata = std::fs::metadata(&fifo_path)
        .with_context(|| format!("Failed to read metadata for {}", fifo_path.display()))?;
    if !is_fifo(&metadata) {
        bail!(
            "Command pipe path exists but is not a FIFO: {}",
            fifo_path.display()
        );
    }

    let line = command.as_fifo_line();
    send_fifo_command(&fifo_path, &format!("{line}\n"))?;

    println!("Sent command: {}", line);
    println!("Target: {}", fifo_path.display());

    Ok(())
}

fn resolve_base_dir(feature: Option<&str>) -> Result<PathBuf> {
    match feature {
        Some(f) => crate::feature_arg::resolve_feature_dir(f)
            .context("Failed to resolve feature directory"),
        None => std::env::current_dir().context("Failed to get current directory"),
    }
}

fn resolve_kasmos_dir(base: &Path) -> Result<PathBuf> {
    let mut config = kasmos::Config::default();

    let config_path = base.join(".kasmos/config.toml");
    if config_path.exists() {
        config
            .load_from_file(&config_path)
            .with_context(|| format!("Failed to load config file {}", config_path.display()))?;
    }

    config
        .load_from_env()
        .context("Failed to load config from environment")?;

    let kasmos_dir = base.join(&config.kasmos_dir);
    if !kasmos_dir.exists() {
        bail!(
            "No kasmos directory found at {}. Is this a feature directory?",
            kasmos_dir.display()
        );
    }

    Ok(kasmos_dir)
}

fn send_fifo_command(fifo_path: &Path, command: &str) -> Result<()> {
    use nix::errno::Errno;
    use nix::fcntl::{open, OFlag};
    use nix::sys::stat::Mode;

    let fd = match open(
        fifo_path,
        OFlag::O_WRONLY | OFlag::O_NONBLOCK,
        Mode::empty(),
    ) {
        Ok(fd) => fd,
        Err(Errno::ENXIO) => {
            bail!(
                "No active kasmos command reader on {}. Is the orchestration session running?",
                fifo_path.display()
            );
        }
        Err(e) => {
            bail!("Failed to open command pipe {}: {}", fifo_path.display(), e);
        }
    };

    let mut file = std::fs::File::from(fd);
    file.write_all(command.as_bytes())
        .with_context(|| format!("Failed to write command to {}", fifo_path.display()))?;
    file.flush()
        .with_context(|| format!("Failed to flush command pipe {}", fifo_path.display()))?;

    Ok(())
}

#[cfg(unix)]
fn is_fifo(metadata: &std::fs::Metadata) -> bool {
    use std::os::unix::fs::FileTypeExt;
    metadata.file_type().is_fifo()
}

#[cfg(not(unix))]
fn is_fifo(_metadata: &std::fs::Metadata) -> bool {
    false
}

#[cfg(test)]
mod tests {
    use super::FifoCommand;

    #[test]
    fn test_fifo_line_formatting() {
        let cases = vec![
            (FifoCommand::Status, "status"),
            (
                FifoCommand::Restart {
                    wp_id: "WP01".into(),
                },
                "restart WP01",
            ),
            (
                FifoCommand::Pause {
                    wp_id: "WP02".into(),
                },
                "pause WP02",
            ),
            (
                FifoCommand::Resume {
                    wp_id: "WP03".into(),
                },
                "resume WP03",
            ),
            (
                FifoCommand::Focus {
                    wp_id: "WP04".into(),
                },
                "focus WP04",
            ),
            (
                FifoCommand::Zoom {
                    wp_id: "WP05".into(),
                },
                "zoom WP05",
            ),
            (FifoCommand::Abort, "abort"),
            (FifoCommand::Advance, "advance"),
            (
                FifoCommand::ForceAdvance {
                    wp_id: "WP06".into(),
                },
                "force-advance WP06",
            ),
            (
                FifoCommand::Retry {
                    wp_id: "WP07".into(),
                },
                "retry WP07",
            ),
            (FifoCommand::Help, "help"),
        ];

        for (command, expected) in cases {
            assert_eq!(command.as_fifo_line(), expected);
        }
    }
}
