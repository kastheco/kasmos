use serde::{Deserialize, Serialize};
use tracing::warn;

use crate::serve::audit::AuditEvent;
use crate::serve::registry::WorkerRole;
use crate::serve::tools::KasmosServer;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct DespawnWorkerInput {
    pub wp_id: String,
    pub role: String,
    pub reason: Option<String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct DespawnWorkerOutput {
    pub ok: bool,
    pub removed: bool,
}

pub async fn handle(
    input: DespawnWorkerInput,
    state: &KasmosServer,
) -> anyhow::Result<DespawnWorkerOutput> {
    let role = WorkerRole::parse(&input.role)?;

    let worker = {
        let registry = state.registry.read().await;
        registry.get(&input.wp_id, role).cloned()
    }
    .ok_or_else(|| anyhow::anyhow!("WORKER_NOT_FOUND: {}:{}", input.wp_id, input.role))?;

    let _ = state
        .runtime
        .close_worker_pane(&state.session_name, &worker.pane_name)
        .await;

    let cleanup_error = if worker.role == WorkerRole::Coder && worker.worktree_path.is_some() {
        state
            .runtime
            .cleanup_coder_worktree(&state.repo_root, &state.feature_slug, &worker.wp_id)
            .await
            .err()
    } else {
        None
    };

    if worker.role == WorkerRole::Coder
        && worker.worktree_path.is_some()
        && let Some(err) = cleanup_error
    {
        warn!(wp_id = %worker.wp_id, error = %err, "failed to cleanup coder worktree during despawn");
    }

    {
        let mut registry = state.registry.write().await;
        registry.remove(&input.wp_id, role);
    }

    state
        .audit_log
        .lock()
        .await
        .push(AuditEvent::despawned(&worker, input.reason.as_deref()));

    Ok(DespawnWorkerOutput {
        ok: true,
        removed: true,
    })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::serve::registry::{WorkerEntry, WorkerRole, WorkerStatus};
    use crate::serve::tools::{KasmosServer, WorkerRuntime};
    use async_trait::async_trait;
    use chrono::Utc;
    use std::path::{Path, PathBuf};
    use std::sync::{Arc, Mutex};

    #[derive(Default)]
    struct MockRuntime {
        closed: Mutex<Vec<String>>,
        cleaned: Mutex<Vec<String>>,
        fail_cleanup: bool,
    }

    #[async_trait]
    impl WorkerRuntime for MockRuntime {
        async fn create_worker_pane(
            &self,
            _session_name: &str,
            _pane_name: &str,
            _role: WorkerRole,
            _prompt: &str,
            _cwd: Option<&Path>,
        ) -> anyhow::Result<()> {
            Ok(())
        }

        async fn close_worker_pane(
            &self,
            _session_name: &str,
            pane_name: &str,
        ) -> anyhow::Result<()> {
            self.closed.lock().unwrap().push(pane_name.to_string());
            Ok(())
        }

        async fn pane_exists(&self, _session_name: &str, _pane_name: &str) -> anyhow::Result<bool> {
            Ok(true)
        }

        async fn ensure_coder_worktree(
            &self,
            _repo_root: &Path,
            _feature_slug: &str,
            _wp_id: &str,
            _base_branch: &str,
        ) -> anyhow::Result<PathBuf> {
            Ok(PathBuf::from(".worktrees/test"))
        }

        async fn cleanup_coder_worktree(
            &self,
            _repo_root: &Path,
            _feature_slug: &str,
            wp_id: &str,
        ) -> anyhow::Result<()> {
            self.cleaned.lock().unwrap().push(wp_id.to_string());

            if self.fail_cleanup {
                anyhow::bail!("simulated cleanup failure");
            }

            Ok(())
        }
    }

    fn make_state(runtime: Arc<dyn WorkerRuntime>) -> KasmosServer {
        KasmosServer::new(
            Config::default(),
            std::env::current_dir().unwrap(),
            "011-mcp-agent-swarm-orchestration".to_string(),
            "kasmos-test".to_string(),
            runtime,
        )
    }

    #[tokio::test]
    async fn despawn_removes_entry() {
        let runtime = Arc::new(MockRuntime::default());
        let state = make_state(runtime.clone());

        state.registry.write().await.upsert(WorkerEntry {
            wp_id: "WP07".to_string(),
            role: WorkerRole::Reviewer,
            status: WorkerStatus::Active,
            pane_name: "WP07-reviewer".to_string(),
            worktree_path: None,
            prompt: "review".to_string(),
            created_at: Utc::now(),
        });

        let out = handle(
            DespawnWorkerInput {
                wp_id: "WP07".to_string(),
                role: "reviewer".to_string(),
                reason: Some("done".to_string()),
            },
            &state,
        )
        .await
        .unwrap();

        assert!(out.removed);
        assert!(state.registry.read().await.list().is_empty());
        assert_eq!(runtime.closed.lock().unwrap().len(), 1);
        assert_eq!(state.audit_log.lock().await.len(), 1);
    }

    #[tokio::test]
    async fn despawn_coder_cleanup_failure_still_removes_entry() {
        let runtime = Arc::new(MockRuntime {
            fail_cleanup: true,
            ..MockRuntime::default()
        });
        let state = make_state(runtime.clone());

        state.registry.write().await.upsert(WorkerEntry {
            wp_id: "WP08".to_string(),
            role: WorkerRole::Coder,
            status: WorkerStatus::Active,
            pane_name: "WP08-coder".to_string(),
            worktree_path: Some(PathBuf::from(".worktrees/011-WP08")),
            prompt: "implement".to_string(),
            created_at: Utc::now(),
        });

        let out = handle(
            DespawnWorkerInput {
                wp_id: "WP08".to_string(),
                role: "coder".to_string(),
                reason: Some("cleanup test".to_string()),
            },
            &state,
        )
        .await
        .unwrap();

        assert!(out.removed);
        assert!(state.registry.read().await.list().is_empty());
        assert_eq!(runtime.cleaned.lock().unwrap().as_slice(), ["WP08"]);
        assert_eq!(state.audit_log.lock().await.len(), 1);
    }

    #[tokio::test]
    async fn despawn_missing_worker_returns_error() {
        let runtime = Arc::new(MockRuntime::default());
        let state = make_state(runtime);

        let err = handle(
            DespawnWorkerInput {
                wp_id: "WP99".to_string(),
                role: "reviewer".to_string(),
                reason: None,
            },
            &state,
        )
        .await
        .unwrap_err();

        assert!(err.to_string().contains("WORKER_NOT_FOUND"));
    }
}
