# Implementation Plan: Workflow Cheatsheet

**Branch**: `004-workflow-cheatsheet` | **Date**: 2026-02-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/kitty-specs/004-workflow-cheatsheet/spec.md`

## Summary

Create a single markdown file (`docs/workflow-cheatsheet.md`) that serves as the authoritative end-to-end workflow reference for the spec-kitty + kasmos development lifecycle. The document covers every phase from feature specification through orchestrated implementation to merge, including both orchestration modes, daily session patterns, and prerequisites. No code changes — pure documentation artifact.

## Technical Context

**Language/Version**: Markdown (GitHub-flavored)
**Primary Dependencies**: None (static document)
**Storage**: N/A
**Testing**: Manual review — verify renders correctly in `bat`/`less`, GitHub, and VS Code preview
**Target Platform**: Any markdown renderer (terminal, GitHub, IDE)
**Project Type**: Documentation only
**Performance Goals**: N/A
**Constraints**: Must be scannable (no prose walls); commands in fenced code blocks; each phase self-contained
**Scale/Scope**: Single file, ~200-300 lines

## Constitution Check

*No constitution file exists. Skipped.*

## Project Structure

### Documentation (this feature)

```
kitty-specs/004-workflow-cheatsheet/
├── spec.md              # Feature specification
├── plan.md              # This file
├── meta.json            # Feature metadata
└── checklists/
    └── requirements.md  # Spec quality checklist
```

### Source Code (repository root)

```
docs/
├── workflow-cheatsheet.md   # NEW — end-to-end workflow reference (this feature)
├── cheatsheet.md            # EXISTING — kasmos CLI/FIFO quick reference
├── getting-started.md       # EXISTING — kasmos fundamentals + Zellij primer
├── keybinds.md              # EXISTING — Zellij keybind reference
└── architecture.md          # EXISTING — kasmos internals
```

**Structure Decision**: Single new file in the existing `docs/` directory. No new directories. The existing `docs/cheatsheet.md` covers kasmos-specific CLI/FIFO commands; the new `docs/workflow-cheatsheet.md` covers the full spec-kitty → kasmos lifecycle. Cross-links between the two will help operators navigate.

## Content Architecture

The cheatsheet is organized into these sections, in order:

### 1. Prerequisites
- Required tools with version-check commands: `spec-kitty --version`, `kasmos --version`, `zellij --version`, `git --version`
- Optional tools: `bat` for syntax-highlighted viewing

### 2. End-to-End Pipeline Overview
- Visual numbered sequence showing all phases
- Clear boundary marker between **Planning** (spec-kitty) and **Execution** (kasmos)
- Optional phases marked with `(optional)` suffix

```
Planning:  specify → (clarify) → plan → (research) → tasks → (analyze)
Execution: implement → launch → monitor → review → accept → merge
```

### 3. Phase Reference Cards
One block per phase, each containing:
- **Phase name and number**
- **Command** (slash command or CLI)
- **What it does** (one line)
- **Input** (what must exist before running)
- **Output** (what it produces)
- **Key flags/options** (if any)

Phases covered:
1. `/spec-kitty.specify` — Create feature specification
2. `/spec-kitty.clarify` (optional) — Probe spec for ambiguities
3. `/spec-kitty.plan` — Generate implementation plan
4. `/spec-kitty.research` (optional) — Phase 0 research scaffolding
5. `/spec-kitty.tasks` — Generate work packages and prompt files
6. `/spec-kitty.analyze` (optional) — Cross-artifact consistency check
7. `/spec-kitty.implement WP##` — Create worktree for a work package
8. `kasmos launch <feature_dir> [--mode continuous|wave-gated]` — Start orchestration
9. Monitor & interact (Zellij navigation + FIFO commands)
10. `/spec-kitty.review` — Review completed work packages
11. `/spec-kitty.accept` — Validate feature readiness
12. `/spec-kitty.merge` — Merge feature and clean up

### 4. Kasmos Orchestration Sub-Workflow
- Launch command with both modes
- Wave progression (continuous vs wave-gated branching point)
- FIFO command quick-reference (link to `docs/cheatsheet.md` for full table)
- Completion detection signals
- Common recovery actions (restart, retry, force-advance)

### 5. Daily Session Quick-Reference
Typical commands for a single sitting:
- Resume: `kasmos attach <feature_dir>`
- Check status: `kasmos status` or `echo "status" > .kasmos/cmd.pipe`
- Advance wave: `echo "advance" > .kasmos/cmd.pipe`
- Review completed WPs: `/spec-kitty.review`
- End session: detach from Zellij (`Ctrl+o` → `d`)

### 6. Cross-References
Links to related docs:
- `docs/cheatsheet.md` — kasmos CLI/FIFO quick reference
- `docs/getting-started.md` — Zellij primer and kasmos fundamentals
- `docs/keybinds.md` — Zellij keybind reference
- `docs/architecture.md` — kasmos internals

## Content Sources

All information needed to write the cheatsheet already exists in the codebase:

| Content | Source |
|---------|--------|
| Spec-kitty slash commands | `.kittify/` command templates + spec-kitty init output |
| Kasmos CLI commands | `crates/kasmos/src/main.rs` (Clap definitions) |
| FIFO commands | `docs/cheatsheet.md` (existing) |
| Orchestration modes | `docs/getting-started.md` (existing) |
| Zellij navigation | `docs/keybinds.md` (existing) |
| State machine | `docs/cheatsheet.md` → State Machine Quick View |

No external research required.

## Complexity Tracking

*No constitution violations. Feature is a single documentation file with no code changes.*

## Risk Assessment

**Low risk** — This is a static markdown file with no code dependencies, no tests to break, and no integration points beyond cross-links to existing docs.

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Content becomes stale when commands change | Medium | ASM-002 in spec: update cheatsheet in same commit as command changes |
| Duplicate info with existing cheatsheet.md | Low | Distinct scope: workflow-cheatsheet = lifecycle overview; cheatsheet = kasmos CLI/FIFO details. Cross-link, don't duplicate. |
