use crate::serve::messages::{KasmosMessage, MessageEvent};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct ReadMessagesInput {
    pub since_index: Option<u64>,
    pub filter_wp: Option<String>,
    pub filter_event: Option<MessageEvent>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct ReadMessagesOutput {
    pub ok: bool,
    pub messages: Vec<KasmosMessage>,
    pub next_index: u64,
}
