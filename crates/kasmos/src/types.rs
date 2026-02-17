//! Core data model for kasmos orchestration.
//!
//! This module defines the fundamental types representing orchestration runs,
//! work packages, waves, and their state machines.

use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::time::SystemTime;

/// Represents a complete orchestration run.
///
/// An orchestration run coordinates multiple work packages across waves,
/// managing their execution, state transitions, and completion.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrchestrationRun {
    /// Unique identifier for this run (UUID or timestamp-based).
    pub id: String,

    /// Feature name (e.g., "001-zellij-agent-orchestrator").
    pub feature: String,

    /// Absolute path to the feature directory.
    pub feature_dir: PathBuf,

    /// Runtime configuration for this run.
    pub config: crate::config::Config,

    /// All work packages in this run.
    pub work_packages: Vec<WorkPackage>,

    /// Wave structure organizing work packages.
    pub waves: Vec<Wave>,

    /// Overall run state.
    pub state: RunState,

    /// When the run started.
    pub started_at: Option<SystemTime>,

    /// When the run completed.
    pub completed_at: Option<SystemTime>,

    /// Progression mode (Continuous or WaveGated).
    pub mode: ProgressionMode,
}

/// Represents a single work package within an orchestration run.
///
/// A work package is a unit of work that can be executed by an agent,
/// with its own state, dependencies, and completion tracking.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkPackage {
    /// Work package identifier (e.g., "WP01", "WP02").
    pub id: String,

    /// Human-readable title.
    pub title: String,

    /// Current state of this work package.
    pub state: WPState,

    /// IDs of upstream work packages this depends on.
    pub dependencies: Vec<String>,

    /// Wave index (0-based) this work package belongs to.
    pub wave: usize,

    /// Zellij pane ID assigned at runtime (if any).
    pub pane_id: Option<u32>,

    /// KDL pane name attribute.
    pub pane_name: String,

    /// Path to the git worktree for this work package (if any).
    pub worktree_path: Option<PathBuf>,

    /// Path to the prompt file for this work package (if any).
    pub prompt_path: Option<PathBuf>,

    /// When execution started.
    pub started_at: Option<SystemTime>,

    /// When execution completed.
    pub completed_at: Option<SystemTime>,

    /// How this work package was completed (if completed).
    pub completion_method: Option<CompletionMethod>,

    /// Number of times this work package has failed and been retried.
    pub failure_count: u32,
}

/// Represents a wave of work packages.
///
/// Waves organize work packages into execution phases, allowing
/// for sequential or gated progression through the orchestration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Wave {
    /// Wave index (0-based).
    pub index: usize,

    /// IDs of work packages in this wave.
    pub wp_ids: Vec<String>,

    /// Current state of this wave.
    pub state: WaveState,
}

/// State of a work package.
///
/// Valid transitions:
/// - Pending → Active (when wave launches)
/// - Active → Completed (on completion detection)
/// - Active → Failed (on crash/error)
/// - Active → Paused (on pause command)
/// - Paused → Active (on resume command)
/// - Failed → Pending (on retry command)
/// - Failed → Active (on restart command)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum WPState {
    /// Waiting to be activated.
    Pending,

    /// Currently executing.
    Active,

    /// Successfully completed.
    Completed,

    /// Failed (may be retried).
    Failed,

    /// Paused (can be resumed).
    Paused,

    /// Awaiting operator review.
    ForReview,
}

/// State of an orchestration run.
///
/// Valid transitions:
/// - Initializing → Running
/// - Running → Paused (wave-gated boundary)
/// - Paused → Running (operator confirms)
/// - Running → Completed (all WPs done)
/// - Running → Failed (unrecoverable error)
/// - Running → Aborted (operator abort)
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum RunState {
    /// Initializing the run.
    Initializing,

    /// Running work packages.
    Running,

    /// Paused at wave boundary.
    Paused,

    /// All work packages completed successfully.
    Completed,

    /// Run failed (unrecoverable error).
    Failed,

    /// Run aborted by operator.
    Aborted,
}

/// State of a wave.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
#[non_exhaustive]
pub enum WaveState {
    /// Waiting to be activated.
    Pending,

    /// Currently executing.
    Active,

    /// All work packages completed.
    Completed,

    /// Some work packages failed.
    PartiallyFailed,
}

/// Progression mode for the orchestration.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum ProgressionMode {
    /// Execute all work packages continuously.
    Continuous,

    /// Execute work packages wave by wave, pausing between waves.
    WaveGated,
}

/// How a work package was completed.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CompletionMethod {
    /// Detected via spec-kitty lane transition.
    AutoDetected,

    /// Detected via git activity.
    GitActivity,

    /// Detected via file marker.
    FileMarker,

    /// Manually marked as complete by operator.
    Manual,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_wp_state_serialization() {
        let state = WPState::Active;
        let json = serde_json::to_string(&state).expect("serialize");
        assert_eq!(json, "\"active\"");

        let deserialized: WPState = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(deserialized, WPState::Active);
    }

    #[test]
    fn test_run_state_serialization() {
        let state = RunState::Running;
        let json = serde_json::to_string(&state).expect("serialize");
        assert_eq!(json, "\"running\"");

        let deserialized: RunState = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(deserialized, RunState::Running);
    }

    #[test]
    fn test_progression_mode_serialization() {
        let mode = ProgressionMode::WaveGated;
        let json = serde_json::to_string(&mode).expect("serialize");
        assert_eq!(json, "\"wave_gated\"");

        let deserialized: ProgressionMode = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(deserialized, ProgressionMode::WaveGated);
    }

    #[test]
    fn test_completion_method_serialization() {
        let method = CompletionMethod::AutoDetected;
        let json = serde_json::to_string(&method).expect("serialize");
        assert_eq!(json, "\"auto_detected\"");

        let deserialized: CompletionMethod = serde_json::from_str(&json).expect("deserialize");
        assert_eq!(deserialized, CompletionMethod::AutoDetected);
    }
}
