---
work_package_id: WP01
title: Core Types & Configuration
lane: "doing"
dependencies: []
base_branch: master
base_commit: 892e081e987699b1a8d388714c21e4726c9ac3c2
created_at: '2026-02-09T02:21:35.014413+00:00'
subtasks: [T001, T002, T003, T004, T005]
phase: Phase 1 - Foundation
assignee: ''
agent: "opencode"
shell_pid: "3190789"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-09T02:30:00Z'
  lane: for_review
  agent: opencode
  shell_pid: ''
  action: "Ready for review: implemented WP01 foundation modules with tests and passing cargo build/test"
---

# Work Package Prompt: WP01 – Core Types & Configuration

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This is a root work package with no dependencies. Branch from `master` directly.

**Implementation command**:
```bash
spec-kitty implement WP01
```

## Objectives & Success Criteria

**Objective**: Define the core data model, configuration system, error types, state machine, and logging infrastructure that form the foundation for all other kasmos work packages.

**Success Criteria**:
1. All core types (OrchestrationRun, WorkPackage, Wave, WPState, Config) are defined with serde Serialize/Deserialize
2. Config loads from CLI args → environment variables → optional TOML file (layered precedence)
3. All error types are defined with thiserror and provide actionable context messages
4. WPState machine enforces valid transitions and rejects invalid ones with clear errors
5. Tracing is configured with RUST_LOG support and structured logging
6. `cargo build` succeeds with no warnings
7. Unit tests pass for state machine transitions (valid + invalid)

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Edition**: Rust 2024 (set in workspace Cargo.toml)
- **Dependencies to add**: serde, serde_json, serde_yaml, thiserror, anyhow, tracing, tracing-subscriber, clap, toml
- **Reference**: [spec.md](../spec.md) for entity definitions, [plan.md](../plan.md) for architecture decisions
- **Constraint**: All types must implement Clone and Debug. Enums should be `#[non_exhaustive]` for future extensibility.
- **Constraint**: Config struct must support both programmatic construction (for tests) and file/env loading (for production).

## Subtasks & Detailed Guidance

### Subtask T001 – Define Orchestration Data Model [P]

**Purpose**: Create the core types that represent the orchestration state. These types are used by every other work package.

**Steps**:

1. Create module `crates/kasmos/src/types.rs`:
   - `OrchestrationRun` struct:
     ```rust
     pub struct OrchestrationRun {
         pub id: String,                    // Unique run ID (UUID or timestamp-based)
         pub feature: String,               // Feature name (e.g., "001-zellij-agent-orchestrator")
         pub feature_dir: PathBuf,          // Absolute path to feature directory
         pub config: Config,                // Runtime configuration
         pub work_packages: Vec<WorkPackage>,
         pub waves: Vec<Wave>,
         pub state: RunState,               // Overall run state
         pub started_at: Option<DateTime>,  // Using chrono or std::time::SystemTime
         pub completed_at: Option<DateTime>,
         pub mode: ProgressionMode,         // Continuous or WaveGated
     }
     ```
   - `WorkPackage` struct:
     ```rust
     pub struct WorkPackage {
         pub id: String,                    // "WP01", "WP02", etc.
         pub title: String,
         pub state: WPState,
         pub dependencies: Vec<String>,     // IDs of upstream WPs
         pub wave: usize,                   // Wave index (0-based)
         pub pane_id: Option<u32>,          // Zellij pane ID (set at runtime)
         pub pane_name: String,             // KDL pane name attribute
         pub worktree_path: Option<PathBuf>,
         pub prompt_path: Option<PathBuf>,
         pub started_at: Option<DateTime>,
         pub completed_at: Option<DateTime>,
         pub completion_method: Option<CompletionMethod>,
         pub failure_count: u32,
     }
     ```
   - `Wave` struct:
     ```rust
     pub struct Wave {
         pub index: usize,
         pub wp_ids: Vec<String>,
         pub state: WaveState,
     }
     ```
   - Enums:
     ```rust
     #[non_exhaustive]
     pub enum WPState {
         Pending,
         Active,
         Completed,
         Failed,
         Paused,
     }

     #[non_exhaustive]
     pub enum RunState {
         Initializing,
         Running,
         Paused,       // Wave-gated pause
         Completed,
         Failed,
         Aborted,
     }

     pub enum WaveState {
         Pending,
         Active,
         Completed,
         PartiallyFailed,
     }

     pub enum ProgressionMode {
         Continuous,
         WaveGated,
     }

     pub enum CompletionMethod {
         AutoDetected,  // spec-kitty lane transition
         GitActivity,
         FileMarker,
         Manual,        // Operator command
     }
     ```

2. All types must derive: `Debug, Clone, Serialize, Deserialize`
3. Add `#[serde(rename_all = "snake_case")]` to enums
4. Export from `crates/kasmos/src/lib.rs` via `pub mod types;`

**Files**:
- `crates/kasmos/src/types.rs` (new, ~150 lines)
- `crates/kasmos/src/lib.rs` (new, module declarations)

**Parallel**: Yes — this subtask is independent of T002-T005.

### Subtask T002 – Config Loading from CLI Args + Env + TOML

**Purpose**: Implement layered configuration that supports CLI arguments (highest priority), environment variables, and an optional TOML config file (lowest priority).

**Steps**:

1. Create `crates/kasmos/src/config.rs`:
   - Define `Config` struct:
     ```rust
     pub struct Config {
         pub max_agent_panes: usize,       // Default: 8
         pub progression_mode: ProgressionMode, // Default: Continuous
         pub zellij_binary: String,        // Default: "zellij"
         pub opencode_binary: String,      // Default: "opencode"
         pub spec_kitty_binary: String,    // Default: "spec-kitty"
         pub kasmos_dir: String,           // Default: ".kasmos"
         pub poll_interval_secs: u64,      // Default: 5 (for crash detection)
         pub debounce_ms: u64,             // Default: 200 (for completion detection)
         pub controller_width_pct: u32,    // Default: 40
     }
     ```
   - Implement `Config::load()`:
     - Parse CLI args via clap (will be wired in WP11, but struct defined here)
     - Check environment variables: `KASMOS_MAX_PANES`, `KASMOS_MODE`, etc.
     - Look for `.kasmos/config.toml` in project root
     - Layer: CLI > env > TOML > defaults
   - Implement `Config::default()` with sensible defaults
   - Implement `Config::validate()` — e.g., max_agent_panes must be 1-16

2. Add `toml` crate to dependencies

**Files**:
- `crates/kasmos/src/config.rs` (new, ~120 lines)

**Parallel**: No — depends on T001 for ProgressionMode enum.

### Subtask T003 – Error Types via thiserror

**Purpose**: Define a comprehensive error type hierarchy covering all kasmos subsystems, enabling rich error messages with context.

**Steps**:

1. Create `crates/kasmos/src/error.rs`:
   ```rust
   use thiserror::Error;

   #[derive(Error, Debug)]
   pub enum KasmosError {
       #[error("Configuration error: {0}")]
       Config(#[from] ConfigError),

       #[error("Zellij error: {0}")]
       Zellij(#[from] ZellijError),

       #[error("Spec parser error: {0}")]
       SpecParser(#[from] SpecParserError),

       #[error("State error: {0}")]
       State(#[from] StateError),

       #[error("Pane error: {0}")]
       Pane(#[from] PaneError),

       #[error("Wave engine error: {0}")]
       Wave(#[from] WaveError),

       #[error(transparent)]
       Io(#[from] std::io::Error),

       #[error(transparent)]
       Other(#[from] anyhow::Error),
   }

   #[derive(Error, Debug)]
   pub enum ConfigError {
       #[error("Config file not found: {path}")]
       NotFound { path: String },
       #[error("Invalid config value: {field} = {value} ({reason})")]
       InvalidValue { field: String, value: String, reason: String },
       #[error("Failed to parse config: {0}")]
       Parse(String),
   }

   #[derive(Error, Debug)]
   pub enum ZellijError {
       #[error("Zellij binary not found in PATH")]
       NotFound,
       #[error("Session '{name}' already exists")]
       SessionExists { name: String },
       #[error("Session '{name}' not found")]
       SessionNotFound { name: String },
       #[error("Failed to create session: {0}")]
       CreateFailed(String),
       #[error("Pane operation failed: {0}")]
       PaneOperation(String),
   }

   #[derive(Error, Debug)]
   pub enum SpecParserError {
       #[error("Feature directory not found: {path}")]
       FeatureDirNotFound { path: String },
       #[error("Invalid YAML frontmatter in {file}: {reason}")]
       InvalidFrontmatter { file: String, reason: String },
       #[error("Circular dependency detected: {cycle}")]
       CircularDependency { cycle: String },
       #[error("Unknown dependency '{dep}' referenced by '{wp}'")]
       UnknownDependency { dep: String, wp: String },
   }

   #[derive(Error, Debug)]
   pub enum StateError {
       #[error("Invalid state transition: {from:?} -> {to:?} for WP {wp_id}")]
       InvalidTransition { wp_id: String, from: WPState, to: WPState },
       #[error("State file corrupted: {0}")]
       Corrupted(String),
       #[error("Stale state detected: last updated {last_updated}")]
       Stale { last_updated: String },
   }

   #[derive(Error, Debug)]
   pub enum PaneError {
       #[error("Pane not found for WP {wp_id}")]
       NotFound { wp_id: String },
       #[error("Pane {pane_id} crashed for WP {wp_id}")]
       Crashed { pane_id: u32, wp_id: String },
       #[error("Prompt injection failed for WP {wp_id}: {reason}")]
       PromptInjectionFailed { wp_id: String, reason: String },
   }

   #[derive(Error, Debug)]
   pub enum WaveError {
       #[error("Wave {wave} has no eligible work packages")]
       NoEligible { wave: usize },
       #[error("Capacity limit reached: {active}/{max} panes")]
       CapacityExceeded { active: usize, max: usize },
       #[error("Wave progression blocked: WP {blocker} failed")]
       Blocked { blocker: String },
   }
   ```

2. Implement `type Result<T> = std::result::Result<T, KasmosError>;`

**Files**:
- `crates/kasmos/src/error.rs` (new, ~120 lines)

**Parallel**: No — uses WPState from T001.

### Subtask T004 – WPState Machine Transitions with Guard Clauses

**Purpose**: Implement a state machine that enforces valid WP state transitions and provides clear error messages for invalid ones.

**Steps**:

1. Add to `crates/kasmos/src/types.rs` (or new `crates/kasmos/src/state_machine.rs`):
   ```rust
   impl WPState {
       /// Valid transitions:
       /// Pending → Active (when wave launches)
       /// Active → Completed (on completion detection)
       /// Active → Failed (on crash/error)
       /// Active → Paused (on pause command)
       /// Paused → Active (on resume command)
       /// Failed → Pending (on retry command — reset to be re-launched)
       /// Failed → Active (on restart command — immediate relaunch)
       pub fn can_transition_to(&self, target: &WPState) -> bool { ... }

       pub fn transition(
           &self,
           target: WPState,
           wp_id: &str,
       ) -> Result<WPState, StateError> { ... }
   }
   ```

2. Write unit tests for all valid transitions and verify that invalid transitions (e.g., Completed→Active, Pending→Completed) return `StateError::InvalidTransition`

3. Similarly implement `RunState` transitions:
   - Initializing → Running
   - Running → Paused (wave-gated boundary)
   - Paused → Running (operator confirms)
   - Running → Completed (all WPs done)
   - Running → Failed (unrecoverable error)
   - Running → Aborted (operator abort)

**Files**:
- `crates/kasmos/src/state_machine.rs` (new, ~100 lines)

**Parallel**: No — depends on types from T001.

### Subtask T005 – Logging Setup via Tracing

**Purpose**: Configure the tracing crate for structured logging with RUST_LOG environment variable support.

**Steps**:

1. Create `crates/kasmos/src/logging.rs`:
   ```rust
   use tracing_subscriber::{fmt, EnvFilter};

   pub fn init_logging() -> Result<(), KasmosError> {
       let filter = EnvFilter::try_from_default_env()
           .unwrap_or_else(|_| EnvFilter::new("kasmos=info"));

       fmt()
           .with_env_filter(filter)
           .with_target(true)
           .with_thread_ids(false)
           .with_file(true)
           .with_line_number(true)
           .init();

       Ok(())
   }
   ```

2. Add `tracing` macros usage examples in a doc comment (info!, debug!, warn!, error!, span!)
3. Ensure RUST_LOG=debug shows detailed output, RUST_LOG=info shows operational messages only

**Files**:
- `crates/kasmos/src/logging.rs` (new, ~40 lines)

**Parallel**: No — uses error types from T003.

## Test Strategy

- Unit test WPState transitions: all valid transitions succeed, all invalid transitions return StateError
- Unit test RunState transitions: same pattern
- Unit test Config::default() returns valid config, Config::validate() catches invalid values
- Unit test Config::load() with mock TOML file
- Compile test: `cargo build -p kasmos` succeeds with no warnings

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Data model doesn't accommodate future needs | High | Use `#[non_exhaustive]` on enums, builder pattern for Config |
| State machine too rigid for edge cases | Medium | Include Paused state, design transition table upfront |
| Config layering complexity | Low | Use established pattern: CLI > env > file > defaults |
| Tracing performance overhead | Low | Negligible at expected log volume; filter via RUST_LOG |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] All types compile with serde derive macros
- [ ] State machine rejects invalid transitions with specific error messages
- [ ] Config loads from defaults, TOML, env vars with correct precedence
- [ ] Error types provide actionable context (not just "something failed")
- [ ] `cargo test -p kasmos` passes all unit tests
- [ ] No compiler warnings
- [ ] Types are documented with rustdoc comments

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP01 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
- 2026-02-09T02:21:35Z – opencode – shell_pid=3018616 – lane=doing – Assigned agent via workflow command
- 2026-02-09T02:32:34Z – opencode – shell_pid=3018616 – lane=doing – Started review via workflow command
- 2026-02-09T02:38:12Z – opencode – shell_pid=3018616 – lane=done – Review passed: All 5 subtasks complete with 43 passing tests. Clean public API, excellent error handling, strong philosophy compliance. Two minor issues noted (RunState error placeholder, unsafe env tests) but non-blocking. Ready for dependent WPs.
- 2026-02-09T02:47:12Z – opencode – shell_pid=3250900 – lane=doing – Started implementation via workflow command
- 2026-02-09T02:52:58Z – opencode – shell_pid=3250900 – lane=for_review – Ready for review: Completed core types/config foundation, all tests passing
- 2026-02-09T03:39:36Z – opencode – shell_pid=3190789 – lane=doing – Started review via workflow command
