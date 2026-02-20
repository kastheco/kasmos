---
work_package_id: WP01
title: TmuxCLI Wrapper Interface
lane: "doing"
dependencies: []
base_branch: main
base_commit: 91b2d8ef5f8f8a8f80f29407e2644b1aa7bc9a85
created_at: '2026-02-20T03:48:00.836862+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
- T041
phase: Phase 1 - TmuxBackend Core
assignee: ''
agent: ''
shell_pid: "3525140"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-19T03:53:34Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
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
2. **Implement `tmuxExec`** as the real implementation using `os/exec.CommandContext("tmux", ...)`.
3. **All interface methods** are implemented and handle errors with descriptive messages.
4. **`PaneInfo` parsing** correctly extracts pane ID, PID, dead status, exit code from tmux format strings.
5. **`go build ./internal/worker/...`** compiles without errors.
6. **Unit tests** with a mock `TmuxCLI` validate interface contract.

**Requirements covered**: FR-015 (WorkerBackend interface foundation), Risk mitigation (tmux CLI format changes).

## Context & Constraints

- **Design Decision**: Direct CLI via `os/exec`, no Go tmux library (see `kitty-specs/019-tmux-worker-mode/research.md` section 1).
- **Minimum tmux version**: 2.6+ (September 2017). Version check is advisory (warn, not error).
- **All tmux commands**: See research.md section 1 table for the exact tmux CLI invocations.
- **Environment tagging**: Uses `tmux set-environment` / `show-environment` with `KASMOS_PANE_<worker_id>=<pane_id>` keys and `KASMOS_*` session metadata tags (research.md section 2).
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
2. Define the `TmuxCLI` interface (from data-model.md):

```go
// TmuxCLI abstracts tmux CLI interactions for testability.
// All methods accept context.Context for timeout/cancellation.
// Real implementation: tmuxExec (os/exec). Test implementation: mock.
type TmuxCLI interface {
    // Pane lifecycle
    SplitWindow(ctx context.Context, opts SplitOpts) (paneID string, err error)
    KillPane(ctx context.Context, paneID string) error
    SelectPane(ctx context.Context, paneID string) error

    // Pane movement (used for both parking and showing)
    JoinPane(ctx context.Context, opts JoinOpts) error

    // Window management
    NewWindow(ctx context.Context, opts NewWindowOpts) (windowID string, err error)

    // Introspection
    ListPanes(ctx context.Context, target string) ([]PaneInfo, error)
    DisplayMessage(ctx context.Context, format string) (string, error)
    CapturePane(ctx context.Context, paneID string) (content string, err error)
    Version(ctx context.Context) (string, error)

    // Environment tagging (session-scope)
    SetEnvironment(ctx context.Context, key, value string) error
    ShowEnvironment(ctx context.Context) (map[string]string, error)
    UnsetEnvironment(ctx context.Context, key string) error

    // Per-pane options
    SetPaneOption(ctx context.Context, paneID, key, value string) error
}

// SplitOpts configures split-window.
type SplitOpts struct {
    Target     string   // pane/window to split from
    Horizontal bool     // -h flag
    Size       string   // -l flag: "50%" or "80"
    Command    []string // command to run in new pane
    Env        []string // environment variables as "KEY=VALUE" for -e flags
}

// JoinOpts configures join-pane (used for both parking and showing).
type JoinOpts struct {
    Source     string // -s: pane to move
    Target     string // -t: destination window/pane
    Horizontal bool   // -h: horizontal split
    Detached   bool   // -d: don't follow focus (used when parking)
    Size       string // -l: size spec ("50%")
}

// NewWindowOpts configures new-window.
type NewWindowOpts struct {
    Detached bool   // -d: don't switch to it
    Name     string // -n: window name
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

// TmuxError wraps a failed tmux command with its stderr output.
type TmuxError struct {
    Command []string
    Stderr  string
    Err     error
}

func (e *TmuxError) Error() string { ... }
func (e *TmuxError) Unwrap() error { return e.Err }

// IsNotFound checks if a tmux error indicates the target doesn't exist.
func IsNotFound(err error) bool { ... } // checks stderr for "can't find", "not found"

// IsNoSpace checks if tmux refused a split due to terminal size.
func IsNoSpace(err error) bool { ... } // checks stderr for "no space for new pane"

// IsSessionGone checks if the tmux session/server is gone.
func IsSessionGone(err error) bool { ... } // checks "no server running", "session not found"
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
func (t *tmuxExec) run(ctx context.Context, args ...string) (string, error) {
    cmd := exec.CommandContext(ctx, t.bin, args...)
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
func (t *tmuxExec) SplitWindow(ctx context.Context, opts SplitOpts) (string, error) {
    args := []string{"split-window"}
    if opts.Horizontal {
        args = append(args, "-h")
    }
    if opts.Target != "" {
        args = append(args, "-t", opts.Target)
    }
    if opts.Size != "" {
        args = append(args, "-l", opts.Size)
    }
    args = append(args, "-P", "-F", "#{pane_id}")
    for _, kv := range opts.Env {
        args = append(args, "-e", kv)
    }
    args = append(args, opts.Command...)
    return t.run(ctx, args...)
}
```

The `-P` flag prints the new pane info, `-F '#{pane_id}'` formats as just the pane ID (e.g., `%42`).

2. **KillPane**: Terminates a pane and its process.

```go
func (t *tmuxExec) KillPane(ctx context.Context, paneID string) error {
    _, err := t.run(ctx, "kill-pane", "-t", paneID)
    return err
}
```

3. **SelectPane**: Moves keyboard focus to a pane.

```go
func (t *tmuxExec) SelectPane(ctx context.Context, paneID string) error {
    _, err := t.run(ctx, "select-pane", "-t", paneID)
    return err
}
```

**Files**: `internal/worker/tmux_cli.go` (~40 lines added)
**Parallel?**: Yes - can be implemented alongside T004.

---

### Subtask T004 - Implement pane movement methods: JoinPane and NewWindow

**Purpose**: Move panes between the visible kasmos window and the hidden parking
window, and create the parking window. Core to the pane swap mechanism (research.md
section 4).

**Steps**:

1. Implement `JoinPane` using `JoinOpts` so callers can select both park/show modes.
2. Implement `NewWindow` with `-d -P -F '#{window_id}'` and optional name.

Note: Both parking (hide) and showing use `JoinPane` with different options:
- **Park**: `JoinOpts{Source: paneID, Target: parkingWindow, Detached: true}`
- **Show**: `JoinOpts{Source: paneID, Target: kasmosWindow, Horizontal: true, Size: "50%"}`

No `BreakPane` method is needed. The interface is simpler with `JoinPane` handling
both directions.

**Files**: `internal/worker/tmux_cli.go` (~40 lines added)
**Parallel?**: Yes - can be implemented alongside T003.

---

### Subtask T005 - Implement pane query methods: ListPanes, CapturePane, DisplayMessage, Version

**Purpose**: Query methods for pane status polling (exit detection), content capture (session ID extraction), self-identification, and version checking.

**Steps**:

1. **ListPanes**: Lists panes in a target window with structured format output.

```go
func (t *tmuxExec) ListPanes(ctx context.Context, target string) ([]PaneInfo, error) {
    format := "#{pane_id} #{pane_pid} #{pane_dead} #{pane_dead_status}"
    args := []string{"list-panes", "-F", format}
    if target == "-s" {
        args = append(args, "-s")
    } else if target != "" {
        args = append(args, "-t", target)
    }
    output, err := t.run(ctx, args...)
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
func (t *tmuxExec) CapturePane(ctx context.Context, paneID string) (string, error) {
    // -p: output to stdout (not paste buffer)
    // -S -: start from beginning of scrollback history
    return t.run(ctx, "capture-pane", "-p", "-t", paneID, "-S", "-")
}
```

4. **DisplayMessage**: Evaluates tmux format strings for self-identification.

```go
// DisplayMessage returns the result of a tmux format string evaluation.
// Used for self-identification: DisplayMessage(ctx, "#{pane_id}"), DisplayMessage(ctx, "#{window_id}")
func (t *tmuxExec) DisplayMessage(ctx context.Context, format string) (string, error) {
    return t.run(ctx, "display-message", "-p", format)
}
```

5. **Version**: Gets the tmux version string.

```go
func (t *tmuxExec) Version(ctx context.Context) (string, error) {
    return t.run(ctx, "-V")
}
```

**Files**: `internal/worker/tmux_cli.go` (~80 lines added)
**Parallel?**: No - `parsePaneList` is a key helper that should be implemented carefully.

**Edge Cases**:
- Empty target (no panes): return empty slice, nil error.
- Malformed lines in list-panes output: skip with warning (don't fail).
- `capture-pane` on dead pane: tmux handles this - content is still in the pane buffer.

---

### Subtask T006 - Implement environment and pane option methods

**Purpose**: Tag managed panes via session-level environment variables for crash
recovery. Set per-pane options like `remain-on-exit`. Uses `tmux set-environment` /
`show-environment` at session scope (research.md section 2).

**Steps**:

1. **SetEnvironment**: Sets a session-level environment variable.

    func (t *tmuxExec) SetEnvironment(ctx context.Context, key, value string) error {
        _, err := t.run(ctx, "set-environment", key, value)
        return err
    }

2. **ShowEnvironment**: Returns all session-level environment variables as a map.

    func (t *tmuxExec) ShowEnvironment(ctx context.Context) (map[string]string, error) {
        output, err := t.run(ctx, "show-environment")
        if err != nil {
            return nil, err
        }
        return parseEnvironment(output)
    }

    Helper parser (pure function):

    func parseEnvironment(output string) (map[string]string, error) {
        env := make(map[string]string)
        for _, line := range strings.Split(output, "\n") {
            line = strings.TrimSpace(line)
            if line == "" || strings.HasPrefix(line, "-") {
                continue // skip unset vars (prefixed with "-")
            }
            if idx := strings.IndexByte(line, '='); idx > 0 {
                env[line[:idx]] = line[idx+1:]
            }
        }
        return env, nil
    }

3. **UnsetEnvironment**: Removes a session-level environment variable.

    func (t *tmuxExec) UnsetEnvironment(ctx context.Context, key string) error {
        _, err := t.run(ctx, "set-environment", "-u", key)
        return err
    }

4. **SetPaneOption**: Sets a per-pane tmux option (used for `remain-on-exit`).

    func (t *tmuxExec) SetPaneOption(ctx context.Context, paneID, key, value string) error {
        _, err := t.run(ctx, "set-option", "-p", "-t", paneID, key, value)
        return err
    }

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
    splitWindowFn    func(ctx context.Context, opts SplitOpts) (string, error)
    joinPaneFn       func(ctx context.Context, opts JoinOpts) error
    selectPaneFn     func(ctx context.Context, paneID string) error
    listPanesFn      func(ctx context.Context, target string) ([]PaneInfo, error)
    killPaneFn       func(ctx context.Context, paneID string) error
    capturePaneFn    func(ctx context.Context, paneID string) (string, error)
    setEnvironmentFn func(ctx context.Context, key, value string) error
    showEnvironmentFn func(ctx context.Context) (map[string]string, error)
    unsetEnvironmentFn func(ctx context.Context, key string) error
    setPaneOptionFn  func(ctx context.Context, paneID, key, value string) error
    newWindowFn      func(ctx context.Context, opts NewWindowOpts) (string, error)
    displayMessageFn func(ctx context.Context, format string) (string, error)
    versionFn        func(ctx context.Context) (string, error)
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

5. **Test `ShowEnvironment` parsing**: Verify `KEY=VALUE` output is split correctly and unset variables (`-KEY`) are ignored.

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
- Verify all methods accept `context.Context` and use `exec.CommandContext` for cancellation.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
