---
name: monitor-kasmos-task
description: Watch an already-running Kasmos task cheaply, stay silent while it is healthy, and escalate only on actionable change. Do not use for creating, planning, approving, implementing, or steering work; use coordinate-kasmos for those actions.
---

# Monitor Kasmos Task

Watch one already-running Kasmos task with a bounded, read-only loop. Stay silent while its stable digest remains healthy and hand actionable changes to a frontier model.

## Inputs

Require both inputs before setup:

| input | rule |
| --- | --- |
| project | Use the explicit registered Kasmos project. Refuse to infer it when multiple repositories are registered. |
| task | Use the exact task filename, not a title or guessed slug. |

Call `daemon_status` once during setup. If the daemon is down, report and stop; never start it. If the repository is unregistered, report the exact path that must be registered and stop; never register it.

## Contract pin

This first YAML block is machine-checked by the repository contract tests. Update it with the live contract, never around the tests.

```yaml
live_status_schema_version: 1
idle_tools: [live_status, task_list]
escalation_tools: [task_show, instance_list, capture_pane]
```

## Model routing

Run the watch loop on `gpt-5.6-luna` with low reasoning. Route escalated work to `gpt-5.6-sol` with low reasoning. This skill cannot change the model of an already-running turn. If the host cannot honor the watch-loop override, say so and tell the operator to start this exact handoff: a dedicated `gpt-5.6-sol` low-reasoning thread containing only the escalation packet.

## Idle tool budget

Each poll uses exactly `live_status` and `task_list`. Call `task_list` without the `status` argument: a filtered list hides the task as soon as it transitions, while `store.List` already returns `done` and `cancelled` rows. While idle, pane capture, `task_show`, source or git inspection, and diffs are forbidden.

## The digest

Diffing the raw payload fires every tick because `generated_at` and `uptime` always move. Reduce each poll for target task `T` to these stable fields:

| digest field | source |
| --- | --- |
| `status(T)` | the unfiltered `task_list` entry whose `filename == T` (absent ⇒ `missing`) |
| `agents(T)` | `live_status.active_agents` filtered to `task == T`, as an ordered list of `(role, wave, stage)` |
| `attention(T)` | `live_status.attention` filtered to `task == T`, as an ordered list of `(kind, detail)` |
| `daemon_running` | `live_status.daemon_running` |
| `reliable` | `false` when the snapshot was truncated after the retry below, else `true` |

Explicitly exclude `generated_at`, `uptime`, `repo_count`, project-wide `lifecycle` counts, and every `active_agents` or `attention` entry for another task. Other tasks' churn must never wake this watcher.

## Truncation rule

Check `truncated.active_agents` and `truncated.attention`. If either is greater than zero, re-poll `live_status` once with `cap: 100`. If either remains truncated, set `reliable: false`: trust only positive observations, never infer that attention cleared or agents disappeared, and suppress the stall heuristic for that poll.

## Trigger table

| digest observation | classification | watcher action |
| --- | --- | --- |
| digest identical to previous | passive | emit nothing; sleep |
| only excluded/volatile fields differ | passive | emit nothing; sleep |
| another task's agents/attention changed | passive | emit nothing; sleep |
| `agents(T)` `stage` churn only (e.g. `loading`→`running`), status unchanged, no new attention | passive | emit nothing; sleep |
| `status(T)` changed (non-terminal) | **actionable** | stop, escalate: lifecycle transition |
| `attention(T)` gains `needs_decision` | **actionable** | stop, escalate: approval/wave checkpoint (no `detail` exists — do not invent one) |
| `attention(T)` gains `review_feedback` | **actionable** | stop, escalate: unresolved review feedback (no `detail` exists — `sol` reads it via `task_show`) |
| `attention(T)` gains `stale_instance` | **actionable** | stop, escalate: include the `detail` health reason verbatim |
| `daemon_running` flips `true → false` | **actionable** | stop, escalate: daemon lost. Do **not** restart it |
| `status(T)` becomes `done` or `cancelled` | **actionable** | stop, escalate final report, self-disable |
| `T` absent from an unfiltered `task_list` | **actionable** | stop, escalate: task missing/deleted. Never create a replacement task |
| `status(T)` ∈ {`implementing`, `reviewing`, `verifying`}, zero agents for `T`, digest `reliable`, unchanged ≥ 5 consecutive polls, daemon up | **actionable (advisory)** | stop, escalate `suspected_stall` — label it a heuristic, never auto-act |
| `live_status` reports `schema_version != 1` | **actionable** | stop, escalate: contract moved; skill may be stale |
| MCP/tool error on 3 consecutive polls | **actionable** | stop, escalate: monitoring unavailable |
| poll budget exhausted | **actionable** | stop, escalate: watch budget spent, ask whether to resume |

`stuck` is deliberately absent from v1. `suspected_stall` is a conservative consumer heuristic, suppressed for unreliable digests and always labelled advisory.

## Attention semantics

| kind | exact meaning | detail |
| --- | --- | --- |
| `needs_decision` | fires only at execution phase `wave_waiting` | none |
| `review_feedback` | fires only when review feedback is pending and status is `implementing` | none |
| `stale_instance` | fires for an instance health reason | the health reason; this is the only kind with `detail` |

Never fabricate detail that the contract does not supply.

## Cadence and budget

Poll every 60 seconds by default, with a hard floor of 30 seconds. `live-status-contract.mdx` recommends 1–5 seconds for a UI process that reuses snapshots, not for a model whose polls grow context. Stop after 120 polls (about two hours). Self-disable on terminal state, escalation, schema mismatch, or budget exhaustion.

## Mechanism ladder

Choose the first supported mechanism:

| priority | mechanism | requirement |
| --- | --- | --- |
| 1 | heartbeat | run on `gpt-5.6-luna` low with a per-heartbeat override |
| 2 | scoped automation | name it `kasmos-watch:<project>:<task>` and self-disable on escalation or terminal state |
| 3 | manual mode | run the loop in a dedicated cheap thread |

Allow one watcher per `(project, task)`. Enumerate watchers and adopt or refuse an existing deterministic name. If existing watchers cannot be enumerated, refuse to create one rather than risk a duplicate.

## Escalation packet

Send `gpt-5.6-sol` low only: project, task filename, trigger kind, previous digest → new digest, `detail` only when one exists, and one bounded next action. Include nothing else. Omitting plan bodies, pane captures, diffs, git state, raw JSON, and speculative evidence is intentional; `sol` pulls what it needs.

## Authority boundary

- **Allowed while watching:** `live_status`, `task_list`, `daemon_status`.
- **Escalation-only (i.e. `sol`'s, not the watcher's):** `task_show`, `instance_list`, `capture_pane`.
- **Never:** `task_transition`, `signal_create`, `instance_send`, `instance_pause`, `instance_resume`, `instance_restart`, `task_create`, `task_delete`, `task_update_content`, and any merge, commit, push, PR, deploy, migration, or service restart. A watcher never auto-approves implementation and never steers a healthy worker.

## Failure and recovery

| condition | response |
| --- | --- |
| daemon down | report and stop; never restart it |
| task missing | report and stop; never create a replacement |
| repository unregistered | report the exact path needed; never register it |
| tool errors | count consecutive failures; escalate and stop after three |
| interrupted session | resume by explicit project plus exact task filename; never create a replacement watcher blindly |

## Retirement

Retire this skill when the native event-driven Codex plugin monitor provides equivalent cost controls. Delete the polling-specific portions of this skill rather than maintaining them alongside the native monitor.
