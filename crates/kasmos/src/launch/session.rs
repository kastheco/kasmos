//! Zellij session/tab creation and lifecycle.

use crate::config::Config;
use anyhow::{Context, Result, bail};
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};
use tokio::process::Command;

/// Create orchestration session/tab based on whether we are already inside Zellij.
pub async fn bootstrap(config: &Config, feature_slug: &str, layout_kdl: &str) -> Result<()> {
    if is_inside_zellij() {
        create_orchestration_tab(config, feature_slug, layout_kdl).await
    } else {
        create_orchestration_session(config, feature_slug, layout_kdl).await
    }
}

fn is_inside_zellij() -> bool {
    std::env::var("ZELLIJ_SESSION_NAME").is_ok()
}

async fn create_orchestration_session(
    config: &Config,
    _feature_slug: &str,
    layout_kdl: &str,
) -> Result<()> {
    // kasmos serve runs as an MCP stdio subprocess owned by the manager agent.
    // It is NOT a dedicated pane or process. The manager's OpenCode profile
    // configures kasmos serve as an MCP server in its mcp config.

    let layout_path = write_temp_layout(layout_kdl).context("failed to write layout file")?;
    let session_name = &config.session.session_name;

    let exists = session_exists(&config.paths.zellij_binary, session_name)
        .await
        .context("failed to check if zellij session already exists")?;
    if exists {
        let kill = Command::new(&config.paths.zellij_binary)
            .args(["kill-sessions", session_name])
            .output()
            .await
            .context("failed to execute zellij kill-sessions")?;
        if !kill.status.success() {
            tracing::warn!(
                session = %session_name,
                stderr = %String::from_utf8_lossy(&kill.stderr),
                "failed to kill existing session before relaunch"
            );
        }
    }

    let output = Command::new(&config.paths.zellij_binary)
        .args([
            "--layout",
            layout_path.to_string_lossy().as_ref(),
            "attach",
            session_name,
            "--create",
        ])
        .output()
        .await
        .context("failed to execute zellij attach --create")?;

    let _ = std::fs::remove_file(&layout_path);

    if !output.status.success() {
        bail!(
            "zellij attach failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }

    Ok(())
}

async fn create_orchestration_tab(
    config: &Config,
    feature_slug: &str,
    layout_kdl: &str,
) -> Result<()> {
    let layout_path = write_temp_layout(layout_kdl).context("failed to write layout file")?;

    let output = Command::new(&config.paths.zellij_binary)
        .args([
            "action",
            "new-tab",
            "--layout",
            layout_path.to_string_lossy().as_ref(),
            "--name",
            feature_slug,
        ])
        .output()
        .await
        .context("failed to execute zellij action new-tab")?;

    let _ = std::fs::remove_file(&layout_path);

    if !output.status.success() {
        bail!(
            "zellij action new-tab failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
    }

    Ok(())
}

async fn session_exists(zellij_binary: &str, session_name: &str) -> Result<bool> {
    let output = Command::new(zellij_binary)
        .args(["list-sessions", "--short", "--no-formatting"])
        .output()
        .await
        .context("failed to execute zellij list-sessions")?;

    if !output.status.success() {
        return Ok(false);
    }

    let stdout = String::from_utf8_lossy(&output.stdout);
    Ok(stdout
        .lines()
        .filter_map(|line| line.split_whitespace().next())
        .any(|name| name == session_name))
}

fn write_temp_layout(layout_kdl: &str) -> Result<PathBuf> {
    let ts = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .context("system clock before unix epoch")?
        .as_nanos();
    let file_name = format!("kasmos-launch-{ts}.kdl");
    let path = std::env::temp_dir().join(file_name);
    std::fs::write(&path, layout_kdl)
        .with_context(|| format!("failed writing temp layout {}", path.display()))?;
    Ok(path)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_inside_zellij_from_env_var() {
        unsafe {
            std::env::set_var("ZELLIJ_SESSION_NAME", "kasmos");
        }
        assert!(is_inside_zellij());
        unsafe {
            std::env::remove_var("ZELLIJ_SESSION_NAME");
        }
    }

    #[test]
    fn writes_temp_layout_file() {
        let path = write_temp_layout("layout {}\n").expect("write layout");
        assert!(path.exists());
        let _ = std::fs::remove_file(path);
    }
}
