//! MCP worker lifecycle tool implementations.

pub mod despawn_worker;
pub mod list_workers;
pub mod spawn_worker;

use async_trait::async_trait;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use tokio::process::Command;
use tokio::sync::{Mutex, RwLock};
use tokio::task;
use tracing::warn;

use crate::config::Config;
use crate::git::WorktreeManager;
use crate::serve::audit::AuditEvent;
use crate::serve::registry::{WorkerRegistry, WorkerRole};

/// Shared server state used by lifecycle handlers.
pub struct KasmosServer {
    pub config: Config,
    pub repo_root: PathBuf,
    pub feature_slug: String,
    pub session_name: String,
    pub registry: Arc<RwLock<WorkerRegistry>>,
    pub runtime: Arc<dyn WorkerRuntime>,
    pub audit_log: Arc<Mutex<Vec<AuditEvent>>>,
}

impl KasmosServer {
    pub fn new(
        config: Config,
        repo_root: PathBuf,
        feature_slug: String,
        session_name: String,
        runtime: Arc<dyn WorkerRuntime>,
    ) -> Self {
        Self {
            config,
            repo_root,
            feature_slug,
            session_name,
            registry: Arc::new(RwLock::new(WorkerRegistry::default())),
            runtime,
            audit_log: Arc::new(Mutex::new(Vec::new())),
        }
    }
}

/// Runtime abstraction for pane and worktree side effects.
#[async_trait]
pub trait WorkerRuntime: Send + Sync {
    async fn create_worker_pane(
        &self,
        session_name: &str,
        pane_name: &str,
        role: WorkerRole,
        prompt: &str,
        cwd: Option<&Path>,
    ) -> anyhow::Result<()>;

    async fn close_worker_pane(&self, session_name: &str, pane_name: &str) -> anyhow::Result<()>;

    async fn pane_exists(&self, session_name: &str, pane_name: &str) -> anyhow::Result<bool>;

    async fn ensure_coder_worktree(
        &self,
        repo_root: &Path,
        feature_slug: &str,
        wp_id: &str,
        base_branch: &str,
    ) -> anyhow::Result<PathBuf>;

    async fn cleanup_coder_worktree(
        &self,
        repo_root: &Path,
        feature_slug: &str,
        wp_id: &str,
    ) -> anyhow::Result<()>;
}

/// Production runtime backed by zellij and git commands.
pub struct RealWorkerRuntime {
    zellij_binary: String,
    opencode_binary: String,
    opencode_profile: Option<String>,
}

impl RealWorkerRuntime {
    pub fn from_config(config: &Config) -> Self {
        Self {
            zellij_binary: config.paths.zellij_binary.clone(),
            opencode_binary: config.agent.opencode_binary.clone(),
            opencode_profile: config.agent.opencode_profile.clone(),
        }
    }

    async fn session_is_active(&self, session_name: &str) -> anyhow::Result<bool> {
        let output = Command::new(&self.zellij_binary)
            .arg("list-sessions")
            .output()
            .await?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            anyhow::bail!("failed to list zellij sessions: {stderr}");
        }

        let stdout = String::from_utf8_lossy(&output.stdout);
        Ok(session_is_active_in_list(&stdout, session_name))
    }
}

#[async_trait]
impl WorkerRuntime for RealWorkerRuntime {
    async fn create_worker_pane(
        &self,
        session_name: &str,
        pane_name: &str,
        role: WorkerRole,
        prompt: &str,
        cwd: Option<&Path>,
    ) -> anyhow::Result<()> {
        let mut args: Vec<String> = vec![
            "--session".to_string(),
            session_name.to_string(),
            "run".to_string(),
            "-n".to_string(),
            pane_name.to_string(),
        ];

        if let Some(path) = cwd {
            args.push("--cwd".to_string());
            args.push(path.display().to_string());
        }

        args.push("--".to_string());
        args.push(self.opencode_binary.clone());
        args.push("oc".to_string());

        if let Some(profile) = &self.opencode_profile {
            args.push("-p".to_string());
            args.push(profile.clone());
        }

        args.push("--".to_string());
        args.push("--agent".to_string());
        args.push(role.as_str().to_string());
        args.push("--prompt".to_string());
        args.push(prompt.to_string());

        let output = Command::new(&self.zellij_binary)
            .args(&args)
            .output()
            .await?;
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            anyhow::bail!("failed to create worker pane '{pane_name}': {stderr}");
        }

        Ok(())
    }

    async fn close_worker_pane(&self, session_name: &str, pane_name: &str) -> anyhow::Result<()> {
        let output = Command::new(&self.zellij_binary)
            .args([
                "--session",
                session_name,
                "action",
                "close-pane",
                "--pane-name",
                pane_name,
            ])
            .output()
            .await?;

        if output.status.success() {
            return Ok(());
        }

        let stderr = String::from_utf8_lossy(&output.stderr);
        if stderr_has_unsupported_pane_name_flag(&stderr) {
            warn!(
                session_name,
                pane_name,
                "zellij close-pane does not support pane-name targeting; treating close as best effort"
            );
            return Ok(());
        }

        if stderr_has_missing_pane(&stderr) || stderr_has_missing_session(&stderr) {
            return Ok(());
        }

        anyhow::bail!("failed to close pane '{pane_name}': {stderr}");
    }

    async fn pane_exists(&self, session_name: &str, pane_name: &str) -> anyhow::Result<bool> {
        let probe_path = std::env::temp_dir().join(format!(
            "kasmos-pane-probe-{session_name}-{pane_name}-{}.txt",
            std::process::id()
        ));
        let probe_path_string = probe_path.display().to_string();

        let output = Command::new(&self.zellij_binary)
            .args([
                "--session",
                session_name,
                "action",
                "dump-screen",
                "--pane-name",
                pane_name,
                probe_path_string.as_str(),
            ])
            .output()
            .await?;

        let _ = std::fs::remove_file(&probe_path);

        if output.status.success() {
            return Ok(true);
        }

        let stderr = String::from_utf8_lossy(&output.stderr);
        if stderr_has_missing_session(&stderr) || stderr_has_missing_pane(&stderr) {
            return Ok(false);
        }

        if stderr_has_unsupported_pane_name_flag(&stderr) {
            let session_active = self.session_is_active(session_name).await?;
            warn!(
                session_name,
                pane_name,
                session_active,
                "zellij dump-screen does not support pane-name targeting; falling back to session-level liveness"
            );
            return Ok(session_active);
        }

        warn!(
            session_name,
            pane_name,
            stderr = %stderr.trim(),
            "unable to verify pane liveness; keeping worker active"
        );
        Ok(true)
    }

    async fn ensure_coder_worktree(
        &self,
        repo_root: &Path,
        feature_slug: &str,
        wp_id: &str,
        base_branch: &str,
    ) -> anyhow::Result<PathBuf> {
        let repo_root = repo_root.to_path_buf();
        let feature_slug = feature_slug.to_string();
        let wp_id = wp_id.to_string();
        let base_branch = base_branch.to_string();

        task::spawn_blocking(move || {
            let manager = WorktreeManager::new(&repo_root, &feature_slug)?;
            manager.ensure_worktree(&wp_id, &base_branch)
        })
        .await
        .map_err(|e| anyhow::anyhow!("worktree create task join error: {e}"))?
    }

    async fn cleanup_coder_worktree(
        &self,
        repo_root: &Path,
        feature_slug: &str,
        wp_id: &str,
    ) -> anyhow::Result<()> {
        let repo_root = repo_root.to_path_buf();
        let feature_slug = feature_slug.to_string();
        let wp_id = wp_id.to_string();

        task::spawn_blocking(move || {
            let manager = WorktreeManager::new(&repo_root, &feature_slug)?;
            manager.remove_worktree(&wp_id)
        })
        .await
        .map_err(|e| anyhow::anyhow!("worktree cleanup task join error: {e}"))?
    }
}

fn stderr_has_unsupported_pane_name_flag(stderr: &str) -> bool {
    stderr.contains("--pane-name") && stderr.contains("wasn't expected")
}

fn stderr_has_missing_session(stderr: &str) -> bool {
    let lower = stderr.to_ascii_lowercase();
    lower.contains("session") && lower.contains("not found")
}

fn stderr_has_missing_pane(stderr: &str) -> bool {
    stderr.to_ascii_lowercase().contains("pane not found")
}

fn strip_ansi(input: &str) -> String {
    let mut out = String::with_capacity(input.len());
    let mut chars = input.chars().peekable();

    while let Some(ch) = chars.next() {
        if ch != '\u{1b}' {
            out.push(ch);
            continue;
        }

        if chars.peek() == Some(&'[') {
            chars.next();
            for next in chars.by_ref() {
                if next.is_ascii_alphabetic() {
                    break;
                }
            }
        }
    }

    out
}

fn session_is_active_in_list(output: &str, session_name: &str) -> bool {
    let clean = strip_ansi(output);
    for line in clean.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() || trimmed.contains("No active zellij sessions found") {
            continue;
        }

        let Some(name) = trimmed.split_whitespace().next() else {
            continue;
        };

        if name == session_name {
            return !trimmed.contains("EXITED");
        }
    }

    false
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn detects_unsupported_pane_name_flag() {
        let stderr = "error: Found argument '--pane-name' which wasn't expected";
        assert!(stderr_has_unsupported_pane_name_flag(stderr));
    }

    #[test]
    fn parses_active_session_from_list_output() {
        let output = "session-a [Created 3s ago]\nsession-b [Created 1s ago] (EXITED - attach to resurrect)\n";
        assert!(session_is_active_in_list(output, "session-a"));
        assert!(!session_is_active_in_list(output, "session-b"));
        assert!(!session_is_active_in_list(output, "session-c"));
    }
}
