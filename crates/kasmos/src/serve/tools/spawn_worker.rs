use crate::serve::{
    KasmosServer,
    registry::{AgentRole, WorkerEntry, WorkerStatus},
};
use anyhow::Result;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct SpawnWorkerInput {
    pub wp_id: String,
    pub role: AgentRole,
    pub prompt: String,
    pub feature_slug: String,
    pub worktree_path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct SpawnWorkerOutput {
    pub ok: bool,
    pub worker: WorkerEntry,
}

pub async fn handle(server: &KasmosServer, input: SpawnWorkerInput) -> Result<SpawnWorkerOutput> {
    let now = chrono::Utc::now().to_rfc3339();
    let worker = WorkerEntry {
        wp_id: input.wp_id.clone(),
        role: input.role,
        pane_name: format!("{}-{}", input.wp_id, input.role.as_str()),
        pane_id: None,
        worktree_path: input.worktree_path,
        status: WorkerStatus::Active,
        spawned_at: now.clone(),
        updated_at: Some(now),
        last_event: None,
    };

    let mut registry = server.registry.write().await;
    registry.upsert(worker.clone());

    Ok(SpawnWorkerOutput { ok: true, worker })
}
