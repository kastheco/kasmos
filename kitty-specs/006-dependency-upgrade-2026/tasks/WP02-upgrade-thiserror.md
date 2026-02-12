---
work_package_id: "WP02"
title: "Upgrade thiserror 2.0"
lane: "planned"
dependencies: []
subtasks: ["T005", "T006", "T007"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP02: Upgrade thiserror 2.0 (Error Types)

**Priority**: P1 | **Risk**: Very Low
**User Story**: US2 — Error Handling Preserved After Thiserror 2.0
**Implements**: FR-003

**Implementation command**:
```bash
spec-kitty implement WP02
```

## Objective

Upgrade thiserror from 1.0 to 2.0. All custom error types must continue to compile and produce identical error messages. This is expected to be a version-bump-only change — kasmos uses standard error patterns that are unaffected by thiserror 2.0 breaking changes.

## Context

kasmos defines error types in two files:
- `crates/kasmos/src/error.rs` — ~40 `#[error(...)]` attributes across 8 error enums (KasmosError, ConfigError, ZellijError, SpecParserError, LayoutError, DetectorError, StateError, PaneError, WaveError)
- `crates/kasmos/src/git.rs` — 6 `#[error(...)]` attributes in GitError enum

**Key research findings**: thiserror 2.0 breaking changes are:
1. `{r#field}` syntax removed — NOT used in kasmos
2. Mixed positional + tuple-index format args — NOT used in kasmos (all named fields)
3. Trait bound inference changes for shadowed fields — NOT applicable

All kasmos error types use standard patterns: `#[error("message {field}")]`, `#[error(transparent)]`, `#[from]`. These are 100% compatible with thiserror 2.0.

## Subtasks

### T005: Update Cargo.toml

**Purpose**: Bump thiserror version constraint.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Change `thiserror = "1.0"` to `thiserror = "2.0"`

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved with correct version string.

---

### T006: Audit error attributes

**Purpose**: Verify all `#[error(...)]` and `#[from]` attributes compile unchanged.

**Steps**:
1. Run `cargo check` — if it compiles, no further action needed
2. If any compile errors:
   - Check for `{r#field}` patterns (unlikely) → replace with `{field}`
   - Check for mixed `{0}` + positional args in tuple variants (unlikely) → use named args
3. Verify error Display output is preserved:
   - `error.rs` tests (`test_config_error_display`, `test_zellij_error_display`, `test_state_error_display`, `test_kasmos_error_from_config`, `test_kasmos_error_from_state`) must all pass
4. Scan `error.rs` and `git.rs` for any patterns that might be affected:
   - All `#[from]` attributes (unchanged in thiserror 2.0)
   - All `#[error(transparent)]` attributes (unchanged)
   - Named field interpolation like `{path}`, `{name}`, `{field}` (unchanged)

**Files**: `crates/kasmos/src/error.rs`, `crates/kasmos/src/git.rs`

**Validation**: `cargo check` passes. `cargo test` passes for error module tests.

---

### T007: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors
2. `cargo test` — must pass with zero new failures (especially `error::tests`)
3. `cargo clippy` — must produce zero new warnings

**Validation**: All three commands pass clean.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `thiserror = "2.0"`
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes (especially error display tests)
- [ ] `cargo clippy` clean
- [ ] Error messages are identical to pre-upgrade (verified by existing tests)

## Risks

- **Extremely low risk**. All kasmos error patterns are standard and fully compatible with thiserror 2.0. The only realistic failure mode is a transitive dependency conflict, which would surface immediately at `cargo check`.

## Reviewer Guidance

1. Verify only Cargo.toml version changed — no source code changes should be needed
2. If source changes were made, scrutinize them — they indicate an unexpected breaking change
3. Confirm error tests still pass and error messages contain expected text
