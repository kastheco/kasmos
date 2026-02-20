---
work_package_id: WP07
title: Constitution Amendment
lane: done
dependencies: []
base_branch: main
base_commit: f055de83e4266ab7ae6feafcc18372838787f867
created_at: '2026-02-20T05:13:09.397628+00:00'
subtasks:
- T038
- T039
- T040
phase: Phase 3 - Persistence & Config
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-19T03:53:34Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP07 - Constitution Amendment

## Important: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you begin addressing feedback, update `review_status: acknowledged`.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** - Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP07
```

No dependencies - this WP can start immediately and run in parallel with any other WP.

---

## Objectives & Success Criteria

1. **Worker mode principle amended**: Reflects the dual-mode architecture (subprocess default, tmux optional).
2. **Session continuation principle amended**: Clarifies that interactivity is available via tmux while session continuation remains in both modes.
3. **Go version updated**: References Go 1.24+ instead of 1.23+.
4. **Amendments are additive**: No existing subprocess behavior is removed or changed in the constitution.

## Context & Constraints

- **Constitution file**: `.kittify/memory/constitution.md`
- **Plan reference**: `kitty-specs/019-tmux-worker-mode/plan.md` section "AMENDMENT REQUIRED" lists the three changes.
- **Rule**: "Do not modify without discussion" - these amendments were identified and approved during planning.
- **Approach**: Surgical text replacements. Do NOT rewrite surrounding content.

**Key reference files**:
- `.kittify/memory/constitution.md` - The file to modify
- `kitty-specs/019-tmux-worker-mode/plan.md` - Amendment descriptions

---

## Subtasks & Detailed Guidance

### Subtask T038 - Amend worker mode principle

**Purpose**: The constitution currently states workers are headless subprocesses. With tmux mode, workers can also be interactive terminal panes. The principle must reflect this dual-mode reality.

**Steps**:
1. Read `.kittify/memory/constitution.md`.
2. Find the principle about workers being headless subprocesses. The exact wording may vary - search for "headless" or "subprocess" in the context of worker execution.
3. Replace with: "Workers are subprocesses (headless by default, interactive tmux panes when configured)"
4. Ensure surrounding context still makes sense.

**Example transformation**:
- Before: "Workers are headless subprocesses managed by kasmos"
- After: "Workers are subprocesses (headless by default, interactive tmux panes when configured) managed by kasmos"

**Files**: `.kittify/memory/constitution.md` (modify, ~1-2 lines changed)
**Parallel?**: Yes - all three amendments target different sections.

---

### Subtask T039 - Amend session continuation principle

**Purpose**: The constitution prioritizes session continuation over interactivity. With tmux mode, interactivity IS a first-class option. The principle should acknowledge both modes.

**Steps**:
1. In `.kittify/memory/constitution.md`, find the principle about session continuation vs interactivity.
2. Replace with: "Headless by default; interactive via tmux when workflows require it. Session continuation remains available in both modes."
3. Preserve any surrounding context about why session continuation matters.

**Example transformation**:
- Before: "Session continuation over interactivity - workers run headless, output is captured"
- After: "Headless by default; interactive via tmux when workflows require it. Session continuation remains available in both modes."

**Files**: `.kittify/memory/constitution.md` (modify, ~1-2 lines changed)
**Parallel?**: Yes - targets different section from T038.

---

### Subtask T040 - Update Go version reference

**Purpose**: The project has moved to Go 1.24+. The constitution should reflect the current minimum version.

**Steps**:
1. In `.kittify/memory/constitution.md`, find the Go version reference (likely "Go (1.23+)" or "Go 1.23+").
2. Replace with "Go (1.24+)" or "Go 1.24+" matching the existing format.
3. The go.mod file targets 1.24.0, confirmed in plan.md technical context.

**Files**: `.kittify/memory/constitution.md` (modify, ~1 word changed)
**Parallel?**: Yes - targets different section from T038/T039.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Constitution wording doesn't match plan.md quotes | Wrong text targeted for replacement | Read the actual file first, find the real text, then amend. |
| Principle not found (different wording) | Amendment missed | Search for keywords: "headless", "subprocess", "session continuation", "interactivity", "1.23". |
| Amendment changes meaning of unrelated principles | Constitution drift | Keep changes surgical. Only modify the specific phrases identified. |

## Review Guidance

- Verify each amendment matches the intent described in plan.md "AMENDMENT REQUIRED" section.
- Verify no unrelated text was modified.
- Verify the amendments are additive (subprocess behavior unchanged).
- Verify Go version matches go.mod (1.24.0).
- Read the full constitution after amendments to ensure consistency.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
- 2026-02-19T04:15:00Z - system - lane=done - Constitution already amended during planning phase. All three changes (worker mode dual-mode, session continuation, Go 1.24+) verified present in .kittify/memory/constitution.md v2.1.0. No implementation needed.
