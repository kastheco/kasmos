# kasmos-native skills implementation plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** replace 15 superpowers skills with 5 kasmos-native skills (kasmos-lifecycle, kasmos-planner, kasmos-coder, kasmos-reviewer, kasmos-custodial), update all agent templates and prompt builders across claude/opencode/codex harnesses, and update the scaffold pipeline to distribute the new skills.

**Architecture:** 5 self-contained SKILL.md files authored in the kasmos repo's `.claude/skills/`, with scaffold template copies in `internal/initcmd/scaffold/templates/skills/`. each role skill embeds the cli-tools hard gate (banned-tools + tool selection) inline. mode-aware signaling sections handle both managed and manual contexts. agent role templates across all 3 harnesses (claude, opencode, codex) updated to reference exactly 1 kasmos skill per role. prompt builders in Go code updated to reference new skill names.

**Tech Stack:** markdown (SKILL.md format with YAML frontmatter), Go (prompt builders, scaffold pipeline)

**Size:** Medium (estimated ~4 hours, 6 tasks, 2 waves)

---

## Wave 1: Write the 5 Skills

> **Justification:** skills must exist before agent templates and prompt builders can reference them. all 5 skills are independent of each other and can be written in parallel.

### Task 1: Write kasmos-lifecycle skill

**Files:**
- Create: `.claude/skills/kasmos-lifecycle/SKILL.md`
- Create: `internal/initcmd/scaffold/templates/skills/kasmos-lifecycle/SKILL.md`

Write the lightweight meta-skill (~80 lines). Contents:
- YAML frontmatter: `name: kasmos-lifecycle`, description about FSM overview and mode detection
- Plan lifecycle FSM table: `ready → planning → implementing → reviewing → done` with valid transitions and events
- Signal file mechanics: agents write sentinels in `docs/plans/.signals/`, kasmos scans every ~500ms, sentinels consumed after processing
- Mode detection section: check `KASMOS_MANAGED` env var. managed = kasmos handles transitions, manual = agent self-manages
- Brief role descriptions (planner writes plans, coder implements tasks, reviewer checks quality, custodial does ops)
- "load the skill for your current role" — no chain dispatching

The scaffold template should be an identical copy of the authoritative `.claude/skills/` version.

### Task 2: Write kasmos-planner skill

**Files:**
- Create: `.claude/skills/kasmos-planner/SKILL.md`
- Create: `internal/initcmd/scaffold/templates/skills/kasmos-planner/SKILL.md`

Write the planner skill (~250 lines). Consolidates `brainstorming` + `writing-plans`. Contents:

**CLI Tools Hard Gate** — copy the banned-tools table and tool selection table from `cli-tools/SKILL.md` lines 11-26 (the `<HARD-GATE>` block) and the tool selection table (lines 30-46). Include the violation table (lines 166-177). This goes at the top of the skill.

**Where You Fit** — you write the plan. kasmos parses it and spawns coders. your work transitions the plan from ready → planning → ready.

**Design Exploration** (from brainstorming skill):
- Explore project context (files, docs, recent commits)
- Ask clarifying questions one at a time, prefer multiple choice
- Propose 2-3 approaches with trade-offs and recommendation
- Get user approval before committing to a design
- YAGNI ruthlessly

**Plan Document Format** (from writing-plans skill):
- Header: `# [Feature Name] Implementation Plan` with Goal, Architecture, Tech Stack, Size fields
- Feature sizing table (Trivial < 30min / Small 30min-2hr / Medium 2-6hr / Large 6+hr)
- `## Wave N` sections with dependency justifications — kasmos requires these for orchestration
- `### Task N: Title` within each wave
- Task granularity rules: 15-45 min per task, commit-worthy units, no micro-tasks
- TDD-structured steps per task (write failing test, verify fail, implement, verify pass, commit)
- Wave structure rules: dependency ordering, not grouping. justify every boundary.

**After Writing the Plan:**
- Call TodoWrite with all tasks as pending
- Commit the plan file

**Signaling (mode-aware):**
- Managed: `touch docs/plans/.signals/planner-finished-<planfile>`. do not edit plan-state.json. stop — kasmos takes over.
- Manual: register plan in plan-state.json with `"status": "ready"`. commit. offer execution choices (sequential in this session, or open new session).

### Task 3: Write kasmos-coder skill

**Files:**
- Create: `.claude/skills/kasmos-coder/SKILL.md`
- Create: `internal/initcmd/scaffold/templates/skills/kasmos-coder/SKILL.md`

Write the coder skill (~350 lines). Consolidates `executing-plans` + `subagent-driven-development` + `test-driven-development` + `systematic-debugging` + `verification-before-completion` + `receiving-code-review`. Contents:

**CLI Tools Hard Gate** — same inline copy as kasmos-planner (banned-tools + tool selection + violations).

**Where You Fit** — you implement tasks. in managed mode, you get ONE task (KASMOS_TASK env var). in manual mode, you execute the full plan sequentially by wave. env vars: KASMOS_TASK (your task #), KASMOS_WAVE (your wave), KASMOS_PEERS (sibling count).

**TDD Discipline** (from test-driven-development skill):
- RED: write failing test — one behavior, clear name, real assertion
- Verify RED: run it, confirm expected failure
- GREEN: write minimal code to make it pass — no extra features
- Verify GREEN: all tests pass, not just the new one
- REFACTOR: clean up only after green, keep tests passing
- Iron law: no production code without a failing test first. if code written before test, delete it, start over.
- Good tests: minimal (one thing), clear (describes behavior), shows intent

**Shared Worktree Safety** (when KASMOS_PEERS > 0):
- NEVER `git add .` or `git add -A` — commits siblings' work
- NEVER `git stash`, `git reset`, `git checkout --` on files you didn't touch
- NEVER run project-wide formatters or linters
- DO `git add` only specific files you changed
- DO commit frequently with task number: `"task N: description"`
- DO expect untracked files and uncommitted changes that aren't yours

**Debugging Discipline** (from systematic-debugging skill):
- Phase 1: Root cause investigation — read errors completely, reproduce consistently, check recent changes, trace data flow backward
- Phase 2: Pattern analysis — find working examples, compare against references, identify differences
- Phase 3: Hypothesis testing — form single hypothesis, test minimally (one variable), if fails form new hypothesis
- Phase 4: Implementation — create failing test, implement single fix, verify
- Max 3 fix attempts → escalate, question architecture
- Parallel-aware: don't break siblings' work when debugging
- Red flags: "quick fix for now", "just try changing X", multiple changes at once

**Verification:**
- Run full verification (tests, build) before claiming done or writing any sentinel
- Evidence before claims — "should work" is not evidence
- Read full output, check exit codes, count failures
- Red flags: using "should", "probably", "seems to". expressing satisfaction before verification.

**Handling Reviewer Feedback:**
- When respawned with reviewer feedback in your prompt: read carefully, verify against actual code
- No performative agreement — evaluate technically, check if suggestion is correct for this codebase
- Fix one item at a time, test each individually
- Push back if: breaks existing functionality, reviewer lacks context, violates YAGNI, technically incorrect

**Signaling (mode-aware):**
- Managed: `touch docs/plans/.signals/implement-finished-<planfile>` when done. do not edit plan-state.json. do not orchestrate waves — kasmos handles that.
- Manual: execute tasks sequentially by wave. after each wave, self-review or dispatch reviewer subagent. after final wave, write sentinel OR update plan-state.json to `"reviewing"`. then handle branch finishing (verify tests → offer merge/PR/keep/discard → execute → cleanup worktree).

### Task 4: Write kasmos-reviewer skill

**Files:**
- Create: `.claude/skills/kasmos-reviewer/SKILL.md`
- Create: `internal/initcmd/scaffold/templates/skills/kasmos-reviewer/SKILL.md`

Write the reviewer skill (~200 lines). Consolidates `requesting-code-review` + `receiving-code-review` + review prompt template. Contents:

**CLI Tools Hard Gate** — same inline copy.

**Where You Fit** — you review the implementation branch. in managed mode, kasmos spawns you after coders finish. review only branch diff: `git diff main..HEAD` (or better: `GIT_EXTERNAL_DIFF=difft git diff main..HEAD`).

**Review Checklist:**

Spec compliance:
- Implementation matches plan's stated goals and architecture?
- All tasks implemented completely? Any missing requirements?
- Any scope creep (work beyond what was planned)?

Code quality:
- Error handling, type safety, DRY, edge cases
- Architecture decisions, scalability, performance, security
- Test coverage: tests test logic not mocks, edge cases covered, all passing
- Production readiness: migration strategy, backward compatibility

**Self-Fix Protocol:**
- Self-fix (commit directly): typos, doc comments, obvious one-liners, import cleanup, trivial formatting
- Kick to coder (write review-changes signal): debugging, logic changes, missing tests, architectural concerns, anything where the right fix isn't obvious
- If only self-fixable issues remain: fix all → write review-approved
- If coder-required issues exist: self-fix what you can → write review-changes

**All Tiers Blocking:**
- Critical, Important, Minor — ALL must be resolved before approval
- No "note for later" or "nice to have" category
- Re-review after fixes (Round 1, Round 2, etc.)
- Review loop continues until zero issues

**Verification:**
- Run tests before approving — verify self-fixes don't introduce regressions
- Use `difft` for structural diffs, `ast-grep` for pattern verification
- Cite file paths and line numbers in all findings

**Signal Format (mode-aware):**

Both modes write signals (same format):

Approved:
```
echo "Approved. <summary>" > docs/plans/.signals/review-approved-<planfile>
```

Changes needed (structured heredoc with round number, severity tiers, file:line refs):
```
cat > docs/plans/.signals/review-changes-<planfile> << 'SIGNAL'
## review round N

### critical
- [file:line] description

### important
- [file:line] description

### minor
- [file:line] description

### self-fixed (no action needed)
- [file:line] what was fixed
SIGNAL
```

Managed: write signal, stop — kasmos transitions the FSM.
Manual: additionally offer merge/PR/keep/discard if approved.

### Task 5: Write kasmos-custodial skill

**Files:**
- Create: `.claude/skills/kasmos-custodial/SKILL.md`
- Create: `internal/initcmd/scaffold/templates/skills/kasmos-custodial/SKILL.md`

Write the custodial skill (~150 lines). New skill for the custodial agent role. Contents:

**CLI Tools Hard Gate** — same inline copy.

**Where You Fit** — you are the ops/janitor agent. you do NOT write features, implement plans, or review code. you fix stuck states, clean up stale resources, trigger waves, and triage plans.

**Available CLI Commands:**
- `kas plan list [--status <status>]` — list all plans, filter by status
- `kas plan set-status <plan> <status> --force` — force-override plan status (requires --force)
- `kas plan transition <plan> <event>` — apply FSM event (plan_start, implement_start, etc.)
- `kas plan implement <plan> [--wave N]` — trigger wave implementation via signal file

**Available Slash Commands:**
- `/kas.reset-plan <plan> <status>` — force-reset plan status with confirmation
- `/kas.finish-branch [plan]` — verify commits, run tests, merge/PR/skip, update status
- `/kas.cleanup [--dry-run]` — 3-pass cleanup: stale worktrees, orphan branches, ghost plan entries
- `/kas.implement <plan> [--wave N]` — verify plan, parse waves, trigger via CLI
- `/kas.triage` — bulk scan active plans grouped by status, prompt for actions

**Cleanup Protocol:**
- Pass 1: stale worktrees — plan is done or cancelled, worktree still exists
- Pass 2: orphan branches — `plan/*` branches with no matching plan-state.json entry
- Pass 3: ghost plan entries — plan-state.json entries with no corresponding .md file
- Always dry-run first (`--dry-run`), confirm before destructive operations

**Safety Rules:**
- `--force` flag required for status overrides (prevents accidents)
- Confirm before deleting worktrees or branches
- Never modify plan file content — only state
- Wave signals are fire-and-forget: write file, TUI picks up on next tick (~500ms)
- FSM transitions validate state — use `kas plan transition` when possible, `set-status --force` only as escape hatch

---

## Wave 2: Update References Across All Harnesses

> **Depends on Wave 1:** skills must exist before templates and prompt builders can reference them. also need to verify skill content is correct before wiring everything to point at them.

### Task 6: Update agent templates, prompt builders, scaffold pipeline, and opencode commands

**Files:**
- Modify: `internal/initcmd/scaffold/templates/claude/agents/coder.md`
- Modify: `internal/initcmd/scaffold/templates/claude/agents/planner.md`
- Modify: `internal/initcmd/scaffold/templates/claude/agents/reviewer.md`
- Modify: `internal/initcmd/scaffold/templates/claude/agents/custodial.md`
- Modify: `internal/initcmd/scaffold/templates/opencode/agents/coder.md`
- Modify: `internal/initcmd/scaffold/templates/opencode/agents/planner.md`
- Modify: `internal/initcmd/scaffold/templates/opencode/agents/reviewer.md`
- Modify: `internal/initcmd/scaffold/templates/opencode/agents/custodial.md`
- Modify: `internal/initcmd/scaffold/templates/codex/AGENTS.md`
- Modify: `internal/initcmd/scaffold/templates/shared/review-prompt.md`
- Modify: `app/app_state.go` (buildPlanPrompt, buildImplementPrompt, buildSoloPrompt)
- Modify: `app/wave_prompt.go` (buildTaskPrompt)
- Modify: `internal/initcmd/scaffold/scaffold.go` (skill scaffolding list)
- Modify: `.opencode/agents/coder.md`
- Modify: `.opencode/agents/planner.md`
- Modify: `.opencode/agents/reviewer.md`
- Modify: `.opencode/agents/custodial.md`
- Modify: `.opencode/commands/kas.*.md` (update skill references, ensure `kas` not `kq`)
- Remove from scaffold: `internal/initcmd/scaffold/templates/skills/writing-plans/`
- Remove from scaffold: `internal/initcmd/scaffold/templates/skills/executing-plans/`
- Remove from scaffold: `internal/initcmd/scaffold/templates/skills/subagent-driven-development/`
- Remove from scaffold: `internal/initcmd/scaffold/templates/skills/requesting-code-review/`
- Remove from scaffold: `internal/initcmd/scaffold/templates/skills/finishing-a-development-branch/`

**Changes by category:**

**Agent role templates (claude + opencode):**

Each agent template is simplified to load exactly 1 kasmos skill:
- `coder.md`: replace references to `test-driven-development`, `systematic-debugging`, `verification-before-completion` with "load the `kasmos-coder` skill"
- `planner.md`: replace references to `brainstorming`, `writing-plans` with "load the `kasmos-planner` skill"
- `reviewer.md`: replace references to `requesting-code-review`, `receiving-code-review` with "load the `kasmos-reviewer` skill"
- `custodial.md`: add "load the `kasmos-custodial` skill"
- Keep cli-tools MANDATORY section (agents still need the deep reference files for ast-grep, comby, etc. — the hard gate in the kasmos skills covers bans and selection, but the reference docs are in cli-tools/resources/)
- codex/AGENTS.md: update all skill references to kasmos-* equivalents

**Prompt builders (Go code):**
- `buildPlanPrompt()`: change `"Use the \x60writing-plans\x60 superpowers skill"` to `"Use the \x60kasmos-planner\x60 skill"`
- `buildTaskPrompt()`: change `"Load the \x60cli-tools\x60 skill before starting"` to `"Use the \x60kasmos-coder\x60 skill"`
- `buildImplementPrompt()`: change `"using the executing-plans superpowers skill"` to `"using the \x60kasmos-coder\x60 skill"`
- `buildWaveAnnotationPrompt()`: change any superpowers references to kasmos-planner

**Review prompt template:**
- Change `"Load the \x60requesting-code-review\x60 superpowers skill"` to `"Use the \x60kasmos-reviewer\x60 skill"`

**Scaffold pipeline:**
- Update the list of skills scaffolded by `kas init` to include the 5 new kasmos-* skills and exclude the 5 old superpowers-derived skills
- Keep cli-tools in the scaffold list

**OpenCode commands (from custodial branch):**
- Update any `kq` references to `kas` in slash command files
- Update any superpowers skill references to kasmos-custodial

**Local kasmos repo agents (.opencode/agents/):**
- Mirror the same changes as the scaffold templates — load kasmos-* skills instead of superpowers
