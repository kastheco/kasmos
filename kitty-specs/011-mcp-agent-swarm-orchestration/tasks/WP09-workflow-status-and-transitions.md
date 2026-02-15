---
work_package_id: WP09
title: Workflow Status and Transition Controls
lane: "doing"
dependencies: [WP04]
base_branch: 011-mcp-agent-swarm-orchestration-WP04
base_commit: a02df49238a89b34cf57dc156237af2bad587046
created_at: '2026-02-15T01:01:51.630409+00:00'
subtasks:
- T051
- T052
- T053
- T054
- T055
- T056
phase: Phase 2 - Safety, State, and Audit Guarantees
assignee: ''
agent: "reviewer"
shell_pid: "543338"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP09 - Workflow Status and Transition Controls

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP09 --base WP04
```

---

## Objectives & Success Criteria

Implement `workflow_status` and `transition_wp` MCP tools against spec-kitty task lanes as the single source of truth, with wave awareness and review loop caps. After this WP:

1. `workflow_status` reports correct phase using expanded model (spec_only, clarifying, planned, analyzing, tasked, implementing, reviewing, releasing, complete) where clarifying/analyzing are optional
2. `workflow_status` includes computed wave structure from WP dependency metadata
3. `transition_wp` validates state machine rules and persists via lane translation
4. Advisory lock protection prevents concurrent task-file corruption
5. Review-rejection loop cap (default 3) is enforced with escalation to user
6. Lane translation follows the protocol: kasmos states -> spec-kitty lanes (see data-model.md)

## Context & Constraints

- **Depends on WP04**: Serve framework and typed tool structs available
- **Contract**: `workflow_status` and `transition_wp` in `contracts/kasmos-serve.json`
- **Data model**: Lane Translation Protocol in `data-model.md`:
  - `pending` <-> `planned` (bidirectional)
  - `active` <-> `doing` (bidirectional)
  - `for_review` <-> `for_review` (shared)
  - `done` <-> `done` (shared)
  - `rework` -> `doing` (write-only; rework context in audit log reason field)
- **Existing code**: `crates/kasmos/src/parser.rs` (433 lines) has `WPFrontmatter`, `parse_frontmatter()`, `wp_state_to_lane()`. `crates/kasmos/src/graph.rs` (376 lines) has `DependencyGraph` with topological sort and wave computation. `crates/kasmos/src/state_machine.rs` (341 lines) has WPState/RunState transition validation.
- **Key constraint**: Task file lanes are SSOT. No parallel shadow state. Kasmos reads from and writes to spec-kitty lanes.

## Subtasks & Detailed Guidance

### Subtask T051 - Implement workflow_status artifact scan

**Purpose**: Scan feature artifacts to determine the current workflow phase and report a complete status snapshot.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/workflow_status.rs`:
   ```rust
   pub async fn handle(
       input: WorkflowStatusInput,
       state: &KasmosServer,
   ) -> Result<WorkflowStatusOutput> {
       let feature_dir = resolve_feature_dir(&input.feature_slug)?;
       let phase = determine_phase(&feature_dir)?;
       let waves = compute_waves(&feature_dir)?;
       let lock = get_lock_state(&input.feature_slug)?;
       Ok(WorkflowStatusOutput {
           ok: true,
           snapshot: WorkflowSnapshot {
               feature_slug: input.feature_slug,
               phase,
               waves,
               active_workers: get_active_workers(state).await,
               last_event_at: get_last_event(state).await,
               lock,
           },
       })
   }
   ```
2. Phase determination logic (per expanded WorkflowSnapshot model in `data-model.md`):
   - `spec_only`: spec.md exists, no plan.md, no clarification artifacts
   - `clarifying`: spec.md exists, clarification session in progress (optional phase - smaller features skip this)
   - `planned`: plan.md exists, no tasks.md or empty tasks/
   - `analyzing`: plan.md and tasks.md exist, analysis in progress (optional phase - smaller features skip this)
   - `tasked`: tasks/ has WP files, all in `planned` lane
   - `implementing`: any WP in `doing` lane
   - `reviewing`: any WP in `for_review` lane (and none in `doing`)
   - `releasing`: all WPs in `done` lane, release process initiated but not yet complete
   - `complete`: all WPs in `done` lane and release completed
   Note: `clarifying` and `analyzing` are optional. Phase derivation is from artifact presence, not WP lane states.
3. Read task file frontmatter using the existing `parser::parse_frontmatter()` function.
4. Include lock metadata from WP05's lock manager.

**Files**: `crates/kasmos/src/serve/tools/workflow_status.rs`
**Validation**: Correct phase reported for each artifact combination.

### Subtask T052 - Integrate dependency graph wave computation

**Purpose**: Compute wave assignments from WP dependency metadata to enable wave-ordered execution.

**Steps**:
1. Reuse the existing `DependencyGraph` in `crates/kasmos/src/graph.rs`:
   ```rust
   fn compute_waves(feature_dir: &Path) -> Result<Vec<WaveStatus>> {
       let feature = FeatureDir::scan(feature_dir)?;
       let mut graph = DependencyGraph::new();
       for wp_file in &feature.wp_files {
           let fm = parse_frontmatter(wp_file)?;
           graph.add_node(&fm.work_package_id, &fm.dependencies);
       }
       let waves = graph.compute_waves()?;
       waves.iter().enumerate().map(|(i, wp_ids)| {
           let complete = wp_ids.iter().all(|id| is_done(feature_dir, id));
           WaveStatus { wave: i as u32, wp_ids: wp_ids.clone(), complete }
       }).collect()
   }
   ```
2. The existing `DependencyGraph` already does topological sort and wave computation. Leverage it.
3. Map wave results to the contract's `WorkflowSnapshot.waves` format.
4. Each wave reports whether all its WPs are complete.

**Parallel?**: Yes - can run alongside T054 once parser contract for task metadata is finalized.
**Files**: `crates/kasmos/src/serve/tools/workflow_status.rs`, using `crates/kasmos/src/graph.rs`
**Validation**: Wave computation matches expected dependency ordering.

### Subtask T053 - Implement transition_wp with validation and lane translation

**Purpose**: Validate and apply WP state transitions, translating between kasmos vocabulary and spec-kitty lanes.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/transition_wp.rs`:
   ```rust
   pub async fn handle(
       input: TransitionWpInput,
       state: &KasmosServer,
   ) -> Result<TransitionWpOutput> {
       let feature_dir = resolve_feature_dir(&input.feature_slug)?;
       let wp_file = find_wp_file(&feature_dir, &input.wp_id)?;
       // 1. Read current state
       let fm = parse_frontmatter(&wp_file)?;
       let from_state = lane_to_kasmos_state(&fm.lane);
       // 2. Validate transition
       validate_transition(&from_state, &input.to_state)?;
       // 3. Check rejection loop cap
       if input.to_state == "rework" {
           check_rejection_cap(&input.wp_id, state).await?;
       }
       // 4. Translate to spec-kitty lane
       let new_lane = kasmos_state_to_lane(&input.to_state);
       // 5. Write to task file (with advisory lock)
       update_task_lane(&wp_file, &new_lane, &input.actor, input.reason.as_deref()).await?;
       // 6. Audit log
       audit_transition(state, &input, &from_state).await;
       Ok(TransitionWpOutput {
           ok: true,
           wp_id: input.wp_id,
           from_state: from_state.to_string(),
           to_state: input.to_state,
       })
   }
   ```
2. Lane translation (from data-model.md):
   ```rust
   fn kasmos_state_to_lane(state: &str) -> &str {
       match state {
           "pending" => "planned",
           "active" => "doing",
           "for_review" => "for_review",
           "done" => "done",
           "rework" => "doing",  // rework context in audit log, not lane
           _ => "planned",
       }
   }

   fn lane_to_kasmos_state(lane: &str) -> String {
       match lane {
           "planned" => "pending",
           "doing" => "active",  // may also be "rework" - check history
           "for_review" => "for_review",
           "done" => "done",
           _ => "pending",
       }
   }
   ```
3. The existing `parser::wp_state_to_lane()` handles some of this. Extend or replace it with the full kasmos vocabulary.
4. **Rework handling**: `rework` writes as `doing` lane but preserves rework semantics via the audit log `reason` field. On read-back, distinguish `active` from `rework` by checking transition history (prior `for_review` implies rework).
5. Transition validation: Use existing `state_machine.rs` patterns or extend them for the new kasmos states.
6. Return `TRANSITION_NOT_ALLOWED` error code for invalid transitions.

**Files**: `crates/kasmos/src/serve/tools/transition_wp.rs`
**Validation**: Valid transitions succeed. Invalid transitions return error. Lane translation is correct.

### Subtask T054 - Implement advisory lock protection for task-file writes

**Purpose**: Prevent concurrent corruption when multiple processes write to the same task file.

**Steps**:
1. Before writing to a task file, acquire an advisory lock:
   ```rust
   async fn update_task_lane(
       wp_file: &Path,
       new_lane: &str,
       actor: &str,
       reason: Option<&str>,
   ) -> Result<()> {
       let lock_path = wp_file.with_extension("lock");
       let lock_file = std::fs::File::create(&lock_path)?;
       // Advisory lock (non-blocking attempt)
       nix::fcntl::flock(
           lock_file.as_raw_fd(),
           nix::fcntl::FlockArg::LockExclusiveNonblock,
       ).map_err(|_| KasmosError::ConcurrentWrite {
           file: wp_file.display().to_string(),
       })?;
       // Perform the write
       write_lane_update(wp_file, new_lane, actor, reason)?;
       // Lock released when lock_file is dropped
       Ok(())
   }
   ```
2. Use nix's `flock` (already a dependency) for cross-process advisory locking.
3. Non-blocking: if lock can't be acquired, return an error rather than waiting.
4. Lock file: `<wp_file>.lock` (sibling to the task file).

**Parallel?**: Yes - can run alongside T052 once parser contract is finalized.
**Files**: `crates/kasmos/src/serve/tools/transition_wp.rs`
**Validation**: Concurrent writes to same file are prevented. Lock files are cleaned up.

### Subtask T055 - Enforce review-rejection loop cap

**Purpose**: Prevent infinite review-rejection-rework cycles by capping at a configurable maximum (FR-023).

**Steps**:
1. Track rejection count per WP:
   ```rust
   async fn check_rejection_cap(
       wp_id: &str,
       state: &KasmosServer,
   ) -> Result<()> {
       let count = get_rejection_count(wp_id, state).await;
       if count >= state.config.agent.review_rejection_cap {
           return Err(KasmosError::RejectionCapReached {
               wp_id: wp_id.to_string(),
               count,
               cap: state.config.agent.review_rejection_cap,
           });
       }
       Ok(())
   }
   ```
2. The rejection count can be tracked in the worker registry or derived from audit log entries.
3. When the cap is reached, return a specific error that tells the manager to pause and escalate to the user.
4. Default cap: 3 iterations (from `config.agent.review_rejection_cap`).

**Files**: `crates/kasmos/src/serve/tools/transition_wp.rs`
**Validation**: Third rejection triggers cap error. Manager is forced to pause.

### Subtask T056 - Add tests for workflow and transition tools

**Purpose**: Test phase derivation, transition guards, wave ordering, and concurrent writers.

**Steps**:
1. Test phase detection for each artifact combination
2. Test wave computation from dependency graph
3. Test valid transitions succeed (pending->active, active->for_review, etc.)
4. Test invalid transitions fail (e.g., pending->done)
5. Test lane translation roundtrip (kasmos -> spec-kitty -> kasmos)
6. Test rejection cap enforcement
7. Test advisory lock prevents concurrent writes
8. Use tempfile for test isolation with mock task files

**Files**: Test modules in workflow_status.rs and transition_wp.rs
**Validation**: `cargo test` passes with workflow/transition tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Malformed task files breaking status API | Robust parse errors with file-level context. Skip unparseable files with warning. |
| Race conditions under multi-process orchestration | Feature lock (WP05) prevents multiple managers. Advisory locks prevent concurrent writes. |
| rework/active ambiguity when reading `doing` lane | Use audit log history to distinguish. Document the heuristic clearly. |

## Review Guidance

- Verify lane translation matches data-model.md protocol exactly
- Verify rework writes as `doing` with reason in audit log
- Verify phase detection covers all artifact combinations
- Verify transition validation matches state machine rules
- Verify rejection cap is enforced and returns clear error
- Verify advisory locking uses nix flock correctly

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-15T01:18:03Z – unknown – shell_pid=212269 – lane=for_review – Ready for review
- 2026-02-15T01:19:16Z – reviewer – shell_pid=418409 – lane=doing – Started review via workflow command
- 2026-02-15T01:22:36Z – reviewer – shell_pid=418409 – lane=for_review – Moved to for_review
- 2026-02-15T01:22:39Z – reviewer – shell_pid=418409 – lane=done – Moved to done
- 2026-02-15T01:22:50Z – reviewer – shell_pid=418409 – lane=done – Review APPROVED — all subtasks verified, lane translation correct, advisory locking correct, 256 tests pass, clippy clean
- 2026-02-15T01:32:59Z – reviewer – shell_pid=418409 – lane=for_review – Moved to for_review
- 2026-02-15T01:33:02Z – reviewer – shell_pid=543338 – lane=doing – Started review via workflow command
