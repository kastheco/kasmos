---
work_package_id: WP08
title: Message Log Parsing and Event Waiting
lane: "doing"
dependencies:
- WP04
base_branch: 011-mcp-agent-swarm-orchestration-WP04
base_commit: a02df49238a89b34cf57dc156237af2bad587046
created_at: '2026-02-15T01:41:35.969048+00:00'
subtasks:
- T045
- T046
- T047
- T048
- T049
- T050
- T075
phase: Phase 2 - Safety, State, and Audit Guarantees
assignee: ''
agent: "coder"
shell_pid: "571423"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP08 - Message Log Parsing and Event Waiting

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP08 --base WP04
```

---

## Objectives & Success Criteria

Implement `read_messages` and `wait_for_event` MCP tools with structured parsing, incremental cursors, timeout behavior, and degraded-mode fallback. After this WP:

1. Structured messages matching `[KASMOS:<sender>:<event>] <json>` are parsed correctly
2. `read_messages` returns messages since a cursor, with optional filtering, no duplicates
3. `wait_for_event` blocks until matching event or timeout, reports elapsed time
4. When pane-tracker is unavailable, system falls back to degraded polling with warning
5. Manager decision events are written to message-log pane for real-time transparency

## Context & Constraints

- **Depends on WP04**: Serve framework and typed tool structs available
- **Depends on WP06**: Audit writer available for logging
- **Contract**: `read_messages` and `wait_for_event` in `contracts/kasmos-serve.json`
- **Communication protocol**: Workers write `[KASMOS:<sender>:<event>] <json_payload>` to the msg-log pane using zellij-pane-tracker's `run-in-pane` capability
- **Events**: STARTED, PROGRESS, DONE, ERROR, REVIEW_PASS, REVIEW_REJECT, NEEDS_INPUT
- **Data model**: `KasmosMessage` entity with message_index, sender, event, payload, timestamp

## Subtasks & Detailed Guidance

### Subtask T045 - Implement structured message parser

**Purpose**: Parse `[KASMOS:<sender>:<event>] <json_payload>` lines from the message-log pane scrollback, handling ANSI escape codes and malformed lines.

**Steps**:
1. Create `crates/kasmos/src/serve/messages.rs`:
   ```rust
   use regex::Regex;
   use std::sync::LazyLock;

   static MSG_PATTERN: LazyLock<Regex> = LazyLock::new(|| {
       Regex::new(r"\[KASMOS:([^:]+):([^\]]+)\]\s*(.*)").unwrap()
   });

   pub fn parse_message(line: &str, index: u64) -> Option<KasmosMessage> {
       // Strip ANSI escape codes first
       let clean = strip_ansi(line);
       let caps = MSG_PATTERN.captures(&clean)?;
       let sender = caps.get(1)?.as_str().to_string();
       let event = caps.get(2)?.as_str().to_string();
       let payload_str = caps.get(3)?.as_str();
       let payload = serde_json::from_str(payload_str).unwrap_or(serde_json::Value::Null);
       Some(KasmosMessage {
           message_index: index,
           sender,
           event,
           payload,
           timestamp: chrono::Utc::now(),
           raw_line: line.to_string(),
       })
   }
   ```
2. ANSI stripping: Terminal scrollback contains escape sequences. Strip them before parsing.
   ```rust
   fn strip_ansi(s: &str) -> String {
       // Use regex or a dedicated crate like `strip-ansi-escapes`
       let ansi_re = Regex::new(r"\x1b\[[0-9;]*[a-zA-Z]").unwrap();
       ansi_re.replace_all(s, "").to_string()
   }
   ```
3. Be strict: only lines matching the exact `[KASMOS:...]` pattern are parsed. All other lines are ignored (they're normal terminal output from workers).
4. Validate that `event` maps to a known enum value. Unknown events should be preserved but flagged.

**Files**: `crates/kasmos/src/serve/messages.rs`
**Validation**: Known message format parses correctly. Malformed lines return None. ANSI codes stripped.

### Subtask T046 - Implement read_messages cursor semantics

**Purpose**: Read messages from the msg-log pane with incremental cursor tracking and optional filtering.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/read_messages.rs`:
   ```rust
   pub async fn handle(
       input: ReadMessagesInput,
       state: &KasmosServer,
   ) -> Result<ReadMessagesOutput> {
       // 1. Read scrollback from msg-log pane
       let scrollback = read_pane_scrollback("msg-log").await?;
       // 2. Parse all message lines
       let all_messages = parse_scrollback(&scrollback);
       // 3. Filter by cursor (since_index)
       let since = input.since_index.unwrap_or(0);
       let mut messages: Vec<_> = all_messages.into_iter()
           .filter(|m| m.message_index >= since)
           .collect();
       // 4. Apply optional filters
       if let Some(ref wp_filter) = input.filter_wp {
           messages.retain(|m| m.payload.get("wp_id")
               .and_then(|v| v.as_str()) == Some(wp_filter.as_str()));
       }
       if let Some(ref event_filter) = input.filter_event {
           messages.retain(|m| m.event == *event_filter);
       }
       // 5. Compute next_index
       let next_index = messages.last()
           .map(|m| m.message_index + 1)
           .unwrap_or(since);
       Ok(ReadMessagesOutput { ok: true, messages, next_index })
   }
   ```
2. Scrollback reading: Use zellij-pane-tracker's `dump-pane` capability to read the msg-log pane contents.
3. Message indexing: Each message gets a monotonically increasing index based on its position in the scrollback. The cursor (`since_index`) allows the manager to read only new messages.
4. No duplicates: The index-based cursor ensures messages aren't processed twice.

**Files**: `crates/kasmos/src/serve/tools/read_messages.rs`
**Validation**: Messages are returned in order. Cursor filters work. No duplicates.

### Subtask T047 - Implement wait_for_event bounded blocking loop

**Purpose**: Block until a matching event appears in the message log or timeout is reached.

**Steps**:
1. Implement in `crates/kasmos/src/serve/tools/wait_for_event.rs`:
   ```rust
   pub async fn handle(
       input: WaitForEventInput,
       state: &KasmosServer,
   ) -> Result<WaitForEventOutput> {
       let start = std::time::Instant::now();
       let timeout = std::time::Duration::from_secs(input.timeout_seconds as u64);
       let poll_interval = std::time::Duration::from_secs(
           state.config.communication.poll_interval_secs
       );
       let mut cursor = state.message_cursor.read().await.clone();

       loop {
           // Check timeout
           let elapsed = start.elapsed();
           if elapsed >= timeout {
               return Ok(WaitForEventOutput {
                   ok: true,
                   status: "timeout".to_string(),
                   elapsed_seconds: elapsed.as_secs() as u32,
                   message: None,
               });
           }

           // Read new messages
           let messages = read_messages_since(cursor, state).await?;
           for msg in &messages {
               let matches = matches_filter(msg, &input);
               if matches {
                   // Update cursor
                   *state.message_cursor.write().await = msg.message_index + 1;
                   return Ok(WaitForEventOutput {
                       ok: true,
                       status: "matched".to_string(),
                       elapsed_seconds: elapsed.as_secs() as u32,
                       message: Some(msg.clone()),
                   });
               }
           }
           if let Some(last) = messages.last() {
               cursor = last.message_index + 1;
           }

           // Sleep before next poll
           tokio::time::sleep(poll_interval).await;
       }
   }
   ```
2. The loop polls the message log at `config.communication.poll_interval_secs` intervals.
3. Filter matching: optional `wp_id` and `event` filters from input.
4. Hard timeout prevents indefinite blocking. Returns `"timeout"` status with elapsed time.
5. On match, returns the matching message with `"matched"` status.

**Files**: `crates/kasmos/src/serve/tools/wait_for_event.rs`
**Validation**: Matching event returns immediately. Timeout returns after specified seconds.

### Subtask T048 - Implement degraded fallback when pane-tracker unavailable

**Purpose**: If the zellij-pane-tracker service is unavailable, fall back to slower direct scrollback reading with explicit warning.

**Steps**:
1. In the scrollback reading function, try the pane-tracker first:
   ```rust
   async fn read_pane_scrollback(pane_name: &str) -> Result<String> {
       match try_pane_tracker_dump(pane_name).await {
           Ok(content) => Ok(content),
           Err(e) => {
               tracing::warn!(
                   "Pane-tracker unavailable: {e}. Falling back to direct scrollback."
               );
               // Fallback: use zellij action dump-screen or similar
               direct_scrollback_read(pane_name).await
           }
       }
   }
   ```
2. In degraded mode, poll intervals should be longer (e.g., 2x normal) to reduce overhead.
3. Log a warning once (not every poll cycle) when operating in degraded mode.
4. The degraded mode still produces correct results, just with higher latency.

**Files**: `crates/kasmos/src/serve/messages.rs`
**Validation**: System works (slower) when pane-tracker is unavailable. Warning is logged.

### Subtask T049 - Write manager decisions to message-log pane

**Purpose**: The manager should log its own orchestration decisions to the msg-log pane for real-time user visibility (FR-026).

**Steps**:
1. Add a helper function for writing manager events to the msg-log pane:
   ```rust
   pub async fn log_manager_event(
       event: &str,
       payload: &serde_json::Value,
   ) -> Result<()> {
       let msg = format!(
           "[KASMOS:manager:{}] {}",
           event,
           serde_json::to_string(payload)?
       );
       // Write to msg-log pane using zellij-pane-tracker run-in-pane
       write_to_pane("msg-log", &format!("echo '{}'", msg)).await
   }
   ```
2. Manager events to log: SPAWN, DESPAWN, TRANSITION, WAVE_COMPLETE, ERROR, PAUSE, RESUME.
3. These messages use the same `[KASMOS:manager:<event>]` format and are parseable by `read_messages`.
4. This is called from the serve tools when they execute manager actions (spawn, despawn, transition).

**Files**: `crates/kasmos/src/serve/messages.rs`
**Validation**: Manager events appear in msg-log pane alongside worker events.

### Subtask T050 - Add tests for message parsing and event tools

**Purpose**: Test parser edge cases, duplicate protection, timeout semantics, and degraded mode.

**Steps**:
1. Test parser with valid message format
2. Test parser ignores non-KASMOS lines
3. Test parser strips ANSI codes before matching
4. Test parser handles malformed JSON payload (returns null)
5. Test cursor-based deduplication (since_index filters correctly)
6. Test wait_for_event timeout returns correct elapsed time
7. Test wait_for_event match returns matched message
8. Test filter combinations (wp_id + event)

**Files**: Test modules in messages.rs and tools files
**Validation**: `cargo test` passes with message/event tests.

### Subtask T075 - Implement dashboard pane update side-effect

**Purpose**: On each `wait_for_event` poll cycle, format the current worker status as a table and write it to the dashboard pane (FR-032).

**Steps**:
1. After reading messages in the `wait_for_event` loop, gather current worker status from the registry:
   ```rust
   async fn update_dashboard(state: &KasmosServer) -> Result<()> {
       let workers = state.registry.read().await;
       let table = format_worker_table(&workers);
       write_to_pane("dashboard", &format!("echo '{}'", table)).await?;
       Ok(())
   }
   ```
2. Format as a simple ANSI table showing: WP ID, Role, Status, Elapsed Time
3. Clear and rewrite the dashboard pane on each update (not append).
4. Use the existing `DashboardState::format_ansi()` pattern from the data model if available, or implement a simple table formatter.
5. **Important**: Dashboard write failures MUST NOT crash the poll loop. Log the error and continue.
6. Dashboard updates are a side-effect of polling, not on a separate timer.

**Files**: `crates/kasmos/src/serve/tools/wait_for_event.rs`, `crates/kasmos/src/serve/messages.rs`
**Validation**: Dashboard pane shows updated worker status. Write failures don't crash polling.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Scrollback noise causing parser misreads | Strict prefix matching plus JSON parse validation. Only `[KASMOS:` prefix triggers parsing. |
| Waiting loop blocking manager progress | Hard timeout with explicit status codes. Manager handles timeout as a recoverable event. |
| High-frequency messages overwhelming parser | Cursor-based incremental reading. Only process new messages. |

## Review Guidance

- Verify message format matches protocol: `[KASMOS:<sender>:<event>] <json>`
- Verify ANSI stripping happens before regex matching
- Verify cursor prevents duplicate processing
- Verify wait_for_event respects timeout
- Verify degraded mode works and logs warning
- Verify manager events are written to msg-log pane

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-15T01:42:14Z – coder – shell_pid=571423 – lane=doing – Assigned agent via workflow command
