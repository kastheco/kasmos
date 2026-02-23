# Tab Focus Ring Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the 3-panel focus model with a 5-slot Tab focus ring so arrow keys are captured per-pane and Tab cycles between sidebar, agent, diff, git, and instance list.

**Architecture:** Single `focusSlot int` (0-4) replaces `focusedPanel int` (0-2). Tab/Shift+Tab cycle the ring. Each slot routes `up/down/h/l` to its pane. Center tab slots (1-3) auto-switch the visible tab. Insert mode (`stateFocusAgent`) remains a separate state for full PTY forwarding.

**Tech Stack:** Go, bubbletea, lipgloss, existing Rosé Pine Moon palette

**Design doc:** `docs/plans/2026-02-22-tab-focus-ring-design.md`

---

### Task 1: Update Key Definitions

**Files:**
- Modify: `keys/keys.go`

**Step 1: Remove old bindings and add new ones**

In the `KeyName` constants block, make these changes:
- Remove: `KeyLeft`, `KeyRight`, `KeyShiftUp`, `KeyShiftDown`, `KeyGitTab`
- Add: `KeyArrowLeft`, `KeyArrowRight` (in-pane horizontal navigation, not panel switching)

In `GlobalKeyStringsMap`, make these changes:
- Remove: `"f1": KeyTabAgent`, `"f2": KeyTabDiff`, `"f3": KeyTabGit`
- Remove: `"shift+up": KeyShiftUp`, `"shift+down": KeyShiftDown`
- Remove: `"g": KeyGitTab`
- Add: `"!": KeyTabAgent`, `"@": KeyTabDiff`, `"#": KeyTabGit`
- Change `"left"` and `"h"` from `KeyLeft` to `KeyArrowLeft`
- Change `"right"` and `"l"` from `KeyRight` to `KeyArrowRight`

In `GlobalkeyBindings`, make these changes:
- Remove: `KeyLeft`, `KeyRight`, `KeyShiftUp`, `KeyShiftDown`, `KeyGitTab` bindings
- Add `KeyArrowLeft` binding:
  ```go
  KeyArrowLeft: key.NewBinding(
      key.WithKeys("left", "h"),
      key.WithHelp("←/h", "left"),
  ),
  ```
- Add `KeyArrowRight` binding:
  ```go
  KeyArrowRight: key.NewBinding(
      key.WithKeys("right", "l"),
      key.WithHelp("→/l", "right"),
  ),
  ```
- Update `KeyTabAgent` binding: keys `"!"`, help `"!/2/3"` → `"switch tab"`
- Update `KeyTabDiff` binding: keys `"@"`, help `"@"` → `"diff tab"`
- Update `KeyTabGit` binding: keys `"#"`, help `"#"` → `"git tab"`
- Update `KeyTab` binding help text: `"tab"` → `"cycle panes"`

**Step 2: Verify compilation**

Run: `go build ./keys/...`
Expected: Compile errors in files that reference removed keys — that's expected, we fix them in subsequent tasks.

**Step 3: Commit**

```bash
git add keys/keys.go
git commit -m "refactor: update key definitions for tab focus ring"
```

---

### Task 2: Replace focusedPanel with focusSlot

**Files:**
- Modify: `app/app.go` (field declaration + initialization)
- Modify: `app/app_state.go` (rewrite `setFocus` → `setFocusSlot`)
- Modify: `ui/tabbed_window.go` (add `focusedTab` field)

**Step 1: Add slot constants and rewrite setFocus**

In `app/app_state.go`, replace the `setFocus` function (the comment + 4-line function at lines 77-84) with:

```go
// focusSlot constants for readability.
const (
	slotSidebar = 0
	slotAgent   = 1
	slotDiff    = 2
	slotGit     = 3
	slotList    = 4
	slotCount   = 5
)

// setFocusSlot updates which pane has focus and syncs visual state.
func (m *home) setFocusSlot(slot int) {
	m.focusSlot = slot
	m.sidebar.SetFocused(slot == slotSidebar)
	m.list.SetFocused(slot == slotList)

	// Center pane is focused when any of the 3 center tabs is active.
	centerFocused := slot >= slotAgent && slot <= slotGit
	m.tabbedWindow.SetFocused(centerFocused)

	// When focusing a center tab, switch the visible tab to match.
	if centerFocused {
		m.tabbedWindow.SetActiveTab(slot - slotAgent) // slotAgent=1 → PreviewTab=0, etc.
		m.tabbedWindow.SetFocusedTab(slot - slotAgent)
	} else {
		m.tabbedWindow.SetFocusedTab(-1)
	}
}

// nextFocusSlot advances the focus ring forward, skipping sidebar when hidden.
func (m *home) nextFocusSlot() {
	next := (m.focusSlot + 1) % slotCount
	if next == slotSidebar && m.sidebarHidden {
		next = slotAgent
	}
	m.setFocusSlot(next)
}

// prevFocusSlot moves the focus ring backward, skipping sidebar when hidden.
func (m *home) prevFocusSlot() {
	prev := (m.focusSlot - 1 + slotCount) % slotCount
	if prev == slotSidebar && m.sidebarHidden {
		prev = slotList
	}
	m.setFocusSlot(prev)
}
```

**Step 2: Update the home struct**

In `app/app.go`, replace:
```go
// focusedPanel tracks which panel has keyboard focus: 0=sidebar (left), 1=preview/center, 2=instance list (right)
focusedPanel int
```
with:
```go
// focusSlot tracks which pane has keyboard focus in the Tab ring:
// 0=sidebar, 1=agent tab, 2=diff tab, 3=git tab, 4=instance list
focusSlot int
```

**Step 3: Update initialization**

In `app/app.go`, in `newHome()`, replace:
```go
h.setFocus(0) // Start with sidebar focused
```
with:
```go
h.setFocusSlot(slotSidebar) // Start with sidebar focused
```

**Step 4: Add focusedTab field to TabbedWindow**

In `ui/tabbed_window.go`, add a field to the `TabbedWindow` struct:
```go
focusedTab int // which tab (0=agent, 1=diff, 2=git) has Tab-ring focus; -1 = none
```

Initialize `focusedTab: -1` in `NewTabbedWindow`.

Add method:
```go
// SetFocusedTab sets which specific tab has focus ring focus. -1 = none.
func (w *TabbedWindow) SetFocusedTab(tab int) {
	w.focusedTab = tab
}
```

**Step 5: Commit**

```bash
git add app/app.go app/app_state.go ui/tabbed_window.go
git commit -m "refactor: replace focusedPanel with 5-slot focusSlot ring"
```

---

### Task 3: Rewrite Key Routing in app_input.go

**Files:**
- Modify: `app/app_input.go`
- Modify: `app/app_state.go` (rewrite `switchToTab`, `fkeyToTab`)
- Modify: `app/app_actions.go` (update `openContextMenu`)

This is the largest task. Replace all `focusedPanel` references and rewrite the key handling for the new model.

**Step 1: Mechanical rename in app_input.go**

Use `sd` to do a mechanical rename first:
```bash
sd 'focusedPanel' 'focusSlot' app/app_input.go
sd 'setFocus\(' 'setFocusSlot(' app/app_input.go
```

Then manually fix the logic — the old `== 0` checks become `== slotSidebar`, `== 1` becomes center-tab checks, `== 2` becomes `== slotList`.

**Step 2: Rewrite KeyUp/KeyDown handler**

Replace the current up/down handler with slot-aware routing:

```go
case keys.KeyUp:
	m.tabbedWindow.ClearDocumentMode()
	switch m.focusSlot {
	case slotSidebar:
		m.sidebar.Up()
		m.filterInstancesByTopic()
	case slotAgent, slotDiff:
		m.tabbedWindow.ScrollUp()
	case slotGit:
		gitPane := m.tabbedWindow.GetGitPane()
		if gitPane != nil && gitPane.IsRunning() {
			gitPane.SendKey([]byte("\x1b[A"))
		}
	case slotList:
		m.list.Up()
	}
	return m, m.instanceChanged()
case keys.KeyDown:
	m.tabbedWindow.ClearDocumentMode()
	switch m.focusSlot {
	case slotSidebar:
		m.sidebar.Down()
		m.filterInstancesByTopic()
	case slotAgent, slotDiff:
		m.tabbedWindow.ScrollDown()
	case slotGit:
		gitPane := m.tabbedWindow.GetGitPane()
		if gitPane != nil && gitPane.IsRunning() {
			gitPane.SendKey([]byte("\x1b[B"))
		}
	case slotList:
		m.list.Down()
	}
	return m, m.instanceChanged()
```

**Step 3: Rewrite KeyTab handler**

Replace the current tab handler (which cycles center tabs via `Toggle()`) with focus ring cycling:

```go
case keys.KeyTab:
	wasGitSlot := m.focusSlot == slotGit
	m.nextFocusSlot()
	return m, m.handleGitTabTransition(wasGitSlot)
```

**Step 4: Add Shift+Tab handling**

In the raw key handler section (around where `tea.KeyTab` is handled in the `switch msg.Type` block near the end of `handleKeyPress`), there is no `tea.KeyShiftTab` case currently. However, Shift+Tab arrives as a `tea.KeyMsg` with string `"shift+tab"`. Since `"shift+tab"` is NOT in `GlobalKeyStringsMap`, it won't be caught by the `name, ok` lookup. We need to handle it before the global key lookup.

Add this block right after the `if msg.Type == tea.KeyEsc` block and before the viewport forwarding block:

```go
// Handle Shift+Tab for reverse focus ring cycling.
// This key is not in GlobalKeyStringsMap, so we intercept it here.
if msg.Type == tea.KeyShiftTab {
	wasGitSlot := m.focusSlot == slotGit
	m.prevFocusSlot()
	return m, m.handleGitTabTransition(wasGitSlot)
}
```

**Step 5: Add git tab transition helper**

In `app/app_state.go`, add:

```go
// handleGitTabTransition manages lazygit spawn/kill when focus moves to/from the git slot.
func (m *home) handleGitTabTransition(wasGitSlot bool) tea.Cmd {
	if m.focusSlot == slotGit {
		cmd := m.spawnGitTab()
		return tea.Batch(m.instanceChanged(), cmd)
	}
	if wasGitSlot {
		m.killGitTab()
	}
	return m.instanceChanged()
}
```

**Step 6: Remove KeyShiftUp/KeyShiftDown handler**

Delete the `case keys.KeyShiftUp:` and `case keys.KeyShiftDown:` blocks (the 6 lines that call `m.tabbedWindow.ScrollUp()`/`ScrollDown()`).

**Step 7: Remove KeyGitTab handler**

Delete the entire `case keys.KeyGitTab:` block (the 8 lines that jump directly to git tab).

**Step 8: Rewrite KeyArrowLeft/KeyArrowRight**

Replace the entire `case keys.KeyLeft:` and `case keys.KeyRight:` blocks with in-pane horizontal navigation:

```go
case keys.KeyArrowLeft:
	switch m.focusSlot {
	case slotSidebar:
		if m.sidebar.IsTreeMode() {
			m.sidebar.Left()
			m.filterInstancesByTopic()
		}
	case slotGit:
		gitPane := m.tabbedWindow.GetGitPane()
		if gitPane != nil && gitPane.IsRunning() {
			gitPane.SendKey([]byte("\x1b[D"))
		}
	}
	return m, nil
case keys.KeyArrowRight:
	switch m.focusSlot {
	case slotSidebar:
		if m.sidebar.IsTreeMode() {
			m.sidebar.Right()
			m.filterInstancesByTopic()
		}
	case slotGit:
		gitPane := m.tabbedWindow.GetGitPane()
		if gitPane != nil && gitPane.IsRunning() {
			gitPane.SendKey([]byte("\x1b[C"))
		}
	}
	return m, nil
```

**Step 9: Rewrite switchToTab in app_state.go**

Replace the existing `switchToTab` function with:

```go
func (m *home) switchToTab(name keys.KeyName) (tea.Model, tea.Cmd) {
	var targetSlot int
	switch name {
	case keys.KeyTabAgent:
		targetSlot = slotAgent
	case keys.KeyTabDiff:
		targetSlot = slotDiff
	case keys.KeyTabGit:
		targetSlot = slotGit
	default:
		return m, nil
	}

	if m.focusSlot == targetSlot {
		return m, nil
	}

	wasGitSlot := m.focusSlot == slotGit
	m.setFocusSlot(targetSlot)
	return m, m.handleGitTabTransition(wasGitSlot)
}
```

**Step 10: Rewrite fkeyToTab for focus mode**

In `app/app_state.go`, replace `fkeyToTab` with a version that maps `!/@/#` instead of F1/F2/F3:

```go
// shiftNumToSlot maps !/@/# key strings to focus slots.
func shiftNumToSlot(key string) (int, bool) {
	switch key {
	case "!":
		return slotAgent, true
	case "@":
		return slotDiff, true
	case "#":
		return slotGit, true
	default:
		return 0, false
	}
}
```

Then in `app/app_input.go`, in the `stateFocusAgent` handler, replace the `fkeyToTab` call:

```go
// !/@/#: exit focus mode and jump to specific tab slot
if targetSlot, ok := shiftNumToSlot(msg.String()); ok {
	wasGitSlot := m.tabbedWindow.IsInGitTab()
	m.exitFocusMode()
	m.setFocusSlot(targetSlot)
	m.menu.SetInDiffTab(m.focusSlot == slotDiff)
	if wasGitSlot && m.focusSlot != slotGit {
		m.killGitTab()
	}
	if m.focusSlot == slotGit && !wasGitSlot {
		cmd := m.spawnGitTab()
		return m, tea.Batch(tea.WindowSize(), m.instanceChanged(), cmd)
	}
	return m, tea.Batch(tea.WindowSize(), m.instanceChanged())
}
```

**Step 11: Update KeyFocusSidebar handler**

Replace `m.setFocus(0)` calls with `m.setFocusSlot(slotSidebar)`.

**Step 12: Update KeyToggleSidebar handler**

Replace `m.focusedPanel == 0` with `m.focusSlot == slotSidebar` and `m.setFocus(1)` with `m.setFocusSlot(slotAgent)`.

**Step 13: Update KeySpace handler**

Replace `m.focusedPanel == 0` checks with `m.focusSlot == slotSidebar`.

**Step 14: Update KeyEnter handler**

Replace `m.focusedPanel == 0` check with `m.focusSlot == slotSidebar`.

**Step 15: Update KeySearch handler**

Replace `m.setFocus(0)` with `m.setFocusSlot(slotSidebar)`.

**Step 16: Update mouse click handler**

In `handleMouse`, replace `m.setFocus(0/1/2)` calls:
- Sidebar click: `m.setFocusSlot(slotSidebar)`
- Center click: `m.setFocusSlot(slotAgent + m.tabbedWindow.GetActiveTab())` — focus whichever center tab is visible
- List click: `m.setFocusSlot(slotList)`

**Step 17: Update app_actions.go**

In `app/app_actions.go`, in `openContextMenu`, replace `m.focusedPanel == 0` with `m.focusSlot == slotSidebar`.

**Step 18: Update handleMenuHighlighting**

In `app/app_input.go`, in `handleMenuHighlighting`, remove the early-return check for `KeyShiftDown`/`KeyShiftUp` (the 3 lines starting with `if name == keys.KeyShiftDown`). These keys no longer exist.

**Step 19: Update viewport forwarding block**

In the viewport forwarding block (the `if (m.tabbedWindow.IsDocumentMode() || ...)` section), remove the `msg.Type != tea.KeyShiftUp && msg.Type != tea.KeyShiftDown` guard since those keys no longer exist. Simplify to just check `ViewportHandlesKey`.

**Step 20: Run tests and fix compilation**

Run: `go build ./...`
Fix any remaining compilation errors.

Run: `go test ./app/... -v`
Expect test failures — the tests reference `focusedPanel`. Fix in Task 4.

**Step 21: Commit**

```bash
git add app/app_input.go app/app_state.go app/app_actions.go
git commit -m "feat: implement tab focus ring key routing"
```

---

### Task 4: Update Tests

**Files:**
- Modify: `app/app_test.go`

**Step 1: Rewrite focus navigation tests**

The existing tests (the `TestFocusNavigation` function, lines ~491-577) test `focusedPanel` with left/right arrow navigation. Rewrite them for the new model:

- Replace all `homeModel.focusedPanel` assertions with `homeModel.focusSlot`
- Replace all `h.setFocus(N)` calls with `h.setFocusSlot(slotXxx)`
- Remove tests for left/right arrow panel switching (those keys no longer switch panels)
- Remove the "left from panel 1 shows and focuses sidebar when hidden" test
- Remove the "left from sidebar hides sidebar and focuses panel 1" test
- Remove the "h moves focus to sidebar when panel 1 is focused" test

Add new tests:

```go
t.Run("Tab cycles forward through all slots", func(t *testing.T) {
    h := newTestHome()
    h.setFocusSlot(slotSidebar)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyTab})
    assert.Equal(t, slotAgent, homeModel.focusSlot)
})

t.Run("Tab wraps from list to sidebar", func(t *testing.T) {
    h := newTestHome()
    h.setFocusSlot(slotList)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyTab})
    assert.Equal(t, slotSidebar, homeModel.focusSlot)
})

t.Run("Tab skips sidebar when hidden", func(t *testing.T) {
    h := newTestHome()
    h.sidebarHidden = true
    h.setFocusSlot(slotList)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyTab})
    assert.Equal(t, slotAgent, homeModel.focusSlot)
})

t.Run("Shift+Tab cycles backward", func(t *testing.T) {
    h := newTestHome()
    h.setFocusSlot(slotAgent)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyShiftTab})
    assert.Equal(t, slotSidebar, homeModel.focusSlot)
})

t.Run("Shift+Tab skips sidebar when hidden", func(t *testing.T) {
    h := newTestHome()
    h.sidebarHidden = true
    h.setFocusSlot(slotAgent)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyShiftTab})
    assert.Equal(t, slotList, homeModel.focusSlot)
})

t.Run("! jumps to agent slot", func(t *testing.T) {
    h := newTestHome()
    h.setFocusSlot(slotList)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("!")})
    assert.Equal(t, slotAgent, homeModel.focusSlot)
})

t.Run("@ jumps to diff slot", func(t *testing.T) {
    h := newTestHome()
    h.setFocusSlot(slotSidebar)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("@")})
    assert.Equal(t, slotDiff, homeModel.focusSlot)
})

t.Run("# jumps to git slot", func(t *testing.T) {
    h := newTestHome()
    h.setFocusSlot(slotSidebar)

    homeModel := handle(t, h, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("#")})
    assert.Equal(t, slotGit, homeModel.focusSlot)
})
```

Keep the `ctrl+s` toggle tests but update assertions:
- `homeModel.focusedPanel` → `homeModel.focusSlot`
- `h.setFocus(N)` → `h.setFocusSlot(slotXxx)`
- `assert.Equal(t, 1, ...)` → `assert.Equal(t, slotAgent, ...)` (when sidebar hides, focus moves to agent slot)
- `assert.Equal(t, 2, ...)` → `assert.Equal(t, slotList, ...)` (when sidebar was hidden and focus was on list)

Keep `s` key tests but update assertions similarly.

**Step 2: Run tests**

Run: `go test ./app/... -v`
Expected: All tests pass.

Run: `go test ./... -v`
Expected: All tests pass.

**Step 3: Commit**

```bash
git add app/app_test.go
git commit -m "test: rewrite focus navigation tests for tab ring model"
```

---

### Task 5: Update Gradient and Tab Rendering

**Files:**
- Modify: `ui/theme.go`
- Modify: `ui/tabbed_window.go`

**Step 1: Update gradient constants**

In `ui/theme.go`, change:
```go
GradientStart = "#ea9a97" // rose
GradientEnd   = "#c4a7e7" // iris
```
to:
```go
GradientStart = "#9ccfd8" // foam
GradientEnd   = "#c4a7e7" // iris
```

**Step 2: Update tab label rendering for focus state**

In `ui/tabbed_window.go`, in the `String()` method's tab rendering loop, replace:
```go
if isActive && !w.focusMode {
	renderedTabs = append(renderedTabs, style.Render(GradientText(t, GradientStart, GradientEnd)))
} else {
	renderedTabs = append(renderedTabs, style.Render(t))
}
```

With:
```go
switch {
case isActive && i == w.focusedTab && !w.focusMode:
	// Focused tab in the ring: foam→iris gradient
	renderedTabs = append(renderedTabs, style.Render(GradientText(t, GradientStart, GradientEnd)))
case isActive:
	// Active but not ring-focused: normal text color
	renderedTabs = append(renderedTabs, style.Render(lipgloss.NewStyle().Foreground(ColorText).Render(t)))
default:
	// Inactive tab: muted
	renderedTabs = append(renderedTabs, style.Render(lipgloss.NewStyle().Foreground(ColorMuted).Render(t)))
}
```

**Step 3: Run and verify**

Run: `go build ./...`
Expected: Compiles cleanly.

Run: `go test ./ui/... -v`
Expected: All tests pass.

**Step 4: Commit**

```bash
git add ui/theme.go ui/tabbed_window.go
git commit -m "feat: foam→iris gradient on focused tab, muted inactive tabs"
```

---

### Task 6: Update Help Screen

**Files:**
- Modify: `app/help.go`

**Step 1: Update help text**

In `app/help.go`, in `helpTypeGeneral.toContent()`, replace the "Other" section:

Replace:
```go
headerStyle.Render("\uf085 Other:"),
keyStyle.Render("tab")+descStyle.Render("       - Switch between preview, diff and git tabs"),
keyStyle.Render("g")+descStyle.Render("         - Open git tab (lazygit, ctrl+space to exit)"),
keyStyle.Render("shift-↓/↑")+descStyle.Render(" - Scroll in diff view"),
keyStyle.Render("q")+descStyle.Render("         - Quit the application"),
keyStyle.Render("R")+descStyle.Render("         - Switch repository"),
```

With:
```go
headerStyle.Render("\uf085 Navigation:"),
keyStyle.Render("tab/S-tab")+descStyle.Render(" - Cycle panes forward/backward"),
keyStyle.Render("!/2/3")+descStyle.Render("     - Jump to agent/diff/git tab"),
keyStyle.Render("↑/↓")+descStyle.Render("       - Navigate within focused pane"),
keyStyle.Render("q")+descStyle.Render("         - Quit the application"),
keyStyle.Render("R")+descStyle.Render("         - Switch repository"),
```

Also replace the sidebar navigation line at the bottom:
```go
keyStyle.Render("←/h, →/l")+descStyle.Render("  - Switch sidebar and instance list"),
```
With:
```go
keyStyle.Render("←/h, →/l")+descStyle.Render("  - In-pane navigation (tree expand/collapse)"),
```

**Step 2: Run and verify**

Run: `go build ./...`
Expected: Compiles cleanly.

**Step 3: Commit**

```bash
git add app/help.go
git commit -m "docs: update help screen for tab focus ring keybindings"
```

---

### Task 7: Final Verification

**Files:**
- No new file changes expected (cleanup only if needed)

**Step 1: Full test suite**

Run: `go test ./... -v`
Expected: All tests pass.

Run: `go build ./...`
Expected: Clean build.

**Step 2: Verify startup default**

Check that `newHome` initializes `focusSlot` to `slotSidebar` (0). The `h.setFocusSlot(slotSidebar)` call in `newHome` handles this.

**Step 3: Verify no stale references**

Run: `rg 'focusedPanel|KeyLeft[^A]|KeyRight[^A]|KeyShiftUp|KeyShiftDown|KeyGitTab|fkeyToTab' --type go`
Expected: Zero matches (all old references removed).

Run: `rg 'setFocus\(' --type go`
Expected: Zero matches (all calls migrated to `setFocusSlot`).

**Step 4: Commit (if any cleanup was needed)**

```bash
git add -A
git commit -m "chore: final cleanup for tab focus ring"
```
