//! Review coordinator that spawns reviewer agents when WPs enter ForReview.
//!
//! Receives `ReviewRequest` events from the engine and spawns reviewer panes
//! in the Zellij session based on the configured `ReviewAutomationPolicy`.

use crate::review::{ReviewAutomationPolicy, ReviewPolicyExecutor};
use crate::types::ReviewRequest;
use crate::zellij::ZellijCli;
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::mpsc;

/// Coordinates review automation by spawning reviewer agents in Zellij panes.
pub struct ReviewCoordinator {
    /// Name of the Zellij session to spawn panes in.
    session_name: String,

    /// Path to the opencode binary (e.g., "ocx").
    opencode_binary: String,

    /// Profile name for `ocx oc -p <profile>` (e.g., "kas").
    opencode_profile: Option<String>,

    /// Zellij CLI for pane operations.
    cli: Arc<dyn ZellijCli>,

    /// Receives review requests from the engine.
    review_rx: mpsc::Receiver<ReviewRequest>,

    /// Review automation policy.
    policy: ReviewAutomationPolicy,

    /// Path to the .kasmos directory for writing wrapper scripts.
    kasmos_dir: PathBuf,
}

impl ReviewCoordinator {
    /// Create a new review coordinator.
    pub fn new(
        session_name: String,
        opencode_binary: String,
        opencode_profile: Option<String>,
        cli: Arc<dyn ZellijCli>,
        review_rx: mpsc::Receiver<ReviewRequest>,
        policy: ReviewAutomationPolicy,
        kasmos_dir: PathBuf,
    ) -> Self {
        Self {
            session_name,
            opencode_binary,
            opencode_profile,
            cli,
            review_rx,
            policy,
            kasmos_dir,
        }
    }

    /// Main event loop — processes review requests until the channel closes.
    pub async fn run(mut self) {
        let executor = ReviewPolicyExecutor::new(self.policy);

        while let Some(request) = self.review_rx.recv().await {
            let decision = executor.on_for_review_transition();

            if !decision.run_automation {
                tracing::info!(
                    wp_id = %request.wp_id,
                    policy = ?self.policy,
                    "Review automation skipped (ManualOnly policy)"
                );
                continue;
            }

            tracing::info!(
                wp_id = %request.wp_id,
                auto_mark_done = decision.auto_mark_done,
                "Spawning reviewer for WP"
            );

            if let Err(e) = self.spawn_reviewer(&request).await {
                tracing::error!(
                    wp_id = %request.wp_id,
                    error = %e,
                    "Failed to spawn reviewer pane"
                );
            }
        }

        tracing::debug!("Review coordinator shutting down");
    }

    /// Spawn a reviewer pane in the Zellij session.
    ///
    /// Writes a wrapper script to `.kasmos/review-WPXX.sh` and launches it
    /// via `zellij run` to avoid shell metacharacter issues with `run_in_pane`.
    async fn spawn_reviewer(&self, request: &ReviewRequest) -> crate::Result<()> {
        let pane_name = format!("review-{}", request.wp_id);
        let script_path = self.kasmos_dir.join(format!("review-{}.sh", request.wp_id));

        // Build the reviewer command with optional cwd
        let cwd_line = if let Some(ref wt) = request.worktree_path {
            format!(
                "cd {} || exit 1\n",
                shell_escape::escape(std::borrow::Cow::Owned(wt.display().to_string()))
            )
        } else {
            String::new()
        };

        let profile_flag = match &self.opencode_profile {
            Some(p) => format!(
                " -p {}",
                shell_escape::escape(std::borrow::Cow::Borrowed(p))
            ),
            None => String::new(),
        };

        let reviewer_cmd = format!(
            "#!/bin/bash\n{}exec {} oc{} -- --agent reviewer --prompt \"/kas:verify {}\"",
            cwd_line,
            shell_escape::escape(std::borrow::Cow::Borrowed(&self.opencode_binary)),
            profile_flag,
            request.wp_id,
        );

        // Write wrapper script
        std::fs::write(&script_path, &reviewer_cmd)?;

        // Make executable
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&script_path, std::fs::Permissions::from_mode(0o755))?;
        }

        // Launch via Zellij run — pass script path as argument to bash
        let script_str = script_path.display().to_string();
        self.cli
            .run_in_pane(&self.session_name, &pane_name, "bash", &[&script_str])
            .await?;

        tracing::info!(
            wp_id = %request.wp_id,
            pane = %pane_name,
            "Reviewer pane spawned"
        );

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::review::ReviewAutomationPolicy;
    use std::path::PathBuf;

    /// (session, pane_name, command, args)
    type RunCall = (String, String, String, Vec<String>);

    /// Mock ZellijCli that records run_in_pane calls.
    struct MockZellijCli {
        run_calls: std::sync::Mutex<Vec<RunCall>>,
    }

    impl MockZellijCli {
        fn new() -> Self {
            Self {
                run_calls: std::sync::Mutex::new(Vec::new()),
            }
        }

        fn call_count(&self) -> usize {
            self.run_calls.lock().unwrap().len()
        }
    }

    #[async_trait::async_trait]
    impl ZellijCli for MockZellijCli {
        async fn list_sessions(&self) -> crate::Result<Vec<crate::zellij::SessionInfo>> {
            Ok(vec![])
        }

        async fn create_session(
            &self,
            _name: &str,
            _layout: Option<&std::path::Path>,
        ) -> crate::Result<()> {
            Ok(())
        }

        async fn session_exists(&self, _name: &str) -> crate::Result<bool> {
            Ok(true)
        }

        async fn attach_session(&self, _name: &str, _create: bool) -> crate::Result<()> {
            Ok(())
        }

        async fn kill_session(&self, _name: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn new_pane(&self, _session: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn run_in_pane(
            &self,
            session: &str,
            name: &str,
            command: &str,
            args: &[&str],
        ) -> crate::Result<()> {
            self.run_calls.lock().unwrap().push((
                session.to_string(),
                name.to_string(),
                command.to_string(),
                args.iter().map(|s| s.to_string()).collect(),
            ));
            Ok(())
        }

        async fn close_focused_pane(&self, _session: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn focus_next_pane(&self, _session: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn focus_previous_pane(&self, _session: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn toggle_fullscreen(&self, _session: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn new_tab(
            &self,
            _session: &str,
            _name: Option<&str>,
            _layout: Option<&std::path::Path>,
        ) -> crate::Result<()> {
            Ok(())
        }

        async fn rename_tab(&self, _session: &str, _name: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn go_to_tab_name(&self, _session: &str, _name: &str) -> crate::Result<()> {
            Ok(())
        }

        async fn query_tab_names(&self, _session: &str) -> crate::Result<Vec<String>> {
            Ok(vec![])
        }
    }

    #[tokio::test]
    async fn test_coordinator_spawns_reviewer_on_request() {
        let temp_dir = tempfile::TempDir::new().unwrap();
        let kasmos_dir = temp_dir.path().to_path_buf();

        let (review_tx, review_rx) = mpsc::channel(10);
        let mock_cli = Arc::new(MockZellijCli::new());

        let coordinator = ReviewCoordinator::new(
            "test-session".to_string(),
            "ocx".to_string(),
            None,
            mock_cli.clone(),
            review_rx,
            ReviewAutomationPolicy::AutoThenManualApprove,
            kasmos_dir.clone(),
        );

        let handle = tokio::spawn(coordinator.run());

        review_tx
            .send(ReviewRequest {
                wp_id: "WP01".to_string(),
                worktree_path: Some(PathBuf::from("/tmp/worktree/WP01")),
                feature_dir: PathBuf::from("/tmp/feature"),
            })
            .await
            .unwrap();

        drop(review_tx);
        handle.await.unwrap();

        // Verify reviewer was spawned
        assert_eq!(mock_cli.call_count(), 1);
        let calls = mock_cli.run_calls.lock().unwrap();
        assert_eq!(calls[0].0, "test-session");
        assert_eq!(calls[0].1, "review-WP01");
        assert_eq!(calls[0].2, "bash");

        // Verify wrapper script was created
        let script_path = kasmos_dir.join("review-WP01.sh");
        assert!(script_path.exists());
        let content = std::fs::read_to_string(&script_path).unwrap();
        assert!(
            content.contains("ocx oc -- --agent reviewer --prompt"),
            "Script content should contain ocx oc invocation: {content}"
        );
        assert!(content.contains("/kas:verify WP01"));
    }

    #[tokio::test]
    async fn test_coordinator_skips_manual_only_policy() {
        let temp_dir = tempfile::TempDir::new().unwrap();
        let kasmos_dir = temp_dir.path().to_path_buf();

        let (review_tx, review_rx) = mpsc::channel(10);
        let mock_cli = Arc::new(MockZellijCli::new());

        let coordinator = ReviewCoordinator::new(
            "test-session".to_string(),
            "ocx".to_string(),
            None,
            mock_cli.clone(),
            review_rx,
            ReviewAutomationPolicy::ManualOnly,
            kasmos_dir,
        );

        let handle = tokio::spawn(coordinator.run());

        review_tx
            .send(ReviewRequest {
                wp_id: "WP01".to_string(),
                worktree_path: None,
                feature_dir: PathBuf::from("/tmp/feature"),
            })
            .await
            .unwrap();

        drop(review_tx);
        handle.await.unwrap();

        assert_eq!(mock_cli.call_count(), 0);
    }

    #[tokio::test]
    async fn test_engine_emits_review_request_on_for_review() {
        use crate::config::Config;
        use crate::detector::{CompletionEvent, DetectedLane};
        use crate::types::{CompletionMethod, ProgressionMode, WPState};

        let run = crate::types::OrchestrationRun {
            id: "test".to_string(),
            feature: "test".to_string(),
            feature_dir: PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages: vec![crate::types::WorkPackage {
                id: "WP01".to_string(),
                title: "Test".to_string(),
                state: WPState::Active,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: Some(PathBuf::from("/tmp/worktree/WP01")),
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }],
            waves: vec![],
            state: crate::types::RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        };
        let run_arc = std::sync::Arc::new(tokio::sync::RwLock::new(run));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);
        let (review_tx, mut review_rx) = mpsc::channel(10);
        let (launch_tx, _launch_rx) = mpsc::channel(10);

        let mut engine =
            crate::engine::WaveEngine::new(run_arc.clone(), completion_rx, action_rx, launch_tx);
        engine.set_review_tx(review_tx);
        engine.init_graph().await.unwrap();

        let event = CompletionEvent::with_lane(
            "WP01".to_string(),
            CompletionMethod::AutoDetected,
            DetectedLane::ForReview,
        );

        engine.handle_completion(event).await.unwrap();

        {
            let r = run_arc.read().await;
            assert_eq!(r.work_packages[0].state, WPState::ForReview);
        }

        let request = review_rx.try_recv().unwrap();
        assert_eq!(request.wp_id, "WP01");
    }

    #[tokio::test]
    async fn test_approve_transitions_for_review_to_completed() {
        use crate::config::Config;
        use crate::types::{ProgressionMode, WPState};

        let run = crate::types::OrchestrationRun {
            id: "test".to_string(),
            feature: "test".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            config: Config::default(),
            work_packages: vec![crate::types::WorkPackage {
                id: "WP01".to_string(),
                title: "Test".to_string(),
                state: WPState::ForReview,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }],
            waves: vec![],
            state: crate::types::RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        };
        let run_arc = std::sync::Arc::new(tokio::sync::RwLock::new(run));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);
        let (launch_tx, _launch_rx) = mpsc::channel(10);

        let mut engine =
            crate::engine::WaveEngine::new(run_arc.clone(), completion_rx, action_rx, launch_tx);
        engine.init_graph().await.unwrap();

        engine.approve_wp("WP01").await.unwrap();

        let r = run_arc.read().await;
        assert_eq!(r.work_packages[0].state, WPState::Completed);
        assert!(r.work_packages[0].completed_at.is_some());
    }

    #[tokio::test]
    async fn test_reject_with_relaunch_transitions_to_active() {
        use crate::config::Config;
        use crate::types::{ProgressionMode, WPState};

        let run = crate::types::OrchestrationRun {
            id: "test".to_string(),
            feature: "test".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            config: Config::default(),
            work_packages: vec![crate::types::WorkPackage {
                id: "WP01".to_string(),
                title: "Test".to_string(),
                state: WPState::ForReview,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }],
            waves: vec![],
            state: crate::types::RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        };
        let run_arc = std::sync::Arc::new(tokio::sync::RwLock::new(run));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);
        let (launch_tx, _launch_rx) = mpsc::channel(10);

        let mut engine =
            crate::engine::WaveEngine::new(run_arc.clone(), completion_rx, action_rx, launch_tx);
        engine.init_graph().await.unwrap();

        engine.reject_wp("WP01", true).await.unwrap();

        let r = run_arc.read().await;
        assert_eq!(r.work_packages[0].state, WPState::Active);
    }

    #[tokio::test]
    async fn test_reject_without_relaunch_transitions_to_pending() {
        use crate::config::Config;
        use crate::types::{ProgressionMode, WPState};

        let run = crate::types::OrchestrationRun {
            id: "test".to_string(),
            feature: "test".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            config: Config::default(),
            work_packages: vec![crate::types::WorkPackage {
                id: "WP01".to_string(),
                title: "Test".to_string(),
                state: WPState::ForReview,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }],
            waves: vec![],
            state: crate::types::RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        };
        let run_arc = std::sync::Arc::new(tokio::sync::RwLock::new(run));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);
        let (launch_tx, _launch_rx) = mpsc::channel(10);

        let mut engine =
            crate::engine::WaveEngine::new(run_arc.clone(), completion_rx, action_rx, launch_tx);
        engine.init_graph().await.unwrap();

        engine.reject_wp("WP01", false).await.unwrap();

        let r = run_arc.read().await;
        assert_eq!(r.work_packages[0].state, WPState::Pending);
    }
}
