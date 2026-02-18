---
work_package_id: WP05
title: Continue Dialog + Overlays + Worker Chains
lane: done
dependencies:
- WP04
subtasks:
- Continue session dialog (huh form + parent info)
- Quit confirmation dialog
- Worker continuation chains (ParentID, tree glyphs)
- Viewport title shows chain reference
- Update key handlers for c (continue)
phase: Wave 1 - Core TUI + Worker Lifecycle
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
- timestamp: '2026-02-18T14:12:27.565191687+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP05 coder - continue dialog + overlays + worker chains)
- timestamp: '2026-02-18T14:23:50.653262752+00:00'
  lane: done
  actor: manager
  shell_pid: '472734'
  action: transition done (Implemented and reviewed - continue dialog, quit confirm, worker chains)
---

# Work Package Prompt: WP05 - Continue Dialog + Overlays + Worker Chains

## Mission

Implement session continuation: the continue dialog, quit confirmation, and
worker chain visualization. After this WP, a user can complete a worker, press
`c` to continue its session with a follow-up message, and see the parent-child
relationship rendered as a tree in the worker table. This delivers User Story 3
(Continue a Worker Session).

## Scope

### Files to Modify

```
internal/tui/overlays.go    # Add continue dialog, quit confirmation dialog
internal/tui/update.go      # Add continue/quit message handlers
internal/tui/panels.go      # Worker tree rendering in table, chain info in viewport title
internal/tui/model.go       # Add continue dialog state, quit confirm state
internal/tui/keys.go        # Enable continue key, updateKeyStates refinement
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 2**: continueDialogSubmittedMsg, continueDialogCancelledMsg,
    quitConfirmedMsg, quitCancelledMsg (lines 320-334)
  - **Section 1**: SpawnConfig.ContinueSession field (lines 64-66)
  - **Section 3**: Worker.ParentID, Children() method (lines 476-517)
- `design-artifacts/tui-mockups.md`:
  - **V5**: Continue session dialog (lines 216-250) -- exact layout
  - **V7**: Worker continuation chains with tree glyphs (lines 292-320)
  - **V12**: Quit confirmation dialog (lines 474-503)
- `design-artifacts/tui-keybinds.md`:
  - `c` key: continue session, enabled when selected worker is exited/done (line 25)
  - Overlay key handling: esc dismisses, ctrl+c force quits (lines 75-86)
- `kitty-specs/016-kasmos-agent-orchestrator/data-model.md`:
  - Worker.ParentID relationship (line 37)
  - "Continue creates a NEW worker, not a state change" (line 52)

## Implementation

### Continue Dialog (overlays.go)

Build a huh form matching the V5 mockup:

**Read-only parent info section** (rendered as styled text above the form, NOT as form fields):
- Worker ID, role (with badge color), status indicator
- Session ID (in lightBlue)
- Last line of output or a summary (truncated)

**Form fields**:
- Follow-up message: `huh.NewText()` multiline textarea
  - Placeholder: "Describe what to do next..."
  - Pre-fill with empty (user writes the follow-up)

**Buttons**: "Continue" (active, purple) + "Cancel" (inactive, darkGray)

**Behavior**:
- On confirm: emit `continueDialogSubmittedMsg{ParentWorkerID, SessionID, FollowUp}`
- On cancel/esc: emit `continueDialogCancelledMsg{}`

### Quit Confirmation Dialog (overlays.go)

Build a dialog matching the V12 mockup:
- ThickBorder in orange (the ONLY view using ThickBorder)
- Header: warning emoji + "Quit kasmos?" in orange bold
- Body: "N workers are still running. They will be terminated."
- Buttons: "Force Quit" (orange bg) + "Cancel" (darkGray bg)
- Width: 36 chars
- On confirm: emit `quitConfirmedMsg{}`
- On cancel/esc: emit `quitCancelledMsg{}`

### Worker Continuation Chains (panels.go)

When rendering table rows, preprocess the worker list to add tree glyphs:

1. Build a flat display order: root workers in spawn order, with children
   indented underneath their parent
2. For each child, prepend tree glyphs to the ID column:
   - Last child of parent: `+-{id}` (or Unicode: `+-`)
   - Non-last child: `+-{id}`
   - Depth connector: `| ` for each ancestor level
3. Tree glyphs rendered in midGray/faint

Example from V7 mockup:
```
w-002       done     reviewer  3m 20s
+-w-005    done     coder     1m 45s
| +-w-006  done     reviewer  2m 10s
+--w-007   running  coder     0m 52s
```

The ID column width should expand to accommodate tree depth (max depth likely 3-4).

**Viewport title for chained workers**: Show `Output: {id} {role} <- {parentID}`
when the selected worker has a parent. First line of viewport content for
continuation workers: `<- continued from {parentID} ({parentRole})` in lightBlue.

### update.go Handlers

**continueDialogSubmittedMsg**:
1. Create new worker with `ParentID = msg.ParentWorkerID`
2. Build SpawnConfig with `ContinueSession = msg.SessionID`, `Prompt = msg.FollowUp`
3. Use same role as parent worker
4. Dispatch spawnWorkerCmd
5. Close dialog

**quitConfirmedMsg**:
1. Trigger graceful shutdown sequence
2. Return `tea.Quit` (detailed shutdown in WP06)

**quitCancelledMsg**:
1. Set `showQuitConfirm = false`
2. Return to dashboard

### keys.go / updateKeyStates

Refine `updateKeyStates()`:
- `m.keys.Continue.SetEnabled(selected != nil && (selected.State == StateExited || selected.State == StateFailed))`
- Continue requires a valid SessionID on the parent worker (if empty, disable and
  the user sees it greyed out in help bar)

### model.go Updates

Add fields:
- `showContinueDialog bool`
- `continueForm *huh.Form`
- `continueParentID string` (which worker we're continuing)
- `showQuitConfirm bool`

## What NOT to Do

- Do NOT implement kill or restart (WP07)
- Do NOT implement fullscreen viewport mode (WP06)
- Do NOT implement AI analysis or gen-prompt (WP11)
- Do NOT implement graceful shutdown protocol (WP06 handles the full shutdown sequence)
- Keep quit confirmation simple: confirm -> tea.Quit for now

## Acceptance Criteria

1. Select a completed worker, press `c` -- continue dialog appears with parent info
2. Type a follow-up, confirm -- new worker spawns with `--continue -s <sessionID>`
3. New worker appears in table as child of parent with tree glyphs
4. Viewport title shows chain reference (`<- w-002`)
5. `c` key is disabled (greyed in help) when selected worker is running or has no session ID
6. Press `q` with running workers -- quit confirmation appears with worker count
7. Press cancel in quit dialog -- returns to dashboard
8. Press "Force Quit" -- exits
9. Multi-level chains display correctly (parent -> child -> grandchild)
10. `go test ./...` passes
