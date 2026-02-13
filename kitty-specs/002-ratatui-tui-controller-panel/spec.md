# Feature Specification: Ratatui TUI Controller Panel

**Feature Branch**: `002-ratatui-tui-controller-panel`
**Created**: 2026-02-10
**Status**: Draft
**Input**: User description: "Convert main panel of the app to be a TUI with ratatui that provides action buttons for managing WPs, progressing through work, addressing issues/input requests and displays a status based on the kanboard + what's available."

## Clarifications

### Session 2026-02-10

- Q: What triggers the "input needed" notification type? → A: Agent in a pane explicitly signals it is blocked waiting for operator input (e.g., writes a marker file or sends a signal).
- Q: What distinguishes "request changes" from "reject" in the review workflow? → A: "Request changes" keeps the WP in `for_review` without relaunching — the operator manually edits files, then re-triggers review. "Reject" sends the WP back to `doing` for agent rework.
- Q: When an "input needed" notification is activated, what does the operator do? → A: The TUI displays the agent's question/message, then the operator focuses/zooms the agent's Zellij pane to interact with it directly. The TUI surfaces the alert; the operator handles it in the agent pane.
- Q: When a WP is rejected (sent back to doing), is agent relaunch automatic or manual? → A: Configurable — defaults to auto-relaunch but the operator can choose "reject and hold" (manual restart) vs "reject and relaunch" (immediate re-execution).

### Session 2026-02-11

- Q: How should automated review be triggered at `for_review`? → A: kasmos should support a tiered review runner that can auto-trigger either a slash command workflow (`/kas:verify` / `/kas:review`) or a built-in prompt workflow.
- Q: Must automated review be tied to Claude-specific tooling? → A: No. The workflow must be model-agnostic and runnable through opencode; default model should be `openai/gpt-5.3-codex` with high reasoning when no model is configured.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - View Orchestration Dashboard (Priority: P1)

An operator launches kasmos and sees a real-time dashboard in the controller pane showing all work packages organized by their kanban lane (planned, doing, for_review, done). The dashboard updates live as WPs transition between states. The operator can see at a glance which WPs are active, which are blocked, and overall orchestration progress.

**Why this priority**: Without a visual dashboard, the operator has no situational awareness. This is the foundational view everything else builds on.

**Independent Test**: Can be fully tested by launching kasmos with a known set of WPs and verifying the dashboard renders all WPs in their correct lanes, updates when state changes, and displays wave/dependency information.

**Acceptance Scenarios**:

1. **Given** an orchestration run is active with WPs in multiple states, **When** the operator views the Dashboard tab, **Then** all WPs are displayed grouped by kanban lane with their current state, wave assignment, and elapsed time.
2. **Given** a WP transitions from `doing` to `for_review`, **When** the dashboard is visible, **Then** the WP moves to the new lane within 1 second without requiring manual refresh.
3. **Given** the operator has no orchestration run loaded, **When** they open the TUI, **Then** the Dashboard shows `No active orchestration run` and actionable guidance `Run: kasmos launch <feature_dir>`.

---

### User Story 2 - Manage Work Packages via Action Buttons (Priority: P1)

An operator uses keyboard shortcuts or mouse clicks on action buttons to control work packages: restart failed WPs, pause/resume active ones, retry from scratch, force-advance past blockers, and advance to the next wave (in wave-gated mode). Actions are contextual — only valid actions for a WP's current state are available.

**Why this priority**: Direct WP control is the core interactive capability. Without it, the TUI is a read-only display.

**Independent Test**: Can be tested by placing WPs in various states (active, failed, paused) and verifying that the correct action buttons appear and produce the expected state transitions when activated.

**Acceptance Scenarios**:

1. **Given** a WP is in the `Failed` state, **When** the operator selects it, **Then** only valid actions (Restart, Retry, Force-Advance) are available as buttons.
2. **Given** the operator clicks "Restart" on a failed WP, **When** the action is dispatched, **Then** the WP transitions to the appropriate state and the dashboard reflects the change.
3. **Given** the orchestration is in wave-gated mode at a wave boundary, **When** the operator activates "Advance", **Then** the next wave begins execution.
4. **Given** a WP is actively running, **When** the operator selects it, **Then** Pause is available but Restart/Retry are not.

---

### User Story 3 - Review Work Packages at For-Review Gate (Priority: P1)

When a WP reaches the `for_review` lane, the orchestrator pauses it and surfaces it to the operator in a dedicated Review tab. A tiered automated review is triggered for that WP (slash-command mode or prompt mode), and results are displayed as context. The operator has three actions: approve (move to done), reject (send back to doing for agent rework — configurable auto-relaunch or manual hold), or request changes (keep in for_review while operator manually edits files, then re-trigger review). This is the human-in-the-loop gate for quality control.

**Why this priority**: The auto-run-then-pause-for-review loop is the central workflow the user described. It ensures quality while maintaining automation speed.

**Independent Test**: Can be tested by manually transitioning a WP to `for_review` and verifying the Review tab populates with the WP details, review results are displayed, and approve/reject actions correctly transition the WP.

**Acceptance Scenarios**:

1. **Given** a WP transitions to `for_review`, **When** the operator navigates to the Review tab, **Then** the WP is listed with its title, summary of changes, and any automated review feedback.
2. **Given** the operator approves a WP in review, **When** approval is confirmed, **Then** the WP moves to `done` and dependent WPs become eligible for execution.
3. **Given** the operator rejects a WP with auto-relaunch enabled, **When** rejection is confirmed, **Then** the WP returns to `doing` and the orchestrator automatically queues it for re-execution by a new agent instance.
4. **Given** the operator rejects a WP with "reject and hold", **When** rejection is confirmed, **Then** the WP returns to the `doing` lane with runtime state `Paused` and remains idle until the operator explicitly resumes or restarts it.
5. **Given** the operator selects "request changes" on a WP, **When** the action is confirmed, **Then** the WP stays in `for_review`, the operator can manually edit files, and a "re-review" action becomes available to re-trigger the review cycle.
6. **Given** a WP is in `for_review` with "request changes" selected and manual edits completed, **When** the operator triggers "re-review", **Then** automated review is re-run, updated review feedback is displayed, and the WP remains in `for_review`.
7. **Given** multiple WPs are awaiting review simultaneously, **When** the operator views the Review tab, **Then** all pending reviews are listed and individually actionable.
8. **Given** automated review trigger mode is `slash`, **When** a WP enters `for_review`, **Then** kasmos injects the configured slash command (default `/kas:verify`) into the reviewer pane for that WP.
9. **Given** automated review trigger mode is `prompt`, **When** a WP enters `for_review`, **Then** kasmos launches a reviewer agent via opencode using a tiered review prompt with default model `openai/gpt-5.3-codex` and high reasoning unless overridden.
10. **Given** a slash command is unavailable or fails to execute, **When** review automation runs, **Then** kasmos records the failure, surfaces it in Notifications/Logs, and falls back to prompt-mode review if enabled.

---

### User Story 4 - Persistent Notification Bar for Attention Items (Priority: P2)

A persistent notification bar is visible across all tabs, showing counts and identifiers for WPs that need operator attention (e.g., "2 awaiting review", "1 failed"). The bar highlights or pulses when new items arrive. A keybinding allows jumping directly from the notification to the relevant WP or tab.

**Why this priority**: Without cross-tab notifications, the operator might miss time-sensitive events (reviews waiting, failures) while viewing a different tab.

**Independent Test**: Can be tested by triggering state changes (WP failure, WP reaching review) while on a different tab and verifying the notification bar updates with counts and allows direct navigation.

**Acceptance Scenarios**:

1. **Given** the operator is on the Logs tab and a WP enters `for_review`, **When** the notification bar updates, **Then** it shows an incremented review count and the WP identifier.
2. **Given** the notification bar shows "WP03: awaiting review", **When** the operator activates the jump-to keybinding, **Then** the TUI switches to the Review tab with WP03 focused.
3. **Given** all attention items are resolved, **When** the operator views the notification bar, **Then** it shows a clean/idle state indicator.
4. **Given** a WP fails, **When** the notification bar updates, **Then** it visually distinguishes failures from reviews (different styling or icon).

---

### User Story 5 - Tab-Based Navigation (Priority: P2)

The operator switches between multiple views (Dashboard, Review, Logs) using keyboard shortcuts (e.g., number keys 1/2/3 or bracket keys) or mouse clicks on tab headers. Each tab retains its scroll position and selection state when switching away and back.

**Why this priority**: Tab navigation is the structural framework that organizes the other features into a usable interface.

**Independent Test**: Can be tested by switching between tabs via keyboard and mouse, verifying each tab renders its content and preserves state across switches.

**Acceptance Scenarios**:

1. **Given** the operator is on the Dashboard tab, **When** they press the keybinding for the Logs tab, **Then** the Logs tab renders immediately.
2. **Given** the operator scrolls down in the Logs tab and switches to Dashboard then back, **When** the Logs tab re-renders, **Then** the scroll position is preserved.
3. **Given** the operator clicks on a tab header with the mouse, **When** the click is registered, **Then** the corresponding tab activates.

---

### User Story 6 - Dual Input: TUI + FIFO Compatibility (Priority: P2)

The existing FIFO command pipe remains functional alongside the TUI. Commands sent via the FIFO are processed identically to TUI actions, and the TUI reflects state changes triggered by FIFO commands in real-time. This preserves scriptability and external tool integration.

**Why this priority**: Retaining the FIFO ensures backward compatibility and allows automation scripts and spec-kitty to interact programmatically.

**Independent Test**: Can be tested by sending commands through the FIFO pipe while the TUI is running and verifying the TUI state updates accordingly.

**Acceptance Scenarios**:

1. **Given** the TUI is running, **When** a `status` command is sent via the FIFO, **Then** the command is processed and the result is logged (not rendered over the TUI).
2. **Given** an external script sends `restart WP03` via FIFO, **When** the command executes, **Then** the TUI dashboard shows WP03 transitioning to its new state.
3. **Given** both FIFO and TUI are active, **When** commands arrive from both sources simultaneously, **Then** all commands are processed without data races or corruption.

---

### User Story 7 - Orchestration Log Viewer (Priority: P3)

The operator views a scrollable, filterable log of orchestration events in the Logs tab. Events include WP state transitions, wave advancements, command executions, errors, and review outcomes. The log supports scrolling and basic text filtering.

**Why this priority**: Logs provide essential debugging and audit trail information but are not needed for primary orchestration control.

**Independent Test**: Can be tested by running an orchestration and verifying log entries appear for each significant event, and that scrolling and filtering work correctly.

**Acceptance Scenarios**:

1. **Given** an orchestration is running, **When** the operator opens the Logs tab, **Then** recent events are displayed in chronological order with timestamps.
2. **Given** the log has many entries, **When** the operator scrolls, **Then** older entries are accessible without performance degradation.
3. **Given** the operator enters a filter term, **When** the filter is applied, **Then** only matching log entries are displayed.

---

### User Story 8 - Respond to Agent Input Requests (Priority: P2)

When an agent signals it is blocked and needs operator input, the TUI surfaces a notification with the agent's question or message. The operator activates the notification, views the agent's message in the TUI, and then focuses/zooms the agent's Zellij pane to interact directly. Once the operator has provided input in the agent pane, the agent resumes and the notification clears.

**Why this priority**: Input-needed signals are a key differentiator from simple failure notifications. They enable agents to ask for guidance mid-task rather than failing outright.

**Independent Test**: Can be tested by having an agent write an input-request signal, verifying the TUI displays the notification with the message, and confirming the focus/zoom action navigates to the correct agent pane.

**Acceptance Scenarios**:

1. **Given** an agent signals it needs input, **When** the notification bar updates, **Then** an "input needed" notification appears with the WP identifier and the agent's message summary.
2. **Given** the operator activates an input-needed notification, **When** the TUI processes the action, **Then** the agent's Zellij pane is focused/zoomed so the operator can interact directly.
3. **Given** the operator has provided input and the agent resumes work, **When** the agent clears its input-needed signal, **Then** the notification is removed from the bar.

---

### Edge Cases

- What happens when the terminal is resized while the TUI is running? The layout must reflow gracefully.
- How does the TUI handle an extremely large number of WPs (e.g., 50+)? The dashboard must remain scrollable and responsive.
- What happens if the orchestration run terminates unexpectedly while the TUI is active? The TUI must render `Run terminated` with the last known run state and a `Press q to exit` action.
- What happens when the FIFO receives malformed input while the TUI is running? The error must be logged but not crash the TUI.
- What if the terminal does not support mouse input? Keyboard-only operation must remain fully functional.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST render an interactive terminal interface using ratatui within the Zellij controller pane.
- **FR-002**: System MUST provide a tab-based layout with at minimum three views: Dashboard, Review, and Logs.
- **FR-003**: System MUST display all work packages grouped by kanban lane (planned, doing, for_review, done) in the Dashboard view.
- **FR-004**: System MUST update the display in real-time (within 1 second) when WP state changes occur.
- **FR-005**: System MUST provide contextual action buttons for each WP based on its current state (restart, pause, resume, retry, force-advance).
- **FR-006**: System MUST support both keyboard navigation (vim-style `h/j/k/l`, tab switching `1/2/3`, actions `R/P/F/T`, notifications `n`, logs filter `/`, quit `Alt+q`) and mouse input (click, scroll).
- **FR-007**: System MUST display a persistent notification bar across all tabs showing WPs requiring operator attention, with counts and identifiers.
- **FR-008**: System MUST visually distinguish between three attention types in the notification bar: reviews pending, failures, and input needed (agent-signaled blocks).
- **FR-009**: System MUST provide a keybinding to jump from a notification directly to the relevant WP or review item.
- **FR-010**: System MUST automatically pause WPs that reach `for_review` and surface them in the Review tab with automated review context.
- **FR-011**: System MUST allow the operator to approve, reject (with configurable auto-relaunch or hold), or request changes (keep in for_review for manual edits) on WPs in the Review tab.
- **FR-012**: System MUST retain the existing FIFO command pipe as a secondary input, processing FIFO commands identically to TUI actions so the same process is runnable without the TUI.
- **FR-013**: System MUST reflect FIFO-triggered state changes in the TUI display in real-time.
- **FR-014**: System MUST provide a scrollable, filterable log view of orchestration events in the Logs tab.
- **FR-015**: System MUST gracefully handle terminal resize events by reflowing the layout.
- **FR-016**: System MUST remain fully functional via keyboard alone when mouse input is unavailable.
- **FR-017**: System MUST detect agent input-needed signals (agent-written markers) and surface them as "input needed" notifications with the agent's message.
- **FR-018**: System MUST provide an action on input-needed notifications that focuses/zooms the agent's Zellij pane for direct operator interaction.
- **FR-019**: System MUST clear input-needed notifications automatically when the agent resumes work (clears its signal).
- **FR-020**: System MUST provide a "re-review" action for WPs in `for_review` after "request changes"; this action MUST re-run automated review for that WP, refresh review context in place, and keep the WP in `for_review` until approve/reject.
- **FR-021**: System MUST support automated tiered review trigger modes: `slash` (inject configurable command, default `/kas:verify`) and `prompt` (run built-in review prompt).
- **FR-022**: System MUST support reviewer execution through opencode with model-agnostic configuration and default to model `openai/gpt-5.3-codex` and high reasoning.
- **FR-023**: System MUST persist per-WP review automation results (status, findings summary, command/mode used, timestamp) so Review tab and status/report outputs remain consistent across restarts.
- **FR-024**: System MUST allow configurable automation policy for `for_review` handling: `manual_only`, `auto_then_manual_approve` (default), or `auto_and_mark_done`.
- **FR-025**: System MUST surface automated review failures (command missing, timeout, non-zero exit, parser error) as typed Notification and LogEntry records containing `wp_id`, `failure_type`, `message`, and `timestamp`; each failure MUST appear in both Notifications and Logs within 1 second and remain visible until dismissed/resolved.
- **FR-026**: System MUST provide a global "Advance Wave" action when in wave-gated mode and paused at a wave boundary.

### Non-Functional Requirements

- **NFR-001 (Latency)**: Under a 50-WP synthetic load, input latency MUST remain p95 <= 100ms and p99 <= 150ms.
- **NFR-002 (Propagation)**: State propagation MUST be source-agnostic (TUI, FIFO, detector, review automation) and preserve event order for each WP.
- **NFR-003 (Resilience)**: The TUI MUST remain keyboard-operable when mouse input is unavailable, with no loss of control paths.
- **NFR-004 (Runtime Safety)**: Render/input handlers on the tokio event loop MUST complete in <= 5ms p95 and <= 10ms p99 under the 50-WP synthetic load, with no single handler execution exceeding 25ms.

### Key Entities

- **Tab**: A named view within the TUI (Dashboard, Review, Logs). Each tab has its own rendering logic, scroll state, and selection state.
- **Notification**: An attention item surfaced in the persistent notification bar. Has `type`, `wp_id`, and `timestamp`; optional `message`; and for failure notifications includes `failure_type` (`command_missing|timeout|non_zero_exit|parser_error`) and `severity`.
- **LogEntry**: A structured orchestration event record with `timestamp`, `level`, optional `wp_id`, `message`, and optional `failure_type` for review-automation failures.
- **ReviewItem**: A work package awaiting operator approval in the Review tab. Contains the WP details, automated review results, and available actions: approve (move to done), reject (back to doing — auto-relaunch or hold per configuration), request changes (stay in for_review for manual edits, with re-review trigger).
- **ActionButton**: A contextual control rendered for a selected WP. Determined by the WP's current state and the orchestration mode.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can identify the state of any work package within 2 seconds of opening the TUI.
- **SC-002**: All WP management actions (restart, pause, approve, reject) are completable within 3 keystrokes or a single mouse click from the relevant view.
- **SC-003**: State changes from any source (TUI action, FIFO command, automatic detection) meet the FR-004 display update SLA.
- **SC-004**: In an audited run, 100% of emitted attention events (review pending, failure, input needed) are surfaced in the notification bar within 1 second and persist until resolved (no dropped event IDs).
- **SC-005**: With 50 concurrent WPs in a synthetic load scenario, input latency is p95 <= 100ms and p99 <= 150ms over a continuous 5-minute run.
- **SC-006**: 100% of existing FIFO commands continue to function identically when the TUI is active.
- **SC-007**: The review workflow (WP reaches for_review → operator reviews → approve/reject) completes without leaving the TUI.
- **SC-008**: Operators can switch between any two tabs in under 1 second via keyboard or mouse.
- **SC-009**: For 95% of WPs entering `for_review`, automated review starts within 3 seconds and records an actionable result (pass/fail/error) within configured timeout.

## Assumptions

- The TUI renders inside the existing Zellij controller pane; the Zellij session/layout management is unchanged.
- ratatui with crossterm backend is the rendering stack (consistent with the Rust ecosystem already in use).
- The existing `mpsc` channel architecture for `EngineAction` dispatch is reused; the TUI becomes an additional producer.
- Terminal supports at minimum 256 colors and Unicode box-drawing characters (standard for modern terminals).
- Review automation can execute either slash-command workflows (if plugin/command available in pane) or built-in prompt mode; slash availability is environment-dependent.
