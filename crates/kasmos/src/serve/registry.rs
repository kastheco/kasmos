//! Worker registry for MCP worker lifecycle tools.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::PathBuf;

/// Role of a worker pane.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WorkerRole {
    Manager,
    Controller,
    Coder,
    Reviewer,
    Release,
    Explore,
}

impl WorkerRole {
    /// Parse a role from user input.
    pub fn parse(raw: &str) -> anyhow::Result<Self> {
        match raw {
            "manager" => Ok(Self::Manager),
            "controller" => Ok(Self::Controller),
            "coder" => Ok(Self::Coder),
            "reviewer" => Ok(Self::Reviewer),
            "release" => Ok(Self::Release),
            "explore" => Ok(Self::Explore),
            _ => anyhow::bail!("invalid role '{raw}'"),
        }
    }

    pub fn as_str(self) -> &'static str {
        match self {
            Self::Manager => "manager",
            Self::Controller => "controller",
            Self::Coder => "coder",
            Self::Reviewer => "reviewer",
            Self::Release => "release",
            Self::Explore => "explore",
        }
    }
}

/// Lifecycle state of a worker.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum WorkerStatus {
    Active,
    Aborted,
}

/// A tracked worker in the lifecycle registry.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct WorkerEntry {
    pub wp_id: String,
    pub role: WorkerRole,
    pub status: WorkerStatus,
    pub pane_name: String,
    pub worktree_path: Option<PathBuf>,
    pub prompt: String,
    pub created_at: DateTime<Utc>,
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
struct WorkerKey {
    wp_id: String,
    role: WorkerRole,
}

impl WorkerKey {
    fn new(wp_id: &str, role: WorkerRole) -> Self {
        Self {
            wp_id: wp_id.to_string(),
            role,
        }
    }
}

/// In-memory worker registry keyed by (wp_id, role).
#[derive(Debug, Default)]
pub struct WorkerRegistry {
    workers: HashMap<WorkerKey, WorkerEntry>,
}

impl WorkerRegistry {
    pub fn upsert(&mut self, entry: WorkerEntry) {
        let key = WorkerKey::new(&entry.wp_id, entry.role);
        self.workers.insert(key, entry);
    }

    pub fn get(&self, wp_id: &str, role: WorkerRole) -> Option<&WorkerEntry> {
        self.workers.get(&WorkerKey::new(wp_id, role))
    }

    pub fn get_mut(&mut self, wp_id: &str, role: WorkerRole) -> Option<&mut WorkerEntry> {
        self.workers.get_mut(&WorkerKey::new(wp_id, role))
    }

    pub fn remove(&mut self, wp_id: &str, role: WorkerRole) -> Option<WorkerEntry> {
        self.workers.remove(&WorkerKey::new(wp_id, role))
    }

    pub fn list(&self) -> Vec<WorkerEntry> {
        self.workers.values().cloned().collect()
    }

    pub fn active_count(&self) -> usize {
        self.workers
            .values()
            .filter(|entry| entry.status == WorkerStatus::Active)
            .count()
    }
}
