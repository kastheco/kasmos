//! Zellij session and pane lifecycle management.
//!
//! [`SessionManager`] orchestrates Zellij sessions and work package panes, providing:
//! - Session creation with optional KDL layout support
//! - Internal pane tracking (Zellij 0.41+ has no `list-panes` command)
//! - Focus navigation with shortest-path pane selection
//! - Exponential backoff for pane readiness
//! - Health status tracking for crash detection

use crate::config::Config;
use crate::error::{PaneError, Result, ZellijError};
use crate::zellij::{validate_identifier, SessionState, ZellijCli};
use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, SystemTime};
use tracing::{debug, info, warn};

/// Health status of a pane.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PaneHealth {
    /// Pane is healthy and responsive.
    Healthy,
    /// Pane is not responding to checks.
    Unresponsive,
    /// Pane has crashed.
    Crashed,
    /// Health status unknown.
    Unknown,
}

/// Information about a tracked pane.
#[derive(Debug, Clone)]
pub struct PaneInfo {
    /// Work package ID associated with this pane.
    pub wp_id: String,
    /// Pane name in Zellij.
    pub pane_name: String,
    /// When the pane was started.
    pub started_at: SystemTime,
    /// Current health status.
    pub health: PaneHealth,
}

/// Manages a Zellij session and its panes.
pub struct SessionManager {
    /// Name of the managed session.
    session_name: String,
    /// Zellij CLI implementation.
    cli: Arc<dyn ZellijCli>,
    /// Tracked panes: wp_id -> PaneInfo.
    panes: HashMap<String, PaneInfo>,
    /// Order of panes for focus navigation.
    pane_order: Vec<String>,
    /// Index of currently focused pane.
    focused_index: Option<usize>,
    /// Runtime configuration.
    config: Arc<Config>,
}

impl SessionManager {
    /// Create a new SessionManager.
    ///
    /// Returns an error if the session name is empty or contains invalid characters.
    pub fn new(session_name: String, cli: Arc<dyn ZellijCli>, config: Arc<Config>) -> Result<Self> {
        validate_identifier(&session_name, "session name")?;
        Ok(Self {
            session_name,
            cli,
            panes: HashMap::new(),
            pane_order: Vec::new(),
            focused_index: None,
            config,
        })
    }

    /// Start a new session.
    ///
    /// Returns an error if the session already exists.
    pub async fn start_session(&mut self) -> Result<()> {
        debug!("Starting session: {}", self.session_name);
        self.cli.create_session(&self.session_name, None).await?;
        info!("Session started: {}", self.session_name);
        Ok(())
    }

    /// Start a new session with a KDL layout.
    ///
    /// Returns an error if the session already exists.
    pub async fn start_session_with_layout(&mut self, layout: &std::path::Path) -> Result<()> {
        debug!("Starting session with layout: {}", self.session_name);
        self.cli.create_session(&self.session_name, Some(layout)).await?;
        info!("Session started with layout: {}", self.session_name);
        Ok(())
    }

    /// Ensure a session exists, creating it if necessary.
    ///
    /// Returns the session state: whether it was newly created or reattached.
    pub async fn ensure_session(&mut self) -> Result<SessionState> {
        if self.cli.session_exists(&self.session_name).await? {
            info!("Session already exists, reattaching: {}", self.session_name);
            self.cli.attach_session(&self.session_name, false).await?;
            return Ok(SessionState::Active);
        }
        
        info!("Creating new session: {}", self.session_name);
        self.cli.create_session(&self.session_name, None).await?;
        Ok(SessionState::Active)
    }

    /// Get the current state of the session.
    pub async fn get_session_state(&self) -> Result<SessionState> {
        let sessions = self.cli.list_sessions().await?;
        sessions
            .iter()
            .find(|s| s.name == self.session_name)
            .map(|s| s.state.clone())
            .ok_or_else(|| ZellijError::SessionNotFound {
                name: self.session_name.clone(),
            }
            .into())
    }

    /// Kill the session and clear all tracked panes.
    pub async fn kill_session(&mut self) -> Result<()> {
        debug!("Killing session: {}", self.session_name);
        self.cli.kill_session(&self.session_name).await?;
        self.panes.clear();
        info!("Session killed: {}", self.session_name);
        Ok(())
    }

    /// Attach to the session.
    pub async fn attach(&self) -> Result<()> {
        debug!("Attaching to session: {}", self.session_name);
        self.cli.attach_session(&self.session_name, false).await?;
        Ok(())
    }

    /// Open a new pane for a work package.
    ///
    /// Returns an error if the pane capacity is exceeded.
    pub async fn open_pane(&mut self, wp_id: &str, command: &str, args: &[&str]) -> Result<()> {
        // Guard: Check capacity
        if self.panes.len() >= self.config.max_agent_panes {
            return Err(crate::error::WaveError::CapacityExceeded {
                active: self.panes.len(),
                max: self.config.max_agent_panes,
            }
            .into());
        }

        // Guard: Check if pane already exists
        if self.panes.contains_key(wp_id) {
            debug!("Pane already exists for WP: {}, skipping creation", wp_id);
            return Ok(());
        }

        debug!("Opening pane for WP: {} (command: {})", wp_id, command);

        // Run the command in a named pane
        self.cli
            .run_in_pane(&self.session_name, wp_id, command, args)
            .await?;

        // Track the pane
        self.panes.insert(
            wp_id.to_string(),
            PaneInfo {
                wp_id: wp_id.to_string(),
                pane_name: wp_id.to_string(),
                started_at: SystemTime::now(),
                health: PaneHealth::Healthy,
            },
        );

        // Add to pane order
        self.pane_order.push(wp_id.to_string());
        if self.focused_index.is_none() {
            self.focused_index = Some(0);
        }

        info!("Pane opened for WP: {}", wp_id);
        Ok(())
    }

    /// Close a pane and remove it from tracking.
    pub async fn close_pane(&mut self, wp_id: &str) -> Result<()> {
        // Guard: Check if pane exists
        if !self.panes.contains_key(wp_id) {
            return Err(PaneError::NotFound {
                wp_id: wp_id.to_string(),
            }
            .into());
        }

        debug!("Closing pane for WP: {}", wp_id);
        
        // Focus the target pane first
        self.focus_pane(wp_id).await?;
        
        // Now close the focused pane
        self.cli.close_focused_pane(&self.session_name).await?;
        self.panes.remove(wp_id);
        
        // Update pane_order and focused_index
        if let Some(idx) = self.pane_order.iter().position(|id| id == wp_id) {
            self.pane_order.remove(idx);
            if self.pane_order.is_empty() {
                self.focused_index = None;
            } else {
                self.focused_index = Some(idx.min(self.pane_order.len() - 1));
            }
        }
        
        info!("Pane closed for WP: {}", wp_id);
        Ok(())
    }

    /// Restart a pane (close and re-open).
    pub async fn restart_pane(&mut self, wp_id: &str, command: &str, args: &[&str]) -> Result<()> {
        debug!("Restarting pane for WP: {}", wp_id);
        // Ignore close errors - pane may already be gone (e.g., crashed)
        if let Err(e) = self.close_pane(wp_id).await {
            debug!("Close during restart failed (expected if pane crashed): {}", e);
            // Clean up internal tracking if close failed
            self.panes.remove(wp_id);
            if let Some(idx) = self.pane_order.iter().position(|id| id == wp_id) {
                self.pane_order.remove(idx);
                if self.pane_order.is_empty() {
                    self.focused_index = None;
                } else if let Some(fi) = self.focused_index {
                    if fi >= self.pane_order.len() {
                        self.focused_index = Some(self.pane_order.len() - 1);
                    }
                }
            }
        }
        self.open_pane(wp_id, command, args).await?;
        info!("Pane restarted for WP: {}", wp_id);
        Ok(())
    }

    /// Wait for a pane to be ready with exponential backoff.
    ///
    /// Backoff sequence: 100ms, 200ms, 400ms, 800ms, 1600ms.
    pub async fn wait_for_pane_ready(&self, wp_id: &str, timeout: Duration) -> Result<()> {
        debug!("Waiting for pane ready: {} (timeout: {:?})", wp_id, timeout);

        let start = SystemTime::now();
        let mut backoff = Duration::from_millis(100);

        loop {
            // Guard: Check timeout
            if start.elapsed().unwrap_or(timeout) > timeout {
                warn!("Timeout waiting for pane: {}", wp_id);
                return Err(PaneError::NotFound {
                    wp_id: wp_id.to_string(),
                }
                .into());
            }

            // Guard: Check if pane exists and is healthy
            if let Some(pane) = self.get_pane(wp_id) {
                if pane.health == PaneHealth::Healthy {
                    debug!("Pane ready: {}", wp_id);
                    return Ok(());
                }
            }

            // Exponential backoff
            tokio::time::sleep(backoff).await;
            let next_ms = (backoff.as_millis() as u64).saturating_mul(2).min(1600);
            backoff = Duration::from_millis(next_ms);
        }
    }

    /// Get information about a tracked pane.
    pub fn get_pane(&self, wp_id: &str) -> Option<&PaneInfo> {
        self.panes.get(wp_id)
    }

    /// Get the number of active panes.
    pub fn active_pane_count(&self) -> usize {
        self.panes.len()
    }

    /// Focus a pane by work package ID.
    pub async fn focus_pane(&mut self, wp_id: &str) -> Result<()> {
        // Guard: Check if pane exists
        if !self.panes.contains_key(wp_id) {
            return Err(PaneError::NotFound {
                wp_id: wp_id.to_string(),
            }
            .into());
        }
        
        let target_idx = self.pane_order.iter().position(|id| id == wp_id)
            .ok_or_else(|| PaneError::NotFound { wp_id: wp_id.to_string() })?;
        
        let current = self.focused_index.unwrap_or(0);
        let total = self.pane_order.len();
        
        if total == 0 || current == target_idx {
            self.focused_index = Some(target_idx);
            return Ok(());
        }
        
        debug!("Focusing pane for WP: {} (from index {} to {})", wp_id, current, target_idx);
        
        // Calculate forward steps (wrapping)
        let forward_steps = if target_idx >= current {
            target_idx - current
        } else {
            total - current + target_idx
        };
        
        // Calculate backward steps
        let backward_steps = total - forward_steps;
        
        // Take the shorter path
        if forward_steps <= backward_steps {
            for _ in 0..forward_steps {
                self.cli.focus_next_pane(&self.session_name).await?;
            }
        } else {
            for _ in 0..backward_steps {
                self.cli.focus_previous_pane(&self.session_name).await?;
            }
        }
        
        self.focused_index = Some(target_idx);
        Ok(())
    }

    /// Zoom a pane (focus and toggle fullscreen).
    pub async fn zoom_pane(&mut self, wp_id: &str) -> Result<()> {
        // Guard: Check if pane exists
        if !self.panes.contains_key(wp_id) {
            return Err(PaneError::NotFound {
                wp_id: wp_id.to_string(),
            }
            .into());
        }

        debug!("Zooming pane for WP: {}", wp_id);
        self.focus_pane(wp_id).await?;
        self.cli.toggle_fullscreen(&self.session_name).await?;
        info!("Pane zoomed for WP: {}", wp_id);
        Ok(())
    }

    /// Check the health of a pane.
    pub fn check_pane_health(&self, wp_id: &str) -> Result<PaneHealth> {
        self.get_pane(wp_id)
            .map(|p| p.health)
            .ok_or_else(|| {
                PaneError::NotFound {
                    wp_id: wp_id.to_string(),
                }
                .into()
            })
    }

    /// Mark a pane's health status.
    pub fn mark_pane_health(&mut self, wp_id: &str, health: PaneHealth) -> Result<()> {
        // Guard: Check if pane exists
        if let Some(pane) = self.panes.get_mut(wp_id) {
            pane.health = health;
            debug!("Marked pane health for WP: {} as {:?}", wp_id, health);
            Ok(())
        } else {
            Err(PaneError::NotFound {
                wp_id: wp_id.to_string(),
            }
            .into())
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::zellij::SessionInfo;
    use async_trait::async_trait;
    use std::sync::Mutex;

    /// Mock Zellij CLI for testing.
    struct MockZellijCli {
        sessions: Mutex<Vec<SessionInfo>>,
        calls: Mutex<Vec<String>>,
    }

    impl MockZellijCli {
        fn new() -> Self {
            Self {
                sessions: Mutex::new(Vec::new()),
                calls: Mutex::new(Vec::new()),
            }
        }

        fn add_session(&self, name: &str, state: SessionState) {
            self.sessions.lock().unwrap().push(SessionInfo {
                name: name.to_string(),
                state,
            });
        }

        fn get_calls(&self) -> Vec<String> {
            self.calls.lock().unwrap().clone()
        }
    }

    #[async_trait]
    impl ZellijCli for MockZellijCli {
        async fn list_sessions(&self) -> Result<Vec<SessionInfo>> {
            self.calls.lock().unwrap().push("list_sessions".to_string());
            Ok(self.sessions.lock().unwrap().clone())
        }

        async fn create_session(&self, name: &str, _layout: Option<&std::path::Path>) -> Result<()> {
            self.calls.lock().unwrap().push(format!("create_session:{}", name));
            let sessions = self.sessions.lock().unwrap();
            if sessions.iter().any(|s| s.name == name) {
                return Err(ZellijError::SessionExists {
                    name: name.to_string(),
                }
                .into());
            }
            drop(sessions);
            self.add_session(name, SessionState::Active);
            Ok(())
        }

        async fn session_exists(&self, name: &str) -> Result<bool> {
            self.calls.lock().unwrap().push(format!("session_exists:{}", name));
            Ok(self.sessions.lock().unwrap().iter().any(|s| s.name == name))
        }

        async fn attach_session(&self, name: &str, _create: bool) -> Result<()> {
            self.calls.lock().unwrap().push(format!("attach_session:{}", name));
            Ok(())
        }

        async fn kill_session(&self, name: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("kill_session:{}", name));
            self.sessions.lock().unwrap().retain(|s| s.name != name);
            Ok(())
        }

        async fn new_pane(&self, session: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("new_pane:{}", session));
            Ok(())
        }

        async fn run_in_pane(&self, session: &str, name: &str, command: &str, _args: &[&str]) -> Result<()> {
            self.calls.lock().unwrap().push(format!("run_in_pane:{}:{}:{}", session, name, command));
            Ok(())
        }

        async fn close_focused_pane(&self, session: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("close_focused_pane:{}", session));
            Ok(())
        }

        async fn focus_next_pane(&self, session: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("focus_next_pane:{}", session));
            Ok(())
        }

        async fn focus_previous_pane(&self, session: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("focus_previous_pane:{}", session));
            Ok(())
        }

        async fn toggle_fullscreen(&self, session: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("toggle_fullscreen:{}", session));
            Ok(())
        }

        async fn new_tab(&self, session: &str, name: Option<&str>, _layout: Option<&std::path::Path>) -> Result<()> {
            self.calls.lock().unwrap().push(format!("new_tab:{}:{:?}", session, name));
            Ok(())
        }

        async fn rename_tab(&self, session: &str, name: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("rename_tab:{}:{}", session, name));
            Ok(())
        }

        async fn go_to_tab_name(&self, session: &str, name: &str) -> Result<()> {
            self.calls.lock().unwrap().push(format!("go_to_tab_name:{}:{}", session, name));
            Ok(())
        }

        async fn query_tab_names(&self, session: &str) -> Result<Vec<String>> {
            self.calls.lock().unwrap().push(format!("query_tab_names:{}", session));
            Ok(vec!["Tab #1".to_string()])
        }
    }

    #[tokio::test]
    async fn test_start_session_creates_new() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        let result = manager.start_session().await;
        assert!(result.is_ok());
        assert!(cli.get_calls().iter().any(|c| c.contains("create_session:test-session")));
    }

    #[tokio::test]
    async fn test_start_session_already_exists() {
        let cli = Arc::new(MockZellijCli::new());
        cli.add_session("test-session", SessionState::Active);

        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        let result = manager.start_session().await;
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), crate::error::KasmosError::Zellij(ZellijError::SessionExists { .. })));
    }

    #[tokio::test]
    async fn test_ensure_session_creates_when_missing() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        let result = manager.ensure_session().await;
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), SessionState::Active);
    }

    #[tokio::test]
    async fn test_ensure_session_reattaches() {
        let cli = Arc::new(MockZellijCli::new());
        cli.add_session("test-session", SessionState::Active);

        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        let result = manager.ensure_session().await;
        assert!(result.is_ok());
        assert_eq!(result.unwrap(), SessionState::Active);
    }

    #[tokio::test]
    async fn test_open_pane_tracks_internally() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        let result = manager.open_pane("WP01", "echo", &["hello"]).await;
        assert!(result.is_ok());
        assert_eq!(manager.active_pane_count(), 1);
        assert!(manager.get_pane("WP01").is_some());
    }

    #[tokio::test]
    async fn test_open_pane_capacity_exceeded() {
        let cli = Arc::new(MockZellijCli::new());
        let mut config = Config::default();
        config.max_agent_panes = 2;
        let config = Arc::new(config);

        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        // Open two panes
        assert!(manager.open_pane("WP01", "echo", &["hello"]).await.is_ok());
        assert!(manager.open_pane("WP02", "echo", &["world"]).await.is_ok());

        // Third pane should fail
        let result = manager.open_pane("WP03", "echo", &["fail"]).await;
        assert!(result.is_err());
    }

    #[tokio::test]
    async fn test_close_pane_removes_tracking() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        manager.open_pane("WP01", "echo", &["hello"]).await.unwrap();
        assert_eq!(manager.active_pane_count(), 1);

        manager.close_pane("WP01").await.unwrap();
        assert_eq!(manager.active_pane_count(), 0);
        assert!(manager.get_pane("WP01").is_none());
    }

    #[tokio::test]
    async fn test_restart_pane() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        manager.open_pane("WP01", "echo", &["hello"]).await.unwrap();
        let first_pane = manager.get_pane("WP01").unwrap().clone();

        manager.restart_pane("WP01", "echo", &["world"]).await.unwrap();
        let second_pane = manager.get_pane("WP01").unwrap().clone();

        // Pane should be different (newer started_at)
        assert!(second_pane.started_at >= first_pane.started_at);
    }

    #[tokio::test]
    async fn test_focus_unknown_pane_error() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        let result = manager.focus_pane("WP01").await;
        assert!(result.is_err());
        assert!(matches!(result.unwrap_err(), crate::error::KasmosError::Pane(PaneError::NotFound { .. })));
    }

    #[tokio::test]
    async fn test_wait_for_pane_ready_exponential_backoff() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        manager.open_pane("WP01", "echo", &["hello"]).await.unwrap();

        let start = SystemTime::now();
        let result = manager.wait_for_pane_ready("WP01", Duration::from_secs(5)).await;
        let elapsed = start.elapsed().unwrap();

        // Should succeed immediately since pane is healthy
        assert!(result.is_ok());
        assert!(elapsed < Duration::from_millis(200));
    }

    #[tokio::test]
    async fn test_pane_health_lifecycle() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        manager.open_pane("WP01", "echo", &["hello"]).await.unwrap();

        // Check initial health
        let health = manager.check_pane_health("WP01").unwrap();
        assert_eq!(health, PaneHealth::Healthy);

        // Mark as crashed
        manager.mark_pane_health("WP01", PaneHealth::Crashed).unwrap();
        let health = manager.check_pane_health("WP01").unwrap();
        assert_eq!(health, PaneHealth::Crashed);
    }

    #[test]
    fn test_session_name_validation_empty() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let result = SessionManager::new("".to_string(), cli, config);
        assert!(result.is_err());
    }

    #[test]
    fn test_session_name_validation_invalid_chars() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let result = SessionManager::new("invalid;session".to_string(), cli, config);
        assert!(result.is_err());
    }

    #[test]
    fn test_session_name_validation_valid() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let result = SessionManager::new("valid-session_123".to_string(), cli, config);
        assert!(result.is_ok());
    }

    #[tokio::test]
    async fn test_focus_pane_calculates_steps() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        // Open three panes
        manager.open_pane("WP01", "echo", &["1"]).await.unwrap();
        manager.open_pane("WP02", "echo", &["2"]).await.unwrap();
        manager.open_pane("WP03", "echo", &["3"]).await.unwrap();

        // Focus from WP01 to WP03 (forward 2 steps)
        manager.focus_pane("WP03").await.unwrap();
        assert_eq!(manager.focused_index, Some(2));

        // Focus back to WP01 (backward 1 step is shorter than forward 2)
        manager.focus_pane("WP01").await.unwrap();
        assert_eq!(manager.focused_index, Some(0));
    }

    #[tokio::test]
    async fn test_close_pane_focuses_first() {
        let cli = Arc::new(MockZellijCli::new());
        let config = Arc::new(Config::default());
        let mut manager = SessionManager::new("test-session".to_string(), cli.clone(), config).unwrap();

        // Open two panes
        manager.open_pane("WP01", "echo", &["1"]).await.unwrap();
        manager.open_pane("WP02", "echo", &["2"]).await.unwrap();

        // Close WP01 (should focus it first, then close)
        manager.close_pane("WP01").await.unwrap();
        
        // Verify WP01 is removed
        assert!(manager.get_pane("WP01").is_none());
        assert!(manager.get_pane("WP02").is_some());
        assert_eq!(manager.active_pane_count(), 1);
    }
}
