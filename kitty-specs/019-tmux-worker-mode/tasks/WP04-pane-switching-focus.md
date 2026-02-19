---
work_package_id: "WP04"
subtasks:
  - "T020"
  - "T021"
  - "T022"
  - "T023"
  - "T024"
  - "T025"
title: "Pane Switching & Focus Management"
phase: "Phase 2 - TUI Integration"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP03"]
history:
  - timestamp: "2026-02-19T03:53:34Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP04 - Pane Switching & Focus Management

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
spec-kitty implement WP04 --base WP03
```

Depends on WP03 (CLI flag and tmux state in Model).

---

## Objectives & Success Criteria

1. **Tmux initialization**: On startup in tmux mode, kasmos pane ID is captured and parking window is confirmed ready (via `tmuxInitMsg`).
2. **Pane swap on selection**: When the user selects a different worker in the table, the right-side pane swaps to show the selected worker's terminal.
3. **Focus management**: Keyboard focus automatically moves to the worker's terminal pane when a worker is selected (FR-008).
4. **Empty state**: When no workers are spawned, the right column shows a placeholder indicating the pane area (FR-006).
5. **SC-002**: Pane switch completes in under 1 second.

**Requirements covered**: FR-006, FR-007, FR-008, User Story 1 (layout), User Story 2 (spawn visible), User Story 3 (switch visible).

## Context & Constraints

- **bubbletea message pattern**: All tmux CLI calls must be wrapped in `tea.Cmd` functions (async). Never call tmux CLI directly in `Update()`.
- **Constitution**: "TUI never blocks Update loop" - all I/O in commands, not handlers.
- **Existing selection handling**: `update.go` tracks `selectedWorkerID` and refreshes viewport on change. In tmux mode, swap pane instead of refreshing viewport.
- **Existing message pattern**: See `internal/tui/messages.go` for the established msg struct pattern.
- **Existing command pattern**: See `internal/tui/commands.go` for `tea.Cmd` function pattern.

**Key reference files**:
- `internal/tui/messages.go` - Existing message types
- `internal/tui/commands.go` - Existing command functions
- `internal/tui/update.go` - Main Update handler, selection tracking
- `internal/tui/panels.go` - Viewport rendering
- `internal/tui/model.go` - Model with tmux fields (from WP03)
- `internal/worker/tmux.go` - TmuxBackend.SwapActive(), ShowPane() (from WP02)

---

## Subtasks & Detailed Guidance

### Subtask T020 - Define tmux-specific message types

**Purpose**: Messages that carry tmux operation results through the bubbletea message loop. Following the established pattern in `messages.go`.

**Steps**:
1. In `internal/tui/messages.go`, add the following message types:

```go
// tmuxInitMsg carries the result of TmuxBackend initialization.
type tmuxInitMsg struct {
    KasmosPaneID  string
    ParkingWindow string
    Err           error
}

// paneSwappedMsg reports that a pane swap completed.
type paneSwappedMsg struct {
    WorkerID string // the worker now visible
    PaneID   string // tmux pane ID now visible
    Err      error
}

// paneExitedMsg reports that a worker pane's process exited.
type paneExitedMsg struct {
    WorkerID string
    PaneID   string
    ExitCode int
    Output   string // captured pane content for session ID extraction
}

// paneDetectedMsg carries results of reattach pane scan.
type paneDetectedMsg struct {
    Workers []worker.ReconnectedWorker
    Err     error
}

// paneFocusMsg requests focus change to a specific pane.
type paneFocusMsg struct {
    PaneID string
    Err    error
}
```

**Files**: `internal/tui/messages.go` (modify, ~30 lines added)
**Parallel?**: Yes - independent file, can proceed alongside T025.

---

### Subtask T021 - Implement tmuxInitCmd

**Purpose**: A `tea.Cmd` that confirms TmuxBackend readiness. Called from `Init()` when `tmuxMode` is true. The Init() was done in main.go, but the TUI needs to know it completed and store the results.

**Steps**:
1. In `internal/tui/commands.go`, add:

```go
// tmuxInitCmd confirms tmux backend initialization.
// The actual Init() was called in main.go; this command reads the state
// and reports it to the TUI via tmuxInitMsg.
func tmuxInitCmd(backend *worker.TmuxBackend) tea.Cmd {
    return func() tea.Msg {
        if backend == nil {
            return tmuxInitMsg{Err: errors.New("tmux backend is nil")}
        }
        return tmuxInitMsg{
            KasmosPaneID:  backend.KasmosPaneID(),
            ParkingWindow: "", // parking window ID tracked by backend
        }
    }
}
```

2. In `internal/tui/model.go`, update `Init()` to dispatch `tmuxInitCmd` when in tmux mode:

```go
func (m *Model) Init() (tea.Model, tea.Cmd) {
    m.tickActive = true
    cmds := []tea.Cmd{tickCmd(), m.spinner.Tick}
    if m.tmuxMode && m.tmuxBackend != nil {
        cmds = append(cmds, tmuxInitCmd(m.tmuxBackend))
    }
    // ... rest of existing Init logic ...
    return m, tea.Batch(cmds...)
}
```

3. In `internal/tui/update.go`, handle `tmuxInitMsg`:

```go
case tmuxInitMsg:
    if msg.Err != nil {
        // Tmux init failed - disable tmux mode, show error in status
        m.tmuxMode = false
        m.tmuxBackend = nil
        // Show error in viewport
        m.setViewportContent(fmt.Sprintf("tmux initialization failed: %v", msg.Err), false)
    } else {
        m.tmuxReady = true
    }
    return m, nil
```

**Files**:
- `internal/tui/commands.go` (modify, ~15 lines added)
- `internal/tui/model.go` (modify, ~3 lines in Init)
- `internal/tui/update.go` (modify, ~12 lines in switch)
**Parallel?**: No - other commands depend on tmuxReady.

---

### Subtask T022 - Implement paneSwapCmd

**Purpose**: A `tea.Cmd` that triggers a pane swap operation via TmuxBackend. Called when the user selects a different worker in tmux mode.

**Steps**:
1. In `internal/tui/commands.go`, add:

```go
// paneSwapCmd swaps the visible worker pane to show a different worker.
func paneSwapCmd(backend *worker.TmuxBackend, workerID string) tea.Cmd {
    return func() tea.Msg {
        if backend == nil {
            return paneSwappedMsg{WorkerID: workerID, Err: errors.New("tmux backend is nil")}
        }
        if err := backend.SwapActive(workerID); err != nil {
            return paneSwappedMsg{WorkerID: workerID, Err: err}
        }
        return paneSwappedMsg{
            WorkerID: workerID,
            PaneID:   backend.ActivePaneID(),
        }
    }
}
```

2. In `internal/tui/update.go`, handle `paneSwappedMsg`:

```go
case paneSwappedMsg:
    if msg.Err != nil {
        // Show swap error in status bar or viewport
        m.setViewportContent(fmt.Sprintf("pane swap failed: %v", msg.Err), false)
    }
    // Pane is now visible and focused via tmux - no TUI state change needed
    // The viewport in tmux mode shows metadata, not output
    m.updateKeyStates()
    m.triggerPersist()
    return m, nil
```

**Files**:
- `internal/tui/commands.go` (modify, ~15 lines added)
- `internal/tui/update.go` (modify, ~10 lines in switch)
**Parallel?**: No - depends on T020 message types.

---

### Subtask T023 - Implement paneFocusCmd

**Purpose**: A `tea.Cmd` that moves tmux keyboard focus to a specific pane. Used for: (a) focusing a worker pane when selected, (b) returning focus to kasmos dashboard when a worker exits.

**Steps**:
1. In `internal/tui/commands.go`, add:

```go
// paneFocusCmd moves tmux keyboard focus to a specific pane.
func paneFocusCmd(backend *worker.TmuxBackend, paneID string) tea.Cmd {
    return func() tea.Msg {
        if backend == nil {
            return paneFocusMsg{PaneID: paneID, Err: errors.New("tmux backend is nil")}
        }
        cli := backend.CLI() // Need accessor for the TmuxCLI
        if cli == nil {
            return paneFocusMsg{PaneID: paneID, Err: errors.New("tmux cli is nil")}
        }
        err := cli.SelectPane(paneID)
        return paneFocusMsg{PaneID: paneID, Err: err}
    }
}
```

2. **Important**: TmuxBackend needs a `CLI()` accessor or the focus command should go through TmuxBackend:

Option A: Add `CLI()` accessor to TmuxBackend (simple but breaks encapsulation):
```go
func (b *TmuxBackend) CLI() TmuxCLI { return b.cli }
```

Option B: Add a `FocusPane(paneID)` method to TmuxBackend (better encapsulation):
```go
func (b *TmuxBackend) FocusPane(paneID string) error {
    return b.cli.SelectPane(paneID)
}
```

**Recommended**: Option B. Update paneFocusCmd to use `backend.FocusPane()`:

```go
func paneFocusCmd(backend *worker.TmuxBackend, paneID string) tea.Cmd {
    return func() tea.Msg {
        if backend == nil {
            return paneFocusMsg{PaneID: paneID, Err: errors.New("tmux backend is nil")}
        }
        err := backend.FocusPane(paneID)
        return paneFocusMsg{PaneID: paneID, Err: err}
    }
}
```

3. Add `FocusPane` to `internal/worker/tmux.go` (WP02 file, additive):

```go
func (b *TmuxBackend) FocusPane(paneID string) error {
    return b.cli.SelectPane(paneID)
}
```

4. In `internal/tui/update.go`, handle `paneFocusMsg`:

```go
case paneFocusMsg:
    if msg.Err != nil {
        // Non-fatal: log but don't crash
    }
    return m, nil
```

**Files**:
- `internal/tui/commands.go` (modify, ~12 lines added)
- `internal/worker/tmux.go` (modify, ~4 lines - add FocusPane)
- `internal/tui/update.go` (modify, ~5 lines in switch)
**Parallel?**: No - depends on T020 message types and T022 pattern.

---

### Subtask T024 - Update selection handling in update.go for tmux pane swap

**Purpose**: When the user navigates the worker table and selects a different worker, trigger a pane swap in tmux mode instead of refreshing the viewport content.

**Steps**:
1. Find the location in `update.go` where `selectedWorkerID` changes. This happens in the table key handling when the cursor moves.

2. The current flow: when table selection changes, `refreshViewportFromSelected()` is called which updates the viewport content with the selected worker's output.

3. In tmux mode, instead of (or in addition to) refreshing the viewport, trigger a pane swap:

```go
// After detecting selection change:
if m.tmuxMode && m.tmuxReady && m.tmuxBackend != nil {
    if newWorkerID != "" && newWorkerID != previousWorkerID {
        // Check if this worker has an active pane
        w := m.manager.Get(newWorkerID)
        if w != nil && w.State == worker.StateRunning {
            cmds = append(cmds, paneSwapCmd(m.tmuxBackend, newWorkerID))
        }
    }
} else {
    m.refreshViewportFromSelected(false)
}
```

4. Look for the existing selection change detection. It likely happens in `updateTableKeys` or after table model update. The key code path is:
   - User presses up/down in table
   - Table cursor moves
   - `selectedWorkerID` is updated from table row
   - Viewport content is refreshed

   In tmux mode, replace step 4 with pane swap.

5. Also handle the case when a worker is spawned (first worker in tmux mode should automatically show):

In the `workerSpawnedMsg` handler, if tmux mode, the pane was already created and shown by `TmuxBackend.Spawn()`. No additional swap needed, but update `selectedWorkerID` to the new worker.

**Files**: `internal/tui/update.go` (modify, ~20 lines in key handling and spawn handler)
**Parallel?**: No - depends on T022 (paneSwapCmd).

**Edge Cases**:
- Selecting a dead worker: Still swap to show the dead pane's final state (scrollable history).
- Selecting a worker with no pane (subprocess fallback): Shouldn't happen in tmux mode, but guard.
- Rapid up/down navigation: Each swap triggers a `tea.Cmd`. If the user navigates faster than swaps complete, the commands queue up. Consider a debounce or "latest wins" guard.

---

### Subtask T025 - Render tmux placeholder in viewport for empty state

**Purpose**: When tmux mode is active but no workers are spawned (or no worker is selected), the right column should show informative placeholder text instead of the standard output viewport.

**Steps**:
1. In `internal/tui/panels.go`, update `renderViewport()`:

```go
func (m *Model) renderViewport() string {
    if m.viewportInnerWidth <= 0 || m.viewportInnerHeight <= 0 {
        return ""
    }

    // Tmux mode: show metadata/placeholder instead of output
    if m.tmuxMode {
        return m.renderTmuxViewportPlaceholder()
    }

    // ... existing viewport rendering ...
}
```

2. Add the placeholder renderer:

```go
func (m *Model) renderTmuxViewportPlaceholder() string {
    title := "tmux worker pane"

    var body string
    if selected := m.selectedWorker(); selected != nil {
        // Worker is selected - show its metadata
        title = fmt.Sprintf("worker: %s %s", selected.ID, selected.Role)
        lines := []string{
            "",
            "  Interactive worker pane is displayed in the",
            "  tmux pane to the right of this dashboard.",
            "",
            fmt.Sprintf("  Worker: %s", selected.ID),
            fmt.Sprintf("  Role: %s", selected.Role),
            fmt.Sprintf("  Status: %s", selected.State),
            "",
            "  Use tmux prefix + arrow keys to switch focus.",
            "  Worker pane receives keyboard input directly.",
        }
        body = strings.Join(lines, "\n")
    } else {
        // No worker selected
        lines := []string{
            "",
            "  Tmux worker mode active.",
            "",
            "  Spawn a worker to see its interactive terminal",
            "  in the pane to the right.",
            "",
            "  Press n to create your first task.",
        }
        body = strings.Join(lines, "\n")
    }

    content := lipgloss.JoinVertical(
        lipgloss.Left,
        lipgloss.NewStyle().Foreground(colorHeader).Bold(true).Render(title),
        lipgloss.NewStyle().
            MaxWidth(m.viewportInnerWidth).
            MaxHeight(max(1, m.viewportInnerHeight-1)).
            Foreground(colorMidGray).
            Render(body),
    )

    return panelStyle(m.focused == panelViewport).
        Width(m.viewportInnerWidth).
        Render(content)
}
```

Note: Adjust the `panelStyle` call to match the current signature in the codebase. The existing code may use different parameters.

**Files**: `internal/tui/panels.go` (modify, ~40 lines added)
**Parallel?**: Yes - can proceed alongside T020 (different files).

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Pane swap during rapid table navigation | Race conditions, visual glitch | Commands are queued by bubbletea. Consider "swap in progress" guard that ignores subsequent swaps until paneSwappedMsg arrives. |
| tmuxInitCmd failure | Tmux features broken | Disable tmux mode, fall back to standard viewport. Show error in status. |
| Selected worker has no tmux pane | Swap fails | Guard: only swap for workers with state Running and a tracked pane in TmuxBackend. |
| Viewport rendering conflicts | Visual artifacts | In tmux mode, viewport shows metadata only (static text), not live output. No conflict with tmux pane rendering. |

## Review Guidance

- Verify all tmux CLI calls are wrapped in `tea.Cmd` (never in `Update` directly).
- Verify `tmuxInitMsg` error handling disables tmux mode gracefully.
- Verify pane swap is only triggered for actual selection changes (not on every key press).
- Verify the viewport placeholder is rendered correctly in both narrow and wide layouts.
- Verify the empty state placeholder has helpful text.
- Verify focus moves to the worker pane (via SelectPane in SwapActive).
- Test scenario: spawn 3 workers, navigate between them, verify right pane changes.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
