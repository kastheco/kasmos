# Data Model: Blocked Task Visual Feedback and Confirmation

**Feature**: 018-blocked-task-visual-feedback
**Date**: 2026-02-18

## Existing Types (unchanged)

### task.Task (internal/task/source.go)

No changes. The feature operates on existing fields:

```go
type Task struct {
    ID            string      // Used in confirm dialog dep list
    Title         string      // Shown in confirm dialog header
    Description   string      // Passed to spawn dialog on proceed
    SuggestedRole string      // Passed to spawn dialog on proceed
    Dependencies  []string    // Filtered for unfinished deps in dialog
    State         TaskState   // Checked for TaskBlocked
    WorkerID      string      // (unused by this feature)
    Metadata      map[string]string // (unused by this feature)
}
```

### task.TaskState (internal/task/source.go)

No changes. Existing states used:

```go
const (
    TaskUnassigned TaskState = iota  // Normal rendering
    TaskBlocked                      // Dimmed rendering + confirm dialog on enter
    TaskInProgress                   // Normal rendering
    TaskForReview                    // Normal rendering
    TaskDone                         // Normal rendering, used for dep resolution
    TaskFailed                       // Normal rendering
)
```

## New Model Fields (internal/tui/model.go)

Added to the `Model` struct:

```go
// Blocked task confirmation dialog state
showBlockedConfirm    bool  // Whether the confirm dialog is visible
blockedConfirmTaskIdx int   // Index into loadedTasks for the blocked task
blockedConfirmFocused int   // 0 = "spawn anyway", 1 = "cancel"
```

**Placement**: After the `quitConfirmFocused int` field (groups all confirm dialog state together).

**Lifecycle**:
- Initialized: zero values (false, 0, 0) - dialog hidden
- Opened: `openBlockedConfirmDialog(taskIdx)` sets `showBlockedConfirm = true`, stores task index, sets `blockedConfirmFocused = 1` (default to cancel)
- Closed: `closeBlockedConfirmDialog()` resets all three to zero values

## New Message Types (internal/tui/messages.go)

### blockedConfirmProceedMsg

```go
type blockedConfirmProceedMsg struct {
    TaskIdx int    // Index into loadedTasks
}
```

Emitted when user selects "spawn anyway" in the blocked confirm dialog. The main Update handler reads the task at this index and calls `openSpawnDialogWithTaskPrefill()`.

No cancel message type needed - cancel is handled inline by `closeBlockedConfirmDialog()` (same pattern as quit confirm).

## New Color Constant (internal/tui/styles.go)

```go
colorBlocked = lipgloss.Color("#555555")
```

Added to the color palette block alongside other semantic colors (`colorRunning`, `colorDone`, etc.).

## New/Modified Methods (internal/tui/)

### New Methods

| Method | File | Signature | Purpose |
|---|---|---|---|
| `openBlockedConfirmDialog` | overlays.go | `(m *Model) openBlockedConfirmDialog(taskIdx int)` | Set dialog state, update key states |
| `closeBlockedConfirmDialog` | overlays.go | `(m *Model) closeBlockedConfirmDialog()` | Reset dialog state, update key states |
| `updateBlockedConfirmDialog` | overlays.go | `(m *Model) updateBlockedConfirmDialog(msg tea.Msg) (tea.Model, tea.Cmd)` | Handle keys: left/right/tab cycle, enter confirm, esc cancel |
| `renderBlockedConfirmDialog` | overlays.go | `(m *Model) renderBlockedConfirmDialog() string` | Render dialog with backdrop |
| `unfinishedDeps` | overlays.go | `(m *Model) unfinishedDeps(t task.Task) []unfinishedDep` | Compute unfinished dependency list with states |

### Helper Type

```go
type unfinishedDep struct {
    ID    string
    State string  // Human-readable state ("in-progress", "unassigned", "blocked", etc.)
}
```

Used only by `renderBlockedConfirmDialog()` for display. Not persisted.

### Modified Methods

| Method | File | Change |
|---|---|---|
| `renderTaskItem` | panels.go | Add `TaskBlocked` branch: apply `colorBlocked` to ID, title, meta text |
| `updateTaskPanelKeys` | update.go | Add `TaskBlocked` branch in Select handler: call `openBlockedConfirmDialog` |
| `Update` | update.go | Add `showBlockedConfirm` early dispatch (between showNewDialog and showSpawnDialog) |
| `Update` | update.go | Add `blockedConfirmProceedMsg` handler in message switch |
| `View` | model.go | Add `showBlockedConfirm` render check in overlay cascade |

## State Transitions

### Blocked Confirm Dialog Lifecycle

```
Task panel (blocked task selected)
    |
    v  [enter key]
openBlockedConfirmDialog(taskIdx)
    showBlockedConfirm = true
    blockedConfirmFocused = 1 (cancel)
    |
    +--[esc / "cancel"]----> closeBlockedConfirmDialog()
    |                            showBlockedConfirm = false
    |                            (no action)
    |
    +--["spawn anyway"]----> closeBlockedConfirmDialog()
                             emit blockedConfirmProceedMsg{TaskIdx}
                                 |
                                 v
                             openSpawnDialogWithTaskPrefill(role, desc, nil, taskID)
                                 (standard spawn flow from here)
```

### Task Rendering Decision Tree

```
renderTaskItem(t, selected):
    icon = taskStatusIndicator(t.State)    // Always uses state-specific color
    |
    +--[t.State == TaskBlocked]
    |    idColor = colorBlocked (even if selected)
    |    titleColor = colorBlocked
    |    metaColor = colorBlocked
    |
    +--[t.State != TaskBlocked && selected]
    |    idColor = colorPurple
    |    titleColor = default
    |    metaColor = (existing logic)
    |
    +--[t.State != TaskBlocked && !selected]
         idColor = default (bold, no foreground)
         titleColor = default
         metaColor = (existing logic)
```

## FR Traceability

| FR | Implementation Point | Change Type |
|---|---|---|
| FR-001 | `renderTaskItem()` in panels.go | Modified |
| FR-002 | `colorBlocked` in styles.go + `taskStatusIndicator()` unchanged | New constant |
| FR-003 | `updateTaskPanelKeys()` Select handler in update.go | Modified |
| FR-004 | `unfinishedDeps()` + `renderBlockedConfirmDialog()` in overlays.go | New methods |
| FR-005 | `updateBlockedConfirmDialog()` + button rendering in overlays.go | New method |
| FR-006 | `blockedConfirmProceedMsg` handler in update.go | New handler |
| FR-007 | Existing batch dialog code (no change, add test) | Test only |
| FR-008 | Existing `resolveTaskDependencies()` + `renderTaskItem()` | Existing + modified |
| FR-009 | `updateBlockedConfirmDialog()` esc handling in overlays.go | New method |
