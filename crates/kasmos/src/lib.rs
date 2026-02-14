//! kasmos -- Zellij Agent Orchestrator
//!
//! A Rust-based orchestration system for coordinating multiple AI agents
//! across Zellij terminal panes, managing work packages, waves, and state transitions.

pub mod cleanup;
pub mod command_handlers;
pub mod commands;
pub mod config;
pub mod detector;
pub mod engine;
pub mod error;
pub mod git;
pub mod graph;
pub mod health;
pub mod launch;
pub mod layout;
pub mod logging;
pub mod parser;
pub mod persistence;
pub mod prompt;
pub mod review;
pub mod review_coordinator;
pub mod serve;
pub mod session;
pub mod setup;
pub mod shutdown;
pub mod signals;
pub mod state_machine;
#[cfg(feature = "tui")]
pub mod tui;
pub mod types;
pub mod zellij;

// Re-export commonly used types
pub use cleanup::cleanup_artifacts;
pub use command_handlers::{CommandHandler, EngineAction, SessionController};
pub use commands::{CommandReader, ControllerCommand, command_help_text};
pub use config::Config;
pub use detector::{CompletionDetector, CompletionEvent, DetectedLane};
pub use engine::{WaveEngine, WaveLaunchEvent};
pub use error::{KasmosError, Result};
pub use git::WorktreeManager;
pub use graph::DependencyGraph;
pub use health::{CrashEvent, HealthMonitor, PaneHealthChecker, PaneRegistration};
pub use layout::LayoutGenerator;
pub use logging::init_logging;
pub use parser::{FeatureDir, WPFrontmatter, parse_frontmatter};
pub use persistence::StatePersister;
pub use prompt::PromptGenerator;
pub use review_coordinator::ReviewCoordinator;
pub use review::{
    ReviewAutomationPolicy, ReviewFailureSeverity, ReviewFailureType, ReviewPolicyDecision,
    ReviewPolicyExecutor,
};
pub use session::SessionManager;
pub use shutdown::{ShutdownCoordinator, ShutdownSession};
pub use signals::setup_signal_handlers;
pub use types::{
    CompletionMethod, OrchestrationRun, ProgressionMode, ReviewRequest, RunState, WPState, Wave,
    WaveState, WorkPackage,
};
pub use zellij::{RealZellijCli, ZellijCli};
