---
work_package_id: "WP03"
subtasks:
  - "T015"
  - "T016"
  - "T017"
  - "T018"
  - "T019"
title: "CLI Flag & Backend Selection"
phase: "Phase 2 - TUI Integration"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP02"]
history:
  - timestamp: "2026-02-19T03:53:34Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP03 - CLI Flag & Backend Selection

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
spec-kitty implement WP03 --base WP02
```

Depends on WP02 (TmuxBackend must be implemented).

---

## Objectives & Success Criteria

1. **`--tmux` flag** is available on the `kasmos` command.
2. **Backend selection**: When `--tmux` is set and `$TMUX` is present, `TmuxBackend` is created instead of `SubprocessBackend`.
3. **Mutual exclusivity**: `kasmos --tmux -d` produces a clear error (FR-016).
4. **Environment validation**: `kasmos --tmux` outside tmux produces a clear error with guidance (FR-003).
5. **TUI Model** has tmux mode state fields ready for WP04 to wire up.
6. **Existing behavior unchanged**: Running `kasmos` without `--tmux` uses SubprocessBackend as before.

**Requirements covered**: FR-001, FR-003, FR-016.

## Context & Constraints

- **Existing main.go**: `cmd/kasmos/main.go` - 175 lines. Has `daemon`, `format`, `spawnAll`, `attach` flags.
- **Existing Model**: `internal/tui/model.go` - `NewModel()` takes `backend worker.WorkerBackend`.
- **TmuxBackend.Init()** must be called after creation but before TUI starts (to capture kasmos pane ID).
- **Priority order**: `--tmux` flag > `cfg.TmuxMode` (WP06) > default (subprocess). For WP03, only the flag is implemented. Config-based activation is WP06.
- **Constitution**: Follow cobra patterns already in main.go.

**Key reference files**:
- `cmd/kasmos/main.go` - Current CLI setup
- `internal/tui/model.go` - Model struct and constructor
- `internal/worker/tmux.go` - TmuxBackend (from WP02)
- `internal/worker/tmux_cli.go` - TmuxCLI, NewTmuxExec (from WP01)

---

## Subtasks & Detailed Guidance

### Subtask T015 - Add --tmux flag to cobra command

**Purpose**: Expose the tmux mode option on the CLI. Users activate tmux mode with `kasmos --tmux`.

**Steps**:
1. In `cmd/kasmos/main.go`, add a `tmux` boolean variable alongside existing flag variables:

```go
var showVersion bool
var daemon bool
var format string
var spawnAll bool
var attach bool
var tmuxMode bool  // NEW
```

2. Register the flag at the bottom of `newRootCmd`:

```go
cmd.Flags().BoolVar(&tmuxMode, "tmux", false, "run workers as interactive tmux panes")
```

**Files**: `cmd/kasmos/main.go` (modify, ~3 lines added)
**Parallel?**: No - other subtasks depend on this flag variable.

---

### Subtask T016 - Implement backend selection logic

**Purpose**: When `--tmux` is active and the environment supports it, construct a `TmuxBackend` instead of `SubprocessBackend`. Initialize it before the TUI starts.

**Steps**:
1. Replace the current unconditional `NewSubprocessBackend()` call with conditional backend selection:

```go
// Current code:
// backend, err := worker.NewSubprocessBackend()
// if err != nil {
//     return err
// }

// New code:
var backend worker.WorkerBackend
if tmuxMode {
    // Validate tmux environment
    if os.Getenv("TMUX") == "" {
        return fmt.Errorf("tmux mode requires running inside a tmux session.\n" +
            "Start one with: tmux new-session -s kasmos\n" +
            "Then run: kasmos --tmux")
    }

    // Create TmuxCLI
    cli, err := worker.NewTmuxExec()
    if err != nil {
        return fmt.Errorf("tmux mode: %w", err)
    }

    // Create TmuxBackend
    tmuxBackend, err := worker.NewTmuxBackend(cli)
    if err != nil {
        return err
    }

    // Initialize (captures kasmos pane ID, creates parking window)
    if err := tmuxBackend.Init(sessionID); err != nil {
        return fmt.Errorf("tmux init: %w", err)
    }

    backend = tmuxBackend
} else {
    var err error
    backend, err = worker.NewSubprocessBackend()
    if err != nil {
        return err
    }
}
```

2. **Important ordering**: `sessionID` is created before the backend selection block. The current code already creates `sessionID` before the backend. Verify this is still the case after modifications.

3. Note: `TmuxBackend.Init()` needs `sessionID`. The current code creates it with `persist.NewSessionID()`. This is fine - the session ID is created early.

**Files**: `cmd/kasmos/main.go` (modify, ~25 lines replacing ~4)
**Parallel?**: No - depends on T015 flag variable.

**Edge Cases**:
- `$TMUX` is set but tmux binary is missing: `NewTmuxExec()` returns `ErrTmuxNotFound`. Error message should suggest installing tmux.
- `TmuxBackend.Init()` fails (e.g., can't create parking window): Error before TUI starts, clean exit.

---

### Subtask T017 - Validate --tmux and -d mutual exclusivity

**Purpose**: Tmux mode requires the interactive dashboard. Daemon mode is headless. They cannot coexist (FR-016).

**Steps**:
1. Add validation before backend selection:

```go
// Validate flag combinations
if tmuxMode && daemon {
    return fmt.Errorf("--tmux and -d (daemon mode) are mutually exclusive.\n" +
        "Tmux mode requires the interactive dashboard and cannot run headless.\n" +
        "Use --tmux for interactive agent sessions, or -d for headless batch processing.")
}
```

2. Place this validation early in the `RunE` function, after flag processing but before any resource allocation.

**Files**: `cmd/kasmos/main.go` (modify, ~5 lines added)
**Parallel?**: No - sequential with T016.

---

### Subtask T018 - Add tmux state fields to TUI Model

**Purpose**: The TUI Model needs to know it's in tmux mode and have access to the TmuxBackend for pane operations. These fields are populated by WP03 and used by WP04/WP05.

**Steps**:
1. In `internal/tui/model.go`, add fields to the `Model` struct:

```go
type Model struct {
    // ... existing fields ...

    // Tmux mode state
    tmuxMode    bool                    // true when running with --tmux
    tmuxBackend *worker.TmuxBackend     // nil when not in tmux mode
    tmuxReady   bool                    // true after tmuxInitCmd completes
}
```

2. Note: `kasmosPaneID` and `activePaneID` are tracked by `TmuxBackend` itself (WP02). The TUI Model stores the backend reference, not duplicated state.

**Files**: `internal/tui/model.go` (modify, ~5 lines added to struct)
**Parallel?**: No - T019 depends on these fields.

---

### Subtask T019 - Update NewModel for tmux mode state and status bar indicator

**Purpose**: Allow the Model to be initialized with tmux mode information. Add a `backendName()` helper. Show backend mode in the status bar alongside (not replacing) the task source mode.

**IMPORTANT**: Do NOT modify `modeName()`. It must continue returning the task source type (yolo/spec-kitty/gsd). Tmux mode is orthogonal to task source -- both must be visible.

**Steps**:
1. Add a `SetTmuxMode` method:

```go
// SetTmuxMode configures the model for tmux worker mode.
// Must be called before Init().
func (m *Model) SetTmuxMode(tmuxBackend *worker.TmuxBackend) {
    m.tmuxMode = true
    m.tmuxBackend = tmuxBackend
}
```

2. Add a `backendName()` helper (separate from `modeName()`):

```go
func (m *Model) backendName() string {
    return m.backend.Name() // "subprocess" or "tmux"
}
```

3. Update the status bar rendering in `internal/tui/panels.go` to show the backend indicator when in tmux mode. Find the existing mode rendering:

```go
// Current (panels.go):
mode := modeIndicatorStyle.Render(" mode: " + m.modeName() + " ")

// New:
modeText := m.modeName()
if m.tmuxMode {
    modeText += " [tmux]"
}
mode := modeIndicatorStyle.Render(" mode: " + modeText + " ")
```

4. In `cmd/kasmos/main.go`, after creating the model and before `program.Run()`, call `SetTmuxMode` if in tmux mode:

```go
model := tui.NewModel(backend, source, version, cfg, showLauncher)
if tmuxMode {
    if tmuxBackend, ok := backend.(*worker.TmuxBackend); ok {
        model.SetTmuxMode(tmuxBackend)
    }
}
```

5. The existing `NewModel` signature doesn't need to change - the backend is already passed as `worker.WorkerBackend`. The TmuxBackend-specific reference is set via `SetTmuxMode`.

**Files**:
- `internal/tui/model.go` (modify, ~15 lines added)
- `cmd/kasmos/main.go` (modify, ~5 lines added)
**Parallel?**: No - depends on T018.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Backend initialization order | TUI crashes if Init() fails | Init() runs before TUI program starts. Errors are returned to cobra, printed cleanly. |
| SessionID not available for Init() | Tagging fails | SessionID is created before backend selection block. Verified in implementation. |
| Type assertion for SetTmuxMode | Runtime panic if wrong backend type | Guard with `ok` check on type assertion. Only call SetTmuxMode when tmuxMode flag is true. |

## Review Guidance

- Verify `--tmux` flag is registered correctly in cobra.
- Verify `--tmux -d` produces the error immediately (before any resource allocation).
- Verify `--tmux` outside tmux produces a descriptive error with guidance.
- Verify existing behavior without `--tmux` is completely unchanged.
- Verify `TmuxBackend.Init()` is called before `tea.NewProgram()`.
- Verify `SetTmuxMode` is called only when tmux mode is active.
- Run `go build ./cmd/kasmos` to verify compilation.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
