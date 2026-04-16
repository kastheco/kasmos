# kasmos Architecture — Canonical Facts Reference

> **Purpose.** This file is the single authoritative citation source for all
> architecture diagrams under `docs/architecture/`. Every claim is backed by an
> exact `path:line` pointer into the live codebase. No diagram prose appears
> here; diagrams live in their own files and link back to specific anchors.
>
> **Do not edit prose in this file without re-verifying the cited lines.**

---

## 1. Lifecycle Statuses and Transition Events

### 1.1 Status constants

Defined in `config/taskfsm/fsm.go:18-26`:

| Constant             | Wire value       |
|----------------------|------------------|
| `StatusReady`        | `"ready"`        |
| `StatusPlanning`     | `"planning"`     |
| `StatusImplementing` | `"implementing"` |
| `StatusReviewing`    | `"reviewing"`    |
| `StatusVerifying`    | `"verifying"`    |
| `StatusDone`         | `"done"`         |
| `StatusCancelled`    | `"cancelled"`    |

### 1.2 Event constants

Defined in `config/taskfsm/fsm.go:39-53`:

| Constant                 | Wire value                   | User-only? |
|--------------------------|------------------------------|------------|
| `PlanStart`              | `"plan_start"`               |            |
| `PlannerFinished`        | `"planner_finished"`         |            |
| `ImplementStart`         | `"implement_start"`          |            |
| `ImplementFinished`      | `"implement_finished"`       |            |
| `ReviewApproved`         | `"review_approved"`          |            |
| `ReviewChangesRequested` | `"review_changes_requested"` |            |
| `VerifyApproved`         | `"verify_approved"`          |            |
| `VerifyFailed`           | `"verify_failed"`            |            |
| `RequestReview`          | `"request_review"`           | yes        |
| `StartOver`              | `"start_over"`               | yes        |
| `Reimplement`            | `"reimplement"`              | yes        |
| `Cancel`                 | `"cancel"`                   | yes        |
| `Reopen`                 | `"reopen"`                   | yes        |

User-only events (`config/taskfsm/fsm.go:85-91`) can only be triggered from the
TUI, never by agent sentinel files or signal-gateway rows.

`ArchitectFinished` (`"architect_finished"`) is an additional internal-only
event constant defined in `config/taskfsm/gateway_signal.go:14`. It is **not**
in the main `Event` iota but is used as a typed constant for the architect
completion path; its gateway wire alias is `"elaborator_finished"` (see §3).

### 1.3 Transition table

Source: `config/taskfsm/fsm.go:95-129`.

```
ready        + plan_start               → planning
ready        + implement_start          → implementing
ready        + cancel                   → cancelled

planning     + plan_start               → planning       (restart after crash)
planning     + planner_finished         → ready
planning     + cancel                   → cancelled

implementing + implement_finished       → reviewing
implementing + cancel                   → cancelled

reviewing    + review_approved          → verifying
reviewing    + review_changes_requested → implementing
reviewing    + cancel                   → cancelled

verifying    + verify_approved          → done
verifying    + verify_failed            → implementing
verifying    + cancel                   → cancelled

done         + start_over              → planning
done         + reimplement             → implementing
done         + request_review          → reviewing
done         + cancel                  → cancelled

cancelled    + reopen                  → planning
```

### 1.4 Core FSM functions

- `func ApplyTransition(current Status, event Event) (Status, error)`  
  `config/taskfsm/fsm.go:133`

- `func TransitionExecutionState(event Event, next Status) taskstore.ExecutionState`  
  `config/taskfsm/fsm.go:147` — returns `{Phase: "planned"}` only when event is
  `PlannerFinished` and next is `StatusReady`; zero value otherwise.

- `func MapLegacyStatus(s taskstate.Status) Status`  
  `config/taskfsm/fsm.go:234` — maps `"in_progress"` → `StatusImplementing`,
  `"completed"` → `StatusDone`; pass-through for all other values.

---

## 2. Execution Phases and Operator Phrases

### 2.1 ExecutionPhase constants

Defined in `config/taskfsm/fsm.go:27-34`:

| Constant                              | Wire value                    |
|---------------------------------------|-------------------------------|
| `ExecutionPhasePlanned`               | `"planned"`                   |
| `ExecutionPhaseArchitecting`          | `"architecting"`              |
| `ExecutionPhaseWaveRunning`           | `"wave_running"`              |
| `ExecutionPhaseWaveWaiting`           | `"wave_waiting"`              |
| `ExecutionPhaseSingleAgentImplementing` | `"single_agent_implementing"` |
| `ExecutionPhaseFixing`                | `"fixing"`                    |
| `ExecutionPhaseReviewing`             | `"reviewing"`                 |

### 2.2 Phase classification helpers

- `IsWaveExecutionPhase(phase)` — true for `wave_running`, `wave_waiting`  
  `config/taskfsm/fsm.go:63`

- `IsSingleAgentImplementingPhase(phase)` — true for `single_agent_implementing`, `fixing`  
  `config/taskfsm/fsm.go:74`

### 2.3 Phase timestamps recorded on transition

`phaseNameForStatus` (`config/taskfsm/fsm.go:208`) maps statuses to the phase
timestamp keys stored via `Store.SetPhaseTimestamp`:

| Status          | Phase key        |
|-----------------|------------------|
| `planning`      | `"planning"`     |
| `implementing`  | `"implementing"` |
| `reviewing`     | `"reviewing"`    |
| `verifying`     | `"verifying"`    |
| `done`          | `"done"`         |

---

## 3. Gateway Signal Names and Compatibility Aliases

### 3.1 Three naming layers — distinguished explicitly

| Layer | Example | Where defined |
|-------|---------|---------------|
| **FSM event constant** (Go code, internal) | `ImplementFinished` → `"implement_finished"` | `config/taskfsm/fsm.go:42` |
| **operator-facing recovery token** (admin UI transition catalog) | `"review_changes"` (alias for `review_changes_requested`) | `config/taskfsm/gateway_signal.go:44`; `config/taskactions/handler.go:71` |
| **gateway wire type** (stored in SQLite `signals.signal_type`) | `"elaborator_finished"` (alias for `architect_finished` / `ArchitectFinished`) | `config/taskfsm/gateway_signal.go:46-47` |

### 3.2 Canonical gateway signal types (stored verbatim in DB)

`validGatewaySignalTypes` at `config/taskfsm/gateway_signal.go:16-27`:

| Wire type                  | Description |
|----------------------------|-------------|
| `plan_start`               | Task begins planning |
| `planner_finished`         | Planner agent complete |
| `implement_finished`       | Coder/fixer complete |
| `review_approved`          | Reviewer approved |
| `review_changes_requested` | Reviewer requested changes |
| `implement_task_finished`  | Individual wave task complete (requires `wave_number`, `task_number` in payload) |
| `implement_wave`           | Full wave complete (requires `wave_number` in payload) |
| `elaborator_finished`      | Architect pass complete (**legacy wire alias** for `architect_finished`) |
| `verify_approved`          | Master/verifier approved |
| `verify_failed`            | Master/verifier failed |

### 3.3 Accepted input aliases → canonical wire type

`CanonicalGatewaySignalType` at `config/taskfsm/gateway_signal.go:39-59`:

| Input alias | Canonical wire type |
|-------------|---------------------|
| `architect_finished`, `elaborator_finished` | `elaborator_finished` |
| `review_changes` | `review_changes_requested` |
| `readiness_approved`, `master_approved` | `verify_approved` |
| `readiness_changes_requested`, `readiness_changes` | `verify_failed` |
| Hyphen variants (e.g. `review-approved`) | Normalized to underscore form |

### 3.4 Event → gateway signal type mapping

`GatewaySignalTypeForEvent` at `config/taskfsm/gateway_signal.go:63-74`:

| FSM Event | Gateway wire type |
|-----------|-------------------|
| `PlanStart` | `"plan_start"` |
| `PlannerFinished` | `"planner_finished"` |
| `ImplementFinished` | `"implement_finished"` |
| `ReviewApproved` | `"review_approved"` |
| `ReviewChangesRequested` | `"review_changes_requested"` |
| `VerifyApproved` | `"verify_approved"` |
| `VerifyFailed` | `"verify_failed"` |
| `ArchitectFinished` or `Event("elaborator_finished")` | `"elaborator_finished"` |

Events `ImplementStart`, `StartOver`, `Reimplement`, `RequestReview`, `Cancel`,
`Reopen` do **not** map to a gateway signal (`GatewaySignalTypeForEvent` returns
an error).

### 3.5 Emit function signature

```go
func EmitGatewaySignal(gw taskstore.SignalGateway, project, signalType, planFile, payload string) error
```
`config/taskfsm/gateway_signal.go:148`

Validates the type, normalizes the payload, then calls `gw.Create(project, SignalEntry{...})`.

---

## 4. Filesystem Sentinel Names

The `.kasmos/signals/` directory (legacy path; gateway-backed signals are
primary — `config/taskfsm/signals.go:49`) is still scanned by the daemon's tick
loop and bridged into the gateway.

### 4.1 FSM sentinel prefixes

`sentinelPrefixes` at `config/taskfsm/signals.go:36-44`:

| Filename prefix          | Maps to FSM event          |
|--------------------------|----------------------------|
| `planner-finished-`      | `PlannerFinished`          |
| `implement-finished-`    | `ImplementFinished`        |
| `review-approved-`       | `ReviewApproved`           |
| `review-changes-`        | `ReviewChangesRequested`   |

Full filename: `<prefix><planFile>` (e.g. `implement-finished-my-task`).  
Body of the file (if non-empty) becomes the `Signal.Body` field
(`config/taskfsm/signals.go:92-94`).

### 4.2 Elaboration sentinel

Prefix: `elaborator-finished-`  
Type: `ElaborationSignal` (`config/taskfsm/elaboration_signal.go:10-15`).  
Example: `elaborator-finished-my-task`.  
Does **not** map to an FSM transition; triggers orchestration to start first wave.

### 4.3 Wave-task sentinel

Pattern: `implement-task-finished-w<N>-t<M>-<planFile>`  
Regex: `config/taskfsm/task_signal.go:19`  
Type: `TaskSignal` with `WaveNumber`, `TaskNumber`, `TaskFile` fields.

### 4.4 Wave-complete sentinel

Pattern: `implement-wave-<N>-<planFile>`  
Regex: `config/taskfsm/wave_signal.go:18`  
Type: `WaveSignal` with `WaveNumber`, `TaskFile` fields.

---

## 5. Signal-Gateway Interface and Row Lifecycle

### 5.1 Interface definition

```go
type SignalGateway interface {
    Create(project string, entry SignalEntry) error
    List(project string, statuses ...SignalStatus) ([]SignalEntry, error)
    Claim(project, claimedBy string) (*SignalEntry, error)
    MarkProcessed(id int64, status SignalStatus, result string) error
    ResetStuck(olderThan time.Duration) (int, error)
    Close() error
}
```
`config/taskstore/signal.go:31-47`

### 5.2 SignalStatus values

`config/taskstore/signal.go:8-13`:

| Constant          | Wire value      |
|-------------------|-----------------|
| `SignalPending`   | `"pending"`     |
| `SignalProcessing`| `"processing"`  |
| `SignalDone`      | `"done"`        |
| `SignalFailed`    | `"failed"`      |

### 5.3 SignalEntry fields

`config/taskstore/signal.go:16-28`:

| Field         | Type          | Notes |
|---------------|---------------|-------|
| `ID`          | `int64`       | SQLite PRIMARY KEY |
| `Project`     | `string`      | |
| `PlanFile`    | `string`      | task filename (no `.md` suffix after normalization) |
| `SignalType`  | `string`      | one of the canonical wire types in §3.2 |
| `Payload`     | `string`      | JSON or empty; validated by `NormalizeGatewaySignalPayload` |
| `Status`      | `SignalStatus`| |
| `CreatedAt`   | `time.Time`   | |
| `ClaimedBy`   | `string`      | daemon instance or processor label |
| `ClaimedAt`   | `time.Time`   | |
| `ProcessedAt` | `time.Time`   | |
| `Result`      | `string`      | error message on failure |

### 5.4 SQLite schema

`config/taskstore/signal_sqlite.go:12-29`:

```sql
CREATE TABLE IF NOT EXISTS signals (
    id           INTEGER PRIMARY KEY,
    project      TEXT    NOT NULL DEFAULT '',
    plan_file    TEXT    NOT NULL DEFAULT '',
    signal_type  TEXT    NOT NULL DEFAULT '',
    payload      TEXT    NOT NULL DEFAULT '',
    status       TEXT    NOT NULL DEFAULT 'pending',
    created_at   TEXT    NOT NULL DEFAULT '',
    claimed_by   TEXT    NOT NULL DEFAULT '',
    claimed_at   TEXT    NOT NULL DEFAULT '',
    processed_at TEXT    NOT NULL DEFAULT '',
    result       TEXT    NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_signals_project_status_created_at
    ON signals(project, status, created_at, id);
```

`Claim` is atomic: it wraps `SELECT ... LIMIT 1` + `UPDATE` in a single
transaction (`config/taskstore/signal_sqlite.go:112-161`) to prevent double-processing.

### 5.5 Row lifecycle

```
[Create] → pending → [Claim] → processing → [MarkProcessed] → done | failed
                                          ↑
                                    [ResetStuck] ← stuck (processing too long)
```

`ResetStuck` resets rows that have been `processing` for longer than `olderThan`
back to `pending` (`config/taskstore/signal_sqlite.go:186-202`).

### 5.6 Filesystem → gateway bridge

`orchestration/loop/bridge.go:19-101` bridges legacy `.kasmos/signals/` sentinel
files into gateway rows. It scans FSM signals (line 27), task signals (line 46),
wave signals (line 67), and elaboration signals (line 87), then calls
`EmitGatewaySignal` / `gw.Create` for each and deletes the file.

### 5.7 `fsm_applied` payload flag

When the HTTP admin handler (`config/taskactions/handler.go:142`) emits a
signal after applying the FSM transition itself, it uses the payload
`{"fsm_applied":true}`. The gateway scanner (`orchestration/loop/gateway_scanner.go:15-20`)
decodes this field into `Signal.PreApplied`, enabling the daemon processor to
skip re-applying the FSM and run only downstream side effects.

---

## 6. Task-Store HTTP Routes

Handler factory: `func NewHandler(store Store) http.Handler`  
`config/taskstore/server.go:21`

All routes are method+path patterns (Go 1.22+ `ServeMux`).

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/v1/ping` | liveness check; calls `store.Ping()` |
| `GET`  | `/v1/projects/{project}/tasks` | list tasks; optional `?status=` and `?topic=` |
| `POST` | `/v1/projects/{project}/tasks` | create task |
| `GET`  | `/v1/projects/{project}/tasks/{filename}` | get task |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}` | update task |
| `DELETE` | `/v1/projects/{project}/tasks/{filename}` | delete task |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/execution-state` | set execution state |
| `GET`  | `/v1/projects/{project}/tasks/{filename}/content` | get markdown content |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/content` | set markdown content |
| `GET`  | `/v1/projects/{project}/tasks/{filename}/subtasks` | get subtasks |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/subtasks` | set subtasks |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/phase-timestamp/{phase}` | record phase timestamp |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/goal` | set goal |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/pr-url` | set PR URL |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/pr-state` | set PR state |
| `POST` | `/v1/projects/{project}/tasks/{filename}/rename` | rename task |
| `GET`  | `/v1/projects/{project}/tasks/{filename}/pr-reviews` | list PR reviews |
| `POST` | `/v1/projects/{project}/tasks/{filename}/pr-reviews` | record PR review |

Source: `config/taskstore/server.go:21-568`.

Filename slugs have `.md` stripped by `normalizeFilename`
(`config/taskstore/server.go:15`).

---

## 7. Admin Task-Action Routes

Handler factory: `func NewHandler(store taskstore.Store, gateway taskstore.SignalGateway) http.Handler`  
`config/taskactions/handler.go:91`

These are **separate** from the task-store CRUD routes above and are mounted
at more-specific patterns in `cmd/serve.go:215-222` (actionsAPI takes precedence
over taskAPI for these paths):

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/v1/projects/{project}/tasks/{filename}/available-actions` | list valid FSM events from current status |
| `POST` | `/v1/projects/{project}/tasks/{filename}/transition` | apply FSM event + emit gateway signal |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/status` | force-set status (no FSM validation) |
| `POST` | `/v1/projects/{project}/tasks/{filename}/rename` | rename with content-aware semantics |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/topic` | set topic |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/goal` | set goal |
| `PUT`  | `/v1/projects/{project}/tasks/{filename}/content` | update content (rich semantics) |

Source: `config/taskactions/handler.go:95-103`.

### 7.1 Transition handler behaviour

`handleTransition` (`config/taskactions/handler.go:249-313`):

1. Calls `taskfsm.ApplyTransition` to validate FSM legality.
2. Calls `TaskStateMachine.Transition` to write new status.
3. Emits a gateway signal with payload `{"fsm_applied":true}` so the daemon
   runs downstream side effects without re-applying the FSM.

### 7.2 Transition catalog (operator-facing names)

`transitionCatalog` at `config/taskactions/handler.go:65-79`:

| Operator token | Maps to FSM event | Label |
|----------------|-------------------|-------|
| `plan_start` | `PlanStart` | start planning |
| `planner_finished` | `PlannerFinished` | mark planning finished |
| `implement_start` | `ImplementStart` | start implement |
| `implement_finished` | `ImplementFinished` | mark implement finished |
| `review_approved` | `ReviewApproved` | mark review approved |
| `review_changes` | `ReviewChangesRequested` | mark changes requested |
| `verify_approved` | `VerifyApproved` | mark verify approved |
| `verify_failed` | `VerifyFailed` | mark verify failed |
| `request_review` | `RequestReview` | request review |
| `start_over` | `StartOver` | start over |
| `reimplement` | `Reimplement` | resume implement |
| `cancel` | `Cancel` | cancel task |
| `reopen` | `Reopen` | reopen task |

---

## 8. Daemon Control API and SSE Event Kinds

### 8.1 StateProvider interface

`config/daemon/api/server.go:99-113`. The `Daemon` struct satisfies this
interface; `DaemonState` provides a lightweight in-memory test implementation.

```go
type StateProvider interface {
    Status() StatusResponse
    ListRepos() []RepoStatus
    AddRepo(path string) error
    RemoveRepo(project string) error
    ListPlans(project string) ([]taskstore.TaskEntry, error)
    ListTasks(project string) ([]TaskStatus, error)
    ListInstances(project string) []InstanceStatus
    StartPlan(project, filename, prompt, program string) error
    EventStream() <-chan Event
    PauseInstance(project, title string) error
    ResumeInstance(project, title string) error
    RestartInstance(project, title string) error
    KillInstance(project, title string) error
}
```

Handler factories (`daemon/api/server.go:228-248`):

- `func NewHandler(state StateProvider) http.Handler`
- `func NewHandlerWithBroadcaster(state StateProvider, b *EventBroadcaster) http.Handler`

### 8.2 Daemon control API routes

`registerRoutes` at `daemon/api/server.go:255-283`:

| Method | Path | Description |
|--------|------|-------------|
| `GET`  | `/v1/ping` | liveness |
| `GET`  | `/v1/status` | daemon overview (`StatusResponse`) |
| `POST` | `/v1/reload` | reload configuration |
| `GET`  | `/v1/repos` | list registered repos |
| `POST` | `/v1/repos` | add repo |
| `DELETE` | `/v1/repos/{project}` | remove repo |
| `GET`  | `/v1/repos/{project}/plans` | list tasks in project |
| `GET`  | `/v1/repos/{project}/tasks` | list tasks with `TaskStatus` shape |
| `GET`  | `/v1/repos/{project}/instances` | list running instances |
| `POST` | `/v1/repos/{project}/instances/{title}/pause` | pause instance |
| `POST` | `/v1/repos/{project}/instances/{title}/resume` | resume instance |
| `POST` | `/v1/repos/{project}/instances/{title}/restart` | restart instance |
| `POST` | `/v1/repos/{project}/instances/{title}/kill` | kill instance |
| `POST` | `/v1/repos/{project}/plans/{filename}/plan` | start planning |
| `POST` | `/v1/repos/{project}/plans/{filename}/implement` | start implementation |
| `GET`  | `/v1/events` | SSE stream |

Served over a Unix domain socket, not TCP. `ListenUnix` at
`daemon/api/server.go:497`.

### 8.3 Daemon socket path

`daemon/daemon.go:359-364`:

- Primary: `$XDG_RUNTIME_DIR/kasmos/kas.sock`
- Fallback: `/tmp/kasmos-<uid>/kas.sock`

### 8.4 SSE event kinds

`daemon/api/events.go:8-14`:

| Constant | Wire value | Meaning |
|----------|-----------|---------|
| `EventKindAgentSpawned` | `"agent_spawned"` | daemon launched an agent |
| `EventKindStuckDetected` | `"stuck_detected"` | implementing agent exited without auto-advance path |

`Event` struct (`daemon/api/events.go:18-25`): `Kind`, `Message`, `Repo`,
`PlanFile`, `AgentType`, `Timestamp`.

`EventBroadcaster` (`daemon/api/events.go:30-94`) fans out to per-subscriber
buffered channels (capacity 64). Missed events are dropped silently when a
subscriber buffer is full.

---

## 9. MCP Tools

### 9.1 Transport

`internal/mcpserver/server.go:35-53`:

- Transport: **Streamable HTTP**, mounted at `/mcp` via
  `server.NewStreamableHTTPServer(mcpSrv, server.WithEndpointPath("/mcp"))`.
- `kas serve --mcp` starts a **separate** HTTP listener for MCP (default port
  different from the task-store port); see `cmd/serve.go:420-434`.
- Stdio transport also available via `Server.ServeStdio()`
  (`internal/mcpserver/server.go:91-98`).

`Server` struct (`internal/mcpserver/server.go:24-30`) holds the `mcp.MCPServer`,
`http.Handler`, `taskstore.Store`, `taskstore.SignalGateway`, and a `[]Closer`
for owned resources.

### 9.2 Task tools

Registered by `tasktools.RegisterTools` at
`internal/mcpserver/tasktools/tasktools.go:401-450`:

| Tool name | Required args | Description |
|-----------|---------------|-------------|
| `task_list` | — | list task entries; optional `status` filter |
| `task_show` | `filename` | read stored markdown content |
| `task_create` | `name` | create a new task entry |
| `task_update_content` | `filename`, `content` | replace stored markdown |
| `task_delete` | `filename` | delete task entry |
| `task_transition` | `filename`, `event` | apply FSM event (optionally `force=true`) |

All tools accept an optional `project` argument for multi-repo routing.

`task_transition` without `force` calls `TaskStateMachine.Transition` then
emits a gateway signal (`internal/mcpserver/tasktools/tasktools.go:362-381`) so
the daemon's tick picks up side effects — equivalent to the HTTP admin handler.

### 9.3 Signal tools

Registered by `signaltools.RegisterTools` at
`internal/mcpserver/signaltools/signaltools.go:75-89`:

| Tool name | Required args | Description |
|-----------|---------------|-------------|
| `signal_create` | `signal_type`, `plan_file` | insert pending signal into gateway |

Optional args: `payload`, `project`.

`signal_create` calls `taskfsm.CanonicalGatewaySignalType` then
`taskfsm.EmitGatewaySignal` (`signaltools.go:54-60`).

---

## 10. Agent Roles and Execution Modes

### 10.1 Agent type constants

From `session/` package (referenced in `orchestration/lifecycle_agents.go`):

| Constant | Wire value | Title pattern |
|----------|-----------|---------------|
| `AgentTypeReviewer` | `"reviewer"` | `<plan>-review-<cycle>` |
| `AgentTypeFixer` | `"fixer"` | `<plan>-fix-<cycle>` |
| `AgentTypeMaster` | `"master"` | `<plan>-verify-<cycle>` |
| `AgentTypeElaborator` | `"elaborator"` | `<plan>-elaborator` |
| (planner, implicit) | — | `<plan>-plan` |

Title builder: `BuildLifecycleAgentTitle` at `orchestration/lifecycle_agents.go:39-58`.

Wave task titles: `BuildWaveTaskTitle` returns `<plan>-W<N>-T<M>`
(`orchestration/lifecycle_agents.go:132`).

### 10.2 Agent spec builders

`orchestration/lifecycle_agents.go`:

| Function | Line | Spawns |
|----------|------|--------|
| `BuildPlannerAgentSpec` | 103 | planner; title `<plan>-plan` |
| `BuildArchitectAgentSpec` | 91 | architect pass; title `<plan>-elaborator` |
| `BuildReviewerAgentSpec` | 62 | reviewer; cycle-aware |
| `BuildFixerAgentSpec` | 77 | fixer; cycle-aware |
| `BuildMasterAgentSpec` | 118 | master/verifier; wraps `BuildMasterAgentSpecWithConfig` |
| `BuildMasterAgentSpecWithConfig` | 124 | configurable self-fix ceiling and verify cap |

### 10.3 Execution modes

`session/execution.go:13-20`:

| Constant | Wire value | Description |
|----------|-----------|-------------|
| `ExecutionModeTmux` | `"tmux"` | default; agent hosted in a tmux pane |
| `ExecutionModeHeadless` | `"headless"` | agent runs as `exec.Cmd`; no tmux |

`NormalizeExecutionMode` (`session/execution.go:77`) returns `ExecutionModeHeadless`
only when the input is exactly `"headless"` (after whitespace trim); all other
values, including `""`, map to `ExecutionModeTmux`.

`NewExecutionSession(mode, name, program, skipPermissions)` dispatches to
`headless.New` or `tmux.NewTmuxSession` accordingly
(`session/execution.go:85-92`).

`ExecutionSession` interface (`session/execution.go:29-65`) provides a unified
surface over both backends: lifecycle (`Start`, `Restore`, `Close`), I/O
(`SendKeys`, `TapEnter`, `CapturePaneContent`), attach/detach, and configuration
builder methods (`SetAgentType`, `SetInitialPrompt`, `SetTaskEnv`, …).

---

## 11. Service Names and Storage Paths

### 11.1 systemd units (Linux)

`internal/platform/service.go:46-53`:

| Unit name | Role |
|-----------|------|
| `kasmos` | background daemon (`daemon/daemon.go`) |
| `kasmosdb` | task-store HTTP server (`cmd/serve.go`) |

Restart command: `systemctl --user start kasmosdb kasmos`
(`internal/platform/service.go:71`).

Install directory: `~/.config/systemd/user`
(`internal/platform/service.go:84`).

### 11.2 launchd plists (macOS)

`internal/platform/service.go:11-15`:

| Plist filename | Role |
|----------------|------|
| `com.kasmos.daemon.plist` | kasmos daemon |
| `com.kasmos.taskstore.plist` | task-store HTTP server |

Install directory: `~/Library/LaunchAgents`
(`internal/platform/service.go:86`).

### 11.3 Storage paths

| Path | Description | Source |
|------|-------------|--------|
| `~/.config/kasmos/taskstore.db` | SQLite file shared by task store, signal gateway, and audit logger | `config/taskstore/factory.go:166-172` (`GlobalDBPath`) |
| `$XDG_RUNTIME_DIR/kasmos/kas.sock` | daemon Unix socket (primary) | `daemon/daemon.go:360-361` |
| `/tmp/kasmos-<uid>/kas.sock` | daemon Unix socket (fallback) | `daemon/daemon.go:363` |
| `.kasmos/signals/` | legacy filesystem sentinel directory | `config/taskfsm/signals.go:49` |
| `web/admin/dist` | embedded admin SPA build artefacts | `web/admin_assets.go:12` |

**DB path note:** The local SQLite database is at
`~/.config/kasmos/taskstore.db` (the global user config directory), **not**
under a per-repo `.kasmos/` directory. `GlobalDBPath` and `ResolvedDBPath`
both return this path (`config/taskstore/factory.go:166-180`). The
per-repo `.kasmos/` directory exists only for sentinel files and
`config.toml`.

### 11.4 Admin SPA

`kas serve` serves the admin SPA at `/admin/` using assets embedded from
`web/admin/dist` via `web/admin_assets.go:12-22` (`//go:embed all:admin/dist`).  
The SPA is mounted in `cmd/serve.go:382-384`:

```go
rootMux.Handle("/admin", http.RedirectHandler("/admin/", http.StatusMovedPermanently))
rootMux.Handle("/admin/", http.StripPrefix("/admin", adminFallbackHandler(adminFS)))
```

`--admin-dir` overrides the embedded assets for local development; it must point
at a directory containing `index.html` (`cmd/serve.go:370-380`).  
Do **not** reference `web/admin/` as a runtime surface — it is a source tree;
the runtime asset root is `web/admin/dist`.

---

*Generated from live source. Update whenever a cited symbol changes.*
