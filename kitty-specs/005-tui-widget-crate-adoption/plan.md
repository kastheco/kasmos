# Implementation Plan: TUI Widget Crate Adoption

**Branch**: `005-tui-widget-crate-adoption` | **Date**: 2026-02-12 | **Spec**: `kitty-specs/005-tui-widget-crate-adoption/spec.md`
**Input**: Feature specification from `kitty-specs/005-tui-widget-crate-adoption/spec.md`

## Summary

Adopt 5 community ratatui crates (`ratatui-macros`, `tui-logger`, `tui-popup`, `throbber-widgets-tui`, `tui-nodes`) into the kasmos TUI, replacing hand-rolled log viewer, confirmation dialogs, and status indicators with maintained community widgets, adding macro-based layout syntax, and introducing a WP dependency graph visualization. Each adoption is independently mergeable and revertible.

## Technical Context

**Language/Version**: Rust (stable, 2024 edition)
**Primary Dependencies**: ratatui 0.30 (post feature 006 merge), crossterm 0.29, tokio, tracing/tracing-subscriber
**New Dependencies**:
  - `tui-logger` 0.18.x (features: `tracing-support`)
  - `tui-popup` 0.7.x
  - `throbber-widgets-tui` 0.10.x
  - `ratatui-macros` 0.7.x
  - `tui-nodes` 0.10.x
**Storage**: N/A (in-memory TUI state only)
**Testing**: `cargo test` (unit tests with `ratatui::backend::TestBackend`), `cargo clippy`
**Target Platform**: Linux (primary), macOS (best-effort)
**Project Type**: Rust workspace — single crate `crates/kasmos/`
**Performance Goals**: TUI render loop stays non-blocking; key handling < 25ms (existing benchmark); spinner animation at 250ms tick
**Constraints**: All 5 latest crate versions require ratatui 0.30 — feature 006 (dependency upgrade) must merge first
**Scale/Scope**: ~5 TUI source files affected + new `tabs/` module; 5 independently-mergeable adoption WPs + 1 UX polish WP

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **Rust 2024 edition**: ✅ All crates are compatible
- **tokio async runtime**: ✅ No crate introduces blocking I/O in the render path
- **ratatui for TUI**: ✅ All crates are ratatui widgets/extensions
- **cargo test required**: ✅ Each adoption includes regression tests (SC-005)
- **TUI must remain responsive / never block render loop**: ✅ All crates are widget-level rendering (no network, no disk I/O in hot path). tui-logger uses internal circular buffer with `move_events()` for hot-path decoupling.
- **Minimize unnecessary allocations in hot paths**: ✅ ratatui-macros is compile-time only; ThrobberState is a single integer counter; tui-logger manages its own buffer pool
- **Linux primary, macOS best-effort**: ✅ No platform-specific concerns in any adopted crate
- **Single binary distribution**: ✅ All crates are pure Rust library dependencies
- **Zellij as terminal multiplexer**: N/A — No changes to Zellij API or pane orchestration
- **just command runner**: N/A — No justfile changes
- **Concurrent pane support**: ✅ No changes to pane orchestration logic; widget adoptions are rendering-only
- **Async must not starve event loop**: ✅ `tui_logger::move_events()` is O(buffer) and called once per 250ms tick — negligible. All other crates are synchronous widget renderers.

**Post-Phase 1 re-check**: The tracing integration (conditional `Registry` with layered approach) adds no new constitution concerns. The `init_logging()` refactor from `fmt().init()` to `Registry + Layer` composition is a standard tracing-subscriber pattern.

## Project Structure

### Documentation (this feature)

```
kitty-specs/005-tui-widget-crate-adoption/
├── plan.md              # This file
├── research.md          # Crate API research and integration patterns
├── data-model.md        # Entity/state changes per adoption
├── quickstart.md        # Build/test/verify instructions
├── spec.md              # Feature specification
├── meta.json            # Feature metadata
├── checklists/          # Spec-kitty checklists
└── tasks/               # WP task files (created by /spec-kitty.tasks)
```

### Source Code (repository root)

```
crates/kasmos/
├── Cargo.toml                      # +5 new dependencies (pinned exact versions)
└── src/
    ├── logging.rs                   # Refactored: fmt().init() → layered Registry
    │                                #   Headless: Registry + fmt layer
    │                                #   TUI mode: Registry + TuiTracingSubscriberLayer
    ├── tui/
    │   ├── mod.rs                   # Updated: tui_logger::init_logger() call added
    │   │                            #   + registers tabs/ and widgets/ modules
    │   ├── app.rs                   # Major changes:
    │   │                            #   - LogsState/LogEntry/LogLevel removed
    │   │                            #   - TuiWidgetState added (tui-logger)
    │   │                            #   - ThrobberState added (shared, single instance)
    │   │                            #   - DashboardState.view_mode enum added (Kanban|Graph)
    │   │                            #   - Popup state (Option<ConfirmAction>) for dialogs
    │   │                            #   - show_help, detail_wp_id, notification_cycle_index (WP06)
    │   │                            #   - Macro syntax throughout (line![], span![], etc.)
    │   │                            #   - Rendering extracted to tabs/ module (WP06)
    │   ├── keybindings.rs           # Updated:
    │   │                            #   - 'v' key toggles Dashboard view mode
    │   │                            #   - tui-logger widget events forwarded in Logs tab
    │   │                            #   - Popup y/n/Esc handling routed through tui-popup
    │   │                            #   - '?' help overlay toggle, Enter WP detail (WP06)
    │   │                            #   - Notification cycling (WP06)
    │   ├── event.rs                 # Minimal changes (event types unchanged)
    │   ├── tabs/                    # NEW module (WP06 — extracted from app.rs)
    │   │   ├── mod.rs               # Re-exports tab rendering functions
    │   │   ├── dashboard.rs         # render_dashboard(), progress summary, responsive layout
    │   │   ├── review.rs            # render_review()
    │   │   └── logs.rs              # render_logs() (thin TuiLoggerSmartWidget wrapper)
    │   └── widgets/                 # NEW module (optional, for graph adapter)
    │       └── dependency_graph.rs  # tui-nodes adapter: builds NodeGraph from OrchestrationRun
    └── tests/                       # Existing tests updated; new tests per adoption
```

**Structure Decision**: Existing `crates/kasmos/src/tui/` module hierarchy is retained. A new `widgets/` submodule may be added for the dependency graph adapter if the tui-nodes integration is non-trivial enough to warrant its own file. A new `tabs/` submodule (WP06) extracts per-tab rendering from `app.rs` to keep the file focused on state and lifecycle. All other changes are in-place modifications to existing files.

## Architecture Decisions

### AD-1: Conditional Tracing Subscriber (Option B)

Refactor `logging.rs` from `tracing_subscriber::fmt().init()` to a layered `tracing_subscriber::Registry` approach:
- **Headless mode** (non-TUI commands): `Registry` + `fmt::Layer` (stderr output, same as today)
- **TUI mode** (`run` command): `Registry` + `TuiTracingSubscriberLayer` (feeds tui-logger widget)

`init_logging()` gains a `tui_mode: bool` parameter (or enum). The caller (`main.rs` or `tui::run()`) decides which mode based on the subcommand. tui-logger's `init_logger()` must be called before the subscriber is set.

**Rationale**: fmt output to stderr is useless during TUI (alternate screen swallows it). File logging adds unnecessary scope for a developer tool. Conditional layering gives clean separation while keeping the Registry composable for future layers.

### AD-2: ratatui-macros First (Option A)

Adopt `ratatui-macros` as the first WP so all subsequent crate integration code uses macro syntax from day one. This establishes the idiom early and avoids a second-pass migration of newly written code.

### AD-3: Single Shared ThrobberState

Use one `ThrobberState` instance in `DashboardState` (not per-WP). All Active WPs render the same animation frame, producing synchronized spinners as required by US3-AC3. The shared state is ticked in `App::on_tick()` (every 250ms).

### AD-4: Dashboard View Mode Toggle

Add `DashboardViewMode` enum (`Kanban`, `DependencyGraph`) to `DashboardState`. The `v` key toggles between modes. When in `DependencyGraph` mode, the graph replaces the entire Dashboard body area. Lane navigation keys (`j/k/h/l`) are inactive in graph mode.

### AD-5: All Crates Target ratatui 0.30

Research confirmed all 5 latest crate versions require ratatui 0.30 (not just tui-nodes as originally assumed). Since feature 006 lands first, all adoptions use latest versions. No version pinning to older ratatui-0.29-compatible releases needed. Spec assumption updated.

### AD-6: tui-logger Keybinding Delegation

The tui-logger `TuiLoggerSmartWidget` has its own key commands (h, f, UP/DOWN, LEFT/RIGHT, -/+, PageUp/PageDown, Esc, Space). In the Logs tab, keys not consumed by global handlers are translated to `TuiWidgetEvent` variants and forwarded via `TuiWidgetState::transition()`. The existing custom filter (`/` key) is removed — tui-logger's target selector replaces it.

## WP Sequencing

```
Feature 002 merges → Feature 006 merges → Feature 005 begins
                                           │
                                           ├── WP01: ratatui-macros (foundation, no deps)
                                           │
                                           ├── WP02: tui-logger (depends on WP01 for macro syntax)
                                           ├── WP03: tui-popup (depends on WP01 for macro syntax)
                                           ├── WP04: throbber-widgets-tui (depends on WP01 for macro syntax)
                                           │         (WP02–WP04 are independent of each other, parallel-capable)
                                           │
                                           ├── WP05: tui-nodes (depends on WP01; largest scope)
                                           │
                                           └── WP06: UX polish (depends on WP01–WP05; uses all adopted crates)
```

WP01–WP05 are independently mergeable per FR-015. WP02–WP05 logically depend on WP01 (macros) being merged first so new code uses macro idioms. WP02–WP04 have no dependency on each other. WP05 is the largest scope (new visualization, new module). WP06 depends on all prior WPs — it is a comprehensive UX polish pass that leverages every adopted crate (tui-popup for help/detail overlays, macros for all new layout code, tui-logger for the logs tab extraction, throbber state for dashboard rendering, tui-nodes graph view toggle).

**Practical note**: WP02–WP04 are logically independent but all modify `crates/kasmos/src/tui/app.rs` and `crates/kasmos/src/tui/keybindings.rs` (different sections). Sequential merging is recommended to avoid merge conflicts even though the features don't depend on each other. WP06 should be the final WP merged — its module extraction (T030) absorbs all prior changes into the new `tabs/` structure.

## Complexity Tracking

No constitution violations. No complexity justifications needed.
