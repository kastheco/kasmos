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

## docs research

before guessing at kasmos behavior, signal semantics, config keys, or cli flags, call `mcp__kasmos__docs_search` first.
use `mcp__kasmos__docs_read` to fetch full docs when a match looks relevant.
the canonical wiki is https://kasmos.kasthe.co/docs/ — the mcp tools serve the same content and work offline inside the kasmos repo.
when acting as planner, coder, or reviewer, validate documented patterns, behavior, config keys, and signal types before deciding.
