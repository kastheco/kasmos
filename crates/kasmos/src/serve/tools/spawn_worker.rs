use crate::serve::{
    KasmosServer,
    audit::AuditEntry,
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
    drop(registry);

    let audit_entry = AuditEntry::new("manager", "spawn_worker", &input.feature_slug)
        .with_wp_id(worker.wp_id.clone())
        .with_status("ok")
        .with_summary("worker spawned")
        .with_details(serde_json::json!({
            "role": worker.role.as_str(),
            "pane_name": worker.pane_name.clone(),
            "status": "active",
        }))
        .with_debug_payload(
            serde_json::json!({
                "prompt": input.prompt,
                "worktree_path": worker.worktree_path.clone(),
            }),
            server.config.audit.debug_full_payload,
        );
    server.emit_audit(audit_entry).await;

    if let Err(err) = log_manager_event(
        "SPAWN",
        &serde_json::json!({
            "wp_id": worker.wp_id.clone(),
            "role": worker.role.as_str(),
            "pane_name": worker.pane_name.clone(),
        }),
    )
    .await
    {
        tracing::warn!(
            wp_id = %worker.wp_id,
            error = %err,
            "failed to log SPAWN event to message pane"
        );
    }

    Ok(SpawnWorkerOutput { ok: true, worker })
}
