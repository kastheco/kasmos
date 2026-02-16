---
work_package_id: WP02
title: Unit Tests
lane: planned
dependencies: []
subtasks: [T008, T009, T010, T011]
history:
- date: '2026-02-16'
  action: created
  by: planner
---

# WP02: Unit Tests

## Objective

Add comprehensive unit tests for the `kasmos new` command covering pre-flight validation, prompt construction, prompt degradation, and CLI parsing. Tests verify all code paths from WP01 are exercised.

## Implementation Command

```bash
spec-kitty implement WP02 --base WP01
```

## Context

**Feature**: 012-kasmos-new-command
**Depends on**: WP01 (all code under test)
**Testing strategy**: From `kitty-specs/012-kasmos-new-command/plan.md` Testing Strategy section
**Constitution requirement**: "All features must have corresponding tests" -- these tests satisfy that requirement.

**Key files to read before starting**:
- `crates/kasmos/src/new.rs` -- the code under test (created in WP01)
- `crates/kasmos/src/prompt.rs` lines 684-843 -- existing test patterns and Fixture struct for reference
- `crates/kasmos/src/main.rs` -- CLI struct for parsing tests

---

## Subtask T008: Test Pre-flight Validation

**Purpose**: Verify that `preflight_check()` correctly detects missing and present binaries, returning actionable errors.

**Steps**:
1. In `crates/kasmos/src/new.rs`, add a `#[cfg(test)] mod tests` block at the bottom of the file.

2. Add test for missing opencode:
   ```rust
   #[cfg(test)]
   mod tests {
       use super::*;

       #[test]
       fn preflight_fails_when_opencode_missing() {
           let mut config = Config::default();
           config.agent.opencode_binary = "__nonexistent_opencode_xyz__".to_string();

           let err = preflight_check(&config).expect_err("should fail");
           let msg = err.to_string();
           assert!(msg.contains("__nonexistent_opencode_xyz__"));
           assert!(msg.contains("not found in PATH"));
           assert!(msg.contains("Install OpenCode"));
       }
   }
   ```

3. Add test for missing spec-kitty:
   ```rust
   #[test]
   fn preflight_fails_when_spec_kitty_missing() {
       let mut config = Config::default();
       // opencode must pass first, so use a real binary
       config.agent.opencode_binary = "bash".to_string();
       config.paths.spec_kitty_binary = "__nonexistent_spec_kitty_xyz__".to_string();

       let err = preflight_check(&config).expect_err("should fail");
       let msg = err.to_string();
       assert!(msg.contains("__nonexistent_spec_kitty_xyz__"));
       assert!(msg.contains("not found in PATH"));
       assert!(msg.contains("spec-kitty"));
   }
   ```

4. Add test for successful pre-flight:
   ```rust
   #[test]
   fn preflight_passes_with_real_binaries() {
       let mut config = Config::default();
       config.agent.opencode_binary = "bash".to_string();
       config.paths.spec_kitty_binary = "bash".to_string();

       preflight_check(&config).expect("preflight should pass with real binaries");
   }
   ```

5. **Important**: Use `"bash"` as a stand-in for real binaries since it's always present on Linux/macOS. Use clearly-fake names like `"__nonexistent_xyz__"` for missing binary tests.

**Files**:
- `crates/kasmos/src/new.rs` (add ~40 lines in test module)

**Validation**:
- [ ] `cargo test -p kasmos -- preflight_fails_when_opencode_missing` passes
- [ ] `cargo test -p kasmos -- preflight_fails_when_spec_kitty_missing` passes
- [ ] `cargo test -p kasmos -- preflight_passes_with_real_binaries` passes

---

## Subtask T009: Test Prompt Construction

**Purpose**: Verify that `build_prompt()` includes the `/spec-kitty.specify` instruction and correctly handles the optional description.

**Steps**:
1. Create a test fixture helper (follow the pattern from `prompt.rs` tests):
   ```rust
   use tempfile::tempdir;

   fn setup_test_repo() -> tempfile::TempDir {
       let root = tempdir().expect("create tempdir");
       // Create Cargo.toml so find_repo_root() would work
       std::fs::write(root.path().join("Cargo.toml"), "[workspace]\n").expect("write Cargo.toml");
       // Create .kittify/memory/ with test content
       let memory = root.path().join(".kittify/memory");
       std::fs::create_dir_all(&memory).expect("create memory dir");
       std::fs::write(
           memory.join("constitution.md"),
           "# Constitution\n\n## Technical Standards\n\n- Rust 2024\n- tokio async",
       ).expect("write constitution");
       std::fs::write(
           memory.join("architecture.md"),
           "# Architecture\n\nARCH_CONTENT_SENTINEL",
       ).expect("write architecture");
       // Create kitty-specs/ with a sample feature
       let specs = root.path().join("kitty-specs/011-test-feature");
       std::fs::create_dir_all(&specs).expect("create specs dir");
       root
   }
   ```

2. Test that prompt always contains the /spec-kitty.specify instruction:
   ```rust
   #[test]
   fn prompt_contains_specify_instruction() {
       let repo = setup_test_repo();
       let prompt = build_prompt(repo.path(), None).expect("build prompt");
       assert!(prompt.contains("/spec-kitty.specify"));
       assert!(prompt.contains("planning agent"));
   }
   ```

3. Test that description is included when provided:
   ```rust
   #[test]
   fn prompt_includes_description_when_provided() {
       let repo = setup_test_repo();
       let prompt = build_prompt(repo.path(), Some("add dark mode toggle")).expect("build prompt");
       assert!(prompt.contains("add dark mode toggle"));
       assert!(prompt.contains("Initial Feature Description"));
   }
   ```

4. Test that description section is absent when not provided:
   ```rust
   #[test]
   fn prompt_omits_description_when_not_provided() {
       let repo = setup_test_repo();
       let prompt = build_prompt(repo.path(), None).expect("build prompt");
       assert!(!prompt.contains("Initial Feature Description"));
   }
   ```

5. Test that project context sections appear:
   ```rust
   #[test]
   fn prompt_includes_project_context() {
       let repo = setup_test_repo();
       let prompt = build_prompt(repo.path(), None).expect("build prompt");
       // Constitution content is present
       assert!(prompt.contains("Rust 2024"));
       // Architecture content is present
       assert!(prompt.contains("ARCH_CONTENT_SENTINEL"));
       // Existing specs are listed
       assert!(prompt.contains("011-test-feature"));
   }
   ```

**Files**:
- `crates/kasmos/src/new.rs` (add ~60 lines in test module)

**Validation**:
- [ ] `cargo test -p kasmos -- prompt_contains_specify_instruction` passes
- [ ] `cargo test -p kasmos -- prompt_includes_description_when_provided` passes
- [ ] `cargo test -p kasmos -- prompt_omits_description_when_not_provided` passes
- [ ] `cargo test -p kasmos -- prompt_includes_project_context` passes

---

## Subtask T010: Test Prompt Degradation

**Purpose**: Verify that `build_prompt()` handles missing `.kittify/memory/` files gracefully (no errors, sections simply omitted).

**Steps**:
1. Create a minimal test repo with NO .kittify/memory/ directory:
   ```rust
   fn setup_bare_repo() -> tempfile::TempDir {
       let root = tempdir().expect("create tempdir");
       std::fs::write(root.path().join("Cargo.toml"), "[workspace]\n").expect("write Cargo.toml");
       root
   }
   ```

2. Test that prompt builds without error on bare repo:
   ```rust
   #[test]
   fn prompt_handles_missing_memory_gracefully() {
       let repo = setup_bare_repo();
       let prompt = build_prompt(repo.path(), None).expect("build prompt");
       // Core instruction is always present
       assert!(prompt.contains("/spec-kitty.specify"));
       // Memory-dependent sections are absent (not errored)
       assert!(!prompt.contains("Constitution"));
       assert!(!prompt.contains("Architecture"));
       assert!(!prompt.contains("Workflow Intelligence"));
   }
   ```

3. Test with partial memory (only constitution, no architecture):
   ```rust
   #[test]
   fn prompt_handles_partial_memory() {
       let repo = setup_bare_repo();
       let memory = repo.path().join(".kittify/memory");
       std::fs::create_dir_all(&memory).expect("create memory dir");
       std::fs::write(
           memory.join("constitution.md"),
           "# Constitution\n\nPARTIAL_SENTINEL",
       ).expect("write constitution");

       let prompt = build_prompt(repo.path(), None).expect("build prompt");
       assert!(prompt.contains("PARTIAL_SENTINEL"));
       assert!(!prompt.contains("Architecture"));
   }
   ```

**Files**:
- `crates/kasmos/src/new.rs` (add ~30 lines in test module)

**Validation**:
- [ ] `cargo test -p kasmos -- prompt_handles_missing_memory_gracefully` passes
- [ ] `cargo test -p kasmos -- prompt_handles_partial_memory` passes

---

## Subtask T011: Test CLI Parsing

**Purpose**: Verify that `Commands::New` parses correctly with various description input formats.

**Steps**:
1. In the test module in `crates/kasmos/src/new.rs`, or better in `crates/kasmos/src/main.rs` if the `Cli` struct is accessible, add parsing tests. Since `Cli` is in main.rs and private, these tests should go in `main.rs` or the Cli struct needs to be made accessible.

   **Recommended approach**: Add a test in `main.rs` at the bottom:
   ```rust
   #[cfg(test)]
   mod tests {
       use super::*;
       use clap::Parser;

       #[test]
       fn new_command_parses_without_description() {
           let cli = Cli::try_parse_from(["kasmos", "new"]).expect("parse");
           match cli.command {
               Some(Commands::New { description }) => {
                   assert!(description.is_empty());
               }
               _ => panic!("expected Commands::New"),
           }
       }

       #[test]
       fn new_command_parses_quoted_description() {
           let cli = Cli::try_parse_from(["kasmos", "new", "add dark mode toggle"])
               .expect("parse");
           match cli.command {
               Some(Commands::New { description }) => {
                   assert_eq!(description, vec!["add", "dark", "mode", "toggle"]);
               }
               _ => panic!("expected Commands::New"),
           }
       }

       #[test]
       fn new_command_parses_unquoted_trailing_words() {
           let cli = Cli::try_parse_from([
               "kasmos", "new", "add", "dark", "mode"
           ]).expect("parse");
           match cli.command {
               Some(Commands::New { description }) => {
                   assert_eq!(description.join(" "), "add dark mode");
               }
               _ => panic!("expected Commands::New"),
           }
       }
   }
   ```

2. **Note on `trailing_var_arg`**: With `trailing_var_arg = true`, clap treats everything after `new` as positional arguments. Both `kasmos new "add dark mode"` (shell splits into 3 words) and `kasmos new add dark mode` produce the same `Vec<String>`.

**Files**:
- `crates/kasmos/src/main.rs` (add ~35 lines in test module)

**Validation**:
- [ ] `cargo test -p kasmos -- new_command_parses_without_description` passes
- [ ] `cargo test -p kasmos -- new_command_parses_quoted_description` passes
- [ ] `cargo test -p kasmos -- new_command_parses_unquoted_trailing_words` passes

---

## Definition of Done

- [ ] All 10 unit tests pass: `cargo test -p kasmos`
- [ ] Tests cover pre-flight (missing opencode, missing spec-kitty, success)
- [ ] Tests cover prompt (/spec-kitty.specify instruction, description present/absent, context loading, missing context)
- [ ] Tests cover CLI parsing (no description, quoted, trailing words)
- [ ] No test relies on external binaries other than `bash` (available on all supported platforms)
- [ ] Temp directories are used for all filesystem fixtures (no pollution)

## Risks

- **Test fixture management**: Tests create temp directories with .kittify/memory/ files. The `tempfile` crate handles cleanup. If tests fail mid-run, temp dirs are still cleaned up by the OS eventually.
- **Platform differences**: `bash` is used as a stand-in for binary existence tests. It's present on both Linux and macOS. If someone runs tests on Windows (unsupported), these tests would fail -- acceptable per constitution.

## Reviewer Guidance

- Verify all test names are descriptive and follow existing naming conventions in the codebase
- Check that test fixtures match real project structure (Cargo.toml at root, .kittify/memory/ directory)
- Confirm no tests depend on the real opencode or spec-kitty binaries being installed
- Ensure `tempdir()` is used (not hardcoded paths) for all fixture directories
- Run `cargo test -p kasmos` to verify all tests pass
