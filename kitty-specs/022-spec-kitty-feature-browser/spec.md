# Feature Specification: Spec-Kitty Feature Browser

**Feature Branch**: `022-spec-kitty-feature-browser`
**Created**: 2026-02-20
**Status**: Draft
**Input**: Add a spec-kitty feature browser to the kasmos launcher, accessible when no spec path is provided. The browser lists all kitty-specs features with phase indicators. Selecting a feature either loads the implementation dashboard (if tasks exist) or shows a context-aware sub-menu of applicable spec-kitty lifecycle actions.

## User Scenarios & Testing

### User Story 1 - Open the Feature Browser (Priority: P1)

A developer launches kasmos with no arguments. The launcher appears with its usual menu items. They press `b` to browse existing spec-kitty features. A list of all features found in `kitty-specs/` appears, showing each feature's number, slug, and current phase (spec only, plan ready, or tasks ready with WP count).

**Why this priority**: This is the entry point for the entire feature. Without a way to view and navigate features, no subsequent selection or routing works.

**Independent Test**: Run `kasmos` with several features in `kitty-specs/` at various phases. Press `b`. Verify all features appear with correct phase indicators. Verify keyboard navigation (up/down arrows) works.

**Acceptance Scenarios**:

1. **Given** the launcher is displayed, **When** the user presses `b`, **Then** the feature browser appears listing all features found in `kitty-specs/`.
2. **Given** features exist at various phases, **When** the browser renders, **Then** each entry shows its feature number, slug, and correct phase indicator (e.g., "spec only", "plan ready", "tasks ready (4 WPs)").
3. **Given** the browser is displayed, **When** the user navigates with up/down arrow keys, **Then** the selection highlight moves between entries.

---

### User Story 2 - Select a Feature Ready for Implementation (Priority: P1)

A developer sees a feature marked "tasks ready" in the browser. They select it and press Enter. The launcher closes and the main kasmos dashboard loads with that feature as the active spec-kitty task source, displaying the task panel with WP statuses -- identical to running `kasmos kitty-specs/<feature>/` directly.

**Why this priority**: This is the primary happy path for features that have completed planning. It replaces the need to remember and type the kitty-specs path.

**Independent Test**: Create a feature with WP files in `kitty-specs/`. Open the browser, select that feature, press Enter. Verify the dashboard loads with the correct task source and WPs are visible in the task panel.

**Acceptance Scenarios**:

1. **Given** a feature with `tasks/WP*.md` files is selected in the browser, **When** the user presses Enter, **Then** the launcher closes and the dashboard loads with that feature as the spec-kitty task source.
2. **Given** the dashboard loads from the browser, **When** the task panel renders, **Then** the WPs and their statuses match what `kasmos kitty-specs/<feature>/` would show.
3. **Given** the dashboard loaded from the browser, **When** the user interacts with the dashboard, **Then** all standard dashboard functionality works (spawn workers, view output, navigate tasks).

---

### User Story 3 - Select a Feature Needing Planning (Priority: P1)

A developer sees a feature marked "spec only" or "plan ready" in the browser. They select it and press Enter. Instead of loading the dashboard, a sub-menu appears showing the applicable next spec-kitty lifecycle actions for that feature's current phase. For a spec-only feature, this includes "clarify" and "plan". For a plan-ready feature, this shows "tasks". Selecting an action transitions to the dashboard and opens a worker spawn dialog pre-configured for that action.

**Why this priority**: This is the routing logic that makes the browser more than a simple list. It guides users to the correct next step without them needing to know the spec-kitty pipeline.

**Independent Test**: Create a feature with only `spec.md`. Open the browser, select it, press Enter. Verify a sub-menu appears with "clarify" and "plan". Select "plan". Verify the dashboard opens with a spawn dialog configured for the planner role targeting that feature.

**Acceptance Scenarios**:

1. **Given** a feature with only `spec.md` (no `plan.md`, no WPs) is selected, **When** the user presses Enter, **Then** a sub-menu appears offering "clarify" and "plan" as lifecycle actions.
2. **Given** a feature with `spec.md` and `plan.md` (no WPs) is selected, **When** the user presses Enter, **Then** a sub-menu appears offering "tasks" as the next lifecycle action.
3. **Given** the user selects a lifecycle action from the sub-menu, **When** the action is confirmed, **Then** the dashboard loads and a worker spawn dialog opens, pre-configured with the appropriate agent role and feature path.

---

### User Story 4 - Empty State (Priority: P2)

A developer presses `b` but no features exist in `kitty-specs/`. The browser shows a helpful message indicating there are no features yet and suggests pressing `f` to create one.

**Why this priority**: Graceful handling of the empty state prevents confusion for new users or fresh projects.

**Independent Test**: Run `kasmos` in a project with no `kitty-specs/` directory or an empty one. Press `b`. Verify a helpful empty state message appears.

**Acceptance Scenarios**:

1. **Given** `kitty-specs/` does not exist or contains no feature directories, **When** the user presses `b`, **Then** a message indicates no features were found.
2. **Given** the empty state is displayed, **When** the user reads the message, **Then** it suggests pressing `f` to create a new feature or Escape to return to the launcher.

---

### User Story 5 - Return to Launcher from Browser (Priority: P2)

A developer opens the browser but decides not to select a feature. They press Escape and return to the launcher menu.

**Why this priority**: Users need a clear exit path from any view. Without it, the browser feels like a dead end.

**Independent Test**: Open the browser, press Escape. Verify the launcher menu reappears. Open the browser, enter the lifecycle sub-menu for a feature, press Escape. Verify it returns to the browser list (not straight to the launcher).

**Acceptance Scenarios**:

1. **Given** the feature browser is displayed, **When** the user presses Escape, **Then** the browser closes and the launcher menu reappears.
2. **Given** the lifecycle sub-menu is displayed for a feature, **When** the user presses Escape, **Then** the sub-menu closes and the browser list reappears with the same feature still highlighted.

---

### Edge Cases

- What happens when `kitty-specs/` contains directories that aren't valid features (no `spec.md`)? They are excluded from the browser list.
- What happens when a feature has `tasks/WP*.md` files but all WPs are in the `done` lane? It still shows as "tasks ready" and loads the dashboard. The dashboard's task panel will show the completed state.
- What happens when `spec-kitty` CLI is not installed? The browser shows an error note in the launcher (same pattern as existing `f` and `p` keys) and does not open.
- What happens when many features exist (scrolling needed)? The browser list supports scrolling, keeping the selected item visible within the viewport.
- What happens when the terminal is too small for the browser? The browser follows the existing `layoutTooSmall` pattern -- it requires the same minimum dimensions as the launcher.

## Requirements

### Functional Requirements

- **FR-001**: The launcher MUST include a new `b` key that opens the feature browser.
- **FR-002**: The feature browser MUST scan `kitty-specs/` and list all directories containing a `spec.md` file.
- **FR-003**: Each browser entry MUST display the feature number, slug, and current phase indicator.
- **FR-004**: Phase detection MUST classify features as "spec only" (has `spec.md`, no `plan.md`), "plan ready" (has `plan.md`, no `tasks/WP*.md`), or "tasks ready" (has `tasks/WP*.md` files, showing the WP count).
- **FR-005**: Selecting a "tasks ready" feature MUST close the launcher and load the main dashboard with that feature directory as the spec-kitty task source.
- **FR-006**: Selecting a non-tasks-ready feature MUST display a context-aware sub-menu of applicable spec-kitty lifecycle actions.
- **FR-007**: The lifecycle sub-menu for "spec only" features MUST offer "clarify" and "plan" actions.
- **FR-008**: The lifecycle sub-menu for "plan ready" features MUST offer "tasks" as the action.
- **FR-009**: Selecting a lifecycle action MUST transition to the dashboard and open a worker spawn dialog pre-configured for the appropriate agent role and feature path.
- **FR-010**: The browser MUST support keyboard navigation (up/down arrows to move selection, Enter to select, Escape to go back).
- **FR-011**: The browser MUST show a helpful empty state message when no features exist in `kitty-specs/`.
- **FR-012**: The browser MUST validate that `spec-kitty` CLI is available before opening, showing an error note if not found.
- **FR-013**: Escape from the lifecycle sub-menu MUST return to the browser list; Escape from the browser MUST return to the launcher.

### Non-Functional Requirements

- **NFR-001**: Feature scanning and phase detection MUST complete without perceptible delay (under 200ms for typical project sizes of up to 50 features).
- **NFR-002**: The browser MUST follow existing kasmos visual styling conventions (shared color palette, component patterns consistent with the launcher and dashboard).
- **NFR-003**: The browser MUST NOT block the main TUI event loop. Filesystem scanning, if it becomes expensive, MUST be performed asynchronously.

### Key Entities

- **FeatureEntry**: A discovered spec-kitty feature. Attributes: feature number (string), slug (string), directory path (string), phase (FeaturePhase).
- **FeaturePhase**: Enumeration of feature lifecycle phases as detected by the browser: SpecOnly, PlanReady, TasksReady. Determined by filesystem presence of `spec.md`, `plan.md`, and `tasks/WP*.md`.
- **LifecycleAction**: An available spec-kitty workflow action for a given phase. Maps phase to action labels and the agent role + arguments needed to spawn the corresponding worker.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can navigate from the launcher to a loaded dashboard for any tasks-ready feature in under 3 key presses (b, navigate, Enter).
- **SC-002**: Feature scanning completes and the browser renders within 200ms of pressing `b`, for projects with up to 50 features.
- **SC-003**: Users can identify the current phase of every feature at a glance without opening any files or running additional commands.
- **SC-004**: Users who select a non-ready feature are presented with only the applicable next steps, requiring no prior knowledge of the spec-kitty pipeline order.

## Assumptions

- The `kitty-specs/` directory follows the standard spec-kitty convention: each feature is a subdirectory named `NNN-slug/` containing at minimum a `spec.md` file.
- Phase detection uses file existence only (`spec.md`, `plan.md`, `tasks/WP*.md`), not file content parsing, for speed and simplicity.
- The existing `newDialog` and worker spawn patterns from the `f` and `p` launcher keys provide reusable infrastructure for the lifecycle action spawning.
- The `listSpecKittyFeatureDirs()` function in `internal/tui/newdialog.go` provides a scanning pattern, though the browser needs a broader scan (all features, not just those with WPs).
