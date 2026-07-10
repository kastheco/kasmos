---
name: coordinate-kasmos
description: Coordinate guarded, headless software-delivery work through the Kasmos MCP server. Use when Codex Desktop or an OpenClaw agent needs to research, plan, implement, monitor, recover, or report on a code or configuration change managed by Kasmos, including when a Workboard card should track a Kasmos task. Do not use for ordinary operational campaign work such as finding jobs, ranking leads, outreach, or filling applications unless the request is to change the software that performs that work.
---

# Coordinate Kasmos

Use Kasmos as the source of truth for software-delivery state. Use Workboard only to track the surrounding operational workflow and human checkpoints.

## Authority boundary

- Use only the Kasmos MCP tools exposed to this agent. Codex Desktop names them `mcp__kasmos__*`; OpenClaw names them `kasmos__*`. Never shell out to `kas`, edit Kasmos state files, or write signal files.
- Treat task creation, plan start, and read-only inspection as authorized when the owner asks to plan or build a concrete change.
- Require the owner's explicit approval of the current plan before calling `task_transition` with `implement_start`.
- A materially changed plan invalidates prior implementation approval. Show the changed sections and ask again.
- Never delete a task, start over, reimplement, cancel, pause or resume an instance, send text to an instance, merge, commit, push, deploy, migrate, or restart a service unless the owner explicitly requests that action.
- Do not expose arbitrary repository control to another agent. Operate only on the named project and task.

## Choose the system

- Use Kasmos for changes to source code, tests, infrastructure-as-code, agent workspaces, OpenClaw plugins, skills, or versioned configuration.
- Use Workboard for recurring searches, lead and job campaigns, approvals, outreach, application processing, and other operational queues.
- When both apply, keep implementation detail and lifecycle in Kasmos. Put the Kasmos project, task filename, current phase, approval state, and verification summary on the Workboard card. Do not copy the full plan into both systems.
- For Jobs, keep the Greenhouse application pipeline and job-search/outreach campaigns in Workboard. Use Kasmos only to build or change the software behind those workflows.

## Start or resume work

1. Call the Kasmos `daemon_status` MCP tool. Stop and report the exact failure if the daemon is unavailable or the requested repository is not registered.
2. Resolve the project explicitly. Never infer it only from the current OpenClaw workspace when multiple repositories are registered.
3. Call the Kasmos `task_list` MCP tool for that project before creating anything. Reuse an existing task when its scope matches; do not create duplicate tasks after a session interruption.
4. For an existing task, use its `task_list` entry for lifecycle status, then call the Kasmos `task_show` MCP tool for the current plan content and continue from that state.
5. For a new task, call the Kasmos `task_create` MCP tool once with a concise description, stable filename, project, and topic when known. Include full plan content only when the owner supplied an already-approved, implementation-ready plan.
6. Start research and planning with the Kasmos `task_transition` MCP tool using `event: "plan_start"` when the task is a draft. Planning is not implementation approval.

Prefer a topic that groups changes likely to touch the same code. This lets Kasmos avoid unsafe concurrent work in one code region.

## Planning checkpoint

Poll lifecycle status with the Kasmos `task_list` MCP tool; use `instance_list` only when task state is insufficient. Do not busy-loop. Once the task leaves `planning`, call `task_show` to read the full current plan before requesting implementation approval.

When planning finishes:

1. Read the full current plan.
2. Summarize scope, waves, affected surfaces, verification, destructive or external effects, and unresolved decisions.
3. Present the exact task filename and plan revision/state.
4. Ask for explicit approval to implement that plan.
5. If a Workboard card is tracking the request, block it on owner approval and record the task filename. Resume it only after approval is recorded.

Do not treat approval to research, create a plan, or create a Workboard card as approval to implement.

## Implementation and monitoring

After explicit approval, re-read the task and verify it is still the approved plan, then call the Kasmos `task_transition` MCP tool using `event: "implement_start"`.

- Let Kasmos own architect elaboration, coder waves, review, fixes, and readiness review.
- Observe through the Kasmos `task_show`, `instance_list`, and `capture_pane` MCP tools when a live instance needs diagnosis.
- Do not manually emit lifecycle signals for healthy agents. Use the Kasmos `signal_create` MCP tool only to recover a known missing signal after confirming the agent completed the required work and the expected signal is absent.
- Do not use the Kasmos `instance_send` MCP tool to steer implementation unless the owner asked for intervention or the agent is visibly blocked on a factual clarification that cannot be handled through the task.
- Report blockers promptly instead of repeatedly retrying a failed transition.

## Completion

Completion means Kasmos reached its terminal reviewed state, not merely that a coder stopped.

Report:

- project and task filename
- final Kasmos state
- branches or worktrees involved
- review and readiness outcome
- verification commands and results reported by Kasmos
- remaining risks or manual actions
- whether any commit, push, PR, deploy, migration, or restart still requires approval

Update a tracking Workboard card with a short status and evidence. Do not mark the operational card complete when a separate human action remains.

## Documentation-drift gate

Before **every commit** that changes a Kasmos repository, stage the intended commit, then run the repository's documentation-drift detector against the staged changes:

```sh
BASE_REF="$(git merge-base HEAD origin/main)" TARGET_REF=INDEX bash scripts/detect-docs-drift.sh
```

- If the report lists `docs_not_changed`, update each mapped document, stage the updates, and rerun the detector until it reports an empty drift list.
- Treat orchestration, lifecycle, daemon, MCP, CLI, config, and scaffold changes as likely mapped code. Inspect the detector output proactively; do not wait for the pre-push hook or CI to catch it.
- Include the clean detector command and result in the commit/PR verification summary. The pre-push hook is a backstop, not the first drift check.

## Failure and recovery

- If the daemon is down, report it; do not restart it automatically.
- If the repository is unregistered, report the exact path needed; do not register an arbitrary path.
- If a transition is invalid, re-read the task and follow its actual state instead of forcing status.
- If a planner or worker appears stuck, inspect the instance and audit state before proposing pause, resume, or signal recovery.
- After interruption, resume by project plus task filename. Never create a replacement task solely because the OpenClaw session lost context.
