use chrono::Utc;
use regex::Regex;
use serde::{Deserialize, Serialize};
use std::sync::LazyLock;

use crate::serve::audit::AuditEvent;
use crate::serve::registry::{WorkerEntry, WorkerRole, WorkerStatus};
use crate::serve::tools::KasmosServer;

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct SpawnWorkerInput {
    pub wp_id: String,
    pub role: String,
    pub prompt: String,
    pub base_branch: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkerDto {
    pub wp_id: String,
    pub role: String,
    pub status: WorkerStatus,
    pub pane_name: String,
    pub worktree_path: Option<String>,
}

impl From<WorkerEntry> for WorkerDto {
    fn from(value: WorkerEntry) -> Self {
        Self {
            wp_id: value.wp_id,
            role: value.role.as_str().to_string(),
            status: value.status,
            pane_name: value.pane_name,
            worktree_path: value.worktree_path.map(|p| p.display().to_string()),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SpawnWorkerOutput {
    pub ok: bool,
    pub worker: WorkerDto,
}

pub async fn handle(
    input: SpawnWorkerInput,
    state: &KasmosServer,
) -> anyhow::Result<SpawnWorkerOutput> {
    validate_wp_id(&input.wp_id)?;
    if input.prompt.trim().is_empty() {
        anyhow::bail!("prompt cannot be empty");
    }

    let role = WorkerRole::parse(&input.role)?;
    let pane_name = format!("{}-{}", input.wp_id, role.as_str());

    {
        let registry = state.registry.read().await;
        if registry.get(&input.wp_id, role).is_some() {
            anyhow::bail!(
                "worker already exists for {}:{}",
                input.wp_id,
                role.as_str()
            );
        }
    }

    check_capacity(state).await?;

    let worktree_path = if role == WorkerRole::Coder {
        Some(
            state
                .runtime
                .ensure_coder_worktree(
                    &state.repo_root,
                    &state.feature_slug,
                    &input.wp_id,
                    input.base_branch.as_deref().unwrap_or("HEAD"),
                )
                .await?,
        )
    } else {
        None
    };

    if let Err(err) = state
        .runtime
        .create_worker_pane(
            &state.session_name,
            &pane_name,
            role,
            &input.prompt,
            worktree_path.as_deref(),
        )
        .await
    {
        if role == WorkerRole::Coder {
            let _ = state
                .runtime
                .cleanup_coder_worktree(&state.repo_root, &state.feature_slug, &input.wp_id)
                .await;
        }
        return Err(err);
    }

    let entry = WorkerEntry {
        wp_id: input.wp_id,
        role,
        status: WorkerStatus::Active,
        pane_name,
        worktree_path,
        prompt: input.prompt,
        created_at: Utc::now(),
    };

    {
        let mut registry = state.registry.write().await;
        registry.upsert(entry.clone());
    }

    state
        .audit_log
        .lock()
        .await
        .push(AuditEvent::spawned(&entry));

    Ok(SpawnWorkerOutput {
        ok: true,
        worker: entry.into(),
    })
}

async fn check_capacity(state: &KasmosServer) -> anyhow::Result<()> {
    let registry = state.registry.read().await;
    let active_count = registry.active_count();
    let max = state.config.agent.max_parallel_workers;
    if active_count >= max {
        anyhow::bail!(
            "capacity exceeded: active workers {active_count}, max parallel workers {max}"
        );
    }
    Ok(())
}

fn validate_wp_id(wp_id: &str) -> anyhow::Result<()> {
    static WP_ID_REGEX: LazyLock<Regex> =
        LazyLock::new(|| Regex::new(r"^WP\d+$").expect("valid wp id regex"));

    if !WP_ID_REGEX.is_match(wp_id) {
        anyhow::bail!("invalid wp_id '{wp_id}', expected format like WP07");
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::serve::registry::WorkerRole;
    use crate::serve::tools::{KasmosServer, WorkerRuntime};
    use async_trait::async_trait;
    use std::collections::HashSet;
    use std::path::{Path, PathBuf};
    use std::sync::{Arc, Mutex};

    #[derive(Default)]
    struct MockRuntime {
        created_panes: Mutex<Vec<(String, WorkerRole, Option<PathBuf>)>>,
        worktrees: Mutex<HashSet<String>>,
        fail_create_for: Mutex<HashSet<String>>,
        cleaned_worktrees: Mutex<Vec<String>>,
    }

    #[async_trait]
    impl WorkerRuntime for MockRuntime {
        async fn create_worker_pane(
            &self,
            _session_name: &str,
            pane_name: &str,
            role: WorkerRole,
            _prompt: &str,
            cwd: Option<&Path>,
        ) -> anyhow::Result<()> {
            if self.fail_create_for.lock().unwrap().contains(pane_name) {
                anyhow::bail!("simulated pane create failure");
            }

            self.created_panes.lock().unwrap().push((
                pane_name.to_string(),
                role,
                cwd.map(Path::to_path_buf),
            ));
            Ok(())
        }

        async fn close_worker_pane(
            &self,
            _session_name: &str,
            _pane_name: &str,
        ) -> anyhow::Result<()> {
            Ok(())
        }

        async fn pane_exists(&self, _session_name: &str, _pane_name: &str) -> anyhow::Result<bool> {
            Ok(true)
        }

        async fn ensure_coder_worktree(
            &self,
            _repo_root: &Path,
            feature_slug: &str,
            wp_id: &str,
            _base_branch: &str,
        ) -> anyhow::Result<PathBuf> {
            let key = format!("{feature_slug}-{wp_id}");
            self.worktrees.lock().unwrap().insert(key.clone());
            Ok(PathBuf::from(format!(".worktrees/{key}")))
        }

        async fn cleanup_coder_worktree(
            &self,
            _repo_root: &Path,
            _feature_slug: &str,
            wp_id: &str,
        ) -> anyhow::Result<()> {
            self.cleaned_worktrees
                .lock()
                .unwrap()
                .push(wp_id.to_string());
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
    async fn spawn_creates_registry_entry() {
        let runtime = Arc::new(MockRuntime::default());
        let state = make_state(runtime);

        let out = handle(
            SpawnWorkerInput {
                wp_id: "WP07".to_string(),
                role: "reviewer".to_string(),
                prompt: "review this".to_string(),
                base_branch: None,
            },
            &state,
        )
        .await
        .unwrap();

        assert!(out.ok);
        assert_eq!(out.worker.pane_name, "WP07-reviewer");
        assert_eq!(state.registry.read().await.list().len(), 1);
        assert_eq!(state.audit_log.lock().await.len(), 1);
    }

    #[tokio::test]
    async fn capacity_limit_is_enforced() {
        let runtime = Arc::new(MockRuntime::default());
        let mut state = make_state(runtime);
        state.config.agent.max_parallel_workers = 1;

        handle(
            SpawnWorkerInput {
                wp_id: "WP01".to_string(),
                role: "reviewer".to_string(),
                prompt: "one".to_string(),
                base_branch: None,
            },
            &state,
        )
        .await
        .unwrap();

        let err = handle(
            SpawnWorkerInput {
                wp_id: "WP02".to_string(),
                role: "reviewer".to_string(),
                prompt: "two".to_string(),
                base_branch: None,
            },
            &state,
        )
        .await
        .unwrap_err();

        assert!(err.to_string().contains("capacity exceeded"));
    }

    #[tokio::test]
    async fn coder_gets_worktree_reviewer_does_not() {
        let runtime = Arc::new(MockRuntime::default());
        let state = make_state(runtime.clone());

        let coder = handle(
            SpawnWorkerInput {
                wp_id: "WP11".to_string(),
                role: "coder".to_string(),
                prompt: "build feature".to_string(),
                base_branch: Some("main".to_string()),
            },
            &state,
        )
        .await
        .unwrap();
        assert!(coder.worker.worktree_path.is_some());

        let reviewer = handle(
            SpawnWorkerInput {
                wp_id: "WP12".to_string(),
                role: "reviewer".to_string(),
                prompt: "review feature".to_string(),
                base_branch: None,
            },
            &state,
        )
        .await
        .unwrap();
        assert!(reviewer.worker.worktree_path.is_none());

        assert_eq!(runtime.worktrees.lock().unwrap().len(), 1);
    }

    #[tokio::test]
    async fn duplicate_spawn_fails_before_creating_second_pane() {
        let runtime = Arc::new(MockRuntime::default());
        let state = make_state(runtime.clone());

        handle(
            SpawnWorkerInput {
                wp_id: "WP13".to_string(),
                role: "reviewer".to_string(),
                prompt: "first".to_string(),
                base_branch: None,
            },
            &state,
        )
        .await
        .unwrap();

        let err = handle(
            SpawnWorkerInput {
                wp_id: "WP13".to_string(),
                role: "reviewer".to_string(),
                prompt: "second".to_string(),
                base_branch: None,
            },
            &state,
        )
        .await
        .unwrap_err();

        assert!(err.to_string().contains("worker already exists"));
        assert_eq!(runtime.created_panes.lock().unwrap().len(), 1);
    }

    #[tokio::test]
    async fn failed_coder_pane_creation_cleans_up_worktree() {
        let runtime = Arc::new(MockRuntime::default());
        runtime
            .fail_create_for
            .lock()
            .unwrap()
            .insert("WP14-coder".to_string());
        let state = make_state(runtime.clone());

        let err = handle(
            SpawnWorkerInput {
                wp_id: "WP14".to_string(),
                role: "coder".to_string(),
                prompt: "build".to_string(),
                base_branch: Some("main".to_string()),
            },
            &state,
        )
        .await
        .unwrap_err();

        assert!(err.to_string().contains("simulated pane create failure"));
        assert!(state.registry.read().await.list().is_empty());
        assert_eq!(
            runtime.cleaned_worktrees.lock().unwrap().as_slice(),
            ["WP14"]
        );
    }
}
