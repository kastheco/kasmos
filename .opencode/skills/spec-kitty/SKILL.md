---
name: spec-kitty
description: >
  Use this skill when working in a spec-kitty project — any repo initialized with
  `spec-kitty init`. Covers the full lifecycle: specify → clarify → plan → tasks →
  implement → review → accept → merge. Includes git worktree management, parallel
  work package execution, optional steps (clarify, analyze, checklist), the kanban
  dashboard, and the `spec-kitty agent` programmatic API. Trigger on any mention of
  spec-kitty, spec-driven development, SDD workflow, `/spec-kitty.*` slash commands,
  work packages (WP01, WP02), or kitty-specs.
---

# Spec-Kitty: Agent Operator Guide

You are an AI coding agent operating inside a spec-kitty project. This skill teaches
you how to navigate the full spec-driven development lifecycle from an agent's
perspective — what to run, when, and why.

---

## Core Concepts

**Project** — One git repo, initialized once with `spec-kitty init`. Contains all
missions, features, and `.kittify/` automation. Never re-initialize.

**Feature** — A single unit of work (e.g., "Add user auth"). Created with
`/spec-kitty.specify`. Has its own spec, plan, tasks, and work packages under
`kitty-specs/<NNN-feature-name>/`.

**Work Package (WP)** — A subset of tasks within a feature (WP01, WP02, etc.).
Each WP gets its own git worktree for isolated implementation.

**Task** — An atomic unit of work (T001, T002, etc.) within a work package. Tasks
are what you actually implement.

**Mission** — A domain adapter (e.g., `software-dev`, `research`, `documentation`)
that configures templates and workflows. Project-wide; all features share the active
mission.

**Constitution** — `/.kittify/memory/constitution.md`. Non-negotiable project
principles (tech stack, testing policy, coding standards). Read it. Follow it. Always.

---

## The Workflow Pipeline

```
specify → [clarify] → plan → tasks → [analyze] → implement → verify → review → accept → merge
            optional                    optional
```

### Phase Overview

| Phase | Command | Artifacts Produced | Where It Runs |
|---|---|---|---|
| Specify | `/spec-kitty.specify <description>` | `spec.md` | Main repo |
| Clarify | `/spec-kitty.clarify` | Updates `spec.md` Clarifications section | Main repo |
| Plan | `/spec-kitty.plan` | `plan.md`, implementation details | Main repo |
| Tasks | `/spec-kitty.tasks` | `tasks.md` with WPs and tasks | Main repo |
| Analyze | `/spec-kitty.analyze` | Consistency report | Main repo |
| Checklist | `/spec-kitty.checklist` | Quality validation checklists | Main repo |
| Implement | `/spec-kitty.implement` or `spec-kitty implement WP##` | Code changes | WP worktree |
| Verify | `/kas.verify` | Verification report | WP worktree |
| Review | `/spec-kitty.review` | Review notes | WP worktree |
| Accept | `spec-kitty accept` | Acceptance validation | Main repo |
| Merge | `spec-kitty merge` | Merged branches, cleanup | Main repo |

---

## Phase Details

### 1. Specify

```
/spec-kitty.specify Add user authentication with email/password and OAuth
```

This runs a **discovery interview** — the command will ask you structured questions
about the feature. Answer each one thoroughly. The output is `spec.md` inside
`kitty-specs/<NNN-feature-name>/`.

**Agent behavior:**
- Answer every discovery question. Don't skip or shortcut.
- Be specific about scope boundaries (what's in, what's out).
- Reference the constitution for tech stack constraints.
- The spec should describe a *change to the status quo*, not document existing code.

**What gets created:**
```
kitty-specs/
└── 001-user-auth/
    ├── spec.md          # The specification
    └── ...
```

### 2. Clarify (Optional but Recommended)

```
/spec-kitty.clarify
```

Surfaces up to 5 targeted questions about underspecified areas marked with
`[NEEDS CLARIFICATION]` in the spec. Answers are recorded in a Clarifications
section of `spec.md`.

**When to use:**
- Before `/spec-kitty.plan` to reduce downstream rework.
- When the spec has ambiguous requirements.
- When multiple valid interpretations exist.

**When to skip:**
- Spikes or exploratory prototypes.
- Simple, well-understood features.
- If skipping, explicitly state it so you don't block waiting for clarification.

**After structured clarify**, you can do free-form refinement:
```
The upload limit should be 50MB, and we need to support both S3 and local storage.
```

### 3. Plan

```
/spec-kitty.plan
```

Generates the technical implementation plan. This is where you specify the tech
stack, architecture decisions, and implementation approach.

**Agent behavior:**
- Provide tech stack context when prompted (e.g., "We're using Go with Chi router
  and PostgreSQL").
- The plan produces multiple artifacts: `plan.md` plus implementation detail
  documents.
- The plan is an **immutable checkpoint** — changes are additive, not destructive.
- After plan generation, agent context is automatically synced via
  `update-agent-context`.

**What gets created:**
```
kitty-specs/001-user-auth/
├── spec.md
├── plan.md
├── implementation-details/
│   ├── 01-database-schema.md
│   ├── 02-api-endpoints.md
│   └── ...
└── ...
```

### 4. Tasks

```
/spec-kitty.tasks
```

Breaks the plan into atomic, testable work packages and tasks. Each task is small
enough for a single implementation pass.

**Key outputs:**
- Work packages (WP01, WP02, etc.) with dependency declarations.
- Tasks (T001, T002, etc.) within each WP.
- Tasks marked `[P]` can run in parallel.
- File paths are specified per task.
- If tests are requested, test tasks precede implementation tasks.

**Agent behavior:**
- Validate that tasks cover all plan requirements.
- Check for circular dependencies between WPs.
- Each WP's frontmatter includes a `dependencies: []` field.

### 5. Analyze (Optional)

```
/spec-kitty.analyze
```

Cross-artifact consistency and coverage analysis. Checks whether constitution,
spec, plan, and tasks are properly aligned.

**When to use:**
- After `/spec-kitty.tasks`, before implementation.
- When you suspect drift between spec and plan.
- After modifying any upstream artifact.

**What it checks:**
- Constitutional compliance (violations flagged as CRITICAL).
- Spec-to-plan traceability.
- Plan-to-tasks coverage gaps.
- Inconsistencies across artifacts.

### 6. Checklist (Optional)

```
/spec-kitty.checklist
```

Generates custom quality checklists — "unit tests for English." Validates
requirements completeness, clarity, and consistency.

**When to use:**
- Anytime after `/spec-kitty.plan`.
- Before implementation as a final sanity check.
- Checklists live in `kitty-specs/<feature>/checklists/`.

---

## Implementation & Git Worktrees

This is where the workspace-per-WP model matters. Every work package gets its own
isolated git worktree.

### Creating a WP Workspace

```bash
# Create worktree for WP01 (branches from main)
spec-kitty implement WP01

# Create worktree for WP02 that depends on WP01 (branches from WP01)
spec-kitty implement WP02 --base WP01
```

**What happens:**
1. A new git branch is created: `<feature>-WP01`
2. A worktree is created at `.worktrees/<NNN-feature>-WP01/`
3. The WP is moved from `planned` → `doing` on the kanban.
4. Constitution is symlinked into the worktree:
   `.worktrees/<feature>-WP01/.kittify/memory -> ../../../../.kittify/memory`

### Directory Layout

```
my-project/                          # Main repo (main branch)
├── .worktrees/
│   ├── 001-user-auth-WP01/          # WP01 worktree (isolated)
│   ├── 001-user-auth-WP02/          # WP02 worktree (parallel)
│   └── 001-user-auth-WP03/          # WP03 worktree (parallel)
├── .kittify/
├── kitty-specs/
└── ... (main branch files)
```

### Implementing Inside a Worktree

Once inside a worktree, use the slash command or the agent API:

```bash
# Via slash command (interactive, from within the worktree)
/spec-kitty.implement

# Via agent API (programmatic)
spec-kitty agent workflow implement WP01 --agent __AGENT__
```

**Agent behavior inside a worktree:**
- You are on an isolated branch. Changes don't affect main or other WPs.
- Read the tasks for your WP from `kitty-specs/<feature>/tasks.md`.
- Implement tasks in order (respect `[P]` markers for parallelizable ones).
- Commit frequently with descriptive messages.
- Run tests as you go.
- Mark tasks done via `spec-kitty agent tasks mark-status T001 --status done`.

### Parallel Implementation

The whole point of worktrees is enabling parallel execution. Multiple agents (or
multiple sessions of the same agent) can work on different WPs simultaneously.

```
Terminal 1 (Agent A):  cd .worktrees/001-user-auth-WP01/  → implements auth models
Terminal 2 (Agent B):  cd .worktrees/001-user-auth-WP02/  → implements API endpoints
Terminal 3 (Agent C):  cd .worktrees/001-user-auth-WP03/  → implements frontend
```

**Rules for parallel work:**
- Respect WP dependency order. If WP02 depends on WP01, use `--base WP01`.
- Independent WPs can run fully in parallel.
- Each agent commits to its own branch — no conflicts during implementation.
- The dashboard tracks all WP progress in real-time.

### Using the Agent API for Workflow Automation

The `spec-kitty agent` namespace provides programmatic access:

```bash
# Feature management
spec-kitty agent feature create-feature "Payment Flow" --json
spec-kitty agent feature check-prerequisites --json --paths-only
spec-kitty agent feature accept --json

# Workflow transitions (moves kanban lanes automatically)
spec-kitty agent workflow implement WP01 --agent __AGENT__    # planned → doing
spec-kitty agent workflow review WP01 --agent __AGENT__       # for_review → reviewing

# Task management
spec-kitty agent tasks list-tasks                          # List all tasks by lane
spec-kitty agent tasks mark-status T001 --status done      # Update task status
spec-kitty agent tasks add-history T001 --note "Fixed edge case in token refresh"
spec-kitty agent tasks validate-workflow WP01 --json       # Validate WP state
```

All agent commands support `--json` output for structured consumption.

---

## Review

Run from within a WP worktree after implementation is complete.

### Step 1: Verify with `/kas.verify`

```
/kas.verify
```

Always run `/kas.verify` **before** the spec-kitty review. This is a custom
verification pass that validates implementation quality against project-specific
criteria. Address any issues it surfaces before proceeding to the formal review.

### Step 2: Formal Review

```
/spec-kitty.review
```

After `/kas.verify` passes, run the spec-kitty review. This performs an adversarial
review with 12 scrutiny categories including mandatory security checks.

**Agent behavior:**
- Run `/kas.verify` first. Fix any flagged issues before continuing.
- Then run `/spec-kitty.review` to move the WP from `doing` → `for_review`.
- Security scrutiny is mandatory — 7 security grep commands must pass.
- If any security check fails, the WP is automatically rejected.
- Review findings may move the WP back to `doing` → `planned` for rework.
- On success, WP moves to `done`.

---

## Accept

```bash
# Auto-detect current feature
spec-kitty accept

# Specific feature
spec-kitty accept --feature 001-user-auth

# Checklist mode only (no commit)
spec-kitty accept --mode checklist

# With test validation
spec-kitty accept --test "pytest tests/" --test "npm run lint"

# JSON for CI
spec-kitty accept --json
```

Validates the entire feature against the spec and acceptance criteria. Run this
from the **main repo** (not a worktree) after all WPs are in the `done` lane.

---

## Merge

```bash
# Standard merge
spec-kitty merge --push

# Squash commits
spec-kitty merge --strategy squash --push

# Preview first (recommended)
spec-kitty merge --dry-run

# Merge to non-default target
spec-kitty merge --target develop --push

# Keep branch/worktree after merge
spec-kitty merge --keep-branch --push
spec-kitty merge --keep-worktree --push

# Recovery
spec-kitty merge --resume     # Continue interrupted merge
spec-kitty merge --abort      # Clear state and start fresh
```

**Pre-flight validation** (runs automatically):
- Checks all WP worktrees for uncommitted changes.
- Detects missing worktrees.
- Checks for target branch divergence.

**Conflict forecasting** (`--dry-run`):
- Predicts which files will conflict.
- Classifies conflicts as auto-resolvable (status files) or manual.

**Merge behavior:**
- Merges WP branches in dependency order.
- State persisted to `.kittify/merge-state.json` for recovery.
- Auto-cleanup: worktrees and branches removed after success (configurable).

---

## Dashboard

```bash
spec-kitty dashboard              # Launch (auto-detects port)
spec-kitty dashboard --port 4000  # Specific port
spec-kitty dashboard --kill       # Stop
```

The dashboard starts automatically on `spec-kitty init` and runs in the background.
It provides a live kanban board showing WP status across lanes:
`planned` → `doing` → `for_review` → `done`.

---

## Common Pitfalls & Agent Guidance

### Don't Reinitialize
`spec-kitty init` runs **once per project**. Use `/spec-kitty.specify` to create
new features within an existing project.

### Planning Happens in Main, Implementation in Worktrees
Specify, clarify, plan, and tasks all run in the **main repo**. Only `implement`
and `review` run in worktrees.

### Read the Constitution First
Before any implementation, read `.kittify/memory/constitution.md`. It contains
non-negotiable project constraints. Violating the constitution will fail review.

### Spec Describes Change, Not State
The specification describes a *change to the status quo*. Don't try to document
existing code in the spec — the agent should read the actual codebase for current
state.

### Artifacts Are Immutable Checkpoints
Once `spec.md` and `plan.md` are generated, changes are additive (clarifications,
refinements). Don't destructively overwrite them.

### Use `--dry-run` Before Merge
Always preview the merge with `--dry-run` first. It predicts conflicts and gives
you a chance to resolve issues before committing.

### Respect Dependencies
If WP02 depends on WP01, always create it with `spec-kitty implement WP02 --base WP01`.
This ensures the worktree branches from WP01's branch, not main.

### Agent Context Sync
After plan generation, agent context is automatically synced. If things seem stale,
manually refresh with:
```bash
spec-kitty agent context update-context
```

---

## Quick Reference: Minimal Happy Path

```bash
# 1. Specify the feature
/spec-kitty.specify Build a REST API for user management with CRUD operations

# 2. (Optional) Clarify ambiguities
/spec-kitty.clarify

# 3. Plan the implementation
/spec-kitty.plan

# 4. Break into tasks
/spec-kitty.tasks

# 5. (Optional) Validate consistency
/spec-kitty.analyze

# 6. Create worktrees and implement
spec-kitty implement WP01
cd .worktrees/<feature>-WP01/
/spec-kitty.implement
# ... implement tasks, commit, test ...

# 7. Verify and review
/kas.verify
/spec-kitty.review

# 8. Repeat 6-7 for remaining WPs (in parallel if independent)

# 9. Accept the feature (from main repo)
cd /path/to/main/repo
spec-kitty accept

# 10. Merge
spec-kitty merge --dry-run
spec-kitty merge --push
```

---

## Quick Reference: Parallel Multi-Agent Workflow

```bash
# Lead architect specifies, plans, and generates tasks
/spec-kitty.specify ...
/spec-kitty.clarify
/spec-kitty.plan
/spec-kitty.tasks

# Create all WP workspaces
spec-kitty implement WP01
spec-kitty implement WP02
spec-kitty implement WP03 --base WP01   # WP03 depends on WP01

# Agent A (Terminal 1)
cd .worktrees/<feature>-WP01/
spec-kitty agent workflow implement WP01 --agent __AGENT__
# ... implement ...
/kas.verify
spec-kitty agent workflow review WP01 --agent __AGENT__

# Agent B (Terminal 2) — runs in parallel with Agent A
cd .worktrees/<feature>-WP02/
spec-kitty agent workflow implement WP02 --agent __AGENT__
# ... implement ...
/kas.verify

# Agent C (Terminal 3) — starts after WP01 completes
cd .worktrees/<feature>-WP03/
spec-kitty agent workflow implement WP03 --agent __AGENT__

# Monitor progress
spec-kitty dashboard

# When all WPs done
spec-kitty accept
spec-kitty merge --dry-run
spec-kitty merge --strategy squash --push
```
