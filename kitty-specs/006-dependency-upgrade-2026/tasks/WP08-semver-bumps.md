---
work_package_id: "WP08"
title: "Semver-compatible dependency bumps"
lane: "planned"
dependencies: []
subtasks: ["T027", "T028", "T029", "T030"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP08: Semver-Compatible Dependency Bumps (Batch)

**Priority**: P3 | **Risk**: Very Low
**User Story**: US5 — Compatible Semver Bumps Applied Cleanly
**Implements**: FR-009, FR-010
**Note**: This WP lands LAST — after all breaking changes are merged.

**Implementation command**:
```bash
spec-kitty implement WP08
```

## Objective

Bump all 12 semver-compatible dependencies to their latest versions. Pin libc to 0.2.x to prevent 1.0-alpha transitive resolution. No code changes expected — this is a pure Cargo.toml + Cargo.lock operation.

## Context

These dependencies follow semver and should not introduce breaking changes:

| Dependency | Current | Target | Section |
|---|---|---|---|
| serde | 1.0 | 1.0.228 | [dependencies] |
| serde_json | 1.0 | 1.0.149 | [dependencies] |
| anyhow | 1.0 | 1.0.101 | [dependencies] |
| tracing | 0.1 | 0.1.44 | [dependencies] |
| tracing-subscriber | 0.3 | 0.3.22 | [dependencies] |
| tokio | 1.0 | 1.49.0 | [dependencies] |
| async-trait | 0.1 | 0.1.89 | [dependencies] |
| clap | 4.0 | 4.5.58 | [dependencies] |
| chrono | 0.4 | 0.4.43 | [dependencies] |
| futures-util | 0.3 | 0.3.31 | [dependencies] |
| libc | 0.2 | 0.2 (pinned) | [dependencies] |
| tempfile | 3.0 | 3.25.0 | [dev-dependencies] |

## Subtasks

### T027: Update Cargo.toml with all version bumps

**Purpose**: Bump all semver-compatible dependencies to their target versions.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Update each dependency line:
   - `serde = { version = "1.0.228", features = ["derive"] }` (was `"1.0"`)
   - `serde_json = "1.0.149"` (was `"1.0"`)
   - `anyhow = "1.0.101"` (was `"1.0"`)
   - `tracing = "0.1.44"` (was `"0.1"`)
   - `tracing-subscriber = { version = "0.3.22", features = ["env-filter", "fmt"] }` (was `"0.3"`)
   - `tokio = { version = "1.49.0", features = ["full"] }` (was `"1.0"`)
   - `async-trait = "0.1.89"` (was `"0.1"`)
   - `clap = { version = "4.5.58", features = ["derive"] }` (was `"4.0"`)
   - `chrono = { version = "0.4.43", features = ["serde"] }` (was `"0.4"`)
   - `futures-util = "0.3.31"` (was `"0.3"`)
   - `tempfile = "3.25.0"` (was `"3.0"`, in `[dev-dependencies]`)
3. Preserve all existing feature flags — do not add or remove features.

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: All version strings updated. Feature flags preserved.

---

### T028: Pin libc to 0.2.x (FR-010)

**Purpose**: Prevent libc 1.0-alpha from being pulled as a transitive dependency.

**Steps**:
1. In `crates/kasmos/Cargo.toml`, verify `libc = "0.2"` is present
2. This constraint already prevents Cargo from resolving to libc 1.0-alpha. The existing `"0.2"` constraint is sufficient.
3. After `cargo update`, verify in Cargo.lock that `libc` resolved to a 0.2.x version (not 1.0.x):
   ```bash
   grep -A1 'name = "libc"' Cargo.lock
   ```

**Files**: `crates/kasmos/Cargo.toml`, `Cargo.lock` (verified)

**Validation**: `libc` in Cargo.lock is 0.2.x, not 1.0-alpha.

---

### T029: Refresh Cargo.lock

**Purpose**: Resolve all transitive dependencies with the new version constraints.

**Steps**:
1. Run `cargo update` to refresh the lock file
2. Verify the lock file resolves cleanly (no conflicts)
3. Check for any unexpected major version bumps in transitive dependencies
4. Specifically verify:
   - No `libc` 1.0-alpha in the dependency tree
   - No incompatible transitive dependency versions

**Files**: `Cargo.lock`

**Validation**: `cargo update` succeeds. Lock file is clean.

---

### T030: Full validation

**Purpose**: Run the complete validation suite (SC-001, SC-002, SC-003).

**Steps**:
1. `cargo build` — must succeed with zero errors (SC-001)
2. `cargo test` — must pass with zero new failures (SC-002)
3. `cargo clippy` — must produce zero new warnings (SC-003)
4. If any warnings appear from new clippy lints (introduced in newer dependency versions), fix them or document why they're acceptable

**Validation**: All three commands pass clean.

## Definition of Done

- [ ] All 12 dependencies bumped to target versions in Cargo.toml
- [ ] `libc` constrained to `"0.2"` (FR-010)
- [ ] `cargo update` resolves cleanly
- [ ] Cargo.lock has no libc 1.0-alpha
- [ ] `cargo build` succeeds (SC-001)
- [ ] `cargo test` passes (SC-002)
- [ ] `cargo clippy` clean (SC-003)
- [ ] No source code changes needed

## Risks

- **Transitive dependency conflicts**: Unlikely with semver-compatible bumps, but possible if two dependencies pin incompatible transitive versions. Mitigation: `cargo update` will surface these immediately.
- **New clippy lints**: Newer dependency versions may trigger new clippy lints. Mitigation: Fix the lints — they're usually improvements.
- **libc 1.0-alpha leaking in**: Some upgraded crate might require libc >= 1.0. Mitigation: The explicit `libc = "0.2"` pin will cause a resolution failure, making the conflict visible immediately.

## Reviewer Guidance

1. Verify all 12 dependencies are bumped to correct target versions
2. Verify feature flags are preserved (not added/removed)
3. Check Cargo.lock for libc version (must be 0.2.x)
4. Confirm zero source code changes — this should be Cargo.toml + Cargo.lock only
5. If any source changes were needed, investigate why (semver violation in upstream crate)
