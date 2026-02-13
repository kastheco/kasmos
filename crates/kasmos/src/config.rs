//! Configuration system for kasmos.
//!
//! Supports layered configuration with precedence: CLI args > env vars > TOML file > defaults.

use crate::error::{ConfigError, KasmosError};
use crate::types::ProgressionMode;
use serde::{Deserialize, Serialize};
use std::path::Path;

/// Runtime configuration for kasmos orchestration.
///
/// Configuration is loaded with the following precedence (highest to lowest):
/// 1. CLI arguments (handled by caller)
/// 2. Environment variables (KASMOS_* prefix)
/// 3. TOML configuration file (.kasmos/config.toml)
/// 4. Built-in defaults
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    /// Maximum number of concurrent agent panes (1-16).
    /// Default: 8
    pub max_agent_panes: usize,

    /// Progression mode (Continuous or WaveGated).
    /// Default: Continuous
    pub progression_mode: ProgressionMode,

    /// Path to zellij binary.
    /// Default: "zellij"
    pub zellij_binary: String,

    /// Path to ocx binary (launches opencode via `ocx oc`).
    /// Default: "ocx"
    pub opencode_binary: String,

    /// Path to spec-kitty binary.
    /// Default: "spec-kitty"
    pub spec_kitty_binary: String,

    /// Directory for kasmos state and artifacts.
    /// Default: ".kasmos"
    pub kasmos_dir: String,

    /// Poll interval in seconds for crash detection.
    /// Default: 5
    pub poll_interval_secs: u64,

    /// Debounce interval in milliseconds for completion detection.
    /// Default: 200
    pub debounce_ms: u64,

    /// Controller pane width as percentage of terminal width (10-90).
    /// Default: 40
    pub controller_width_pct: u32,

    /// Profile name passed to `ocx oc -p <profile>`.
    /// When set, all opencode invocations include `-p <profile>` before the `--` separator.
    /// Default: Some("kas")
    pub opencode_profile: Option<String>,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            max_agent_panes: 8,
            progression_mode: ProgressionMode::Continuous,
            zellij_binary: "zellij".to_string(),
            opencode_binary: "ocx".to_string(),
            spec_kitty_binary: "spec-kitty".to_string(),
            kasmos_dir: ".kasmos".to_string(),
            poll_interval_secs: 5,
            debounce_ms: 200,
            controller_width_pct: 40,
            opencode_profile: Some("kas".to_string()),
        }
    }
}

impl Config {
    /// Create a new config with default values.
    pub fn new() -> Self {
        Self::default()
    }

    /// Load configuration from environment variables.
    ///
    /// Looks for the following environment variables:
    /// - KASMOS_MAX_PANES: max_agent_panes
    /// - KASMOS_MODE: progression_mode (continuous or wave_gated)
    /// - KASMOS_ZELLIJ: zellij_binary
    /// - KASMOS_OPENCODE: opencode_binary
    /// - KASMOS_SPEC_KITTY: spec_kitty_binary
    /// - KASMOS_DIR: kasmos_dir
    /// - KASMOS_POLL_INTERVAL: poll_interval_secs
    /// - KASMOS_DEBOUNCE: debounce_ms
    /// - KASMOS_CONTROLLER_WIDTH: controller_width_pct
    pub fn load_from_env(&mut self) -> Result<(), KasmosError> {
        if let Ok(val) = std::env::var("KASMOS_MAX_PANES") {
            self.max_agent_panes = val.parse().map_err(|_| ConfigError::InvalidValue {
                field: "max_agent_panes".to_string(),
                value: val.clone(),
                reason: "must be a positive integer".to_string(),
            })?;
        }

        if let Ok(val) = std::env::var("KASMOS_MODE") {
            self.progression_mode = match val.as_str() {
                "continuous" => ProgressionMode::Continuous,
                "wave_gated" => ProgressionMode::WaveGated,
                _ => {
                    return Err(ConfigError::InvalidValue {
                        field: "progression_mode".to_string(),
                        value: val,
                        reason: "must be 'continuous' or 'wave_gated'".to_string(),
                    }
                    .into());
                }
            };
        }

        if let Ok(val) = std::env::var("KASMOS_ZELLIJ") {
            self.zellij_binary = val;
        }

        if let Ok(val) = std::env::var("KASMOS_OPENCODE") {
            self.opencode_binary = val;
        }

        if let Ok(val) = std::env::var("KASMOS_SPEC_KITTY") {
            self.spec_kitty_binary = val;
        }

        if let Ok(val) = std::env::var("KASMOS_DIR") {
            self.kasmos_dir = val;
        }

        if let Ok(val) = std::env::var("KASMOS_POLL_INTERVAL") {
            self.poll_interval_secs = val.parse().map_err(|_| ConfigError::InvalidValue {
                field: "poll_interval_secs".to_string(),
                value: val.clone(),
                reason: "must be a positive integer".to_string(),
            })?;
        }

        if let Ok(val) = std::env::var("KASMOS_DEBOUNCE") {
            self.debounce_ms = val.parse().map_err(|_| ConfigError::InvalidValue {
                field: "debounce_ms".to_string(),
                value: val.clone(),
                reason: "must be a positive integer".to_string(),
            })?;
        }

        if let Ok(val) = std::env::var("KASMOS_CONTROLLER_WIDTH") {
            self.controller_width_pct = val.parse().map_err(|_| ConfigError::InvalidValue {
                field: "controller_width_pct".to_string(),
                value: val.clone(),
                reason: "must be a positive integer".to_string(),
            })?;
        }

        if let Ok(val) = std::env::var("KASMOS_OPENCODE_PROFILE") {
            if val.is_empty() {
                self.opencode_profile = None;
            } else {
                self.opencode_profile = Some(val);
            }
        }

        Ok(())
    }

    /// Load configuration from a TOML file.
    ///
    /// The file should be in the format:
    /// ```toml
    /// max_agent_panes = 8
    /// progression_mode = "continuous"
    /// zellij_binary = "zellij"
    /// # ... etc
    /// ```
    pub fn load_from_file<P: AsRef<Path>>(&mut self, path: P) -> Result<(), KasmosError> {
        let path = path.as_ref();
        let content = std::fs::read_to_string(path).map_err(|_| ConfigError::NotFound {
            path: path.display().to_string(),
        })?;

        let file_config: ConfigFile = toml::from_str(&content)
            .map_err(|e| ConfigError::Parse(format!("Failed to parse TOML: {}", e)))?;

        if let Some(val) = file_config.max_agent_panes {
            self.max_agent_panes = val;
        }
        if let Some(val) = file_config.progression_mode {
            self.progression_mode = val;
        }
        if let Some(val) = file_config.zellij_binary {
            self.zellij_binary = val;
        }
        if let Some(val) = file_config.opencode_binary {
            self.opencode_binary = val;
        }
        if let Some(val) = file_config.spec_kitty_binary {
            self.spec_kitty_binary = val;
        }
        if let Some(val) = file_config.kasmos_dir {
            self.kasmos_dir = val;
        }
        if let Some(val) = file_config.poll_interval_secs {
            self.poll_interval_secs = val;
        }
        if let Some(val) = file_config.debounce_ms {
            self.debounce_ms = val;
        }
        if let Some(val) = file_config.controller_width_pct {
            self.controller_width_pct = val;
        }
        if let Some(val) = file_config.opencode_profile {
            self.opencode_profile = val;
        }

        Ok(())
    }

    /// Validate the configuration.
    ///
    /// Ensures all values are within acceptable ranges.
    pub fn validate(&self) -> Result<(), KasmosError> {
        if self.max_agent_panes < 1 || self.max_agent_panes > 16 {
            return Err(ConfigError::InvalidValue {
                field: "max_agent_panes".to_string(),
                value: self.max_agent_panes.to_string(),
                reason: "must be between 1 and 16".to_string(),
            }
            .into());
        }

        if self.controller_width_pct < 10 || self.controller_width_pct > 90 {
            return Err(ConfigError::InvalidValue {
                field: "controller_width_pct".to_string(),
                value: self.controller_width_pct.to_string(),
                reason: "must be between 10 and 90".to_string(),
            }
            .into());
        }

        if self.poll_interval_secs == 0 {
            return Err(ConfigError::InvalidValue {
                field: "poll_interval_secs".to_string(),
                value: self.poll_interval_secs.to_string(),
                reason: "must be greater than 0".to_string(),
            }
            .into());
        }

        Ok(())
    }
}

/// Intermediate struct for TOML deserialization.
/// All fields are optional to support partial configuration files.
#[derive(Debug, Deserialize)]
struct ConfigFile {
    max_agent_panes: Option<usize>,
    progression_mode: Option<ProgressionMode>,
    zellij_binary: Option<String>,
    opencode_binary: Option<String>,
    spec_kitty_binary: Option<String>,
    kasmos_dir: Option<String>,
    poll_interval_secs: Option<u64>,
    debounce_ms: Option<u64>,
    controller_width_pct: Option<u32>,
    /// Wrapped in Option<Option<>> to distinguish "not set" from "set to null/empty".
    /// In TOML: `opencode_profile = "kas"` or omit entirely.
    opencode_profile: Option<Option<String>>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    // Mutex to serialize environment variable tests and prevent race conditions
    static ENV_TEST_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn test_default_config() {
        let config = Config::default();
        assert_eq!(config.max_agent_panes, 8);
        assert_eq!(config.progression_mode, ProgressionMode::Continuous);
        assert_eq!(config.zellij_binary, "zellij");
        assert_eq!(config.poll_interval_secs, 5);
        assert_eq!(config.debounce_ms, 200);
        assert_eq!(config.controller_width_pct, 40);
    }

    #[test]
    fn test_validate_default_config() {
        let config = Config::default();
        assert!(config.validate().is_ok());
    }

    #[test]
    fn test_validate_max_agent_panes_too_low() {
        let config = Config {
            max_agent_panes: 0,
            ..Default::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_validate_max_agent_panes_too_high() {
        let config = Config {
            max_agent_panes: 17,
            ..Default::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_validate_controller_width_too_low() {
        let config = Config {
            controller_width_pct: 5,
            ..Default::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_validate_controller_width_too_high() {
        let config = Config {
            controller_width_pct: 95,
            ..Default::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_validate_poll_interval_zero() {
        let config = Config {
            poll_interval_secs: 0,
            ..Default::default()
        };
        assert!(config.validate().is_err());
    }

    #[test]
    fn test_load_from_env() {
        let _guard = ENV_TEST_LOCK.lock().unwrap();

        unsafe {
            std::env::set_var("KASMOS_MAX_PANES", "12");
            std::env::set_var("KASMOS_MODE", "wave_gated");
        }

        let mut config = Config::default();
        let result = config.load_from_env();

        unsafe {
            std::env::remove_var("KASMOS_MAX_PANES");
            std::env::remove_var("KASMOS_MODE");
        }

        assert!(result.is_ok());
        assert_eq!(config.max_agent_panes, 12);
        assert_eq!(config.progression_mode, ProgressionMode::WaveGated);
    }

    #[test]
    fn test_load_from_env_invalid_value() {
        let _guard = ENV_TEST_LOCK.lock().unwrap();

        unsafe {
            std::env::set_var("KASMOS_MAX_PANES", "not_a_number");
        }

        let mut config = Config::default();
        let result = config.load_from_env();

        unsafe {
            std::env::remove_var("KASMOS_MAX_PANES");
        }

        assert!(result.is_err());
    }
}
