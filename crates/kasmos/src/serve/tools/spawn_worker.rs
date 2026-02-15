use crate::serve::{
    KasmosServer,
    messages::log_manager_event,
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
    let wp_id = input.wp_id.clone();
    let role = input.role;
    let pane_name = format!("{}-{}", wp_id, role.as_str());

    let worker = WorkerEntry {
        wp_id,
        role,
        pane_name,
        pane_id: None,
        worktree_path: input.worktree_path,
        status: WorkerStatus::Active,
        spawned_at: now.clone(),
        updated_at: Some(now),
        last_event: None,
    };

    let mut registry = server.registry.write().await;
    registry.upsert(worker.clone());

    let _ = log_manager_event(
        "SPAWN",
        &serde_json::json!({
            "wp_id": worker.wp_id.clone(),
            "role": worker.role.as_str(),
            "pane_name": worker.pane_name.clone(),
        }),
    )
    .await;

    Ok(SpawnWorkerOutput { ok: true, worker })
}
