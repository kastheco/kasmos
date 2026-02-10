//! Ordered shutdown coordination.

use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::sync::atomic::{AtomicBool, Ordering};
use tokio::sync::watch;

/// Coordinates ordered shutdown of all orchestrator subsystems.
pub struct ShutdownCoordinator {
    shutdown_flag: Arc<AtomicBool>,
    shutdown_tx: watch::Sender<bool>,
    kasmos_dir: PathBuf,
}

/// Trait for session operations needed during shutdown.
#[async_trait::async_trait]
pub trait ShutdownSession: Send + Sync {
    /// Close a specific pane by name.
    async fn close_pane(&self, pane_name: &str) -> crate::error::Result<()>;
    /// Kill the entire Zellij session.
    async fn kill_session(&self) -> crate::error::Result<()>;
    /// List all active pane names.
    async fn list_pane_names(&self) -> crate::error::Result<Vec<String>>;
}

impl ShutdownCoordinator {
    pub fn new(kasmos_dir: &Path) -> (Self, watch::Receiver<bool>) {
        let shutdown_flag = Arc::new(AtomicBool::new(false));
        let (shutdown_tx, shutdown_rx) = watch::channel(false);

        (
            Self {
                shutdown_flag,
                shutdown_tx,
                kasmos_dir: kasmos_dir.to_path_buf(),
            },
            shutdown_rx,
        )
    }

    /// Returns a clone of the shutdown flag for signal handlers.
    pub fn flag(&self) -> Arc<AtomicBool> {
        self.shutdown_flag.clone()
    }

    /// Trigger shutdown — notifies all watchers.
    pub fn trigger(&self) {
        tracing::info!("Shutdown triggered");
        self.shutdown_flag.store(true, Ordering::SeqCst);
        let _ = self.shutdown_tx.send(true);
    }

    /// Check if shutdown has been requested.
    pub fn is_shutdown(&self) -> bool {
        self.shutdown_flag.load(Ordering::SeqCst)
    }

    /// Execute ordered shutdown sequence.
    ///
    /// Steps (in order):
    /// 1. Stop filesystem watchers (signal via shutdown_tx — already done by trigger)
    /// 2. Stop health monitor (signal via shutdown_tx — already done by trigger)
    /// 3. Close command FIFO
    /// 4. Persist final state
    /// 5. Close all panes
    /// 6. Kill zellij session
    pub async fn execute<S: ShutdownSession>(
        &self,
        session: &S,
        run: Option<&crate::types::OrchestrationRun>,
    ) -> crate::error::Result<()> {
        tracing::info!("Beginning ordered shutdown sequence");

        // Step 1 & 2: Watchers and health monitor stop via shutdown_tx (already triggered)
        tracing::info!("[1/6] Filesystem watchers stopped");
        tracing::info!("[2/6] Health monitor stopped");

        // Step 3: Close command FIFO
        self.close_fifo();
        tracing::info!("[3/6] Command FIFO closed");

        // Step 4: Persist final state
        if let Some(run) = run {
            self.persist_final_state(run)?;
            tracing::info!("[4/6] Final state persisted");
        } else {
            tracing::info!("[4/6] No state to persist");
        }

        // Step 5: Close all panes
        match session.list_pane_names().await {
            Ok(panes) => {
                for pane_name in &panes {
                    if let Err(e) = session.close_pane(pane_name).await {
                        tracing::warn!(pane = %pane_name, error = %e, "Failed to close pane");
                    }
                }
                tracing::info!(count = panes.len(), "[5/6] Panes closed");
            }
            Err(e) => {
                tracing::warn!(error = %e, "[5/6] Failed to list panes for cleanup");
            }
        }

        // Step 6: Kill zellij session
        match session.kill_session().await {
            Ok(()) => tracing::info!("[6/6] Zellij session killed"),
            Err(e) => tracing::warn!(error = %e, "[6/6] Failed to kill session"),
        }

        tracing::info!("Shutdown sequence complete");
        Ok(())
    }

    fn close_fifo(&self) {
        let fifo_path = self.kasmos_dir.join("cmd.pipe");
        match std::fs::remove_file(&fifo_path) {
            Ok(()) => {}
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
            Err(e) => {
                tracing::warn!(path = %fifo_path.display(), error = %e, "Failed to remove FIFO");
            }
        }
    }

    fn persist_final_state(
        &self,
        run: &crate::types::OrchestrationRun,
    ) -> crate::error::Result<()> {
        let state_path = self.kasmos_dir.join("state.json");
        let tmp_path = self.kasmos_dir.join("state.json.tmp");

        let json = serde_json::to_string_pretty(run).map_err(|e| {
            crate::error::StateError::Corrupted(format!("Final state serialization: {}", e))
        })?;

        std::fs::write(&tmp_path, &json)?;
        std::fs::rename(&tmp_path, &state_path)?;

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;
    use tokio::sync::Mutex;

    struct MockSession {
        closed_panes: Arc<Mutex<Vec<String>>>,
        session_killed: Arc<AtomicBool>,
        pane_names: Vec<String>,
    }

    #[async_trait::async_trait]
    impl ShutdownSession for MockSession {
        async fn close_pane(&self, pane_name: &str) -> crate::error::Result<()> {
            self.closed_panes.lock().await.push(pane_name.to_string());
            Ok(())
        }

        async fn kill_session(&self) -> crate::error::Result<()> {
            self.session_killed.store(true, Ordering::SeqCst);
            Ok(())
        }

        async fn list_pane_names(&self) -> crate::error::Result<Vec<String>> {
            Ok(self.pane_names.clone())
        }
    }

    #[test]
    fn test_trigger_sets_flag() {
        let (coordinator, _rx) = ShutdownCoordinator::new(Path::new("/tmp/test-kasmos"));
        assert!(!coordinator.is_shutdown());

        coordinator.trigger();
        assert!(coordinator.is_shutdown());
    }

    #[tokio::test]
    async fn test_shutdown_rx_notified() {
        let (coordinator, mut rx) = ShutdownCoordinator::new(Path::new("/tmp/test-kasmos"));

        coordinator.trigger();
        rx.changed().await.expect("should receive");
        assert!(*rx.borrow());
    }

    #[tokio::test]
    async fn test_execute_closes_panes_then_kills() {
        let temp_dir = tempfile::TempDir::new().expect("tmp");
        let (coordinator, _rx) = ShutdownCoordinator::new(temp_dir.path());

        let closed_panes = Arc::new(Mutex::new(Vec::new()));
        let session_killed = Arc::new(AtomicBool::new(false));

        let session = MockSession {
            closed_panes: closed_panes.clone(),
            session_killed: session_killed.clone(),
            pane_names: vec!["wp01-pane".into(), "wp02-pane".into()],
        };

        coordinator.trigger();
        coordinator.execute(&session, None).await.expect("shutdown");

        let closed = closed_panes.lock().await;
        assert_eq!(closed.len(), 2);
        assert!(closed.contains(&"wp01-pane".to_string()));
        assert!(closed.contains(&"wp02-pane".to_string()));
        assert!(session_killed.load(Ordering::SeqCst));
    }

    #[tokio::test]
    async fn test_execute_persists_state() {
        let temp_dir = tempfile::TempDir::new().expect("tmp");
        let (coordinator, _rx) = ShutdownCoordinator::new(temp_dir.path());

        let session = MockSession {
            closed_panes: Arc::new(Mutex::new(Vec::new())),
            session_killed: Arc::new(AtomicBool::new(false)),
            pane_names: vec![],
        };

        let run = crate::types::OrchestrationRun {
            id: "test-run".into(),
            feature: "test".into(),
            feature_dir: std::path::PathBuf::from("/tmp"),
            config: crate::Config::default(),
            work_packages: vec![],
            waves: vec![],
            state: crate::types::RunState::Running,
            started_at: None,
            completed_at: None,
            mode: crate::types::ProgressionMode::Continuous,
        };

        coordinator.trigger();
        coordinator
            .execute(&session, Some(&run))
            .await
            .expect("shutdown");

        let state_path = temp_dir.path().join("state.json");
        assert!(state_path.exists(), "state.json should be written");

        let content = std::fs::read_to_string(&state_path).expect("read");
        assert!(content.contains("test-run"));
    }

    #[tokio::test]
    async fn test_fifo_cleanup() {
        let temp_dir = tempfile::TempDir::new().expect("tmp");
        let fifo_path = temp_dir.path().join("cmd.pipe");
        std::fs::write(&fifo_path, "dummy").expect("create fake fifo");

        let (coordinator, _rx) = ShutdownCoordinator::new(temp_dir.path());
        coordinator.close_fifo();

        assert!(!fifo_path.exists(), "FIFO should be removed");
    }
}
