---
work_package_id: WP02
title: Worker Backend Package
lane: done
dependencies: []
subtasks:
- internal/worker/backend.go - WorkerBackend interface + types
- internal/worker/subprocess.go - SubprocessBackend (os/exec)
- internal/worker/worker.go - Worker struct + state machine
- internal/worker/output.go - OutputBuffer (ring buffer)
- internal/worker/session.go - Session ID extraction
- internal/worker/manager.go - WorkerManager + ID generation
- Unit tests for all files
phase: Wave 1 - Core TUI + Worker Lifecycle
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
- timestamp: '2026-02-18T05:39:41.254819519+00:00'
  lane: doing
  actor: manager
  shell_pid: '14003'
  action: 'transition active (Launching Wave 1 parallel pair: WP01 (bootstrap) + WP02 (worker backend))'
- timestamp: '2026-02-18T06:08:48.088165679+00:00'
  lane: done
  actor: manager
  shell_pid: '401658'
  action: 'transition done (Verified: 18 PASS, 1 WARN (acceptable). Tests, race, vet all clean.)'
---

# Work Package Prompt: WP02 - Worker Backend Package

## Mission

Implement the complete `internal/worker/` package: the WorkerBackend interface,
SubprocessBackend (os/exec), Worker domain type with state machine, OutputBuffer
ring buffer, session ID extraction, and WorkerManager. This package has zero TUI
dependency -- it is the pure process-management layer.

## Scope

### Files to Create

```
internal/worker/backend.go      # WorkerBackend interface, SpawnConfig, WorkerHandle, ExitResult
internal/worker/subprocess.go   # SubprocessBackend (os/exec MVP)
internal/worker/worker.go       # Worker struct, WorkerState enum, Duration(), FormatDuration()
internal/worker/output.go       # OutputBuffer (thread-safe ring buffer)
internal/worker/session.go      # extractSessionID regex patterns
internal/worker/manager.go      # WorkerManager, NextWorkerID(), atomic counter
internal/worker/backend_test.go
internal/worker/output_test.go
internal/worker/session_test.go
internal/worker/worker_test.go
internal/worker/manager_test.go
```

### Technical References (copy-paste ready)

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 1**: WorkerBackend interface, SpawnConfig, WorkerHandle, ExitResult,
    SubprocessBackend implementation (lines 30-197)
  - **Section 3**: Worker struct, WorkerState enum, Duration(), FormatDuration(),
    Children(), WorkerID generation (lines 452-556)
  - **Section 7**: OutputBuffer design (lines 1037-1084)
  - **Section 9**: extractSessionID regex, OpenCode CLI patterns (lines 1139-1215)
- `kitty-specs/016-kasmos-agent-orchestrator/data-model.md`:
  - Worker entity fields and state machine (lines 22-52)
  - WorkerBackend and WorkerHandle interfaces (lines 109-140)

## Implementation

### backend.go

Implement exactly as defined in tui-technical.md Section 1:
- `WorkerBackend` interface: `Spawn(ctx, cfg) (WorkerHandle, error)`, `Name() string`
- `SpawnConfig` struct: ID, Role, Prompt, Files, ContinueSession, Model, WorkDir, Env
- `WorkerHandle` interface: `Stdout() io.Reader`, `Wait() ExitResult`, `Kill(grace)`, `PID()`
- `ExitResult` struct: Code, Duration, SessionID, Error

### subprocess.go

Implement `SubprocessBackend` and `subprocessHandle` from tui-technical.md Section 1:
- `NewSubprocessBackend()` resolves `opencode` binary via `exec.LookPath`
- `Spawn()` builds args from SpawnConfig, sets `Setpgid: true` for process group isolation
- Merges stderr into stdout: `cmd.Stderr = cmd.Stdout`
- `buildArgs()` constructs the `opencode run` command line
- `subprocessHandle.Kill()`: SIGTERM with grace period, escalate to SIGKILL
- `subprocessHandle.Wait()`: blocks until exit, returns ExitResult with duration

### worker.go

Implement Worker struct and state machine from tui-technical.md Section 3:
- `WorkerState` enum: StatePending, StateSpawning, StateRunning, StateExited, StateFailed, StateKilled
- `Worker` struct with all fields from data-model.md
- `Duration()`, `FormatDuration()`, `Children()` methods

### output.go

Implement OutputBuffer from tui-technical.md Section 7:
- Thread-safe ring buffer with `sync.RWMutex`
- `DefaultMaxLines = 5000`
- `Append(data string)`: split on `\n`, replace non-UTF8 with U+FFFD
- `Lines()`, `Content()`, `Tail(n)`, `LineCount()`, `TotalLines()`, `Truncated()`
- Ring buffer behavior: when exceeding maxLines, discard oldest lines

### session.go

Implement session ID extraction from tui-technical.md Section 9:
- Pattern 1: `session:\s+(ses_[a-zA-Z0-9]+)` (text format)
- Pattern 2: `"session_id"\s*:\s*"(ses_[a-zA-Z0-9]+)"` (JSON format)
- Returns empty string if not found

### manager.go

Implement WorkerManager:
- Holds `[]*Worker` slice, tracks all workers
- `NextWorkerID()` with `atomic.Int64` counter, format `w-NNN` (zero-padded 3 digits)
- `ResetWorkerCounter(n)` for session restore
- `Add(w)`, `Get(id)`, `All()`, `Running()` methods

### Testing

Use table-driven tests (constitution requirement):
- **output_test.go**: Test Append with multi-line data, ring buffer overflow, Content() joining,
  Tail(), Truncated() count, concurrent Append safety
- **session_test.go**: Test both regex patterns, no match, partial matches
- **worker_test.go**: Test Duration() for running/exited/pending, FormatDuration() output
- **manager_test.go**: Test ID generation sequence, ResetWorkerCounter, Add/Get/Running
- **backend_test.go**: Test buildArgs() for various SpawnConfig combinations

Do NOT spawn real `opencode` processes in unit tests. Integration tests (gated by
`KASMOS_INTEGRATION=1`) can test real subprocess spawning with a simple command
like `echo hello`.

## Acceptance Criteria

1. `go test ./internal/worker/...` passes with all tests green
2. `go vet ./internal/worker/...` reports no issues
3. WorkerBackend interface is satisfied by SubprocessBackend (compile check)
4. OutputBuffer handles concurrent writes without data races (`go test -race`)
5. Session ID extraction works for both text and JSON patterns
6. Worker state machine transitions are validated (no invalid transitions)
