---
work_package_id: WP07
title: Kill + Restart Workers
lane: done
dependencies:
- WP04
subtasks:
- Kill worker handler (x key -> SIGTERM -> workerKilledMsg)
- Restart worker handler (r key -> spawn dialog pre-filled)
- Worker state transitions for killed/restarted
- Kill confirmation for running workers
phase: Wave 2 - Task Sources + Worker Management
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-17T00:00:00Z'
  lane: planned
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-18T14:12:28.785022125+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP07 coder - kill + restart workers)
- timestamp: '2026-02-18T14:23:51.661595646+00:00'
  lane: done
  actor: manager
  shell_pid: '472734'
  action: transition done (Implemented and reviewed - kill/restart workers)
---

# Work Package Prompt: WP07 - Kill + Restart Workers

## Mission

Implement kill and restart actions for workers: press `x` to terminate a running
worker, press `r` to restart a failed/killed worker with its original role and
prompt pre-filled in the spawn dialog for editing. This delivers User Story 4
(Kill and Restart Workers).

## Scope

### Files to Modify

```
internal/tui/update.go      # Kill and restart key handlers + message handlers
internal/tui/overlays.go    # Spawn dialog pre-fill for restart
internal/tui/keys.go        # Enable kill/restart in updateKeyStates
internal/tui/panels.go      # Killed state rendering in table
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 2**: workerKilledMsg (lines 252-254), killWorkerCmd (lines 265-267)
  - **Section 1**: WorkerHandle.Kill() with grace period (lines 89-92)
  - **Section 3**: StateKilled in state machine (lines 523-529)
- `design-artifacts/tui-keybinds.md`:
  - `x` kill: enabled when selected worker is running (line 21)
  - `r` restart: enabled when selected worker is failed/killed (line 23)
- `design-artifacts/tui-styles.md`:
  - StateKilled indicator: hot pink skull icon (line 341)
- `kitty-specs/016-kasmos-agent-orchestrator/spec.md`:
  - User Story 4 acceptance scenarios (lines 59-71)

## Implementation

### Kill Worker (`x` key)

When `x` is pressed with a running worker selected:
1. Call `killWorkerCmd(worker.ID, worker.Handle, 3*time.Second)`
2. Optimistically update worker state to `StateKilled` (visual feedback)
3. The kill command sends SIGTERM, waits grace period, escalates to SIGKILL
4. On `workerKilledMsg`:
   - If Err is nil: confirm killed state, set ExitedAt
   - If Err is non-nil: log error, worker may still be running (revert state?)
5. The `waitWorkerCmd` goroutine (started in WP04) will also fire `workerExitedMsg`
   -- handle the race: if worker is already StateKilled, the exitedMsg just
   confirms the exit code

Grace period: 3 seconds (matches tui-technical.md Section 1 line 91).

### Restart Worker (`r` key)

When `r` is pressed with a failed or killed worker selected:
1. Open the spawn dialog (same huh form from WP04)
2. Pre-fill the form fields with the original worker's data:
   - Role selector: set to original worker's role
   - Prompt textarea: set to original worker's prompt
   - Files input: set to original worker's files (comma-joined)
3. User can edit any field before confirming
4. On confirm: spawn a NEW worker (not a state change on the old one)
5. The new worker does NOT have ParentID set (restart is not continuation)
6. The new worker gets a fresh ID from the counter

To pre-fill the huh form, create the form with `huh.NewSelect().Value(&role)` where
role is pre-set. For the textarea, use `.Value(&prompt)` with the original prompt.

### updateKeyStates Refinement

In `updateKeyStates()`:
```go
m.keys.Kill.SetEnabled(selected != nil && selected.State == StateRunning)
m.keys.Restart.SetEnabled(selected != nil &&
    (selected.State == StateFailed || selected.State == StateKilled))
```

### Edge Cases

- Killing an already-exited worker: no-op (Kill should check state first)
- Killing a worker that is still in StateSpawning: wait for it to reach
  StateRunning first, or cancel the spawn
- Rapid kill-restart: ensure the old worker is fully dead before the restart
  completes (the kill is async, but the restart creates a new independent worker)
- Kill returns an error (process already dead): handle gracefully, update state to
  killed anyway since the process is gone

## What NOT to Do

- Do NOT implement batch kill (select multiple workers and kill all)
- Do NOT implement auto-restart on failure
- Do NOT implement AI-suggested restart prompts (that is WP11)
- Restart uses the standard spawn dialog, not a special restart dialog

## Acceptance Criteria

1. Select a running worker, press `x` -- worker shows as "killed" with skull icon
2. Worker process actually terminates (verify PID is gone)
3. Select a failed or killed worker, press `r` -- spawn dialog opens pre-filled
4. Edit the prompt and confirm -- new worker spawns with new ID
5. `x` key is disabled (not in help) when selected worker is not running
6. `r` key is disabled when selected worker is running or exited successfully
7. Killing a worker that has already exited is a no-op
8. `go test ./...` passes
