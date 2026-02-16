use crate::serve::{
    KasmosServer,
    messages::{KasmosMessage, MessageEvent, message_targets_wp, read_messages_since},
};
use anyhow::Result;
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

pub async fn handle(
    _server: &KasmosServer,
    input: ReadMessagesInput,
) -> Result<ReadMessagesOutput> {
    let since_index = input.since_index.unwrap_or(0);
    let read = read_messages_since(since_index).await?;
    let messages = filter_messages(
        read.messages,
        input.filter_wp.as_deref(),
        input.filter_event,
    );
    let next_index = messages
        .last()
        .map(|message| message.message_index.saturating_add(1))
        .unwrap_or(since_index);

    Ok(ReadMessagesOutput {
        ok: true,
        messages,
        next_index,
    })
}

pub(crate) fn filter_messages(
    messages: Vec<KasmosMessage>,
    wp_filter: Option<&str>,
    event_filter: Option<MessageEvent>,
) -> Vec<KasmosMessage> {
    messages
        .into_iter()
        .filter(|message| wp_filter.is_none_or(|wp| message_targets_wp(message, wp)))
        .filter(|message| {
            event_filter
                .as_ref()
                .is_none_or(|event| message.event == *event)
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn message(index: u64, sender: &str, payload: serde_json::Value, event: &str) -> KasmosMessage {
        KasmosMessage {
            message_index: index,
            sender: sender.to_string(),
            event: MessageEvent::new(event),
            known_event: true,
            payload,
            timestamp: "2026-02-14T00:00:00Z".to_string(),
            raw_line: None,
        }
    }

    #[test]
    fn cursor_filter_prevents_duplicates() {
        let all = vec![
            message(0, "worker", json!({ "wp_id": "WP01" }), "STARTED"),
            message(1, "worker", json!({ "wp_id": "WP01" }), "PROGRESS"),
            message(2, "worker", json!({ "wp_id": "WP01" }), "DONE"),
        ];
        let new_only = all
            .into_iter()
            .filter(|msg| msg.message_index >= 2)
            .collect::<Vec<_>>();

        assert_eq!(new_only.len(), 1);
        assert_eq!(new_only[0].message_index, 2);
    }

    #[test]
    fn combines_wp_and_event_filters() {
        let filtered = filter_messages(
            vec![
                message(0, "worker", json!({ "wp_id": "WP01" }), "PROGRESS"),
                message(1, "worker", json!({ "wp_id": "WP02" }), "PROGRESS"),
                message(2, "worker", json!({ "wp_id": "WP01" }), "DONE"),
            ],
            Some("WP01"),
            Some(MessageEvent::new("DONE")),
        );

        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].message_index, 2);
    }

    #[test]
    fn wp_filter_matches_sender_when_payload_wp_id_missing() {
        let filtered = filter_messages(
            vec![
                message(0, "WP08", json!({ "detail": "from sender" }), "PROGRESS"),
                message(1, "worker", json!({ "wp_id": "WP09" }), "PROGRESS"),
            ],
            Some("WP08"),
            None,
        );

        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].message_index, 0);
    }
}
