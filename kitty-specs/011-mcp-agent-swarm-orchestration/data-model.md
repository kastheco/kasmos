# Data Model: MCP Agent Swarm Orchestration

**Feature**: 011-mcp-agent-swarm-orchestration  
**Date**: 2026-02-13  
**Status**: Complete

---

## Entity Relationship Overview

```
FeatureSpec 1──* WorkPackage *──1 Wave
     │                │
     │                │ assigned_to
     │                ▼
     │          WorkerAgent *──1 Pane
     │                │
     │                │ sends
     │                ▼
     └──────── Message *──1 MessageLog
                      │
                      ▼
               AuditLogEntry
```

---

## Entities

### 1. FeatureSpec

The top-level unit of work that kasmos orchestrates. Maps to a `kitty-specs/NNN-slug/` directory.

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| number          | String          | Three-digit prefix (e.g., "011")                     | meta.json                 |
| slug            | String          | Full kebab-case name (e.g., "011-mcp-agent-swarm")   | meta.json                 |
| friendly_name   | String          | Human-readable title                                 | meta.json                 |
| mission         | Enum            | software-dev, research                               | meta.json                 |
| phase           | Enum            | specify, clarify, plan, analyze, tasks, implement, review, release | Derived from file presence |
| has_spec        | Boolean         | spec.md exists and is non-empty                      | Filesystem                |
| has_plan        | Boolean         | plan.md exists                                       | Filesystem                |
| has_tasks       | Boolean         | tasks/ contains WP files                             | Filesystem                |
| target_branch   | String          | Branch to merge into (default: "main")               | meta.json                 |

**Relationships**:
- Has many WorkPackages (via tasks/ directory)
- Has many AuditLogEntries (via orchestration-audit.log)
- Owns one MessageLog per session (via .kasmos/messages.jsonl)

---

### 2. WorkPackage (WP)

A unit of implementation work. Each WP has a task file in the feature's `tasks/` directory with YAML frontmatter defining its state.

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| id              | String          | Identifier (e.g., "WP01")                            | Task file name            |
| title           | String          | Descriptive title                                    | Task file frontmatter     |
| lane            | Enum            | pending, active, for_review, done, failed             | Task file frontmatter     |
| wave            | Integer         | Wave number for execution ordering                   | Task file frontmatter     |
| dependencies    | List<String>    | IDs of WPs that must complete first                  | Task file frontmatter     |
| assignee        | Option<String>  | Pane ID of current worker (if active)                | In-memory (kasmos serve)  |
| review_count    | Integer         | Number of review iterations completed                | Task file frontmatter     |

**State Machine** (lane transitions):
```
pending → active → for_review → done
                 ↘ failed       ↗ (rework)
         active ← for_review (rejected)
```

Valid transitions enforced by `state_machine.rs`:
- pending → active (worker spawned)
- active → for_review (implementation complete)
- active → failed (worker error/abort)
- for_review → done (review approved)
- for_review → active (review rejected, rework)
- failed → active (retry)

**Relationships**:
- Belongs to one FeatureSpec
- Belongs to one Wave
- May be assigned to one WorkerAgent
- Depends on zero or more other WorkPackages

---

### 3. Wave

An ordered group of WorkPackages that can execute concurrently. All WPs in wave N must complete before wave N+1 begins.

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| index           | Integer         | Wave number (0-based)                                | Computed from WP deps     |
| state           | Enum            | pending, active, completed                           | Derived from WP states    |
| wp_ids          | List<String>    | WorkPackage IDs in this wave                         | Computed by graph.rs      |

**Derivation**: Waves are computed by `graph.rs::compute_waves()` from WP dependency relationships using topological sort. Not stored — recomputed on demand.

**Relationships**:
- Contains one or more WorkPackages
- Ordered sequentially (wave N < wave N+1)

---

### 4. WorkerAgent

A running agent instance in a Zellij pane, performing a specific task.

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| pane_id         | String          | Zellij terminal pane identifier (e.g., "terminal_5") | zellij-pane-tracker JSON  |
| pane_name       | String          | Display name (e.g., "WP01-coder")                    | Zellij pane title         |
| role            | Enum            | coder, reviewer, release                             | Spawn parameters          |
| wp_id           | String          | Assigned work package ID                             | Spawn parameters          |
| status          | Enum            | active, complete, errored, aborted                   | In-memory + monitoring    |
| started_at      | Timestamp       | When the worker was spawned                          | In-memory                 |
| worktree_path   | Option<String>  | Git worktree path for isolation                      | Computed at spawn         |
| tab_name        | Option<String>  | Zellij tab containing this worker                    | Tracked at spawn          |

**Lifecycle**:
```
(spawn) → active → complete → (despawn)
                  → errored → (despawn + report)
                  → aborted → (despawn + report)
```

**Relationships**:
- Assigned to one WorkPackage
- Runs in one Pane (Zellij terminal)
- Sends Messages to the MessageLog

---

### 5. Message

A structured communication unit between workers and the manager, written to the message-log pane.

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| timestamp       | ISO 8601        | When the message was sent                            | Generated by sender       |
| sender          | String          | Pane name or "manager"                               | Agent identity            |
| event           | Enum            | task_complete, error, status_update, review_result, spawn, transition | Protocol definition |
| data            | JSON Object     | Event-specific payload                               | Varies by event type      |
| line_number     | Option<Integer> | Position in message-log pane scrollback              | Computed by reader        |

**Event Types**:
- `task_complete`: Worker finished its assigned task. Data: `{wp_id, status, summary}`
- `error`: Worker encountered an error. Data: `{wp_id, error_type, message}`
- `status_update`: Periodic progress report. Data: `{wp_id, progress, detail}`
- `review_result`: Reviewer's verdict. Data: `{wp_id, decision: "approve"|"reject", findings}`
- `spawn`: Manager spawned a worker. Data: `{wp_id, role, pane_id}`
- `transition`: WP state changed. Data: `{wp_id, from_state, to_state, reason}`

**Wire Format** (in message-log pane):
```
echo "[KASMOS:<sender>:<event>] <json_data>"
```

**Relationships**:
- Sent by one WorkerAgent (or manager)
- Written to the MessageLog
- May trigger an AuditLogEntry

---

### 6. MessageLog

The communication channel between workers and the manager. Dual-layered: pane (real-time) + file (persistent).

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| pane_id         | String          | Zellij pane for real-time messages                   | Session layout            |
| file_path       | Path            | `.kasmos/messages.jsonl` for persistence              | Convention                |
| last_read_line  | Integer         | Manager's read cursor in pane scrollback             | In-memory (manager)       |

**Relationships**:
- Contains many Messages
- Read by the Manager Agent
- Written to by Worker Agents

---

### 7. AuditLogEntry

A persistent record of orchestration decisions for traceability (FR-027).

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| timestamp       | ISO 8601        | When the action occurred                             | Generated                 |
| action          | String          | What happened (e.g., "spawn_worker", "transition_wp")| kasmos serve              |
| actor           | String          | Who initiated (e.g., "manager", "WP01-coder")       | Caller identity           |
| details         | JSON Object     | Action-specific context                              | Varies                    |
| feature         | String          | Feature slug                                         | Session binding           |

**Storage**: Appended to `kitty-specs/<feature>/orchestration-audit.log` (one JSON line per entry). Committed to git for full traceability.

**Relationships**:
- Belongs to one FeatureSpec
- May reference one WorkPackage
- May reference one WorkerAgent

---

### 8. Session

The runtime context for a kasmos orchestration session within Zellij.

| Attribute       | Type            | Description                                          | Source                    |
|-----------------|-----------------|------------------------------------------------------|---------------------------|
| session_name    | String          | Zellij session name (default: "kasmos")              | Launch parameter          |
| bound_feature   | Option<String>  | Feature slug currently being orchestrated            | Manager binding           |
| active_phase    | Enum            | planning, implementing, releasing                    | Derived from feature state|
| manager_pane    | String          | Pane ID of the manager agent                         | Session layout            |
| msglog_pane     | String          | Pane ID of the message-log pane                      | Session layout            |
| worker_panes    | List<String>    | Pane IDs of active workers                           | In-memory registry        |
| tabs            | List<String>    | Tab names in the session                             | Zellij query-tab-names    |

**Relationships**:
- Binds to one FeatureSpec
- Contains one Manager Agent (pane)
- Contains one MessageLog (pane)
- Contains zero or more WorkerAgents (panes)

---

## Configuration Entities

### KasmosConfig

Runtime configuration for the kasmos binary.

| Attribute             | Type    | Description                                     | Default        |
|-----------------------|---------|-------------------------------------------------|----------------|
| max_concurrent_workers| Integer | Maximum worker panes active simultaneously      | 4              |
| workers_per_row       | Integer | Max workers in a single layout row              | 4              |
| poll_interval_secs    | Integer | Seconds between scrollback/message polls        | 10             |
| max_review_iterations | Integer | Max reject-rework cycles before escalation      | 3              |
| opencode_profile      | String  | OpenCode profile name for agent spawning        | "kas"          |
| message_log_path      | Path    | Path to persistent message file                 | ".kasmos/messages.jsonl" |
| audit_log_enabled     | Boolean | Whether to write orchestration audit entries    | true           |

### AgentProfile

Configuration for a specific agent role (defined in OpenCode profile).

| Attribute       | Type            | Description                                     |
|-----------------|-----------------|-------------------------------------------------|
| name            | String          | Agent identifier (manager, coder, reviewer, release) |
| model           | String          | LLM model identifier                           |
| temperature     | Float           | Sampling temperature                            |
| reasoning_effort| Enum            | low, medium, high                               |
| mcp_servers     | List<String>    | MCP servers this agent can access               |
| permissions     | Object          | Read/write/edit/bash permissions                |
