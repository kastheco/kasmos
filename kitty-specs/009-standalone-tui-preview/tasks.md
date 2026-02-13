# Work Packages: Standalone TUI Preview Mode

**Inputs**: Design documents from `kitty-specs/009-standalone-tui-preview/`
**Prerequisites**: plan.md (required), spec.md (user stories), research.md, data-model.md, contracts/, quickstart.md

**Tests**: Not explicitly requested. Existing `cargo test` must pass with no regressions (AC-09).

**Organization**: Fine-grained subtasks (`Txxx`) roll up into work packages (`WPxx`). This feature is small enough for a single work package.

---

## Work Package WP01: Add kasmos tui Subcommand with Animated Mock Data (Priority: P0)

**Goal**: Add `kasmos tui [--count N]` subcommand that launches the TUI with deterministic animated mock data, no external dependencies.
**Independent Test**: `cargo build -p kasmos` succeeds; `cargo run -p kasmos -- tui` launches TUI with animated WPs; `cargo run -p kasmos -- tui --count 3` works with 3 WPs; pressing `q` exits cleanly; `cargo test -p kasmos` passes.
**Prompt**: `kitty-specs/009-standalone-tui-preview/tasks/WP01-add-tui-preview-subcommand.md`
**Estimated Prompt Size**: ~350 lines

### Included Subtasks
- [x] T001 Add `mod tui_preview;` declaration and `Tui` variant to `Commands` enum in `crates/kasmos/src/main.rs`
- [ ] T002 Add match arm for `Commands::Tui { count }` in `main()` calling `tui_preview::run(count)`
- [ ] T003 Update `after_help` text in `crates/kasmos/src/main.rs` to document `kasmos tui`
- [ ] T004 Create `crates/kasmos/src/tui_preview.rs` with `pub async fn run(count: usize)` entry point
- [ ] T005 Implement `fn generate_mock_run(count: usize) -> OrchestrationRun` in `crates/kasmos/src/tui_preview.rs`
- [ ] T006 Implement `async fn animation_loop(...)` in `crates/kasmos/src/tui_preview.rs`

### Implementation Notes
- T001-T003 modify `crates/kasmos/src/main.rs` (~15 lines added)
- T004-T006 create `crates/kasmos/src/tui_preview.rs` (~80-100 lines)
- Total new code must stay under ~120 lines (NFR-004)
- Use `clap::value_parser!(usize).range(1..)` for `--count` validation (FR-002)
- Animation is deterministic: round-robin WP selection, `tick_count % 7 == 0` for ~15% failure path
- Drop `mpsc::Receiver` immediately after channel creation (research.md decision)
- Use `let _ = watch_tx.send(...)` to silently handle send errors (risk mitigation)

### Parallel Opportunities
- None internal to this WP (single-file changes are sequential)
- This WP has no dependencies on other features and can execute immediately

### Dependencies
- None (standalone feature, no upstream WPs)

### Risks & Mitigations
- `tui_logger::init_logger` double-call: TUI's `run()` handles this; preview module does NOT call it separately
- `watch_tx.send()` failure on TUI exit: Use `let _ =` to ignore; animation task exits cleanly when channel closes
- `mpsc::Receiver` drop: TUI keybindings use `let _ = try_send(...)` -- already handles closed channel

---

## Dependency & Execution Summary

- **Sequence**: WP01 is the only work package -- no sequencing needed
- **Parallelization**: N/A (single WP)
- **MVP Scope**: WP01 is the entire feature

---

## Subtask Index (Reference)

| Subtask ID | Summary | Work Package | Priority | Parallel? |
|------------|---------|--------------|----------|-----------|
| T001 | Add `mod tui_preview` and `Tui` variant to Commands enum | WP01 | P0 | No |
| T002 | Add match arm for `Commands::Tui` in `main()` | WP01 | P0 | No |
| T003 | Update `after_help` text with `kasmos tui` | WP01 | P0 | No |
| T004 | Create `tui_preview.rs` with `run()` entry point | WP01 | P0 | No |
| T005 | Implement `generate_mock_run()` | WP01 | P0 | No |
| T006 | Implement `animation_loop()` | WP01 | P0 | No |
