//! Send a controller command to a running orchestration via FIFO.

use anyhow::{bail, Context, Result};
use std::io::Write;

/// Send a command string to the orchestration FIFO.
pub fn run(feature: Option<&str>, command: &str) -> Result<()> {
    let base = match feature {
        Some(f) => crate::feature_arg::resolve_feature_dir(f)
            .context("Failed to resolve feature directory")?,
        None => find_kasmos_parent()?,
    };

    let fifo_path = base.join(".kasmos/cmd.pipe");
    if !fifo_path.exists() {
        bail!(
            "No command pipe found at {}.\nIs an orchestration running?",
            fifo_path.display()
        );
    }

    // Open FIFO with write-only (may block briefly until the reader opens it)
    let file = std::fs::OpenOptions::new()
        .write(true)
        .open(&fifo_path)
        .with_context(|| format!("Failed to open FIFO: {}", fifo_path.display()))?;

    let mut writer = std::io::BufWriter::new(file);
    writer
        .write_all(format!("{}\n", command).as_bytes())
        .context("Failed to write to FIFO")?;
    writer.flush().context("Failed to flush FIFO")?;

    println!("Sent command: {}", command);
    println!("Target: {}", fifo_path.display());

    Ok(())
}

/// Walk up from the current directory to find a directory containing `.kasmos/cmd.pipe`.
fn find_kasmos_parent() -> Result<std::path::PathBuf> {
    let mut dir = std::env::current_dir().context("Failed to get current directory")?;
    loop {
        if dir.join(".kasmos/cmd.pipe").exists() {
            return Ok(dir);
        }
        if !dir.pop() {
            break;
        }
    }
    // Also check kitty-specs subdirectories (common layout)
    let cwd = std::env::current_dir()?;
    let specs_dir = cwd.join("kitty-specs");
    if specs_dir.is_dir() {
        if let Ok(entries) = std::fs::read_dir(&specs_dir) {
            for entry in entries.flatten() {
                let pipe = entry.path().join(".kasmos/cmd.pipe");
                if pipe.exists() {
                    return Ok(entry.path());
                }
            }
        }
    }
    bail!(
        "No running orchestration found.\n\
         Could not locate .kasmos/cmd.pipe in any parent directory or kitty-specs/."
    );
}
