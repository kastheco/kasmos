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
| `planning` | `implementing` | planner emits signal `planner-finished-<planfile>` |
| `implementing` | `reviewing` | coder emits signal `implement-finished-<planfile>` |
| `reviewing` | `implementing` | reviewer emits signal `review-changes-<planfile>` |
| `reviewing` | `verifying` | reviewer emits signal `review-approved-<planfile>` |
| `verifying` | `done` | master emits signal `verify-approved-<planfile>` |
| `verifying` | `implementing` | master emits signal `verify-failed-<planfile>` |
| `done` | — | terminal state, no further transitions |

State is persisted in the **task store** — a SQLite database (`~/.config/kasmos/taskstore.db` locally) or a remote HTTP API server. Agents never write to the store directly — kasmos owns state transitions. Agents emit signals (managed mode) or use task tools (manual mode). To retrieve plan content, agents use MCP `task_show` (`filename: "<plan-file>"`, `project: "$KASMOS_PROJECT"`).

### Verifying State

`verifying` is a first-class FSM state between `reviewing` and `done`. When the reviewer emits `review-approved`, kasmos transitions the task to `verifying`. The master readiness agent then runs and issues one of two verdicts:

| Signal type (MCP) | CLI equivalent | Effect |
|-------------------|---------------|--------|
| `verify-approved` | `kas signal emit verify_approved <planfile>` | completes the task (transitions to `done`) |
| `verify-failed` | `kas signal emit verify_failed <planfile>` | sends the task back to `implementing` for fixes |

The `auto_readiness_review` config key controls whether the master agent is spawned. When `auto_readiness_review` is disabled, kasmos immediately chains `verify-approved` after `review-approved` without spawning the master agent — the task transitions directly to `done`.

**Compatibility aliases**: `readiness-approved` and `readiness-changes-requested` are accepted as deprecated aliases for `verify-approved` and `verify-failed` respectively. New code must use the canonical `verify-*` names.

## Signal Mechanics

Signals are gateway-backed. Agents emit signals via the DB-backed signal gateway — `.kasmos/signals/` sentinel files still exist for compatibility but are not the primary path.

**Primary path — MCP `signal_create`:**

Use MCP `signal_create` with `signal_type`, `plan_file`, and `project: "$KASMOS_PROJECT"` to emit signals. This writes directly to the signal gateway.

**Fallback — `kas signal emit`:**

```bash
kas signal emit <signal_type> <planfile>
```

**Last-resort fallback — sentinel files:**

Write a sentinel file to `.kasmos/signals/` only when MCP and CLI are both unavailable. Naming convention: `<event>-<planfile>`.

Examples:
- `planner-finished-2026-02-27-feature.md`
- `implement-finished-2026-02-27-feature.md`
- `review-approved-2026-02-27-feature.md`
- `review-changes-2026-02-27-feature.md`

**How kasmos processes signals:**
1. The gateway receives signals via MCP, CLI, or sentinel file scan (~500ms)
2. kasmos validates the event against the current task state and applies the transition
3. Sentinel files are consumed (deleted) after processing — do not rely on them persisting

Keep signal emission as the **last action** before yielding control. Do not emit a signal and then continue modifying plans — kasmos may begin the next phase immediately.

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
