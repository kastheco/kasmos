# Task Breakdown: Kasmos New Command

**Feature**: 012-kasmos-new-command
**Total Work Packages**: 2
**Total Subtasks**: 11
**Estimated Total Lines**: ~165 new/changed

## Subtask Index

| ID | Description | WP | Parallel |
|----|-------------|----|----------|
| T001 | Make `read_file_if_exists` and `summarize_markdown` `pub(crate)` in prompt.rs | WP01 | [P] |
| T002 | Add CLI wiring: `pub mod new`, `Commands::New`, dispatch arm | WP01 | [P] |
| T003 | Create new.rs with `preflight_check()` for opencode + spec-kitty | WP01 | |
| T004 | Implement repo root discovery from CWD | WP01 | |
| T005 | Implement `build_prompt()` with context loading + description injection | WP01 | |
| T006 | Implement opencode process spawning with exit code propagation | WP01 | |
| T007 | Wire `run()` orchestrator: config -> preflight -> prompt -> spawn | WP01 | |
| T008 | Test pre-flight validation (missing/present binaries) | WP02 | [P] |
| T009 | Test prompt construction (instruction, description handling) | WP02 | [P] |
| T010 | Test prompt degradation (missing .kittify/memory/) | WP02 | [P] |
| T011 | Test CLI parsing for `Commands::New` | WP02 | [P] |

---

## Phase 1: Setup & Core Implementation

### WP01: CLI Wiring, Pre-flight & Prompt Builder

**Priority**: P1 (critical path -- the entire feature)
**Subtasks**: T001, T002, T003, T004, T005, T006, T007 (7 subtasks)
**Estimated prompt size**: ~400 lines
**Dependencies**: None (foundation WP)
**Prompt file**: `tasks/WP01-cli-preflight-prompt-launch.md`

**Goal**: Implement the complete `kasmos new` command from CLI parsing through opencode launch. After this WP, `kasmos new` is fully functional.

**Summary**:
- Expose 2 helper functions in prompt.rs as `pub(crate)` (T001)
- Wire up `Commands::New` in main.rs/lib.rs with dispatch (T002)
- Create `new.rs` with pre-flight binary validation (T003)
- Add repo root discovery from CWD (T004)
- Build the planning agent prompt with project context and optional description (T005)
- Spawn opencode as child process, propagate exit code (T006)
- Wire the `run()` function that orchestrates all steps (T007)

**Implementation sequence**: T001 and T002 are independent (parallel). T003-T007 are sequential within new.rs. T007 integrates everything.

**Included subtasks**:
- [x] T001: Make `read_file_if_exists` and `summarize_markdown` `pub(crate)` in prompt.rs
- [x] T002: Add `pub mod new;` to lib.rs, `Commands::New` to main.rs, dispatch arm
- [ ] T003: Create new.rs with `preflight_check()` for opencode + spec-kitty
- [ ] T004: Implement repo root discovery from CWD
- [ ] T005: Implement `build_prompt()` with context loading + description injection
- [ ] T006: Implement opencode process spawning with exit code propagation
- [ ] T007: Wire `run()` orchestrator: config -> preflight -> prompt -> spawn

**Risks**:
- Shell escaping of prompt content with special characters (mitigate: use shell-escape crate)
- Prompt too long for --prompt arg (mitigate: summarize context with summarize_markdown, shell arg limit is ~2MB)

**Independent test**: Run `kasmos new` from terminal, verify opencode launches with planner role and initiates `/spec-kitty.specify`.

---

## Phase 2: Quality & Verification

### WP02: Unit Tests

**Priority**: P2 (required by constitution, but feature works without them)
**Subtasks**: T008, T009, T010, T011 (4 subtasks)
**Estimated prompt size**: ~300 lines
**Dependencies**: WP01
**Prompt file**: `tasks/WP02-unit-tests.md`

**Goal**: Add comprehensive unit tests for pre-flight validation, prompt construction, and CLI parsing.

**Summary**:
- Test pre-flight catches missing opencode/spec-kitty binaries and passes when present (T008)
- Test prompt includes /spec-kitty.specify instruction and handles description correctly (T009)
- Test prompt degrades gracefully when .kittify/memory/ files are absent (T010)
- Test CLI parsing for `Commands::New` with various input formats (T011)

**Implementation sequence**: All tests are independent (parallel). Each test function stands alone.

**Included subtasks**:
- [ ] T008: Test pre-flight validation (missing/present binaries)
- [ ] T009: Test prompt construction (instruction present, description handling)
- [ ] T010: Test prompt degradation (missing .kittify/memory/)
- [ ] T011: Test CLI parsing for `Commands::New`

**Risks**:
- Test fixture management for .kittify/memory/ files (mitigate: use tempfile crate, already in deps)

**Independent test**: `cargo test -p kasmos -- new` passes all tests.

---

## Parallelization

- **WP01 and WP02 are sequential** (WP02 tests WP01's code)
- **Within WP01**: T001 and T002 are parallel (different files). T003-T007 are sequential.
- **Within WP02**: All tests are parallel (independent test functions).

## MVP Scope

**WP01 alone** is the MVP. It delivers a fully functional `kasmos new` command. WP02 adds test coverage required by the constitution but is not needed for the feature to work.
