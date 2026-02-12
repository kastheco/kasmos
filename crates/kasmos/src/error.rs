//! Error types for kasmos.
//!
//! Provides a comprehensive error hierarchy covering all kasmos subsystems,
//! enabling rich error messages with context.

use crate::types::WPState;
use thiserror::Error;

/// Result type alias for kasmos operations.
pub type Result<T> = std::result::Result<T, KasmosError>;

/// Top-level error type for kasmos.
///
/// Aggregates all domain-specific errors and provides a unified error interface.
#[derive(Error, Debug)]
pub enum KasmosError {
    /// Configuration error.
    #[error("Configuration error: {0}")]
    Config(#[from] ConfigError),

    /// Zellij-related error.
    #[error("Zellij error: {0}")]
    Zellij(#[from] ZellijError),

    /// Spec parser error.
    #[error("Spec parser error: {0}")]
    SpecParser(#[from] SpecParserError),

    /// State machine error.
    #[error("State error: {0}")]
    State(#[from] StateError),

    /// Pane operation error.
    #[error("Pane error: {0}")]
    Pane(#[from] PaneError),

    /// Wave engine error.
    #[error("Wave engine error: {0}")]
    Wave(#[from] WaveError),

    /// Layout generation error.
    #[error("Layout error: {0}")]
    Layout(#[from] LayoutError),

    /// Detector error.
    #[error("Detector error: {0}")]
    Detector(#[from] DetectorError),

    /// I/O error.
    #[error(transparent)]
    Io(#[from] std::io::Error),

    /// Generic error.
    #[error(transparent)]
    Other(#[from] anyhow::Error),
}

/// Configuration-related errors.
#[derive(Error, Debug)]
pub enum ConfigError {
    /// Configuration file not found.
    #[error("Config file not found: {path}")]
    NotFound { path: String },

    /// Invalid configuration value.
    #[error("Invalid config value: {field} = {value} ({reason})")]
    InvalidValue {
        field: String,
        value: String,
        reason: String,
    },

    /// Failed to parse configuration.
    #[error("Failed to parse config: {0}")]
    Parse(String),
}

/// Zellij-related errors.
#[derive(Error, Debug)]
pub enum ZellijError {
    /// Zellij binary not found in PATH.
    #[error("Zellij binary not found in PATH")]
    NotFound,

    /// Session already exists.
    #[error("Session '{name}' already exists")]
    SessionExists { name: String },

    /// Session not found.
    #[error("Session '{name}' not found")]
    SessionNotFound { name: String },

    /// Failed to create session.
    #[error("Failed to create session: {0}")]
    CreateFailed(String),

    /// Pane operation failed.
    #[error("Pane operation failed: {0}")]
    PaneOperation(String),
}

/// Spec parser errors.
#[derive(Error, Debug)]
pub enum SpecParserError {
    /// Feature directory not found.
    #[error("Feature directory not found: {path}")]
    FeatureDirNotFound { path: String },

    /// Invalid YAML frontmatter.
    #[error("Invalid YAML frontmatter in {file}: {reason}")]
    InvalidFrontmatter { file: String, reason: String },

    /// Circular dependency detected.
    #[error("Circular dependency detected: {cycle}")]
    CircularDependency { cycle: String },

    /// Unknown dependency.
    #[error("Unknown dependency '{dep}' referenced by '{wp}'")]
    UnknownDependency { dep: String, wp: String },

    /// I/O error during spec parsing.
    #[error("I/O error: {0}")]
    IoError(#[from] std::io::Error),

    /// YAML parsing error.
    #[error("YAML error: {0}")]
    YamlError(#[from] serde_yml::Error),
}

/// Layout generation errors.
#[derive(Error, Debug)]
pub enum LayoutError {
    /// KDL generation failed.
    #[error("KDL generation failed: {0}")]
    KdlGeneration(String),

    /// KDL validation failed.
    #[error("KDL validation failed: {0}")]
    KdlValidation(String),

    /// Invalid pane count.
    #[error("Invalid pane count: {0}")]
    InvalidPaneCount(String),
}

/// Detector errors.
#[derive(Error, Debug)]
pub enum DetectorError {
    /// Filesystem watcher error.
    #[error("Watcher error: {0}")]
    WatcherError(String),

    /// File read error.
    #[error("Read error: {0}")]
    ReadError(String),

    /// YAML parse error.
    #[error("YAML error: {0}")]
    YamlError(String),
}

impl From<notify::Error> for DetectorError {
    fn from(e: notify::Error) -> Self {
        DetectorError::WatcherError(e.to_string())
    }
}

/// State machine errors.
#[derive(Error, Debug)]
pub enum StateError {
    /// Invalid state transition.
    #[error("Invalid state transition: {from:?} -> {to:?} for WP {wp_id}")]
    InvalidTransition {
        wp_id: String,
        from: WPState,
        to: WPState,
    },

    /// State file corrupted.
    #[error("State file corrupted: {0}")]
    Corrupted(String),

    /// Stale state detected.
    #[error("Stale state detected: last updated {last_updated}")]
    Stale { last_updated: String },
}

/// Pane operation errors.
#[derive(Error, Debug)]
pub enum PaneError {
    /// Pane not found.
    #[error("Pane not found for WP {wp_id}")]
    NotFound { wp_id: String },

    /// Pane crashed.
    #[error("Pane {pane_id} crashed for WP {wp_id}")]
    Crashed { pane_id: u32, wp_id: String },

    /// Prompt injection failed.
    #[error("Prompt injection failed for WP {wp_id}: {reason}")]
    PromptInjectionFailed { wp_id: String, reason: String },
}

/// Wave engine errors.
#[derive(Error, Debug)]
pub enum WaveError {
    /// Wave has no eligible work packages.
    #[error("Wave {wave} has no eligible work packages")]
    NoEligible { wave: usize },

    /// Work package not found.
    #[error("Work package not found: {wp_id}")]
    WpNotFound { wp_id: String },

    /// Capacity limit reached.
    #[error("Capacity limit reached: {active}/{max} panes")]
    CapacityExceeded { active: usize, max: usize },

    /// Wave progression blocked.
    #[error("Wave progression blocked: WP {blocker} failed")]
    Blocked { blocker: String },
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_config_error_display() {
        let err = ConfigError::NotFound {
            path: "/path/to/config".to_string(),
        };
        assert!(err.to_string().contains("not found"));
    }

    #[test]
    fn test_zellij_error_display() {
        let err = ZellijError::NotFound;
        assert!(err.to_string().contains("not found"));
    }

    #[test]
    fn test_state_error_display() {
        let err = StateError::InvalidTransition {
            wp_id: "WP01".to_string(),
            from: WPState::Completed,
            to: WPState::Active,
        };
        assert!(err.to_string().contains("Invalid state transition"));
    }

    #[test]
    fn test_kasmos_error_from_config() {
        let config_err = ConfigError::NotFound {
            path: "/path".to_string(),
        };
        let kasmos_err: KasmosError = config_err.into();
        assert!(kasmos_err.to_string().contains("Configuration error"));
    }

    #[test]
    fn test_kasmos_error_from_state() {
        let state_err = StateError::InvalidTransition {
            wp_id: "WP01".to_string(),
            from: WPState::Pending,
            to: WPState::Failed,
        };
        let kasmos_err: KasmosError = state_err.into();
        assert!(kasmos_err.to_string().contains("State error"));
    }
}
