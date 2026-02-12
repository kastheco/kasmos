# Work Packages: TUI Widget Crate Adoption

**Inputs**: Design documents from `kitty-specs/005-tui-widget-crate-adoption/`
**Prerequisites**: plan.md (required), spec.md (user stories), research.md, data-model.md, quickstart.md

**Tests**: Not explicitly requested. Each WP validates via `cargo build`, `cargo test`, `cargo clippy` (SC-005, SC-006).

**Organization**: 39 fine-grained subtasks (`T001`–`T039`) roll up into 6 work packages (`WP01`–`WP06`). WP01–WP05 each adopt one crate and are independently mergeable and revertible (FR-015, SC-009). WP06 is a UX polish pass that depends on all prior WPs.

**Prompt Files**: Each work package references a matching prompt file in `kitty-specs/005-tui-widget-crate-adoption/tasks/`.

---

## Work Package WP01: Adopt ratatui-macros — Layout & Text Macro Migration (Priority: P4)

**Goal**: Add `ratatui-macros` 0.7.x and migrate all verbose `Layout::default().direction(...).constraints(...)` and `Line::from(vec![Span::...])` patterns across TUI source files to concise macro syntax (`vertical![]`, `horizontal![]`, `line![]`, `span![]`). This establishes the macro idiom for all subsequent WPs.
**Independent Test**: `cargo build -p kasmos && cargo test -p kasmos && cargo clippy -p kasmos -- -D warnings` pass with zero regressions. No remaining verbose layout patterns in `crates/kasmos/src/tui/`.
**Prompt**: `kitty-specs/005-tui-widget-crate-adoption/tasks/WP01-ratatui-macros.md`
**Estimated prompt size**: ~350 lines

### Included Subtasks
- [x] T001 Add `ratatui-macros` 0.7.x dependency to `crates/kasmos/Cargo.toml`
- [x] T002 Migrate Layout construction patterns in `crates/kasmos/src/tui/app.rs` to `vertical![]`/`horizontal![]` macros
- [x] T003 Migrate Line/Span text construction patterns in `crates/kasmos/src/tui/app.rs` to `line![]`/`span![]` macros
- [x] T004 Update imports — add `ratatui_macros` use statements, remove newly-unused ratatui imports
- [x] T005 Verify `cargo build`, `cargo test`, `cargo clippy` pass with zero new warnings

### Implementation Notes
- AD-2 (plan.md): ratatui-macros is adopted first so all subsequent WP code uses macro syntax from the start.
- R-6 (research.md): Provides exact before/after migration patterns for layouts, text, and tabs.
- Focus exclusively on `crates/kasmos/src/tui/app.rs` — the only TUI file with layout/text construction.
- Do NOT modify `keybindings.rs`, `event.rs`, or `mod.rs` — they contain no layout or text construction code.

### Parallel Opportunities
- T002 (layouts) and T003 (text) can be done concurrently since they touch different code patterns within the same file.
- T004 (import cleanup) follows naturally after T002+T003.

### Dependencies
- None (starting package). Implementation command: `spec-kitty implement WP01`

### Risks & Mitigations
- Macro expansion conflicts with existing imports: Caught by `cargo clippy` (SC-006).
- Behavioral regressions from subtle layout differences: Existing render tests (test_resize_reflow, test_dashboard_renders_wps) catch any visual regressions.

---

## Work Package WP02: Adopt tui-logger — Replace Hand-Rolled Log Viewer (Priority: P1) 🎯 MVP

**Goal**: Replace the entire hand-rolled log system (`LogEntry`, `LogLevel`, `LogsState`, `render_logs()`, 10k-entry cap, filter mechanism) with `tui-logger`'s `TuiLoggerSmartWidget` integrated via `TuiTracingSubscriberLayer`. Refactor `logging.rs` to a conditional Registry-based subscriber (TUI mode vs headless mode).
**Independent Test**: Launch kasmos with `--tui`, open Logs tab (press `3`), verify events appear with per-target grouping. Toggle target visibility with `h`, enter page mode with `PageUp`.
**Prompt**: `kitty-specs/005-tui-widget-crate-adoption/tasks/WP02-tui-logger.md`
**Estimated prompt size**: ~550 lines

### Included Subtasks
- [x] T006 Add `tui-logger` 0.18.x (features: `tracing-support`) to `crates/kasmos/Cargo.toml`
- [x] T007 Refactor `crates/kasmos/src/logging.rs` — conditional `Registry`-based subscriber with TUI vs headless modes
- [x] T008 Wire up TUI-mode logging in `crates/kasmos/src/tui/mod.rs` and `crates/kasmos/src/main.rs`
- [x] T009 Remove hand-rolled log types and state from `crates/kasmos/src/tui/app.rs`
- [x] T010 Add `TuiLoggerSmartWidget` rendering in the Logs tab
- [x] T011 Replace `handle_logs_key()` in `crates/kasmos/src/tui/keybindings.rs` with tui-logger key event delegation
- [x] T012 Update all tests referencing removed log types and state

### Implementation Notes
- AD-1 (plan.md): Conditional subscriber — `init_logging()` gains a `tui_mode: bool` parameter.
- AD-6 (plan.md): tui-logger has built-in key commands; delegate unhandled keys via `TuiWidgetState::transition()`.
- R-2 (research.md): `tui_logger::init_logger()` MUST be called before subscriber init. `move_events()` called in `on_tick()`.
- R-3 (research.md): Full keybinding table for tui-logger widget events.
- data-model.md: LogEntry, LogLevel, LogsState removed. `logger_state: TuiWidgetState` added to App.
- SC-007: Must reduce Logs tab code by ≥100 LOC.
- `record_review_failure()` switches from pushing to `self.logs.entries` to `tracing::error!()`.
- `capture_state_logs()` switches from pushing to `self.logs.entries` to `tracing::info!/warn!/error!()`.

### Parallel Opportunities
- T007 (logging.rs refactor) and T009 (remove old types from app.rs) can start concurrently — different files.
- T011 (keybindings) is independent of T010 (rendering) — different files.

### Dependencies
- Depends on WP01 (macro syntax established). Implementation command: `spec-kitty implement WP02 --base WP01`

### Risks & Mitigations
- Tracing subscriber can only be initialized once per process: `init_logging()` must be called exactly once. Tests that call it must be gated.
- `tui_logger::init_logger()` call ordering: Must precede subscriber init — enforced by code placement in `tui::run()`.
- Test suite references `app.logs.entries`: All such tests must be updated or removed (T012).

---

## Work Package WP03: Adopt tui-popup — Confirmation Dialog Widget (Priority: P2)

**Goal**: Add `tui-popup` and introduce a `ConfirmAction` enum + confirmation dialog flow for destructive actions (Force-Advance, Abort). Popup renders centered with auto-sizing, styled borders, and title text. Dismissable with `y`/`n`/`Esc`.
**Independent Test**: Navigate to a Failed WP in Dashboard, press `F` for Force-Advance, verify centered popup appears. Press `n` to dismiss, verify no action dispatched. Press `F` again then `y` to confirm.
**Prompt**: `kitty-specs/005-tui-widget-crate-adoption/tasks/WP03-tui-popup.md`
**Estimated prompt size**: ~400 lines

### Included Subtasks
- [x] T013 Add `tui-popup` 0.7.x dependency to `crates/kasmos/Cargo.toml`
- [x] T014 Add `ConfirmAction` enum with `title()` and `description()` methods to `crates/kasmos/src/tui/app.rs`
- [x] T015 Add `pending_confirm: Option<ConfirmAction>` to `App` struct and `App::new()`
- [x] T016 Add popup rendering overlay in `App::render()` using `tui_popup::Popup`
- [x] T017 Update `crates/kasmos/src/tui/keybindings.rs` — route destructive actions through confirmation flow, add popup-active key interception

### Implementation Notes
- R-4 (research.md): `Popup::new(content).title(title).style(style)` auto-centers in `frame.area()`.
- data-model.md: ConfirmAction lifecycle — None → Some (action triggered) → None (y/n/Esc).
- Currently `F` key in keybindings.rs dispatches `EngineAction::ForceAdvance` immediately — must be changed to set `pending_confirm` instead.
- Popup key interception (`y`/`n`/`Esc`) must be checked at the TOP of `handle_key()`, before global and tab-specific handlers.
- Edge case: Popup must clamp to terminal dimensions (tui-popup handles this automatically via `frame.area()`).

### Parallel Opportunities
- T014+T015 (state additions) can be done together.
- T016 (rendering) and T017 (keybindings) are in different files — parallel-capable.

### Dependencies
- Depends on WP01 (macro syntax). Implementation command: `spec-kitty implement WP03 --base WP01`

### Risks & Mitigations
- Existing tests for `F` key expect immediate `ForceAdvance` dispatch: Tests must be updated to account for the confirmation step.
- Terminal overflow: Action descriptions kept to 1-2 lines to avoid truncation.

---

## Work Package WP04: Adopt throbber-widgets-tui — Animated Activity Indicators (Priority: P3)

**Goal**: Add animated spinners to Active work packages in the Dashboard using `throbber-widgets-tui`. A single shared `ThrobberState` is ticked every 250ms, producing synchronized spinners across all Active WPs. Non-Active WPs retain static state badges.
**Independent Test**: Launch kasmos with Active WPs, verify spinners animate on each tick. Transition a WP to Paused — verify spinner replaced by static badge.
**Prompt**: `kitty-specs/005-tui-widget-crate-adoption/tasks/WP04-throbber-widgets.md`
**Estimated prompt size**: ~350 lines

### Included Subtasks
- [x] T018 Add `throbber-widgets-tui` 0.10.x dependency to `crates/kasmos/Cargo.toml`
- [x] T019 Add `throbber_state: ThrobberState` to `DashboardState` and initialize in `Default` impl
- [x] T020 Tick `throbber_state.calc_next()` in `App::on_tick()`
- [x] T021 Update `render_dashboard()` — render `Throbber` widget for Active WPs, static `Span` badge for non-Active WPs
- [x] T022 Update dashboard rendering tests to account for throbber state

### Implementation Notes
- AD-3 (plan.md): Single shared ThrobberState — all Active WPs show same animation frame (US3-AC3 synchronized).
- R-5 (research.md): Use `BRAILLE_SIX` or `DOT` throbber set. `frame.render_stateful_widget(throbber, area, &mut state)`.
- data-model.md: ThrobberState ticked unconditionally every 250ms regardless of active tab.
- SC-004: Animation must update at least once per second (250ms tick × 4 frames minimum per cycle).
- The throbber widget needs a small area (1-2 cells wide) prepended to the WP item in the lane list.

### Parallel Opportunities
- T019 (state addition) and T020 (tick wiring) are small and sequential.
- T021 (render changes) is the bulk of the work.

### Dependencies
- Depends on WP01 (macro syntax). Implementation command: `spec-kitty implement WP04 --base WP01`

### Risks & Mitigations
- `render_stateful_widget` requires `&mut ThrobberState` — the `render_dashboard` method takes `&self`. Solution: Use `Throbber::default().throbber_set(set).use_type(throbber_widgets_tui::WhichUse::Spin)` and calculate the current frame from `throbber_state` manually, or change render to take `&mut self` if architecturally acceptable.
- Tick alignment: 250ms tick with BRAILLE_SIX (6 frames) = 1.5s per rotation — acceptable per spec.

---

## Work Package WP05: Adopt tui-nodes — WP Dependency Graph Visualization (Priority: P2)

**Goal**: Add a WP dependency graph visualization to the Dashboard tab using `tui-nodes`. Introduce `DashboardViewMode` enum to toggle between kanban and graph views via `v` key. Graph shows WPs as state-colored nodes with directed dependency edges.
**Independent Test**: Load a feature with known WP dependencies, press `v` to activate graph view, verify all nodes and edges render correctly. Press `v` again to return to kanban. Verify node colors change on state transitions.
**Prompt**: `kitty-specs/005-tui-widget-crate-adoption/tasks/WP05-tui-nodes.md`
**Estimated prompt size**: ~500 lines

### Included Subtasks
- [x] T023 Add `tui-nodes` 0.10.x dependency to `crates/kasmos/Cargo.toml`
- [x] T024 Add `DashboardViewMode` enum and `view_mode` field to `DashboardState` in `crates/kasmos/src/tui/app.rs`
- [x] T025 Create `crates/kasmos/src/tui/widgets/mod.rs` and `crates/kasmos/src/tui/widgets/dependency_graph.rs` with graph builder
- [x] T026 [P] Implement cycle detection for WP dependency graphs before rendering
- [x] T027 Update `render_dashboard()` to dispatch between kanban and graph views based on `view_mode`
- [x] T028 Update `crates/kasmos/src/tui/keybindings.rs` — `v` key toggles view mode; disable lane nav keys in graph mode
- [x] T029 Register `widgets` module in `crates/kasmos/src/tui/mod.rs`

### Implementation Notes
- AD-4 (plan.md): DashboardViewMode enum (Kanban, DependencyGraph). `v` key toggle. Lane nav keys inactive in graph mode.
- R-7 (research.md): tui-nodes API — `NodeGraph::new()`, `NodeLayout`, `Connection`. Very low docs coverage (6.67%) — reference crate examples/source.
- data-model.md: Graph derived from `OrchestrationRun` on each render frame (no caching needed — WP count small).
- Edge cases: Cycle detection (render warning banner), 20+ WPs (rely on tui-nodes layout + scrolling).
- SC-008: Must correctly render all nodes and edges for a feature with ≥10 WPs.
- `state_to_style()` mapping: Pending→DarkGray, Active→Yellow, Completed→Green, Failed→Red, ForReview→Magenta, Paused→Blue.

### Parallel Opportunities
- T025 (new widget module) and T024 (state additions to app.rs) are in different files — parallel.
- T026 (cycle detection) is a standalone utility function — parallel with T025.
- T027 (render dispatch) and T028 (keybindings) are in different files — parallel.

### Dependencies
- Depends on WP01 (macro syntax). Implementation command: `spec-kitty implement WP05 --base WP01`

### Risks & Mitigations
- tui-nodes low documentation: Implementers should read crate examples and source directly. API is small (4 types).
- Large graph rendering: For 20+ WPs, rely on tui-nodes' built-in layout. Add scroll offset if graph exceeds viewport.
- tui-nodes API may differ from research assumptions: The crate has very low docs coverage. Implementer should verify the API by examining `tui-nodes 0.10.0` source on crates.io or GitHub before starting.

---

## Work Package WP06: TUI UX Polish — Help Overlay, Status Footer, Dashboard Enhancements, and Responsive Layout (Priority: P2)

**Goal**: Comprehensive UI/UX polish pass on the kasmos TUI. Extract rendering into a `tabs/` module, add a persistent status footer with run state and progress, add a `?` help overlay, implement dashboard lane scrolling (using the existing unused `scroll_offsets`), add WP detail popups on `Enter`, add a progress summary bar, implement responsive column layout for narrow terminals, show failure count badges, and enable notification cycling.
**Independent Test**: Launch kasmos TUI, verify status footer visible on all tabs. Press `?` for help overlay. Navigate a long WP list with `j`/`k` and verify scrolling. Press `Enter` on a WP for detail popup. Resize terminal and verify responsive column layout.
**Prompt**: `kitty-specs/005-tui-widget-crate-adoption/tasks/WP06-tui-ux-polish.md`
**Estimated prompt size**: ~700 lines

### Included Subtasks
- [ ] T030 Extract tab rendering into `tabs/` module (`dashboard.rs`, `review.rs`, `logs.rs`)
- [ ] T031 Add persistent status footer (run state, elapsed time, WP counts, wave progress)
- [ ] T032 Add `?` key help overlay with tab-contextual keybindings via tui-popup
- [ ] T033 Implement dashboard lane scrolling using existing `scroll_offsets` field
- [ ] T034 Add WP detail popup on `Enter` key showing all WP fields
- [ ] T035 Add progress summary bar above kanban lanes (gauge + status counts)
- [ ] T036 Implement responsive column layout (4/2/1 columns based on terminal width)
- [ ] T037 Show failure count badges on WPs with `failure_count > 0`
- [ ] T038 Implement notification cycling (`n` key advances through notification list)
- [ ] T039 Update tests and verify build/clippy pass

### Implementation Notes
- T030 (module extraction) must be done first within this WP — all subsequent subtasks add code to the new `tabs/` files.
- Uses tui-popup (WP03) for help overlay and WP detail popup — consistent overlay behavior.
- Uses ratatui-macros (WP01) for all new layout and text construction.
- Status footer uses `on_tick()` (previously an empty placeholder) to refresh elapsed time.
- Overlay priority order: help > confirmation > detail > normal rendering. Key interception follows the same order.
- `DashboardState.scroll_offsets` is currently dead code — this WP brings it to life.
- `NotificationKind::InputNeeded` remains unused — not in scope for this WP.

### Parallel Opportunities
- T031 (status footer) and T032 (help overlay) are independent — different rendering areas and key handling.
- T033 (lane scrolling) and T035 (progress summary) are independent within the dashboard.
- T034 (WP detail popup) is independent of T036 (responsive layout).
- T037 (failure badges) and T038 (notification cycling) are independent.
- T030 (module extraction) must precede T033, T035, T036 (they modify `tabs/dashboard.rs`).

### Dependencies
- Depends on WP01 (macro syntax), WP02 (tui-logger — logs tab rendering), WP03 (tui-popup — overlay rendering), WP04 (throbber — dashboard spinners), WP05 (tui-nodes — graph view toggle). Implementation command: `spec-kitty implement WP06 --base WP05`

### Risks & Mitigations
- Module extraction merge conflicts: T030 is a pure refactor — do it first to absorb prior WP changes before adding new code.
- Responsive layout boundary cases: Test with `TestBackend` at widths 59, 60, 99, 100, 120.
- Overlay stacking: Enforce single-active-overlay rule via key interception priority.

---

## Dependency & Execution Summary

```
WP01 (ratatui-macros) ← Foundation, no dependencies
  ├── WP02 (tui-logger)         ← depends on WP01
  ├── WP03 (tui-popup)          ← depends on WP01
  ├── WP04 (throbber-widgets)   ← depends on WP01
  ├── WP05 (tui-nodes)          ← depends on WP01
  └── WP06 (UX polish)          ← depends on WP01–WP05
```

- **Sequence**: WP01 first → WP02–WP05 can proceed in parallel after WP01 merges → WP06 after all others merge.
- **Practical note**: WP02–WP04 all modify `app.rs` and `keybindings.rs`. Sequential merging recommended to avoid merge conflicts (see plan.md).
- **Recommended merge order**: WP01 → WP02 → WP03 → WP04 → WP05 → WP06 (by priority P4→P1→P2→P3→P2→P2).
- **MVP Scope**: WP01 + WP02 (macro foundation + highest-value log viewer replacement, SC-007).
- **Parallelization**: After WP01 merges, WP02–WP05 are all independent. Assign to separate agents for maximum throughput. WP06 must wait for all prior WPs.

---

## Subtask Index (Reference)

| Subtask | Summary | WP | Priority | Parallel? |
|---------|---------|-----|----------|-----------|
| T001 | Add ratatui-macros dependency | WP01 | P4 | No |
| T002 | Migrate Layout patterns to macros | WP01 | P4 | Yes |
| T003 | Migrate Line/Span patterns to macros | WP01 | P4 | Yes |
| T004 | Update imports, remove unused | WP01 | P4 | No |
| T005 | Verify build/test/clippy pass | WP01 | P4 | No |
| T006 | Add tui-logger dependency | WP02 | P1 | No |
| T007 | Refactor logging.rs conditional subscriber | WP02 | P1 | Yes |
| T008 | Wire TUI-mode logging in mod.rs + main.rs | WP02 | P1 | No |
| T009 | Remove hand-rolled log types from app.rs | WP02 | P1 | Yes |
| T010 | Add TuiLoggerSmartWidget rendering | WP02 | P1 | No |
| T011 | Replace handle_logs_key with tui-logger delegation | WP02 | P1 | Yes |
| T012 | Update tests for removed log types | WP02 | P1 | No |
| T013 | Add tui-popup dependency | WP03 | P2 | No |
| T014 | Add ConfirmAction enum to app.rs | WP03 | P2 | No |
| T015 | Add pending_confirm to App struct | WP03 | P2 | No |
| T016 | Add Popup rendering overlay in render() | WP03 | P2 | Yes |
| T017 | Route destructive actions through confirm flow | WP03 | P2 | Yes |
| T018 | Add throbber-widgets-tui dependency | WP04 | P3 | No |
| T019 | Add ThrobberState to DashboardState | WP04 | P3 | No |
| T020 | Tick ThrobberState in on_tick() | WP04 | P3 | No |
| T021 | Render Throbber for Active WPs in dashboard | WP04 | P3 | No |
| T022 | Update dashboard rendering tests | WP04 | P3 | No |
| T023 | Add tui-nodes dependency | WP05 | P2 | No |
| T024 | Add DashboardViewMode enum + view_mode field | WP05 | P2 | Yes |
| T025 | Create widgets/ module with dependency_graph.rs | WP05 | P2 | Yes |
| T026 | Implement cycle detection for dependency graphs | WP05 | P2 | Yes |
| T027 | Update render_dashboard() for graph view dispatch | WP05 | P2 | No |
| T028 | Add v-key toggle + disable nav in graph mode | WP05 | P2 | Yes |
| T029 | Register widgets module in tui/mod.rs | WP05 | P2 | No |
| T030 | Extract tab rendering into tabs/ module | WP06 | P2 | No |
| T031 | Add persistent status footer | WP06 | P2 | Yes |
| T032 | Add ? key help overlay via tui-popup | WP06 | P2 | Yes |
| T033 | Implement dashboard lane scrolling | WP06 | P2 | Yes |
| T034 | Add WP detail popup on Enter key | WP06 | P2 | Yes |
| T035 | Add progress summary bar above kanban | WP06 | P2 | Yes |
| T036 | Implement responsive column layout | WP06 | P2 | Yes |
| T037 | Show failure count badges on WPs | WP06 | P2 | Yes |
| T038 | Implement notification cycling | WP06 | P2 | Yes |
| T039 | Update tests and verify build/clippy | WP06 | P2 | No |
