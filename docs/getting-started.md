# Getting Started with kasmos

Welcome to kasmos, a Zellij-based orchestrator for managing concurrent AI coding agents. This guide covers the basics of Zellij, kasmos fundamentals, and practical workflows.

## What is kasmos?

kasmos orchestrates multiple OpenCode agent sessions inside a single Zellij terminal multiplexer. It manages:

- **Work package coordination** — executes discrete units of work across waves
- **Agent pane lifecycle** — spawns, monitors, and controls agent sessions
- **Operator interface** — provides a controller pane for real-time command dispatch
- **State persistence** — tracks progress and allows resume/retry operations

## Prerequisites

Before using kasmos, ensure these tools are installed and in your `PATH`:

- **zellij** (0.41+) — terminal multiplexer
- **opencode** — AI coding agent TUI
- **spec-kitty** — feature specification tool
- **git** — version control (for worktree management)
- **Rust toolchain** — to build kasmos

## A Brief Zellij Primer

### What is Zellij?

Zellij is a terminal multiplexer similar to tmux/screen, but with a focus on ergonomics and programmatic control. kasmos uses Zellij's layout system and pane management to orchestrate agents.

### Key Zellij Concepts

**Panes** — Independent terminal windows within a session. Each agent runs in its own pane.

**Layouts** — KDL (KDL Document Language) files that define pane structure and initial commands. kasmos generates layouts dynamically.

**Sessions** — Top-level Zellij containers. kasmos creates one Zellij session per orchestration run, named `kasmos-<feature>`.

**Keybinds** — Zellij uses mode-based keybinds (pane mode, tab mode, etc.). Learn common keys in the [keybinds reference](./keybinds.md).

### Basic Zellij Navigation (Inside a Session)

| Action | Keys |
|--------|------|
| Enter pane mode | `Ctrl+p` |
| Enter resize mode | `Ctrl+n` |
| Focus next pane | `Ctrl+p` → `Tab` |
| Close pane | `Ctrl+p` → `d` |
| Toggle fullscreen | `Ctrl+p` → `f` |
| Move focus (arrow keys) | `Ctrl+p` → `h/j/k/l` |
| Quit Zellij | `Ctrl+q` |

See [keybinds.md](./keybinds.md) for a complete reference.

## kasmos Quickstart

### 1. Launch an Orchestration

```bash
kasmos launch path/to/feature --mode continuous
```

This creates a `.kasmos/` directory and starts a Zellij session with:
- **Controller pane** (left 40%) — runs `opencode` for command input
- **Agent grid** (right 60%) — contains panes for each work package

### 2. Understanding the Layout

```
┌─────────────────────┬─────────────────────────────┐
│   Controller        │       Agent Grid            │
│   (40% width)       │       (60% width)           │
│                     │                             │
│  opencode           │  ┌─────────────┬─────────┐  │
│  Interactive TUI    │  │   WP01      │  WP02   │  │
│  [Enter commands]   │  │   Agent     │ Agent   │  │
│                     │  ├─────────────┼─────────┤  │
│                     │  │  WP03       │  WP04   │  │
│                     │  │  Agent      │ Agent   │  │
│                     │  └─────────────┴─────────┘  │
└─────────────────────┴─────────────────────────────┘
```

Agent panes are arranged in an adaptive grid:
- **Columns** = ceil(sqrt(n_agents))
- **Rows** = ceil(n_agents / cols)

For example, 5 agents: cols=2, rows=3 (with one empty slot).

### 3. The `.kasmos/` Directory

After launching, you'll see:

```
.kasmos/
├── cmd.pipe           # Named pipe for command input
├── layout.kdl         # Generated Zellij layout
├── state.json         # Orchestration state (persistent)
├── prompts/           # Work package prompts
│   ├── WP01/prompt.md
│   ├── WP02/prompt.md
│   └── ...
└── logs/              # Execution logs (optional)
```

## Live Command Control via FIFO

The `.kasmos/cmd.pipe` is a named pipe (FIFO) that accepts operator commands. Write commands to it from another terminal:

```bash
# From the controller pane or another terminal:
echo "status" > .kasmos/cmd.pipe
echo "restart WP01" > .kasmos/cmd.pipe
echo "pause WP02" > .kasmos/cmd.pipe
```

### Available FIFO Commands

| Command | Effect |
|---------|--------|
| `status` | Display current orchestration state |
| `restart <WP_ID>` | Restart a failed/crashed work package |
| `pause <WP_ID>` | Pause a running work package |
| `resume <WP_ID>` | Resume a paused work package |
| `focus <WP_ID>` | Focus (navigate to) a work package pane |
| `zoom <WP_ID>` | Focus and zoom a pane to full view |
| `abort` | Gracefully shutdown orchestration |
| `advance` | Confirm wave advancement (wave-gated mode) |
| `force-advance <WP_ID>` | Skip a failed WP, unblock dependents |
| `retry <WP_ID>` | Re-run a failed WP from scratch |
| `help` | Show available commands |

### Example Workflow: Multi-Terminal Control

**Terminal A (Zellij session):**
```bash
kasmos launch path/to/feature
# [Now inside Zellij with agents running]
```

**Terminal B (Control pane):**
```bash
cd path/to/feature

# Check status
echo "status" > .kasmos/cmd.pipe

# An agent crashed, restart it
echo "restart WP01" > .kasmos/cmd.pipe

# Focus on another pane to watch it
echo "focus WP02" > .kasmos/cmd.pipe

# Something's stuck, force-advance to unblock
echo "force-advance WP03" > .kasmos/cmd.pipe
```

## CLI Commands

### `kasmos launch`

Start a new orchestration:

```bash
kasmos launch <feature_dir> [--mode continuous|wave-gated]
```

- `<feature_dir>` — Path to the feature directory (required)
- `--mode` — Progression mode (default: `continuous`)
  - `continuous` — Execute all work packages in dependency order, no pauses
  - `wave_gated` — Pause between waves, requiring operator confirmation

**Example:**
```bash
kasmos launch ./features/001-zellij-orchestrator --mode continuous
```

### `kasmos status`

Show current orchestration status:

```bash
kasmos status [feature_dir]
```

- `feature_dir` — Optional; auto-detects from `.kasmos/` if omitted

**Output:**
```
[kasmos] Orchestration Status: features/001-zellij-orchestrator
Mode: Continuous | State: Running
────────────────────────────────────────────────────────────────
WP       State        Pane         Duration   Wave
────────────────────────────────────────────────────────────────
WP01     Active       5            2m34s...   0
WP02     Pending      -            -          1
WP03     Completed    8            1m45s      0
────────────────────────────────────────────────────────────────
```

### `kasmos attach`

Attach to an existing orchestration session:

```bash
kasmos attach <feature_dir>
```

Useful if you disconnected or are joining from another terminal. Reconnects to the running Zellij session.

### `kasmos stop`

Stop a running orchestration:

```bash
kasmos stop [feature_dir]
```

- `feature_dir` — Optional; auto-detects from `.kasmos/` if omitted

Gracefully shuts down the Zellij session and saves final state to `state.json`.

## Progression Modes

### Continuous Mode (Default)

Work packages execute in dependency order with no pauses. The orchestrator automatically launches the next wave after the current wave completes.

**Use when:** You want hands-off execution with agents handling long-running tasks.

### Wave-Gated Mode

Waves execute sequentially, but the orchestrator pauses between waves, requiring explicit operator confirmation via the `advance` command before proceeding.

**Use when:** You want to review progress, adjust parameters, or make decisions between phases.

**Example workflow:**
```bash
# Launch in wave-gated mode
kasmos launch path/to/feature --mode wave_gated

# [Wave 0 completes, orchestrator pauses]
# Review results...

# Advance to wave 1
echo "advance" > .kasmos/cmd.pipe

# [Wave 1 executes...]
```

## State Persistence & Resume

kasmos saves the full orchestration state to `.kasmos/state.json` after every status update. If a session crashes:

1. The state file persists
2. You can `attach` to reconnect to the Zellij session
3. Or `launch` again with the same feature dir to resume from saved state

## Practical Workflow Example

### Scenario: Implementing a Complex Feature

You have a feature with 6 work packages (WP01–WP06) that depend on each other.

**Step 1: Launch in wave-gated mode for control**
```bash
kasmos launch ./features/my-feature --mode wave_gated
# Zellij window opens with controller (left) and agents (right)
```

**Step 2: Wave 0 starts (WP01, WP02 in parallel)**
- Both agents begin running
- Monitor progress in their panes
- Use `Ctrl+p` to enter pane mode, then `Tab` or arrow keys to navigate

**Step 3: Wave 0 completes**
- Orchestrator pauses
- Check results in `.kasmos/state.json` or via `status`

**Step 4: Resolve issues if needed**
```bash
# If WP01 failed, restart it
echo "restart WP01" > .kasmos/cmd.pipe

# Or force-advance to skip and unblock dependents
echo "force-advance WP01" > .kasmos/cmd.pipe
```

**Step 5: Advance to wave 1**
```bash
echo "advance" > .kasmos/cmd.pipe
# WP03, WP04 launch
```

**Step 6: Complete orchestration**
- Continue monitoring and advancing waves
- Or switch to continuous monitoring (less intervention)

## Troubleshooting

### "Command channel closed" Error

The FIFO reader crashed. Restart the orchestration:
```bash
kasmos stop && kasmos launch path/to/feature
```

### Agent pane not updating

Zellij may not have detected the pane yet (health check limitation). Wait a few seconds, or focus/refocus the pane:
```bash
echo "focus WP01" > .kasmos/cmd.pipe
```

### FIFO permission errors

Ensure the `.kasmos/` directory is readable/writable:
```bash
chmod 700 .kasmos
```

### Zellij not in PATH

Install Zellij or add its bin directory to `PATH`:
```bash
export PATH="/path/to/zellij/bin:$PATH"
```

## Next Steps

- **[Cheatsheet](./cheatsheet.md)** — Quick reference for CLI and FIFO commands
- **[Keybinds Reference](./keybinds.md)** — Zellij keybinds and floating pane tips
- **[Architecture](./architecture.md)** — Deep dive into orchestration design and state machines
