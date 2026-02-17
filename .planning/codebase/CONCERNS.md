# Codebase Concerns

**Analysis Date:** 2026-02-16

## Tech Debt

**Config Duplication - Legacy Flat Fields vs Sectioned Config:**
- Issue: `Config` still carries legacy flat fields alongside sectioned config structs.
- Files: `crates/kasmos/src/config.rs`
- Impact: Two representations can drift and complicate maintenance.
- Fix approach: Remove legacy flat fields once all callers rely only on sectioned fields.

**Empty Dashboard Module:**
- Issue: `crates/kasmos/src/serve/dashboard.rs` is effectively a placeholder while dashboard formatting logic lives in `crates/kasmos/src/serve/tools/wait_for_event.rs`.
- Impact: Module boundaries are unclear.
- Fix approach: Move dashboard formatting into `serve/dashboard.rs` or remove the module.

## Known Bugs

**Message Cursor Re-parse on Every Poll:**
- Symptoms: `read_messages_since()` reparses full pane scrollback on each poll and then filters by cursor.
- Files: `crates/kasmos/src/serve/messages.rs`
- Trigger: Long-running sessions with large message history.
- Workaround: None; functional correctness is intact but performance degrades over time.

## Security Considerations

**Unsafe libc Calls in Locking + Env Tests:**
- Risk: Direct `unsafe` usage in file locking and test env-var mutation paths.
- Files: `crates/kasmos/src/serve/lock.rs`, `crates/kasmos/src/config.rs`, `crates/kasmos/src/setup/mod.rs`, `crates/kasmos/src/launch/session.rs`
- Current mitigation: Return codes are checked; env-var mutation is test-only.
- Recommendation: Move test env-var mutation to an isolated helper and keep unsafe blocks minimal and documented.

## Fragile Areas

**MCP Message Protocol Parsing:**
- Files: `crates/kasmos/src/serve/messages.rs`
- Why fragile: Regex parsing against free-form scrollback can silently drop malformed lines.
- Safe modification: Update `KNOWN_EVENTS` and add parser tests whenever event shapes change.

**Zellij Session Bootstrap Assumptions:**
- Files: `crates/kasmos/src/launch/session.rs`, `crates/kasmos/src/launch/layout.rs`
- Why fragile: Behavior depends on Zellij CLI capabilities and session/tab context.
- Safe modification: Validate inside/outside-Zellij flows and real session lifecycle when changing launch logic.

## Dependencies at Risk

**`serde_yml` (0.0.12) - Pre-1.0:**
- Risk: Frontmatter parsing relies on a pre-1.0 crate.
- Files: `crates/kasmos/Cargo.toml`, `crates/kasmos/src/parser.rs`, `crates/kasmos/src/serve/tools/transition_wp.rs`
- Migration plan: Pin and monitor releases; parsing surface is narrow.

**`rmcp` (0.15):**
- Risk: MCP SDK and macros are pre-1.0 and may introduce breaking changes.
- Files: `crates/kasmos/Cargo.toml`, `crates/kasmos/src/serve/mod.rs`
- Migration plan: Track upstream and keep tool-schema tests current.

## Missing Critical Features

**No Pane Lifecycle Enforcement in MCP Server:**
- Problem: `spawn_worker` records workers, but actual pane creation is delegated to the manager agent.
- Files: `crates/kasmos/src/serve/tools/spawn_worker.rs`
- Impact: Registry entries can exist without a real pane if manager orchestration drifts.

**Legacy Health/Shutdown Coordinators Removed:**
- Status: `health.rs` and `shutdown.rs` were removed with the legacy orchestration engine.
- Impact: MCP runtime currently relies on manager-driven supervision and normal process shutdown behavior.

## Test Coverage Gaps

**Untested Source Files (no `#[cfg(test)]` module):**
- What's not tested: `feature_arg.rs`, `lib.rs`, `list_specs.rs`, `main.rs`, `status.rs`, `serve/dashboard.rs`, `serve/registry.rs`, `serve/tools/spawn_worker.rs`, `serve/tools/list_workers.rs`, `serve/tools/list_features.rs`, `serve/tools/infer_feature.rs`, `serve/tools/despawn_worker.rs`
- Risk: Core command and worker-registry paths have less direct unit coverage.

**No Full End-to-End Test Harness:**
- What's not tested: Full launch + manager + MCP tool flow against real Zellij.
- Files: No dedicated `tests/` integration harness.
- Risk: Cross-process timing and CLI integration regressions can slip through unit-only coverage.

**Degraded Message Mode Not Tested:**
- What's not tested: Fallback scrollback path when pane-tracker is unavailable.
- Files: `crates/kasmos/src/serve/messages.rs`
- Risk: Fallback behavior can diverge from primary pane-tracker output.

---

*Concerns audit: 2026-02-16*
