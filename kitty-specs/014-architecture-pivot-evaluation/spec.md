# 014 - Architecture Pivot Evaluation

## Problem

kasmos is an MCP-first orchestration CLI that coordinates AI coding agent swarms inside Zellij terminal sessions. While tests pass and core functionality works, development has hit repeated friction walls stemming from fundamental architectural constraints:

1. **Zellij CLI opacity**: There is no `list-panes` or `focus-pane-by-name` CLI command (Zellij 0.41+). kasmos must maintain its own in-memory `WorkerRegistry` to track panes it spawned, but has no way to reconcile this with Zellij's actual pane state. If a pane crashes, is closed by the user, or Zellij restarts, the registry is stale and unrecoverable.

2. **Process boundary friction**: kasmos runs as three separate processes -- a launcher binary, an MCP stdio server (subprocess of the manager agent), and the manager agent itself (OpenCode). Communication between these is indirect: the manager agent calls MCP tools, which shell out to `zellij action` commands, which affect panes that kasmos cannot observe. This creates a blind orchestrator problem.

3. **Layout generation complexity**: kasmos generates KDL layout files programmatically using the `kdl` crate, with extensive workarounds for KDL v2 vs Zellij's KDL v1 expectations (`#true` -> `true` replacements), manual string escaping, and zjstatus configuration embedded in Rust code. This is brittle and hard to maintain.

4. **Worktree path confusion**: Every subsystem must distinguish between main repo paths and worktree paths. File watchers, task file transitions, prompt construction, and agent CWD setup all need to resolve the correct path variant. This has been a recurring source of bugs.

5. **External dependency chain**: kasmos requires Zellij, OpenCode (ocx), spec-kitty, bun (for pane-tracker MCP server), git, and two Zellij plugins (zjstatus, zellij-pane-tracker) all installed and configured. The `kasmos setup` command validates these, but the dependency surface is large and fragile.

6. **No pane introspection**: The MCP server's `read_messages` and `wait_for_event` tools rely on parsing a message-log pane's scrollback via the zellij-pane-tracker plugin. This is an indirect, fragile communication channel. If the pane-tracker plugin isn't loaded or the message format changes, inter-agent communication breaks silently.

### Root Cause

kasmos is an external process trying to orchestrate a terminal multiplexer that was not designed for programmatic orchestration. The Zellij CLI provides session/tab/pane creation but minimal introspection or event subscription. kasmos compensates with workarounds (in-memory registry, message-log pane parsing, layout KDL generation), but each workaround introduces its own failure modes.

### The Question

Would kasmos be better served by one of these alternative architectures?

- **A) Zellij plugin (WASM)**: Move orchestration logic into a Zellij plugin that has native access to pane lifecycle events, pane metadata, and the plugin API.
- **B) OpenCode fork/extension**: Embed orchestration directly into the AI agent runtime, eliminating the Zellij dependency for agent coordination.
- **C) Hybrid**: Keep kasmos as the MCP server but replace the Zellij CLI shelling with a Zellij plugin that acts as a bridge, providing the introspection kasmos currently lacks.
- **D) Status quo with targeted fixes**: Accept the current architecture and address specific pain points incrementally.

## Evaluation Dimensions

This specification defines the dimensions along which each architecture option must be evaluated. The plan phase will conduct the actual evaluation.

### ED-001: Pane Lifecycle Observability

Can the architecture observe pane creation, destruction, focus changes, and command exit status without polling or indirect inference?

- Current: No. Registry is maintained manually; no reconciliation with Zellij state.
- Zellij plugin API offers: `PaneUpdate` event subscription, `ListClients` command, pane metadata access.

### ED-002: Inter-Agent Communication

How do agents (manager, workers) exchange structured messages?

- Current: Message-log pane with text parsing via zellij-pane-tracker MCP server.
- Zellij plugin API offers: `pipe_message_to_plugin`, plugin-to-plugin messaging, `write_chars`/`write_bytes` to pane stdin.

### ED-003: Process Spawning and Management

Can the architecture spawn, monitor, and terminate agent processes?

- Current: `zellij action new-pane` with `--command` flag, no process monitoring.
- Zellij plugin API offers: `open_command_pane`, `open_terminal_pane`, `close_pane`, `run_command` (background), command exit status events.

### ED-004: Filesystem and State Access

Can the architecture read/write spec files, task files, config, and worktree state?

- Current: Full filesystem access (native Rust binary).
- Zellij plugin API offers: Mapped filesystem access (`/host/`, `/data/`, `/tmp/`), but with performance caveats.

### ED-005: MCP Server Compatibility

Can the architecture continue to expose MCP tools for AI agent consumption?

- Current: `rmcp` crate with stdio transport, works well.
- Zellij plugin: WASM plugins cannot run TCP/stdio servers directly; would need a bridge process or pipe-based transport.

### ED-006: Development Velocity

How much existing code can be reused? What is the migration cost?

- Current codebase: ~3500 lines of Rust across 17 source files, 9 MCP tools, comprehensive test suite.
- Zellij plugin: Requires `wasm32-wasip1` target, different async model, different dependency constraints (no tokio in WASM).

### ED-007: User Experience

Setup complexity, runtime reliability, error recovery, and debugging ergonomics.

- Current: 7+ external dependencies, `kasmos setup` validation, config generation.
- Zellij plugin: Single WASM file in `~/.config/zellij/plugins/`, loaded via config.

### ED-008: Extensibility

How easily can new agent roles, workflow patterns, or integrations be added?

- Current: Add a new MCP tool handler, update registry types, add prompt template.
- Zellij plugin: Add event handler, update plugin state, re-compile WASM.

## User Stories

### US-001: Architecture Decision

As a kasmos maintainer, I want a structured evaluation of alternative architectures so that I can make an informed decision about whether to pivot, and if so, to which architecture.

**Acceptance Scenario:**
- GIVEN the current kasmos codebase and its known pain points
- WHEN the evaluation is complete
- THEN there is a clear recommendation with rationale, migration cost estimate, and risk assessment for each option

### US-002: Proof of Concept Scope

As a kasmos maintainer, I want to know what a minimal proof-of-concept looks like for each viable option so that I can validate the recommendation before committing to a full migration.

**Acceptance Scenario:**
- GIVEN the recommended architecture
- WHEN a PoC scope is defined
- THEN it covers the highest-risk integration point (pane lifecycle observability) and can be built in 1-2 work packages

### US-003: Migration Path

As a kasmos maintainer, I want to understand the migration path from the current architecture to the recommended one so that I can plan the transition without losing existing functionality.

**Acceptance Scenario:**
- GIVEN the recommended architecture
- WHEN the migration path is documented
- THEN it identifies which current modules are reusable, which must be rewritten, and which can be incrementally migrated

### US-004: Risk Identification

As a kasmos maintainer, I want to understand the risks of each architecture option so that I can weigh them against the known pain points of the status quo.

**Acceptance Scenario:**
- GIVEN each architecture option
- WHEN risks are documented
- THEN they cover technical risk (API stability, WASM limitations), operational risk (deployment complexity), and strategic risk (upstream dependency changes)

## Functional Requirements

### FR-001: Evaluate Zellij Plugin Architecture (Option A)

MUST evaluate the feasibility of implementing kasmos as a Zellij WASM plugin, covering:
- Plugin API coverage for all 9 current MCP tool equivalents
- WASM runtime constraints (no tokio, no TCP sockets, mapped filesystem)
- Plugin-to-agent communication patterns (replacing MCP stdio transport)
- Build and distribution model (single .wasm file vs current cargo install)
- State persistence across plugin reloads

### FR-002: Evaluate OpenCode Fork/Extension Architecture (Option B)

MUST evaluate the feasibility of embedding orchestration into the AI agent runtime, covering:
- OpenCode's extension/plugin model (if any)
- Whether orchestration logic can run inside the agent's process
- Impact on agent-agnostic design (currently supports OpenCode and Claude Code)
- Maintenance burden of forking vs extending

### FR-003: Evaluate Hybrid Architecture (Option C)

MUST evaluate a hybrid where kasmos keeps its MCP server but uses a Zellij plugin as a bridge for pane introspection, covering:
- Plugin-to-external-process communication (pipes, shared files, HTTP)
- Which current pain points this resolves vs which remain
- Additional complexity of maintaining both a binary and a plugin
- Whether the zellij-pane-tracker plugin already partially fills this role

### FR-004: Evaluate Status Quo with Targeted Fixes (Option D)

MUST evaluate incremental improvements to the current architecture, covering:
- Specific fixes for each pain point listed in the Problem section
- Estimated effort per fix
- Whether fixes compound or remain isolated improvements
- Long-term maintainability trajectory

### FR-005: Comparative Scoring

MUST produce a comparison matrix scoring each option against all evaluation dimensions (ED-001 through ED-008) on a consistent scale, with weighted scoring based on pain point severity.

### FR-006: Recommendation with Rationale

MUST produce a single recommended architecture with:
- Clear rationale tied to evaluation dimensions
- Dissenting considerations (why the other options were not chosen)
- Confidence level (high/medium/low) with explanation
- Conditions under which the recommendation should be revisited

### FR-007: PoC Definition

MUST define a proof-of-concept scope for the recommended architecture that:
- Targets the highest-risk integration point
- Can be implemented in 1-2 work packages
- Has clear success/failure criteria
- Is reversible (does not burn bridges with the current architecture)

### FR-008: Migration Roadmap

MUST produce a migration roadmap from current to recommended architecture that:
- Maps current modules to their equivalents in the new architecture
- Identifies reusable code vs rewrite-required code
- Defines migration phases (can be done incrementally, not big-bang)
- Estimates total effort in work packages

## Non-Functional Requirements

### NFR-001: Objectivity

The evaluation MUST consider each option on its technical merits, not on sunk cost in the current implementation. The status quo option (D) must be evaluated with the same rigor as the alternatives.

### NFR-002: Verifiability

Claims about Zellij plugin API capabilities, OpenCode extensibility, and WASM runtime constraints MUST be verified against current documentation or source code, not assumed. Research artifacts must cite sources.

### NFR-003: Actionability

The output must be specific enough that a developer can begin implementing the recommended option without additional architectural research. Vague recommendations like "consider using a plugin" are insufficient.

### NFR-004: Reversibility Awareness

The evaluation MUST flag any recommended changes that are difficult to reverse, and provide mitigation strategies for those changes.

## Key Entities

| Entity | Description |
|--------|-------------|
| ArchitectureOption | One of the four evaluated approaches (A/B/C/D) |
| EvaluationDimension | A criterion for comparing options (ED-001 through ED-008) |
| ComparisonMatrix | Scored grid of options vs dimensions |
| MigrationPhase | A discrete step in transitioning from current to recommended architecture |
| ProofOfConcept | Minimal implementation to validate the highest-risk aspect of the recommendation |
| PainPoint | A specific, documented friction in the current architecture |

## Edge Cases

### EC-001: Zellij Plugin API Insufficient

If the Zellij plugin API does not cover a critical kasmos capability (e.g., MCP server hosting), the evaluation must document the gap and assess whether a bridge/workaround is viable or whether it disqualifies the option.

### EC-002: OpenCode Not Extensible

If OpenCode has no plugin/extension model, Option B reduces to "fork OpenCode," which has different cost/benefit characteristics than extending it. The evaluation must distinguish between these sub-options.

### EC-003: Multiple Options Score Similarly

If two or more options score within margin of error on the comparison matrix, the evaluation must use tiebreaker criteria: migration cost, reversibility, and alignment with the Zellij ecosystem direction.

### EC-004: Upstream Dependency Instability

If a recommended architecture depends on an unstable upstream API (e.g., Zellij plugin API is pre-1.0), the evaluation must assess the risk of breaking changes and recommend mitigation (pinned versions, abstraction layers).

### EC-005: Partial Migration Viability

The evaluation must consider whether a partial migration is viable (e.g., move pane management to a plugin but keep MCP server as a binary), or whether the architecture must be all-or-nothing.

## Success Criteria

- SC-001: A comparison matrix exists with scores for all 4 options across all 8 evaluation dimensions
- SC-002: A single recommended architecture is identified with clear rationale
- SC-003: A PoC scope is defined with success/failure criteria
- SC-004: A migration roadmap exists with phase definitions and effort estimates
- SC-005: All claims about external APIs are backed by documentation references or source code citations
- SC-006: The evaluation is reviewed and the recommendation is accepted or a specific alternative is chosen with documented reasoning

## Constraints

- C-001: The evaluation must be completable in the plan phase (no implementation required for the evaluation itself)
- C-002: The PoC must be scoped to 1-2 work packages maximum
- C-003: The recommended architecture must support Linux as the primary platform (macOS best-effort, per constitution)
- C-004: The recommended architecture must not require users to modify their Zellij configuration beyond adding a plugin (if applicable)
- C-005: The recommended architecture must preserve the ability to orchestrate multiple AI agent roles (planner, coder, reviewer, release) concurrently
