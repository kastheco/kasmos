---
work_package_id: "WP06"
title: "Upgrade which 8.0"
lane: "planned"
dependencies: []
subtasks: ["T020", "T021", "T022"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP06: Upgrade which 8.0 (Executable Discovery)

**Priority**: P2 | **Risk**: Very Low
**User Story**: US4 — File System Operations After Which Upgrade
**Implements**: FR-007

**Implementation command**:
```bash
spec-kitty implement WP06
```

## Objective

Upgrade which from 5.0 to 8.0. The `which::which()` function signature is stable across all versions. This is a version-bump-only change with zero code modifications expected.

## Context

kasmos uses which in exactly one location:
- `crates/kasmos/src/prompt.rs:278` — `which::which(binary)` to discover executables in PATH

**Key research findings**: Breaking changes in which 6.0→7.0→8.0 only affect `WhichConfig` (gains a generic/lifetime) and internal `Sys` trait. The `which::which()` top-level function is stable and unchanged.

## Subtasks

### T020: Update Cargo.toml

**Purpose**: Bump which version constraint.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Change `which = "5.0"` to `which = "8.0"`

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved with correct version string.

---

### T021: Verify executable discovery

**Purpose**: Confirm `prompt.rs` compiles without changes.

**Steps**:
1. Run `cargo check` — should pass with no errors
2. Verify `prompt.rs:278` — `which::which(binary).map_err(...)` compiles unchanged
3. Check that the `which::Error` type is still compatible with the `.map_err()` closure

**Files**: `crates/kasmos/src/prompt.rs`

**Validation**: `cargo check` passes. No changes needed in `prompt.rs`.

---

### T022: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors
2. `cargo test` — must pass with zero new failures
3. `cargo clippy` — must produce zero new warnings

**Validation**: All three commands pass clean.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `which = "8.0"`
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes
- [ ] `cargo clippy` clean
- [ ] No source code changes needed

## Risks

- **Negligible risk**. The `which::which()` function is the most stable API in the crate. Three major versions crossed with zero signature changes.

## Reviewer Guidance

1. Verify only Cargo.toml changed
2. If any source changes were made, scrutinize — they indicate unexpected breakage
3. Quick spot-check: ensure `prompt.rs` still compiles the `which::which(binary)` call
