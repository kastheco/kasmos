# lifecycle operator guide

this document describes the operator-facing lifecycle model used by the tui and cli.

## layered lifecycle model

every task exposes two related pieces of lifecycle state:

- `status`: coarse progress used for scheduling and filtering (`ready`, `planning`, `implementing`, `reviewing`, `done`, `cancelled`)
- `execution_phase`: persisted fine-grained execution detail used for operator visibility

the main operator phrases are:

- `planned`
- `architecting`
- `wave N running`
- `waiting for confirmation`
- `fixing round N`
- `reviewing round N`
- `readiness review`

examples:

- `ready` + `planned`: planning is done and the task is queued for implementation
- `implementing` + `architecting`: the architect pass is still running before wave work starts
- `implementing` + `wave 2 running`: wave 2 coder work is active
- `implementing` + `waiting for confirmation`: the active wave finished and kasmos is waiting for the next explicit handoff
- `implementing` + `fixing round 3`: the fixer is applying round-3 review feedback
- `reviewing` + `reviewing round 3`: the reviewer is running round 3
- `reviewing` + `readiness review`: reviewer approved; the master agent is running the holistic readiness gate before the task transitions to `done`

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
- `readiness-approved`
- `readiness-changes --feedback ...`

`readiness-approved` and `readiness-changes` apply only while the task is in the `readiness_reviewing` execution phase (master agent active). they correspond to `readiness_approved` / `readiness_changes_requested` gateway signals.

## compatibility note for architect completion

the canonical operator-facing action name is `architect-finished`.

for compatibility with existing automation, the persisted gateway / signal wire name remains `elaborator_finished`. use the operator-facing name in docs, tui actions, and cli commands; treat `elaborator_finished` as a wire-level compatibility detail only.
