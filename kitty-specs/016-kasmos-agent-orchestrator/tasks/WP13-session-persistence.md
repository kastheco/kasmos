---
work_package_id: "WP13"
title: "Session Persistence + Reattach"
lane: "planned"
dependencies:
  - "WP04"
subtasks:
  - "internal/persist/schema.go - SessionState struct (maps to JSON schema)"
  - "internal/persist/session.go - SessionPersister (save/load, atomic write, debounce)"
  - "cmd/kasmos/main.go - --attach flag"
  - "Reattach logic: detect running session, restore state"
  - "Orphan detection (PID dead, mark workers killed)"
  - "Output tail preservation (last 200 lines per worker)"
  - "Unit tests"
phase: "Wave 3 - Daemon Mode + Persistence"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
history:
  - timestamp: "2026-02-17T00:00:00Z"
    lane: "planned"
    agent: "planner"
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP13 - Session Persistence + Reattach

## Mission

Implement session state persistence and reattach: kasmos saves session state to
`.kasmos/session.json` after every state change (debounced), and `kasmos --attach`
restores the session from disk. Orphaned sessions (PID dead) get their running
workers marked as killed. This delivers User Story 8 (Session Persistence and
Reattach).

## Scope

### Files to Create

```
internal/persist/schema.go      # SessionState, WorkerSnapshot, TaskSourceConfig structs
internal/persist/session.go     # SessionPersister: save, load, atomic write, debounce
internal/persist/session_test.go
internal/persist/schema_test.go
```

### Files to Modify

```
cmd/kasmos/main.go              # --attach flag, session restore on startup
internal/tui/model.go           # Add SessionPersister, trigger saves
internal/tui/update.go          # Call persister.Save() after state-mutating messages
internal/worker/manager.go      # ResetWorkerCounter for session restore
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 5**: Session persistence schema, JSON schema, example file,
    persistence behavior (lines 703-904)
  - **Section 8**: Graceful shutdown persist step (lines 1089-1121)
- `kitty-specs/016-kasmos-agent-orchestrator/data-model.md`:
  - SessionState entity (lines 80-93)
- `kitty-specs/016-kasmos-agent-orchestrator/spec.md`:
  - User Story 8 acceptance scenarios (lines 123-135)

## Implementation

### schema.go

Define Go structs matching the JSON schema in tui-technical.md Section 5:

```go
type SessionState struct {
    Version      int              `json:"version"`         // always 1
    SessionID    string           `json:"session_id"`      // "ks-{unix_ts}-{rand4}"
    StartedAt    time.Time        `json:"started_at"`
    TaskSource   *TaskSourceConfig `json:"task_source"`    // null for ad-hoc
    Workers      []WorkerSnapshot `json:"workers"`
    NextWorkerNum int             `json:"next_worker_num"`
    PID          int              `json:"pid"`
}

type TaskSourceConfig struct {
    Type string `json:"type"` // "spec-kitty", "gsd", "ad-hoc"
    Path string `json:"path"`
}

type WorkerSnapshot struct {
    ID         string     `json:"id"`
    Role       string     `json:"role"`
    Prompt     string     `json:"prompt"`
    Files      []string   `json:"files"`
    State      string     `json:"state"`       // "pending","spawning","running","exited","failed","killed"
    ExitCode   *int       `json:"exit_code"`   // null if not exited
    SpawnedAt  time.Time  `json:"spawned_at"`
    ExitedAt   *time.Time `json:"exited_at"`   // null if running
    DurationMs *int64     `json:"duration_ms"` // null if running
    SessionID  string     `json:"session_id"`
    ParentID   string     `json:"parent_id"`
    TaskID     string     `json:"task_id"`
    PID        *int       `json:"pid"`         // null if not running
    OutputTail string     `json:"output_tail"` // last 200 lines
}
```

**Session ID generation**:
```go
func NewSessionID() string {
    return fmt.Sprintf("ks-%d-%s", time.Now().Unix(), randomAlpha(4))
}
```

**Conversion functions**:
- `WorkerToSnapshot(w *worker.Worker) WorkerSnapshot` -- converts live worker to snapshot
- `SnapshotToWorker(s WorkerSnapshot) *worker.Worker` -- restores worker from snapshot
  (Handle will be nil, Output will be populated from OutputTail)

### session.go

Implement SessionPersister:

```go
type SessionPersister struct {
    Path     string        // ".kasmos/session.json"
    debounce time.Duration // 1 second
    mu       sync.Mutex
    dirty    bool
    timer    *time.Timer
}
```

**Save(state SessionState)**:
- Set dirty flag
- If no timer running, start debounce timer
- When timer fires: acquire lock, write to temp file, rename atomically

**SaveSync(state SessionState)**:
- Immediate write (no debounce). Used during shutdown.

**Atomic write**:
```go
func (p *SessionPersister) writeAtomic(state SessionState) error {
    data, err := json.MarshalIndent(state, "", "  ")
    if err != nil { return err }
    
    tmpPath := p.Path + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0o644); err != nil { return err }
    return os.Rename(tmpPath, p.Path)
}
```

**Load()**:
- Read `.kasmos/session.json`
- Unmarshal into SessionState
- Validate version field

### Reattach Logic (main.go)

`kasmos --attach` flag:

1. Check if `.kasmos/session.json` exists
2. Load session state
3. Check if PID in session is alive: `syscall.Kill(pid, 0)`
   - If alive: "Session already active (PID {pid}). Cannot reattach to a running session."
     Exit 1. (True reattach to a running process would require IPC -- future work)
   - If dead: proceed with restore
4. Mark all workers with state "running" as "killed" (orphaned)
5. Reset worker counter to `NextWorkerNum` from session
6. Restore worker list (from snapshots, with OutputTail loaded into OutputBuffers)
7. Start TUI with restored state
8. Generate new session PID

If no session file exists: "No session found. Start a new session with `kasmos`."

### Integration with TUI (model.go / update.go)

Add `persister *persist.SessionPersister` to Model.

Call `persister.Save(m.buildSessionState())` after every state-mutating message:
- workerSpawnedMsg
- workerExitedMsg
- workerKilledMsg
- spawnDialogSubmittedMsg
- continueDialogSubmittedMsg
- taskStateChangedMsg

`buildSessionState()` converts current Model state to a SessionState struct.

On graceful shutdown (from WP06): call `persister.SaveSync()` before exit.

### Startup Session File

On normal startup (not --attach):
1. Create `.kasmos/` directory if not exists
2. Generate new session ID
3. Write initial session state with PID = os.Getpid()
4. On exit: write final state

### Testing

**schema_test.go**:
- Test SessionState JSON marshaling/unmarshaling roundtrip
- Test WorkerToSnapshot/SnapshotToWorker conversion
- Test SessionID generation format
- Test null handling for optional fields (ExitCode, ExitedAt, PID)

**session_test.go**:
- Test atomic write (file exists after write, no partial writes)
- Test debounce (multiple saves within 1s produce one write)
- Test SaveSync (immediate write, no debounce)
- Test Load with valid/invalid/missing files
- Test PID-alive detection (mock with current PID)

## What NOT to Do

- Do NOT implement true process reattach (connecting to a running kasmos instance)
  -- that would require IPC/socket communication, which is future work
- Do NOT persist worker output beyond OutputTail (200 lines) -- full output is lost
- Do NOT persist the task source state (tasks are re-loaded from files on reattach)
- Do NOT encrypt session.json (no secrets stored)
- Do NOT persist TUI state (focus, scroll position, layout mode) -- only worker data

## Acceptance Criteria

1. `.kasmos/session.json` is written after worker state changes
2. File contents match the JSON schema from tui-technical.md Section 5
3. Atomic write: no partial/corrupt files even on crash
4. Debounce: rapid state changes produce at most one write per second
5. `kasmos --attach` restores worker states from session file
6. Orphaned workers (PID dead) are marked as killed on restore
7. Worker counter resets correctly (no ID collisions after restore)
8. OutputTail (last 200 lines) is preserved and displayed on restore
9. "No session found" message when no session file exists
10. `go test ./internal/persist/...` passes, including `-race`
