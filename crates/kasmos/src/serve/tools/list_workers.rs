use serde::{Deserialize, Serialize};

use crate::serve::audit::AuditEvent;
use crate::serve::registry::WorkerStatus;
use crate::serve::tools::KasmosServer;
use crate::serve::tools::spawn_worker::WorkerDto;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ListWorkersInput {
    pub status: Option<WorkerStatus>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ListWorkersOutput {
    pub ok: bool,
    pub workers: Vec<WorkerDto>,
}

pub async fn handle(
    input: ListWorkersInput,
    state: &KasmosServer,
) -> anyhow::Result<ListWorkersOutput> {
    let candidates = state.registry.read().await.list();

    for worker in &candidates {
        if worker.status != WorkerStatus::Active {
            continue;
        }

        let is_alive = state
            .runtime
            .pane_exists(&state.session_name, &worker.pane_name)
            .await
            .unwrap_or(false);

        if !is_alive {
            let mut registry = state.registry.write().await;
            if let Some(entry) = registry.get_mut(&worker.wp_id, worker.role)
                && entry.status == WorkerStatus::Active
            {
                entry.status = WorkerStatus::Aborted;
                state.audit_log.lock().await.push(AuditEvent::aborted(
                    entry,
                    Some("pane missing during reconciliation"),
                ));
            }
        }
    }

    let workers = state
        .registry
        .read()
        .await
        .list()
        .into_iter()
        .filter(|w| input.status.is_none_or(|s| s == w.status))
        .map(WorkerDto::from)
        .collect();

    Ok(ListWorkersOutput { ok: true, workers })
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::serve::registry::{WorkerEntry, WorkerRole};
    use crate::serve::tools::{KasmosServer, WorkerRuntime};
    use async_trait::async_trait;
    use chrono::Utc;
    use std::collections::HashSet;
    use std::path::{Path, PathBuf};
    use std::sync::{Arc, Mutex};

    struct MockRuntime {
        alive: Mutex<HashSet<String>>,
    }

    impl MockRuntime {
        fn new(alive: &[&str]) -> Self {
            Self {
                alive: Mutex::new(alive.iter().map(|p| p.to_string()).collect()),
            }
        }
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
            _pane_name: &str,
        ) -> anyhow::Result<()> {
            Ok(())
        }

        async fn pane_exists(&self, _session_name: &str, pane_name: &str) -> anyhow::Result<bool> {
            Ok(self.alive.lock().unwrap().contains(pane_name))
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
            _wp_id: &str,
        ) -> anyhow::Result<()> {
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
    async fn list_returns_workers_and_filters_by_status() {
        let runtime = Arc::new(MockRuntime::new(&["WP01-coder"]));
        let state = make_state(runtime);

        state.registry.write().await.upsert(WorkerEntry {
            wp_id: "WP01".to_string(),
            role: WorkerRole::Coder,
            status: WorkerStatus::Active,
            pane_name: "WP01-coder".to_string(),
            worktree_path: Some(PathBuf::from(".worktrees/011-WP01")),
            prompt: "implement".to_string(),
            created_at: Utc::now(),
        });
        state.registry.write().await.upsert(WorkerEntry {
            wp_id: "WP02".to_string(),
            role: WorkerRole::Reviewer,
            status: WorkerStatus::Aborted,
            pane_name: "WP02-reviewer".to_string(),
            worktree_path: None,
            prompt: "review".to_string(),
            created_at: Utc::now(),
        });

        let all = handle(ListWorkersInput { status: None }, &state)
            .await
            .unwrap();
        assert_eq!(all.workers.len(), 2);

        let only_aborted = handle(
            ListWorkersInput {
                status: Some(WorkerStatus::Aborted),
            },
            &state,
        )
        .await
        .unwrap();
        assert_eq!(only_aborted.workers.len(), 1);
        assert_eq!(only_aborted.workers[0].wp_id, "WP02");
    }

    #[tokio::test]
    async fn reconciliation_marks_missing_panes_aborted() {
        let runtime = Arc::new(MockRuntime::new(&[]));
        let state = make_state(runtime);

        state.registry.write().await.upsert(WorkerEntry {
            wp_id: "WP03".to_string(),
            role: WorkerRole::Reviewer,
            status: WorkerStatus::Active,
            pane_name: "WP03-reviewer".to_string(),
            worktree_path: None,
            prompt: "review".to_string(),
            created_at: Utc::now(),
        });

        let out = handle(ListWorkersInput { status: None }, &state)
            .await
            .unwrap();
        assert_eq!(out.workers.len(), 1);
        assert_eq!(out.workers[0].status, WorkerStatus::Aborted);
    }
}
