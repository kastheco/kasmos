//! Display orchestration status from persisted state.

use anyhow::{Context, Result, bail};
use std::path::PathBuf;

/// Resolves kasmos directory from feature path or current directory.
fn resolve_kasmos_dir(feature: Option<&str>) -> Result<PathBuf> {
    let base = match feature {
        Some(f) => PathBuf::from(f),
        None => std::env::current_dir().context("Failed to get current directory")?,
    };

    let kasmos_dir = base.join(".kasmos");
    if !kasmos_dir.exists() {
        bail!(
            "No .kasmos directory found in {}. Is this a feature directory?",
            base.display()
        );
    }

    Ok(kasmos_dir)
}

/// Display orchestration status.
pub fn run(feature: Option<&str>) -> Result<()> {
    let kasmos_dir = resolve_kasmos_dir(feature)?;
    let state_path = kasmos_dir.join("state.json");

    if !state_path.exists() {
        bail!("No state file found. Has orchestration been launched?");
    }

    let content = std::fs::read_to_string(&state_path).context("Failed to read state file")?;

    let run: kasmos::OrchestrationRun =
        serde_json::from_str(&content).context("Failed to parse state file")?;

    // Display formatted status
    println!("Orchestration: {}", run.id);
    println!("Feature:       {}", run.feature);
    println!("State:         {:?}", run.state);
    println!("Mode:          {:?}", run.mode);
    println!();

    if run.work_packages.is_empty() {
        println!("No work packages.");
        return Ok(());
    }

    // Count states
    let mut pending = 0u32;
    let mut active = 0u32;
    let mut completed = 0u32;
    let mut failed = 0u32;
    let mut paused = 0u32;

    println!("Work Packages:");
    for wp in &run.work_packages {
        let status_icon = match wp.state {
            kasmos::WPState::Pending => {
                pending += 1;
                "○"
            }
            kasmos::WPState::Active => {
                active += 1;
                "▶"
            }
            kasmos::WPState::Completed => {
                completed += 1;
                "✓"
            }
            kasmos::WPState::Failed => {
                failed += 1;
                "✗"
            }
            kasmos::WPState::Paused => {
                paused += 1;
                "⏸"
            }
            #[allow(unreachable_patterns)]
            _ => "?",
        };

        println!(
            "  {} {}: {:?} (attempt {}/{})",
            status_icon,
            wp.id,
            wp.state,
            wp.failure_count
                + if wp.state == kasmos::WPState::Active {
                    1
                } else {
                    0
                },
            3 // max_retries — could come from config
        );
    }

    println!();
    println!(
        "Summary: {} pending, {} active, {} completed, {} failed, {} paused",
        pending, active, completed, failed, paused
    );

    if let Some(started) = run.started_at
        && let Ok(duration) = started.elapsed() {
            println!("Running for: {}s", duration.as_secs());
        }

    if run.completed_at.is_some() {
        println!("Status: Completed");
    }

    Ok(())
}
