//! kasmos — Zellij Agent Orchestrator
//!
//! A Rust-based orchestration system for coordinating multiple AI agents
//! across Zellij terminal panes, managing work packages, waves, and state transitions.

pub mod config;
pub mod error;
pub mod logging;
pub mod state_machine;
pub mod types;

// Re-export commonly used types
pub use config::Config;
pub use error::{KasmosError, Result};
pub use logging::init_logging;
pub use types::{
    CompletionMethod, OrchestrationRun, ProgressionMode, RunState, WPState, Wave, WaveState,
    WorkPackage,
};
