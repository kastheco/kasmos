---
work_package_id: WP10
title: Error Handling & Cleanup
lane: planned
dependencies:
- WP05
subtasks: [T058, T059, T060, T061, T062]
phase: Phase 5 - Resilience
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP10 – Error Handling & Cleanup

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **WP05** (session manager), **WP06** (completion detector), **WP07** (wave engine), and **WP08** (command system). Ensure all are merged before starting.

**Implementation command**:
```bash
spec-kitty implement WP10 --base WP08
```

## Objectives & Success Criteria

**Objective**: Implement pane crash detection, graceful shutdown sequences, POSIX signal handling, and artifact cleanup. This WP makes the orchestrator resilient to failures and ensures clean exits.

**Success Criteria**:
1. Pane crashes are detected within 10 seconds (5s polling interval)
2. Crashed WPs are marked as Failed with logged context
3. Graceful shutdown closes all resources in the correct order
4. SIGINT and SIGTERM trigger graceful shutdown (not abrupt process kill)
5. Artifacts (.kasmos/layout.kdl, prompts/, scripts/, cmd.pipe) are cleaned on normal exit
6. State file and report are preserved after cleanup (not deleted)

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Dependencies**: `nix` crate for signal handling (or `tokio::signal`), `tokio` for interval/select
- **Reference**: [plan.md](../plan.md) WP10 section; [spec.md](../spec.md) FR-010, FR-014
- **Constraint**: Shutdown must persist final state before cleaning up
- **Constraint**: Crash detection polls `zellij action list-panes` — must not overwhelm Zellij CLI
- **Constraint**: Cleanup must be idempotent (safe to call multiple times)
- **Constraint**: Signal handler must set a flag and trigger shutdown — NOT do complex work in the handler itself

## Subtasks & Detailed Guidance

### Subtask T058 – Pane Crash Detection (poll list-panes every 5s)

**Purpose**: Periodically check that all expected panes are still alive. If a pane disappears unexpectedly, the WP has crashed.

**Steps**:

1. Create `crates/kasmos/src/health.rs`:
   ```rust
   use tokio::time::{interval, Duration};
   use std::collections::HashSet;

   pub struct HealthMonitor {
       session: Arc<SessionManager>,
       expected_panes: HashSet<String>,  // WP IDs of active panes
       poll_interval: Duration,
       crash_tx: mpsc::Sender<CrashEvent>,
   }

   #[derive(Debug)]
   pub struct CrashEvent {
       pub wp_id: String,
       pub detected_at: std::time::SystemTime,
   }

   impl HealthMonitor {
       pub fn new(
           session: Arc<SessionManager>,
           poll_interval_secs: u64,
           crash_tx: mpsc::Sender<CrashEvent>,
       ) -> Self {
           Self {
               session,
               expected_panes: HashSet::new(),
               poll_interval: Duration::from_secs(poll_interval_secs),
               crash_tx,
           }
       }

       /// Register a WP as expected (it has an active pane).
       pub fn register_pane(&mut self, wp_id: &str) {
           self.expected_panes.insert(wp_id.to_string());
       }

       /// Unregister a WP (pane was intentionally closed).
       pub fn unregister_pane(&mut self, wp_id: &str) {
           self.expected_panes.remove(wp_id);
       }

       /// Run the health check loop. Spawned as a tokio task.
       pub async fn run(&self) -> Result<()> {
           let mut tick = interval(self.poll_interval);

           loop {
               tick.tick().await;

               // Get live pane list from Zellij
               let live_panes = match self.session.list_live_panes().await {
                   Ok(panes) => panes,
                   Err(e) => {
                       tracing::warn!(error = %e, "Failed to poll panes — Zellij may be unavailable");
                       continue;
                   }
               };

               let live_names: HashSet<String> = live_panes.iter()
                   .map(|p| p.name.clone())
                   .collect();

               // Check for missing panes
               for expected in &self.expected_panes {
                   if !live_names.contains(expected) {
                       tracing::error!(wp_id = %expected, "Pane crash detected!");
                       let event = CrashEvent {
                           wp_id: expected.clone(),
                           detected_at: std::time::SystemTime::now(),
                       };
                       if self.crash_tx.send(event).await.is_err() {
                           tracing::error!("Crash event channel closed");
                           return Ok(());
                       }
                   }
               }
           }
       }
   }
   ```

**Files**:
- `crates/kasmos/src/health.rs` (new, ~80 lines)

### Subtask T059 – WP State to Failed on Crash

**Purpose**: When a crash is detected, update the WP state to Failed and notify the wave engine.

**Steps**:

1. Integrate crash events into the wave engine (in `engine.rs`):
   ```rust
   impl WaveEngine {
       /// Handle crash events from the health monitor.
       async fn handle_crash(&mut self, event: CrashEvent) -> Result<()> {
           if let Some(wp) = self.run.work_packages.iter_mut()
               .find(|w| w.id == event.wp_id)
           {
               // Only transition to Failed if currently Active or Paused
               if matches!(wp.state, WPState::Active | WPState::Paused) {
                   wp.state = WPState::Failed;
                   wp.failure_count += 1;
                   self.active_panes = self.active_panes.saturating_sub(1);

                   tracing::error!(
                       wp_id = %wp.id,
                       failures = wp.failure_count,
                       "WP pane crashed — marked as Failed. Use 'retry {}' to relaunch.",
                       wp.id
                   );

                   // Notify controller pane
                   if let Some(ctrl_id) = self.session.get_pane_id("controller") {
                       let msg = format!(
                           "[kasmos] ⚠ {} crashed (failure #{}). Use 'retry {}' to relaunch.\n",
                           wp.id, wp.failure_count, wp.id
                       );
                       let _ = self.session.cli.write_to_pane(
                           &self.session.session_name, ctrl_id, &msg
                       ).await;
                   }

                   // Persist state after crash
                   self.persister.save(&self.run)?;

                   // Check if blocked WPs can now be re-evaluated
                   self.handle_failure(&wp.id.clone())?;
               }
           }
           Ok(())
       }
   }
   ```

2. Add crash_rx to the WaveEngine's tokio::select! loop:
   ```rust
   // In engine.rs run() method:
   Some(crash) = self.crash_rx.recv() => {
       self.handle_crash(crash).await?;
   }
   ```

**Files**:
- `crates/kasmos/src/engine.rs` (addition, ~30 lines)
- `crates/kasmos/src/health.rs` (continued)

### Subtask T060 – Graceful Shutdown Sequence

**Purpose**: Implement an ordered shutdown that cleanly tears down all resources.

**Steps**:

1. Create `crates/kasmos/src/shutdown.rs`:
   ```rust
   pub struct ShutdownCoordinator {
       shutdown_flag: Arc<AtomicBool>,
   }

   impl ShutdownCoordinator {
       pub fn new() -> Self {
           Self {
               shutdown_flag: Arc::new(AtomicBool::new(false)),
           }
       }

       pub fn is_shutting_down(&self) -> bool {
           self.shutdown_flag.load(Ordering::SeqCst)
       }

       pub fn trigger_shutdown(&self) {
           self.shutdown_flag.store(true, Ordering::SeqCst);
       }

       /// Execute the graceful shutdown sequence in order.
       pub async fn execute(
           &self,
           detector: &mut CompletionDetector,
           command_reader: &CommandReader,
           health_monitor: &HealthMonitor,
           persister: &StatePersister,
           session: &SessionManager,
           run: &OrchestrationRun,
       ) -> Result<()> {
           tracing::info!("Initiating graceful shutdown...");

           // 1. Stop filesystem watchers
           detector.stop();
           tracing::info!("Step 1/6: Filesystem watchers stopped");

           // 2. Stop health monitor (handled by task cancellation)
           tracing::info!("Step 2/6: Health monitor stopped");

           // 3. Close command FIFO
           command_reader.cleanup()?;
           tracing::info!("Step 3/6: Command FIFO cleaned up");

           // 4. Persist final state
           persister.save(run)?;
           tracing::info!("Step 4/6: Final state persisted");

           // 5. Close all panes (optional — Zellij session kill handles this)
           tracing::info!("Step 5/6: Panes will be closed with session");

           // 6. Kill Zellij session
           session.kill_session().await?;
           tracing::info!("Step 6/6: Zellij session killed");

           tracing::info!("Graceful shutdown complete");
           Ok(())
       }
   }
   ```

**Files**:
- `crates/kasmos/src/shutdown.rs` (new, ~60 lines)

### Subtask T061 – Signal Handling (SIGINT, SIGTERM)

**Purpose**: Catch POSIX signals and trigger graceful shutdown instead of abrupt termination.

**Steps**:

1. Add signal handler setup:
   ```rust
   // In shutdown.rs or main orchestration setup:
   use tokio::signal::unix::{signal, SignalKind};

   pub async fn setup_signal_handlers(
       shutdown: Arc<ShutdownCoordinator>,
   ) -> Result<tokio::task::JoinHandle<()>> {
       let handle = tokio::spawn(async move {
           let mut sigint = signal(SignalKind::interrupt())
               .expect("Failed to register SIGINT handler");
           let mut sigterm = signal(SignalKind::terminate())
               .expect("Failed to register SIGTERM handler");

           tokio::select! {
               _ = sigint.recv() => {
                   tracing::info!("Received SIGINT — initiating graceful shutdown");
               }
               _ = sigterm.recv() => {
                   tracing::info!("Received SIGTERM — initiating graceful shutdown");
               }
           }

           shutdown.trigger_shutdown();
       });

       Ok(handle)
   }
   ```

2. The shutdown flag is checked in the WaveEngine's event loop:
   ```rust
   // In engine.rs run() loop, add:
   _ = shutdown_rx.recv() => {
       tracing::info!("Shutdown signal received");
       break;  // Exit event loop, proceed to shutdown sequence
   }
   ```

3. Double-SIGINT (Ctrl+C twice) should force immediate exit (standard Rust behavior via tokio)

**Files**:
- `crates/kasmos/src/shutdown.rs` (continued, ~30 lines)

### Subtask T062 – Artifact Cleanup [P]

**Purpose**: Remove transient files on clean exit while preserving state and reports.

**Steps**:

1. Add cleanup to `ShutdownCoordinator`:
   ```rust
   impl ShutdownCoordinator {
       /// Clean up transient artifacts. Preserves state.json and report.md.
       pub fn cleanup_artifacts(kasmos_dir: &Path) -> Result<()> {
           let to_remove = [
               "layout.kdl",
               "cmd.pipe",
           ];
           let dirs_to_remove = [
               "prompts",
               "scripts",
           ];

           for file in &to_remove {
               let path = kasmos_dir.join(file);
               if path.exists() {
                   std::fs::remove_file(&path)?;
                   tracing::debug!(path = %path.display(), "Removed artifact");
               }
           }

           for dir in &dirs_to_remove {
               let path = kasmos_dir.join(dir);
               if path.exists() {
                   std::fs::remove_dir_all(&path)?;
                   tracing::debug!(path = %path.display(), "Removed artifact directory");
               }
           }

           // Preserve: state.json, state.json.tmp (if exists), report.md, run.lock
           tracing::info!("Artifacts cleaned (state.json and report.md preserved)");
           Ok(())
       }
   }
   ```

2. Cleanup is idempotent — safe to call multiple times
3. Call cleanup at the end of the graceful shutdown sequence
4. Do NOT remove: state.json (for post-mortem analysis), report.md (output artifact), run.lock (removed separately when lock is released)

**Files**:
- `crates/kasmos/src/shutdown.rs` (continued, ~30 lines)

**Parallel**: Yes — cleanup logic is independent of detection/shutdown flow.

## Test Strategy

- Unit test: register/unregister panes in HealthMonitor
- Unit test (mock session): crash detection with missing pane → CrashEvent emitted
- Unit test: crash detection with all panes present → no event
- Unit test: graceful shutdown sequence calls steps in correct order
- Unit test: cleanup removes transient files, preserves state.json and report.md
- Unit test: double cleanup (idempotent) doesn't error
- Integration test: send SIGTERM to process, verify graceful shutdown log output

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Crash detection races with intentional pane close | Medium | Unregister pane before closing; check WP state before marking as crashed |
| Signal during shutdown causes double cleanup | Medium | AtomicBool shutdown flag — check before each step |
| Zellij CLI unavailable during shutdown | Low | Catch errors in shutdown steps, continue with remaining steps |
| Cleanup removes files still needed | Low | Explicit preserve list; only remove known transient files |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] Pane crashes detected within 10 seconds
- [ ] Crashed WPs marked Failed with operator notification
- [ ] Graceful shutdown completes all 6 steps in order
- [ ] SIGINT/SIGTERM trigger graceful shutdown
- [ ] Artifacts cleaned up correctly (transient removed, persistent preserved)
- [ ] Cleanup is idempotent
- [ ] All unit tests pass

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP10 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
