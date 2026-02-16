# Data Model: Kasmos New Command

## Overview

This feature introduces no new persistent entities or state. The data model is limited to transient runtime structures used during a single `kasmos new` invocation.

## Structure: NewCommandConfig

- Purpose: Holds resolved configuration for a `kasmos new` invocation.
- Lifetime: Created at command start, consumed to build the opencode command, discarded on exit.
- Fields:
  - `opencode_binary: String` -- resolved path to the opencode launcher (from `Config.agent.opencode_binary`)
  - `opencode_profile: Option<String>` -- optional profile name (from `Config.agent.opencode_profile`)
  - `spec_kitty_binary: String` -- resolved path to spec-kitty (from `Config.paths.spec_kitty_binary`, used for pre-flight only)
  - `description: Option<String>` -- user-provided initial feature description, joined from CLI args
  - `repo_root: PathBuf` -- project root, used to locate `.kittify/memory/` and `kitty-specs/`

## Structure: PlanningPrompt

- Purpose: The fully rendered prompt string passed to opencode via `--prompt`.
- Lifetime: Built once from project context files, consumed by the opencode command.
- Sections (in order):
  1. Role instruction (fixed text: planning agent, invoke `/spec-kitty.specify`)
  2. User description (optional, only if provided)
  3. Constitution summary (from `.kittify/memory/constitution.md`, if present)
  4. Architecture summary (from `.kittify/memory/architecture.md`, if present)
  5. Workflow intelligence summary (from `.kittify/memory/workflow-intelligence.md`, if present)
  6. Existing specs list (directory names under `kitty-specs/`)
  7. Project structure (top-level directory listing)
- All sections degrade gracefully: if a source file is missing, the section is omitted (no error).

## No Persistent State

- No lock files created (FR-012)
- No audit logs written
- No feature directories created by kasmos (spec-kitty handles this during the agent session)
- No Zellij state tracked
