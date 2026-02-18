# kasmos TUI — Layout Specification

> Responsive layout system, breakpoint rules, dimension arithmetic, and panel
> composition patterns. All measurements are in terminal cells (characters).

## Layout Architecture

The TUI uses a fixed vertical structure with a responsive horizontal content area:

```
┌─────────────────────────────────────────────────┐
│ Header (2 lines: title + optional subtitle)     │  fixed
├─────────────────────────────────────────────────┤
│                                                 │
│ Content Area (responsive, fills remaining)      │  flexible
│                                                 │
├─────────────────────────────────────────────────┤
│ Status Bar (1 line)                             │  fixed
├─────────────────────────────────────────────────┤
│ Help Bar (1 line)                               │  fixed
└─────────────────────────────────────────────────┘
```

### Vertical Dimension Math

```go
const (
    headerLines    = 2  // gradient title + blank line (3 if subtitle present)
    statusBarLines = 1  // purple background bar
    helpBarLines   = 1  // keybind hints
    chromeTotal    = 4  // headerLines + statusBarLines + helpBarLines
)

// Available content height
contentHeight := m.height - chromeTotal

// When subtitle is present (spec-kitty/GSD mode):
// headerLines = 3 (title + source line + blank)
// chromeTotal = 5
```

---

## Responsive Breakpoints

Four layout modes based on terminal width. Height minimum is 24 for all modes.

### Breakpoint Summary

| Width         | Mode           | Columns | Content Layout                     |
|---------------|----------------|---------|-------------------------------------|
| < 80          | Too Small      | —       | Centered "resize" message           |
| 80–99         | Narrow         | 1       | Stacked: table above viewport       |
| 100–159       | Standard       | 2       | Split: table (40%) + viewport (60%) |
| ≥ 160         | Wide           | 3       | Three-col: tasks + table + viewport |

### Too Small (< 80 cols or < 24 rows)

```go
if m.width < 80 || m.height < 24 {
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        warnStyle.Render("Terminal too small 🫧\nMinimum: 80×24\nCurrent: "+
            fmt.Sprintf("%d×%d", m.width, m.height)),
    )
}
```

No panels rendered. No keybinds active except `ctrl+c` and `q`.

### Narrow Mode (80–99 cols)

```
┌──────────────────────────────────────────────┐
│ Header                                        │
├──────────────────────────────────────────────┤
│ ╭─ Workers ────────────────────────────────╮  │
│ │ table (full width, ~45% content height)  │  │
│ ╰──────────────────────────────────────────╯  │
│ ╭─ Output ─────────────────────────────────╮  │
│ │ viewport (full width, ~55% content ht)   │  │
│ ╰──────────────────────────────────────────╯  │
├──────────────────────────────────────────────┤
│ Status Bar                                    │
│ Help Bar (compact)                            │
└──────────────────────────────────────────────┘
```

**Dimension math:**

```go
// Narrow mode: stacked layout
panelWidth := m.width  // full width

// Table gets 45% of content, viewport gets 55%
tableOuterHeight := int(float64(contentHeight) * 0.45)
viewportOuterHeight := contentHeight - tableOuterHeight

// Inner dimensions (subtract border + padding)
// RoundedBorder: 2 vertical (top + bottom), Padding(0,1): 0 vertical
borderV := 2
paddingV := 0
tableInnerHeight := tableOuterHeight - borderV - paddingV
viewportInnerHeight := viewportOuterHeight - borderV - paddingV

borderH := 2
paddingH := 2  // Padding(0,1) = 1 each side
tableInnerWidth := panelWidth - borderH - paddingH
viewportInnerWidth := panelWidth - borderH - paddingH
```

**Table columns (narrow):**

| Column   | Width | Visible |
|----------|-------|---------|
| ID       | 8     | ✓       |
| Status   | 13    | ✓       |
| Role     | 10    | ✓       |
| Duration | 8     | ✓       |
| Task     | —     | ✗ hidden |

### Standard Mode (100–159 cols)

```
┌────────────────────────────────────────────────────────────────────────────────┐
│ Header                                                                         │
├────────────────────────────────────────────────────────────────────────────────┤
│ ╭─ Workers ─────────────────────╮ ╭─ Output ─────────────────────────────────╮ │
│ │ table (40% width)             │ │ viewport (60% width)                     │ │
│ │ full content height           │ │ full content height                      │ │
│ ╰───────────────────────────────╯ ╰─────────────────────────────────────────╯ │
├────────────────────────────────────────────────────────────────────────────────┤
│ Status Bar                                                                     │
│ Help Bar                                                                       │
└────────────────────────────────────────────────────────────────────────────────┘
```

**Dimension math:**

```go
// Standard mode: side-by-side
leftWidthRatio := 0.40
rightWidthRatio := 0.60

// Gap between panels: 1 space
gap := 1

leftOuterWidth := int(float64(m.width) * leftWidthRatio)
rightOuterWidth := m.width - leftOuterWidth - gap

panelHeight := contentHeight  // both panels same height

// Inner dimensions
leftInnerWidth := leftOuterWidth - borderH - paddingH
leftInnerHeight := panelHeight - borderV - paddingV
rightInnerWidth := rightOuterWidth - borderH - paddingH
rightInnerHeight := panelHeight - borderV - paddingV
```

**Table columns (standard):**

| Column   | Width   | Visible     |
|----------|---------|-------------|
| ID       | 8       | ✓           |
| Status   | 13      | ✓           |
| Role     | 10      | ✓           |
| Duration | 8       | ✓           |
| Task     | remaining | ✓ (truncated) |

### Wide Mode (≥ 160 cols)

Only activates when a task source is loaded (spec-kitty or GSD). Without a task source, uses Standard mode even at wide terminals.

```
┌────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│ Header + subtitle                                                                                          │
├────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ ╭─ Tasks ──────────╮ ╭─ Workers ──────────────────────╮ ╭─ Output ────────────────────────────────────────╮│
│ │ list (25%)        │ │ table (35%)                    │ │ viewport (40%)                                 ││
│ │ full content ht   │ │ full content ht                │ │ full content ht                                ││
│ ╰──────────────────╯ ╰────────────────────────────────╯ ╰────────────────────────────────────────────────╯│
├────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
│ Status Bar                                                                                                 │
│ Help Bar                                                                                                   │
└────────────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Dimension math:**

```go
// Wide mode: three columns
gap := 1  // between each panel

tasksWidthRatio := 0.25
workersWidthRatio := 0.35
outputWidthRatio := 0.40

totalGaps := gap * 2  // two gaps between three panels

availableWidth := m.width - totalGaps
tasksOuterWidth := int(float64(availableWidth) * tasksWidthRatio)
workersOuterWidth := int(float64(availableWidth) * workersWidthRatio)
outputOuterWidth := availableWidth - tasksOuterWidth - workersOuterWidth

panelHeight := contentHeight
```

**Table columns (wide):**

| Column   | Width    | Visible |
|----------|----------|---------|
| ID       | 8        | ✓       |
| Status   | 13       | ✓       |
| Role     | 10       | ✓       |
| Duration | 8        | ✓       |
| Task     | remaining | ✓       |

---

## Panel Specifications

### Header

```go
func (m Model) renderHeader() string {
    // Line 1: gradient title + version (right-aligned)
    title := gradientRender("kasmos")  // see styles doc for gradient
    version := faintStyle.Render("v0.1.0")
    titleLine := title + "  " + dimStyle.Render("agent orchestrator") +
        strings.Repeat(" ", m.width-lipgloss.Width(title)-lipgloss.Width(version)-20) +
        version

    // Line 2: task source subtitle (conditional)
    var subtitle string
    if m.taskSource != nil {
        subtitle = "\n" + faintStyle.Render(fmt.Sprintf("  %s: %s",
            m.taskSource.Type(), m.taskSource.Path()))
    }

    return titleLine + subtitle + "\n"
}
```

**Height:** 2 lines (no task source) or 3 lines (with task source).
**Width:** Full terminal width.

### Worker Table Panel

```
╭─ Workers ──────────────────────────────────╮
│                                            │  ← Padding(0, 1)
│   ID      Status       Role     Duration   │
│  ────────────────────────────────────────  │  ← header border (purple, bottom only)
│  w-001   ✓ done       coder    4m 12s      │
│ >w-002   ⣾ running    reviewer 1m 48s      │  ← selected row (purple bg)
│  w-003   ⟳ running    planner  0m 34s      │
│                                            │
╰────────────────────────────────────────────╯
```

**Border:** RoundedBorder. Purple when focused, darkGray when unfocused.
**Title:** `" Workers "` or `" Workers (N) "` in border top — rendered via panel title pattern (not built-in bubbles/table title).
**Padding:** `Padding(0, 1)` — 1 cell horizontal, 0 vertical.
**Table height:** Set via `table.WithHeight(innerHeight)`.
**Table width:** Set via `t.SetWidth(innerWidth)` or column sum.

**Column width allocation:**

```go
func (m Model) workerTableColumns() []table.Column {
    available := innerWidth
    fixed := 0

    cols := []table.Column{
        {Title: "ID", Width: 10},       // "w-001" or "├─w-005"
        {Title: "Status", Width: 14},    // "⣾ running" or "✗ failed(1)"
        {Title: "Role", Width: 10},      // role badge
        {Title: "Duration", Width: 9},   // "4m 12s" or "  —  "
    }
    for _, c := range cols {
        fixed += c.Width
    }

    // Task column gets remaining space (hidden if < threshold)
    remaining := available - fixed - len(cols) // account for cell padding
    if remaining >= 15 && m.width >= 100 {
        cols = append(cols, table.Column{Title: "Task", Width: remaining})
    }

    return cols
}
```

### Output Viewport Panel

```
╭─ Output: w-002 reviewer ──────────────────╮
│                                            │
│ [14:32:01] Reviewing changes in auth/...   │
│ [14:32:03] Found 3 files modified:         │
│   • internal/auth/middleware.go             │
│   • internal/auth/handler.go               │
│                                            │
╰────────────────────────────────────────────╯
```

**Border:** RoundedBorder. Purple when focused, darkGray when unfocused.
**Title:** Dynamic — `" Output: {id} {role} "` or `" Output "` when no worker selected, or `" Analysis: {id} {role} "` in analysis mode.
**Padding:** `Padding(0, 1)`.
**Content:** Set via `viewport.SetContent(string)`.
**Auto-follow:** Track `viewport.AtBottom()` before setting content. If true, call `GotoBottom()` after.

### Task List Panel (Wide mode only)

```
╭─ Tasks (6) ───────────────╮
│                           │
│  WP-001  Auth middleware  │  ← list item: Title()
│  JWT RS256 validation     │  ← list item: Description()
│  deps: none               │  ← custom delegate extra line
│  ✓ done                   │  ← status badge
│                           │
│ >WP-002  Login endpoint   │  ← selected (purple highlight)
│  POST /api/v1/login       │
│  deps: WP-001             │
│  ○ unassigned             │
│                           │
╰───────────────────────────╯
```

**Component:** `bubbles/list` with custom `list.ItemDelegate`.
**Border:** RoundedBorder. Purple when focused, darkGray when unfocused.
**Title:** `" Tasks (N) "` showing count.
**Item height:** 4 lines per item (title, description, deps, status) + 1 blank separator.
**Filtering:** Built-in `/` search using `FilterValue()` which returns task title.

### Status Bar

```
 ⣾ 2 running  ✓ 1 done  ✗ 1 failed  ☠ 1 killed  ○ 1 pending              mode: ad-hoc  scroll: 100%
```

**Style:** Purple background, cream foreground, full terminal width.
**Content:** Left-aligned worker counts, right-aligned mode + scroll percentage.
**Padding:** `Padding(0, 1)`.

```go
func (m Model) renderStatusBar() string {
    // Left: worker state counts
    counts := m.workerCounts()
    left := fmt.Sprintf(" ⣾ %d running  ✓ %d done  ✗ %d failed  ☠ %d killed  ○ %d pending",
        counts.running, counts.done, counts.failed, counts.killed, counts.pending)

    // Right: mode + scroll
    scrollStr := "—"
    if m.focused == panelViewport && m.viewport.TotalLineCount() > 0 {
        scrollStr = fmt.Sprintf("%.0f%%", m.viewport.ScrollPercent()*100)
    }
    right := fmt.Sprintf("mode: %s  scroll: %s ", m.modeName(), scrollStr)

    // Fill gap
    gap := strings.Repeat(" ", max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)))

    return statusBarStyle.Width(m.width).Render(left + gap + right)
}
```

### Help Bar

**Component:** `bubbles/help` in short mode (`ShowAll = false`).
**Style:** Keys in purple bold, descriptions in midGray, `·` separators in darkGray.

**Context-dependent content:**
- Dashboard focused on table: `s spawn · x kill · c continue · r restart · g gen prompt · a analyze · f fullscreen · tab panel · ? help · q quit`
- Dashboard focused on viewport: `f fullscreen · j/k scroll · / search · tab panel · ? help · q quit`
- Full-screen output: `esc back · c continue · r restart · j/k scroll · G bottom · g top · / search`
- Empty dashboard: `s spawn · ? help · q quit`
- Analysis active: `r restart with suggestion · c continue · esc dismiss · ? help · q quit`

```go
// Disable keys based on state
m.keys.Kill.SetEnabled(m.hasRunningSelected())
m.keys.Continue.SetEnabled(m.hasCompletedSelected())
m.keys.Restart.SetEnabled(m.hasFailedOrKilledSelected())
m.keys.Analyze.SetEnabled(m.hasFailedSelected())
m.keys.GenPrompt.SetEnabled(m.hasTaskSelected())
```

---

## Overlay Layout

All overlays (spawn dialog, continue dialog, help, quit confirmation) use the same centering pattern:

```go
func (m Model) renderOverlay(content string, borderColor lipgloss.TerminalColor, borderType lipgloss.Border) string {
    dialog := lipgloss.NewStyle().
        Border(borderType).
        BorderForeground(borderColor).
        Padding(1, 2).
        Render(content)

    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        dialog,
        lipgloss.WithWhitespaceChars("░"),
        lipgloss.WithWhitespaceForeground(darkGray),
    )
}
```

**Overlay widths:**

| Overlay            | Width | Border          | Border Color |
|--------------------|-------|-----------------|--------------|
| Spawn dialog       | 70    | RoundedBorder   | hotPink      |
| Continue dialog    | 65    | RoundedBorder   | hotPink      |
| Help overlay       | 78    | RoundedBorder   | hotPink      |
| Quit confirmation  | 36    | ThickBorder     | orange       |

---

## Focus System

### Panel Enumeration

```go
type panel int

const (
    panelTable panel = iota     // Worker table
    panelViewport               // Output viewport
    panelTasks                  // Task list (only in wide mode with task source)
)
```

### Focus Cycling

```go
func (m Model) cyclablePanels() []panel {
    panels := []panel{panelTable, panelViewport}
    if m.hasTaskSource() && m.width >= 160 {
        panels = []panel{panelTasks, panelTable, panelViewport}
    }
    return panels
}

// Tab: next panel
// Shift+Tab: previous panel
```

### Visual Focus Indicator

Focused panel gets purple RoundedBorder. All unfocused panels get darkGray RoundedBorder. The border color is the ONLY focus indicator — no other visual change.

### Focus Routing

Only the focused panel receives `tea.KeyMsg` for navigation (j/k/enter/etc). Global keys (?, q, ctrl+c, tab) are handled before panel routing.

When an overlay is visible (spawn dialog, help, quit confirm), ALL key input goes to the overlay. No panel receives input.

---

## Full-Screen Toggle

Press `f` to expand the output viewport to full screen. Press `esc` to return.

```go
// Full-screen dimensions
vpWidth := m.width - borderH - paddingH  // or m.width - 4 for some breathing room
vpHeight := contentHeight - borderV - paddingV
```

Full-screen viewport gets purple RoundedBorder (always focused). The table and task panels are not rendered.

---

## Resize Handling

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height

    if !m.ready {
        m.ready = true
    }

    // Recalculate all panel dimensions
    m.recalculateLayout()
    return m, nil
```

`recalculateLayout()` recomputes all panel dimensions based on the current breakpoint and sets them on sub-models:

```go
func (m *Model) recalculateLayout() {
    contentHeight := m.height - m.chromeHeight()

    switch {
    case m.width >= 160 && m.hasTaskSource():
        m.layoutMode = layoutWide
        // ... set 3-col dimensions
    case m.width >= 100:
        m.layoutMode = layoutStandard
        // ... set 2-col dimensions
    case m.width >= 80:
        m.layoutMode = layoutNarrow
        // ... set stacked dimensions
    default:
        m.layoutMode = layoutTooSmall
    }

    // Apply to sub-models
    m.table.SetWidth(m.tableInnerWidth)
    m.table.SetHeight(m.tableInnerHeight)
    m.table.SetColumns(m.workerTableColumns())
    m.viewport.Width = m.viewportInnerWidth
    m.viewport.Height = m.viewportInnerHeight
    if m.hasTaskSource() {
        m.taskList.SetSize(m.tasksInnerWidth, m.tasksInnerHeight)
    }
}
```

### Minimum Viable Dimensions

| Component      | Min Width | Min Height |
|----------------|-----------|------------|
| Worker table   | 45        | 6          |
| Output viewport| 30        | 5          |
| Task list      | 25        | 8          |
| Spawn dialog   | 70        | 24         |
| Help overlay   | 78        | 22         |
| Quit dialog    | 36        | 11         |
