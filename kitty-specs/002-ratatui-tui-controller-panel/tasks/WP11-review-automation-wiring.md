---
work_package_id: WP11
title: "Review Automation Wiring — Engine → Reviewer Pane"
lane: "done"
dependencies:
  - WP02
  - WP06
base_branch: master
created_at: '2026-02-12T02:15:00+00:00'
subtasks:
  - T057
  - T058
  - T059
  - T060
  - T061
  - T062
phase: "Phase 4 - Integration"
assignee: 'unassigned'
agent: "reviewer"
shell_pid: "775722"
review_status: 'approved'
reviewed_by: 'kas'
history:
  - timestamp: '2026-02-12T02:15:00Z'
    lane: planned
    agent: system
    shell_pid: ''
    action: "Manual WP created — review automation gap identified during 006 orchestration"
  - timestamp: '2026-02-12T10:48:00Z'
    lane: done
    agent: kas
    shell_pid: ''
    action: "WP11 implemented: ReviewCoordinator, DetectedLane, approve/reject FIFO commands, engine wiring. All 227 tests pass."
---

# Work Package Prompt: WP11 — Review Automation Wiring

## Problem Statement

When a WP transitions to `ForReview`, the engine logs `"WP moved to review"` (`engine.rs:255`) and the TUI's `update_state` logs the `ReviewPolicyDecision` (`app.rs:326-338`), but **nothing actually launches a reviewer**. The review policy infrastructure (policy types, executor, decision structs) exists in `review.rs`, and the `/kas:verify` + `/kas:review` slash commands exist in `.opencode/command/`, but there is no execution bridge connecting the `ForReview` state transition to spawning a reviewer agent.

## Objectives & Success Criteria

- When a WP transitions to `ForReview`, the engine (or a new review coordinator task) spawns a reviewer OpenCode session in a Zellij pane running `/kas:verify WPXX`
- The reviewer pane is named `review-WPXX` and runs in the WP's worktree directory
- Review automation respects `ReviewAutomationPolicy`: `ManualOnly` does nothing, `AutoThenManualApprove` spawns reviewer but requires operator approval, `AutoAndMarkDone` spawns reviewer and auto-completes on success
- Review results are captured (exit code at minimum) and fed back to the engine
- If the reviewer pane fails to launch, a `ReviewFailureType::CommandMissing` notification is emitted
- All existing tests pass, new behavior has test coverage
- `cargo build` succeeds with no warnings in new code

## Context & Constraints

- **Engine event loop**: `engine.rs:167-215` — `tokio::select!` on `completion_rx` + `action_rx`. ForReview transition happens in `handle_completion` at line 242-255.
- **EngineAction enum**: `command_handlers.rs:16-31` — currently has Restart/Pause/Resume/ForceAdvance/Retry/Advance/Abort. No Review/Approve/Reject variants.
- **Review policy**: `review.rs` — `ReviewPolicyExecutor::on_for_review_transition()` returns `ReviewPolicyDecision { run_automation, auto_mark_done }`.
- **TUI review hooks**: `tui/app.rs:326-338` — logs policy decision but takes no action.
- **Zellij CLI**: `zellij.rs:282-306` — `run_in_pane(session, name, command, args)` spawns `zellij --session <s> run -n <name> -- <command> <args>`. Note: `validate_identifier` rejects shell metacharacters, so the reviewer command must be a clean binary + args, not a shell string.
- **Slash command**: `.opencode/command/kas.verify.md` — `/kas:verify WPXX` runs tiered review. Invoked inside an OpenCode session via the prompt.
- **Agent pane pattern**: `layout.rs:400-406` — existing coder panes use `bash -c "ocx oc -- --agent coder --prompt ..."`. Reviewer panes should follow the same pattern but with `--agent reviewer`.
- **Session name**: `start.rs:302` — `kasmos-{feature_name}`.

## Subtasks & Detailed Guidance

### T057 — Add `EngineAction::Approve`, `EngineAction::Reject`, and review event channel

**Purpose**: The engine needs actions to approve/reject WPs from review, and a channel to emit "review needed" events that a coordinator can act on.

**Steps**:
1. Add to `EngineAction` in `command_handlers.rs`:
   ```rust
   /// Approve a reviewed work package (move to Completed).
   Approve(String),
   /// Reject a reviewed work package (move back to Active for rework).
   Reject { wp_id: String, relaunch: bool },
   ```

2. Add to `ControllerCommand` in `commands.rs`:
   ```rust
   Approve { wp_id: String },
   Reject { wp_id: String },
   ```

3. Wire command handler in `command_handlers.rs` for Approve/Reject → EngineAction dispatch.

4. Handle in `engine.rs::handle_action`:
   - `Approve(wp_id)`: Transition `ForReview → Completed`, set `completed_at`.
   - `Reject { wp_id, relaunch: true }`: Transition `ForReview → Active`, trigger relaunch.
   - `Reject { wp_id, relaunch: false }`: Transition `ForReview → Pending` (hold).

5. Add a new `review_tx: mpsc::Sender<ReviewRequest>` to `WaveEngine` (or emit via `launch_tx`):
   ```rust
   pub struct ReviewRequest {
       pub wp_id: String,
       pub worktree_path: Option<PathBuf>,
       pub feature_dir: PathBuf,
   }
   ```

6. In `engine.rs::handle_completion`, after transitioning to `ForReview`, send a `ReviewRequest` on the channel.

**Files**: `command_handlers.rs`, `commands.rs`, `engine.rs`, `types.rs` (add `ReviewRequest`)

### T058 — Add `ReviewCoordinator` task in `start.rs`

**Purpose**: A tokio task that receives `ReviewRequest` events and spawns reviewer Zellij panes.

**Steps**:
1. Create `crates/kasmos/src/review_coordinator.rs`:
   ```rust
   pub struct ReviewCoordinator {
       session_name: String,
       opencode_binary: String,
       cli: Arc<dyn ZellijCli>,
       review_rx: mpsc::Receiver<ReviewRequest>,
       /// Sends completion events back when reviewer finishes.
       completion_tx: mpsc::Sender<CompletionEvent>,
       policy: ReviewAutomationPolicy,
   }
   ```

2. Implement `ReviewCoordinator::run()`:
   - Receive `ReviewRequest` from channel
   - Check policy: if `ManualOnly`, log and skip
   - Build reviewer command: `bash -c "ocx oc -- --agent reviewer --prompt '/kas:verify WPXX'"`
   - Because `run_in_pane` rejects shell metacharacters, use a wrapper script approach instead:
     - Write a one-shot script to `.kasmos/review-WPXX.sh` with the `ocx oc -- --agent reviewer --prompt "/kas:verify WPXX"` command
     - Call `run_in_pane(session, "review-WPXX", "bash", &[script_path])`
   - Or: extend `ZellijCli` with a `run_shell_in_pane` method that accepts a raw shell command (more general, less hacky)

3. Add the coordinator to `start.rs` Phase 3, wired to the review channel from the engine.

**Files**: `review_coordinator.rs` (new), `start.rs`, `lib.rs` (re-export)

**Notes**: The reviewer pane runs in the WP's worktree directory so `git diff` in `/kas:verify` sees the right changes. The pane name `review-WPXX` avoids collision with agent panes (`wpXX-pane`).

### T059 — Wire `ReviewCoordinator` into `start.rs` orchestration pipeline

**Purpose**: Integrate the coordinator into the existing Phase 3 channel topology.

**Steps**:
1. In `start.rs` Phase 3, after creating the engine channels:
   ```rust
   let (review_tx, review_rx) = mpsc::channel::<kasmos::ReviewRequest>(16);
   ```
2. Pass `review_tx` to `WaveEngine::new()` (extend its constructor).
3. Create `ReviewCoordinator` and spawn it:
   ```rust
   let coordinator = kasmos::ReviewCoordinator::new(
       session_name.clone(),
       config.opencode_binary.clone(),
       Arc::new(kasmos::RealZellijCli::new(config.zellij_binary.clone())),
       review_rx,
       completion_tx.clone(), // reuse existing completion channel for review results
       kasmos::ReviewAutomationPolicy::default(),
   );
   let _review_handle = tokio::spawn(async move {
       coordinator.run().await;
   });
   ```

**Files**: `start.rs`, `engine.rs` (constructor update)

### T060 — Review result capture and feedback

**Purpose**: When a reviewer pane exits, detect the result and feed it back to the engine.

**Steps**:
1. The reviewer pane runs as a Zellij pane process. When it exits, we need to detect completion.
   - **Option A (recommended)**: The reviewer writes a structured result file to `.kasmos/review-results/WPXX.json` with `{ "decision": "VERIFIED|NEEDS_CHANGES|BLOCKED", ... }`. The `ReviewCoordinator` watches for this file (reuse `notify` crate already in the project for the completion detector).
   - **Option B**: The coordinator monitors pane exit via periodic polling (less elegant).

2. On result detection:
   - `VERIFIED` + `auto_mark_done` policy → send `EngineAction::Approve(wp_id)` or `CompletionEvent` with `DetectedLane::Done`
   - `VERIFIED` + `AutoThenManualApprove` → log to notifications, wait for operator Approve action
   - `NEEDS_CHANGES` / `BLOCKED` → emit notification, WP stays in `ForReview`

3. Parse the structured output from `/kas:review`'s strict format (see `.opencode/command/kas.review.md` lines 95-115).

**Files**: `review_coordinator.rs`, potentially `detector.rs` (reuse file watching pattern)

**Notes**: This subtask can be simplified in v1 — just detect pane exit and require manual Approve/Reject from the operator. Automatic result parsing can be a follow-up.

### T061 — FIFO commands for Approve/Reject

**Purpose**: Allow the operator to approve/reject from the FIFO (controller pane), not just the TUI.

**Steps**:
1. Add `approve <wp_id>` and `reject <wp_id>` to the FIFO command parser in `commands.rs`.
2. Wire through `CommandHandler` → `EngineAction::Approve` / `EngineAction::Reject`.
3. Add `kasmos cmd approve WP01` and `kasmos cmd reject WP01` subcommands to `cmd.rs` / `main.rs`.

**Files**: `commands.rs`, `command_handlers.rs`, `cmd.rs`, `main.rs`

### T062 — Tests for review automation flow

**Purpose**: Verify the end-to-end review automation pipeline.

**Steps**:
1. Unit test: `ReviewCoordinator` receives a `ReviewRequest` and spawns the correct command via a mock `ZellijCli`.
2. Unit test: Engine emits `ReviewRequest` when WP transitions to `ForReview`.
3. Unit test: `Approve` action transitions `ForReview → Completed`.
4. Unit test: `Reject { relaunch: true }` transitions `ForReview → Active`.
5. Unit test: `Reject { relaunch: false }` transitions `ForReview → Pending`.
6. Unit test: `ManualOnly` policy skips automation.
7. Integration test: FIFO `approve WP01` / `reject WP01` commands parse and dispatch correctly.

**Files**: `engine.rs` (tests), `review_coordinator.rs` (tests), `commands.rs` (tests), `command_handlers.rs` (tests)

## Risks & Mitigations

- **Shell metacharacter rejection in `run_in_pane`**: The `ocx oc -- --agent reviewer --prompt "/kas:verify WPXX"` command contains quotes/spaces. Mitigation: use a wrapper script or extend `ZellijCli` to support shell commands.
- **Reviewer pane exit detection**: Zellij doesn't notify when panes exit. Mitigation: use file-based signaling (reviewer writes result file) or periodic polling.
- **Race condition on approval**: Operator may approve via FIFO while auto-approve is in flight. Mitigation: use `ForReview → ForReview` self-transition as idempotent guard.

## Review Guidance

- Verify `cargo build -p kasmos` compiles with zero warnings
- Verify `cargo test -p kasmos` — all existing tests pass + new tests
- Verify reviewer pane spawns correctly when a WP transitions to `ForReview` (manual test with a dummy WP)
- Verify FIFO approve/reject commands work from CLI

## Activity Log

- 2026-02-12T10:14:14Z – coder – lane=doing – Implementation complete
- 2026-02-12T10:15:48Z – coder – lane=for_review – Submitted for review via swarm
- 2026-02-12T10:15:52Z – reviewer – shell_pid=775722 – lane=doing – Started review via workflow command
