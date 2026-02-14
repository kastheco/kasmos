# Implementation Plan: MCP Agent Swarm Orchestration

**Branch**: `011-mcp-agent-swarm-orchestration` | **Date**: 2026-02-14 | **Spec**: `kitty-specs/011-mcp-agent-swarm-orchestration/spec.md`
**Input**: Feature specification from `/home/kas/dev/kasmos/kitty-specs/011-mcp-agent-swarm-orchestration/spec.md`

## Summary

Replace the legacy TUI orchestration flow with an MCP-driven swarm model where the manager agent controls lifecycle transitions, worker spawning, review loops, and release handoff. The `kasmos` command becomes a launcher for orchestration tabs, while `kasmos serve` runs as a manager-scoped MCP stdio subprocess. Feature execution ownership is enforced by a repository-wide lock, and orchestration audit logs are persisted per feature under `kitty-specs/<feature>/.kasmos/messages.jsonl` with rotation by size or age.

## Technical Context

**Language/Version**: Rust 2024 edition (latest stable)
**Primary Dependencies**: `tokio`, `clap`, `serde`, `serde_json`, `serde_yaml`, `rmcp`, `schemars`, `regex`, `kdl`, `nix`, `thiserror`, `anyhow`, `tracing`
**Storage**: Filesystem only; spec-kitty task files are SSOT for WP state, per-feature audit log at `kitty-specs/<feature>/.kasmos/messages.jsonl`
**Testing**: `cargo test` for unit and integration suites; scenario checks for launch, lock handling, and MCP tool behavior
**Target Platform**: Linux primary, macOS best-effort
**Project Type**: Single Rust binary (`crates/kasmos/`)
**Performance Goals**: Launch ready under 10s, event detection under 15s, error report under 30s, support 4+ concurrent workers
**Constraints**:
- Launch must preflight dependencies and fail fast with actionable guidance and non-zero exit on missing requirements
- `kasmos serve` runs as manager-spawned MCP stdio subprocess (no dedicated MCP tab process)
- Feature ownership lock is repository-wide across running processes
- Stale lock takeover requires explicit confirmation after 15 minutes timeout
- Audit log retention rotates when either threshold is hit: size > 512MB OR age > 14 days
- Metadata-only audit logging by default; full payloads only in opt-in debug mode
**Scale/Scope**: Platform-level workflow pivot touching launch, serve, prompting, workflow state, and operational safeguards

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| Rust 2024 edition | PASS | Plan keeps Rust 2024 across modules |
| tokio async runtime | PASS | MCP server and polling loops are async on tokio |
| ratatui for TUI | PASS | Existing TUI remains preserved and feature-gated per spec FR-024 |
| Zellij substrate | PASS | Launch/session flow and pane lifecycle remain Zellij-based |
| OpenCode primary agent | PASS | Manager, planner, coder, reviewer, release roles use OpenCode profile set |
| cargo test required | PASS | Unit and integration test scopes defined for changed subsystems |
| Linux primary, macOS best-effort | PASS | Platform support unchanged |
| Single binary distribution | PASS | `kasmos` remains one installable binary with subcommands |

## Engineering Alignment

Planning interrogation decisions accepted by stakeholder:

1. `kasmos serve` runtime model: manager-spawned MCP stdio subprocess
2. Audit log path: `kitty-specs/<feature>/.kasmos/messages.jsonl`
3. Missing dependency behavior: fail before launch with actionable guidance and non-zero exit
4. Duplicate binding prevention scope: repository-wide across running processes
5. Stale lock recovery: takeover only after timeout with explicit user confirmation
6. Default stale lock timeout: 15 minutes
7. Audit detail level: metadata default, debug mode enables full payload logging
8. Audit retention policy: rotate/prune when size exceeds 512MB or age exceeds 14 days
9. No-inference feature selection: CLI selection before any session or tab creation

No planning clarifications remain unresolved.

## Architecture Overview

### Runtime Topology

- `kasmos [spec-prefix]` performs dependency preflight, feature resolution, and lock acquisition attempt before creating tabs
- Orchestration tab hosts manager pane, message-log pane, dashboard pane, and dynamic worker area
- Manager process owns a dedicated `kasmos serve` MCP stdio subprocess for tool calls
- Workers never call kasmos MCP directly; they write status events through zellij pane tools to message-log

### Session Layout

```
Tab: orchestration
- manager (60%)
- message-log (20%)
- dashboard (20%)
- worker rows (dynamic, max parallel from config)
```

No dedicated MCP hosting tab is required in this design.

### Feature Ownership Locking

- Lock key: `<repo_root>::<feature_slug>`
- Scope: repository-wide across all running kasmos processes
- Record fields: owner id, session/tab metadata, acquired time, heartbeat time, expires time
- Stale policy: stale when heartbeat exceeds 15 minutes
- Takeover policy: present stale-owner details and require explicit user confirmation before stealing lock

### Manager Orchestration Loop

1. Load workflow state from spec artifacts and task lanes
2. Present next action and confirmation gate to user
3. Spawn worker(s) via `spawn_worker`
4. Wait using blocking `wait_for_event` (with bounded timeout)
5. Process event, transition WP status, emit manager status update
6. Repeat until stage completion, then pause for user confirmation

### Communication Protocol

Workers send structured messages in message-log pane:

```
[KASMOS:<sender>:<event>] <json_payload>
```

Supported events include `STARTED`, `PROGRESS`, `DONE`, `ERROR`, `REVIEW_PASS`, `REVIEW_REJECT`, `NEEDS_INPUT`.

### Audit Logging and Retention

- Storage path: `kitty-specs/<feature>/.kasmos/messages.jsonl`
- Default payload: metadata only (`timestamp`, `actor`, `action`, `wp_id`, `status`, `summary`)
- Debug payload mode: gated by config flag, includes prompts/tool args for incident analysis
- Retention trigger: rotate/prune when either condition is true:
  - active log > 512MB
  - entry age > 14 days

### Failure and Recovery

- Missing dependencies at launch -> fail fast before tab/session creation
- Lock already held and fresh -> refuse bind and show active owner details
- Lock stale -> offer takeover (confirmation required)
- `kasmos serve` subprocess crash -> manager pauses automation and surfaces restart guidance
- Worker pane loss or crash -> mark aborted, notify user, and offer respawn/skip path
- Review rejection loops -> enforce configurable cap (default 3), then pause for user intervention

## Project Structure

### Documentation (this feature)

```
kitty-specs/011-mcp-agent-swarm-orchestration/
|- spec.md
|- plan.md
|- research.md
|- data-model.md
|- quickstart.md
|- contracts/
|  \- kasmos-serve.json
\- tasks.md
```

### Source Code (repository root)

```
crates/kasmos/
|- Cargo.toml
\- src/
   |- main.rs
   |- config.rs
   |- launch/
   |  |- mod.rs
   |  |- detect.rs
   |  |- layout.rs
   |  \- session.rs
   |- serve/
   |  |- mod.rs
   |  |- registry.rs
   |  |- messages.rs
   |  |- audit.rs
   |  |- dashboard.rs
   |  |- lock.rs
   |  \- tools/
   |     |- spawn_worker.rs
   |     |- despawn_worker.rs
   |     |- list_workers.rs
   |     |- read_messages.rs
   |     |- wait_for_event.rs
   |     |- workflow_status.rs
   |     |- transition_wp.rs
   |     |- list_features.rs
   |     \- infer_feature.rs
   |- setup/
   |  \- mod.rs
   |- prompt.rs
   |- zellij.rs
   |- parser.rs
   |- state_machine.rs
   |- graph.rs
   \- types.rs

config/profiles/kasmos/
|- opencode.jsonc
\- agent/
   |- manager.md
   |- planner.md
   |- coder.md
   |- reviewer.md
   \- release.md
```

**Structure Decision**: Keep a single-binary Rust workspace crate and add focused modules for `launch/`, `serve/`, and `setup/`; preserve existing reusable domain modules and keep legacy TUI code disconnected behind feature gates.

## MCP Tool Contracts

The MCP surface remains 9 tools, defined in `kitty-specs/011-mcp-agent-swarm-orchestration/contracts/kasmos-serve.json`:

- `spawn_worker`
- `despawn_worker`
- `list_workers`
- `read_messages`
- `wait_for_event`
- `workflow_status`
- `transition_wp`
- `list_features`
- `infer_feature`

Contract updates in this plan include explicit lock-conflict responses, audit metadata fields, and timeout semantics.

## Data Model Snapshot

Detailed model is documented in `kitty-specs/011-mcp-agent-swarm-orchestration/data-model.md`. Key entities:

- `FeatureBindingLock`
- `WorkerEntry`
- `KasmosMessage`
- `AuditEntry`
- `AuditPolicy`
- `WorkflowSnapshot`

## Phase Outputs

### Phase 0: Research

Produce `kitty-specs/011-mcp-agent-swarm-orchestration/research.md` with finalized decisions, rationale, and alternatives for runtime model, lock scope, retention, and operational behavior.

### Phase 1: Design and Contracts

- Produce `kitty-specs/011-mcp-agent-swarm-orchestration/data-model.md`
- Produce `kitty-specs/011-mcp-agent-swarm-orchestration/contracts/kasmos-serve.json`
- Produce `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`
- Update agent context via `spec-kitty agent context update-context --feature 011-mcp-agent-swarm-orchestration --agent-type opencode`

## Constitution Check (Post-Design Recheck)

Post-design status remains PASS for all constitution principles. No exceptions or complexity waivers are required.

## Complexity Tracking

No constitution violations or exception justifications were introduced.
