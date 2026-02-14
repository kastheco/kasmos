# Quickstart: MCP Agent Swarm Orchestration

**Feature**: 011-mcp-agent-swarm-orchestration
**Date**: 2026-02-13

---

## Prerequisites

| Dependency | Min Version | Check Command |
|-----------|-------------|---------------|
| Rust | Latest stable (2024 edition) | `rustc --version` |
| Zellij | 0.43.x+ | `zellij --version` |
| OpenCode | Latest | `opencode --version` |
| zellij-pane-tracker plugin | Forked version | `zellij plugin list` (verify loaded) |
| zellij-pane-tracker MCP server | Forked version | Check OpenCode profile MCP config |

## Build & Install

```bash
# From repository root
cargo build -p kasmos
# or install globally
cargo install --path crates/kasmos
```

## First-Time Setup

```bash
kasmos setup
```

This validates:
1. All dependencies are installed and in PATH
2. Zellij version is compatible (0.43.x+)
3. OpenCode is configured with the kasmos profile
4. zellij-pane-tracker plugin is installed
5. `kasmos.toml` exists (generates defaults if missing)
6. OpenCode kasmos profile exists (generates if missing)

## Configuration

### Project Config (`kasmos.toml` in repo root)

```toml
[agent]
binary = "opencode"
profile = "kasmos"
max_parallel_workers = 4
review_retry_cap = 3

[communication]
poll_interval_seconds = 5
wait_timeout_seconds = 120

[paths]
worktree_base = ".worktrees"
audit_dir = ".kasmos"
specs_dir = "kitty-specs"

[session]
name = "kasmos"
```

### OpenCode Profile (`~/.config/opencode/profiles/kasmos/opencode.jsonc`)

The kasmos profile defines 5 agent roles and registers kasmos as an MCP server:

```jsonc
{
  "mcp": {
    "kasmos": {
      "type": "local",
      "command": ["kasmos", "serve"],
      "enabled": true
    }
    // zellij-pane-tracker, exa, context7 also configured
  },
  "agents": {
    "manager": { "model": "anthropic/claude-opus-4-6", "temperature": 0.3 },
    "planner": { "model": "anthropic/claude-opus-4-6", "temperature": 0.3 },
    "coder":   { "model": "anthropic/claude-sonnet-4-20250514", "temperature": 0.5 },
    "reviewer": { "model": "anthropic/claude-opus-4-6", "temperature": 0.2 },
    "release": { "model": "anthropic/claude-opus-4-6", "temperature": 0.3 }
  }
}
```

## Usage

### Launch a Session

```bash
# Bind to a specific feature spec:
kasmos 011

# Auto-detect from branch (e.g., on branch 011-mcp-agent-swarm-orchestration):
kasmos

# Feature selector (when no spec can be inferred):
kasmos
```

### What Happens After Launch

1. `kasmos` generates a Zellij layout with two tabs (MCP + orchestration)
2. Launches the Zellij session (or adds tabs if already inside Zellij)
3. `kasmos` exits (fire-and-forget)
4. Manager agent starts in the orchestration tab
5. OpenCode auto-spawns `kasmos serve` as the manager's MCP subprocess
6. Manager assesses workflow state and greets you
7. You confirm the next phase (planning / implementation / release)
8. Manager spawns workers, monitors progress via dashboard
9. You see live status updates in the dashboard pane

### Inside the Session

| Pane | What's Happening |
|------|-----------------|
| **manager** | Your primary interaction point. The manager asks for confirmations and reports status. |
| **msg-log** | Raw structured messages from all agents. Watch this for the full event stream. |
| **dashboard** | Formatted live status table. Updated every 5s during `wait_for_event`. |
| **worker panes** | Appear/disappear as workers are spawned/despawned. Each shows an OpenCode agent working. |

### Standalone Commands

```bash
# List all feature specs and their status:
kasmos list

# Check WP progress for a feature:
kasmos status 011

# Validate environment:
kasmos setup
```

## Workflow Lifecycle

```
kasmos 011
  |
  v
Manager: "Feature 011 is at planning stage. Shall I start clarify?"
  |
  v (user confirms)
Manager spawns planner agent -> user interacts with planner
  |
  v (planner completes, writes to msg-log)
Manager: "Clarify done. Shall I proceed to plan?"
  |
  v (continues through specify -> clarify -> plan -> analyze -> tasks)
  |
  v
Manager: "Planning complete. 6 WPs in 3 waves. Start implementation?"
  |
  v (user confirms)
Manager spawns coder agents for wave 0
  |
  v (coders complete -> reviewers spawned -> reviews pass)
Manager progresses through waves
  |
  v (all WPs done)
Manager: "All WPs done. Proceed to release?"
  |
  v (user confirms)
Manager spawns release agent
  |
  v
Done.
```

## Troubleshooting

| Problem | Solution |
|---------|----------|
| `kasmos setup` reports missing dependency | Install the missing tool and re-run |
| Manager seems frozen | Check dashboard pane — if timestamp updates, it's waiting for workers. If frozen, check MCP tab for kasmos serve errors. |
| Worker pane disappeared | Manager will detect on next poll and report. Offer respawn or skip. |
| Review loop stuck | Automatically escalates after 3 iterations (configurable via `review_retry_cap`). |
| Dashboard not updating | Check that kasmos serve is running (MCP tab). Restart session if needed. |
| Session crashed | Re-run `kasmos 011` — manager reconstructs state from task files. |
