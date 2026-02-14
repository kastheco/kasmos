---
work_package_id: WP10
title: Test, Compatibility, and Performance Gates
lane: "done"
dependencies:
- WP02
- WP04
- WP05
- WP06
- WP08
- WP09
base_branch: 002-ratatui-tui-controller-panel-WP02
base_commit: a1c4a7e3bc679aa136bf5ea2a44b0e9bfe44ceee
created_at: '2026-02-11T14:01:47.301791+00:00'
subtasks:
- T049
- T050
- T051
- T052
- T053
- T054
- T055
- T061
- T062
phase: Phase 4 - Validation
assignee: 'unknown'
agent: "coder"
shell_pid: "3777501"
review_status: "has_feedback"
reviewed_by: "kas"
history:
- timestamp: '2026-02-10T22:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP10 – Test, Compatibility, and Performance Gates

## Objectives & Success Criteria

- `cargo test` passes for all kasmos crates and binaries
- ForReview transition/approval/rejection behaviors are covered by tests
- FIFO and TUI command paths stay behaviorally equivalent
- Notification and input-needed lifecycles are validated end-to-end
- Review automation mode selection/fallback is validated (`slash` -> `prompt`)
- Review results persist and remain visible across process restart

**Implementation command**: `spec-kitty implement WP10 --base WP09`

## Subtasks & Detailed Guidance

### T049-T055: Core validation and performance gates

- T049: Unit tests for ForReview transitions
- T050: Unit tests for contextual action availability
- T051: Integration parity tests for FIFO vs TUI
- T052: Integration tests for input-needed lifecycle
- T053: Notification delivery audit (emitted IDs == surfaced IDs)
- T054: Synthetic 50-WP latency test with SC-005 thresholds
- T055: Final validation gate documentation

### T061: Review runner mode selection and fallback

- Add tests covering:
  - `mode=slash` success path
  - `mode=slash` failure with `fallback_to_prompt=true`
  - `mode=slash` failure with fallback disabled
  - `mode=prompt` default model/reasoning selection

### T062: Persisted review result lifecycle

- Add tests covering:
  - ReviewResult persistence to state/storage
  - Process restart + reload + visibility in status/review views
  - Error review results being surfaced in notifications/logs

## Review Guidance

- Run full suite: `cargo test -p kasmos`
- Verify no flaky timing assumptions in async tests
- Verify review automation tests use deterministic mocks for pane command injection and opencode execution

## Activity Log

- 2026-02-10T22:00:00Z - system - lane=planned - Prompt created.
- 2026-02-11T14:01:47Z – coder – shell_pid=3560101 – lane=doing – Assigned agent via workflow command
- 2026-02-11T14:48:27Z – coder – shell_pid=3560101 – lane=for_review – Submitted for review via swarm
- 2026-02-11T14:48:27Z – reviewer – shell_pid=3704032 – lane=doing – Started review via workflow command
- 2026-02-11T15:10:46Z – coder – shell_pid=3704032 – lane=doing – Reassign to coder for implementation handoff
- 2026-02-11T15:10:54Z – coder – shell_pid=3704032 – lane=for_review – Submitted for review via swarm
- 2026-02-11T15:10:55Z – reviewer – shell_pid=3777501 – lane=doing – Started review via workflow command
- 2026-02-11T16:03:45Z – reviewer – shell_pid=3777501 – lane=planned – Moved to planned
- 2026-02-11T16:15:10Z – coder – shell_pid=3777501 – lane=doing – Addressed review feedback and resumed implementation
- 2026-02-11T16:15:26Z – coder – shell_pid=3777501 – lane=for_review – Resubmitted after WP10 validation-gate fixes
- 2026-02-11T16:19:21Z – coder – shell_pid=3777501 – lane=planned – Moved to planned
- 2026-02-11T17:57:54Z – coder – shell_pid=3777501 – lane=done – Review approved (gpt-5.3-codex high)
