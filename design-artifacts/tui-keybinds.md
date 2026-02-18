# kasmos TUI — Keybinding Specification

> Complete keybinding map, context-dependent activation rules, and implementable
> Go code for `keys.go`. Integrates with `bubbles/help` for short/full help display.

## Keybind Map

### Global Keys (always active, any context)

| Key      | Action           | Notes                                    |
|----------|------------------|------------------------------------------|
| `ctrl+c` | Force quit       | Immediate exit, no confirmation          |
| `?`      | Toggle help      | Full help overlay on/off                 |
| `tab`    | Next panel       | Cycles focus through visible panels      |
| `S-tab`  | Previous panel   | Reverse cycle                            |

### Dashboard — Table Focused

| Key   | Action          | Enabled When                          |
|-------|-----------------|---------------------------------------|
| `j/↓` | Move down       | Always                                |
| `k/↑` | Move up         | Always                                |
| `s`   | Spawn worker    | Always                                |
| `x`   | Kill worker     | Selected worker is running            |
| `c`   | Continue session | Selected worker is exited/done        |
| `r`   | Restart worker  | Selected worker is failed/killed      |
| `g`   | Generate prompt | Task source loaded, task selected     |
| `a`   | Analyze failure | Selected worker is failed             |
| `f`   | Fullscreen output | Any worker selected                  |
| `b`   | Batch spawn     | Task source loaded, tasks available   |
| `enter`| View output     | Any worker selected (→ fullscreen)    |
| `q`   | Quit            | Always (confirms if workers running)  |

### Dashboard — Viewport Focused

| Key   | Action           | Notes                                  |
|-------|------------------|----------------------------------------|
| `j/↓` | Scroll down      | One line                               |
| `k/↑` | Scroll up        | One line                               |
| `d`   | Half page down   | Standard vim motion                    |
| `u`   | Half page up     | Standard vim motion                    |
| `G`   | Jump to bottom   | Re-enables auto-follow                 |
| `g`   | Jump to top      |                                        |
| `/`   | Search output    | Opens search input overlay on viewport |
| `f`   | Fullscreen       | Expand viewport to full terminal       |
| `q`   | Quit             |                                        |

### Dashboard — Task List Focused (wide mode)

| Key    | Action           | Notes                                 |
|--------|------------------|---------------------------------------|
| `j/↓`  | Move down        | Next task in list                     |
| `k/↑`  | Move up          | Previous task                         |
| `/`    | Filter tasks     | Activates bubbles/list built-in filter|
| `enter`| Assign + spawn   | Opens spawn dialog pre-filled         |
| `s`    | Spawn from task  | Same as enter                         |
| `b`    | Batch spawn      | Multi-select then spawn all           |
| `q`    | Quit             |                                       |

### Full-Screen Output View

| Key    | Action            | Notes                                |
|--------|-------------------|--------------------------------------|
| `esc`  | Back to dashboard | Returns to split view                |
| `j/↓`  | Scroll down       |                                      |
| `k/↑`  | Scroll up         |                                      |
| `d`    | Half page down    |                                      |
| `u`    | Half page up      |                                      |
| `G`    | Jump to bottom    |                                      |
| `g`    | Jump to top       |                                      |
| `/`    | Search            |                                      |
| `c`    | Continue session  | If worker is exited/done             |
| `r`    | Restart           | If worker is failed/killed           |

### Overlay Active (spawn, continue, help, quit)

| Key    | Action            | Notes                                |
|--------|-------------------|--------------------------------------|
| `esc`  | Dismiss overlay   | Returns to previous view             |
| `ctrl+c` | Force quit     | Still works through overlays         |

Form-specific keys (spawn/continue dialogs) are handled by huh:
- `tab` / `S-tab` → next/prev form field
- `enter` → confirm/select
- Arrow keys → navigate within select fields
- Text input → standard editing keys

---

## Implementation: keys.go

```go
package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
    // Navigation
    Up       key.Binding
    Down     key.Binding
    NextPanel key.Binding
    PrevPanel key.Binding

    // Worker actions
    Spawn    key.Binding
    Kill     key.Binding
    Continue key.Binding
    Restart  key.Binding
    Batch    key.Binding

    // Output
    Fullscreen  key.Binding
    ScrollDown  key.Binding
    ScrollUp    key.Binding
    HalfDown    key.Binding
    HalfUp      key.Binding
    GotoBottom  key.Binding
    GotoTop     key.Binding
    Search      key.Binding

    // AI helpers
    GenPrompt key.Binding
    Analyze   key.Binding

    // Task list
    Filter    key.Binding
    Select    key.Binding

    // General
    Help     key.Binding
    Quit     key.Binding
    ForceQuit key.Binding
    Back     key.Binding
}

func defaultKeyMap() keyMap {
    return keyMap{
        Up: key.NewBinding(
            key.WithKeys("k", "up"),
            key.WithHelp("↑/k", "up"),
        ),
        Down: key.NewBinding(
            key.WithKeys("j", "down"),
            key.WithHelp("↓/j", "down"),
        ),
        NextPanel: key.NewBinding(
            key.WithKeys("tab"),
            key.WithHelp("tab", "next panel"),
        ),
        PrevPanel: key.NewBinding(
            key.WithKeys("shift+tab"),
            key.WithHelp("S-tab", "prev panel"),
        ),
        Spawn: key.NewBinding(
            key.WithKeys("s"),
            key.WithHelp("s", "spawn worker"),
        ),
        Kill: key.NewBinding(
            key.WithKeys("x"),
            key.WithHelp("x", "kill worker"),
        ),
        Continue: key.NewBinding(
            key.WithKeys("c"),
            key.WithHelp("c", "continue session"),
        ),
        Restart: key.NewBinding(
            key.WithKeys("r"),
            key.WithHelp("r", "restart worker"),
        ),
        Batch: key.NewBinding(
            key.WithKeys("b"),
            key.WithHelp("b", "batch spawn"),
        ),
        Fullscreen: key.NewBinding(
            key.WithKeys("f"),
            key.WithHelp("f", "fullscreen"),
        ),
        ScrollDown: key.NewBinding(
            key.WithKeys("j", "down"),
            key.WithHelp("↓/j", "scroll down"),
        ),
        ScrollUp: key.NewBinding(
            key.WithKeys("k", "up"),
            key.WithHelp("↑/k", "scroll up"),
        ),
        HalfDown: key.NewBinding(
            key.WithKeys("d"),
            key.WithHelp("d", "half page down"),
        ),
        HalfUp: key.NewBinding(
            key.WithKeys("u"),
            key.WithHelp("u", "half page up"),
        ),
        GotoBottom: key.NewBinding(
            key.WithKeys("G"),
            key.WithHelp("G", "bottom"),
        ),
        GotoTop: key.NewBinding(
            key.WithKeys("g"),
            key.WithHelp("g", "top"),
        ),
        Search: key.NewBinding(
            key.WithKeys("/"),
            key.WithHelp("/", "search"),
        ),
        GenPrompt: key.NewBinding(
            key.WithKeys("g"),
            key.WithHelp("g", "gen prompt (AI)"),
        ),
        Analyze: key.NewBinding(
            key.WithKeys("a"),
            key.WithHelp("a", "analyze failure (AI)"),
        ),
        Filter: key.NewBinding(
            key.WithKeys("/"),
            key.WithHelp("/", "filter"),
        ),
        Select: key.NewBinding(
            key.WithKeys("enter"),
            key.WithHelp("enter", "select"),
        ),
        Help: key.NewBinding(
            key.WithKeys("?"),
            key.WithHelp("?", "help"),
        ),
        Quit: key.NewBinding(
            key.WithKeys("q"),
            key.WithHelp("q", "quit"),
        ),
        ForceQuit: key.NewBinding(
            key.WithKeys("ctrl+c"),
            key.WithHelp("ctrl+c", "force quit"),
        ),
        Back: key.NewBinding(
            key.WithKeys("esc"),
            key.WithHelp("esc", "back"),
        ),
    }
}
```

### help.KeyMap Implementation

The help bubble reads from these methods to render short and full help views.

```go
// ShortHelp — shown in the help bar at the bottom of the dashboard
// Returns context-dependent bindings based on current focus and state
func (k keyMap) ShortHelp() []key.Binding {
    return []key.Binding{
        k.Spawn, k.Kill, k.Continue, k.Restart,
        k.GenPrompt, k.Analyze, k.Fullscreen,
        k.NextPanel, k.Help, k.Quit,
    }
}

// FullHelp — shown in the ? overlay, grouped into columns
func (k keyMap) FullHelp() [][]key.Binding {
    return [][]key.Binding{
        // Column 1: Navigation
        {k.Up, k.Down, k.NextPanel, k.PrevPanel, k.Select, k.Back},
        // Column 2: Worker actions
        {k.Spawn, k.Kill, k.Continue, k.Restart, k.Batch, k.GenPrompt, k.Analyze},
        // Column 3: Output
        {k.Fullscreen, k.ScrollDown, k.ScrollUp, k.GotoBottom, k.GotoTop, k.Search},
        // Column 4: General + Tasks
        {k.Help, k.Quit, k.ForceQuit, k.Filter},
    }
}
```

---

## Context-Dependent Key Activation

Keys are enabled/disabled based on current state. Disabled keys don't appear in help and don't match in `key.Matches()`.

```go
func (m *Model) updateKeyStates() {
    hasWorkers := len(m.workers) > 0
    selected := m.selectedWorker() // may be nil

    // Worker action keys — depend on selected worker's state
    m.keys.Kill.SetEnabled(selected != nil && selected.State == StateRunning)
    m.keys.Continue.SetEnabled(selected != nil &&
        (selected.State == StateExited || selected.State == StateFailed))
    m.keys.Restart.SetEnabled(selected != nil &&
        (selected.State == StateFailed || selected.State == StateKilled))
    m.keys.Analyze.SetEnabled(selected != nil && selected.State == StateFailed)
    m.keys.Fullscreen.SetEnabled(hasWorkers && selected != nil)

    // AI helpers — depend on task source
    m.keys.GenPrompt.SetEnabled(m.hasTaskSource())
    m.keys.Batch.SetEnabled(m.hasTaskSource() && m.hasUnassignedTasks())

    // Task keys — only in wide mode with task source
    hasTaskPanel := m.hasTaskSource() && m.width >= 160
    m.keys.Filter.SetEnabled(hasTaskPanel && m.focused == panelTasks)
}
```

Call `updateKeyStates()` at the end of every `Update()` cycle, after state changes.

---

## Key Routing in Update

```go
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ── Phase 0: Overlay intercepts ALL input ──
    if m.showOverlay() {
        return m.updateOverlay(msg)
    }

    switch msg := msg.(type) {
    case tea.KeyMsg:
        // ── Phase 1: Global keys (always handled) ──
        switch {
        case key.Matches(msg, m.keys.ForceQuit):
            return m, tea.Quit
        case key.Matches(msg, m.keys.Help):
            m.showHelp = !m.showHelp
            return m, nil
        case key.Matches(msg, m.keys.NextPanel):
            m.cyclePanel(1)
            return m, nil
        case key.Matches(msg, m.keys.PrevPanel):
            m.cyclePanel(-1)
            return m, nil
        case key.Matches(msg, m.keys.Quit):
            if m.hasRunningWorkers() {
                m.showQuitConfirm = true
                return m, nil
            }
            return m, m.gracefulShutdown()
        }

        // ── Phase 2: Full-screen mode keys ──
        if m.fullScreen {
            return m.updateFullScreen(msg)
        }

        // ── Phase 3: Panel-specific keys ──
        switch m.focused {
        case panelTable:
            return m.updateTableKeys(msg)
        case panelViewport:
            return m.updateViewportKeys(msg)
        case panelTasks:
            return m.updateTaskKeys(msg)
        }

    // ── Non-key messages: always process ──
    case tea.WindowSizeMsg:
        // ... resize handling
    case workerExitedMsg:
        // ... worker state update
    case spinner.TickMsg:
        // ... spinner animation
    case tickMsg:
        // ... periodic refresh
    }

    // Delegate to focused sub-model for unhandled messages
    return m.updateFocused(msg)
}
```

---

## Key Conflict Resolution

Some keys have different meanings depending on focused panel:

| Key | Table Focus      | Viewport Focus | Task Focus     |
|-----|------------------|----------------|----------------|
| `g` | Gen prompt (AI)  | Jump to top    | Gen prompt     |
| `/` | (unused)         | Search output  | Filter tasks   |
| `j` | Table row down   | Scroll down    | List item down |
| `k` | Table row up     | Scroll up      | List item up   |

The `g` key conflict (gen prompt vs goto top) is resolved by panel context: `g` means "gen prompt" when table is focused, "goto top" when viewport is focused. In the full help overlay, both are listed in their respective columns.

For the `g` dual binding, use separate `key.Binding` objects and enable only the appropriate one:

```go
// In updateKeyStates():
if m.focused == panelViewport || m.fullScreen {
    m.keys.GotoTop.SetEnabled(true)
    m.keys.GenPrompt.SetEnabled(false)
} else {
    m.keys.GotoTop.SetEnabled(false)
    m.keys.GenPrompt.SetEnabled(m.hasTaskSource())
}
```

---

## Message Types for Key Actions

Each key action produces a Msg or triggers a Cmd:

```go
// messages.go — key-triggered messages

type spawnRequestedMsg struct{}                   // s key → open spawn dialog
type killRequestedMsg struct{ workerID string }   // x key → kill worker
type continueRequestedMsg struct{ workerID string } // c key → open continue dialog
type restartRequestedMsg struct{ workerID string }  // r key → open restart dialog
type batchSpawnRequestedMsg struct{}              // b key → open batch dialog
type analyzeRequestedMsg struct{ workerID string }  // a key → start AI analysis
type genPromptRequestedMsg struct{ taskID string }  // g key → start AI prompt gen
type fullscreenToggledMsg struct{}                // f key → toggle fullscreen
type quitRequestedMsg struct{}                    // q key → quit (or confirm)
```

These messages flow through Update and trigger state changes (opening dialogs, spawning commands, etc.) rather than performing side effects directly in the key handler.
