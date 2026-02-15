---
work_package_id: WP06
title: Audit Log Persistence and Retention Policy
lane: "doing"
dependencies: [WP04]
base_branch: 011-mcp-agent-swarm-orchestration-WP04
base_commit: a02df49238a89b34cf57dc156237af2bad587046
created_at: '2026-02-15T00:59:41.708899+00:00'
subtasks:
- T033
- T034
- T035
- T036
- T037
- T038
phase: Phase 2 - Safety, State, and Audit Guarantees
assignee: ''
agent: ''
shell_pid: "212269"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP06 - Audit Log Persistence and Retention Policy

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP06 --base WP04
```

---

## Objectives & Success Criteria

Persist orchestration audit records at the feature-local path with configurable payload depth and retention. After this WP:

1. Audit file at `kitty-specs/<feature>/.kasmos/messages.jsonl` is created and written to
2. Default entries contain metadata only (timestamp, actor, action, wp_id, status, summary)
3. Debug mode includes full payloads (prompts, tool args) when enabled in config
4. Retention: rotation/pruning triggers when EITHER file size > 512MB OR entry age > 14 days
5. Audit writes are integrated across core action paths (lock, spawn, transition, error)

## Context & Constraints

- **Depends on WP04**: Serve state and config available
- **Data model**: `AuditEntry` and `AuditPolicy` entities in `data-model.md`
- **Research decision 5**: Audit at `kitty-specs/<feature>/.kasmos/messages.jsonl`, dual threshold rotation
- **Research decision 6**: Metadata-only default, debug-mode full payload
- **Key constraint**: Append-only writes within a file generation. Buffered for performance.

## Subtasks & Detailed Guidance

### Subtask T033 - Implement audit directory and file bootstrap

**Purpose**: Create the `.kasmos/` directory and `messages.jsonl` file under the feature path on first use.

**Steps**:
1. Create `crates/kasmos/src/serve/audit.rs`:
   ```rust
   pub struct AuditWriter {
       path: PathBuf,
       config: AuditConfig,
       file: Option<std::fs::File>,
   }

   impl AuditWriter {
       pub fn new(feature_dir: &Path, config: &AuditConfig) -> Result<Self> {
           let audit_dir = feature_dir.join(".kasmos");
           std::fs::create_dir_all(&audit_dir)?;
           let path = audit_dir.join("messages.jsonl");
           Ok(Self { path, config: config.clone(), file: None })
       }
   }
   ```
2. The `.kasmos/` directory should be created idempotently (no error if it exists).
3. Add `.kasmos/` to `.gitignore` for the feature directory? No - per the spec, audit logs should be "committed to version control with the rest of the spec artifacts" (FR-027). So do NOT gitignore them.
4. Ensure the audit directory path is derived from the feature directory, not hard-coded.

**Files**: `crates/kasmos/src/serve/audit.rs`
**Validation**: Directory and file are created correctly on first write.

### Subtask T034 - Implement append-only JSONL writer

**Purpose**: Write audit entries as newline-delimited JSON, append-only within a file generation.

**Steps**:
1. Implement the write method:
   ```rust
   impl AuditWriter {
       pub fn write_entry(&mut self, entry: &AuditEntry) -> Result<()> {
           let file = self.get_or_open_file()?;
           let json = serde_json::to_string(entry)?;
           use std::io::Write;
           writeln!(file, "{}", json)?;
           file.flush()?;  // Ensure durability
           Ok(())
       }

       fn get_or_open_file(&mut self) -> Result<&mut std::fs::File> {
           if self.file.is_none() {
               let file = std::fs::OpenOptions::new()
                   .create(true)
                   .append(true)
                   .open(&self.path)?;
               self.file = Some(file);
           }
           Ok(self.file.as_mut().unwrap())
       }
   }
   ```
2. The `AuditEntry` struct (matching data-model.md):
   ```rust
   #[derive(Serialize, Deserialize)]
   pub struct AuditEntry {
       pub timestamp: DateTime<Utc>,
       pub actor: String,       // "manager", "kasmos-serve", or worker id
       pub action: String,      // "spawn_worker", "transition_wp", etc.
       pub feature_slug: String,
       pub wp_id: Option<String>,
       pub status: String,
       pub summary: String,
       pub details: serde_json::Value,  // metadata by default
       #[serde(skip_serializing_if = "Option::is_none")]
       pub debug_payload: Option<serde_json::Value>,
   }
   ```
3. Each write is flushed immediately for durability (crash safety).
4. Keep writes fast - avoid holding locks during serialization.

**Files**: `crates/kasmos/src/serve/audit.rs`
**Validation**: Entries are written as valid JSONL. File can be read line-by-line.

### Subtask T035 - Implement metadata-only default and debug payload opt-in

**Purpose**: Control what goes into audit entries based on config.

**Steps**:
1. Add a builder pattern for audit entries:
   ```rust
   impl AuditEntry {
       pub fn new(actor: &str, action: &str, feature_slug: &str) -> Self {
           Self {
               timestamp: Utc::now(),
               actor: actor.to_string(),
               action: action.to_string(),
               feature_slug: feature_slug.to_string(),
               wp_id: None,
               status: String::new(),
               summary: String::new(),
               details: serde_json::Value::Null,
               debug_payload: None,
           }
       }

       pub fn with_debug_payload(mut self, payload: serde_json::Value, enabled: bool) -> Self {
           if enabled {
               self.debug_payload = Some(payload);
           }
           self
       }
   }
   ```
2. The `AuditWriter` checks `config.debug_full_payload` before including debug payloads.
3. Default mode: `details` contains minimal metadata (e.g., `{"pane_name": "WP01-coder"}`).
4. Debug mode: `debug_payload` additionally contains full prompts, tool arguments, etc.
5. Sensitive data (if any) should be redacted even in debug mode.

**Files**: `crates/kasmos/src/serve/audit.rs`
**Validation**: Default entries have no `debug_payload`. Debug-enabled entries include it.

### Subtask T036 - Implement retention evaluator with either-threshold trigger

**Purpose**: Rotate/prune audit logs when EITHER size exceeds 512MB OR entries older than 14 days.

**Steps**:
1. Add retention check method:
   ```rust
   impl AuditWriter {
       pub fn check_retention(&self) -> Result<bool> {
           let metadata = std::fs::metadata(&self.path)?;
           // Size threshold
           if metadata.len() > self.config.max_bytes {
               return Ok(true);
           }
           // Age threshold - check first line timestamp
           if let Some(oldest) = self.read_oldest_entry_timestamp()? {
               let age = Utc::now() - oldest;
               if age.num_days() > self.config.max_age_days as i64 {
                   return Ok(true);
               }
           }
           Ok(false)
       }

       pub fn rotate(&mut self) -> Result<()> {
           // Close current file
           self.file = None;
           // Rename current to timestamped archive
           let archive_name = format!("messages.{}.jsonl",
               Utc::now().format("%Y%m%d-%H%M%S"));
           let archive_path = self.path.with_file_name(archive_name);
           std::fs::rename(&self.path, &archive_path)?;
           // Optionally prune old archives
           Ok(())
       }
   }
   ```
2. Either-threshold: rotation triggers if size > `max_bytes` (512MB) OR age > `max_age_days` (14).
3. Rotation: rename current file to timestamped archive, start fresh.
4. Call `check_retention()` periodically (e.g., every N writes, not every write for performance).

**Parallel?**: Can proceed alongside T037 once writer API is stable.
**Files**: `crates/kasmos/src/serve/audit.rs`
**Validation**: Rotation triggers on size threshold. Rotation triggers on age threshold.

### Subtask T037 - Integrate audit writes across core actions

**Purpose**: Wire audit event emission into lock, spawn/despawn, transition, and error paths.

**Steps**:
1. Add `AuditWriter` to the `KasmosServer` shared state.
2. Emit audit entries at key points:
   - Lock acquired/released/conflict/stale-takeover
   - Worker spawned/despawned/errored/aborted
   - WP state transitions (pending->active, active->for_review, etc.)
   - Error conditions (lock conflicts, transition failures, dependency missing)
3. Each audit call should be non-blocking. If audit write fails, log a warning but do NOT fail the main operation.
4. Use the builder pattern from T035 for consistent entry construction.

**Files**: Integration across `crates/kasmos/src/serve/lock.rs`, `crates/kasmos/src/serve/tools/*.rs`
**Validation**: Core actions produce audit entries in the JSONL file.

### Subtask T038 - Add tests for audit policy

**Purpose**: Test payload redaction defaults, debug inclusion, and retention trigger correctness.

**Steps**:
1. Test default entry has no debug_payload
2. Test debug-enabled entry includes debug_payload
3. Test retention triggers on size threshold
4. Test retention triggers on age threshold
5. Test rotation creates archive file
6. Test append-only behavior (no overwriting)
7. Use tempfile for test isolation

**Files**: Test module in `crates/kasmos/src/serve/audit.rs`
**Validation**: `cargo test` passes with audit tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Audit writes impacting runtime latency | Buffered append, flush on significant events only. Non-blocking writes. |
| Retention deleting recent diagnostics | Rotate (archive) rather than delete. Deterministic naming. |
| Disk space exhaustion | Either-threshold prevents unbounded growth. Archive pruning optional. |

## Review Guidance

- Verify audit path matches spec: `kitty-specs/<feature>/.kasmos/messages.jsonl`
- Verify default entries are metadata-only (no prompts/payloads)
- Verify debug_payload only present when debug mode enabled
- Verify either-threshold retention (size OR age)
- Verify audit writes don't fail the main operation
- Verify append-only within a file generation

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
