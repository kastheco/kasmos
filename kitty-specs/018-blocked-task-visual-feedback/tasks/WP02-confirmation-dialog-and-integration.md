---
work_package_id: WP02
title: Confirmation Dialog and Integration
lane: "done"
dependencies: [WP01]
base_branch: 018-blocked-task-visual-feedback-WP01
base_commit: 8f5dc5deda9b17e2044c1514a4341c4667bbae4e
created_at: '2026-02-19T19:28:13.254909+00:00'
subtasks:
- T005
- T006
- T007
- T008
- T009
- T010
phase: Phase 2 - Dialog and Dispatch
assignee: ''
agent: ''
shell_pid: "2696393"
review_status: 'passed'
reviewed_by: 'manager-reviewer'
history:
- timestamp: '2026-02-19T04:05:55Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-19T19:28:13Z'
  lane: doing
  agent: coder
  shell_pid: '2696393'
  action: Implementation started (base WP01)
- timestamp: '2026-02-19T20:00:00Z'
  lane: for_review
  agent: manager
  shell_pid: ''
  action: Submitted for review
- timestamp: '2026-02-19T20:15:00Z'
  lane: done
  agent: reviewer
  shell_pid: ''
  action: "Review PASS - FR-003/004/005/006/009 AD-002 compliant, dispatch order correct, 2 minor non-blocking findings"
---

# Work Package Prompt: WP02 - Confirmation Dialog and Integration

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
spec-kitty implement WP02 --base WP01
```

This work package depends on WP01 (uses `colorBlocked`, model fields, and `blockedConfirmProceedMsg`).

---

## Objectives & Success Criteria

This work package delivers the **confirmation dialog** and **key dispatch integration** for blocked tasks:

1. A helper method that computes which dependencies are unfinished for a given blocked task.
2. Open/close lifecycle methods for the dialog.
3. Key handling (left/right/tab to cycle buttons, enter to confirm, esc to cancel).
4. Dialog renderer following the quit confirm visual pattern (alertDialogStyle, orange border).
5. Wiring into the Update() dispatch chain and View() overlay cascade.
6. The enter key on a blocked task in the task panel opens this dialog instead of silently doing nothing.

**Success Criteria**:
- Pressing enter on a blocked task shows the confirmation dialog with unfinished dependency IDs and their states.
- "spawn anyway" closes the dialog and opens the standard spawn dialog pre-filled with the task's role and description.
- "cancel" or escape closes the dialog without action.
- Left/right/tab cycle between the two buttons. Default focus is on "cancel".
- The dialog renders with `alertDialogStyle` (orange thick border) consistent with quit confirm.
- Done dependencies are NOT listed (only unfinished ones appear).
- Non-blocked tasks continue to open the spawn dialog directly on enter (no regression).
- `go build ./cmd/kasmos` compiles cleanly.

---

## Context & Constraints

**Feature spec**: `kitty-specs/018-blocked-task-visual-feedback/spec.md`
**Plan**: `kitty-specs/018-blocked-task-visual-feedback/plan.md` (Change Sets 3-5)
**Data model**: `kitty-specs/018-blocked-task-visual-feedback/data-model.md` (new methods table, state transitions)
**Research**: `kitty-specs/018-blocked-task-visual-feedback/research.md` (R-002 through R-006)

**Key architecture decisions**:
- **AD-002**: Follow quit confirm dialog pattern - `alertDialogStyle`, orange border, two-button layout.
- **AD-004**: Boolean flag + int fields (already added in WP01 T002).
- **AD-005**: Compute unfinished deps at render time for freshness.

**Constitution rules**: bubbletea v2 Elm architecture. No blocking in Update. All rendering in View.

**Existing patterns (quit confirm reference)**:
- `renderQuitConfirm()` at `internal/tui/overlays.go` line 573: alertDialogStyle, two buttons, focus cycling.
- `updateQuitConfirm()` at `internal/tui/overlays.go`: esc cancel, enter confirm, left/right/tab cycle.
- `showQuitConfirm` dispatch at `internal/tui/update.go` line 27: early return in Update().
- `showQuitConfirm` render at `internal/tui/model.go` View() line 369.

**WP01 provides**: `colorBlocked` (styles.go), `showBlockedConfirm`/`blockedConfirmTaskIdx`/`blockedConfirmFocused` fields (model.go), `blockedConfirmProceedMsg` (messages.go).

---

## Subtasks & Detailed Guidance

### Subtask T005 - Add `unfinishedDep` helper and `unfinishedDeps()` method

**Purpose**: Compute the list of unfinished dependencies for a blocked task. Used by the dialog renderer to show which deps are still outstanding and their current states.

**Steps**:
1. Open `internal/tui/overlays.go`.
2. Add the helper struct (place near the top of the file or near the blocked confirm methods):

   ```go
   type unfinishedDep struct {
       ID    string
       State string // Human-readable: "in-progress", "unassigned", "blocked", "failed", "unknown"
   }
   ```

3. Add the method on Model:

   ```go
   func (m *Model) unfinishedDeps(t task.Task) []unfinishedDep {
       // Build lookup of task states by ID
       stateByID := make(map[string]task.TaskState, len(m.loadedTasks))
       for _, lt := range m.loadedTasks {
           stateByID[lt.ID] = lt.State
       }

       var deps []unfinishedDep
       for _, depID := range t.Dependencies {
           state, exists := stateByID[depID]
           if !exists {
               // Orphaned dependency - task ID doesn't exist in loaded tasks
               deps = append(deps, unfinishedDep{ID: depID, State: "unknown"})
               continue
           }
           if state == task.TaskDone {
               continue // Done deps are filtered out
           }
           deps = append(deps, unfinishedDep{ID: depID, State: taskStateLabel(state)})
       }
       return deps
   }
   ```

4. Add a simple helper to convert TaskState to human-readable label:

   ```go
   func taskStateLabel(s task.TaskState) string {
       switch s {
       case task.TaskUnassigned:
           return "unassigned"
       case task.TaskBlocked:
           return "blocked"
       case task.TaskInProgress:
           return "in-progress"
       case task.TaskForReview:
           return "for-review"
       case task.TaskFailed:
           return "failed"
       case task.TaskDone:
           return "done"
       default:
           return "unknown"
       }
   }
   ```

**Files**: `internal/tui/overlays.go` (~30 lines added)

**Parallel?**: Yes - can be implemented independently of T006-T008.

**Notes**:
- Per AD-005, this is called at render time (inside `renderBlockedConfirmDialog`), not when opening the dialog. This ensures the displayed list is always current.
- Orphaned dependency IDs (not found in loaded tasks) are included with state "unknown" per the edge case in the spec.
- The `stateByID` map is rebuilt each render call. With typically <20 tasks, this is negligible cost.

---

### Subtask T006 - Add open/close dialog lifecycle methods

**Purpose**: Encapsulate dialog state transitions in methods, consistent with the existing quit confirm pattern.

**Steps**:
1. In `internal/tui/overlays.go`, add:

   ```go
   func (m *Model) openBlockedConfirmDialog(taskIdx int) {
       m.showBlockedConfirm = true
       m.blockedConfirmTaskIdx = taskIdx
       m.blockedConfirmFocused = 1 // Default to "cancel" (safe choice)
       m.updateKeyStates()
   }

   func (m *Model) closeBlockedConfirmDialog() {
       m.showBlockedConfirm = false
       m.blockedConfirmTaskIdx = 0
       m.blockedConfirmFocused = 0
       m.updateKeyStates()
   }
   ```

2. `openBlockedConfirmDialog` sets focus to 1 (cancel button) as the safe default. This matches the quit confirm pattern where the destructive action requires deliberate navigation.

**Files**: `internal/tui/overlays.go` (~12 lines added)

**Parallel?**: No - T007 and T008 call these methods.

**Notes**:
- `updateKeyStates()` is called to refresh keybinding visibility in the help bar.
- The method signature takes `taskIdx int` (index into `loadedTasks`), not a task ID string. This is consistent with how `updateTaskPanelKeys()` accesses tasks.

---

### Subtask T007 - Add `updateBlockedConfirmDialog()` key handler

**Purpose**: Handle keyboard input when the blocked confirm dialog is visible. Follows the quit confirm key handling pattern exactly.

**Steps**:
1. In `internal/tui/overlays.go`, add:

   ```go
   func (m *Model) updateBlockedConfirmDialog(msg tea.Msg) (tea.Model, tea.Cmd) {
       keyMsg, ok := msg.(tea.KeyMsg)
       if !ok {
           return m, nil
       }

       switch {
       case key.Matches(keyMsg, m.keys.Back):
           // Esc cancels
           m.closeBlockedConfirmDialog()
           return m, nil

       case keyMsg.String() == "left" || keyMsg.String() == "right" || keyMsg.String() == "tab":
           // Cycle between buttons: 0 = "spawn anyway", 1 = "cancel"
           if m.blockedConfirmFocused == 0 {
               m.blockedConfirmFocused = 1
           } else {
               m.blockedConfirmFocused = 0
           }
           return m, nil

       case keyMsg.String() == "enter":
           if m.blockedConfirmFocused == 0 {
               // "spawn anyway" selected
               taskIdx := m.blockedConfirmTaskIdx
               m.closeBlockedConfirmDialog()
               return m, func() tea.Msg {
                   return blockedConfirmProceedMsg{TaskIdx: taskIdx}
               }
           }
           // "cancel" selected
           m.closeBlockedConfirmDialog()
           return m, nil
       }

       return m, nil
   }
   ```

2. The handler follows these conventions:
   - `keys.Back` (esc) always cancels/closes.
   - Left/right/tab toggle between the two buttons (binary toggle, same as quit confirm).
   - Enter on button 0 ("spawn anyway") emits `blockedConfirmProceedMsg` as a tea.Cmd.
   - Enter on button 1 ("cancel") just closes the dialog.
   - Capture `taskIdx` before `closeBlockedConfirmDialog()` resets it to 0.

**Files**: `internal/tui/overlays.go` (~30 lines added)

**Parallel?**: No - depends on T006 (`closeBlockedConfirmDialog`) and uses `blockedConfirmProceedMsg` from WP01 T003.

**Notes**:
- The `blockedConfirmProceedMsg` is returned as a `tea.Cmd` (closure returning the msg), not dispatched directly. This follows the bubbletea v2 pattern where messages flow through the Update loop.
- Must import `"github.com/charmbracelet/bubbles/v2/key"` - but this is already imported in overlays.go.

---

### Subtask T008 - Add `renderBlockedConfirmDialog()` renderer

**Purpose**: Render the confirmation dialog overlay following the quit confirm visual pattern. Shows the blocked task info, lists unfinished dependencies with states, and presents "spawn anyway" / "cancel" buttons.

**Steps**:
1. In `internal/tui/overlays.go`, add the renderer:

   ```go
   func (m *Model) renderBlockedConfirmDialog() string {
       if m.blockedConfirmTaskIdx < 0 || m.blockedConfirmTaskIdx >= len(m.loadedTasks) {
           return m.renderWithBackdrop("")
       }

       t := m.loadedTasks[m.blockedConfirmTaskIdx]
       deps := m.unfinishedDeps(t)

       // Header
       header := lipgloss.NewStyle().Foreground(colorOrange).Bold(true).Render("! blocked task")

       // Task info
       taskInfo := lipgloss.NewStyle().Foreground(colorLightGray).Render(
           fmt.Sprintf("%s - %s", t.ID, t.Title),
       )

       // Dependency list
       depLines := make([]string, 0, len(deps))
       for _, dep := range deps {
           stateStyled := lipgloss.NewStyle().Foreground(colorOrange).Render(dep.State)
           depLines = append(depLines, fmt.Sprintf("  %s (%s)", dep.ID, stateStyled))
       }

       var depSection string
       if len(depLines) > 0 {
           depHeader := lipgloss.NewStyle().Foreground(colorCream).Render("unfinished dependencies:")
           depSection = lipgloss.JoinVertical(lipgloss.Left,
               depHeader,
               strings.Join(depLines, "\n"),
           )
       } else {
           depSection = lipgloss.NewStyle().Foreground(colorMidGray).Render("(no unfinished dependencies detected)")
       }

       // Buttons (same pattern as quit confirm)
       spawnStyle := inactiveButtonStyle
       cancelStyle := inactiveButtonStyle
       if m.blockedConfirmFocused == 0 {
           spawnStyle = alertButtonStyle
       } else {
           cancelStyle = activeButtonStyle
       }

       buttons := lipgloss.JoinHorizontal(
           lipgloss.Left,
           spawnStyle.Render("spawn anyway"),
           "  ",
           cancelStyle.Render("cancel"),
       )

       // Help text
       helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render("left/right or tab switch  enter select  esc cancel")

       // Assemble
       content := lipgloss.JoinVertical(
           lipgloss.Left,
           header,
           "",
           taskInfo,
           "",
           depSection,
           "",
           buttons,
           "",
           helpText,
       )

       dialog := alertDialogStyle.Width(64).Render(content)
       return m.renderWithBackdrop(dialog)
   }
   ```

2. Key visual elements:
   - **Header**: Orange bold warning icon + "blocked task" (same as quit confirm header style).
   - **Task info**: Shows `WP## - Title` in light gray so the user knows which task they selected.
   - **Dep list**: Each unfinished dep on its own line with ID and state in orange. Indented with 2 spaces.
   - **Buttons**: "spawn anyway" (alert style when focused, orange background) | "cancel" (active style when focused, purple background).
   - **Default focus**: Cancel button (index 1), set by `openBlockedConfirmDialog`.
   - **Dialog width**: 64 chars (same as quit confirm).
   - **Backdrop**: Uses `renderWithBackdrop()` for the standard dimmed background overlay.

**Files**: `internal/tui/overlays.go` (~55 lines added)

**Parallel?**: No - depends on T005 (`unfinishedDeps`), T006 (lifecycle methods).

**Notes**:
- `fmt` and `strings` are already imported in overlays.go.
- The warning icon uses `!` instead of a unicode warning triangle for maximum terminal compatibility. The spec doesn't mandate a specific icon.
- Per AD-005, `unfinishedDeps(t)` is called inside the render method, computing fresh data each render cycle.
- The `deps` slice may be empty if all dependencies completed while the dialog was open. This is handled gracefully with a "(no unfinished dependencies detected)" message.

---

### Subtask T009 - Wire blocked task dispatch into Update chain

**Purpose**: Connect the blocked confirm dialog to the bubbletea Update loop:
1. When enter is pressed on a blocked task, open the dialog instead of doing nothing.
2. When the dialog is visible, route all messages through its handler.
3. When "spawn anyway" is confirmed, open the spawn dialog pre-filled.

**Steps**:

**Step 9a - Add blocked branch to `updateTaskPanelKeys()` Select handler:**

1. Open `internal/tui/update.go`.
2. Locate `updateTaskPanelKeys()` at line 908.
3. In the `case key.Matches(msg, m.keys.Select) || msg.String() == " ":` handler (line 947), add a `TaskBlocked` branch BEFORE the existing `TaskUnassigned` check:

   ```go
   case key.Matches(msg, m.keys.Select) || msg.String() == " ":
       if m.selectedTaskIdx >= 0 && m.selectedTaskIdx < len(m.loadedTasks) {
           t := m.loadedTasks[m.selectedTaskIdx]
           if t.State == task.TaskBlocked {
               m.openBlockedConfirmDialog(m.selectedTaskIdx)
               return m, nil
           }
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

   The blocked check must come BEFORE the unassigned check since both react to the Select key.

**Step 9b - Add `showBlockedConfirm` dispatch in `Update()`:**

4. In the `Update()` method at the top of the file (line 18), add a `showBlockedConfirm` check between the `showNewDialog` check (line 43) and the `showSpawnDialog` check (line 47):

   ```go
   if m.showNewDialog {
       return m.updateNewDialog(msg)
   }

   if m.showBlockedConfirm {
       return m.updateBlockedConfirmDialog(msg)
   }

   if m.showSpawnDialog {
       return m.updateSpawnDialog(msg)
   }
   ```

   This position ensures:
   - Higher-priority overlays (quit confirm, settings, continue dialog, history) still take precedence.
   - The blocked confirm dialog intercepts keys before the spawn dialog.

**Step 9c - Add `blockedConfirmProceedMsg` handler:**

5. In the `switch msg := msg.(type)` block in `Update()`, add a case for `blockedConfirmProceedMsg`:

   ```go
   case blockedConfirmProceedMsg:
       if msg.TaskIdx >= 0 && msg.TaskIdx < len(m.loadedTasks) {
           t := m.loadedTasks[msg.TaskIdx]
           role := t.SuggestedRole
           if role == "" {
               role = "coder"
           }
           return m, m.openSpawnDialogWithTaskPrefill(role, strings.TrimSpace(t.Description), nil, t.ID)
       }
       return m, nil
   ```

   This reuses the same `openSpawnDialogWithTaskPrefill()` call as the unassigned task path, fulfilling FR-006 (identical behavior).

**Files**: `internal/tui/update.go` (~20 lines added/modified)

**Parallel?**: No - depends on T006, T007 (dialog methods).

**Notes**:
- The dispatch order in `Update()` is critical. Research R-004 analyzed the existing order and determined the correct insertion point. Do not place the `showBlockedConfirm` check after `showSpawnDialog`.
- The `blockedConfirmProceedMsg` handler does a bounds check on `TaskIdx` to guard against stale indices (e.g., if tasks were reloaded between dialog open and confirm).
- `openSpawnDialogWithTaskPrefill` takes `(role string, prompt string, files []string, taskID string)`. Pass `nil` for files.

---

### Subtask T010 - Add `showBlockedConfirm` render check in View()

**Purpose**: Insert the dialog render call in the View() overlay cascade so the dialog is drawn when visible.

**Steps**:
1. Open `internal/tui/model.go`.
2. In the `View()` method, locate the overlay cascade section. There are two locations that need updating:

   **Location 1 - Fullscreen mode** (around line 322-328):

   Add `showBlockedConfirm` check between `showBatchDialog` and `showSpawnDialog`:

   ```go
   if m.showBatchDialog {
       return m.renderBatchDialog()
   }
   if m.showBlockedConfirm {
       return m.renderBlockedConfirmDialog()
   }
   if m.showSpawnDialog {
       return m.renderSpawnDialog()
   }
   ```

   **Location 2 - Normal mode** (around line 371-379):

   Add the same check in the same position:

   ```go
   if m.showBatchDialog {
       return m.renderBatchDialog()
   }
   if m.showBlockedConfirm {
       return m.renderBlockedConfirmDialog()
   }
   if m.showSpawnDialog {
       return m.renderSpawnDialog()
   }
   ```

3. Both locations follow the same insertion point: between `showBatchDialog` and `showSpawnDialog`.

**Files**: `internal/tui/model.go` (~6 lines added, in 2 locations)

**Parallel?**: No - depends on T008 (`renderBlockedConfirmDialog`).

**Notes**:
- The View() overlay cascade determines which overlay is rendered. Only one overlay renders at a time (early returns).
- The blocked confirm must render BEFORE the spawn dialog. If both were somehow true (shouldn't happen but defensive), the confirm takes priority.
- The `showLauncher` branch (line 289-303) does NOT need a `showBlockedConfirm` check because the task panel is not visible in launcher mode.

---

## Risks & Mitigations

1. **Dispatch order mistake**: Placing the `showBlockedConfirm` check at the wrong position in `Update()` could cause keys to be swallowed by another overlay. Mitigation: Research R-004 explicitly analyzed and documented the correct insertion point. Follow it exactly.

2. **Stale task index**: Between opening the dialog and confirming, the `loadedTasks` slice could theoretically change (task source reload). Mitigation: Bounds-check `msg.TaskIdx` in the `blockedConfirmProceedMsg` handler before accessing the slice.

3. **Dialog visible with no unfinished deps**: If all dependencies complete while the dialog is open, the dep list renders empty. Mitigation: `renderBlockedConfirmDialog` handles this with a "(no unfinished dependencies detected)" fallback message. The user can still proceed or cancel normally.

4. **Regression in unblocked task spawning**: Changes to `updateTaskPanelKeys()` could accidentally break the existing unassigned task spawn flow. Mitigation: The `TaskBlocked` branch is added BEFORE the `TaskUnassigned` branch with an early return, so the unassigned path remains untouched.

---

## Review Guidance

- **FR-003 check**: Press enter on a blocked task. Confirm dialog MUST appear (not the spawn dialog, not nothing).
- **FR-004 check**: Confirm dialog lists ONLY unfinished dependency IDs. Done dependencies must NOT appear.
- **FR-005 check**: Two buttons visible: "spawn anyway" and "cancel". Both work correctly.
- **FR-006 check**: "spawn anyway" opens the standard spawn dialog pre-filled with role and description, identical to spawning an unassigned task.
- **FR-009 check**: Escape key dismisses the dialog without action.
- **AD-002 check**: Dialog uses `alertDialogStyle` (orange thick border), consistent with quit confirm.
- **Dispatch order check**: With the dialog open, no other overlay's keys should be reachable.
- **Regression check**: Enter on an unassigned task still opens the spawn dialog directly (no confirm).
- **Build check**: `go build ./cmd/kasmos` compiles cleanly. `go vet ./internal/tui/...` passes.

---

## Activity Log

- 2026-02-19T04:05:55Z - system - lane=planned - Prompt created.
