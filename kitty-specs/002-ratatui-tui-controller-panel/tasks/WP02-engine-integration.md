---
work_package_id: "WP02"
subtasks:
  - "T006"
  - "T007"
  - "T008"
  - "T009"
  - "T010"
  - "T011"
  - "T056"
title: "Engine Integration — Watch Channel & Review States"
phase: "Phase 1 - Foundation"
lane: "done"
dependencies: ["WP01"]
assignee: "unknown"
agent: "reviewer"
shell_pid: "1415369"
review_status: "has_feedback"
reviewed_by: "kas"
history:
  - timestamp: "2026-02-10T22:00:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
  - timestamp: "2026-02-11T03:51:00Z"
    lane: "for_review"
    agent: "claude-sonnet-4-5"
    shell_pid: ""
    action: "Implementation completed - all 6 subtasks done, 213 tests passing"
---

# Work Package Prompt: WP02 – Engine Integration — Watch Channel & Review States

## Objectives & Success Criteria

- WaveEngine broadcasts `OrchestrationRun` state via `tokio::sync::watch` after every mutation
- `WPState::ForReview` variant exists with valid state machine transitions
- `EngineAction::Approve` and `EngineAction::Reject` variants exist with working handlers
- Completion detector distinguishes `for_review` from `done` lane transitions
- TUI is spawned from `launch.rs` and receives state updates
- Engine emits review-ready events when WPs transition to ForReview
- All existing tests pass, new transitions have test coverage

**Implementation command**: `spec-kitty implement WP02 --base WP01`

## Context & Constraints

- **Engine**: `crates/kasmos/src/engine.rs` — WaveEngine struct, `tokio::select!` loop with `completion_rx` + `action_rx`
- **Types**: `crates/kasmos/src/types.rs` — OrchestrationRun, WPState, EngineAction
- **State machine**: `crates/kasmos/src/state_machine.rs` — `can_transition_to()`, `transition()`
- **Detector**: `crates/kasmos/src/detector.rs` — CompletionDetector, watches task file frontmatter lane changes
- **Command handlers**: `crates/kasmos/src/command_handlers.rs` — EngineAction enum defined here
- **Launch**: `crates/kasmos/src/launch.rs` (binary crate) — 17-step orchestration pipeline
- **Lib**: `crates/kasmos/src/lib.rs` — public API exports

**Key architecture**: Channel topology is `engine → watch_tx → TUI`, `TUI → action_tx → engine`. Both FIFO and TUI produce to the same `action_rx`.

## Subtasks & Detailed Guidance

### Subtask T006 – Add `watch::Sender` to WaveEngine, broadcast after mutations

**Purpose**: Enable the TUI to receive real-time state updates from the engine without polling.

**Steps**:
1. In `crates/kasmos/src/engine.rs`, add field to `WaveEngine`:
   ```rust
   pub struct WaveEngine {
       run: Arc<RwLock<OrchestrationRun>>,
       graph: DependencyGraph,
       completion_rx: mpsc::Receiver<CompletionEvent>,
       action_rx: mpsc::Receiver<EngineAction>,
       launch_queue: VecDeque<String>,
       active_panes: usize,
       current_wave: usize,
       watch_tx: watch::Sender<OrchestrationRun>,  // NEW
   }
   ```

2. Update `WaveEngine::new()` constructor to accept `watch_tx: watch::Sender<OrchestrationRun>`

3. Add a helper method to broadcast state:
   ```rust
   async fn broadcast_state(&self) {
       let run = self.run.read().await;
       let _ = self.watch_tx.send(run.clone());
   }
   ```

4. Call `self.broadcast_state().await` after every state-mutating operation:
   - After `handle_completion()` in the event loop
   - After `handle_action()` in the event loop
   - After `launch_wp()` (WP state changes to Active)
   - After `handle_wave_gated_progression()` when pausing at wave boundary
   - After initial wave launch in `run()` method

5. Use `let _ =` to ignore send errors (TUI may have exited before engine)

**Files**: `crates/kasmos/src/engine.rs` (~20 lines changed)

**Notes**: `OrchestrationRun` must implement `Clone`. Check if it already does — it has `#[derive(Serialize, Deserialize)]` but may not have `Clone`. If missing, add `#[derive(Clone)]` to `OrchestrationRun`, `WorkPackage`, `Wave`, `Config`, and any nested types. All fields are owned types (String, PathBuf, Option, Vec) so Clone is derivable.

### Subtask T007 – Add `ForReview` variant to `WPState` + state machine transitions

**Purpose**: The TUI needs to distinguish WPs awaiting review from those that are truly completed. Adding ForReview as a first-class state enables clean rendering and action filtering.

**Steps**:
1. In `crates/kasmos/src/types.rs`, add variant:
   ```rust
   #[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
   #[serde(rename_all = "snake_case")]
   pub enum WPState {
       Pending,
       Active,
       Completed,
       Failed,
       Paused,
       ForReview,  // NEW
   }
   ```

2. In `crates/kasmos/src/state_machine.rs`, add transitions:
   - `Active → ForReview` (agent completed, task file lane is for_review)
   - `ForReview → Completed` (operator approves)
   - `ForReview → Active` (operator rejects with relaunch)
   - `ForReview → Pending` (operator rejects with hold)

3. Update `can_transition_to()` match arms:
   ```rust
   (WPState::Active, WPState::ForReview) => true,
   (WPState::ForReview, WPState::Completed) => true,
   (WPState::ForReview, WPState::Active) => true,
   (WPState::ForReview, WPState::Pending) => true,
   ```

4. Add tests for new transitions in the existing state_machine test module

5. Update `engine.rs` `is_complete()` logic: ForReview WPs should NOT count as complete — only Completed and Failed

**Files**:
- `crates/kasmos/src/types.rs` (~3 lines)
- `crates/kasmos/src/state_machine.rs` (~15 lines)
- `crates/kasmos/src/engine.rs` — update `is_complete()` check

### Subtask T008 – Add `Approve` and `Reject` to `EngineAction`, implement handlers

**Purpose**: Enable the TUI (and FIFO) to approve or reject WPs in the review state.

**Steps**:
1. In `crates/kasmos/src/command_handlers.rs`, extend `EngineAction`:
   ```rust
   pub enum EngineAction {
       Restart(String),
       Pause(String),
       Resume(String),
       ForceAdvance(String),
       Retry(String),
       Advance,
       Abort,
       Approve(String),                                    // NEW
       Reject { wp_id: String, relaunch: bool },           // NEW
   }
   ```

2. In `crates/kasmos/src/engine.rs`, add handler methods:
   ```rust
   async fn handle_approve(&mut self, wp_id: &str) -> Result<()> {
       let mut run = self.run.write().await;
       let wp = run.work_packages.iter_mut()
           .find(|wp| wp.id == wp_id)
           .ok_or_else(|| anyhow!("WP not found: {}", wp_id))?;
       wp.state.transition(WPState::Completed, &wp.id)?;
       wp.completed_at = Some(SystemTime::now());
       wp.completion_method = Some(CompletionMethod::Manual);
       Ok(())
       // After return, caller broadcasts state + launches dependents
   }

   async fn handle_reject(&mut self, wp_id: &str, relaunch: bool) -> Result<()> {
       let mut run = self.run.write().await;
       let wp = run.work_packages.iter_mut()
           .find(|wp| wp.id == wp_id)
           .ok_or_else(|| anyhow!("WP not found: {}", wp_id))?;
       if relaunch {
           wp.state.transition(WPState::Active, &wp.id)?;
           // Trigger re-execution (similar to restart)
       } else {
           wp.state.transition(WPState::Pending, &wp.id)?;
           // Hold — operator must manually restart later
       }
       Ok(())
   }
   ```

3. Add the new arms to `handle_action()` match in the engine event loop:
   ```rust
   EngineAction::Approve(wp_id) => {
       self.handle_approve(&wp_id).await?;
       // After approval, check if dependents can now launch
   }
   EngineAction::Reject { wp_id, relaunch } => {
       self.handle_reject(&wp_id, relaunch).await?;
   }
   ```

**Note**: Approve/Reject are TUI-only actions (human-in-the-loop review gate). Do NOT add FIFO command parsing for these — the review gate must require explicit operator interaction.

**Files**:
- `crates/kasmos/src/command_handlers.rs` (~15 lines — EngineAction enum only)
- `crates/kasmos/src/engine.rs` (~40 lines — handlers)

### Subtask T009 – Update completion detector for `for_review` lane distinction

**Purpose**: When the completion detector reads a task file and sees `lane: for_review`, it should emit a distinct event so the engine transitions to ForReview (not Completed).

**Steps**:
1. In `crates/kasmos/src/detector.rs`, update `CompletionEvent`:
   ```rust
   pub struct CompletionEvent {
       pub wp_id: String,
       pub method: CompletionMethod,
       pub success: bool,
       pub timestamp: SystemTime,
       pub for_review: bool,  // NEW — true when lane is "for_review"
   }
   ```

2. In the frontmatter parsing logic (`parse_frontmatter` or lane detection), when `lane == "for_review"`:
   - Emit `CompletionEvent { success: true, for_review: true, ... }`

3. When `lane == "done"`:
   - Emit `CompletionEvent { success: true, for_review: false, ... }`

4. In `engine.rs` `handle_completion()`, check the `for_review` flag:
   ```rust
   if event.for_review {
       wp.state.transition(WPState::ForReview, &wp.id)?;
   } else if event.success {
       wp.state.transition(WPState::Completed, &wp.id)?;
   } else {
       wp.state.transition(WPState::Failed, &wp.id)?;
   }
   ```

**Files**:
- `crates/kasmos/src/detector.rs` (~10 lines)
- `crates/kasmos/src/engine.rs` (~10 lines)

**Notes**: The existing detector already parses frontmatter and checks for lane values. The change is adding the `for_review` field to `CompletionEvent` and setting it based on lane content.

### Subtask T010 – Wire watch channel creation in `launch.rs`

**Purpose**: Create the `watch::channel` in the launch pipeline and pass the sender to the engine.

**Steps**:
1. In `crates/kasmos/src/launch.rs`, after creating `OrchestrationRun` (step 10) and before creating WaveEngine (step 17):
   ```rust
   // Step 16.5: Create watch channel for TUI state updates
   let (watch_tx, watch_rx) = tokio::sync::watch::channel(run.read().await.clone());
   ```

2. Pass `watch_tx` to `WaveEngine::new()`:
   ```rust
   let engine = WaveEngine::new(
       run.clone(),
       graph,
       merged_completion_rx,
       action_rx,
       watch_tx,  // NEW
   );
   ```

3. Clone `action_tx` for the TUI:
   ```rust
   let tui_action_tx = action_tx.clone();
   ```

**Files**: `crates/kasmos/src/launch.rs` (~10 lines)

**Notes**: The watch channel must be created AFTER the initial OrchestrationRun is built so the initial value is populated. The `watch_rx` will be passed to the TUI in T011.

### Subtask T011 – Spawn TUI task in `launch.rs`, re-export `tui` module

**Purpose**: Start the TUI as a tokio task after all other components are wired, and make the tui module accessible from the library crate.

**Steps**:
1. In `crates/kasmos/src/launch.rs`, after the engine is spawned (step 17):
   ```rust
   // Step 17.5: Spawn TUI
   let tui_handle = tokio::spawn(async move {
       if let Err(e) = kasmos::tui::run(watch_rx, tui_action_tx).await {
           tracing::error!("TUI error: {}", e);
       }
   });
   ```

2. In `crates/kasmos/src/lib.rs`, add the module declaration and re-export:
   ```rust
   pub mod tui;
   ```

3. Ensure the TUI task is awaited or aborted during shutdown:
   ```rust
   // In cleanup phase, after engine completes:
   tui_handle.abort();
   ```

**Files**:
- `crates/kasmos/src/launch.rs` (~15 lines)
- `crates/kasmos/src/lib.rs` (~2 lines)

**Notes**: The TUI should not block the engine. If the TUI exits (user presses `q`), the engine continues running. If the engine finishes, the TUI should detect the terminal state (Completed/Failed/Aborted) via watch channel and show a final summary.

### Subtask T056 – Emit review-ready events on ForReview transition

**Purpose**: Ensure review automation can start immediately when a WP hits `for_review`.

**Steps**:
1. Add a review-ready signal path from engine completion handling:
   - On transition `Active -> ForReview`, publish event with `wp_id`, `worktree_path`, `pane_id`.
2. Add a review runner queue receiver (or callback) in startup wiring.
3. Ensure events are idempotent (no duplicate enqueue for same WP state/version).
4. Persist review automation pending/running markers so restarts can resume safely.

**Files**:
- `crates/kasmos/src/engine.rs`
- `crates/kasmos/src/start.rs` or launch wiring module
- persistence/state model files for review queue metadata

## Risks & Mitigations

- **Clone on OrchestrationRun**: May need to derive Clone on several nested types. Do this incrementally and verify compilation after each addition.
- **Test breakage**: Adding ForReview to WPState will require updating exhaustive match statements. Run `cargo test` frequently during implementation.
- **CompletionEvent field addition**: The `for_review: bool` field changes the struct. Update all construction sites — search for `CompletionEvent {` in the codebase.

## Review Guidance

- Verify all existing 198 tests pass
- Verify new state machine tests cover ForReview transitions
- Verify watch channel: engine mutations produce updates visible to a watch::Receiver
- Verify ForReview WPs don't count as "complete" in `is_complete()` check
- Check that `Approve` followed by dependent WP launch works end-to-end in the engine

## Activity Log

- 2026-02-10T22:00:00Z – system – lane=planned – Prompt created.
- 2026-02-11T04:04:58Z – openai-gpt5-codex – shell_pid=1680308 – lane=doing – Started review via workflow command
- 2026-02-11T04:11:01Z – openai-gpt5-codex – shell_pid=1680308 – lane=planned – Moved to planned
- 2026-02-11T05:22:18Z – coder – shell_pid=1444681 – lane=doing – Started implementation via workflow command
- 2026-02-11T06:04:29Z – coder – shell_pid=1444681 – lane=for_review – Ready for review
- 2026-02-11T07:14:29Z – coder – shell_pid=1444681 – lane=doing – Started review via workflow command
- 2026-02-11T08:07:29Z – coder – shell_pid=1444681 – lane=for_review – Implementation complete: All 7 subtasks done (T006-T011, T056). Review-ready events now emitted on ForReview transitions. 218 tests passing.
- 2026-02-11T08:34:08Z – coder – shell_pid=1444681 – lane=planned – Tiered review BLOCKED via krdone
- 2026-02-11T08:39:49Z – coder – shell_pid=1415369 – lane=doing – Started implementation via workflow command
- 2026-02-11T08:47:01Z – coder – shell_pid=1415369 – lane=for_review – Ready for review: All review feedback addressed - fixed CLI alias conflict, re-added state persistence to engine, removed launcher-process TUI spawn, fixed terminal cleanup, removed dead code. All 218 tests passing.
- 2026-02-11T08:55:47Z – coder – shell_pid=1415369 – lane=planned – Tiered review NEEDS_CHANGES via krdone
- 2026-02-11T08:56:06Z – coder – shell_pid=1415369 – lane=doing – Started implementation via workflow command
- 2026-02-11T09:03:06Z – coder – shell_pid=1415369 – lane=for_review – Review feedback addressed: Added FIFO approve/reject commands, fixed launch error propagation, improved error handling for completion/review channels, added Serialize/Deserialize to ReviewReadyEvent. All 228 tests passing.
- 2026-02-11T09:10:15Z – coder – shell_pid=1415369 – lane=doing – Moved to doing
- 2026-02-11T09:16:43Z – coder – shell_pid=1415369 – lane=for_review – Review feedback addressed: Fixed launch channel rollback (Critical), enabled TUI integration (High), added CLI approve/reject commands (Medium), improved review-ready event error handling (Medium), added regression test (Low). All 229 tests passing.
- 2026-02-11T09:24:43Z – coder – shell_pid=1415369 – lane=doing – Moved to doing
- 2026-02-11T09:34:37Z – coder – shell_pid=1415369 – lane=for_review – Review feedback addressed: Removed TUI from launcher process (High), added idempotent review-ready event emission (Medium), set run.completed_at on finalization (Medium), fixed all clippy warnings (Medium), added Approve/Reject FIFO test coverage (Low). All 233 tests passing, clippy clean.
- 2026-02-11T09:38:27Z – coder – shell_pid=1415369 – lane=doing – Moved to doing
- 2026-02-11T09:46:46Z – coder – shell_pid=1415369 – lane=planned – Tiered review BLOCKED via krdone
- 2026-02-11T09:46:49Z – coder – shell_pid=1415369 – lane=planned – Tiered review BLOCKED via krdone
- 2026-02-11T09:46:53Z – coder – shell_pid=1415369 – lane=planned – Tiered review BLOCKED via krdone
- 2026-02-11T09:46:59Z – coder – shell_pid=1415369 – lane=planned – Tiered review BLOCKED via krdone
- 2026-02-11T09:48:44Z – reviewer – shell_pid=1415369 – lane=done – Review VERIFIED
