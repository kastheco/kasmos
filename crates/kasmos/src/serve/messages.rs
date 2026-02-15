//! Structured message parsing for agent communication.

use anyhow::{Context, Result, anyhow};
use regex::Regex;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use serde_json::Value;
use shell_escape::unix::escape as shell_escape;
use std::borrow::Cow;
use std::sync::LazyLock;
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};
use tokio::process::Command;

const MSG_LOG_PANE: &str = "msg-log";

static MESSAGE_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"\[KASMOS:([^:\]]+):([^\]]+)\]\s*(.*)").expect("valid regex"));
static ANSI_PATTERN: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"\x1B\[[0-9;?]*[ -/]*[@-~]").expect("valid regex"));
static DEGRADED_WARNING_EMITTED: AtomicBool = AtomicBool::new(false);
static DUMP_PANE_ATTEMPT_HINT: AtomicU8 = AtomicU8::new(u8::MAX);
static RUN_IN_PANE_ATTEMPT_HINT: AtomicU8 = AtomicU8::new(u8::MAX);

/// Event values currently recognized by the protocol.
const KNOWN_EVENTS: &[&str] = &[
    "STARTED",
    "PROGRESS",
    "DONE",
    "ERROR",
    "REVIEW_PASS",
    "REVIEW_REJECT",
    "NEEDS_INPUT",
];

/// Message event kinds emitted by workers and understood by the manager.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(transparent)]
pub struct MessageEvent(pub String);

impl MessageEvent {
    pub fn new(value: impl Into<String>) -> Self {
        Self(value.into())
    }

    pub fn as_str(&self) -> &str {
        &self.0
    }

    pub fn is_known(&self) -> bool {
        KNOWN_EVENTS.contains(&self.0.as_str())
    }
}

/// Structured message parsed from the message log pane.
#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct KasmosMessage {
    pub message_index: u64,
    pub sender: String,
    pub event: MessageEvent,
    pub known_event: bool,
    pub payload: Value,
    pub timestamp: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub raw_line: Option<String>,
}

#[derive(Debug, Clone)]
pub struct MessageRead {
    pub messages: Vec<KasmosMessage>,
    pub degraded_mode: bool,
}

pub fn parse_message(line: &str, index: u64) -> Option<KasmosMessage> {
    let clean = strip_ansi(line);
    let captures = MESSAGE_PATTERN.captures(&clean)?;

    let sender = captures.get(1)?.as_str().trim().to_string();
    let event_raw = captures.get(2)?.as_str().trim().to_string();
    let payload_str = captures.get(3)?.as_str().trim();

    let payload = if payload_str.is_empty() {
        Value::Null
    } else {
        serde_json::from_str(payload_str).unwrap_or(Value::Null)
    };

    let event = MessageEvent::new(event_raw);
    Some(KasmosMessage {
        message_index: index,
        sender,
        known_event: event.is_known(),
        event,
        payload,
        timestamp: chrono::Utc::now().to_rfc3339(),
        raw_line: Some(line.to_string()),
    })
}

pub fn parse_scrollback(scrollback: &str) -> Vec<KasmosMessage> {
    let mut next_index = 0_u64;
    let mut messages = Vec::new();
    for line in scrollback.lines() {
        if let Some(message) = parse_message(line, next_index) {
            messages.push(message);
            next_index = next_index.saturating_add(1);
        }
    }
    messages
}

pub fn message_targets_wp(message: &KasmosMessage, wp_id: &str) -> bool {
    message.sender == wp_id
        || message
            .payload
            .get("wp_id")
            .and_then(|value| value.as_str())
            == Some(wp_id)
}

pub async fn read_messages_since(since_index: u64) -> Result<MessageRead> {
    let pane = read_pane_scrollback(MSG_LOG_PANE).await?;
    let messages = parse_scrollback(&pane.scrollback)
        .into_iter()
        .filter(|message| message.message_index >= since_index)
        .collect();

    Ok(MessageRead {
        messages,
        degraded_mode: pane.degraded_mode,
    })
}

pub async fn log_manager_event(event: &str, payload: &Value) -> Result<()> {
    let event = event.trim().to_ascii_uppercase();
    let payload = serde_json::to_string(payload).context("serialize manager event payload")?;
    let message = format!("[KASMOS:manager:{event}] {payload}");
    write_to_pane(MSG_LOG_PANE, &message, false).await
}

pub async fn rewrite_dashboard(content: &str) -> Result<()> {
    write_to_pane("dashboard", content, true).await
}

fn strip_ansi(line: &str) -> String {
    ANSI_PATTERN.replace_all(line, "").into_owned()
}

#[derive(Debug, Clone)]
struct PaneScrollback {
    scrollback: String,
    degraded_mode: bool,
}

async fn read_pane_scrollback(pane_name: &str) -> Result<PaneScrollback> {
    match try_pane_tracker_dump(pane_name).await {
        Ok(scrollback) => Ok(PaneScrollback {
            scrollback,
            degraded_mode: false,
        }),
        Err(error) => {
            if !DEGRADED_WARNING_EMITTED.swap(true, Ordering::Relaxed) {
                tracing::warn!(
                    "pane-tracker unavailable: {error}. falling back to degraded scrollback mode"
                );
            }
            let scrollback = direct_scrollback_read().await?;
            Ok(PaneScrollback {
                scrollback,
                degraded_mode: true,
            })
        }
    }
}

async fn try_pane_tracker_dump(pane_name: &str) -> Result<String> {
    let binary =
        pane_tracker_binary().ok_or_else(|| anyhow!("pane-tracker binary not found in PATH"))?;

    let mut last_error = None;
    for attempt in preferred_attempt_order(4, DUMP_PANE_ATTEMPT_HINT.load(Ordering::Relaxed)) {
        let args = dump_pane_args(attempt, pane_name);
        match run_command_capture(binary, &args).await {
            Ok(output) => {
                DUMP_PANE_ATTEMPT_HINT.store(attempt, Ordering::Relaxed);
                return Ok(output);
            }
            Err(error) => last_error = Some(error),
        }
    }

    Err(last_error.unwrap_or_else(|| anyhow!("dump-pane failed with unknown error")))
}

async fn write_to_pane(pane_name: &str, content: &str, rewrite: bool) -> Result<()> {
    let binary =
        pane_tracker_binary().ok_or_else(|| anyhow!("pane-tracker binary not found in PATH"))?;

    let escaped = shell_escape(Cow::Borrowed(content)).to_string();
    let command = if rewrite {
        format!("clear && printf '%s\\n' {escaped}")
    } else {
        format!("printf '%s\\n' {escaped}")
    };

    let mut last_error = None;
    for attempt in preferred_attempt_order(3, RUN_IN_PANE_ATTEMPT_HINT.load(Ordering::Relaxed)) {
        let args = run_in_pane_args(attempt, pane_name, &command);
        match run_command_capture(binary, &args).await {
            Ok(_) => {
                RUN_IN_PANE_ATTEMPT_HINT.store(attempt, Ordering::Relaxed);
                return Ok(());
            }
            Err(error) => last_error = Some(error),
        }
    }

    Err(last_error.unwrap_or_else(|| anyhow!("run-in-pane failed with unknown error")))
}

async fn direct_scrollback_read() -> Result<String> {
    let dump_path = std::env::temp_dir().join(format!(
        "kasmos-scrollback-{}-{}.txt",
        std::process::id(),
        std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|duration| duration.as_nanos())
            .unwrap_or_default()
    ));
    let dump_path_arg = dump_path.to_string_lossy().into_owned();

    run_command_capture("zellij", &["action", "dump-screen", dump_path_arg.as_str()]).await?;

    let content = tokio::fs::read_to_string(&dump_path)
        .await
        .with_context(|| {
            format!(
                "failed to read dump-screen output at {}",
                dump_path.display()
            )
        });
    let _ = tokio::fs::remove_file(&dump_path).await;
    content
}

fn dump_pane_args(attempt: u8, pane_name: &str) -> Vec<&str> {
    match attempt {
        0 => vec!["dump-pane", "--pane-name", pane_name],
        1 => vec!["dump-pane", "--pane", pane_name],
        2 => vec!["dump-pane", "--name", pane_name],
        _ => vec!["dump-pane", pane_name],
    }
}

fn run_in_pane_args<'a>(attempt: u8, pane_name: &'a str, command: &'a str) -> Vec<&'a str> {
    match attempt {
        0 => vec![
            "run-in-pane",
            "--pane-name",
            pane_name,
            "--command",
            command,
        ],
        1 => vec!["run-in-pane", "--pane", pane_name, "--command", command],
        _ => vec!["run-in-pane", pane_name, command],
    }
}

fn preferred_attempt_order(total: u8, hint: u8) -> Vec<u8> {
    let mut order = (0..total).collect::<Vec<_>>();
    if hint < total {
        order.retain(|attempt| *attempt != hint);
        order.insert(0, hint);
    }
    order
}

async fn run_command_capture(binary: &str, args: &[&str]) -> Result<String> {
    let output = Command::new(binary)
        .args(args)
        .output()
        .await
        .with_context(|| format!("failed to execute `{binary} {}`", args.join(" ")))?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        return Err(anyhow!(
            "command `{binary} {}` failed: {}",
            args.join(" "),
            stderr.trim()
        ));
    }

    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

fn pane_tracker_binary() -> Option<&'static str> {
    ["pane-tracker", "zellij-pane-tracker"]
        .into_iter()
        .find(|binary| which::which(binary).is_ok())
}

#[cfg(test)]
mod tests {
    use super::*;
    use serde_json::json;

    #[test]
    fn parse_valid_message_line() {
        let line = r#"[KASMOS:WP08:STARTED] {"wp_id":"WP08"}"#;
        let parsed = parse_message(line, 5).expect("must parse");

        assert_eq!(parsed.message_index, 5);
        assert_eq!(parsed.sender, "WP08");
        assert_eq!(parsed.event.as_str(), "STARTED");
        assert!(parsed.known_event);
        assert_eq!(parsed.payload["wp_id"], "WP08");
    }

    #[test]
    fn parse_ignores_non_kasmos_lines() {
        assert!(parse_message("normal terminal output", 0).is_none());
    }

    #[test]
    fn parse_strips_ansi_before_matching() {
        let line = "\x1b[32m[KASMOS:worker:PROGRESS]\x1b[0m {\"pct\":50}";
        let parsed = parse_message(line, 1).expect("must parse");

        assert_eq!(parsed.sender, "worker");
        assert_eq!(parsed.event.as_str(), "PROGRESS");
        assert_eq!(parsed.payload["pct"], 50);
    }

    #[test]
    fn malformed_payload_becomes_null() {
        let line = "[KASMOS:worker:PROGRESS] {";
        let parsed = parse_message(line, 2).expect("must parse");
        assert_eq!(parsed.payload, Value::Null);
    }

    #[test]
    fn unknown_event_is_preserved_and_flagged() {
        let line = r#"[KASMOS:worker:SOMETHING_NEW] {"x":1}"#;
        let parsed = parse_message(line, 3).expect("must parse");

        assert_eq!(parsed.event.as_str(), "SOMETHING_NEW");
        assert!(!parsed.known_event);
    }

    #[test]
    fn wp_target_match_allows_sender_or_payload() {
        let sender_match = KasmosMessage {
            message_index: 1,
            sender: "WP08".to_string(),
            event: MessageEvent::new("PROGRESS"),
            known_event: true,
            payload: json!({"detail":"no wp_id in payload"}),
            timestamp: "2026-01-01T00:00:00Z".to_string(),
            raw_line: None,
        };
        let payload_match = KasmosMessage {
            sender: "worker".to_string(),
            payload: json!({"wp_id":"WP08"}),
            ..sender_match.clone()
        };

        assert!(message_targets_wp(&sender_match, "WP08"));
        assert!(message_targets_wp(&payload_match, "WP08"));
        assert!(!message_targets_wp(&sender_match, "WP07"));
    }

    #[test]
    fn preferred_order_uses_hint_first() {
        assert_eq!(preferred_attempt_order(4, 2), vec![2, 0, 1, 3]);
    }

    #[test]
    fn preferred_order_ignores_invalid_hint() {
        assert_eq!(preferred_attempt_order(3, u8::MAX), vec![0, 1, 2]);
    }
}
