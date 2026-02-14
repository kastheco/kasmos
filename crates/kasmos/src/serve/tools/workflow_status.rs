use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct WorkflowStatusInput {
    pub feature_slug: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WorkflowStatusOutput {
    pub ok: bool,
    pub snapshot: WorkflowSnapshot,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WorkflowSnapshot {
    pub feature_slug: String,
    pub phase: String,
    pub waves: Vec<WaveInfo>,
    pub lock: LockInfo,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WaveInfo {
    pub wave: u64,
    pub wp_ids: Vec<String>,
    pub complete: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct LockInfo {
    pub state: LockState,
    pub owner_id: Option<String>,
    pub expires_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum LockState {
    Active,
    Stale,
    None,
}
