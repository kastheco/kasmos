//! Command handlers that dispatch controller commands to the orchestration engine.
//!
//! This module provides the CommandHandler that maps ControllerCommand variants
//! to EngineAction messages and session manager operations, producing user-facing
//! status messages and control feedback.

use crate::Result;
use crate::commands::{ControllerCommand, command_help_text};
use crate::types::OrchestrationRun;
use std::sync::Arc;
use tokio::sync::RwLock;
use tokio::sync::mpsc;

/// Actions dispatched to the wave engine for orchestration control.
#[derive(Debug, Clone)]
pub enum EngineAction {
    /// Restart a failed or crashed work package.
    Restart(String),
    /// Pause a running work package.
    Pause(String),
    /// Resume a paused work package.
    Resume(String),
    /// Skip a failed work package and unblock dependents.
    ForceAdvance(String),
    /// Re-run a failed work package from scratch.
    Retry(String),
    /// Confirm wave advancement (wave-gated mode).
    Advance,
    /// Gracefully abort the entire orchestration.
    Abort,
    /// Approve a reviewed work package (ForReview → Completed).
    Approve(String),
    /// Reject a reviewed work package.
    Reject {
        wp_id: String,
        /// If true, relaunch (ForReview → Active); if false, hold (ForReview → Pending).
        relaunch: bool,
    },
}

/// Trait for session manager operations (focus, zoom).
#[async_trait::async_trait]
pub trait SessionController: Send + Sync {
    /// Focus a specific work package pane.
    async fn focus_pane(&self, wp_id: &str) -> Result<()>;
    /// Focus and zoom a specific work package pane.
    async fn focus_and_zoom(&self, wp_id: &str) -> Result<()>;
}

/// Handles controller commands and dispatches them to the engine and session manager.
pub struct CommandHandler<S: SessionController> {
    /// The current orchestration run state.
    orchestration_run: Arc<RwLock<OrchestrationRun>>,
    /// Session controller for pane operations.
    session_controller: Arc<S>,
    /// Sender for engine actions.
    engine_tx: mpsc::Sender<EngineAction>,
}

impl<S: SessionController> CommandHandler<S> {
    /// Create a new command handler.
    pub fn new(
        orchestration_run: Arc<RwLock<OrchestrationRun>>,
        session_controller: Arc<S>,
        engine_tx: mpsc::Sender<EngineAction>,
    ) -> Self {
        Self {
            orchestration_run,
            session_controller,
            engine_tx,
        }
    }

    /// Handle a controller command and return a user-facing message.
    pub async fn handle(&self, cmd: ControllerCommand) -> Result<String> {
        match cmd {
            ControllerCommand::Restart { wp_id } => {
                self.engine_tx
                    .send(EngineAction::Restart(wp_id.clone()))
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Restarting {}...", wp_id))
            }
            ControllerCommand::Pause { wp_id } => {
                self.engine_tx
                    .send(EngineAction::Pause(wp_id.clone()))
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Pausing {}...", wp_id))
            }
            ControllerCommand::Resume { wp_id } => {
                self.engine_tx
                    .send(EngineAction::Resume(wp_id.clone()))
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Resuming {}...", wp_id))
            }
            ControllerCommand::Status => {
                let run = self.orchestration_run.read().await;
                Ok(Self::format_status(&run))
            }
            ControllerCommand::Focus { wp_id } => {
                self.session_controller.focus_pane(&wp_id).await?;
                Ok(format!("[kasmos] Focused on {}", wp_id))
            }
            ControllerCommand::Zoom { wp_id } => {
                self.session_controller.focus_and_zoom(&wp_id).await?;
                Ok(format!("[kasmos] Zoomed on {}", wp_id))
            }
            ControllerCommand::Abort => {
                self.engine_tx
                    .send(EngineAction::Abort)
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok("[kasmos] Aborting orchestration...".to_string())
            }
            ControllerCommand::Advance => {
                self.engine_tx
                    .send(EngineAction::Advance)
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok("[kasmos] Advancing to next wave...".to_string())
            }
            ControllerCommand::ForceAdvance { wp_id } => {
                self.engine_tx
                    .send(EngineAction::ForceAdvance(wp_id.clone()))
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Force-advancing {}...", wp_id))
            }
            ControllerCommand::Retry { wp_id } => {
                self.engine_tx
                    .send(EngineAction::Retry(wp_id.clone()))
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Retrying {}...", wp_id))
            }
            ControllerCommand::Approve { wp_id } => {
                self.engine_tx
                    .send(EngineAction::Approve(wp_id.clone()))
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Approving {}...", wp_id))
            }
            ControllerCommand::Reject { wp_id } => {
                self.engine_tx
                    .send(EngineAction::Reject {
                        wp_id: wp_id.clone(),
                        relaunch: true,
                    })
                    .await
                    .map_err(|e| crate::error::KasmosError::Other(anyhow::anyhow!(e)))?;
                Ok(format!("[kasmos] Rejecting {} (will relaunch)...", wp_id))
            }
            ControllerCommand::Help => Ok(command_help_text().to_string()),
            ControllerCommand::Unknown { input } => Ok(format!(
                "[kasmos] Unknown command: '{}'\nType 'help' for available commands.",
                input
            )),
        }
    }

    /// Format the orchestration status as a readable table.
    fn format_status(run: &OrchestrationRun) -> String {
        let mut output = String::new();
        output.push_str(&format!(
            "\n[kasmos] Orchestration Status: {}\n",
            run.feature
        ));
        output.push_str(&format!("Mode: {:?} | State: {:?}\n", run.mode, run.state));
        output.push_str(&"-".repeat(100));
        output.push('\n');

        // Header row: WP, State, Pane, Duration, Wave
        output.push_str(&format!(
            "{:<8} {:<12} {:<12} {:<10} {:<8}\n",
            "WP", "State", "Pane", "Duration", "Wave"
        ));
        output.push_str(&"-".repeat(100));
        output.push('\n');

        // Work package rows
        for wp in &run.work_packages {
            let pane_str = wp
                .pane_id
                .map(|id| format!("{}", id))
                .unwrap_or_else(|| "-".to_string());

            let duration = match (&wp.started_at, &wp.completed_at) {
                (Some(start), Some(end)) => match end.duration_since(*start) {
                    Ok(d) => format_duration(d),
                    Err(_) => "-".to_string(),
                },
                (Some(start), None) => match start.elapsed() {
                    Ok(d) => format!("{}...", format_duration(d)),
                    Err(_) => "-".to_string(),
                },
                _ => "-".to_string(),
            };

            output.push_str(&format!(
                "{:<8} {:<12} {:<12} {:<10} {:<8}\n",
                wp.id,
                format!("{:?}", wp.state),
                pane_str,
                duration,
                wp.wave
            ));
        }

        output.push_str(&"-".repeat(100));
        output.push('\n');
        output
    }
}

/// Format a duration as a human-readable string.
fn format_duration(duration: std::time::Duration) -> String {
    let secs = duration.as_secs();
    if secs < 60 {
        format!("{}s", secs)
    } else if secs < 3600 {
        format!("{}m{}s", secs / 60, secs % 60)
    } else {
        format!("{}h{}m", secs / 3600, (secs % 3600) / 60)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::types::{ProgressionMode, RunState, WPState, Wave, WaveState};
    use std::time::SystemTime;

    /// Mock session controller for testing.
    struct MockSessionController {
        focus_called: std::sync::Mutex<Option<String>>,
        zoom_called: std::sync::Mutex<Option<String>>,
    }

    impl MockSessionController {
        fn new() -> Self {
            Self {
                focus_called: std::sync::Mutex::new(None),
                zoom_called: std::sync::Mutex::new(None),
            }
        }
    }

    #[async_trait::async_trait]
    impl SessionController for MockSessionController {
        async fn focus_pane(&self, wp_id: &str) -> Result<()> {
            *self.focus_called.lock().unwrap() = Some(wp_id.to_string());
            Ok(())
        }

        async fn focus_and_zoom(&self, wp_id: &str) -> Result<()> {
            *self.zoom_called.lock().unwrap() = Some(wp_id.to_string());
            Ok(())
        }
    }

    fn create_test_run() -> OrchestrationRun {
        OrchestrationRun {
            id: "test-run".to_string(),
            feature: "test-feature".to_string(),
            feature_dir: "/tmp/test".into(),
            config: crate::config::Config::default(),
            work_packages: vec![
                crate::types::WorkPackage {
                    id: "WP01".to_string(),
                    title: "First Work Package".to_string(),
                    state: WPState::Active,
                    dependencies: vec![],
                    wave: 0,
                    pane_id: Some(1),
                    pane_name: "wp01".to_string(),
                    worktree_path: None,
                    prompt_path: None,
                    started_at: Some(SystemTime::now()),
                    completed_at: None,
                    completion_method: None,
                    failure_count: 0,
                },
                crate::types::WorkPackage {
                    id: "WP02".to_string(),
                    title: "Second Work Package".to_string(),
                    state: WPState::Pending,
                    dependencies: vec!["WP01".to_string()],
                    wave: 1,
                    pane_id: None,
                    pane_name: "wp02".to_string(),
                    worktree_path: None,
                    prompt_path: None,
                    started_at: None,
                    completed_at: None,
                    completion_method: None,
                    failure_count: 0,
                },
            ],
            waves: vec![
                Wave {
                    index: 0,
                    wp_ids: vec!["WP01".to_string()],
                    state: WaveState::Active,
                },
                Wave {
                    index: 1,
                    wp_ids: vec!["WP02".to_string()],
                    state: WaveState::Pending,
                },
            ],
            state: RunState::Running,
            started_at: Some(SystemTime::now()),
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[tokio::test]
    async fn test_handle_restart_command() {
        let (tx, mut rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler
            .handle(ControllerCommand::Restart {
                wp_id: "WP01".to_string(),
            })
            .await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Restarting WP01"));

        let action = rx.recv().await;
        assert!(action.is_some());
        match action.unwrap() {
            EngineAction::Restart(wp_id) => assert_eq!(wp_id, "WP01"),
            _ => panic!("Expected Restart action"),
        }
    }

    #[tokio::test]
    async fn test_handle_pause_command() {
        let (tx, mut rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler
            .handle(ControllerCommand::Pause {
                wp_id: "WP01".to_string(),
            })
            .await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Pausing WP01"));

        let action = rx.recv().await;
        assert!(action.is_some());
        match action.unwrap() {
            EngineAction::Pause(wp_id) => assert_eq!(wp_id, "WP01"),
            _ => panic!("Expected Pause action"),
        }
    }

    #[tokio::test]
    async fn test_handle_status_command() {
        let (tx, _rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler.handle(ControllerCommand::Status).await;

        assert!(result.is_ok());
        let status = result.unwrap();
        assert!(status.contains("Orchestration Status"));
        assert!(status.contains("WP01"));
        assert!(status.contains("WP02"));
        assert!(status.contains("Active"));
        assert!(status.contains("Pending"));
    }

    #[tokio::test]
    async fn test_handle_focus_command() {
        let (tx, _rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session.clone(), tx);

        let result = handler
            .handle(ControllerCommand::Focus {
                wp_id: "WP01".to_string(),
            })
            .await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Focused on WP01"));
        assert_eq!(
            *session.focus_called.lock().unwrap(),
            Some("WP01".to_string())
        );
    }

    #[tokio::test]
    async fn test_handle_zoom_command() {
        let (tx, _rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session.clone(), tx);

        let result = handler
            .handle(ControllerCommand::Zoom {
                wp_id: "WP02".to_string(),
            })
            .await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Zoomed on WP02"));
        assert_eq!(
            *session.zoom_called.lock().unwrap(),
            Some("WP02".to_string())
        );
    }

    #[tokio::test]
    async fn test_handle_abort_command() {
        let (tx, mut rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler.handle(ControllerCommand::Abort).await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Aborting"));

        let action = rx.recv().await;
        assert!(action.is_some());
        match action.unwrap() {
            EngineAction::Abort => (),
            _ => panic!("Expected Abort action"),
        }
    }

    #[tokio::test]
    async fn test_handle_advance_command() {
        let (tx, mut rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler.handle(ControllerCommand::Advance).await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Advancing"));

        let action = rx.recv().await;
        assert!(action.is_some());
        match action.unwrap() {
            EngineAction::Advance => (),
            _ => panic!("Expected Advance action"),
        }
    }

    #[tokio::test]
    async fn test_handle_force_advance_command() {
        let (tx, mut rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler
            .handle(ControllerCommand::ForceAdvance {
                wp_id: "WP01".to_string(),
            })
            .await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Force-advancing WP01"));

        let action = rx.recv().await;
        assert!(action.is_some());
        match action.unwrap() {
            EngineAction::ForceAdvance(wp_id) => assert_eq!(wp_id, "WP01"),
            _ => panic!("Expected ForceAdvance action"),
        }
    }

    #[tokio::test]
    async fn test_handle_retry_command() {
        let (tx, mut rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler
            .handle(ControllerCommand::Retry {
                wp_id: "WP01".to_string(),
            })
            .await;

        assert!(result.is_ok());
        assert!(result.unwrap().contains("Retrying WP01"));

        let action = rx.recv().await;
        assert!(action.is_some());
        match action.unwrap() {
            EngineAction::Retry(wp_id) => assert_eq!(wp_id, "WP01"),
            _ => panic!("Expected Retry action"),
        }
    }

    #[tokio::test]
    async fn test_handle_help_command() {
        let (tx, _rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler.handle(ControllerCommand::Help).await;

        assert!(result.is_ok());
        let help = result.unwrap();
        assert!(help.contains("Available Commands"));
        assert!(help.contains("restart"));
        assert!(help.contains("status"));
    }

    #[tokio::test]
    async fn test_handle_unknown_command() {
        let (tx, _rx) = mpsc::channel(10);
        let run = Arc::new(RwLock::new(create_test_run()));
        let session = Arc::new(MockSessionController::new());
        let handler = CommandHandler::new(run, session, tx);

        let result = handler
            .handle(ControllerCommand::Unknown {
                input: "invalid_cmd".to_string(),
            })
            .await;

        assert!(result.is_ok());
        let msg = result.unwrap();
        assert!(msg.contains("Unknown command"));
        assert!(msg.contains("invalid_cmd"));
    }

    #[test]
    fn test_format_duration() {
        assert_eq!(format_duration(std::time::Duration::from_secs(30)), "30s");
        assert_eq!(format_duration(std::time::Duration::from_secs(90)), "1m30s");
        assert_eq!(
            format_duration(std::time::Duration::from_secs(3661)),
            "1h1m"
        );
    }

    #[test]
    fn test_status_formatting() {
        let run = create_test_run();
        let status = CommandHandler::<MockSessionController>::format_status(&run);

        assert!(status.contains("Orchestration Status"));
        assert!(status.contains("test-feature"));
        assert!(status.contains("WP01"));
        assert!(status.contains("WP02"));
        assert!(status.contains("Active"));
        assert!(status.contains("Pending"));
    }
}
