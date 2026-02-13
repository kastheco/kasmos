//! Wave progression engine for orchestration.
//!
//! This module implements the core wave engine that drives the orchestration lifecycle.
//! It receives completion events, checks dependency satisfaction, and launches eligible
//! work packages. Supports both continuous mode (auto-launch on dependency resolution)
//! and wave-gated mode (pause for operator confirmation at wave boundaries).

use crate::Result;
use crate::command_handlers::EngineAction;
use crate::detector::{CompletionEvent, DetectedLane};
use crate::error::WaveError;
use crate::graph::DependencyGraph;
use crate::persistence::StatePersister;
use crate::types::{CompletionMethod, OrchestrationRun, ProgressionMode, ReviewRequest, RunState, WPState};
use std::collections::{HashSet, VecDeque};
use std::sync::Arc;
use tokio::sync::{RwLock, mpsc, watch};

/// Event emitted when the engine launches work packages for a wave.
///
/// The orchestrator receives these events and creates actual Zellij panes/tabs.
#[derive(Debug, Clone)]
pub struct WaveLaunchEvent {
    /// Wave index being launched.
    pub wave_index: usize,
    /// Work package IDs being launched in this batch.
    pub wp_ids: Vec<String>,
    /// Whether this is the first wave (panes already exist in initial layout).
    pub is_initial_wave: bool,
}

/// The wave engine that orchestrates work package execution.
///
/// Manages wave progression, capacity limiting, and dependency satisfaction.
pub struct WaveEngine {
    /// Shared orchestration run state.
    run: Arc<RwLock<OrchestrationRun>>,

    /// Dependency graph for efficient querying.
    graph: DependencyGraph,

    /// Receiver for completion events.
    completion_rx: mpsc::Receiver<CompletionEvent>,

    /// Receiver for engine actions (commands).
    action_rx: mpsc::Receiver<EngineAction>,

    /// Sender for wave launch events — notifies the orchestrator to create panes.
    launch_tx: mpsc::Sender<WaveLaunchEvent>,

    /// Sender for review requests when WPs enter ForReview.
    review_tx: Option<mpsc::Sender<ReviewRequest>>,

    /// Queue of work packages waiting for capacity.
    launch_queue: VecDeque<String>,

    /// Number of currently active panes.
    active_panes: usize,

    /// The currently approved wave index for wave-gated mode.
    current_wave: usize,

    /// Tracks WP IDs launched in the current batch (accumulated during launch_eligible_wps).
    pending_launch_batch: Vec<String>,

    /// Optional state persister for saving state after mutations.
    persister: Option<Arc<StatePersister>>,

    /// Optional watch channel sender for broadcasting state to the TUI.
    watch_tx: Option<watch::Sender<OrchestrationRun>>,
}

impl WaveEngine {
    /// Create a new wave engine.
    ///
    /// The dependency graph is built lazily on first use to avoid blocking
    /// in async contexts.
    pub fn new(
        run: Arc<RwLock<OrchestrationRun>>,
        completion_rx: mpsc::Receiver<CompletionEvent>,
        action_rx: mpsc::Receiver<EngineAction>,
        launch_tx: mpsc::Sender<WaveLaunchEvent>,
    ) -> Self {
        // Build dependency graph from work packages
        // We create a dummy graph here and rebuild it on first use if needed
        let graph = DependencyGraph {
            dependencies: std::collections::HashMap::new(),
            dependents: std::collections::HashMap::new(),
        };

        Self {
            run,
            graph,
            completion_rx,
            action_rx,
            launch_tx,
            review_tx: None,
            launch_queue: VecDeque::new(),
            active_panes: 0,
            current_wave: 0,
            pending_launch_batch: Vec::new(),
            persister: None,
            watch_tx: None,
        }
    }

    /// Set a state persister for automatic state saving after mutations.
    pub fn with_persister(mut self, persister: Arc<StatePersister>) -> Self {
        self.persister = Some(persister);
        self
    }

    /// Set a watch channel sender for broadcasting state updates to the TUI.
    pub fn with_watch_tx(mut self, tx: watch::Sender<OrchestrationRun>) -> Self {
        self.watch_tx = Some(tx);
        self
    }

    /// Set the review request sender for emitting review events.
    pub fn set_review_tx(&mut self, tx: mpsc::Sender<ReviewRequest>) {
        self.review_tx = Some(tx);
    }

    /// Initialize the dependency graph from the current run state.
    /// This should be called once before the main event loop starts.
    pub(crate) async fn init_graph(&mut self) -> Result<()> {
        let run = self.run.read().await;
        self.graph = DependencyGraph::new(&run.work_packages);
        Ok(())
    }

    /// Reconcile WPs that were already completed/for_review at startup.
    ///
    /// When WPs are seeded from frontmatter with terminal states, the engine
    /// needs to log their status and ensure downstream waves can be evaluated.
    async fn reconcile_initial_state(&self) {
        let run = self.run.read().await;

        let pre_completed: Vec<(&str, &WPState)> = run
            .work_packages
            .iter()
            .filter(|wp| {
                matches!(wp.state, WPState::Completed | WPState::ForReview | WPState::Active)
            })
            .map(|wp| (wp.id.as_str(), &wp.state))
            .collect();

        if !pre_completed.is_empty() {
            for (wp_id, state) in &pre_completed {
                tracing::info!(
                    wp_id = %wp_id,
                    state = ?state,
                    "WP pre-seeded from frontmatter"
                );
            }

            let completed_count = pre_completed
                .iter()
                .filter(|(_, s)| matches!(s, WPState::Completed))
                .count();
            let review_count = pre_completed
                .iter()
                .filter(|(_, s)| matches!(s, WPState::ForReview))
                .count();

            if completed_count > 0 || review_count > 0 {
                tracing::info!(
                    completed = completed_count,
                    for_review = review_count,
                    "Pre-existing terminal WPs detected — downstream waves may be unblocked"
                );
            }
        }
    }

    /// Persist the current run state to disk (if a persister is configured).
    async fn persist_state(&self) {
        if let Some(ref persister) = self.persister {
            let run = self.run.read().await;
            if let Err(e) = persister.save(&run) {
                tracing::error!("Failed to persist state: {}", e);
            }
        }
    }

    /// Broadcast the current run state to the TUI via the watch channel.
    async fn broadcast_state(&self) {
        if let Some(ref tx) = self.watch_tx {
            let run = self.run.read().await;
            let _ = tx.send(run.clone());
        }
    }

    /// Main event loop — runs until orchestration completes or aborts.
    pub async fn run(&mut self) -> Result<()> {
        // Initialize dependency graph from current run state
        self.init_graph().await?;

        // Check for WPs that were already completed at startup (seeded from frontmatter).
        // These need to be accounted for in active_panes count and may already satisfy
        // dependencies for later waves.
        self.reconcile_initial_state().await;

        // Launch initial wave (panes created by the initial layout — mark as initial)
        self.launch_eligible_wps_and_notify(true).await?;
        self.update_wave_states().await;
        self.persist_state().await;
        self.broadcast_state().await;

        loop {
            // Check if aborted
            let is_aborted = { let r = self.run.read().await; r.state == RunState::Aborted };
            if is_aborted {
                break;
            }

            tokio::select! {
                // Handle completion events
                Some(event) = self.completion_rx.recv() => {
                    self.handle_completion(event).await?;
                }
                // Handle engine actions (commands)
                Some(action) = self.action_rx.recv() => {
                    self.handle_action(action).await?;
                }
                // All channels closed — done
                else => break,
            }

            // Check if orchestration is complete
            if self.is_complete().await {
                let mut run = self.run.write().await;
                let all_failed = run.work_packages.iter().all(|wp| wp.state == WPState::Failed);
                run.state = if all_failed { RunState::Failed } else { RunState::Completed };
                run.completed_at = Some(std::time::SystemTime::now());
                tracing::info!(state = ?run.state, "Orchestration complete!");
                drop(run);
                self.persist_state().await;
                break;
            }
        }

        Ok(())
    }

    /// Handle a completion event.
    pub(crate) async fn handle_completion(&mut self, event: CompletionEvent) -> Result<()> {
        let wp_id = event.wp_id.clone();
        let success = event.success;
        let method = event.method;
        let detected_lane = event.detected_lane;

        // Track whether we need to emit a review request after releasing the lock
        let mut review_request: Option<ReviewRequest> = None;

        {
            let mut run = self.run.write().await;

            // Guard: Unknown work package
            let wp_idx = run
                .work_packages
                .iter()
                .position(|w| w.id == wp_id)
                .ok_or_else(|| {
                    crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.clone() })
                })?;

            if success {
                if detected_lane == Some(DetectedLane::ForReview) {
                    // Agent finished — move to review
                    let new_state = run.work_packages[wp_idx].state.transition(WPState::ForReview, &wp_id)?;
                    run.work_packages[wp_idx].state = new_state;
                    self.active_panes = self.active_panes.saturating_sub(1);
                    tracing::info!(wp_id = %wp_id, "WP moved to review");

                    // Prepare review request data
                    if self.review_tx.is_some() {
                        review_request = Some(ReviewRequest {
                            wp_id: wp_id.clone(),
                            worktree_path: run.work_packages[wp_idx].worktree_path.clone(),
                            feature_dir: run.feature_dir.clone(),
                        });
                    }
                } else if detected_lane == Some(DetectedLane::Done) {
                    // Review approved — mark completed
                    let wp = &mut run.work_packages[wp_idx];
                    wp.state = wp.state.transition(WPState::Completed, &wp.id)?;
                    wp.completed_at = Some(std::time::SystemTime::now());
                    wp.completion_method = Some(method);
                    self.active_panes = self.active_panes.saturating_sub(1);
                    tracing::info!(wp_id = %wp.id, "WP completed — review approved");
                } else if detected_lane == Some(DetectedLane::Rejected) {
                    // Review rejected — re-queue for rework
                    let wp = &mut run.work_packages[wp_idx];
                    wp.state = wp.state.transition(WPState::Pending, &wp.id)?;
                    wp.failure_count += 1;
                    self.active_panes = self.active_panes.saturating_sub(1);
                    tracing::warn!(wp_id = %wp.id, failure_count = wp.failure_count, "WP rejected by review — re-queued");
                } else {
                    // Non-frontmatter completion (file marker, git activity)
                    let wp = &mut run.work_packages[wp_idx];
                    wp.state = wp.state.transition(WPState::Completed, &wp.id)?;
                    wp.completed_at = Some(std::time::SystemTime::now());
                    wp.completion_method = Some(method);
                    self.active_panes = self.active_panes.saturating_sub(1);
                    tracing::info!(wp_id = %wp.id, "WP completed successfully");
                }
            } else {
                // Failure
                let wp = &mut run.work_packages[wp_idx];
                wp.state = wp.state.transition(WPState::Failed, &wp.id)?;
                wp.failure_count += 1;
                self.active_panes = self.active_panes.saturating_sub(1);
                tracing::warn!(wp_id = %wp.id, failure_count = wp.failure_count, "WP failed");

                // Log blocked dependents
                let blocked = self.graph.get_dependents(&wp.id);
                if !blocked.is_empty() {
                    tracing::warn!(
                        failed_wp = %wp.id,
                        blocked = ?blocked,
                        "WP failed — blocking {} direct dependents. Use 'retry {}' or 'force-advance {}' to unblock.",
                        blocked.len(), wp.id, wp.id
                    );
                }
            }
        }

        // Emit review request outside the lock
        if let (Some(review_tx), Some(request)) = (&self.review_tx, review_request)
            && let Err(e) = review_tx.send(request).await
        {
            tracing::error!(wp_id = %wp_id, error = %e, "Failed to send review request");
        }

        // Check for newly eligible WPs and launch them
        self.launch_eligible_wps_and_notify(false).await?;

        // Process launch queue (freed slot might allow queued WPs)
        self.process_launch_queue().await?;

        // Update wave states to reflect all WP changes (completions + new launches)
        self.update_wave_states().await;

        // Persist state after handling completion
        self.persist_state().await;
        self.broadcast_state().await;

        Ok(())
    }

    /// Handle an engine action (command).
    async fn handle_action(&mut self, action: EngineAction) -> Result<()> {
        match action {
            EngineAction::Restart(wp_id) => {
                self.restart_wp(&wp_id).await?;
            }
            EngineAction::Pause(wp_id) => {
                self.pause_wp(&wp_id).await?;
            }
            EngineAction::Resume(wp_id) => {
                self.resume_wp(&wp_id).await?;
            }
            EngineAction::ForceAdvance(wp_id) => {
                self.force_advance(&wp_id).await?;
            }
            EngineAction::Retry(wp_id) => {
                self.retry_wp(&wp_id).await?;
            }
            EngineAction::Advance => {
                self.advance_wave().await?;
            }
            EngineAction::Abort => {
                let mut run = self.run.write().await;
                run.state = RunState::Aborted;
                tracing::info!("Orchestration aborted by operator");
                drop(run);
                self.persist_state().await;
                return Ok(());
            }
            EngineAction::Finalize => {
                self.finalize_run().await?;
                self.persist_state().await;
                self.broadcast_state().await;
                return Ok(());
            }
            EngineAction::Approve(wp_id) => {
                self.approve_wp(&wp_id).await?;
            }
            EngineAction::Reject { wp_id, relaunch } => {
                self.reject_wp(&wp_id, relaunch).await?;
            }
        }

        // Update wave states to reflect any WP changes from the action
        self.update_wave_states().await;

        // Persist state after handling action
        self.persist_state().await;
        self.broadcast_state().await;

        Ok(())
    }

    /// Launch eligible WPs and emit a WaveLaunchEvent if any were launched.
    async fn launch_eligible_wps_and_notify(&mut self, is_initial: bool) -> Result<()> {
        self.pending_launch_batch.clear();
        self.launch_eligible_wps().await?;

        if !self.pending_launch_batch.is_empty() {
            let wp_ids = std::mem::take(&mut self.pending_launch_batch);
            // Determine the wave index from the first launched WP
            let wave_index = {
                let run = self.run.read().await;
                run.work_packages
                    .iter()
                    .find(|wp| wp.id == wp_ids[0])
                    .map(|wp| wp.wave)
                    .unwrap_or(self.current_wave)
            };
            let event = WaveLaunchEvent {
                wave_index,
                wp_ids,
                is_initial_wave: is_initial,
            };
            if let Err(e) = self.launch_tx.send(event).await {
                tracing::error!("Failed to send wave launch event: {}", e);
            }
        }

        self.broadcast_state().await;

        Ok(())
    }

    /// Find and launch all eligible work packages.
    async fn launch_eligible_wps(&mut self) -> Result<()> {
        // Collect completed WP IDs and mode
        let (completed, mode, max_panes) = {
            let run = self.run.read().await;

            let completed: HashSet<String> = run
                .work_packages
                .iter()
                .filter(|wp| matches!(wp.state, WPState::Completed))
                .map(|wp| wp.id.clone())
                .collect();

            (completed, run.mode, run.config.max_agent_panes)
        };

        // Find eligible WPs (Pending + deps satisfied)
        let eligible: Vec<String> = {
            let run = self.run.read().await;
            run.work_packages
                .iter()
                .filter(|wp| matches!(wp.state, WPState::Pending))
                .filter(|wp| self.graph.deps_satisfied(&wp.id, &completed))
                .map(|wp| wp.id.clone())
                .collect()
        };

        match mode {
            ProgressionMode::Continuous => {
                // Launch immediately, respecting capacity
                for wp_id in eligible {
                    if self.active_panes >= max_panes {
                        self.launch_queue.push_back(wp_id.clone());
                        tracing::info!(wp_id = %wp_id, "Queued (capacity limit reached)");
                    } else {
                        self.launch_wp(&wp_id).await?;
                    }
                }
            }
            ProgressionMode::WaveGated => {
                self.handle_wave_gated_progression(eligible).await?;
            }
        }

        Ok(())
    }

    /// Handle wave-gated progression (pause at boundaries).
    async fn handle_wave_gated_progression(&mut self, eligible: Vec<String>) -> Result<()> {
        // Guard: No eligible WPs
        if eligible.is_empty() {
            return Ok(());
        }

        let run = self.run.read().await;

        // Partition eligible WPs into those in the current approved wave and those beyond
        let (launchable, _beyond): (Vec<String>, Vec<String>) = eligible
            .into_iter()
            .partition(|wp_id| {
                run.work_packages
                    .iter()
                    .find(|wp| wp.id == *wp_id)
                    .is_some_and(|wp| wp.wave <= self.current_wave)
            });

        // Launch WPs that are within the approved wave
        if !launchable.is_empty() {
            drop(run);
            for wp_id in launchable {
                self.launch_wp(&wp_id).await?;
            }
            return Ok(());
        }

        // All eligible WPs are beyond the current wave — check if the current wave is complete
        let all_current_done = run
            .work_packages
            .iter()
            .filter(|wp| wp.wave == self.current_wave)
            .all(|wp| matches!(wp.state, WPState::Completed | WPState::Failed));

        let has_active = run
            .work_packages
            .iter()
            .any(|wp| matches!(wp.state, WPState::Active));

        if all_current_done && !has_active {
            // Pause at wave boundary — wait for operator to advance
            drop(run);
            let mut run = self.run.write().await;
            run.state = RunState::Paused;
            tracing::info!(
                wave = self.current_wave,
                "Wave {} complete. Waiting for operator confirmation to proceed.",
                self.current_wave
            );
        }

        Ok(())
    }

    /// Called when operator confirms wave advance.
    async fn advance_wave(&mut self) -> Result<()> {
        self.current_wave += 1;
        tracing::info!(wave = self.current_wave, "Advancing to wave {}", self.current_wave);
        let mut run = self.run.write().await;
        run.state = RunState::Running;
        drop(run);

        self.launch_eligible_wps_and_notify(false).await
    }

    /// Finalize the orchestration run, marking all completed WPs and the run itself as done.
    ///
    /// This is used when all work packages have been merged to the target branch
    /// and the operator wants to close out the run. Any active WPs are marked completed,
    /// all waves are marked completed, and the run state is set to Completed.
    async fn finalize_run(&mut self) -> Result<()> {
        let mut run = self.run.write().await;
        let now = std::time::SystemTime::now();

        // Mark any non-terminal WPs as completed
        for wp in &mut run.work_packages {
            if !matches!(wp.state, WPState::Completed | WPState::Failed) {
                wp.state = WPState::Completed;
                wp.completed_at = Some(now);
                wp.completion_method = Some(CompletionMethod::Manual);
                tracing::info!(wp_id = %wp.id, "Finalized WP as completed");
            }
        }

        // Mark all waves as completed
        for wave in &mut run.waves {
            wave.state = crate::types::WaveState::Completed;
        }

        // Mark the run itself as completed
        run.state = RunState::Completed;
        run.completed_at = Some(now);

        tracing::info!("Orchestration finalized by operator");

        Ok(())
    }

    /// Launch a single work package.
    async fn launch_wp(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        // Guard: Unknown work package
        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Active, &wp.id)?;
        wp.started_at = Some(std::time::SystemTime::now());
        self.active_panes += 1;

        tracing::info!(
            wp_id = %wp.id,
            active = self.active_panes,
            "WP launched"
        );

        // Track for batch notification
        self.pending_launch_batch.push(wp_id.to_string());

        Ok(())
    }

    /// Restart a failed or crashed work package.
    async fn restart_wp(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        // Guard: Unknown work package
        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Active, &wp.id)?;
        wp.started_at = Some(std::time::SystemTime::now());
        self.active_panes += 1;

        tracing::info!(wp_id = %wp.id, "WP restarted");

        Ok(())
    }

    /// Pause a running work package.
    async fn pause_wp(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        // Guard: Unknown work package
        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Paused, &wp.id)?;
        self.active_panes = self.active_panes.saturating_sub(1);

        tracing::info!(wp_id = %wp.id, "WP paused");

        Ok(())
    }

    /// Resume a paused work package.
    async fn resume_wp(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        // Guard: Unknown work package
        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Active, &wp.id)?;
        self.active_panes += 1;

        tracing::info!(wp_id = %wp.id, "WP resumed");

        Ok(())
    }

    /// Force-advance: treat a failed WP as completed for dependency purposes.
    async fn force_advance(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        // Guard: Unknown work package
        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Completed, &wp.id)?;
        wp.completion_method = Some(CompletionMethod::Manual);

        tracing::warn!(wp_id = %wp.id, "Force-advanced — dependents unblocked");

        drop(run);

        // Launch newly eligible WPs
        self.launch_eligible_wps_and_notify(false).await
    }

    /// Retry a failed work package.
    async fn retry_wp(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        // Guard: Unknown work package
        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Pending, &wp.id)?;
        wp.started_at = None;
        wp.completed_at = None;
        wp.completion_method = None;

        tracing::info!(wp_id = %wp.id, "WP retry initiated");

        drop(run);

        // Try to launch immediately if capacity available
        if self.active_panes < self.run.read().await.config.max_agent_panes {
            self.launch_wp(wp_id).await?;
        } else {
            self.launch_queue.push_back(wp_id.to_string());
        }

        Ok(())
    }

    /// Process the launch queue when capacity becomes available.
    async fn process_launch_queue(&mut self) -> Result<()> {
        let max_panes = self.run.read().await.config.max_agent_panes;

        while self.active_panes < max_panes {
            if let Some(wp_id) = self.launch_queue.pop_front() {
                self.launch_wp(&wp_id).await?;
            } else {
                break;
            }
        }

        Ok(())
    }

    /// Approve a work package in review (ForReview → Completed).
    pub(crate) async fn approve_wp(&mut self, wp_id: &str) -> Result<()> {
        let mut run = self.run.write().await;

        let feature_dir = run.feature_dir.clone();

        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        wp.state = wp.state.transition(WPState::Completed, &wp.id)?;
        wp.completed_at = Some(std::time::SystemTime::now());
        wp.completion_method = Some(CompletionMethod::Manual);

        tracing::info!(wp_id = %wp.id, "WP approved — marked as completed");

        drop(run);

        if let Err(e) = crate::parser::update_task_file_lane(
            &feature_dir,
            wp_id,
            crate::parser::wp_state_to_lane(WPState::Completed),
        ) {
            tracing::warn!(
                wp_id = %wp_id,
                error = %e,
                "Failed to write lane back to task file (hub may show stale state)"
            );
        }

        // Launch newly eligible WPs
        self.launch_eligible_wps_and_notify(false).await
    }

    /// Reject a work package in review.
    /// If `relaunch` is true, transitions ForReview → Active (rework).
    /// If `relaunch` is false, transitions ForReview → Pending (hold).
    pub(crate) async fn reject_wp(&mut self, wp_id: &str, relaunch: bool) -> Result<()> {
        let mut run = self.run.write().await;

        let feature_dir = run.feature_dir.clone();

        let wp = run
            .work_packages
            .iter_mut()
            .find(|w| w.id == wp_id)
            .ok_or_else(|| crate::error::KasmosError::Wave(WaveError::WpNotFound { wp_id: wp_id.to_string() }))?;

        let new_state;
        if relaunch {
            wp.state = wp.state.transition(WPState::Active, &wp.id)?;
            wp.started_at = Some(std::time::SystemTime::now());
            self.active_panes += 1;
            new_state = WPState::Active;
            tracing::info!(wp_id = %wp.id, "WP rejected — relaunching for rework");
        } else {
            wp.state = wp.state.transition(WPState::Pending, &wp.id)?;
            wp.started_at = None;
            wp.completed_at = None;
            new_state = WPState::Pending;
            tracing::info!(wp_id = %wp.id, "WP rejected — held in pending");
        }

        drop(run);

        // Write lane back to task file so the hub (and spec-kitty) see the update.
        if let Err(e) = crate::parser::update_task_file_lane(
            &feature_dir,
            wp_id,
            crate::parser::wp_state_to_lane(new_state),
        ) {
            tracing::warn!(
                wp_id = %wp_id,
                error = %e,
                "Failed to write lane back to task file (hub may show stale state)"
            );
        }

        Ok(())
    }

    /// Recompute wave states from constituent WP states.
    ///
    /// For each wave, examine its WPs and derive the wave state:
    /// - All WPs Completed → Wave Completed
    /// - All WPs terminal (Completed or Failed) with at least one Failed → PartiallyFailed
    /// - Any WP Active/Paused/ForReview → Active
    /// - All WPs Pending → Pending
    async fn update_wave_states(&self) {
        use crate::types::WaveState;

        let mut run = self.run.write().await;

        // First pass: compute new states for each wave by index
        let new_states: Vec<(usize, WaveState)> = run
            .waves
            .iter()
            .map(|wave| {
                let wp_states: Vec<WPState> = wave
                    .wp_ids
                    .iter()
                    .filter_map(|id| run.work_packages.iter().find(|wp| wp.id == *id))
                    .map(|wp| wp.state)
                    .collect();

                let new_state = if wp_states.is_empty() {
                    wave.state
                } else {
                    let all_completed = wp_states
                        .iter()
                        .all(|s| matches!(s, WPState::Completed));
                    let all_terminal = wp_states
                        .iter()
                        .all(|s| matches!(s, WPState::Completed | WPState::Failed));
                    let any_failed = wp_states.iter().any(|s| matches!(s, WPState::Failed));
                    let all_pending = wp_states.iter().all(|s| matches!(s, WPState::Pending));

                    if all_completed {
                        WaveState::Completed
                    } else if all_terminal && any_failed {
                        WaveState::PartiallyFailed
                    } else if all_pending {
                        WaveState::Pending
                    } else {
                        WaveState::Active
                    }
                };

                (wave.index, new_state)
            })
            .collect();

        // Second pass: apply computed states
        for (index, new_state) in new_states {
            if let Some(wave) = run.waves.iter_mut().find(|w| w.index == index) {
                wave.state = new_state;
            }
        }
    }

    /// Check if orchestration is complete (all WPs done).
    async fn is_complete(&self) -> bool {
        let run = self.run.read().await;
        run.work_packages
            .iter()
            .all(|wp| matches!(wp.state, WPState::Completed | WPState::Failed))
    }

}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::types::{Wave, WaveState};
    use std::path::PathBuf;

    /// Helper to create a WaveEngine with a dummy launch_tx for tests.
    fn create_test_engine(
        run: Arc<RwLock<OrchestrationRun>>,
        completion_rx: mpsc::Receiver<CompletionEvent>,
        action_rx: mpsc::Receiver<EngineAction>,
    ) -> (WaveEngine, mpsc::Receiver<WaveLaunchEvent>) {
        let (launch_tx, launch_rx) = mpsc::channel(10);
        let engine = WaveEngine::new(run, completion_rx, action_rx, launch_tx);
        (engine, launch_rx)
    }

    fn create_test_run(
        wps: Vec<(String, Vec<String>, usize)>,
        mode: ProgressionMode,
    ) -> OrchestrationRun {
        let work_packages = wps
            .into_iter()
            .map(|(id, deps, wave)| crate::types::WorkPackage {
                id,
                title: "Test WP".to_string(),
                state: WPState::Pending,
                dependencies: deps,
                wave,
                pane_id: None,
                pane_name: "test".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            })
            .collect();

        OrchestrationRun {
            id: "test-run".to_string(),
            feature: "test".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            config: Config::default(),
            work_packages,
            waves: vec![Wave {
                index: 0,
                wp_ids: vec!["WP01".to_string()],
                state: WaveState::Pending,
            }],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode,
        }
    }

    #[tokio::test]
    async fn test_continuous_auto_launch() {
        let run = Arc::new(RwLock::new(create_test_run(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec!["WP01".to_string()], 1),
            ],
            ProgressionMode::Continuous,
        )));

        let (completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch initial wave
        engine.launch_eligible_wps().await.unwrap();

        // WP01 should be active
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Active);
            assert_eq!(r.work_packages[1].state, WPState::Pending);
        }

        // Complete WP01
        completion_tx
            .send(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        // WP02 should now be active (continuous mode)
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Completed);
            assert_eq!(r.work_packages[1].state, WPState::Active);
        }
    }

    #[tokio::test]
    async fn test_capacity_limiting() {
        let config = Config { max_agent_panes: 2, ..Default::default() };

        let mut run_data = create_test_run(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec![], 0),
                ("WP03".to_string(), vec![], 0),
            ],
            ProgressionMode::Continuous,
        );
        run_data.config = config;

        let run = Arc::new(RwLock::new(run_data));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch eligible WPs
        engine.launch_eligible_wps().await.unwrap();

        // Only 2 should be active, 1 queued
        {
            let r = run.read().await;
            let active_count = r
                .work_packages
                .iter()
                .filter(|wp| matches!(wp.state, WPState::Active))
                .count();
            assert_eq!(active_count, 2);
        }
        assert_eq!(engine.launch_queue.len(), 1);
    }

    #[tokio::test]
    async fn test_partial_failure_blocks_dependents() {
        let run = Arc::new(RwLock::new(create_test_run(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec!["WP01".to_string()], 1),
                ("WP03".to_string(), vec![], 0),
            ],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch initial wave
        engine.launch_eligible_wps().await.unwrap();

        // WP01 and WP03 should be active
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Active);
            assert_eq!(r.work_packages[2].state, WPState::Active);
        }

        // WP01 fails
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                false,
            ))
            .await
            .unwrap();

        // WP01 should be failed, WP02 should still be pending (blocked)
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Failed);
            assert_eq!(r.work_packages[1].state, WPState::Pending);
            assert_eq!(r.work_packages[2].state, WPState::Active);
        }
    }

    #[tokio::test]
    async fn test_force_advance_unblocks_dependents() {
        let run = Arc::new(RwLock::new(create_test_run(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec!["WP01".to_string()], 1),
            ],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch and fail WP01
        engine.launch_eligible_wps().await.unwrap();
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                false,
            ))
            .await
            .unwrap();

        // WP02 should be blocked
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Failed);
            assert_eq!(r.work_packages[1].state, WPState::Pending);
        }

        // Force-advance WP01
        engine.force_advance("WP01").await.unwrap();

        // WP02 should now be active
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Completed);
            assert_eq!(r.work_packages[1].state, WPState::Active);
        }
    }

    #[tokio::test]
    async fn test_is_complete() {
        let run = Arc::new(RwLock::new(create_test_run(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec![], 0),
            ],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);

        // Not complete initially
        assert!(!engine.is_complete().await);

        // Complete both
        {
            let mut r = run.write().await;
            r.work_packages[0].state = WPState::Completed;
            r.work_packages[1].state = WPState::Completed;
        }

        assert!(engine.is_complete().await);
    }

    #[tokio::test]
    async fn test_wave_gated_pause() {
        let run = Arc::new(RwLock::new(create_test_run(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec![], 1),
            ],
            ProgressionMode::WaveGated,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch initial wave
        engine.launch_eligible_wps().await.unwrap();

        // WP01 should be active
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Active);
        }

        // Complete WP01
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        // Should be paused at wave boundary
        {
            let r = run.read().await;
            assert_eq!(r.state, RunState::Paused);
            assert_eq!(r.work_packages[1].state, WPState::Pending);
        }

        // Advance wave
        engine.advance_wave().await.unwrap();

        // Should be running and WP02 active
        {
            let r = run.read().await;
            assert_eq!(r.state, RunState::Running);
            assert_eq!(r.work_packages[1].state, WPState::Active);
        }
    }

    #[tokio::test]
    async fn test_retry_resets_wp() {
        let run = Arc::new(RwLock::new(create_test_run(
            vec![("WP01".to_string(), vec![], 0)],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch and fail WP01
        engine.launch_eligible_wps().await.unwrap();
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                false,
            ))
            .await
            .unwrap();

        // Should be failed
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Failed);
        }

        // Retry
        engine.retry_wp("WP01").await.unwrap();

        // Should be pending and active again
        {
            let r = run.read().await;
            assert_eq!(r.work_packages[0].state, WPState::Active);
        }
    }

    /// Helper to create a test run with proper wave structs derived from WP data.
    fn create_test_run_with_waves(
        wps: Vec<(String, Vec<String>, usize)>,
        mode: ProgressionMode,
    ) -> OrchestrationRun {
        // Build waves from WP wave indices
        let mut wave_map: std::collections::BTreeMap<usize, Vec<String>> =
            std::collections::BTreeMap::new();
        for (id, _, wave_idx) in &wps {
            wave_map.entry(*wave_idx).or_default().push(id.clone());
        }
        let waves: Vec<Wave> = wave_map
            .into_iter()
            .map(|(index, wp_ids)| Wave {
                index,
                wp_ids,
                state: WaveState::Pending,
            })
            .collect();

        let work_packages = wps
            .into_iter()
            .map(|(id, deps, wave)| crate::types::WorkPackage {
                id,
                title: "Test WP".to_string(),
                state: WPState::Pending,
                dependencies: deps,
                wave,
                pane_id: None,
                pane_name: "test".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            })
            .collect();

        OrchestrationRun {
            id: "test-run".to_string(),
            feature: "test".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            config: Config::default(),
            work_packages,
            waves,
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode,
        }
    }

    #[tokio::test]
    async fn test_wave_state_completed_when_all_wps_done() {
        let run = Arc::new(RwLock::new(create_test_run_with_waves(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec![], 0),
                ("WP03".to_string(), vec!["WP01".to_string(), "WP02".to_string()], 1),
            ],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch wave 0
        engine.launch_eligible_wps().await.unwrap();
        engine.update_wave_states().await;

        // Wave 0 should be Active (WPs launched)
        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::Active);
            assert_eq!(r.waves[1].state, WaveState::Pending);
        }

        // Complete WP01
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        // Wave 0 still Active (WP02 still running)
        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::Active);
        }

        // Complete WP02
        engine
            .handle_completion(CompletionEvent::new(
                "WP02".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        // Wave 0 should now be Completed, wave 1 Active (WP03 launched)
        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::Completed);
            assert_eq!(r.waves[1].state, WaveState::Active);
        }
    }

    #[tokio::test]
    async fn test_wave_state_partially_failed() {
        let run = Arc::new(RwLock::new(create_test_run_with_waves(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec![], 0),
            ],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Launch wave 0
        engine.launch_eligible_wps().await.unwrap();
        engine.update_wave_states().await;

        // Complete WP01 successfully
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        // Fail WP02
        engine
            .handle_completion(CompletionEvent::new(
                "WP02".to_string(),
                CompletionMethod::AutoDetected,
                false,
            ))
            .await
            .unwrap();

        // Wave 0 should be PartiallyFailed
        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::PartiallyFailed);
        }
    }

    #[tokio::test]
    async fn test_watch_channel_broadcasts_on_completion() {
        let run_data = create_test_run_with_waves(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec!["WP01".to_string()], 1),
            ],
            ProgressionMode::Continuous,
        );

        let (watch_tx, mut watch_rx) = watch::channel(run_data.clone());

        let run = Arc::new(RwLock::new(run_data));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine = engine.with_watch_tx(watch_tx);
        engine.init_graph().await.unwrap();

        // Launch initial wave — WP01 should become Active
        engine.launch_eligible_wps_and_notify(true).await.unwrap();

        // The watch channel should have received a broadcast with WP01 Active
        assert!(watch_rx.has_changed().unwrap());
        watch_rx.mark_changed(); // ensure we catch the next update
        let state = watch_rx.borrow_and_update().clone();
        assert_eq!(state.work_packages[0].state, WPState::Active);
        assert_eq!(state.work_packages[1].state, WPState::Pending);

        // Complete WP01 — should trigger broadcast with WP01 Completed and WP02 Active
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        // Verify the watch channel received the updated state
        let state = watch_rx.borrow_and_update().clone();
        assert_eq!(state.work_packages[0].state, WPState::Completed);
        assert_eq!(state.work_packages[1].state, WPState::Active);
    }

    #[tokio::test]
    async fn test_watch_channel_none_does_not_panic() {
        // Ensure engine works fine without a watch channel (Option::None path)
        let run = Arc::new(RwLock::new(create_test_run_with_waves(
            vec![("WP01".to_string(), vec![], 0)],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        // No watch_tx set — should not panic
        engine.init_graph().await.unwrap();
        engine.launch_eligible_wps_and_notify(true).await.unwrap();

        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        let r = run.read().await;
        assert_eq!(r.work_packages[0].state, WPState::Completed);
    }

    #[tokio::test]
    async fn test_wave_state_active_when_wps_in_flight() {
        let run = Arc::new(RwLock::new(create_test_run_with_waves(
            vec![
                ("WP01".to_string(), vec![], 0),
                ("WP02".to_string(), vec![], 0),
            ],
            ProgressionMode::Continuous,
        )));

        let (_completion_tx, completion_rx) = mpsc::channel(10);
        let (_action_tx, action_rx) = mpsc::channel(10);

        let (mut engine, _launch_rx) = create_test_engine(run.clone(), completion_rx, action_rx);
        engine.init_graph().await.unwrap();

        // Before launch, wave should be Pending
        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::Pending);
        }

        // Launch wave 0
        engine.launch_eligible_wps().await.unwrap();
        engine.update_wave_states().await;

        // Wave 0 should be Active
        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::Active);
        }

        // Complete one WP — wave still Active (other WP still running)
        engine
            .handle_completion(CompletionEvent::new(
                "WP01".to_string(),
                CompletionMethod::AutoDetected,
                true,
            ))
            .await
            .unwrap();

        {
            let r = run.read().await;
            assert_eq!(r.waves[0].state, WaveState::Active);
        }
    }
}
