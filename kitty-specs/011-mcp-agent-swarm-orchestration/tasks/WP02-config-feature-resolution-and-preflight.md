---
work_package_id: WP02
title: Config, Feature Resolution, and Launch Preflight
lane: "doing"
dependencies: [WP01]
base_branch: 011-mcp-agent-swarm-orchestration-WP01
base_commit: e8f430fb6a020367f628f9d80fbcec56c22b7d6a
created_at: '2026-02-14T20:48:46.329430+00:00'
subtasks:
- T007
- T008
- T009
- T010
- T011
- T012
- T013
phase: Phase 0 - CLI Pivot and Core Foundation
assignee: ''
agent: ''
shell_pid: "3406135"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP02 - Config, Feature Resolution, and Launch Preflight

## Important: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP02 --base WP01
```

---

## Objectives & Success Criteria

Implement the new sectioned config model, deterministic feature resolution pipeline, CLI feature selector fallback, and launch preflight hard-fail checks. After this WP:

1. Config loads with precedence: defaults -> `kasmos.toml` -> env overrides
2. Feature detection pipeline resolves: arg -> branch -> directory -> none
3. When no feature can be inferred, a CLI selector appears BEFORE any Zellij session/tab creation (FR-005)
4. When a required dependency is missing, launch exits non-zero with actionable guidance BEFORE creating any session/tab (FR-021)
5. When no feature specs exist in the repository, CLI reports this and exits cleanly

## Context & Constraints

- **Depends on WP01**: New CLI surface with `spec_prefix` positional arg and module stubs
- **Plan reference**: `kitty-specs/011-mcp-agent-swarm-orchestration/plan.md` - Engineering Alignment decisions 3, 7, 9
- **Research decisions**: Missing deps = hard fail before launch. Feature selection = CLI before session/tab creation.
- **Existing code**: `crates/kasmos/src/config.rs` (386 lines) has the current flat `Config` struct with env/file/validation. `crates/kasmos/src/feature_arg.rs` (88 lines) has `resolve_feature_dir()` for prefix matching.

## Subtasks & Detailed Guidance

### Subtask T007 - Define sectioned config structs

**Purpose**: Replace the flat `Config` struct with a sectioned model that supports the new MCP-focused configuration needs (agent settings, communication, paths, session, audit, lock policies).

**Steps**:
1. Restructure `crates/kasmos/src/config.rs` with nested sections:
   ```rust
   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct Config {
       pub agent: AgentConfig,
       pub communication: CommunicationConfig,
       pub paths: PathsConfig,
       pub session: SessionConfig,
       pub audit: AuditConfig,
       pub lock: LockConfig,
   }

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct AgentConfig {
       pub max_parallel_workers: usize,  // default: 4
       pub opencode_binary: String,      // default: "ocx"
       pub opencode_profile: Option<String>, // default: Some("kas")
       pub review_rejection_cap: u32,    // default: 3 (FR-023)
   }

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct CommunicationConfig {
       pub poll_interval_secs: u64,      // default: 5
       pub event_timeout_secs: u64,      // default: 300
   }

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct PathsConfig {
       pub zellij_binary: String,        // default: "zellij"
       pub spec_kitty_binary: String,    // default: "spec-kitty"
       pub specs_root: String,           // default: "kitty-specs"
   }

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct SessionConfig {
       pub session_name: String,         // default: "kasmos"
       pub manager_width_pct: u32,       // default: 60
       pub message_log_width_pct: u32,   // default: 20
       pub max_workers_per_row: usize,   // default: 4
   }

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct AuditConfig {
       pub metadata_only: bool,          // default: true
       pub debug_full_payload: bool,     // default: false
       pub max_bytes: u64,               // default: 536_870_912 (512MB)
       pub max_age_days: u32,            // default: 14
   }

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct LockConfig {
       pub stale_timeout_minutes: u64,   // default: 15
   }
   ```
2. Provide sensible defaults via `Default` implementations for each section.
3. Keep the old `Config` struct available behind `#[cfg(feature = "tui")]` if needed for TUI compatibility, or migrate the existing fields into the new structure.

**Files**: `crates/kasmos/src/config.rs`
**Validation**: `cargo build` succeeds. Default config validates successfully.

### Subtask T008 - Implement config loading precedence and validation

**Purpose**: Support layered config loading: defaults -> `kasmos.toml` at repo root -> env overrides.

**Steps**:
1. Add a `Config::load()` method that:
   - Starts from `Config::default()`
   - Attempts to load `kasmos.toml` from repo root (or current directory). Use the existing `load_from_file` pattern but adapted for TOML sections.
   - Applies env overrides using `KASMOS_` prefix (e.g., `KASMOS_AGENT_MAX_PARALLEL_WORKERS`, `KASMOS_LOCK_STALE_TIMEOUT_MINUTES`)
   - Calls `validate()` at the end
2. Update validation to check new fields:
   - `max_parallel_workers` in range 1..=16
   - `manager_width_pct` in range 10..=80
   - `stale_timeout_minutes` >= 1
   - `max_age_days` >= 1
   - `review_rejection_cap` >= 1
3. Handle the TOML file being absent gracefully (not an error, just use defaults + env).

**Files**: `crates/kasmos/src/config.rs`
**Validation**: Config loads with partial TOML + env overrides. Invalid values produce clear errors.

### Subtask T009 - Implement feature detection pipeline

**Purpose**: Create a reusable feature detection pipeline that resolves feature slug from multiple sources in priority order: CLI arg -> git branch -> directory name -> none.

**Steps**:
1. Create `crates/kasmos/src/launch/detect.rs` with:
   ```rust
   pub enum FeatureSource {
       Arg(String),
       Branch(String),
       Directory(String),
       None,
   }

   pub struct FeatureDetection {
       pub source: FeatureSource,
       pub feature_slug: Option<String>,
       pub feature_dir: Option<PathBuf>,
   }

   pub fn detect_feature(
       spec_prefix: Option<&str>,
       specs_root: &Path,
   ) -> Result<FeatureDetection> { ... }
   ```
2. Detection logic:
   - **Arg**: If `spec_prefix` is provided, use existing `feature_arg::resolve_feature_dir()` logic
   - **Branch**: Run `git branch --show-current`, parse prefix pattern (e.g., `011-mcp-agent-swarm-orchestration` -> `011`), match against `kitty-specs/011-*` directories
   - **Directory**: Check if current working directory is inside a feature spec directory
   - **None**: No feature could be inferred
3. Integrate the existing `feature_arg.rs` logic rather than duplicating it. The new detect module should call into the existing resolver for the arg path.

**Files**: `crates/kasmos/src/launch/detect.rs`, referencing `crates/kasmos/src/feature_arg.rs`
**Validation**: Detection returns correct source for each scenario.

### Subtask T010 - Implement CLI feature selector for no-inference case

**Purpose**: When no feature can be inferred (FR-005), present an interactive selector in the CLI BEFORE any Zellij actions.

**Steps**:
1. In the launch flow (within `crates/kasmos/src/launch/mod.rs`), after detection returns `FeatureSource::None`:
   - Scan `kitty-specs/` for available feature directories
   - Display a numbered list to stdout
   - Read user selection from stdin
   - Resolve the selected feature
2. Keep this simple - no TUI library needed. Use basic terminal I/O:
   ```
   No feature specified and none could be inferred from the environment.
   Available feature specs:
     1) 010-hub-tui-navigator
     2) 011-mcp-agent-swarm-orchestration
   Select a feature [1-2]:
   ```
3. **Critical**: This MUST happen before any `zellij` commands are executed. The selection gate is in the launch entry function, before layout generation or session creation.

**Files**: `crates/kasmos/src/launch/mod.rs`
**Validation**: Running `kasmos` on `main` with no inferable feature shows the selector.

### Subtask T011 - Implement launch dependency preflight checks

**Purpose**: Validate that all required runtime dependencies are available before launching (FR-021). Fail hard with actionable guidance.

**Steps**:
1. Create a preflight check function in `crates/kasmos/src/launch/mod.rs` (or a dedicated submodule):
   ```rust
   pub fn preflight_checks(config: &Config) -> Result<(), Vec<PreflightFailure>> {
       let mut failures = Vec::new();
       // Check each dependency
       check_binary(&config.paths.zellij_binary, "zellij", &mut failures);
       check_binary(&config.agent.opencode_binary, "opencode", &mut failures);
       check_binary(&config.paths.spec_kitty_binary, "spec-kitty", &mut failures);
       // Check pane-tracker plugin availability
       check_pane_tracker(&mut failures);
       if failures.is_empty() { Ok(()) } else { Err(failures) }
   }
   ```
2. Each check uses `which::which()` (already a dependency) to verify binary existence.
3. Each failure includes:
   - Which dependency is missing
   - What it's needed for
   - Installation guidance (e.g., "Install zellij: cargo install zellij")
4. Print all failures (not just the first) so the user can fix everything at once.
5. Exit with non-zero code. No session/tab must be created.

**Files**: `crates/kasmos/src/launch/mod.rs`
**Validation**: Remove a binary from PATH, run `kasmos 011`, verify non-zero exit and guidance message.

### Subtask T012 - Implement "no specs found" early-exit path

**Purpose**: When no feature specs exist in the repository, exit cleanly before creating any session or tab.

**Steps**:
1. In the launch flow, after config loading but before feature detection:
   - Check if `kitty-specs/` exists and contains at least one feature directory
   - If empty or missing: print a message like "No feature specs found in kitty-specs/. Create one with: spec-kitty init" and exit with code 0 (not an error, just nothing to do)
2. This check runs before the feature detection pipeline.

**Files**: `crates/kasmos/src/launch/mod.rs`
**Validation**: Remove all spec directories, run `kasmos`, verify clean exit message.

### Subtask T013 - Add tests for config, detection, and preflight

**Purpose**: Unit test coverage for config loading/validation, feature detection scenarios, selector gate timing, and preflight hard-fail behavior.

**Steps**:
1. Config tests in `crates/kasmos/src/config.rs`:
   - Default config validates
   - Partial TOML loads correctly (missing sections use defaults)
   - Invalid values produce clear errors
   - Env override precedence works
2. Detection tests in `crates/kasmos/src/launch/detect.rs`:
   - Arg detection resolves correctly
   - Branch detection parses prefix from branch name
   - None detection when no sources available
   - Ambiguous prefix handling (multiple matches)
3. Preflight tests:
   - All dependencies present -> success
   - Missing binary -> failure with guidance
   - Multiple missing -> all reported

**Files**: Test modules within the respective source files
**Validation**: `cargo test` passes with new tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Selector accidentally launching Zellij before user picks | Enforce gate in launch entry function - preflight and selection MUST complete before any zellij commands |
| Brittle dependency checks across Linux/macOS | Centralize check logic using `which` crate. Test with PATH manipulation. |
| Config migration breaking existing users | Old `kasmos.toml` format (flat) should either be auto-migrated or clearly reported as incompatible |

## Review Guidance

- Verify selector runs BEFORE any Zellij session/tab creation
- Verify preflight exits non-zero on missing deps with ALL failures reported
- Verify config loads with partial TOML + env overrides correctly
- Verify feature detection handles all four sources (arg, branch, dir, none)
- Verify "no specs" path exits before any Zellij actions
- Verify all new public functions have doc comments

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
