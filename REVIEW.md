# REVIEW.md - WP12: Integration, Legacy Preservation, and Acceptance Hardening

**Feature**: 011-mcp-agent-swarm-orchestration
**Branch**: `011-mcp-agent-swarm-orchestration-WP12`
**Dependencies**: WP07, WP08, WP09, WP10, WP11 (all merged to main)

## Summary

WP12 is the final integration and acceptance hardening pass for the MCP agent swarm orchestration feature. It merges outputs from WP07-WP11, adds integration/acceptance tests for lock conflict, audit logging, and selector pre-launch gating, preserves the legacy TUI build path, aligns documentation, and produces FR/SC traceability.

## Changes Overview

64 files changed, ~8145 insertions, ~1146 deletions (cumulative across all merged WPs).

### WP12-specific changes (commit `fb920ef3`)

| File | What changed |
|------|-------------|
| `crates/kasmos/src/launch/mod.rs` | Refactored launch flow: selector runs before preflight, lock always released on bootstrap failure, testable prompt injection, 4 new integration tests (T072) |
| `crates/kasmos/src/serve/audit.rs` | Complete rewrite: AuditEntry, AuditWriter, JSONL persistence, retention rotation, debug-payload redaction, 6 unit tests (T071) |
| `crates/kasmos/src/serve/mod.rs` | AuditWriter lifecycle, emit_audit/emit_audit_error methods, error audit paths on spawn/despawn |
| `crates/kasmos/src/serve/tools/spawn_worker.rs` | Audit entry emission on spawn |
| `crates/kasmos/src/serve/tools/despawn_worker.rs` | Audit entry emission on despawn |
| `crates/kasmos/src/setup/mod.rs` | Clippy fix (vec_init_then_push) |
| `README.md` | MCP-first command surface, legacy TUI feature gate documentation |
| `kitty-specs/.../quickstart.md` | Build matrix, list/status scenarios updated |
| `kitty-specs/.../traceability.md` | NEW: FR->WP mapping (FR-001 to FR-033), SC->evidence mapping (SC-001 to SC-010) |

## Subtask Completion

| Subtask | Description | Status |
|---------|-------------|--------|
| T069 | Legacy TUI compile path preservation | Done - both `cargo build` and `cargo build --features tui` pass |
| T070 | Lock conflict and stale takeover tests | Done - `serve::lock::tests` covers both scenarios |
| T071 | Audit logging modes and retention tests | Done - `serve::audit::tests` covers metadata-only, debug, size rotation, age rotation |
| T072 | Selector pre-launch gate tests | Done - 4 integration tests in `launch::tests` |
| T073 | README/quickstart alignment | Done - README and quickstart reflect final command surface |
| T074 | FR/SC traceability checklist | Done - `traceability.md` maps all 33 FRs and 10 SCs |

## Verification Matrix

All checks passed:

| Check | Result |
|-------|--------|
| `cargo build` | Passed |
| `cargo test` | 298 tests passed |
| `cargo build --features tui` | Passed |
| `cargo test --features tui` | 327 lib + 92 main tests passed |
| `cargo clippy -p kasmos -- -D warnings` | Passed (zero warnings) |
| `kasmos --help` | Shows MCP-first command surface |
| `kasmos list` | Lists features from kitty-specs |
| `kasmos status 011` | Graceful failure (no state file; expected) |
| `kasmos setup` | Reports dependency checks (fails on missing pane-tracker; expected) |

## Key Design Decisions in WP12

1. **Selector before preflight**: Launch flow now runs feature selection BEFORE any Zellij or preflight checks. This ensures no side-effects occur before the user has chosen a feature.

2. **Lock safety on failure**: If bootstrap fails after acquiring a lock, the lock is always released in the error path. Prevents stale locks from failed launches.

3. **Testable prompt injection**: The launch flow accepts an optional prompt override for testing, allowing integration tests to simulate user selection without actual stdin.

4. **Audit debug redaction**: In metadata-only mode (default), audit entries omit the `debug_payload` field entirely rather than setting it to null.

## Review Focus Areas

Per the WP12 spec Review Guidance:

1. Both build configs succeed (default and tui) -- verified
2. Lock scenario tests are deterministic (temp dirs, no timing dependencies)
3. Audit scenario tests cover both modes and both retention triggers
4. Selector gate test proves no Zellij commands execute before selection
5. README accurately describes new command surface
6. FR/SC traceability checklist is complete (33 FRs, 10 SCs)
7. All locked decisions (1-9 from plan.md) are validated

## How to Review

```bash
# Start the review workflow
spec-kitty agent workflow review WP12 --agent <your-name>

# Or in opencode:
/kasmos.review WP12
```
