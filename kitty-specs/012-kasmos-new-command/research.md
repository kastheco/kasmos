# Research: Kasmos New Command

**Feature**: 012-kasmos-new-command
**Date**: 2026-02-16

## R-001: Process Execution Strategy for Launching OpenCode

**Question**: Should `kasmos new` use `exec()` (process replacement) or spawn-and-wait to launch opencode?

**Findings**:
- `std::process::Command::status()` spawns a child process, waits for it, and returns the exit status. It is cross-platform (Linux + macOS) and handles signal forwarding naturally (Ctrl+C reaches the child since it inherits the terminal's foreground process group).
- `std::os::unix::process::CommandExt::exec()` replaces the current process image entirely. Efficient (no parent lingers) but Unix-only, and requires the tokio runtime to be shut down cleanly before calling exec (which is awkward).
- `Command::status()` is the standard pattern used by CLI wrapper tools (e.g., `cargo` launching `rustc`, `npm` launching node scripts).

**Decision**: Use `std::process::Command::status()` (spawn-and-wait).
**Rationale**: Cross-platform within supported targets, simpler signal/exit handling, no tokio shutdown concerns.
**Alternatives rejected**: `exec()` -- unnecessary complexity for negligible performance gain. The parent process exits immediately after the child anyway.

## R-002: Prompt Construction Without a Feature Directory

**Question**: The existing `RolePromptBuilder` requires `feature_slug` and `feature_dir` (both mandatory). For `kasmos new`, no feature exists yet. How should the prompt be built?

**Findings**:
- `RolePromptBuilder::new()` takes `feature_slug: impl Into<String>` and `feature_dir: impl Into<PathBuf>` as required parameters (`crates/kasmos/src/prompt.rs:128-141`).
- The builder uses `feature_dir` to: locate `spec.md`/`plan.md` (lines 182-204), resolve WP task files (lines 297-341), find the repo root by walking ancestors (lines 389-401).
- The Planner context boundary (`prompt.rs:104-114`) enables `spec: true, plan: true` -- but `read_file_if_exists()` gracefully returns `None` when files are missing, so no errors would occur.
- However, `find_repo_root()` walks up from `feature_dir` looking for `Cargo.toml` or `.kittify/`. If we pass a fake feature_dir, this could fail.
- The template `config/profiles/kasmos/agent/planner.md` has `{{FEATURE_SLUG}}` placeholder, which would render as something meaningless for a new feature.

**Decision**: Build the prompt directly in `new.rs` without `RolePromptBuilder`. Reuse the helper functions `read_file_if_exists` and `summarize_markdown` (made `pub(crate)`).
**Rationale**: The builder's assumptions (feature exists, feature_dir is a real path) don't hold for `kasmos new`. A purpose-built prompt function is cleaner than working around the builder's constraints.
**Alternatives rejected**: (1) Pass dummy values to RolePromptBuilder -- fragile, repo root discovery could fail. (2) Make all RolePromptBuilder fields optional -- invasive change for a single use case.

## R-003: Opencode `--agent` Flag Semantics

**Question**: What `--agent` value should `kasmos new` pass to opencode, and what does it control?

**Findings**:
- The existing `ManagerCommand::to_kdl_pane()` (`launch/layout.rs:122-141`) passes `--agent manager` alongside `--prompt "<full_prompt>"`.
- The `--agent` flag selects an agent profile within opencode's configuration, which may affect model selection, available tools, and system prompt behavior.
- The `--prompt` flag provides the full initial prompt/context injected into the session.
- For `kasmos new`, the planning agent runs `/spec-kitty.specify`, which is a controller-tier task. However, kasmos only defines agent profiles for: manager, planner, coder, reviewer, release.

**Decision**: Use `--agent planner` as the agent profile selector. The `--prompt` content will override the planner template's feature-specific parts with the `/spec-kitty.specify` instruction.
**Rationale**: `planner` is the closest existing profile to the spec-creation use case. The prompt content (not the agent flag) is what drives behavior.
**Alternatives rejected**: Creating a new `controller` agent profile -- adds configuration complexity for no functional benefit. The planner profile's model and tool access are appropriate for specification work.

## R-004: Pre-flight Check Scope for `kasmos new`

**Question**: The existing `validate_environment()` in `setup/mod.rs` checks 6 dependencies (zellij, opencode, spec-kitty, pane-tracker, git, config). Which checks apply to `kasmos new`?

**Findings**:
- `kasmos new` only needs: opencode binary (to launch the session) and spec-kitty binary (used by the planning agent during the session).
- Zellij, pane-tracker, git repo presence, and config file presence are NOT required since `kasmos new` doesn't create sessions/tabs/panes and operates without feature locks.
- The existing `check_binary()` function (`setup/mod.rs:108-124`) is private but can be replicated trivially (it's a `which::which()` call with formatted output).

**Decision**: Implement a dedicated lightweight pre-flight in `new.rs` that only checks opencode and spec-kitty via `which::which()`. Do not reuse `validate_environment()`.
**Rationale**: Avoids false-negative failures (e.g., failing because zellij is missing when it's not needed). Keeps the pre-flight fast and focused.
**Alternatives rejected**: Filtering `validate_environment()` results -- still runs unnecessary checks, and the Zellij/pane-tracker checks involve filesystem probing that adds latency.

## R-005: Visibility of Helper Functions in `prompt.rs`

**Question**: The prompt context helpers (`read_file_if_exists`, `summarize_markdown`) are currently private in `prompt.rs`. Should `new.rs` reuse them?

**Findings**:
- `read_file_if_exists` (`prompt.rs:470-476`) -- trivial 6-line function that checks file existence and reads to string.
- `summarize_markdown` (`prompt.rs:421-443`) -- 22-line function that extracts heading lines and first N content lines.
- Both are pure functions with no side effects.
- The `new.rs` prompt builder needs the same pattern: read optional files from `.kittify/memory/` and summarize their content.

**Decision**: Make `read_file_if_exists` and `summarize_markdown` `pub(crate)` in `prompt.rs` so `new.rs` can import them.
**Rationale**: Avoids code duplication. These are stable utility functions unlikely to change.
**Alternatives rejected**: Copying the functions into `new.rs` -- duplicates logic and creates maintenance burden.
