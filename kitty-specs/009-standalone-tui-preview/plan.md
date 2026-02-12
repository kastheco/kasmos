# Implementation Plan: Standalone TUI Preview Mode

**Branch**: `master` | **Date**: 2026-02-12 | **Spec**: `kitty-specs/009-standalone-tui-preview/spec.md`
**Input**: Feature specification from `kitty-specs/009-standalone-tui-preview/spec.md`

## Summary

Add a `kasmos tui` subcommand that launches the ratatui TUI with animated mock data — no Zellij, no git, no orchestration. Exploits the existing `tui::run(watch_rx, action_tx)` channel-based interface to feed synthetic `OrchestrationRun` state into the TUI, with a deterministic background loop cycling work packages through all states every ~3 seconds.

Two files touched: `crates/kasmos/src/main.rs` (~15 lines) and new `crates/kasmos/src/tui_preview.rs` (~80-100 lines). Zero changes to existing TUI or orchestration code.

## Technical Context

**Language/Version**: Rust 2024 edition (workspace already configured)
**Primary Dependencies**: `tokio` (async runtime — already in workspace), `ratatui`/`crossterm` (already in workspace), `clap` (already in workspace). No new dependencies.
**Storage**: N/A — all data is in-memory mock state
**Testing**: `cargo test` — no new test files required (existing TUI tests cover rendering; preview is a runtime-only binary entry point)
**Target Platform**: Any terminal supporting crossterm alternate screen (same as existing TUI)
**Project Type**: Single Rust workspace — binary crate addition
**Performance Goals**: N/A — preview mode has no performance-sensitive path
**Constraints**: Total new code ≤ 120 lines (NFR-004). Zero new crate dependencies. Zero changes to `crates/kasmos/src/tui/`.
**Scale/Scope**: 1 new file, 1 modified file

## Constitution Check

*Skipped — no `.kittify/memory/constitution.md` found in repository.*

## Project Structure

### Documentation (this feature)

```
kitty-specs/009-standalone-tui-preview/
├── spec.md              # Feature specification (complete)
├── plan.md              # This file
├── research.md          # Phase 0 output (minimal — no unknowns)
├── data-model.md        # Phase 1 output (state model reference)
├── quickstart.md        # Phase 1 output (dev workflow)
└── contracts/
    └── cli-contract.md  # CLI interface contract (extracted from spec)
```

### Source Code (repository root)

```
crates/kasmos/src/
├── main.rs              # MODIFIED: Add `Tui` variant to Commands enum,
│                        #   `mod tui_preview;`, match arm, updated help text
├── tui_preview.rs       # NEW: Mock data generator + animation loop (~80-100 lines)
│                        #   - pub async fn run(count: usize)
│                        #   - fn generate_mock_run(count: usize) -> OrchestrationRun
│                        #   - async fn animation_loop(tx, initial_run)
└── tui/                 # UNCHANGED — no modifications to any file in this directory
    ├── mod.rs
    ├── app.rs
    ├── event.rs
    ├── keybindings.rs
    ├── tabs/
    └── widgets/
```

**Structure Decision**: Binary-only module. `tui_preview.rs` lives in the `main.rs` binary crate scope (declared via `mod tui_preview;`), not exported through `lib.rs`. This keeps the library API surface unchanged and signals that preview mode is a development tool, not a library feature.

## Complexity Tracking

No violations — feature is trivially simple. No complexity justifications needed.
