# Feature Specification: Blocked Task Visual Feedback and Confirmation

**Feature Branch**: `018-blocked-task-visual-feedback`
**Created**: 2026-02-18
**Status**: Draft
**Input**: User description: "grey out work packages that have dependencies unfinished that are blocking it in the spec-kitty view. if a user tries to start one a confirmation dialog should appear"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Blocked Tasks Appear Dimmed (Priority: P1)

When viewing the task panel in spec-kitty mode, any work package whose dependencies are not yet complete should be rendered with visually dimmed/faint styling. This makes it immediately obvious which tasks are actionable and which are waiting on upstream work, without the user needing to read status indicators individually.

**Why this priority**: This is the core visual feedback that prevents users from wasting time trying to figure out which tasks they can act on. It transforms the task panel from a flat list into a prioritized view at a glance.

**Independent Test**: Can be fully tested by loading a spec-kitty feature directory with dependency chains and verifying that blocked tasks render with faint/dim text styling while unassigned, in-progress, and done tasks retain their normal styling.

**Acceptance Scenarios**:

1. **Given** a spec-kitty feature with WP01 (no deps, unassigned), WP02 (depends on WP01, blocked), and WP03 (depends on WP02, blocked), **When** the task panel renders, **Then** WP01 appears with normal styling and WP02/WP03 appear with dimmed/faint text for their entire row (ID, title, and meta line).
2. **Given** a blocked task WP02 that becomes unblocked because WP01 is marked done, **When** the task panel re-renders after dependency resolution, **Then** WP02 transitions from dimmed styling to normal styling.
3. **Given** a task panel with a mix of done, in-progress, unassigned, and blocked tasks, **When** viewing the panel, **Then** only blocked tasks appear dimmed; all other states retain their existing styling and indicators.

---

### User Story 2 - Confirmation Dialog on Blocked Task Spawn (Priority: P1)

When a user selects a blocked task and presses enter to spawn a worker, a confirmation dialog appears warning them that the task has unfinished dependencies. The dialog lists which specific dependency IDs are still incomplete so the user can make an informed decision to proceed or cancel.

**Why this priority**: This is the safety net that prevents accidental spawning of tasks whose prerequisites are not met. Without it, the visual dimming alone could be missed or ignored, leading to wasted agent cycles on tasks that may fail due to missing upstream work.

**Independent Test**: Can be fully tested by navigating to a blocked task in the task panel, pressing enter, and verifying the confirmation dialog appears with the correct dependency list and both action buttons work correctly.

**Acceptance Scenarios**:

1. **Given** a blocked task WP03 with dependencies [WP01, WP02] where WP01 is done and WP02 is in-progress, **When** the user selects WP03 and presses enter, **Then** a confirmation dialog appears listing "WP02" as the unfinished dependency (WP01 is omitted because it is done).
2. **Given** the confirmation dialog is showing for a blocked task, **When** the user selects "spawn anyway", **Then** the spawn dialog opens with the task's role and description pre-filled (same behavior as spawning an unassigned task) and the task transitions to in-progress once the worker is spawned.
3. **Given** the confirmation dialog is showing for a blocked task, **When** the user selects "cancel" or presses escape, **Then** the dialog closes and no worker is spawned; the task remains in blocked state.

---

### User Story 3 - Blocked Tasks in Batch Spawn (Priority: P2)

In the batch spawn dialog, blocked tasks should not appear in the selectable list. This prevents users from accidentally including blocked tasks in a batch operation where individual confirmation is not practical.

**Why this priority**: The batch dialog is a bulk operation. Injecting per-task confirmation dialogs into a batch flow would be confusing. Instead, blocked tasks are simply excluded from the batch selection list, consistent with the current behavior that only shows unassigned tasks.

**Independent Test**: Can be fully tested by opening the batch spawn dialog with a mix of unassigned and blocked tasks and verifying that only unassigned tasks appear in the selectable list.

**Acceptance Scenarios**:

1. **Given** tasks WP01 (unassigned), WP02 (blocked), WP03 (unassigned), **When** the user opens the batch spawn dialog, **Then** only WP01 and WP03 appear as selectable options; WP02 is not listed.

---

### Edge Cases

- What happens when a blocked task has all its dependencies listed but none of them exist as actual tasks (orphaned dependency IDs)? The task should still appear as blocked and the confirmation dialog should list the missing IDs as unfinished.
- What happens when a task has no dependencies but is in the blocked state due to a transitive dependency chain resolution? The dimming and confirmation should apply based on actual task state, regardless of how it got there.
- What happens when all dependencies of a blocked task complete while the confirmation dialog is open? The dialog should still function normally; the user proceeds or cancels. The task state will update on the next render cycle after the dialog closes.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The task panel MUST render blocked tasks with visually dimmed/faint styling applied to the entire task item (status indicator, ID, title, and meta line).
- **FR-002**: The dimmed styling MUST be visually distinct from all other task states (unassigned, in-progress, for-review, done, failed) while preserving the blocked status indicator icon.
- **FR-003**: When a user presses the select key (enter) on a blocked task in the task panel, the system MUST display a confirmation dialog instead of directly opening the spawn dialog.
- **FR-004**: The confirmation dialog MUST list only the unfinished dependency IDs (dependencies that are not in the done state) for the selected blocked task.
- **FR-005**: The confirmation dialog MUST provide two actions: "spawn anyway" (proceeds to spawn dialog) and "cancel" (closes dialog, no action taken).
- **FR-006**: Selecting "spawn anyway" MUST open the standard spawn dialog pre-filled with the task's suggested role and description, identical to the existing unassigned task spawn flow.
- **FR-007**: The batch spawn dialog MUST continue to exclude blocked tasks from the selectable list (maintaining current behavior where only unassigned tasks are shown).
- **FR-008**: When a blocked task's dependencies are resolved (all become done), the task MUST transition from dimmed styling to normal styling on the next render cycle.
- **FR-009**: The confirmation dialog MUST be dismissable via the escape key, consistent with all other kasmos dialogs.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can visually distinguish blocked tasks from actionable tasks within 1 second of viewing the task panel, without reading individual status indicators.
- **SC-002**: 100% of blocked task spawn attempts present the confirmation dialog before any worker is created.
- **SC-003**: The confirmation dialog accurately lists all unfinished dependencies for the selected task with zero false positives (done dependencies are never listed).
- **SC-004**: Blocked tasks never appear in the batch spawn selection list.
- **SC-005**: Task panel rendering performance remains unaffected - no perceptible delay when switching between task states or resolving dependencies.

## Assumptions

- The existing `TaskBlocked` state and `resolveDependencyStates()` logic correctly identifies blocked tasks. This feature builds on that foundation without modifying dependency resolution behavior.
- The dimmed styling will use lipgloss `.Faint(true)` or equivalent, which is well-supported across modern terminal emulators.
- The confirmation dialog follows the existing kasmos dialog patterns (backdrop overlay, key hints, escape to dismiss) for visual and behavioral consistency.
- The "spawn anyway" action treats the task identically to an unassigned task for the purpose of worker creation and task state transitions.
