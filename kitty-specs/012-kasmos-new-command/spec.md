# Feature Specification: Kasmos New Command

**Feature Branch**: `012-kasmos-new-command`
**Created**: 2026-02-16
**Status**: Draft
**Input**: User description: "i want to make a new 'kasmos new' that opens up an opencode session with kasmos' planning agent calling /spec-kitty.specify"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create a New Feature Spec (Priority: P1)

A developer wants to create a new feature specification for the kasmos project. They run `kasmos new` from their terminal. Kasmos validates that the opencode agent runtime and spec-kitty are available, constructs a planning agent prompt that includes project context (constitution, architecture memory, workflow intelligence, existing specs awareness), and launches opencode directly in the current terminal. The planning agent automatically invokes `/spec-kitty.specify` to begin the interactive discovery and specification workflow. The user interacts with the planning agent to describe their feature, answer discovery questions, and produce a complete `spec.md` in `kitty-specs/`. When the planning agent finishes (or the user exits opencode), control returns to the user's terminal. The resulting spec can then be launched with `kasmos [spec-prefix]`.

**Why this priority**: This is the entire feature. Without the ability to launch the planning agent with the right context and command, nothing else exists. This is the foundational and only critical user journey.

**Independent Test**: Run `kasmos new` from a terminal. Verify opencode launches in-place with the planning agent role. Verify the agent has project context loaded and initiates `/spec-kitty.specify`. Complete a spec creation flow. Verify a new `kitty-specs/###-feature/spec.md` is created. Exit and verify the terminal returns to the shell prompt.

**Acceptance Scenarios**:

1. **Given** a terminal with opencode and spec-kitty installed, **When** the user runs `kasmos new`, **Then** opencode launches in the current terminal configured as a planning agent and automatically begins the `/spec-kitty.specify` workflow.
2. **Given** the planning agent is active, **When** the user interacts with the discovery interview and completes the spec, **Then** a new feature directory and `spec.md` are created in `kitty-specs/`.
3. **Given** opencode finishes or the user exits, **When** the process ends, **Then** control returns to the user's shell in the same terminal.

---

### User Story 2 - Pass an Initial Feature Description (Priority: P2)

A developer already knows what they want to build and wants to skip ahead by providing an initial description. They run `kasmos new "add dark mode toggle to settings"`. The planning agent receives this description as seed input for `/spec-kitty.specify`, using it as the starting point for discovery rather than starting with a blank prompt. This reduces back-and-forth for users who have a clear idea of their feature.

**Why this priority**: This is a convenience enhancement over the core P1 flow. It improves the experience for users with a clear feature idea but is not essential for the command to function.

**Independent Test**: Run `kasmos new "add dark mode toggle"`. Verify the planning agent launches and begins `/spec-kitty.specify` with the provided description already captured as the initial feature input.

**Acceptance Scenarios**:

1. **Given** a terminal with dependencies installed, **When** the user runs `kasmos new "add dark mode toggle"`, **Then** the planning agent launches and uses "add dark mode toggle" as the seed description for the specification workflow.
2. **Given** an initial description is provided, **When** the planning agent starts, **Then** it treats the description as a starting point for discovery (not the final truth) and still conducts appropriate follow-up.

---

### User Story 3 - Pre-flight Validation Catches Missing Dependencies (Priority: P2)

A developer runs `kasmos new` but does not have spec-kitty or opencode installed. Before launching anything, kasmos detects the missing dependency, prints a clear error message with install guidance, and exits with a non-zero code. No partial state is created.

**Why this priority**: Preventing cryptic runtime failures improves the developer experience, but the primary audience already has dependencies installed via `kasmos setup`.

**Independent Test**: Temporarily rename the opencode binary. Run `kasmos new`. Verify an actionable error message is printed and the exit code is non-zero. Restore the binary and verify `kasmos new` works.

**Acceptance Scenarios**:

1. **Given** the opencode binary is missing from PATH, **When** the user runs `kasmos new`, **Then** the command prints an error identifying the missing dependency with install guidance and exits with a non-zero code.
2. **Given** spec-kitty is missing from PATH, **When** the user runs `kasmos new`, **Then** the command prints an error identifying spec-kitty as missing with install guidance and exits with a non-zero code.
3. **Given** all dependencies are present, **When** the user runs `kasmos new`, **Then** pre-flight passes silently and the planning agent launches.

---

### Edge Cases

- What happens when the user runs `kasmos new` from outside the project root (no `kitty-specs/` directory or `kasmos.toml`)? The command should detect the missing project context, print guidance to navigate to the project root or run `kasmos setup`, and exit.
- What happens when opencode crashes mid-session? The terminal returns to the shell prompt with a non-zero exit code from the opencode process. No cleanup is needed since no Zellij session or lock state was created.
- What happens when the user provides a very long initial description? The description should be passed through to the prompt without truncation. Opencode and the planning agent handle arbitrarily long prompt inputs.
- What happens when `kasmos new` is run inside a Zellij session? It should still work identically -- opencode runs in the current pane regardless of whether that pane is inside Zellij or a bare terminal.
- What happens when a planning agent session is interrupted (Ctrl+C)? The opencode process is terminated and the terminal returns to the shell. Any partially-created spec artifacts from spec-kitty remain on disk (spec-kitty handles its own atomicity).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST provide a `new` subcommand on the `kasmos` CLI that launches the planning agent workflow.
- **FR-002**: The `new` subcommand MUST accept an optional positional argument containing an initial feature description (e.g., `kasmos new "add dark mode toggle"`).
- **FR-003**: The system MUST launch the opencode agent runtime directly in the current terminal process (exec or equivalent), not in a Zellij session, tab, or pane.
- **FR-004**: The system MUST configure the launched opencode session with the planning agent role, using the existing `AgentRole::Planner` context boundaries (spec, plan, architecture, workflow intelligence, constitution, project structure).
- **FR-005**: The planning agent prompt MUST include an instruction to invoke `/spec-kitty.specify` as its primary task.
- **FR-006**: When an initial feature description is provided via the positional argument, the prompt MUST include that description so the planning agent can pass it through to `/spec-kitty.specify`.
- **FR-007**: The system MUST validate that the opencode binary and spec-kitty binary are available before launching. If either is missing, the command MUST print an actionable error with install guidance and exit with a non-zero code.
- **FR-008**: The system MUST load project configuration from `kasmos.toml` to determine the opencode binary path, profile, and specs root directory.
- **FR-009**: The system MUST use the configured opencode profile (if set) when launching the agent session.
- **FR-010**: When opencode exits, the `kasmos new` process MUST exit with the same exit code, returning control to the user's terminal.
- **FR-011**: The system MUST NOT create any Zellij sessions, tabs, or panes. The `new` subcommand has no Zellij dependency.
- **FR-012**: The system MUST NOT acquire feature locks, since no feature exists yet at invocation time.
- **FR-013**: The system MUST work regardless of whether it is run inside or outside of a Zellij session.

### Key Entities

- **Planning Agent**: An opencode session configured with the Planner role's context boundaries. Receives project-wide context (constitution, architecture memory, workflow intelligence, project structure) sufficient to make informed specification decisions. Its primary task is to run `/spec-kitty.specify`.
- **Initial Description**: An optional free-text string provided by the user at invocation time. Passed through to the planning agent's prompt as seed input for the specification workflow.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user can go from running `kasmos new` to an interactive planning agent session within 3 seconds.
- **SC-002**: The planning agent successfully invokes `/spec-kitty.specify` on first launch without manual user intervention to configure or trigger it.
- **SC-003**: A complete specification workflow (from `kasmos new` through a finished `spec.md`) can be completed in a single uninterrupted session.
- **SC-004**: When dependencies are missing, the user receives an actionable error message within 1 second of running the command.
- **SC-005**: The command adds no new runtime dependencies beyond what kasmos already requires (opencode, spec-kitty).
