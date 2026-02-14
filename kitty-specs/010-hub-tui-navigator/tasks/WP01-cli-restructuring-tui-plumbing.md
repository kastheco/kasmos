---
work_package_id: WP01
title: CLI Restructuring & TUI Plumbing Extraction
lane: "done"
dependencies: []
subtasks:
- T001
- T002
- T003
- T004
- T005
phase: Phase 1 - Foundation
assignee: ''
agent: ''
shell_pid: ''
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-13T03:53:23Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP01 - CLI Restructuring & TUI Plumbing Extraction

## Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** -- Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Markdown Formatting
Wrap HTML/XML tags in backticks: `` `<div>` ``, `` `<script>` ``
Use language identifiers in code blocks: ````rust`, ````bash`

---

## Objectives & Success Criteria

- Make the CLI subcommand optional so bare `kasmos` (no args) dispatches to the hub TUI
- Extract shared terminal plumbing from `tui/mod.rs` as `pub` functions for reuse by the new hub module
- Add a hub module stub that compiles and is wired into the CLI dispatch
- Update CLI help text to document the new default behavior
- **All existing subcommands must continue to function identically** (FR-019)
- `cargo build` succeeds with zero new warnings
- `cargo test` passes (existing tests unbroken)

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-001: CLI Restructuring, AD-002: Hub Module Structure)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-001, FR-014, FR-019)
- **Research**: `kitty-specs/010-hub-tui-navigator/research.md` (R-003: clap Optional Subcommand Pattern)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md`
- **Quickstart**: `kitty-specs/010-hub-tui-navigator/quickstart.md`

### Key Architectural Decisions

- **AD-001**: Use `Option<Commands>` for the subcommand field. When `None`, dispatch to `hub::run()`.
- **AD-002**: Hub module lives at `crates/kasmos/src/hub/` as a peer to `crates/kasmos/src/tui/`.
- Shared terminal plumbing (`setup_terminal`, `restore_terminal`, `install_panic_hook`) becomes `pub` on the `tui` module.

## Subtasks & Detailed Guidance

### Subtask T001 - Make Commands optional in Cli struct

- **Purpose**: Allow bare `kasmos` invocation without a subcommand.
- **Steps**:
  1. Open `crates/kasmos/src/main.rs`
  2. Change `command: Commands` to `command: Option<Commands>` in the `Cli` struct
  3. Update the `match cli.command` block to handle `None` and wrap existing arms in `Some(...)`
- **Files**: `crates/kasmos/src/main.rs`
- **Parallel?**: No (T002 and T003 depend on this)
- **Notes**: This is the idiomatic clap 4.x pattern per R-003. The `#[command(subcommand)]` attribute works with `Option<T>`.

**Current code** (lines 39-42 of `crates/kasmos/src/main.rs`):
```rust
struct Cli {
    #[command(subcommand)]
    command: Commands,
}
```

**Target code**:
```rust
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}
```

**Current match** (lines 88-109):
```rust
match cli.command {
    Commands::List => { ... }
    Commands::Start { feature, mode } => { ... }
    // ...
}
```

**Target match**:
```rust
match cli.command {
    None => hub::run().await.context("Hub TUI failed")?,
    Some(Commands::List) => { ... }
    Some(Commands::Start { feature, mode }) => { ... }
    // ...
}
```

### Subtask T002 - Add hub module declaration stub

- **Purpose**: Create the hub module directory and a minimal `mod.rs` that compiles.
- **Steps**:
  1. Create directory `crates/kasmos/src/hub/`
  2. Create `crates/kasmos/src/hub/mod.rs` with a placeholder `pub async fn run() -> anyhow::Result<()>` that returns `Ok(())`
  3. Add `mod hub;` to `crates/kasmos/src/main.rs` (in the module declarations section, after `mod report;`)
- **Files**:
  - `crates/kasmos/src/hub/mod.rs` (new)
  - `crates/kasmos/src/main.rs` (add `mod hub;`)
- **Parallel?**: No (T003 depends on this)
- **Notes**: The hub module is declared in `main.rs` (binary crate), not in `lib.rs`. This is because the hub is a binary-only feature that uses types from the library but doesn't need to be part of the public API. Later WPs will add `app.rs`, `scanner.rs`, `actions.rs`, `keybindings.rs` submodules.

**Stub content for `crates/kasmos/src/hub/mod.rs`**:
```rust
//! Hub TUI module -- interactive project command center.
//!
//! Provides a ratatui-based TUI for browsing feature specs, launching
//! OpenCode agent panes, and starting implementation sessions.

/// Run the hub TUI.
///
/// This is the entry point when `kasmos` is invoked with no subcommand.
/// Currently a placeholder -- full implementation in WP03.
pub async fn run() -> anyhow::Result<()> {
    println!("Hub TUI placeholder -- full implementation coming in WP03");
    Ok(())
}
```

### Subtask T003 - Wire None match arm to hub::run()

- **Purpose**: Connect the `None` case (bare `kasmos`) to the hub module.
- **Steps**:
  1. In the `match cli.command` block in `main.rs`, add `None => hub::run().await.context("Hub TUI failed")?,` as the first arm
  2. Ensure all other arms are wrapped in `Some(...)`
- **Files**: `crates/kasmos/src/main.rs`
- **Parallel?**: No (depends on T001 and T002)
- **Notes**: **Important (R-006)**: The current `init_logging()` call initializes tracing to stderr, which would corrupt the hub TUI's alternate screen. Move `let _ = kasmos::init_logging();` from before the match into each `Some(Commands::...)` arm, so it only runs for subcommands. The `None` (hub) arm must NOT call `init_logging()`.

### Subtask T004 - Extract TUI plumbing to pub

- **Purpose**: Make `setup_terminal()`, `restore_terminal()`, and `install_panic_hook()` available to the hub module.
- **Steps**:
  1. Open `crates/kasmos/src/tui/mod.rs`
  2. Change `fn setup_terminal()` to `pub fn setup_terminal()`
  3. Change `fn restore_terminal(...)` to `pub fn restore_terminal(...)`
  4. Change `fn install_panic_hook()` to `pub fn install_panic_hook()`
  5. Verify the `tui::run()` function still compiles and works (it calls these internally)
- **Files**: `crates/kasmos/src/tui/mod.rs`
- **Parallel?**: Yes (independent of T001-T003)
- **Notes**: This is a visibility-only change. No logic modifications. The functions are already well-documented. The hub will call them as `crate::tui::setup_terminal()` etc. Since `tui` is declared as `pub mod tui` in `lib.rs`, these functions become accessible from the binary crate via `kasmos::tui::setup_terminal()`.

**Current signatures** (lines 33, 42, 56 of `crates/kasmos/src/tui/mod.rs`):
```rust
fn setup_terminal() -> anyhow::Result<Terminal<CrosstermBackend<Stdout>>> {
fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<Stdout>>) -> anyhow::Result<()> {
fn install_panic_hook() {
```

**Target signatures**:
```rust
pub fn setup_terminal() -> anyhow::Result<Terminal<CrosstermBackend<Stdout>>> {
pub fn restore_terminal(terminal: &mut Terminal<CrosstermBackend<Stdout>>) -> anyhow::Result<()> {
pub fn install_panic_hook() {
```

### Subtask T005 - Update after_help CLI text

- **Purpose**: Document the new default behavior in CLI help output.
- **Steps**:
  1. In `crates/kasmos/src/main.rs`, update the `after_help` string in the `#[command(...)]` attribute
  2. Change the first line from `kasmos                              List available features` to `kasmos                              Launch hub TUI (interactive project navigator)`
  3. Update the "Typical Workflow" section to start with the hub
  4. Add a note about `--no-tui` for `kasmos start`
- **Files**: `crates/kasmos/src/main.rs`
- **Parallel?**: No (should be done after T001 for consistency)
- **Notes**: FR-014 requires `kasmos --help` to document the hub TUI as the no-argument default.

**Target after_help text**:
```
Quick Start:
  kasmos                              Launch hub TUI (project navigator)
  kasmos start <feature>              Start orchestration (TUI dashboard)
  kasmos start <feature> --no-tui     Start without TUI (direct Zellij attach)
  kasmos start <feature> --mode wave-gated
                                       Start with wave gates (default: continuous)
  kasmos status [feature]             Check WP progress
  kasmos cmd status                   Send controller command via FIFO
  kasmos cmd focus WP02               Focus a work package pane
  kasmos attach <feature>             Attach to Zellij session
  kasmos stop [feature]               Gracefully stop orchestration

Typical Workflow:
  1. kasmos                           Open the hub TUI
  2. Select a feature and start       Hub launches orchestration in new tab
  3. kasmos cmd status                Query live orchestration state
  4. Alt+h in orchestration TUI       Switch back to hub
  5. kasmos stop                      Stop when done
```

## Test Strategy

- **Build verification**: `cargo build -p kasmos` must succeed with zero new warnings.
- **Existing tests**: `cargo test -p kasmos` must pass (no regressions).
- **Manual verification**:
  - `kasmos` (no args) prints the hub placeholder message and exits
  - `kasmos list` still lists features
  - `kasmos --help` shows updated help text with hub as default
  - `kasmos start --help` still shows start options

## Risks & Mitigations

- **Breaking existing CLI**: The `Option<Commands>` change could affect argument parsing. Mitigation: All existing subcommands are wrapped in `Some(...)` -- behavior is identical.
- **Module visibility**: Making TUI functions `pub` could expose internal APIs. Mitigation: These are utility functions with no security implications; they're already well-documented.

## Review Guidance

- Verify `Option<Commands>` pattern matches clap 4.x idioms
- Verify all existing `Commands::*` arms are wrapped in `Some(...)`
- Verify `pub` visibility on TUI plumbing functions
- Verify `after_help` text is accurate and well-formatted
- Run `cargo build` and `cargo test` to confirm no regressions

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T04:41:06Z – unknown – lane=done – Moved to done
