# kasmos Architecture Intelligence

> Codebase discoveries and architectural knowledge accumulated during development.
> This file is the authority on how kasmos internals work and interact.
> Updated: 2026-02-16

## System Overview

kasmos is an MCP-first orchestration CLI. It has three runtime modes:

1. **Bootstrap/launcher** (`kasmos [PREFIX]`) -- resolves a feature spec, runs preflight checks, acquires a feature lock, generates a KDL layout, and creates a Zellij session/tab with a manager agent pane, message-log pane, dashboard pane, and worker area.
2. **MCP server** (`kasmos serve`) -- stdio transport server providing tools for worker lifecycle, message reading, workflow status, WP lane transitions, and feature lock management. Spawned as a subprocess by the manager agent (not as a separate pane).
3. **Utilities** (`kasmos setup`, `kasmos list`, `kasmos status`) -- environment validation, feature listing, and progress reporting.

The TUI code from specs 002-010 is preserved behind `#[cfg(feature = "tui")]` and is not compiled by default. It is inert.

## Worktree Structure

kasmos uses git worktrees for WP isolation during orchestration.

- Location: `<repo_root>/.worktrees/<feature_slug>-<wp_id>/`
- Each worktree is a full repo checkout on its own branch.
- The worktree contains its own copy of `kitty-specs/` files.
- The `.kittify/memory/` directory inside worktrees is a **symlink** back to the main repo's `.kittify/memory/`, so constitution and memory are shared.

### Worktree vs main repo file paths

This is a critical distinction that affects multiple subsystems:

- **Main repo** `kitty-specs/<slug>/tasks/WPxx.md` -- the canonical task files, versioned in git.
- **Worktree** `.worktrees/<slug>-<wp_id>/kitty-specs/<slug>/tasks/WPxx.md` -- the agent's working copy.

When an agent modifies a task file (e.g., moving its lane from `doing` to `for_review`), it modifies the **worktree copy**, not the main repo copy. Any subsystem that watches for file changes (e.g., `CompletionDetector` in `crates/kasmos/src/detector.rs`) must watch the worktree path, not the main repo path, when worktrees are in use.

## Zellij Integration

### Session architecture (MCP era)

- `kasmos [PREFIX]` creates or attaches to a Zellij session named per `config.session.session_name` (default: `kasmos`).
- If already inside Zellij, it creates a new tab instead of a new session (`launch/session.rs`).
- The layout has a fixed top row (22% height) with three panes: **manager** (left), **msg-log** (center), **dashboard** (right). Below is the **worker-area** where worker panes are dynamically spawned.
- `swap_tiled_layout` rules handle reflowing as workers are added/removed (up to `max_parallel_workers + 3` panes).
- The session starts in Zellij `locked` mode to avoid accidental keybind interference.

### Layout structure (from `launch/layout.rs`)

```
+---manager(60%)---+--msg-log(20%)--+--dashboard(20%)--+  <- 22% height
|                                                       |
|                    worker-area                         |  <- remaining
|                                                       |
+-------------------------------------------------------+
```

Width percentages are configurable via `session.manager_width_pct` and `session.message_log_width_pct`. Dashboard gets the remainder.

### Manager pane command

The manager pane runs `ocx oc [-p <profile>] -- --agent manager --prompt <prompt>`. The prompt is built by `RolePromptBuilder` with a phase hint (specify/plan/tasks/implement) derived from which artifacts exist in the feature directory.

`kasmos serve` is NOT a pane command -- it runs as an MCP stdio subprocess owned by the manager agent's OpenCode/Claude Code profile config.

### Zellij CLI limitations (v0.41+)

- There is **no** `list-panes` or `focus-pane-by-name` CLI command.
- Inside a Zellij session, use `zellij action <cmd>` directly (no `--session` flag needed).
- From outside a session, use `zellij --session <name> action <cmd>`.
- Worker panes are tracked by the MCP server's `WorkerRegistry` (in-memory HashMap keyed by `wp_id:role`), not by Zellij introspection.

## MCP Server Architecture

### Server (`serve/mod.rs`)

`KasmosServer` is an `rmcp` server with stdio transport. State:
- `config: Config` -- loaded from `kasmos.toml` + env
- `registry: Arc<RwLock<WorkerRegistry>>` -- tracks spawned workers
- `message_cursor: Arc<RwLock<u64>>` -- tracks read position in message log
- `feature_slug: Option<String>` -- inferred from `specs_root` path
- `audit: Arc<Mutex<Option<AuditWriter>>>` -- per-feature audit log

### MCP tools (9 registered)

| Tool | Purpose |
|------|---------|
| `spawn_worker` | Create a planner/coder/reviewer/release worker pane |
| `despawn_worker` | Close a worker pane and remove from registry |
| `list_workers` | List tracked workers with status filter |
| `read_messages` | Parse message-log pane events |
| `wait_for_event` | Block until matching event or timeout |
| `workflow_status` | Return feature phase, wave status, lock metadata |
| `transition_wp` | Validate and apply WP lane transitions in task files |
| `list_features` | List known specs and artifact availability |
| `infer_feature` | Resolve feature slug from arg, branch, or cwd |

### Agent roles

Defined in `serve/registry.rs`. Worker roles: `planner`, `coder`, `reviewer`, `release`. The `Manager` role exists in `prompt.rs` but is never a registry worker -- it's the orchestrator agent that calls MCP tools.

### Context boundaries (`prompt.rs`)

Each role gets different context injected into its prompt:

| Context | Manager | Planner | Coder | Reviewer | Release |
|---------|---------|---------|-------|----------|---------|
| spec.md | yes | yes | no | no | no |
| plan.md | yes | yes | no | no | no |
| all tasks | yes | no | no | no | yes |
| architecture memory | yes | yes | yes | yes | no |
| workflow intelligence | yes | yes | no | no | no |
| constitution | yes | yes | yes | yes | yes |
| project structure | yes | yes | no | no | yes |
| WP task file | no | no | yes | yes | no |
| coding standards | no | no | yes | yes | no |

## Configuration

Config is loaded from `kasmos.toml` (discovered by walking up from CWD) with `KASMOS_*` env var overrides. Key sections:

| Section | Key fields | Location |
|---------|-----------|----------|
| `[agent]` | `max_parallel_workers`, `opencode_binary`, `opencode_profile`, `review_rejection_cap` | `config.rs:51` |
| `[communication]` | `poll_interval_secs`, `event_timeout_secs` | `config.rs:64` |
| `[paths]` | `zellij_binary`, `spec_kitty_binary`, `specs_root` | `config.rs:73` |
| `[session]` | `session_name`, `manager_width_pct`, `message_log_width_pct`, `max_workers_per_row` | `config.rs:84` |
| `[audit]` | `metadata_only`, `debug_full_payload`, `max_bytes`, `max_age_days` | `config.rs:97` |
| `[lock]` | `stale_timeout_minutes` | `config.rs:110` |

Legacy flat keys (`max_agent_panes`, `controller_width_pct`, etc.) are still accepted for backward compatibility and synced into the sectioned fields.

## Key Type Definitions

| Type | Location | Notes |
|------|----------|-------|
| `KasmosServer` | `serve/mod.rs` | MCP server with tool router, registry, audit |
| `WorkerRegistry` | `serve/registry.rs` | In-memory worker tracking, keyed by `wp_id:role` |
| `WorkerEntry` | `serve/registry.rs` | Worker metadata: role, pane_name, status, events |
| `AgentRole` (worker) | `serve/registry.rs` | Planner, Coder, Reviewer, Release |
| `AgentRole` (prompt) | `prompt.rs` | Adds Manager variant for orchestrator prompts |
| `OrchestrationLayout` | `launch/layout.rs` | KDL layout builder with swap-tiled reflow |
| `ManagerCommand` | `launch/layout.rs` | Manager pane command (binary, profile, prompt) |
| `RolePromptBuilder` | `prompt.rs` | Context-boundary-aware prompt construction |
| `FeatureLockManager` | `serve/lock.rs` | Per-feature lock with heartbeat and stale detection |
| `Config` | `config.rs` | Sectioned TOML config with env overrides |
| `WorkPackage` | `types.rs` | Has `pane_id: Option<u32>`, `worktree_path`, `pane_name` |
| `CompletionDetector` | `detector.rs` | Watches task files for lane transitions |
| `WorktreeManager` | `git.rs` | Creates worktrees at `.worktrees/{feature_name}-{wp_id}` |

## Agent Permissions and External Directories

### Problem discovered (2026-02-14)

Agents running in worktrees (e.g., `.worktrees/011-...-WP02/`) need read access to paths outside their CWD -- specifically the main repo's `kitty-specs/` directory (which is gitignored and doesn't exist in worktrees) and `/tmp/` (where spec-kitty writes review prompts).

OpenCode's `external_directory` permission config does **not** expand `~` to the home directory. Paths like `"~/dev/kasmos/**": "allow"` silently fail to match absolute paths like `/home/kas/dev/kasmos/kitty-specs/...`, causing `auto-rejecting` when the agent runs non-interactively (e.g., `ocx oc -- run`).

**Fix**: Always use fully-qualified absolute paths in `external_directory` rules.

### Paths agents commonly need

| Path | Who needs it | Why |
|------|-------------|-----|
| `/home/kas/dev/kasmos/**` (main repo) | All agents | `kitty-specs/`, `.kittify/memory/`, docs |
| `/tmp/*`, `/tmp/**` | All agents | spec-kitty review prompts, temp files |
| `~/.config/opencode/**` | All agents | Self-reference for config |
| `~/.config/zellij/**` | Manager, release | Layout management |
