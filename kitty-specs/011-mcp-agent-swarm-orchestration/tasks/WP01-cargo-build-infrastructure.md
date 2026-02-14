---
work_package_id: "WP01"
title: "Cargo & Build Infrastructure"
phase: "Phase 0: Foundation"
lane: "planned"
dependencies: []
subtasks: ["T001", "T002", "T003", "T004", "T005", "T006", "T007"]
history:
  - date: "2026-02-13"
    agent: "controller"
    action: "Created WP prompt"
---

# WP01: Cargo & Build Infrastructure

## Implementation Command

```bash
spec-kitty implement WP01
```

## Objective

Update `crates/kasmos/Cargo.toml` to add the `rmcp` MCP SDK dependency, `schemars` for JSON schema generation, and `regex` for message parsing. Feature-gate the TUI dependencies (`ratatui`, `crossterm`, `futures-util`) behind a `tui` feature flag. Add `#[cfg(feature = "tui")]` gates to all TUI-related module declarations and code paths. The project must compile cleanly with both `cargo build` (default, no TUI) and `cargo build --features tui`.

## Context

kasmos is pivoting from a TUI-based orchestrator to an MCP-powered agent swarm. The TUI code (ratatui dashboard, hub navigator) must be preserved but gated behind an optional feature flag so the default build produces a lean binary focused on MCP orchestration.

**Key files:**
- `crates/kasmos/Cargo.toml` — dependency manifest
- `crates/kasmos/src/main.rs` — binary entry point with mod declarations and CLI dispatch
- `crates/kasmos/src/lib.rs` — library crate with pub mod declarations and re-exports

**Current state of Cargo.toml:**
```toml
[dependencies]
ratatui = { version = "0.29", features = ["crossterm"] }
crossterm = { version = "0.28", features = ["event-stream"] }
futures-util = "0.3"
# ... plus serde, tokio, clap, kdl, chrono, etc.
```

**Current mod declarations in main.rs:**
```rust
mod attach;
mod cmd;
mod feature_arg;
mod hub;         // TUI — needs cfg gate
mod list_specs;
mod report;      // TUI — needs cfg gate
mod start;
mod status;
mod stop;
mod tui_cmd;     // TUI — needs cfg gate
```

**Current lib.rs exports (22 pub mod + re-exports):**
All modules are unconditionally exported. TUI-related: `pub mod tui;` and re-exports from `command_handlers`, `commands`, etc.

## Subtasks

### T001: Add rmcp dependency

**Purpose**: Bring in the official Rust MCP SDK for building the `kasmos serve` MCP server.

**Steps**:
1. Add to `[dependencies]` in `crates/kasmos/Cargo.toml`:
   ```toml
   rmcp = { version = "0.15", features = ["server", "transport-io"] }
   ```
2. The `server` feature enables `#[tool]` proc macros and `ServerHandler` trait
3. The `transport-io` feature enables stdio transport (JSON-RPC over stdin/stdout)
4. `schemars` comes transitively via rmcp — no need to add separately unless direct use is needed

**Validation**: `cargo build` succeeds with rmcp resolved

### T002: Add schemars dependency

**Purpose**: Enable JSON Schema generation for MCP tool input types (required by rmcp's `#[tool]` macros).

**Steps**:
1. Add to `[dependencies]`:
   ```toml
   schemars = "0.8"
   ```
2. This is used by MCP tool input structs that need `#[derive(JsonSchema)]`

**Validation**: `cargo build` succeeds

### T003: Add regex dependency

**Purpose**: Enable structured message parsing from the msg-log pane using the `[KASMOS:<sender>:<event>]` pattern.

**Steps**:
1. Add to `[dependencies]`:
   ```toml
   regex = "1"
   ```
2. The message parsing regex is: `\[KASMOS:([^:]+):([^\]]+)\]\s*(.*)`

**Validation**: `cargo build` succeeds

### T004: Create features section in Cargo.toml

**Purpose**: Define the `tui` feature flag that controls whether TUI code is compiled.

**Steps**:
1. Add a `[features]` section to `Cargo.toml`:
   ```toml
   [features]
   default = []
   tui = ["dep:ratatui", "dep:crossterm", "dep:futures-util"]
   ```
2. The `dep:` prefix is the Rust 2024 edition syntax for optional dependency features
3. `default = []` means default builds exclude TUI — this is intentional for the MCP-focused binary

**Validation**: `cargo build` succeeds (no TUI deps pulled)

### T005: Make TUI dependencies optional

**Purpose**: Move ratatui, crossterm, and futures-util from required to optional dependencies.

**Steps**:
1. Modify existing dependency lines in `[dependencies]`:
   ```toml
   ratatui = { version = "0.29", features = ["crossterm"], optional = true }
   crossterm = { version = "0.28", features = ["event-stream"], optional = true }
   futures-util = { version = "0.3", optional = true }
   ```
2. These are now only compiled when `--features tui` is specified

**Validation**: `cargo build` succeeds without pulling ratatui/crossterm. `cargo build --features tui` also succeeds.

### T006: Add cfg gates to TUI modules

**Purpose**: Conditionally compile TUI-related modules so the default build doesn't reference them.

**Steps**:

1. **In `main.rs`** — gate mod declarations:
   ```rust
   #[cfg(feature = "tui")]
   mod hub;
   #[cfg(feature = "tui")]
   mod report;
   #[cfg(feature = "tui")]
   mod tui_cmd;
   ```

2. **In `main.rs`** — gate the CLI commands that use TUI modules:
   - The `Tui` variant in the `Commands` enum needs `#[cfg(feature = "tui")]`
   - The `None` case that calls `hub::run()` needs a conditional:
     ```rust
     None => {
         #[cfg(feature = "tui")]
         hub::run().await.context("Hub TUI failed")?;
         #[cfg(not(feature = "tui"))]
         {
             eprintln!("TUI not available. Build with --features tui to enable.");
             eprintln!("Usage: kasmos <spec-prefix> | kasmos serve | kasmos setup | kasmos list | kasmos status");
         }
     }
     ```
   - The `Some(Commands::Tui { .. })` match arm needs `#[cfg(feature = "tui")]`

3. **In `lib.rs`** — gate TUI module declarations and re-exports:
   ```rust
   #[cfg(feature = "tui")]
   pub mod tui;
   ```
   - Also gate re-exports that reference TUI types
   - Modules like `command_handlers`, `commands`, `engine`, `detector`, `health`, `shutdown`, `session` are NOT gated here — they'll be removed entirely in WP13 but for now they must still compile

4. **In any files that import from gated modules** — add corresponding cfg gates to `use` statements

**Edge cases**:
- The `hub/` module may import from non-TUI modules (like `config`, `list_specs`) — those imports are fine since they flow from TUI → non-TUI
- If `report.rs` is imported by non-TUI code, those imports need cfg gates too

**Validation**: Both `cargo build` and `cargo build --features tui` succeed with no errors

### T007: Verify compilation both ways

**Purpose**: Final validation that both build configurations work.

**Steps**:
1. Run `cargo build` — must succeed with no errors
2. Run `cargo build --features tui` — must succeed with no errors
3. Run `cargo test` — must pass (tests may need cfg gates if they reference TUI types)
4. Check for warnings about unused imports or dead code — fix any introduced by the gating

**Validation**: Zero errors, minimal warnings

## Test Strategy

- `cargo build` succeeds without TUI features (primary)
- `cargo build --features tui` succeeds with TUI features
- `cargo test` passes for both configurations
- No unused import warnings introduced by cfg gating

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Conditional compilation may break cross-module imports | Grep for all `use` statements referencing `hub`, `tui`, `tui_cmd`, `report` and gate them |
| Existing tests may reference TUI types | Add `#[cfg(feature = "tui")]` to test modules that use TUI types |
| rmcp version may have breaking API changes | Pin to 0.15.x, check crates.io for latest compatible version |

## Review Guidance

- Verify all three TUI deps are `optional = true`
- Verify `default = []` (not `default = ["tui"]`)
- Verify every `mod hub`, `mod tui`, `mod tui_cmd`, `mod report` has `#[cfg(feature = "tui")]`
- Verify the `None =>` match arm has both `cfg(feature = "tui")` and `cfg(not(feature = "tui"))` branches
- Test both `cargo build` and `cargo build --features tui`

## Activity Log

<!-- Chronological, append-only. Agents: add entries as you work. -->
| Date | Agent | Event |
|------|-------|-------|
| 2026-02-13 | controller | Created WP prompt |
