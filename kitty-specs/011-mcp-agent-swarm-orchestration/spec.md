# Feature Specification: MCP Agent Swarm Orchestration

**Feature Branch**: `011-mcp-agent-swarm-orchestration`  
**Created**: 2026-02-13  
**Status**: Draft  
**Input**: User description: "Pivot kasmos from TUI-based orchestrator to MCP-powered agent swarm using zellij-pane-tracker for inter-agent communication"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Launch Kasmos Session (Priority: P1)

A developer runs `kasmos` (optionally with a spec prefix like `kasmos 011`) from any terminal. Kasmos validates that required tools are available, generates a session layout, and opens a Zellij session named "kasmos" with a manager agent in the top-left pane, a message-log pane to its right, and an empty worker row below. The manager agent is automatically primed with the project context and, if a spec prefix was provided, binds to that feature specification. Once the layout is ready, the manager greets the user and reports what feature (if any) it has bound to and what the current workflow state is.

If kasmos is already running inside a Zellij session, it opens the layout as a new tab within the current session instead of creating a new session.

**Why this priority**: Without session bootstrapping, nothing else works. This is the foundation that creates the agent environment and primes the manager. It replaces the current orchestrator TUI entry point.

**Independent Test**: Run `kasmos 011` from a terminal outside Zellij. Verify a new Zellij session named "kasmos" opens with the correct layout. Verify the manager agent is active and reports binding to spec 011. Then, from within that session, run `kasmos 012` and verify a new tab opens (not a new session) with a separate manager bound to spec 012.

**Acceptance Scenarios**:

1. **Given** a terminal outside Zellij with all dependencies installed, **When** the user runs `kasmos`, **Then** a new Zellij session named "kasmos" launches with the manager pane, message-log pane, and an empty worker area, and the manager reports readiness.
2. **Given** a terminal outside Zellij, **When** the user runs `kasmos 011`, **Then** the session launches and the manager automatically binds to the `011-*` feature spec and reports the current workflow phase.
3. **Given** a terminal inside an active Zellij session, **When** the user runs `kasmos 011`, **Then** a new named tab opens within the existing session (no new session created) with the same layout.
4. **Given** the user runs `kasmos` without a spec prefix on the `master` branch, **When** no feature can be inferred from branch or directory, **Then** the manager presents a selection of available feature specs found in the repository and asks the user to choose one.
5. **Given** a feature branch like `011-mcp-agent-swarm-orchestration`, **When** the user runs `kasmos` without a spec prefix, **Then** the manager infers the spec prefix `011` from the branch name and binds to it automatically.

---

### User Story 2 - Manager Orchestrates Planning Phase (Priority: P1)

After binding to a feature spec, the manager agent assesses where the spec is in the planning lifecycle (specify → clarify → plan → analyze → tasks). It presents a summary of the current state and what needs to happen next. Before taking any action, the manager asks the user for explicit confirmation. Upon approval, the manager delegates planning work to worker agents by spawning them in the worker area with appropriate context and commands, monitoring their progress, and collecting results.

**Why this priority**: The planning phase produces the work packages that drive everything else. Without automated planning orchestration, the system cannot progress to implementation. This story validates the core delegation and monitoring loop.

**Independent Test**: Start a kasmos session bound to a spec that has `spec.md` but no plan. Verify the manager identifies "clarify" or "plan" as the next step, presents a summary, waits for confirmation, spawns a worker agent with the correct command, monitors completion, and reports the outcome.

**Acceptance Scenarios**:

1. **Given** a feature spec at the "draft" stage (spec.md exists, no plan), **When** the manager analyzes the workflow state, **Then** it correctly identifies the next phase (clarify or plan) and presents a summary to the user.
2. **Given** the manager has presented the next planning action, **When** the user confirms, **Then** the manager spawns a worker agent in the worker area with the correct spec-kitty command and context.
3. **Given** a worker agent is executing a planning task, **When** the worker completes successfully, **Then** the manager detects completion via scrollback monitoring and message-log messages, reports the outcome, and identifies the next step.
4. **Given** a worker agent encounters an error during planning, **When** the error is detected via scrollback or message-log, **Then** the manager reports the error to the user and suggests corrective action.
5. **Given** the planning phase is fully complete (all work packages defined), **When** the manager detects this, **Then** it pauses automation and notifies the user that planning is done, asking for confirmation before transitioning to the implementation phase.

---

### User Story 3 - Manager Orchestrates Implementation and Review (Priority: P1)

Once work packages are available, the manager orchestrates the implementation-and-review cycle. It spawns coder agents for work packages (respecting wave ordering), monitors their progress, detects when implementation is done, transitions work packages to "for_review" status, spawns reviewer agents, processes review outcomes (approved → done, rejected → rework cycle), and manages the overall wave progression. The manager proactively cleans up completed agent panes and spawns new ones as needed.

**Why this priority**: This is the core value proposition — fully automated implementation and review cycles with human oversight at stage boundaries. It replaces the WaveEngine, CompletionDetector, and SessionManager.

**Independent Test**: Start a kasmos session with a feature that has defined work packages. Confirm the manager spawns a coder for WP01, monitors it, detects completion, transitions to for_review, spawns a reviewer, and handles the review outcome. Verify wave ordering is respected (WP02 doesn't start until WP01's wave completes if they are in different waves).

**Acceptance Scenarios**:

1. **Given** a feature with work packages ready for implementation, **When** the user confirms starting implementation, **Then** the manager spawns coder agents for the first wave's work packages (up to the configured concurrency limit).
2. **Given** a coder agent has completed implementing a work package, **When** the manager detects completion through scrollback polling or message-log, **Then** the manager transitions that work package to "for_review" status and spawns a reviewer agent in a replacement pane.
3. **Given** a reviewer approves a work package, **When** the manager detects the approval, **Then** it transitions the work package to "done" status, cleans up the reviewer pane, and checks if the current wave is complete.
4. **Given** a reviewer rejects a work package with feedback, **When** the manager detects the rejection, **Then** it spawns a new coder agent with the review feedback as context for rework, and the cycle repeats.
5. **Given** all work packages in a wave are "done", **When** the manager detects wave completion, **Then** it starts the next wave or, if all waves are complete, pauses and notifies the user that implementation is finished.
6. **Given** a coder or reviewer agent errors out or is aborted by the user, **When** the manager detects this via scrollback polling, **Then** the manager reports the issue, cleans up the broken pane, and presents recovery options.

---

### User Story 4 - Worker-to-Manager Communication via Message Log (Priority: P1)

A message-log pane sits to the right of the manager pane (~25% width). Worker agents send structured messages to this pane (status updates, completion signals, error reports) using the zellij-pane-tracker's run-in-pane capability. The manager reads these messages to supplement its scrollback polling, providing faster and more reliable event detection than scrollback-only monitoring.

**Why this priority**: Without a reliable communication channel, the manager must rely solely on periodic scrollback polling, which is slow and error-prone. The message-log provides an explicit, structured communication path that dramatically reduces missed events and latency.

**Independent Test**: Spawn a worker agent and have it send a structured message to the message-log pane. Verify the manager can read and parse the message. Verify that the combination of message-log and scrollback provides reliable completion detection.

**Acceptance Scenarios**:

1. **Given** a worker agent is active in a pane, **When** it reaches a milestone (task complete, error, needs input), **Then** it sends a structured message to the message-log pane using the zellij MCP run-in-pane tool.
2. **Given** messages have been written to the message-log pane, **When** the manager checks for updates, **Then** it reads and parses all new messages since the last check.
3. **Given** a worker sends a "task_complete" message, **When** the manager reads it, **Then** it triggers the appropriate workflow transition without waiting for the next scrollback poll cycle.
4. **Given** the message-log pane accumulates many messages, **When** the manager reads messages, **Then** it processes them in order and does not miss or duplicate any messages.

---

### User Story 5 - Dynamic Pane Management with Swap Layouts (Priority: P2)

As workers are spawned and despawned during the workflow, the session layout automatically adjusts. Kasmos generates swap-layout-aware configurations so that adding or removing worker panes causes Zellij to reflow the layout cleanly. The manager area remains fixed while the worker area expands and contracts. Workers are arranged in rows with a configurable maximum per row (default 4), and the layout adapts as the worker count changes.

**Why this priority**: Without dynamic layout management, spawning and removing workers would result in messy, unusable pane arrangements. Swap layouts make the experience feel polished and professional. This is important but secondary to core orchestration.

**Independent Test**: Start a session, spawn 1 worker, observe layout. Spawn 3 more workers, observe reflow to a row of 4. Despawn 2, observe reflow. Verify the manager and message-log areas remain stable throughout.

**Acceptance Scenarios**:

1. **Given** a session with the manager and message-log only, **When** the first worker is spawned, **Then** a worker row appears below the manager area with a single pane.
2. **Given** a worker row with 4 panes, **When** a 5th worker is spawned, **Then** a second worker row appears below the first (or the layout reflows to accommodate the new count).
3. **Given** multiple workers active, **When** a worker pane is closed, **Then** the remaining panes reflow to fill the space without disrupting the manager area.
4. **Given** the manager spawns workers, **When** the pane count changes, **Then** Zellij automatically applies the appropriate swap layout for that pane count.

---

### User Story 6 - Acceptance, Merge, and Release (Priority: P2)

After all work packages pass implementation and review, the manager pauses automation and asks the user to confirm transition to the release phase. Upon confirmation, the manager spawns a release agent that performs acceptance testing, merges the feature branch, and handles cleanup. The manager monitors the release agent and reports the outcome.

**Why this priority**: Release is the final stage of the workflow. It's critical for completing the full lifecycle but is sequentially dependent on implementation and review being complete.

**Independent Test**: Start with a feature where all work packages are "done". Confirm the manager detects this, pauses, asks for release confirmation, spawns a release agent on approval, and monitors it through completion.

**Acceptance Scenarios**:

1. **Given** all work packages in a feature are "done", **When** the manager detects this, **Then** it pauses automation and presents a release-readiness summary to the user.
2. **Given** the user confirms release, **When** the manager proceeds, **Then** it spawns a release agent with the appropriate context (feature branch, target branch, work package summary).
3. **Given** the release agent completes successfully, **When** the manager detects this, **Then** it reports the merge result and any cleanup actions taken.
4. **Given** the release agent encounters a merge conflict or failure, **When** the manager detects this, **Then** it reports the issue and presents options (manual resolution, abort, retry).

---

### User Story 7 - Environment Setup and Validation (Priority: P2)

A first-time user runs `kasmos setup` to validate that all required tools are installed and properly configured. The setup process checks for the presence of required dependencies (terminal multiplexer, agent runtime, pane-tracking plugin) and the necessary configuration files. It reports any missing dependencies and, where possible, generates default configurations.

**Why this priority**: Setup reduces friction for new users and ensures the environment is correct before any orchestration is attempted. It prevents cryptic runtime errors.

**Independent Test**: Run `kasmos setup` in a clean environment with all dependencies. Verify it reports success. Remove one dependency and re-run. Verify it reports the specific missing tool and guidance.

**Acceptance Scenarios**:

1. **Given** all dependencies are installed, **When** the user runs `kasmos setup`, **Then** it validates each dependency and reports all checks passed.
2. **Given** a dependency is missing, **When** the user runs `kasmos setup`, **Then** it reports which dependency is missing and provides installation guidance.
3. **Given** configuration files need to be generated, **When** setup detects they don't exist, **Then** it generates sensible defaults and tells the user what was created.

---

### User Story 8 - Status Updates and Transparency (Priority: P2)

While agents are working, the manager provides periodic status updates to the user. It reports significant events: worker spawned, task completed, review started, review outcome, wave completion, errors encountered, panes cleaned up. The user is never left wondering what's happening — the manager is proactively communicative.

**Why this priority**: Transparency builds trust in automation. Without status updates, the user has no visibility into what the swarm is doing and cannot make informed decisions about intervention.

**Independent Test**: Start an implementation cycle. Verify the manager reports when each worker is spawned, when each finishes, when reviews start and complete, and when waves transition. Verify errors are reported promptly.

**Acceptance Scenarios**:

1. **Given** the manager spawns a worker agent, **When** the spawn completes, **Then** the manager reports which work package the worker is handling and in which pane.
2. **Given** a worker completes a task, **When** the manager detects this, **Then** it reports the completion and the next action it will take.
3. **Given** an error occurs in any worker, **When** the manager detects it, **Then** it reports the error immediately with context about which work package and what went wrong.
4. **Given** the user has been idle for a configurable period during active work, **When** the period elapses, **Then** the manager provides a summary of overall progress.

---

### Edge Cases

- What happens when the Zellij session is terminated unexpectedly while workers are active? The next `kasmos` invocation should detect orphaned state and offer recovery or clean restart.
- What happens when the pane-tracking service becomes unresponsive? The manager should detect connection failures, report them, and degrade gracefully (e.g., fall back to slower polling or pause automation).
- What happens when multiple `kasmos` tabs are running in the same session and both try to manage the same spec? The system should detect the conflict and refuse to bind, directing the user to the existing tab.
- What happens when a worker agent's pane is manually closed by the user? The manager should detect the missing pane on its next poll and handle it as an abort — reporting the loss and offering to respawn or skip.
- What happens when the message-log pane fills up with thousands of messages? The manager should handle message parsing efficiently and not slow down. Old messages beyond a threshold can be considered consumed and ignored.
- What happens when a work package has no clear completion signal in scrollback? The manager should use a timeout-based heuristic plus message-log as the primary detection mechanism, with configurable timeout before flagging the work package for user attention.
- What happens when the user runs `kasmos` but there are no feature specs in the repository? The manager should report that no specs were found and guide the user to create one.
- What happens when the layout generation fails? The system should report the error clearly and fall back to a minimal layout (manager + message-log only).
- What happens when a review-rejection-rework cycle loops more than a configurable number of times? The manager should pause automation and escalate to the user after the configured maximum (default: 3 iterations).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST launch a named session with a manager pane, message-log pane, and worker area when run from outside an existing session.
- **FR-002**: The system MUST open a new named tab within the current session when run from inside an existing session.
- **FR-003**: The system MUST accept an optional spec prefix argument to bind to a specific feature specification.
- **FR-004**: The system MUST infer the spec prefix from the current git branch name when no prefix is provided and the branch matches a known spec pattern.
- **FR-005**: The system MUST present a feature selector when no spec prefix is provided and no spec can be inferred from the environment.
- **FR-006**: The system MUST provide a local service (`kasmos serve`) that exposes orchestration capabilities to the manager agent, including spawning workers, despawning workers, listing workers, reading messages, querying workflow status, transitioning work packages, listing features, and inferring features from the environment.
- **FR-007**: The system MUST generate layout configurations that support automatic reflow when panes are added or removed.
- **FR-008**: The manager agent MUST assess the current workflow phase of the bound feature and present a summary to the user before taking action.
- **FR-009**: The manager agent MUST request explicit user confirmation before beginning any workflow stage (planning, implementation/review, release).
- **FR-010**: The manager agent MUST spawn worker agents with appropriate context, role assignment, and commands for the current task.
- **FR-011**: The manager agent MUST monitor worker agents via message-log reading and scrollback polling to detect completion, errors, and other actionable events.
- **FR-012**: The manager agent MUST transition work packages through their lifecycle states (pending → active → for_review → done/rework) based on detected events.
- **FR-013**: The manager agent MUST respect wave ordering — work packages in later waves do not start until all work packages in earlier waves are complete.
- **FR-014**: The manager agent MUST spawn reviewer agents when a work package transitions to "for_review" status.
- **FR-015**: The manager agent MUST handle review rejections by spawning new coder agents with the review feedback for rework.
- **FR-016**: The manager agent MUST pause automation when transitioning between the three major workflow stages (planning, implementation/review, release).
- **FR-017**: The manager agent MUST actively clean up worker panes that are no longer needed while preserving any panes still in use.
- **FR-018**: The manager agent MUST proactively report status updates for significant events (spawns, completions, errors, wave transitions).
- **FR-019**: Worker agents MUST send structured messages to the message-log pane to communicate status, completion, and errors back to the manager.
- **FR-020**: The system MUST detect and prevent duplicate bindings to the same feature spec from multiple tabs within the same session.
- **FR-021**: The system MUST validate that all required dependencies are available before launching.
- **FR-022**: The system MUST provide a setup command for first-time environment validation and configuration generation.
- **FR-023**: The system MUST cap the review-rejection-rework cycle at a configurable maximum (default: 3 iterations) before pausing for user intervention.
- **FR-024**: The system MUST preserve existing TUI code in a disconnected state (not deleted, just unwired from entry points) for potential future reintegration.
- **FR-025**: All agent interactions MUST use a single agent runtime (OpenCode), regardless of the underlying model, to maintain a consistent execution model.

### Key Entities

- **Session**: A kasmos workspace within the terminal multiplexer, containing one manager, one message-log, and zero or more workers. Attributes: session/tab name, bound feature spec, active workflow phase, worker inventory.
- **Manager Agent**: The controller agent occupying the primary pane. Holds workflow state, makes orchestration decisions, communicates with workers. Attributes: bound feature, current phase, active workers, pending actions.
- **Worker Agent**: A coder, reviewer, or release agent in the worker area. Has a specific task assignment and communicates via message-log. Attributes: pane ID, role (coder/reviewer/release), assigned work package, status (active/complete/errored/aborted).
- **Message Log**: A dedicated pane serving as the communication channel between workers and the manager. Contains structured messages with timestamps, sender IDs, and event types.
- **Work Package (WP)**: A unit of work from the feature plan. Progresses through states: pending → active → for_review → done (or rework loop). Belongs to a wave for ordering.
- **Wave**: An ordered group of work packages that can execute concurrently. All WPs in wave N must complete before wave N+1 begins.
- **Feature Spec**: A specification in the specs directory containing spec document, plan, tasks, and metadata. The unit of work that kasmos orchestrates.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from running the launch command to having a fully operational manager agent session within 10 seconds.
- **SC-002**: The manager detects worker completion events within 15 seconds of the event occurring (via message-log or scrollback polling).
- **SC-003**: The full planning phase (specify through tasks) can be completed with no more than 3 user confirmations (one per sub-phase transition).
- **SC-004**: The implementation-and-review cycle for a single work package (code → review → approve) completes without manual intervention when the review passes on first attempt.
- **SC-005**: The system correctly handles at least 4 concurrent worker agents without layout degradation or missed events.
- **SC-006**: 100% of stage transitions (planning → implementation, implementation → release) pause for user confirmation — no silent phase jumps.
- **SC-007**: When a worker errors out, the manager detects and reports it to the user within 30 seconds.
- **SC-008**: The existing TUI code compiles and passes tests after being disconnected, confirming no destructive changes were made.
- **SC-009**: A clean setup command run on a properly configured machine completes within 5 seconds and reports all-green.
- **SC-010**: The system supports the full workflow lifecycle (specify → clarify → plan → analyze → tasks → implement → review → release) end-to-end through the agent swarm without falling back to manual file editing or pipe-based commands.
