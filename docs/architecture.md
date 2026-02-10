# kasmos Architecture

This document explains kasmos's internal design: how it orchestrates agents, manages state, and controls the Zellij runtime.

## Overview

kasmos is a Zellij orchestration engine that:

1. **Parses** feature specifications into work packages
2. **Organizes** work packages into execution waves based on dependencies
3. **Generates** KDL layouts that structure the Zellij session
4. **Launches** agents (OpenCode) in Zellij panes
5. **Monitors** progress via polling and completion detection
6. **Accepts** operator commands via a FIFO interface
7. **Persists** state to JSON for resumability

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     kasmos Binary                           │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────────────────────────────────────────────┐   │
│  │         Launch / Status / Attach / Stop            │   │
│  │         (CLI entry points in main.rs)              │   │
│  └──────────────────┬──────────────────────────────────┘   │
│                     │                                       │
│  ┌──────────────────▼──────────────────────────────────┐   │
│  │         Feature Parser                             │   │
│  │  Reads spec.md, extracts work packages            │   │
│  └──────────────────┬──────────────────────────────────┘   │
│                     │                                       │
│  ┌──────────────────▼──────────────────────────────────┐   │
│  │         Dependency Graph & Wave Builder            │   │
│  │  Organizes WPs into waves, resolves deps          │   │
│  └──────────────────┬──────────────────────────────────┘   │
│                     │                                       │
│  ┌──────────────────▼──────────────────────────────────┐   │
│  │         Layout Generator (KDL)                     │   │
│  │  Creates Zellij layout from work packages         │   │
│  └──────────────────┬──────────────────────────────────┘   │
│                     │                                       │
│  ┌──────────────────▼──────────────────────────────────┐   │
│  │         Zellij Session Spawner                     │   │
│  │  Launches Zellij with generated layout            │   │
│  └──────────────────┬──────────────────────────────────┘   │
│                     │                                       │
│  ┌──────────────────▼──────────────────────────────────┐   │
│  │  Orchestration Engine (Continuous / Wave-Gated)   │   │
│  │  - FIFO Command Reader                            │   │
│  │  - Health Checker (polls pane exits)              │   │
│  │  - Completion Detector (spec-kitty integration)   │   │
│  │  - State Machine (WP/Run state transitions)       │   │
│  │  - Wave Executor                                  │   │
│  └──────────────────┬──────────────────────────────────┘   │
│                     │                                       │
│  ┌──────────────────▼──────────────────────────────────┐   │
│  │         State Persistence (state.json)            │   │
│  │  Saves orchestration state for resumability       │   │
│  └──────────────────────────────────────────────────────┘   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
              ┌─────────────────────────┐
              │   Zellij Session        │
              │  (Terminal Multiplexer) │
              │                         │
              │  ┌─────────────────┐   │
              │  │  Controller     │   │
              │  │  Pane           │   │
              │  │  (opencode)     │   │
              │  └─────────────────┘   │
              │  ┌──────────┬──────┐   │
              │  │   WP01   │ WP02 │   │
              │  │  Agent   │Agent │   │
              │  ├──────────┼──────┤   │
              │  │   WP03   │ WP04 │   │
              │  │  Agent   │Agent │   │
              │  └──────────┴──────┘   │
              │                         │
              │   .kasmos/cmd.pipe      │
              │   (FIFO for commands)   │
              └─────────────────────────┘
```

## Orchestration Pipeline

### Phase 1: Initialization

```
User Input (CLI)
      │
      ├─ kasmos launch <feature_dir> --mode continuous
      │
      ▼
Load Feature Specification
      │
      ├─ Parse spec.md or kitty spec
      ├─ Extract work packages
      ├─ Identify dependencies
      │
      ▼
Build Dependency Graph
      │
      ├─ Topological sort WPs
      ├─ Assign to waves
      ├─ Validate no cycles
      │
      ▼
Generate KDL Layout
      │
      ├─ Controller pane (left 40%)
      ├─ Agent grid (right 60%)
      ├─ Grid dimensions: cols=ceil(sqrt(n)), rows=ceil(n/cols)
      │
      ▼
Create .kasmos Directory
       │
       ├─ layout.kdl (generated layout)
       ├─ cmd.pipe (FIFO for commands)
       ├─ state.json (state file)
      │
      ▼
Spawn Zellij Session
       │
       ├─ zellij --session kasmos-<feature> --layout layout.kdl
       ├─ Controller pane runs opencode
       ├─ Each agent pane runs: bash -c "cat prompt.md | opencode -p 'context:'"
      │
      ▼
Initialize Orchestration Run
       │
       ├─ Create OrchestrationRun struct
       ├─ Set RunState::Initializing
       ├─ Save to state.json
      │
      ▼
Launch Orchestration Engine
      │
      └─ Proceed to Phase 2
```

### Phase 2: Execution (Continuous Mode)

```
Loop: While RunState != Completed, Aborted, Failed
  │
  ├─ FIFO Command Reader Task
  │  │
  │  ├─ Listen on .kasmos/cmd.pipe
  │  ├─ Parse commands (restart, pause, resume, etc.)
  │  ├─ Send to command_tx channel
  │  │
  │  ▼
  │  CommandHandler
  │  │
  │  ├─ Match command
  │  ├─ Dispatch EngineAction (Restart, Pause, Abort, etc.)
  │  └─ Update orchestration state
  │
  ├─ Health Checker Task (every 5 seconds)
  │  │
  │  ├─ Poll each active pane
  │  ├─ Check if process exited
  │  ├─ Mark WP::Failed if crash detected
  │  │
  │  └─ (Currently uses NoOpChecker; no live pane listing)
  │
  ├─ Completion Detector Task
  │  │
  │  ├─ Monitor spec-kitty lane transitions
  │  ├─ Track file markers
  │  ├─ Detect git activity
  │  ├─ Mark WP::Completed when criteria met
  │  │
  │  └─ Debounce with 200ms window
  │
  ├─ Wave Executor Task
  │  │
  │  ├─ Check if wave ready (all deps completed)
  │  ├─ Activate Pending → Active WPs
  │  ├─ In Continuous mode: auto-advance when wave done
  │  │
  │  └─ In Wave-Gated mode: wait for `advance` command
  │
   └─ State Persistence
      │
      ├─ After each state transition
      ├─ Serialize OrchestrationRun to JSON
      ├─ Write to .kasmos/state.json
      │
      └─ Allows resume/attach if session crashes
```

### Phase 3: Termination

```
Shutdown Signal (Ctrl+C or abort command)
      │
      ▼
Stop All Tasks
      │
      ├─ FIFO reader
      ├─ Health checker
      ├─ Completion detector
      ├─ Wave executor
      │
      ▼
Persist Final State
       │
       ├─ Save state.json
       ├─ Mark any Active WPs as Aborted/Failed
      │
      ▼
Kill Zellij Session
      │
      └─ (User can `kasmos attach` to reconnect or `kasmos stop` to cleanly exit)
```

## Layout Strategy

### Adaptive Grid Calculation

Given N work packages:

```
cols = ceil(sqrt(N))
rows = ceil(N / cols)

Examples:
  N=1:  cols=1, rows=1  →  1×1 grid
  N=2:  cols=2, rows=1  →  2×1 grid
  N=4:  cols=2, rows=2  →  2×2 grid
  N=5:  cols=3, rows=2  →  3×2 grid (1 empty slot)
  N=8:  cols=3, rows=3  →  3×3 grid (1 empty slot)
  N=9:  cols=3, rows=3  →  3×3 grid
```

### KDL Layout Structure

```kdl
layout {
  pane split_direction="vertical" {
    pane size="40%" name="controller" {
      command "opencode"
    }
    pane size="60%" split_direction="horizontal" {
      pane split_direction="vertical" {
        pane name="agent-1" {
          command "bash"
          args "-c" "cat /tmp/prompts/WP01/prompt.md | opencode -p 'context:'"
          cwd "/path/to/worktree/WP01"
        }
        pane name="agent-2" { ... }
      }
      pane split_direction="vertical" {
        pane name="agent-3" { ... }
        pane name="agent-4" { ... }
      }
    }
  }
}
```

**Width distribution:**
- **Controller pane**: 40% (configurable via `KASMOS_CONTROLLER_WIDTH`)
- **Agent grid**: 60% (remainder)

**Height distribution:**
- Rows split equally among row groups
- Columns within each row split equally

## Run and Work Package State Lifecycle

### Work Package State Diagram

```
        ┌─────────┐
        │ Pending │
        └────┬────┘
             │
             │ Wave launches (dependencies met)
             ▼
        ┌─────────┐
        │ Active  │◄────────┐
        └────┬────┘         │
             │              │
             ├─ Success ────────┐
             │              │   │
             ├─ Crash ───┐  │   │
             │           │  │   │
             ├─ pause ───┐  │   │
             │           │  │   │
             ▼           ▼  │   │
        ┌─────────┐  ┌────────┐   │
        │ Failed  │  │ Paused │   │
        └────┬────┘  └───┬────┘   │
             │           │        │
             │ restart   │ resume │
             │ retry     │        │
             └─────┬─────┘        │
                   │              │
                   ▼              ▼
            ┌────────────┐
            │ Completed  │
            └────────────┘
```

**State Transitions:**

| From | To | Trigger |
|------|-----|---------|
| Pending | Active | Wave launches (all dependencies completed) |
| Active | Completed | Completion detected (spec-kitty, git, file marker) |
| Active | Failed | Pane process exits unexpectedly |
| Active | Paused | `pause <WP_ID>` command |
| Paused | Active | `resume <WP_ID>` command |
| Failed | Pending | `retry <WP_ID>` command |
| Failed | Active | `restart <WP_ID>` command |

### Run State Diagram

```
                ┌───────────────┐
                │ Initializing  │
                └────────┬──────┘
                         │
                         ▼
                ┌───────────────┐
                │   Running     │◄─────────┐
                └────────┬──────┘          │
                         │                 │
                         ├─ all done       │
                         │                 │
                         ├─ error ──┐      │
                         │          │      │
                         ├─ abort ──┐      │
                         │          │      │
                         ├─ (wave-gated)   │
                         │    pause ───┐   │
                         │             │   │
                         ▼             ▼   ▼
                 ┌──────────────┐  ┌─────────────┐
                 │  Completed   │  │    Failed   │
                 └──────────────┘  └─────────────┘
                                   
                                   ┌─────────┐
                                   │ Paused  │
                                   │(wave-   │
                                   │ gated)  │
                                   └────┬────┘
                                        │ advance
                                        └─────►(back to Running)

                 ┌──────────────┐
                 │    Aborted   │
                 └──────────────┘
```

**State Transitions:**

| From | To | Trigger |
|------|-----|---------|
| Initializing | Running | Engine starts, wave 0 launches |
| Running | Completed | All work packages completed |
| Running | Failed | Unrecoverable error detected |
| Running | Aborted | `abort` command received |
| Running | Paused | Wave-gated mode + wave completes |
| Paused | Running | `advance` command received |

## Health Checking & Completion Detection

### Health Checker (Polling)

**Current Implementation**: `NoOpChecker`

The health checker periodically polls work packages to detect crashes:

```rust
// Pseudo-code
loop {
    for each active work_package:
        check if pane process exited
        if exited unexpectedly:
            mark WP::Failed
            trigger dependent unblock (if force-advance)
    sleep(poll_interval_secs)  // default: 5 seconds
}
```

**Limitation**: Zellij 0.41+ doesn't expose `list-panes`, so kasmos can't dynamically list panes. The `NoOpChecker` is a placeholder; pane count must be pre-configured via `max_agent_panes`.

**Future enhancement**: When Zellij API improves, implement a real pane lister to detect crash vs. normal completion.

### Completion Detector

Monitors multiple completion signals:

1. **spec-kitty lane transitions** — Monitor `.spec` state for lane completions
2. **Git activity** — Track worktree state changes
3. **File markers** — Look for `.done`, `.completed` files

**Debounce**: 200ms window (configurable) to prevent flapping on transient states.

```rust
// Pseudo-code
loop {
    for each active work_package:
        check spec-kitty lane state
        check git status
        check for completion markers
        if any signal met:
            add to completion_candidates
    
    if debounce_window expired:
        for each candidate in completion_candidates:
            mark WP::Completed
            debounce_window.reset()
}
```

## Command Processing

### FIFO Command Flow

```
Controller pane (opencode) or external terminal
      │
      ├─ echo "restart WP01" > .kasmos/cmd.pipe
      │
      ▼
CommandReader Task
      │
      ├─ Listen on FIFO (epoll-based, async)
      ├─ Parse command string
      │  └─ Validate syntax, extract arguments
      │
      ▼
CommandHandler
      │
      ├─ Match ControllerCommand variant
      ├─ Dispatch to engine or session controller
      │
      ├─ Restart WP01
      │  └─ Send EngineAction::Restart("WP01")
      │
      ├─ Pause WP02
      │  └─ Send EngineAction::Pause("WP02")
      │
      ├─ Focus WP03
      │  └─ Call SessionController::focus_pane("WP03")
      │     (Currently NoOpSessionCtrl; future enhancement)
      │
      ├─ Status
      │  └─ Read OrchestrationRun, format status table
      │
      └─ Abort
         └─ Send EngineAction::Abort, trigger shutdown
```

### Available Commands

| Command | Type | Handler |
|---------|------|---------|
| `restart <WP_ID>` | EngineAction | Restart pane, unblock dependents |
| `pause <WP_ID>` | EngineAction | Pause WP, wait for `resume` |
| `resume <WP_ID>` | EngineAction | Resume paused WP |
| `status` | Read-only | Display current state table |
| `focus <WP_ID>` | SessionCtrl | Navigate to pane (not yet implemented) |
| `zoom <WP_ID>` | SessionCtrl | Maximize pane (not yet implemented) |
| `abort` | EngineAction | Gracefully shutdown |
| `advance` | EngineAction | Confirm wave advancement (wave-gated) |
| `force-advance <WP_ID>` | EngineAction | Skip failed WP, unblock dependents |
| `retry <WP_ID>` | EngineAction | Reset WP to Pending, re-run |
| `help` | Read-only | Display command reference |

## Configuration Hierarchy

Configuration is resolved in this order (highest to lowest priority):

1. **CLI arguments** (if any)
2. **Environment variables** (`KASMOS_*`)
3. **Config file** (`.kasmos/config.toml` or `~/.kasmos/config.toml`)
4. **Built-in defaults**

**Example:**
```bash
# Defaults: controller_width_pct = 40
export KASMOS_CONTROLLER_WIDTH=35          # Env overrides default
# Config file can override env
# CLI args (if any) override config

kasmos launch . --mode wave-gated           # Mode from CLI
```

## State Persistence

### state.json Structure

```json
{
  "id": "run-20250210-143022-abc123",
  "feature": "001-zellij-agent-orchestrator",
  "feature_dir": "/home/user/projects/kasmos/features/001-zellij-agent-orchestrator",
  "config": {
    "max_agent_panes": 8,
    "progression_mode": "continuous",
    "zellij_binary": "zellij",
    "opencode_binary": "opencode",
    "spec_kitty_binary": "spec-kitty",
    "kasmos_dir": ".kasmos",
    "poll_interval_secs": 5,
    "debounce_ms": 200,
    "controller_width_pct": 40
  },
  "work_packages": [
    {
      "id": "WP01",
      "title": "Set up Zellij integration",
      "state": "active",
      "dependencies": [],
      "wave": 0,
      "pane_id": 5,
      "pane_name": "agent-1",
      "worktree_path": "/tmp/worktree/WP01",
      "prompt_path": "/tmp/prompts/WP01/prompt.md",
      "started_at": "2025-02-10T14:30:22.000Z",
      "completed_at": null,
      "completion_method": null,
      "failure_count": 0
    },
    ...
  ],
  "waves": [
    {
      "index": 0,
      "wp_ids": ["WP01", "WP02"],
      "state": "active"
    },
    {
      "index": 1,
      "wp_ids": ["WP03", "WP04"],
      "state": "pending"
    }
  ],
  "state": "running",
  "started_at": "2025-02-10T14:30:22.000Z",
  "completed_at": null,
  "mode": "continuous"
}
```

**Persistence Strategy:**
- After every state transition, serialize full `OrchestrationRun`
- Write to `.kasmos/state.json` atomically
- Allows `attach` to reconnect without re-reading spec
- Allows `status` to report current state even if Zellij crashes

## Known Limitations & Design Trade-offs

### 1. NoOpChecker for Health Detection
**Limitation:** Zellij 0.41+ removed `list-panes` API. kasmos cannot dynamically query live panes or check real pane liveness.

**Current Status:** Health monitor uses `NoOpChecker` as a placeholder. Real crash detection depends on external signals: pane process exit status (from shell wrapper), spec-kitty lane completion markers, and git worktree state changes.

**Trade-off:** Pane count must be pre-configured via `max_agent_panes`; no dynamic pane discovery.

**Future:** When Zellij exposes pane introspection, upgrade to real health checker with `list_live_panes`.

### 2. Focus/Zoom Commands Not Fully Implemented
**Limitation:** Zellij 0.41+ removed `focus-terminal-pane --pane-id` and `write-chars-to-pane-id`.

**Current Status:** `focus <WP_ID>` and `zoom <WP_ID>` FIFO commands are accepted and parsed, but route to `NoOpSessionCtrl`—they do not actually control Zellij pane focus today.

**Workaround:** Use Zellij keybinds (`Ctrl+p → h/j/k/l` for directional navigation, `Tab` for cyclic focus) to manually navigate panes.

**Future:** When Zellij API stabilizes, implement pane focusing via alternate mechanisms.

### 3. No Built-in Floating Pane for Quick Help
**Limitation:** kasmos doesn't auto-spawn a help overlay in Zellij.

**Trade-off:** Users manually create floating panes with `Ctrl+p → w` if desired.

**Workaround:** See [keybinds.md](./keybinds.md) for floating pane recipes.

### 4. Single Progression Mode Active at Runtime
**Limitation:** Zellij layout is static; can't reconfigure grid mid-run.

**Trade-off:** Choose progression mode at launch; can't switch between Continuous and Wave-Gated without restart.

**Future:** Support dynamic mode switching or wave reconfiguration.

## Summary Table

| Component | Purpose | Status |
|-----------|---------|--------|
| Feature Parser | Extract work packages from spec | ✅ Implemented |
| Dependency Graph | Organize WPs into waves | ✅ Implemented |
| Layout Generator | Create KDL from WP list | ✅ Implemented |
| Zellij Spawner | Launch Zellij session | ✅ Implemented |
| FIFO Command Reader | Accept operator input | ✅ Implemented |
| Health Checker | Detect pane crashes | ⚠️ Placeholder (NoOpChecker) |
| Completion Detector | Detect WP completion | ✅ Implemented |
| Wave Executor | Manage wave progression | ✅ Implemented |
| Session Controller (focus/zoom) | Navigate panes | ⚠️ Placeholder (NoOpSessionCtrl) |
| State Persistence | Save/resume orchestration | ✅ Implemented |
| State Machine | WP/Run state transitions | ✅ Implemented |

---

See [getting-started.md](./getting-started.md) for user-facing workflows and [cheatsheet.md](./cheatsheet.md) for quick command reference.
