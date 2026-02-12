---
work_package_id: "WP07"
title: "Upgrade notify 9.0.0-rc.1"
lane: "done"
dependencies: []
subtasks: ["T023", "T024", "T025", "T026"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP07: Upgrade notify 9.0.0-rc.1 (File Watcher)

**Priority**: P2 | **Risk**: Low-Medium (RC release)
**User Story**: US4 — File System Operations After Notify Upgrade
**Implements**: FR-008

**Implementation command**:
```bash
spec-kitty implement WP07
```

## Objective

Upgrade notify from 6.1 to 9.0.0-rc.1. Despite crossing three major versions (6→7→8→9), kasmos's usage of the stable watcher API means zero code changes are expected. The spec accepted the RC status in the clarification session (2026-02-11).

## Context

kasmos uses notify in two files:
- `crates/kasmos/src/detector.rs:11-12` — imports `Watcher`, `RecommendedWatcher`, `RecursiveMode`, `Event`, `EventKind`, `ModifyKind`, `DataChange`
- `crates/kasmos/src/detector.rs:133` — creates watcher: `notify::recommended_watcher(move |res: notify::Result<Event>| { ... })`
- `crates/kasmos/src/error.rs:162-163` — manual `From<notify::Error>` impl for `DetectorError`

**Key research findings**:
- `crossbeam` feature renamed to `crossbeam-channel` in v7 — kasmos doesn't use it
- `Watcher::paths_mut` removed in v9 — kasmos doesn't use it
- Event types (`Event`, `EventKind`, `ModifyKind`, `DataChange`) — stable across all versions
- `recommended_watcher()` creation pattern — stable across all versions
- `notify::Error` — stable type
- MSRV bumped to 1.85 — kasmos uses latest stable, no issue

## Subtasks

### T023: Update Cargo.toml

**Purpose**: Bump notify version constraint.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Change `notify = "6.1"` to `notify = "9.0.0-rc.1"`

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved with correct version string.

---

### T024: Verify watcher API compatibility

**Purpose**: Confirm `detector.rs` compiles without changes.

**Steps**:
1. Run `cargo check` — should pass with no errors
2. Verify imports at `detector.rs:11-12`:
   ```rust
   use notify::{Watcher, RecommendedWatcher, RecursiveMode, Event, EventKind};
   use notify::event::{ModifyKind, DataChange};
   ```
   All these types must still exist and be importable.
3. Verify watcher creation at `detector.rs:133`:
   ```rust
   let mut watcher = notify::recommended_watcher(move |res: notify::Result<Event>| { ... });
   ```
   The `recommended_watcher()` function and its callback signature must be unchanged.
4. If any compilation errors:
   - Check if event types moved to a different module path
   - Check if `recommended_watcher` changed to require a `Config` parameter
   - Consult notify changelog for the specific version that introduced the break

**Files**: `crates/kasmos/src/detector.rs`

**Validation**: `cargo check` passes. No changes needed in `detector.rs`.

---

### T025: Verify error type compatibility

**Purpose**: Confirm `notify::Error` conversion still works.

**Steps**:
1. Check `error.rs:162-163` — manual `From<notify::Error>` impl:
   ```rust
   impl From<notify::Error> for DetectorError {
       fn from(e: notify::Error) -> Self {
           DetectorError::WatcherError(e.to_string())
       }
   }
   ```
2. Verify `notify::Error` still implements `Display` (needed for `.to_string()`)
3. Run `cargo check` to confirm the impl compiles

**Files**: `crates/kasmos/src/error.rs`

**Validation**: `cargo check` passes. Error conversion unchanged.

---

### T026: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors
2. `cargo test` — must pass with zero new failures
3. `cargo clippy` — must produce zero new warnings
4. If there are integration tests that exercise file watching, verify they pass. The spec notes that file watcher should detect signal files within 1 second.

**Validation**: All three commands pass clean.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `notify = "9.0.0-rc.1"`
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes
- [ ] `cargo clippy` clean
- [ ] No source code changes needed
- [ ] File watcher functionality preserved

## Risks

- **RC stability**: notify 9.0.0-rc.1 is a release candidate. It may have undiscovered bugs. Mitigation: kasmos uses stable, well-tested API surface (watcher creation and basic events). If issues arise, fallback to `notify = "8.0"` (latest stable).
- **macOS backend change**: notify 9.0 replaced the macOS FSEvents backend. This could affect macOS users. Mitigation: kasmos targets Linux primarily; macOS is best-effort.

## Reviewer Guidance

1. Verify only Cargo.toml changed — no source code changes expected
2. Pay attention to notify version string format: `"9.0.0-rc.1"` (not `"9.0"`)
3. If source changes were needed, check whether they indicate an RC regression
4. Consider whether an integration test for file watching would be valuable
