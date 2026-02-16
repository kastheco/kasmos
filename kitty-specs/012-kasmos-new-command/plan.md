# Implementation Plan: Kasmos New Command

**Branch**: `012-kasmos-new-command` | **Date**: 2026-02-16 | **Spec**: `kitty-specs/012-kasmos-new-command/spec.md`
**Input**: Feature specification from `/home/kas/dev/kasmos/kitty-specs/012-kasmos-new-command/spec.md`

## Summary

Add a `kasmos new` CLI subcommand that launches opencode directly in the current terminal as a planning agent configured to run `/spec-kitty.specify`. The command loads project context (constitution, architecture memory, workflow intelligence, existing specs) into a purpose-built prompt, validates that opencode and spec-kitty are available, then spawns opencode as a child process and waits for it to exit. No Zellij sessions, feature locks, or complex layouts are involved.

## Technical Context

**Language/Version**: Rust 2024 edition (latest stable)
**Primary Dependencies**: `clap` (CLI parsing), `which` (binary validation), `anyhow` (error handling), `std::process::Command` (process spawning) -- all already in `Cargo.toml`
**Storage**: N/A (reads `.kittify/memory/` files; spec-kitty handles spec creation)
**Testing**: `cargo test` -- unit tests for pre-flight validation and prompt construction
**Target Platform**: Linux primary, macOS best-effort
**Project Type**: Single Rust binary (`crates/kasmos/`)
**Performance Goals**: Launch to interactive session in under 3 seconds (SC-001)
**Constraints**:
- No Zellij dependency (FR-011)
- No feature locks (FR-012)
- No new runtime dependencies (SC-005)
- Must propagate opencode's exit code (FR-010)
**Scale/Scope**: Small feature -- adds ~150 lines of new code across 2-3 files, modifies 3 existing files minimally

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Rust 2024 edition | PASS | New code follows Rust 2024 conventions |
| tokio async runtime | PASS | Command handler uses sync `Command::status()` but is called from async `main()`; no conflict |
| Zellij substrate | N/A | This command intentionally bypasses Zellij (FR-011) |
| OpenCode primary agent | PASS | Launches opencode with planner agent profile |
| cargo test required | PASS | Unit tests for pre-flight and prompt construction |
| Linux primary, macOS best-effort | PASS | `std::process::Command` is cross-platform |
| Single binary distribution | PASS | New subcommand in existing binary |

No constitution violations. No complexity exceptions needed.

## Engineering Alignment

Planning interrogation decisions accepted by stakeholder:

1. Process execution strategy: spawn-and-wait via `std::process::Command::status()` (R-001)
2. Prompt construction: purpose-built function in `new.rs`, not `RolePromptBuilder` (R-002)
3. Agent profile: `--agent planner` with custom `--prompt` content (R-003)
4. Pre-flight scope: only opencode + spec-kitty, dedicated check function (R-004)
5. Helper reuse: make `read_file_if_exists` and `summarize_markdown` `pub(crate)` (R-005)

No planning clarifications remain unresolved.

## Project Structure

### Documentation (this feature)

```
kitty-specs/012-kasmos-new-command/
  plan.md              # This file
  research.md          # Phase 0 output (R-001 through R-005)
  data-model.md        # Phase 1 output (minimal for this feature)
  quickstart.md        # Phase 1 output
  spec.md              # Feature specification
  meta.json            # Feature metadata
  checklists/
    requirements.md    # Spec quality checklist
```

### Source Code (repository root)

```
crates/kasmos/src/
  main.rs              # MODIFY: add Commands::New variant + dispatch
  lib.rs               # MODIFY: add `pub mod new;`
  new.rs               # NEW: command handler, pre-flight, prompt builder
  prompt.rs            # MODIFY: make read_file_if_exists + summarize_markdown pub(crate)
```

**Structure Decision**: Single new module `new.rs` following the pattern of other subcommand handlers (`list_specs.rs`, `status.rs`). These are thin modules that load config, perform validation, and delegate to library functions.

## Architecture Decisions

### AD-001: New Module Instead of Extending Launch Flow

**Decision**: Create `crates/kasmos/src/new.rs` as a standalone module rather than adding a code path to `launch/mod.rs`.

**Rationale**: The launch flow (`launch/mod.rs`) is built around a fundamentally different lifecycle: feature detection -> lock acquisition -> layout generation -> session bootstrap. None of these steps apply to `kasmos new`. Sharing code between the two flows would require threading conditionals through a pipeline that doesn't naturally fit the `new` use case.

**Alternatives rejected**: Adding a `--no-zellij` flag to the launch flow -- introduces coupling between two unrelated execution models.

### AD-002: Purpose-Built Prompt Instead of RolePromptBuilder

**Decision**: Build the planning agent prompt directly in `new.rs` using a `build_new_prompt()` function that reads `.kittify/memory/` files and injects the `/spec-kitty.specify` instruction.

**Rationale**: `RolePromptBuilder` assumes a feature directory exists (required constructor parameter), uses it for repo root discovery, and populates feature-specific context (spec, plan, tasks). For `kasmos new`, none of this exists. Working around the builder (passing dummy paths) is fragile and semantically wrong.

The prompt structure for `kasmos new` is:

```
# kasmos planning agent

Your task is to create a new feature specification for this project.
Run `/spec-kitty.specify` to begin the interactive specification workflow.

[If description provided]:
The user has provided this initial feature description:
> <description>

Pass this to /spec-kitty.specify as the starting feature description.

## Project Context

### Constitution
<summarized .kittify/memory/constitution.md>

### Architecture
<summarized .kittify/memory/architecture.md>

### Workflow Intelligence
<summarized .kittify/memory/workflow-intelligence.md>

### Existing Specs
<list of directories in kitty-specs/>

### Project Structure
<top-level directory listing>
```

**Alternatives rejected**: Making `RolePromptBuilder` fields optional -- invasive change to a working system for one consumer.

### AD-003: Lightweight Pre-flight Instead of Full Validation

**Decision**: `new.rs` implements its own pre-flight function that checks only opencode and spec-kitty binaries via `which::which()`.

**Rationale**: The existing `validate_environment()` (`setup/mod.rs:71-106`) checks 6 dependencies including Zellij, pane-tracker, git repo, and config file. Running all 6 checks would either: (a) fail unnecessarily on missing Zellij, or (b) require filtering logic to ignore irrelevant failures. A 2-check function is simpler and faster.

**Alternatives rejected**: Reusing `validate_environment()` with result filtering -- still runs unnecessary filesystem checks for pane-tracker plugin detection.

### AD-004: Synchronous Process Spawning

**Decision**: Use `std::process::Command::status()` (blocking) wrapped in `tokio::task::spawn_blocking()` since `main()` is async.

**Rationale**: `Command::status()` is the simplest way to spawn a child process and wait for it. Since `kasmos new` does nothing else after spawning opencode (no concurrent tasks, no polling), blocking is fine. The `spawn_blocking` wrapper satisfies tokio's requirement that blocking calls don't starve the runtime, though in practice the runtime is idle.

**Alternatives rejected**: `tokio::process::Command` -- adds async complexity for no benefit since there's nothing to do concurrently.

### AD-005: CLI Argument Structure

**Decision**: `Commands::New` takes an optional `description` field using `Vec<String>` with `trailing_var_arg = true` so both `kasmos new "add dark mode"` and `kasmos new add dark mode` work.

```rust
/// Create a new feature specification
New {
    /// Initial feature description (optional)
    #[arg(trailing_var_arg = true)]
    description: Vec<String>,
}
```

The description words are joined with spaces before being injected into the prompt.

**Rationale**: Reduces friction -- the user doesn't need to remember to quote their description. Both styles work naturally.

**Alternatives rejected**: `Option<String>` -- requires quoting for multi-word descriptions, which users commonly forget.

## File Change Summary

| File | Change | Lines |
|------|--------|-------|
| `crates/kasmos/src/new.rs` | NEW: command handler + pre-flight + prompt builder + tests | ~150 |
| `crates/kasmos/src/main.rs` | MODIFY: add `Commands::New` variant + dispatch arm | ~10 |
| `crates/kasmos/src/lib.rs` | MODIFY: add `pub mod new;` | 1 |
| `crates/kasmos/src/prompt.rs` | MODIFY: make 2 helper functions `pub(crate)` | 2 |

**Total estimated new/changed lines**: ~165

## Testing Strategy

### Unit Tests (in `new.rs`)

1. **Pre-flight detects missing opencode**: Configure a fake binary name, verify the check returns an error with actionable guidance.
2. **Pre-flight detects missing spec-kitty**: Same pattern for spec-kitty.
3. **Pre-flight passes with real binaries**: Use known-present binaries (e.g., `bash`) as stand-ins.
4. **Prompt includes /spec-kitty.specify instruction**: Build a prompt with a tempdir repo root, verify the instruction text is present.
5. **Prompt includes user description when provided**: Build with a description string, verify it appears in the output.
6. **Prompt omits description section when not provided**: Build without description, verify no description section.
7. **Prompt loads project context from .kittify/memory/**: Create fixture files, verify constitution/architecture/workflow sections appear.
8. **Prompt handles missing .kittify/memory/ gracefully**: Build with no memory files, verify no errors and context sections are absent.

### Integration Test Considerations

- End-to-end `kasmos new` requires an interactive opencode session, which is not automatable in `cargo test`. Integration testing is manual: run `kasmos new` and verify the planning agent activates `/spec-kitty.specify`.
- The `Commands::New` clap parsing can be tested via `Cli::try_parse_from()`.

## Dependency Impact

No new crate dependencies. All required functionality (`which`, `clap`, `anyhow`, `std::process`, `std::fs`) is already in `Cargo.toml`.

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Prompt too large for opencode's `--prompt` arg | Low | Medium | Shell argument limits are typically 2MB+; project context is ~5KB summarized |
| Planner agent doesn't auto-invoke /spec-kitty.specify | Low | High | Prompt explicitly instructs it; test manually before merging |
| Description with special characters breaks shell escaping | Medium | Low | Use `shell-escape` crate (already a dependency) for the prompt argument |
