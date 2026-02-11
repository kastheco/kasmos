//! Zellij CLI abstraction layer.
//!
//! Provides the [`ZellijCli`] trait that abstracts all Zellij terminal multiplexer
//! interactions, and [`RealZellijCli`] which implements it using `tokio::process::Command`.
//!
//! ## Zellij 0.41+ Adaptations
//!
//! The original spec assumed several Zellij CLI commands that don't exist in 0.41+:
//! - **`list-panes`**: Does not exist. Panes are tracked internally via `HashMap` in
//!   [`SessionManager`](crate::session::SessionManager).
//! - **`focus-terminal-pane --pane-id`**: Does not exist. Focus navigation uses
//!   `focus-next-pane`/`focus-previous-pane` with shortest-path calculation.
//! - **`write-chars-to-pane-id`**: Does not exist. Commands are launched in new panes
//!   via `zellij run -n <name> -- <command>`.
//!
//! These adaptations were validated by Zellij CLI research and are the correct approach
//! for the available CLI surface.

use crate::error::{Result, ZellijError};
use async_trait::async_trait;
use std::process::Stdio;
use tokio::process::Command;
use tracing::{debug, warn};

/// Validate that a string is a safe identifier (session name, pane name, wp_id).
/// Only allows alphanumeric, hyphens, and underscores.
pub(crate) fn validate_identifier(s: &str, context: &str) -> Result<()> {
    if s.is_empty() {
        return Err(ZellijError::PaneOperation(format!("{} cannot be empty", context)).into());
    }
    if !s.chars().all(|c| c.is_alphanumeric() || c == '-' || c == '_') {
        return Err(ZellijError::PaneOperation(
            format!("Invalid {}: '{}' — only alphanumeric, hyphens, and underscores allowed", context, s)
        ).into());
    }
    Ok(())
}

/// Check if a string contains shell metacharacters.
fn contains_shell_metacharacters(s: &str) -> bool {
    s.contains(&[';', '|', '&', '$', '`', '(', ')', '<', '>', '\n', '\r', '\'', '"'][..])
}

/// Information about a Zellij session.
#[derive(Debug, Clone, PartialEq)]
pub struct SessionInfo {
    /// Session name.
    pub name: String,
    /// Current session state.
    pub state: SessionState,
}

/// State of a Zellij session.
#[derive(Debug, Clone, PartialEq)]
pub enum SessionState {
    /// Session is active and running.
    Active,
    /// Session has exited but can be resurrected.
    Exited,
}

/// Abstraction over Zellij CLI operations.
///
/// This trait enables both real CLI interactions and mock implementations for testing.
#[async_trait]
pub trait ZellijCli: Send + Sync {
    /// List all sessions.
    async fn list_sessions(&self) -> Result<Vec<SessionInfo>>;

    /// Create a new session with optional KDL layout.
    async fn create_session(&self, name: &str, layout: Option<&std::path::Path>) -> Result<()>;

    /// Check if a session exists.
    async fn session_exists(&self, name: &str) -> Result<bool>;

    /// Attach to a session, optionally creating it if it doesn't exist.
    async fn attach_session(&self, name: &str, create: bool) -> Result<()>;

    /// Kill a session.
    async fn kill_session(&self, name: &str) -> Result<()>;

    /// Create a new pane in a session.
    async fn new_pane(&self, session: &str) -> Result<()>;

    /// Run a command in a named pane within a session.
    async fn run_in_pane(&self, session: &str, name: &str, command: &str, args: &[&str]) -> Result<()>;

    /// Close the focused pane in a session.
    async fn close_focused_pane(&self, session: &str) -> Result<()>;

    /// Focus the next pane in a session.
    async fn focus_next_pane(&self, session: &str) -> Result<()>;

    /// Focus the previous pane in a session.
    async fn focus_previous_pane(&self, session: &str) -> Result<()>;

    /// Toggle fullscreen for the focused pane.
    async fn toggle_fullscreen(&self, session: &str) -> Result<()>;
}

/// Real Zellij CLI implementation using tokio::process::Command.
pub struct RealZellijCli {
    zellij_binary: String,
}

impl RealZellijCli {
    /// Create a new RealZellijCli instance.
    pub fn new(zellij_binary: String) -> Self {
        Self { zellij_binary }
    }

    /// Run a Zellij command and return its output.
    async fn run_command(&self, args: &[&str]) -> Result<String> {
        debug!("Running zellij command: {:?}", args);

        let output = Command::new(&self.zellij_binary)
            .args(args)
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .output()
            .await
            .map_err(|e| -> crate::error::KasmosError {
                if e.kind() == std::io::ErrorKind::NotFound {
                    warn!("Zellij binary not found: {}", self.zellij_binary);
                    ZellijError::NotFound.into()
                } else {
                    warn!("Failed to execute zellij: {}", e);
                    ZellijError::CreateFailed(format!("Failed to execute zellij: {}", e)).into()
                }
            })?;

        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            warn!("Zellij command failed: {}", stderr);
            return Err(ZellijError::PaneOperation(stderr.to_string()).into());
        }

        Ok(String::from_utf8_lossy(&output.stdout).to_string())
    }
}

/// Strip ANSI escape sequences from a string.
fn strip_ansi(s: &str) -> String {
    let mut result = String::with_capacity(s.len());
    let mut chars = s.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\x1b' {
            // Skip ESC [ ... (letter) sequences
            if chars.peek() == Some(&'[') {
                chars.next();
                while let Some(&next) = chars.peek() {
                    chars.next();
                    if next.is_ascii_alphabetic() {
                        break;
                    }
                }
            }
        } else {
            result.push(c);
        }
    }
    result
}

/// Parse the output of `zellij list-sessions`.
///
/// Handles both legacy format and Zellij 0.44+ format (with ANSI codes):
/// - Legacy: `session-name`, `session-name (current)`, `session-name EXITED`
/// - 0.44+:  `session-name [Created Xh Ym Zs ago]`,
///           `session-name [Created Xh Ym Zs ago] (EXITED - attach to resurrect)`
fn parse_list_sessions(output: &str) -> Vec<SessionInfo> {
    let clean = strip_ansi(output);
    clean
        .lines()
        .filter(|line| {
            let trimmed = line.trim();
            !trimmed.is_empty() && !trimmed.contains("No active zellij sessions found")
        })
        .filter_map(|line| {
            let trimmed = line.trim();
            // Extract session name: first token before ` [` (0.44+) or ` (` or ` EXITED` (legacy)
            let name = trimmed
                .split(&[' ', '\t'][..])
                .next()?
                .trim()
                .to_string();
            if name.is_empty() {
                return None;
            }
            let state = if trimmed.contains("EXITED") {
                SessionState::Exited
            } else {
                SessionState::Active
            };
            Some(SessionInfo { name, state })
        })
        .collect()
}

#[async_trait]
impl ZellijCli for RealZellijCli {
    async fn list_sessions(&self) -> Result<Vec<SessionInfo>> {
        let output = self.run_command(&["list-sessions"]).await?;
        Ok(parse_list_sessions(&output))
    }

    async fn create_session(&self, name: &str, layout: Option<&std::path::Path>) -> Result<()> {
        validate_identifier(name, "session name")?;
        debug!("Creating Zellij session: {}", name);

        // Note: TOCTOU race is acceptable for single-user orchestrator
        let sessions = self.list_sessions().await?;
        if sessions.iter().any(|s| s.name == name) {
            return Err(ZellijError::SessionExists { name: name.to_string() }.into());
        }

        // Use `attach --create-background` for detached session creation.
        // Plain `zellij --session <name>` tries to attach interactively (needs TTY).
        let mut args: Vec<&str> = Vec::new();
        let layout_str;
        if let Some(layout_path) = layout {
            layout_str = layout_path.display().to_string();
            args.push("--layout");
            args.push(&layout_str);
        }
        args.extend_from_slice(&["attach", "--create-background", name]);

        self.run_command(&args).await?;
        debug!("Session created: {}", name);
        Ok(())
    }

    async fn session_exists(&self, name: &str) -> Result<bool> {
        validate_identifier(name, "session name")?;
        let sessions = self.list_sessions().await?;
        Ok(sessions.iter().any(|s| s.name == name))
    }

    async fn attach_session(&self, name: &str, create: bool) -> Result<()> {
        validate_identifier(name, "session name")?;
        
        let mut args = vec!["attach"];
        if create {
            args.push("--create");
        }
        args.push(name);

        self.run_command(&args).await?;
        debug!("Attached to session: {}", name);
        Ok(())
    }

    async fn kill_session(&self, name: &str) -> Result<()> {
        validate_identifier(name, "session name")?;
        
        self.run_command(&["kill-sessions", name]).await?;
        debug!("Killed session: {}", name);
        Ok(())
    }

    async fn new_pane(&self, session: &str) -> Result<()> {
        validate_identifier(session, "session name")?;
        
        self.run_command(&["--session", session, "action", "new-pane"])
            .await?;
        debug!("Created new pane in session: {}", session);
        Ok(())
    }

    async fn run_in_pane(&self, session: &str, name: &str, command: &str, args: &[&str]) -> Result<()> {
        validate_identifier(session, "session name")?;
        validate_identifier(name, "pane name")?;
        
        if contains_shell_metacharacters(command) {
            return Err(ZellijError::PaneOperation(
                format!("Command contains shell metacharacters: '{}'", command)
            ).into());
        }
        
        for arg in args {
            if contains_shell_metacharacters(arg) {
                return Err(ZellijError::PaneOperation(
                    format!("Argument contains shell metacharacters: '{}'", arg)
                ).into());
            }
        }
        
        let mut cmd_args = vec!["--session", session, "run", "-n", name, "--", command];
        cmd_args.extend_from_slice(args);

        self.run_command(&cmd_args).await?;
        debug!("Ran command in pane '{}' of session '{}': {}", name, session, command);
        Ok(())
    }

    async fn close_focused_pane(&self, session: &str) -> Result<()> {
        validate_identifier(session, "session name")?;
        
        self.run_command(&["--session", session, "action", "close-pane"])
            .await?;
        debug!("Closed focused pane in session: {}", session);
        Ok(())
    }

    async fn focus_next_pane(&self, session: &str) -> Result<()> {
        validate_identifier(session, "session name")?;
        
        self.run_command(&["--session", session, "action", "focus-next-pane"])
            .await?;
        debug!("Focused next pane in session: {}", session);
        Ok(())
    }

    async fn focus_previous_pane(&self, session: &str) -> Result<()> {
        validate_identifier(session, "session name")?;
        
        self.run_command(&["--session", session, "action", "focus-previous-pane"])
            .await?;
        debug!("Focused previous pane in session: {}", session);
        Ok(())
    }

    async fn toggle_fullscreen(&self, session: &str) -> Result<()> {
        validate_identifier(session, "session name")?;
        
        self.run_command(&["--session", session, "action", "ToggleFocusFullscreen"])
            .await?;
        debug!("Toggled fullscreen in session: {}", session);
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_list_sessions_empty() {
        let output = "";
        let sessions = parse_list_sessions(output);
        assert_eq!(sessions, vec![]);
    }

    #[test]
    fn test_parse_list_sessions_no_active() {
        let output = "No active zellij sessions found.";
        let sessions = parse_list_sessions(output);
        assert_eq!(sessions, vec![]);
    }

    #[test]
    fn test_parse_list_sessions_single_active() {
        let output = "my-session";
        let sessions = parse_list_sessions(output);
        assert_eq!(
            sessions,
            vec![SessionInfo {
                name: "my-session".to_string(),
                state: SessionState::Active,
            }]
        );
    }

    #[test]
    fn test_parse_list_sessions_with_current() {
        let output = "my-session (current)";
        let sessions = parse_list_sessions(output);
        assert_eq!(
            sessions,
            vec![SessionInfo {
                name: "my-session".to_string(),
                state: SessionState::Active,
            }]
        );
    }

    #[test]
    fn test_parse_list_sessions_exited() {
        let output = "old-session EXITED";
        let sessions = parse_list_sessions(output);
        assert_eq!(
            sessions,
            vec![SessionInfo {
                name: "old-session".to_string(),
                state: SessionState::Exited,
            }]
        );
    }

    #[test]
    fn test_parse_list_sessions_multiple() {
        let output = "session1\nsession2 (current)\nold-session EXITED";
        let sessions = parse_list_sessions(output);
        assert_eq!(
            sessions,
            vec![
                SessionInfo {
                    name: "session1".to_string(),
                    state: SessionState::Active,
                },
                SessionInfo {
                    name: "session2".to_string(),
                    state: SessionState::Active,
                },
                SessionInfo {
                    name: "old-session".to_string(),
                    state: SessionState::Exited,
                },
            ]
        );
    }

    #[test]
    fn test_parse_list_sessions_zellij_044_format() {
        // Zellij 0.44+ outputs ANSI codes and different format
        let output = "\x1b[32;1mkasmos-feat\x1b[m [Created \x1b[35;1m21h 25m 24s\x1b[m ago] (\x1b[31;1mEXITED\x1b[m - attach to resurrect)\n\x1b[32;1mkasmos-probe\x1b[m [Created \x1b[35;1m2h 8m 3s\x1b[m ago] \n";
        let sessions = parse_list_sessions(output);
        assert_eq!(
            sessions,
            vec![
                SessionInfo {
                    name: "kasmos-feat".to_string(),
                    state: SessionState::Exited,
                },
                SessionInfo {
                    name: "kasmos-probe".to_string(),
                    state: SessionState::Active,
                },
            ]
        );
    }

    #[test]
    fn test_strip_ansi_codes() {
        let input = "\x1b[32;1mhello\x1b[m world";
        assert_eq!(strip_ansi(input), "hello world");
    }

    #[test]
    fn test_validate_identifier_valid() {
        assert!(validate_identifier("valid-session", "session name").is_ok());
        assert!(validate_identifier("valid_session", "session name").is_ok());
        assert!(validate_identifier("ValidSession123", "session name").is_ok());
    }

    #[test]
    fn test_validate_identifier_invalid() {
        assert!(validate_identifier("", "session name").is_err());
        assert!(validate_identifier("invalid.session", "session name").is_err());
        assert!(validate_identifier("invalid;session", "session name").is_err());
        assert!(validate_identifier("invalid|session", "session name").is_err());
        assert!(validate_identifier("invalid&session", "session name").is_err());
        assert!(validate_identifier("invalid$session", "session name").is_err());
        assert!(validate_identifier("invalid`session", "session name").is_err());
        assert!(validate_identifier("invalid(session)", "session name").is_err());
        assert!(validate_identifier("invalid<session>", "session name").is_err());
        assert!(validate_identifier("invalid'session'", "session name").is_err());
        assert!(validate_identifier("invalid\"session\"", "session name").is_err());
    }

    #[test]
    fn test_contains_shell_metacharacters() {
        assert!(contains_shell_metacharacters("echo; rm -rf /"));
        assert!(contains_shell_metacharacters("echo | cat"));
        assert!(contains_shell_metacharacters("echo & background"));
        assert!(contains_shell_metacharacters("echo $VAR"));
        assert!(contains_shell_metacharacters("echo `date`"));
        assert!(contains_shell_metacharacters("echo (subshell)"));
        assert!(contains_shell_metacharacters("echo <input"));
        assert!(contains_shell_metacharacters("echo >output"));
        assert!(contains_shell_metacharacters("echo 'quoted'"));
        assert!(contains_shell_metacharacters("echo \"quoted\""));
        assert!(!contains_shell_metacharacters("echo hello"));
        assert!(!contains_shell_metacharacters("echo-with-dashes"));
        assert!(!contains_shell_metacharacters("echo_with_underscores"));
    }
}
