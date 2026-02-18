---
work_package_id: WP06
title: Output Viewport + Fullscreen + Update Dispatch + Shutdown
lane: doing
dependencies:
- WP04
- WP05
subtasks:
- Full-screen viewport mode (f key toggle)
- Auto-follow logic (track AtBottom, GotoBottom on new content)
- Viewport scroll controls (d/u half-page, G bottom, g top)
- internal/tui/update.go - Complete 4-phase key routing
- Context-dependent key activation (updateKeyStates)
- Graceful shutdown protocol (SIGTERM -> SIGKILL -> persist)
- Signal handling refinement in main.go
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
- timestamp: '2026-02-18T14:24:28.504267429+00:00'
  lane: doing
  actor: manager
  shell_pid: '472734'
  action: transition active (Launching WP06 coder - output viewport + fullscreen + shutdown)
---

# Work Package Prompt: WP06 - Output Viewport + Fullscreen + Update Dispatch + Shutdown

## Mission

Polish the output viewport into a first-class feature (fullscreen mode, auto-follow,
vim-style scroll controls), finalize the Update dispatch with correct 4-phase key
routing, implement context-dependent key activation, and add the graceful shutdown
protocol. This WP completes Wave 1 -- after it, all P1 user stories (1, 2, 3) are
fully delivered.

## Scope

### Files to Modify

```
internal/tui/update.go      # Complete 4-phase key routing
internal/tui/model.go       # Fullscreen state, auto-follow tracking
internal/tui/panels.go      # Fullscreen viewport rendering, status bar in fullscreen
internal/tui/keys.go        # updateKeyStates() full implementation, key conflict resolution
cmd/kasmos/main.go          # Signal handling refinement
```

### Technical References

- `design-artifacts/tui-mockups.md`:
  - **V3**: Full-screen output viewport (lines 119-163) -- exact layout + status bar
- `design-artifacts/tui-layout-spec.md`:
  - Full-screen dimensions (lines 466-475)
  - Viewport auto-follow note (line 311)
- `design-artifacts/tui-keybinds.md`:
  - Full-screen output keys: esc, j/k, d/u, G, g, /, c, r (lines 62-73)
  - Key routing in Update: 4-phase dispatch (lines 309-366)
  - Context-dependent key activation (lines 276-299)
  - Key conflict resolution: g = gen-prompt vs goto-top (lines 370-395)
- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 8**: Graceful shutdown protocol (lines 1089-1135)
  - **Section 2**: tickMsg, focusChangedMsg (lines 276-303)

## Implementation

### Full-Screen Viewport Mode

Add to Model:
- `fullScreen bool` -- toggled by `f` key
- `autoFollow bool` -- true when viewport is tracking the bottom

**Enter fullscreen** (`f` key or `enter` on selected worker):
1. Set `fullScreen = true`
2. Recalculate viewport dimensions: full terminal width minus borders/padding,
   full content height minus borders/padding
3. Viewport always gets purple border in fullscreen (always focused)

**Exit fullscreen** (`esc` key):
1. Set `fullScreen = false`
2. Recalculate layout back to split mode

**Fullscreen rendering** (V3 mockup):
- Header: same gradient title
- Viewport: fills entire content area, purple RoundedBorder
- Viewport title: `Output: {id} {role} - {prompt truncated}` (include prompt)
- Status bar (fullscreen variant): `{id} {role}  {status}  exit({code})  duration: {dur}  session: {sessID}  parent: {parentID or "-"}  scroll: N%`
- Help bar: fullscreen-specific bindings (esc back, c continue, r restart, j/k, G, g, /)

### Auto-Follow Logic

When the viewport is at the bottom and new output arrives, automatically scroll to
show the new content. When the user scrolls up, disable auto-follow.

```go
// Before setting new content:
wasAtBottom := m.viewport.AtBottom()

// Set content:
m.viewport.SetContent(worker.Output.Content())

// After:
if wasAtBottom || m.autoFollow {
    m.viewport.GotoBottom()
    m.autoFollow = true
}
```

When user presses `k` (scroll up) or `u` (half page up): set `autoFollow = false`.
When user presses `G` (goto bottom): set `autoFollow = true`.

### Viewport Scroll Controls

In updateViewportKeys and updateFullScreen, handle:
- `j`/down: `m.viewport.LineDown(1)` (auto-follow = false if not at bottom)
- `k`/up: `m.viewport.LineUp(1)`, auto-follow = false
- `d`: `m.viewport.HalfViewDown()`, check bottom
- `u`: `m.viewport.HalfViewUp()`, auto-follow = false
- `G`: `m.viewport.GotoBottom()`, auto-follow = true
- `g`: `m.viewport.GotoTop()`, auto-follow = false

### 4-Phase Key Routing (update.go)

Implement the complete dispatch from tui-keybinds.md lines 309-366:

```
Phase 0: Overlay intercept -- if any overlay visible, ALL input goes to overlay
Phase 1: Global keys -- ctrl+c (force quit), ? (help), tab/S-tab (focus cycle), q (quit)
Phase 2: Fullscreen keys -- if fullScreen, handle esc/scroll/continue/restart
Phase 3: Panel-specific -- switch on m.focused: panelTable / panelViewport / panelTasks
```

Create dedicated handler functions:
- `updateOverlay(msg)` -- routes to spawn form, continue form, or quit dialog
- `updateFullScreen(msg)` -- fullscreen viewport keys
- `updateTableKeys(msg)` -- table navigation + worker action keys
- `updateViewportKeys(msg)` -- viewport scroll keys

### Key Conflict Resolution

Handle the `g` key dual binding (tui-keybinds.md lines 370-395):
- When viewport focused or fullscreen: `g` = goto top
- When table focused: `g` = gen-prompt (enabled only when task source loaded, Wave 2)
- Use separate key.Binding objects (GotoTop vs GenPrompt) and toggle enabled state

In `updateKeyStates()`:
```go
if m.focused == panelViewport || m.fullScreen {
    m.keys.GotoTop.SetEnabled(true)
    m.keys.GenPrompt.SetEnabled(false)
} else {
    m.keys.GotoTop.SetEnabled(false)
    m.keys.GenPrompt.SetEnabled(m.hasTaskSource())
}
```

### Context-Dependent Key Activation

Implement full `updateKeyStates()` from tui-keybinds.md lines 279-299:
- Kill: selected worker is running
- Continue: selected worker is exited or failed AND has SessionID
- Restart: selected worker is failed or killed
- Analyze: selected worker is failed
- Fullscreen: any worker selected
- GenPrompt: task source loaded (disabled for now, Wave 2)
- Batch: task source loaded with unassigned tasks (disabled for now, Wave 2)
- Filter: task panel visible and focused (disabled for now, Wave 2)

Call `updateKeyStates()` at the end of every Update() cycle.

### Graceful Shutdown Protocol

Implement from tui-technical.md Section 8:

```go
func (m Model) gracefulShutdown() tea.Cmd {
    return func() tea.Msg {
        // 1. Send SIGTERM to all running workers
        for _, w := range m.runningWorkers() {
            w.Handle.Kill(3 * time.Second)
        }
        // 2. Wait for all workers to exit (up to 5s total)
        // 3. Return tea.Quit
    }
}
```

Wire this into:
- `quitConfirmedMsg` handler (from WP05)
- `q` key when no workers running (direct quit)
- Signal handling (SIGINT/SIGTERM from outside)

### Signal Handling (main.go)

Ensure `tea.WithContext(ctx)` is used where ctx comes from `signal.NotifyContext`.
When signal arrives, the context cancels, and bubbletea exits. The model's
shutdown logic should trigger on context cancellation.

## What NOT to Do

- Do NOT implement search/filter in viewport (future enhancement)
- Do NOT implement output line styling/colorization (future enhancement)
- Do NOT implement task panel keys (WP09)
- Do NOT implement AI helper keys (WP11)

## Acceptance Criteria

1. Press `f` on a selected worker -- viewport expands to full screen matching V3 mockup
2. Press `esc` in fullscreen -- returns to split dashboard
3. Auto-follow: viewport tracks new output at bottom; scrolling up disables auto-follow
4. `G` re-enables auto-follow, `g` goes to top
5. `d`/`u` half-page scroll works in both split and fullscreen
6. Status bar shows scroll percentage in viewport (not "100%" when scrolled up)
7. Quit with running workers shows confirmation; quit without workers exits immediately
8. Kill kasmos process (SIGTERM) -- workers receive SIGTERM, kasmos exits cleanly
9. All key routing phases work: overlays block input, global keys work everywhere
10. Disabled keys don't appear in help bar and don't trigger actions
11. `g` means "goto top" in viewport, "gen prompt" in table (gen prompt disabled until Wave 2)
12. `go test ./...` passes, `go vet ./...` clean
