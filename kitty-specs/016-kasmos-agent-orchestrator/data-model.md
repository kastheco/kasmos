# Data Model: kasmos Agent Orchestrator

**Feature**: 016-kasmos-agent-orchestrator
**Date**: 2026-02-17

## Entity Relationship Diagram

```
Session 1---* Worker
Worker  0..1---1 Worker (parent -> child continuation)
Worker  0..1---1 Task
Source  1---* Task

Session --> SessionPersister --> .kasmos/session.json
Worker  --> WorkerBackend --> os/exec (SubprocessBackend)
Worker  --> OutputBuffer (ring buffer, 5000 lines default)
Source  <|-- SpecKittySource | GsdSource | AdHocSource
```

## Entities

### Worker

The central entity. Represents a managed OpenCode agent process.

| Field        | Type          | Description                                        | Validation                              |
|--------------|---------------|----------------------------------------------------|-----------------------------------------|
| ID           | string        | Unique worker identifier                           | Format: `w-NNN`, auto-incremented       |
| Role         | string        | Agent role                                         | One of: planner, coder, reviewer, release |
| Prompt       | string        | Task prompt sent to agent                          | Non-empty                               |
| Files        | []string      | Attached file paths                                | Valid paths (not validated at model layer) |
| State        | WorkerState   | Current lifecycle state                            | Enum, see state machine below           |
| ExitCode     | int           | Process exit code                                  | Set only when State is Exited/Failed    |
| SpawnedAt    | time.Time     | When worker was spawned                            | Set on spawn                            |
| ExitedAt     | time.Time     | When worker exited                                 | Zero if still running                   |
| SessionID    | string        | OpenCode session ID                                | Extracted from output via regex          |
| ParentID     | string        | Parent worker ID (continuations)                   | Empty if root worker                    |
| TaskID       | string        | Associated task from source                        | Empty for ad-hoc workers                |

**State Machine**:

```
StatePending --> StateSpawning --> StateRunning --+--> StateExited  (code 0)
                                                  +--> StateFailed  (code != 0)
                                                  +--> StateKilled  (user kill)
```

Transitions:
- Pending -> Spawning: User confirms spawn dialog
- Spawning -> Running: Process started successfully (workerSpawnedMsg)
- Running -> Exited/Failed/Killed: Process exits or user kills (workerExitedMsg/workerKilledMsg)
- Exited/Failed/Killed -> (new Worker): Continue or Restart creates a NEW worker, not a state change

### Task

A work item from an external task source.

| Field          | Type          | Description                                  | Validation                        |
|----------------|---------------|----------------------------------------------|-----------------------------------|
| ID             | string        | Task identifier                              | Source-dependent format            |
| Title          | string        | Short task name                              | Non-empty                         |
| Description    | string        | Detailed description (used as default prompt)| May be empty for GSD              |
| SuggestedRole  | string        | Recommended agent role                       | Empty if not inferrable           |
| Dependencies   | []string      | Task IDs this depends on                     | Empty for GSD/ad-hoc              |
| State          | TaskState     | Assignment status                            | Enum: Unassigned/Blocked/InProgress/Done/Failed |
| WorkerID       | string        | Assigned worker ID                           | Empty if unassigned               |
| Metadata       | map[string]string | Source-specific extra data               | spec-kitty: phase, lane, subtasks |

**State Machine**:

```
TaskUnassigned --+--> TaskInProgress  (worker spawned for this task)
                 +--> TaskBlocked     (dependency not met)
TaskBlocked    ----> TaskUnassigned   (dependency resolved)
TaskInProgress --+--> TaskDone        (worker exited successfully)
                 +--> TaskFailed      (worker failed)
TaskFailed     ----> TaskInProgress   (worker restarted)
```

### SessionState (persistence)

Serialized to `.kasmos/session.json`. See `research/tui-technical.md` Section 5 for full JSON schema.

| Field           | Type          | Description                                  |
|-----------------|---------------|----------------------------------------------|
| Version         | int           | Schema version (currently 1)                 |
| SessionID       | string        | Unique session identifier (ks-timestamp-rand)|
| StartedAt       | time.Time     | Session creation time                        |
| TaskSource      | *TaskSourceConfig | Source type + path (null for ad-hoc)      |
| Workers         | []WorkerSnapshot | All workers (active + historical)          |
| NextWorkerNum   | int           | Next worker ID number                        |
| PID             | int           | kasmos process PID (for reattach detection)  |

### SpawnConfig (runtime only, not persisted)

Parameters for creating a worker via `WorkerBackend.Spawn()`.

| Field            | Type          | Description                               |
|------------------|---------------|-------------------------------------------|
| ID               | string        | Pre-assigned worker ID                    |
| Role             | string        | Agent role for --agent flag               |
| Prompt           | string        | Task prompt (final argument)              |
| Files            | []string      | Paths for --file flags                    |
| ContinueSession  | string        | Session ID for --continue -s flag         |
| Model            | string        | Model override for --model flag           |
| WorkDir          | string        | Working directory (defaults to project root)|
| Env              | map[string]string | Additional environment variables        |

## Interfaces

### WorkerBackend

```go
type WorkerBackend interface {
    Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error)
    Name() string
}
```

### WorkerHandle

```go
type WorkerHandle interface {
    Stdout() io.Reader
    Wait() ExitResult
    Kill(gracePeriod time.Duration) error
    PID() int
}
```

### Source (Task Source)

```go
type Source interface {
    Type() string
    Path() string
    Load() ([]Task, error)
    Tasks() []Task
}
```

Full interface definitions with implementation details are in
`kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`.
