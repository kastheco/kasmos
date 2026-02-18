---
work_package_id: WP12
title: Daemon Mode (Headless Operation)
lane: done
dependencies:
- WP04
subtasks:
- internal/tui/daemon.go - Daemon event logging (NDJSON + human-readable)
- cmd/kasmos/main.go - -d flag, --format flag, TTY detection
- tea.WithoutRenderer() setup for daemon mode
- DaemonEvent types and formatting
- Exit code logic (0 if all pass, 1 if any fail)
- Integration with --spawn-all for batch execution
phase: Wave 3 - Daemon Mode + Persistence
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
- timestamp: '2026-02-18T14:52:47.582546632+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP12 coder - daemon mode)
- timestamp: '2026-02-18T14:59:22.317571864+00:00'
  lane: done
  actor: manager
  shell_pid: '472734'
  action: transition done (Implemented - daemon mode with event logging + batch execution)
---

# Work Package Prompt: WP12 - Daemon Mode (Headless Operation)

## Mission

Implement daemon mode: when kasmos runs with `-d` flag or in a non-interactive
terminal, it operates headless -- spawning workers and logging status to stdout
as structured events without rendering a TUI. Supports NDJSON and human-readable
output formats. This delivers User Story 7 (Daemon Mode for Headless Operation).

## Scope

### Files to Create

```
internal/tui/daemon.go       # DaemonEvent types, formatters, logEvent()
```

### Files to Modify

```
cmd/kasmos/main.go           # -d flag, --format flag, --spawn-all flag, TTY detection
internal/tui/model.go        # Daemon mode state, conditional View()
internal/tui/update.go       # Emit daemon events on state changes
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 6**: Daemon mode output format, NDJSON schema, human-readable format,
    implementation notes (lines 908-1027)
- `design-artifacts/tui-mockups.md`:
  - **V10**: Daemon mode output examples (lines 406-434)
- `kitty-specs/016-kasmos-agent-orchestrator/spec.md`:
  - User Story 7 acceptance scenarios (lines 107-119)
- `.kittify/memory/constitution.md`:
  - Daemon mode: same Model/Update loop, no View rendering (line 47)

## Implementation

### daemon.go

Define DaemonEvent types and formatting:

**Event types** (from tui-technical.md Section 6):
- `session_start`: session_id, mode, source path, task count
- `worker_spawn`: worker id, role, task ref, parent ref
- `worker_output`: worker id, data (optional, can be noisy)
- `worker_exit`: worker id, exit code, duration, session id
- `worker_kill`: worker id
- `analysis_complete`: worker id, root cause summary
- `session_end`: total workers, passed, failed, duration, exit code

**JSON format** (NDJSON -- one JSON object per line):
```go
type DaemonEvent struct {
    Timestamp time.Time              `json:"ts"`
    Event     string                 `json:"event"`
    Data      map[string]interface{} `json:"-"` // merged into top level
}
```

Each event marshals as a flat JSON object:
```json
{"ts":"2026-02-17T14:28:01Z","event":"worker_spawn","id":"w-001","role":"coder","task":"Implement auth"}
```

**Human-readable format** (default):
```
[14:28:01] w-001 spawned   coder     "Implement auth"
[14:30:12] w-003 exited(0) reviewer  2m 11s  ses_k2m9
[14:34:02] session ended: 3 passed, 1 failed (6m 02s) exit=1
```

**logEvent function**:
```go
func (m Model) logEvent(event DaemonEvent) {
    if m.daemonFormat == "json" {
        b, _ := json.Marshal(event)
        fmt.Println(string(b))
    } else {
        fmt.Println(event.HumanString())
    }
}
```

### Daemon Mode View

In daemon mode, `View()` returns empty string:
```go
func (m Model) View() string {
    if m.daemon {
        return ""  // no rendering
    }
    // ... normal TUI rendering
}
```

The Update loop is IDENTICAL in TUI and daemon mode. State changes that would
update the TUI instead call `logEvent()` in the Update handlers.

Add `logEvent()` calls to existing Update handlers:
- `workerSpawnedMsg` -> log worker_spawn event
- `workerExitedMsg` -> log worker_exit event
- `workerKilledMsg` -> log worker_kill event
- Session start (Init) -> log session_start event

### Session End Logic

Track when all workers have completed. In daemon mode, when no workers are
running and no more tasks to spawn:
1. Calculate aggregate stats: total, passed (exit 0), failed (exit != 0)
2. Calculate total session duration
3. Log session_end event
4. Return tea.Quit with exit code: 0 if all passed, 1 if any failed

For `--spawn-all` mode: exit after all spawned workers complete.
Without `--spawn-all`: exit after all workers complete (user must spawn via
task source or have pre-spawned).

### CLI Flags (main.go)

Add flags to cobra root command:
```go
rootCmd.Flags().BoolVarP(&daemon, "daemon", "d", false, "Run in headless daemon mode")
rootCmd.Flags().StringVar(&format, "format", "default", "Output format: default or json")
rootCmd.Flags().Bool("spawn-all", false, "Spawn workers for all tasks immediately")
```

**TTY detection** (auto-daemon):
```go
if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsCygwinTerminal(os.Stdout.Fd()) {
    daemon = true
}
```

**Program setup**:
```go
var opts []tea.ProgramOption
if daemon {
    opts = append(opts, tea.WithoutRenderer())
} else {
    opts = append(opts, tea.WithAltScreen())
}
p := tea.NewProgram(model, opts...)
```

### --spawn-all Flag

When `--spawn-all` is set (typically with daemon mode and a task source):
1. After loading tasks, spawn a worker for each unblocked task
2. Use suggested role and description as prompt
3. As workers complete and blocked tasks become unblocked, spawn those too
4. Exit when all tasks are done or failed

This creates a batch execution pipeline perfect for CI/CD.

### SIGPIPE Handling

In daemon mode, stdout may be a pipe. Handle SIGPIPE gracefully:
```go
signal.Ignore(syscall.SIGPIPE)
```
Continue managing workers even if stdout pipe breaks.

## What NOT to Do

- Do NOT log worker_output events by default (too noisy). Maybe add a `--verbose` flag later.
- Do NOT implement interactive input in daemon mode (no stdin reading)
- Do NOT implement worker output capture differently for daemon mode (same OutputBuffer)
- Do NOT make daemon mode exit immediately on first failure (wait for all workers)

## Acceptance Criteria

1. `kasmos -d --format json` outputs NDJSON events to stdout
2. `kasmos -d` (default format) outputs human-readable log lines
3. Exit code is 0 if all workers passed, 1 if any failed
4. `kasmos -d --tasks tasks.md --spawn-all` spawns all tasks and exits when done
5. Non-interactive terminal auto-detects daemon mode
6. Session end event shows aggregate stats
7. SIGPIPE doesn't crash kasmos (daemon stdout to a broken pipe)
8. Same Update loop drives both TUI and daemon mode (no code duplication)
9. `go test ./...` passes
