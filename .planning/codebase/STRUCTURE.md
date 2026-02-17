# Codebase Structure

**Analysis Date:** 2026-02-16

## Directory Layout

```
kasmos/
├── crates/
│   └── kasmos/                 # Single workspace crate (binary + library)
│       ├── Cargo.toml          # Crate manifest
│       ├── kitty-specs/        # Crate-local spec-kitty specs (for crate tests)
│       └── src/                # All source code
├── config/
│   └── profiles/
│       └── kasmos/             # Agent prompt templates and OpenCode config
│           ├── agent/          # Role-specific prompt template files
│           └── opencode.jsonc  # OpenCode profile configuration
├── contracts/
│   └── cli-contract.md         # CLI interface contract (versioned API surface)
├── docs/                       # User-facing documentation
│   ├── architecture.md         # High-level architecture overview
│   ├── cheatsheet.md           # Command cheatsheet
│   ├── getting-started.md      # Quickstart guide
│   ├── keybinds.md             # Keybinding reference
│   └── workflow-cheatsheet.md  # Workflow patterns
├── kitty-specs/                # Feature specifications (spec-kitty managed)
│   ├── 001-zellij-agent-orchestrator/
│   ├── 002-ratatui-tui-controller-panel/
│   ├── ...
│   └── 011-mcp-agent-swarm-orchestration/
├── scripts/                    # Shell scripts for manual workflows
│   ├── check-cli-contract.sh   # Verify CLI matches contract
│   ├── review-cycle.sh         # Manual review automation
│   └── sk-start.sh             # Spec-kitty swarm lifecycle launcher
├── .kittify/                   # Spec-kitty project config and memory
│   ├── AGENTS.md               # Agent rules for spec-kitty projects
│   ├── config.yaml             # Spec-kitty configuration
│   ├── memory/                 # Persistent project memory (symlinked into worktrees)
│   │   ├── architecture.md     # Codebase architecture knowledge
│   │   ├── constitution.md     # Project technical standards
│   │   └── workflow-intelligence.md  # Workflow lessons learned
│   ├── metadata.yaml           # Spec-kitty metadata
│   └── missions/               # Mission definitions
├── .planning/                  # GSD planning artifacts
│   └── codebase/               # Codebase analysis documents (this directory)
├── .worktrees/                 # Git worktrees for WP isolation (gitignored)
├── .kasmos/                    # Runtime state directory (gitignored)
│   ├── state.json              # Persisted orchestration state
│   ├── locks/                  # Feature lock files
│   └── audit/                  # Audit log files
├── Cargo.toml                  # Workspace manifest
├── Cargo.lock                  # Dependency lockfile
├── Justfile                    # Just task runner recipes
├── kasmos.toml                 # Runtime configuration
├── AGENTS.md                   # Agent instructions for the project
└── README.md                   # Project overview
```

## Directory Purposes

**`crates/kasmos/src/`:**
- Purpose: All Rust source code for the single kasmos binary/library crate
- Contains: ~30 `.rs` files organized as flat modules + subdirectories for subsystems
- Key principle: `lib.rs` exposes the public API, `main.rs` is the thin CLI entrypoint

**`crates/kasmos/src/serve/`:**
- Purpose: MCP server implementation and tools
- Contains: Server struct, tool implementations, worker registry, message parser, audit, lock, dashboard
- Key files:
  - `mod.rs`: `KasmosServer` struct, `run()` entry, tool router registration
  - `tools/`: One file per MCP tool (9 tools total)
  - `registry.rs`: `WorkerRegistry` and `AgentRole`/`WorkerStatus` enums
  - `messages.rs`: Message protocol parser for `[KASMOS:event:sender]` format
  - `lock.rs`: `FeatureLockManager` for repository-wide feature locking
  - `audit.rs`: `AuditWriter` for JSONL audit logging

**`crates/kasmos/src/launch/`:**
- Purpose: Feature resolution, layout generation, session bootstrap
- Contains: Feature detection from args/branch/cwd, KDL layout builder, Zellij session/tab creation
- Key files:
  - `mod.rs`: `run()` entry, preflight checks, feature selection UI
  - `detect.rs`: Feature slug resolution from prefix, branch name, or cwd
  - `layout.rs`: KDL layout generation with manager/msg-log/dashboard/worker areas
  - `session.rs`: Zellij session/tab creation (adapts to inside-vs-outside Zellij)

**`crates/kasmos/src/setup/`:**
- Purpose: Environment validation and baseline asset generation
- Contains: Binary checks (zellij, opencode, spec-kitty), repo context validation, config file generation
- Key files: `mod.rs`

**`config/profiles/kasmos/`:**
- Purpose: Agent prompt templates and OpenCode configuration
- Contains: Role-specific markdown prompt templates loaded by `RolePromptBuilder`
- Key files: `agent/manager.md`, `agent/coder.md`, `agent/reviewer.md`, `agent/planner.md`, `agent/release.md`, `opencode.jsonc`

**`kitty-specs/`:**
- Purpose: Feature specifications managed by spec-kitty
- Contains: One subdirectory per feature (`###-slug/`), each with `spec.md`, `plan.md`, `tasks.md`, `tasks/WPxx.md`
- Committed to git; worktree agents modify their own copies

**`.kittify/memory/`:**
- Purpose: Persistent project memory shared across worktrees via symlinks
- Contains: Architecture knowledge, project constitution, workflow intelligence
- Important: Symlinked from worktrees back to main repo so all agents share the same memory

## Key File Locations

**Entry Points:**
- `crates/kasmos/src/main.rs`: Binary entrypoint (CLI parsing + dispatch)
- `crates/kasmos/src/lib.rs`: Library re-exports (public API surface)
- `crates/kasmos/src/serve/mod.rs`: MCP server entrypoint (`run()`)
- `crates/kasmos/src/launch/mod.rs`: Launch flow entrypoint (`run()`)

**Configuration:**
- `kasmos.toml`: Runtime configuration (TOML, discovered by walking up from cwd)
- `crates/kasmos/src/config.rs`: Config struct, loading, validation
- `crates/kasmos/Cargo.toml`: Crate manifest and dependency declarations

**Core Logic:**
- `crates/kasmos/src/config.rs`: Runtime config model, loading, and validation
- `crates/kasmos/src/types.rs`: Core enums/data structures (`WPState`, `RunState`, `WorkPackage`, `OrchestrationRun`)
- `crates/kasmos/src/graph.rs`: Dependency graph with topological sort and wave computation
- `crates/kasmos/src/parser.rs`: YAML frontmatter parser for spec-kitty task files
- `crates/kasmos/src/prompt.rs`: Role-based prompt builder with context boundaries
- `crates/kasmos/src/feature_arg.rs`: Feature argument resolution helpers
- `crates/kasmos/src/list_specs.rs`: Feature listing command implementation
- `crates/kasmos/src/status.rs`: Status command implementation
- `crates/kasmos/src/error.rs`: Error type hierarchy
- `crates/kasmos/src/logging.rs`: Tracing subscriber initialization

**MCP Tools:**
- `crates/kasmos/src/serve/tools/spawn_worker.rs`: Spawn a worker pane
- `crates/kasmos/src/serve/tools/despawn_worker.rs`: Close a worker pane
- `crates/kasmos/src/serve/tools/list_workers.rs`: List tracked workers
- `crates/kasmos/src/serve/tools/read_messages.rs`: Read message-log pane events
- `crates/kasmos/src/serve/tools/wait_for_event.rs`: Block until event or timeout
- `crates/kasmos/src/serve/tools/workflow_status.rs`: Return workflow phase/status
- `crates/kasmos/src/serve/tools/transition_wp.rs`: Validate and apply WP lane transitions
- `crates/kasmos/src/serve/tools/list_features.rs`: List available feature specs
- `crates/kasmos/src/serve/tools/infer_feature.rs`: Infer feature from context

**Testing:**
- Tests are co-located with source (inline `#[cfg(test)] mod tests` blocks)
- No separate test directory; all tests in the same `.rs` file as the code they test

**Contracts & Docs:**
- `contracts/cli-contract.md`: CLI interface contract (breaking change boundary)
- `docs/architecture.md`: Architecture overview
- `docs/getting-started.md`: Setup guide

## Naming Conventions

**Files:**
- `snake_case.rs` for all Rust source files
- `mod.rs` for directory-module entrypoints
- Flat modules for most concerns; subdirectories (`serve/`, `launch/`, `setup/`) only for subsystems with multiple files

**Directories:**
- `snake_case` for Rust module directories
- `kebab-case` for non-Rust directories (`kitty-specs/`, `config/profiles/`)
- `###-slug` pattern for feature spec directories (e.g., `011-mcp-agent-swarm-orchestration`)
- `.kasmos/` for runtime state (gitignored)
- `.worktrees/` for git worktrees (gitignored)
- `.planning/` for GSD planning artifacts

**Types:**
- `PascalCase` for types: `WorkPackage`, `FeatureDetection`, `KasmosServer`
- State enums: `WPState`, `RunState`, `WaveState`, `WorkerStatus`
- Config structs: `Config`, `AgentConfig`, `PathsConfig`, etc.
- Error enums: `KasmosError`, `ConfigError`, `ZellijError`, etc.

**Functions:**
- `snake_case` for all functions
- Async functions: `run()`, `bootstrap()`, `emit_audit()`
- Builders: `with_wp_id()`, `with_status()`, `with_details()`
- Validators: `validate()`, `validate_role()`, `can_transition_to()`

**Constants:**
- `SCREAMING_SNAKE_CASE`: `MSG_LOG_PANE`, `KNOWN_EVENTS`, `FEATURE_LOCK_CONFLICT_CODE`

## Where to Add New Code

**New MCP Tool:**
1. Create `crates/kasmos/src/serve/tools/my_tool.rs` with `Input`/`Output` structs + `handle()` function
2. Add `pub mod my_tool;` to `crates/kasmos/src/serve/tools/mod.rs`
3. Add `use` import and `#[tool]` method in `crates/kasmos/src/serve/mod.rs::KasmosServer`
4. Add tool to `contracts/cli-contract.md` if it's a contract surface

**New CLI Subcommand:**
1. Add variant to `Commands` enum in `crates/kasmos/src/main.rs`
2. Create handler module at `crates/kasmos/src/my_command.rs` (or inline if small)
3. Wire dispatch in `main()` match block
4. Add to `contracts/cli-contract.md`

**New Configuration Section:**
1. Add section struct (e.g., `MyConfig`) and file struct (e.g., `MyConfigFile`) in `crates/kasmos/src/config.rs`
2. Add field to `Config` struct and `ConfigFile` struct
3. Add loading logic in `load_from_file()` and `load_from_env()`
4. Add validation in `validate()`
5. Update `kasmos.toml` with defaults

**New Error Type:**
1. Create domain error enum in `crates/kasmos/src/error.rs`
2. Add variant to `KasmosError` with `#[from]` derive
3. Use `thiserror::Error` for the domain enum

**New Launch Step:**
1. Add behavior in `crates/kasmos/src/launch/mod.rs::run()`
2. Implement feature/session helpers under `crates/kasmos/src/launch/`
3. Add tests in the touched launch module

**New Agent Role:**
1. Add variant to `AgentRole` in `crates/kasmos/src/serve/registry.rs`
2. Add prompt template to `config/profiles/kasmos/agent/`
3. Add context boundary in `crates/kasmos/src/prompt.rs::allowed_context()`

## Special Directories

**`.kasmos/` (runtime state):**
- Purpose: Runtime artifacts created during orchestration
- Generated: Yes (by kasmos at runtime)
- Committed: No (gitignored)
- Contains: `state.json`, `locks/`, `audit/`

**`.worktrees/` (git worktrees):**
- Purpose: Isolated git worktrees per work package
- Generated: Yes (by spec-kitty/manager workflow)
- Committed: No (gitignored)
- Pattern: `.worktrees/{feature_slug}-{wp_id}/`
- Note: `.kittify/memory/` inside worktrees is symlinked to main repo

**`target/` (build artifacts):**
- Purpose: Cargo build output
- Generated: Yes (by cargo)
- Committed: No (gitignored)

**`kitty-specs/` (feature specs):**
- Purpose: Feature specifications and task files
- Generated: Partially (by spec-kitty CLI)
- Committed: Yes (tracked in git)
- Pattern: `kitty-specs/{###-slug}/` with `spec.md`, `plan.md`, `tasks/WPxx.md`

**`config/profiles/kasmos/agent/`:**
- Purpose: Role-specific prompt templates loaded at runtime by `RolePromptBuilder`
- Generated: No (manually authored)
- Committed: Yes

**`config/profiles/kasmos/commands/`:**
- Purpose: Spec-kitty slash command definitions installed to `.opencode/commands/` by `kasmos setup`
- Generated: No (manually authored)
- Committed: Yes

---

*Structure analysis: 2026-02-16*
