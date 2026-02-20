# Data Model: Tmux Worker Mode

**Feature**: 019-tmux-worker-mode
**Date**: 2026-02-18

## Entity Definitions

### TmuxCLI (Interface)

Thin wrapper around tmux CLI commands. All methods shell out to `tmux` via `os/exec`.
Real implementation: `tmuxExec`. Test implementation: mock.

```
TmuxCLI
  SplitWindow(ctx context.Context, opts SplitOpts) -> (paneID string, error)
  JoinPane(ctx context.Context, opts JoinOpts) -> error
  SelectPane(ctx context.Context, paneID string) -> error
  KillPane(ctx context.Context, paneID string) -> error
  NewWindow(ctx context.Context, opts NewWindowOpts) -> (windowID string, error)
  ListPanes(ctx context.Context, target string) -> ([]PaneInfo, error)
  DisplayMessage(ctx context.Context, format string) -> (string, error)
  CapturePane(ctx context.Context, paneID string) -> (content string, error)
  SetEnvironment(ctx context.Context, key, value string) -> error
  ShowEnvironment(ctx context.Context) -> (map[string]string, error)
  UnsetEnvironment(ctx context.Context, key string) -> error
  SetPaneOption(ctx context.Context, paneID, key, value string) -> error
  Version(ctx context.Context) -> (string, error)

SplitOpts
  Target      string    // pane/window to split from
  Horizontal  bool      // -h flag
  Size        string    // -l flag: "50%" or "80"
  Command     []string  // command to run in the new pane
  Env         []string  // environment variables as "KEY=VALUE" for -e flags

JoinOpts
  Source      string    // -s: pane to move
  Target      string    // -t: destination window/pane
  Horizontal  bool      // -h: horizontal split
  Detached    bool      // -d: don't follow focus
  Size        string    // -l: size spec ("50%")

NewWindowOpts
  Detached    bool      // -d: don't switch to it
  Name        string    // -n: window name
```

### PaneInfo (Value Object)

Parsed output from `tmux list-panes -F`.

```
PaneInfo
  ID          string    // tmux pane identifier (e.g., "%42")
  PID         int       // process running in the pane
  Dead        bool      // true if the pane process has exited
  DeadStatus  int       // exit code of the dead process
```

### TmuxBackend (WorkerBackend Implementation)

Spawns workers as tmux panes. Manages the parking window and pane lifecycle.

```
TmuxBackend
  cli             TmuxCLI       // injected dependency
  kasmosPaneID    string        // tmux pane ID of the kasmos dashboard
  kasmosWindowID  string        // tmux window ID of the kasmos dashboard (join-pane target)
  parkingWindow   string        // tmux window ID for hidden panes
  sessionTag      string        // kasmos session ID for pane tagging
  activePaneID    string        // currently visible worker pane (empty if none)
  managedPanes    map[string]*ManagedPane  // workerID -> pane tracking
  mu              sync.RWMutex  // protects pane operations

  Spawn(ctx, cfg) -> (WorkerHandle, error)    // implements WorkerBackend
  Name() -> "tmux"                            // implements WorkerBackend

  Init(sessionTag string) -> error            // setup parking window, capture kasmos pane/window IDs, set env tags
  ShowPane(workerID string) -> error          // join-pane from parking to kasmos window
  HidePane(workerID string) -> error          // join-pane -d to parking
  SwapActive(workerID string) -> error        // hide current + show new
  PollPanes() -> ([]PaneStatus, error)        // list-panes -s for session-wide exit detection
  Reconnect(sessionTag string) -> ([]ReconnectedWorker, error)  // scan env vars + list-panes
  Cleanup() -> error                          // kill parking window, unset all KASMOS_* env vars
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
Visible -> Hidden  (user selects different worker: join-pane -d to parking)
Hidden  -> Visible (user selects this worker: join-pane from parking)
Visible -> Dead    (process exits while visible: detected by poll, pane retained via remain-on-exit)
Hidden  -> Dead    (process exits while hidden: detected by poll, pane retained via remain-on-exit)
*       -> Removed (pane externally killed: missing from list-panes -s)
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
  KasmosPaneID    string   // this process's tmux pane ID
  KasmosWindowID  string   // this process's tmux window ID (join-pane target)
  ParkingWindow   string   // created parking window ID
  Err             error
```

## Validation Rules

- `TmuxBackend.Init()` fails if `$TMUX` is not set (not inside tmux)
- `TmuxBackend.Init()` fails if `tmux` binary is not in PATH
- `TmuxBackend.Spawn()` fails if `Init()` was not called
- `--tmux` and `-d` cannot be combined (validated in `main.go` before backend creation)
- `WorkerHandle.Interactive()` must return consistent value for the lifetime of the handle
- `ManagedPane.PaneID` must be non-empty after successful spawn
- Session `BackendMode` is set once at session creation and never changes mid-session
