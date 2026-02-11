//! State persistence for orchestration runs.
//!
//! Provides atomic file-based persistence of orchestration state with
//! reconciliation support for session reattachment scenarios.

use crate::error::{Result, StateError};
use crate::types::{OrchestrationRun, WPState};
use std::collections::HashSet;
use std::path::{Path, PathBuf};
use std::time::SystemTime;

/// Manages atomic persistence of orchestration state.
///
/// Uses atomic write pattern (write to temp file, then rename) to prevent
/// corruption from crashes or interruptions.
pub struct StatePersister {
    state_path: PathBuf,
    tmp_path: PathBuf,
}

impl StatePersister {
    /// Creates a new state persister for the given kasmos directory.
    ///
    /// # Arguments
    /// * `kasmos_dir` - Directory where state files will be stored (typically `.kasmos/`)
    pub fn new(kasmos_dir: &Path) -> Self {
        Self {
            state_path: kasmos_dir.join("state.json"),
            tmp_path: kasmos_dir.join("state.json.tmp"),
        }
    }

    /// Ensures the kasmos directory exists.
    ///
    /// Must be called before save() to prevent write failures.
    pub fn ensure_dir(&self) -> Result<()> {
        if let Some(parent) = self.state_path.parent() {
            std::fs::create_dir_all(parent)?;
        }
        Ok(())
    }

    /// Saves the current orchestration state to disk atomically.
    ///
    /// Uses write-to-temp-then-rename pattern to ensure either the old or new
    /// state exists on disk, never a partial write.
    ///
    /// # Errors
    /// Returns `StateError::Corrupted` if serialization or file operations fail.
    pub fn save(&self, run: &OrchestrationRun) -> Result<()> {
        let json = serde_json::to_string_pretty(run)
            .map_err(|e| StateError::Corrupted(format!("Serialization failed: {}", e)))?;

        self.atomic_write(&json)?;

        tracing::debug!(path = %self.state_path.display(), "State persisted");
        Ok(())
    }

    /// Loads state from disk if it exists.
    ///
    /// Returns `Ok(None)` if no state file exists (fresh start).
    /// Returns `Ok(Some(run))` if state loaded successfully.
    ///
    /// # Errors
    /// Returns `StateError::Corrupted` if the file exists but cannot be read or parsed.
    pub fn load(&self) -> Result<Option<OrchestrationRun>> {
        if !self.state_path.exists() {
            return Ok(None);
        }

        let content = std::fs::read_to_string(&self.state_path)
            .map_err(|e| StateError::Corrupted(format!("Failed to read state: {}", e)))?;

        let run: OrchestrationRun = serde_json::from_str(&content)
            .map_err(|e| StateError::Corrupted(format!("Failed to parse state: {}", e)))?;

        tracing::info!(path = %self.state_path.display(), "State loaded");
        Ok(Some(run))
    }

    /// Performs atomic write using temp file + rename pattern.
    ///
    /// POSIX guarantees that rename is atomic on the same filesystem,
    /// so we either have the old state.json or the new one, never partial.
    fn atomic_write(&self, content: &str) -> Result<()> {
        // Write to temporary file first
        std::fs::write(&self.tmp_path, content)
            .map_err(|e| StateError::Corrupted(format!("Failed to write tmp state: {}", e)))?;

        // Atomic rename (POSIX guarantee on same filesystem)
        std::fs::rename(&self.tmp_path, &self.state_path)
            .map_err(|e| StateError::Corrupted(format!("Failed to rename state file: {}", e)))?;

        Ok(())
    }

    /// Reconciles persisted state against live pane status.
    ///
    /// Called when reattaching to a session to detect work packages that
    /// completed, crashed, or changed state while detached.
    ///
    /// # Arguments
    /// * `run` - Mutable reference to the loaded state (will be corrected in-place)
    /// * `pane_lister` - Object that can query live pane status
    ///
    /// # Returns
    /// List of corrections that were applied to the state.
    pub async fn reconcile<L: PaneLister>(
        &self,
        run: &mut OrchestrationRun,
        pane_lister: &L,
    ) -> Result<Vec<StateCorrection>> {
        let mut corrections = Vec::new();

        // Get current live pane names from Zellij
        let live_panes = pane_lister.list_live_panes().await?;
        let live_pane_names: HashSet<String> = live_panes.into_iter().collect();

        for wp in &mut run.work_packages {
            let pane_exists = live_pane_names.contains(&wp.pane_name);

            match (&wp.state, pane_exists) {
                // Active WP + pane exists → continue as-is
                (WPState::Active, true) => {
                    tracing::debug!(wp_id = %wp.id, "Active WP still running");
                }

                // Active WP + pane missing → crashed while detached
                (WPState::Active, false) => {
                    tracing::warn!(wp_id = %wp.id, "Active WP pane missing — marking as crashed");
                    let old_state = wp.state;
                    wp.state = WPState::Failed;
                    wp.failure_count += 1;
                    corrections.push(StateCorrection {
                        wp_id: wp.id.clone(),
                        from: old_state,
                        to: WPState::Failed,
                        reason: "Pane missing on reattach".into(),
                    });
                }

                // Completed WP + pane exists → stale pane
                (WPState::Completed, true) => {
                    tracing::info!(wp_id = %wp.id, "Completed WP still has pane — scheduling close");
                    corrections.push(StateCorrection {
                        wp_id: wp.id.clone(),
                        from: WPState::Completed,
                        to: WPState::Completed,
                        reason: "Stale pane — will close".into(),
                    });
                }

                // Completed WP + pane missing → normal, pane was cleaned up
                (WPState::Completed, false) => {
                    tracing::debug!(wp_id = %wp.id, "Completed WP with no pane (normal)");
                }

                // Failed WP → offer retry
                (WPState::Failed, _) => {
                    tracing::info!(wp_id = %wp.id, "Failed WP — use 'retry {}' to relaunch", wp.id);
                }

                // Paused WP + pane exists → resume possible
                (WPState::Paused, true) => {
                    tracing::info!(wp_id = %wp.id, "Paused WP — use 'resume {}' to continue", wp.id);
                }

                // Paused WP + pane missing → crashed while paused
                (WPState::Paused, false) => {
                    let old_state = wp.state;
                    wp.state = WPState::Failed;
                    wp.failure_count += 1;
                    corrections.push(StateCorrection {
                        wp_id: wp.id.clone(),
                        from: old_state,
                        to: WPState::Failed,
                        reason: "Paused pane missing on reattach".into(),
                    });
                }

                // ForReview WP → waiting for operator review, no pane required
                (WPState::ForReview, _) => {
                    tracing::info!(
                        wp_id = %wp.id,
                        "ForReview WP — use Review tab to approve/reject"
                    );
                }

                // Pending WP → no action needed
                (WPState::Pending, _) => {}
            }
        }

        // Persist corrected state if any corrections were made
        if !corrections.is_empty() {
            self.save(run)?;
            tracing::info!(corrections = corrections.len(), "State reconciled");
        }

        Ok(corrections)
    }

    /// Checks if the state file is stale relative to a reference time.
    ///
    /// Returns a warning if the state file's modification time is older than
    /// the given reference time (typically session start time).
    ///
    /// # Arguments
    /// * `session_start` - Reference time to compare against (e.g., when session was created)
    ///
    /// # Returns
    /// `Some(warning)` if state is stale, `None` if state is fresh.
    pub fn check_staleness(&self, session_start: SystemTime) -> Result<Option<StalenessWarning>> {
        // Early exit: state file doesn't exist
        if !self.state_path.exists() {
            return Ok(None);
        }

        let metadata = std::fs::metadata(&self.state_path)?;
        let state_mtime = metadata.modified()?;

        // Early exit: state is fresh
        if state_mtime >= session_start {
            return Ok(None);
        }

        // State is stale
        let age = session_start
            .duration_since(state_mtime)
            .unwrap_or_default();

        tracing::warn!(
            age_secs = age.as_secs(),
            "State file is older than session — may be stale"
        );

        Ok(Some(StalenessWarning {
            state_age: age,
            state_mtime,
            session_start,
        }))
    }
}

/// Trait for objects that can query live pane status from Zellij.
///
/// This abstraction allows reconciliation to work without tight coupling
/// to a specific session management implementation.
#[async_trait::async_trait]
pub trait PaneLister {
    /// Returns the names of all currently live panes in the session.
    async fn list_live_panes(&self) -> Result<Vec<String>>;
}

/// Represents a state correction applied during reconciliation.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct StateCorrection {
    /// Work package ID that was corrected.
    pub wp_id: String,
    /// State before correction.
    pub from: WPState,
    /// State after correction.
    pub to: WPState,
    /// Human-readable reason for the correction.
    pub reason: String,
}

/// Warning issued when state file is older than the session.
#[derive(Debug, Clone)]
pub struct StalenessWarning {
    /// How old the state file is relative to session start.
    pub state_age: std::time::Duration,
    /// State file's modification time.
    pub state_mtime: SystemTime,
    /// Session start time used for comparison.
    pub session_start: SystemTime,
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::types::{ProgressionMode, RunState, Wave, WaveState, WorkPackage};
    use std::time::Duration;
    use tempfile::TempDir;

    /// Creates a minimal test orchestration run.
    fn create_test_run() -> OrchestrationRun {
        OrchestrationRun {
            id: "test-run-001".to_string(),
            feature: "test-feature".to_string(),
            feature_dir: PathBuf::from("/tmp/test"),
            config: Config::default(),
            work_packages: vec![WorkPackage {
                id: "WP01".to_string(),
                title: "Test Package".to_string(),
                state: WPState::Pending,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01-pane".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }],
            waves: vec![Wave {
                index: 0,
                wp_ids: vec!["WP01".to_string()],
                state: WaveState::Pending,
            }],
            state: RunState::Initializing,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[test]
    fn test_state_round_trip() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());
        persister.ensure_dir().expect("ensure dir");

        let original = create_test_run();

        // Save and load
        persister.save(&original).expect("save");
        let loaded = persister.load().expect("load").expect("state exists");

        // Verify round-trip
        assert_eq!(original.id, loaded.id);
        assert_eq!(original.feature, loaded.feature);
        assert_eq!(original.work_packages.len(), loaded.work_packages.len());
        assert_eq!(original.work_packages[0].id, loaded.work_packages[0].id);
    }

    #[test]
    fn test_load_missing_file() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());

        let result = persister.load().expect("load should succeed");
        assert!(result.is_none(), "should return None for missing file");
    }

    #[test]
    fn test_load_corrupted_file() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());
        persister.ensure_dir().expect("ensure dir");

        // Write invalid JSON
        std::fs::write(&persister.state_path, "not valid json").expect("write corrupted file");

        let result = persister.load();
        assert!(result.is_err(), "should fail on corrupted file");

        if let Err(crate::KasmosError::State(StateError::Corrupted(msg))) = result {
            assert!(msg.contains("Failed to parse"));
        } else {
            panic!("expected StateError::Corrupted");
        }
    }

    #[test]
    fn test_atomic_write_creates_final_file() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());
        persister.ensure_dir().expect("ensure dir");

        let run = create_test_run();
        persister.save(&run).expect("save");

        // Final file should exist
        assert!(persister.state_path.exists(), "state.json should exist");

        // Temp file should be cleaned up by rename
        assert!(
            !persister.tmp_path.exists(),
            "state.json.tmp should not exist after rename"
        );
    }

    #[tokio::test]
    async fn test_reconcile_active_with_missing_pane() {
        struct MockPaneLister {
            panes: Vec<String>,
        }

        #[async_trait::async_trait]
        impl PaneLister for MockPaneLister {
            async fn list_live_panes(&self) -> Result<Vec<String>> {
                Ok(self.panes.clone())
            }
        }

        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());

        let mut run = create_test_run();
        run.work_packages[0].state = WPState::Active; // WP is active

        // Mock pane lister returns no panes (pane missing)
        let lister = MockPaneLister { panes: vec![] };

        let corrections = persister
            .reconcile(&mut run, &lister)
            .await
            .expect("reconcile");

        assert_eq!(corrections.len(), 1, "should have one correction");
        assert_eq!(corrections[0].from, WPState::Active);
        assert_eq!(corrections[0].to, WPState::Failed);
        assert!(corrections[0].reason.contains("missing"));

        // State should be updated
        assert_eq!(run.work_packages[0].state, WPState::Failed);
        assert_eq!(run.work_packages[0].failure_count, 1);
    }

    #[tokio::test]
    async fn test_reconcile_active_with_existing_pane() {
        struct MockPaneLister {
            panes: Vec<String>,
        }

        #[async_trait::async_trait]
        impl PaneLister for MockPaneLister {
            async fn list_live_panes(&self) -> Result<Vec<String>> {
                Ok(self.panes.clone())
            }
        }

        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());

        let mut run = create_test_run();
        run.work_packages[0].state = WPState::Active;

        // Mock pane lister returns the expected pane
        let lister = MockPaneLister {
            panes: vec!["wp01-pane".to_string()],
        };

        let corrections = persister
            .reconcile(&mut run, &lister)
            .await
            .expect("reconcile");

        assert_eq!(corrections.len(), 0, "should have no corrections");
        assert_eq!(
            run.work_packages[0].state,
            WPState::Active,
            "state should remain active"
        );
    }

    #[tokio::test]
    async fn test_reconcile_completed_with_stale_pane() {
        struct MockPaneLister {
            panes: Vec<String>,
        }

        #[async_trait::async_trait]
        impl PaneLister for MockPaneLister {
            async fn list_live_panes(&self) -> Result<Vec<String>> {
                Ok(self.panes.clone())
            }
        }

        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());

        let mut run = create_test_run();
        run.work_packages[0].state = WPState::Completed;

        // Pane still exists (stale)
        let lister = MockPaneLister {
            panes: vec!["wp01-pane".to_string()],
        };

        let corrections = persister
            .reconcile(&mut run, &lister)
            .await
            .expect("reconcile");

        assert_eq!(corrections.len(), 1, "should have stale pane correction");
        assert_eq!(corrections[0].from, WPState::Completed);
        assert_eq!(corrections[0].to, WPState::Completed);
        assert!(corrections[0].reason.contains("Stale pane"));
    }

    #[test]
    fn test_staleness_detection_fresh_state() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());
        persister.ensure_dir().expect("ensure dir");

        let run = create_test_run();
        persister.save(&run).expect("save");

        // Session started before state was written (state is fresh)
        let session_start = SystemTime::now() - Duration::from_secs(10);

        let warning = persister
            .check_staleness(session_start)
            .expect("check staleness");

        assert!(warning.is_none(), "state should be fresh");
    }

    #[test]
    fn test_staleness_detection_stale_state() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());
        persister.ensure_dir().expect("ensure dir");

        let run = create_test_run();
        persister.save(&run).expect("save");

        // Simulate session starting after state was written
        std::thread::sleep(Duration::from_millis(100));
        let session_start = SystemTime::now();

        let warning = persister
            .check_staleness(session_start)
            .expect("check staleness");

        assert!(warning.is_some(), "state should be stale");

        let warning = warning.unwrap();
        assert!(warning.state_age.as_millis() > 0);
    }

    #[test]
    fn test_staleness_with_missing_file() {
        let temp_dir = TempDir::new().expect("create temp dir");
        let persister = StatePersister::new(temp_dir.path());

        let session_start = SystemTime::now();
        let warning = persister
            .check_staleness(session_start)
            .expect("check staleness");

        assert!(warning.is_none(), "should return None for missing file");
    }
}
