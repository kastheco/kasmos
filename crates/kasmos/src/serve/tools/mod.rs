//! MCP worker lifecycle tool implementations.

pub mod despawn_worker;
pub mod list_workers;
pub mod spawn_worker;

use async_trait::async_trait;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use tokio::process::Command;
use tokio::sync::{Mutex, RwLock};

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
        // Name-based close support varies by zellij version; this is best effort.
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

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            anyhow::bail!("failed to close pane '{pane_name}': {stderr}");
        }

        Ok(())
    }

    async fn pane_exists(&self, session_name: &str, pane_name: &str) -> anyhow::Result<bool> {
        let output = Command::new(&self.zellij_binary)
            .args([
                "--session",
                session_name,
                "action",
                "dump-screen",
                "--pane-name",
                pane_name,
            ])
            .output()
            .await?;

        Ok(output.status.success())
    }

    async fn ensure_coder_worktree(
        &self,
        repo_root: &Path,
        feature_slug: &str,
        wp_id: &str,
        base_branch: &str,
    ) -> anyhow::Result<PathBuf> {
        let manager = WorktreeManager::new(repo_root, feature_slug)?;
        manager.ensure_worktree(wp_id, base_branch)
    }

    async fn cleanup_coder_worktree(
        &self,
        repo_root: &Path,
        feature_slug: &str,
        wp_id: &str,
    ) -> anyhow::Result<()> {
        let manager = WorktreeManager::new(repo_root, feature_slug)?;
        manager.remove_worktree(wp_id)
    }
}
