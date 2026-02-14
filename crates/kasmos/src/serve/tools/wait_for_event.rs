use crate::serve::messages::{KasmosMessage, MessageEvent};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct WaitForEventInput {
    pub wp_id: Option<String>,
    pub event: Option<MessageEvent>,
    pub timeout_seconds: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum WaitForEventStatus {
    Matched,
    Timeout,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WaitForEventOutput {
    pub ok: bool,
    pub status: WaitForEventStatus,
    pub elapsed_seconds: u64,
    pub message: Option<KasmosMessage>,
}
