---
work_package_id: "WP01"
subtasks:
  - "T001"
  - "T002"
  - "T003"
  - "T004"
  - "T005"
  - "T006"
  - "T007"
title: "Create End-to-End Workflow Cheatsheet"
phase: "Phase 1 - Implementation"
lane: "planned"
dependencies: []
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
history:
  - timestamp: "2026-02-10T20:59:42Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP01 – Create End-to-End Workflow Cheatsheet

## IMPORTANT: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: Update `review_status: acknowledged` in frontmatter when you begin addressing feedback.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** – Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

No dependencies — start from target branch:
```bash
spec-kitty implement WP01
```

---

## Objectives & Success Criteria

Create `docs/workflow-cheatsheet.md` — a single-page, scannable reference covering the full spec-kitty + kasmos development lifecycle from feature inception to merge.

**Success criteria**:
- Covers 100% of spec-kitty slash commands and kasmos CLI commands used in a standard feature lifecycle (SC-003)
- An operator can identify the next command within 10 seconds of scanning (SC-001)
- A new team member understands the full workflow after a single read of the overview section (SC-002)
- Renders correctly in terminal viewers (`cat`/`less`/`bat`) and GitHub markdown (SC-004)
- Optional phases are clearly marked; both orchestration modes documented
- Cross-links to existing docs; no duplication of kasmos CLI/FIFO detail already in `docs/cheatsheet.md`

## Context & Constraints

**Spec**: `kitty-specs/004-workflow-cheatsheet/spec.md`
**Plan**: `kitty-specs/004-workflow-cheatsheet/plan.md`

**Key constraints**:
- Scannable format only — headers per section, commands in fenced code blocks, no prose walls (FR-007)
- Dashboard integration is **deferred** — this WP creates the markdown file only
- Do not duplicate the full FIFO command table from `docs/cheatsheet.md` — summarize the key commands and link to it
- Do not duplicate the Zellij navigation table from `docs/keybinds.md` — link to it

**Content sources** (read these for accurate command syntax):
- `crates/kasmos/src/main.rs` — kasmos CLI subcommands and flags
- `docs/cheatsheet.md` — kasmos FIFO commands, config, state machine
- `docs/getting-started.md` — Zellij primer, orchestration modes, practical workflow
- `docs/keybinds.md` — Zellij keybinds
- spec-kitty init output — slash command listing (visible during `spec-kitty init`)

---

## Subtasks & Detailed Guidance

### Subtask T001 – Create file with header and prerequisites section

**Purpose**: Establish the document structure and immediately orient the reader with tool requirements.

**Steps**:
1. Create `docs/workflow-cheatsheet.md`
2. Add a title header: `# Spec-Kitty + Kasmos Workflow Cheatsheet`
3. Add a one-line description: quick-reference for the end-to-end feature development lifecycle
4. Write a `## Prerequisites` section listing required tools with version-check commands:

```markdown
## Prerequisites

| Tool | Check | Purpose |
|------|-------|---------|
| spec-kitty | `spec-kitty --version` | Feature specification & planning |
| kasmos | `kasmos --version` | Zellij-based agent orchestration |
| zellij | `zellij --version` | Terminal multiplexer runtime |
| git | `git --version` | Version control & worktrees |
```

5. Optionally mention `bat` for syntax-highlighted terminal viewing

**Files**: `docs/workflow-cheatsheet.md` (new file)

---

### Subtask T002 – Write end-to-end pipeline overview with visual flow

**Purpose**: Give the reader the full mental model in one glance. This is the most important section — it anchors everything else.

**Steps**:
1. Add a `## Pipeline Overview` section
2. Show the complete phase sequence as a numbered list or ASCII flow. Clearly separate **Planning** and **Execution** phases:

```markdown
## Pipeline Overview

### Planning (spec-kitty)
1. `/spec-kitty.specify` — Create feature specification
2. `/spec-kitty.clarify` *(optional)* — Probe spec for ambiguities
3. `/spec-kitty.plan` — Generate implementation plan
4. `/spec-kitty.research` *(optional)* — Phase 0 research scaffolding
5. `/spec-kitty.tasks` — Generate work packages & prompt files
6. `/spec-kitty.analyze` *(optional)* — Cross-artifact consistency check

### Execution (kasmos + spec-kitty)
7. `/spec-kitty.implement WP##` — Create worktree per work package
8. `kasmos launch` — Start Zellij orchestration session
9. Monitor & interact — Navigate panes, issue FIFO commands
10. `/spec-kitty.review` — Review completed work packages
11. `/spec-kitty.accept` — Validate feature readiness
12. `/spec-kitty.merge` — Merge to main & clean up
```

3. Add a visual separator or note making the planning→execution boundary unmistakable
4. Mark optional phases with `*(optional)*` consistently

**Files**: `docs/workflow-cheatsheet.md`

---

### Subtask T003 – Write planning phase reference cards

**Purpose**: Provide command-level detail for each spec-kitty planning phase so the operator knows exactly what to type and what they'll get.

**Steps**:
1. Add a `## Phase Reference` section (or use it as a parent for both T003 and T004)
2. For each planning phase, write a compact reference card using a consistent format:

```markdown
### 1. Specify

> Create the feature specification from a natural language description.

| | |
|---|---|
| **Command** | `/spec-kitty.specify` |
| **Input** | Feature description (text or empty for interactive) |
| **Output** | `kitty-specs/###-feature/spec.md`, `meta.json` |
| **Prerequisites** | spec-kitty initialized (`spec-kitty init`) |
```

3. Cover these planning phases with the same card format:
   - **Specify** (`/spec-kitty.specify`) — creates spec.md, meta.json
   - **Clarify** (`/spec-kitty.clarify`, optional) — probes ambiguities, updates spec.md with clarifications section
   - **Plan** (`/spec-kitty.plan`) — creates plan.md, optionally research.md, data-model.md
   - **Research** (`/spec-kitty.research`, optional) — Phase 0 research scaffolding
   - **Tasks** (`/spec-kitty.tasks`) — creates tasks.md, `tasks/WP##-*.md` prompt files
   - **Analyze** (`/spec-kitty.analyze`, optional) — cross-artifact consistency report

4. Each card should fit in roughly 6-10 lines. Keep it tight.

**Files**: `docs/workflow-cheatsheet.md`

---

### Subtask T004 – Write execution phase reference cards

**Purpose**: Cover the implementation and finalization phases including kasmos orchestration.

**Steps**:
1. Continue the Phase Reference section with execution phase cards
2. Cover these phases:
   - **Implement** (`/spec-kitty.implement WP##`) — creates a worktree for the work package. Note: `--base WP##` flag for dependent WPs.
   - **Launch** (`kasmos launch <feature_dir> [--mode continuous|wave-gated]`) — starts Zellij orchestration. Source flags from `crates/kasmos/src/main.rs`. Mention default is `continuous`.
   - **Monitor & Interact** — not a single command; describe as: navigate Zellij panes (`Ctrl+p`), issue FIFO commands. Link to `docs/cheatsheet.md` for full command table and `docs/keybinds.md` for keybinds.
   - **Review** (`/spec-kitty.review`) — structured code review, moves WPs between kanban lanes
   - **Accept** (`/spec-kitty.accept`) — validates feature readiness before merge
   - **Merge** (`/spec-kitty.merge`) — merges feature branch to main, cleans up worktrees

3. For the **Launch** card, include both invocation forms:
   ```bash
   kasmos launch <feature_dir>                        # continuous (default)
   kasmos launch <feature_dir> --mode wave-gated      # wave-gated
   ```

4. For **Monitor**, reference key FIFO commands inline (the top 5 most-used):
   - `echo "status" > .kasmos/cmd.pipe`
   - `echo "advance" > .kasmos/cmd.pipe` (wave-gated)
   - `echo "restart <WP_ID>" > .kasmos/cmd.pipe`
   - `echo "abort" > .kasmos/cmd.pipe`
   - Link: "Full command list → [cheatsheet.md](./cheatsheet.md)"

**Files**: `docs/workflow-cheatsheet.md`

---

### Subtask T005 – Write kasmos orchestration sub-workflow section

**Purpose**: Detail the kasmos-specific workflow within the broader lifecycle — how orchestration actually works once launched.

**Steps**:
1. Add a `## Kasmos Orchestration` section
2. Document the two progression modes with a clear branching point:

```markdown
## Kasmos Orchestration

### Continuous Mode (default)
Waves execute automatically. No operator intervention needed between waves.
1. `kasmos launch <dir>` — agents start in wave 0
2. Wave 0 completes → wave 1 auto-launches → ... → all waves done
3. `kasmos stop` or orchestration completes naturally

### Wave-Gated Mode
Operator confirms each wave advancement.
1. `kasmos launch <dir> --mode wave-gated`
2. Wave 0 completes → orchestrator pauses
3. Operator reviews results
4. `echo "advance" > .kasmos/cmd.pipe` → wave 1 launches
5. Repeat until all waves complete
```

3. Add a brief "Recovery Actions" subsection covering the most common interventions:
   - Restart failed WP: `echo "restart WP01" > .kasmos/cmd.pipe`
   - Skip failed WP: `echo "force-advance WP01" > .kasmos/cmd.pipe`
   - Pause a WP: `echo "pause WP01" > .kasmos/cmd.pipe`
   - Resume: `echo "resume WP01" > .kasmos/cmd.pipe`
   - Full reference: link to `docs/cheatsheet.md`

4. Add a brief "Other CLI Commands" subsection:
   - `kasmos status [feature_dir]` — check orchestration state
   - `kasmos attach <feature_dir>` — reconnect to detached session
   - `kasmos stop [feature_dir]` — graceful shutdown

**Files**: `docs/workflow-cheatsheet.md`

---

### Subtask T006 – Write daily session quick-reference

**Purpose**: Provide the 5-6 commands an operator uses in a typical daily session — the most-reached-for section of the cheatsheet.

**Steps**:
1. Add a `## Daily Session` section
2. Cover the typical daily workflow as a compact numbered list:

```markdown
## Daily Session

1. **Resume session**: `kasmos attach <feature_dir>`
2. **Check status**: `kasmos status` or `echo "status" > .kasmos/cmd.pipe`
3. **Monitor agents**: Navigate Zellij panes — `Ctrl+p` then `h/j/k/l` or `Tab`
4. **Advance wave** (wave-gated): `echo "advance" > .kasmos/cmd.pipe`
5. **Review completed WPs**: `/spec-kitty.review`
6. **End session**: Detach from Zellij — `Ctrl+o` then `d`
```

3. Keep this section deliberately short — it should be the section operators memorize fastest

**Files**: `docs/workflow-cheatsheet.md`

---

### Subtask T007 – Add cross-references and update existing docs

**Purpose**: Connect the new cheatsheet to existing documentation and make it discoverable.

**Steps**:
1. Add a `## See Also` section at the bottom of the new cheatsheet with links:
   ```markdown
   ## See Also

   - [Kasmos CLI & FIFO Cheatsheet](./cheatsheet.md) — Full command reference for kasmos operations
   - [Getting Started](./getting-started.md) — Zellij primer and kasmos fundamentals
   - [Keybinds Reference](./keybinds.md) — Zellij keyboard shortcuts
   - [Architecture](./architecture.md) — kasmos internals and state machine design
   ```

2. Update `docs/getting-started.md` — add a link to the new cheatsheet in its "Next Steps" section (line ~328-332). Add:
   ```markdown
   - **[Workflow Cheatsheet](./workflow-cheatsheet.md)** — End-to-end spec-kitty + kasmos lifecycle reference
   ```

3. Update `docs/cheatsheet.md` — add a note at the top (after the `# kasmos Cheatsheet` header) linking to the workflow cheatsheet:
   ```markdown
   > For the full end-to-end workflow (spec-kitty planning → kasmos execution → merge), see [Workflow Cheatsheet](./workflow-cheatsheet.md).
   ```

**Files**:
- `docs/workflow-cheatsheet.md` (add See Also section)
- `docs/getting-started.md` (add cross-link in Next Steps)
- `docs/cheatsheet.md` (add cross-link at top)

---

## Risks & Mitigations

- **Stale commands**: Kasmos CLI flags may have changed since docs were written. Mitigation: verify against `crates/kasmos/src/main.rs` Clap definitions before writing.
- **Overlap with existing docs**: Mitigation: the workflow cheatsheet covers the lifecycle flow; existing `cheatsheet.md` covers CLI/FIFO detail. Summarize and link, don't duplicate.
- **Rendering issues**: Markdown tables and code blocks may render differently across viewers. Mitigation: use simple GFM constructs only (no HTML, no complex nesting).

## Review Guidance

- **Completeness**: Verify every spec-kitty slash command and kasmos CLI command from the plan's Phase Reference Cards list appears in the document.
- **Accuracy**: Spot-check 2-3 command signatures against the actual CLI (`kasmos --help`, `spec-kitty --help`).
- **Scannability**: Time yourself finding a specific phase — should take <10 seconds.
- **Cross-links**: Verify all links to existing docs resolve correctly.
- **No duplication**: Confirm FIFO command table is NOT duplicated from `docs/cheatsheet.md` — only summarized with a link.

## Activity Log

> **CRITICAL**: Activity log entries MUST be in chronological order (oldest first, newest last).

- 2026-02-10T20:59:42Z – system – lane=planned – Prompt created.

---

### Updating Lane Status

To change this work package's lane, either:

1. **Edit directly**: Change the `lane:` field in frontmatter AND append activity log entry (at the end)
2. **Use CLI**: `spec-kitty agent tasks move-task WP01 --to <lane> --note "message"` (recommended)

**Valid lanes**: `planned`, `doing`, `for_review`, `done`
