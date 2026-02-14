---
work_package_id: "WP12"
subtasks:
  - "T069"
  - "T070"
  - "T071"
  - "T072"
  - "T073"
  - "T074"
title: "Integration, Legacy Preservation, and Acceptance Hardening"
phase: "Phase 3 - Setup UX, Role Context, and End-to-End Hardening"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP07", "WP08", "WP09", "WP10", "WP11"]
history:
  - timestamp: "2026-02-14T16:27:48Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP12 - Integration, Legacy Preservation, and Acceptance Hardening

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP12 --base WP07
```

---

## Objectives & Success Criteria

Validate end-to-end behavior against locked decisions and success criteria, confirm legacy TUI preservation, and finalize documentation. After this WP:

1. `cargo build` (default) succeeds with new MCP launcher flow
2. `cargo build --features tui` succeeds with legacy TUI code intact (FR-024)
3. Lock conflict and stale takeover flow work end-to-end
4. Audit logging modes and retention thresholds trigger correctly
5. Feature selector pre-launch gate works correctly
6. README and quickstart docs reflect final command behavior
7. FR/SC traceability checklist is complete

## Context & Constraints

- **Depends on WP07, WP08, WP09, WP10, WP11**: All core subsystems complete
- **Locked decisions 1-9**: Engineering alignment from plan.md must be validated
- **Success criteria SC-001 through SC-010**: Measurable outcomes from spec.md
- **FR-024**: Legacy TUI code compiles and passes tests after disconnection

## Subtasks & Detailed Guidance

### Subtask T069 - Verify and preserve legacy TUI compile path

**Purpose**: Confirm that the TUI feature gate works and legacy code still compiles (FR-024, SC-008).

**Steps**:
1. Run `cargo build --features tui` and verify zero errors
2. Run `cargo test --features tui` and verify all existing TUI tests pass
3. Verify these TUI modules still compile when feature-gated:
   - `crates/kasmos/src/hub/` (hub TUI)
   - `crates/kasmos/src/tui/` (orchestrator TUI)
   - `crates/kasmos/src/report.rs`
   - `crates/kasmos/src/tui_cmd.rs`
   - `crates/kasmos/src/tui_preview.rs`
4. Run `cargo build` (default, no TUI) and verify it produces a working binary
5. Verify `kasmos --help` with default build shows new command surface
6. Document the feature gate in README: "Legacy TUI available via `cargo build --features tui`"

**Files**: Various - verification only, minimal changes
**Validation**: Both build configs succeed. TUI tests pass with `--features tui`.

### Subtask T070 - Add integration scenario for lock conflict and stale takeover

**Purpose**: End-to-end test of the lock system under realistic conditions.

**Steps**:
1. Scenario: Lock conflict
   - Process A acquires lock for feature 011
   - Process B attempts to bind to feature 011
   - Verify B receives `FEATURE_LOCK_CONFLICT` with A's owner details
   - Process A releases lock
   - Process B retries and succeeds
2. Scenario: Stale takeover
   - Create a lock file with old heartbeat (> 15 minutes ago)
   - Attempt to bind to the feature
   - Verify `STALE_LOCK_CONFIRMATION_REQUIRED` is returned
   - Provide confirmation token
   - Verify takeover succeeds and new lock is written
3. Use temp directories for isolation. Mock clock if needed for stale timeout testing.
4. These can be integration tests in `crates/kasmos/src/serve/lock.rs` or separate test files.

**Parallel?**: Yes - independent of T071 and T072.
**Files**: Integration test module
**Validation**: Both scenarios pass deterministically.

### Subtask T071 - Add integration scenario for audit logging modes and retention

**Purpose**: End-to-end test of audit system behavior under both modes and retention triggers.

**Steps**:
1. Scenario: Default metadata-only mode
   - Trigger several audit events (spawn, transition, despawn)
   - Read `messages.jsonl` and verify entries have metadata but no `debug_payload`
2. Scenario: Debug full payload mode
   - Enable debug mode in config
   - Trigger audit events
   - Verify entries include `debug_payload` field
3. Scenario: Size-based retention trigger
   - Create an audit file exceeding 512MB (or use a smaller test threshold)
   - Trigger retention check
   - Verify rotation occurs (file renamed, new file started)
4. Scenario: Age-based retention trigger
   - Create entries with old timestamps (> 14 days)
   - Trigger retention check
   - Verify rotation occurs

**Parallel?**: Yes - independent of T070 and T072.
**Files**: Integration test module
**Validation**: All audit scenarios pass with correct behavior.

### Subtask T072 - Add integration scenario for feature selector pre-launch gate

**Purpose**: Verify the selector runs before Zellij and the no-specs path exits cleanly.

**Steps**:
1. Scenario: Selector gate
   - Set up environment with no inferable feature (master branch, no spec prefix)
   - Verify the selector is presented
   - Verify NO Zellij commands have been executed before selection
2. Scenario: No specs available
   - Remove all spec directories from `kitty-specs/`
   - Run `kasmos`
   - Verify clean exit message and exit code 0
   - Verify no Zellij session/tab was created
3. These tests may need to mock Zellij commands to verify they weren't called.

**Parallel?**: Yes - independent of T070 and T071.
**Files**: Integration test module
**Validation**: Selector gate and no-specs path work correctly.

### Subtask T073 - Align README, quickstart, and docs with final behavior

**Purpose**: Update documentation to reflect the final command behavior and architecture.

**Steps**:
1. Update `README.md`:
   - Remove references to old TUI-first workflow
   - Document new commands: `kasmos [spec-prefix]`, `kasmos serve`, `kasmos setup`, `kasmos list`, `kasmos status`
   - Document the MCP agent swarm architecture
   - Document the feature gate: `cargo build --features tui` for legacy TUI
2. Update `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`:
   - Verify all scenarios match actual command behavior
   - Update any outdated examples
3. Verify `kasmos --help` output matches documentation.
4. Do NOT update docs until final command behavior is stable (this runs last).

**Files**: `README.md`, `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`
**Validation**: README matches actual command behavior. quickstart scenarios work.

### Subtask T074 - Run final verification matrix and FR/SC traceability

**Purpose**: Comprehensive verification that all functional requirements and success criteria are met.

**Steps**:
1. Run `cargo test` for default features - all pass
2. Run `cargo test --features tui` - all pass
3. Run `cargo build` - succeeds
4. Manual smoke checks (document results):
   - `kasmos setup` reports all checks
   - `kasmos 011` launches session (if Zellij available)
   - `kasmos list` shows features
   - `kasmos status` shows WP progress
   - `kasmos serve` starts and responds to tools/list
5. FR traceability checklist - map each FR to the WP that implements it:
   - FR-001: WP03 (session launch)
   - FR-002: WP03 (tab creation inside Zellij)
   - FR-003: WP02 (spec prefix arg)
   - FR-004: WP02 (branch inference)
   - FR-005: WP02 (CLI selector)
   - FR-006: WP04 (kasmos serve)
   - FR-007: WP03 (swap layouts)
   - FR-008 to FR-018: WP07-WP09 (manager orchestration)
   - FR-019: WP08 (worker messages)
   - FR-020: WP05 (feature locking)
   - FR-021: WP02/WP10 (preflight)
   - FR-022: WP10 (setup command)
   - FR-023: WP09 (rejection cap)
   - FR-024: WP01 (TUI preservation)
   - FR-025: WP11 (single agent runtime)
   - FR-026: WP08 (manager decision logging)
   - FR-027: WP06 (audit persistence)
   - FR-028-031: WP11 (role context boundaries)
6. SC traceability - verify each success criterion has a corresponding test or validation.

**Files**: New traceability checklist file (if needed), test results
**Validation**: All FRs mapped to implementations. All SCs have validation evidence.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Flaky integration tests around terminal tooling | Split deterministic unit-level checks from optional environment-dependent smoke tests |
| Docs drifting from implementation | Update docs only after final command/help output is stable (this WP runs last) |
| TUI feature gate bit rot | Include `--features tui` in CI matrix to catch regressions early |

## Review Guidance

- Verify both build configs succeed (default and tui)
- Verify lock scenario tests are deterministic
- Verify audit scenario tests cover both modes and both retention triggers
- Verify selector gate test proves no Zellij commands before selection
- Verify README accurately describes new command surface
- Verify FR/SC traceability checklist is complete
- Verify all locked decisions (1-9 from plan.md) are validated

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
