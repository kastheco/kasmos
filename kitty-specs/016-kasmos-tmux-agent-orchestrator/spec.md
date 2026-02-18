# Feature Specification: kasmos - tmux agent orchestrator

**Feature Branch**: `016-kasmos-tmux-agent-orchestrator`
**Created**: 2026-02-17
**Status**: Draft
**Input**: Fresh Go program replacing the Rust/Zellij kasmos. TUI-based orchestrator for concurrent AI coding agent sessions using bubbletea + tmux.

## User Scenarios & Testing

### User Story 1 - Launch and Spawn Workers (Priority: P1)

A developer runs `kasmos` and sees a dashboard. They spawn multiple AI coding agents to work on tasks in parallel, monitoring progress from the dashboard without switching contexts.

**Why this priority**: This is the core value proposition -- orchestrating concurrent agent sessions from a single interface. Without this, nothing else matters.

**Independent Test**: Run `kasmos`, spawn 3 workers with different prompts, verify all 3 run concurrently and their output appears in the dashboard. Verify workers exit with captured exit codes.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos` in a terminal, **When** the TUI launches, **Then** a dashboard appears showing an empty worker list and available commands.
2. **Given** the dashboard is running, **When** the user presses the spawn key and provides an agent role and prompt, **Then** a worker process starts, appears in the worker list as "running", and its output streams into the dashboard.
3. **Given** 3 workers are running, **When** one worker completes, **Then** its status updates to "exited (0)" with duration, and the other workers continue unaffected.
4. **Given** a worker exits with a non-zero code, **When** the dashboard refreshes, **Then** the worker shows as "failed" with the exit code visible.

---

### User Story 2 - Monitor Worker Output (Priority: P1)

A developer selects a running or completed worker in the dashboard and views its output in a scrollable viewport, allowing them to monitor progress and diagnose issues without leaving the TUI.

**Why this priority**: Visibility into what workers are doing is essential for making orchestration decisions (kill, continue, respawn).

**Independent Test**: Spawn a worker, select it in the dashboard, verify output streams in real time. Scroll through output of a completed worker.

**Acceptance Scenarios**:

1. **Given** a worker is running, **When** the user selects it in the dashboard, **Then** a viewport displays the worker's stdout/stderr output, updating in real time.
2. **Given** a worker has completed, **When** the user selects it, **Then** the full captured output is displayed in a scrollable viewport.
3. **Given** a worker produces thousands of lines of output, **When** the user scrolls the viewport, **Then** navigation is responsive and the buffer retains a configurable number of lines.

---

### User Story 3 - Continue a Worker Session (Priority: P1)

A reviewer agent finishes and reports "verified with suggestions." The developer reads the suggestions in the dashboard, then spawns a follow-up worker that continues the same OpenCode session with full context, directing it to apply specific suggestions.

**Why this priority**: Context continuity between agent sessions is critical for iterative workflows (review -> fix, plan -> implement). Without this, every worker starts from scratch.

**Independent Test**: Spawn a worker, let it complete, press continue, type a follow-up message, verify the new worker runs with the previous session's context.

**Acceptance Scenarios**:

1. **Given** a worker has completed, **When** the user presses the continue key, **Then** the TUI prompts for a follow-up message.
2. **Given** the user types "Apply suggestions 1 and 3" and confirms, **Then** a new worker spawns using `opencode run --continue -s <session_id>` with the follow-up message, preserving the original session's context.
3. **Given** a continuation worker is running, **When** it appears in the dashboard, **Then** it shows a visual link to the parent worker it continued from.

---

### User Story 4 - Kill and Restart Workers (Priority: P2)

A developer notices a worker is going in the wrong direction or is stuck. They kill it from the dashboard and optionally restart it with a modified prompt.

**Why this priority**: Error recovery is essential for practical orchestration. Workers go off-track; the developer needs fast, direct control.

**Independent Test**: Spawn a worker, kill it from the dashboard, verify it stops. Restart it with a different prompt, verify the new worker starts.

**Acceptance Scenarios**:

1. **Given** a running worker is selected, **When** the user presses the kill key, **Then** the worker process is terminated and its status updates to "killed."
2. **Given** a killed or failed worker is selected, **When** the user presses the restart key, **Then** the spawn dialog pre-fills with the original agent role and prompt, allowing edits before re-launching.

---

### User Story 5 - Load Tasks from External Sources (Priority: P2)

A developer starts kasmos with a reference to a task source (spec-kitty plan, GSD task file, or ad-hoc), and the dashboard pre-populates with available work packages that can be assigned to workers.

**Why this priority**: Integrating with existing planning tools avoids duplicate data entry and connects orchestration to the planning pipeline.

**Independent Test**: Run `kasmos kitty-specs/015/plan.md`, verify WPs from the plan appear in the task list. Select a WP and spawn a worker for it.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos kitty-specs/015-feature/`, **When** the TUI launches, **Then** it reads plan.md and displays work packages with their descriptions and dependencies.
2. **Given** the user runs `kasmos tasks.md`, **When** the TUI launches, **Then** it reads the task file and displays tasks as spawnable work items.
3. **Given** the user runs `kasmos` with no arguments, **When** the TUI launches, **Then** it starts in ad-hoc mode with an empty task list. Workers are spawned with manual prompts.
4. **Given** a task source is loaded, **When** the user selects a task and presses spawn, **Then** the spawn dialog pre-fills the agent role and prompt from the task definition.

---

### User Story 6 - Setup and Agent Configuration (Priority: P2)

A developer runs `kasmos setup` and it scaffolds preconfigured OpenCode agent definitions (planner, coder, reviewer, release), AI helper configurations, and validates that required dependencies (opencode, git) are installed.

**Why this priority**: All-in-one setup reduces friction for new users and ensures consistent agent behavior across sessions.

**Independent Test**: Run `kasmos setup` in a project without OpenCode agent configs. Verify agent files are created and opencode recognizes them.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos setup`, **When** the command completes, **Then** `.opencode/agents/` contains agent definitions for planner, coder, reviewer, and release roles.
2. **Given** `kasmos setup` has run, **When** the user spawns a worker with role "planner", **Then** OpenCode uses the planner agent definition (custom prompt, tool permissions, model config).
3. **Given** opencode is not installed, **When** the user runs `kasmos setup`, **Then** it reports the missing dependency with installation instructions.

---

### User Story 7 - Daemon Mode for Headless Operation (Priority: P3)

A developer runs `kasmos -d` (or kasmos detects a non-interactive terminal) and it operates in headless mode -- spawning workers and logging status to stdout without rendering a TUI.

**Why this priority**: Enables CI/CD pipelines and scripted automation without a terminal.

**Independent Test**: Run `kasmos -d --tasks tasks.md --spawn-all`, verify workers spawn and status is logged as structured output. Verify exit code reflects worker results.

**Acceptance Scenarios**:

1. **Given** the user runs `kasmos -d`, **When** workers are spawned, **Then** status updates are logged to stdout as structured text (or JSON with `--format json`).
2. **Given** kasmos is running in daemon mode, **When** all workers complete, **Then** kasmos exits with code 0 if all succeeded, or non-zero if any failed.
3. **Given** kasmos is piped or run in a non-TTY context, **When** it detects no interactive terminal, **Then** it automatically enters daemon mode.

---

### User Story 8 - Session Persistence and Reattach (Priority: P3)

A developer's terminal disconnects or they intentionally close the TUI. Later, they run `kasmos --attach` and reconnect to the running orchestration session, seeing current worker states including any that completed while they were away.

**Why this priority**: Long-running orchestration sessions should survive disconnects. This is critical for remote development and long tasks.

**Independent Test**: Start kasmos, spawn workers, kill the TUI process, run `kasmos --attach`, verify worker states are accurate and completed workers show their results.

**Acceptance Scenarios**:

1. **Given** kasmos is running with active workers, **When** the TUI process is terminated, **Then** the orchestration daemon continues managing workers in the background.
2. **Given** workers completed while the TUI was disconnected, **When** the user runs `kasmos --attach`, **Then** the dashboard shows accurate states for all workers including those that completed.
3. **Given** no running session exists, **When** the user runs `kasmos --attach`, **Then** it reports "no active session" and exits cleanly.

---

### User Story 9 - Remote Access via SSH (Priority: P3)

A developer SSHs into a machine running kasmos and gets the full TUI dashboard, allowing remote monitoring and control of worker sessions.

**Why this priority**: Supports remote development workflows. Not MVP but architecturally important to consider early.

**Independent Test**: Start kasmos with SSH server enabled, connect from another machine via SSH, verify the full TUI is rendered and interactive.

**Acceptance Scenarios**:

1. **Given** kasmos is running with `--ssh` enabled on port 2222, **When** a user connects via `ssh -p 2222 localhost`, **Then** the full kasmos TUI is rendered in their terminal.
2. **Given** a remote user is connected via SSH, **When** they spawn a worker, **Then** the worker runs on the host machine and output is visible to all connected clients.

---

### Edge Cases

- What happens when kasmos is killed while workers are running? Workers are child processes -- they receive SIGTERM. kasmos should attempt graceful shutdown (signal workers, wait briefly, then force-kill). Session state is persisted so the next launch can detect orphaned processes.
- What happens when a worker produces binary or malformed output? The output buffer should handle non-UTF8 gracefully, replacing invalid sequences.
- What happens when the user spawns more workers than the system can handle? kasmos should enforce a configurable max-workers limit and warn when approaching it.
- What happens when OpenCode is not installed? `kasmos setup` validates dependencies. At runtime, spawn failures should produce clear error messages, not panics.
- What happens when the user tries to continue a session that no longer exists? OpenCode returns an error. kasmos should display it and offer to spawn a fresh worker instead.
- What happens during daemon mode if stdout is a pipe that breaks? kasmos should handle SIGPIPE gracefully and continue managing workers.

## Requirements

### Functional Requirements

- **FR-001**: System MUST provide a terminal user interface (TUI) that displays a live dashboard of all workers with status, role, duration, and task association.
- **FR-002**: System MUST spawn workers as child processes running `opencode run` with configurable agent role, prompt, and optional file attachments.
- **FR-003**: System MUST capture worker stdout/stderr and display it in scrollable viewports within the TUI.
- **FR-004**: System MUST track worker lifecycle: spawned, running, exited (with code), killed.
- **FR-005**: System MUST support continuing a completed worker's OpenCode session with a follow-up message, preserving full context.
- **FR-006**: System MUST allow killing running workers and restarting failed/killed workers with editable prompts.
- **FR-007**: System MUST support three task sources: spec-kitty (reads plan.md for WPs), GSD (reads a task file), and ad-hoc (manual prompts).
- **FR-008**: System MUST provide a `kasmos setup` command that scaffolds OpenCode agent definitions and validates dependencies.
- **FR-009**: System MUST support daemon mode (headless, no TUI) activated by `-d` flag or non-interactive terminal detection, using bubbletea's `WithoutRenderer()`.
- **FR-010**: System MUST persist session state (worker history, prompts, task associations, session IDs) to a file that survives TUI restarts.
- **FR-011**: System MUST support reattaching to a running session via `kasmos --attach`.
- **FR-012**: System MUST provide on-demand AI helpers: prompt generation from task context and failure analysis from worker output.
- **FR-013**: System MUST implement a pluggable worker backend interface so the subprocess-based MVP can be extended with alternative backends (e.g., tmux) without rewriting the TUI.
- **FR-014**: System MUST handle graceful shutdown: signal workers on exit, wait for graceful termination, persist final state.
- **FR-015**: System MUST provide batch operations: spawn multiple workers from a task source selection.

### Key Entities

- **Worker**: A managed child process running an OpenCode agent session. Key attributes: ID, agent role, prompt, process state, OpenCode session ID, stdout/stderr buffer, associated task, parent worker (for continuations), spawn time, exit code.
- **Task Source**: A pluggable adapter that reads work items from an external source (spec-kitty plan, GSD file, or ad-hoc). Provides task ID, description, agent role suggestion, and dependency information.
- **Session**: A persistent record of the current kasmos orchestration session including all workers (active and historical), their relationships, and configuration. Serialized to disk for reattach.
- **Worker Backend**: An interface abstracting how worker processes are created and managed. MVP implementation uses Go's os/exec for subprocess management. Future implementations may use tmux or PTY allocation.
- **Agent Definition**: An OpenCode custom agent configuration file (.opencode/agents/*.md) specifying role-specific prompt, model, tools, and permissions.

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can go from `kasmos` to 4 concurrent workers running in under 60 seconds.
- **SC-002**: Worker status updates appear in the dashboard within 1 second of the process state change.
- **SC-003**: Session continuation preserves full agent context -- follow-up workers can reference files and decisions from the parent session without re-reading.
- **SC-004**: kasmos operates with zero AI token cost for orchestration. Only worker agents and on-demand AI helpers consume tokens.
- **SC-005**: Single binary distribution with no runtime dependencies beyond OpenCode and git.
- **SC-006**: `kasmos setup` completes in under 30 seconds and produces working agent configurations.
- **SC-007**: Session reattach restores accurate worker state within 2 seconds.
- **SC-008**: Daemon mode produces structured output parseable by CI/CD pipelines.

## Assumptions

- OpenCode v1.2+ is installed and supports `run --continue -s <session_id>` for session continuation.
- OpenCode custom agents (`.opencode/agents/*.md`) are the mechanism for role-specific configuration.
- The host system has sufficient resources to run multiple concurrent OpenCode processes (each is ~100-200MB RSS).
- Linux is the primary platform. macOS support is desirable but not required for MVP.
- The current kasmos Rust codebase is available as reference for workflow patterns (task sources, agent roles, prompt construction) but no code is reused.

## Research Work Package

### TUI Design (delegated to specialized agent)

Before implementation, a dedicated research work package should produce:

1. **Component mapping**: Which bubbles components map to each TUI element (worker table, output viewport, spawn dialog, task list, status bar).
2. **Layout system**: How the TUI arranges components at different terminal sizes. Responsive breakpoints.
3. **Keybind scheme**: Full keybinding map for all interactions (spawn, kill, continue, focus, scroll, filter, help).
4. **Message architecture**: bubbletea Msg types for worker events, subprocess events, user input, and timer ticks. How they flow through the Elm Update cycle.
5. **Color scheme and styling**: lipgloss styles for worker states (running, done, failed, killed), agent roles, status indicators.
6. **Daemon mode behavior**: Exactly what stdout output looks like in headless mode. JSON schema for structured output.
7. **State persistence format**: JSON schema for the session file written to disk.
8. **Worker backend interface**: Exact Go interface definition for WorkerBackend, with SubprocessBackend as MVP and TmuxBackend as future option.

This research should produce a design document with mockups (ASCII art), interface definitions, and implementation recommendations that a coder agent can execute directly.
