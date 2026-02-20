# Coding Conventions

**Analysis Date:** 2026-02-16

## Naming Patterns

**Files:**
- Use `snake_case.rs` for all Rust source files: `feature_arg.rs`, `workflow_status.rs`, `list_specs.rs`
- Module directories use `snake_case`: `serve/`, `launch/`, `setup/`
- Sub-module entry points use `mod.rs`: `crates/kasmos/src/serve/mod.rs`, `crates/kasmos/src/launch/mod.rs`
- MCP tool implementations are one file per tool: `crates/kasmos/src/serve/tools/spawn_worker.rs`

**Functions:**
- Use `snake_case` for all functions: `detect_feature()`, `parse_frontmatter()`, `write_temp_layout()`
- Public constructors use `new()` or `default()`: `Config::new()`, `WorkerRegistry::new()`, `KasmosServer::new()`
- Boolean predicates use `is_` or `can_`: `is_inside_zellij()`, `is_known()`, `is_stale()`, `can_transition_to()`
- Builder methods return `Self` with `with_` prefix: `with_wp_id()`, `with_status()`, `with_details()`
- Async handlers use `handle()` as the entry point function name: `tools::spawn_worker::handle()`, `tools::list_features::handle()`
- Internal helper functions are private (no `pub`): `sync_legacy_fields()`, `detect_phase_hint()`

**Variables:**
- Use `snake_case` for all bindings: `feature_slug`, `wp_id`, `max_parallel_workers`
- Work package identifiers are consistently `wp_id: String`: never `id` or `work_package_id` in function params
- Configuration fields use descriptive names: `stale_timeout_minutes`, `max_parallel_workers`, `poll_interval_secs`

**Types:**
- Use `PascalCase` for all types: `KasmosError`, `KasmosServer`, `FeatureDetection`, `WorkerRegistry`
- Enums use `PascalCase` variants: `WPState::Active`, `RunState::Running`, `AgentRole::Coder`
- Error enums end with `Error`: `ConfigError`, `ZellijError`, `SpecParserError`, `StateError`, `LayoutError`
- Input/Output structs for MCP tools follow `{ToolName}Input` / `{ToolName}Output`: `SpawnWorkerInput`, `SpawnWorkerOutput`

**Constants:**
- Use `SCREAMING_SNAKE_CASE`: `FEATURE_LOCK_CONFLICT_CODE`, `STALE_LOCK_CONFIRMATION_REQUIRED_CODE`, `RETENTION_CHECK_EVERY_WRITES`
- Module-level string constants use `const`: `const PROFILE_ROOT: &str = "config/default";`

## Code Style

**Formatting:**
- No `rustfmt.toml` or `.rustfmt.toml` — uses default `rustfmt` settings
- Rust 2024 edition (set in `Cargo.toml` workspace: `edition = "2024"`)
- Uses `let`-chains (`if let ... && let ...`) in Rust 2024 style: `crates/kasmos/src/parser.rs`, `crates/kasmos/src/launch/detect.rs`

**Linting:**
- Clippy with `-D warnings` (treat all warnings as errors): `cargo clippy -p kasmos -- -D warnings`
- All-targets, all-features lint in Justfile: `cargo clippy --all-targets --all-features -- -D warnings`

## Import Organization

**Order:**
1. Standard library imports (`std::collections::HashMap`, `std::path::PathBuf`)
2. External crate imports (`serde`, `tokio`, `anyhow`, `tracing`, `chrono`)
3. Internal crate imports (`crate::config::Config`, `crate::types::WPState`)

**Path Aliases:**
- No path aliases configured — all imports use full `crate::` paths
- Re-exports in `crates/kasmos/src/lib.rs` provide a flat public API surface
- Use `use super::*;` in `#[cfg(test)]` modules to import parent module items

**Re-export Pattern:**
- `crates/kasmos/src/lib.rs` re-exports key types for external use:
  ```rust
  pub use config::Config;
  pub use error::{KasmosError, Result};
  pub use graph::DependencyGraph;
  ```
- Binary crate (`main.rs`) uses `kasmos::` prefix to access library items: `kasmos::serve::run()`, `kasmos::init_logging()`

## Error Handling

**Strategy: Dual-layer with `thiserror` + `anyhow`**

Use `thiserror` for domain-specific typed errors and `anyhow` for contextual propagation.

**Domain Error Hierarchy (`crates/kasmos/src/error.rs`):**
- Top-level `KasmosError` enum aggregates all subsystem errors via `#[from]`
- Custom `Result<T>` alias: `pub type Result<T> = std::result::Result<T, KasmosError>;`
- Sub-errors: `ConfigError`, `ZellijError`, `SpecParserError`, `StateError`, `LayoutError`
- Each error variant includes contextual fields (e.g., `wp_id`, `path`, `field`, `value`, `reason`)

**Standalone Error Types:**
- `crates/kasmos/src/serve/lock.rs` defines its own `LockError` enum with `thiserror` — not integrated into `KasmosError`
- Uses structured error codes: `pub fn code(&self) -> Option<&'static str>`

**Pattern: Return `anyhow::Result` from top-level commands:**
```rust
pub async fn run() -> anyhow::Result<()> {
    let config = Config::load().context("failed to load config for serve mode")?;
    // ...
}
```

**Pattern: Convert domain errors to `anyhow` at boundaries:**
```rust
fn internal_error(err: anyhow::Error) -> ErrorData {
    ErrorData::internal_error(format!("INTERNAL_ERROR: {}", err), None)
}
```

**Pattern: Use `.context()` for adding human-readable context:**
```rust
kasmos::serve::run().await.context("MCP serve failed")?;
config.load_from_file(&path)
    .with_context(|| format!("Failed to load config file {}", config_path.display()))?;
```

**Pattern: Use `bail!` for early returns with anyhow:**
```rust
if !feature_dir.is_dir() {
    bail!("Feature directory not found: {}", feature_dir.display());
}
```

**Pattern: Guard clauses with comments:**
```rust
// Guard: feature path must resolve to a directory
if !candidate.is_dir() {
    bail!("Feature directory not found: {}", candidate.display());
}
```

## Logging

**Framework:** `tracing` crate with `tracing-subscriber`

**Initialization:** `crates/kasmos/src/logging.rs` — `init_logging(false)`
- Headless mode: `fmt` layer to stderr with target, file, and line number
- Compatibility note: function retains an unused bool parameter for backward compatibility
- Default filter: `kasmos=info` (override with `RUST_LOG` env var)
- Uses `try_init()` for idempotent initialization

**Patterns:**
- Use structured fields for machine-parseable context:
  ```rust
  tracing::info!(wp_id = %wp_id, active = self.active_panes, "WP launched");
  tracing::warn!(wp_id = %wp_id, error = %err, "failed to log SPAWN event");
  ```
- Use `debug!` for operational details: `debug!("Running zellij command: {:?}", args);`
- Use `info!` for state transitions and lifecycle events: `info!("Session started: {}", self.session_name);`
- Use `warn!` for recoverable failures: `warn!("Zellij command failed: {}", stderr);`
- Use `tracing::error!` for unrecoverable conditions: `tracing::error!("Failed to persist state: {}", e);`

## Comments

**When to Comment:**
- Module-level doc comments (`//!`) on every module: explain purpose and key behaviors
- Doc comments (`///`) on all public types, functions, and traits
- Inline comments for non-obvious logic, guard clauses, and Zellij CLI workarounds
- State machine transition tables documented in doc comments on enum types

**Doc Comment Style:**
```rust
/// Check if all dependencies of a work package are satisfied.
///
/// # Arguments
/// * `wp_id` - The work package ID to check
/// * `completed` - Set of completed work package IDs
///
/// # Returns
/// `true` if all dependencies are in the completed set, `false` otherwise
```

**Guard Comment Pattern:**
```rust
// Guard: Unknown work package
// Guard: Check if pane exists
// Guard: Check capacity
// Guard: No eligible WPs
```

## Function Design

**Size:** Most functions are focused and under 50 lines. Longer command handlers are split into helper functions for readability.

**Parameters:** 
- Prefer `&str` over `String` for input parameters
- Use `impl Into<String>` for builder-style APIs: `pub fn with_wp_id(mut self, wp_id: impl Into<String>) -> Self`
- Pass `&Config` by reference; wrap in `Arc` for shared ownership across async tasks

**Return Values:**
- Functions that can fail return `Result<T>` (either `crate::Result<T>` or `anyhow::Result<T>`)
- Functions that might not find something return `Option<T>`: `feature_slug_from_dir()`, `current_branch_name()`
- MCP tool handlers return `Result<Json<Output>, ErrorData>`

## Module Design

**Exports:**
- Each module has a clear public API — prefer explicit `pub` on items that need to be public
- Use `pub(crate)` for items shared internally: `pub(crate) fn validate_identifier()`
- Sub-module types are made public through parent module re-exports

**Barrel Files:**
- `crates/kasmos/src/lib.rs` serves as the main barrel file with `pub mod` declarations and `pub use` re-exports
- `crates/kasmos/src/serve/tools/mod.rs` exports tool sub-modules

**Module Organization Pattern:**
- One concern per module: `config.rs` for configuration, `graph.rs` for dependency graph, `parser.rs` for spec parsing
- Complex subsystems use directory modules: `serve/`, `launch/`, `setup/`
- `#[cfg(test)] mod tests` at the bottom of every source file — co-located tests

## Serde Conventions

**Serialization:**
- Use `#[derive(Serialize, Deserialize)]` on all data types that cross boundaries
- Use `#[serde(rename_all = "snake_case")]` on enums for JSON/TOML compatibility
- Use `#[serde(deny_unknown_fields)]` on MCP tool input structs for strict validation
- Use `#[serde(skip_serializing_if = "Option::is_none")]` for optional fields
- Use `#[serde(default)]` and `#[serde(default = "function_name")]` for optional config fields
- Use `#[non_exhaustive]` on state enums (`WPState`, `RunState`, `WaveState`) for forward compatibility

**Schema Generation:**
- MCP tool input/output structs derive `JsonSchema` from `schemars` crate for auto-generated schemas

## Async Patterns

**Runtime:** `tokio` with `features = ["full"]`

**Shared State:**
- Use `Arc<RwLock<T>>` for shared mutable state across async tasks: `Arc<RwLock<OrchestrationRun>>`
- Use `Arc<Mutex<T>>` for write-heavy shared state: `Arc<Mutex<Option<AuditWriter>>>`
- Use `tokio::sync::mpsc` channels for event communication between subsystems
- Use shared state + explicit function boundaries instead of feature-specific channel layers

**Builder Pattern (Audit Entry):**
```rust
let entry = AuditEntry::new("manager", "spawn_worker", "011-feature")
    .with_wp_id("WP01")
    .with_status("ok")
    .with_details(json!({ "role": "coder" }));
```

## Configuration Conventions

**Precedence:** defaults → `kasmos.toml` file → `KASMOS_` environment variable overrides

**Config Loading Pattern (`crates/kasmos/src/config.rs`):**
- `Config::default()` provides sensible defaults
- `Config::load()` applies full precedence chain and validates
- `Config::load_from_file()` applies partial TOML overlay (all fields optional)
- `Config::load_from_env()` reads `KASMOS_*` environment variables
- `Config::validate()` enforces range invariants

**Environment Variable Naming:**
- Sectioned: `KASMOS_{SECTION}_{FIELD}` (e.g., `KASMOS_AGENT_MAX_PARALLEL_WORKERS`)
- Legacy aliases supported for backward compatibility: `KASMOS_MAX_PANES`, `KASMOS_MODE`

---

*Convention analysis: 2026-02-16*
