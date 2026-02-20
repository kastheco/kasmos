---
work_package_id: WP04
title: Browser Interaction Logic (Update)
lane: "done"
dependencies: [WP02]
base_branch: 022-spec-kitty-feature-browser-WP02
base_commit: 33ce9398923a77135b43ec8d1a71ddadeb49caee
created_at: '2026-02-20T09:02:01.406060+00:00'
subtasks: [T018, T019, T020, T021, T022, T023]
shell_pid: "11683"
reviewed_by: "kas"
review_status: "approved"
history:
- timestamp: '2026-02-20T12:00:00Z'
  lane: planned
  actor: planner
  action: created work package
---

# WP04: Browser Interaction Logic (Update)

## Implementation Command

```bash
spec-kitty implement WP04 --base WP02
```

## Objective

Replace the stub `updateFeatureBrowser()` from WP02 with a full interaction implementation. This WP implements the Update side of the browser: the update dispatcher, list navigation with scroll management, feature selection routing (dashboard load vs sub-menu expansion), action selection with spawn dialog pre-fill, filter mode, and back navigation.

**Parallel note**: This WP can run simultaneously with WP03 (rendering). Both depend on WP02's model state fields but not on each other. The Update writes state; the View reads state.

## Context

### Existing Update Patterns

**Sub-view update dispatch** (update.go lines 20-55):
Sub-view handlers return `(tea.Model, tea.Cmd)` and intercept all key events while active. Example from `updateRestorePicker`:
```go
if m.showRestorePicker {
    return m.updateRestorePicker(msg)
}
```

**Key matching** uses `key.Matches(keyMsg, m.keys.Back)` for structured bindings and `keyMsg.String()` for single-char shortcuts. Example:
```go
case key.Matches(keyMsg, m.keys.Back):
    m.closeRestorePicker()
    return m, nil
```

### Feature Selection Routing

When the user selects a feature, the browser routes based on phase:

1. **PhaseTasksReady**: Load the dashboard directly.
   - Call `task.DetectSourceType(entry.Dir)` to create a `SpecKittySource`
   - Call `m.swapTaskSource(source)` to load the tasks
   - Call `m.transitionFromLauncher()` to close the launcher
   - Call `m.closeFeatureBrowser()` to clean up browser state

   Pattern reference: `DetectSourceType` at `internal/task/source.go` lines 60-86:
   ```go
   source, err := task.DetectSourceType(path)
   // If dir contains tasks/*.md, returns &SpecKittySource{Dir: path}
   ```

   `swapTaskSource` at model.go lines 434-466 loads and resolves dependencies.
   `transitionFromLauncher` at update.go lines 704-712 closes the launcher.

2. **PhaseSpecOnly or PhasePlanReady**: Expand the lifecycle sub-menu.
   - Set `m.featureActionsOpen = true`
   - Set `m.featureActionIdx = 0`

3. **Action selected**: Spawn a worker for the lifecycle action.
   - Get the action from `actionsForPhase(entry.Phase)[m.featureActionIdx]`
   - Build the prompt: `fmt.Sprintf(action.promptFmt, entry.Dir)`
   - Call `m.closeFeatureBrowser()`
   - Call `m.transitionFromLauncher()`
   - Call `m.openSpawnDialogWithPrefill(action.role, prompt, nil)`

   Pattern reference: `openSpawnDialogWithPrefill` at overlays.go lines 71-79:
   ```go
   func (m *Model) openSpawnDialogWithPrefill(role, prompt string, files []string) tea.Cmd {
       m.showSpawnDialog = true
       m.spawnDraft = spawnDialogDraft{Role: role, Prompt: prompt, Files: strings.Join(files, ", ")}
       m.spawnForm = newSpawnDialogModelWithPrefill(role, prompt, files)
       m.spawnForm.taskID = ""
       m.resizeSpawnPrompt()
       m.updateKeyStates()
       return m.spawnForm.focusCurrentField()
   }
   ```

---

## Subtask T018: Implement updateFeatureBrowser() Dispatcher

**Purpose**: Replace the stub with a proper dispatcher that routes key events based on browser sub-state (filter active, actions expanded, or normal list mode).

**Steps**:

1. Replace the stub `updateFeatureBrowser()` in `internal/tui/browser.go`:

   ```go
   func (m *Model) updateFeatureBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
       // Handle filter mode first (textinput captures all keys)
       if m.featureFilterActive {
           return m.updateBrowserFilter(msg)
       }

       keyMsg, ok := msg.(tea.KeyMsg)
       if !ok {
           return m, nil
       }

       // Global browser keys
       if key.Matches(keyMsg, m.keys.Back) || keyMsg.String() == "left" {
           return m.handleBrowserBack()
       }

       // Actions sub-menu mode
       if m.featureActionsOpen {
           return m.updateBrowserActions(keyMsg)
       }

       // Normal list navigation mode
       return m.updateBrowserList(keyMsg)
   }
   ```

2. This creates a clear dispatch hierarchy:
   - Filter mode intercepts ALL messages (including non-key messages for textinput)
   - Back/Esc is always available
   - Actions mode handles sub-menu navigation
   - List mode handles feature navigation, selection, and filter activation

3. Add the import for `"github.com/charmbracelet/bubbles/v2/key"` in browser.go if not already present.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Filter mode captures all messages (textinput needs non-key msgs too)
- [ ] Back/Esc handled before other routing
- [ ] Actions mode is only active when featureActionsOpen is true
- [ ] Falls through to list mode by default

---

## Subtask T019: Implement List Navigation

**Purpose**: Handle j/k/up/down in the feature list. Navigate through featureFiltered indices, clamping to bounds.

**Steps**:

1. Implement `updateBrowserList(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd)`:

   ```go
   func (m *Model) updateBrowserList(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
       switch keyMsg.String() {
       case "j", "down":
           if m.featureSelectedIdx < len(m.featureFiltered)-1 {
               m.featureSelectedIdx++
           }
           return m, nil

       case "k", "up":
           if m.featureSelectedIdx > 0 {
               m.featureSelectedIdx--
           }
           return m, nil

        case "enter", "right":
            return m.handleFeatureSelect()

        case "/":
            return m.activateBrowserFilter()

        case "f":
            if len(m.featureFiltered) == 0 {
                m.closeFeatureBrowser()
                m.transitionFromLauncher()
                _ = m.openNewDialog()
                return m, m.startNewDialogForm(newDialogTypeFeatureSpec)
            }
            return m, nil

        default:
            return m, nil
        }
    }
   ```

2. Navigation uses `featureSelectedIdx` which indexes into `featureFiltered`. The actual entry is `m.featureEntries[m.featureFiltered[m.featureSelectedIdx]]`.

3. Bounds checking: clamp to `[0, len(featureFiltered)-1]`. If featureFiltered is empty (all filtered out), selection stays at 0 and Enter does nothing.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] j/down moves selection down, clamped to list end
- [ ] k/up moves selection up, clamped to list start
- [ ] Enter/right triggers feature selection
- [ ] `f` in empty browser state routes to feature creation (US4 shortcut)
- [ ] `f` in populated browser does nothing (no conflict with feature list)
- [ ] / activates filter mode
- [ ] Navigation works with filtered list (operates on featureFiltered indices)
- [ ] Empty filtered list doesn't panic

---

## Subtask T020: Implement Feature Selection

**Purpose**: When the user presses Enter/right on a feature, route based on phase: tasks-ready features load the dashboard directly; non-ready features expand the lifecycle sub-menu.

**Steps**:

1. Implement `handleFeatureSelect() (tea.Model, tea.Cmd)`:

   ```go
   func (m *Model) handleFeatureSelect() (tea.Model, tea.Cmd) {
       if len(m.featureFiltered) == 0 || m.featureSelectedIdx >= len(m.featureFiltered) {
           return m, nil
       }

       entryIdx := m.featureFiltered[m.featureSelectedIdx]
       entry := m.featureEntries[entryIdx]

       if entry.Phase == PhaseTasksReady {
           // Direct dashboard load
           source, err := task.DetectSourceType(entry.Dir)
           if err != nil {
               m.launcherNote = fmt.Sprintf("failed to load %s: %v", entry.Dir, err)
               m.closeFeatureBrowser()
               return m, nil
           }
           m.closeFeatureBrowser()
           m.swapTaskSource(source)
           m.transitionFromLauncher()
           return m, nil
       }

       // Non-ready: expand lifecycle sub-menu
       actions := actionsForPhase(entry.Phase)
       if len(actions) == 0 {
           return m, nil // shouldn't happen, but defensive
       }
       m.featureActionsOpen = true
       m.featureActionIdx = 0
       return m, nil
   }
   ```

2. For tasks-ready features:
   - `DetectSourceType(entry.Dir)` returns `&SpecKittySource{Dir: entry.Dir}` when `tasks/*.md` files exist
   - `swapTaskSource` loads the tasks, resolves dependencies, recalculates layout
   - `transitionFromLauncher` hides the launcher and shows the dashboard

3. Import `"github.com/user/kasmos/internal/task"` in browser.go.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Tasks-ready feature: browser closes, launcher closes, dashboard loads with correct task source (FR-005)
- [ ] Dashboard loads identically to running `kasmos kitty-specs/<feature>/` directly (US2 acceptance)
- [ ] Non-ready feature: sub-menu expands with featureActionsOpen=true
- [ ] featureActionIdx reset to 0 on expansion
- [ ] DetectSourceType error sets launcherNote (graceful failure)
- [ ] Empty featureFiltered doesn't panic

---

## Subtask T021: Implement Action Selection

**Purpose**: When the user presses Enter/right on an expanded lifecycle action, spawn a worker with the appropriate agent role and prompt.

**Steps**:

1. Implement `updateBrowserActions(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd)`:

   ```go
   func (m *Model) updateBrowserActions(keyMsg tea.KeyMsg) (tea.Model, tea.Cmd) {
       if len(m.featureFiltered) == 0 || m.featureSelectedIdx >= len(m.featureFiltered) {
           return m, nil
       }

       entryIdx := m.featureFiltered[m.featureSelectedIdx]
       entry := m.featureEntries[entryIdx]
       actions := actionsForPhase(entry.Phase)

       switch keyMsg.String() {
       case "j", "down":
           if m.featureActionIdx < len(actions)-1 {
               m.featureActionIdx++
           }
           return m, nil

       case "k", "up":
           if m.featureActionIdx > 0 {
               m.featureActionIdx--
           }
           return m, nil

       case "enter", "right":
           if m.featureActionIdx >= len(actions) {
               return m, nil
           }
           action := actions[m.featureActionIdx]
           prompt := fmt.Sprintf(action.promptFmt, entry.Dir)

           m.closeFeatureBrowser()
           m.transitionFromLauncher()
           return m, m.openSpawnDialogWithPrefill(action.role, prompt, nil)

       default:
           return m, nil
       }
   }
   ```

2. The spawn dialog opens pre-filled with:
   - **Role**: `action.role` (always "planner" for lifecycle actions)
   - **Prompt**: e.g., "Run /spec-kitty.plan for feature kitty-specs/022-spec-kitty-feature-browser"
   - **Files**: nil (no file attachments needed)

3. The user sees the spawn dialog and can review/edit the prompt before confirming.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] j/k navigates between actions within the expanded sub-menu
- [ ] Enter/right on action: closes browser, closes launcher, opens spawn dialog (FR-009)
- [ ] Spawn dialog is pre-filled with correct role and prompt (US3 acceptance)
- [ ] Prompt includes the full feature directory path
- [ ] User can review/edit the prompt before spawning

---

## Subtask T022: Implement Filter Mode

**Purpose**: When the user presses `/`, activate the filter textinput. Keystrokes update the filter and recompute the filtered list in real-time. Enter confirms the filter, Esc clears it.

**Steps**:

1. Implement `activateBrowserFilter() (tea.Model, tea.Cmd)`:

   ```go
   func (m *Model) activateBrowserFilter() (tea.Model, tea.Cmd) {
       m.featureFilterActive = true
       m.featureActionsOpen = false // collapse any expanded sub-menu
       m.featureActionIdx = 0
       return m, m.featureFilter.Focus()
   }
   ```

2. Implement `updateBrowserFilter(msg tea.Msg) (tea.Model, tea.Cmd)`:

   ```go
   func (m *Model) updateBrowserFilter(msg tea.Msg) (tea.Model, tea.Cmd) {
       if keyMsg, ok := msg.(tea.KeyMsg); ok {
           switch keyMsg.String() {
           case "enter":
               // Confirm filter, return to navigation mode
               m.featureFilterActive = false
               m.featureFilter.Blur()
               return m, nil

           case "esc":
               // Clear filter, restore full list, return to navigation mode
               m.featureFilterActive = false
               m.featureFilter.SetValue("")
               m.featureFilter.Blur()
               m.featureFiltered = filterFeatures(m.featureEntries, "")
               m.featureSelectedIdx = 0
               return m, nil
           }
       }

       // Forward all other messages to the textinput
       var cmd tea.Cmd
       m.featureFilter, cmd = m.featureFilter.Update(msg)

       // Recompute filtered list on every change
       m.featureFiltered = filterFeatures(m.featureEntries, m.featureFilter.Value())
       if m.featureSelectedIdx >= len(m.featureFiltered) {
           m.featureSelectedIdx = max(0, len(m.featureFiltered)-1)
       }

       return m, cmd
   }
   ```

3. Key behaviors:
   - `/` activates filter mode (focus textinput)
   - Each keystroke recomputes featureFiltered via `filterFeatures()`
   - `Enter` confirms (keeps filter text, blurs textinput, returns to nav mode)
   - `Esc` clears (empties filter, restores full list, returns to nav mode)

4. Selection is clamped after filtering to prevent out-of-bounds access.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] `/` activates filter and focuses textinput
- [ ] Keystrokes update the filtered list in real-time
- [ ] Enter confirms filter and returns to navigation
- [ ] Esc clears filter and restores full list
- [ ] Selection clamped after filtering (no out-of-bounds)
- [ ] Filter collapses any expanded action sub-menu
- [ ] Case-insensitive matching ("SPEC" matches "spec-kitty")

---

## Subtask T023: Implement Back Navigation

**Purpose**: Handle Esc/left key for context-dependent back navigation. From the action sub-menu, collapse to the feature list. From the feature list, close the browser and return to the launcher.

**Steps**:

1. Implement `handleBrowserBack() (tea.Model, tea.Cmd)`:

   ```go
   func (m *Model) handleBrowserBack() (tea.Model, tea.Cmd) {
       if m.featureActionsOpen {
           // Collapse sub-menu, return to feature list
           m.featureActionsOpen = false
           m.featureActionIdx = 0
           return m, nil
       }

       // Close browser, return to launcher
       m.closeFeatureBrowser()
       return m, nil
   }
   ```

2. Navigation hierarchy:
   - **Actions expanded** -> Esc/left collapses to feature list (same feature stays highlighted)
   - **Feature list** -> Esc/left closes browser, returns to launcher menu

3. This matches spec US5 acceptance criteria:
   - "Escape from browser closes it, launcher reappears"
   - "Escape from sub-menu closes sub-menu, browser list reappears with same feature highlighted"

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] Esc from actions sub-menu: collapses sub-menu, stays on same feature (US5-2)
- [ ] Esc from feature list: closes browser, returns to launcher (US5-1)
- [ ] Left arrow has same behavior as Esc
- [ ] featureActionIdx reset to 0 on collapse
- [ ] No state leak (all browser state properly cleaned up on close)

---

## Definition of Done

- [ ] All 6 interaction subtasks implemented
- [ ] `go build ./internal/tui/` succeeds
- [ ] j/k navigation works in feature list and action sub-menu
- [ ] Enter on tasks-ready feature loads dashboard with correct task source
- [ ] Enter on non-ready feature expands lifecycle sub-menu
- [ ] Enter on action opens spawn dialog with correct role and prompt
- [ ] `/` filter mode works: type to filter, Enter to confirm, Esc to clear
- [ ] Esc navigation: sub-menu -> feature list -> launcher
- [ ] No panics on empty filtered list or bounds violations

## Risks

- **DetectSourceType failure**: If the feature directory exists but has no valid tasks/*.md, DetectSourceType returns an error. The browser handles this gracefully by setting launcherNote and closing.
- **Race with filesystem**: If features change on disk between scanning and selection (unlikely in practice), the selection could reference stale data. Re-scanning on selection would add complexity for minimal benefit.
- **Filter interaction with expanded sub-menu**: Activating the filter while a sub-menu is expanded could be confusing. The implementation collapses the sub-menu when filter activates.

## Reviewer Guidance

- Verify tasks-ready selection loads dashboard identically to CLI path (FR-005, US2)
- Verify spawn dialog pre-fill matches spec: role is "planner", prompt includes feature dir (FR-009, US3)
- Verify filter clears on Esc but persists on Enter
- Verify back navigation matches the two-level hierarchy (US5)
- Verify no new tea.Msg types are needed (all routing uses existing spawn infrastructure)
- Check that task import is added to browser.go

## Activity Log

- 2026-02-20T09:08:28Z – unknown – shell_pid=11683 – lane=done – All 6 subtasks (T018-T023) implemented. Build+tests+vet clean. Commit 4a86fa2. Untracked .opencode symlink is benign.
