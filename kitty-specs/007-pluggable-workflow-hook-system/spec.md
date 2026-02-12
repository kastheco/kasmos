# Feature Specification: Pluggable Workflow Hook System

**Feature Branch**: `007-pluggable-workflow-hook-system`
**Created**: 2026-02-11
**Status**: Draft
**Input**: Build a pluggable workflow/tooling layer so kasmos can run the same lifecycle with different phase providers (SpecKitty, ClaudeFlow MCP, custom workflow), with lifecycle hooks and stronger TUI configuration.

## Clarifications

### Session 2026-02-11

- Q: Should kasmos become fully workflow-agnostic, or keep current lifecycle and make tooling pluggable? → A: Keep current lifecycle and make phase tooling pluggable.
- Q: Should provider selection be single-provider or per-phase mix-and-match? → A: Per-phase mix-and-match.
- Q: Which phases are required in v1? → A: Planning (`specify`, `clarify`, `plan`, `tasks`) and implementation (`start-work`, `parallel execution/dispatch`, `review`). `status/query` is low priority and should align to a shared communication contract.
- Q: Which lifecycle hooks are required? → A: `pre-planning`, `post-planning`, `pre-implementation`, `pre-task-run`, `moving_to_review`, `on_reject` (loops back to `pre-task-run`), `on_approve`, `done`.
- Q: Extensibility model for v1? → A: Built-in adapters in v1 plus an explicit extension contract for future third-party plugins.
- Q: What must robust TUI settings include in v1? → A: Per-phase provider selection, hook config (disabled by default with script input), provider-specific settings, fallback behavior, provider toggles (e.g., disable Anthropic when quota is hit), and named profiles.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Providers Per Phase (Priority: P1)

An operator configures kasmos so each workflow phase uses the most suitable provider (for example: SpecKitty for planning phases, custom command runner for implementation dispatch, ClaudeFlow-backed review provider for review). The lifecycle remains unchanged, but tooling is selected per phase.

**Why this priority**: This is the core business outcome of the refactor. Without per-phase selection, the system remains effectively hardcoded.

**Independent Test**: Can be tested by assigning different providers to all required planning/implementation phases and confirming each phase executes through the selected provider.

**Acceptance Scenarios**:

1. **Given** the operator assigns provider mappings for all required phases, **When** a run executes, **Then** each phase is handled by its configured provider and no phase falls back to hardcoded SpecKitty behavior.
2. **Given** planning phases use one provider and implementation phases use another, **When** execution crosses from planning to implementation, **Then** provider routing switches correctly with no manual intervention.
3. **Given** a phase has no valid provider configured, **When** that phase is reached, **Then** kasmos blocks progression and surfaces a clear configuration error with remediation guidance.

---

### User Story 2 - Run Lifecycle Hooks Around Phase Events (Priority: P1)

An operator enables selected lifecycle hooks so scripts can run before/after planning, before implementation loops, before each task run, during review transitions, and on approve/reject completion events. Hooks are optional and disabled by default.

**Why this priority**: Hooks are the requested extensibility mechanism for injecting custom behavior without rewriting core orchestration.

**Independent Test**: Can be tested by enabling each required hook with observable script outputs and confirming hooks fire in the correct order with correct event context.

**Acceptance Scenarios**:

1. **Given** `pre-planning` and `post-planning` hooks are enabled, **When** planning executes, **Then** both hooks run exactly once in order around planning.
2. **Given** `pre-task-run` is enabled, **When** each task run begins, **Then** the hook executes once per task attempt.
3. **Given** a task is rejected in review, **When** `on_reject` executes, **Then** control returns to `pre-task-run` for the next attempt.
4. **Given** a task is approved, **When** `on_approve` executes, **Then** the task exits the review loop and continues lifecycle progression.
5. **Given** hooks are disabled by default, **When** no hooks are explicitly enabled, **Then** lifecycle execution proceeds with no hook invocations.

---

### User Story 3 - Keep Review Loop Behavior While Swapping Review Provider (Priority: P1)

A reviewer can keep the same approve/reject workflow semantics while changing the review execution provider (including slash-command verify workflows). Review outcomes continue to drive the same orchestration transitions.

**Why this priority**: Review is a high-impact control point; provider flexibility must not alter governance behavior.

**Independent Test**: Can be tested by running review via different providers and verifying reject loops and approve completion behavior remain consistent.

**Acceptance Scenarios**:

1. **Given** review provider A is configured, **When** review returns reject, **Then** the lifecycle routes through `on_reject` and re-enters `pre-task-run`.
2. **Given** review provider B is configured, **When** review returns approve, **Then** the lifecycle routes through `on_approve` and marks the work package ready for completion.
3. **Given** slash verify is configured for review mode, **When** review is triggered, **Then** verify execution is routed through the active review provider and the result is captured in the standard outcome format.

---

### User Story 4 - Manage Provider Fallbacks and Quota-Based Disablement (Priority: P2)

An operator can define fallback behavior and quickly disable unavailable providers (for example, disabling all Anthropic-backed providers when quota is exhausted), so runs continue using permitted alternatives.

**Why this priority**: Real-world operations require graceful degradation when provider limits or outages occur.

**Independent Test**: Can be tested by simulating provider failure/quota exhaustion and verifying the configured fallback path is taken.

**Acceptance Scenarios**:

1. **Given** a primary provider fails for a phase, **When** fallback is configured, **Then** kasmos routes execution to the fallback provider automatically and logs the failover reason.
2. **Given** a provider family is toggled off in settings, **When** a phase attempts to use a disabled provider, **Then** execution either reroutes via configured fallback or blocks with a clear message.
3. **Given** no fallback is configured and a provider is unavailable, **When** the phase starts, **Then** kasmos halts safely and surfaces operator action requirements.

---

### User Story 5 - Configure Workflow System in TUI with Profiles (Priority: P2)

An operator manages workflow configuration directly from the TUI: per-phase provider selection, hook scripts, provider settings, fallback policy, and named profiles (global and per-project). Switching profiles updates the active execution behavior immediately for new phase transitions.

**Why this priority**: The refactor has high configuration complexity; TUI configuration is required for operational usability.

**Independent Test**: Can be tested by creating multiple profiles, switching between them in TUI, and confirming phase routing/hook behavior changes accordingly.

**Acceptance Scenarios**:

1. **Given** multiple profiles exist, **When** the operator selects a profile in the TUI, **Then** all phase-provider mappings and hook settings update to that profile.
2. **Given** hook scripts are disabled by default, **When** the operator enables a hook and provides a script command, **Then** the hook becomes active for subsequent lifecycle events.
3. **Given** provider-specific settings are modified in TUI, **When** the next relevant phase executes, **Then** the provider runs with updated settings.
4. **Given** both global and project-specific profiles exist, **When** a project override is enabled, **Then** project settings take precedence over global defaults for that project only.

---

### User Story 6 - Standardize Cross-Provider Communication Contract (Priority: P3)

Operators and maintainers can rely on a common result and status contract across providers, so outcomes from planning, execution, and review phases are interpreted consistently regardless of provider origin.

**Why this priority**: A shared contract prevents provider lock-in and enables predictable behavior in mixed-provider deployments.

**Independent Test**: Can be tested by running at least two providers for the same phase and verifying kasmos receives equivalent normalized statuses/outcomes.

**Acceptance Scenarios**:

1. **Given** two different providers implement the same phase, **When** each returns success/failure/retry outcomes, **Then** kasmos normalizes outcomes into a shared status model without provider-specific branching.
2. **Given** phase outputs include metadata, **When** kasmos records phase history, **Then** provider-origin metadata is preserved while core status fields remain uniform.
3. **Given** status/query operations are later implemented, **When** they consume provider data, **Then** they use the same normalized contract as other phases.

---

### Edge Cases

- A configured provider is present in profile data but not installed/enabled at runtime.
- A hook script exits non-zero, times out, or writes invalid output.
- Multiple hooks are enabled for neighboring events and produce conflicting side effects.
- Review provider returns an unrecognized outcome string that does not map cleanly to approve/reject/retry semantics.
- Fallback chain contains circular references (A→B→A) and must be detected.
- Provider toggles disable all providers for a required phase.
- Profile import contains phase mappings for unknown phases.
- Concurrent task execution triggers the same hook event for multiple tasks simultaneously.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support per-phase provider assignment for required planning phases: `specify`, `clarify`, `plan`, `tasks`.
- **FR-002**: System MUST support per-phase provider assignment for required implementation phases: `start-work`, `parallel execution/dispatch`, `review`.
- **FR-003**: System MUST preserve the existing kasmos lifecycle semantics while allowing provider swaps per phase.
- **FR-004**: System MUST provide built-in providers for SpecKitty-compatible flows, ClaudeFlow/MCP-compatible flows, and custom workflow command execution.
- **FR-005**: System MUST define and enforce a normalized phase result contract used by all providers.
- **FR-006**: System MUST support lifecycle hook points: `pre-planning`, `post-planning`, `pre-implementation`, `pre-task-run`, `moving_to_review`, `on_reject`, `on_approve`, and `done`.
- **FR-007**: Hooks MUST be disabled by default and only execute when explicitly enabled.
- **FR-008**: For enabled hooks, system MUST execute configured scripts with event context and capture success/failure/timeout outcomes.
- **FR-009**: `on_reject` hook outcome MUST route lifecycle control back to `pre-task-run` for the affected work package.
- **FR-010**: `on_approve` hook outcome MUST route lifecycle control to normal post-review progression.
- **FR-011**: System MUST support provider fallback policy per phase when primary provider fails or is disabled.
- **FR-012**: System MUST support provider toggles at runtime, including the ability to disable a provider family and prevent new invocations.
- **FR-013**: System MUST expose TUI settings to configure per-phase provider mappings.
- **FR-014**: System MUST expose TUI settings to configure hook enablement and script commands.
- **FR-015**: System MUST expose TUI settings to configure provider-specific settings and fallback behavior.
- **FR-016**: System MUST support named configuration profiles with global defaults and project-level overrides.
- **FR-017**: System MUST include an explicit provider extension contract so future third-party providers can be added without changing lifecycle semantics.
- **FR-018**: System MUST emit clear operator-facing errors when required phase configuration is missing, invalid, or unresolved.

### Key Entities

- **Workflow Phase**: A lifecycle step where a provider performs work (e.g., specify, plan, review).
- **Provider**: A pluggable executor that handles one or more workflow phases.
- **Provider Family**: A logical grouping of providers that can be toggled together (e.g., by quota/risk policy).
- **Hook Event**: A named lifecycle trigger point where optional scripts may run.
- **Hook Script**: Operator-configured command executed when a hook event fires.
- **Phase Result Contract**: Normalized status/outcome schema used across providers.
- **Fallback Policy**: Ordered provider failover behavior for a phase.
- **Configuration Profile**: Named settings bundle containing phase mappings, hook settings, provider settings, and toggles.
- **Project Override**: Project-specific profile data that supersedes global defaults.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can configure all required planning and implementation phases in under 10 minutes from the TUI without editing files manually.
- **SC-002**: In validation runs, 100% of required phases execute using their configured providers (no hardcoded routing).
- **SC-003**: For enabled hooks, at least 99% of hook executions complete with recorded outcomes (success/failure/timeout) and traceable event context.
- **SC-004**: In failover tests, at least 95% of simulated provider outages transition to configured fallback providers without manual intervention.
- **SC-005**: Review reject loops route back to `pre-task-run` correctly in 100% of tested reject scenarios.
- **SC-006**: Profile switching applies new phase mappings for subsequent phase transitions within 2 seconds.
- **SC-007**: Disabling a provider family prevents new executions from that family in 100% of tested scenarios.
- **SC-008**: At least two distinct providers can execute the same phase while producing normalized outcomes consumable by the same lifecycle logic.

## Assumptions

- The existing kasmos orchestration lifecycle (waves, WP lanes, review loop semantics) remains the source of truth and is not being redesigned.
- Built-in providers in v1 include a SpecKitty-aligned provider, a ClaudeFlow/MCP-aligned provider, and a custom command provider.
- Status/query operations are not fully implemented in v1 but must align with the same normalized phase result contract.
- Hook scripts are operator-supplied and trusted within the deployment environment.
- Existing slash verify review behavior is retained, but invocation is routed through the active review provider.
