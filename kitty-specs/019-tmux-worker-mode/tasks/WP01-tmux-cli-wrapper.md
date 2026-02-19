---
work_package_id: "WP01"
subtasks:
  - "T001"
  - "T002"
  - "T003"
  - "T004"
  - "T005"
  - "T006"
  - "T041"
title: "TmuxCLI Wrapper Interface"
phase: "Phase 1 - TmuxBackend Core"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: []
history:
  - timestamp: "2026-02-19T03:53:34Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP01 - TmuxCLI Wrapper Interface

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
spec-kitty implement WP01
```

No dependencies - this WP can start immediately.

---

## Objectives & Success Criteria

1. **Define `TmuxCLI` interface** in `internal/worker/tmux_cli.go` that abstracts all tmux CLI interactions.
2. **Implement `tmuxExec`** as the real implementation using `os/exec.Command("tmux", ...)`.
3. **All 12 interface methods** are implemented and handle errors with descriptive messages.
4. **`PaneInfo` parsing** correctly extracts pane ID, PID, dead status, exit code from tmux format strings.
5. **`go build ./internal/worker/...`** compiles without errors.
6. **Unit tests** with a mock `TmuxCLI` validate interface contract.

**Requirements covered**: FR-015 (WorkerBackend interface foundation), Risk mitigation (tmux CLI format changes).

## Context & Constraints

- **Design Decision**: Direct CLI via `os/exec`, no Go tmux library (see `kitty-specs/019-tmux-worker-mode/research.md` section 1).
- **Minimum tmux version**: 2.6+ (September 2017). Version check is advisory (warn, not error).
- **All tmux commands**: See research.md section 1 table for the exact tmux CLI invocations.
- **Environment tagging**: Uses `tmux set-environment` / `show-environment` per session scope (research.md section 2).
- **Data model**: See `kitty-specs/019-tmux-worker-mode/data-model.md` for TmuxCLI interface and PaneInfo definitions.
- **Constitution**: Follow Go conventions (`internal/` packages, explicit error handling with `fmt.Errorf` wrapping).

**Key reference files**:
- `internal/worker/backend.go` - Existing WorkerBackend interface pattern
- `internal/worker/subprocess.go` - Existing backend implementation pattern (follow similar structure)
- `kitty-specs/019-tmux-worker-mode/research.md` - tmux CLI commands and format strings
- `kitty-specs/019-tmux-worker-mode/data-model.md` - Entity definitions

---

## Subtasks & Detailed Guidance

### Subtask T001 - Define TmuxCLI interface, PaneInfo struct, and error types

**Purpose**: Establish the contract for all tmux CLI interactions. This interface enables mock injection for unit tests and keeps the real `os/exec` calls isolated.

**Steps**:
1. Create `internal/worker/tmux_cli.go`.
2. Define the `TmuxCLI` interface with all 12 methods (from data-model.md):

```go
// TmuxCLI abstracts tmux CLI interactions for testability.
// Real implementation: tmuxExec (os/exec). Test implementation: mock.
type TmuxCLI interface {
    // Pane lifecycle
    SplitWindow(target, cmd string, horizontal bool, size int) (paneID string, err error)
    KillPane(paneID string) error
    SelectPane(paneID string) error

    // Pane movement
    JoinPane(src, dst string, horizontal bool, size int) error
    BreakPane(paneID string) error

    // Window management
    NewWindow(name string) (windowID string, err error)

    // Pane queries
    ListPanes(target string) ([]PaneInfo, error)
    CapturePane(paneID string) (content string, err error)
    CurrentPaneID() (paneID string, err error)
    Version() (string, error)

    // Environment tagging
    SetPaneEnv(paneID, key, value string) error
    GetPaneEnv(paneID, key string) (value string, err error)
}
```

3. Define the `PaneInfo` value object:

```go
// PaneInfo represents parsed output from tmux list-panes.
type PaneInfo struct {
    ID         string // tmux pane identifier (e.g., "%42")
    PID        int    // process running in the pane
    Dead       bool   // true if the pane process has exited
    DeadStatus int    // exit code of the dead process
}
```

Note: `WorkerID` and `SessionTag` fields from the data model are derived at a higher level (TmuxBackend reads env vars), not from `list-panes` output directly. Keep `PaneInfo` focused on what `list-panes -F` returns.

4. Define a sentinel error for tmux-not-found:

```go
var ErrTmuxNotFound = errors.New("tmux binary not found in PATH")
```

**Files**: `internal/worker/tmux_cli.go` (new, ~60 lines)
**Parallel?**: No - other subtasks depend on these types.

---

### Subtask T002 - Implement tmuxExec base struct with command execution helper

**Purpose**: Create the real `TmuxCLI` implementation that shells out to `tmux`. Establish the base command execution pattern used by all methods.

**Steps**:
1. In the same file or a section below the interface, define `tmuxExec`:

```go
type tmuxExec struct {
    bin string // resolved path to tmux binary
}
```

2. Create constructor:

```go
// NewTmuxExec creates a tmuxExec that shells out to the tmux binary.
// Returns ErrTmuxNotFound if tmux is not in PATH.
func NewTmuxExec() (*tmuxExec, error) {
    bin, err := exec.LookPath("tmux")
    if err != nil {
        return nil, ErrTmuxNotFound
    }
    return &tmuxExec{bin: bin}, nil
}
```

3. Add a private helper for running tmux commands:

```go
// run executes a tmux command and returns stdout. Stderr is captured in error messages.
func (t *tmuxExec) run(args ...string) (string, error) {
    cmd := exec.Command(t.bin, args...)
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    if err := cmd.Run(); err != nil {
        return "", fmt.Errorf("tmux %s: %w (stderr: %s)", args[0], err, strings.TrimSpace(stderr.String()))
    }
    return strings.TrimSpace(stdout.String()), nil
}
```

4. Add compile-time interface satisfaction check:

```go
var _ TmuxCLI = (*tmuxExec)(nil)
```

**Files**: `internal/worker/tmux_cli.go` (~40 lines added)
**Parallel?**: No - other methods build on this.

---

### Subtask T003 - Implement pane lifecycle methods: SplitWindow, KillPane, SelectPane

**Purpose**: These methods create, destroy, and focus tmux panes. They are the building blocks for worker spawning and focus management.

**Steps**:

1. **SplitWindow**: Creates a new pane by splitting an existing one.

```go
func (t *tmuxExec) SplitWindow(target, cmd string, horizontal bool, size int) (string, error) {
    args := []string{"split-window"}
    if horizontal {
        args = append(args, "-h")
    }
    if target != "" {
        args = append(args, "-t", target)
    }
    if size > 0 {
        args = append(args, "-l", fmt.Sprintf("%d%%", size))
    }
    args = append(args, "-P", "-F", "#{pane_id}")
    if cmd != "" {
        args = append(args, cmd)
    }
    return t.run(args...)
}
```

The `-P` flag prints the new pane info, `-F '#{pane_id}'` formats as just the pane ID (e.g., `%42`).

2. **KillPane**: Terminates a pane and its process.

```go
func (t *tmuxExec) KillPane(paneID string) error {
    _, err := t.run("kill-pane", "-t", paneID)
    return err
}
```

3. **SelectPane**: Moves keyboard focus to a pane.

```go
func (t *tmuxExec) SelectPane(paneID string) error {
    _, err := t.run("select-pane", "-t", paneID)
    return err
}
```

**Files**: `internal/worker/tmux_cli.go` (~40 lines added)
**Parallel?**: Yes - can be implemented alongside T004.

---

### Subtask T004 - Implement pane movement methods: JoinPane, BreakPane, NewWindow

**Purpose**: These methods move panes between the visible kasmos window and the hidden parking window. Core to the pane swap mechanism (research.md section 4).

**Steps**:

1. **JoinPane**: Brings a pane from one location to another (parking -> kasmos window).

```go
func (t *tmuxExec) JoinPane(src, dst string, horizontal bool, size int) error {
    args := []string{"join-pane", "-s", src, "-t", dst}
    if horizontal {
        args = append(args, "-h")
    }
    if size > 0 {
        args = append(args, "-l", fmt.Sprintf("%d%%", size))
    }
    _, err := t.run(args...)
    return err
}
```

2. **BreakPane**: Moves a pane to its own window (kasmos window -> parking). The `-d` flag prevents switching to the new window.

```go
func (t *tmuxExec) BreakPane(paneID string) error {
    // -d: don't switch to the new window
    // -s: source pane
    _, err := t.run("break-pane", "-d", "-s", paneID)
    return err
}
```

Note: `break-pane` moves the pane to a new window. For parking, we need to move it to the *existing* parking window. Actually, `break-pane` creates a new window by default. To move to parking, we use `join-pane` in reverse or `move-pane`. Let me reconsider.

**Correction**: For the parking mechanism:
- **Hide a pane**: `tmux join-pane -s <visible-pane> -t <parking-window>` (moves pane TO parking)
- **Show a pane**: `tmux join-pane -s <parking-window>.<pane> -t <kasmos-window> -h -l 50%` (moves pane FROM parking)

So `BreakPane` should move the pane to the parking window, not create a new window:

```go
func (t *tmuxExec) BreakPane(paneID string) error {
    // break-pane with -d keeps the focus on the source window
    _, err := t.run("break-pane", "-d", "-s", paneID)
    return err
}
```

Actually, `break-pane` always creates a new window. The TmuxBackend at a higher level will handle the parking window logic using `join-pane` for both show and hide. Keep `BreakPane` as a thin wrapper. The higher-level `HidePane`/`ShowPane` in WP02 will compose these primitives correctly.

3. **NewWindow**: Creates a new tmux window (used for the parking window).

```go
func (t *tmuxExec) NewWindow(name string) (string, error) {
    args := []string{"new-window", "-d", "-P", "-F", "#{window_id}"}
    if name != "" {
        args = append(args, "-n", name)
    }
    return t.run(args...)
}
```

`-d` prevents switching to the new window. `-P -F '#{window_id}'` prints the window ID.

**Files**: `internal/worker/tmux_cli.go` (~40 lines added)
**Parallel?**: Yes - can be implemented alongside T003.

---

### Subtask T005 - Implement pane query methods: ListPanes, CapturePane, CurrentPaneID, Version

**Purpose**: Query methods for pane status polling (exit detection), content capture (session ID extraction), self-identification, and version checking.

**Steps**:

1. **ListPanes**: Lists panes in a target window with structured format output.

```go
func (t *tmuxExec) ListPanes(target string) ([]PaneInfo, error) {
    format := "#{pane_id} #{pane_pid} #{pane_dead} #{pane_dead_status}"
    args := []string{"list-panes", "-F", format}
    if target != "" {
        args = append(args, "-t", target)
    }
    output, err := t.run(args...)
    if err != nil {
        return nil, err
    }
    return parsePaneList(output)
}
```

2. **Implement `parsePaneList` helper**:

```go
func parsePaneList(output string) ([]PaneInfo, error) {
    if output == "" {
        return nil, nil
    }
    var panes []PaneInfo
    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        // Format: "%42 12345 0 0" or "%42 12345 1 127"
        parts := strings.Fields(line)
        if len(parts) < 4 {
            continue // skip malformed lines
        }
        pid, _ := strconv.Atoi(parts[1])
        dead := parts[2] == "1"
        deadStatus, _ := strconv.Atoi(parts[3])
        panes = append(panes, PaneInfo{
            ID:         parts[0],
            PID:        pid,
            Dead:       dead,
            DeadStatus: deadStatus,
        })
    }
    return panes, nil
}
```

3. **CapturePane**: Captures the full scrollback content of a pane.

```go
func (t *tmuxExec) CapturePane(paneID string) (string, error) {
    // -p: output to stdout (not paste buffer)
    // -S -: start from beginning of scrollback history
    return t.run("capture-pane", "-p", "-t", paneID, "-S", "-")
}
```

4. **CurrentPaneID**: Gets the pane ID of the current (kasmos) process.

```go
func (t *tmuxExec) CurrentPaneID() (string, error) {
    return t.run("display-message", "-p", "#{pane_id}")
}
```

5. **Version**: Gets the tmux version string.

```go
func (t *tmuxExec) Version() (string, error) {
    return t.run("-V")
}
```

**Files**: `internal/worker/tmux_cli.go` (~80 lines added)
**Parallel?**: No - `parsePaneList` is a key helper that should be implemented carefully.

**Edge Cases**:
- Empty target (no panes): return empty slice, nil error.
- Malformed lines in list-panes output: skip with warning (don't fail).
- `capture-pane` on dead pane: tmux handles this - content is still in the pane buffer.

---

### Subtask T006 - Implement environment tagging methods: SetPaneEnv, GetPaneEnv

**Purpose**: Tag managed panes with kasmos session ID and worker ID for rediscovery after kasmos restart. Uses tmux session-level environment variables (research.md section 2).

**Steps**:

1. **SetPaneEnv**: Sets an environment variable on a tmux session/pane scope.

```go
func (t *tmuxExec) SetPaneEnv(paneID, key, value string) error {
    // set-environment with -t targets the session of the specified pane
    _, err := t.run("set-environment", "-t", paneID, key, value)
    return err
}
```

Note: tmux `set-environment` sets variables at the session level, not per-pane. For per-pane tagging, we use a naming convention: `KASMOS_WORKER_<paneID>=<workerID>`. The TmuxBackend (WP02) will handle the naming convention; this method is the raw primitive.

Actually, looking more carefully at the research: tmux 3.0+ has `set-option -p` for per-pane options, but we're targeting 2.6+ compatibility. The research says to use session-level env vars with a naming scheme. Let me adjust:

```go
func (t *tmuxExec) SetPaneEnv(paneID, key, value string) error {
    // Uses session-level environment with pane-specific key naming.
    // The caller is responsible for the naming convention (e.g., KASMOS_WORKER_%42=w-001).
    _, err := t.run("set-environment", key, value)
    return err
}

func (t *tmuxExec) GetPaneEnv(paneID, key string) (string, error) {
    output, err := t.run("show-environment", key)
    if err != nil {
        return "", err
    }
    // Output format: "KEY=VALUE" or "KEY" (if unset, with -u flag)
    if idx := strings.IndexByte(output, '='); idx >= 0 {
        return output[idx+1:], nil
    }
    return "", fmt.Errorf("environment variable %q not set", key)
}
```

Wait, the data model says the tagging scheme is:
- `KASMOS_SESSION=<session-id>`
- `KASMOS_WORKER=<worker-id>`

But these are session-level, not per-pane. For multiple workers, we need per-pane differentiation. The TmuxBackend will use a compound key like `KASMOS_PANE_<pane-id>=<worker-id>` or track the mapping internally. The CLI wrapper just provides the raw set/get primitives.

Keep the implementation simple - the TmuxBackend composes the tagging logic:

```go
func (t *tmuxExec) SetPaneEnv(paneID, key, value string) error {
    _, err := t.run("set-environment", key, value)
    return err
}

func (t *tmuxExec) GetPaneEnv(paneID, key string) (string, error) {
    output, err := t.run("show-environment", key)
    if err != nil {
        return "", err
    }
    if idx := strings.IndexByte(output, '='); idx >= 0 {
        return output[idx+1:], nil
    }
    return "", fmt.Errorf("tmux environment variable %q not found", key)
}
```

2. **Add imports** at the top of the file: `bytes`, `errors`, `fmt`, `os/exec`, `strconv`, `strings`.

**Files**: `internal/worker/tmux_cli.go` (~30 lines added)
**Parallel?**: No (final subtask, depends on T002 for `run` helper).

**Total file estimate**: `internal/worker/tmux_cli.go` should be approximately 250-300 lines.

---

### Subtask T041 - Unit tests for TmuxCLI

**Purpose**: Validate the interface contract, parsing logic, and error handling. Uses a mock `TmuxCLI` implementation and direct tests of `parsePaneList`.

**Steps**:

1. Create `internal/worker/tmux_cli_test.go`.

2. **Define a mock TmuxCLI** for use in tests across WP01 and WP02:

```go
type mockTmuxCLI struct {
    splitWindowFn  func(target, cmd string, horizontal bool, size int) (string, error)
    joinPaneFn     func(src, dst string, horizontal bool, size int) error
    breakPaneFn    func(paneID string) error
    selectPaneFn   func(paneID string) error
    listPanesFn    func(target string) ([]PaneInfo, error)
    killPaneFn     func(paneID string) error
    capturePane Fn func(paneID string) (string, error)
    setPaneEnvFn   func(paneID, key, value string) error
    getPaneEnvFn   func(paneID, key string) (string, error)
    newWindowFn    func(name string) (string, error)
    currentPaneIDFn func() (string, error)
    versionFn      func() (string, error)
}
```

Each method delegates to the corresponding `Fn` field, returning zero values if nil. This pattern lets individual tests override only the methods they care about.

3. **Test `parsePaneList`** with table-driven tests:

```go
func TestParsePaneList(t *testing.T) {
    tests := []struct {
        name   string
        input  string
        want   []PaneInfo
    }{
        {"empty", "", nil},
        {"single live pane", "%1 12345 0 0", []PaneInfo{{ID: "%1", PID: 12345, Dead: false, DeadStatus: 0}}},
        {"single dead pane", "%2 99 1 127", []PaneInfo{{ID: "%2", PID: 99, Dead: true, DeadStatus: 127}}},
        {"multiple panes", "%1 100 0 0\n%2 200 1 1", []PaneInfo{...}},
        {"malformed line skipped", "%1 100 0 0\nbad\n%3 300 0 0", []PaneInfo{...}},
        {"trailing whitespace", "  %1 100 0 0  \n", []PaneInfo{...}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := parsePaneList(tt.input)
            // assert err == nil, got matches tt.want
        })
    }
}
```

4. **Test `NewTmuxExec`**: Verify it returns `ErrTmuxNotFound` when tmux is not in PATH (set PATH to empty in test, or use a build tag).

5. **Test `GetPaneEnv` parsing**: Verify `KEY=VALUE` output is split correctly, and missing key returns error.

**Files**: `internal/worker/tmux_cli_test.go` (new, ~120 lines)
**Parallel?**: Yes - can proceed once T001-T006 are done.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| tmux output format changes between versions | PaneInfo parsing breaks | Pin to documented format strings (`#{pane_id}` etc.). These have been stable since tmux 2.6. |
| `os/exec` errors are opaque | Hard to debug failures | Wrap every error with the tmux command and stderr output. |
| tmux not installed | Feature unusable | `ErrTmuxNotFound` sentinel allows callers to provide clear user guidance. |
| `list-panes` output has unexpected whitespace | Parsing fails silently | Use `strings.Fields()` for robust whitespace splitting. Skip malformed lines. |

## Review Guidance

- Verify every `TmuxCLI` method signature matches the data model definition.
- Verify `parsePaneList` handles edge cases: empty output, single pane, malformed lines.
- Verify all `tmuxExec` methods include descriptive error wrapping.
- Verify the `run()` helper captures stderr for debugging.
- Verify `var _ TmuxCLI = (*tmuxExec)(nil)` compile-time check is present.
- Verify no `context.Context` is needed at this layer (tmux commands are fast, <100ms).

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
