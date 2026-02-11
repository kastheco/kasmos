# Feature Specification: Dependency Upgrade 2026

**Feature Branch**: `006-dependency-upgrade-2026`
**Created**: 2026-02-11
**Status**: Draft
**Base**: `master` (after features 002 and 005 merge)
**Input**: Upgrade all kasmos dependencies to latest versions, migrating through breaking changes. Replace the deprecated `serde_yaml` crate with `serde_yml`.

## Clarifications

### Session 2026-02-11

- Q: For `serde_yaml` (deprecated), migrate to `serde_yml`, switch configs to TOML, or keep as-is? → A: Migrate to `serde_yml` (drop-in replacement, same API surface).
- Q: `notify` latest is `9.0.0-rc.1` (release candidate). Target `8.x` stable instead? → A: Go with `9.0.0-rc.1`.

## Upgrade Manifest

The following dependencies require breaking-change migration:

| Dependency | Current | Target | Migration Scope |
|---|---|---|---|
| ratatui | 0.29 | 0.30.0 | Widget API changes, style/layout refactors |
| crossterm | 0.28 | 0.29.0 | Event types, terminal API changes |
| thiserror | 1.0 | 2.0 | Derive macro syntax changes |
| toml | 0.8 | 0.9 | Parser/serializer API changes |
| nix | 0.29 | 0.31 | Syscall wrapper changes |
| which | 5.0 | 8.0 | Path resolution API changes |
| notify | 6.1 | 9.0.0-rc.1 | Watcher API, event model changes |
| serde_yaml | 0.9 (deprecated) | serde_yml (replacement) | Crate rename, import path changes |

The following dependencies are semver-compatible and only need version bumps (no code changes expected):

| Dependency | Current | Target |
|---|---|---|
| serde | 1.0 | 1.0.228 |
| serde_json | 1.0 | 1.0.149 |
| anyhow | 1.0 | 1.0.101 |
| tracing | 0.1 | 0.1.44 |
| tracing-subscriber | 0.3 | 0.3.22 |
| tokio | 1.0 | 1.49.0 |
| async-trait | 0.1 | 0.1.89 |
| clap | 4.0 | 4.5.58 |
| chrono | 0.4 | 0.4.43 |
| futures-util | 0.3 | 0.3.31 |
| libc | 0.2 | 0.2 (latest stable, not 1.0-alpha) |
| tempfile (dev) | 3.0 | 3.25.0 |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Seamless TUI After Ratatui/Crossterm Upgrade (Priority: P1)

An operator launches kasmos after the ratatui 0.30 and crossterm 0.29 upgrades and uses the TUI exactly as before — tab switching, kanban navigation, action buttons, confirmation dialogs, log viewer, and notification bar all work identically. No visual regressions, no input handling changes, no layout breakage.

**Why this priority**: ratatui and crossterm are the rendering foundation. Any breakage here affects every TUI feature. This is the highest-risk upgrade.

**Independent Test**: Can be tested by running `cargo test`, launching kasmos with a test orchestration, and exercising every TUI interaction (tab switch, WP selection, action dispatch, popup confirm/cancel, log scroll, resize).

**Acceptance Scenarios**:

1. **Given** the ratatui/crossterm upgrade is applied, **When** the operator launches kasmos, **Then** the TUI renders identically to the pre-upgrade version with no visual artifacts.
2. **Given** the operator uses keyboard navigation (hjkl, 1/2/3, Enter, q), **When** keys are pressed, **Then** all keybindings respond correctly with no input lag or missed events.
3. **Given** the operator resizes the terminal, **When** the resize event fires, **Then** the layout reflows correctly without panics or rendering corruption.
4. **Given** the operator triggers a confirmation dialog, **When** the popup renders, **Then** it centers correctly and responds to y/n/Esc.
5. **Given** the full test suite exists, **When** `cargo test` is run after the upgrade, **Then** all tests pass with zero new failures.

---

### User Story 2 - Error Handling Preserved After Thiserror 2.0 Migration (Priority: P1)

All custom error types in kasmos continue to produce clear, actionable error messages after migrating from thiserror 1.0 to 2.0. Error propagation chains, Display implementations, and From conversions work identically.

**Why this priority**: thiserror is used across the entire codebase for error type definitions. A broken migration would surface as cryptic runtime errors.

**Independent Test**: Can be tested by triggering known error conditions (missing config file, invalid WP state, FIFO parse error) and verifying error messages match pre-upgrade output.

**Acceptance Scenarios**:

1. **Given** the thiserror 2.0 migration is applied, **When** `cargo build` runs, **Then** all error type derivations compile without modification or with minimal syntax adjustments.
2. **Given** an error condition occurs at runtime, **When** the error propagates, **Then** the error message and chain are identical to pre-upgrade behavior.
3. **Given** error types use `#[from]` attributes, **When** conversion occurs, **Then** From implementations work correctly.

---

### User Story 3 - Configuration Loading After TOML and YAML Migrations (Priority: P2)

Kasmos loads all configuration files (TOML and YAML formats) correctly after upgrading toml 0.8→0.9 and replacing serde_yaml with serde_yml. Existing config files do not need modification.

**Why this priority**: Config loading is an early-boot operation. If it breaks, kasmos won't start at all.

**Independent Test**: Can be tested by loading known-good config files and verifying all fields parse correctly, and by introducing intentional config errors and verifying error messages are clear.

**Acceptance Scenarios**:

1. **Given** existing TOML configuration files, **When** kasmos loads them with toml 0.9, **Then** all fields parse correctly with no changes to the config files.
2. **Given** existing YAML configuration files, **When** kasmos loads them with serde_yml, **Then** all fields parse correctly with no changes to the config files.
3. **Given** a malformed config file, **When** kasmos attempts to load it, **Then** a clear error message is produced (not a raw deserialization panic).
4. **Given** the serde_yaml→serde_yml migration is complete, **When** searching the codebase, **Then** no references to `serde_yaml` remain in source code or Cargo.toml.

---

### User Story 4 - File System Operations After Nix, Which, and Notify Upgrades (Priority: P2)

Kasmos correctly detects executables in PATH (zellij, opencode, etc.), monitors filesystem changes for agent signal files, and performs FIFO operations after upgrading nix, which, and notify.

**Why this priority**: These crates underpin executable discovery, file watching (agent completion/input-needed signals), and FIFO pipe operations. Breakage would disable orchestration.

**Independent Test**: Can be tested by verifying `which` finds executables, `notify` detects file creation/modification in watched directories, and nix FIFO operations succeed.

**Acceptance Scenarios**:

1. **Given** the which 8.0 upgrade is applied, **When** kasmos checks for `zellij` in PATH, **Then** the executable is found correctly.
2. **Given** the notify 9.0 upgrade is applied, **When** an agent writes a completion signal file, **Then** kasmos detects the file event within 1 second.
3. **Given** the nix 0.31 upgrade is applied, **When** kasmos creates and reads from FIFO pipes, **Then** pipe operations work correctly with no errors.
4. **Given** all three upgrades are applied, **When** a full orchestration run executes, **Then** agent lifecycle (launch, monitor, detect completion) works end-to-end.

---

### User Story 5 - Compatible Semver Bumps Applied Cleanly (Priority: P3)

All semver-compatible dependencies are bumped to their latest patch/minor versions. The project builds and all tests pass. This is a low-risk housekeeping operation.

**Why this priority**: No code changes expected, but ensures the project picks up security fixes and performance improvements in transitive dependencies.

**Independent Test**: Can be tested by running `cargo update` for compatible crates, then `cargo build && cargo test && cargo clippy`.

**Acceptance Scenarios**:

1. **Given** all compatible dependencies are bumped, **When** `cargo build` runs, **Then** the project compiles with zero errors.
2. **Given** all compatible dependencies are bumped, **When** `cargo test` runs, **Then** all tests pass.
3. **Given** all compatible dependencies are bumped, **When** `cargo clippy` runs, **Then** zero new warnings are introduced.

---

### Edge Cases

- What if ratatui 0.30 changes the `Widget` trait signature? All custom widget implementations must be updated.
- What if crossterm 0.29 changes the `Event` enum variants? All event matching code must be updated.
- What if thiserror 2.0 changes the derive macro attribute syntax? All `#[error(...)]` attributes must be audited.
- What if toml 0.9 changes serialization defaults (e.g., inline table formatting)? Config round-trip tests should verify.
- What if serde_yml has subtle parsing differences from serde_yaml? Edge cases in YAML config files (anchors, multi-line strings) must be tested.
- What if notify 9.0 changes the event model (e.g., different debounce behavior)? File watcher integration tests must verify signal detection timing.
- What if which 8.0 changes error types for "not found"? Error handling in executable discovery must be updated.
- What if libc 1.0-alpha is pulled as a transitive dependency? Pin libc to 0.2.x explicitly to avoid alpha instability.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST upgrade ratatui from 0.29 to 0.30 and migrate all TUI code through any breaking API changes.
- **FR-002**: System MUST upgrade crossterm from 0.28 to 0.29 and migrate all terminal/event handling code through any breaking changes.
- **FR-003**: System MUST upgrade thiserror from 1.0 to 2.0 and update all error type derivations to the new macro syntax.
- **FR-004**: System MUST upgrade toml from 0.8 to 0.9 and ensure all configuration parsing/serialization works correctly.
- **FR-005**: System MUST replace serde_yaml 0.9 (deprecated) with serde_yml and update all import paths and API calls.
- **FR-006**: System MUST upgrade nix from 0.29 to 0.31 and migrate all syscall/FIFO code through breaking changes.
- **FR-007**: System MUST upgrade which from 5.0 to 8.0 and migrate all executable discovery code through breaking changes.
- **FR-008**: System MUST upgrade notify from 6.1 to 9.0.0-rc.1 and migrate all file watcher code through breaking changes.
- **FR-009**: System MUST bump all semver-compatible dependencies to their latest versions (serde, serde_json, anyhow, tracing, tracing-subscriber, tokio, async-trait, clap, chrono, futures-util, tempfile).
- **FR-010**: System MUST pin libc to 0.2.x stable (not 1.0-alpha) to avoid alpha instability in transitive dependencies.
- **FR-011**: System MUST preserve all existing functionality — zero behavioral regressions after upgrades.
- **FR-012**: Each breaking upgrade MUST be independently mergeable so that if one migration is problematic, others can still land.
- **FR-013**: System MUST NOT require changes to existing configuration files — all config formats remain backward-compatible.

### Key Entities

- **Cargo.toml**: The dependency manifest files (workspace root and crate-level) that define version constraints.
- **Migration**: A set of code changes required to adapt to a breaking API change in an upgraded dependency.
- **Semver Bump**: A version constraint update for a compatible dependency that requires no code changes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: `cargo build` succeeds with zero errors after all upgrades are applied.
- **SC-002**: `cargo test` passes with zero new test failures after all upgrades are applied.
- **SC-003**: `cargo clippy` produces zero new warnings after all upgrades are applied.
- **SC-004**: All 8 breaking dependency upgrades are completed and merged.
- **SC-005**: The deprecated `serde_yaml` crate is fully removed — zero references remain in source code or Cargo manifests.
- **SC-006**: Each breaking upgrade can be reverted via a single `git revert` of its merge commit without breaking other upgrades.
- **SC-007**: No existing configuration files require modification after upgrades.
- **SC-008**: kasmos launches and completes a full orchestration run end-to-end after all upgrades.

## Assumptions

- Features 002 and 005 are fully merged to `master` before this feature begins implementation.
- `serde_yml` is API-compatible with `serde_yaml` 0.9 (drop-in replacement with import path changes only).
- `notify` 9.0.0-rc.1 is stable enough for production use despite the release-candidate label.
- `libc` 1.0-alpha will not be forced as a transitive dependency by any of the upgraded crates; if it is, explicit pinning to 0.2.x resolves it.
- The ratatui 0.29→0.30 migration guide (if available) documents all breaking changes.
- The crossterm 0.28→0.29 upgrade is coordinated with ratatui 0.30 (ratatui typically re-exports crossterm types).
