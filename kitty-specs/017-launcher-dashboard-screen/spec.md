# Feature Specification: Launcher Dashboard Screen

**Feature Branch**: `017-launcher-dashboard-screen`
**Created**: 2026-02-18
**Status**: Draft
**Input**: Create a launcher screen like LazyVim when opened with no args. Centered branding with a menu of actions: new task, create feature spec, create plan, view history, restore session, settings, quit.

## User Scenarios & Testing

### User Story 1 - Launcher Appears on Bare Invocation (Priority: P1)

A developer runs `kasmos` with no arguments. Instead of dropping directly into the worker dashboard, they see a centered launcher screen with ASCII art branding and a menu of actions. They press a single key to jump into their desired workflow.

**Why this priority**: This is the entry point for every kasmos session. Without it, new users see an empty dashboard with no guidance, and experienced users must remember keybindings to start common workflows.

**Independent Test**: Run `kasmos` with no arguments, verify the launcher screen appears with branding and all menu items. Press each key and verify it routes to the correct action.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos` with no arguments, **When** the TUI starts, **Then** a centered launcher screen appears with ASCII art branding and a list of actions with keybindings.
2. **Given** the launcher is displayed, **When** the user presses `n`, **Then** the spawn worker dialog opens (yolo mode).
3. **Given** the launcher is displayed, **When** the user presses `f`, **Then** the spec-kitty feature creation flow begins.
4. **Given** the launcher is displayed, **When** the user presses `p`, **Then** the spec-kitty plan flow begins for an existing feature.
5. **Given** the launcher is displayed, **When** the user presses `h`, **Then** the history view opens.
6. **Given** the launcher is displayed, **When** the user presses `r`, **Then** the session restore picker appears.
7. **Given** the launcher is displayed, **When** the user presses `s`, **Then** the settings view opens.
8. **Given** the launcher is displayed, **When** the user presses `q`, **Then** kasmos exits cleanly.

---

### User Story 2 - Restore Session from Launcher (Priority: P1)

A developer ran kasmos yesterday and left workers in various states. Today they run `kasmos` again, see the launcher, and press `r` to restore. They see their last active session prominently displayed, with older archived sessions listed below. They select one and the dashboard loads with the previous session's workers and state.

**Why this priority**: Session continuity is critical for multi-day workflows. The launcher is the natural place to surface session history rather than requiring CLI flags like `--attach`.

**Independent Test**: Create a session with workers, exit kasmos, relaunch, press `r`, verify the last session appears at top with archived sessions below. Select one and verify the dashboard loads with restored state.

**Acceptance Scenarios**:

1. **Given** the user presses `r` on the launcher, **When** a previous session exists in `.kasmos/session.json`, **Then** the restore picker shows the last active session prominently at the top.
2. **Given** archived sessions exist in `.kasmos/sessions/`, **When** the restore picker opens, **Then** archived sessions are listed below the last active session, sorted by recency.
3. **Given** the user selects a session from the restore picker, **When** they confirm, **Then** the worker dashboard loads with that session's workers, states, and task source restored.
4. **Given** no previous sessions exist, **When** the user presses `r`, **Then** a message indicates no sessions are available and the launcher remains visible.

---

### User Story 3 - Settings: Per-Agent Model Configuration (Priority: P2)

A developer wants their planner agents to use a high-reasoning model while coder agents use a faster model. They press `s` on the launcher, navigate to agent configuration, and set model and reasoning level per role. These settings persist across sessions.

**Why this priority**: Different agent roles benefit from different model configurations. Planners need deep reasoning; coders need speed. Without per-role config, users must manually override settings for each spawn.

**Independent Test**: Open settings, change the model for the planner role, exit settings, spawn a planner worker, verify it uses the configured model. Restart kasmos and verify the setting persists.

**Acceptance Scenarios**:

1. **Given** the user presses `s` on the launcher, **When** the settings view opens, **Then** it displays configurable agent roles (planner, coder, reviewer, release) with their current model and reasoning level.
2. **Given** the user selects a role, **When** they change the model or reasoning level, **Then** the change is reflected immediately in the settings display.
3. **Given** the user exits the settings view, **When** the settings are saved, **Then** a configuration file persists the settings in `.kasmos/config.json` or equivalent.
4. **Given** per-role settings are configured, **When** a worker is spawned with that role, **Then** the worker uses the configured model and reasoning level.
5. **Given** no custom settings exist, **When** the settings view opens, **Then** sensible defaults are shown for each role (system default model, default reasoning level).

---

### User Story 4 - Launch Spec-Kitty Flows (Priority: P2)

A developer wants to start a new feature. From the launcher, they press `f` to create a feature spec. Alternatively, they press `p` to run the planning phase on an existing feature. Both actions transition out of the launcher into the appropriate workflow.

**Why this priority**: Feature planning is a core kasmos workflow. The launcher provides a discoverable entry point rather than requiring users to know about `n` → picker → spec-kitty.

**Independent Test**: Press `f` on the launcher, verify spec-kitty feature creation begins. Press `p`, verify a feature picker appears (if multiple features exist) followed by the plan flow.

**Acceptance Scenarios**:

1. **Given** the user presses `f` on the launcher, **When** spec-kitty is available, **Then** the feature creation flow begins (equivalent to spec-kitty create-feature).
2. **Given** the user presses `p` on the launcher, **When** one or more features exist in `kitty-specs/`, **Then** a picker shows available features, and selecting one starts the plan phase.
3. **Given** the user presses `p` on the launcher, **When** no features exist in `kitty-specs/`, **Then** a message indicates no features are available and suggests creating one with `f`.
4. **Given** spec-kitty is not installed, **When** the user presses `f` or `p`, **Then** an error message indicates spec-kitty is required and the launcher remains visible.

---

### User Story 5 - Launcher with CLI Arguments Bypassed (Priority: P3)

A developer runs `kasmos kitty-specs/016-kasmos-agent-orchestrator` with an explicit task source argument. The launcher is skipped entirely, and kasmos opens directly into the worker dashboard with the specified task source loaded.

**Why this priority**: Power users and scripts that pass explicit arguments should not be interrupted by the launcher. The launcher is for the "what do I want to do?" moment, not when the user already knows.

**Independent Test**: Run `kasmos <path>` and verify the dashboard opens directly without showing the launcher. Run `kasmos --attach` and verify the session restores directly.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos <task-source-path>`, **When** the TUI starts, **Then** the worker dashboard opens directly with the specified task source loaded. The launcher is not shown.
2. **Given** the user runs `kasmos --attach`, **When** the TUI starts, **Then** the session restores directly into the dashboard. The launcher is not shown.
3. **Given** the user runs `kasmos --daemon`, **When** the process starts, **Then** headless mode runs without the launcher.

---

### Edge Cases

- What happens when the terminal is too small to render the launcher? Show the existing "terminal too small" message with minimum dimensions.
- What happens when the user resizes the terminal while on the launcher? The launcher recenters and reflows to the new dimensions.
- What happens when `spec-kitty` binary is not found? The `f` and `p` menu items show but display an error message when selected, directing the user to install spec-kitty.
- What happens when `.kasmos/config.json` is corrupt? Settings loads with defaults and warns the user that the config was reset.

## Requirements

### Functional Requirements

- **FR-001**: System MUST display a centered launcher screen with ASCII art branding when invoked with no arguments.
- **FR-002**: System MUST present 7 menu items on the launcher: new task (n), create feature spec (f), create plan (p), view history (h), restore session (r), settings (s), quit (q).
- **FR-003**: Each menu item MUST respond to its single-key shortcut without requiring Enter.
- **FR-004**: System MUST skip the launcher and open the dashboard directly when CLI arguments or flags are provided (`<path>`, `--attach`, `--daemon`).
- **FR-005**: The restore session picker MUST display the last active session prominently, with archived sessions listed below in reverse chronological order.
- **FR-006**: The settings view MUST allow configuration of model name and reasoning level for each agent role (planner, coder, reviewer, release).
- **FR-007**: Settings MUST persist to disk and survive kasmos restarts.
- **FR-008**: System MUST handle missing external tools gracefully (spec-kitty not installed, no sessions to restore) with informative messages rather than crashes.
- **FR-009**: The launcher MUST respect the minimum terminal size requirement and display the "terminal too small" fallback when dimensions are insufficient.
- **FR-010**: Selecting a launcher action MUST transition the user to the appropriate view or dialog. Pressing Esc from any sub-view MUST return to the launcher.

### Key Entities

- **LauncherAction**: A selectable menu item with a key, label, description, and handler that determines what happens when activated.
- **AgentSettings**: Per-role configuration containing model name and reasoning level. Stored in `.kasmos/config.json`.
- **SessionEntry**: A restorable session with ID, timestamp, worker count, task source type, and path. Sourced from `.kasmos/session.json` (active) and `.kasmos/sessions/*.json` (archived).

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can navigate from launcher to any workflow in a single keypress (under 1 second).
- **SC-002**: Session restore from the launcher loads a previous session's full state (workers, tasks, source) within 2 seconds.
- **SC-003**: Settings changes persist across kasmos restarts with zero data loss.
- **SC-004**: Users who have never used kasmos can discover all available actions within 10 seconds of launching the application.
- **SC-005**: The launcher renders correctly on terminals from 80x24 (minimum) to arbitrarily large sizes.

## Assumptions

- The ASCII art branding will be a stylized "kasmos" text, similar in spirit to LazyVim's block-letter logo but using kasmos's existing gradient color palette (hot pink → purple).
- The launcher menu layout follows LazyVim's pattern: icon + label left-aligned, key hint right-aligned, vertically centered in the terminal.
- Settings configuration uses a simple key-value model (role → model + reasoning level), not a complex nested configuration system.
- The `f` (create feature spec) action will shell out to `spec-kitty` or use kasmos's existing spec creation integration. The exact mechanism is an implementation detail.
- The `p` (create plan) action requires an existing feature spec to operate on. If multiple features exist, a picker is shown.
