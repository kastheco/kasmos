---
work_package_id: "WP01"
title: "Upgrade ratatui 0.30 + crossterm 0.29"
lane: "done"
dependencies: []
subtasks: ["T001", "T002", "T003", "T004"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP01: Upgrade ratatui 0.30 + crossterm 0.29 (TUI Stack)

**Priority**: P1 | **Risk**: Low
**User Story**: US1 — Seamless TUI After Ratatui/Crossterm Upgrade
**Implements**: FR-001, FR-002

**Implementation command**:
```bash
spec-kitty implement WP01
```

## Objective

Upgrade ratatui from 0.29 to 0.30 and crossterm from 0.28 to 0.29 as an atomic unit. Ratatui 0.30 bundles crossterm 0.29 by default. After this WP, the entire TUI renders and behaves identically to the pre-upgrade state.

## Context

kasmos uses ratatui with CrosstermBackend for its terminal UI. The TUI has 4 modules:
- `crates/kasmos/src/tui/mod.rs` — Terminal setup/teardown, main event loop
- `crates/kasmos/src/tui/app.rs` — App state and rendering (~690 lines)
- `crates/kasmos/src/tui/event.rs` — EventStream wrapper
- `crates/kasmos/src/tui/keybindings.rs` — Key dispatch

**Key research findings**: kasmos does NOT use any of the removed/changed APIs:
- No `block::Title` (removed) — kasmos uses `Block::default().title("string")` pattern
- No `WidgetRef` (removed) — not used
- No crossterm↔ratatui color conversions (From/Into removed) — not used
- No `Flex::SpaceAround` (semantics changed) — not used
- No `Marker` matching (now #[non_exhaustive]) — not used

The main areas to check:
1. `Backend` trait now has associated `Error` type — kasmos uses concrete `CrosstermBackend<Stdout>`, likely unaffected
2. `TestBackend` error type changed to `Infallible` — used in `tui/app.rs` tests

## Subtasks

### T001: Update Cargo.toml

**Purpose**: Bump ratatui and crossterm version constraints.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Change `ratatui = { version = "0.29", features = ["crossterm"] }` to `ratatui = { version = "0.30", features = ["crossterm"] }`
3. Change `crossterm = { version = "0.28", features = ["event-stream"] }` to `crossterm = { version = "0.29", features = ["event-stream"] }`

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved with correct version strings.

---

### T002: Verify TUI module compatibility

**Purpose**: Ensure `tui/mod.rs` compiles with the new Backend trait.

**Steps**:
1. Run `cargo check` to identify any compilation errors
2. Check `tui/mod.rs:33` — `setup_terminal()` returns `Terminal<CrosstermBackend<Stdout>>`. Since this uses a concrete type (not generic `B: Backend`), the associated `Error` type change should be transparent.
3. Check `tui/mod.rs:42` — `restore_terminal()` takes `&mut Terminal<CrosstermBackend<Stdout>>` — same reasoning, concrete type.
4. Check `tui/mod.rs:94` — `terminal.draw(|frame| app.render(frame))` — Frame type may have changed generics. Verify it compiles.
5. If any compilation errors arise in `tui/mod.rs`, `tui/event.rs`, or `tui/keybindings.rs`, fix them following the ratatui 0.30 migration patterns documented in `research.md`.

**Key imports to verify compile** (all should be stable):
- `ratatui::Terminal` — stable
- `ratatui::backend::CrosstermBackend` — stable
- `crossterm::event::{DisableMouseCapture, EnableMouseCapture}` — stable
- `crossterm::execute!` — stable
- `crossterm::terminal::{EnterAlternateScreen, LeaveAlternateScreen, disable_raw_mode, enable_raw_mode}` — stable
- `crossterm::event::{Event as CrosstermEvent, EventStream, KeyEvent, MouseEvent}` — stable
- `crossterm::event::{KeyCode, KeyEvent, KeyEventKind}` — stable

**Files**: `crates/kasmos/src/tui/mod.rs`, `crates/kasmos/src/tui/event.rs`, `crates/kasmos/src/tui/keybindings.rs`

**Validation**: `cargo check` passes with zero errors in tui modules.

---

### T003: Fix TestBackend compatibility in tests

**Purpose**: Ensure `tui/app.rs` test code compiles with new TestBackend.

**Steps**:
1. Check `tui/app.rs:679` — `TestBackend::new(120, 40)` constructor. Verify it still exists and has the same signature.
2. Check `tui/app.rs:680` — `Terminal::new(backend)` with TestBackend. The TestBackend error type changed from `io::Error` to `Infallible`. If tests use `?` operator with `io::Result`, this may cause a type mismatch.
3. If the `Terminal::new(backend)` call no longer returns `io::Result`, update the test function accordingly:
   - The `.expect("create terminal")` pattern on line 680 should work regardless since both `io::Error` and `Infallible` implement `Debug`.
4. Check `tui/app.rs:685` — `terminal.backend_mut().resize(80, 20)` — verify `resize` method still exists on TestBackend.
5. Run `cargo test` to verify all 3 test functions in `tui/app.rs::tests` pass:
   - `test_review_policy_mode_selection_and_auto_mark_done_path`
   - `test_review_failure_surfaces_notification_and_log_entry`
   - `test_resize_reflow_render_does_not_panic`

**Files**: `crates/kasmos/src/tui/app.rs` (test section, lines 573-689+)

**Validation**: `cargo test` passes with zero failures in tui::app::tests.

---

### T004: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors (SC-001)
2. `cargo test` — must pass with zero new failures (SC-002)
3. `cargo clippy` — must produce zero new warnings (SC-003)
4. Verify no regressions in imports — all `use ratatui::` and `use crossterm::` statements resolve correctly

**Validation**: All three commands pass clean. No warnings, no errors.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `ratatui = "0.30"` and `crossterm = "0.29"`
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes (all tests including TUI tests)
- [ ] `cargo clippy` clean
- [ ] No functional regressions in TUI rendering or input handling

## Risks

- **TestBackend type mismatch**: If `Terminal::new(TestBackend::new(...))` changes return type, tests need adjustment. Mitigation: The `.expect()` pattern used in tests should handle both error types.
- **Frame generic changes**: If `Frame` gains new generic parameters, the `app.render(frame)` signature may need updating. Mitigation: Research shows Frame API is stable in 0.30.

## Reviewer Guidance

1. Verify Cargo.toml has correct version constraints
2. Check that no new `#[allow(...)]` or suppression attributes were added to work around breaking changes
3. Confirm TUI tests still exercise rendering (not skipped or emptied)
4. If any API migration was needed beyond version bump, verify it follows ratatui 0.30 patterns (not workarounds)
