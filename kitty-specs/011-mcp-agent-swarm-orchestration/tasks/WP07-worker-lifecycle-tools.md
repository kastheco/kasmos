---
work_package_id: "WP07"
subtasks:
  - "T039"
  - "T040"
  - "T041"
  - "T042"
  - "T043"
  - "T044"
title: "Worker Lifecycle MCP Tools"
phase: "Phase 2 - Safety, State, and Audit Guarantees"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP03", "WP05", "WP06"]
history:
  - timestamp: "2026-02-14T16:27:48Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP07 - Worker Lifecycle MCP Tools

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP07 --base WP03
```

---

## Objectives & Success Criteria

Implement the worker pane lifecycle tools (`spawn_worker`, `despawn_worker`, `list_workers`) with registry consistency and role-aware behavior. After this WP:

1. Manager can spawn worker panes via `spawn_worker` MCP tool
2. Coder workers automatically get worktree provisioning
3. Manager can despawn workers via `despawn_worker` with registry + pane cleanup
4. `list_workers` returns accurate state with live-pane reconciliation
5. Max parallel worker limit is enforced with clear backpressure response
6. Registry stays consistent even when panes are manually closed by users

## Context & Constraints

- **Depends on WP03**: Launch layout and Zellij session wrappers available
- **Depends on WP05**: Feature locking ensures single owner
- **Depends on WP06**: Audit writer available for event logging
- **Contract**: `spawn_worker`, `despawn_worker`, `list_workers` in `contracts/kasmos-serve.json`
- **Data model**: `WorkerEntry` entity with role, status, pane tracking
- **Existing code**: `crates/kasmos/src/session.rs` (718 lines) has `SessionManager` with pane tracking via HashMap. `crates/kasmos/src/git.rs` (467 lines) has `WorktreeManager` for worktree creation/cleanup.
- **Zellij**: Worker panes are created with `zellij run -n <name> -- <command>`. No `list-panes` command exists - tracking is internal.

## Subtasks & Detailed Guidance

### Subtask T039 - Implement spawn_worker tool

**Purpose**: Create a new worker pane with role-specific setup and registry tracking.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/spawn_worker.rs`:
   ```rust
   pub async fn handle(
       input: SpawnWorkerInput,
       state: &KasmosServer,
   ) -> Result<SpawnWorkerOutput> {
       // 1. Validate input
       validate_wp_id(&input.wp_id)?;
       validate_role(&input.role)?;
       // 2. Check worker limit
       check_capacity(state).await?;
       // 3. Generate pane name
       let pane_name = format!("{}-{}", input.wp_id, input.role);
       // 4. Provision worktree (coders only)
       let worktree_path = if input.role == "coder" {
           Some(provision_worktree(&input).await?)
       } else { None };
       // 5. Build agent command
       let cmd = build_agent_command(&input, &worktree_path)?;
       // 6. Create Zellij pane
       create_worker_pane(&pane_name, &cmd).await?;
       // 7. Register in worker registry
       let entry = register_worker(state, &input, &pane_name, &worktree_path).await;
       // 8. Audit log
       audit_spawn(state, &entry).await;
       Ok(SpawnWorkerOutput { ok: true, worker: entry.into() })
   }
   ```
2. Pane naming: `<wp_id>-<role>` (e.g., `WP01-coder`, `WP03-reviewer`)
3. The Zellij command to create a worker pane:
   ```bash
   zellij run -n <pane_name> -- ocx oc -p kas -- --agent <role> --prompt "<prompt>"
   ```
   Reference the existing pattern in `crates/kasmos/src/session.rs` `spawn_agent()` method.
4. The `prompt` field from input is passed to the agent command.
5. Worker starts in `Active` status.

**Files**: `crates/kasmos/src/serve/tools/spawn_worker.rs`
**Validation**: Calling spawn_worker creates a Zellij pane with the correct command.

### Subtask T040 - Implement coder-only worktree provisioning

**Purpose**: Coders work in isolated git worktrees. Other roles work in the main repo.

**Steps**:
1. When `role == "coder"`:
   - Create worktree at `.worktrees/<feature_slug>-<wp_id>/`
   - Use the existing `WorktreeManager` in `crates/kasmos/src/git.rs`:
     ```rust
     let wt_manager = WorktreeManager::new(repo_root);
     let worktree_path = wt_manager.create_worktree(
         &feature_slug, &wp_id, &base_branch
     ).await?;
     ```
   - The worktree_path is passed to the agent command as `--cwd`
2. When `role != "coder"`:
   - No worktree. Agent runs in the main repo directory.
   - `worktree_path` in the registry entry is `None`.
3. Reference the existing `WorktreeManager` for create/cleanup patterns (including `.kittify/memory/` symlink handling).
4. Handle the case where a worktree already exists (idempotent creation or error).

**Parallel?**: Can run alongside T041 once spawn contract is stable.
**Files**: `crates/kasmos/src/serve/tools/spawn_worker.rs`, referencing `crates/kasmos/src/git.rs`
**Validation**: Coder spawn creates worktree. Reviewer spawn does not.

### Subtask T041 - Implement despawn_worker tool

**Purpose**: Close a worker pane, update registry, and emit audit event.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/despawn_worker.rs`:
   ```rust
   pub async fn handle(
       input: DespawnWorkerInput,
       state: &KasmosServer,
   ) -> Result<DespawnWorkerOutput> {
       // 1. Find worker in registry
       let worker = find_worker(state, &input.wp_id, &input.role).await?;
       // 2. Close Zellij pane
       close_pane(&worker.pane_name).await;
       // 3. Cleanup worktree (if coder)
       if let Some(ref wt_path) = worker.worktree_path {
           cleanup_worktree(wt_path).await?;
       }
       // 4. Remove from registry
       remove_worker(state, &input.wp_id, &input.role).await;
       // 5. Audit log
       audit_despawn(state, &worker, input.reason.as_deref()).await;
       Ok(DespawnWorkerOutput { ok: true, removed: true })
   }
   ```
2. Closing a Zellij pane: Use `zellij action close-pane` after focusing the target pane, or close by name if Zellij supports it.
3. If the pane is already gone (user manually closed it), the despawn should still succeed and clean up the registry.
4. Worktree cleanup for coders: The `WorktreeManager` in `git.rs` has cleanup methods. Only clean up if the worktree exists.

**Files**: `crates/kasmos/src/serve/tools/despawn_worker.rs`
**Validation**: Despawn closes pane, removes from registry, cleans up worktree.

### Subtask T042 - Implement list_workers with live-pane reconciliation

**Purpose**: Return the current worker list with detection of panes that disappeared (user closed, crashed).

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/list_workers.rs`:
   ```rust
   pub async fn handle(
       input: ListWorkersInput,
       state: &KasmosServer,
   ) -> Result<ListWorkersOutput> {
       let mut workers = get_workers(state).await;
       // Reconcile: check which panes still exist
       reconcile_panes(&mut workers).await;
       // Filter by status if requested
       if let Some(status_filter) = input.status {
           workers.retain(|w| w.status == status_filter);
       }
       Ok(ListWorkersOutput { ok: true, workers })
   }
   ```
2. Reconciliation: For each worker with status `Active`, check if its Zellij pane still exists. Since Zellij has no `list-panes` command, use the zellij-pane-tracker MCP tool (`get_panes`) to check pane existence, or use `zellij action dump-screen` with the pane name as a proxy.
3. If a pane no longer exists, mark the worker as `Aborted` in the registry.
4. This reconciliation happens on every `list_workers` call to keep state fresh.

**Files**: `crates/kasmos/src/serve/tools/list_workers.rs`
**Validation**: Workers with dead panes are marked as Aborted. Status filter works.

### Subtask T043 - Implement max parallel worker enforcement

**Purpose**: Prevent spawning more workers than the configured maximum.

**Steps**:
1. In the `spawn_worker` handler, before creating the pane:
   ```rust
   async fn check_capacity(state: &KasmosServer) -> Result<()> {
       let registry = state.registry.read().await;
       let active_count = registry.workers.values()
           .filter(|w| w.status == WorkerStatus::Active)
           .count();
       if active_count >= state.config.agent.max_parallel_workers {
           return Err(KasmosError::CapacityExceeded {
               current: active_count,
               max: state.config.agent.max_parallel_workers,
           });
       }
       Ok(())
   }
   ```
2. Return a clear error with current count and max, so the manager can make informed decisions (e.g., wait for a worker to finish before spawning a new one).
3. The limit is from `config.agent.max_parallel_workers` (default: 4).

**Files**: `crates/kasmos/src/serve/tools/spawn_worker.rs`
**Validation**: Spawning beyond limit returns capacity error with counts.

### Subtask T044 - Add worker lifecycle tests

**Purpose**: Test spawn/despawn/list behavior and registry edge cases.

**Steps**:
1. Test spawn creates registry entry with correct fields
2. Test despawn removes entry and returns removed=true
3. Test despawn of non-existent worker returns WORKER_NOT_FOUND
4. Test list returns all workers with correct statuses
5. Test capacity enforcement at limit
6. Test reconciliation marks missing panes as Aborted
7. Test coder spawn includes worktree_path, reviewer spawn does not
8. Use mocked Zellij commands for test isolation

**Files**: Test modules in serve/tools files
**Validation**: `cargo test` passes with worker lifecycle tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Pane lifecycle race with manual user pane closure | Reconcile on each list_workers call. Despawn handles already-gone panes. |
| Worktree leaks for aborted coders | Cleanup hooks in despawn. Document manual recovery: `git worktree prune`. |
| Zellij pane naming collisions | Deterministic naming: `<wp_id>-<role>`. Validate uniqueness before spawn. |

## Review Guidance

- Verify pane naming follows `<wp_id>-<role>` convention
- Verify coder gets worktree, other roles do not
- Verify reconciliation detects dead panes
- Verify capacity enforcement returns clear error
- Verify despawn handles already-closed panes gracefully
- Verify audit events emitted for spawn and despawn

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
