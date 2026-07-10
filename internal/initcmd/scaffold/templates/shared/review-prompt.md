Review the implementation of plan: {{PLAN_NAME}}

Load the `kasmos-reviewer` skill before starting. Treat that skill as the authoritative
static review workflow; do not restate or load overlapping review instructions.

## Dynamic context

- Plan file: `{{PLAN_FILE}}`
- Project: `{{PROJECT}}`
- Current review round: {{CURRENT_REVIEW_ROUND}}

Retrieve the plan with MCP `task_show` (filename: "{{PLAN_FILE}}", project: "{{PROJECT}}").
Use `kas task show {{PLAN_FILE}}` only when MCP is unavailable.

## Round scope

- Round 1: perform the full first-pass review defined by the skill.
- Round 2+: perform a targeted re-review. Verify the previous findings in their cited
  files and affected tests, then inspect only fixer-touched surfaces needed to detect
  regressions. Do not repeat the full first-pass review or full-suite verification unless
  fixes changed a shared/integration boundary or the plan's verification checks require it.

### Previous round findings

{{PREVIOUS_REVIEW_CONTEXT}}

## Completion

Emit exactly one gateway signal using the skill's output contract:

- `signal_create` (signal_type: "review-approved", plan_file: "{{PLAN_FILENAME}}", project: "{{PROJECT}}")
- `signal_create` (signal_type: "review-changes", plan_file: "{{PLAN_FILENAME}}", project: "{{PROJECT}}")

Keep the payload limited to the decision, verification evidence, and actionable findings
needed by the next role. Then stop.
