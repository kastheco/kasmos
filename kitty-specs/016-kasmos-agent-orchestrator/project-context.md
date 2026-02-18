# kasmos - Agent Orchestrator: Project Context

> This document packages all architectural decisions, research, and design context for the kasmos rewrite. It is intended as the knowledge base for a Claude.ai project focused on TUI design.

## What is kasmos?

kasmos is a terminal-based orchestrator for managing concurrent AI coding agent sessions. A developer runs `kasmos`, sees a dashboard, and can spawn/monitor/kill multiple AI agent workers in parallel. Workers are OpenCode instances that execute coding tasks (planning, implementation, review, etc.).

The developer drives orchestration directly through the TUI -- there is no "manager AI agent" consuming tokens for orchestration decisions. The TUI is deterministic and instant; only workers use AI.

## Tech Stack (locked in)

- **Language**: Go
- **TUI framework**: bubbletea (Elm architecture: Model/Update/View)
- **Styling**: lipgloss
- **Components**: bubbles (table, spinner, text input, viewport, list, paginator)
- **Worker harness**: OpenCode (`opencode run` for headless workers)
- **Distribution**: Single Go binary
- **Daemon mode**: bubbletea's `WithoutRenderer()` for headless/CI operation
- **Future SSH access**: charmbracelet/wish

## Why this architecture?

kasmos was previously a Rust binary that orchestrated AI agents inside Zellij (a terminal multiplexer). After extensive evaluation (see Architecture Evaluation below), we determined:

1. **Zellij lacks programmatic introspection.** No `list-panes`, no pane event hooks, no way to know if a pane crashed. kasmos maintained an in-memory registry that went stale constantly.

2. **A manager AI agent is unnecessary.** The manager was translating human instructions into tool calls -- "spawn a planner" becomes `spawn_worker(role=planner)`. That's what a TUI button does, instantly and for free.

3. **Workers don't need terminal panes.** Workers run `opencode run` (headless) -- they take a prompt, execute, and exit. Their output is captured via Go pipes and displayed in the TUI. No terminal multiplexer needed.

4. **Session continuation replaces interactivity.** When a reviewer says "verified with suggestions", the developer reads the output in the dashboard and spawns a continuation: `opencode run --continue -s <session_id> "Apply suggestions 1 and 3"`. Full context is preserved without needing to type into a running session.

5. **Go/bubbletea is the best framework fit.** The Elm architecture maps naturally to kasmos's event loop (worker events -> update state -> render dashboard). Built-in daemon mode, charmbracelet ecosystem (wish for SSH, bubbles for components, lipgloss for styling).

## Architecture Overview

```
kasmos binary (Go)
    |
    +-- bubbletea TUI (interactive mode)
    |     |
    |     +-- Worker Dashboard (bubbles/table)
    |     +-- Output Viewport (bubbles/viewport)
    |     +-- Spawn Dialog (bubbles/textinput + huh)
    |     +-- Task Source Panel (spec-kitty / GSD / ad-hoc)
    |     +-- Status Bar (lipgloss styled)
    |
    +-- bubbletea daemon (headless mode, -d flag)
    |     +-- Same Model/Update loop, no View rendering
    |     +-- Structured stdout logging
    |
    +-- Worker Manager
    |     +-- WorkerBackend interface
    |     |     +-- SubprocessBackend (MVP: os/exec)
    |     |     +-- TmuxBackend (future: optional)
    |     +-- Output capture via Go pipes
    |     +-- Process lifecycle tracking
    |     +-- Session ID tracking for continuations
    |
    +-- Task Source adapters
    |     +-- SpecKittySource (reads plan.md -> WPs)
    |     +-- GsdSource (reads tasks.md -> tasks)
    |     +-- AdHocSource (empty, manual prompts)
    |
    +-- Session persistence (JSON to disk)
    +-- kasmos setup (scaffolds .opencode/agents/)
```

## Worker Lifecycle

```
[pending]  -- user selects task, hasn't spawned yet
    |
    v
[spawning] -- os/exec.Command starting
    |
    v
[running]  -- process alive, stdout streaming into buffer
    |
    +---> [exited(0)]  -- success, output captured
    +---> [exited(N)]  -- failure, output captured, N = exit code
    +---> [killed]     -- user terminated via TUI
    
From exited/killed:
    +---> [continue]   -- user spawns follow-up with --continue -s <id>
    +---> [restart]    -- user spawns fresh with edited prompt
```

## Worker Continuation Flow

This is the key interaction pattern for iterative workflows:

1. Worker (e.g., reviewer) runs and completes
2. kasmos captures output + OpenCode session ID
3. User reads output in the TUI viewport
4. User presses `c` (continue) on the completed worker
5. TUI shows input for follow-up message
6. User types: "Apply suggestions 1 and 3. Skip suggestion 2."
7. kasmos spawns: `opencode run --continue -s <session_id> "Apply suggestions 1 and 3..."`
8. New worker appears in dashboard linked to parent, runs with full context

## Workflow Modes

### spec-kitty (formal planning pipeline)
```
kasmos kitty-specs/015-auth-overhaul/
```
Reads `plan.md`, extracts work packages with descriptions, dependencies, and suggested agent roles. Dashboard pre-populates with WPs. User selects and spawns.

### GSD (lightweight task tracking)
```
kasmos tasks.md
```
Reads a markdown task file (checkbox format). Dashboard shows tasks. User assigns agent roles and spawns.

### Ad-hoc (no planning artifacts)
```
kasmos
```
Empty dashboard. User manually types agent role + prompt for each worker.

## Agent Roles (shipped with kasmos setup)

kasmos scaffolds these as OpenCode custom agents in `.opencode/agents/`:

| Role     | Mode     | Description                                     |
|----------|----------|-------------------------------------------------|
| planner  | subagent | Research and planning, read-only filesystem      |
| coder    | subagent | Implementation, full tool access                 |
| reviewer | subagent | Code review, read-only + test execution          |
| release  | subagent | Merge, finalization, cleanup operations          |

Each agent has a markdown file defining its system prompt, model, tools, and permissions.

## On-Demand AI Helpers

These are NOT persistent agents -- they're one-shot `opencode run` calls triggered by TUI keybinds:

- **Prompt generation** (`g` key): "Given this task description and the project's plan.md, generate a focused prompt for a coder agent." Returns suggested prompt the user can edit before spawning.
- **Failure analysis** (`a` key): "This worker failed. Here's the last 50 lines of output. Explain what went wrong and suggest a revised prompt." Returns analysis displayed in the TUI.

## Key Decisions Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Language | Go | bubbletea ecosystem, single binary, goroutines for concurrency |
| TUI framework | bubbletea | Elm architecture maps to event-driven orchestration |
| Worker execution | Headless subprocess (opencode run) | Simplest, captures output via pipes |
| Worker interaction | Session continuation (--continue -s) | Preserves full context without PTY/tmux |
| Worker backend | Pluggable interface | SubprocessBackend MVP, TmuxBackend future option |
| Terminal multiplexer | None for MVP | Workers are subprocesses, TUI handles display |
| Manager agent | Eliminated | TUI is deterministic, instant, zero token cost |
| Daemon mode | bubbletea WithoutRenderer() | Same logic, no TUI -- for CI/scripting |
| Remote access | wish (SSH TUI server, future) | Built into charmbracelet ecosystem |
| Task sources | Pluggable adapters | spec-kitty, GSD, ad-hoc |

## OpenCode CLI Reference (relevant flags)

```
# Headless execution (worker mode)
opencode run [message..]
  --agent <name>          # Agent to use (planner, coder, reviewer, release)
  --model <provider/model> # Override model
  --file <path>           # Attach file(s) to message
  --continue, -c          # Continue last session
  --session, -s <id>      # Specific session to continue
  --fork                  # Fork session when continuing
  --format <fmt>          # Output format: default or json
  --attach <url>          # Attach to running opencode server

# Headless server (for SDK-driven control, future)
opencode serve
  --port <port>
  --hostname <host>

# Agent management
opencode agent list
opencode agent create
```

## OpenCode Custom Agent Format

```markdown
# .opencode/agents/coder.md
---
description: Implementation agent for writing and modifying code
model: anthropic/claude-sonnet-4-20250514
tools:
  write: true
  edit: true
  bash: true
  glob: true
  grep: true
  fetch: true
permission:
  bash:
    "git push --force*": deny
    "rm -rf /*": deny
---

You are a coder agent for the kasmos project.
Implement the work package described in your prompt.
Run tests after changes. Follow the project's AGENTS.md conventions.
```

## TUI Design Requirements (for the research phase)

The TUI design should address:

1. **Dashboard layout**: Worker list (table), output viewport, status bar, help overlay
2. **Spawn dialog**: Agent role selector, prompt editor (multiline), file attachment, task association
3. **Worker states**: Visual indicators for running (spinner), done (checkmark), failed (X), killed (skull), pending (circle)
4. **Output viewing**: Split view (dashboard + output), full-screen output mode, output search/filter
5. **Task panel**: When a task source is loaded, show tasks with status (unassigned, in-progress, done)
6. **Continuation UI**: Show parent-child relationships between workers, pre-fill context
7. **Responsive layout**: Adapt to terminal size (minimum 80x24, graceful degradation)
8. **Color scheme**: Consistent with terminal conventions, works on light and dark backgrounds
9. **Keybinds**: vim-inspired navigation (j/k for list, enter to select, ? for help)
10. **Daemon output**: What structured logging looks like in headless mode

### Reference TUI applications for design inspiration

- **lazydocker**: Container management dashboard with split views
- **k9s**: Kubernetes cluster management with table + detail views
- **gitui**: Git operations with panel-based layout
- **opencode**: The OpenCode TUI itself (bubbletea-based)
- **charm apps**: gum, soft-serve, mods -- charmbracelet design language

## bubbletea Architecture Primer

bubbletea uses the Elm architecture:

```go
// Model holds all application state
type Model struct {
    workers    []Worker
    selected   int
    viewport   viewport.Model
    table      table.Model
    // ...
}

// Init returns initial commands (start timers, etc.)
func (m Model) Init() tea.Cmd { ... }

// Update handles messages and returns new model + commands
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case WorkerExitedMsg:
        // update worker state
    case tea.KeyMsg:
        // handle keybinds
    case tickMsg:
        // periodic refresh
    }
}

// View renders the UI as a string
func (m Model) View() string {
    return lipgloss.JoinVertical(
        lipgloss.Top,
        m.renderHeader(),
        m.renderDashboard(),
        m.renderStatusBar(),
    )
}
```

Key concepts:
- **Msgs**: Events that flow into Update (key presses, timer ticks, worker events, window resize)
- **Cmds**: Side effects returned from Update (spawn process, start timer, read file)
- **Sub-models**: bubbles components (table, viewport, textinput) have their own Update/View
- **tea.Batch**: Run multiple Cmds concurrently
- **tea.Program options**: WithoutRenderer() for daemon, WithAltScreen() for full-screen

## Daemon Mode Pattern (from bubbletea examples)

```go
func main() {
    var daemonMode bool
    flag.BoolVar(&daemonMode, "d", false, "run as daemon")
    flag.Parse()

    var opts []tea.ProgramOption
    if daemonMode || !isatty.IsTerminal(os.Stdout.Fd()) {
        opts = []tea.ProgramOption{tea.WithoutRenderer()}
        log.SetOutput(os.Stdout)  // log to stdout in daemon mode
    } else {
        log.SetOutput(io.Discard) // silence logs in TUI mode
    }

    p := tea.NewProgram(newModel(), opts...)
    if _, err := p.Run(); err != nil {
        os.Exit(1)
    }
}
```

Same Model/Update loop, just no View() calls. Worker events still flow through Update, but instead of rendering a TUI, they log to stdout.
