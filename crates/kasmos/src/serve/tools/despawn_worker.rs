use crate::serve::{KasmosServer, registry::AgentRole};
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
    Ok(DespawnWorkerOutput { ok: true, removed })
}
