# Implementation Plan: MCP Agent Swarm Orchestration

**Branch**: `011-mcp-agent-swarm-orchestration` | **Date**: 2026-02-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `kitty-specs/011-mcp-agent-swarm-orchestration/spec.md`

## Summary

Pivot kasmos from a TUI-based orchestrator to an MCP-powered agent swarm where a manager agent coordinates planning, implementation, review, and release through Zellij panes. The `kasmos` binary becomes a fire-and-forget session launcher, `kasmos serve` becomes an MCP server (stdio subprocess per manager), and all orchestration intelligence moves into the manager agent's prompt + MCP tool calls. Five distinct OpenCode agent profiles (manager, planner, coder, reviewer, release) replace the current engine/detector/session-manager architecture.

## Technical Context

**Language/Version**: Rust (2024 edition, latest stable)
**Primary Dependencies**:
- `rmcp` v0.15 — MCP SDK (official, proc macros, schemars integration)
- `tokio` — async runtime (already in deps)
- `clap` — CLI framework (already in deps)
- `serde` + `serde_json` + `serde_yaml` — serialization (already in deps)
- `toml` — config parsing (already in deps)
- `kdl` — Zellij layout generation (already in deps)
- `chrono` — timestamps (already in deps)
- `tracing` — logging (already in deps)
- `thiserror` + `anyhow` — error handling (already in deps)
- New: `schemars` (transitive via rmcp), `flock`/`nix` for advisory locks (nix already in deps)
**Storage**: Filesystem — spec-kitty task files as SSOT, `.kasmos/messages.jsonl` for audit persistence
**Testing**: `cargo test` — unit tests for MCP tool handlers, integration tests for layout generation and message parsing
**Target Platform**: Linux (primary), macOS (best-effort)
**Project Type**: Single Rust binary (workspace crate `crates/kasmos/`)
**Performance Goals**: Session launch <10s (SC-001), event detection <15s (SC-002), 4+ concurrent workers without degradation (SC-005)
**Constraints**: Single binary via `cargo install`, Zellij 0.43.x+ and OpenCode must be in PATH

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Rust 2024 edition | ✅ PASS | No change |
| tokio async runtime | ✅ PASS | Already used, MCP server is async |
| ratatui for TUI | ✅ PASS | Preserved behind `#[cfg(feature = "tui")]` per FR-024; not deleted |
| Zellij substrate | ✅ PASS | Core of the architecture |
| OpenCode primary agent | ✅ PASS | All 5 agent roles use OpenCode |
| cargo test required | ✅ PASS | Unit + integration tests planned |
| Linux primary, macOS best-effort | ✅ PASS | No platform-specific changes |
| Single binary | ✅ PASS | `kasmos` remains one binary with subcommands |

**No constitution violations. No complexity justifications needed.**

## Engineering Alignment

Resolved during planning interrogation (7 questions):

| # | Decision | Resolution |
|---|----------|-----------|
| Q1 | kasmos serve architecture | One `kasmos serve` per manager agent, spawned as stdio MCP subprocess by OpenCode. Feature-scoped. Workers do NOT get kasmos MCP. |
| Q2 | Worktree strategy | Keep per-WP worktrees (`.worktrees/<slug>-<wp_id>/`). Manager creates before spawning coder, sets `--cwd` to worktree. |
| Q3 | Manager lifecycle | Fire-and-forget launcher. `kasmos` generates Zellij layout, spawns session with manager pane, then exits. OpenCode auto-spawns `kasmos serve` as stdio subprocess. |
| Q4 | Failure & recovery | Hybrid: retry-in-place for review rejections (2-3 cap), abandon & escalate for crashes and merge conflicts. |
| Q5 | Worker→manager comms | Blocking MCP tools (`wait_for_event`) keep manager in agentic loop. Workers write structured markers to msg-log pane. Dashboard pane updated by kasmos serve during poll cycles for live status. Timeout returns after configurable ceiling to prevent indefinite blocking. |
| Q6 | Config format | TOML for project config (`kasmos.toml`). Zellij layout files remain KDL (Zellij's native format). |
| Q7 | Planner agent | Distinct OpenCode profile (`planner`), not mapped to `controller`. User-interactive, main repo, zellij MCP only. Spawned by manager for planning stages. |
| CF | Agent binary | `opencode` as default (configurable in kasmos.toml). No `ocx` dependency. Existing hardcoded refs replaced during ADAPT phase. |

## Architecture Overview

### Agent Roles and MCP Access

| Role | kasmos MCP | zellij MCP | Interaction | Working Dir | OpenCode Profile |
|------|-----------|------------|-------------|-------------|-----------------|
| Manager | ✅ | ✅ | User (hub) | main repo | `manager` (new) |
| Planner | ❌ | ✅ (msg-log) | User (direct) | main repo | `planner` (new) |
| Coder | ❌ | ✅ (msg-log) | Autonomous | worktree | `coder` (existing) |
| Reviewer | ❌ | ✅ (msg-log) | Autonomous | worktree | `reviewer` (existing) |
| Release | ❌ | ✅ (msg-log) | Autonomous | main repo | `release` (existing) |

### Zellij Session Layout

```
┌─────────────────────────────────────────────────────────────┐
│ Tab: "MCP" (hidden, background)                             │
│ ┌─────────────────────────────────────────────────────────┐ │
│ │ kasmos serve (visible for debugging if needed)          │ │
│ └─────────────────────────────────────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│ Tab: "orchestration" (active)                               │
│ ┌──────────────────────────┬──────────────┬───────────────┐ │
│ │ manager (60%)            │ msg-log (20%)│ dashboard(20%)│ │
│ │ OpenCode --agent manager │ structured   │ live status   │ │
│ │                          │ messages     │ (updated by   │ │
│ │                          │ from workers │ kasmos serve)  │ │
│ ├──────────────────────────┴──────────────┴───────────────┤ │
│ │ Worker area (dynamically managed)                       │ │
│ │ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐        │ │
│ │ │ WP-01 coder │ │ WP-02 coder │ │ WP-03 coder │        │ │
│ │ └─────────────┘ └─────────────┘ └─────────────┘        │ │
│ └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Manager Agentic Loop (Blocking MCP Pattern)

```
Manager spawns workers → calls wait_for_event() → BLOCKED
    kasmos serve internally:
      - polls msg-log pane every 3-5s via zellij dump-pane
      - parses structured messages
      - updates dashboard pane with formatted status
      - on matching event: returns to LLM
Manager processes event → takes action → calls wait_for_event() again
```

The manager LLM never "sleeps" — it's always mid-tool-call waiting for a response. `wait_for_event` returns `{status: "timeout", elapsed: Ns}` after a configurable ceiling (default 120s), allowing the manager to re-assess and re-wait.

### Communication Protocol

Workers write to msg-log pane via zellij MCP `run_in_pane`:
```
echo "[KASMOS:<sender>:<event>] <json_data>"
```

Events: `STARTED`, `PROGRESS`, `DONE`, `ERROR`, `REVIEW_PASS`, `REVIEW_REJECT`, `NEEDS_INPUT`

Example:
```
[KASMOS:WP-01-coder:DONE] {"wp_id":"WP-01","summary":"Implemented config parser"}
[KASMOS:WP-01-reviewer:REVIEW_PASS] {"wp_id":"WP-01","approved":true}
[KASMOS:WP-02-coder:ERROR] {"wp_id":"WP-02","error":"Build failed","retryable":true}
```

### Failure & Recovery Model

| Failure Type | Response | Cap |
|-------------|----------|-----|
| Review rejection | Retry: re-spawn coder with reviewer feedback | 2-3 iterations |
| Coder crash | Escalate: log to msg-log, mark WP blocked, continue others | N/A |
| Merge conflict | Escalate: halt merge, log details, notify user | N/A |
| kasmos serve crash | Manager detects MCP disconnect, pauses automation, notifies user | N/A |
| Pane manually closed | Detect on next poll, treat as abort, offer respawn/skip | N/A |
| wait_for_event timeout | Return timeout status, manager re-assesses | Configurable |

## Project Structure

### Documentation (this feature)

```
kitty-specs/011-mcp-agent-swarm-orchestration/
├── spec.md              # Feature specification
├── research.md          # Research decisions and risks
├── plan.md              # This file
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # MCP tool JSON schemas
│   └── kasmos-serve.json
└── tasks/               # Work packages (created by /spec-kitty.tasks)
```

### Source Code (repository root)

```
crates/kasmos/
├── Cargo.toml           # Updated deps (add rmcp, schemars; feature-gate ratatui)
└── src/
    ├── main.rs           # ADAPT: New CLI (kasmos [spec], serve, setup, list, status)
    ├── lib.rs            # ADAPT: Re-export public API
    │
    ├── launch/           # NEW: Session launcher (replaces start.rs)
    │   ├── mod.rs        # kasmos [spec-prefix] entry point
    │   ├── layout.rs     # ADAPT from layout.rs: Session + worker KDL generation
    │   ├── session.rs    # Zellij session/tab creation
    │   └── detect.rs     # Feature detection (branch, dir, arg, selector)
    │
    ├── serve/            # NEW: MCP server
    │   ├── mod.rs        # kasmos serve entry point + MCP router
    │   ├── tools/        # MCP tool handlers (one file per tool)
    │   │   ├── spawn_worker.rs
    │   │   ├── despawn_worker.rs
    │   │   ├── list_workers.rs
    │   │   ├── read_messages.rs
    │   │   ├── wait_for_event.rs   # Blocking poll with dashboard updates
    │   │   ├── workflow_status.rs
    │   │   ├── transition_wp.rs
    │   │   ├── list_features.rs
    │   │   └── infer_feature.rs
    │   ├── registry.rs   # In-memory worker registry (pane_id → role/wp/status)
    │   ├── messages.rs   # Message parsing and JSONL persistence
    │   └── dashboard.rs  # Dashboard pane formatter (ANSI status display)
    │
    ├── setup/            # NEW: Environment validation
    │   └── mod.rs        # kasmos setup entry point
    │
    ├── config.rs         # ADAPT: TOML-based config (kasmos.toml schema)
    ├── zellij.rs         # ADAPT: Zellij CLI wrapper (new pane ops)
    ├── prompt.rs         # ADAPT: OpenCode prompt generation per role
    │
    │   # KEEP (unchanged)
    ├── types.rs          # WP types, lane states, wave definitions
    ├── state_machine.rs  # Lane transition logic
    ├── git.rs            # Git operations (branch detection, worktree management)
    ├── graph.rs          # Dependency graph for wave ordering
    ├── error.rs          # Error types
    ├── parser.rs         # Task file parser
    ├── persistence.rs    # State file I/O
    ├── logging.rs        # Tracing setup
    ├── signals.rs        # Signal handling
    ├── cleanup.rs        # Resource cleanup
    ├── feature_arg.rs    # Feature argument resolution
    ├── review.rs         # Review result types
    ├── list_specs.rs     # kasmos list implementation
    ├── status.rs         # kasmos status implementation
    │
    │   # UNWIRE (behind cfg(feature = "tui"))
    ├── tui/              # TUI module (preserved, feature-gated)
    ├── hub/              # Hub TUI (preserved, feature-gated)
    ├── tui_cmd.rs        # TUI launcher (preserved, feature-gated)
    │
    │   # REMOVE (replaced by serve/ and manager agent)
    ├── engine.rs         # → serve/tools/workflow_status.rs + manager agent
    ├── session.rs        # → launch/session.rs
    ├── detector.rs       # → serve/tools/wait_for_event.rs
    ├── cmd.rs            # → serve/ MCP tools
    ├── commands.rs       # → serve/ MCP tools
    ├── command_handlers.rs # → serve/ MCP tools
    ├── health.rs         # → serve/tools/workflow_status.rs
    ├── shutdown.rs       # → cleanup.rs (simplified)
    ├── review_coordinator.rs # → manager agent logic
    ├── start.rs          # → launch/mod.rs
    ├── sendmsg.rs        # → serve/messages.rs
    ├── attach.rs         # REMOVE (Zellij native attach)
    ├── stop.rs           # REMOVE (Zellij native kill)
    └── report.rs         # UNWIRE behind tui feature

config/                   # NEW: OpenCode agent profiles for kasmos
├── profiles/
│   └── kasmos/
│       ├── opencode.jsonc  # Agent definitions (manager, planner, coder, reviewer, release)
│       └── agent/
│           ├── manager.md  # Manager agent instructions
│           ├── planner.md  # Planner agent instructions
│           ├── coder.md    # Coder agent instructions (scoped)
│           ├── reviewer.md # Reviewer agent instructions
│           └── release.md  # Release agent instructions
```

**Structure Decision**: This is a single Rust binary workspace crate (`crates/kasmos/`) with the source reorganized into three primary modules: `launch/` (session bootstrapping), `serve/` (MCP server), and `setup/` (validation). Existing portable modules (`types.rs`, `state_machine.rs`, `git.rs`, `graph.rs`, `parser.rs`, etc.) are kept in place. TUI code is preserved behind a feature flag. OpenCode agent profiles are shipped in `config/profiles/kasmos/` and installed during `kasmos setup`.

## MCP Tool Contracts

### kasmos serve — 9 tools

#### 1. `spawn_worker`
Spawns a worker agent pane in the Zellij session.
```json
{
  "name": "spawn_worker",
  "description": "Spawn a worker agent in a new Zellij pane",
  "inputSchema": {
    "type": "object",
    "properties": {
      "wp_id": { "type": "string", "description": "Work package ID (e.g., 'WP-01')" },
      "role": { "type": "string", "enum": ["planner", "coder", "reviewer", "release"], "description": "Agent role" },
      "prompt": { "type": "string", "description": "Initial prompt for the agent" },
      "cwd": { "type": "string", "description": "Working directory (worktree path for coders, main repo for others)" },
      "context_files": { "type": "array", "items": { "type": "string" }, "description": "Files to include in agent context" }
    },
    "required": ["wp_id", "role", "prompt"]
  }
}
```
**Implementation**: Calls `zellij --session kasmos action new-pane --name "<wp_id>-<role>" --cwd <cwd> -- opencode -p kasmos -- --agent <role> --prompt <prompt>`. Registers worker in in-memory registry. Returns `{pane_name, pane_id, wp_id, role}`.

#### 2. `despawn_worker`
Closes a worker agent's pane.
```json
{
  "name": "despawn_worker",
  "inputSchema": {
    "type": "object",
    "properties": {
      "wp_id": { "type": "string", "description": "Work package ID to despawn" }
    },
    "required": ["wp_id"]
  }
}
```
**Implementation**: Looks up pane from registry, navigates via zellij-pane-tracker focus-cycling, calls `zellij action close-pane`. Removes from registry. Returns `{success, wp_id}`.

#### 3. `list_workers`
Lists all active worker panes and their statuses.
```json
{
  "name": "list_workers",
  "inputSchema": { "type": "object", "properties": {} }
}
```
**Implementation**: Returns in-memory registry reconciled with `zellij-pane-tracker get_panes`. Returns `{workers: [{wp_id, role, pane_name, status, elapsed_seconds}]}`.

#### 4. `read_messages`
Reads structured messages from the msg-log pane.
```json
{
  "name": "read_messages",
  "inputSchema": {
    "type": "object",
    "properties": {
      "since": { "type": "integer", "description": "Return messages after this index (0-based). Omit for all." },
      "filter_wp": { "type": "string", "description": "Filter to messages from a specific WP" },
      "filter_event": { "type": "string", "description": "Filter to a specific event type" }
    }
  }
}
```
**Implementation**: Dumps msg-log pane via `zellij dump-pane`, parses `[KASMOS:*:*]` lines, applies filters. Returns `{messages: [{index, sender, event, data, raw_line}], total}`.

#### 5. `wait_for_event`
Blocks until a matching event appears in the msg-log pane. Updates dashboard as a side effect.
```json
{
  "name": "wait_for_event",
  "inputSchema": {
    "type": "object",
    "properties": {
      "filter_event": { "type": "array", "items": { "type": "string" }, "description": "Event types to match (e.g., ['DONE', 'ERROR', 'REVIEW_PASS'])" },
      "filter_wp": { "type": "string", "description": "Optional: only match events from this WP" },
      "timeout_seconds": { "type": "integer", "description": "Max seconds to wait before returning timeout (default: 120)" },
      "poll_interval_seconds": { "type": "integer", "description": "Seconds between polls (default: 5)" }
    }
  }
}
```
**Implementation**: Enters async poll loop:
1. Dump msg-log pane, parse new messages since last known index
2. Update dashboard pane with formatted worker status table
3. If matching event found → return `{status: "event", event: {...}, elapsed_seconds}`
4. If timeout exceeded → return `{status: "timeout", elapsed_seconds, last_messages: [...]}`
5. Sleep `poll_interval_seconds`, repeat

#### 6. `workflow_status`
Queries the current workflow state by scanning spec-kitty task files.
```json
{
  "name": "workflow_status",
  "inputSchema": {
    "type": "object",
    "properties": {
      "feature": { "type": "string", "description": "Feature slug (auto-detected if omitted)" }
    }
  }
}
```
**Implementation**: Scans `kitty-specs/<feature>/tasks/` for task files, reads YAML frontmatter for lane states. Returns `{feature, phase, waves: [{wave_id, wps: [{wp_id, lane, title}]}], summary}`.

#### 7. `transition_wp`
Moves a work package to a new lifecycle state.
```json
{
  "name": "transition_wp",
  "inputSchema": {
    "type": "object",
    "properties": {
      "wp_id": { "type": "string" },
      "target_lane": { "type": "string", "enum": ["pending", "active", "for_review", "done", "rework", "blocked"] },
      "reason": { "type": "string", "description": "Why this transition (logged to audit)" }
    },
    "required": ["wp_id", "target_lane"]
  }
}
```
**Implementation**: Validates transition against `state_machine.rs` rules. Acquires `flock` on task file. Updates YAML frontmatter `lane` field. Appends to `.kasmos/messages.jsonl` audit log. Returns `{wp_id, from_lane, to_lane, valid}`.

#### 8. `list_features`
Lists available feature specs in the repository.
```json
{
  "name": "list_features",
  "inputSchema": { "type": "object", "properties": {} }
}
```
**Implementation**: Scans `kitty-specs/` for directories matching `###-*` pattern. Returns `{features: [{slug, has_spec, has_plan, has_tasks, wp_summary}]}`.

#### 9. `infer_feature`
Detects the current feature from environment context.
```json
{
  "name": "infer_feature",
  "inputSchema": { "type": "object", "properties": {} }
}
```
**Implementation**: Checks git branch name for `###-*` pattern, then current directory. Returns `{detected: bool, feature_slug, method: "branch"|"directory"|"none"}`.

## Data Model

### Configuration (`kasmos.toml`)

```toml
[agent]
binary = "opencode"           # Agent binary (default: "opencode")
profile = "kasmos"            # OpenCode profile name
max_parallel_workers = 4      # Concurrency limit per wave
review_retry_cap = 3          # Max review→rework iterations

[communication]
poll_interval_seconds = 5     # wait_for_event poll frequency
wait_timeout_seconds = 120    # wait_for_event max block duration
message_prefix = "KASMOS"     # Structured message prefix

[paths]
worktree_base = ".worktrees"  # Worktree root relative to repo
audit_dir = ".kasmos"         # Audit log directory
specs_dir = "kitty-specs"     # Feature specs directory

[session]
name = "kasmos"               # Zellij session name
```

### Worker Registry (in-memory)

```rust
pub struct WorkerEntry {
    pub wp_id: String,
    pub role: AgentRole,
    pub pane_name: String,
    pub spawned_at: chrono::DateTime<chrono::Utc>,
    pub status: WorkerStatus,
}

pub enum AgentRole {
    Planner,
    Coder,
    Reviewer,
    Release,
}

pub enum WorkerStatus {
    Active,
    Done,
    Errored(String),
    Aborted,
}
```

### Structured Message

```rust
pub struct KasmosMessage {
    pub index: usize,
    pub sender: String,          // e.g., "WP-01-coder"
    pub event: MessageEvent,
    pub data: serde_json::Value,
    pub timestamp: chrono::DateTime<chrono::Utc>,
    pub raw_line: String,
}

pub enum MessageEvent {
    Started,
    Progress,
    Done,
    Error,
    ReviewPass,
    ReviewReject,
    NeedsInput,
}
```

### Audit Log Entry (`.kasmos/messages.jsonl`)

```rust
pub struct AuditEntry {
    pub timestamp: chrono::DateTime<chrono::Utc>,
    pub actor: String,           // "manager", "WP-01-coder", "kasmos-serve"
    pub action: String,          // "spawn_worker", "transition_wp", etc.
    pub details: serde_json::Value,
}
```

### Dashboard Display State

```rust
pub struct DashboardState {
    pub workers: Vec<DashboardWorker>,
    pub last_event: Option<String>,
    pub elapsed: std::time::Duration,
    pub feature: String,
    pub phase: String,
}

pub struct DashboardWorker {
    pub wp_id: String,
    pub role: AgentRole,
    pub status: String,
    pub elapsed: std::time::Duration,
}
```

## Module Categorization (Codebase Migration)

From research Decision 6, refined with planning decisions:

### KEEP (use as-is)
| File | Purpose | Why Keep |
|------|---------|----------|
| `types.rs` | WP types, lane states, wave definitions | Core domain types, no TUI coupling |
| `state_machine.rs` | Lane transition validation | Pure logic, used by `transition_wp` tool |
| `git.rs` | Git operations, branch detection, worktrees | Essential for feature detection and worktree management |
| `graph.rs` | Dependency graph for wave ordering | Pure algorithm, used by `workflow_status` |
| `error.rs` | Error types | Foundation |
| `parser.rs` | Task file parser | Used by `workflow_status` and `transition_wp` |
| `persistence.rs` | State file I/O | Used by audit log persistence |
| `logging.rs` | Tracing setup | Shared infrastructure |
| `signals.rs` | Signal handling | Shared infrastructure |
| `cleanup.rs` | Resource cleanup | Shared infrastructure |
| `feature_arg.rs` | Feature argument resolution | Used by `infer_feature` tool |
| `review.rs` | Review result types | Used by reviewer workflow |
| `list_specs.rs` | `kasmos list` command | Kept as-is, also feeds `list_features` tool |
| `status.rs` | `kasmos status` command | Kept as-is |

### ADAPT (modify for new architecture)
| File | Changes |
|------|---------|
| `config.rs` | TOML-based `KasmosConfig` struct, remove TUI settings, add MCP/agent/session settings |
| `layout.rs` | → `launch/layout.rs`: Generate session KDL (MCP tab + orchestration tab), worker pane KDL |
| `zellij.rs` | Add `new_pane_with_name_and_cwd()`, `close_pane_by_name()`, `dump_pane()` wrappers |
| `prompt.rs` | Generate OpenCode launch commands per role with context file lists |

### UNWIRE (preserve behind `#[cfg(feature = "tui")]`)
| File/Module | Notes |
|-------------|-------|
| `tui/` (entire module) | ratatui TUI dashboard |
| `hub/` (entire module) | Hub TUI navigator |
| `tui_cmd.rs` | TUI launcher |
| `report.rs` | TUI-specific reporting |
| `ratatui`, `crossterm`, `futures-util` deps | Move to `[features]` section in Cargo.toml |

### REPLACE (functionality moves to serve/ + manager agent)
| File | Replacement |
|------|-------------|
| `engine.rs` | `serve/tools/workflow_status.rs` + manager agent orchestration |
| `session.rs` | `launch/session.rs` |
| `detector.rs` | `serve/tools/wait_for_event.rs` |
| `cmd.rs` | `serve/` MCP tools |
| `commands.rs` | `serve/` MCP tools |
| `command_handlers.rs` | `serve/` MCP tools |
| `health.rs` | `serve/tools/workflow_status.rs` |
| `shutdown.rs` | Simplified into `cleanup.rs` |
| `review_coordinator.rs` | Manager agent logic |
| `start.rs` | `launch/mod.rs` |
| `sendmsg.rs` | `serve/messages.rs` |
| `attach.rs` | Removed (use `zellij attach kasmos`) |
| `stop.rs` | Removed (use `zellij kill-session kasmos`) |

## CLI Design

```
kasmos                        # Launch session (no spec = feature selector)
kasmos <spec-prefix>          # Launch session bound to feature
kasmos serve                  # MCP server (stdio, spawned by OpenCode)
kasmos setup                  # Validate environment, generate configs
kasmos list                   # List unfinished feature specs
kasmos status [feature]       # Show WP progress for a feature
```

### `kasmos [spec-prefix]` Flow
1. Load `kasmos.toml` config
2. Detect if inside Zellij session (`$ZELLIJ_SESSION_NAME`)
3. Resolve feature: arg > branch > selector
4. Generate session layout KDL (MCP tab + orchestration tab)
5. If outside Zellij: `zellij --session kasmos --layout <path>`
6. If inside Zellij: `zellij action new-tab --layout <mcp.kdl>` + `zellij action new-tab --layout <orch.kdl>`
7. Exit (fire-and-forget)

### `kasmos serve` Flow
1. Initialize MCP server via rmcp stdio transport
2. Register 9 tool handlers
3. Initialize in-memory worker registry
4. Enter stdio read loop (JSON-RPC 2.0)
5. Handle tool calls, return results
6. Dashboard updates happen as side effects of `wait_for_event` poll cycles

## OpenCode Agent Profiles

### Manager (`config/profiles/kasmos/agent/manager.md`)
- **Model**: claude-opus-4-6, temperature 0.3, reasoning high
- **MCP servers**: kasmos (orchestration), zellij (pane management), exa (search), context7 (docs)
- **Context**: Full spec, plan, task board, architecture memory, project structure
- **Instructions**: Workflow assessment, delegation, monitoring, transitions, status reporting. Always confirm before phase transitions. Use `wait_for_event` to stay in loop.

### Planner (`config/profiles/kasmos/agent/planner.md`)
- **Model**: claude-opus-4-6, temperature 0.3, reasoning high
- **MCP servers**: zellij (msg-log write only)
- **Context**: Spec, existing plan artifacts, constitution, architecture memory
- **Instructions**: Run spec-kitty planning lifecycle (specify, clarify, plan, analyze, tasks). Write completion marker to msg-log when done. Interactive — user communicates directly.

### Coder (`config/profiles/kasmos/agent/coder.md`)
- **Model**: claude-sonnet-4-20250514 (or configurable), temperature 0.5
- **MCP servers**: zellij (msg-log write only)
- **Context**: WP task file (contract), coding standards, scoped architecture memory
- **Instructions**: Implement assigned WP in worktree. Write structured progress/completion messages to msg-log. Do not access other WPs or full spec.

### Reviewer (`config/profiles/kasmos/agent/reviewer.md`)
- **Model**: claude-opus-4-6, temperature 0.2, reasoning high
- **MCP servers**: zellij (msg-log write only)
- **Context**: WP task file, coder's changes (diff), acceptance criteria, standards
- **Instructions**: Review changes against WP criteria. Write REVIEW_PASS or REVIEW_REJECT with feedback to msg-log.

### Release (`config/profiles/kasmos/agent/release.md`)
- **Model**: claude-opus-4-6, temperature 0.3
- **MCP servers**: zellij (msg-log write only)
- **Context**: All WP statuses, branch structure, merge target, commit conventions
- **Instructions**: Run acceptance tests, merge feature branch, handle cleanup. Write completion/error to msg-log.

## Key Risks and Mitigations

| Risk | Severity | Mitigation |
|------|----------|-----------|
| Focus-cycling reliability (zellij-pane-tracker) | HIGH | Mutex lock around operations, minimize focus-cycling, serialize MCP calls, direct CLI where possible |
| Multi-instance state coordination | MEDIUM | Only manager gets kasmos MCP, flock on task file writes, worker registry per-instance |
| Message-log parsing reliability | MEDIUM | Distinctive `[KASMOS:*:*]` prefix, tolerant parser, JSONL fallback for persistence |
| Manager token cost | MEDIUM | Structured JSON responses, selective tool calls, OpenCode context management |
| Zellij version compatibility | LOW | Pin 0.43.x+, validate in setup, document requirements |
| wait_for_event indefinite block | MEDIUM | Configurable timeout with explicit timeout return status |

## Dependencies to Add

```toml
# Cargo.toml changes
[dependencies]
rmcp = { version = "0.15", features = ["server", "transport-io"] }
# schemars comes transitively via rmcp

[features]
default = []
tui = ["dep:ratatui", "dep:crossterm", "dep:futures-util"]

# Move from [dependencies] to optional:
ratatui = { version = "0.29", features = ["crossterm"], optional = true }
crossterm = { version = "0.28", features = ["event-stream"], optional = true }
futures-util = { version = "0.3", optional = true }
```

## Quickstart

### Prerequisites
- Rust (latest stable, 2024 edition)
- Zellij 0.43.x+
- OpenCode (latest)
- zellij-pane-tracker plugin + MCP server (forked version with close_pane, rename_pane, list_tabs)

### Build & Install
```bash
cargo install --path crates/kasmos
```

### First-Time Setup
```bash
kasmos setup
# Validates: zellij, opencode, pane-tracker plugin
# Generates: kasmos.toml (if missing), OpenCode profile (if missing)
```

### Launch a Session
```bash
# From project root:
kasmos 011                    # Bind to feature 011
# or
kasmos                        # Feature selector
```

### What Happens
1. `kasmos` generates Zellij layout, launches session, exits
2. Manager agent starts in the orchestration tab
3. Manager auto-spawns `kasmos serve` as its MCP subprocess
4. Manager assesses workflow state and greets user
5. User confirms next phase → manager spawns workers
6. Dashboard shows live progress
7. Manager handles transitions, reviews, retries
8. User confirms release when implementation complete
