use crate::serve::{
    KasmosServer, audit::AuditEntry, messages::log_manager_event, registry::AgentRole,
};
use anyhow::Result;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct DespawnWorkerInput {
    pub wp_id: String,
    pub role: AgentRole,
    pub reason: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct DespawnWorkerOutput {
    pub ok: bool,
    pub removed: bool,
}

pub async fn handle(
    server: &KasmosServer,
    input: DespawnWorkerInput,
) -> Result<DespawnWorkerOutput> {
    let mut registry = server.registry.write().await;
    let removed = registry.remove(&input.wp_id, input.role).is_some();

    if let Err(err) = log_manager_event(
        "DESPAWN",
        &serde_json::json!({
            "wp_id": input.wp_id,
            "role": input.role.as_str(),
            "removed": removed,
            "reason": input.reason.as_deref(),
        }),
    )
    .await
    {
        tracing::warn!(
            wp_id = %input.wp_id,
            error = %err,
            "failed to log DESPAWN event to message pane"
        );
    }

    if let Some(feature_slug) = server.feature_slug.as_deref() {
        let status = if removed { "ok" } else { "not_found" };
        let summary = if removed {
            "worker despawned"
        } else {
            "worker not found for despawn"
        };
        let entry = AuditEntry::new("manager", "despawn_worker", feature_slug)
            .with_wp_id(input.wp_id)
            .with_status(status)
            .with_summary(summary)
            .with_details(serde_json::json!({
                "role": input.role.as_str(),
                "removed": removed,
                "reason": input.reason,
            }));
        server.emit_audit(entry).await;
    }

    Ok(DespawnWorkerOutput { ok: true, removed })
}
