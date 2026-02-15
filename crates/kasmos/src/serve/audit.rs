//! Audit logging for worker lifecycle MCP tools.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::serve::registry::{WorkerEntry, WorkerRole};

/// Worker lifecycle audit event kind.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuditEventKind {
    WorkerSpawned,
    WorkerDespawned,
    WorkerAborted,
}

/// Structured audit event payload.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct AuditEvent {
    pub timestamp: DateTime<Utc>,
    pub kind: AuditEventKind,
    pub wp_id: String,
    pub role: WorkerRole,
    pub pane_name: String,
    pub reason: Option<String>,
}

impl AuditEvent {
    pub fn spawned(entry: &WorkerEntry) -> Self {
        Self {
            timestamp: Utc::now(),
            kind: AuditEventKind::WorkerSpawned,
            wp_id: entry.wp_id.clone(),
            role: entry.role,
            pane_name: entry.pane_name.clone(),
            reason: None,
        }
    }

    pub fn despawned(entry: &WorkerEntry, reason: Option<&str>) -> Self {
        Self {
            timestamp: Utc::now(),
            kind: AuditEventKind::WorkerDespawned,
            wp_id: entry.wp_id.clone(),
            role: entry.role,
            pane_name: entry.pane_name.clone(),
            reason: reason.map(str::to_string),
        }
    }

    pub fn aborted(entry: &WorkerEntry, reason: Option<&str>) -> Self {
        Self {
            timestamp: Utc::now(),
            kind: AuditEventKind::WorkerAborted,
            wp_id: entry.wp_id.clone(),
            role: entry.role,
            pane_name: entry.pane_name.clone(),
            reason: reason.map(str::to_string),
        }
    }
}
