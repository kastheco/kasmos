---
work_package_id: "WP04"
title: "Replace serde_yaml with serde_yml"
lane: "planned"
dependencies: []
subtasks: ["T011", "T012", "T013", "T014", "T015"]
history:
  - date: "2026-02-12"
    action: "created"
    by: "spec-kitty.tasks"
---

# WP04: Replace serde_yaml with serde_yml (YAML Parsing)

**Priority**: P2 | **Risk**: Medium (upstream maintenance)
**User Story**: US3 — Configuration Loading After YAML Migration
**Implements**: FR-005

**Implementation command**:
```bash
spec-kitty implement WP04
```

## Objective

Replace the deprecated `serde_yaml` crate (archived Jan 2024) with `serde_yml` (community replacement). All YAML frontmatter parsing must continue to work identically. The migration is a crate swap + find-replace of import paths across 4 source files.

## Context

kasmos uses serde_yaml for parsing YAML frontmatter in spec/task files. There are exactly 4 call sites and 1 Cargo.toml entry:

1. `crates/kasmos/src/parser.rs:118` — `serde_yaml::from_str(yaml_str)` parses WP frontmatter
2. `crates/kasmos/src/detector.rs:202` — `serde_yaml::from_str(parts[1].trim())` parses lane check data
3. `crates/kasmos/src/list_specs.rs:137` — `serde_yaml::from_str(&body[..end])` parses WP frontmatter
4. `crates/kasmos/src/error.rs:127` — `#[from] serde_yaml::Error` in SpecParserError enum

**Important note**: Both `serde_yaml` and `serde_yml` are archived/unmaintained. This migration is per the spec clarification decision (2026-02-11). The API surface is compatible — `serde_yml` provides the same `from_str`, `to_string`, `Value`, `Error` types.

## Subtasks

### T011: Update Cargo.toml

**Purpose**: Swap serde_yaml for serde_yml in dependency manifest.

**Steps**:
1. Open `crates/kasmos/Cargo.toml`
2. Remove the line: `serde_yaml = "0.9"`
3. Add in alphabetical position: `serde_yml = "0.0.12"`
4. Verify the new entry is syntactically correct

**Files**: `crates/kasmos/Cargo.toml`

**Validation**: File saved, `serde_yaml` removed, `serde_yml` added.

---

### T012: Rename imports in YAML parsing files

**Purpose**: Replace all `serde_yaml` references with `serde_yml` in source code.

**Steps**:
1. **`parser.rs:118`**: Change `serde_yaml::from_str(yaml_str)` to `serde_yml::from_str(yaml_str)`
   - Full context: This is inside `parse_frontmatter()` function. The `.map_err()` chain after it remains unchanged.

2. **`detector.rs:202`**: Change `serde_yaml::from_str(parts[1].trim())` to `serde_yml::from_str(parts[1].trim())`
   - Full context: This is inside a function that parses lane check YAML. The `.map_err()` chain remains unchanged.

3. **`list_specs.rs:137`**: Change `serde_yaml::from_str(&body[..end])` to `serde_yml::from_str(&body[..end])`
   - Full context: Inside `extract_lane()` function. The `.ok()?` chain remains unchanged.

**Files**: `crates/kasmos/src/parser.rs`, `crates/kasmos/src/detector.rs`, `crates/kasmos/src/list_specs.rs`

**Validation**: All three files have `serde_yml::from_str` instead of `serde_yaml::from_str`.

---

### T013: Rename error type

**Purpose**: Update the `#[from]` error type reference.

**Steps**:
1. Open `crates/kasmos/src/error.rs`
2. Line 127: Change `YamlError(#[from] serde_yaml::Error)` to `YamlError(#[from] serde_yml::Error)`
3. Verify no other `serde_yaml` references exist in `error.rs`

**Files**: `crates/kasmos/src/error.rs`

**Validation**: `error.rs` references `serde_yml::Error` only.

---

### T014: Verify zero references remain (SC-005)

**Purpose**: Confirm complete removal of the deprecated crate.

**Steps**:
1. Run: `grep -r "serde_yaml" crates/` — must return zero results
2. Run: `grep -r "serde_yaml" Cargo.toml` — must return zero results (check workspace root too)
3. Run: `grep -r "serde_yaml" Cargo.lock` — should no longer list serde_yaml as a direct dependency
4. Check for any comments or doc strings that mention `serde_yaml` — update them to reference `serde_yml` if found

**Validation**: Zero hits for `serde_yaml` in source code and Cargo manifests.

---

### T015: Full validation

**Purpose**: Run the complete validation suite.

**Steps**:
1. `cargo build` — must succeed with zero errors
2. `cargo test` — must pass with zero new failures (especially parser tests that exercise YAML frontmatter)
3. `cargo clippy` — must produce zero new warnings
4. Verify YAML frontmatter parsing still works by checking existing test fixtures parse correctly

**Validation**: All three commands pass clean. YAML parsing behavior preserved.

## Definition of Done

- [ ] `crates/kasmos/Cargo.toml` has `serde_yml = "0.0.12"` and no `serde_yaml` entry
- [ ] All 4 source files reference `serde_yml` instead of `serde_yaml`
- [ ] `grep -r "serde_yaml" crates/` returns zero results (SC-005)
- [ ] `cargo build` succeeds
- [ ] `cargo test` passes
- [ ] `cargo clippy` clean

## Risks

- **serde_yml parsing differences**: If `serde_yml` handles YAML edge cases differently (anchors, multi-line strings), frontmatter parsing may break. Mitigation: kasmos uses simple YAML frontmatter (key: value pairs), not advanced YAML features.
- **Upstream maintenance**: Both crates are archived. This is a known risk accepted by the spec. Future work should consider migrating to TOML frontmatter or a maintained YAML parser.

## Reviewer Guidance

1. Verify the find-replace is complete — zero `serde_yaml` references anywhere
2. Check that `serde_yml` version is `"0.0.12"` (latest working release)
3. Verify no behavioral changes — same parse errors, same success paths
4. Run parser tests with attention to YAML frontmatter edge cases
