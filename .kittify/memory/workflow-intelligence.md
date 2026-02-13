# Spec-Kitty Workflow Intelligence

> Lessons learned from the 010-hub-tui-navigator planning lifecycle.
> Reference session: 2026-02-12 through 2026-02-13, 7 commits across 5 phases.

## The Workflow Pipeline

```
specify -> clarify -> plan -> tasks -> analyze -> [implement]
  (req)    (opt)     (req)   (req)    (opt)       (req)
```

Each phase produces artifacts that downstream phases consume. Skipping optional phases is fine for simple features but costly for complex ones.

## Phase-by-Phase Learnings

### 1. /specify - Discovery & Requirements

**What it produces**: `spec.md` with user stories, functional requirements, NFRs, edge cases, success criteria.

**What went well**:
- 8 user stories with Gherkin-style acceptance scenarios made intent unambiguous
- 19 FRs with MUST language gave clear implementation targets
- Edge cases section (6 items) caught things like "no Zellij", "narrow terminal", "agent fails mid-workflow"
- Key Entities section provided a shared vocabulary (FeatureEntry, HubAction, etc.)

**Pitfalls observed**:
- The spec listed 7 HubAction variants but the implementation needed 9 (Clarify was missing; StartImplementation needed splitting into StartContinuous/StartWaveGated). The entity list drifted from the data model. **Lesson: Key Entities in spec should be considered provisional -- the data model is the authority.**
- One entire action (Clarify) existed in clarification answers and the data model but was never formalized as an FR. **Lesson: Every action/capability mentioned in clarifications should be traced back to an FR.**

### 2. /clarify - Ambiguity Resolution

**What it produces**: Clarification Q&A appended to spec.md.

**Why it was essential for this feature**:
- 4 questions resolved, each of which would have caused implementation rework:
  1. **Plan vs task detection** -> file-based (`plan.md` existence vs `tasks/WPxx-*.md` existence). Without this, the scanner could have used content parsing, frontmatter flags, or other approaches.
  2. **Pane stacking behavior** -> stack alongside (not replace). This affected the Zellij command strategy fundamentally.
  3. **Orchestration detection** -> dual approach (lock file + Zellij sessions). Single-source detection would have missed edge cases (stale locks, EXITED sessions).
  4. **Mode selection UX** -> Enter=continuous, Shift+Enter=wave-gated, >6 WP dialog. This is a UX decision that can't be inferred from requirements alone.

**Pattern**: Clarify is most valuable when the feature involves **multiple systems interacting** (hub + Zellij + OpenCode + filesystem) or **UX decisions with multiple valid answers**.

### 3. /plan - Architecture & Research

**What it produces**: `plan.md` (architecture decisions), `research.md` (validated technical approaches), `data-model.md` (entities/enums), `quickstart.md`.

**What went well**:
- Research phase (R-001 through R-006) validated assumptions against real CLI behavior. R-001 discovered that inside-session Zellij commands don't need `--session` -- this would have been a debugging nightmare during implementation.
- Data model with state transition diagrams gave implementers a clear mental model.
- Architecture decisions (AD-001 through AD-007) each had rationale and alternatives-considered sections.
- `quickstart.md` provided the "what does success look like" reference.

**Pitfalls observed**:
- **AD-003 contradicted R-001**: The plan proposed extending the `ZellijCli` trait (session-based), but research proved the hub needs direct `zellij action` calls (sessionless). The plan was written before research was complete. **Lesson: Research findings should be reviewed against all ADs before finalizing the plan. Run research first, then write ADs.**
- **Constitution check was skipped** (plan said "not found" but the file existed). **Lesson: The plan command must always attempt to load the constitution. If the agent can't find it, it should search harder, not skip.**
- **Plan summary used outdated language** ("manages pane/tab lifecycle through the existing ZellijCli abstraction") even though the body contradicted this. **Lesson: Summaries are the most-read, least-updated section. Re-read the summary after writing the body.**

### 4. /tasks - Work Package Decomposition

**What it produces**: `tasks.md` (master index), `tasks/WPxx-*.md` (detailed prompts per work package).

**What went well**:
- 44 subtasks across 8 WPs with clear dependency chain: WP01 -> WP02 -> WP03 -> WP04/WP05 (parallel) -> WP06 -> WP07 -> WP08
- Each WP prompt included: exact file paths, code snippets from existing codebase, reference patterns from existing modules, and test strategies
- Parallel opportunities marked at both WP and subtask level
- WP prompt files referenced specific line numbers in existing source files (e.g., "lines 43-59 of list_specs.rs")

**Key technique**: Reading the actual source files (`main.rs`, `start.rs`, `tui/mod.rs`, `zellij.rs`, `list_specs.rs`) during task generation produced much higher quality prompts than working from the plan alone. The prompts included:
- Current code that needs changing (with line numbers)
- Target code (what it should look like after)
- Reusable patterns from adjacent modules
- Exact import paths and type signatures

**Pitfall**: The `finalize-tasks` command committed `tasks.md` but NOT the individual `WP*.md` files because `.gitignore` has `kitty-specs/**/tasks/*.md`. This is by design (prevents merge conflicts in worktrees), but it means WP prompt files are ephemeral working documents. **Lesson: WP prompts exist on disk, not in git. They're consumed by implementation agents, not versioned.**

### 5. /analyze - Cross-Artifact Consistency Check

**What it produces**: Read-only analysis report with findings table, coverage matrix, and remediation suggestions.

**Why it was essential for this feature**:
- Found 2 HIGH, 8 MEDIUM, 7 LOW issues across 3 artifacts
- The two HIGH findings (F1: constitution check skipped, F2: AD-003 contradicts research) would have confused implementers who read the plan
- F6 (missing FR for Clarify action) meant a feature existed in tasks but had no spec authority -- a reviewer could have rejected it
- F7 (HubAction variant count mismatch) would have caused confusion between spec readers and code readers
- F13 (lib.rs vs main.rs) would have sent an implementer to the wrong file
- F17 (init_logging corrupts TUI) would have caused a runtime bug that's hard to diagnose

**Pattern**: Analyze catches **drift between artifacts** that accumulates as each phase builds on the previous one. The spec says X, the plan refines it to Y, the tasks implement Z -- and X/Y/Z slowly diverge. Analyze is the reconciliation step.

**Most valuable finding categories** (in order):
1. **Inconsistency** (same concept described differently across files) -- highest signal
2. **Coverage gaps** (requirement with no task, or task with no requirement) -- catches omissions
3. **Underspecification** (mentioned but not detailed enough to implement) -- prevents ambiguity

## Workflow Anti-Patterns to Avoid

1. **Writing ADs before research**: Research may invalidate architectural assumptions. Do research first.
2. **Summarizing from memory, not from the body**: Always re-read the plan body before writing/updating the summary.
3. **Trusting entity lists across artifacts**: The data model is the authority. Spec entity lists are provisional.
4. **Assuming clarification answers propagate to FRs**: Every clarification that introduces a new capability needs a corresponding FR.
5. **Skipping analyze for "simple" features**: Even this well-executed workflow had 17 findings. The cost of analyze is low; the cost of implementing against inconsistent specs is high.

## Session Architecture Notes

This feature was planned across multiple agent sessions with handoffs:
- Session 1: `/specify` + `/clarify` (controller agent)
- Session 2: `/plan` (controller agent) -- research + plan + data model
- Session 3: `/tasks` (controller agent) -- task decomposition + WP prompt generation
- Session 4: `/analyze` + remediation (controller agent)

The handoff summary between sessions was critical -- it carried forward:
- Key design decisions and their rationale
- Codebase discoveries (file paths, existing patterns, dependency versions)
- Clarification outcomes
- What was done vs what remains

**Lesson**: Handoff summaries should include **discoveries about the codebase** (not just decisions). The session that reads `main.rs` and discovers the CLI structure saves the next session from re-reading it.

## Metrics for This Feature

| Metric | Value |
|--------|-------|
| Phases completed | 5/5 (specify, clarify, plan, tasks, analyze) |
| Commits | 7 (2 scaffolding + 5 content) |
| Total artifacts | 10 files (spec, plan, research, data-model, quickstart, tasks.md, 8 WP prompts, requirements checklist) |
| Functional requirements | 20 (19 original + FR-006b from analyze) |
| Non-functional requirements | 4 |
| User stories | 8 |
| Work packages | 8 |
| Subtasks | 44 |
| Analyze findings | 17 (0 critical, 2 high, 8 medium, 7 low) |
| Findings remediated | 7 (both HIGHs + 5 others) |
| FR coverage by tasks | 100% |
| NFR coverage by tasks | 50% (2/4 lack benchmark tasks) |
