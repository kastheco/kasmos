# Data Model: MCP Agent Swarm Orchestration

## Overview

This model defines runtime entities required for launch orchestration, worker lifecycle, feature ownership locking, and audit persistence.

## Entity: FeatureBindingLock

- Purpose: Ensure a single active manager owns a feature in a repository at any time.
- Identity:
  - Primary key: `lock_key` (`<repo_root>::<feature_slug>`)
- Fields:
  - `lock_key: String`
  - `repo_root: String`
  - `feature_slug: String`
  - `owner_id: String` (stable process/session token)
  - `owner_session: String`
  - `owner_tab: String`
  - `acquired_at: DateTime<Utc>`
  - `last_heartbeat_at: DateTime<Utc>`
  - `expires_at: DateTime<Utc>`
  - `status: LockStatus` (`active | stale | released`)
- Validation rules:
  - Only one `active` lock per `lock_key`
  - `expires_at = last_heartbeat_at + stale_timeout`
  - Default stale timeout is 15 minutes
- State transitions:
  - `released -> active` on successful acquisition
  - `active -> stale` when heartbeat misses timeout
  - `stale -> active` on confirmed takeover
  - `active/stale -> released` on clean shutdown

## Entity: WorkerEntry

- Purpose: Track active and completed worker panes managed by one manager.
- Identity:
  - Primary key: `wp_id + role`
- Fields:
  - `wp_id: String`
  - `role: AgentRole` (`planner | coder | reviewer | release`)
  - `pane_name: String`
  - `pane_id: Option<String>`
  - `worktree_path: Option<String>`
  - `status: WorkerStatus` (`active | done | errored | aborted`)
  - `spawned_at: DateTime<Utc>`
  - `updated_at: DateTime<Utc>`
  - `last_event: Option<MessageEvent>`
- Validation rules:
  - `worktree_path` required for coder role
  - `pane_name` must be unique within owning orchestration tab

## Entity: KasmosMessage

- Purpose: Parsed structured event line from message-log pane.
- Identity:
  - Primary key: `message_index` (monotonic per manager)
- Fields:
  - `message_index: u64`
  - `sender: String`
  - `event: MessageEvent`
  - `payload: JsonValue`
  - `timestamp: DateTime<Utc>`
  - `raw_line: String`
- Validation rules:
  - Must match format: `[KASMOS:<sender>:<event>] <json_payload>`
  - `event` must map to known enum values

## Entity: AuditEntry

- Purpose: Persisted orchestration trail for post-session diagnostics.
- Storage path:
  - `kitty-specs/<feature>/.kasmos/messages.jsonl`
- Fields:
  - `timestamp: DateTime<Utc>`
  - `actor: String` (`manager`, `kasmos-serve`, or worker id)
  - `action: String` (`spawn_worker`, `transition_wp`, etc.)
  - `feature_slug: String`
  - `wp_id: Option<String>`
  - `status: String`
  - `summary: String`
  - `details: JsonValue` (metadata by default)
  - `debug_payload: Option<JsonValue>` (present only in debug mode)
- Validation rules:
  - `debug_payload` must be omitted unless debug logging is enabled
  - Writes must be append-only within a file generation

## Entity: AuditPolicy

- Purpose: Retention and payload policy controls.
- Fields:
  - `metadata_only_default: bool` (default `true`)
  - `debug_full_payload_enabled: bool` (default `false`)
  - `max_bytes: u64` (default `536870912`)
  - `max_age_days: u32` (default `14`)
  - `trigger_mode: TriggerMode` (`either_threshold`)
- Validation rules:
  - Rotation/pruning triggers when either size or age threshold is reached

## Entity: WorkflowSnapshot

- Purpose: Report current feature phase and wave execution state.
- Fields:
  - `feature_slug: String`
  - `phase: String` (`spec_only | clarifying | planned | analyzing | tasked | implementing | reviewing | releasing | complete`)
  - `waves: Vec<WaveStatus>`
  - `active_workers: Vec<WorkerEntry>`
  - `last_event_at: Option<DateTime<Utc>>`
- Phase derivation notes:
  - `clarifying` and `analyzing` are optional planning phases; smaller features may skip directly from `spec_only` to `planned` to `tasked`
  - Phase is derived from artifact presence (e.g., spec.md exists without plan.md -> `spec_only` or `clarifying`), not from WP lane states
  - Workflow phases and WP lane states are orthogonal concepts; the Lane Translation Protocol (below) applies only to WP states

## Relationships

- One `FeatureBindingLock` controls one active manager per feature key.
- One manager owns many `WorkerEntry` records.
- Many `KasmosMessage` records reference zero or one `WorkerEntry` by `wp_id` and sender.
- Many `AuditEntry` records belong to one feature slug and optional WP.
- One `AuditPolicy` is loaded from config and applied to all `AuditEntry` writes.

## Lane Translation Protocol

Kasmos uses its own orchestration state vocabulary internally and in the MCP contract. Spec-kitty task files use a different lane vocabulary. Translation occurs at the file I/O boundary (inside `transition_wp` writes and `workflow_status` reads).

| Kasmos State | Spec-Kitty Lane | Direction | Notes |
|--------------|-----------------|-----------|-------|
| `pending` | `planned` | Bidirectional | WP ready but not started |
| `active` | `doing` | Bidirectional | Worker assigned and executing |
| `for_review` | `for_review` | Bidirectional | Shared term, no translation needed |
| `done` | `done` | Bidirectional | Shared term, no translation needed |
| `rework` | `doing` | Write-only | Written as `doing`; rework context is preserved in the audit log (`reason` field), not the lane name. On read-back, `doing` with prior `for_review` history implies rework. |

- `transition_wp` translates kasmos state to spec-kitty lane before writing task file frontmatter.
- `workflow_status` translates spec-kitty lane to kasmos state when reading task file frontmatter.
- The audit log always records the kasmos vocabulary (e.g., `rework`, not `doing`) for precise history.

## Scale Assumptions

- Concurrent workers: 4+ expected, configurable upper bound validated in config.
- Message volume: bursty but line-oriented; parser must handle thousands of lines without duplication.
- Audit retention: bounded by 512MB and 14-day age using either-threshold policy.
