# Spec-Kitty + Kasmos Workflow Cheatsheet

Quick-reference for the end-to-end feature development lifecycle — from specification through orchestrated implementation to merge.

---

## Prerequisites

| Tool | Check | Purpose |
|------|-------|---------|
| spec-kitty | `spec-kitty --version` | Feature specification & planning |
| kasmos | `kasmos --version` | Zellij-based agent orchestration |
| zellij | `zellij --version` | Terminal multiplexer runtime |
| git | `git --version` | Version control & worktrees |

Optional: [`bat`](https://github.com/sharkdp/bat) for syntax-highlighted viewing (`bat docs/workflow-cheatsheet.md`).

---

## Pipeline Overview

```
 PLANNING (spec-kitty)                    EXECUTION (kasmos + spec-kitty)
─────────────────────────                ──────────────────────────────────
 1. specify                               7. implement
 2. clarify       (optional)              8. launch (kasmos)
 3. plan                                  9. monitor & interact
 4. research      (optional)             10. review
 5. tasks                                11. accept
 6. analyze       (optional)             12. merge
```

**Flow**: `specify → [clarify] → plan → [research] → tasks → [analyze] → implement → launch → monitor → review → accept → merge`

Steps in brackets are optional. All others are required.

---

## Phase Reference

### Planning Phases

#### 1. Specify

> Create the feature specification from a natural language description.

| | |
|---|---|
| **Command** | `/spec-kitty.specify` |
| **Input** | Feature description (text, or empty for interactive interview) |
| **Output** | `kitty-specs/###-feature/spec.md`, `meta.json` |
| **Prerequisites** | spec-kitty initialized (`spec-kitty init --here --ai claude`) |

#### 2. Clarify *(optional)*

> Probe the spec for ambiguities via structured Q&A.

| | |
|---|---|
| **Command** | `/spec-kitty.clarify` |
| **Input** | Existing `spec.md` |
| **Output** | Updated `spec.md` with `## Clarifications` section |
| **Prerequisites** | `/spec-kitty.specify` completed |

#### 3. Plan

> Generate the implementation plan with technical architecture and design artifacts.

| | |
|---|---|
| **Command** | `/spec-kitty.plan` |
| **Input** | `spec.md` (required), constitution (if exists) |
| **Output** | `plan.md`, optionally `research.md`, `data-model.md`, `contracts/` |
| **Prerequisites** | `/spec-kitty.specify` completed |

#### 4. Research *(optional)*

> Run Phase 0 research to scaffold investigation artifacts before task planning.

| | |
|---|---|
| **Command** | `/spec-kitty.research` |
| **Input** | `plan.md` with unresolved technical questions |
| **Output** | `research.md` with decisions, rationale, alternatives |
| **Prerequisites** | `/spec-kitty.plan` completed |

#### 5. Tasks

> Generate work packages with subtasks and prompt files for implementation.

| | |
|---|---|
| **Command** | `/spec-kitty.tasks` |
| **Input** | `spec.md`, `plan.md` (required); `research.md`, `data-model.md` (optional) |
| **Output** | `tasks.md`, `tasks/WP##-*.md` prompt files |
| **Prerequisites** | `/spec-kitty.plan` completed |

#### 6. Analyze *(optional)*

> Cross-artifact consistency and quality check across spec, plan, and tasks.

| | |
|---|---|
| **Command** | `/spec-kitty.analyze` |
| **Input** | `spec.md`, `plan.md`, `tasks.md` |
| **Output** | Consistency report (terminal output) |
| **Prerequisites** | `/spec-kitty.tasks` completed |

---

### Execution Phases

#### 7. Implement

> Create an isolated worktree for a work package.

| | |
|---|---|
| **Command** | `/spec-kitty.implement WP##` |
| **Input** | Work package prompt file (`tasks/WP##-*.md`) |
| **Output** | `.worktrees/###-feature-WP##/` directory with isolated git branch |
| **Key flags** | `--base WP##` for dependent work packages |

```bash
# No dependencies:
/spec-kitty.implement WP01

# Depends on WP01:
/spec-kitty.implement WP02 --base WP01
```

#### 8. Launch

> Start Zellij orchestration session with agent panes.

| | |
|---|---|
| **Command** | `kasmos launch <feature_dir> [--mode continuous\|wave-gated]` |
| **Input** | Feature directory with work packages |
| **Output** | Zellij session with controller pane + agent grid |
| **Default mode** | `continuous` |

```bash
kasmos launch <feature_dir>                       # continuous (default)
kasmos launch <feature_dir> --mode wave-gated     # wave-gated
```

#### 9. Monitor & Interact

> Navigate Zellij panes and issue commands during orchestration.

**Pane navigation**: `Ctrl+p` → `h/j/k/l` or `Tab`
**Fullscreen toggle**: `Ctrl+p` → `f`

Key FIFO commands (write to `.kasmos/cmd.pipe`):
```bash
echo "status"              > .kasmos/cmd.pipe   # Show state
echo "advance"             > .kasmos/cmd.pipe   # Next wave (wave-gated)
echo "restart WP01"        > .kasmos/cmd.pipe   # Restart failed WP
echo "force-advance WP01"  > .kasmos/cmd.pipe   # Skip failed WP
echo "abort"               > .kasmos/cmd.pipe   # Graceful shutdown
```

Full command list → [cheatsheet.md](./cheatsheet.md) | Keybinds → [keybinds.md](./keybinds.md)

#### 10. Review

> Structured code review and kanban lane transitions for completed work packages.

| | |
|---|---|
| **Command** | `/spec-kitty.review` |
| **Input** | Work package in `for_review` lane |
| **Output** | Feedback in prompt file, lane moved to `done` or back to `doing` |
| **Prerequisites** | Implementation committed in worktree |

#### 11. Accept

> Validate feature readiness before merging to main.

| | |
|---|---|
| **Command** | `/spec-kitty.accept` |
| **Input** | All work packages in `done` lane |
| **Output** | Acceptance report |
| **Prerequisites** | All WPs reviewed and passed |

#### 12. Merge

> Merge completed feature into main branch and clean up worktrees.

| | |
|---|---|
| **Command** | `/spec-kitty.merge` |
| **Input** | Accepted feature |
| **Output** | Feature merged to target branch, worktrees removed |
| **Prerequisites** | `/spec-kitty.accept` passed |

---

## Kasmos Orchestration

### Continuous Mode (default)

Waves execute automatically — no operator intervention between waves.

```
kasmos launch <dir>
  → Wave 0 starts (parallel WPs)
  → Wave 0 completes → Wave 1 auto-launches
  → ... → All waves done
  → Orchestration completes
```

### Wave-Gated Mode

Operator confirms each wave advancement.

```
kasmos launch <dir> --mode wave-gated
  → Wave 0 starts
  → Wave 0 completes → Orchestrator pauses
  → Operator reviews results
  → echo "advance" > .kasmos/cmd.pipe
  → Wave 1 launches
  → Repeat until done
```

### Recovery Actions

```bash
echo "restart <WP_ID>"        > .kasmos/cmd.pipe   # Restart failed WP
echo "retry <WP_ID>"          > .kasmos/cmd.pipe   # Re-run from scratch
echo "force-advance <WP_ID>"  > .kasmos/cmd.pipe   # Skip failed, unblock dependents
echo "pause <WP_ID>"          > .kasmos/cmd.pipe   # Pause running WP
echo "resume <WP_ID>"         > .kasmos/cmd.pipe   # Resume paused WP
```

### Other CLI Commands

```bash
kasmos status [feature_dir]   # Check orchestration state
kasmos attach <feature_dir>   # Reconnect to detached session
kasmos stop [feature_dir]     # Graceful shutdown
```

---

## Daily Session

1. **Resume session**: `kasmos attach <feature_dir>`
2. **Check status**: `kasmos status` or `echo "status" > .kasmos/cmd.pipe`
3. **Monitor agents**: Navigate panes — `Ctrl+p` then `h/j/k/l` or `Tab`
4. **Advance wave** *(wave-gated)*: `echo "advance" > .kasmos/cmd.pipe`
5. **Review completed WPs**: `/spec-kitty.review`
6. **End session**: Detach from Zellij — `Ctrl+o` then `d`

---

## See Also

- [Kasmos CLI & FIFO Cheatsheet](./cheatsheet.md) — Full command reference for kasmos operations
- [Getting Started](./getting-started.md) — Zellij primer and kasmos fundamentals
- [Keybinds Reference](./keybinds.md) — Zellij keyboard shortcuts
- [Architecture](./architecture.md) — kasmos internals and state machine design
