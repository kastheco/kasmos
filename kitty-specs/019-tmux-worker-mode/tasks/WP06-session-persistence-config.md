---
work_package_id: "WP06"
subtasks:
  - "T032"
  - "T033"
  - "T034"
  - "T035"
  - "T036"
  - "T037"
title: "Session Persistence, Config & Reattach"
phase: "Phase 3 - Persistence & Config"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP05"]
history:
  - timestamp: "2026-02-19T03:53:34Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP06 - Session Persistence, Config & Reattach

## Important: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you begin addressing feedback, update `review_status: acknowledged`.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** - Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP06 --base WP05
```

Depends on WP05 (full tmux TUI integration must work before persistence makes sense).

---

## Objectives & Success Criteria

1. **Session metadata**: `BackendMode` field in `SessionState` records whether the session used subprocess or tmux backend.
2. **Config setting**: `TmuxMode` bool in `Config` allows users to set tmux as the default mode.
3. **Reattach inference**: `kasmos --attach` reads `BackendMode` from the session file and auto-selects the correct backend (FR-013).
4. **Pane reconnection**: During reattach with tmux mode, kasmos calls `Reconnect()` to rediscover surviving worker panes (FR-012, FR-013).
5. **Config-based activation**: When `tmux_mode = true` in config and `$TMUX` is set, tmux mode activates without `--tmux` flag (FR-002). When `$TMUX` is not set, falls back to subprocess with a notice (FR-004).
6. **SC-003**: Reattach reconnects to all surviving workers within 5 seconds.

**Requirements covered**: FR-002, FR-004, FR-012, FR-013.

## Context & Constraints

- **Existing SessionState**: `internal/persist/schema.go` - JSON serialized, currently has Version, SessionID, StartedAt, Workers, etc.
- **Existing Config**: `internal/config/config.go` - TOML serialized, currently has DefaultTaskSource and Agents.
- **Existing reattach flow**: `cmd/kasmos/main.go` - loads session, checks PID, restores workers, resets counter.
- **Backward compatibility**: Existing sessions without `BackendMode` should default to "subprocess".
- **Config backward compatibility**: Existing configs without `tmux_mode` should default to `false`.

**Key reference files**:
- `internal/persist/schema.go` - SessionState, WorkerSnapshot
- `internal/config/config.go` - Config struct, Load, Save
- `cmd/kasmos/main.go` - reattach logic, backend selection
- `internal/tui/model.go` - buildSessionState()

---

## Subtasks & Detailed Guidance

### Subtask T032 - Add BackendMode to SessionState

**Purpose**: Record the backend mode in the session file so reattach can infer which backend to use.

**Steps**:
1. In `internal/persist/schema.go`, add field to `SessionState`:

```go
type SessionState struct {
    Version       int               `json:"version"`
    SessionID     string            `json:"session_id"`
    StartedAt     time.Time         `json:"started_at"`
    FinishedAt    *time.Time        `json:"finished_at,omitempty"`
    TaskSource    *TaskSourceConfig `json:"task_source,omitempty"`
    Workers       []WorkerSnapshot  `json:"workers"`
    NextWorkerNum int64             `json:"next_worker_num"`
    PID           int               `json:"pid"`
    BackendMode   string            `json:"backend_mode,omitempty"` // NEW: "subprocess" or "tmux"
}
```

2. The `omitempty` tag ensures backward compatibility: existing sessions without this field will deserialize to empty string, treated as "subprocess".

**Files**: `internal/persist/schema.go` (modify, ~1 line added)
**Parallel?**: Yes - independent from T033.

---

### Subtask T033 - Add TmuxMode to Config

**Purpose**: Allow users to set tmux mode as their default via configuration file.

**Steps**:
1. In `internal/config/config.go`, add field to `Config`:

```go
type Config struct {
    DefaultTaskSource string                 `toml:"default_task_source"`
    TmuxMode          bool                   `toml:"tmux_mode"` // NEW
    Agents            map[string]AgentConfig `toml:"agents"`
}
```

2. The default value for `bool` in Go is `false`, which matches the desired default (subprocess mode).

3. `DefaultConfig()` doesn't need changes - `TmuxMode: false` is the zero value.

4. Example config.toml entry users would add:
```toml
# Enable tmux worker mode by default (requires running inside tmux)
tmux_mode = true
```

**Files**: `internal/config/config.go` (modify, ~1 line added to struct)
**Parallel?**: Yes - independent from T032.

---

### Subtask T034 - Update buildSessionState to include BackendMode

**Purpose**: When the session state is persisted, include the backend mode so it's available on reattach.

**Steps**:
1. In `internal/tui/model.go`, update `buildSessionState()`:

```go
func (m *Model) buildSessionState() persist.SessionState {
    workers := m.manager.All()
    snapshots := make([]persist.WorkerSnapshot, 0, len(workers))
    for _, w := range workers {
        snapshots = append(snapshots, persist.WorkerToSnapshot(w))
    }

    var ts *persist.TaskSourceConfig
    if m.taskSource != nil && m.taskSource.Type() != "yolo" {
        ts = &persist.TaskSourceConfig{
            Type: m.taskSource.Type(),
            Path: m.taskSource.Path(),
        }
    }

    return persist.SessionState{
        Version:       1,
        SessionID:     m.sessionID,
        StartedAt:     m.sessionStartedAt,
        TaskSource:    ts,
        Workers:       snapshots,
        NextWorkerNum: m.manager.Counter(),
        PID:           os.Getpid(),
        BackendMode:   m.backend.Name(), // NEW: "subprocess" or "tmux"
    }
}
```

2. `m.backend.Name()` returns `"subprocess"` or `"tmux"` depending on the backend type. This is set at session creation and never changes mid-session.

**Files**: `internal/tui/model.go` (modify, ~1 line added)
**Parallel?**: No - depends on T032 (field must exist in SessionState).

---

### Subtask T035 - Update reattach logic to read BackendMode

**Purpose**: When `kasmos --attach` is used, read the `BackendMode` from the session file and auto-select the correct backend. The user does not need to pass `--tmux` again (FR-013).

**Steps**:
1. In `cmd/kasmos/main.go`, in the `attach` block, after loading the session state:

```go
if attach {
    state, err := persister.Load()
    if err != nil {
        // ... existing error handling ...
    }
    if persist.IsPIDAlive(state.PID) {
        return fmt.Errorf("session already active (PID %d)", state.PID)
    }

    // NEW: Infer backend mode from session metadata
    if state.BackendMode == "tmux" && !tmuxMode {
        // Session was tmux mode - auto-enable if we're in tmux
        if os.Getenv("TMUX") != "" {
            tmuxMode = true
        } else {
            log.Printf("notice: session used tmux mode but not in tmux session. Restoring as subprocess mode.")
        }
    }

    // ... existing worker restoration ...
}
```

2. The backend selection logic (from WP03 T016) runs AFTER the attach block, so setting `tmuxMode = true` here will cause the correct backend to be created.

3. **Important ordering**: The attach block must run before backend creation. Current code flow:
   - Parse flags
   - Load config
   - Load task source
   - Backend creation <- must be after attach inference
   - Session persister setup <- must be before attach
   
   Verify this ordering is maintained. The current code creates the backend before the attach block. This needs to be reordered:

```go
// REORDER: Create persister and handle attach BEFORE backend creation
persister := persist.NewSessionPersister(".")
sessionID := persist.NewSessionID()

if attach {
    state, err := persister.Load()
    // ... existing load logic ...
    // ... NEW backend mode inference ...
    sessionID = state.SessionID
    // ... existing worker restoration will happen after model creation ...
}

// NOW create backend (tmuxMode may have been set by attach inference)
var backend worker.WorkerBackend
if tmuxMode {
    // ... tmux backend creation ...
} else {
    // ... subprocess backend creation ...
}
```

4. Store the loaded session state for later use in worker restoration:

```go
var attachState *persist.SessionState
if attach {
    state, err := persister.Load()
    // ...
    attachState = state
    sessionID = state.SessionID
}

// ... backend creation ...

model := tui.NewModel(backend, source, version, cfg, showLauncher)

// Restore workers from attach state
if attachState != nil {
    for _, snap := range attachState.Workers {
        w := persist.SnapshotToWorker(snap)
        if w.State == worker.StateRunning || w.State == worker.StateSpawning {
            w.State = worker.StateKilled
            w.ExitedAt = time.Now()
        }
        model.RestoreWorker(w)
    }
    model.ResetWorkerCounter(attachState.NextWorkerNum)
    model.SetSessionStartedAt(attachState.StartedAt)
}
```

**Files**: `cmd/kasmos/main.go` (modify, ~30 lines refactored)
**Parallel?**: No - depends on T032 field and backend creation reorder.

---

### Subtask T036 - Config-based tmux activation

**Purpose**: When `cfg.TmuxMode` is true and `$TMUX` is set, activate tmux mode without requiring the `--tmux` flag (FR-002). When `$TMUX` is not set, fall back to subprocess with a notice (FR-004).

**Steps**:
1. In `cmd/kasmos/main.go`, after loading config but before backend creation, add config-based activation:

```go
// Config-based tmux activation (FR-002, FR-004)
// Priority: --tmux flag > cfg.TmuxMode > default (subprocess)
if !tmuxMode && cfg.TmuxMode {
    if os.Getenv("TMUX") != "" {
        tmuxMode = true
        log.Printf("info: tmux mode activated from config")
    } else {
        log.Printf("notice: tmux_mode configured but not in tmux session, using subprocess mode")
    }
}
```

2. Place this AFTER the attach inference (T035) and BEFORE backend creation.

3. The priority chain is now:
   - `--tmux` flag explicitly set -> use tmux
   - Attach session was tmux + currently in tmux -> use tmux (T035)
   - Config `tmux_mode = true` + currently in tmux -> use tmux (this)
   - Otherwise -> subprocess

**Files**: `cmd/kasmos/main.go` (modify, ~8 lines added)
**Parallel?**: No - must be placed correctly in the initialization order.

---

### Subtask T037 - Reattach pane reconnection

**Purpose**: When reattaching to a tmux session, call `TmuxBackend.Reconnect()` to discover surviving worker panes and update their state. Workers that survived the kasmos crash get reconnected to their panes.

**Steps**:
1. After backend creation and model creation, if in tmux mode and attaching:

```go
if tmuxMode && attachState != nil && tmuxBackend != nil {
    // Reconnect to surviving tmux panes
    reconnected, err := tmuxBackend.Reconnect(sessionID)
    if err != nil {
        log.Printf("warning: tmux reconnect failed: %v", err)
    } else {
        // Update worker states based on reconnected panes
        for _, rw := range reconnected {
            w := model.FindWorker(rw.WorkerID) // Need accessor
            if w == nil {
                continue
            }
            if rw.Dead {
                w.State = worker.StateExited
                w.ExitCode = rw.ExitCode
                w.ExitedAt = time.Now()
            } else {
                // Worker is still running! Create a new handle for it
                w.State = worker.StateRunning
                w.Handle = tmuxBackend.Handle(rw.WorkerID, w.SpawnedAt)
            }
        }
    }
}
```

2. The Model needs a `FindWorker(id)` method or we can iterate through `attachState.Workers`:

```go
// In internal/tui/model.go:
func (m *Model) FindWorker(id string) *worker.Worker {
    return m.manager.Get(id)
}
```

Or use the existing `m.manager.Get(id)` directly if accessible from main.go. Since model is a `*tui.Model`, we need the accessor.

3. For workers that survived (still running):
   - Their state stays `StateRunning`
   - They get a new `tmuxHandle` via `tmuxBackend.Handle()`
   - The handle's `exitCh` is not closed, so `Wait()` will block until the pane exits
   - Tick polling will detect their eventual exit

4. For workers that died while kasmos was down:
   - Mark as `StateExited` or `StateFailed` based on exit code
   - Their terminal content is still visible in the dead tmux pane (scrollable)

5. **Important**: Don't mark surviving workers as `StateKilled` during the standard attach restoration. The current attach code marks all Running workers as Killed:

```go
if w.State == worker.StateRunning || w.State == worker.StateSpawning {
    w.State = worker.StateKilled
    w.ExitedAt = time.Now()
}
```

In tmux mode, skip this for workers that have surviving panes:

```go
// Modified attach restoration:
survivingWorkerIDs := make(map[string]bool)
if tmuxMode && reconnected != nil {
    for _, rw := range reconnected {
        if !rw.Dead {
            survivingWorkerIDs[rw.WorkerID] = true
        }
    }
}

for _, snap := range attachState.Workers {
    w := persist.SnapshotToWorker(snap)
    if (w.State == worker.StateRunning || w.State == worker.StateSpawning) && !survivingWorkerIDs[w.ID] {
        w.State = worker.StateKilled
        w.ExitedAt = time.Now()
    }
    model.RestoreWorker(w)
}
```

**Files**:
- `cmd/kasmos/main.go` (modify, ~40 lines)
- `internal/tui/model.go` (modify, ~5 lines - FindWorker accessor)
**Parallel?**: No - depends on T035 reorder and tmux backend creation.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Stale session file from subprocess mode | Wrong backend selected on attach | Check BackendMode field; empty/missing defaults to subprocess. |
| Reconnect finds no surviving panes | Workers shown as dead | Standard orphan recovery applies - workers marked as killed. |
| Config change between sessions | Backend mismatch on attach | Session BackendMode takes priority over config on attach. |
| Backend creation reorder breaks existing flow | Attach fails | Test both `kasmos --attach` with and without tmux mode. Verify subprocess-only path unchanged. |
| Multiple kasmos instances in same tmux session | Pane tag conflicts | Session tag is unique per kasmos instance. Only manage panes with matching tag. |

## Review Guidance

- Verify `BackendMode` defaults to empty string (backward compatible with existing sessions).
- Verify `TmuxMode` defaults to `false` (backward compatible with existing configs).
- Verify the initialization order: parse flags -> load config -> persister setup -> attach inference -> config inference -> backend creation -> model creation -> worker restoration.
- Verify surviving workers are NOT marked as killed during attach.
- Verify `kasmos --attach` works correctly for both subprocess and tmux sessions.
- Verify `kasmos --attach` for a tmux session outside tmux degrades gracefully.
- Run `go build ./cmd/kasmos` to verify compilation.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
