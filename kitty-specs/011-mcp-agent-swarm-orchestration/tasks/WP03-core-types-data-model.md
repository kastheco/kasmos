---
work_package_id: "WP03"
title: "Core Types & Data Model"
phase: "Phase 0: Foundation"
lane: "planned"
dependencies: ["WP01"]
subtasks: ["T015", "T016", "T017", "T018", "T019", "T020", "T021"]
history:
  - date: "2026-02-13"
    agent: "controller"
    action: "Created WP prompt"
---

# WP03: Core Types & Data Model

## Implementation Command

```bash
spec-kitty implement WP03 --base WP01
```

## Objective

Define the new domain types required by the MCP swarm architecture: `AgentRole`, `WorkerEntry`, `WorkerStatus`, `KasmosMessage`, `MessageEvent`, `AuditEntry`, `DashboardState`, and `WorkerRegistry`. Create the `serve/` module directory with submodules for messages, audit, dashboard, and registry.

## Context

The existing `types.rs` has orchestration types (`OrchestrationRun`, `WorkPackage`, `Wave`, `WPState`, etc.) that are used by KEEP modules like `graph.rs`, `parser.rs`, and `state_machine.rs`. These must be preserved. The new types are additive — they live alongside existing types (in `types.rs` for simple enums/structs) or in new `serve/` submodules (for complex domain logic).

**Key reference**: `data-model.md` — contains exact struct definitions, field types, and parsing logic.

**New module structure being created:**
```
src/serve/
├── mod.rs        # Module declarations (stubs for now, full impl in WP07)
├── messages.rs   # KasmosMessage + MessageEvent + parse()
├── audit.rs      # AuditEntry + JSONL append
├── dashboard.rs  # DashboardState + DashboardWorker + ANSI formatter
└── registry.rs   # WorkerRegistry + CRUD methods
```

## Subtasks

### T015: Add AgentRole enum to types.rs

**Purpose**: Define the 5 agent roles used throughout the swarm architecture.

**Steps**:
1. Add to `types.rs`:
   ```rust
   #[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
   #[serde(rename_all = "lowercase")]
   pub enum AgentRole {
       Manager,
       Planner,
       Coder,
       Reviewer,
       Release,
   }

   impl AgentRole {
       /// Returns the OpenCode --agent flag value
       pub fn agent_flag(&self) -> &'static str {
           match self {
               Self::Manager => "manager",
               Self::Planner => "planner",
               Self::Coder => "coder",
               Self::Reviewer => "reviewer",
               Self::Release => "release",
           }
       }

       /// Returns whether this role is user-interactive
       pub fn is_interactive(&self) -> bool {
           matches!(self, Self::Manager | Self::Planner)
       }
   }

   impl std::fmt::Display for AgentRole {
       fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
           write!(f, "{}", self.agent_flag())
       }
   }
   ```

**Validation**: Serialization round-trip test

### T016: Add WorkerEntry and WorkerStatus to types.rs

**Purpose**: Define the worker tracking types used by the in-memory registry.

**Steps**:
1. Add to `types.rs`:
   ```rust
   #[derive(Debug, Clone)]
   pub struct WorkerEntry {
       pub wp_id: String,
       pub role: AgentRole,
       pub pane_name: String,
       pub spawned_at: chrono::DateTime<chrono::Utc>,
       pub status: WorkerStatus,
   }

   #[derive(Debug, Clone)]
   pub enum WorkerStatus {
       Active,
       Done,
       Errored(String),
       Aborted,
   }

   impl WorkerStatus {
       pub fn label(&self) -> &str {
           match self {
               Self::Active => "active",
               Self::Done => "done",
               Self::Errored(_) => "errored",
               Self::Aborted => "aborted",
           }
       }

       pub fn is_terminal(&self) -> bool {
           !matches!(self, Self::Active)
       }
   }
   ```

**Validation**: Compiles, label() returns expected strings

### T017: Create serve/messages.rs

**Purpose**: Implement structured message parsing for the `[KASMOS:<sender>:<event>]` protocol.

**Steps**:
1. Create `src/serve/` directory and `src/serve/mod.rs`:
   ```rust
   pub mod messages;
   pub mod audit;
   pub mod dashboard;
   pub mod registry;
   ```
2. Create `src/serve/messages.rs` with:
   ```rust
   use chrono::Utc;
   use regex::Regex;
   use serde::{Deserialize, Serialize};
   use std::sync::LazyLock;

   static MSG_RE: LazyLock<Regex> = LazyLock::new(|| {
       Regex::new(r"\[KASMOS:([^:]+):([^\]]+)\]\s*(.*)").unwrap()
   });

   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct KasmosMessage {
       pub index: usize,
       pub sender: String,
       pub event: MessageEvent,
       pub data: serde_json::Value,
       pub timestamp: chrono::DateTime<chrono::Utc>,
       pub raw_line: String,
   }

   #[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
   #[serde(rename_all = "SCREAMING_SNAKE_CASE")]
   pub enum MessageEvent {
       Started,
       Progress,
       Done,
       Error,
       ReviewPass,
       ReviewReject,
       NeedsInput,
   }
   ```
3. Implement `KasmosMessage::parse(line: &str, index: usize) -> Option<Self>`:
   - Strip ANSI escape codes first: `line.replace(|c: char| c == '\x1b', "")` or a proper ANSI strip regex `\x1b\[[0-9;]*m`
   - Apply `MSG_RE` to find groups
   - Parse group 2 as `MessageEvent` (case-insensitive match)
   - Parse group 3 as `serde_json::Value` (fall back to `Value::Null` if empty or invalid JSON)
   - Return `None` for non-matching lines (tolerant parser)
4. Implement `MessageEvent::from_str()` for the event string mapping
5. Implement a helper `strip_ansi(input: &str) -> String`

**Validation**: Parse valid messages, reject invalid ones, handle ANSI codes

### T018: Create serve/audit.rs

**Purpose**: Implement JSONL audit log persistence with file locking.

**Steps**:
1. Create `src/serve/audit.rs`:
   ```rust
   use chrono::Utc;
   use serde::{Deserialize, Serialize};
   use std::fs::{OpenOptions, create_dir_all};
   use std::io::Write;
   use std::path::Path;

   #[derive(Debug, Serialize, Deserialize)]
   pub struct AuditEntry {
       pub timestamp: chrono::DateTime<chrono::Utc>,
       pub actor: String,
       pub action: String,
       pub details: serde_json::Value,
   }

   impl AuditEntry {
       pub fn new(actor: impl Into<String>, action: impl Into<String>, details: serde_json::Value) -> Self {
           Self {
               timestamp: Utc::now(),
               actor: actor.into(),
               action: action.into(),
               details,
           }
       }
   }
   ```
2. Implement `pub fn append_audit(dir: &Path, entry: &AuditEntry) -> Result<()>`:
   - Ensure `dir` exists (create_dir_all)
   - Open `dir/messages.jsonl` with append mode
   - Acquire advisory lock: `nix::fcntl::flock(fd, FlockArg::LockExclusive)`
   - Serialize entry as single-line JSON + newline
   - Write to file
   - Lock is released when file handle is dropped
3. Use `nix` crate (already in deps with `fs` feature) for flock

**Validation**: Append multiple entries, verify JSONL format, verify concurrent-safe with flock

### T019: Create serve/dashboard.rs

**Purpose**: Implement the dashboard display state and ANSI formatter for the status pane.

**Steps**:
1. Create `src/serve/dashboard.rs`:
   ```rust
   use crate::types::AgentRole;
   use std::time::{Duration, Instant};

   pub struct DashboardState {
       pub feature: String,
       pub phase: String,
       pub workers: Vec<DashboardWorker>,
       pub last_event: Option<String>,
       pub start_time: Instant,
   }

   pub struct DashboardWorker {
       pub wp_id: String,
       pub role: AgentRole,
       pub status_label: String,
       pub elapsed: Duration,
   }
   ```
2. Implement `DashboardState::format_ansi(&self) -> String`:
   - Clear screen: `\x1b[2J\x1b[H`
   - Header: `" Swarm Status         elapsed: {elapsed}"`
   - Separator line
   - For each worker: `" {wp_id:<7} {role:<9} {progress_bar}  {status}"`
   - Progress bar: filled blocks `█` based on status (active=partial, done=full, error=red)
   - Separator line
   - Last event line: `" Last: {last_event} {time_ago}"`
   - Use ANSI colors: green for done, yellow for active, red for error, gray for pending
3. Implement `DashboardState::new(feature: &str) -> Self` with empty initial state
4. Implement `DashboardState::update_worker(&mut self, wp_id: &str, role: AgentRole, status: &str)`

**Validation**: Format produces valid ANSI output, visual inspection

### T020: Create serve/registry.rs [P]

**Purpose**: Implement the in-memory worker registry with CRUD operations.

**Steps**:
1. Create `src/serve/registry.rs`:
   ```rust
   use crate::types::{AgentRole, WorkerEntry, WorkerStatus};
   use std::collections::HashMap;

   pub struct WorkerRegistry {
       workers: HashMap<String, WorkerEntry>,
   }

   impl WorkerRegistry {
       pub fn new() -> Self {
           Self { workers: HashMap::new() }
       }

       pub fn register(&mut self, entry: WorkerEntry) {
           self.workers.insert(entry.wp_id.clone(), entry);
       }

       pub fn deregister(&mut self, wp_id: &str) -> Option<WorkerEntry> {
           self.workers.remove(wp_id)
       }

       pub fn get(&self, wp_id: &str) -> Option<&WorkerEntry> {
           self.workers.get(wp_id)
       }

       pub fn get_mut(&mut self, wp_id: &str) -> Option<&mut WorkerEntry> {
           self.workers.get_mut(wp_id)
       }

       pub fn list(&self) -> Vec<&WorkerEntry> {
           self.workers.values().collect()
       }

       pub fn active_count(&self) -> usize {
           self.workers.values()
               .filter(|w| matches!(w.status, WorkerStatus::Active))
               .count()
       }

       pub fn update_status(&mut self, wp_id: &str, status: WorkerStatus) -> bool {
           if let Some(worker) = self.workers.get_mut(wp_id) {
               worker.status = status;
               true
           } else {
               false
           }
       }
   }
   ```

**Validation**: CRUD operations work correctly, active_count is accurate

### T021: Write unit tests for message parsing

**Purpose**: Ensure the message parser handles all edge cases.

**Steps**:
1. In `serve/messages.rs`, add `#[cfg(test)] mod tests`:
   ```rust
   #[test]
   fn test_parse_valid_done_message() {
       let line = r#"[KASMOS:WP-01-coder:DONE] {"wp_id":"WP-01","summary":"Done"}"#;
       let msg = KasmosMessage::parse(line, 0).unwrap();
       assert_eq!(msg.sender, "WP-01-coder");
       assert_eq!(msg.event, MessageEvent::Done);
       assert_eq!(msg.index, 0);
   }

   #[test]
   fn test_parse_valid_no_data() {
       let line = "[KASMOS:WP-02-reviewer:REVIEW_PASS]";
       let msg = KasmosMessage::parse(line, 5).unwrap();
       assert_eq!(msg.event, MessageEvent::ReviewPass);
       assert_eq!(msg.data, serde_json::Value::Null);
   }

   #[test]
   fn test_parse_with_ansi_codes() {
       let line = "\x1b[32m[KASMOS:WP-01-coder:STARTED] {}\x1b[0m";
       let msg = KasmosMessage::parse(line, 1).unwrap();
       assert_eq!(msg.event, MessageEvent::Started);
   }

   #[test]
   fn test_parse_non_matching_line() {
       assert!(KasmosMessage::parse("just some output", 0).is_none());
       assert!(KasmosMessage::parse("", 0).is_none());
       assert!(KasmosMessage::parse("[NOT_KASMOS:foo:bar]", 0).is_none());
   }

   #[test]
   fn test_parse_all_event_types() {
       for event in ["STARTED", "PROGRESS", "DONE", "ERROR", "REVIEW_PASS", "REVIEW_REJECT", "NEEDS_INPUT"] {
           let line = format!("[KASMOS:test:{}] {{}}", event);
           let msg = KasmosMessage::parse(&line, 0);
           assert!(msg.is_some(), "Failed to parse event: {}", event);
       }
   }
   ```

**Validation**: All tests pass

## Test Strategy

- Unit tests for message parsing (T021)
- Compilation tests: all new types compile, derive macros work
- Serialization round-trips for AgentRole, MessageEvent
- Registry CRUD correctness

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| ANSI stripping regex misses some escape codes | Test with various terminal output samples |
| Message parsing false positives | Distinctive `[KASMOS:` prefix minimizes collisions |
| flock behavior differs on NFS/macOS | Document as Linux-primary, macOS best-effort |

## Review Guidance

- Verify `AgentRole` includes all 5 roles (Manager, Planner, Coder, Reviewer, Release)
- Verify `MessageEvent` uses `SCREAMING_SNAKE_CASE` serialization
- Verify message parse regex matches data-model.md: `\[KASMOS:([^:]+):([^\]]+)\]\s*(.*)`
- Verify ANSI stripping happens before regex matching
- Verify flock uses `FlockArg::LockExclusive` not `LockShared`

## Activity Log

| Date | Agent | Event |
|------|-------|-------|
| 2026-02-13 | controller | Created WP prompt |
