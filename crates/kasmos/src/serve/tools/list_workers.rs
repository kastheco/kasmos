use crate::serve::{
    KasmosServer,
    registry::{WorkerEntry, WorkerStatus},
};
use anyhow::Result;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct ListWorkersInput {
    pub status: Option<WorkerStatus>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct ListWorkersOutput {
    pub ok: bool,
    pub workers: Vec<WorkerEntry>,
}

pub async fn handle(server: &KasmosServer, input: ListWorkersInput) -> Result<ListWorkersOutput> {
    let registry = server.registry.read().await;
    let workers = registry
        .list()
        .filter(|entry| input.status.is_none_or(|status| entry.status == status))
        .cloned()
        .collect();

    Ok(ListWorkersOutput { ok: true, workers })
}
