# Feature Specification: Tmux Worker Mode

**Feature Branch**: `019-tmux-worker-mode`
**Created**: 2026-02-18
**Status**: Draft
**Input**: Add a tmux-backed worker mode so users can interact directly with agent sessions when workflows require back-and-forth (planning, clarification, review discussions), while maintaining full orchestration visibility in the kasmos dashboard.

## Clarifications

### Session 2026-02-18

- Q: Should AI helpers (failure analysis, prompt generation) work in tmux mode? -> A: Out of scope for tmux mode MVP. They remain available in subprocess mode only. Can be extended later.
- Q: When reattaching (`--attach`), how is tmux mode determined? -> A: Inferred from session metadata. The session file records whether the session used tmux mode; reattach automatically selects the correct backend.
- Q: Are daemon mode (`-d`) and tmux mode (`--tmux`) mutually exclusive? -> A: Yes, mutually exclusive. Providing both flags produces a clear error.

## User Scenarios & Testing

### User Story 1 - Enable Tmux Mode (Priority: P1)

A developer wants interactive access to their agent sessions. They start kasmos with the `--tmux` flag (or have it configured as their default in settings). kasmos validates that it is running inside a tmux session, then sets up a split layout: the kasmos dashboard on the left, and a worker pane area on the right where the output viewport normally appears.

**Why this priority**: This is the activation path for the entire feature. Without reliable detection, validation, and layout setup, no other tmux functionality works.

**Independent Test**: Run `kasmos --tmux` inside a tmux session, verify the dashboard renders on the left and the right column is reserved for worker panes. Run `kasmos --tmux` outside tmux, verify a clear error message.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos --tmux` inside a tmux session, **When** the TUI starts, **Then** the dashboard renders in the left portion and the right column is prepared for interactive worker panes (replacing the standard output viewport).
2. **Given** the user runs `kasmos --tmux` outside a tmux session, **When** the TUI attempts to start, **Then** a clear error message explains that tmux mode requires running inside a tmux session, with guidance on how to start one.
3. **Given** the user has configured tmux mode as default in settings (feature 017), **When** they run `kasmos` with no flags inside tmux, **Then** tmux mode activates automatically.
4. **Given** the user has configured tmux mode as default but is not inside tmux, **When** they run `kasmos`, **Then** kasmos falls back to standard subprocess mode and displays a notice.

---

### User Story 2 - Spawn Interactive Worker (Priority: P1)

A developer presses the spawn key in the kasmos dashboard and creates a new worker. Instead of running as a headless subprocess with captured output, the worker appears as a live terminal in the right-side pane area. The developer can see the agent's full terminal output in real time, exactly as if they had run the agent command directly.

**Why this priority**: Interactive worker spawning is the core differentiator of tmux mode. The headless subprocess captures output but strips away the terminal experience. Interactive panes restore that while keeping orchestration intact.

**Independent Test**: In tmux mode, spawn a worker from the dashboard. Verify the worker's terminal appears in the right pane area. Verify the worker's status updates in the dashboard table (running, then exited with code).

**Acceptance Scenarios**:

1. **Given** kasmos is running in tmux mode, **When** the user spawns a worker, **Then** the worker's live terminal appears in the right-side pane area, replacing the standard output viewport.
2. **Given** a worker is spawned in tmux mode, **When** the worker produces output, **Then** the output appears in real time in the worker's terminal pane, with full terminal formatting (colors, cursor movement, interactive prompts).
3. **Given** a worker is spawned in tmux mode, **When** the worker's status changes (running, exited, failed), **Then** the kasmos dashboard table reflects the updated status.
4. **Given** no workers are spawned yet, **When** the user is in tmux mode, **Then** the right pane area shows a placeholder or empty state indicating no active worker.

---

### User Story 3 - Switch Active Worker (Priority: P1)

A developer has multiple workers running. They navigate the worker table in the kasmos dashboard and select a different worker. The right-side pane swaps to show the newly selected worker's terminal. Only one worker pane is visible at a time; all other workers continue running in the background.

**Why this priority**: Multi-worker orchestration is kasmos's purpose. Users must be able to switch between workers without losing any worker's state or interrupting background workers.

**Independent Test**: Spawn 3 workers in tmux mode. Select each one in the dashboard table. Verify the right pane swaps to show the selected worker's terminal each time. Verify non-selected workers remain running.

**Acceptance Scenarios**:

1. **Given** multiple workers are running in tmux mode, **When** the user selects a different worker in the dashboard table, **Then** the right-side pane swaps to display the selected worker's terminal.
2. **Given** the user switches from worker A to worker B, **When** worker A was mid-output, **Then** worker A continues running in the background and its output is not lost.
3. **Given** the user switches to a completed worker, **When** the pane displays, **Then** the worker's final terminal state is visible (scrollable history of the completed session).
4. **Given** the user switches between workers rapidly, **When** the pane swap occurs, **Then** the transition completes without visual artifacts or delay perceivable by the user.

---

### User Story 4 - Direct Interaction with Worker (Priority: P1)

A developer selects a running worker (e.g., a planner agent that is asking clarifying questions). The worker's terminal pane is displayed and focus automatically moves to that pane. The developer types responses directly into the agent's terminal, engaging in a real-time conversation. This is the core value of tmux mode: direct, interactive agent communication during workflows that need human input.

**Why this priority**: This is the reason tmux mode exists. Headless subprocess mode cannot support interactive workflows. Planning, clarification, and review discussions all benefit from direct terminal interaction.

**Independent Test**: Spawn a worker in tmux mode that requires user input. Select it in the dashboard. Verify focus moves to the worker pane and keyboard input goes to the worker's terminal. Type a response, verify the agent receives and processes it.

**Acceptance Scenarios**:

1. **Given** a worker is selected in the dashboard, **When** the worker's pane is displayed, **Then** keyboard focus automatically moves to the worker's terminal pane.
2. **Given** focus is on the worker pane, **When** the user types, **Then** keystrokes go to the worker's terminal process (the agent receives user input).
3. **Given** the user is interacting with a planner agent, **When** the agent asks a question and the user types a response, **Then** the agent processes the response and continues its workflow.
4. **Given** focus is on the worker pane, **When** the user needs to return to the kasmos dashboard, **Then** standard terminal multiplexer navigation transfers focus back to the dashboard pane.

---

### User Story 5 - Return to Dashboard (Priority: P2)

A developer finishes interacting with a worker and wants to return to the kasmos dashboard to check other workers or spawn new ones. They use standard terminal multiplexer navigation to move focus back to the dashboard pane. Alternatively, when a worker exits, focus automatically returns to the dashboard.

**Why this priority**: Seamless round-trip between dashboard and worker panes is essential for the orchestration workflow. The user should never feel "stuck" in a worker pane.

**Independent Test**: Focus on a worker pane, use terminal multiplexer navigation to return to the dashboard, verify kasmos responds to keystrokes normally. Let a focused worker exit, verify focus returns to the dashboard automatically.

**Acceptance Scenarios**:

1. **Given** focus is on a worker pane, **When** the user navigates back to the dashboard pane using terminal multiplexer keybindings, **Then** the kasmos dashboard regains focus and responds to navigation keys normally.
2. **Given** focus is on a worker pane, **When** the focused worker's process exits, **Then** focus automatically returns to the kasmos dashboard pane.
3. **Given** focus returns to the dashboard after a worker exits, **When** the dashboard updates, **Then** the exited worker's status is immediately reflected in the table (exited/failed with exit code).
4. **Given** the user returns to the dashboard, **When** they select a different worker, **Then** the right pane swaps and focus moves to the newly selected worker's pane.

---

### User Story 6 - Worker Survival Across Restarts (Priority: P2)

A developer's kasmos process crashes, is killed, or they intentionally exit. Their workers continue running because they are managed by the terminal multiplexer, not as direct child processes of kasmos. When the developer restarts kasmos with `--attach`, it rediscovers the surviving worker panes and resumes orchestration -- reconnecting the dashboard to the still-running workers.

**Why this priority**: Long-running agent sessions (multi-hour planning, large codebases) should not be lost due to kasmos instability. Terminal multiplexer-backed workers provide natural crash resilience.

**Independent Test**: Start kasmos in tmux mode, spawn 2 workers. Kill the kasmos process. Verify workers continue running in their panes. Start kasmos again with --attach, verify it reconnects to the running worker panes and shows accurate status.

**Acceptance Scenarios**:

1. **Given** kasmos is killed while workers are running in tmux mode, **When** the kasmos process terminates, **Then** worker terminal panes continue running independently.
2. **Given** workers survived a kasmos crash, **When** the user restarts kasmos with `--attach`, **Then** kasmos infers tmux mode from the saved session metadata, reconnects to surviving worker panes, and displays their current status without requiring the `--tmux` flag.
3. **Given** a worker completed while kasmos was down, **When** kasmos reattaches, **Then** the completed worker's final status and terminal history are available.
4. **Given** the terminal multiplexer session itself was terminated, **When** the user starts kasmos fresh, **Then** standard orphan recovery applies (no panes to reconnect to, clean start).

---

### Edge Cases

- What happens when tmux is not installed? The `--tmux` flag produces a clear error with installation instructions. Settings-based tmux default is ignored with a warning.
- What happens when a worker pane is manually closed by the user (e.g., via terminal multiplexer kill-pane)? kasmos detects the pane is gone and marks the worker as killed in the dashboard.
- What happens when the terminal is too narrow for the split layout? kasmos shows the dashboard at full width. When a worker is selected, the dashboard hides and the worker pane takes full width. A keybinding toggles between them (similar to existing fullscreen toggle behavior).
- What happens when the user spawns many workers? All workers run in hidden panes. Only the selected worker's pane is visible. No pane count limit beyond system resources.
- What happens when kasmos restarts but the terminal multiplexer naming convention changed? kasmos uses a consistent pane naming/tagging scheme to identify its managed panes. Unrecognized panes are ignored.
- What happens when subprocess-mode workers and tmux-mode are mixed? In tmux mode, all workers use the tmux backend. Mixed mode within a single session is not supported.
- What happens when the user scrolls in a worker pane? Standard terminal multiplexer scrollback applies. The user can scroll through the worker's output history using terminal multiplexer scroll mode.
- What happens when the user presses AI helper keys (analyze failure, gen-prompt) in tmux mode? These features are not available in tmux mode as they depend on captured output from subprocess workers. The keybindings are disabled when tmux mode is active.
- What happens when the user passes both `--tmux` and `-d` (daemon mode)? The flags are mutually exclusive. kasmos produces a clear error explaining that tmux mode requires the interactive dashboard and cannot run headless.

## Requirements

### Functional Requirements

- **FR-001**: System MUST support a tmux worker mode activated by the `--tmux` CLI flag.
- **FR-002**: System MUST support configuring tmux mode as the default via the settings view (feature 017 dependency).
- **FR-003**: System MUST validate that it is running inside a terminal multiplexer session when tmux mode is requested, and produce a clear error if not.
- **FR-004**: System MUST fall back to standard subprocess mode when tmux mode is configured as default but the terminal multiplexer environment is unavailable, with a visible notice.
- **FR-005**: System MUST spawn workers as interactive terminal panes instead of headless subprocesses when in tmux mode.
- **FR-006**: System MUST display the selected worker's terminal pane in the right column, replacing the standard output viewport.
- **FR-007**: System MUST show only one worker pane at a time, corresponding to the currently selected worker in the dashboard table.
- **FR-008**: System MUST automatically move keyboard focus to the worker's terminal pane when a worker is selected in the dashboard.
- **FR-009**: System MUST automatically return focus to the dashboard pane when the focused worker's process exits.
- **FR-010**: System MUST keep non-visible worker processes running in hidden panes while a different worker is displayed.
- **FR-011**: System MUST track worker lifecycle status (running, exited, failed, killed) in the dashboard table regardless of which pane is currently visible.
- **FR-012**: Worker processes MUST survive kasmos process termination when running in tmux mode.
- **FR-013**: System MUST reconnect to surviving worker panes when restarting with `--attach`. Tmux mode is inferred from session metadata; the user does not need to pass `--tmux` again.
- **FR-014**: System MUST detect and handle externally killed panes (user manually closes a worker pane) by marking the worker as killed.
- **FR-015**: System MUST implement the existing `WorkerBackend` interface (from feature 016) for the tmux backend, maintaining compatibility with all existing orchestration logic.
- **FR-016**: System MUST reject the combination of `--tmux` and `-d` (daemon mode) flags with a clear error, as they are mutually exclusive.
- **FR-017**: System MUST disable AI helper features (failure analysis, prompt generation) when in tmux mode, as they depend on captured subprocess output. The associated keybindings are hidden.

### Key Entities

- **TmuxBackend**: An implementation of the `WorkerBackend` interface that creates and manages terminal multiplexer panes for workers instead of headless subprocesses. Handles pane creation, visibility toggling, focus management, and process lifecycle monitoring.
- **ManagedPane**: A terminal multiplexer pane created and tracked by kasmos. Key attributes: pane identifier, associated worker ID, visibility state (shown/hidden), process status. Uses a consistent naming/tagging scheme for rediscovery after kasmos restart.
- **TmuxMode Setting**: A persistent configuration option (stored in `.kasmos/config.toml` alongside other settings from feature 017) that determines whether kasmos uses interactive terminal panes or headless subprocesses for workers.
- **Session Backend Metadata**: The session persistence file (`.kasmos/session.json`) records the backend mode (subprocess or tmux) used by the session. On reattach, kasmos reads this to automatically select the correct backend and reconnect to managed panes.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can go from spawning a worker to typing into its interactive terminal in under 3 seconds.
- **SC-002**: Switching the visible worker pane (selecting a different worker in the dashboard) completes in under 1 second.
- **SC-003**: Workers survive kasmos process termination -- restarting kasmos and reattaching reconnects to all surviving workers within 5 seconds.
- **SC-004**: The dashboard reflects accurate worker status within 2 seconds of any state change (spawn, exit, failure, kill).
- **SC-005**: Users can round-trip between the dashboard and a worker pane (focus out and back) with a single key combination in each direction.
- **SC-006**: No worker output or terminal state is lost when switching between workers or when kasmos restarts.

## Assumptions

- The terminal multiplexer (tmux) is installed and available in PATH when tmux mode is used. It is not a hard dependency for kasmos itself -- only for this mode.
- The feature 017 settings view and `.kasmos/config.toml` persistence are available for storing the tmux mode default preference.
- The existing `WorkerBackend` interface from feature 016 is sufficient for the tmux backend without interface changes. If minor additions are needed (e.g., a method for pane visibility), they will be additive and backward-compatible.
- Terminal multiplexer pane naming/tagging is reliable enough for kasmos to identify its managed panes after a restart (tmux supports `set-option` for pane metadata).
- Workers in tmux mode run the same `opencode run` commands as subprocess mode -- the difference is the execution environment (interactive terminal vs captured pipe), not the command itself.
- Users are familiar with basic terminal multiplexer navigation (e.g., prefix + arrow keys) for returning focus to the kasmos pane. kasmos will document the recommended keybindings.
