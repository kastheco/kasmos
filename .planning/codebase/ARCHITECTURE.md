# Architecture

**Analysis Date:** 2026-02-16

## Pattern Overview

**Overall:** MCP-first CLI orchestrator with event-driven tool flows

kasmos is a single Rust binary (one workspace crate) that orchestrates AI coding agents across Zellij terminal panes. It has two primary runtime modes:

1. **Launch mode** (`kasmos [PREFIX]`) - Resolves a feature, generates a KDL layout, and bootstraps a Zellij session with a manager agent pane, message-log pane, and worker area.
2. **Serve mode** (`kasmos serve`) - Runs as an MCP stdio server, spawned by the manager agent, exposing tools for worker lifecycle and workflow status.

**Key Characteristics:**
- MCP server is the primary control plane -- the manager agent calls MCP tools to spawn/despawn workers, read messages, and transition WP states
- Wave-based execution model: work packages are grouped into dependency waves, launched in parallel within each wave
- File-based communication: agents write structured `[KASMOS:event:sender]` messages to a shared message-log pane; task-file frontmatter is the source of truth for lane transitions

## Layers

**CLI Layer:**
- Purpose: Parse user commands, dispatch to appropriate subsystem
- Location: `crates/kasmos/src/main.rs`
- Contains: Clap-derived `Cli` struct, `Commands` enum, tokio main entrypoint
- Depends on: lib.rs re-exports, `launch`, `serve`, `setup`, `list_specs`, `status`
- Used by: User at terminal

**MCP Server Layer:**
- Purpose: Expose orchestration tools to the manager agent via Model Context Protocol
- Location: `crates/kasmos/src/serve/`
- Contains: `KasmosServer` struct implementing `ServerHandler`, tool handlers, worker registry, message parser, audit writer, feature lock manager
- Depends on: `rmcp` crate, `config`, `parser`, `feature_arg`, `serve::registry`, `serve::messages`, `serve::lock`, `serve::audit`
- Used by: Manager agent (via stdio MCP transport)

**Launch Layer:**
- Purpose: Feature resolution, preflight validation, layout generation, Zellij session bootstrap
- Location: `crates/kasmos/src/launch/`
- Contains: Feature detection (`detect.rs`), KDL layout generation (`layout.rs`), Zellij session/tab creation (`session.rs`)
- Depends on: `config`, `setup`, `prompt`, `serve::lock`
- Used by: CLI layer (bare `kasmos` or `kasmos PREFIX`)

**State Machine Layer:**
- Purpose: Enforce valid state transitions for work packages and orchestration runs
- Location: `crates/kasmos/src/types.rs`
- Contains: `WPState` and `RunState` enums with `can_transition_to()` and `transition()` methods
- Depends on: `error`
- Used by: `graph`, `serve::tools::transition_wp`, `serve::tools/workflow_status`

**Zellij Integration Layer:**
- Purpose: Create or attach sessions/tabs and run pane lifecycle actions needed by launch and MCP tools
- Location: `crates/kasmos/src/launch/session.rs`, `crates/kasmos/src/serve/messages.rs`
- Contains: session bootstrap helpers, pane command wrappers, fallback scrollback support
- Depends on: `config`, `error`
- Used by: `launch::run`, `serve::tools::despawn_worker`, `serve::tools::read_messages`

**Configuration Layer:**
- Purpose: Load and validate runtime configuration with multi-source precedence
- Location: `crates/kasmos/src/config.rs`
- Contains: `Config` struct with sectioned TOML, env var overrides, validation
- Precedence: defaults -> `kasmos.toml` (discovered by walking up from cwd) -> `KASMOS_*` env vars
- Used by: All subsystems

**Task-State Layer:**
- Purpose: Parse and mutate work package state stored in task-file frontmatter
- Location: `crates/kasmos/src/parser.rs`, `crates/kasmos/src/serve/tools/transition_wp.rs`, `crates/kasmos/src/serve/tools/workflow_status.rs`
- Contains: YAML frontmatter parsing, lane transition validation, workflow snapshot derivation
- Used by: MCP tool handlers and status reporting

## Data Flow

**Launch Flow:**

1. User runs `kasmos 011`
2. `main.rs` parses args, calls `kasmos::launch::run(Some("011"))`
3. `launch::detect` resolves prefix "011" to a feature slug and directory via `kitty-specs/`
4. `launch::preflight_checks()` validates zellij, opencode, spec-kitty binaries via `setup::validate_environment()`
5. `serve::lock::FeatureLockManager` acquires exclusive feature lock (`.kasmos/locks/{slug}.lock`)
6. `prompt::RolePromptBuilder` constructs the manager agent's initial prompt
7. `launch::layout::generate_layout()` produces KDL layout string with manager pane, msg-log pane, dashboard pane, worker area
8. `launch::session::bootstrap()` writes layout to temp file, creates/attaches Zellij session

**MCP Serve Flow:**

1. Manager agent spawns `kasmos serve` as MCP subprocess (stdio transport)
2. `KasmosServer::new()` loads config, infers feature slug, initializes audit writer
3. Manager agent calls MCP tools:
   - `workflow_status` - reads task file frontmatter, lock state, wave status
   - `spawn_worker` - registers worker in `WorkerRegistry`, builds prompt, writes to audit log
   - `read_messages` - scrapes message-log pane via `zellij action dump-pane`, parses `[KASMOS:event:sender]` format
   - `wait_for_event` - polls message-log pane until matching event appears or timeout
   - `transition_wp` - validates and applies lane transitions in task file frontmatter
   - `list_workers` / `despawn_worker` / `list_features` / `infer_feature`
4. All tool invocations are audit-logged to `kitty-specs/{feature}/.kasmos/messages.jsonl`

**Message Protocol:**

Workers and the manager communicate through a shared `msg-log` Zellij pane using structured messages:
```
[KASMOS:EVENT_TYPE:SENDER_ID] optional payload text
```
Event types: `STARTED`, `PROGRESS`, `DONE`, `ERROR`, `REVIEW_PASS`, `REVIEW_REJECT`, `NEEDS_INPUT`, `SPAWN`, `DESPAWN`

**State Management:**
- In-memory: `WorkerRegistry` + message cursor in `KasmosServer`
- Persisted: task-file frontmatter lane state in `kitty-specs/{feature}/tasks/WP*.md`
- Feature locks: file-based advisory locks at `.kasmos/locks/{slug}.lock` with heartbeat
- Audit log: JSONL entries in `kitty-specs/{feature}/.kasmos/messages.jsonl`

## Key Abstractions

**OrchestrationRun:**
- Purpose: Root aggregate for an entire orchestration session
- Location: `crates/kasmos/src/types.rs`
- Contains: `work_packages: Vec<WorkPackage>`, `waves: Vec<Wave>`, `state: RunState`, `config: Config`
- Pattern: Shared via `Arc<RwLock<OrchestrationRun>>`

**WorkPackage:**
- Purpose: Unit of work assigned to an agent pane
- Location: `crates/kasmos/src/types.rs`
- Fields: `id`, `title`, `state: WPState`, `dependencies`, `wave`, `pane_id`, `worktree_path`, `prompt_path`, `failure_count`
- State machine: `Pending -> Active -> ForReview -> Completed` (with Failed/Paused branches)

**DependencyGraph:**
- Purpose: Track forward/reverse work package dependencies, compute waves
- Location: `crates/kasmos/src/graph.rs`
- Methods: `deps_satisfied()`, `get_dependents()`, `topological_sort()`, `compute_waves()`
- Algorithm: Kahn's algorithm for topological sort, BFS for wave computation

**KasmosServer:**
- Purpose: MCP server exposing orchestration tools
- Location: `crates/kasmos/src/serve/mod.rs`
- Pattern: `rmcp` `ServerHandler` + `ToolRouter` with `#[tool]` attribute macros
- 9 registered tools matching the CLI contract

**RolePromptBuilder:**
- Purpose: Construct role-specific prompts with context boundaries
- Location: `crates/kasmos/src/prompt.rs`
- Roles: Manager, Planner, Coder, Reviewer, Release
- Each role has a `ContextBoundary` defining which project artifacts it can access
- Agent templates loaded from `config/profiles/kasmos/agent/`
- Spec-kitty slash commands loaded from `config/profiles/kasmos/commands/`

## Entry Points

**Binary Entrypoint:**
- Location: `crates/kasmos/src/main.rs`
- Triggers: `cargo run -p kasmos -- <args>` or installed `kasmos` binary
- Responsibilities: Parse CLI, init logging, dispatch to `launch::run()`, `serve::run()`, `setup::run()`, `list_specs::run()`, or `status::run()`

**MCP Server Entry:**
- Location: `crates/kasmos/src/serve/mod.rs::run()`
- Triggers: `kasmos serve` (spawned by manager agent as MCP subprocess)
- Responsibilities: Load config, create `KasmosServer`, bind to stdio transport, serve until quit

**Library Entry:**
- Location: `crates/kasmos/src/lib.rs`
- Re-exports all public types and functions for use by `main.rs` and tests

## Error Handling

**Strategy:** Hierarchical error types with `thiserror` + `anyhow` for context

**Error Hierarchy:**
- `KasmosError` (top-level enum in `crates/kasmos/src/error.rs`)
  - `Config(ConfigError)` - missing files, invalid values, parse failures
  - `Zellij(ZellijError)` - binary not found, session exists/missing, pane operations
  - `SpecParser(SpecParserError)` - missing feature dirs, invalid frontmatter, circular deps
  - `State(StateError)` - invalid transitions, corrupted/stale state files
  - `Layout(LayoutError)` - KDL generation/validation failures
  - `Io(std::io::Error)` - transparent I/O errors
  - `Other(anyhow::Error)` - catch-all

**Patterns:**
- All public functions return `crate::error::Result<T>` (alias for `std::result::Result<T, KasmosError>`)
- MCP tool handlers convert domain errors to `rmcp::model::ErrorData` with `internal_error()` or `map_transition_error()`
- Launch flow uses `anyhow::Result` with `.context()` for user-facing error messages
- Guard clauses with early returns are used consistently throughout the codebase

## Cross-Cutting Concerns

**Logging:** `tracing` + `tracing-subscriber` with env-filter. Initialized via `kasmos::init_logging()` in `crates/kasmos/src/logging.rs`. Log levels: `info` for state transitions, `debug` for internal operations, `warn` for recoverable errors, `error` for failures.

**Validation:** Multi-layer validation:
- Config validation in `Config::validate()` with range checks
- Role and status validation in MCP tool inputs (`spawn_worker`, `list_workers`, `transition_wp`)
- State machine transition validation in `types.rs`
- YAML frontmatter validation in `parser.rs`

**Authentication:** Not applicable -- kasmos is a local orchestrator. External tool auth (OpenCode API keys, etc.) is handled by the respective tools' config.

**Persistence:** Task state is persisted in spec-kitty task frontmatter. Feature locks use file-based advisory locking with heartbeat.

**Audit:** JSONL audit log per feature at `kitty-specs/{feature}/.kasmos/messages.jsonl`. Configurable metadata-only vs full-payload mode. Automatic rotation by size and age.

---

*Architecture analysis: 2026-02-16*
