# Feature Specification: Workflow Cheatsheet

**Feature Branch**: `004-workflow-cheatsheet`
**Created**: 2026-02-10
**Status**: Draft
**Input**: A quick-reference cheatsheet covering the full end-to-end spec-kitty + kasmos workflow—from `/spec-kitty.specify` through planning, task generation, kasmos orchestration via Zellij, and finalization. Delivered as a markdown source file (primary) and a dashboard page (secondary), so the operator never has to recall every step from memory.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Quick Command Lookup During Workflow (Priority: P1)

An operator is partway through developing a feature and cannot remember the next spec-kitty command or what flags are needed. They open the cheatsheet (either `cat`/`less` in the terminal or glance at the dashboard) and immediately see the ordered list of workflow phases with their corresponding commands, expected inputs, and outputs. They find the step they need and proceed without searching docs or chat history.

**Why this priority**: The entire value proposition of this feature is rapid lookup. If the operator cannot scan and find the right command within seconds, the cheatsheet has failed its purpose.

**Independent Test**: Can be fully tested by giving the cheatsheet to an operator unfamiliar with the exact command sequence, asking them to identify the correct command for each phase of the workflow, and verifying they can do so without any external reference within 30 seconds per step.

**Acceptance Scenarios**:

1. **Given** the operator is in the middle of a feature development session, **When** they open the cheatsheet, **Then** they can identify the next command they need within 10 seconds of scanning.
2. **Given** the cheatsheet is open in the terminal, **When** the operator looks for a specific phase (e.g., task generation), **Then** the phase, its command, and its key details are co-located in a single scannable block.
3. **Given** the operator has never used kasmos before, **When** they read the cheatsheet top-to-bottom, **Then** they understand the full end-to-end workflow sequence without needing supplementary documentation.

---

### User Story 2 - End-to-End Workflow Overview (Priority: P1)

An operator (or onboarding team member) wants to understand the entire lifecycle of a feature from inception to merge—including the spec-kitty planning phases and the kasmos orchestration phases. They view a single-page overview that shows the full pipeline: specify → clarify → plan → tasks → implement (kasmos launch) → monitor → review → accept → merge. The overview includes a visual flow or numbered sequence making the progression unmistakable.

**Why this priority**: Without the big picture, operators cannot reason about where they are in the process or what comes next. This mental model is foundational—every other cheatsheet detail hangs off this structure.

**Independent Test**: Can be fully tested by presenting the overview section to a new team member and asking them to draw the workflow from memory after a single read. Success if they reproduce the correct phase order with no missing steps.

**Acceptance Scenarios**:

1. **Given** the cheatsheet's overview section, **When** an operator reads it, **Then** all workflow phases are presented in sequential order with clear progression indicators (numbers, arrows, or indentation).
2. **Given** the overview, **When** the operator looks for the boundary between planning and execution, **Then** the transition from spec-kitty planning to kasmos orchestration is visually distinct.

---

### User Story 3 - Dashboard Page Access (Priority: P2)

An operator prefers a browser-based view of the cheatsheet rather than terminal access. They open the spec-kitty dashboard and navigate to a "Workflow Cheatsheet" page that renders the same content as the markdown file in a readable, styled format. The page is always in sync with the markdown source.

**Why this priority**: The dashboard provides a richer visual experience and is already open during development sessions. Adding the cheatsheet there makes it zero-friction to access. However, the markdown file is the primary deliverable and this page is additive.

**Independent Test**: Can be fully tested by opening the dashboard, navigating to the cheatsheet page, comparing its content to the markdown source file, and verifying parity and readability.

**Acceptance Scenarios**:

1. **Given** the spec-kitty dashboard is running, **When** the operator navigates to the cheatsheet page, **Then** the full workflow cheatsheet is rendered with readable formatting.
2. **Given** the markdown source file is updated, **When** the dashboard page is refreshed, **Then** the dashboard reflects the updated content.

---

### Edge Cases

- What happens if the operator skips optional phases (e.g., `/spec-kitty.clarify`)? The cheatsheet clearly marks optional steps so the operator knows they can proceed without them.
- What if the kasmos CLI isn't installed yet? The cheatsheet includes a prerequisites section listing required tools and how to verify they're available.
- How does the cheatsheet handle workflow variations (wave-gated vs continuous mode)? Both modes are documented with clear branching points showing where the workflow diverges.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST provide a markdown file (`docs/workflow-cheatsheet.md`) containing the complete end-to-end workflow reference.
- **FR-002**: The cheatsheet MUST present all workflow phases in sequential order: specify → (clarify) → plan → (research) → tasks → (analyze) → implement/launch → monitor → review → accept → merge. Optional phases MUST be clearly marked as optional.
- **FR-003**: Each workflow phase MUST include: the slash command or CLI command, a one-line description of what it does, expected inputs, expected outputs, and any prerequisites.
- **FR-004**: The cheatsheet MUST include a prerequisites section listing all required tools (spec-kitty, kasmos, zellij, git) with version-check commands.
- **FR-005**: The cheatsheet MUST include the kasmos orchestration sub-workflow: launch → monitor/interact → wave progression → completion detection → finalization.
- **FR-006**: The cheatsheet MUST document both wave-gated and continuous orchestration modes with clear branching points.
- **FR-007**: The cheatsheet MUST be scannable—phases clearly delineated with headers, commands in code blocks, and no prose walls.
- **FR-008**: System MUST provide a dashboard page that renders the markdown cheatsheet content in the spec-kitty dashboard.
- **FR-009**: The dashboard page MUST stay in sync with the markdown source (rendered from the same file, not a separate copy).
- **FR-010**: The cheatsheet MUST include a "daily session" quick-reference showing the typical commands an operator runs in a single sitting (resume, monitor, advance wave, finalize).

### Key Entities

- **Workflow Phase**: A discrete step in the end-to-end feature lifecycle, each mapped to a specific command and producing defined outputs.
- **Cheatsheet Document**: The markdown source file serving as the single source of truth for the workflow reference.
- **Dashboard Page**: A browser-rendered view of the cheatsheet served by the spec-kitty dashboard.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An operator can identify the correct next command for any workflow phase within 10 seconds of opening the cheatsheet.
- **SC-002**: A new team member can understand the full end-to-end workflow sequence after a single read of the overview section.
- **SC-003**: The cheatsheet covers 100% of the spec-kitty slash commands and kasmos CLI commands used in a standard feature lifecycle.
- **SC-004**: The markdown file renders correctly in both terminal viewers (`cat`/`less`/`bat`) and GitHub/web markdown renderers.
- **SC-005**: The dashboard page displays the cheatsheet content with no manual sync steps required after markdown file updates.

## Assumptions

- **ASM-001**: The spec-kitty dashboard supports adding new pages or routes (confirmed by existing dashboard infrastructure).
- **ASM-002**: The markdown file will be maintained alongside workflow changes—when commands change, the cheatsheet is updated in the same commit.
- **ASM-003**: The operator has basic familiarity with terminal commands and Zellij (the cheatsheet is a reference, not a tutorial).
- **ASM-004**: The spec-kitty dashboard can render markdown content from a file path (either natively or via a simple integration).

## Out of Scope

- Interactive tutorials or guided walkthroughs (this is a reference, not a tutorial).
- Auto-detection of the operator's current workflow phase (no "you are here" indicator in V1).
- Video or animated content.
- Internationalization or multi-language support.
- Command auto-completion or CLI integration (the cheatsheet is a passive document).
