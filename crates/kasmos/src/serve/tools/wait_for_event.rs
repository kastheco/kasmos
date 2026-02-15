use crate::serve::{
    KasmosServer,
    messages::{KasmosMessage, MessageEvent, read_messages_since, rewrite_dashboard},
    registry::WorkerEntry,
};
use anyhow::Result;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::time::Duration;
use tokio::time::Instant;

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

pub async fn handle(server: &KasmosServer, input: WaitForEventInput) -> Result<WaitForEventOutput> {
    let poll_interval = Duration::from_secs(server.config.communication.poll_interval_secs);
    let timeout = Duration::from_secs(input.timeout_seconds);
    let mut cursor = *server.message_cursor.read().await;
    let start = Instant::now();

    loop {
        let elapsed = start.elapsed();
        if elapsed >= timeout {
            return Ok(WaitForEventOutput {
                ok: true,
                status: WaitForEventStatus::Timeout,
                elapsed_seconds: elapsed.as_secs(),
                message: None,
            });
        }

        let read = read_messages_since(cursor).await?;

        if let Err(error) = update_dashboard(server).await {
            tracing::warn!("failed to update dashboard pane: {error}");
        }

        for message in &read.messages {
            if message_matches(message, &input) {
                let next_index = message.message_index.saturating_add(1);
                *server.message_cursor.write().await = next_index;
                return Ok(WaitForEventOutput {
                    ok: true,
                    status: WaitForEventStatus::Matched,
                    elapsed_seconds: start.elapsed().as_secs(),
                    message: Some(message.clone()),
                });
            }
        }

        if let Some(last) = read.messages.last() {
            cursor = last.message_index.saturating_add(1);
            *server.message_cursor.write().await = cursor;
        }

        let multiplier = if read.degraded_mode { 2_u32 } else { 1_u32 };
        let sleep_for = poll_interval.saturating_mul(multiplier);
        let remaining = timeout.saturating_sub(start.elapsed());
        tokio::time::sleep(std::cmp::min(sleep_for, remaining)).await;
    }
}

fn message_matches(message: &KasmosMessage, input: &WaitForEventInput) -> bool {
    if let Some(wp_id) = input.wp_id.as_deref()
        && message
            .payload
            .get("wp_id")
            .and_then(|value| value.as_str())
            != Some(wp_id)
    {
        return false;
    }

    if let Some(event) = &input.event
        && &message.event != event
    {
        return false;
    }

    true
}

async fn update_dashboard(server: &KasmosServer) -> Result<()> {
    let workers = server
        .registry
        .read()
        .await
        .list()
        .cloned()
        .collect::<Vec<_>>();
    let dashboard = format_worker_table(&workers);
    rewrite_dashboard(&dashboard).await
}

fn format_worker_table(workers: &[WorkerEntry]) -> String {
    let mut lines = vec!["WP ID | Role | Status | Elapsed".to_string()];
    lines.push("----- | ---- | ------ | -------".to_string());

    let now = chrono::Utc::now();
    for worker in workers {
        let elapsed = chrono::DateTime::parse_from_rfc3339(&worker.spawned_at)
            .map(|started| {
                now.signed_duration_since(started.with_timezone(&chrono::Utc))
                    .num_seconds()
                    .max(0)
            })
            .unwrap_or(0);

        lines.push(format!(
            "{} | {} | {:?} | {}s",
            worker.wp_id,
            worker.role.as_str(),
            worker.status,
            elapsed
        ));
    }

    lines.join("\n")
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    fn message(event: &str, wp_id: &str) -> KasmosMessage {
        KasmosMessage {
            message_index: 0,
            sender: "worker".to_string(),
            event: MessageEvent::new(event),
            known_event: true,
            payload: json!({ "wp_id": wp_id }),
            timestamp: "2026-02-14T00:00:00Z".to_string(),
            raw_line: None,
        }
    }

    #[test]
    fn message_matching_honors_both_filters() {
        let input = WaitForEventInput {
            wp_id: Some("WP08".to_string()),
            event: Some(MessageEvent::new("DONE")),
            timeout_seconds: 1,
        };

        assert!(message_matches(&message("DONE", "WP08"), &input));
        assert!(!message_matches(&message("PROGRESS", "WP08"), &input));
        assert!(!message_matches(&message("DONE", "WP07"), &input));
    }

    #[test]
    fn table_formatter_includes_expected_columns() {
        let table = format_worker_table(&[]);
        assert!(table.contains("WP ID | Role | Status | Elapsed"));
    }
}
