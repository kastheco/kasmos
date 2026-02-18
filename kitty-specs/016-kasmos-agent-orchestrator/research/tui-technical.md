# kasmos TUI — Technical Research Artifacts

> Companion to the visual design artifacts in `design-artifacts/`. This document
> defines Go interfaces, message types, JSON schemas, and behavioral contracts
> that a coder agent can implement directly.

## Table of Contents

- [1. WorkerBackend Interface](#1-workerbackend-interface)
- [2. bubbletea Message Type Catalog](#2-bubbletea-message-type-catalog)
- [3. Core Domain Types](#3-core-domain-types)
- [4. Task Source Interface](#4-task-source-interface)
- [5. Session Persistence Schema](#5-session-persistence-schema)
- [6. Daemon Mode Output Format](#6-daemon-mode-output-format)
- [7. Output Buffer Design](#7-output-buffer-design)
- [8. Graceful Shutdown Protocol](#8-graceful-shutdown-protocol)
- [9. OpenCode Integration Contract](#9-opencode-integration-contract)
- [10. Package Structure](#10-package-structure)

---

## 1. WorkerBackend Interface

The `WorkerBackend` abstracts how worker processes are created, managed, and
observed. The MVP uses `SubprocessBackend` (os/exec). A future `TmuxBackend`
can be added without touching the TUI layer.

### Interface Definition

```go
package worker

import (
    "context"
    "io"
)

// WorkerBackend abstracts the mechanism for running worker processes.
// The TUI and worker manager interact only through this interface.
type WorkerBackend interface {
    // Spawn starts a new worker process. Returns a WorkerHandle for
    // lifecycle management. The context controls cancellation.
    // SpawnConfig contains all parameters needed to start the worker.
    Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error)

    // Name returns the backend identifier (e.g., "subprocess", "tmux").
    Name() string
}

// SpawnConfig contains everything needed to start a worker.
type SpawnConfig struct {
    // ID is the kasmos-assigned worker identifier (e.g., "w-001").
    ID string

    // Role is the agent role (planner, coder, reviewer, release).
    Role string

    // Prompt is the task description sent to the agent.
    Prompt string

    // Files is an optional list of file paths to attach via --file flags.
    Files []string

    // ContinueSession, if non-empty, is the OpenCode session ID to continue.
    // Triggers --continue -s <id> flag.
    ContinueSession string

    // Model overrides the default model for this worker (optional).
    // Maps to --model flag.
    Model string

    // WorkDir is the working directory for the worker process.
    // Defaults to the project root if empty.
    WorkDir string

    // Env is additional environment variables for the worker process.
    Env map[string]string
}

// WorkerHandle provides lifecycle control over a running worker.
type WorkerHandle interface {
    // Stdout returns a reader for the worker's combined stdout/stderr stream.
    // The reader is closed when the process exits.
    Stdout() io.Reader

    // Wait blocks until the worker process exits. Returns the exit result.
    Wait() ExitResult

    // Kill sends SIGTERM to the worker process. If the process doesn't exit
    // within the grace period, sends SIGKILL.
    Kill(gracePeriod time.Duration) error

    // PID returns the OS process ID, or 0 if not applicable (e.g., tmux).
    PID() int
}

// ExitResult contains the outcome of a completed worker process.
type ExitResult struct {
    // Code is the process exit code. 0 = success, non-zero = failure.
    Code int

    // Duration is how long the worker ran.
    Duration time.Duration

    // SessionID is the OpenCode session ID extracted from worker output.
    // Empty if not found (output parsing failed or worker crashed early).
    SessionID string

    // Error is set if the process couldn't be started or was killed.
    Error error
}
```

### SubprocessBackend (MVP Implementation)

```go
package worker

import (
    "context"
    "io"
    "os/exec"
    "syscall"
    "time"
)

type SubprocessBackend struct {
    // OpenCodeBin is the path to the opencode binary.
    // Resolved once at startup via exec.LookPath.
    OpenCodeBin string
}

func NewSubprocessBackend() (*SubprocessBackend, error) {
    bin, err := exec.LookPath("opencode")
    if err != nil {
        return nil, fmt.Errorf("opencode not found in PATH: %w", err)
    }
    return &SubprocessBackend{OpenCodeBin: bin}, nil
}

func (b *SubprocessBackend) Name() string { return "subprocess" }

func (b *SubprocessBackend) Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error) {
    args := b.buildArgs(cfg)
    cmd := exec.CommandContext(ctx, b.OpenCodeBin, args...)
    cmd.Dir = cfg.WorkDir
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // own process group

    // Merge stdout + stderr into a single pipe
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("stdout pipe: %w", err)
    }
    cmd.Stderr = cmd.Stdout // merge stderr into stdout

    // Inject environment
    cmd.Env = os.Environ()
    for k, v := range cfg.Env {
        cmd.Env = append(cmd.Env, k+"="+v)
    }

    startTime := time.Now()
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("start opencode: %w", err)
    }

    return &subprocessHandle{
        cmd:       cmd,
        stdout:    stdout,
        startTime: startTime,
        cfg:       cfg,
    }, nil
}

func (b *SubprocessBackend) buildArgs(cfg SpawnConfig) []string {
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
    for _, f := range cfg.Files {
        args = append(args, "--file", f)
    }

    // Prompt is always the last argument
    if cfg.Prompt != "" {
        args = append(args, cfg.Prompt)
    }

    return args
}
```

### Future TmuxBackend (Interface Only)

```go
// TmuxBackend would implement WorkerBackend by creating tmux sessions
// instead of bare subprocesses. This enables:
// - Scrollback persistence managed by tmux
// - Attaching to running workers interactively
// - Surviving kasmos process death (tmux keeps running)
//
// Not implemented in MVP. Listed here to validate the interface design.
type TmuxBackend struct {
    SessionPrefix string // e.g., "kasmos-"
}
```

---

## 2. bubbletea Message Type Catalog

All messages that flow through the `Update` loop. Organized by source.

### Worker Lifecycle Messages

```go
package tui

import "time"

// workerSpawnedMsg is sent when a worker process starts successfully.
type workerSpawnedMsg struct {
    WorkerID string
    PID      int
}

// workerOutputMsg carries a chunk of output from a running worker.
// Sent by the output reader goroutine via tea.Program.Send().
type workerOutputMsg struct {
    WorkerID string
    Data     string // may contain multiple lines
}

// workerExitedMsg is sent when a worker process terminates.
type workerExitedMsg struct {
    WorkerID  string
    ExitCode  int
    Duration  time.Duration
    SessionID string // OpenCode session ID (parsed from output)
    Err       error  // non-nil if process failed to start or was killed
}

// workerKilledMsg is sent when a user-initiated kill completes.
type workerKilledMsg struct {
    WorkerID string
    Err      error // non-nil if kill failed
}
```

### Worker Command Messages (user-initiated → side effects)

```go
// spawnWorkerCmd returns a tea.Cmd that spawns a worker and sends
// workerSpawnedMsg on success or workerExitedMsg on failure.
func spawnWorkerCmd(backend WorkerBackend, cfg SpawnConfig) tea.Cmd

// killWorkerCmd returns a tea.Cmd that kills a worker and sends
// workerKilledMsg when done.
func killWorkerCmd(handle WorkerHandle, gracePeriod time.Duration) tea.Cmd

// readOutputCmd returns a tea.Cmd that reads from a worker's stdout
// and sends workerOutputMsg chunks. Loops until EOF, then sends
// nothing (the workerExitedMsg comes from the Wait goroutine).
func readOutputCmd(workerID string, reader io.Reader) tea.Cmd
```

### UI State Messages

```go
// tickMsg is sent by a periodic timer for duration updates.
// The TUI recalculates running worker durations on each tick.
type tickMsg time.Time

// Tick interval: 1 second. Started in Init(), restarted after each tick.
func tickCmd() tea.Cmd {
    return tea.Tick(time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}

// spinnerTickMsg is the spinner animation tick.
// Handled by forwarding to spinner.Model.Update().
// Uses spinner's built-in tick mechanism.

// focusChangedMsg is sent internally when panel focus changes.
// Triggers key state recalculation and viewport title update.
type focusChangedMsg struct {
    From panel
    To   panel
}

// layoutChangedMsg is sent when a resize crosses a breakpoint boundary.
type layoutChangedMsg struct {
    From layoutMode
    To   layoutMode
}
```

### Overlay/Dialog Messages

```go
// spawnDialogSubmittedMsg carries the completed spawn form data.
type spawnDialogSubmittedMsg struct {
    Role   string
    Prompt string
    Files  []string
    TaskID string // empty for ad-hoc
}

// spawnDialogCancelledMsg indicates the user dismissed the spawn dialog.
type spawnDialogCancelledMsg struct{}

// continueDialogSubmittedMsg carries the follow-up message for continuation.
type continueDialogSubmittedMsg struct {
    ParentWorkerID string
    SessionID      string
    FollowUp       string
}

// continueDialogCancelledMsg indicates the user dismissed the continue dialog.
type continueDialogCancelledMsg struct{}

// quitConfirmedMsg indicates the user confirmed quit with running workers.
type quitConfirmedMsg struct{}

// quitCancelledMsg indicates the user cancelled the quit dialog.
type quitCancelledMsg struct{}
```

### AI Helper Messages

```go
// analyzeStartedMsg indicates failure analysis has begun.
type analyzeStartedMsg struct {
    WorkerID string
}

// analyzeCompletedMsg carries the analysis result from the AI helper.
type analyzeCompletedMsg struct {
    WorkerID       string
    RootCause      string
    SuggestedPrompt string
    Err            error
}

// genPromptStartedMsg indicates prompt generation has begun.
type genPromptStartedMsg struct {
    TaskID string
}

// genPromptCompletedMsg carries the generated prompt.
type genPromptCompletedMsg struct {
    TaskID  string
    Prompt  string
    Err     error
}
```

### Task Source Messages

```go
// tasksLoadedMsg carries parsed tasks from a task source.
type tasksLoadedMsg struct {
    Source string // "spec-kitty", "gsd", "ad-hoc"
    Path   string
    Tasks  []Task
    Err    error
}

// taskStateChangedMsg is sent when a task's state changes
// (e.g., unassigned → in-progress when a worker is spawned for it).
type taskStateChangedMsg struct {
    TaskID   string
    NewState TaskState
    WorkerID string // the worker that caused the change
}
```

### Session Persistence Messages

```go
// sessionSavedMsg confirms session state was persisted to disk.
type sessionSavedMsg struct {
    Path string
    Err  error
}

// sessionLoadedMsg carries restored session state on startup/reattach.
type sessionLoadedMsg struct {
    Session *SessionState
    Err     error
}
```

### Message Flow Diagram

```
User Input (tea.KeyMsg)
  │
  ├── 's' key ──────────────────────→ Show spawn dialog
  │                                      │
  │                                      ├── Submit ──→ spawnDialogSubmittedMsg
  │                                      │                │
  │                                      │                └──→ spawnWorkerCmd()
  │                                      │                       │
  │                                      │                       ├──→ workerSpawnedMsg
  │                                      │                       │      │
  │                                      │                       │      └──→ readOutputCmd()
  │                                      │                       │             │
  │                                      │                       │             └──→ workerOutputMsg (loop)
  │                                      │                       │
  │                                      │                       └──→ waitCmd() ──→ workerExitedMsg
  │                                      │
  │                                      └── Cancel ──→ spawnDialogCancelledMsg
  │
  ├── 'x' key ──────────────────────→ killWorkerCmd() ──→ workerKilledMsg
  │
  ├── 'c' key ──────────────────────→ Show continue dialog
  │                                      │
  │                                      └── Submit ──→ continueDialogSubmittedMsg
  │                                                       │
  │                                                       └──→ spawnWorkerCmd(cfg with ContinueSession)
  │
  ├── 'a' key ──────────────────────→ analyzeCmd() ──→ analyzeStartedMsg
  │                                                       │
  │                                                       └──→ analyzeCompletedMsg
  │
  └── 'g' key ──────────────────────→ genPromptCmd() ──→ genPromptStartedMsg
                                                            │
                                                            └──→ genPromptCompletedMsg

Timer
  └── tea.Tick(1s) ─────────────────→ tickMsg (duration refresh)

Spinner
  └── spinner.Tick ─────────────────→ spinner.TickMsg (animation frame)

Window
  └── terminal resize ──────────────→ tea.WindowSizeMsg ──→ recalculateLayout()
```

---

## 3. Core Domain Types

```go
package worker

import "time"

// Worker represents a managed agent session.
type Worker struct {
    // Identity
    ID       string // "w-001", "w-002", etc. Auto-incrementing.
    Role     string // "planner", "coder", "reviewer", "release"
    Prompt   string // The task prompt sent to the agent.
    Files    []string // Attached file paths.

    // Lifecycle
    State     WorkerState
    ExitCode  int
    SpawnedAt time.Time
    ExitedAt  time.Time // zero if still running

    // OpenCode integration
    SessionID string // OpenCode session ID, parsed from output.

    // Relationships
    ParentID string // ID of parent worker (for continuations). Empty if root.
    TaskID   string // Associated task ID from task source. Empty for ad-hoc.

    // Runtime (not persisted)
    Handle WorkerHandle // nil after process exit
    Output *OutputBuffer // ring buffer of captured output
}

// Duration returns how long the worker has been running (or ran).
func (w *Worker) Duration() time.Duration {
    if w.ExitedAt.IsZero() {
        if w.SpawnedAt.IsZero() {
            return 0
        }
        return time.Since(w.SpawnedAt)
    }
    return w.ExitedAt.Sub(w.SpawnedAt)
}

// FormatDuration returns a human-readable duration string.
// Examples: "4m 12s", "0m 34s", "1h 2m", "—" (for pending).
func (w *Worker) FormatDuration() string {
    if w.State == StatePending || w.State == StateSpawning {
        return "  —  "
    }
    d := w.Duration()
    if d < time.Hour {
        return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
    }
    return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// Children returns IDs of workers that continued from this worker.
func (w *Worker) Children(all []*Worker) []string {
    var ids []string
    for _, other := range all {
        if other.ParentID == w.ID {
            ids = append(ids, other.ID)
        }
    }
    return ids
}
```

### Worker State Machine

```
StatePending ──→ StateSpawning ──→ StateRunning ──┬──→ StateExited (code 0)
                                                   ├──→ StateFailed (code != 0)
                                                   └──→ StateKilled (user kill)

From StateExited/StateFailed/StateKilled:
  ├──→ Continue (spawns new worker with ParentID set)
  └──→ Restart  (spawns new worker with same role/prompt, no ParentID)
```

### Worker ID Generation

```go
package worker

import (
    "fmt"
    "sync/atomic"
)

var workerCounter atomic.Int64

// NextWorkerID generates the next sequential worker ID.
// Format: "w-NNN" where NNN is zero-padded to 3 digits.
// Thread-safe via atomic increment.
func NextWorkerID() string {
    n := workerCounter.Add(1)
    return fmt.Sprintf("w-%03d", n)
}

// ResetWorkerCounter sets the counter to a value (for session restore).
func ResetWorkerCounter(n int64) {
    workerCounter.Store(n)
}
```

---

## 4. Task Source Interface

```go
package task

// Source is a pluggable adapter that reads work items from external files.
type Source interface {
    // Type returns the source kind: "spec-kitty", "gsd", or "ad-hoc".
    Type() string

    // Path returns the file/directory path this source reads from.
    // Empty for ad-hoc.
    Path() string

    // Load reads and parses the task source. Returns parsed tasks.
    // Called once at startup and can be called again to refresh.
    Load() ([]Task, error)

    // Tasks returns the currently loaded tasks (cached from last Load).
    Tasks() []Task
}

// Task represents a single work item from a task source.
type Task struct {
    // ID is the task identifier. Format depends on source:
    // spec-kitty: "WP-001", GSD: "T-001", ad-hoc: empty.
    ID string

    // Title is the short task name.
    Title string

    // Description is the detailed task description.
    // Used as the default prompt when spawning a worker for this task.
    Description string

    // SuggestedRole is the recommended agent role for this task.
    // Parsed from plan.md metadata or inferred from task content.
    SuggestedRole string

    // Dependencies is a list of task IDs this task depends on.
    // A task is "blocked" if any dependency is not in TaskDone state.
    Dependencies []string

    // State tracks the task's current assignment status.
    State TaskState

    // WorkerID is the ID of the worker assigned to this task.
    // Empty if unassigned.
    WorkerID string

    // Metadata carries source-specific extra data.
    // spec-kitty: phase, lane, subtasks
    // GSD: checkbox state, line number
    Metadata map[string]string
}
```

### SpecKittySource Implementation Notes

```go
package task

// SpecKittySource reads a spec-kitty feature directory.
// It parses plan.md (markdown) and tasks/WP*.md (YAML frontmatter + markdown).
//
// Feature directory structure:
//   kitty-specs/<number>-<slug>/
//     spec.md        - feature specification (not parsed for tasks)
//     plan.md        - implementation plan (parsed for WP summaries)
//     tasks/
//       WP-001.md    - work package with YAML frontmatter
//       WP-002.md    - work package with YAML frontmatter
//
// WP frontmatter format (from the old kasmos Rust parser):
//   ---
//   work_package_id: WP-001
//   title: Auth middleware
//   dependencies: []
//   lane: planned          # planned | doing | for_review | done
//   subtasks: []
//   phase: implementation
//   ---
//   <markdown body = detailed description>
//
// Task.ID = work_package_id
// Task.Title = title
// Task.Description = markdown body
// Task.Dependencies = dependencies
// Task.SuggestedRole = inferred from phase:
//   - "spec" / "clarifying" → "planner"
//   - "implementation" → "coder"
//   - "reviewing" → "reviewer"
//   - "releasing" → "release"
// Task.State = mapped from lane:
//   - "planned" → TaskUnassigned
//   - "doing" → TaskInProgress
//   - "for_review" → TaskInProgress
//   - "done" → TaskDone
type SpecKittySource struct {
    Dir string // feature directory path
}
```

### GsdSource Implementation Notes

```go
// GsdSource reads a simple markdown task file.
// Format: a markdown file with checkboxes.
//
// Example tasks.md:
//   # Sprint 12 Tasks
//
//   - [ ] Implement auth middleware
//   - [ ] Fix login flow
//   - [x] Review PR #42
//   - [ ] Plan DB schema migration
//
// Task.ID = "T-NNN" (sequential from line order)
// Task.Title = checkbox text
// Task.Description = same as title (no separate description)
// Task.SuggestedRole = "" (user selects at spawn time)
// Task.Dependencies = [] (GSD doesn't track dependencies)
// Task.State = TaskDone if [x], TaskUnassigned if [ ]
type GsdSource struct {
    FilePath string
}
```

### AdHocSource Implementation Notes

```go
// AdHocSource is the zero-value source for manual orchestration.
// It has no file, no tasks. Workers are spawned with manual prompts.
//
// Source.Type() = "ad-hoc"
// Source.Path() = ""
// Source.Load() = nil, nil
// Source.Tasks() = []
type AdHocSource struct{}
```

---

## 5. Session Persistence Schema

Session state is persisted to `.kasmos/session.json` in the project root.
Written after every state change (debounced to at most once per second).
Read at startup for `kasmos --attach` reattach.

### JSON Schema

```json
{
  "$schema": "https://json-schema.org/draft/2024-12/schema",
  "title": "kasmos Session State",
  "type": "object",
  "required": ["version", "session_id", "started_at", "workers"],
  "properties": {
    "version": {
      "type": "integer",
      "const": 1,
      "description": "Schema version for forward compatibility."
    },
    "session_id": {
      "type": "string",
      "description": "Unique session identifier. Format: ks-<unix_timestamp>-<random4>.",
      "pattern": "^ks-[0-9]+-[a-z0-9]{4}$",
      "examples": ["ks-1739808000-a3f8"]
    },
    "started_at": {
      "type": "string",
      "format": "date-time",
      "description": "When the session was created (RFC3339)."
    },
    "task_source": {
      "type": ["object", "null"],
      "description": "The task source configuration. Null for ad-hoc mode.",
      "properties": {
        "type": {
          "type": "string",
          "enum": ["spec-kitty", "gsd", "ad-hoc"]
        },
        "path": {
          "type": "string",
          "description": "Absolute path to the task source file/directory."
        }
      },
      "required": ["type", "path"]
    },
    "workers": {
      "type": "array",
      "description": "All workers in this session (active and historical).",
      "items": {
        "$ref": "#/$defs/worker"
      }
    },
    "next_worker_num": {
      "type": "integer",
      "minimum": 1,
      "description": "Next worker ID number to assign. Ensures IDs never collide across restarts."
    },
    "pid": {
      "type": "integer",
      "description": "PID of the kasmos process that owns this session. Used for reattach detection."
    }
  },
  "$defs": {
    "worker": {
      "type": "object",
      "required": ["id", "role", "prompt", "state", "spawned_at"],
      "properties": {
        "id": {
          "type": "string",
          "pattern": "^w-[0-9]{3,}$",
          "examples": ["w-001", "w-042"]
        },
        "role": {
          "type": "string",
          "enum": ["planner", "coder", "reviewer", "release"]
        },
        "prompt": {
          "type": "string",
          "description": "The prompt sent to the agent."
        },
        "files": {
          "type": "array",
          "items": { "type": "string" },
          "description": "File paths attached to the agent invocation."
        },
        "state": {
          "type": "string",
          "enum": ["pending", "spawning", "running", "exited", "failed", "killed"]
        },
        "exit_code": {
          "type": ["integer", "null"],
          "description": "Process exit code. Null if not yet exited."
        },
        "spawned_at": {
          "type": "string",
          "format": "date-time"
        },
        "exited_at": {
          "type": ["string", "null"],
          "format": "date-time",
          "description": "When the worker exited. Null if still running."
        },
        "duration_ms": {
          "type": ["integer", "null"],
          "description": "Worker duration in milliseconds. Null if still running."
        },
        "session_id": {
          "type": "string",
          "description": "OpenCode session ID. Empty if not yet captured."
        },
        "parent_id": {
          "type": "string",
          "description": "ID of the parent worker for continuations. Empty if root."
        },
        "task_id": {
          "type": "string",
          "description": "Associated task ID from task source. Empty for ad-hoc."
        },
        "pid": {
          "type": ["integer", "null"],
          "description": "OS process ID. Null if not running."
        },
        "output_tail": {
          "type": "string",
          "description": "Last N lines of output (for display on reattach). Truncated to 200 lines max.",
          "maxLength": 50000
        }
      }
    }
  }
}
```

### Example Session File

```json
{
  "version": 1,
  "session_id": "ks-1739808000-a3f8",
  "started_at": "2026-02-17T14:00:00Z",
  "task_source": {
    "type": "spec-kitty",
    "path": "/home/kas/dev/project/kitty-specs/015-auth-overhaul/"
  },
  "workers": [
    {
      "id": "w-001",
      "role": "coder",
      "prompt": "Implement the auth middleware as described in WP-001...",
      "files": [],
      "state": "exited",
      "exit_code": 0,
      "spawned_at": "2026-02-17T14:01:15Z",
      "exited_at": "2026-02-17T14:05:27Z",
      "duration_ms": 252000,
      "session_id": "ses_j4m9x1",
      "parent_id": "",
      "task_id": "WP-001",
      "pid": null,
      "output_tail": "[14:05:27] Done. Auth middleware implemented and tested.\n"
    },
    {
      "id": "w-002",
      "role": "reviewer",
      "prompt": "Review the auth middleware implementation...",
      "files": [],
      "state": "running",
      "exit_code": null,
      "spawned_at": "2026-02-17T14:06:00Z",
      "exited_at": null,
      "duration_ms": null,
      "session_id": "",
      "parent_id": "",
      "task_id": "WP-001",
      "pid": 48291,
      "output_tail": ""
    }
  ],
  "next_worker_num": 3,
  "pid": 47100
}
```

### Persistence Behavior

```go
// SessionPersister manages session state serialization.
type SessionPersister struct {
    Path     string        // ".kasmos/session.json"
    Debounce time.Duration // 1 second
}

// Behavior:
// - Save() is called after every state-mutating message in Update().
// - Writes are debounced: at most one write per second.
// - Uses atomic write: write to .tmp, then rename.
// - On startup: if session.json exists and pid is alive → reattach mode.
// - On startup: if session.json exists and pid is dead → restore state,
//   mark running workers as "killed" (orphaned), assign new PID.
// - On clean exit: update all worker states, write final session.json.
```

---

## 6. Daemon Mode Output Format

Two output modes: `--format default` (human-readable) and `--format json` (machine-parseable).

### JSON Format (NDJSON)

One JSON object per line. All events share a common envelope.

```json
{
  "$schema": "https://json-schema.org/draft/2024-12/schema",
  "title": "kasmos Daemon Event",
  "type": "object",
  "required": ["ts", "event"],
  "properties": {
    "ts": {
      "type": "string",
      "format": "date-time",
      "description": "Event timestamp (RFC3339)."
    },
    "event": {
      "type": "string",
      "enum": [
        "session_start",
        "worker_spawn",
        "worker_output",
        "worker_exit",
        "worker_kill",
        "analysis_complete",
        "session_end"
      ]
    }
  },
  "allOf": [
    {
      "if": { "properties": { "event": { "const": "session_start" } } },
      "then": {
        "properties": {
          "session_id": { "type": "string" },
          "mode": { "type": "string", "enum": ["spec-kitty", "gsd", "ad-hoc"] },
          "source": { "type": "string" },
          "tasks": { "type": "integer" }
        },
        "required": ["session_id", "mode"]
      }
    },
    {
      "if": { "properties": { "event": { "const": "worker_spawn" } } },
      "then": {
        "properties": {
          "id": { "type": "string" },
          "role": { "type": "string" },
          "task": { "type": "string" },
          "parent": { "type": "string" }
        },
        "required": ["id", "role"]
      }
    },
    {
      "if": { "properties": { "event": { "const": "worker_exit" } } },
      "then": {
        "properties": {
          "id": { "type": "string" },
          "code": { "type": "integer" },
          "duration": { "type": "string" },
          "session": { "type": "string" }
        },
        "required": ["id", "code", "duration"]
      }
    },
    {
      "if": { "properties": { "event": { "const": "session_end" } } },
      "then": {
        "properties": {
          "total": { "type": "integer" },
          "passed": { "type": "integer" },
          "failed": { "type": "integer" },
          "duration": { "type": "string" },
          "exit_code": { "type": "integer" }
        },
        "required": ["total", "passed", "failed", "exit_code"]
      }
    }
  ]
}
```

### Human-Readable Format (default)

```
[14:28:00] session started (gsd, tasks.md, 4 tasks)
[14:28:01] w-001 spawned   coder     "Implement auth"
[14:28:01] w-002 spawned   coder     "Fix login flow"
[14:28:01] w-003 spawned   reviewer  "Review PR #42"
[14:28:01] w-004 spawned   planner   "Plan DB schema"
[14:30:12] w-003 exited(0) reviewer  2m 11s  ses_k2m9
[14:32:14] w-001 exited(0) coder     4m 13s  ses_j4m9
[14:33:01] w-004 exited(0) planner   5m 00s  ses_m7x2
[14:34:02] w-002 exited(1) coder     6m 01s  ses_p1q3  ← FAILED
[14:34:02] session ended: 3 passed, 1 failed (6m 02s) exit=1
```

### Implementation Notes

```go
// In daemon mode, the View() function returns "" (empty).
// State changes that would update the TUI instead emit log lines:

func (m Model) logEvent(event DaemonEvent) {
    if m.format == "json" {
        b, _ := json.Marshal(event)
        fmt.Println(string(b))
    } else {
        fmt.Println(event.HumanString())
    }
}

// The Update() loop is identical in TUI and daemon mode.
// Only the output side differs: View() vs logEvent().
```

---

## 7. Output Buffer Design

Each worker has an `OutputBuffer` that accumulates stdout/stderr data. The buffer
has a configurable max line count to prevent unbounded memory growth.

```go
package worker

import "sync"

// OutputBuffer is a thread-safe ring buffer of output lines.
// It preserves the last MaxLines lines and discards older ones.
type OutputBuffer struct {
    mu       sync.RWMutex
    lines    []string
    maxLines int
    total    int // total lines ever added (for "N lines truncated" display)
}

// DefaultMaxLines is the default output buffer size per worker.
const DefaultMaxLines = 5000

func NewOutputBuffer(maxLines int) *OutputBuffer {
    if maxLines <= 0 {
        maxLines = DefaultMaxLines
    }
    return &OutputBuffer{
        lines:    make([]string, 0, min(maxLines, 1024)),
        maxLines: maxLines,
    }
}

// Append adds raw data to the buffer. Data may contain multiple lines
// (split on \n). Non-UTF8 bytes are replaced with U+FFFD.
func (b *OutputBuffer) Append(data string)

// Lines returns all buffered lines (oldest first).
func (b *OutputBuffer) Lines() []string

// Content returns all buffered lines joined with \n.
// This is what gets set on viewport.SetContent().
func (b *OutputBuffer) Content() string

// Tail returns the last n lines. Used for session persistence (output_tail).
func (b *OutputBuffer) Tail(n int) string

// LineCount returns the number of buffered lines.
func (b *OutputBuffer) LineCount() int

// TotalLines returns the total lines ever received (including truncated).
func (b *OutputBuffer) TotalLines() int

// Truncated returns the number of lines that were discarded.
func (b *OutputBuffer) Truncated() int
```

---

## 8. Graceful Shutdown Protocol

Triggered by `q` key (with confirmation if workers running), `ctrl+c`, or OS signals.

```go
// gracefulShutdown returns a tea.Cmd that orchestrates the shutdown sequence.
func (m Model) gracefulShutdown() tea.Cmd {
    return func() tea.Msg {
        // 1. Persist current session state
        m.persister.SaveSync()

        // 2. Send SIGTERM to all running workers
        for _, w := range m.runningWorkers() {
            w.Handle.Kill(3 * time.Second) // 3s grace, then SIGKILL
        }

        // 3. Wait for all workers to exit (up to 5s total)
        deadline := time.After(5 * time.Second)
        for _, w := range m.runningWorkers() {
            select {
            case <-w.done: // channel closed when worker exits
            case <-deadline:
                break
            }
        }

        // 4. Persist final state (all workers now exited/killed)
        m.persister.SaveSync()

        // 5. Exit
        return tea.Quit()
    }
}
```

### Signal Handling

```go
// In main(), set up OS signal handling:
ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
defer stop()

// Pass ctx to tea.Program. When signal arrives:
// - SIGINT (ctrl+c from outside TUI): triggers graceful shutdown
// - SIGTERM: triggers graceful shutdown
// Inside TUI, ctrl+c is caught by bubbletea as tea.KeyMsg before it becomes a signal.
```

---

## 9. OpenCode Integration Contract

### Session ID Extraction

The OpenCode session ID must be extracted from worker output to enable continuation.
OpenCode prints session info at the start of a run.

```go
// extractSessionID scans output lines for the OpenCode session identifier.
// Expected format in output: "[agent_name] session: ses_<alphanumeric>"
// or JSON format: {"session_id": "ses_..."}
//
// Returns empty string if not found.
func extractSessionID(output string) string {
    // Pattern 1: text format
    // [reviewer] session: ses_a8f3k2
    re := regexp.MustCompile(`session:\s+(ses_[a-zA-Z0-9]+)`)
    if m := re.FindStringSubmatch(output); len(m) > 1 {
        return m[1]
    }

    // Pattern 2: JSON format (--format json output)
    // {"session_id": "ses_a8f3k2", ...}
    re2 := regexp.MustCompile(`"session_id"\s*:\s*"(ses_[a-zA-Z0-9]+)"`)
    if m := re2.FindStringSubmatch(output); len(m) > 1 {
        return m[1]
    }

    return ""
}
```

### OpenCode CLI Invocation Patterns

```
# Basic worker spawn
opencode run --agent coder "Implement the auth middleware"

# Worker with file attachments
opencode run --agent coder --file spec.md --file plan.md "Implement WP-001"

# Session continuation
opencode run --continue -s ses_a8f3k2 --agent coder "Apply suggestions 1 and 3"

# Model override
opencode run --agent planner --model anthropic/claude-sonnet-4-20250514 "Plan the DB migration"

# JSON output format (useful for structured parsing)
opencode run --agent reviewer --format json "Review the auth changes"
```

### Dependency Validation

```go
// ValidateDependencies checks that required external tools are available.
// Called by `kasmos setup` and at TUI startup.
func ValidateDependencies() []DependencyCheck {
    return []DependencyCheck{
        {Name: "opencode", Check: func() error {
            _, err := exec.LookPath("opencode")
            return err
        }, Required: true, InstallHint: "go install github.com/anomalyco/opencode@latest"},

        {Name: "git", Check: func() error {
            _, err := exec.LookPath("git")
            return err
        }, Required: true, InstallHint: "install via system package manager"},
    }
}

type DependencyCheck struct {
    Name        string
    Check       func() error
    Required    bool
    InstallHint string
}
```

---

## 10. Package Structure

Recommended Go package layout for the kasmos binary:

```
kasmos/
├── main.go                    # Entry point, flag parsing, tea.Program setup
├── cmd/
│   ├── root.go               # Root command (TUI launch)
│   └── setup.go              # `kasmos setup` subcommand
├── internal/
│   ├── tui/
│   │   ├── model.go          # Main Model struct, Init(), Update(), View()
│   │   ├── keys.go           # keyMap, defaultKeyMap(), ShortHelp(), FullHelp()
│   │   ├── styles.go         # All lipgloss styles, colors, indicators
│   │   ├── messages.go       # All tea.Msg types
│   │   ├── commands.go       # All tea.Cmd constructors (spawn, kill, read, etc.)
│   │   ├── layout.go         # Layout calculation, breakpoints, recalculateLayout()
│   │   ├── panels.go         # Panel rendering (table, viewport, tasks, status bar)
│   │   ├── overlays.go       # Overlay rendering (spawn dialog, help, quit confirm)
│   │   ├── daemon.go         # Daemon mode event logging
│   │   └── update.go         # Update dispatch (updateTableKeys, updateViewportKeys, etc.)
│   ├── worker/
│   │   ├── backend.go        # WorkerBackend interface
│   │   ├── subprocess.go     # SubprocessBackend implementation
│   │   ├── worker.go         # Worker struct, WorkerState, lifecycle
│   │   ├── output.go         # OutputBuffer
│   │   ├── manager.go        # WorkerManager (orchestrates spawns, tracks workers)
│   │   └── session.go        # Session ID extraction from output
│   ├── task/
│   │   ├── source.go         # Source interface, Task struct, TaskState
│   │   ├── speckitty.go      # SpecKittySource implementation
│   │   ├── gsd.go            # GsdSource implementation
│   │   └── adhoc.go          # AdHocSource implementation
│   ├── persist/
│   │   ├── session.go        # SessionPersister, save/load, atomic write
│   │   └── schema.go         # SessionState struct (maps to JSON schema)
│   └── setup/
│       ├── setup.go          # Setup orchestration (validate deps, scaffold agents)
│       ├── agents.go         # Agent definition templates
│       └── deps.go           # Dependency validation
├── go.mod
├── go.sum
└── .goreleaser.yml           # Build configuration
```

### Key Dependencies (go.mod)

```
module github.com/user/kasmos

go 1.23

require (
    github.com/charmbracelet/bubbletea v2.0.0
    github.com/charmbracelet/bubbles   v0.20.0
    github.com/charmbracelet/lipgloss  v2.0.0
    github.com/charmbracelet/huh       v0.6.0
    github.com/muesli/gamut            v0.3.1
    github.com/mattn/go-isatty         v0.0.20
    github.com/spf13/cobra             v1.8.0
    gopkg.in/yaml.v3                   v3.0.1
)
```

### Build Tags

```go
//go:build !test

// The daemon mode detection uses isatty, which needs a real terminal.
// Tests should mock the terminal detection.
```

---

## Cross-Reference: Design Artifacts ↔ Technical Artifacts

| Design Artifact (design-artifacts/) | Technical Contract (this document) |
|--------------------------------------|-------------------------------------|
| tui-layout-spec.md § Panel Specs     | §10 Package Structure (panels.go)   |
| tui-layout-spec.md § Focus System    | §2 focusChangedMsg, §3 panel enum   |
| tui-mockups.md § V2 Spawn Dialog     | §2 spawnDialogSubmittedMsg          |
| tui-mockups.md § V5 Continue Dialog  | §2 continueDialogSubmittedMsg       |
| tui-mockups.md § V7 Worker Chains    | §3 Worker.ParentID, Children()      |
| tui-mockups.md § V9 AI Analysis      | §2 analyzeCompletedMsg              |
| tui-mockups.md § V10 Daemon Mode     | §6 Daemon Mode Output Format        |
| tui-keybinds.md § keys.go            | §2 Message types triggered by keys  |
| tui-styles.md § WorkerState          | §3 Worker.State, WorkerState enum   |
| tui-styles.md § TaskState            | §4 Task.State, TaskState enum       |
