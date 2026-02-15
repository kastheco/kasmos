//! Agent registry for tracking active agent sessions.

use crate::serve::messages::MessageEvent;
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Worker role kind from the contract.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum AgentRole {
    Planner,
    Coder,
    Reviewer,
    Release,
}

impl AgentRole {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Planner => "planner",
            Self::Coder => "coder",
            Self::Reviewer => "reviewer",
            Self::Release => "release",
        }
    }
}

/// Worker status kind from the contract.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum WorkerStatus {
    Active,
    Done,
    Errored,
    Aborted,
}

/// Tracked worker metadata.
#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WorkerEntry {
    pub wp_id: String,
    pub role: AgentRole,
    pub pane_name: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pane_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub worktree_path: Option<String>,
    pub status: WorkerStatus,
    pub spawned_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub updated_at: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_event: Option<MessageEvent>,
}

/// In-memory worker registry, keyed by `wp_id:role`.
#[derive(Debug, Default)]
pub struct WorkerRegistry {
    workers: HashMap<String, WorkerEntry>,
}

impl WorkerRegistry {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn upsert(&mut self, worker: WorkerEntry) {
        self.workers
            .insert(Self::key(&worker.wp_id, worker.role), worker);
    }

    pub fn remove(&mut self, wp_id: &str, role: AgentRole) -> Option<WorkerEntry> {
        self.workers.remove(&Self::key(wp_id, role))
    }

    pub fn list(&self) -> impl Iterator<Item = &WorkerEntry> {
        self.workers.values()
    }

    fn key(wp_id: &str, role: AgentRole) -> String {
        format!("{}:{}", wp_id, role.as_str())
    }
}
