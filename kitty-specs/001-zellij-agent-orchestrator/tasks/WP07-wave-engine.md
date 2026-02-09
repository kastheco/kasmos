---
work_package_id: WP07
title: Wave Engine
lane: "doing"
dependencies:
- WP01
base_branch: 001-zellij-agent-orchestrator-WP01
base_commit: eb6d2fdce54e8e2cd1773b50e133e860760a33f2
created_at: '2026-02-09T04:42:25.724533+00:00'
subtasks: [T040, T041, T042, T043, T044]
phase: Phase 4 - Control
assignee: ''
agent: ''
shell_pid: "3535601"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP07 – Wave Engine

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **WP01** (core types), **WP02** (dependency graph), **WP05** (session manager), and **WP06** (completion detector). Ensure all are merged before starting.

**Implementation command**:
```bash
spec-kitty implement WP07 --base WP06
```

## Objectives & Success Criteria

**Objective**: Implement the wave progression engine that drives the orchestration lifecycle. It receives completion events from the detector, checks dependency satisfaction, and launches the next eligible work packages. Supports both continuous mode (auto-launch as deps resolve) and wave-gated mode (pause for operator confirmation at wave boundaries).

**Success Criteria**:
1. Wave progression correctly advances when all WPs in a wave complete
2. Continuous mode auto-launches eligible WPs immediately when dependencies resolve
3. Wave-gated mode pauses at wave boundaries and waits for operator confirmation
4. Capacity limiting enforces max 8 concurrent agent panes, queuing excess
5. Partial wave failure blocks only direct dependents, not the entire wave
6. Wave engine handles all WPState transitions correctly
7. Event loop is clean and non-blocking (tokio::select! based)

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Input**: CompletionEvent from mpsc channel (WP06), operator commands via separate channel (WP08)
- **Output**: Pane launch/close commands to SessionManager (WP05), state updates to OrchestrationRun
- **Reference**: [plan.md](../plan.md) WP07 section; [spec.md](../spec.md) FR-008, FR-015
- **Constraint**: Single-threaded event loop via tokio::select! to prevent race conditions
- **Constraint**: Max 8 concurrent agent panes (configurable via Config)
- **Constraint**: Failed WPs block only their direct dependents in the dependency graph

## Subtasks & Detailed Guidance

### Subtask T040 – Wave Progression Logic

**Purpose**: Core logic that determines when to advance the orchestration: check completion status, evaluate dependencies, decide what to launch next.

**Steps**:

1. Create `crates/kasmos/src/engine.rs`:
   ```rust
   use tokio::sync::mpsc;

   pub struct WaveEngine {
       run: OrchestrationRun,
       session: SessionManager,
       graph: DependencyGraph,
       completion_rx: mpsc::Receiver<CompletionEvent>,
       command_rx: mpsc::Receiver<ControllerCommand>,  // From WP08
       active_panes: usize,
   }

   impl WaveEngine {
       pub fn new(
           run: OrchestrationRun,
           session: SessionManager,
           graph: DependencyGraph,
           completion_rx: mpsc::Receiver<CompletionEvent>,
           command_rx: mpsc::Receiver<ControllerCommand>,
       ) -> Self {
           Self {
               run,
               session,
               graph,
               completion_rx,
               command_rx,
               active_panes: 0,
           }
       }

       /// Main event loop — runs until orchestration completes or aborts.
       pub async fn run(&mut self) -> Result<()> {
           // Launch initial wave
           self.launch_eligible_wps().await?;

           loop {
               tokio::select! {
                   // Handle completion events
                   Some(event) = self.completion_rx.recv() => {
                       self.handle_completion(event).await?;
                   }
                   // Handle operator commands (from WP08)
                   Some(cmd) = self.command_rx.recv() => {
                       self.handle_command(cmd).await?;
                   }
                   // All channels closed — done
                   else => break,
               }

               // Check if orchestration is complete
               if self.is_complete() {
                   tracing::info!("Orchestration complete!");
                   self.run.state = RunState::Completed;
                   break;
               }
           }

           Ok(())
       }

       fn handle_completion(&mut self, event: CompletionEvent) -> Result<()> {
           // 1. Update WP state to Completed
           if let Some(wp) = self.run.work_packages.iter_mut().find(|w| w.id == event.wp_id) {
               wp.state = wp.state.transition(WPState::Completed, &wp.id)?;
               wp.completed_at = Some(std::time::SystemTime::now());
               wp.completion_method = Some(event.method);
               self.active_panes -= 1;
           }

           // 2. Check for newly eligible WPs and launch them
           self.launch_eligible_wps().await?;

           Ok(())
       }

       fn is_complete(&self) -> bool {
           self.run.work_packages.iter().all(|wp|
               matches!(wp.state, WPState::Completed | WPState::Failed)
           )
       }
   }
   ```

**Files**:
- `crates/kasmos/src/engine.rs` (new, ~100 lines)

### Subtask T041 – Continuous Mode (Auto-Launch on Dependency Resolution)

**Purpose**: In continuous mode, launch WPs as soon as all their dependencies are satisfied, without waiting for the entire wave to complete.

**Steps**:

1. Add to `WaveEngine`:
   ```rust
   impl WaveEngine {
       /// Find and launch all eligible work packages.
       async fn launch_eligible_wps(&mut self) -> Result<()> {
           let completed: HashSet<String> = self.run.work_packages.iter()
               .filter(|wp| matches!(wp.state, WPState::Completed))
               .map(|wp| wp.id.clone())
               .collect();

           let eligible: Vec<String> = self.run.work_packages.iter()
               .filter(|wp| matches!(wp.state, WPState::Pending))
               .filter(|wp| self.graph.deps_satisfied(&wp.id, &completed))
               .map(|wp| wp.id.clone())
               .collect();

           match self.run.mode {
               ProgressionMode::Continuous => {
                   for wp_id in eligible {
                       if self.active_panes >= self.run.config.max_agent_panes {
                           tracing::info!(wp_id = %wp_id, "Queued (capacity limit reached)");
                           continue;
                       }
                       self.launch_wp(&wp_id).await?;
                   }
               }
               ProgressionMode::WaveGated => {
                   self.handle_wave_gated_progression(eligible).await?;
               }
           }

           Ok(())
       }

       async fn launch_wp(&mut self, wp_id: &str) -> Result<()> {
           if let Some(wp) = self.run.work_packages.iter_mut().find(|w| w.id == *wp_id) {
               wp.state = wp.state.transition(WPState::Active, wp_id)?;
               wp.started_at = Some(std::time::SystemTime::now());
               // Session manager handles the actual Zellij pane operations
               // For initial wave, panes are already created via layout
               // For subsequent waves, new panes need to be created
               self.active_panes += 1;
               tracing::info!(wp_id = wp_id, active = self.active_panes, "WP launched");
           }
           Ok(())
       }
   }
   ```

**Files**:
- `crates/kasmos/src/engine.rs` (continued, ~50 lines)

### Subtask T042 – Wave-Gated Mode (Operator Confirmation)

**Purpose**: In wave-gated mode, pause at wave boundaries and prompt the operator for confirmation before launching the next wave.

**Steps**:

1. Add wave-gated logic:
   ```rust
   impl WaveEngine {
       async fn handle_wave_gated_progression(&mut self, eligible: Vec<String>) -> Result<()> {
           if eligible.is_empty() {
               return Ok(());
           }

           // Check if we're at a wave boundary
           let current_wave = self.get_current_wave();
           let all_current_wave_done = self.run.work_packages.iter()
               .filter(|wp| wp.wave == current_wave)
               .all(|wp| matches!(wp.state, WPState::Completed | WPState::Failed));

           if all_current_wave_done {
               // Pause and wait for operator confirmation
               self.run.state = RunState::Paused;
               tracing::info!(
                   wave = current_wave,
                   next_wps = ?eligible,
                   "Wave {} complete. Waiting for operator confirmation to proceed.",
                   current_wave
               );

               // Send notification to controller (write to controller pane)
               let msg = format!(
                   "[kasmos] Wave {} complete. {} WPs ready. Type 'advance' to proceed.\n",
                   current_wave,
                   eligible.len()
               );
               if let Some(controller_id) = self.session.get_pane_id("controller") {
                   self.session.cli.write_to_pane(
                       &self.session.session_name,
                       controller_id,
                       &msg,
                   ).await?;
               }

               // Don't launch — wait for advance command via command_rx
           }

           Ok(())
       }

       /// Called when operator confirms wave advance.
       async fn advance_wave(&mut self) -> Result<()> {
           self.run.state = RunState::Running;
           self.launch_eligible_wps().await
       }

       fn get_current_wave(&self) -> usize {
           self.run.work_packages.iter()
               .filter(|wp| matches!(wp.state, WPState::Active))
               .map(|wp| wp.wave)
               .max()
               .unwrap_or(0)
       }
   }
   ```

**Files**:
- `crates/kasmos/src/engine.rs` (continued, ~50 lines)

### Subtask T043 – Capacity Limiting (Max 8 Agent Panes)

**Purpose**: Enforce a maximum number of concurrent agent panes. When the limit is reached, queue eligible WPs and launch them as slots free up.

**Steps**:

1. Capacity tracking is already integrated in T041 via `self.active_panes` and the capacity check in `launch_eligible_wps`.

2. Add a launch queue for overflow:
   ```rust
   pub struct WaveEngine {
       // ... existing fields
       launch_queue: VecDeque<String>,  // WP IDs waiting for capacity
   }

   impl WaveEngine {
       /// After a completion, check if queued WPs can now be launched.
       fn process_launch_queue(&mut self) -> Result<()> {
           while self.active_panes < self.run.config.max_agent_panes {
               if let Some(wp_id) = self.launch_queue.pop_front() {
                   self.launch_wp(&wp_id).await?;
               } else {
                   break;
               }
           }
           Ok(())
       }
   }
   ```

3. The capacity limit is read from `Config.max_agent_panes` (default 8, set in WP01)

**Files**:
- `crates/kasmos/src/engine.rs` (continued, ~20 lines)

### Subtask T044 – Partial Wave Failure Policy

**Purpose**: When a WP fails, only its direct dependents are blocked — not the entire wave or orchestration.

**Steps**:

1. Add failure handling:
   ```rust
   impl WaveEngine {
       fn handle_failure(&mut self, wp_id: &str) -> Result<()> {
           // 1. Mark WP as failed
           if let Some(wp) = self.run.work_packages.iter_mut().find(|w| w.id == *wp_id) {
               wp.state = wp.state.transition(WPState::Failed, wp_id)?;
               wp.failure_count += 1;
               self.active_panes -= 1;
           }

           // 2. Find direct dependents and log which are blocked
           let blocked: Vec<String> = self.graph.dependents
               .get(wp_id)
               .cloned()
               .unwrap_or_default();

           if !blocked.is_empty() {
               tracing::warn!(
                   failed_wp = wp_id,
                   blocked = ?blocked,
                   "WP failed — blocking {} direct dependents. Use 'retry {}' or 'force-advance {}' to unblock.",
                   blocked.len(), wp_id, wp_id
               );
           }

           // 3. Non-dependent WPs in the same wave continue normally
           // (This is automatic — launch_eligible_wps checks deps_satisfied)

           // 4. Process launch queue (a freed slot might allow queued WPs)
           self.process_launch_queue().await?;

           Ok(())
       }

       /// Force-advance: treat a failed WP as completed for dependency purposes.
       fn force_advance(&mut self, wp_id: &str) -> Result<()> {
           // Mark as "force-advanced" (special Completed variant or flag)
           if let Some(wp) = self.run.work_packages.iter_mut().find(|w| w.id == *wp_id) {
               wp.state = WPState::Completed;
               wp.completion_method = Some(CompletionMethod::Manual);
               tracing::warn!(wp_id = wp_id, "Force-advanced — dependents unblocked");
           }
           self.launch_eligible_wps().await
       }
   }
   ```

**Files**:
- `crates/kasmos/src/engine.rs` (continued, ~50 lines)

## Test Strategy

- Unit test (mock session + detector): simulate 3-wave orchestration → verify correct launch order
- Unit test: continuous mode launches WP as soon as deps resolve
- Unit test: wave-gated mode pauses at boundary, advances on command
- Unit test: capacity limit queues 9th WP, launches when one completes
- Unit test: failed WP blocks dependent but not independent WPs
- Unit test: force-advance unblocks dependents
- Unit test: is_complete returns true when all WPs are Completed or Failed

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Race between completion detection and pane launch | High | Single-threaded tokio::select! event loop |
| Wave-gated confirmation never arrives | Medium | Timeout with reminder every 60s |
| Capacity limit not decremented on crash | Medium | Crash handler (WP10) decrements active_panes |
| Event loop exits prematurely | Medium | Only break on explicit complete/abort, not channel close |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] Continuous mode launches WPs immediately on dep resolution
- [ ] Wave-gated mode pauses and prompts at wave boundaries
- [ ] Capacity limit enforced (queue + launch on free)
- [ ] Partial failure blocks only direct dependents
- [ ] Force-advance and retry work correctly
- [ ] Event loop handles all event types cleanly
- [ ] Unit tests cover multi-wave scenarios

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP07 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
