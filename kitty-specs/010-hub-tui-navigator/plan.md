# Implementation Plan: Hub TUI Navigator

**Branch**: `010-hub-tui-navigator` | **Date**: 2026-02-12 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/kitty-specs/010-hub-tui-navigator/spec.md`

## Summary

Replace bare `kasmos` (no args) with an interactive ratatui TUI that serves as a project command center — browsing feature specs, launching OpenCode agent panes for spec creation and planning, and starting implementation sessions in new Zellij tabs. Additionally, invert `kasmos start` to default to TUI mode with `--no-tui` opt-out.

The hub TUI is a lightweight navigator (`src/hub/`) that delegates heavy workflows to agent panes (spec creation, planning) and orchestration sessions (implementation). It scans `kitty-specs/` for feature state, detects running orchestrations via lock files and Zellij sessions, and manages pane/tab lifecycle through the existing `ZellijCli` abstraction.

## Technical Context

**Language/Version**: Rust 2024 edition (workspace edition), matching existing crate
**Primary Dependencies**: ratatui 0.30, crossterm 0.29, clap 4.5, tokio 1.49 (all already in Cargo.toml)
**Storage**: Filesystem — reads `kitty-specs/` directory structure, `.kasmos/run.lock`, Zellij session list
**Testing**: `cargo test` — unit tests for feature scanning, state derivation, action resolution; integration tests for CLI changes
**Target Platform**: Linux terminal (Zellij required for actions, graceful degradation without)
**Project Type**: Single crate binary (existing `kasmos` binary extended)
**Performance Goals**: Hub renders initial feature list within 500ms, keyboard input latency <50ms (NFR-001, NFR-002)
**Constraints**: Periodic disk refresh must not block event loop (NFR-003); minimum 80x24 terminal (NFR-004)
**Scale/Scope**: Up to 50 features in `kitty-specs/`

## Constitution Check

*No constitution file found (`.kittify/memory/constitution.md` absent). Skipped.*

## Project Structure

### Documentation (this feature)

```
kitty-specs/010-hub-tui-navigator/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # N/A (no external APIs)
└── tasks.md             # Phase 2 output (/spec-kitty.tasks)
```

### Source Code (repository root)

```
crates/kasmos/src/
├── main.rs              # CLI restructured: optional subcommand, hub as default
├── hub/                 # NEW — Hub TUI module
│   ├── mod.rs           # Hub event loop (setup, run, restore)
│   ├── app.rs           # Hub App state, rendering, event handling
│   ├── scanner.rs       # FeatureScanner — reads kitty-specs/ into Vec<FeatureEntry>
│   ├── actions.rs       # HubAction resolution + Zellij pane/tab dispatch
│   └── keybindings.rs   # Hub-specific key handlers
├── tui/
│   ├── mod.rs           # MODIFIED — extract setup_terminal/restore_terminal/install_panic_hook to pub
│   ├── app.rs           # Orchestration TUI (existing, add hub keybinding)
│   ├── event.rs         # EventHandler (already pub, reused by hub)
│   └── keybindings.rs   # Orchestration keybindings (existing, add hub keybinding)
├── zellij.rs            # MODIFIED — add new_pane_direction() to ZellijCli trait
├── list_specs.rs        # Existing (can be simplified to call scanner::FeatureScanner)
├── start.rs             # MODIFIED — invert TUI default, add --no-tui flag
└── ...                  # Other existing modules unchanged
```

**Structure Decision**: New `hub/` module as a peer to `tui/`. The hub has a completely different state model (`Vec<FeatureEntry>` vs `OrchestrationRun`) and lifecycle (no engine channels). Shared terminal plumbing (`setup_terminal`, `restore_terminal`, `install_panic_hook`, `EventHandler`) is made `pub` on the existing `tui` module for reuse.

## Architecture Decisions

### AD-001: CLI Restructuring — Optional Subcommand

**Current**: `Cli { command: Commands }` with required subcommand. Bare `kasmos` errors.

**Change**: Make subcommand optional using `#[command(subcommand)] command: Option<Commands>`. When `None`, dispatch to `hub::run()`. All existing subcommands remain unchanged.

```rust
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

// In main():
match cli.command {
    None => hub::run().await?,        // NEW: bare kasmos → hub TUI
    Some(Commands::List) => { ... },  // unchanged
    Some(Commands::Start { .. }) => { ... },
    // ...
}
```

**Rationale**: Minimal change to existing CLI parsing. `Option<Commands>` is idiomatic clap for default commands.

### AD-002: Hub Module Structure

**`hub/mod.rs`** — Entry point `pub async fn run() -> anyhow::Result<()>`:
- Read `ZELLIJ_SESSION_NAME` env var (absent = read-only mode)
- Call `tui::setup_terminal()`, `tui::install_panic_hook()`
- Create `hub::App` with initial `FeatureScanner::scan()` results
- Run async event loop (crossterm events + periodic refresh timer)
- Call `tui::restore_terminal()` on exit

**`hub/app.rs`** — Hub application state:
- `features: Vec<FeatureEntry>` — current feature list
- `selected: usize` — currently highlighted feature
- `view: HubView` — enum { List, Detail(feature_idx) }
- `input_mode: InputMode` — enum { Normal, NewFeaturePrompt(String) }
- `zellij_session: Option<String>` — session name from env
- `should_quit: bool`

**`hub/scanner.rs`** — Feature state detection:
- Scans `kitty-specs/` directory entries
- For each feature: checks `spec.md` (exists + non-empty), `plan.md` (exists), `tasks/WPxx-*.md` (count + lane parsing)
- Checks `.kasmos/run.lock` in worktree for orchestration status (PID liveness)
- Queries Zellij session list for `kasmos-<feature>` pattern
- Returns `Vec<FeatureEntry>` sorted by feature number

**`hub/actions.rs`** — Action dispatch:
- Resolves available `HubAction` variants based on `FeatureEntry` state
- Executes Zellij commands via `ZellijCli`:
  - `CreateSpec` / `Plan` / `GenerateTasks`: `zellij action new-pane --direction right -- ocx oc -- --prompt "<slash-cmd>" --agent controller`
  - `StartImplementation`: `zellij action new-tab -- kasmos start <feature> [--mode wave-gated]`
  - `Attach`: `zellij action go-to-tab-name kasmos-<feature>`
  - `OpenHub` (from orchestration TUI): `zellij action new-tab -- kasmos`

### AD-003: ZellijCli Trait Extension

Add `new_pane_direction()` to the `ZellijCli` trait:

```rust
async fn new_pane_direction(
    &self,
    session: &str,
    direction: PaneDirection,
    command: &str,
    args: &[&str],
) -> Result<()>;
```

Where `PaneDirection` is `enum PaneDirection { Right, Down }`.

Implementation calls `zellij --session <session> action new-pane --direction <dir> -- <command> <args>`.

Existing `new_pane()` and `run_in_pane()` remain unchanged.

### AD-004: `kasmos start` TUI Inversion

**Current**: `Start { feature, mode }` — always attaches to Zellij directly.

**Change**: Add `--no-tui` flag (default `false`), change `--tui` to hidden deprecated alias.

```rust
Start {
    feature: String,
    #[arg(long, default_value = "continuous")]  // Changed from wave-gated
    mode: String,
    #[arg(long)]
    no_tui: bool,
    #[arg(long, hide = true)]  // backward compat
    tui: bool,
}
```

When `!no_tui` (default): after creating the Zellij session and starting the engine, launch the orchestration TUI via `tui::run()` instead of `zellij attach`.

When `no_tui`: use existing `zellij attach` behavior.

The `--mode` default also changes from `wave-gated` to `continuous` (per clarification Q4).

### AD-005: Orchestration TUI → Hub Navigation

Add keybinding (e.g., `Alt+h`) in `tui/keybindings.rs` that:
1. Queries `zellij action query-tab-names` for a tab named `hub` or `kasmos-hub`
2. If found: `zellij action go-to-tab-name <hub-tab>`
3. If not found: `zellij action new-tab -- kasmos` (launches new hub instance)

### AD-006: Mode Selection UX

In `hub/actions.rs`, when `StartImplementation` is triggered:
- **Enter**: Default continuous mode. If feature has >6 WPs, show a `tui-popup` confirmation: "This feature has N WPs. Use wave-gated mode? [y/n]"
- **Shift+Enter**: Always wave-gated mode, no prompt

### AD-007: Feature Status Refresh

Hub uses `tokio::time::interval(Duration::from_secs(5))` in the event loop. Each tick calls `FeatureScanner::scan()` and diffs against the current `features` vec, preserving selection state. The scan runs on `tokio::task::spawn_blocking` to avoid blocking the event loop (NFR-003).

Manual refresh keybinding (`r`) triggers an immediate scan outside the interval.

## Complexity Tracking

No constitution violations — single crate, no new external dependencies required (all deps already in Cargo.toml), no new infrastructure.
