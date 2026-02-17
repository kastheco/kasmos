//! kasmos -- Zellij Agent Orchestrator
//!
//! MCP-first orchestration system for coordinating AI agents
//! across Zellij terminal panes.

pub mod config;
pub mod error;
pub mod feature_arg;
pub mod graph;
pub mod launch;
pub mod logging;
pub mod parser;
pub mod prompt;
pub mod serve;
pub mod setup;
pub mod types;

// Re-export commonly used types
pub use config::Config;
pub use error::{KasmosError, Result};
pub use graph::DependencyGraph;
pub use logging::init_logging;
pub use parser::{FeatureDir, WPFrontmatter, parse_frontmatter};
pub use prompt::{AgentRole, ContextBoundary, RolePromptBuilder};
pub use types::{
    CompletionMethod, OrchestrationRun, ProgressionMode, RunState, WPState, Wave, WaveState,
    WorkPackage,
};
