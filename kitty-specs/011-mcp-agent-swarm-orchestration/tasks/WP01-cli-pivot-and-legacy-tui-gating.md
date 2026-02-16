---
work_package_id: WP01
title: CLI Pivot and Legacy TUI Gating
lane: "done"
dependencies: []
base_branch: main
base_commit: e2efe83d5f7238ed6104250098ac15f90cc6038e
created_at: '2026-02-14T19:22:02.295961+00:00'
subtasks:
- T001
- T002
- T003
- T004
- T005
- T006
phase: Phase 0 - CLI Pivot and Core Foundation
assignee: 'opencode'
agent: "reviewer"
shell_pid: "3419985"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP01 - CLI Pivot and Legacy TUI Gating

## Important: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** - Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP01
```

---

## Objectives & Success Criteria

Replace the legacy orchestration entry points with the new command surface while preserving old TUI code behind a feature gate (FR-024). After this WP:

1. `cargo build` succeeds with default features (no TUI) - produces a lean MCP-focused binary
2. `cargo build --features tui` succeeds - all legacy TUI code still compiles
3. `kasmos --help` shows the new command topology: `kasmos [spec-prefix]`, `kasmos serve`, `kasmos setup`, `kasmos list`, `kasmos status`
4. Default invocation (`kasmos` with no args) no longer launches the hub TUI
5. Legacy commands (`start`, `stop`, `attach`, `cmd`, `tui-ctrl`, `tui-preview`) are removed from the CLI surface
6. `kasmos list` and `kasmos status` continue to work

## Context & Constraints

- **Constitution**: Rust 2024 edition, tokio async, single binary distribution. See `.kittify/memory/constitution.md`.
- **Plan reference**: `kitty-specs/011-mcp-agent-swarm-orchestration/plan.md` - Section "Project Structure" defines the target module layout.
- **FR-024**: Preserve existing TUI code in a disconnected state (not deleted, just unwired from entry points).
- **Spec FR-006**: `kasmos serve` exposes MCP orchestration capabilities to the manager agent.
- **Key constraint**: Do NOT delete any TUI source files. Feature-gate them behind `#[cfg(feature = "tui")]`.

**Current codebase state** (read these files for accurate context):
- `crates/kasmos/Cargo.toml` (36 lines) - 33 dependencies, no `[features]` section, edition inherited from workspace
- `crates/kasmos/src/main.rs` (208 lines) - 11 binary-only mod declarations, `Cli` struct with `Commands` enum (List, Start, Status, Cmd, Attach, Stop, TuiCtrl, TuiPreview), `bootstrap_start_in_zellij()` helper
- `crates/kasmos/src/lib.rs` (60 lines) - 28 `pub mod` declarations + re-exports

## Subtasks & Detailed Guidance

### Subtask T001 - Add MCP and schema dependencies to Cargo.toml

**Purpose**: Bring in `rmcp` (Rust MCP SDK), `schemars` (JSON Schema generation for tool inputs), and `regex` (message parsing) as new dependencies.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Add to `[dependencies]`:
   ```toml
   rmcp = { version = "0.1", features = ["server", "transport-io"] }
   schemars = "0.8"
   regex = "1"
   ```
3. **Important**: Check crates.io for the latest `rmcp` version. The plan specifies it but the exact version may differ. The required features are `server` (for `#[tool]` proc macros and `ServerHandler` trait) and `transport-io` (for stdio transport).
4. `schemars` is needed by rmcp's `#[tool]` macro for `#[derive(JsonSchema)]` on tool input structs.
5. `regex` is for parsing structured messages matching `[KASMOS:<sender>:<event>] <json_payload>`.

**Files**: `crates/kasmos/Cargo.toml`
**Validation**: `cargo check` resolves the new dependencies without errors.

### Subtask T002 - Feature-gate TUI dependencies and modules

**Purpose**: Make TUI-specific dependencies optional and gate TUI module declarations behind `#[cfg(feature = "tui")]` so the default build produces a lean binary.

**Steps**:
1. Add a `[features]` section to `crates/kasmos/Cargo.toml`:
   ```toml
   [features]
   default = []
   tui = ["dep:ratatui", "dep:crossterm", "dep:futures-util", "dep:tui-popup", "dep:tui-nodes", "dep:tui-logger", "dep:ratatui-macros"]
   ```
2. Mark TUI dependencies as optional in `[dependencies]`:
   ```toml
   ratatui = { version = "0.30", features = ["crossterm"], optional = true }
   ratatui-macros = { version = "0.7", optional = true }
   crossterm = { version = "0.29", features = ["event-stream"], optional = true }
   tui-popup = { version = "0.7", optional = true }
   tui-nodes = { version = "0.10.0", optional = true }
   futures-util = { version = "0.3.31", optional = true }
   tui-logger = { version = "0.18", features = ["tracing-support"], optional = true }
   ```
3. In `crates/kasmos/src/lib.rs`, gate TUI modules:
   ```rust
   #[cfg(feature = "tui")]
   pub mod tui;
   ```
   Also gate any re-exports that reference TUI types.
4. In `crates/kasmos/src/main.rs`, gate TUI-related mod declarations:
   ```rust
   #[cfg(feature = "tui")]
   mod hub;
   #[cfg(feature = "tui")]
   mod report;
   #[cfg(feature = "tui")]
   mod tui_cmd;
   #[cfg(feature = "tui")]
   mod tui_preview;
   ```
5. Gate any `use` statements in other files that import from gated modules.
6. Modules that are used by BOTH TUI and non-TUI paths (like `config`, `list_specs`, `feature_arg`, `types`) remain ungated.

**Edge cases**:
- The `log` crate is used by `tui-logger` but may also be needed by non-TUI code. Keep it ungated if so.
- If `report.rs` is imported by non-TUI code, those imports need cfg gates too.
- Legacy engine modules (`engine`, `detector`, `session`, `command_handlers`, `commands`, etc.) stay ungated for now - they'll be cleaned up separately.

**Files**: `crates/kasmos/Cargo.toml`, `crates/kasmos/src/lib.rs`, `crates/kasmos/src/main.rs`
**Validation**: `cargo build` succeeds without TUI deps. `cargo build --features tui` also succeeds.

### Subtask T003 - Replace clap command model with new launcher-first surface

**Purpose**: Redesign the CLI from `kasmos start <feature>` to `kasmos [spec-prefix]` as the primary entry point, adding `serve`, `setup`, `list`, and `status` subcommands.

**Steps**:
1. Rewrite the `Commands` enum in `crates/kasmos/src/main.rs`:
   ```rust
   #[derive(Subcommand)]
   enum Commands {
       /// Run MCP server (stdio transport, spawned by manager agent)
       Serve,
       /// Validate environment and generate default configs
       Setup,
       /// List available feature specs
       List,
       /// Show orchestration status for a feature
       Status {
           /// Feature directory (optional, auto-detects)
           feature: Option<String>,
       },
   }
   ```
2. Add `spec_prefix` as an optional positional argument to `Cli`:
   ```rust
   #[derive(Parser)]
   #[command(name = "kasmos", version, about = "MCP agent swarm orchestrator")]
   struct Cli {
       /// Feature spec prefix (e.g., "011") - launches orchestration session
       spec_prefix: Option<String>,
       #[command(subcommand)]
       command: Option<Commands>,
   }
   ```
3. Update `main()` dispatch:
   - If `command` is `Some(...)`: dispatch to `Serve`, `Setup`, `List`, `Status`
   - If `command` is `None` and `spec_prefix` is `Some(prefix)`: dispatch to launch flow (stub for now - `todo!("Launch with spec prefix")`)
   - If both are `None`: dispatch to launch flow with no prefix (stub for now)
4. Remove old commands: `Start`, `Cmd`, `Attach`, `Stop`, `TuiCtrl`, `TuiPreview`
5. Remove `bootstrap_start_in_zellij()` function (legacy)
6. Update `after_help` text to reflect new commands

**Files**: `crates/kasmos/src/main.rs`
**Validation**: `kasmos --help` shows new topology. `kasmos list` works. `kasmos status` works.

### Subtask T004 - Create module stubs for launch, serve, and setup

**Purpose**: Create the directory structure and minimal module stubs that will be populated in subsequent WPs.

**Steps**:
1. Create `crates/kasmos/src/launch/mod.rs`:
   ```rust
   //! Launch flow: feature resolution, preflight, layout generation, session bootstrap.
   pub mod detect;
   pub mod layout;
   pub mod session;

   pub async fn run(_spec_prefix: Option<&str>) -> anyhow::Result<()> {
       todo!("Launch flow implementation in WP02/WP03")
   }
   ```
2. Create `crates/kasmos/src/launch/detect.rs`:
   ```rust
   //! Feature detection pipeline: arg -> branch -> directory -> none.
   ```
3. Create `crates/kasmos/src/launch/layout.rs`:
   ```rust
   //! KDL layout generation for orchestration tabs.
   ```
4. Create `crates/kasmos/src/launch/session.rs`:
   ```rust
   //! Zellij session/tab creation and lifecycle.
   ```
5. Create `crates/kasmos/src/serve/mod.rs`:
   ```rust
   //! MCP server: kasmos serve (stdio transport).
   pub mod registry;
   pub mod messages;
   pub mod audit;
   pub mod dashboard;
   pub mod lock;
   pub mod tools;

   pub async fn run() -> anyhow::Result<()> {
       todo!("MCP serve implementation in WP04")
   }
   ```
6. Create `crates/kasmos/src/serve/tools/mod.rs` and stub files for each tool.
7. Create `crates/kasmos/src/setup/mod.rs`:
   ```rust
   //! Environment validation and default config generation.
   pub async fn run() -> anyhow::Result<()> {
       todo!("Setup implementation in WP10")
   }
   ```
8. Add `pub mod launch;`, `pub mod serve;`, `pub mod setup;` to `crates/kasmos/src/lib.rs`.

**Files**: New files under `crates/kasmos/src/launch/`, `crates/kasmos/src/serve/`, `crates/kasmos/src/setup/`
**Parallel?**: Yes - can proceed once T003 command routing shape is agreed.
**Validation**: `cargo build` succeeds with new module stubs.

### Subtask T005 - Preserve list/status and remove old command wiring

**Purpose**: Keep `kasmos list` and `kasmos status` working while removing old `start`, `stop`, `attach`, `cmd` command wiring.

**Steps**:
1. The existing `list_specs.rs` (146 lines) and `status.rs` (123 lines) are binary-only modules. Keep them and wire them to the new `Commands::List` and `Commands::Status` variants.
2. Remove (or cfg-gate) the following binary-only modules that are no longer wired:
   - `start.rs` (647 lines) - the old orchestration launcher (heavy TUI + engine wiring)
   - `stop.rs` (90 lines) - FIFO/SIGTERM stop command
   - `attach.rs` (83 lines) - session reattachment
   - `cmd.rs` (257 lines) - FIFO command subcommand
   - `sendmsg.rs` (69 lines) - FIFO message sender
3. For safety per FR-024: use `#[cfg(feature = "tui")]` on these modules rather than deleting them. This way `cargo build --features tui` still compiles them.
4. Update any imports in `main.rs` that reference removed/gated modules.
5. Verify `kasmos list` still scans `kitty-specs/` and shows unfinished features.
6. Verify `kasmos status` still reads `.kasmos/state.json` and shows WP progress.

**Files**: `crates/kasmos/src/main.rs`, potentially `crates/kasmos/src/start.rs`, `stop.rs`, `attach.rs`, `cmd.rs`, `sendmsg.rs`
**Validation**: `kasmos list` and `kasmos status` produce correct output.

### Subtask T006 - Validate compile matrix and fix broken imports

**Purpose**: Final verification that both build configurations work after all changes.

**Steps**:
1. Run `cargo build` - must succeed with zero errors
2. Run `cargo build --features tui` - must succeed with zero errors
3. Run `cargo test` - must pass for default features
4. Run `cargo test --features tui` - must pass with TUI features
5. Fix any unused import warnings introduced by cfg gating
6. Fix any dead_code warnings that are new (existing ones are acceptable)
7. Verify `kasmos --help` shows new command topology

**Files**: Any files with warnings or errors
**Validation**: Zero errors on both build configs, minimal new warnings.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Accidental deletion of TUI code instead of feature-gating | Gate and preserve - never delete. Grep for all `mod hub`, `mod tui`, `use tui::` to ensure complete gating. |
| clap regressions breaking list/status | Smoke-test both commands immediately after CLI refactor. |
| Conditional compilation breaking cross-module imports | Grep for all `use` statements referencing gated modules and add corresponding cfg gates. |
| rmcp version incompatibility | Check crates.io for latest compatible version. Pin to specific minor version. |

## Review Guidance

- Verify all TUI deps are `optional = true` and `default = []` (not `default = ["tui"]`)
- Verify every `mod hub`, `mod tui`, `mod tui_cmd`, `mod report`, `mod tui_preview` has `#[cfg(feature = "tui")]`
- Verify old commands (Start, Cmd, Attach, Stop, TuiCtrl, TuiPreview) are removed from CLI surface
- Verify new commands (Serve, Setup, List, Status) are present
- Verify `spec_prefix` positional argument exists on `Cli` struct
- Test both `cargo build` and `cargo build --features tui`
- Verify `kasmos list` and `kasmos status` still work

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-14T20:40:54Z – coder – shell_pid=3202027 – lane=for_review – Submitted for review via swarm
- 2026-02-14T20:40:54Z – reviewer – shell_pid=3419985 – lane=doing – Started review via workflow command
- 2026-02-14T20:44:35Z – reviewer – shell_pid=3419985 – lane=done – Review passed via swarm
