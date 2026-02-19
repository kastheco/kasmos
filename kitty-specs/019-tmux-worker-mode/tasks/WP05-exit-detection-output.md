---
work_package_id: "WP05"
subtasks:
  - "T026"
  - "T027"
  - "T028"
  - "T029"
  - "T030"
  - "T031"
title: "Exit Detection, Output Capture & Key Disabling"
phase: "Phase 2 - TUI Integration"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP04"]
history:
  - timestamp: "2026-02-19T03:53:34Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP05 - Exit Detection, Output Capture & Key Disabling

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
spec-kitty implement WP05 --base WP04
```

Depends on WP04 (message types and pane commands).

---

## Objectives & Success Criteria

1. **Worker exit detection**: Dead tmux panes are detected within 2 seconds (SC-004) via tick-based polling.
2. **Session ID extraction**: When a worker exits, its pane content is captured and the session ID is extracted using the existing regex.
3. **Externally killed panes**: If a user manually kills a pane (`tmux kill-pane`), kasmos detects it and marks the worker as killed (FR-014).
4. **Interactive handle skip**: `workerSpawnedMsg` handler skips `readWorkerOutput()` and `waitWorkerCmd()` for interactive handles.
5. **AI helper keys disabled**: Analyze and GenPrompt keys are hidden/disabled in tmux mode (FR-017).
6. **Auto-focus return**: When the focused worker exits, focus returns to the kasmos dashboard pane (FR-009).

**Requirements covered**: FR-009, FR-011, FR-014, FR-017.

## Context & Constraints

- **Existing tick handler**: `tickMsg` in update.go already handles duration updates and spinner ticks. Add tmux polling here.
- **Existing exit flow**: `workerExitedMsg` triggers state update, table refresh, session ID extraction, task state update, persist. Reuse this flow.
- **Existing output reading**: `readWorkerOutput()` in commands.go starts a goroutine reading from `handle.Stdout()`. For interactive handles, `Stdout()` returns `nil` - must skip.
- **Existing key state management**: `updateKeyStates()` in keys.go manages enabled/disabled state for all keys.

**Key reference files**:
- `internal/tui/update.go` - tickMsg handler, workerSpawnedMsg handler, workerExitedMsg handler
- `internal/tui/commands.go` - readWorkerOutput(), waitWorkerCmd(), extractSessionID()
- `internal/tui/keys.go` - updateKeyStates()
- `internal/tui/messages.go` - paneExitedMsg (from WP04)
- `internal/worker/tmux.go` - PollPanes(), tmuxHandle.NotifyExit(), CaptureOutput()

---

## Subtasks & Detailed Guidance

### Subtask T026 - Add tmux pane polling to tick handler

**Purpose**: Every second, check managed tmux panes for status changes (dead or missing). This is the primary exit detection mechanism for tmux mode.

**Steps**:
1. In `internal/tui/update.go`, find the `tickMsg` handler. Currently it updates worker durations and triggers the next tick.

2. Add tmux polling after existing tick logic:

```go
case tickMsg:
    // ... existing duration update logic ...

    // Tmux mode: poll pane status
    if m.tmuxMode && m.tmuxReady && m.tmuxBackend != nil {
        cmds = append(cmds, tmuxPollCmd(m.tmuxBackend))
    }

    // ... existing tick continuation ...
```

3. In `internal/tui/commands.go`, add the poll command:

```go
// tmuxPollCmd polls tmux panes for status changes.
// Called every tick (1 second) when in tmux mode.
func tmuxPollCmd(backend *worker.TmuxBackend) tea.Cmd {
    return func() tea.Msg {
        if backend == nil {
            return nil
        }
        statuses, err := backend.PollPanes()
        if err != nil {
            // Non-fatal: log but don't crash. Will retry on next tick.
            return nil
        }
        if len(statuses) == 0 {
            return nil
        }
        // Return the first status change. Multiple changes are handled
        // across multiple ticks (one per second is fast enough for SC-004).
        s := statuses[0]
        if s.Missing {
            return workerKilledMsg{WorkerID: s.WorkerID}
        }
        if s.Dead {
            return paneExitedMsg{
                WorkerID: s.WorkerID,
                PaneID:   s.PaneID,
                ExitCode: s.ExitCode,
            }
        }
        return nil
    }
}
```

**Alternative**: Return all status changes at once. Create a batch message or multiple cmds. But the simpler approach (one per tick) is fine given 1-second granularity.

**Files**:
- `internal/tui/update.go` (modify, ~5 lines in tickMsg handler)
- `internal/tui/commands.go` (modify, ~25 lines added)
**Parallel?**: No - foundational for T027/T028.

---

### Subtask T027 - Handle paneExitedMsg: capture output, extract session ID

**Purpose**: When a pane death is detected, capture its content for session ID extraction, then flow into the existing `workerExitedMsg` pipeline.

**Steps**:
1. In `internal/tui/update.go`, add handler for `paneExitedMsg`:

```go
case paneExitedMsg:
    // Capture pane output for session ID extraction
    w := m.manager.Get(msg.WorkerID)
    if w == nil {
        return m, nil
    }

    // Get the tmux handle to capture output
    var sessionID string
    if h, ok := w.Handle.(*worker.TmuxHandle); ok {
        output, err := h.CaptureOutput()
        if err == nil && output != "" {
            sessionID = strings.TrimSpace(worker.ExtractSessionID(output))
            // Store captured output in the worker's output buffer for persistence
            if w.Output == nil {
                w.Output = worker.NewOutputBuffer(worker.DefaultMaxLines)
            }
            w.Output.Append(output)
        }
    }

    // Notify the handle that the pane exited (unblocks Wait())
    if h, ok := w.Handle.(*worker.TmuxHandle); ok {
        h.NotifyExit(msg.ExitCode, time.Since(w.SpawnedAt))
    }

    // Flow into existing exit handling
    return m, func() tea.Msg {
        return workerExitedMsg{
            WorkerID:  msg.WorkerID,
            ExitCode:  msg.ExitCode,
            Duration:  time.Since(w.SpawnedAt),
            SessionID: sessionID,
        }
    }
```

2. **Important**: The `tmuxHandle` type is unexported (`tmuxHandle`). The type assertion needs either:
   - Export the type as `TmuxHandle` (adjust in WP02 implementation)
   - Or add a method to WorkerHandle for capture: e.g., check `Interactive()` and use a `CaptureOutput` interface

   **Recommended approach**: Keep `tmuxHandle` unexported but add a type-safe approach:

```go
// In internal/worker/backend.go, add an optional interface:
type OutputCapturer interface {
    CaptureOutput() (string, error)
}

type ExitNotifier interface {
    NotifyExit(code int, duration time.Duration)
}
```

Then in the handler:
```go
    if capturer, ok := w.Handle.(worker.OutputCapturer); ok {
        output, err := capturer.CaptureOutput()
        // ...
    }
    if notifier, ok := w.Handle.(worker.ExitNotifier); ok {
        notifier.NotifyExit(msg.ExitCode, time.Since(w.SpawnedAt))
    }
```

This avoids exporting the tmuxHandle type while still enabling the TUI to call its methods.

**Files**:
- `internal/tui/update.go` (modify, ~25 lines in switch)
- `internal/worker/backend.go` (modify, ~10 lines - optional interfaces)
**Parallel?**: No - depends on T026 for the message flow.

**Edge Cases**:
- `CaptureOutput` fails (pane already destroyed): sessionID remains empty. Not fatal.
- Worker has no output buffer: Create one on-the-fly for persistence.
- `NotifyExit` already called (duplicate detection): `tmuxHandle.NotifyExit` is idempotent (guarded by `exited` flag).

---

### Subtask T028 - Handle externally killed panes

**Purpose**: When `PollPanes()` reports a pane as missing (not dead, but gone), the user has manually killed it via `tmux kill-pane` or similar. Map this to the existing `workerKilledMsg` flow (FR-014).

**Steps**:
1. The polling command (T026) already emits `workerKilledMsg` when `PaneStatus.Missing == true`. The existing `workerKilledMsg` handler in update.go should already handle this:

```go
case workerKilledMsg:
    // Existing handler marks worker as killed, updates table, persists
    // ...
```

2. Verify the existing handler works for this case. Check that:
   - Worker state transitions to `StateKilled`
   - Worker exit time is set
   - Table is refreshed
   - Session is persisted
   - If the killed pane was the active one, clear `activePaneID` in the backend

3. If additional tmux-specific cleanup is needed, add it:

```go
case workerKilledMsg:
    // ... existing handling ...

    // Tmux cleanup: if the killed worker's pane was active, clear it
    if m.tmuxMode && m.tmuxBackend != nil {
        // The pane is already gone, just clear tracking
        // Backend's managedPanes will be updated on next PollPanes
    }
```

4. **Key insight**: The `workerKilledMsg` flow already exists for subprocess kills. The same flow handles externally killed tmux panes. The only difference is the source of the message (poll vs. direct kill command).

**Files**: `internal/tui/update.go` (verify existing handler, ~5 lines if adjustments needed)
**Parallel?**: Yes - independent code path from T027.

---

### Subtask T029 - Skip readWorkerOutput/waitWorkerCmd for interactive handles

**Purpose**: Interactive tmux handles don't produce output on a pipe (`Stdout()` returns `nil`) and don't exit via `Wait()` (exit is detected by polling). The `workerSpawnedMsg` handler must skip these for interactive handles.

**Steps**:
1. Find the `workerSpawnedMsg` handler in `internal/tui/update.go`. Current code:

```go
case workerSpawnedMsg:
    w := m.manager.Get(msg.WorkerID)
    if w != nil {
        w.Handle = msg.Handle
        w.State = worker.StateRunning
        // ... existing logic ...
    }
    readWorkerOutput(msg.WorkerID, msg.Handle.Stdout(), m.program)
    cmds = append(cmds, waitWorkerCmd(msg.WorkerID, msg.Handle))
```

2. Add conditional based on `Interactive()`:

```go
case workerSpawnedMsg:
    w := m.manager.Get(msg.WorkerID)
    if w != nil {
        w.Handle = msg.Handle
        w.State = worker.StateRunning
        // ... existing logic ...
    }

    if !msg.Handle.Interactive() {
        // Subprocess mode: read output from pipe, wait for process exit
        readWorkerOutput(msg.WorkerID, msg.Handle.Stdout(), m.program)
        cmds = append(cmds, waitWorkerCmd(msg.WorkerID, msg.Handle))
    }
    // For interactive handles: exit detection happens via tick polling (T026)
    // Output is in the tmux pane, not a pipe
```

3. This is the key integration point from research.md section 5:
   > "if `true`, it skips `readWorkerOutput()` and `waitWorkerCmd()` (exit is detected by tick polling instead)"

**Files**: `internal/tui/update.go` (modify, ~5 lines wrapping existing calls)
**Parallel?**: Yes - can proceed alongside T030.

---

### Subtask T030 - Disable AI helper keys in tmux mode

**Purpose**: Analyze Failure and Generate Prompt features depend on captured subprocess output. In tmux mode, output goes to the tmux pane, not a buffer. Disable these keys (FR-017).

**Steps**:
1. In `internal/tui/keys.go`, find `updateKeyStates()`. Add tmux mode guard early:

```go
func (m *Model) updateKeyStates() {
    // ... existing always-enabled keys ...

    if m.analysisMode {
        // ... existing analysis mode handling ...
        return
    }

    // Tmux mode: disable AI helpers (they depend on captured output)
    if m.tmuxMode {
        m.keys.Analyze.SetEnabled(false)
        m.keys.GenPrompt.SetEnabled(false)
    }

    // ... rest of existing logic ...
```

2. More precisely, the `Analyze` and `GenPrompt` keys should be disabled when `m.tmuxMode` is true, regardless of other conditions. Place the check after the analysis mode block but before the per-worker key logic:

```go
    selected := m.selectedWorker()

    // Worker action keys
    isRunning := selected != nil && selected.State == worker.StateRunning
    m.keys.Kill.SetEnabled(isRunning)
    // ... existing logic ...

    // AI helpers - disabled in tmux mode (FR-017)
    if m.tmuxMode {
        m.keys.Analyze.SetEnabled(false)
        m.keys.GenPrompt.SetEnabled(false)
    } else {
        m.keys.Analyze.SetEnabled(selected != nil && selected.State == worker.StateFailed && !m.analysisLoading)
        // ... existing GenPrompt logic ...
    }
```

3. Also update `ShortHelp()` and `FullHelp()` to conditionally exclude these keys when disabled (they're already excluded when disabled via `key.Binding.Enabled()`).

**Files**: `internal/tui/keys.go` (modify, ~10 lines)
**Parallel?**: Yes - independent file from T029.

---

### Subtask T031 - Auto-focus return on worker exit

**Purpose**: When the focused worker's process exits, automatically return keyboard focus to the kasmos dashboard pane (FR-009, User Story 5).

**Steps**:
1. In the `paneExitedMsg` handler (T027), after processing the exit, check if the exited pane was the active one:

```go
case paneExitedMsg:
    // ... existing exit handling from T027 ...

    // Auto-focus return: if the exited pane was the active one, focus kasmos
    if m.tmuxMode && m.tmuxBackend != nil {
        activePaneID := m.tmuxBackend.ActivePaneID()
        if activePaneID == msg.PaneID {
            cmds = append(cmds, paneFocusCmd(m.tmuxBackend, m.tmuxBackend.KasmosPaneID()))
        }
    }

    // Flow into existing exit handling
    return m, tea.Batch(cmds...)
```

2. This also applies to `workerKilledMsg` for externally killed panes:

```go
case workerKilledMsg:
    // ... existing handling ...

    // Auto-focus return for tmux mode
    if m.tmuxMode && m.tmuxBackend != nil {
        // If the killed worker was the active one, return focus
        cmds = append(cmds, paneFocusCmd(m.tmuxBackend, m.tmuxBackend.KasmosPaneID()))
    }
```

3. SC-005: "Users can round-trip between the dashboard and a worker pane with a single key combination in each direction." The return-to-dashboard direction is handled by tmux navigation (prefix + arrow key) or by auto-focus on exit.

**Files**: `internal/tui/update.go` (modify, ~10 lines in two handlers)
**Parallel?**: No - depends on T027 handler.

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Poll adds overhead to tick handler | Slight latency on tick processing | `tmux list-panes` is <10ms. One call per second is negligible vs. the 1-second tick period. |
| `capture-pane` on destroyed pane | CaptureOutput fails | Handle error gracefully; sessionID remains empty. Worker still gets proper exit status. |
| Type assertion for CaptureOutput/NotifyExit | Runtime panic | Use optional interface pattern (OutputCapturer, ExitNotifier) with comma-ok assertions. |
| Multiple panes exit in same tick | Only first processed | Poll returns all statuses; process one per tick. With 1-second polling, this is fine for SC-004 (2-second detection). |
| AI helper keys re-enabled by updateKeyStates | Keys appear/disappear | Place tmux override AFTER all other key logic, or use an early return for tmux-specific overrides. |

## Review Guidance

- Verify `readWorkerOutput` and `waitWorkerCmd` are skipped for interactive handles (T029).
- Verify Analyze and GenPrompt keys are disabled in tmux mode (T030).
- Verify pane exit triggers session ID extraction from captured output (T027).
- Verify externally killed panes are detected and handled (T028).
- Verify focus returns to kasmos pane on worker exit (T031).
- Verify no blocking tmux CLI calls in Update() - all wrapped in tea.Cmd.
- Test scenario: spawn worker, let it exit, verify status updates in <2 seconds.
- Test scenario: spawn worker, manually `tmux kill-pane`, verify kasmos shows killed status.

## Activity Log

- 2026-02-19T03:53:34Z - system - lane=planned - Prompt created.
