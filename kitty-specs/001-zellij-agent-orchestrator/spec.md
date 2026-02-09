# Feature Specification: Zellij Agent Orchestrator

**Feature Branch**: `001-zellij-agent-orchestrator`  
**Created**: 2026-02-09  
**Status**: Draft  
**Input**: Rework of kasr's 004 autonomous swarm daemon as a Rust-based orchestrator (kasmos/ top-level crate) that uses Zellij as the terminal runtime for managing concurrent AI coding agent sessions.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch Orchestration Run (Priority: P1)

An operator wants to start a coordinated multi-agent coding session for a feature. They run a command from their controller OpenCode session to initiate the orchestration. The system reads the feature's work package specifications and dependency graph, generates a Zellij terminal layout, and creates a session with the operator's controller pane on the left and a grid of work package agent panes on the right. Each agent pane automatically launches OpenCode with a prompt file tailored to that work package's scope and instructions.

**Why this priority**: This is the foundational entry point. Without the ability to launch and structure the orchestration session, no other functionality is possible. It directly enables the operator to begin coordinating multiple agents.

**Independent Test**: Can be fully tested by running the launch command with a feature containing 2–4 work packages and verifying that: (1) a Zellij session is created with the expected layout, (2) the controller pane is accessible on the left, (3) each work package pane displays an OpenCode TUI session with the correct prompt file loaded, and (4) all panes are responsive within 30 seconds of invocation.

**Acceptance Scenarios**:

1. **Given** a feature directory with work package specifications and dependencies, **When** the operator runs the launch command, **Then** a Zellij session is created with a 3-column layout (controller left, agent grid center/right) and all initial-wave work package agents are running in their own panes.
2. **Given** a work package with a defined scope and context, **When** the agent pane launches, **Then** OpenCode loads with a prompt file containing the work package description, context, and instructions.
3. **Given** multiple work packages with dependencies, **When** the orchestration starts, **Then** only work packages with no unmet dependencies are launched in the first wave.

---

### User Story 2 - Monitor and Interact with Agents (Priority: P1)

While agents are working, the operator needs to observe their progress and occasionally provide guidance. The operator can focus any work package pane to see the agent's full terminal output and type input directly into the focused pane. They can unfocus to return to a grid view of all active panes, allowing them to monitor multiple agents simultaneously and switch focus as needed.

**Why this priority**: Real-time visibility and the ability to intervene are critical for orchestration success. Agents may encounter ambiguous situations, need clarification, or require human judgment. This story ensures the operator can stay informed and responsive without leaving the Zellij session.

**Independent Test**: Can be fully tested by launching an orchestration with 2–3 work packages, focusing one pane, typing input into it, verifying the input is received by the agent, unfocusing, and confirming that all panes remain visible and responsive in the grid view.

**Acceptance Scenarios**:

1. **Given** an active orchestration with multiple agent panes, **When** the operator focuses a specific pane, **Then** that pane expands to full view and the operator can type input directly into the agent's TUI.
2. **Given** a focused pane, **When** the operator unfocuses, **Then** the view returns to the grid layout showing all active panes.
3. **Given** multiple agents working in parallel, **When** the operator switches focus between panes, **Then** each pane's state is preserved and the agent continues working.

---

### User Story 3 - Automatic Work Package Completion Detection (Priority: P1)

As agents complete their work, the system must detect completion automatically without requiring manual intervention. The system monitors multiple signals—spec-kitty lane transitions, git commit patterns, and designated file markers—to determine when a work package is finished. Upon detection, the system updates internal state, optionally collapses or closes the completed pane, and launches the next eligible work package if dependencies are met (in continuous mode) or queues it for the next wave (in wave-gated mode).

**Why this priority**: Automatic completion detection is essential for orchestration to feel responsive and reduce operator overhead. Without it, the operator would need to manually confirm every completion, defeating the purpose of automation. This directly impacts the perceived efficiency of the system.

**Independent Test**: Can be fully tested by launching an orchestration, allowing an agent to complete a work package (simulated by moving the work package to the "done" lane in spec-kitty or creating a completion marker file), and verifying that: (1) the system detects the completion within a reasonable time window (e.g., 5 seconds), (2) the pane state is updated, and (3) the next eligible work package is launched (in continuous mode) or queued (in wave-gated mode).

**Acceptance Scenarios**:

1. **Given** a work package agent that has finished its task, **When** the agent moves the work package to the "done" lane in spec-kitty, **Then** the orchestrator detects the completion and updates the work package state to "completed".
2. **Given** a completed work package with dependent work packages, **When** dependencies are satisfied, **Then** the next eligible work package is automatically launched (in continuous mode) or marked as ready for the next wave (in wave-gated mode).
3. **Given** multiple completion signals (lane move, git commit, file marker), **When** any signal is detected, **Then** the system recognizes completion without requiring all signals to be present.

---

### User Story 4 - Manual Work Package Lifecycle Commands (Priority: P2)

The operator needs fallback mechanisms to manage work package state when automatic detection fails or when manual intervention is required. The operator can issue commands from the controller pane to restart a failed work package, pause a running work package, manually mark a work package as complete, focus or zoom a specific pane, or view the current orchestration status. These commands provide fine-grained control over the orchestration flow.

**Why this priority**: While automatic detection is the primary mechanism, edge cases and failures will occur. Manual commands ensure the operator can always recover from unexpected situations and maintain control over the orchestration. This is a safety net that increases confidence in the system.

**Independent Test**: Can be fully tested by launching an orchestration, issuing a restart command for a work package, verifying the pane is relaunched with the same prompt file, issuing a pause command, verifying the pane stops accepting input, and issuing a status command, verifying the output reflects the current state of all work packages.

**Acceptance Scenarios**:

1. **Given** a failed work package pane, **When** the operator issues a restart command, **Then** the pane is relaunched with the same prompt file and the work package state is reset to "active".
2. **Given** an active work package, **When** the operator issues a pause command, **Then** the pane stops accepting input and the work package state is set to "paused".
3. **Given** an orchestration in progress, **When** the operator issues a status command, **Then** the system displays the current state of all work packages, active panes, and wave progress.

---

### User Story 5 - Wave-Gated Progression (Priority: P2)

For orchestrations that require operator review and approval between waves, the system supports wave-gated mode. When a wave completes (all work packages in the wave are finished), the system pauses and prompts the operator to review the results and approve progression to the next wave. The operator can review the completed work, make decisions about whether to proceed, and explicitly confirm before the next wave launches.

**Why this priority**: Wave-gated mode is essential for high-stakes orchestrations where human judgment is required between phases. It allows the operator to review intermediate results, catch issues early, and make informed decisions about how to proceed. This is a key feature for quality assurance and risk management.

**Independent Test**: Can be fully tested by launching an orchestration in wave-gated mode with 2 waves, allowing the first wave to complete, verifying that the system pauses and prompts for confirmation, issuing a confirmation command, and verifying that the second wave launches.

**Acceptance Scenarios**:

1. **Given** an orchestration in wave-gated mode with multiple waves, **When** all work packages in a wave are completed, **Then** the system pauses and displays a prompt in the controller pane requesting confirmation to proceed to the next wave.
2. **Given** a paused orchestration at a wave boundary, **When** the operator reviews the results and issues a confirmation command, **Then** the next wave launches automatically.
3. **Given** a paused orchestration, **When** the operator issues a pause or abort command, **Then** the orchestration remains paused and no further waves are launched until explicitly resumed or aborted.

---

### User Story 6 - Resume After Detach (Priority: P2)

The operator may need to detach from the Zellij session due to network interruption, laptop sleep, or other reasons. When they reattach, the orchestration state must be fully preserved. Running agents continue their work, the state file reflects the current progress, and the layout is restored. The operator can seamlessly resume monitoring and controlling the orchestration without data loss or manual recovery steps.

**Why this priority**: Resilience to detach/reattach is critical for long-running orchestrations. Without state persistence, a brief network interruption could lose hours of work. This feature ensures the system is reliable for real-world usage where interruptions are inevitable.

**Independent Test**: Can be fully tested by launching an orchestration, allowing agents to work for a period, detaching from the Zellij session, waiting a moment, reattaching, and verifying that: (1) the session layout is restored, (2) agents continue working, (3) the state file reflects the current progress, and (4) no work is lost.

**Acceptance Scenarios**:

1. **Given** an active orchestration with running agents, **When** the operator detaches from the Zellij session, **Then** agents continue working in the background.
2. **Given** a detached orchestration, **When** the operator reattaches to the Zellij session, **Then** the layout is restored, agents are still running, and the state file reflects the current progress.
3. **Given** a reattached session, **When** the operator issues commands, **Then** the orchestration responds normally without requiring any manual recovery steps.

---

### Edge Cases

- What happens when an agent pane crashes mid-execution (OpenCode exits unexpectedly)? The system detects the crash, updates the work package state to "failed", and allows the operator to restart the pane via a manual command.
- How does the system handle Zellij session death while agents are running? The system detects the session loss, persists state to disk, and allows the operator to recover by reattaching or relaunching the session.
- What if multiple orchestration runs are attempted simultaneously? The system validates that only one orchestration run is active at a time and rejects or queues subsequent attempts.
- How does the system handle circular dependencies in the work package graph? The system validates the dependency graph before starting and rejects the orchestration with a clear error message if circular dependencies are detected.
- What happens if the operator tries to interact with a completed or closed pane? The system prevents interaction with closed panes and displays a message indicating the pane is no longer active.
- How does the system handle a missing or malformed prompt file? The system detects the error during pane launch, displays an error message in the pane, and allows the operator to restart with a corrected prompt file.
- What if the `zellij` CLI is not available in PATH? The system detects the missing dependency during initialization and displays a clear error message with instructions for installation.
- How does the system handle network interruption while agents are pulling context? Agents continue working with cached context, and the system logs the interruption. The operator can manually retry context pulls via agent commands if needed.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST read work package specifications and dependency graphs from kitty-specs feature directories.
- **FR-002**: System MUST generate valid Zellij KDL layout files dynamically based on the number and arrangement of active work packages.
- **FR-003**: System MUST create a Zellij session with a 3-column layout: controller pane (left column), work package agent grid (center and right columns).
- **FR-004**: System MUST launch each work package agent as an OpenCode TUI session in its own Zellij pane with a prompt file injected via command-line argument.
- **FR-005**: System MUST generate work-package-specific prompt files containing the work package description, context, and instructions, stored in the feature directory for traceability.
- **FR-006**: System MUST detect work package completion automatically by monitoring spec-kitty lane transitions, git activity, or designated file markers.
- **FR-007**: System MUST support manual work package state transitions via commands (restart, pause, complete, abort).
- **FR-008**: System MUST support configurable wave progression: wave-gated (with operator confirmation) or continuous (auto-launch on dependency resolution).
- **FR-009**: System MUST persist orchestration state to disk so that detach/reattach preserves progress.
- **FR-010**: System MUST handle work package agent crashes gracefully—detect the crash, update state, and allow restart.
- **FR-011**: System MUST validate the dependency graph and reject circular dependencies before starting.
- **FR-012**: System MUST provide a command to view current orchestration status (work package states, active panes, wave progress).
- **FR-013**: System MUST manage pane focus and zoom operations via Zellij CLI commands.
- **FR-014**: System MUST clean up Zellij sessions and temporary files when orchestration completes or is explicitly stopped.
- **FR-015**: System MUST support concurrent work package execution within a wave, up to a configurable capacity limit.
- **FR-016**: System MUST integrate with spec-kitty CLI to move work packages between lanes as they progress.

### Key Entities *(include if feature involves data)*

- **Orchestration Run**: A single execution of the orchestrator for a feature, containing all wave and work package state, persisted to disk for recovery and audit.
- **Work Package (WP)**: A unit of work with dependencies, an assigned Zellij pane, and lifecycle state (pending, active, completed, failed, paused).
- **Wave**: A group of work packages that can execute in parallel; wave ordering is determined by dependencies and operator confirmation (in wave-gated mode).
- **Zellij Session**: The terminal session containing all panes for one orchestration run; persists across detach/reattach.
- **Pane**: A Zellij terminal pane running an OpenCode TUI session for one work package; can be focused, zoomed, paused, or restarted.
- **Prompt File**: A generated markdown file containing instructions for a work package agent, stored in the feature directory and passed to OpenCode via command-line argument.
- **State File**: Persistent on-disk representation of the orchestration run's current state, including work package states, wave progress, and pane mappings.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operator can launch an orchestration run and see all initial-wave work package agents working in a structured Zellij layout within 30 seconds of invocation.
- **SC-002**: Operator can focus any work package pane and interact directly with the agent's TUI without leaving the Zellij session.
- **SC-003**: 90% of work package completions are automatically detected without manual intervention.
- **SC-004**: Orchestration state survives Zellij session detach and reattach without data loss.
- **SC-005**: Failed work package agents can be restarted from the controller within a single command.
- **SC-006**: Wave-gated mode pauses at wave boundaries and requires explicit operator confirmation before proceeding.
- **SC-007**: The system handles up to 8 concurrent work package panes without degrading terminal responsiveness.

## Assumptions

- Zellij is installed and available in PATH.
- OpenCode supports prompt file injection via command-line argument (or equivalent mechanism).
- spec-kitty CLI is available for lane management and work package state tracking.
- The operator's terminal is large enough to display a 3-column layout (assumes a modern widescreen display, minimum 120 columns).
- Git is installed and work packages use git worktrees for isolation.
- Work package specifications are stored in kitty-specs feature directories with consistent structure.
