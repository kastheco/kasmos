//! kasmos — Zellij Agent Orchestrator
//!
//! A Rust-based orchestration system for coordinating multiple AI agents
//! across Zellij terminal panes, managing work packages, waves, and state transitions.

pub mod command_handlers;
pub mod commands;
pub mod config;
pub mod detector;
pub mod engine;
pub mod error;
pub mod graph;
pub mod logging;
pub mod state_machine;
pub mod types;

// Re-export commonly used types
pub use command_handlers::{CommandHandler, EngineAction, SessionController};
pub use commands::{CommandReader, ControllerCommand, command_help_text};
pub use config::Config;
pub use detector::CompletionEvent;
pub use engine::WaveEngine;
pub use error::{KasmosError, Result};
pub use graph::DependencyGraph;
pub use logging::init_logging;
pub use types::{
    CompletionMethod, OrchestrationRun, ProgressionMode, RunState, WPState, Wave, WaveState,
    WorkPackage,
};
