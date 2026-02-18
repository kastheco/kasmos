---
work_package_id: "WP03"
title: "TUI Foundation (Layout, Styles, Keys)"
lane: "planned"
dependencies:
  - "WP01"
subtasks:
  - "internal/tui/model.go - Main Model struct, Init(), View()"
  - "internal/tui/layout.go - recalculateLayout(), breakpoints, dimension math"
  - "internal/tui/styles.go - Full color palette, all style definitions"
  - "internal/tui/keys.go - keyMap, defaultKeyMap(), ShortHelp(), FullHelp()"
  - "internal/tui/messages.go - All tea.Msg type definitions"
  - "internal/tui/panels.go - Empty panel rendering (table, viewport, status bar, help)"
phase: "Wave 1 - Core TUI + Worker Lifecycle"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
history:
  - timestamp: "2026-02-17T00:00:00Z"
    lane: "planned"
    agent: "planner"
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP03 - TUI Foundation (Layout, Styles, Keys)

## Mission

Build the complete TUI skeleton: Model struct, responsive layout system with
breakpoints, the full lipgloss style palette, keybind definitions, message types,
and empty-state panel rendering. After this WP, `kasmos` launches a beautiful
empty dashboard matching the V11 mockup (empty dashboard) with working resize,
focus cycling, and help overlay.

## Scope

### Files to Create

```
internal/tui/model.go       # Main Model struct, Init(), top-level Update(), View()
internal/tui/layout.go      # recalculateLayout(), breakpoint detection, dimension math
internal/tui/styles.go      # Full Charm bubblegum palette, all component styles
internal/tui/keys.go        # keyMap struct, defaultKeyMap(), ShortHelp(), FullHelp()
internal/tui/messages.go    # All tea.Msg type definitions (stubs for command msgs)
internal/tui/panels.go      # Panel rendering functions (empty states for now)
```

### Files to Modify

```
cmd/kasmos/main.go          # Replace placeholder model with real tui.Model
```

### Technical References

- `kitty-specs/016-kasmos-agent-orchestrator/research/tui-technical.md`:
  - **Section 2**: Full message type catalog (lines 218-401) -- define all types now
  - **Section 3**: panel enum (panelTable, panelViewport, panelTasks)
  - **Section 10**: Package structure (lines 1219-1263)
- `design-artifacts/tui-layout-spec.md`: Entire document
  - Vertical dimension math (lines 26-40)
  - Breakpoint summary table (lines 50-56)
  - Narrow/Standard/Wide dimension formulas (lines 58-206)
  - Panel specs: header, table, viewport, status bar, help bar (lines 220-388)
  - Focus system: panel enum, cycling, visual indicator (lines 422-458)
  - Overlay layout with centering (lines 392-419)
  - Resize handling (lines 478-524)
- `design-artifacts/tui-styles.md`: Entire document
  - Core color palette (lines 12-31)
  - Panel styles (lines 90-111)
  - Header + gradient (lines 115-161)
  - Table styles (lines 166-186)
  - Status bar, help bar styles (lines 190-228)
  - Status indicators: WorkerState, TaskState, role badges (lines 316-402)
  - Panel title rendering (lines 462-511)
  - Overlay backdrop (lines 519-530)
- `design-artifacts/tui-keybinds.md`:
  - keys.go implementation (lines 92-239)
  - ShortHelp/FullHelp (lines 247-269)
- `design-artifacts/tui-mockups.md`:
  - **V11**: Empty dashboard mockup (lines 437-470)
  - **V6**: Help overlay mockup (lines 254-290)

## Implementation

### model.go

The Model is the central state container. For this WP, include:
- `width`, `height` int (terminal dimensions)
- `ready` bool (set on first WindowSizeMsg)
- `focused` panel (current focus: panelTable or panelViewport)
- `layoutMode` (narrow/standard/wide/tooSmall)
- `showHelp` bool
- `keys` keyMap
- `help` help.Model
- `table` table.Model (empty, configured with styles)
- `viewport` viewport.Model (welcome message content)
- `spinner` spinner.Model
- `statusBar` string (rendered each frame)
- Layout dimension fields: tableInnerWidth, tableInnerHeight, viewportInnerWidth, etc.

`Init()`: return `tea.Batch(tickCmd(), m.spinner.Tick)`

`Update()`: handle tea.WindowSizeMsg (call recalculateLayout), tea.KeyMsg for
global keys (q, ctrl+c, ?, tab, shift+tab), tickMsg, spinner.TickMsg.
For now, delegate to focused panel for j/k navigation.

`View()`: compose header + content area + status bar + help bar.
If `layoutMode == layoutTooSmall`, render centered resize warning.
If `showHelp`, render help overlay with backdrop.

### layout.go

Implement `recalculateLayout()` exactly as specified in tui-layout-spec.md:
- layoutMode enum: `layoutTooSmall`, `layoutNarrow`, `layoutStandard`, `layoutWide`
- Breakpoints: <80 too small, 80-99 narrow, 100-159 standard, >=160 wide
- Vertical: `contentHeight = height - chromeTotal` (chromeTotal = 4 or 5)
- Narrow: stacked, 45%/55% vertical split
- Standard: side-by-side, 40%/60% horizontal split with 1-cell gap
- Wide: three-col, 25%/35%/40% (only with task source)
- Apply dimensions to sub-models (table.SetWidth, viewport.Width, etc.)

### styles.go

Copy the complete style definitions from tui-styles.md:
- Color palette (colorPurple through colorLightGray)
- Adaptive colors (subtleColor, highlightColor, specialColor)
- Semantic state colors (colorRunning, colorDone, colorFailed, etc.)
- Panel styles (focusedPanelStyle, unfocusedPanelStyle, panelStyle helper)
- Header styles (gradient, dim subtitle, version)
- `renderGradientTitle()` using gamut.Blends
- `workerTableStyles()` (header, selected, cell)
- statusBarStyle
- `styledHelp()` helper
- Dialog styles (dialogStyle, alertDialogStyle, buttons)
- `styledSpinner()`, `styledTextInput()`, `styledTextArea()`
- Status indicator functions: `statusIndicator()`, `taskStatusBadge()`, `roleBadge()`
- `renderWithBackdrop()` using lipgloss.Place with the backdrop pattern

### keys.go

Copy the keyMap struct and defaultKeyMap() from tui-keybinds.md (lines 92-239).
Implement ShortHelp() and FullHelp() from tui-keybinds.md (lines 247-269).
Include `updateKeyStates()` stub that disables all worker-dependent keys (no workers yet).

### messages.go

Define ALL message types from tui-technical.md Section 2:
- Worker lifecycle: workerSpawnedMsg, workerOutputMsg, workerExitedMsg, workerKilledMsg
- UI state: tickMsg, focusChangedMsg, layoutChangedMsg
- Overlay/dialog: spawnDialogSubmittedMsg, spawnDialogCancelledMsg,
  continueDialogSubmittedMsg, continueDialogCancelledMsg, quitConfirmedMsg, quitCancelledMsg
- AI helpers: analyzeStartedMsg, analyzeCompletedMsg, genPromptStartedMsg, genPromptCompletedMsg
- Task source: tasksLoadedMsg, taskStateChangedMsg
- Session persistence: sessionSavedMsg, sessionLoadedMsg
- tickCmd() function

Define the types now; handlers will be added in WP04+.

### panels.go

Render functions for each panel:
- `renderHeader()`: gradient title + version + optional source subtitle
- `renderWorkerTable()`: table inside titled panel border (empty state: centered
  "No workers yet / Press s to spawn your first worker")
- `renderViewport()`: viewport inside titled panel border (welcome message from V11)
- `renderStatusBar()`: purple bar with worker counts + mode + scroll percentage
- `renderHelpBar()`: bubbles/help short mode with styled keys

### cmd/kasmos/main.go Update

Replace the placeholder model with `tui.NewModel()` (or `tui.Model` constructor).
Pass it to `tea.NewProgram` with `tea.WithAltScreen()`.

## What NOT to Do

- Do NOT implement worker spawning, output reading, or commands.go (WP04)
- Do NOT implement overlays (spawn/continue/quit dialogs) (WP05)
- Do NOT implement task panel or list component (WP09)
- Do NOT implement daemon mode (WP12)
- The table should be configured but EMPTY (no worker data yet)
- The viewport shows a static welcome message, not worker output

## Acceptance Criteria

1. `kasmos` launches and displays the empty dashboard matching V11 mockup
2. Terminal resize switches between narrow/standard layouts correctly
3. Tab/Shift-Tab cycles focus between table and viewport (border color changes)
4. `?` toggles help overlay with backdrop matching V6 mockup
5. `q` exits cleanly, `ctrl+c` force-quits
6. Status bar shows "0 workers" and "mode: ad-hoc"
7. Header shows gradient "kasmos" title with version
8. Terminals <80 cols show "Terminal too small" warning
9. `go build ./cmd/kasmos` compiles, `go vet ./...` clean
