---
work_package_id: "WP01"
title: "CLI Wiring, Pre-flight & Prompt Builder"
lane: "planned"
dependencies: []
subtasks: ["T001", "T002", "T003", "T004", "T005", "T006", "T007"]
history:
  - date: "2026-02-16"
    action: "created"
    by: "planner"
---

# WP01: CLI Wiring, Pre-flight & Prompt Builder

## Objective

Implement the complete `kasmos new` command: CLI wiring, dependency pre-flight, prompt construction with project context, and opencode process spawning with exit code propagation. After this WP, `kasmos new` is fully functional end-to-end.

## Implementation Command

```bash
spec-kitty implement WP01
```

## Context

**Feature**: 012-kasmos-new-command
**Architecture decisions**: AD-001 (new module), AD-002 (purpose-built prompt), AD-003 (lightweight pre-flight), AD-004 (sync spawn), AD-005 (trailing var arg)
**Research references**: R-001 through R-005 in `kitty-specs/012-kasmos-new-command/research.md`

**Key files to read before starting**:
- `crates/kasmos/src/main.rs` -- current CLI structure and dispatch pattern
- `crates/kasmos/src/lib.rs` -- module exports
- `crates/kasmos/src/prompt.rs` -- helpers to make pub(crate), and existing RolePromptBuilder for pattern reference
- `crates/kasmos/src/config.rs` -- Config struct, especially `AgentConfig` and `PathsConfig`
- `crates/kasmos/src/launch/layout.rs:110-141` -- ManagerCommand for opencode invocation pattern
- `config/profiles/kasmos/agent/planner.md` -- existing planner template for reference

---

## Subtask T001: Make Helper Functions `pub(crate)` in prompt.rs

**Purpose**: Expose `read_file_if_exists` and `summarize_markdown` so `new.rs` can reuse them for loading project context without duplicating code.

**Steps**:
1. Open `crates/kasmos/src/prompt.rs`
2. Change `fn read_file_if_exists` (line ~470) to `pub(crate) fn read_file_if_exists`
3. Change `fn summarize_markdown` (line ~421) to `pub(crate) fn summarize_markdown`
4. No other changes needed -- both functions are pure and have no side effects

**Files**:
- `crates/kasmos/src/prompt.rs` (modify 2 lines)

**Validation**:
- [ ] `cargo build` succeeds (no breakage from visibility change)
- [ ] `cargo test -p kasmos` still passes (existing tests unaffected)

---

## Subtask T002: Add CLI Wiring (lib.rs + main.rs)

**Purpose**: Register the `new` subcommand so `kasmos new [description...]` is parseable by clap and dispatches to the handler.

**Steps**:
1. In `crates/kasmos/src/lib.rs`, add `pub mod new;` in the module list (alphabetical order, after `pub mod logging;`)

2. In `crates/kasmos/src/main.rs`, add the `New` variant to the `Commands` enum:
   ```rust
   /// Create a new feature specification
   New {
       /// Initial feature description (optional, can be multiple words)
       #[arg(trailing_var_arg = true)]
       description: Vec<String>,
   },
   ```

3. In `crates/kasmos/src/main.rs`, add the dispatch arm in `match cli.command`:
   ```rust
   Some(Commands::New { description }) => {
       if let Err(err) = kasmos::init_logging(false) {
           eprintln!("Warning: logging init failed: {err}");
       }
       let desc = if description.is_empty() {
           None
       } else {
           Some(description.join(" "))
       };
       let code = kasmos::new::run(desc.as_deref())
           .context("New feature spec failed")?;
       std::process::exit(code);
   }
   ```

4. Update the `after_help` string to include the `new` command:
   ```
   kasmos new [description]              Create a new feature specification
   ```

**Files**:
- `crates/kasmos/src/lib.rs` (add 1 line)
- `crates/kasmos/src/main.rs` (add ~15 lines)

**Validation**:
- [ ] `kasmos --help` shows the `new` subcommand with description
- [ ] `kasmos new --help` shows the optional description argument
- [ ] Code compiles (will fail until new.rs exists -- that's expected if doing T001+T002 before T003)

**Note**: This subtask and T001 can be done in parallel since they touch different files.

---

## Subtask T003: Create new.rs with preflight_check()

**Purpose**: Create the new module and implement dependency validation that checks only opencode and spec-kitty are present, per AD-003.

**Steps**:
1. Create `crates/kasmos/src/new.rs`

2. Add module-level doc comment:
   ```rust
   //! `kasmos new` -- launch a planning agent to create a new feature specification.
   ```

3. Implement `preflight_check()`:
   ```rust
   use crate::config::Config;
   use anyhow::{Context, Result, bail};
   use std::path::PathBuf;

   /// Validate that required binaries (opencode, spec-kitty) are in PATH.
   /// Returns Ok(()) if both found, or a descriptive error with install guidance.
   fn preflight_check(config: &Config) -> Result<()> {
       // Check opencode
       if which::which(&config.agent.opencode_binary).is_err() {
           bail!(
               "{} not found in PATH\n  Needed for: launching the planning agent session\n  Fix: Install OpenCode and ensure its launcher binary is on PATH",
               config.agent.opencode_binary
           );
       }
       // Check spec-kitty
       if which::which(&config.paths.spec_kitty_binary).is_err() {
           bail!(
               "{} not found in PATH\n  Needed for: feature/task lifecycle commands\n  Fix: Install spec-kitty and ensure `spec-kitty` is on PATH",
               config.paths.spec_kitty_binary
           );
       }
       Ok(())
   }
   ```

**Files**:
- `crates/kasmos/src/new.rs` (new file, ~30 lines so far)

**Validation**:
- [ ] With a valid config, `preflight_check()` returns `Ok(())`
- [ ] With a bad opencode binary name, it returns an error mentioning the binary and install guidance
- [ ] With a bad spec-kitty binary name, same pattern

---

## Subtask T004: Implement Repo Root Discovery

**Purpose**: Find the project root directory from CWD so we can locate `.kittify/memory/` and `kitty-specs/` for context loading.

**Steps**:
1. In `new.rs`, add a function to discover repo root:
   ```rust
   /// Walk up from CWD to find project root (directory containing Cargo.toml or .kittify/).
   fn find_repo_root() -> Result<PathBuf> {
       let cwd = std::env::current_dir().context("failed to determine current directory")?;
       for ancestor in cwd.ancestors() {
           if ancestor.join("Cargo.toml").exists() || ancestor.join(".kittify").exists() {
               return Ok(ancestor.to_path_buf());
           }
       }
       bail!(
           "Could not find project root (no Cargo.toml or .kittify/ found).\n\
            Run `kasmos new` from the project root, or run `kasmos setup` first."
       );
   }
   ```

**Design note**: This mirrors `RolePromptBuilder::find_repo_root()` (`prompt.rs:389-401`) but starts from CWD instead of `feature_dir`. We can't reuse the existing function because it's a method on `RolePromptBuilder` and requires a `&self` reference.

**Files**:
- `crates/kasmos/src/new.rs` (add ~15 lines)

**Validation**:
- [ ] Running from project root returns the root path
- [ ] Running from a subdirectory (e.g., `crates/kasmos/`) still finds the root
- [ ] Running from outside any project returns an actionable error

---

## Subtask T005: Implement build_prompt() with Context and Description

**Purpose**: Construct the full planning agent prompt that instructs opencode to run `/spec-kitty.specify`, includes project context, and optionally embeds the user's initial description.

**Steps**:
1. In `new.rs`, implement the prompt builder:
   ```rust
   use crate::prompt::{read_file_if_exists, summarize_markdown};

   /// Build the planning agent prompt with project context and optional description.
   fn build_prompt(repo_root: &Path, description: Option<&str>) -> Result<String> {
       let mut sections = Vec::new();

       // Role instruction
       sections.push(
           "# kasmos planning agent\n\n\
            Your task is to create a new feature specification for this project.\n\
            Run `/spec-kitty.specify` to begin the interactive specification workflow.\n\
            Follow the discovery interview process to understand the feature before generating the spec."
               .to_string(),
       );

       // Optional description
       if let Some(desc) = description {
           sections.push(format!(
               "## Initial Feature Description\n\n\
                The user has provided this initial feature description:\n\n\
                > {desc}\n\n\
                Pass this to /spec-kitty.specify as the starting feature description."
           ));
       }

       // Project context from .kittify/memory/
       let memory_dir = repo_root.join(".kittify/memory");

       if let Some(constitution) = read_file_if_exists(&memory_dir.join("constitution.md"))? {
           sections.push(format!(
               "## Constitution\n\n{}",
               summarize_markdown(&constitution, 15)
           ));
       }

       if let Some(architecture) = read_file_if_exists(&memory_dir.join("architecture.md"))? {
           sections.push(format!(
               "## Architecture\n\n{}",
               summarize_markdown(&architecture, 15)
           ));
       }

       if let Some(workflow) = read_file_if_exists(&memory_dir.join("workflow-intelligence.md"))? {
           sections.push(format!(
               "## Workflow Intelligence\n\n{}",
               summarize_markdown(&workflow, 12)
           ));
       }

       // Existing specs for awareness
       let specs_dir = repo_root.join("kitty-specs");
       if specs_dir.is_dir() {
           let mut specs = Vec::new();
           for entry in std::fs::read_dir(&specs_dir)? {
               let entry = entry?;
               if entry.path().is_dir() {
                   if let Some(name) = entry.file_name().to_str() {
                       specs.push(format!("- `{name}`"));
                   }
               }
           }
           if !specs.is_empty() {
               specs.sort();
               sections.push(format!(
                   "## Existing Feature Specs\n\n{}",
                   specs.join("\n")
               ));
           }
       }

       // Project structure (top-level dirs)
       let mut dirs = Vec::new();
       for entry in std::fs::read_dir(repo_root)? {
           let entry = entry?;
           if entry.path().is_dir() {
               if let Some(name) = entry.file_name().to_str() {
                   if !name.starts_with('.') {
                       dirs.push(format!("- `{name}/`"));
                   }
               }
           }
       }
       if !dirs.is_empty() {
           dirs.sort();
           sections.push(format!("## Project Structure\n\n{}", dirs.join("\n")));
       }

       Ok(sections.join("\n\n"))
   }
   ```

2. The prompt structure follows AD-002 exactly:
   - Role instruction with `/spec-kitty.specify` command
   - Optional description (only if provided)
   - Constitution, architecture, workflow intelligence (each summarized)
   - Existing specs list
   - Project structure

**Files**:
- `crates/kasmos/src/new.rs` (add ~70 lines)

**Validation**:
- [ ] Prompt always contains "/spec-kitty.specify"
- [ ] With description "add dark mode", prompt contains `> add dark mode`
- [ ] Without description, no "Initial Feature Description" section
- [ ] With .kittify/memory/ files present, their summaries appear
- [ ] With .kittify/memory/ absent, no error and those sections are simply missing
- [ ] Existing specs in kitty-specs/ are listed
- [ ] Top-level directories are listed (excluding dotfiles)

---

## Subtask T006: Implement Opencode Process Spawning

**Purpose**: Build the opencode command with correct arguments and spawn it as a child process, returning the exit code to the caller.

**Steps**:
1. In `new.rs`, implement the spawn function:
   ```rust
   use std::process::Command;

   /// Spawn opencode as a child process with the planning agent prompt.
   /// Returns the process exit code (0 on success).
   fn spawn_opencode(config: &Config, prompt: &str) -> Result<i32> {
       let mut cmd = Command::new(&config.agent.opencode_binary);
       cmd.arg("oc");

       // Add profile if configured
       if let Some(ref profile) = config.agent.opencode_profile {
           cmd.args(["-p", profile]);
       }

       // Separator and agent args
       cmd.arg("--");
       cmd.args(["--agent", "planner"]);
       cmd.args(["--prompt", prompt]);

       let status = cmd
           .status()
           .with_context(|| format!("failed to launch {}", config.agent.opencode_binary))?;

       Ok(status.code().unwrap_or(1))
   }
   ```

2. **Key design points**:
   - Uses `Command::status()` per AD-004 (spawn-and-wait)
   - Passes `--agent planner` per R-003
   - Passes `--prompt` with the full rendered prompt
   - Returns the exit code for propagation per FR-010
   - `unwrap_or(1)` handles signal termination (where code() returns None)

**Shell escaping note**: `Command::arg()` handles argument boundaries correctly -- no shell escaping needed since we're not going through a shell. The prompt string is passed as a single OS argument directly.

**Files**:
- `crates/kasmos/src/new.rs` (add ~25 lines)

**Validation**:
- [ ] With valid config and prompt, opencode launches in current terminal
- [ ] Profile flag is included when config has `opencode_profile = Some("kas")`
- [ ] Profile flag is omitted when config has `opencode_profile = None`
- [ ] Exit code from opencode is returned correctly

---

## Subtask T007: Wire run() Orchestrator

**Purpose**: Create the public `run()` function that ties together config loading, pre-flight, prompt building, and process spawning into the complete `kasmos new` flow.

**Steps**:
1. In `new.rs`, implement the top-level orchestrator:
   ```rust
   /// Run the `kasmos new` command.
   ///
   /// Loads config, validates dependencies, builds the planning agent prompt
   /// with project context, and launches opencode in the current terminal.
   /// Returns the opencode process exit code.
   pub fn run(description: Option<&str>) -> Result<i32> {
       let config = Config::load().context("Failed to load config")?;

       preflight_check(&config)?;

       let repo_root = find_repo_root()?;
       let prompt = build_prompt(&repo_root, description)?;

       spawn_opencode(&config, &prompt)
   }
   ```

2. This is the function called from `main.rs` (wired in T002). The flow is:
   - Load config (kasmos.toml + env overrides + defaults)
   - Pre-flight (validate opencode + spec-kitty in PATH)
   - Find repo root (walk up from CWD)
   - Build prompt (load project context + description)
   - Spawn opencode (launch child process, wait, return exit code)

3. The function is intentionally synchronous. It's called from the async `main()` but does no async work. Since `main()` calls `std::process::exit()` with the returned code, the tokio runtime is dropped cleanly.

**Files**:
- `crates/kasmos/src/new.rs` (add ~15 lines)

**Validation**:
- [ ] `kasmos new` from project root launches opencode with planner agent
- [ ] `kasmos new add dark mode` launches opencode with description in prompt
- [ ] `kasmos new` with missing opencode prints error and returns non-zero
- [ ] `kasmos new` from outside project root prints guidance and returns non-zero
- [ ] After opencode exits, `kasmos new` exits with same code

---

## Definition of Done

- [ ] `kasmos new` launches opencode in current terminal as planning agent
- [ ] Planning agent prompt contains `/spec-kitty.specify` instruction
- [ ] `kasmos new "description"` and `kasmos new description words` both work
- [ ] Missing opencode/spec-kitty produces actionable error
- [ ] Exit code propagation works
- [ ] `cargo build` succeeds
- [ ] `kasmos --help` shows `new` subcommand
- [ ] No Zellij sessions/tabs/panes created
- [ ] No feature locks acquired

## Risks

- **Shell argument size**: Prompt could theoretically exceed OS argument limits. Mitigated by summarize_markdown() keeping context compact (~5KB). OS limits are typically 2MB+.
- **Special characters in description**: User might include quotes or shell metacharacters. Mitigated by `Command::arg()` passing args directly without shell interpretation.
- **Agent doesn't invoke /spec-kitty.specify**: The prompt explicitly instructs it, but LLM behavior isn't deterministic. Manual verification required before merge.

## Reviewer Guidance

- Verify the prompt structure matches AD-002 in plan.md
- Check that `preflight_check()` only checks opencode + spec-kitty (not zellij or other deps)
- Confirm `Command::arg()` is used (not shell string interpolation) for process spawning
- Verify exit code propagation handles signal termination (code() returns None)
- Run `kasmos new` manually to verify end-to-end behavior
