# Implementation Plan: Spec-Kitty Feature Browser

**Branch**: `022-spec-kitty-feature-browser` | **Date**: 2026-02-20 | **Spec**: `kitty-specs/022-spec-kitty-feature-browser/spec.md`
**Input**: Feature specification from `kitty-specs/022-spec-kitty-feature-browser/spec.md`

## Summary

Add a feature browser to the kasmos launcher, accessible via the `b` key, that lists all spec-kitty features from `kitty-specs/` with phase indicators (spec only, plan ready, tasks ready). Tasks-ready features load the dashboard directly; non-ready features show an inline lifecycle sub-menu with tree expansion offering the next applicable spec-kitty actions (clarify, plan, tasks). Includes `/` filter via `bubbles/textinput` for projects with many features. Implementation uses a custom lipgloss renderer (not `bubbles/list`) to support the inline tree expansion pattern, consistent with all existing launcher sub-views.

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: bubbletea v2, lipgloss v2, bubbles v2 (textinput)
**Storage**: Filesystem only (`kitty-specs/` directory scanning via `filepath.Glob` + `os.Stat`)
**Testing**: `go test ./...` with table-driven tests for scanner and phase detection
**Target Platform**: Linux (primary), macOS (secondary)
**Project Type**: Single Go module, TUI application
**Performance Goals**: Feature scan + render in under 200ms for up to 50 features
**Constraints**: Must not block the bubbletea Update loop; must follow existing kasmos palette and visual conventions
**Scale/Scope**: Handles up to 50 features in `kitty-specs/`; typical projects have 5-20

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-checked after Phase 1 design.*

| Requirement | Status | Notes |
|-------------|--------|-------|
| Go 1.24+ | PASS | Standard Go, no new language features needed |
| bubbletea v2 Elm architecture | PASS | Standard Model/Update/View pattern; browser state on Model, rendering in View |
| lipgloss v2 for styling | PASS | Uses existing kasmos palette (colorCream, colorMidGray, colorPurple, etc.) |
| bubbles for components | PASS | Uses `textinput` for filter; no new component types |
| No manager AI agent | PASS | Browser is a navigation aid; no AI orchestration added |
| Tests required | PASS | Table-driven tests for phase detection, scanner; mock filesystem for edge cases |
| Async worker output | N/A | Feature does not change worker output handling |
| Linux primary, macOS secondary | PASS | `filepath.Glob` and `os.Stat` are cross-platform |
| Never block Update loop | PASS | Filesystem scanning is synchronous but sub-millisecond for expected scale |

No violations. No complexity tracking needed.

## Architecture Decisions

### AD-001: Custom Renderer Over bubbles/list

**Decision**: Use a custom lipgloss renderer for the feature browser, not the `bubbles/list` component.

**Rationale**: The inline tree expansion (inserting lifecycle action lines below the selected feature) requires dynamic line manipulation that is awkward with `bubbles/list`'s item model. With a custom renderer, expanded action lines are simply conditionally inserted during rendering. Additionally, all existing launcher sub-views (restore picker, history, plan picker) use custom rendering, making this consistent with the codebase.

**Alternatives considered**:
- `bubbles/list` with custom `ItemDelegate`: Provides built-in filtering, scrolling, and navigation. Rejected because expanding/collapsing the sub-menu would require mutating the items list, and the component's chrome (title bar, pagination) would need suppression to match the launcher aesthetic.

**Trade-off**: We lose built-in scrolling from `bubbles/list`. For the expected scale (5-50 features), manual scroll management via index clamping is trivial. The `/` filter is implemented separately via `bubbles/textinput`.

### AD-002: Synchronous Feature Scanning

**Decision**: Scan `kitty-specs/` synchronously in the `b` key handler, not via an async `tea.Cmd`.

**Rationale**: For a project with 50 features, the scan involves ~1 glob + ~150 stat calls. On any modern filesystem (ext4, APFS, even NFS), this completes in under 1ms. Async scanning would add complexity (loading state, spinner, scan-complete message type) for no perceptible benefit.

**Alternatives considered**:
- Async scan via `tea.Cmd` with spinner: Correct for network filesystems or massive project counts. Rejected for the expected scale. If projects regularly exceed 100 features or `kitty-specs/` is on a network filesystem, this should be revisited. This satisfies NFR-003 because current target scale (<50 features, local filesystem) keeps scan latency comfortably under the NFR-001 200ms budget.

### AD-003: Browser as Launcher Sub-View

**Decision**: The feature browser is a launcher sub-view, rendered in the same View dispatch chain as the restore picker, history overlay, and settings view. It uses the backdrop-dialog pattern (`renderWithBackdrop`).

**Rationale**: Follows the established launcher architecture exactly. The browser appears as a centered dialog over the launcher backdrop, consistent with every other launcher sub-view. No new rendering paradigms needed.

**Implementation**: Add `showFeatureBrowser` check in `View()` at `model.go` line 318 (before `showRestorePicker`). Add `updateFeatureBrowser()` dispatch in the Update chain.

### AD-004: Inline Tree Expansion for Lifecycle Sub-Menu

**Decision**: When the user presses Enter on a non-tasks-ready feature, lifecycle actions appear as indented tree-structured lines immediately below the selected entry. Navigation switches to the action items. Esc collapses back to the feature list.

**Rationale**: Inline expansion keeps the feature list context visible. The user can see which feature they selected and what other features exist nearby. A separate view (full replacement) would lose this context.

**Navigation**: `up/down` and `j/k` for list navigation. `Enter` or `right` to expand/select. `Esc` or `left` to collapse/go back. This matches standard TUI directional semantics (right = drill in, left = go back).

**Visual design**:
```
  > 018  blocked-task-visual-feedback   spec only
    |-- clarify    run /spec-kitty.clarify
    '-- plan       run /spec-kitty.plan
```

Using ASCII tree chars (`|--`, `'--`) for UTF-8 safety per the spec-kitty AGENTS.md encoding rules. Action lines styled with `colorMidGray` for the tree chars and `colorCream` for the action label. Selected action highlighted with `>` prefix and `colorPurple`.

### AD-005: Filter via bubbles/textinput

**Decision**: Press `/` to activate a textinput at the bottom of the browser dialog. Case-insensitive substring match against feature slug. Enter confirms, Esc clears.

**Rationale**: Standard TUI filter pattern (k9s, lazygit, fzf). The `styledTextInput()` function in `styles.go` provides the correctly-styled textinput already used by spawn dialog, continue dialog, and new dialog.

**Implementation**: `featureFiltered []int` stores indices into the full `featureEntries` slice. When filter text changes, recompute `featureFiltered`. The renderer iterates `featureFiltered` instead of `featureEntries` directly.

### AD-006: Lifecycle Action Spawning via Existing Infrastructure

**Decision**: Lifecycle actions use `transitionFromLauncher()` + `swapTaskSource()` + `openSpawnDialogWithPrefill()` to spawn workers. Tasks-ready features use `swapTaskSource()` + `transitionFromLauncher()` to load the dashboard directly.

**Rationale**: Reuses all existing worker spawning logic. No new message types, command constructors, or spawn patterns needed. The spawn dialog gives the user a chance to review/edit the prompt before spawning.

**Prompt construction**: Each lifecycle action has a `promptFmt` string. Example for "plan":
```
"Run /spec-kitty.plan for feature %s"  ->  "Run /spec-kitty.plan for feature kitty-specs/022-spec-kitty-feature-browser"
```

## Project Structure

### Documentation (this feature)

```
kitty-specs/022-spec-kitty-feature-browser/
|-- spec.md              # Feature specification
|-- plan.md              # This file
|-- research.md          # Phase 0 research findings
|-- data-model.md        # Types, state, file placement
|-- meta.json            # Feature metadata
|-- checklists/
|   '-- requirements.md  # Spec quality checklist
|-- research/            # (empty, research consolidated in research.md)
'-- tasks/               # (populated by /spec-kitty.tasks)
```

### Source Code (repository root)

```
internal/tui/
|-- browser.go           # NEW: FeatureEntry, FeaturePhase, lifecycleAction types,
|                        #      scanFeatures(), renderFeatureBrowser(),
|                        #      updateFeatureBrowser(), openFeatureBrowser(),
|                        #      closeFeatureBrowser(), actionsForPhase()
|-- browser_test.go      # NEW: Table-driven tests for phase detection, scanning,
|                        #      filter logic, action mapping
|-- model.go             # MODIFY: Add browser state fields to Model struct,
|                        #         add showFeatureBrowser check in View()
|-- update.go            # MODIFY: Add 'b' case in updateLauncherKeys(),
|                        #         dispatch to updateFeatureBrowser()
|-- keys.go              # MODIFY: Add showFeatureBrowser to overlayActive check
|-- launcher.go          # MODIFY: Add menu item {key: "b", label: "browse features", ...}
|-- styles.go            # MODIFY (optional): Add phaseBadge() helper if styling warrants it
'-- messages.go          # NO CHANGE: No new message types needed (synchronous scan)
```

## Quickstart

After implementation, the feature browser works as follows:

1. Run `kasmos` with no arguments
2. Press `b` at the launcher
3. Browse features with `j/k` or arrow keys, filter with `/`, select with Enter or right arrow
4. Tasks-ready features load the dashboard immediately
5. Non-ready features show lifecycle actions; select one to spawn a planner worker

No new CLI flags, config options, or external dependencies.
