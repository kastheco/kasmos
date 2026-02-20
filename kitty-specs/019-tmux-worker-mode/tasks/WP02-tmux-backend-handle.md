---
work_package_id: "WP02"
subtasks:
  - "T007"
  - "T008"
  - "T009"
  - "T010"
  - "T011"
  - "T012"
  - "T013"
  - "T014"
  - "T042"
title: "TmuxBackend & tmuxHandle Implementation"
phase: "Phase 1 - TmuxBackend Core"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP01"]
history:
  - timestamp: "2026-02-19T03:53:34Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP02 - TmuxBackend & tmuxHandle Implementation

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
spec-kitty implement WP02 --base WP01
```

Depends on WP01 (TmuxCLI interface must exist).

---

## Objectives & Success Criteria

1. **`TmuxBackend` implements `WorkerBackend`**: `Spawn()` creates a tagged tmux pane running `opencode run`. `Name()` returns `"tmux"`.
2. **`tmuxHandle` implements `WorkerHandle`**: `Interactive()` returns `true`. `Stdout()` returns `nil`. `Wait()` blocks until exit detection. `Kill()` terminates the pane.
3. **`WorkerHandle` interface extended**: `Interactive() bool` added. `subprocessHandle` returns `false`.
4. **Pane visibility**: `ShowPane`, `HidePane`, `SwapActive` correctly move panes between parking and visible positions.
5. **`PollPanes()`**: Detects dead and missing panes for exit detection by the TUI tick handler.
6. **`Reconnect()`**: Scans for surviving tagged panes after kasmos restart.
7. **`go test ./internal/worker/...`** passes with mock `TmuxCLI`.

**Requirements covered**: FR-005, FR-010, FR-012, FR-015, partially FR-014.

## Context & Constraints

- **Data model**: See `kitty-specs/019-tmux-worker-mode/data-model.md` for TmuxBackend, ManagedPane, tmuxHandle, PaneStatus, ReconnectedWorker definitions.
- **Pane parking design**: research.md section 4 - hidden parking window with `join-pane` in both directions (`-d` for parking).
- **Exit detection design**: research.md section 3 - poll `list-panes` on tick, check `pane_dead`.
- **Interface extension design**: research.md section 5 - `Interactive() bool` on WorkerHandle.
- **Tagging design**: research.md section 2 - session env vars `KASMOS_PANE_<worker_id>=<pane_id>` plus `KASMOS_SESSION_ID`, `KASMOS_PARKING`, `KASMOS_DASHBOARD`.
- **Session ID extraction**: research.md section 6 - `capture-pane` on exit, apply existing regex.
- **Existing patterns**: `internal/worker/subprocess.go` for WorkerBackend/WorkerHandle implementation pattern.

**Key reference files**:
- `internal/worker/backend.go` - WorkerBackend and WorkerHandle interfaces
- `internal/worker/subprocess.go` - Reference implementation pattern
- `internal/worker/tmux_cli.go` - TmuxCLI interface (from WP01)
- `kitty-specs/019-tmux-worker-mode/data-model.md` - Full type definitions

---

## Subtasks & Detailed Guidance

### Subtask T007 - Define TmuxBackend, ManagedPane, PaneStatus, and ReconnectedWorker types

**Purpose**: Establish all type definitions for the tmux backend. These are the data structures that track the relationship between kasmos workers and tmux panes.

**Steps**:
1. Create `internal/worker/tmux.go`.
2. Define `TmuxBackend` struct:

```go
// TmuxBackend spawns workers as interactive tmux panes.
type TmuxBackend struct {
    cli           TmuxCLI
    openCodeBin   string        // resolved path to opencode binary
    kasmosPaneID  string        // tmux pane ID of the kasmos dashboard
    parkingWindow string        // tmux window ID for hidden panes
    sessionTag    string        // kasmos session ID for pane tagging
    activePaneID  string        // currently visible worker pane (empty if none)
    managedPanes  map[string]*ManagedPane // workerID -> pane tracking
    mu            sync.Mutex    // protects pane operations
}
```

3. Define `ManagedPane`:

```go
// ManagedPane tracks the mapping between a kasmos worker and its tmux pane.
type ManagedPane struct {
    WorkerID  string
    PaneID    string
    Visible   bool
    Dead      bool
    ExitCode  int
    CreatedAt time.Time
}
```

4. Define `PaneStatus` (returned by PollPanes):

```go
// PaneStatus reports the current state of a managed pane.
type PaneStatus struct {
    WorkerID string
    PaneID   string
    Dead     bool
    ExitCode int
    Missing  bool // true if pane no longer exists (externally killed)
}
```

5. Define `ReconnectedWorker` (returned by Reconnect):

```go
// ReconnectedWorker represents a pane discovered during reattach.
type ReconnectedWorker struct {
    WorkerID string
    PaneID   string
    PID      int
    Dead     bool
    ExitCode int
}
```

6. Add constructor:

```go
func NewTmuxBackend(cli TmuxCLI) (*TmuxBackend, error) {
    bin, err := exec.LookPath("opencode")
    if err != nil {
        return nil, fmt.Errorf("opencode not found in PATH: %w", err)
    }
    return &TmuxBackend{
        cli:          cli,
        openCodeBin:  bin,
        managedPanes: make(map[string]*ManagedPane),
    }, nil
}
```

7. Add compile-time check: `var _ WorkerBackend = (*TmuxBackend)(nil)`.

**Files**: `internal/worker/tmux.go` (new, ~80 lines)
**Parallel?**: Yes - can proceed alongside T008.

---

### Subtask T008 - Add Interactive() bool to WorkerHandle interface and subprocessHandle

**Purpose**: The TUI needs to distinguish interactive panes from pipe-captured subprocesses. This is a backward-compatible interface extension.

**Steps**:
1. In `internal/worker/backend.go`, add `Interactive() bool` to `WorkerHandle`:

```go
type WorkerHandle interface {
    Stdout() io.Reader
    Wait() ExitResult
    Kill(gracePeriod time.Duration) error
    PID() int
    Interactive() bool // NEW: true for tmux, false for subprocess
}
```

2. In `internal/worker/subprocess.go`, add implementation:

```go
func (h *subprocessHandle) Interactive() bool {
    return false
}
```

3. Verify `go build ./internal/worker/...` compiles - `subprocessHandle` must satisfy the updated interface.

**Files**:
- `internal/worker/backend.go` (modify, add 1 line to interface)
- `internal/worker/subprocess.go` (modify, add 4 lines)
**Parallel?**: Yes - different files from T007.

---

### Subtask T009 - Implement TmuxBackend.Init()

**Purpose**: Initialize the tmux backend: capture the kasmos pane/window IDs,
create the hidden parking window, and set session infrastructure tags. Must be
called before `Spawn()`.

**Steps**:
1. Implement `Init`:

```go
func (b *TmuxBackend) Init(sessionTag string) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    ctx := context.Background()

    b.sessionTag = sessionTag

    // Capture our own pane ID
    paneID, err := b.cli.DisplayMessage(ctx, "#{pane_id}")
    if err != nil {
        return fmt.Errorf("get kasmos pane ID: %w", err)
    }
    b.kasmosPaneID = paneID

    // Capture our own window ID (needed as join-pane target)
    windowID, err := b.cli.DisplayMessage(ctx, "#{window_id}")
    if err != nil {
        return fmt.Errorf("get kasmos window ID: %w", err)
    }
    b.kasmosWindowID = windowID

    // Create hidden parking window for non-visible worker panes
    parkingWindowID, err := b.cli.NewWindow(ctx, NewWindowOpts{Detached: true, Name: "kasmos-parking"})
    if err != nil {
        return fmt.Errorf("create parking window: %w", err)
    }
    b.parkingWindow = parkingWindowID

    // Tag session with kasmos identity for crash recovery
    if err := b.cli.SetEnvironment(ctx, "KASMOS_SESSION_ID", sessionTag); err != nil {
        return fmt.Errorf("set session tag: %w", err)
    }
    if err := b.cli.SetEnvironment(ctx, "KASMOS_DASHBOARD", paneID); err != nil {
        // Non-fatal
    }
    if err := b.cli.SetEnvironment(ctx, "KASMOS_PARKING", parkingWindowID); err != nil {
        // Non-fatal
    }

    return nil
}
```

2. The parking window is created with `-d` (don't switch to it) and named `kasmos-parking`.

**Files**: `internal/worker/tmux.go` (~25 lines added)
**Parallel?**: No - foundational method that other methods depend on.

**Edge Cases**:
- Already initialized (called twice): Guard with a check, or make idempotent.
- tmux session already has a `kasmos-parking` window: The `NewWindow` will create a second one. Consider checking first or using a unique name with session tag.

---

### Subtask T010 - Implement TmuxBackend.Spawn()

**Purpose**: Create a new worker as a tmux pane. The pane runs the `opencode run` command with the same arguments as `SubprocessBackend`. The pane is tagged and tracked.

**Steps**:
1. Implement `Spawn`:

```go
func (b *TmuxBackend) Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.kasmosPaneID == "" {
        return nil, errors.New("TmuxBackend.Init() must be called before Spawn()")
    }

    // Build the opencode command string
    args := b.buildArgs(cfg)
    cmdStr := b.openCodeBin + " " + strings.Join(args, " ")

    // Create a new pane in the kasmos window (will be shown immediately)
    paneID, err := b.cli.SplitWindow(b.kasmosPaneID, cmdStr, true, 50)
    if err != nil {
        return nil, fmt.Errorf("create worker pane: %w", err)
    }

    // Retain the pane after process exit (for poll detection and capture)
    if err := b.cli.SetPaneOption(ctx, paneID, "remain-on-exit", "on"); err != nil {
        // Non-fatal: pane works but won't survive exit for capture
        // Log warning
    }

    // Tag the pane with worker and session IDs
    tagKey := fmt.Sprintf("KASMOS_PANE_%s", cfg.ID)
    if err := b.cli.SetEnvironment(ctx, tagKey, paneID); err != nil {
        // Non-fatal: pane exists but tagging failed
        // Log warning but continue
    }

    startTime := time.Now()

    // Track the managed pane
    managed := &ManagedPane{
        WorkerID:  cfg.ID,
        PaneID:    paneID,
        Visible:   true,
        CreatedAt: startTime,
    }
    b.managedPanes[cfg.ID] = managed

    // If there was a previously visible pane, hide it
    if b.activePaneID != "" && b.activePaneID != paneID {
        if prev := b.findPaneByID(b.activePaneID); prev != nil {
            // Move previous to parking
            if err := b.cli.JoinPane(ctx, JoinOpts{
                Source:   b.activePaneID,
                Target:   b.parkingWindow,
                Detached: true,
            }); err != nil {
                // Non-fatal, log warning
            }
            prev.Visible = false
        }
    }
    b.activePaneID = paneID

    // Focus the new pane
    _ = b.cli.SelectPane(ctx, paneID)

    handle := &tmuxHandle{
        cli:       b.cli,
        paneID:    paneID,
        workerID:  cfg.ID,
        startTime: startTime,
        exitCh:    make(chan struct{}),
    }

    return handle, nil
}
```

2. Reuse the `buildArgs` pattern from `SubprocessBackend`:

```go
func (b *TmuxBackend) buildArgs(cfg SpawnConfig) []string {
    args := []string{"run"}
    if cfg.Role != "" {
        args = append(args, "--agent", cfg.Role)
    }
    if cfg.ContinueSession != "" {
        args = append(args, "--continue", "-s", cfg.ContinueSession)
    }
    if cfg.Model != "" {
        args = append(args, "--model", cfg.Model)
    }
    if cfg.Reasoning != "" && cfg.Reasoning != "default" {
        args = append(args, "--variant", cfg.Reasoning)
    }
    for _, f := range cfg.Files {
        args = append(args, "--file", f)
    }
    if cfg.Prompt != "" {
        args = append(args, cfg.Prompt)
    }
    return args
}
```

3. Add a helper for finding a managed pane by its tmux pane ID:

```go
func (b *TmuxBackend) findPaneByID(paneID string) *ManagedPane {
    for _, mp := range b.managedPanes {
        if mp.PaneID == paneID {
            return mp
        }
    }
    return nil
}
```

4. Implement `Name()`:

```go
func (b *TmuxBackend) Name() string {
    return "tmux"
}
```

**Files**: `internal/worker/tmux.go` (~80 lines added)
**Parallel?**: No - depends on T009 (Init pattern).

**Edge Cases**:
- Spawn when kasmos window is too narrow: tmux will refuse the split. Return a descriptive error.
- SpawnConfig has environment variables: For tmux panes, env vars must be set via the command string (e.g., `env KEY=VAL opencode run ...`) or via `tmux set-environment` before the command runs.

---

### Subtask T011 - Implement tmuxHandle struct

**Purpose**: The `WorkerHandle` for tmux-backed workers. Unlike `subprocessHandle`, there's no stdout pipe - output goes to the tmux pane. Exit is detected via poll-driven `NotifyExit()`, not by `cmd.Wait()`.

**Steps**:
1. Define the struct:

```go
type tmuxHandle struct {
    cli        TmuxCLI
    paneID     string
    workerID   string
    startTime  time.Time
    exitCh     chan struct{}
    exitResult ExitResult
    mu         sync.Mutex
    exited     bool
}
```

2. Implement `WorkerHandle` methods:

```go
func (h *tmuxHandle) Stdout() io.Reader {
    return nil // output goes to tmux pane, not a pipe
}

func (h *tmuxHandle) Wait() ExitResult {
    <-h.exitCh
    h.mu.Lock()
    defer h.mu.Unlock()
    return h.exitResult
}

func (h *tmuxHandle) Kill(gracePeriod time.Duration) error {
    return h.cli.KillPane(h.paneID)
}

func (h *tmuxHandle) PID() int {
    panes, err := h.cli.ListPanes("")
    if err != nil {
        return 0
    }
    for _, p := range panes {
        if p.ID == h.paneID {
            return p.PID
        }
    }
    return 0
}

func (h *tmuxHandle) Interactive() bool {
    return true
}
```

3. Add `NotifyExit` (called by TUI tick poller when pane death is detected):

```go
// NotifyExit signals that the pane's process has exited.
// Called by the TUI tick handler when PollPanes detects a dead pane.
func (h *tmuxHandle) NotifyExit(code int, duration time.Duration) {
    h.mu.Lock()
    defer h.mu.Unlock()
    if h.exited {
        return
    }
    h.exited = true
    h.exitResult = ExitResult{
        Code:     code,
        Duration: duration,
    }
    close(h.exitCh)
}
```

4. Add `CaptureOutput` (for session ID extraction on exit):

```go
// CaptureOutput captures the pane's terminal content for session ID extraction.
func (h *tmuxHandle) CaptureOutput() (string, error) {
    return h.cli.CapturePane(h.paneID)
}
```

5. Add compile-time check: `var _ WorkerHandle = (*tmuxHandle)(nil)`.

**Files**: `internal/worker/tmux.go` (~70 lines added)
**Parallel?**: No - depends on T007 types.

**Edge Cases**:
- `Kill()` on already-dead pane: tmux `kill-pane` returns error. Ignore ESRCH-equivalent.
- `PID()` on dead pane: `list-panes` may still show the pane with a PID of the dead process. Return it anyway.
- `NotifyExit()` called twice: Guard with `h.exited` flag to prevent double-close of channel.

---

### Subtask T012 - Implement pane visibility management: ShowPane, HidePane, SwapActive

**Purpose**: Move worker panes between the parking window and the visible position next to kasmos. Only one worker pane is visible at a time (FR-007).

**Steps**:
1. **ShowPane**: Brings a pane from parking to the kasmos window.

```go
func (b *TmuxBackend) ShowPane(workerID string) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    ctx := context.Background()

    managed, ok := b.managedPanes[workerID]
    if !ok {
        return fmt.Errorf("unknown worker %q", workerID)
    }
    if managed.Visible {
        return nil // already visible
    }

    // Join from parking to kasmos window, horizontal split, 50% width
    if err := b.cli.JoinPane(ctx, JoinOpts{
        Source:     managed.PaneID,
        Target:     b.kasmosWindowID,
        Horizontal: true,
        Size:       "50%",
    }); err != nil {
        return fmt.Errorf("show pane for worker %q: %w", workerID, err)
    }

    managed.Visible = true
    b.activePaneID = managed.PaneID
    return nil
}
```

2. **HidePane**: Moves a pane from visible to parking.

```go
func (b *TmuxBackend) HidePane(workerID string) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    ctx := context.Background()

    managed, ok := b.managedPanes[workerID]
    if !ok {
        return fmt.Errorf("unknown worker %q", workerID)
    }
    if !managed.Visible {
        return nil // already hidden
    }

    // Move to parking window
    if err := b.cli.JoinPane(ctx, JoinOpts{
        Source:   managed.PaneID,
        Target:   b.parkingWindow,
        Detached: true,
    }); err != nil {
        return fmt.Errorf("hide pane for worker %q: %w", workerID, err)
    }

    managed.Visible = false
    if b.activePaneID == managed.PaneID {
        b.activePaneID = ""
    }
    return nil
}
```

3. **SwapActive**: Hide the current worker, show the new one, focus it.

```go
func (b *TmuxBackend) SwapActive(workerID string) error {
    b.mu.Lock()
    defer b.mu.Unlock()
    ctx := context.Background()

    // Hide current active pane (if any)
    if b.activePaneID != "" {
        if current := b.findPaneByID(b.activePaneID); current != nil && current.Visible {
            if err := b.cli.JoinPane(ctx, JoinOpts{
                Source:   current.PaneID,
                Target:   b.parkingWindow,
                Detached: true, // don't follow focus into parking
            }); err != nil {
                return fmt.Errorf("hide current pane: %w", err)
            }
            current.Visible = false
        }
    }

    // Show new worker pane
    managed, ok := b.managedPanes[workerID]
    if !ok {
        return fmt.Errorf("unknown worker %q", workerID)
    }

    if err := b.cli.JoinPane(ctx, JoinOpts{
        Source:     managed.PaneID,
        Target:     b.kasmosWindowID, // NOTE: window ID, not pane ID
        Horizontal: true,
        Size:       "50%",
    }); err != nil {
        return fmt.Errorf("show pane for worker %q: %w", workerID, err)
    }

    managed.Visible = true
    b.activePaneID = managed.PaneID

    // Move focus to the worker pane
    if err := b.cli.SelectPane(ctx, managed.PaneID); err != nil {
        return fmt.Errorf("focus worker pane: %w", err)
    }

    return nil
}
```

**Files**: `internal/worker/tmux.go` (~80 lines added)
**Parallel?**: No - depends on T009/T010 patterns.

**Important notes**:
- The locking in ShowPane/HidePane/SwapActive is critical for preventing races during rapid selection changes.
- `JoinPane` show path uses `JoinOpts{Horizontal: true, Size: "50%"}` to create a 50/50 split.
- `JoinPane` park path uses `JoinOpts{Detached: true}` so focus does not follow the pane into parking.

---

### Subtask T013 - Implement PollPanes()

**Purpose**: Called by the TUI tick handler (every 1 second) to detect dead or missing panes. Returns a list of status changes that the TUI converts into messages.

**Steps**:
1. Implement `PollPanes`:

```go
// PollPanes checks all managed panes for status changes.
// Returns PaneStatus for any pane that is dead or missing.
func (b *TmuxBackend) PollPanes() ([]PaneStatus, error) {
    b.mu.Lock()
    defer b.mu.Unlock()

    if len(b.managedPanes) == 0 {
        return nil, nil
    }

    // Collect all live pane IDs we're tracking
    var results []PaneStatus

    // Check parking window panes
    parkingPanes, err := b.cli.ListPanes(b.parkingWindow)
    if err != nil {
        // Parking window might be gone if all panes exited
        parkingPanes = nil
    }

    // Check active pane (if visible)
    var activePanes []PaneInfo
    if b.activePaneID != "" {
        // List panes in the current window to find the active one
        currentPanes, err := b.cli.ListPanes("")
        if err == nil {
            activePanes = currentPanes
        }
    }

    allPanes := append(parkingPanes, activePanes...)
    paneMap := make(map[string]PaneInfo)
    for _, p := range allPanes {
        paneMap[p.ID] = p
    }

    // Check each managed pane
    for workerID, managed := range b.managedPanes {
        if managed.Dead {
            continue // already reported
        }

        info, found := paneMap[managed.PaneID]
        if !found {
            // Pane is missing - externally killed (FR-014)
            results = append(results, PaneStatus{
                WorkerID: workerID,
                PaneID:   managed.PaneID,
                Missing:  true,
            })
            managed.Dead = true
            managed.ExitCode = -1
            continue
        }

        if info.Dead {
            // Pane process exited
            results = append(results, PaneStatus{
                WorkerID: workerID,
                PaneID:   managed.PaneID,
                Dead:     true,
                ExitCode: info.DeadStatus,
            })
            managed.Dead = true
            managed.ExitCode = info.DeadStatus
        }
    }

    return results, nil
}
```

**Files**: `internal/worker/tmux.go` (~50 lines added)
**Parallel?**: No - depends on T007 types and T009 init pattern.

**Edge Cases**:
- All panes dead: parking window may no longer exist. Handle `list-panes` error gracefully.
- Pane appears in both parking and active lists: shouldn't happen, but deduplicate via `paneMap`.
- No managed panes: return early with nil.

---

### Subtask T014 - Implement Reconnect() and Cleanup()

**Purpose**: `Reconnect` scans for surviving tagged panes after kasmos restart
(FR-013). `Cleanup` tears down panes/windows and clears `KASMOS_*` tags on
graceful exit.

**Steps**:
1. Implement `Reconnect`:

```go
// Reconnect scans for surviving worker panes tagged with the given session.
// Called during --attach to rediscover panes from a previous kasmos instance.
func (b *TmuxBackend) Reconnect(sessionTag string) ([]ReconnectedWorker, error) {
    b.mu.Lock()
    defer b.mu.Unlock()
    ctx := context.Background()

    b.sessionTag = sessionTag

    // Get our current pane ID
    paneID, err := b.cli.DisplayMessage(ctx, "#{pane_id}")
    if err != nil {
        return nil, fmt.Errorf("get kasmos pane ID: %w", err)
    }
    b.kasmosPaneID = paneID

    // Recover infrastructure tags
    env, err := b.cli.ShowEnvironment(ctx)
    if err == nil {
        if parking, ok := env["KASMOS_PARKING"]; ok {
            b.parkingWindow = parking
        }
        // KASMOS_DASHBOARD is informational; we already have our own pane ID
    }

    // If no parking window found, create one
    if b.parkingWindow == "" {
        windowID, err := b.cli.NewWindow(ctx, NewWindowOpts{Detached: true, Name: "kasmos-parking"})
        if err != nil {
            return nil, fmt.Errorf("create parking window: %w", err)
        }
        b.parkingWindow = windowID
        _ = b.cli.SetEnvironment(ctx, "KASMOS_PARKING", windowID)
    }

    // List all panes across all windows in the current session
    allPanes, err := b.cli.ListPanes(ctx, "-s") // -s flag = all panes in session
    if err != nil {
        return nil, fmt.Errorf("list session panes: %w", err)
    }

    paneMap := make(map[string]PaneInfo)
    for _, pane := range allPanes {
        paneMap[pane.ID] = pane
    }

    if env == nil {
        env = map[string]string{}
    }

    var workers []ReconnectedWorker

    // Scan session env vars for KASMOS_PANE_* entries
    for key, paneID := range env {
        if !strings.HasPrefix(key, "KASMOS_PANE_") {
            continue
        }
        workerID := strings.TrimPrefix(key, "KASMOS_PANE_")

        pane, ok := paneMap[paneID]
        if !ok {
            _ = b.cli.UnsetEnvironment(ctx, key) // stale tag
            continue
        }
        if pane.ID == b.kasmosPaneID {
            continue // skip our own pane
        }

        workers = append(workers, ReconnectedWorker{
            WorkerID: workerID,
            PaneID:   pane.ID,
            PID:      pane.PID,
            Dead:     pane.Dead,
            ExitCode: pane.DeadStatus,
        })

        // Track the rediscovered pane
        b.managedPanes[workerID] = &ManagedPane{
            WorkerID: workerID,
            PaneID:   pane.ID,
            Visible:  false, // start hidden, TUI will show as needed
            Dead:     pane.Dead,
            ExitCode: pane.DeadStatus,
        }
    }

    return workers, nil
}
```

2. Implement `Cleanup`:

```go
// Cleanup kills the parking window and all managed panes.
// Called on graceful kasmos exit.
func (b *TmuxBackend) Cleanup() error {
    b.mu.Lock()
    defer b.mu.Unlock()
    ctx := context.Background()

    if b.parkingWindow != "" {
        // Kill the parking window (and all panes in it)
        _ = b.cli.KillPane(ctx, b.parkingWindow)
        b.parkingWindow = ""
    }

    // Kill any visible worker pane
    if b.activePaneID != "" {
        _ = b.cli.KillPane(ctx, b.activePaneID)
        b.activePaneID = ""
    }

    // Clean up kasmos env tags
    env, err := b.cli.ShowEnvironment(ctx)
    if err == nil {
        for key := range env {
            if strings.HasPrefix(key, "KASMOS_") {
                _ = b.cli.UnsetEnvironment(ctx, key)
            }
        }
    }

    b.managedPanes = make(map[string]*ManagedPane)
    return nil
}
```

3. Add `KasmosPaneID()` accessor for TUI:

```go
// KasmosPaneID returns the tmux pane ID of the kasmos dashboard.
func (b *TmuxBackend) KasmosPaneID() string {
    return b.kasmosPaneID
}
```

4. Add `ActivePaneID()` accessor:

```go
// ActivePaneID returns the tmux pane ID of the currently visible worker.
func (b *TmuxBackend) ActivePaneID() string {
    b.mu.Lock()
    defer b.mu.Unlock()
    return b.activePaneID
}
```

5. Add `Handle()` accessor to retrieve a tmuxHandle for a reconnected worker:

```go
// Handle creates a tmuxHandle for an existing managed pane (used during reconnect).
func (b *TmuxBackend) Handle(workerID string, startTime time.Time) WorkerHandle {
    b.mu.Lock()
    defer b.mu.Unlock()

    managed, ok := b.managedPanes[workerID]
    if !ok {
        return nil
    }

    h := &tmuxHandle{
        cli:       b.cli,
        paneID:    managed.PaneID,
        workerID:  workerID,
        startTime: startTime,
        exitCh:    make(chan struct{}),
    }

    if managed.Dead {
        h.NotifyExit(managed.ExitCode, time.Since(startTime))
    }

    return h
}
```

**Files**: `internal/worker/tmux.go` (~100 lines added)
**Parallel?**: No - final subtask, depends on established patterns.

**Edge Cases**:
- Reconnect finds no tagged panes: Return empty slice (clean start).
- Reconnect finds panes from a different kasmos session: Filter by session tag.
- Cleanup called when no parking window exists: Ignore errors from KillPane.
- tmux session destroyed before cleanup: All KillPane calls will fail silently.

**Total file estimate**: `internal/worker/tmux.go` should be approximately 450-500 lines total.

---

### Subtask T042 - Unit tests for TmuxBackend with mock TmuxCLI

**Purpose**: Validate TmuxBackend behavior (spawn, swap, poll, reconnect) and tmuxHandle lifecycle using the mock TmuxCLI from WP01's `tmux_cli_test.go`.

**Steps**:

1. Create `internal/worker/tmux_test.go`.

2. **Test TmuxBackend.Init()**:
   - Mock `DisplayMessage("#{pane_id}")` returns `%1`, `DisplayMessage("#{window_id}")` returns `@1`, `NewWindow` returns `@2`.
   - Verify `kasmosPaneID`, `parkingWindow`, `sessionTag` are set.
   - Test Init failure when `DisplayMessage` errors.

3. **Test TmuxBackend.Spawn()**:
   - Mock `SplitWindow` returns `%3`.
   - Verify returned handle is `tmuxHandle` with `Interactive() == true`, `Stdout() == nil`.
   - Verify managed pane is tracked with correct worker ID.
   - Verify spawn before Init returns error.

4. **Test TmuxBackend.SwapActive()**:
   - Spawn 2 workers (mock returns `%3` then `%4`).
   - Call `SwapActive` to second worker.
   - Verify `JoinPane` called with first worker's pane to parking, then second worker's pane to kasmos.
   - Verify `SelectPane` called for focus.

5. **Test TmuxBackend.PollPanes()**:
   - Mock `ListPanes` returns one dead pane.
   - Verify `PollPanes` returns `PaneStatus{Dead: true, ExitCode: 1}`.
   - Mock `ListPanes` returns missing pane (not in list).
   - Verify `PaneStatus{Missing: true}`.

6. **Test tmuxHandle lifecycle**:
   - Create handle, verify `Wait()` blocks.
   - Call `NotifyExit(0, 5*time.Second)`.
   - Verify `Wait()` unblocks and returns correct `ExitResult`.
   - Call `NotifyExit` again, verify no panic (idempotent).

7. **Test TmuxBackend.Reconnect()**:
   - Mock `ListPanes("-s")` returns 2 panes with matching env tags.
   - Verify `Reconnect` returns 2 `ReconnectedWorker` entries.
   - Verify managed panes are tracked.

8. **Test WorkerHandle.Interactive()**:
   - Verify `subprocessHandle.Interactive()` returns `false`.
   - Verify `tmuxHandle.Interactive()` returns `true`.

**Files**: `internal/worker/tmux_test.go` (new, ~250 lines)
**Parallel?**: Yes - can proceed once T007-T014 are done.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Mutex contention during rapid spawn/swap | Slow pane operations | Mutex is held briefly (tmux commands are <100ms). No goroutines hold locks. |
| join-pane fails on narrow terminal | Worker can't be shown | Check for error, provide descriptive message, don't crash. |
| Parking window accumulates dead panes | Memory/pane leak | PollPanes marks them dead; Cleanup removes all on exit. |
| opencode command has special characters | Shell injection | Command is passed directly to `split-window` as a single arg, not through shell. But verify tmux doesn't split on spaces. Consider using `tmux send-keys` pattern instead if issues arise. |
| Reconnect finds stale env vars | Wrong worker mapping | Include session tag in env var check. Only reconnect panes from the same session. |

## Review Guidance

- Verify `var _ WorkerBackend = (*TmuxBackend)(nil)` and `var _ WorkerHandle = (*tmuxHandle)(nil)` are present.
- Verify `subprocessHandle.Interactive()` returns `false` (backward compat).
- Verify mutex locking is correct in all TmuxBackend methods (no lock held across CLI calls when possible, but lock held for state consistency).
- Verify `NotifyExit` is idempotent (double-call safe).
- Verify `Wait()` blocks correctly on `exitCh` channel.
- Verify `Spawn()` handles the "first worker" case (no previous active pane).
- Verify `PollPanes()` handles the "parking window gone" case.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
