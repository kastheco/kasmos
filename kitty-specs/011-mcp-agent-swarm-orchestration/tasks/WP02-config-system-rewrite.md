---
work_package_id: "WP02"
title: "Configuration System Rewrite"
phase: "Phase 0: Foundation"
lane: "planned"
dependencies: ["WP01"]
subtasks: ["T008", "T009", "T010", "T011", "T012", "T013", "T014"]
history:
  - date: "2026-02-13"
    agent: "controller"
    action: "Created WP prompt"
---

# WP02: Configuration System Rewrite

## Implementation Command

```bash
spec-kitty implement WP02 --base WP01
```

## Objective

Replace the flat `Config` struct with a sectioned `KasmosConfig` structure (`agent`, `communication`, `paths`, `session`) per the data model specification. Support TOML loading from `kasmos.toml` in the repository root, environment variable overrides, and validation. Update all call sites throughout the codebase.

## Context

The current `config.rs` has a flat `Config` struct with 9 fields oriented toward the old TUI orchestrator (e.g., `controller_width_pct`, `debounce_ms`, `progression_mode`). The MCP swarm architecture needs a different config shape organized around agent settings, communication parameters, path conventions, and session identity.

**Target config file**: `kasmos.toml` in the repository root (not `.kasmos/config.toml`)

**Current config.rs structure** (being replaced):
```rust
pub struct Config {
    pub max_agent_panes: usize,        // 8
    pub progression_mode: ProgressionMode,
    pub zellij_binary: String,         // "zellij"
    pub opencode_binary: String,       // "ocx"  ← MUST change to "opencode"
    pub spec_kitty_binary: String,
    pub kasmos_dir: String,            // ".kasmos"
    pub poll_interval_secs: u64,       // 5
    pub debounce_ms: u64,
    pub controller_width_pct: u32,     // 40
}
```

**Target structure from data-model.md:**
```toml
[agent]
binary = "opencode"
profile = "kasmos"
max_parallel_workers = 4
review_retry_cap = 3

[communication]
poll_interval_seconds = 5
wait_timeout_seconds = 120
message_prefix = "KASMOS"

[paths]
worktree_base = ".worktrees"
audit_dir = ".kasmos"
specs_dir = "kitty-specs"

[session]
name = "kasmos"
```

**Call sites to update**: Any file that references `Config`, `config.max_agent_panes`, `config.opencode_binary`, etc. Run `grep -r "Config" crates/kasmos/src/` to find them all.

## Subtasks

### T008: Define KasmosConfig structs

**Purpose**: Create the new sectioned config types per data-model.md.

**Steps**:
1. In `config.rs`, define:
   ```rust
   use serde::Deserialize;
   use std::path::PathBuf;

   #[derive(Debug, Clone, Deserialize)]
   pub struct KasmosConfig {
       #[serde(default)]
       pub agent: AgentConfig,
       #[serde(default)]
       pub communication: CommunicationConfig,
       #[serde(default)]
       pub paths: PathsConfig,
       #[serde(default)]
       pub session: SessionConfig,
   }

   #[derive(Debug, Clone, Deserialize)]
   pub struct AgentConfig {
       #[serde(default = "default_binary")]
       pub binary: String,
       #[serde(default = "default_profile")]
       pub profile: String,
       #[serde(default = "default_max_workers")]
       pub max_parallel_workers: usize,
       #[serde(default = "default_retry_cap")]
       pub review_retry_cap: usize,
   }

   #[derive(Debug, Clone, Deserialize)]
   pub struct CommunicationConfig {
       #[serde(default = "default_poll_interval")]
       pub poll_interval_seconds: u64,
       #[serde(default = "default_wait_timeout")]
       pub wait_timeout_seconds: u64,
       #[serde(default = "default_prefix")]
       pub message_prefix: String,
   }

   #[derive(Debug, Clone, Deserialize)]
   pub struct PathsConfig {
       #[serde(default = "default_worktree_base")]
       pub worktree_base: PathBuf,
       #[serde(default = "default_audit_dir")]
       pub audit_dir: PathBuf,
       #[serde(default = "default_specs_dir")]
       pub specs_dir: PathBuf,
   }

   #[derive(Debug, Clone, Deserialize)]
   pub struct SessionConfig {
       #[serde(default = "default_session_name")]
       pub name: String,
   }
   ```
2. Implement the `default_*` helper functions for each field
3. Keep the old `Config` struct temporarily (renamed to `LegacyConfig` or behind `#[cfg(feature = "tui")]`) until WP14 cleans it up

### T009: Implement Default for all config structs

**Purpose**: Ensure `KasmosConfig::default()` returns a usable configuration.

**Steps**:
1. Implement `Default` for each struct matching the data-model defaults:
   - `AgentConfig`: binary="opencode", profile="kasmos", max_parallel_workers=4, review_retry_cap=3
   - `CommunicationConfig`: poll_interval_seconds=5, wait_timeout_seconds=120, message_prefix="KASMOS"
   - `PathsConfig`: worktree_base=".worktrees", audit_dir=".kasmos", specs_dir="kitty-specs"
   - `SessionConfig`: name="kasmos"
2. **CRITICAL**: `binary` defaults to `"opencode"`, NOT `"ocx"`. This is the carry-forward decision from planning.

### T010: Implement TOML file loading

**Purpose**: Load configuration from `kasmos.toml` in the repository root.

**Steps**:
1. Implement `KasmosConfig::load()` that:
   - Detects repo root via `git rev-parse --show-toplevel` or walks up from CWD looking for `.git/`
   - Looks for `kasmos.toml` in repo root
   - If found: deserialize via `toml::from_str::<KasmosConfig>(content)?`
   - If not found: return `KasmosConfig::default()`
2. Because all fields have `#[serde(default)]`, a partial TOML file works (e.g., only setting `[agent]\nbinary = "custom-agent"`)
3. Return a descriptive error if TOML parsing fails

### T011: Implement environment variable override layer

**Purpose**: Allow runtime overrides via environment variables.

**Steps**:
1. Implement `KasmosConfig::apply_env_overrides(&mut self)`:
   ```
   KASMOS_AGENT_BINARY      → agent.binary
   KASMOS_PROFILE           → agent.profile
   KASMOS_MAX_WORKERS       → agent.max_parallel_workers
   KASMOS_REVIEW_RETRY_CAP  → agent.review_retry_cap
   KASMOS_POLL_INTERVAL     → communication.poll_interval_seconds
   KASMOS_WAIT_TIMEOUT      → communication.wait_timeout_seconds
   KASMOS_MESSAGE_PREFIX    → communication.message_prefix
   KASMOS_WORKTREE_BASE     → paths.worktree_base
   KASMOS_AUDIT_DIR         → paths.audit_dir
   KASMOS_SPECS_DIR         → paths.specs_dir
   KASMOS_SESSION_NAME      → session.name
   ```
2. Parse numeric values with descriptive error messages
3. Only override if the env var is set (preserve TOML/default values otherwise)

### T012: Implement validation

**Purpose**: Catch configuration errors early.

**Steps**:
1. Implement `KasmosConfig::validate(&self) -> Result<()>`:
   - `max_parallel_workers` must be 1–16
   - `review_retry_cap` must be 1–10
   - `poll_interval_seconds` must be > 0
   - `wait_timeout_seconds` must be > 0 and ≥ poll_interval_seconds
   - `message_prefix` must be non-empty and contain only ASCII alphanumeric + underscore
   - `binary` must be non-empty
   - `profile` must be non-empty
2. Return `KasmosError` with descriptive messages on failure

### T013: Write unit tests

**Purpose**: Verify all config loading paths.

**Steps**:
1. Test `KasmosConfig::default()` returns expected values
2. Test TOML loading with complete and partial files (use `tempfile` crate)
3. Test env var overrides (use test mutex like existing tests)
4. Test validation: valid config passes, each invalid field fails with correct error
5. Test layering: TOML values overridden by env vars
6. Test missing kasmos.toml returns defaults

### T014: Update all call sites

**Purpose**: Replace `Config` references with `KasmosConfig` throughout the codebase.

**Steps**:
1. Search for all `Config::` and `: Config` references
2. Key mappings:
   - `config.opencode_binary` → `config.agent.binary`
   - `config.max_agent_panes` → `config.agent.max_parallel_workers`
   - `config.zellij_binary` → not in new config (hardcode "zellij" or add if needed)
   - `config.spec_kitty_binary` → not in new config (hardcode "spec-kitty")
   - `config.kasmos_dir` → `config.paths.audit_dir`
   - `config.poll_interval_secs` → `config.communication.poll_interval_seconds`
   - `config.debounce_ms` → removed (TUI-only setting)
   - `config.controller_width_pct` → removed (TUI-only setting)
   - `config.progression_mode` → removed (manager agent decides)
3. Update `OrchestrationRun` in `types.rs` if it references old `Config` type
4. Files that will be removed in WP13 (engine.rs, session.rs, etc.) — update minimally or leave with `#[allow(dead_code)]` if they'll be deleted soon

**Edge cases**:
- If `zellij_binary` is needed (some files reference it), add a top-level field or hardcode "zellij"
- The old `ProgressionMode` enum in `types.rs` is used by other KEEP modules — keep it available

## Test Strategy

- `cargo test` passes with all config tests
- `cargo build` succeeds with updated call sites
- Verify `KasmosConfig::default().agent.binary == "opencode"` (not "ocx")

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Breaking call sites in modules slated for deletion | Minimal updates — these files are removed in WP13 |
| Missing env var coverage | Systematic grep for `KASMOS_` in existing code |
| TOML parsing edge cases with nested sections | All section fields have defaults, so partial TOML works |

## Review Guidance

- Verify `binary` default is `"opencode"` not `"ocx"`
- Verify `kasmos.toml` is loaded from repo root, not `.kasmos/`
- Verify all serde defaults match data-model.md values
- Check that validation ranges are sensible
- Verify call site updates are correct (field path mapping)

## Activity Log

| Date | Agent | Event |
|------|-------|-------|
| 2026-02-13 | controller | Created WP prompt |
