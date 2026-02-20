---
work_package_id: WP02
title: Browser Model State and Launcher Wiring
lane: "done"
dependencies: [WP01]
base_branch: 022-spec-kitty-feature-browser-WP01
base_commit: 94ddda5a17e154a0c6485a61073ecbf486001d9e
created_at: '2026-02-20T07:08:50.396239+00:00'
subtasks: [T007, T008, T009, T010, T011, T012]
shell_pid: "3972069"
reviewed_by: "kas"
review_status: "approved"
history:
- timestamp: '2026-02-20T12:00:00Z'
  lane: planned
  actor: planner
  action: created work package
---

# WP02: Browser Model State and Launcher Wiring

## Implementation Command

```bash
spec-kitty implement WP02 --base WP01
```

## Objective

Wire the feature browser into the kasmos launcher by adding state fields to the Model struct, implementing open/close helpers, adding the `b` key handler, adding the menu item, and connecting the browser to the View and Update dispatch chains. After this WP, pressing `b` in the launcher will open the browser (though rendering and interaction logic come in WP03 and WP04).

## Context

### Current Launcher Sub-View Pattern

The launcher uses boolean flags to switch between sub-views. Each sub-view has:
1. A `show*` boolean on Model
2. An `open*()` method that sets the flag and initializes state
3. A `close*()` method that resets state
4. A check in View() to dispatch rendering
5. A check in Update() to dispatch key handling
6. An entry in `updateKeyStates()` overlayActive check

**Existing sub-views** (model.go lines 317-331):
```go
if m.showRestorePicker { return m.renderRestorePicker() }
if m.showHistory { return m.renderHistoryOverlay() }
if m.showSettings { return m.renderSettingsView() }
if m.showQuitConfirm { return m.renderQuitConfirm() }
return m.renderLauncher(m.width, m.height)
```

**Existing Update dispatch** (update.go lines 20-55):
```go
if m.showSettings { return m.updateSettings(msg) }
if m.showHistory { return m.updateHistory(msg) }
if m.showRestorePicker { return m.updateRestorePicker(msg) }
```

### Key Handler Pattern

The `b` key in `updateLauncherKeys()` follows the same pattern as `r` (restore) and `s` (settings):

```go
// Existing pattern (update.go lines 773-778):
case "r":
    m.launcherNote = ""
    return m, m.openRestorePicker()
case "s":
    m.launcherNote = ""
    return m, m.openSettingsView()
```

---

## Subtask T007: Add Browser State Fields to Model Struct

**Purpose**: Add the state fields that track the browser's current display state. These fields are read by the renderer (WP03) and written by the interaction handler (WP04).

**Steps**:

1. Open `internal/tui/model.go` and add the following fields to the `Model` struct. Place them after the `showRestorePicker`/`restoreEntries` block (after line 97):

   ```go
   // Feature browser state (launcher sub-view)
   showFeatureBrowser bool
   featureEntries     []FeatureEntry
   featureFiltered    []int // indices into featureEntries matching filter
   featureSelectedIdx int   // index into featureFiltered
   featureActionsOpen bool  // true when lifecycle sub-menu is expanded
   featureActionIdx   int   // selected action within expanded sub-menu
   featureFilterActive bool // true when filter textinput has focus
   featureFilter      textinput.Model
   ```

2. Add the `textinput` import if not already present. Check existing imports in model.go - `textinput` is not currently imported there (it's imported in overlays.go and newdialog.go). Add:
   ```go
   "github.com/charmbracelet/bubbles/v2/textinput"
   ```

**Files**: `internal/tui/model.go` (lines ~92-97, add after restoreErr/launcherNote block)

**Validation**:
- [ ] All 8 fields from data-model.md are present
- [ ] textinput import added
- [ ] File compiles: `go build ./internal/tui/`

---

## Subtask T008: Implement openFeatureBrowser() and closeFeatureBrowser()

**Purpose**: Provide methods to initialize and teardown browser state. `openFeatureBrowser()` validates spec-kitty availability, scans features, and populates all state fields. `closeFeatureBrowser()` resets everything.

**Steps**:

1. Add to `internal/tui/browser.go` (after the pure functions from WP01):

   ```go
   func (m *Model) openFeatureBrowser() tea.Cmd {
       if err := ensureSpecKittyAvailable(); err != nil {
           m.launcherNote = err.Error()
           return nil
       }

       entries, err := scanFeatures()
       if err != nil {
           m.launcherNote = fmt.Sprintf("failed to scan features: %v", err)
           return nil
       }

       m.showFeatureBrowser = true
       m.featureEntries = entries
       m.featureFiltered = filterFeatures(entries, "")
       m.featureSelectedIdx = 0
       m.featureActionsOpen = false
       m.featureActionIdx = 0
       m.featureFilterActive = false
       m.featureFilter = styledTextInput()
       m.featureFilter.Placeholder = "filter features..."
       m.featureFilter.SetWidth(40)
       m.launcherNote = ""
       m.updateKeyStates()
       return nil
   }

   func (m *Model) closeFeatureBrowser() {
       m.showFeatureBrowser = false
       m.featureEntries = nil
       m.featureFiltered = nil
       m.featureSelectedIdx = 0
       m.featureActionsOpen = false
       m.featureActionIdx = 0
       m.featureFilterActive = false
       m.featureFilter = textinput.Model{}
       m.updateKeyStates()
   }
   ```

2. `ensureSpecKittyAvailable()` already exists in `newdialog.go` (line 536-542). Reuse it directly.
3. `styledTextInput()` exists in `styles.go` (lines 224-231). Reuse it directly.
4. Import `tea "github.com/charmbracelet/bubbletea/v2"` if not already in browser.go.

**Pattern reference**: `openRestorePicker()` and `closeRestorePicker()` follow the same flag+state pattern but are defined inline. The browser follows the same convention.

**Files**: `internal/tui/browser.go`

**Validation**:
- [ ] openFeatureBrowser validates spec-kitty availability before scanning
- [ ] Scan failure sets launcherNote (does not crash)
- [ ] featureFiltered initialized to all indices (no filter active)
- [ ] closeFeatureBrowser resets all 8 state fields
- [ ] updateKeyStates called in both open and close

---

## Subtask T009: Add `b` Key Case to updateLauncherKeys()

**Purpose**: Handle the `b` key press in the launcher to open the feature browser.

**Steps**:

1. Open `internal/tui/update.go` and add a `b` case to the switch in `updateLauncherKeys()`. Add it after the `s` case (after line 778):

   ```go
   case "b":
       m.launcherNote = ""
       return m, m.openFeatureBrowser()
   ```

**Current code** at update.go lines 741-781:
```go
switch msg.String() {
case "f":
    // ... ensureSpecKittyAvailable, transitionFromLauncher, openNewDialog ...
case "p":
    // ... ensureSpecKittyAvailable, listSpecKittyFeatureDirs, transitionFromLauncher ...
case "r":
    m.launcherNote = ""
    return m, m.openRestorePicker()
case "s":
    m.launcherNote = ""
    return m, m.openSettingsView()
default:
    return m, nil
}
```

2. Add the `b` case BEFORE the `default` case, AFTER the `s` case.

**Files**: `internal/tui/update.go` (line ~778)

**Validation**:
- [ ] Pressing `b` in launcher opens the feature browser
- [ ] launcherNote is cleared before opening
- [ ] No other keys are affected

---

## Subtask T010: Add Menu Item to launcherMenuItems

**Purpose**: Show the "browse features" option in the launcher menu so users discover the `b` key.

**Steps**:

1. Open `internal/tui/launcher.go` and add a new entry to `launcherMenuItems`. Insert it after the "create plan" entry (line 28) and before "view history" (line 29):

   **Current** (launcher.go lines 25-33):
   ```go
   var launcherMenuItems = []launcherMenuItem{
       {key: "n", label: "new task", description: "spawn a worker in yolo mode"},
       {key: "f", label: "create feature spec", description: "start spec-kitty feature creation"},
       {key: "p", label: "create plan", description: "start spec-kitty plan flow"},
       {key: "h", label: "view history", description: "browse past sessions"},
       // ...
   }
   ```

   **Target** (add after the `p` entry):
   ```go
   {key: "b", label: "browse features", description: "open spec-kitty feature browser"},
   ```

2. Place it logically after the creation commands (`f`, `p`) and before navigation (`h`, `r`).

**Files**: `internal/tui/launcher.go` (line ~29)

**Validation**:
- [ ] Menu item appears between "create plan" and "view history"
- [ ] Key is "b", label is "browse features"
- [ ] Launcher renders correctly with the new item

---

## Subtask T011: Add showFeatureBrowser Dispatches in View() and Update()

**Purpose**: Connect the browser to the main View and Update dispatch chains so it renders and receives key events.

**Steps**:

1. **View dispatch** - Open `internal/tui/model.go`. In the `View()` method, add a `showFeatureBrowser` check BEFORE the `showRestorePicker` check (before line 318):

   **Current** (model.go lines 317-331):
   ```go
   if m.showLauncher {
       if m.showRestorePicker {
           return m.renderRestorePicker()
       }
       // ...
   }
   ```

   **Target**:
   ```go
   if m.showLauncher {
       if m.showFeatureBrowser {
           return m.renderFeatureBrowser()
       }
       if m.showRestorePicker {
           return m.renderRestorePicker()
       }
       // ...
   }
   ```

2. **Update dispatch** - Open `internal/tui/update.go`. In the `Update()` method, add a `showFeatureBrowser` dispatch BEFORE the `showRestorePicker` dispatch (before line 37):

   **Current** (update.go lines 37-39):
   ```go
   if m.showRestorePicker {
       return m.updateRestorePicker(msg)
   }
   ```

   **Target** (add before showRestorePicker):
   ```go
   if m.showFeatureBrowser {
       return m.updateFeatureBrowser(msg)
   }
   ```

3. **Stub functions** - Since WP03 and WP04 haven't been implemented yet, add placeholder stubs in `browser.go` so the code compiles:

   ```go
   func (m *Model) renderFeatureBrowser() string {
       return m.renderWithBackdrop(dialogStyle.Width(70).Render("feature browser (loading...)"))
   }

   func (m *Model) updateFeatureBrowser(msg tea.Msg) (tea.Model, tea.Cmd) {
       if keyMsg, ok := msg.(tea.KeyMsg); ok {
           if key.Matches(keyMsg, m.keys.Back) {
               m.closeFeatureBrowser()
               return m, nil
           }
       }
       return m, nil
   }
   ```

   These stubs will be replaced by WP03 and WP04. The stub renderFeatureBrowser uses the existing `renderWithBackdrop` and `dialogStyle` patterns. The stub updateFeatureBrowser handles Esc to close.

**Files**: `internal/tui/model.go` (View), `internal/tui/update.go` (Update), `internal/tui/browser.go` (stubs)

**Validation**:
- [ ] showFeatureBrowser checked before showRestorePicker in View
- [ ] showFeatureBrowser dispatched before showRestorePicker in Update
- [ ] Stub functions compile and provide minimal functionality (Esc closes)
- [ ] Browser appears as a dialog over the launcher backdrop

---

## Subtask T012: Update updateKeyStates() to Handle Browser Overlay

**Purpose**: When the browser is open, disable keys that shouldn't be active (spawn, cycle mode, etc.). The browser is an overlay like history, restore picker, and settings.

**Steps**:

1. Open `internal/tui/keys.go`. In `updateKeyStates()`, find the `overlayActive` check (line 299):

   **Current**:
   ```go
   overlayActive := m.showHelp || m.showSpawnDialog || m.showContinueDialog || m.showBatchDialog || m.showQuitConfirm || m.showNewDialog || m.showHistory || m.showRestorePicker || m.showSettings
   ```

   **Target** (add `m.showFeatureBrowser`):
   ```go
   overlayActive := m.showHelp || m.showSpawnDialog || m.showContinueDialog || m.showBatchDialog || m.showQuitConfirm || m.showNewDialog || m.showHistory || m.showRestorePicker || m.showSettings || m.showFeatureBrowser
   ```

**Files**: `internal/tui/keys.go` (line ~299)

**Validation**:
- [ ] showFeatureBrowser included in overlayActive check
- [ ] When browser is open, New/CycleMode/History keys are disabled
- [ ] File compiles

---

## Definition of Done

- [ ] All 6 subtasks implemented
- [ ] `go build ./internal/tui/` succeeds
- [ ] Pressing `b` in the launcher opens a stub browser dialog
- [ ] Pressing `Esc` in the browser closes it and returns to the launcher
- [ ] Menu shows "browse features" option with `b` key
- [ ] Key states correctly updated when browser is open/closed
- [ ] No regressions in existing launcher functionality (n, f, p, h, r, s, q all work)

## Risks

- **Import cycles**: browser.go uses types from the same `tui` package, so no import issues. The `textinput` import in model.go is new but safe.
- **View dispatch order**: showFeatureBrowser MUST be checked before showRestorePicker. If checked after, the restore picker would intercept when both are true (shouldn't happen, but defense in depth).
- **Stub replacement**: WP03 and WP04 will replace the stubs. The stubs provide minimal Esc-to-close functionality so the feature is testable at this stage.

## Reviewer Guidance

- Verify View dispatch order: showFeatureBrowser before showRestorePicker
- Verify Update dispatch order: showFeatureBrowser before showRestorePicker
- Verify overlayActive includes showFeatureBrowser
- Verify openFeatureBrowser validates spec-kitty before scanning
- Verify closeFeatureBrowser resets ALL browser fields
- Check menu item placement is logically grouped (after creation, before navigation)

## Activity Log

- 2026-02-20T07:09:54Z – unknown – shell_pid=3972069 – lane=for_review – Ready for review: wired browser launcher state, dispatch, and key/menu integration
- 2026-02-20T08:34:40Z – unknown – shell_pid=3972069 – lane=done – Previously approved by user.
