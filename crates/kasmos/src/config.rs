//! Configuration system for kasmos.
//!
//! The launch/runtime configuration uses sectioned TOML with precedence:
//! defaults -> `kasmos.toml` -> `KASMOS_` environment overrides.

use crate::error::{ConfigError, KasmosError};
use crate::types::ProgressionMode;
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

/// Runtime configuration for kasmos orchestration.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    /// Agent/runtime limits and binary config.
    pub agent: AgentConfig,
    /// Polling and timeout settings for runtime communication.
    pub communication: CommunicationConfig,
    /// Binary paths and repository paths.
    pub paths: PathsConfig,
    /// Session and layout defaults.
    pub session: SessionConfig,
    /// Audit retention and payload policy.
    pub audit: AuditConfig,
    /// Feature lock and stale timeout policy.
    pub lock: LockConfig,

    // Legacy flat fields are kept for compatibility with pre-WP02 code paths.
    #[serde(default)]
    pub max_agent_panes: usize,
    #[serde(default = "default_progression_mode")]
    pub progression_mode: ProgressionMode,
    #[serde(default)]
    pub zellij_binary: String,
    #[serde(default)]
    pub opencode_binary: String,
    #[serde(default)]
    pub spec_kitty_binary: String,
    #[serde(default)]
    pub kasmos_dir: String,
    #[serde(default)]
    pub poll_interval_secs: u64,
    #[serde(default)]
    pub debounce_ms: u64,
    #[serde(default)]
    pub controller_width_pct: u32,
    #[serde(default)]
    pub opencode_profile: Option<String>,
}

/// Agent/runtime settings.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentConfig {
    /// Maximum number of parallel worker panes.
    pub max_parallel_workers: usize,
    /// OpenCode launcher binary.
    pub opencode_binary: String,
    /// Optional OpenCode profile.
    pub opencode_profile: Option<String>,
    /// Maximum review rejection attempts before escalation.
    pub review_rejection_cap: u32,
}

/// Runtime communication settings.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CommunicationConfig {
    /// Poll interval for status/event reads.
    pub poll_interval_secs: u64,
    /// Timeout for event wait operations.
    pub event_timeout_secs: u64,
}

/// Paths and binary settings.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PathsConfig {
    /// Zellij binary name/path.
    pub zellij_binary: String,
    /// spec-kitty binary name/path.
    pub spec_kitty_binary: String,
    /// Feature specs root path.
    pub specs_root: String,
}

/// Zellij session/layout settings.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionConfig {
    /// Session name used for orchestration.
    pub session_name: String,
    /// Manager pane width percentage.
    pub manager_width_pct: u32,
    /// Message-log pane width percentage.
    pub message_log_width_pct: u32,
    /// Maximum worker panes in one row.
    pub max_workers_per_row: usize,
}

/// Audit policy settings.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuditConfig {
    /// Default to metadata-only records.
    pub metadata_only: bool,
    /// Include full payloads when debug mode is enabled.
    pub debug_full_payload: bool,
    /// Maximum audit file size before rotation/prune.
    pub max_bytes: u64,
    /// Maximum record age in days before prune.
    pub max_age_days: u32,
}

/// Feature lock settings.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LockConfig {
    /// Stale lock timeout in minutes.
    pub stale_timeout_minutes: u64,
}

impl Default for AgentConfig {
    fn default() -> Self {
        Self {
            max_parallel_workers: 4,
            opencode_binary: "ocx".to_string(),
            opencode_profile: Some("kas".to_string()),
            review_rejection_cap: 3,
        }
    }
}

impl Default for CommunicationConfig {
    fn default() -> Self {
        Self {
            poll_interval_secs: 5,
            event_timeout_secs: 300,
        }
    }
}

impl Default for PathsConfig {
    fn default() -> Self {
        Self {
            zellij_binary: "zellij".to_string(),
            spec_kitty_binary: "spec-kitty".to_string(),
            specs_root: "kitty-specs".to_string(),
        }
    }
}

impl Default for SessionConfig {
    fn default() -> Self {
        Self {
            session_name: "kasmos".to_string(),
            manager_width_pct: 60,
            message_log_width_pct: 20,
            max_workers_per_row: 4,
        }
    }
}

impl Default for AuditConfig {
    fn default() -> Self {
        Self {
            metadata_only: true,
            debug_full_payload: false,
            max_bytes: 512 * 1024 * 1024,
            max_age_days: 14,
        }
    }
}

impl Default for LockConfig {
    fn default() -> Self {
        Self {
            stale_timeout_minutes: 15,
        }
    }
}

impl Default for Config {
    fn default() -> Self {
        let agent = AgentConfig::default();
        let communication = CommunicationConfig::default();
        let paths = PathsConfig::default();
        let session = SessionConfig::default();
        let audit = AuditConfig::default();
        let lock = LockConfig::default();

        Self {
            agent,
            communication,
            paths,
            session,
            audit,
            lock,
            max_agent_panes: 4,
            progression_mode: ProgressionMode::Continuous,
            zellij_binary: "zellij".to_string(),
            opencode_binary: "ocx".to_string(),
            spec_kitty_binary: "spec-kitty".to_string(),
            kasmos_dir: ".kasmos".to_string(),
            poll_interval_secs: 5,
            debounce_ms: 200,
            controller_width_pct: 60,
            opencode_profile: Some("kas".to_string()),
        }
    }
}

impl Config {
    /// Create a new config with default values.
    pub fn new() -> Self {
        Self::default()
    }

    /// Load effective config using precedence:
    /// defaults -> `kasmos.toml` -> env overrides.
    pub fn load() -> Result<Self, KasmosError> {
        let mut config = Self::default();
        if let Some(path) = discover_kasmos_toml() {
            config.load_from_file(path)?;
        }
        config.load_from_env()?;
        config.validate()?;
        Ok(config)
    }

    /// Apply environment overrides (`KASMOS_...`) to this config.
    pub fn load_from_env(&mut self) -> Result<(), KasmosError> {
        // New sectioned env vars.
        if let Some(val) = read_env_usize("KASMOS_AGENT_MAX_PARALLEL_WORKERS")? {
            self.agent.max_parallel_workers = val;
        }
        if let Ok(val) = std::env::var("KASMOS_AGENT_OPENCODE_BINARY") {
            self.agent.opencode_binary = val;
        }
        if let Ok(val) = std::env::var("KASMOS_AGENT_OPENCODE_PROFILE") {
            self.agent.opencode_profile = if val.is_empty() { None } else { Some(val) };
        }
        if let Some(val) = read_env_u32("KASMOS_AGENT_REVIEW_REJECTION_CAP")? {
            self.agent.review_rejection_cap = val;
        }

        if let Some(val) = read_env_u64("KASMOS_COMMUNICATION_POLL_INTERVAL_SECS")? {
            self.communication.poll_interval_secs = val;
        }
        if let Some(val) = read_env_u64("KASMOS_COMMUNICATION_EVENT_TIMEOUT_SECS")? {
            self.communication.event_timeout_secs = val;
        }

        if let Ok(val) = std::env::var("KASMOS_PATHS_ZELLIJ_BINARY") {
            self.paths.zellij_binary = val;
        }
        if let Ok(val) = std::env::var("KASMOS_PATHS_SPEC_KITTY_BINARY") {
            self.paths.spec_kitty_binary = val;
        }
        if let Ok(val) = std::env::var("KASMOS_PATHS_SPECS_ROOT") {
            self.paths.specs_root = val;
        }

        if let Ok(val) = std::env::var("KASMOS_SESSION_SESSION_NAME") {
            self.session.session_name = val;
        }
        if let Some(val) = read_env_u32("KASMOS_SESSION_MANAGER_WIDTH_PCT")? {
            self.session.manager_width_pct = val;
        }
        if let Some(val) = read_env_u32("KASMOS_SESSION_MESSAGE_LOG_WIDTH_PCT")? {
            self.session.message_log_width_pct = val;
        }
        if let Some(val) = read_env_usize("KASMOS_SESSION_MAX_WORKERS_PER_ROW")? {
            self.session.max_workers_per_row = val;
        }

        if let Some(val) = read_env_bool("KASMOS_AUDIT_METADATA_ONLY")? {
            self.audit.metadata_only = val;
        }
        if let Some(val) = read_env_bool("KASMOS_AUDIT_DEBUG_FULL_PAYLOAD")? {
            self.audit.debug_full_payload = val;
        }
        if let Some(val) = read_env_u64("KASMOS_AUDIT_MAX_BYTES")? {
            self.audit.max_bytes = val;
        }
        if let Some(val) = read_env_u32("KASMOS_AUDIT_MAX_AGE_DAYS")? {
            self.audit.max_age_days = val;
        }

        if let Some(val) = read_env_u64("KASMOS_LOCK_STALE_TIMEOUT_MINUTES")? {
            self.lock.stale_timeout_minutes = val;
        }

        // Legacy env aliases for backward compatibility.
        if let Some(val) = read_env_usize("KASMOS_MAX_PANES")? {
            self.agent.max_parallel_workers = val;
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
            self.paths.zellij_binary = val;
        }
        if let Ok(val) = std::env::var("KASMOS_OPENCODE") {
            self.agent.opencode_binary = val;
        }
        if let Ok(val) = std::env::var("KASMOS_SPEC_KITTY") {
            self.paths.spec_kitty_binary = val;
        }
        if let Ok(val) = std::env::var("KASMOS_DIR") {
            self.kasmos_dir = val;
        }
        if let Some(val) = read_env_u64("KASMOS_POLL_INTERVAL")? {
            self.communication.poll_interval_secs = val;
        }
        if let Some(val) = read_env_u64("KASMOS_DEBOUNCE")? {
            self.debounce_ms = val;
        }
        if let Some(val) = read_env_u32("KASMOS_CONTROLLER_WIDTH")? {
            self.session.manager_width_pct = val;
        }
        if let Ok(val) = std::env::var("KASMOS_OPENCODE_PROFILE") {
            self.agent.opencode_profile = if val.is_empty() { None } else { Some(val) };
        }

        self.sync_legacy_fields();
        Ok(())
    }

    /// Apply config values from a TOML file.
    pub fn load_from_file<P: AsRef<Path>>(&mut self, path: P) -> Result<(), KasmosError> {
        let path = path.as_ref();
        let content = std::fs::read_to_string(path).map_err(|_| ConfigError::NotFound {
            path: path.display().to_string(),
        })?;

        let file_config: ConfigFile = toml::from_str(&content)
            .map_err(|e| ConfigError::Parse(format!("Failed to parse TOML: {}", e)))?;

        if let Some(agent) = file_config.agent {
            if let Some(v) = agent.max_parallel_workers {
                self.agent.max_parallel_workers = v;
            }
            if let Some(v) = agent.opencode_binary {
                self.agent.opencode_binary = v;
            }
            if let Some(v) = agent.opencode_profile {
                self.agent.opencode_profile = v;
            }
            if let Some(v) = agent.review_rejection_cap {
                self.agent.review_rejection_cap = v;
            }
        }

        if let Some(communication) = file_config.communication {
            if let Some(v) = communication.poll_interval_secs {
                self.communication.poll_interval_secs = v;
            }
            if let Some(v) = communication.event_timeout_secs {
                self.communication.event_timeout_secs = v;
            }
        }

        if let Some(paths) = file_config.paths {
            if let Some(v) = paths.zellij_binary {
                self.paths.zellij_binary = v;
            }
            if let Some(v) = paths.spec_kitty_binary {
                self.paths.spec_kitty_binary = v;
            }
            if let Some(v) = paths.specs_root {
                self.paths.specs_root = v;
            }
        }

        if let Some(session) = file_config.session {
            if let Some(v) = session.session_name {
                self.session.session_name = v;
            }
            if let Some(v) = session.manager_width_pct {
                self.session.manager_width_pct = v;
            }
            if let Some(v) = session.message_log_width_pct {
                self.session.message_log_width_pct = v;
            }
            if let Some(v) = session.max_workers_per_row {
                self.session.max_workers_per_row = v;
            }
        }

        if let Some(audit) = file_config.audit {
            if let Some(v) = audit.metadata_only {
                self.audit.metadata_only = v;
            }
            if let Some(v) = audit.debug_full_payload {
                self.audit.debug_full_payload = v;
            }
            if let Some(v) = audit.max_bytes {
                self.audit.max_bytes = v;
            }
            if let Some(v) = audit.max_age_days {
                self.audit.max_age_days = v;
            }
        }

        if let Some(lock) = file_config.lock
            && let Some(v) = lock.stale_timeout_minutes
        {
            self.lock.stale_timeout_minutes = v;
        }

        // Legacy flat keys for old config files.
        if let Some(v) = file_config.max_agent_panes {
            self.agent.max_parallel_workers = v;
        }
        if let Some(v) = file_config.progression_mode {
            self.progression_mode = v;
        }
        if let Some(v) = file_config.zellij_binary {
            self.paths.zellij_binary = v;
        }
        if let Some(v) = file_config.opencode_binary {
            self.agent.opencode_binary = v;
        }
        if let Some(v) = file_config.spec_kitty_binary {
            self.paths.spec_kitty_binary = v;
        }
        if let Some(v) = file_config.kasmos_dir {
            self.kasmos_dir = v;
        }
        if let Some(v) = file_config.poll_interval_secs {
            self.communication.poll_interval_secs = v;
        }
        if let Some(v) = file_config.debounce_ms {
            self.debounce_ms = v;
        }
        if let Some(v) = file_config.controller_width_pct {
            self.session.manager_width_pct = v;
        }
        if let Some(v) = file_config.opencode_profile {
            self.agent.opencode_profile = v;
        }

        self.sync_legacy_fields();
        Ok(())
    }

    /// Validate config ranges and invariants.
    pub fn validate(&self) -> Result<(), KasmosError> {
        if !(1..=16).contains(&self.agent.max_parallel_workers) {
            return Err(ConfigError::InvalidValue {
                field: "agent.max_parallel_workers".to_string(),
                value: self.agent.max_parallel_workers.to_string(),
                reason: "must be between 1 and 16".to_string(),
            }
            .into());
        }

        if !(10..=80).contains(&self.session.manager_width_pct) {
            return Err(ConfigError::InvalidValue {
                field: "session.manager_width_pct".to_string(),
                value: self.session.manager_width_pct.to_string(),
                reason: "must be between 10 and 80".to_string(),
            }
            .into());
        }

        if self.lock.stale_timeout_minutes < 1 {
            return Err(ConfigError::InvalidValue {
                field: "lock.stale_timeout_minutes".to_string(),
                value: self.lock.stale_timeout_minutes.to_string(),
                reason: "must be greater than or equal to 1".to_string(),
            }
            .into());
        }

        if self.audit.max_age_days < 1 {
            return Err(ConfigError::InvalidValue {
                field: "audit.max_age_days".to_string(),
                value: self.audit.max_age_days.to_string(),
                reason: "must be greater than or equal to 1".to_string(),
            }
            .into());
        }

        if self.agent.review_rejection_cap < 1 {
            return Err(ConfigError::InvalidValue {
                field: "agent.review_rejection_cap".to_string(),
                value: self.agent.review_rejection_cap.to_string(),
                reason: "must be greater than or equal to 1".to_string(),
            }
            .into());
        }

        if self.communication.poll_interval_secs == 0 {
            return Err(ConfigError::InvalidValue {
                field: "communication.poll_interval_secs".to_string(),
                value: self.communication.poll_interval_secs.to_string(),
                reason: "must be greater than 0".to_string(),
            }
            .into());
        }

        Ok(())
    }

    fn sync_legacy_fields(&mut self) {
        self.max_agent_panes = self.agent.max_parallel_workers;
        self.opencode_binary = self.agent.opencode_binary.clone();
        self.opencode_profile = self.agent.opencode_profile.clone();

        self.poll_interval_secs = self.communication.poll_interval_secs;

        self.zellij_binary = self.paths.zellij_binary.clone();
        self.spec_kitty_binary = self.paths.spec_kitty_binary.clone();

        self.controller_width_pct = self.session.manager_width_pct;
    }
}

#[derive(Debug, Deserialize)]
struct ConfigFile {
    agent: Option<AgentConfigFile>,
    communication: Option<CommunicationConfigFile>,
    paths: Option<PathsConfigFile>,
    session: Option<SessionConfigFile>,
    audit: Option<AuditConfigFile>,
    lock: Option<LockConfigFile>,

    // Legacy flat keys.
    max_agent_panes: Option<usize>,
    progression_mode: Option<ProgressionMode>,
    zellij_binary: Option<String>,
    opencode_binary: Option<String>,
    spec_kitty_binary: Option<String>,
    kasmos_dir: Option<String>,
    poll_interval_secs: Option<u64>,
    debounce_ms: Option<u64>,
    controller_width_pct: Option<u32>,
    opencode_profile: Option<Option<String>>,
}

#[derive(Debug, Deserialize)]
struct AgentConfigFile {
    max_parallel_workers: Option<usize>,
    opencode_binary: Option<String>,
    opencode_profile: Option<Option<String>>,
    review_rejection_cap: Option<u32>,
}

#[derive(Debug, Deserialize)]
struct CommunicationConfigFile {
    poll_interval_secs: Option<u64>,
    event_timeout_secs: Option<u64>,
}

#[derive(Debug, Deserialize)]
struct PathsConfigFile {
    zellij_binary: Option<String>,
    spec_kitty_binary: Option<String>,
    specs_root: Option<String>,
}

#[derive(Debug, Deserialize)]
struct SessionConfigFile {
    session_name: Option<String>,
    manager_width_pct: Option<u32>,
    message_log_width_pct: Option<u32>,
    max_workers_per_row: Option<usize>,
}

#[derive(Debug, Deserialize)]
struct AuditConfigFile {
    metadata_only: Option<bool>,
    debug_full_payload: Option<bool>,
    max_bytes: Option<u64>,
    max_age_days: Option<u32>,
}

#[derive(Debug, Deserialize)]
struct LockConfigFile {
    stale_timeout_minutes: Option<u64>,
}

fn discover_kasmos_toml() -> Option<PathBuf> {
    let cwd = std::env::current_dir().ok()?;
    for ancestor in cwd.ancestors() {
        let candidate = ancestor.join("kasmos.toml");
        if candidate.is_file() {
            return Some(candidate);
        }
    }
    None
}

fn read_env_usize(var: &str) -> Result<Option<usize>, KasmosError> {
    read_env_parse(var, |s| s.parse::<usize>())
}

fn read_env_u64(var: &str) -> Result<Option<u64>, KasmosError> {
    read_env_parse(var, |s| s.parse::<u64>())
}

fn read_env_u32(var: &str) -> Result<Option<u32>, KasmosError> {
    read_env_parse(var, |s| s.parse::<u32>())
}

fn read_env_bool(var: &str) -> Result<Option<bool>, KasmosError> {
    read_env_parse(var, parse_bool)
}

fn read_env_parse<T, E, F>(var: &str, parser: F) -> Result<Option<T>, KasmosError>
where
    F: FnOnce(&str) -> std::result::Result<T, E>,
    E: std::fmt::Display,
{
    match std::env::var(var) {
        Ok(val) => parser(&val).map(Some).map_err(|err| {
            ConfigError::InvalidValue {
                field: var.to_string(),
                value: val,
                reason: err.to_string(),
            }
            .into()
        }),
        Err(std::env::VarError::NotPresent) => Ok(None),
        Err(err) => Err(ConfigError::InvalidValue {
            field: var.to_string(),
            value: String::new(),
            reason: err.to_string(),
        }
        .into()),
    }
}

fn parse_bool(value: &str) -> Result<bool, &'static str> {
    match value.to_ascii_lowercase().as_str() {
        "1" | "true" | "yes" | "on" => Ok(true),
        "0" | "false" | "no" | "off" => Ok(false),
        _ => Err("must be a boolean (true/false)"),
    }
}

fn default_progression_mode() -> ProgressionMode {
    ProgressionMode::Continuous
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    static ENV_TEST_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn default_config_validates() {
        let config = Config::default();
        assert!(config.validate().is_ok());
        assert_eq!(config.agent.max_parallel_workers, 4);
        assert_eq!(config.paths.specs_root, "kitty-specs");
    }

    #[test]
    fn partial_toml_loads_with_defaults() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let path = tmp.path().join("kasmos.toml");
        std::fs::write(
            &path,
            r#"
[agent]
max_parallel_workers = 6

[lock]
stale_timeout_minutes = 30
"#,
        )
        .expect("write toml");

        let mut config = Config::default();
        config.load_from_file(&path).expect("load toml");

        assert_eq!(config.agent.max_parallel_workers, 6);
        assert_eq!(config.lock.stale_timeout_minutes, 30);
        assert_eq!(config.session.session_name, "kasmos");
    }

    #[test]
    fn invalid_values_produce_clear_errors() {
        let mut config = Config::default();
        config.agent.max_parallel_workers = 0;
        let err = config.validate().expect_err("validation should fail");
        assert!(err.to_string().contains("agent.max_parallel_workers"));
    }

    #[test]
    fn env_overrides_take_precedence() {
        let _guard = ENV_TEST_LOCK.lock().expect("env lock");

        let tmp = tempfile::tempdir().expect("create tempdir");
        let path = tmp.path().join("kasmos.toml");
        std::fs::write(
            &path,
            r#"
[agent]
max_parallel_workers = 2
"#,
        )
        .expect("write toml");

        let mut config = Config::default();
        config.load_from_file(&path).expect("load toml");

        unsafe {
            std::env::set_var("KASMOS_AGENT_MAX_PARALLEL_WORKERS", "9");
        }
        config.load_from_env().expect("load env");
        unsafe {
            std::env::remove_var("KASMOS_AGENT_MAX_PARALLEL_WORKERS");
        }

        assert_eq!(config.agent.max_parallel_workers, 9);
    }
}
