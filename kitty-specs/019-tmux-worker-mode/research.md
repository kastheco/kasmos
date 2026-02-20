# Research: Tmux Worker Mode

**Feature**: 019-tmux-worker-mode
**Date**: 2026-02-18

## 1. tmux CLI Capabilities for Pane Management

### Decision: Direct CLI via os/exec, no Go library

**Rationale**: No production-quality Go tmux library exists. The go-tmux package
is a thin wrapper around the CLI with minimal adoption. Direct `exec.Command("tmux", ...)`
is simpler, more debuggable, and matches how every other tool interacts with tmux.

**Alternatives considered**:
- go-tmux library: Too thin, adds dependency for no real abstraction benefit.
- tmux control mode (`-C`): Powerful but complex. Requires persistent connection
  and parsing of control-mode output. Overkill for our use case.

### Key tmux commands needed

| Operation          | Command                                                                         | Notes                                                 |
| ------------------ | ------------------------------------------------------------------------------- | ----------------------------------------------------- |
| Create pane        | `tmux split-window -h -t <target> -P -F '#{pane_id}' [-e KEY=VAL] <cmd>`       | `-P` prints pane info, `-F` formats output, `-e` passes env |
| Park pane (hide)   | `tmux join-pane -d -s <pane> -t <parking-window>`                               | `-d` prevents focus from following                    |
| Show pane          | `tmux join-pane -h -s <pane> -t <kasmos-window> -l 50%`                         | `-h` horizontal split, `-l` size                      |
| Focus pane         | `tmux select-pane -t <pane>`                                                     | Moves keyboard focus                                  |
| List panes         | `tmux list-panes -s -F '#{pane_id} #{pane_pid} #{pane_dead} #{pane_dead_status}'` | `-s` = session-wide (includes parking)                |
| Kill pane          | `tmux kill-pane -t <pane>`                                                       | Terminates pane and process                           |
| Capture content    | `tmux capture-pane -p -t <pane> -S -`                                            | `-S -` captures from start of history                 |
| Set env var        | `tmux set-environment KASMOS_PANE_<wid> <pane_id>`                               | Session-level, unique key per worker                  |
| Read all env vars  | `tmux show-environment`                                                          | Parse `KASMOS_PANE_*` prefix to discover workers      |
| Unset env var      | `tmux set-environment -u KASMOS_PANE_<wid>`                                      | Clean up stale tags                                   |
| Create window      | `tmux new-window -d -n kasmos-parking -P -F '#{window_id}'`                     | `-d` don't switch, `-P -F` captures ID                |
| Get current pane   | `tmux display-message -p '#{pane_id}'`                                           | Kasmos's own pane ID                                  |
| Get current window | `tmux display-message -p '#{window_id}'`                                         | Kasmos's own window ID (for join-pane target)         |
| Retain dead pane   | `tmux set-option -p -t <pane> remain-on-exit on`                                 | Pane survives after process exit                      |
| Get version        | `tmux -V`                                                                        | Advisory minimum check (2.6+)                         |

## 2. Pane Tagging for Rediscovery

### Decision: Session-level environment variables with unique-per-worker keys

**Rationale**: tmux supports session-level environment variables via `set-environment`.
These persist across pane lifecycles and kasmos restarts. Note: `set-environment -t`
targets a **session** (not a pane), so all env vars exist in a shared session namespace.

**Tagging scheme** (unique key per worker to avoid collisions):
- `KASMOS_PANE_<worker_id>=<pane_id>` - Maps each worker to its tmux pane (e.g., `KASMOS_PANE_w001=%42`)
- `KASMOS_SESSION_ID=<session-id>` - Identifies which kasmos session owns these panes
- `KASMOS_PARKING=<window-id>` - Parking window ID for crash recovery
- `KASMOS_DASHBOARD=<pane-id>` - Dashboard pane ID for layout recovery

On reattach, kasmos reads all `KASMOS_PANE_*` env vars via `show-environment`,
filters by `KASMOS_SESSION_ID` match, then cross-references with `list-panes -s`
to validate which panes still exist.

Cleanup: `set-environment -u <key>` removes stale tags during crash recovery
and graceful shutdown.

**Alternatives considered**:
- tmux pane title (`select-pane -T`): Works but titles are visible to the user and
  can be overwritten by the process running in the pane.
- tmux pane options (3.0+): More elegant but requires tmux 3.0+. Env vars work on 2.x.
- File-based tracking (write pane IDs to a file): Works but fragile if file is stale.

## 3. Worker Exit Detection

### Decision: Poll `tmux list-panes` on existing 1-second tick

**Rationale**: kasmos already has a 1-second tick for duration updates. Adding a
`tmux list-panes` call to the tick handler is zero additional timers. The format
string `#{pane_dead} #{pane_dead_status}` tells us if a pane's process exited and
its exit code. This satisfies SC-004 (status within 2 seconds).

**Polling behavior**:
- On each tick (tmux mode only), run `tmux list-panes -s -F '#{pane_id} #{pane_pid} #{pane_dead} #{pane_dead_status}'`.
  The `-s` flag lists panes across ALL windows in the session (including the parking
  window) in a single call. Without `-s`, parked panes would be missed.
- If `pane_dead` is `1`, emit `paneExitedMsg` with the exit code from `pane_dead_status`.
- The Update handler maps `paneExitedMsg` to the existing `workerExitedMsg` flow.
- For externally killed panes (FR-014), a missing pane ID in the listing means
  the pane was destroyed -- emit `workerKilledMsg`.

**Alternatives considered**:
- tmux hooks (`set-hook after-pane-exit`): Would eliminate polling but hooks are
  global to the tmux session and could conflict with user hooks. Also requires
  a callback mechanism (write to pipe/file) that adds complexity.
- tmux `wait-for` channel: Elegant but requires a per-worker `wait-for` setup
  and a goroutine to monitor the channel. More complex than polling.

## 3a. Pane Retention After Process Exit

### Decision: `remain-on-exit` per-pane option

**Rationale**: By default tmux destroys a pane when its process exits. This breaks
exit detection (`pane_dead` polling) and session ID capture (`capture-pane` on dead
pane). The pane must survive after the worker process exits so kasmos can:
1. See `pane_dead=1` in `list-panes` output (vs the pane disappearing entirely)
2. Run `capture-pane` to extract the session ID from scrollback
3. Show the dead pane's final terminal state to the user

**Approach**: Set `remain-on-exit on` per-pane immediately after spawning:

    tmux set-option -p -t <pane_id> remain-on-exit on

This requires tmux 2.6+ (per-pane options were stabilized here). Since our minimum
version is 2.6+, this is safe.

**Fallback**: If per-pane options fail (older tmux), wrap the command with a shell
wrapper that keeps the pane alive:

    sh -c '<cmd>; EXIT=$?; echo "[exited: $EXIT]"; exec cat'

The trailing `exec cat` keeps the pane alive indefinitely. The exit code marker
provides a visual indicator.

**Cleanup**: Dead-but-retained panes are cleaned up by `TmuxBackend.Cleanup()` on
graceful exit, or by the user manually. During `PollPanes`, dead panes with
`remain-on-exit` show `pane_dead=1` in `list-panes` - exactly what the polling
design depends on.

**Alternatives considered**:
- No retention (default): Pane disappears on exit. All exits look like external kills
  (FR-014). Session ID capture impossible. Rejected.
- Shell wrapper only: Works across all tmux versions but adds shell quoting complexity
  and masks the actual exit code in `pane_dead_status`. Use as fallback only.

## 4. Pane Visibility Management

### Decision: Hidden parking window with join-pane in both directions

**Rationale**: A dedicated hidden tmux window named `kasmos-parking` holds all
non-visible worker panes. Both parking and showing use `join-pane`:
- **Park** (hide): `join-pane -d -s <visible-pane> -t <parking-window>`
- **Show**: `join-pane -h -s <pane> -t <kasmos-window> -l 50%`

Using `join-pane` in both directions means the TmuxCLI interface only needs one
pane-movement method. The `-d` flag on parking prevents focus from chasing the
pane into the hidden window.

**Pane swap sequence** (switching from worker A to worker B):
1. `tmux join-pane -d -s %<pane-A> -t @<parking-window>` (A moves to parking, focus stays)
2. `tmux join-pane -h -s %<pane-B> -t @<kasmos-window> -l 50%` (B joins kasmos window)
3. `tmux select-pane -t <pane-B>` (focus moves to B)

The whole sequence completes in < 100ms.

**Edge case: no active worker** (all workers hidden or none spawned):
- Kasmos pane occupies full width. No right-side pane exists.
- On first worker spawn or selection: `join-pane` creates the split.

**Edge case: narrow terminal**:
- If terminal width < 160 (threshold TBD during implementation), kasmos and worker
  alternate full-width. Worker shown: kasmos pane is at minimum width or hidden.
  Existing fullscreen toggle (`f` key) applies.

**Alternatives considered**:
- break-pane for parking: `break-pane` without `-t` creates a NEW window per park
  operation. With `-t` (tmux 2.4+) it works but adds a second command type. Using
  `join-pane` for both directions is simpler and only requires one interface method.
- swap-pane: Swaps two panes in place. Cleaner for exactly 2 workers but messy
  with N workers (need to track who is in which slot).
- Resize to zero: Some tmux versions don't support zero-width panes.
- Multiple windows: Each worker in its own tmux window. Breaks the "kasmos dashboard
  stays visible" requirement.

## 5. WorkerHandle Interface Extension

### Decision: Add `Interactive() bool` to WorkerHandle interface

**Rationale**: The TUI needs to know whether to call `readWorkerOutput()` and
`waitWorkerCmd()`. Interactive handles don't produce output on a pipe (output goes
to the tmux pane) and don't exit via `Wait()` (exit is detected by polling).

**Interface change**:
```go
type WorkerHandle interface {
    Stdout() io.Reader
    Wait() ExitResult
    Kill(gracePeriod time.Duration) error
    PID() int
    Interactive() bool  // NEW
}
```

**SubprocessBackend**: `Interactive()` returns `false`. No behavior change.
**TmuxBackend**: `Interactive()` returns `true`. `Stdout()` returns `nil`.
`Wait()` blocks on an internal channel that is closed when the tick poller
detects pane death (or pane destruction).

**TUI impact** (in `update.go` after `workerSpawnedMsg`):
```go
if !handle.Interactive() {
    readWorkerOutput(msg.WorkerID, handle.Stdout(), m.program)
    cmds = append(cmds, waitWorkerCmd(msg.WorkerID, handle))
}
// For interactive handles, exit detection happens via tick polling
```

**Alternatives considered**:
- Separate interface (e.g., `InteractiveHandle`): Type assertions throughout TUI code.
  Messy and breaks the single-interface design.
- Nil-check on Stdout(): Fragile. Future backends might have different Stdout semantics.

## 6. Session ID Extraction in Tmux Mode

### Decision: `tmux capture-pane` on worker exit, apply existing regex

**Rationale**: The existing `extractSessionID()` function parses output text with
a regex (`session:\s+(ses_[a-zA-Z0-9]+)`). In tmux mode, we capture the pane
content on exit with `tmux capture-pane -p -t <pane> -S -` (full scrollback)
and pass it to the same function. One-time capture, no continuous monitoring.

**When captured**: After `paneExitedMsg` is received, before emitting `workerExitedMsg`.
The capture content is also stored in the worker's `OutputBuffer` for persistence
(output_tail in session.json).

**Limitation**: If the pane scrollback is shorter than the session ID output
(e.g., huge output pushed it out), the session ID won't be found. This is the
same limitation as subprocess mode with a full ring buffer. Acceptable.

## 7. tmux Version Requirements

### Decision: Minimum tmux 2.6+ (September 2017)

**Rationale**: All commands used in this feature are available since tmux 2.6:
- `split-window`, `join-pane`, `break-pane`: Available since tmux 1.x
- `list-panes -F` with `pane_dead` and `pane_dead_status`: Available since 2.6
- `set-environment` / `show-environment`: Available since 1.x
- `capture-pane -p`: Available since 1.8
- `display-message -p`: Available since 1.x

tmux 2.6 is from September 2017. Any modern Linux distribution ships 3.x+.
This is not a practical concern.

**Version detection**: At TmuxBackend initialization, run `tmux -V` and parse
the version string. Warn (not error) if below 2.6.

## 8. bubbletea + tmux Interaction

### Decision: No conflicts expected; kasmos pane uses alt-screen normally

**Rationale**: kasmos already runs in alt-screen mode (`tea.WithAltScreen()`).
When running inside a tmux pane, bubbletea renders to that pane's PTY. This is
standard behavior -- countless bubbletea apps run inside tmux. The worker panes
are separate PTYs managed by tmux and are not affected by kasmos's rendering.

**Key handling**: tmux intercepts its prefix key (default `Ctrl-b`) before it
reaches bubbletea. This is desirable -- it's how users return focus to the
kasmos pane. No keybinding conflicts with kasmos's key map.

**Signal handling**: `SIGWINCH` (terminal resize) is delivered per-pane by tmux.
bubbletea handles this via `tea.WindowSizeMsg`. Works correctly in tmux panes.
