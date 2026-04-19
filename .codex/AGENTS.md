# kasmos Agents

## Coder
Implementation agent. Writes code, fixes bugs, runs tests.

## Reviewer
Review agent. Checks quality, security, spec compliance.
Use `difft` for structural diffs (not line-based `git diff`).
Use `sg` (ast-grep) to verify patterns across the codebase.
Load the `kasmos-reviewer` skill.

## Planner
Planning agent. Writes specs, plans, decomposes work into packages.
Use `scc` for codebase metrics when scoping work.
Load the `kasmos-planner` skill.

## Task State (CRITICAL)
Task state is stored in the **global task store** (`~/.config/kasmos/taskstore.db` by default, or a remote http api), not in files on disk.
Prefer mcp task tools for task-state work: `task_create`, `task_show`, `task_update_content`, `task_list`, and `task_transition`.
Use `kas task ...` only when mcp is genuinely unavailable or a workflow has no mcp equivalent. Do not write sentinel files in normal operation.
If you need to create a brand-new standalone task, prefer mcp `task_create` with `content`; `kas task register <plan>.md` is the cli fallback for legacy/manual environments.
Valid statuses: `ready` → `planning` → `implementing` → `reviewing` → `verifying` → `done` (plus `cancelled`).

## cli tools

Read the `cli-tools` skill (SKILL.md) at session start. Read individual
resource files in `resources/` when using that specific tool.

## GitHub PR reviews

When addressing GitHub PR review comments, completion requires all of the following:

- implement and verify the requested fixes locally
- commit the changes
- push the branch
- reply inline on GitHub to each addressed review thread with a concise resolution note

Do not report PR review work as finished before the branch is pushed and the inline replies are posted.
