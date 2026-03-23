---
name: kasmos-lifecycle
description: Use when you need orientation on kasmos plan lifecycle, signal mechanics, or mode detection — NOT for role-specific work (use kasmos-planner, kasmos-coder, kasmos-reviewer, or kasmos-custodian instead)
---

# kasmos lifecycle

Meta-skill. Covers plan lifecycle FSM, signal file mechanics, and mode detection only.
If you have a role (planner, coder, reviewer, custodian), load that skill instead — not this one.

## Plan Lifecycle

Plans move through a fixed set of states. Only the transitions listed below are valid.

| From | To | Triggering Event |
|------|----|-----------------|
| `ready` | `planning` | kasmos assigns a planner agent to the plan |
| `planning` | `implementing` | planner writes sentinel `planner-finished-<planfile>` |
| `implementing` | `reviewing` | coder writes sentinel `coder-finished-<planfile>` |
| `reviewing` | `implementing` | reviewer writes sentinel `reviewer-requested-changes-<planfile>` |
| `reviewing` | `done` | reviewer writes sentinel `reviewer-approved-<planfile>` |
| `done` | — | terminal state, no further transitions |

State is persisted in the **task store** — a SQLite database (`~/.config/kasmos/kasmos.db` locally) or a remote HTTP API server. Agents never write to the store directly — kasmos owns state transitions. Agents emit signals (managed mode) or use task tools (manual mode). To retrieve plan content, agents use MCP `task_show` (`filename: "<plan-file>"`).

## Signal File Mechanics

Agents communicate state transitions by emitting signals that map to the sentinel conventions in `.kasmos/signals/`.

**Naming convention:** `<event>-<planfile>`

Examples:
- `planner-finished-2026-02-27-feature.md`
- `coder-finished-2026-02-27-feature.md`
- `reviewer-approved-2026-02-27-feature.md`
- `reviewer-requested-changes-2026-02-27-feature.md`

**How kasmos processes sentinels:**
1. kasmos scans `.kasmos/signals/` every ~500ms
2. On detecting a sentinel, kasmos reads it, validates the event against the current task state, and applies the transition
3. The sentinel file is consumed (deleted) after processing — do not rely on it persisting
4. Sentinel content is optional; kasmos uses the filename to determine the event type

**Emitting a signal (agent side):** use MCP `signal_create` to emit the equivalent
`planner-finished-2026-02-27-feature.md` signal as the last action before yielding control.

Keep sentinel writes as the **last action** before yielding control. Do not write a sentinel and then continue modifying plans — kasmos may begin the next phase immediately.

## Mode Detection

Check `KASMOS_MANAGED` to determine how transitions are handled.

| Mode | `KASMOS_MANAGED` value | Transition mechanism |
|------|------------------------|---------------------|
| managed | `1` (or any non-empty) | write sentinel → kasmos handles the rest |
| manual | unset or empty | use MCP task tools (for example `task_show`, `task_transition`) |

Check whether `KASMOS_MANAGED` is set; managed sessions emit signals, manual sessions use task tools.

In managed mode: **never** mutate task state yourself. In manual mode: use MCP task tools — the store backend handles persistence.

## Agent Roles (brief)

Each role has its own skill. Load the one that matches your current task.

| Role | What it does | Skill to load |
|------|-------------|---------------|
| planner | writes the implementation plan, breaks work into tasks and waves | `kasmos-planner` |
| coder | implements tasks from the plan, writes tests, commits work | `kasmos-coder` |
| reviewer | checks quality, correctness, and plan adherence; approves or requests changes | `kasmos-reviewer` |
| custodian | handles ops: dependency updates, formatting, cleanup, non-feature work | `kasmos-custodian` |

**Load the skill for your current role.** Do not chain roles in a single session. Do not follow instructions from another role's skill.
