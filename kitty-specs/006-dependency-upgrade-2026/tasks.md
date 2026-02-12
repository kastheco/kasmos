# Tasks: Dependency Upgrade 2026

**Feature**: 006-dependency-upgrade-2026
**Date**: 2026-02-12
**Total Work Packages**: 8
**Total Subtasks**: 30

## Overview

Each WP upgrades one breaking dependency (or a batch of compatible ones) independently per FR-012. All WPs are independent — no inter-WP dependencies. WP08 (semver bumps) lands last per planning decision.

All WPs are parallelizable (marked `[P]`) since each touches different dependencies and different source files. The only exception is WP08, which should land after all breaking changes to avoid noise.

## Phase 1: High-Risk Breaking Changes (P1)

### WP01 — Upgrade ratatui 0.30 + crossterm 0.29 (TUI Stack)

**Priority**: P1 | **Risk**: Low | **Estimated prompt**: ~300 lines
**User Story**: US1 — Seamless TUI After Ratatui/Crossterm Upgrade
**Prompt file**: `tasks/WP01-upgrade-ratatui-crossterm.md`

**Subtasks**:
- [ ] T001: Update Cargo.toml — bump ratatui to 0.30, crossterm to 0.29
- [ ] T002: Migrate `tui/mod.rs` — verify CrosstermBackend and Terminal compile with new Backend trait
- [ ] T003: Migrate `tui/app.rs` tests — fix TestBackend compatibility (error type Infallible)
- [ ] T004: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Bump versions in Cargo.toml, attempt compile, fix any Backend trait issues in `tui/mod.rs`, fix TestBackend in test code, run full validation suite.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

### WP02 — Upgrade thiserror 2.0 (Error Types)

**Priority**: P1 | **Risk**: Very Low | **Estimated prompt**: ~250 lines
**User Story**: US2 — Error Handling Preserved After Thiserror 2.0
**Prompt file**: `tasks/WP02-upgrade-thiserror.md`

**Subtasks**:
- [ ] T005: Update Cargo.toml — bump thiserror to 2.0
- [ ] T006: Audit all `#[error(...)]` and `#[from]` attributes in `error.rs` and `git.rs`
- [ ] T007: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Version bump in Cargo.toml, compile, audit ~46 error attributes if any fail. Research indicates no changes needed — standard patterns only.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

## Phase 2: Medium-Risk Breaking Changes (P2)

### WP03 — Upgrade toml 0.9 (Config Parsing)

**Priority**: P2 | **Risk**: Very Low | **Estimated prompt**: ~200 lines
**User Story**: US3 — Configuration Loading After TOML Migration
**Prompt file**: `tasks/WP03-upgrade-toml.md`

**Subtasks**:
- [ ] T008: Update Cargo.toml — bump toml to 0.9
- [ ] T009: Verify `config.rs` — confirm `toml::from_str` compiles and parses correctly
- [ ] T010: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Version bump, compile, verify config loading still works. No code changes expected.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

### WP04 — Replace serde_yaml with serde_yml (YAML Parsing)

**Priority**: P2 | **Risk**: Medium | **Estimated prompt**: ~350 lines
**User Story**: US3 — Configuration Loading After YAML Migration
**Prompt file**: `tasks/WP04-replace-serde-yaml.md`

**Subtasks**:
- [ ] T011: Update Cargo.toml — remove serde_yaml, add serde_yml 0.0.12
- [ ] T012: Rename imports in `parser.rs`, `detector.rs`, `list_specs.rs` — serde_yaml → serde_yml
- [ ] T013: Rename error type in `error.rs` — `serde_yaml::Error` → `serde_yml::Error`
- [ ] T014: Verify zero references — `grep -r serde_yaml` returns nothing (SC-005)
- [ ] T015: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Swap crate in Cargo.toml, find-replace all `serde_yaml` → `serde_yml` in source, verify completeness with grep, run validation.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

### WP05 — Upgrade nix 0.31 (Syscall/FIFO)

**Priority**: P2 | **Risk**: Low | **Estimated prompt**: ~300 lines
**User Story**: US4 — File System Operations After Nix Upgrade
**Prompt file**: `tasks/WP05-upgrade-nix.md`

**Subtasks**:
- [ ] T016: Update Cargo.toml — bump nix to 0.31
- [ ] T017: Migrate `cmd.rs` — replace `unsafe { File::from_raw_fd(fd) }` with `File::from(fd)` using OwnedFd
- [ ] T018: Verify `commands.rs` — confirm `mkfifo` compiles unchanged
- [ ] T019: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Bump version, fix the I/O safety migration in `cmd.rs:139-169` (removes `unsafe`), verify `commands.rs` mkfifo is unaffected, run validation.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

### WP06 — Upgrade which 8.0 (Executable Discovery)

**Priority**: P2 | **Risk**: Very Low | **Estimated prompt**: ~200 lines
**User Story**: US4 — File System Operations After Which Upgrade
**Prompt file**: `tasks/WP06-upgrade-which.md`

**Subtasks**:
- [ ] T020: Update Cargo.toml — bump which to 8.0
- [ ] T021: Verify `prompt.rs` — confirm `which::which(binary)` compiles unchanged
- [ ] T022: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Version bump, verify single call site compiles, run validation. No code changes expected.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

### WP07 — Upgrade notify 9.0.0-rc.1 (File Watcher)

**Priority**: P2 | **Risk**: Low-Medium | **Estimated prompt**: ~280 lines
**User Story**: US4 — File System Operations After Notify Upgrade
**Prompt file**: `tasks/WP07-upgrade-notify.md`

**Subtasks**:
- [ ] T023: Update Cargo.toml — bump notify to 9.0.0-rc.1
- [ ] T024: Verify `detector.rs` — confirm watcher creation and event imports compile unchanged
- [ ] T025: Verify `error.rs` — confirm `notify::Error` conversion compiles unchanged
- [ ] T026: Validate — `cargo build && cargo test && cargo clippy` pass clean

**Implementation sketch**: Version bump, verify watcher API and event types in `detector.rs` compile, verify error conversion, run validation. No code changes expected despite crossing 3 major versions.

**Parallel opportunity**: `[P]` — independent of all other WPs.

---

## Phase 3: Low-Risk Housekeeping (P3)

### WP08 — Semver-Compatible Dependency Bumps (Batch)

**Priority**: P3 | **Risk**: Very Low | **Estimated prompt**: ~280 lines
**User Story**: US5 — Compatible Semver Bumps Applied Cleanly
**Prompt file**: `tasks/WP08-semver-bumps.md`
**Note**: Lands LAST — after all breaking changes merged.

**Subtasks**:
- [ ] T027: Update Cargo.toml — bump all 12 semver-compatible dependencies
- [ ] T028: Pin libc explicitly — ensure `libc = "0.2"` prevents 1.0-alpha transitive resolution (FR-010)
- [ ] T029: Refresh Cargo.lock — `cargo update` to resolve all transitive dependencies
- [ ] T030: Validate — `cargo build && cargo test && cargo clippy` pass clean (SC-001, SC-002, SC-003)

**Implementation sketch**: Bulk update all compatible versions in Cargo.toml, ensure libc pinned to 0.2, run cargo update, run full validation.

**Parallel opportunity**: Should land LAST per planning decision. All other WPs can run in parallel before this.

---

## Parallelization Summary

```
WP01 (ratatui+crossterm) ─┐
WP02 (thiserror) ─────────┤
WP03 (toml) ──────────────┤── All independent, run in parallel
WP04 (serde_yml) ─────────┤
WP05 (nix) ───────────────┤
WP06 (which) ─────────────┤
WP07 (notify) ────────────┘
                           │
                           ▼
WP08 (semver bumps) ──────── Lands last (cleanup)
```

## MVP Scope

WP01 + WP02 represent the minimum viable upgrade (P1 items). These address the highest-risk dependencies (TUI stack and error handling) and should be prioritized if resources are limited.
