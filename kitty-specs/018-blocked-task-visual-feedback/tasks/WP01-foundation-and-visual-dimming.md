---
work_package_id: "WP01"
subtasks:
  - "T001"
  - "T002"
  - "T003"
  - "T004"
title: "Foundation and Visual Dimming"
phase: "Phase 1 - Foundation"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: []
history:
  - timestamp: "2026-02-19T04:05:55Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP01 - Foundation and Visual Dimming

## Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** - Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP01
```

No dependencies - this is the starting work package.

---

## Objectives & Success Criteria

This work package delivers the **visual foundation** for blocked task feedback:

1. A new `colorBlocked` color constant that renders blocked task text as dimmed/inactive.
2. Model struct fields to support the confirmation dialog state (used by WP02).
3. A new message type for the dialog "spawn anyway" action (used by WP02).
4. Modified task panel rendering that dims blocked tasks while preserving the orange status icon.

**Success Criteria**:
- Blocked tasks in the task panel render with `#555555` foreground on ID, title, and meta line.
- The orange status icon (`colorOrange`) is unchanged for blocked tasks.
- Non-blocked tasks render identically to current behavior (no regression).
- When a blocked task is selected (cursor on it), the ID uses `colorBlocked` instead of the usual `colorPurple`.
- The project compiles cleanly with `go build ./cmd/kasmos`.

---

## Context & Constraints

**Feature spec**: `kitty-specs/018-blocked-task-visual-feedback/spec.md`
**Plan**: `kitty-specs/018-blocked-task-visual-feedback/plan.md`
**Data model**: `kitty-specs/018-blocked-task-visual-feedback/data-model.md`
**Research**: `kitty-specs/018-blocked-task-visual-feedback/research.md`

**Key architecture decisions**:
- **AD-001**: Use explicit `#555555` color instead of lipgloss `.Faint(true)` for deterministic cross-terminal rendering.
- **AD-003**: Dim the entire row (ID, title, meta) but keep the status icon in `colorOrange` for scanability.
- **AD-004**: Use boolean flag + int fields for dialog state (same pattern as quit confirm).

**Constitution rules**: Follow bubbletea v2 Elm architecture. Use lipgloss v2 for styling. No new packages. Never block the Update loop.

**Existing patterns to follow**:
- Color constants are grouped at the top of `internal/tui/styles.go` (lines 21-33 for base colors, lines 42-48 for semantic aliases).
- Model fields are grouped by concern in `internal/tui/model.go` (dialog fields at lines 53-67).
- Message types follow the pattern in `internal/tui/messages.go` (simple struct with relevant data).

---

## Subtasks & Detailed Guidance

### Subtask T001 - Add `colorBlocked` constant to styles.go

**Purpose**: Establish the semantic color for dimmed blocked task text. This color is used by `renderTaskItem()` (T004) and the confirmation dialog renderer (WP02 T008).

**Steps**:
1. Open `internal/tui/styles.go`.
2. In the base color palette block (after line 32, `colorLightGray`), add:
   ```go
   colorBlocked = lipgloss.Color("#555555")
   ```
3. This sits between `colorDarkGray` (#383838) and `colorMidGray` (#5C5C5C) in the brightness spectrum. It reads as "inactive" on dark terminal backgrounds while remaining legible.

**Files**: `internal/tui/styles.go` (~1 line added)

**Parallel?**: Yes - independent of T002 and T003.

**Notes**:
- Do NOT add a semantic alias for this color (like `colorRunning = colorPurple`). It has a single use case and the name `colorBlocked` is already semantic.
- Do NOT modify `taskStatusIndicator()` or `taskStatusBadge()` - the orange icon/badge for blocked state must be preserved.

---

### Subtask T002 - Add blocked confirm dialog state fields to Model

**Purpose**: The confirmation dialog (built in WP02) needs state fields on the Model to track visibility, which task triggered it, and which button is focused. Adding them now ensures WP01 compiles cleanly and WP02 can reference them immediately.

**Steps**:
1. Open `internal/tui/model.go`.
2. In the Model struct, after the `quitConfirmFocused int` field (line 63), add three new fields grouped with a comment:

   ```go
   showBlockedConfirm    bool
   blockedConfirmTaskIdx int
   blockedConfirmFocused int  // 0 = "spawn anyway", 1 = "cancel"
   ```

3. These follow the same pattern as `showQuitConfirm` / `quitConfirmFocused`. No initialization needed - zero values (false, 0, 0) mean "dialog hidden, default to spawn-anyway button".

**Files**: `internal/tui/model.go` (~3 lines added)

**Parallel?**: Yes - independent of T001 and T003.

**Notes**:
- Place these fields directly after `quitConfirmFocused` and before `showNewDialog` to group all confirm dialog state together.
- The `blockedConfirmFocused` default is 0 (spawn anyway), but `openBlockedConfirmDialog()` in WP02 will set it to 1 (cancel) when opening. This matches the quit confirm pattern where cancel is the safe default.
- No constructor changes needed in `NewModel()` - Go zero values are correct for "hidden dialog".

---

### Subtask T003 - Add `blockedConfirmProceedMsg` message type

**Purpose**: When the user selects "spawn anyway" in the confirmation dialog, this message is emitted so the main `Update()` handler can trigger `openSpawnDialogWithTaskPrefill()`. The decoupled message pattern follows existing kasmos conventions.

**Steps**:
1. Open `internal/tui/messages.go`.
2. After the `quitCancelledMsg` struct (line 80), add:

   ```go
   type blockedConfirmProceedMsg struct {
       TaskIdx int
   }
   ```

3. `TaskIdx` is the index into `loadedTasks` for the blocked task the user chose to spawn anyway.

**Files**: `internal/tui/messages.go` (~3 lines added)

**Parallel?**: Yes - independent of T001 and T002.

**Notes**:
- No cancel message type is needed. Cancel is handled inline by `closeBlockedConfirmDialog()` (same pattern as quit confirm where cancel just resets state without emitting a message).
- The field is `TaskIdx int` (not `TaskID string`) because the spawn flow in `updateTaskPanelKeys()` uses index-based access to `loadedTasks`. This avoids a lookup.

---

### Subtask T004 - Modify `renderTaskItem()` for blocked task dimming

**Purpose**: This is the core visual change - making blocked tasks appear dimmed/faint in the task panel. It fulfills FR-001 (dim entire row) and FR-002 (visually distinct from other states).

**Steps**:
1. Open `internal/tui/panels.go`.
2. Locate `renderTaskItem()` at line 347. Current implementation:

   ```go
   func (m *Model) renderTaskItem(t task.Task, selected bool) string {
       idStyle := lipgloss.NewStyle().Bold(true)
       if selected {
           idStyle = idStyle.Foreground(colorPurple)
       }
       line1 := fmt.Sprintf("%s %s  %s", taskStatusIndicator(t.State), idStyle.Render(t.ID), t.Title)

       line2 := m.taskMetaLine(t)
       if line2 == "" {
           return line1
       }

       return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
   }
   ```

3. Replace with logic that applies `colorBlocked` when the task is blocked:

   ```go
   func (m *Model) renderTaskItem(t task.Task, selected bool) string {
       blocked := t.State == task.TaskBlocked

       idStyle := lipgloss.NewStyle().Bold(true)
       switch {
       case blocked:
           idStyle = idStyle.Foreground(colorBlocked)
       case selected:
           idStyle = idStyle.Foreground(colorPurple)
       }

       title := t.Title
       if blocked {
           title = lipgloss.NewStyle().Foreground(colorBlocked).Render(t.Title)
       }

       line1 := fmt.Sprintf("%s %s  %s", taskStatusIndicator(t.State), idStyle.Render(t.ID), title)

       line2 := m.taskMetaLine(t)
       if line2 == "" {
           return line1
       }

       if blocked {
           line2 = lipgloss.NewStyle().Foreground(colorBlocked).Render(lipgloss.StyleRunes(line2, nil))
       }

       return lipgloss.JoinVertical(lipgloss.Left, line1, line2)
   }
   ```

4. **Key behavior changes**:
   - `blocked && selected`: ID uses `colorBlocked` (NOT `colorPurple`). The dimming signal persists even when the cursor is on the row.
   - `blocked && !selected`: ID uses `colorBlocked`. Same dim appearance.
   - `!blocked && selected`: ID uses `colorPurple` (unchanged from current behavior).
   - `!blocked && !selected`: ID has no foreground (unchanged from current behavior).
   - Title text: wrapped in `colorBlocked` style when blocked.
   - Meta line: wrapped in `colorBlocked` when blocked. Note that `taskMetaLine()` already applies its own foreground colors (`colorLightBlue` for worker link, `colorMidGray` for role). The blocked overlay wraps the entire rendered string.

**Important**: The `taskMetaLine()` return value already contains ANSI escape sequences from lipgloss styling. To override the foreground color, you need to strip or override the existing styling. The simplest correct approach is:

   ```go
   if blocked && line2 != "" {
       // Re-render the meta content with blocked color
       raw := ""
       if t.WorkerID != "" {
           raw = "-> " + t.WorkerID
       } else if strings.TrimSpace(t.SuggestedRole) != "" {
           raw = "role: " + t.SuggestedRole
       }
       if raw != "" {
           line2 = lipgloss.NewStyle().Foreground(colorBlocked).Render(raw)
       }
   }
   ```

   This re-renders the meta content with `colorBlocked` instead of trying to override already-styled text.

**Files**: `internal/tui/panels.go` (~20 lines modified/added)

**Parallel?**: No - depends on T001 (uses `colorBlocked`).

**Validation**:
- [ ] Blocked task: ID, title, and meta line all render with `#555555` foreground
- [ ] Blocked task: Status icon remains orange (not dimmed)
- [ ] Blocked + selected task: ID uses `colorBlocked`, not `colorPurple`
- [ ] Non-blocked tasks: Rendering unchanged (no regression)
- [ ] Done tasks with no meta line: Still render as single line (no crash)
- [ ] `go build ./cmd/kasmos` compiles without errors

**Edge Cases**:
- Task with state `TaskBlocked` but empty `SuggestedRole` and no `WorkerID`: only line1 renders (line2 is empty). The dimming still applies to line1.
- Task with state `TaskDone`: taskMetaLine returns empty string, task renders as single line. Not affected by blocked dimming.

---

## Risks & Mitigations

1. **Color contrast on light themes**: `#555555` may be too faint on light terminal backgrounds. Mitigation: The plan documents `#4D4D4D` as a fallback (research R-007). This can be adjusted post-implementation if needed.

2. **ANSI override in meta line**: Wrapping already-styled text in a new foreground color may not override inner styles in all terminal emulators. Mitigation: Re-render the meta content from raw strings instead of wrapping the styled output (documented in T004 implementation detail).

3. **Regression in non-blocked rendering**: Changes to `renderTaskItem()` could accidentally affect other states. Mitigation: The `blocked` boolean is only true for `TaskBlocked` state, and all changes are gated behind it.

---

## Review Guidance

- **FR-001 check**: Verify the entire task item row (ID, title, meta) uses `colorBlocked` foreground for blocked tasks.
- **FR-002 check**: Verify blocked styling is visually distinct from unassigned (no foreground), in-progress (purple worker link), done (no meta), and failed (orange) states.
- **AD-003 check**: Verify `taskStatusIndicator()` is NOT modified - the orange icon must be preserved.
- **Regression check**: Non-blocked task rendering must be byte-identical to pre-change behavior.
- **Build check**: `go build ./cmd/kasmos` compiles cleanly. `go vet ./internal/tui/...` passes.

---

## Activity Log

- 2026-02-19T04:05:55Z - system - lane=planned - Prompt created.
