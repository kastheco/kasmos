---
work_package_id: "WP03"
title: "Upgrade toml 0.9"
lane: "done"
dependencies: []
subtasks: ["T008", "T009", "T010"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP03: Upgrade toml 0.9 (Config Parsing)

**Priority**: P2 | **Risk**: Very Low
**User Story**: US3 — Configuration Loading After TOML Migration
**Implements**: FR-004

**Implementation command**:
```bash
spec-kitty implement WP03
```

## Objective

Upgrade toml from 0.8 to 0.9. Configuration loading must continue to work identically. This is expected to be a version-bump-only change — kasmos uses only the high-level `toml::from_str` API which is unchanged.

## Context

kasmos uses toml in exactly one location:
- `crates/kasmos/src/config.rs:172` — `toml::from_str(&content)` to parse configuration files

**Key research findings**: toml 0.9 breaking changes affect:
1. `Serializer::new`/`pretty` — NOT used by kasmos (no custom serialization)
2. `impl FromStr for Value` — NOT used by kasmos (uses `toml::from_str::<T>()`)
3. `Deserializer::new` deprecated — NOT used by kasmos (uses `toml::from_str`)

The high-level `toml::from_str::<T>()` deserialization API is unchanged.

## Subtasks

### T008: Update Cargo.toml

**Purpose**: Bump toml version constraint.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Change `toml = "0.8"` to `toml = "0.9"`

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved with correct version string.

---

### T009: Verify config parsing

**Purpose**: Confirm `config.rs` compiles and parses correctly.

**Steps**:
1. Run `cargo check` — should pass with no errors
2. Inspect `config.rs:172` — the call `toml::from_str(&content)` should compile unchanged
3. If there are existing config parsing tests, verify they pass
4. FR-013 requires no changes to existing config files — verify the parse output is identical

**Files**: `crates/kasmos/src/config.rs`

**Validation**: `cargo check` passes. Config parsing code compiles without modification.

---

### T010: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors
2. `cargo test` — must pass with zero new failures
3. `cargo clippy` — must produce zero new warnings

**Validation**: All three commands pass clean.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `toml = "0.9"`
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes
- [ ] `cargo clippy` clean
- [ ] No config file format changes required (FR-013)

## Risks

- **Negligible risk**. The high-level API is stable. The only realistic failure is if toml 0.9 changes default serialization behavior, but kasmos only deserializes (reads), never serializes (writes) TOML config.

## Reviewer Guidance

1. Verify only Cargo.toml changed — no source code changes expected
2. If `config.rs` was modified, verify the change is necessary (not a workaround)
3. Confirm config files do not need any modifications
