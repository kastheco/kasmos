# lifecycle operator guide

Remote audit polling is documented in the [remote task store guide](../web/docs/docs/guides/remote-task-store.mdx#audit-querying).

Wave failure audit details may include `retry_generation`. Operators can use this key to distinguish the first failure from later retry attempts; duplicate wave failures for the same generation are suppressed so Eagle Eye surfaces one actionable row per retry generation instead of repeated noise.

this document describes the operator-facing lifecycle model used by the tui and cli.

## layered lifecycle model

every task exposes two related pieces of lifecycle state:

- `status`: coarse progress used for scheduling and filtering (`ready`, `planning`, `implementing`, `reviewing`, `verifying`, `done`, `cancelled`)
- `execution_phase`: persisted fine-grained execution detail used for operator visibility

the main operator phrases are:

- `planned`
- `architecting`
- `wave N running`
- `waiting for confirmation`
- `fixing round N`
- `reviewing round N`
- `verifying`

examples:

- `ready` + `planned`: planning is done and the task is queued for implementation
- `implementing` + `architecting`: the architect pass is still running before wave work starts
- `implementing` + `wave 2 running`: wave 2 coder work is active
- `implementing` + `waiting for confirmation`: the active wave finished and kasmos is waiting for the next explicit handoff
- `implementing` + `fixing round 3`: the fixer is applying round-3 review feedback
- `reviewing` + `reviewing round 3`: the reviewer is running round 3
- `verifying`: reviewer approved; the master agent is running the holistic readiness gate before the task transitions to `done`

## where it shows up

- `kas status` text output and json output
- tui navigation row labels and plan icons
- tui info pane lifecycle, progress, and compact header summaries

## manual recovery paths

manual recovery is intentionally available in both primary operator surfaces.

### tui

select the instance or task, open the context menu, then use the `manage` actions:

- `mark planning finished`
- `mark architect finished`
- `mark implement finished`

review approval and changes-requested flows continue through the same operator actions exposed from review/fix management flows.

### cli

use `kas task recover <task-file> --action ...`.

supported actions:

- `planner-finished`
- `architect-finished`
- `implement-finished`
- `review-approved`
- `review-changes --feedback ...`
- `advance-review-cycle --feedback ...`
- `verify-approved`
- `verify-failed --feedback ...`

`verify-approved` and `verify-failed` apply only while the task is in the `verifying` status (master agent active). they correspond to `verify_approved` / `verify_failed` gateway signals.

## compatibility note for architect completion

the canonical operator-facing action name is `architect-finished`.

for compatibility with existing automation, the persisted gateway / signal wire name remains `elaborator_finished`. use the operator-facing name in docs, tui actions, and cli commands; treat `elaborator_finished` as a wire-level compatibility detail only.

## compatibility note for verifying signals

the canonical signal names for the master agent verdict are `verify_approved` and `verify_failed`.

deprecated aliases (`readiness_approved`, `readiness_changes_requested`, `readiness-approved`, `readiness-changes`, `readiness-changes-requested`, `master_approved`) are still accepted and canonicalized at ingress. use the canonical names in new automation and operator scripts.

- Docs drift prevention is automated via `.github/workflows/docs-drift.yml` (nightly + PR-triggered). See `docs/docs-mcp-tool.md` for the MCP tool that agents use to look up wiki content.
