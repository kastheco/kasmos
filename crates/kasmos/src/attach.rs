//! Reattach to an existing orchestration session.

use anyhow::{Context, Result, bail};
use std::path::PathBuf;

/// Reattach to a running orchestration session.
pub async fn run(feature: &str) -> Result<()> {
    let _span = tracing::info_span!("attach", feature = %feature).entered();

    let feature_dir = PathBuf::from(feature);
    if !feature_dir.exists() {
        bail!(
            "Feature directory does not exist: {}",
            feature_dir.display()
        );
    }

    let kasmos_dir = feature_dir.join(".kasmos");
    if !kasmos_dir.exists() {
        bail!("No .kasmos directory found. Launch orchestration first.");
    }

    // Load persisted state
    let state_path = kasmos_dir.join("state.json");
    if !state_path.exists() {
        bail!("No state file found. Has orchestration been launched?");
    }

    let content = std::fs::read_to_string(&state_path).context("Failed to read state file")?;

    let run: kasmos::OrchestrationRun =
        serde_json::from_str(&content).context("Failed to parse state file")?;

    tracing::info!(run_id = %run.id, state = ?run.state, "Loaded orchestration state");

    // Check if the session might still be alive
    let lock_path = kasmos_dir.join("run.lock");
    if !lock_path.exists() {
        tracing::warn!("No lock file found — session may not be running");
    }

    // Display current state before attaching
    println!("Attaching to orchestration: {}", run.id);
    println!("Feature: {}", run.feature);
    println!("State: {:?}", run.state);
    println!(
        "Work packages: {} total, {} active",
        run.work_packages.len(),
        run.work_packages
            .iter()
            .filter(|wp| wp.state == kasmos::WPState::Active)
            .count()
    );

    // Attempt to exec into the Zellij session
    let session_name = format!("kasmos-{}", run.feature);
    println!("\nTo attach to the Zellij session:");
    println!("  zellij attach {}", session_name);

    // If zellij is available, offer direct attach
    let zellij = which_zellij();
    if let Some(zellij_path) = zellij {
        println!("\nAttempting attach...");
        let status = tokio::process::Command::new(&zellij_path)
            .args(["attach", &session_name])
            .status()
            .await
            .context("Failed to run zellij attach")?;

        if !status.success() {
            bail!("zellij attach exited with status: {}", status);
        }
    } else {
        println!("\nzellij not found in PATH. Run the attach command manually.");
    }

    Ok(())
}

fn which_zellij() -> Option<String> {
    std::env::var("PATH").ok().and_then(|path| {
        path.split(':')
            .map(|dir| PathBuf::from(dir).join("zellij"))
            .find(|p| p.exists())
            .map(|p| p.to_string_lossy().to_string())
    })
}
