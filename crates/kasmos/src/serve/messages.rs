//! Structured message parsing for agent communication.

use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_json::Value;

/// Message event kinds emitted by workers and understood by the manager.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "SCREAMING_SNAKE_CASE")]
pub enum MessageEvent {
    Started,
    Progress,
    Done,
    Error,
    ReviewPass,
    ReviewReject,
    NeedsInput,
}

/// Structured message parsed from the message log pane.
#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct KasmosMessage {
    pub message_index: u64,
    pub sender: String,
    pub event: MessageEvent,
    pub payload: Value,
    pub timestamp: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub raw_line: Option<String>,
}
