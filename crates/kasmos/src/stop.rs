//! Stop a running orchestration.

use anyhow::{Context, Result, bail};
use std::path::PathBuf;

/// Stop a running orchestration gracefully.
pub async fn run(feature: Option<&str>) -> Result<()> {
    let base = match feature {
        Some(f) => crate::feature_arg::resolve_feature_dir(f)
            .context("Failed to resolve feature directory")?,
        None => std::env::current_dir().context("Failed to get current directory")?,
    };

    let kasmos_dir = base.join(".kasmos");
    if !kasmos_dir.exists() {
        bail!("No .kasmos directory found in {}.", base.display());
    }

    // Try FIFO-based abort first (preferred — graceful)
    let fifo_path = kasmos_dir.join("cmd.pipe");
    if fifo_path.exists() {
        tracing::info!("Sending abort command via FIFO");
        match send_fifo_command(&fifo_path, "abort\n") {
            Ok(()) => {
                println!("Abort command sent. Orchestration will stop gracefully.");
                return Ok(());
            }
            Err(e) => {
                tracing::warn!(error = %e, "FIFO send failed, falling back to PID kill");
            }
        }
    }

    // Fallback: kill the process via lock file PID
    let lock_path = kasmos_dir.join("run.lock");
    if !lock_path.exists() {
        bail!("No running orchestration found (no lock file).");
    }

    let content = std::fs::read_to_string(&lock_path).context("Failed to read lock file")?;

    let pid: i32 = content.trim().parse().context("Invalid PID in lock file")?;

    tracing::info!(pid = pid, "Sending SIGTERM to orchestrator process");

    // Send SIGTERM for graceful shutdown
    let result = unsafe { libc::kill(pid, libc::SIGTERM) };
    if result == 0 {
        println!(
            "SIGTERM sent to PID {}. Orchestration will stop gracefully.",
            pid
        );

        // Clean up stale lock after a moment
        tokio::time::sleep(tokio::time::Duration::from_secs(2)).await;
        if lock_path.exists() {
            let _ = std::fs::remove_file(&lock_path);
        }
    } else {
        let errno = std::io::Error::last_os_error();
        if errno.raw_os_error() == Some(libc::ESRCH) {
            // Process doesn't exist — clean up stale files
            tracing::warn!(pid = pid, "Process not found — cleaning up stale files");
            let _ = std::fs::remove_file(&lock_path);
            println!("No running process found. Cleaned up stale lock file.");
        } else {
            bail!("Failed to send signal to PID {}: {}", pid, errno);
        }
    }

    Ok(())
}

fn send_fifo_command(fifo_path: &PathBuf, command: &str) -> std::io::Result<()> {
    use std::io::Write;
    use std::os::unix::fs::OpenOptionsExt;

    // Open FIFO with O_WRONLY | O_NONBLOCK to avoid blocking if no reader
    let file = std::fs::OpenOptions::new()
        .write(true)
        .custom_flags(libc::O_NONBLOCK)
        .open(fifo_path)?;

    let mut writer = std::io::BufWriter::new(file);
    writer.write_all(command.as_bytes())?;
    writer.flush()?;

    Ok(())
}
