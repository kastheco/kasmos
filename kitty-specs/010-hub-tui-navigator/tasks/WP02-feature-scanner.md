---
work_package_id: WP02
title: Feature Scanner
lane: done
dependencies:
- WP01
subtasks:
- T006
- T007
- T008
- T009
- T010
- T011
phase: Phase 1 - Foundation
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-13T03:53:23Z'
---

# Work Package Prompt: WP02 - Feature Scanner

## Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** -- Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Markdown Formatting
Wrap HTML/XML tags in backticks: `` `<div>` ``, `` `<script>` ``
Use language identifiers in code blocks: ````rust`, ````bash`

---

## Objectives & Success Criteria

- Implement `hub::scanner::FeatureScanner` that reads `kitty-specs/` and produces `Vec<FeatureEntry>`
- Each `FeatureEntry` includes: number, slug, spec status, plan status, task progress, orchestration status
- Scanner correctly handles: missing directories, empty specs, partial tasks, stale lock files, no Zellij
- Scanner is `Send + Sync` for use in `tokio::task::spawn_blocking` (AD-007)
- Unit tests cover all state combinations
- `cargo test -p kasmos` passes

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-002: Hub Module Structure, AD-007: Feature Status Refresh)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-002, FR-010)
- **Research**: `kitty-specs/010-hub-tui-navigator/research.md` (R-005: Lock File and PID Liveness Check)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md` (FeatureEntry, SpecStatus, PlanStatus, TaskProgress, OrchestrationStatus)
- **Existing code**: `crates/kasmos/src/list_specs.rs` has directory scanning and frontmatter extraction patterns to reuse
- **Existing code**: `crates/kasmos/src/start.rs` has `is_pid_alive()` pattern (line 34)
- **Existing code**: `crates/kasmos/src/zellij.rs` has `ZellijCli::list_sessions()` for session detection

### Key Architectural Decisions

- Scanner lives at `crates/kasmos/src/hub/scanner.rs`
- Reuse patterns from `list_specs.rs` for directory scanning and `extract_lane()` for frontmatter parsing
- PID liveness uses `libc::kill(pid, 0)` pattern from `start.rs`
- Zellij session detection uses `ZellijCli::list_sessions()` -- but scanner must work without Zellij (graceful degradation)
- Scanner must be callable from `spawn_blocking` -- no async in the scan path (use sync I/O)

## Subtasks & Detailed Guidance

### Subtask T006 - Define FeatureEntry and status types

- **Purpose**: Create the core data types that represent feature state in the hub.
- **Steps**:
  1. Create `crates/kasmos/src/hub/scanner.rs`
  2. Add `pub mod scanner;` to `crates/kasmos/src/hub/mod.rs`
  3. Define the following types per `kitty-specs/010-hub-tui-navigator/data-model.md`:

```rust
use std::path::PathBuf;

/// Status of a feature's specification file.
#[derive(Debug, Clone, PartialEq)]
pub enum SpecStatus {
    /// spec.md missing or zero-length
    Empty,
    /// spec.md exists and is non-empty
    Present,
}

/// Status of a feature's implementation plan.
#[derive(Debug, Clone, PartialEq)]
pub enum PlanStatus {
    /// plan.md does not exist
    Absent,
    /// plan.md exists
    Present,
}

/// Progress of work packages for a feature.
#[derive(Debug, Clone, PartialEq)]
pub enum TaskProgress {
    /// tasks/ directory missing or no WPxx-*.md files
    NoTasks,
    /// Some WPs exist, not all done
    InProgress { done: usize, total: usize },
    /// All WPs have lane "done"
    Complete { total: usize },
}

/// Status of orchestration for a feature.
#[derive(Debug, Clone, PartialEq)]
pub enum OrchestrationStatus {
    /// No lock file or dead PID, no Zellij session
    None,
    /// Live lock file PID AND Zellij session exists
    Running,
    /// No live process but Zellij session exists (EXITED state)
    Completed,
}

/// A feature discovered in kitty-specs/.
#[derive(Debug, Clone)]
pub struct FeatureEntry {
    /// Feature number for sorting and display (e.g., "010")
    pub number: String,
    /// Feature slug for display (e.g., "hub-tui-navigator")
    pub slug: String,
    /// Full directory name (e.g., "010-hub-tui-navigator")
    pub full_slug: String,
    /// Whether the feature has a specification
    pub spec_status: SpecStatus,
    /// Whether the feature has a plan
    pub plan_status: PlanStatus,
    /// WP completion state
    pub task_progress: TaskProgress,
    /// Whether orchestration is running
    pub orchestration_status: OrchestrationStatus,
    /// Absolute path to kitty-specs/<full_slug>/
    pub feature_dir: PathBuf,
}
```

- **Files**: `crates/kasmos/src/hub/scanner.rs` (new), `crates/kasmos/src/hub/mod.rs` (add module)
- **Parallel?**: Yes (type definitions are independent)
- **Notes**: All types derive `Debug, Clone, PartialEq` for testability. `FeatureEntry` does not derive `PartialEq` because `PathBuf` comparison is platform-dependent -- tests should compare individual fields.

### Subtask T007 - Implement kitty-specs/ directory scanning

- **Purpose**: Scan the `kitty-specs/` directory and parse feature number/slug from directory names.
- **Steps**:
  1. Implement `FeatureScanner` struct with a `specs_root: PathBuf` field
  2. Implement `pub fn new(specs_root: PathBuf) -> Self`
  3. Implement `pub fn scan(&self) -> Vec<FeatureEntry>` that:
     a. Reads `specs_root` directory entries
     b. Filters to directories only
     c. Parses directory names as `<number>-<slug>` (split on first `-`)
     d. Creates `FeatureEntry` for each valid directory
     e. Sorts by feature number
     f. Returns empty vec if `specs_root` doesn't exist
- **Files**: `crates/kasmos/src/hub/scanner.rs`
- **Parallel?**: Yes
- **Notes**: Follow the pattern from `crates/kasmos/src/list_specs.rs` lines 43-59. The scanner should not panic on invalid directory names -- skip them silently.

**Reference pattern from `list_specs.rs`**:
```rust
let mut entries: Vec<_> = std::fs::read_dir(specs_root)
    .context("Failed to read kitty-specs/")?
    .filter_map(|e| e.ok())
    .filter(|e| e.path().is_dir())
    .collect();
entries.sort_by_key(|e| e.file_name());
```

### Subtask T008 - Implement spec.md status check

- **Purpose**: Determine whether a feature has a specification.
- **Steps**:
  1. Add a private function `fn check_spec_status(feature_dir: &Path) -> SpecStatus`
  2. Check if `feature_dir/spec.md` exists and has non-zero length
  3. Return `SpecStatus::Present` if both conditions met, `SpecStatus::Empty` otherwise
- **Files**: `crates/kasmos/src/hub/scanner.rs`
- **Parallel?**: Yes (independent detection module)
- **Notes**: Follow the pattern from `list_specs.rs` line 63-64:
```rust
let has_spec_content =
    spec_path.is_file() && std::fs::metadata(&spec_path).map_or(false, |m| m.len() > 0);
```

### Subtask T009 - Implement plan.md and tasks/ scanning

- **Purpose**: Determine plan status and work package progress.
- **Steps**:
  1. Add `fn check_plan_status(feature_dir: &Path) -> PlanStatus` -- checks `plan.md` existence
  2. Add `fn check_task_progress(feature_dir: &Path) -> TaskProgress` that:
     a. Checks if `tasks/` directory exists
     b. Scans for `WPxx-*.md` files (pattern: starts with "WP", ends with ".md")
     c. For each WP file, extracts the `lane` field from YAML frontmatter
     d. Counts total WPs and done WPs (lane == "done")
     e. Returns `NoTasks` if no WP files, `Complete` if all done, `InProgress` otherwise
- **Files**: `crates/kasmos/src/hub/scanner.rs`
- **Parallel?**: Yes (independent detection module)
- **Notes**: Reuse the `extract_lane()` pattern from `crates/kasmos/src/list_specs.rs` lines 133-139:
```rust
fn extract_lane(path: &Path) -> Option<String> {
    let content = std::fs::read_to_string(path).ok()?;
    let body = content.strip_prefix("---")?;
    let end = body.find("\n---")?;
    let fm: WpFrontmatter = serde_yaml::from_str(&body[..end]).ok()?;
    fm.lane
}
```
Gracefully handle parse failures by treating unparseable WPs as "planned" (not done).

### Subtask T010 - Implement orchestration status detection

- **Purpose**: Detect whether an orchestration session is running for a feature.
- **Steps**:
  1. Add `fn check_orchestration_status(feature_slug: &str, zellij_sessions: &[String]) -> OrchestrationStatus`
  2. Check for `.kasmos/run.lock` in the feature's worktree directory (`.worktrees/<feature>/.kasmos/run.lock`) or the feature directory itself
  3. If lock file exists, parse PID and check liveness with `libc::kill(pid as i32, 0) == 0`
  4. Check if `kasmos-<feature_slug>` exists in the provided Zellij session list
  5. Return:
     - `Running` if PID alive AND Zellij session exists
     - `Completed` if PID dead but Zellij session exists (EXITED)
     - `None` otherwise
  6. The scanner's `scan()` method should call `zellij list-sessions` once (synchronously via `std::process::Command`) and pass the results to each feature's check
- **Files**: `crates/kasmos/src/hub/scanner.rs`
- **Parallel?**: Yes (independent detection module)
- **Notes**:
  - PID liveness pattern from `crates/kasmos/src/start.rs` line 34: `unsafe { libc::kill(pid as i32, 0) == 0 }`
  - Zellij session listing must be synchronous (scanner runs in `spawn_blocking`). Use `std::process::Command` instead of `tokio::process::Command`.
  - If Zellij is not available (binary not found), treat all features as `OrchestrationStatus::None`.
  - The worktree path convention is `.worktrees/<feature-slug>/` relative to the repo root. The scanner needs the repo root -- derive it from `kitty-specs/` parent directory.

### Subtask T011 - Write scanner unit tests

- **Purpose**: Verify scanner correctness across all state combinations.
- **Steps**:
  1. Add `#[cfg(test)] mod tests` in `crates/kasmos/src/hub/scanner.rs`
  2. Create a test helper that sets up a temporary directory with `kitty-specs/` structure
  3. Write tests for:
     - Empty `kitty-specs/` directory -> empty vec
     - Feature with empty spec -> `SpecStatus::Empty`
     - Feature with non-empty spec -> `SpecStatus::Present`
     - Feature with spec but no plan -> `PlanStatus::Absent`
     - Feature with spec and plan -> `PlanStatus::Present`
     - Feature with no tasks directory -> `TaskProgress::NoTasks`
     - Feature with 3 WPs, 1 done -> `TaskProgress::InProgress { done: 1, total: 3 }`
     - Feature with all WPs done -> `TaskProgress::Complete`
     - Features sorted by number
     - Invalid directory names skipped
- **Files**: `crates/kasmos/src/hub/scanner.rs`
- **Parallel?**: No (depends on T006-T009 being implemented)
- **Notes**: Use `tempfile::TempDir` for test fixtures. Create minimal WP files with just frontmatter for lane testing. Orchestration status tests are harder (require PID/Zellij mocking) -- test the logic with mock session lists.

**Example test fixture**:
```rust
#[test]
fn test_scan_feature_with_spec_and_tasks() {
    let tmp = tempfile::tempdir().unwrap();
    let specs = tmp.path().join("kitty-specs");
    let feature = specs.join("001-my-feature");
    std::fs::create_dir_all(feature.join("tasks")).unwrap();
    std::fs::write(feature.join("spec.md"), "# Spec\nContent here").unwrap();
    std::fs::write(feature.join("plan.md"), "# Plan").unwrap();
    std::fs::write(
        feature.join("tasks/WP01-setup.md"),
        "---\nlane: \"done\"\n---\n# WP01",
    ).unwrap();
    std::fs::write(
        feature.join("tasks/WP02-impl.md"),
        "---\nlane: \"doing\"\n---\n# WP02",
    ).unwrap();

    let scanner = FeatureScanner::new(specs);
    let features = scanner.scan();
    assert_eq!(features.len(), 1);
    assert_eq!(features[0].spec_status, SpecStatus::Present);
    assert_eq!(features[0].plan_status, PlanStatus::Present);
    assert_eq!(features[0].task_progress, TaskProgress::InProgress { done: 1, total: 2 });
}
```

## Test Strategy

- **Unit tests**: All in `crates/kasmos/src/hub/scanner.rs` `#[cfg(test)]` module
- **Run**: `cargo test -p kasmos -- hub::scanner`
- **Fixtures**: Use `tempfile::TempDir` to create filesystem structures
- **Coverage**: Every `SpecStatus`, `PlanStatus`, `TaskProgress` variant must have at least one test
- **Edge cases**: Empty directory, non-UTF8 filenames (skipped), malformed frontmatter (graceful degradation)

## Risks & Mitigations

- **Frontmatter parsing failures**: Use `Option` returns and default to safe states (Empty/Absent/NoTasks)
- **Stale lock files**: Always verify PID liveness -- never trust lock file existence alone
- **Zellij unavailable**: Catch `std::process::Command` spawn errors and default to `OrchestrationStatus::None`
- **Performance**: Scanning 50 features with frontmatter parsing should complete well within 500ms (NFR-001)

## Review Guidance

- Verify all data model types match `kitty-specs/010-hub-tui-navigator/data-model.md`
- Verify frontmatter parsing handles malformed YAML gracefully
- Verify PID liveness check uses `unsafe` block correctly with `libc::kill`
- Verify scanner works when Zellij is not installed
- Run `cargo test -p kasmos` to confirm all tests pass

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T12:00:00Z - release opencode agent - lane=done - Acceptance validation passed
