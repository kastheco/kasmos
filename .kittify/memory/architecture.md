# kasmos Architecture Intelligence

> Codebase discoveries and architectural knowledge accumulated during development.
> This file is the authority on how kasmos internals work and interact.
> Updated: 2026-02-20

## System Overview

kasmos is a Go/bubbletea TUI-based agent orchestrator. It manages concurrent AI coding
agent sessions (OpenCode workers) from a terminal dashboard. The human drives orchestration
directly -- no manager AI agent, zero token cost for orchestration.

### Runtime Modes

1. **TUI mode** (`kasmos [path]`) - Interactive terminal dashboard with worker table,
   output viewport, task panel. Responsive layout at 4 breakpoints.
2. **Daemon mode** (`kasmos -d`) - Same Model/Update loop, no View rendering
   (`WithoutRenderer()`). Status events logged to stdout as NDJSON or human-readable text.
3. **Setup** (`kasmos setup`) - Scaffolds `.opencode/agents/*.md` definitions and validates
   dependencies (opencode, git).
4. **Reattach** (`kasmos --attach`) - Reconnects TUI to a running daemon session,
   restoring worker states from `.kasmos/session.json`.

## Package Architecture

```
cmd/kasmos/main.go          Entry point, cobra commands, tea.Program setup
internal/tui/               bubbletea TUI (Elm architecture)
  model.go                  Main Model struct, Init(), Update(), View()
  update.go                 Update dispatch per panel/overlay
  keys.go                   keyMap with context-dependent activation
  styles.go                 lipgloss palette, component styles, indicators
  messages.go               All tea.Msg types (worker, UI, overlay, task, persist)
  commands.go               All tea.Cmd constructors
  layout.go                 Responsive breakpoints, dimension math
  panels.go                 Panel rendering (table, viewport, tasks, status bar)
  overlays.go               Overlays (spawn dialog, continue, help, quit confirm)
  daemon.go                 Daemon mode event logging
internal/worker/            Worker process management
  backend.go                WorkerBackend interface
  subprocess.go             SubprocessBackend (os/exec)
  worker.go                 Worker struct, WorkerState state machine
  output.go                 OutputBuffer (thread-safe ring buffer)
  session.go                OpenCode session ID extraction
  manager.go                WorkerManager (ID generation, worker tracking)
internal/task/              Task source adapters
  source.go                 Source interface, Task struct, TaskState
  speckitty.go              SpecKittySource (plan.md + tasks/WP*.md frontmatter)
  gsd.go                    GsdSource (checkbox markdown)
  adhoc.go                  AdHocSource (empty, manual prompts)
internal/persist/           Session persistence
  session.go                SessionPersister (debounced atomic JSON writes)
  schema.go                 SessionState struct
internal/setup/             Setup command
  setup.go                  Orchestration (deps + scaffolding)
  agents.go                 Agent definition templates
  deps.go                   Dependency validation
```

## Worker Lifecycle

```
StatePending -> StateSpawning -> StateRunning --+--> StateExited (code 0)
                                                +--> StateFailed (code != 0)
                                                +--> StateKilled (user kill)
```

Workers are spawned through two backend modes:

- `SubprocessBackend` (headless): `opencode run --agent <role> "prompt"`.
  Prompt is positional. `--variant` (reasoning) and `--file` attachments are supported.
  `cfg.WorkDir` maps to `exec.Cmd.Dir`.
- `TmuxBackend` (interactive): `opencode --agent <role> --prompt "prompt"` in a tmux pane.
  Prompt is passed by `--prompt` (not positional). `--variant` and `--file` are not
  available in interactive mode and are intentionally ignored. `cfg.WorkDir` maps to
  `tmux split-window -c <dir>`.

Subprocess mode merges stdout and stderr into a single pipe, read by a goroutine that
sends `workerOutputMsg` through `tea.Program.Send()`. When the process exits, a
`workerExitedMsg` is sent with the exit code and parsed session ID.

Continuation spawns a NEW worker (new ID, new process) that preserves the parent
session context. Subprocess mode uses `opencode run --continue -s <session_id> "follow-up"`.
Tmux mode uses `opencode --continue -s <session_id> --prompt "follow-up"`.
The child worker's `ParentID` field links it to its parent for tree display.

## Key Interfaces

### WorkerBackend

```go
type WorkerBackend interface {
    Spawn(ctx context.Context, cfg SpawnConfig) (WorkerHandle, error)
    Name() string
}
```

`SubprocessBackend` (os/exec, headless default). `TmuxBackend` (interactive tmux panes, feature 019).

`WorkerHandle` interface includes `Interactive() bool` to distinguish backends.
SubprocessBackend returns `false`; TmuxBackend returns `true`. TUI skips pipe-based
output reading for interactive handles (output goes to tmux pane instead).

### Source (Task)

```go
type Source interface {
    Type() string    // "spec-kitty", "gsd", "ad-hoc"
    Path() string    // file/directory path
    Load() ([]Task, error)
    Tasks() []Task
}
```

### bubbletea Message Flow

```
User input (tea.KeyMsg) -> Update() -> tea.Cmd (side effect) -> tea.Msg (result) -> Update() -> View()
Worker event flow: spawnWorkerCmd -> workerSpawnedMsg -> readOutputCmd -> workerOutputMsg(loop) -> workerExitedMsg
Timer: tea.Tick(1s) -> tickMsg (duration refresh)
Spinner: spinner.Tick -> spinner.TickMsg (animation)
```

## Task Source Patterns

### spec-kitty (SpecKittySource)

Reads `kitty-specs/<slug>/` directory:
- `plan.md` for WP summaries (markdown, not structured)
- `tasks/WP*.md` for work packages with YAML frontmatter

Frontmatter fields: `work_package_id`, `title`, `dependencies`, `lane`, `subtasks`, `phase`
Lane mapping: planned -> TaskUnassigned, doing -> TaskInProgress, for_review -> TaskInProgress, done -> TaskDone
Role inference from phase: spec/clarifying -> planner, implementation -> coder, reviewing -> reviewer, releasing -> release

### GSD (GsdSource)

Reads a markdown file with checkboxes:
```
- [ ] Implement auth   -> Task{ID: "T-001", State: TaskUnassigned}
- [x] Review PR #42    -> Task{ID: "T-002", State: TaskDone}
```

### Ad-hoc (AdHocSource)

No file. Empty task list. Workers spawned with manual prompts only.

## Session Persistence

State persisted to `.kasmos/session.json`:
- Schema version 1
- Session ID (ks-timestamp-random)
- All workers (active + historical) with output tails
- Task source configuration
- PID for reattach detection

Write behavior: debounced to 1 write/second, atomic (write .tmp then rename).
Reattach: if session.json exists and PID is alive, connect to running session.
Orphan recovery: if PID is dead, mark running workers as killed, assign new PID.

## Design Reference

Visual design defined in `design-artifacts/`:
- `tui-layout-spec.md` - 4 responsive breakpoints, dimension math, focus system
- `tui-mockups.md` - 12 ASCII mockups covering all view states
- `tui-keybinds.md` - Full keybind map with implementation code
- `tui-styles.md` - Charm bubblegum palette, component styles, status indicators

Technical contracts in `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
- Go interface definitions (WorkerBackend, Source, WorkerHandle)
- Complete tea.Msg type catalog (20+ types)
- Session persistence JSON schema
- Daemon mode NDJSON event schema
- Output buffer ring design
- Graceful shutdown protocol
- Package structure
