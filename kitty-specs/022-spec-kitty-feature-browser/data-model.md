# Data Model: Spec-Kitty Feature Browser

**Feature**: 022-spec-kitty-feature-browser
**Date**: 2026-02-20

## Types

### FeaturePhase

Enumeration of detected lifecycle phases. Determined by filesystem presence checks.

```go
// FeaturePhase represents the detected lifecycle phase of a spec-kitty feature.
type FeaturePhase int

const (
    PhaseSpecOnly   FeaturePhase = iota // has spec.md only
    PhasePlanReady                      // has spec.md + plan.md, no WPs
    PhaseTasksReady                     // has tasks/WP*.md files
)
```

Display mapping:
- `PhaseSpecOnly` -> "spec only" (styled `colorMidGray`)
- `PhasePlanReady` -> "plan ready" (styled `colorLightBlue`)
- `PhaseTasksReady` -> "tasks ready (N WPs)" (styled `colorGreen`)

### FeatureEntry

Represents a discovered spec-kitty feature for the browser listing.

```go
// FeatureEntry represents a discovered spec-kitty feature.
type FeatureEntry struct {
    Number  string       // e.g., "022"
    Slug    string       // e.g., "spec-kitty-feature-browser"
    Dir     string       // e.g., "kitty-specs/022-spec-kitty-feature-browser"
    Phase   FeaturePhase
    WPCount int          // number of WP files (only meaningful when Phase == PhaseTasksReady)
}
```

Parsing: directory name split on first `-` yields Number and Slug.
Sorting: by Number descending (most recent features first).

### lifecycleAction

Maps a feature phase to available spec-kitty workflow actions.

```go
// lifecycleAction represents an available spec-kitty lifecycle action.
type lifecycleAction struct {
    label       string // display label, e.g., "clarify"
    description string // display description, e.g., "run /spec-kitty.clarify"
    role        string // agent role for spawn dialog
    promptFmt   string // prompt format string with %s for feature dir
}
```

Phase-to-action mapping:

| Phase | Actions |
|-------|---------|
| PhaseSpecOnly | clarify (planner), plan (planner) |
| PhasePlanReady | tasks (planner) |
| PhaseTasksReady | (no sub-menu -- direct dashboard load) |

## Model State Extensions

New fields added to the `Model` struct in `internal/tui/model.go`:

```go
// Feature browser state (launcher sub-view)
showFeatureBrowser   bool
featureEntries       []FeatureEntry
featureFiltered      []int             // indices into featureEntries matching current filter
featureSelectedIdx   int               // index into featureFiltered
featureActionsOpen   bool              // true when lifecycle sub-menu is expanded
featureActionIdx     int               // selected action within expanded sub-menu
featureFilterActive  bool              // true when textinput has focus
featureFilter        textinput.Model   // filter textinput (initialized via styledTextInput())
```

### State Transitions

```
Launcher menu
    |
    | press 'b' (scan kitty-specs/, populate featureEntries)
    v
Feature browser (showFeatureBrowser = true)
    |
    |-- up/down or j/k: navigate featureSelectedIdx
    |-- Enter or right on tasks-ready: swapTaskSource + transitionFromLauncher
    |-- Enter or right on non-ready: expand actions (featureActionsOpen = true)
    |-- /: activate filter (featureFilterActive = true)
    |-- Esc or left: close browser, return to launcher
    |
    v
Lifecycle sub-menu (featureActionsOpen = true)
    |
    |-- up/down or j/k: navigate featureActionIdx
    |-- Enter or right: transitionFromLauncher + openSpawnDialogWithPrefill
    |-- Esc or left: collapse sub-menu (featureActionsOpen = false)
    |
    v
Dashboard (with spawn dialog or loaded task source)
```

### Filter State

```
Browser navigation mode
    |
    | press '/'
    v
Filter mode (featureFilterActive = true, textinput focused)
    |
    |-- keystroke: update textinput, recompute featureFiltered
    |-- Enter: confirm filter, return to navigation mode
    |-- Esc: clear filter, restore full list, return to navigation mode
    |
    v
Browser navigation mode (filtered or full list)
```

## File Placement

| File | Purpose |
|------|---------|
| `internal/tui/browser.go` | FeatureEntry, FeaturePhase, lifecycleAction types. Scanner function. Render function. Update handler. |
| `internal/tui/browser_test.go` | Table-driven tests for phase detection, scanner, filter logic. |
| `internal/tui/model.go` | New fields on Model struct. |
| `internal/tui/update.go` | `b` case in `updateLauncherKeys`. Dispatch to `updateFeatureBrowser`. |
| `internal/tui/model.go` (View) | `showFeatureBrowser` check in launcher View dispatch chain. |
| `internal/tui/launcher.go` | New menu item `{key: "b", label: "browse features", description: "open spec-kitty feature browser"}`. |
