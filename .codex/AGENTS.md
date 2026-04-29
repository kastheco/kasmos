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

## test philosophy

Tests should protect durable behavior contracts and user-visible workflows, not only historical symptoms. Plans, implementations, and reviews should name the invariant each new test protects and prefer extending existing invariant suites over adding one-off regression files.

## PR review fixes

When addressing GitHub PR review feedback, treat each comment as evidence of a broken invariant, not a line-local patch request. Before replying to or resolving a review thread, trace the affected value/control flow from the reviewed line to the final observable side effect, check adjacent sibling paths for the same failure class, and add or update regression coverage at the final behavior boundary whenever practical. Intermediate helper tests are not enough when a downstream path can overwrite, bypass, or reinterpret the fixed value.

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
