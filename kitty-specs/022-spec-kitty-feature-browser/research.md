# Research: Spec-Kitty Feature Browser

**Feature**: 022-spec-kitty-feature-browser
**Date**: 2026-02-20

## R-001: Feature Scanning Patterns

**Question**: How should the browser discover and classify features in `kitty-specs/`?

**Findings**:

The existing `listSpecKittyFeatureDirs()` in `internal/tui/newdialog.go` (lines 544-562) only finds features that have `tasks/WP*.md` files. The browser needs a broader scan that finds ALL features with `spec.md`.

The existing `autoDetectSpecKittySource()` in `internal/task/source.go` (lines 125-176) has a useful grouping pattern: it collects candidates by feature directory, tracks modification times, and sorts by recency. However, it also only scans for WP files.

**Decision**: New scanner function that:
1. Globs `kitty-specs/*/spec.md` to find all valid features
2. For each feature dir, checks `plan.md` existence and globs `tasks/WP*.md`
3. Extracts number and slug from directory name (split on first `-`)
4. Returns `[]FeatureEntry` sorted by number descending

**Rationale**: File existence checks are the simplest, fastest, and most reliable detection method. No need to parse file contents. A full scan of 50 features involves ~150 stat calls, completing in under 1ms on any modern filesystem.

**Alternatives considered**:
- Parse `meta.json` for feature metadata: More data but slower, and `meta.json` may not exist for older features
- Use `spec-kitty agent` CLI for feature listing: Adds subprocess overhead for something trivially done with filepath.Glob

## R-002: Textinput Filter Integration

**Question**: How to implement `/` filter with `bubbles/textinput` in a custom-rendered list?

**Findings**:

`styledTextInput()` in `internal/tui/styles.go` (lines 224-231) provides the standard kasmos textinput with purple prompt, cream text, hot-pink cursor, and gray placeholder. This is reusable directly.

The filter workflow:
1. User presses `/` -> `featureFilterActive = true`, focus the textinput
2. Each keystroke: Update the textinput, recompute `featureFiltered` (indices into `featureEntries` matching the filter)
3. Enter: Confirm filter (keep filtered, blur textinput, return to navigation mode)
4. Esc: Clear filter, restore full list, blur textinput, return to navigation

**Decision**: Case-insensitive substring match against feature slug. The filtered list is stored as `[]int` indices into the full `featureEntries` slice, avoiding copies.

**Rationale**: Substring matching is the standard TUI filter pattern (k9s, lazygit, etc.). Index-based filtering avoids copying FeatureEntry structs. Case-insensitive because feature slugs are kebab-case.

## R-003: Inline Tree Expansion Rendering

**Question**: How to render lifecycle action sub-menu items inline below the selected feature?

**Findings**:

No existing kasmos pattern for inline tree expansion. The restore picker and plan picker render flat lists with `>` selection indicators. The closest analog is the batch dialog which renders per-item metadata below each line.

The tree chars (`---`, `|--` using ASCII equivalents) need to be styled dimmer than the feature entries to create visual hierarchy. The `colorMidGray` and `colorLightGray` from the existing palette are appropriate.

Rendering approach:
```
  > 018  blocked-task-visual-feedback   spec only
    |-- clarify    run /spec-kitty.clarify
    '-- plan       run /spec-kitty.plan
```

Using ASCII tree chars (`|--` and `'--`) instead of Unicode box-drawing (`---` and `'--`) for maximum terminal compatibility, consistent with the spec-kitty AGENTS.md UTF-8 rule.

**Decision**: When `featureActionsOpen` is true, insert action lines immediately after the selected entry. Actions use indented tree chars with dimmer styling. `j/k` navigates between actions when expanded. Selection of an action uses `>` prefix on the action line.

**Rationale**: Inline expansion keeps context visible (the user can still see surrounding features). Tree chars provide clear visual grouping. ASCII-safe chars prevent encoding issues.

## R-004: Lifecycle Action to Spawn Dialog Mapping

**Question**: How do lifecycle actions connect to worker spawning?

**Findings**:

The existing `openSpawnDialogWithPrefill(role, prompt, files)` in `internal/tui/overlays.go` (lines 71-79) pre-fills the spawn dialog with a role, prompt, and file list. This is the natural integration point.

The existing `p` key flow (lines 751-772 of `update.go`) demonstrates the pattern: scan features, select one, transition from launcher, open spawn dialog. The browser's lifecycle actions follow the same pattern but with phase-aware routing.

Action-to-spawn mapping:
- "clarify" -> role: "planner", prompt: "Run /spec-kitty.clarify for feature kitty-specs/022-spec-kitty-feature-browser"
- "plan" -> role: "planner", prompt: "Run /spec-kitty.plan for feature kitty-specs/022-spec-kitty-feature-browser"
- "tasks" -> role: "planner", prompt: "Run /spec-kitty.tasks for feature kitty-specs/022-spec-kitty-feature-browser"

For tasks-ready features (direct dashboard load), the pattern is:
1. Call `task.DetectSourceType(featureDir)` to create the source
2. Call `m.swapTaskSource(source)` to load it
3. Call `m.transitionFromLauncher()` to close the launcher

This matches how `kasmos kitty-specs/<feature>/` works via the CLI arg path in `main.go` (lines 73-81).

**Decision**: Use `openSpawnDialogWithPrefill()` for lifecycle actions. Use `swapTaskSource()` + `transitionFromLauncher()` for tasks-ready direct load. The feature browser is responsible for constructing the correct prompt string with the feature directory path.

**Rationale**: Reusing existing spawn infrastructure avoids duplicating worker management logic. The spawn dialog gives the user a chance to review/edit the prompt before spawning.

## R-005: Browser View as Launcher Sub-View

**Question**: Where does the browser render in the View hierarchy?

**Findings**:

The launcher's View dispatch in `model.go` (lines 317-331) checks sub-views in order:
```go
if m.showRestorePicker { return m.renderRestorePicker() }
if m.showHistory { return m.renderHistoryOverlay() }
if m.showSettings { return m.renderSettingsView() }
if m.showQuitConfirm { return m.renderQuitConfirm() }
return m.renderLauncher(m.width, m.height)
```

The browser should be added as another check in this chain. It follows the same backdrop-dialog pattern as the restore picker.

The Update dispatch in `updateLauncherKeys` (lines 714-782) handles key routing. The `b` key case goes here, following the same pattern as `r` (restore) and `s` (settings).

**Decision**: Add `showFeatureBrowser` check before the restore picker check in View. Add `updateFeatureBrowser()` handler in Update, dispatched from the launcher update chain. New `b` case in `updateLauncherKeys`.

**Rationale**: Consistent with the existing launcher sub-view architecture. No new rendering paradigms needed.
