# kasmos Agents

## Coder
Implementation agent. Writes code, fixes bugs, runs tests.
Follow TDD: write failing test first, implement, verify green.
Prefer the bundled project skills that still exist in this repo (`cli-tools`, `kasmos-coder`, `kasmos-cli`, `kasmos-lifecycle`, `tui-design`).

## Reviewer
Review agent. Checks quality, security, spec compliance.
Use `difft` for structural diffs (not line-based `git diff`).
Use `sg` (ast-grep) to verify patterns across the codebase.
Prefer current bundled skills such as `cli-tools`, `kasmos-reviewer`, `kasmos-lifecycle`, and `tui-design` when the task matches them.

## Planner
Planning agent. Writes specs, plans, decomposes work into packages.
Use `scc` for codebase metrics when scoping work.
Prefer current bundled skills such as `cli-tools`, `kasmos-planner`, `kasmos-architect`, and `tui-design`.

## Task Store (CRITICAL)
Task state lives in the task store — a project-local SQLite database (`<repo-root>/.kasmos/taskstore.db`) or a remote HTTP API.
Use `kas task` CLI commands for all lifecycle operations:
- `kas task list` — list tasks and statuses
- `kas task show <task-file>` — read task content
- `kas task create <name>` — create a new task
- `kas task register <task-file>` — register a task file from disk
- `kas task update-content <task-file>` — update task content
- `kas task transition <task-file> <event>` — FSM state transition
- `kas task set-status <task-file> <status> --force` — force override
Never modify task state directly. Unregistered tasks are invisible in the kasmos sidebar.
Valid statuses: `ready` → `planning` → `implementing` → `reviewing` → `done` (plus terminal `cancelled`).

## CLI Tools

Read the bundled `cli-tools` skill (SKILL.md) at session start. Read individual
resource files in `resources/` when using that specific tool.
