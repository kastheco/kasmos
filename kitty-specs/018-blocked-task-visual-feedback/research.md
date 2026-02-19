# Research: Blocked Task Visual Feedback and Confirmation

**Feature**: 018-blocked-task-visual-feedback
**Date**: 2026-02-18

## R-001: Current Task Panel Rendering Pipeline

**Question**: How does the task panel currently render items, and where should dimming be injected?

**Findings**:

The rendering chain is:

```
renderTasksPanel() -> renderTaskItem(t, selected) -> returns line1 + line2
  line1: taskStatusIndicator(t.State) + idStyle.Render(t.ID) + t.Title
  line2: taskMetaLine(t) -> worker ID link, or suggested role, or empty
```

Key observations from `internal/tui/panels.go`:

1. `renderTaskItem()` (line 347) takes a `task.Task` and `selected bool`. It builds line1 with `taskStatusIndicator()` + bold ID + title, and line2 from `taskMetaLine()`.
2. `taskStatusIndicator()` (line 430) returns a styled single-character icon. For `TaskBlocked`, it returns orange `colorOrange` icon.
3. The ID style is `lipgloss.NewStyle().Bold(true)` with `colorPurple` foreground when selected, no foreground when unselected.
4. The title is rendered as plain text (no explicit style).
5. `taskMetaLine()` (line 417) returns empty string for done tasks, worker ID link for in-progress, or suggested role for unassigned.

**Decision**: Inject blocked dimming in `renderTaskItem()` by wrapping the ID, title, and meta line text in `colorBlocked` foreground when `t.State == task.TaskBlocked`. The `taskStatusIndicator()` call is left unchanged to preserve the orange icon.

**Implementation detail**: When blocked AND selected, use `colorBlocked` for the ID instead of `colorPurple` to maintain the visual "dimmed" signal even when the cursor is on the row.

## R-002: Current Enter Key Dispatch for Task Panel

**Question**: How does the enter key currently work on the task panel, and what changes are needed?

**Findings**:

From `internal/tui/update.go`, `updateTaskPanelKeys()` (line 908):

```go
case key.Matches(msg, m.keys.Select) || msg.String() == " ":
    if m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
        t := m.loadedTasks[m.selectedTaskIdx]
        if t.State == task.TaskUnassigned {
            role := t.SuggestedRole
            if role == "" {
                role = "coder"
            }
            return m, m.openSpawnDialogWithTaskPrefill(role, strings.TrimSpace(t.Description), nil, t.ID)
        }
    }
    return m, nil
```

Currently: enter/space only acts on `TaskUnassigned`. All other states (including `TaskBlocked`) silently do nothing.

**Decision**: Add a second branch before the `TaskUnassigned` check:

```go
if t.State == task.TaskBlocked {
    return m, m.openBlockedConfirmDialog(m.selectedTaskIdx)
}
```

The "spawn anyway" path from the confirm dialog reuses `openSpawnDialogWithTaskPrefill()` identically to the unassigned path.

## R-003: Existing Dialog Patterns for Reference

**Question**: What existing dialog pattern should the blocked confirm dialog follow?

**Findings**:

kasmos has several dialog patterns in `internal/tui/overlays.go`:

| Dialog | Style | Buttons | Focus | Key handling |
|---|---|---|---|---|
| Quit confirm | `alertDialogStyle` (orange thick border) | "force quit" / "cancel" | Default: cancel (idx 1) | left/right/tab cycle, enter confirm, esc cancel |
| Spawn dialog | `dialogStyle` (pink rounded border) | No buttons (enter on fields) | Role selector | tab/s-tab cycle fields |
| Continue dialog | `dialogStyle` | No buttons (enter submits) | Follow-up textarea | tab/s-tab cycle |
| Batch dialog | `dialogStyle` | No buttons (enter spawns) | Task list | j/k navigate, space toggle, enter submit |
| Help overlay | `dialogStyle` | None | N/A | esc close |

**Decision**: Use `alertDialogStyle` (orange border) with the quit confirm button pattern. This is the only dialog that presents a warning with a binary choice. The blocked confirm dialog is the same category: "you're about to do something potentially undesirable, are you sure?"

**Layout reference** (from `renderQuitConfirm()` at line 573):
```go
forceStyle := inactiveButtonStyle
cancelStyle := inactiveButtonStyle
if m.quitConfirmFocused == 0 {
    forceStyle = alertButtonStyle
} else {
    cancelStyle = activeButtonStyle
}
buttons := lipgloss.JoinHorizontal(lipgloss.Left, forceStyle.Render("..."), "  ", cancelStyle.Render("..."))
```

## R-004: Overlay Dispatch Order in Update()

**Question**: Where in the Update() dispatch chain should the blocked confirm dialog be checked?

**Findings**:

Current dispatch order at the top of `Update()` (line 18-49):

```
1. showContinueDialog
2. showSettings
3. showQuitConfirm
4. showHistory
5. showRestorePicker
6. showBatchDialog
7. showNewDialog
8. showSpawnDialog
```

The blocked confirm dialog should appear BEFORE `showSpawnDialog` because "spawn anyway" opens the spawn dialog. If blocked confirm were checked after spawn dialog, the spawn dialog would intercept keys meant for the confirm dialog when both are theoretically active.

**Decision**: Insert `showBlockedConfirm` check between `showNewDialog` (7) and `showSpawnDialog` (8). This ensures:
- Blocked confirm gets priority over spawn dialog
- Higher-priority dialogs (quit, settings) still override

Similarly in `View()`, the render order for overlays (line 353-381) follows the same pattern. Insert `showBlockedConfirm` render between new dialog and spawn dialog checks.

## R-005: Batch Dialog Already Excludes Blocked Tasks

**Question**: Does the batch dialog currently exclude blocked tasks? Is FR-007 already satisfied?

**Findings**:

From `internal/tui/overlays.go`:

1. `renderBatchDialog()` (line 502): Only renders tasks where `t.State == task.TaskUnassigned` (line 505)
2. `updateBatchDialog()` (line 415): Space toggle only works on `task.TaskUnassigned` (line 439)
3. `moveBatchFocus()` (line 477): Only stops on `task.TaskUnassigned` tasks (line 486)
4. Enter handler (line 443): Only spawns if `t.State == task.TaskUnassigned` (line 450)

**Decision**: FR-007 is fully satisfied by existing code. No changes needed. Add a unit test to document this guarantee: load tasks with mixed states including blocked, verify blocked tasks don't appear in batch selection.

## R-006: Dependency Resolution and Unfinished Dep Computation

**Question**: How to compute the list of unfinished dependencies for the confirmation dialog?

**Findings**:

The Task struct (`internal/task/source.go` line 19):
```go
type Task struct {
    ID            string
    Title         string
    Dependencies  []string  // WP IDs this task depends on
    State         TaskState
    ...
}
```

The `resolveTaskDependencies()` method in `internal/tui/update.go` (line 1159) builds a `doneIDs` set and unblocks tasks whose deps are all done. This only runs on state transitions (worker exit, mark done, approve, reject).

For the confirm dialog, we need the inverse: given a blocked task, list its deps that are NOT done. This is a simple filter:

```go
func (m *Model) unfinishedDeps(t task.Task) []string {
    doneIDs := make(map[string]bool)
    for _, lt := range m.loadedTasks {
        if lt.State == task.TaskDone {
            doneIDs[lt.ID] = true
        }
    }
    var unfinished []string
    for _, dep := range t.Dependencies {
        if !doneIDs[dep] {
            unfinished = append(unfinished, dep)
        }
    }
    return unfinished
}
```

We also want to show the state of each unfinished dep in the dialog for context. This means looking up each dep ID in `loadedTasks` and getting its state string.

**Decision**: Compute at render time (per AD-005). Helper method on Model returns `[]struct{ID string, State string}` for display.

## R-007: Color Choice Validation

**Question**: Is #555555 appropriate for the blocked dim color?

**Findings**:

Current palette context:
- Background: terminal default (typically #1a1a2e or similar dark)
- `colorDarkGray`: #383838 (used for borders, separators - very dark)
- `colorMidGray`: #5C5C5C (used for pending indicator, help desc, placeholders)
- `colorLightGray`: #9B9B9B (used for secondary text, timestamps)
- `colorPending` = `colorMidGray` (pending tasks use #5C5C5C)

The blocked dim color needs to be:
1. Darker than `colorMidGray` (#5C5C5C) to read as "inactive" vs pending's "waiting"
2. Lighter than `colorDarkGray` (#383838) to remain legible
3. Distinct from `colorPending`/`colorMidGray` so blocked != pending visually

`#555555` is 3 hex steps below `colorMidGray` (#5C5C5C). On a dark terminal background, this reads as clearly subdued text that's still legible. It won't be confused with pending state because pending tasks also show the `colorPending` icon, while blocked tasks keep the `colorOrange` icon.

**Decision**: `#555555` confirmed. If testing reveals insufficient contrast on lighter terminal themes, `#4D4D4D` is the fallback.
