---
work_package_id: WP03
title: Browser Rendering (View)
lane: "for_review"
dependencies: [WP02]
base_branch: 022-spec-kitty-feature-browser-WP02
base_commit: 33ce9398923a77135b43ec8d1a71ddadeb49caee
created_at: '2026-02-20T07:48:39.287846+00:00'
subtasks: [T013, T014, T015, T016, T017]
shell_pid: "4041860"
history:
- timestamp: '2026-02-20T12:00:00Z'
  lane: planned
  actor: planner
  action: created work package
---

# WP03: Browser Rendering (View)

## Implementation Command

```bash
spec-kitty implement WP03 --base WP02
```

## Objective

Replace the stub `renderFeatureBrowser()` from WP02 with a full rendering implementation. This WP implements the View side of the browser: the backdrop dialog frame, feature entry lines with phase badges and selection highlights, inline tree expansion with ASCII tree chars for lifecycle actions, filter textinput display, and the empty state. All rendering is read-only against Model state (pure View function).

**Parallel note**: This WP can run simultaneously with WP04 (interaction logic). Both depend on WP02's model state fields but not on each other. The View reads state; the Update writes state.

## Context

### Visual Design (from plan AD-004)

```
+------------------------------------------------------+
|  browse features                                     |
|                                                      |
|    022  spec-kitty-feature-browser   tasks ready (5) |
|  > 018  blocked-task-visual-feedback   spec only     |
|    |-- clarify    run /spec-kitty.clarify            |
|    '-- plan       run /spec-kitty.plan               |
|    016  kasmos-agent-orchestrator     tasks ready (8) |
|    010  hub-tui-navigator            plan ready       |
|                                                      |
|  / filter...                                         |
|                                                      |
|  j/k navigate  enter select  / filter  esc back      |
+------------------------------------------------------+
```

### Existing Rendering Patterns

**Backdrop dialog** (`styles.go` line 389-396):
```go
func (m Model) renderWithBackdrop(dialog string) string {
    return lipgloss.Place(m.width, m.height,
        lipgloss.Center, lipgloss.Center,
        dialog,
        lipgloss.WithWhitespaceChars("..."),
        lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(colorDarkGray)),
    )
}
```

**Dialog styling** (`styles.go` lines 185-198):
```go
dialogStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).
    BorderForeground(colorDialogBorder).
    Padding(1, 2)
dialogHeaderStyle = lipgloss.NewStyle().
    Foreground(colorHotPink).Bold(true)
```

**Selection highlight pattern** (from `newdialog.go` renderNewDialog plan picker, lines 378-394):
```go
for i, dir := range m.newForm.planFeatureDirs {
    selector := " "
    if i == m.newForm.planSelectedIdx {
        selector = ">"
    }
    name := filepath.Base(dir)
    row := fmt.Sprintf("%s %s", selector, name)
    if i == m.newForm.planSelectedIdx {
        style := lipgloss.NewStyle().Foreground(colorCream).Bold(true)
        row = style.Render(row)
    }
    lines = append(lines, row)
}
```

### Color Roles

- Selected feature: `colorCream` + Bold
- Unselected feature: `colorLightGray`
- Feature number: `colorMidGray`
- Phase badge: per-phase colors (T001 in WP01)
- Tree chars (`|--`, `'--`): `colorMidGray`
- Selected action: `colorPurple` + Bold
- Unselected action: `colorLightGray`
- Help text: `colorMidGray`

---

## Subtask T013: Implement renderFeatureBrowser() Main Structure

**Purpose**: Build the outer dialog frame with header, feature list area, optional filter, and help bar. This function orchestrates the rendering of all sub-components.

**Steps**:

1. Replace the stub `renderFeatureBrowser()` in `internal/tui/browser.go` with:

   ```go
   func (m *Model) renderFeatureBrowser() string {
       // 1. Check empty state
       if len(m.featureEntries) == 0 {
           return m.renderFeatureBrowserEmpty()
       }

       // 2. Build feature list lines
       lines := m.renderFeatureList()

       // 3. Build filter line (if active or has content)
       filterLine := ""
       if m.featureFilterActive || m.featureFilter.Value() != "" {
           filterLine = m.renderBrowserFilter()
       }

       // 4. Build help text
       helpText := m.browserHelpText()

       // 5. Assemble
       parts := []string{
           dialogHeaderStyle.Render("browse features"),
           "",
       }
       parts = append(parts, strings.Join(lines, "\n"))
       parts = append(parts, "")
       if filterLine != "" {
           parts = append(parts, filterLine)
       }
       parts = append(parts, helpText)

       content := strings.Join(parts, "\n")
       dialog := dialogStyle.Width(min(76, m.width-4)).Render(content)
       return m.renderWithBackdrop(dialog)
   }
   ```

2. Implement `browserHelpText()`:
   ```go
   func (m *Model) browserHelpText() string {
       if m.featureFilterActive {
           return lipgloss.NewStyle().Foreground(colorMidGray).Render(
               "type to filter  enter confirm  esc clear")
       }
       if m.featureActionsOpen {
           return lipgloss.NewStyle().Foreground(colorMidGray).Render(
               "j/k navigate  enter/right select  esc/left back")
       }
       return lipgloss.NewStyle().Foreground(colorMidGray).Render(
           "j/k navigate  enter/right select  / filter  esc back")
   }
   ```

3. Dialog width: `min(76, m.width-4)` ensures it fits narrow terminals but uses available space.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Dialog renders with backdrop pattern (consistent with restore picker, etc.)
- [ ] Header says "browse features" in dialogHeaderStyle
- [ ] Help text changes based on state (filter active, actions open, or normal)
- [ ] Dialog width adapts to terminal width

---

## Subtask T014: Implement Feature Entry Line Rendering

**Purpose**: Render each feature as a line showing its number, slug, and phase badge. The selected feature is highlighted with a `>` prefix and bold cream text.

**Steps**:

1. Implement `renderFeatureList() []string` in `browser.go`:

   ```go
   func (m *Model) renderFeatureList() []string {
       lines := make([]string, 0, len(m.featureFiltered)*2)

       for listIdx, entryIdx := range m.featureFiltered {
           entry := m.featureEntries[entryIdx]
           isSelected := listIdx == m.featureSelectedIdx

           line := m.renderFeatureEntry(entry, isSelected)
           lines = append(lines, line)

           // If this entry is selected and actions are expanded, insert action lines
           if isSelected && m.featureActionsOpen {
               actionLines := m.renderActionLines(entry)
               lines = append(lines, actionLines...)
           }
       }

       return lines
   }
   ```

2. Implement `renderFeatureEntry(entry FeatureEntry, selected bool) string`:

   ```go
   func (m *Model) renderFeatureEntry(entry FeatureEntry, selected bool) string {
       selector := "  "
       if selected && !m.featureActionsOpen {
           selector = "> "
       } else if selected && m.featureActionsOpen {
           selector = "  " // no arrow on feature when sub-menu is open
       }

       numStyle := lipgloss.NewStyle().Foreground(colorMidGray)
       slugStyle := lipgloss.NewStyle().Foreground(colorLightGray)
       if selected {
           numStyle = numStyle.Foreground(colorCream)
           slugStyle = slugStyle.Foreground(colorCream).Bold(true)
       }

       badge := phaseBadge(entry.Phase, entry.WPCount)
       return fmt.Sprintf("%s%s  %s   %s",
           selector,
           numStyle.Render(entry.Number),
           slugStyle.Render(entry.Slug),
           badge,
       )
   }
   ```

3. Feature number is right-padded to align slugs (numbers are typically 3 digits). The `%s` formatting handles this naturally since numbers are already zero-padded strings like "022".

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Selected feature has `>` prefix (when actions not expanded)
- [ ] Selected feature uses colorCream + Bold
- [ ] Unselected features use colorLightGray
- [ ] Number, slug, and phase badge all appear on each line
- [ ] Phase badge uses correct colors per phase

---

## Subtask T015: Implement Inline Tree Expansion Rendering

**Purpose**: When a non-tasks-ready feature is selected and the user expands it, show lifecycle action lines below the selected entry using ASCII tree characters. The selected action gets a `>` prefix.

**Steps**:

1. Implement `renderActionLines(entry FeatureEntry) []string`:

   ```go
   func (m *Model) renderActionLines(entry FeatureEntry) []string {
       actions := actionsForPhase(entry.Phase)
       if len(actions) == 0 {
           return nil
       }

       treeStyle := lipgloss.NewStyle().Foreground(colorMidGray)
       actionStyle := lipgloss.NewStyle().Foreground(colorLightGray)
       descStyle := lipgloss.NewStyle().Foreground(colorMidGray)
       selectedActionStyle := lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
       selectedDescStyle := lipgloss.NewStyle().Foreground(colorCream)

       lines := make([]string, 0, len(actions))
       for i, action := range actions {
           isLast := i == len(actions)-1
           isActionSelected := i == m.featureActionIdx

           // Tree character: |-- for non-last, '-- for last
           treeChar := "|--"
           if isLast {
               treeChar = "'--"
           }

           selector := " "
           aStyle := actionStyle
           dStyle := descStyle
           if isActionSelected {
               selector = ">"
               aStyle = selectedActionStyle
               dStyle = selectedDescStyle
           }

           line := fmt.Sprintf("    %s %s %-10s %s",
               treeStyle.Render(treeChar),
               selector,
               aStyle.Render(action.label),
               dStyle.Render(action.description),
           )
           lines = append(lines, line)
       }

       return lines
   }
   ```

2. Tree chars use ASCII only (`|--` and `'--`) per the AGENTS.md UTF-8 encoding rule. No Unicode box-drawing characters.

3. Actions are indented 4 spaces from the feature line to create visual nesting.

4. The `>` selector appears on the currently focused action (featureActionIdx).

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Tree chars are ASCII only (|-- and '--)
- [ ] Last action uses '-- instead of |--
- [ ] Selected action has > prefix and colorPurple styling
- [ ] Actions are visually indented below the parent feature
- [ ] Only appears when featureActionsOpen is true

---

## Subtask T016: Implement Filter Textinput Rendering

**Purpose**: Show the filter textinput at the bottom of the dialog when filter mode is active.

**Steps**:

1. Implement `renderBrowserFilter() string`:

   ```go
   func (m *Model) renderBrowserFilter() string {
       prefix := lipgloss.NewStyle().Foreground(colorPurple).Render("/")
       return prefix + " " + m.featureFilter.View()
   }
   ```

2. The filter shows the `/` prefix (styled in colorPurple) followed by the textinput's view. The textinput is a `bubbles/textinput` initialized via `styledTextInput()` which already has the correct kasmos styling (purple prompt, cream text, hot-pink cursor, gray placeholder).

3. The filter line only renders when `m.featureFilterActive || m.featureFilter.Value() != ""`. This is checked in `renderFeatureBrowser()` (T013).

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] `/` prefix renders in colorPurple
- [ ] Textinput uses kasmos styling (via styledTextInput)
- [ ] Only visible when filter is active or has content
- [ ] Filter text is readable and cursor is visible

---

## Subtask T017: Implement Empty State Rendering

**Purpose**: When no features exist in `kitty-specs/`, show a helpful message instead of an empty list.

**Steps**:

1. Implement `renderFeatureBrowserEmpty() string`:

   ```go
   func (m *Model) renderFeatureBrowserEmpty() string {
       msg := lipgloss.NewStyle().Foreground(colorMidGray).Render(
           "no spec-kitty features found")
       hint := lipgloss.NewStyle().Foreground(colorLightGray).Render(
           "press f to create a new feature, or esc to go back")
       helpText := lipgloss.NewStyle().Foreground(colorMidGray).Render(
           "esc back")

       content := lipgloss.JoinVertical(
           lipgloss.Left,
           dialogHeaderStyle.Render("browse features"),
           "",
           msg,
           hint,
           "",
           helpText,
       )

       dialog := dialogStyle.Width(min(76, m.width-4)).Render(content)
       return m.renderWithBackdrop(dialog)
   }
   ```

2. The empty state message matches spec User Story 4: "suggests pressing `f` to create one" and "Escape to return to the launcher."

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Empty state renders with backdrop dialog (same frame as populated browser)
- [ ] Message indicates no features found (FR-011)
- [ ] Hint suggests pressing f or esc (acceptance scenario US4-2)
- [ ] Consistent styling with other kasmos empty states

---

## Definition of Done

- [ ] All 5 rendering subtasks implemented
- [ ] `go build ./internal/tui/` succeeds
- [ ] Browser dialog renders with correct backdrop
- [ ] Feature entries show number, slug, and correct phase badge
- [ ] Inline tree expansion shows action lines with ASCII tree chars
- [ ] Filter textinput appears when active
- [ ] Empty state renders helpful message
- [ ] All colors use existing palette variables (no new color literals)

## Risks

- **Line count for scrolling**: If many features exist (50+), the list will exceed the dialog height. The current implementation does not include scroll offset management (that's handled in WP04 via featureSelectedIdx clamping). The rendering should work correctly even with many items - lipgloss will clip to dialog bounds.
- **Wide slugs**: Very long feature slugs could push the phase badge off-screen. Consider truncating slugs or using a fixed-width layout if this becomes a problem (deferred - not expected with typical slug lengths).

## Reviewer Guidance

- Verify all rendering uses existing kasmos palette colors (colorMidGray, colorLightGray, colorCream, colorPurple, colorHotPink, colorGreen, colorLightBlue)
- Verify ASCII-only tree chars (no Unicode box-drawing)
- Verify empty state matches spec US4 acceptance criteria
- Verify dialog uses renderWithBackdrop consistently with other launcher sub-views
- Verify help text changes based on browser state (normal, filter active, actions open)

## Activity Log

- 2026-02-20T07:50:05Z – unknown – shell_pid=4041860 – lane=for_review – Ready for review: implemented browser dialog rendering with feature list, action tree, filter line, dynamic help text, and empty state
