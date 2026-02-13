# Feature Specification: Hub TUI Navigator

**Feature Branch**: `010-hub-tui-navigator`
**Created**: 2026-02-12
**Status**: Draft
**Input**: Replace `kasmos` with no args as a new TUI view that allows browsing available feature specs, creating new ones (with planning and task generation flow), or starting implementation. Invert `kasmos start` to default to TUI mode. The hub TUI launches Zellij panes/tabs for actions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Browse Feature Specs (Priority: P1)

An operator runs `kasmos` with no arguments and sees an interactive TUI showing all features from `kitty-specs/`. Each feature displays its number, slug, spec status, and task progress. The operator can scroll through the list, select a feature, and view its details — including individual work package breakdown, dependency information, and lane status — without taking any action.

**Why this priority**: The feature browser is the foundational view of the hub. Without it, the operator has no way to discover what's available or decide what to work on next.

**Independent Test**: Can be tested by populating `kitty-specs/` with features in various states (empty spec, spec without tasks, tasks partially done, all tasks complete) and verifying the list renders correctly with accurate status indicators.

**Acceptance Scenarios**:

1. **Given** `kitty-specs/` contains features in various states, **When** the operator runs `kasmos` with no arguments, **Then** an interactive TUI displays all features with their number, slug, and status indicator (empty spec / no tasks / X/Y done / complete).
2. **Given** the feature list is displayed, **When** the operator selects a feature and presses a detail key, **Then** a detail view shows individual work packages with their lane status (planned/doing/for_review/done), dependencies, and wave assignment.
3. **Given** a feature has no tasks directory, **When** it appears in the list, **Then** it shows a "no tasks" indicator and its available actions reflect that state (e.g., "Plan / Generate Tasks").
4. **Given** all work packages for a feature are marked "done", **When** the feature appears in the list, **Then** it is visually distinguished as complete (e.g., dimmed or marked with a checkmark).
5. **Given** the operator is viewing feature details, **When** they press Escape, **Then** they return to the feature list with their previous selection preserved.

---

### User Story 2 - Create New Feature Spec (Priority: P1)

An operator selects a feature with an empty spec (or chooses "New Feature") and triggers spec creation. The hub opens a Zellij pane to the right of itself (same tab, side-by-side split) running an OpenCode controller agent session pre-configured for the spec-kitty specify workflow. The operator conducts the discovery conversation in that pane while the hub remains visible on the left.

**Why this priority**: Creating new specs is one of the two primary workflows the hub enables. The side-by-side layout lets the operator reference existing features in the hub while working with the agent.

**Independent Test**: Can be tested by triggering spec creation from the hub, verifying that a new Zellij pane appears to the right running OpenCode, and confirming the hub remains responsive and visible.

**Acceptance Scenarios**:

1. **Given** the operator selects a feature with an empty spec, **When** they trigger the "Create Spec" action, **Then** a new Zellij pane opens to the right of the hub running an OpenCode controller agent session.
2. **Given** the OpenCode agent pane is open for spec creation, **When** the operator looks at the hub on the left, **Then** the hub is still visible and interactive (not blocked or hidden).
3. **Given** the operator selects "New Feature" (not an existing empty spec), **When** they trigger creation, **Then** they are prompted for a feature name/description before the agent pane opens.
4. **Given** an agent pane is already open to the right, **When** the operator triggers another action that would open a pane, **Then** the existing pane is replaced or the operator is warned about the active session.
5. **Given** the agent completes spec creation and the pane is closed, **When** the operator returns to the hub, **Then** the feature list refreshes and shows the newly created feature with its updated status.

---

### User Story 3 - Plan and Generate Tasks (Priority: P1)

An operator selects a feature that has a spec but no tasks (or incomplete planning) and triggers the planning workflow. The hub opens a Zellij pane to the right running an OpenCode controller agent session for the spec-kitty plan and tasks workflow. The operator can monitor the planning conversation alongside the hub.

**Why this priority**: Planning and task generation is the bridge between specification and implementation. Without it, the operator cannot progress features toward implementation.

**Independent Test**: Can be tested by selecting a feature with a spec but no tasks, triggering planning, verifying the agent pane opens with the correct context, and confirming task files appear after the workflow completes.

**Acceptance Scenarios**:

1. **Given** the operator selects a feature with a spec but no tasks, **When** they trigger the "Plan" action, **Then** a Zellij pane opens to the right running an OpenCode agent pre-loaded with the spec-kitty planning workflow for that feature.
2. **Given** the operator selects a feature with a spec but no tasks, **When** they trigger the "Generate Tasks" action, **Then** a Zellij pane opens with the task generation workflow, and tasks appear in `kitty-specs/<feature>/tasks/` upon completion.
3. **Given** the planning agent pane is open, **When** the hub detects new files in the feature's directory, **Then** the feature's status in the hub list updates to reflect the new state.
4. **Given** a feature already has a plan but no tasks, **When** the operator views its details, **Then** "Generate Tasks" is available as a distinct action from "Plan".

---

### User Story 4 - Start Implementation (Priority: P1)

An operator selects a feature with work packages ready (tasks generated, WPs in "planned" lane) and triggers implementation. The hub launches `kasmos start <feature>` in a new Zellij tab, which defaults to the orchestration TUI dashboard. The hub tab remains accessible for the operator to switch back to.

**Why this priority**: Starting implementation is the other primary workflow. Launching it in a new tab keeps the hub available as a navigation home base.

**Independent Test**: Can be tested by selecting a feature with ready tasks, triggering implementation, and verifying a new Zellij tab appears with the orchestration TUI running.

**Acceptance Scenarios**:

1. **Given** the operator selects a feature with tasks in the "planned" lane, **When** they trigger "Start Implementation", **Then** a new Zellij tab is created running `kasmos start <feature>` with the orchestration TUI.
2. **Given** implementation is started in a new tab, **When** the operator switches back to the hub tab, **Then** the hub shows the feature's status as "running" with a summary (e.g., "2/8 WPs active").
3. **Given** an orchestration session is already running for a feature, **When** the operator selects that feature, **Then** the available action is "Attach" (switch to existing tab) rather than "Start".
4. **Given** the operator triggers "Start" with mode selection available, **When** they choose between wave-gated and continuous mode, **Then** the orchestration starts in the selected mode.

---

### User Story 5 - Invert `kasmos start` TUI Default (Priority: P1)

The `kasmos start <feature>` command defaults to launching the orchestration TUI dashboard instead of directly attaching to the Zellij session. A `--no-tui` flag is available to opt into the old behavior of direct Zellij session attachment.

**Why this priority**: This is a direct user request that changes the default experience for existing `kasmos start` users. It ensures the TUI is the primary interface.

**Independent Test**: Can be tested by running `kasmos start <feature>` and verifying the TUI launches by default, and running `kasmos start <feature> --no-tui` to verify direct attachment still works.

**Acceptance Scenarios**:

1. **Given** the operator runs `kasmos start <feature>`, **When** orchestration begins, **Then** the orchestration TUI dashboard launches (previously required `--tui` flag).
2. **Given** the operator runs `kasmos start <feature> --no-tui`, **When** orchestration begins, **Then** kasmos directly attaches to the Zellij session without the TUI (previous default behavior).
3. **Given** the old `--tui` flag is present in scripts or commands, **When** it is used, **Then** it is accepted silently (backward compatible) and the TUI launches as normal.

---

### User Story 6 - CLI Help Preserved (Priority: P2)

Running `kasmos --help` displays the existing CLI help text with all available subcommands and usage guidance. The help output documents the new default behavior (bare `kasmos` launches the hub TUI) and all subcommands remain unchanged.

**Why this priority**: Preserving CLI discoverability ensures users can learn the tool's capabilities without consulting external documentation.

**Independent Test**: Can be tested by running `kasmos --help` and verifying the output includes all subcommands, usage examples, and documents the hub TUI as the no-argument default.

**Acceptance Scenarios**:

1. **Given** the operator runs `kasmos --help`, **When** the help text is displayed, **Then** it includes all subcommands (list, start, status, cmd, attach, stop) and documents the hub TUI as the default when no subcommand is given.
2. **Given** the operator runs `kasmos start --help`, **When** the help text is displayed, **Then** it shows the `--no-tui` flag and indicates TUI is the default mode.
3. **Given** an existing script uses `kasmos list`, **When** it runs, **Then** it produces the same stdout output as before (backward compatible).

---

### User Story 7 - Open Hub from Orchestration TUI (Priority: P2)

While using the orchestration TUI (`kasmos start <feature>`), the operator can open the hub TUI in a new Zellij tab. This lets the operator navigate to other features, start planning, or check project-wide status without leaving the orchestration session.

**Why this priority**: Bi-directional navigation between the hub and orchestration views creates a seamless workflow. The operator should never feel trapped in one view.

**Independent Test**: Can be tested by running the orchestration TUI, pressing the hub keybinding, and verifying a new Zellij tab opens with the hub TUI.

**Acceptance Scenarios**:

1. **Given** the operator is in the orchestration TUI, **When** they press the "open hub" keybinding, **Then** a new Zellij tab opens running the hub TUI.
2. **Given** the hub is already open in another tab, **When** the operator triggers "open hub" from the orchestration TUI, **Then** Zellij switches to the existing hub tab rather than creating a duplicate.
3. **Given** the operator opened the hub from the orchestration TUI, **When** they switch back to the orchestration tab, **Then** the orchestration TUI is unaffected and continues displaying live state.

---

### User Story 8 - Feature Status Refresh (Priority: P2)

The hub TUI periodically refreshes feature status from disk (re-scanning `kitty-specs/`) so that changes made by agents in side panes or by other processes are reflected without restarting the hub. The operator can also manually trigger a refresh.

**Why this priority**: Without refresh, the hub becomes stale as agents create specs and tasks in adjacent panes. Staleness would require the operator to quit and restart the hub.

**Independent Test**: Can be tested by modifying files in `kitty-specs/` while the hub is running and verifying the display updates within the refresh interval or upon manual refresh.

**Acceptance Scenarios**:

1. **Given** the hub is running and an agent creates a new spec file, **When** the refresh interval elapses (or the operator presses the refresh key), **Then** the feature list updates to show the new spec's status.
2. **Given** work packages are completed by an orchestration in another tab, **When** the hub refreshes, **Then** the feature's progress indicator updates (e.g., "3/8 done" → "5/8 done").
3. **Given** the operator presses the manual refresh keybinding, **When** `kitty-specs/` is re-scanned, **Then** the feature list updates immediately without waiting for the automatic interval.

---

### Edge Cases

- What happens when `kitty-specs/` does not exist? The hub displays "No kitty-specs/ directory found" with guidance to create one.
- What happens if Zellij is not running (hub launched outside a Zellij session)? The hub displays feature status in read-only mode with a warning that actions requiring Zellij panes/tabs are unavailable.
- What happens when the operator tries to start implementation but the feature has no WP files? The hub shows an error and suggests running the task generation workflow first.
- What happens if the terminal is too narrow for side-by-side split? The hub should warn the operator or fall back to opening the agent in a new tab instead.
- What happens when the operator creates a spec and the agent fails mid-workflow? The hub shows the feature in its last known state; the operator can re-trigger the workflow.
- What happens when multiple features have running orchestrations? The hub shows each with its running status; the operator can attach to any of them.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: When invoked with no subcommand, the system MUST launch an interactive TUI (the "hub") that displays all features found in `kitty-specs/`.
- **FR-002**: The hub MUST display each feature's number, slug, spec status (empty/present), and task progress (no tasks / X/Y done / complete).
- **FR-003**: The hub MUST allow the operator to select a feature and view its details, including individual work package lane status, dependencies, and wave assignments.
- **FR-004**: The hub MUST provide a "Create Spec" action for features with empty specs that opens a Zellij pane to the right of the hub running an OpenCode controller agent session.
- **FR-005**: The hub MUST provide a "New Feature" action that prompts for a feature name/description and then opens an OpenCode agent pane for spec creation.
- **FR-006**: The hub MUST provide "Plan" and "Generate Tasks" actions for features with specs but no tasks, opening an OpenCode agent pane to the right of the hub.
- **FR-007**: The hub MUST provide a "Start Implementation" action for features with ready work packages that launches `kasmos start <feature>` in a new Zellij tab.
- **FR-008**: The hub MUST provide an "Attach" action for features with active orchestration sessions that switches to the existing Zellij tab.
- **FR-009**: The hub MUST remain visible and interactive when agent panes are opened to its right.
- **FR-010**: The hub MUST periodically refresh feature status from disk and support manual refresh via keybinding.
- **FR-011**: The `kasmos start <feature>` command MUST default to TUI mode (previously required `--tui` flag).
- **FR-012**: The `kasmos start` command MUST accept a `--no-tui` flag to opt into direct Zellij session attachment (previous default behavior).
- **FR-013**: The `kasmos start` command MUST silently accept the existing `--tui` flag for backward compatibility.
- **FR-014**: Running `kasmos --help` MUST display help text documenting all subcommands and the hub TUI as the no-argument default.
- **FR-015**: The orchestration TUI MUST provide a keybinding to open the hub TUI in a new Zellij tab.
- **FR-016**: The hub MUST support keyboard-only operation with vim-style navigation (j/k for up/down, Enter for select/action, Esc for back, Alt+q to quit).
- **FR-017**: The hub MUST detect when it is running outside a Zellij session and operate in read-only mode with appropriate warnings for unavailable pane/tab actions.
- **FR-018**: When an agent pane to the right is already active, triggering another pane action MUST either replace the existing pane or warn the operator.
- **FR-019**: All existing subcommands (list, start, status, cmd, attach, stop) MUST continue to function identically.

### Non-Functional Requirements

- **NFR-001 (Startup)**: The hub TUI MUST render the initial feature list within 500ms of invocation on a repository with up to 50 features.
- **NFR-002 (Responsiveness)**: Keyboard input latency in the hub MUST remain under 50ms for navigation operations.
- **NFR-003 (Refresh)**: Periodic disk refresh MUST complete without blocking the UI event loop.
- **NFR-004 (Compatibility)**: The hub MUST gracefully handle terminals with at least 80 columns and 24 rows. Narrower terminals MUST display a minimum viable view.

### Key Entities

- **FeatureEntry**: A feature discovered in `kitty-specs/`. Has `number`, `slug`, `spec_status` (empty/present), `task_progress` (no tasks / done_count/total_count / complete), and `orchestration_status` (none/running/completed).
- **FeatureDetail**: Expanded view of a feature. Includes the list of work packages with their lane, dependencies, and wave assignment.
- **HubAction**: A contextual action available for a feature based on its state: CreateSpec, NewFeature, Plan, GenerateTasks, StartImplementation, Attach, ViewDetails.
- **AgentPane**: A Zellij pane opened to the right of the hub for spec creation, planning, or task generation. Tracked by the hub to prevent duplicate panes.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can identify the state of any feature (spec status, task progress, active orchestration) within 3 seconds of launching the hub.
- **SC-002**: The correct contextual action (Create Spec / Plan / Generate Tasks / Start / Attach) is available for every feature based on its lifecycle state, with zero incorrect action offerings.
- **SC-003**: Agent panes for spec creation and planning open within 2 seconds of the operator triggering the action, with the correct OpenCode session pre-configured.
- **SC-004**: Implementation launches in a new Zellij tab within 3 seconds of the operator triggering "Start", with the orchestration TUI displaying.
- **SC-005**: Feature status in the hub updates to reflect agent-created files (specs, tasks) within 10 seconds or immediately upon manual refresh.
- **SC-006**: 100% of existing CLI subcommands and `--help` output remain backward compatible.
- **SC-007**: The hub remains responsive (keyboard input processed within 50ms) while agent panes are active in adjacent splits.
- **SC-008**: Operators can navigate from the orchestration TUI back to the hub within 2 keystrokes.

## Assumptions

- The hub TUI renders in a terminal using ratatui with the crossterm backend, consistent with the existing orchestration TUI stack.
- Zellij is available in PATH when pane/tab actions are used. The hub degrades gracefully when Zellij is unavailable.
- OpenCode is available in PATH for agent session panes. The hub shows an error if OpenCode is not found when an agent action is triggered.
- The `kitty-specs/` directory structure follows the established convention: `<NNN>-<slug>/spec.md` for specs, `<NNN>-<slug>/tasks/WPxx-*.md` for work packages.
- spec-kitty slash commands (`/spec-kitty.specify`, `/spec-kitty.plan`, `/spec-kitty.tasks`) are available to the OpenCode agent sessions.
- The hub does not need to parse spec.md content beyond checking its existence and non-emptiness. Detailed spec content is the agent's domain.
- Periodic refresh interval defaults to 5 seconds and is not user-configurable in the initial version.
- The "New Feature" prompt for feature name/description is a simple inline input field in the hub, not a multi-step wizard.
