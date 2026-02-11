//! FIFO-based command input system for orchestration control.
//!
//! This module provides a named pipe (FIFO) reader that processes operator commands
//! and dispatches them through an mpsc channel for handling by the orchestration engine.

use crate::Result;
use std::path::{Path, PathBuf};
use tokio::io::{AsyncBufReadExt, BufReader};
use tokio::sync::mpsc;

/// Represents a command issued by the operator through the controller pane.
#[derive(Debug, Clone)]
pub enum ControllerCommand {
    /// Restart a failed or crashed work package.
    Restart { wp_id: String },
    /// Pause a running work package.
    Pause { wp_id: String },
    /// Resume a paused work package.
    Resume { wp_id: String },
    /// Display current orchestration status.
    Status,
    /// Focus a specific work package pane.
    Focus { wp_id: String },
    /// Focus and zoom a specific work package pane.
    Zoom { wp_id: String },
    /// Gracefully abort the entire orchestration.
    Abort,
    /// Confirm wave advancement (wave-gated mode).
    Advance,
    /// Skip a failed work package and unblock dependents.
    ForceAdvance { wp_id: String },
    /// Re-run a failed work package from scratch.
    Retry { wp_id: String },
    /// Display available commands.
    Help,
    /// Unknown or malformed command.
    Unknown { input: String },
}

/// Manages the FIFO command input channel.
///
/// Creates a named pipe at `.kasmos/cmd.pipe` and spawns an async reader task
/// that processes command lines and sends them through an mpsc channel.
#[derive(Debug)]
pub struct CommandReader {
    pipe_path: PathBuf,
    command_tx: mpsc::Sender<ControllerCommand>,
}

impl CommandReader {
    /// Create a new command reader and initialize the FIFO.
    ///
    /// # Arguments
    /// * `kasmos_dir` - Path to the `.kasmos` directory
    /// * `command_tx` - Sender for dispatching parsed commands
    ///
    /// # Errors
    /// Returns an error if FIFO creation fails or if the path exists but is not a FIFO.
    pub fn new(kasmos_dir: &Path, command_tx: mpsc::Sender<ControllerCommand>) -> Result<Self> {
        let pipe_path = kasmos_dir.join("cmd.pipe");

        // Guard: Check if path exists and validate it's a FIFO
        if pipe_path.exists() {
            let metadata = std::fs::metadata(&pipe_path)?;
            if !is_fifo(&metadata) {
                return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                    "Path exists but is not a FIFO: {}",
                    pipe_path.display()
                )));
            }
            tracing::info!(path = %pipe_path.display(), "Using existing FIFO");
        } else {
            // Create FIFO with user-only permissions (0o600)
            use nix::sys::stat::Mode;
            use nix::unistd::mkfifo;

            mkfifo(&pipe_path, Mode::S_IRUSR | Mode::S_IWUSR).map_err(|e| {
                crate::error::KasmosError::Other(anyhow::anyhow!("Failed to create FIFO: {}", e))
            })?;
            tracing::info!(path = %pipe_path.display(), "Command FIFO created");
        }

        Ok(Self {
            pipe_path,
            command_tx,
        })
    }

    /// Spawn the FIFO reader as a tokio task.
    ///
    /// The reader runs indefinitely, reopening the FIFO after each writer disconnects.
    /// Commands are parsed and sent through the mpsc channel.
    /// Opens FIFO with O_NONBLOCK to prevent blocking the event loop.
    pub async fn start(self) -> Result<tokio::task::JoinHandle<()>> {
        let handle = tokio::spawn(async move {
            loop {
                // Open FIFO for reading using tokio's native pipe support (epoll-based)
                match Self::open_fifo(&self.pipe_path) {
                    Ok(file) => {
                        let reader = BufReader::new(file);
                        let mut lines = reader.lines();

                        while let Ok(Some(line)) = lines.next_line().await {
                            let line = line.trim().to_string();
                            // Guard: Skip empty lines
                            if line.is_empty() {
                                continue;
                            }

                            match Self::parse_command(&line) {
                                Ok(cmd) => {
                                    tracing::info!(command = ?cmd, "Received command");
                                    if self.command_tx.send(cmd).await.is_err() {
                                        tracing::error!("Command channel closed, exiting reader");
                                        return;
                                    }
                                }
                                Err(e) => {
                                    tracing::warn!(input = %line, error = %e, "Invalid command");
                                }
                            }
                        }

                        // Writer disconnected (EOF) — wait before reopening to avoid busy spin
                        tokio::time::sleep(std::time::Duration::from_millis(100)).await;
                    }
                    Err(e) => {
                        tracing::error!(error = %e, "Failed to open FIFO, retrying...");
                        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
                    }
                }
            }
        });

        Ok(handle)
    }

    /// Open FIFO with O_NONBLOCK flag to prevent blocking the event loop.
    fn open_fifo(path: &std::path::Path) -> std::io::Result<tokio::net::unix::pipe::Receiver> {
        // Open the FIFO using tokio's native pipe support, which uses epoll
        // for proper async I/O (unlike tokio::fs::File which uses spawn_blocking).
        // OpenOptions defaults to non-blocking and read-write to prevent EOF.
        tokio::net::unix::pipe::OpenOptions::new()
            .read_write(true)
            .open_receiver(path)
    }

    /// Parse a command string into a ControllerCommand.
    ///
    /// Validates command grammar strictly and returns helpful error messages for invalid input.
    /// Commands with required arguments must have exactly the right number of args.
    fn parse_command(input: &str) -> Result<ControllerCommand> {
        let parts: Vec<&str> = input.split_whitespace().collect();

        // Guard: Empty input
        if parts.is_empty() {
            return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                "Empty command"
            )));
        }

        match parts[0].to_lowercase().as_str() {
            "restart" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: restart <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::Restart {
                    wp_id: parts[1].to_string(),
                })
            }
            "pause" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: pause <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::Pause {
                    wp_id: parts[1].to_string(),
                })
            }
            "resume" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: resume <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::Resume {
                    wp_id: parts[1].to_string(),
                })
            }
            "status" => {
                if parts.len() != 1 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: status"
                    )));
                }
                Ok(ControllerCommand::Status)
            }
            "focus" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: focus <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::Focus {
                    wp_id: parts[1].to_string(),
                })
            }
            "zoom" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: zoom <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::Zoom {
                    wp_id: parts[1].to_string(),
                })
            }
            "abort" => {
                if parts.len() != 1 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: abort"
                    )));
                }
                Ok(ControllerCommand::Abort)
            }
            "advance" => {
                if parts.len() != 1 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: advance"
                    )));
                }
                Ok(ControllerCommand::Advance)
            }
            "force-advance" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: force-advance <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::ForceAdvance {
                    wp_id: parts[1].to_string(),
                })
            }
            "retry" => {
                if parts.len() != 2 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: retry <WP_ID>"
                    )));
                }
                Ok(ControllerCommand::Retry {
                    wp_id: parts[1].to_string(),
                })
            }
            "help" => {
                if parts.len() != 1 {
                    return Err(crate::error::KasmosError::Other(anyhow::anyhow!(
                        "Usage: help"
                    )));
                }
                Ok(ControllerCommand::Help)
            }
            _ => Ok(ControllerCommand::Unknown {
                input: input.to_string(),
            }),
        }
    }

    /// Clean up the FIFO on shutdown.
    pub fn cleanup(&self) -> Result<()> {
        if self.pipe_path.exists() {
            std::fs::remove_file(&self.pipe_path)?;
            tracing::info!(path = %self.pipe_path.display(), "FIFO cleaned up");
        }
        Ok(())
    }
}

/// Helper function to check if metadata represents a FIFO.
#[cfg(unix)]
fn is_fifo(metadata: &std::fs::Metadata) -> bool {
    use std::os::unix::fs::FileTypeExt;
    metadata.file_type().is_fifo()
}

#[cfg(not(unix))]
fn is_fifo(_metadata: &std::fs::Metadata) -> bool {
    false
}

/// Display help text for available commands.
pub fn command_help_text() -> &'static str {
    r#"
[kasmos] Available Commands:

  restart <WP_ID>       - Restart a failed or crashed work package
  pause <WP_ID>         - Pause a running work package
  resume <WP_ID>        - Resume a paused work package
  status                - Show current orchestration status
  focus <WP_ID>         - Focus a specific work package pane
  zoom <WP_ID>          - Focus and zoom a work package pane
  abort                 - Gracefully shutdown the entire orchestration
  advance               - Confirm wave advancement (wave-gated mode)
  force-advance <WP_ID> - Skip a failed work package, unblock dependents
  retry <WP_ID>         - Re-run a failed work package from scratch
  help                  - Show this message

Write commands to: .kasmos/cmd.pipe
Example: echo "status" > .kasmos/cmd.pipe
Alternative: kasmos cmd status
"#
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_restart_command() {
        let cmd = CommandReader::parse_command("restart WP01").unwrap();
        match cmd {
            ControllerCommand::Restart { wp_id } => assert_eq!(wp_id, "WP01"),
            _ => panic!("Expected Restart command"),
        }
    }

    #[test]
    fn test_parse_pause_command() {
        let cmd = CommandReader::parse_command("pause WP02").unwrap();
        match cmd {
            ControllerCommand::Pause { wp_id } => assert_eq!(wp_id, "WP02"),
            _ => panic!("Expected Pause command"),
        }
    }

    #[test]
    fn test_parse_resume_command() {
        let cmd = CommandReader::parse_command("resume WP03").unwrap();
        match cmd {
            ControllerCommand::Resume { wp_id } => assert_eq!(wp_id, "WP03"),
            _ => panic!("Expected Resume command"),
        }
    }

    #[test]
    fn test_parse_status_command() {
        let cmd = CommandReader::parse_command("status").unwrap();
        match cmd {
            ControllerCommand::Status => (),
            _ => panic!("Expected Status command"),
        }
    }

    #[test]
    fn test_parse_focus_command() {
        let cmd = CommandReader::parse_command("focus WP04").unwrap();
        match cmd {
            ControllerCommand::Focus { wp_id } => assert_eq!(wp_id, "WP04"),
            _ => panic!("Expected Focus command"),
        }
    }

    #[test]
    fn test_parse_zoom_command() {
        let cmd = CommandReader::parse_command("zoom WP05").unwrap();
        match cmd {
            ControllerCommand::Zoom { wp_id } => assert_eq!(wp_id, "WP05"),
            _ => panic!("Expected Zoom command"),
        }
    }

    #[test]
    fn test_parse_abort_command() {
        let cmd = CommandReader::parse_command("abort").unwrap();
        match cmd {
            ControllerCommand::Abort => (),
            _ => panic!("Expected Abort command"),
        }
    }

    #[test]
    fn test_parse_advance_command() {
        let cmd = CommandReader::parse_command("advance").unwrap();
        match cmd {
            ControllerCommand::Advance => (),
            _ => panic!("Expected Advance command"),
        }
    }

    #[test]
    fn test_parse_force_advance_command() {
        let cmd = CommandReader::parse_command("force-advance WP06").unwrap();
        match cmd {
            ControllerCommand::ForceAdvance { wp_id } => assert_eq!(wp_id, "WP06"),
            _ => panic!("Expected ForceAdvance command"),
        }
    }

    #[test]
    fn test_parse_retry_command() {
        let cmd = CommandReader::parse_command("retry WP07").unwrap();
        match cmd {
            ControllerCommand::Retry { wp_id } => assert_eq!(wp_id, "WP07"),
            _ => panic!("Expected Retry command"),
        }
    }

    #[test]
    fn test_parse_help_command() {
        let cmd = CommandReader::parse_command("help").unwrap();
        match cmd {
            ControllerCommand::Help => (),
            _ => panic!("Expected Help command"),
        }
    }

    #[test]
    fn test_parse_unknown_command() {
        let cmd = CommandReader::parse_command("unknown_cmd").unwrap();
        match cmd {
            ControllerCommand::Unknown { input } => assert_eq!(input, "unknown_cmd"),
            _ => panic!("Expected Unknown command"),
        }
    }

    #[test]
    fn test_parse_restart_missing_arg() {
        let result = CommandReader::parse_command("restart");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Usage: restart"));
    }

    #[test]
    fn test_parse_pause_missing_arg() {
        let result = CommandReader::parse_command("pause");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Usage: pause"));
    }

    #[test]
    fn test_parse_empty_command() {
        let result = CommandReader::parse_command("");
        assert!(result.is_err());
    }

    #[test]
    fn test_parse_whitespace_only() {
        let result = CommandReader::parse_command("   ");
        assert!(result.is_err());
    }

    #[test]
    fn test_parse_case_insensitive() {
        let cmd1 = CommandReader::parse_command("RESTART WP01").unwrap();
        let cmd2 = CommandReader::parse_command("restart WP01").unwrap();
        match (cmd1, cmd2) {
            (
                ControllerCommand::Restart { wp_id: id1 },
                ControllerCommand::Restart { wp_id: id2 },
            ) => {
                assert_eq!(id1, id2);
            }
            _ => panic!("Expected both to be Restart commands"),
        }
    }

    #[test]
    fn test_help_text_contains_commands() {
        let help = command_help_text();
        assert!(help.contains("restart"));
        assert!(help.contains("pause"));
        assert!(help.contains("status"));
        assert!(help.contains("abort"));
    }

    #[test]
    fn test_parse_status_with_extra_args() {
        let result = CommandReader::parse_command("status now");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Usage: status"));
    }

    #[test]
    fn test_parse_abort_with_extra_args() {
        let result = CommandReader::parse_command("abort now");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Usage: abort"));
    }

    #[test]
    fn test_parse_advance_with_extra_args() {
        let result = CommandReader::parse_command("advance WP01");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Usage: advance"));
    }

    #[test]
    fn test_parse_help_with_extra_args() {
        let result = CommandReader::parse_command("help me");
        assert!(result.is_err());
        assert!(result.unwrap_err().to_string().contains("Usage: help"));
    }

    #[test]
    fn test_fifo_creation_and_validation() {
        use tempfile::TempDir;

        // Create a temporary directory for the FIFO
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let kasmos_dir = temp_dir.path();

        // Create command channel
        let (tx, _rx) = tokio::sync::mpsc::channel(10);

        // Create CommandReader
        let reader = CommandReader::new(kasmos_dir, tx).expect("Failed to create CommandReader");
        let fifo_path = kasmos_dir.join("cmd.pipe");

        // Verify FIFO was created
        assert!(fifo_path.exists(), "FIFO path should exist");
        let metadata = std::fs::metadata(&fifo_path).expect("Failed to get metadata");
        assert!(is_fifo(&metadata), "Path should be a FIFO");

        // Verify FIFO has correct permissions (user read/write only)
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mode = metadata.permissions().mode();
            // Check that it has read and write for user (0o600 = 0o100000 | 0o100000 | 0o100000 | 0o100000 | 0o100000 | 0o100000)
            // Actually 0o600 = 384 in decimal, which is S_IRUSR | S_IWUSR
            assert_eq!(mode & 0o777, 0o600, "FIFO should have 0o600 permissions");
        }

        // Cleanup
        reader.cleanup().expect("Failed to cleanup FIFO");
        assert!(!fifo_path.exists(), "FIFO should be removed after cleanup");
    }

    #[test]
    fn test_fifo_reuse_validation() {
        use tempfile::TempDir;

        // Create a temporary directory for the FIFO
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let kasmos_dir = temp_dir.path();

        // Create command channel
        let (tx, _rx) = tokio::sync::mpsc::channel(10);

        // Create first CommandReader (creates FIFO)
        let reader1 =
            CommandReader::new(kasmos_dir, tx.clone()).expect("Failed to create first reader");
        let fifo_path = kasmos_dir.join("cmd.pipe");
        assert!(fifo_path.exists(), "FIFO should exist after first creation");

        // Create second CommandReader (should reuse existing FIFO)
        let reader2 = CommandReader::new(kasmos_dir, tx).expect("Failed to create second reader");
        assert!(fifo_path.exists(), "FIFO should still exist");

        // Cleanup
        reader1.cleanup().expect("Failed to cleanup");
        reader2.cleanup().expect("Failed to cleanup");
    }

    #[test]
    fn test_fifo_invalid_path_rejection() {
        use tempfile::TempDir;

        // Create a temporary directory
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let kasmos_dir = temp_dir.path();

        // Create a regular file instead of a FIFO
        let file_path = kasmos_dir.join("cmd.pipe");
        std::fs::File::create(&file_path).expect("Failed to create regular file");

        // Create command channel
        let (tx, _rx) = tokio::sync::mpsc::channel(10);

        // Try to create CommandReader with existing non-FIFO file
        let result = CommandReader::new(kasmos_dir, tx);
        assert!(result.is_err(), "Should reject non-FIFO file");
        assert!(
            result.unwrap_err().to_string().contains("not a FIFO"),
            "Error should mention FIFO validation"
        );
    }

    #[tokio::test]
    async fn test_fifo_end_to_end_integration() {
        use std::io::Write;
        use std::thread;
        use tempfile::TempDir;

        // Create a temporary directory for the FIFO
        let temp_dir = TempDir::new().expect("Failed to create temp dir");
        let kasmos_dir = temp_dir.path();

        // Create command channel
        let (tx, mut rx) = tokio::sync::mpsc::channel(10);

        // Create CommandReader
        let reader = CommandReader::new(kasmos_dir, tx).expect("Failed to create CommandReader");
        let fifo_path = kasmos_dir.join("cmd.pipe");

        // Verify FIFO was created
        assert!(fifo_path.exists(), "FIFO should exist");

        // Start the reader task
        let reader_handle = reader.start().await.expect("Failed to start reader");

        // Spawn a blocking thread to write to the FIFO after a delay
        // (can't use async task because writing to FIFO blocks until reader opens it)
        // With O_NONBLOCK, reader may need to retry, so give it time
        let fifo_path_clone = fifo_path.clone();
        let write_thread = thread::spawn(move || {
            thread::sleep(std::time::Duration::from_millis(2500));
            match std::fs::OpenOptions::new()
                .write(true)
                .open(&fifo_path_clone)
            {
                Ok(mut file) => {
                    let _ = writeln!(file, "status");
                }
                Err(e) => {
                    eprintln!("Failed to open FIFO for writing: {}", e);
                }
            }
        });

        // Wait for command with timeout
        let timeout_duration = std::time::Duration::from_secs(15);
        let result = tokio::time::timeout(timeout_duration, rx.recv()).await;

        // Verify we received the status command
        match result {
            Ok(Some(cmd)) => {
                match cmd {
                    ControllerCommand::Status => {
                        // Success! We received the status command
                    }
                    _ => panic!("Expected Status command, got {:?}", cmd),
                }
            }
            Ok(None) => panic!("Channel closed before receiving command"),
            Err(_) => panic!("Timeout waiting for command on FIFO"),
        }

        // Wait for write thread to complete
        let _ = write_thread.join();

        // Abort the reader task
        reader_handle.abort();

        // Give it a moment to shut down
        tokio::time::sleep(std::time::Duration::from_millis(100)).await;

        // Cleanup FIFO
        if fifo_path.exists() {
            let _ = std::fs::remove_file(&fifo_path);
        }
    }
}
