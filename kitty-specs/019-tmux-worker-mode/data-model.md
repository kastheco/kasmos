# Data Model: Tmux Worker Mode

**Feature**: 019-tmux-worker-mode
**Date**: 2026-02-18

## Entity Definitions

### TmuxCLI (Interface)

Thin wrapper around tmux CLI commands. All methods shell out to `tmux` via `os/exec`.
Real implementation: `tmuxExec`. Test implementation: mock.

```
TmuxCLI
  SplitWindow(target, cmd string, horizontal bool, size int) -> (paneID string, error)
  JoinPane(src, dst string, horizontal bool, size int) -> error
  BreakPane(paneID string) -> error
  SelectPane(paneID string) -> error
  ListPanes(target string) -> ([]PaneInfo, error)
  KillPane(paneID string) -> error
  CapturePane(paneID string) -> (content string, error)
  SetPaneEnv(paneID, key, value string) -> error
  GetPaneEnv(paneID, key string) -> (value string, error)
  NewWindow(name string) -> (windowID string, error)
  CurrentPaneID() -> (paneID string, error)
  Version() -> (string, error)
```

### PaneInfo (Value Object)

Parsed output from `tmux list-panes -F`.

```
PaneInfo
  ID          string    // tmux pane identifier (e.g., "%42")
  PID         int       // process running in the pane
  Dead        bool      // true if the pane process has exited
  DeadStatus  int       // exit code of the dead process
  WorkerID    string    // kasmos worker ID (from env tag), empty if untagged
  SessionTag  string    // kasmos session ID (from env tag), empty if untagged
```

### TmuxBackend (WorkerBackend Implementation)

Spawns workers as tmux panes. Manages the parking window and pane lifecycle.

```
TmuxBackend
  cli             TmuxCLI       // injected dependency
  kasmosPaneID    string        // tmux pane ID of the kasmos dashboard
  parkingWindow   string        // tmux window ID for hidden panes
  sessionTag      string        // kasmos session ID for pane tagging
  activePaneID    string        // currently visible worker pane (empty if none)
  managedPanes    map[string]*ManagedPane  // workerID -> pane tracking

  Spawn(ctx, cfg) -> (WorkerHandle, error)    // implements WorkerBackend
  Name() -> "tmux"                            // implements WorkerBackend

  Init(sessionTag string) -> error            // setup parking window, capture kasmos pane ID
  ShowPane(workerID string) -> error          // join-pane from parking to main window
  HidePane(workerID string) -> error          // break-pane to parking
  SwapActive(workerID string) -> error        // hide current + show new
  PollPanes() -> ([]PaneStatus, error)        // list-panes for exit detection
  Reconnect(sessionTag string) -> ([]ReconnectedWorker, error)  // scan for surviving panes
  Cleanup() -> error                          // kill parking window on graceful exit
```

### ManagedPane (Internal Tracking)

Tracks the mapping between kasmos workers and tmux panes.

```
ManagedPane
  WorkerID    string        // kasmos worker ID (e.g., "w-001")
  PaneID      string        // tmux pane ID (e.g., "%42")
  Visible     bool          // true if currently in the main kasmos window
  Dead        bool          // true if the pane process has exited
  ExitCode    int           // exit code, valid only when Dead=true
  CreatedAt   time.Time     // when the pane was created
```

State transitions:
```
Created -> Visible (initial spawn shows the pane)
Visible -> Hidden  (user selects different worker: break-pane to parking)
Hidden  -> Visible (user selects this worker: join-pane from parking)
Visible -> Dead    (process exits while visible: detected by poll)
Hidden  -> Dead    (process exits while hidden: detected by poll)
*       -> Removed (pane externally killed: missing from list-panes)
```

### tmuxHandle (WorkerHandle Implementation)

Implements WorkerHandle for tmux-backed workers.

```
tmuxHandle
  cli         TmuxCLI
  paneID      string        // tmux pane identifier
  workerID    string        // kasmos worker ID
  startTime   time.Time
  exitCh      chan struct{}  // closed when pane death is detected
  exitResult  ExitResult    // populated when exit detected
  mu          sync.Mutex

  Stdout() -> nil                     // output goes to tmux pane, not a pipe
  Wait() -> ExitResult                // blocks on exitCh
  Kill(gracePeriod) -> error          // kills the pane process, then pane
  PID() -> int                        // queries tmux for pane_pid
  Interactive() -> true               // distinguishes from subprocess
  NotifyExit(code int, dur Duration)  // called by TUI tick poller to unblock Wait()
  CaptureOutput() -> (string, error)  // tmux capture-pane for session ID extraction
```

### WorkerHandle Interface (Modified)

Added `Interactive() bool` method:

```
WorkerHandle (interface)
  Stdout() -> io.Reader
  Wait() -> ExitResult
  Kill(gracePeriod time.Duration) -> error
  PID() -> int
  Interactive() -> bool               // NEW: true for tmux, false for subprocess
```

### PaneStatus (Value Object)

Returned by TmuxBackend.PollPanes() for the TUI tick handler.

```
PaneStatus
  WorkerID    string
  PaneID      string
  Dead        bool
  ExitCode    int
  Missing     bool    // true if pane no longer exists (externally killed)
```

### ReconnectedWorker (Value Object)

Returned by TmuxBackend.Reconnect() during reattach.

```
ReconnectedWorker
  WorkerID    string
  PaneID      string
  PID         int
  Dead        bool
  ExitCode    int
```

## Schema Changes

### SessionState (persist/schema.go)

New field:

```
SessionState
  ...existing fields...
  BackendMode   string   `json:"backend_mode,omitempty"`  // "subprocess" or "tmux"
```

Default: `"subprocess"` (omitted for backward compatibility with existing sessions).
When tmux mode is active, set to `"tmux"`. On reattach, if `BackendMode` is `"tmux"`,
kasmos auto-selects the TmuxBackend.

### Config (config/config.go)

New field:

```
Config
  ...existing fields...
  TmuxMode    bool   `toml:"tmux_mode"`   // default: false
```

When `true` and `$TMUX` is set, kasmos starts in tmux mode without requiring `--tmux`.
When `true` but `$TMUX` is not set, kasmos falls back to subprocess mode with a notice.

## New Message Types (tui/messages.go)

```
paneSwappedMsg
  WorkerID    string    // the worker now visible
  PaneID      string    // tmux pane ID now visible
  Err         error

paneExitedMsg
  WorkerID    string
  PaneID      string
  ExitCode    int
  Output      string    // captured pane content for session ID extraction

paneDetectedMsg
  Workers     []ReconnectedWorker   // surviving panes found during reattach
  Err         error

tmuxInitMsg
  KasmosPaneID   string   // this process's tmux pane ID
  ParkingWindow  string   // created parking window ID
  Err            error
```

## Validation Rules

- `TmuxBackend.Init()` fails if `$TMUX` is not set (not inside tmux)
- `TmuxBackend.Init()` fails if `tmux` binary is not in PATH
- `TmuxBackend.Spawn()` fails if `Init()` was not called
- `--tmux` and `-d` cannot be combined (validated in `main.go` before backend creation)
- `WorkerHandle.Interactive()` must return consistent value for the lifetime of the handle
- `ManagedPane.PaneID` must be non-empty after successful spawn
- Session `BackendMode` is set once at session creation and never changes mid-session
