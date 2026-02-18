---
work_package_id: WP09
title: Task Panel UI + Worker-Task Association + Batch Spawn
lane: doing
dependencies:
- WP03
- WP04
- WP08
subtasks:
- Task list panel using bubbles/list (wide mode)
- Custom list.ItemDelegate for multi-line task items
- Task panel focus cycling (3-panel mode)
- 'Spawn from task: pre-fill dialog with task data'
- 'Worker-task association: taskStateChangedMsg flow'
- Batch spawn dialog
- Header subtitle with source info
phase: Wave 2 - Task Sources + Worker Management
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-17T00:00:00Z'
  lane: planned
  agent: planner
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-18T14:31:13.383985135+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP09 coder - task panel UI + batch spawn)
---

# Work Package Prompt: WP09 - Task Panel UI + Worker-Task Association + Batch Spawn

## Mission

Build the task panel UI for wide mode (3-column layout), connect task sources to
the TUI with worker-task associations, and implement batch spawning. After this
WP, users can see their WPs in the dashboard, spawn workers directly from tasks
with pre-filled prompts, and batch-spawn multiple tasks. This delivers User Story 5
(Load Tasks from External Sources).

## Scope

### Files to Modify

```
internal/tui/model.go       # Add task list model, task source fields
internal/tui/panels.go      # Task list panel rendering, header subtitle
internal/tui/overlays.go    # Spawn dialog pre-fill from task, batch spawn dialog
internal/tui/update.go      # Task-related message handlers, task panel key routing
internal/tui/layout.go      # Wide mode activation with task source
internal/tui/keys.go        # Enable task panel keys, batch spawn key
internal/tui/messages.go    # Ensure tasksLoadedMsg, taskStateChangedMsg defined
```

### Technical References

- `design-artifacts/tui-mockups.md`:
  - **V4**: Task source panel, 3-column layout (lines 167-211)
- `design-artifacts/tui-layout-spec.md`:
  - Wide mode dimensions: 25%/35%/40% (lines 170-206)
  - Task list panel specs (lines 313-336)
  - Focus system with 3 panels (lines 440-451)
- `design-artifacts/tui-keybinds.md`:
  - Task list focused keys: j/k, /, enter, s, b (lines 49-58)
- `design-artifacts/tui-styles.md`:
  - Task state badges: taskStatusBadge() (lines 359-384)
  - Source subtitle style (line 129)
- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 2**: tasksLoadedMsg, taskStateChangedMsg (lines 368-384)
  - **Section 4**: Task struct, TaskState enum (lines 582-614)

## Implementation

### Task List Panel (panels.go)

Use `bubbles/list` with a custom `list.ItemDelegate`:

**List item structure** (4 lines per item + 1 blank separator):
```
WP-001  Auth middleware        <- Title line (bold)
JWT RS256 validation layer     <- Description (truncated)
deps: none                     <- Dependencies (orange if blocking)
check done                     <- Task state badge
```

Custom delegate:
```go
type taskItemDelegate struct{}
func (d taskItemDelegate) Height() int                         { return 5 } // 4 lines + separator
func (d taskItemDelegate) Spacing() int                        { return 0 }
func (d taskItemDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd { return nil }
func (d taskItemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item)
```

Each Task must implement `list.Item`:
```go
func (t Task) FilterValue() string { return t.Title }
func (t Task) Title() string       { return t.ID + "  " + t.Title }
func (t Task) Description() string  { return t.Description }
```

The delegate renders: title line, description (truncated to width), deps line
(with dependency IDs), status badge using `taskStatusBadge()`.

### Wide Mode Activation (layout.go)

Modify `recalculateLayout()`:
- Wide mode (>=160 cols) only activates when `m.taskSource != nil && m.taskSource.Type() != "ad-hoc"`
- When active: calculate 3-column dimensions (25%/35%/40%)
- Set task list size: `m.taskList.SetSize(tasksInnerWidth, tasksInnerHeight)`

### Focus Cycling Update

Modify `cyclablePanels()`:
```go
func (m Model) cyclablePanels() []panel {
    if m.hasTaskSource() && m.layoutMode == layoutWide {
        return []panel{panelTasks, panelTable, panelViewport}
    }
    return []panel{panelTable, panelViewport}
}
```

### Spawn from Task

When `enter` or `s` is pressed on a selected task in the task list:
1. Get the selected task from the list
2. Open spawn dialog with pre-filled values:
   - Role: task.SuggestedRole (or first option if empty)
   - Prompt: task.Description
   - TaskID: task.ID (stored on the form for association)
3. User can edit before confirming

On spawn confirm with a TaskID:
- Set task.WorkerID = newWorker.ID
- Set task.State = TaskInProgress
- Emit taskStateChangedMsg

### Worker-Task Association

When a worker exits:
- If worker has TaskID: update the associated task's state
  - Exit code 0: task.State = TaskDone
  - Exit code != 0: task.State = TaskFailed
- Emit taskStateChangedMsg to refresh the task list

When a task's dependencies are all TaskDone:
- If task was TaskBlocked, transition to TaskUnassigned

### Batch Spawn (`b` key)

When `b` is pressed with a task source loaded:
1. Show a selection overlay listing all unassigned/unblocked tasks
2. User toggles tasks on/off (checkboxes)
3. For each selected task, auto-assign suggested role and use description as prompt
4. Confirm spawns all selected tasks as workers simultaneously
5. Each spawned worker gets its TaskID set

Implementation: Use a `huh.NewMultiSelect()` with task titles as options.
On confirm, loop through selections and dispatch spawnWorkerCmd for each.

### Header Subtitle

When a task source is loaded, render the subtitle line in the header:
```
spec-kitty: kitty-specs/016-kasmos-agent-orchestrator/
```
Or for GSD:
```
gsd: tasks.md (6 tasks)
```

This adds 1 line to headerLines (chromeTotal becomes 5 instead of 4).

### Status Bar Update

When task source is loaded, show task counts on the left side:
```
tasks: 1 done . 1 in-progress . 4 pending     workers: 2 running . 1 done
```

## What NOT to Do

- Do NOT implement task file modification (writing back lane changes to WP files)
- Do NOT implement task drag-and-drop or reordering
- Do NOT implement the AI gen-prompt feature (WP11)
- Task filtering uses bubbles/list built-in `/` search -- no custom filter UI

## Acceptance Criteria

1. Run `kasmos kitty-specs/016-kasmos-agent-orchestrator/` at >=160 cols -- 3-column layout with tasks
2. Task list shows WP items with title, description, deps, status badges
3. Tab cycles through all 3 panels (tasks, workers, output)
4. Select a task, press `enter` -- spawn dialog opens pre-filled with task data
5. Spawned worker shows task ID in the table's Task column
6. Worker exit updates associated task state (done/failed)
7. Blocked tasks show dependency info, become unassigned when deps resolve
8. Press `b` -- batch spawn dialog appears, select multiple tasks, all spawn
9. Header shows source subtitle when task source is loaded
10. At <160 cols, task panel hides and layout falls back to 2-column
11. `go test ./...` passes
