---
work_package_id: WP05
title: Repository-Wide Feature Locking
lane: "done"
dependencies: [WP04]
base_branch: 011-mcp-agent-swarm-orchestration-WP04
base_commit: 839ff563e7dfa7894ce4b53b37f439478bf887a6
created_at: '2026-02-14T22:40:00.621692+00:00'
subtasks:
- T027
- T028
- T029
- T030
- T031
- T032
phase: Phase 2 - Safety, State, and Audit Guarantees
assignee: ''
agent: "opencode"
shell_pid: "3957848"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP05 - Repository-Wide Feature Locking

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP05 --base WP04
```

---

## Objectives & Success Criteria

Enforce single active owner per feature across the repository (FR-020), with stale lock handling that requires explicit takeover confirmation after timeout. After this WP:

1. Lock key format: `<repo_root>::<feature_slug>`
2. Second process binding same feature receives conflict with active owner details
3. Stale lock (no heartbeat for 15+ minutes) requires explicit confirmation before takeover
4. Clean shutdown releases the lock
5. Lock integrates with launch binding path and serve tool operations

## Context & Constraints

- **Depends on WP04**: Serve state containers and tool framework available
- **Data model**: `FeatureBindingLock` entity in `data-model.md` defines fields and state transitions
- **Research decision 2**: Lock scope is repository-wide across all running kasmos processes
- **Research decision 3**: Stale after 15 minutes, takeover requires explicit confirmation
- **Engineering decision 5**: Stale lock timeout = 15 minutes (configurable via `LockConfig`)
- **Key constraint**: Use atomic file operations plus advisory file locking to prevent race conditions

## Subtasks & Detailed Guidance

### Subtask T027 - Implement lock key derivation and repo-root resolution

**Purpose**: Derive the canonical lock key from repo root and feature slug, ensuring consistency across invocations.

**Steps**:
1. Create `crates/kasmos/src/serve/lock.rs`:
   ```rust
   pub struct LockKey {
       pub repo_root: PathBuf,
       pub feature_slug: String,
   }

   impl LockKey {
       pub fn new(feature_slug: &str) -> Result<Self> {
           let repo_root = resolve_repo_root()?;
           Ok(Self { repo_root, feature_slug: feature_slug.to_string() })
       }

       pub fn to_string(&self) -> String {
           format!("{}::{}", self.repo_root.display(), self.feature_slug)
       }

       pub fn lock_file_path(&self) -> PathBuf {
           // Store lock in .kasmos/ at repo root
           self.repo_root.join(".kasmos").join("locks")
               .join(format!("{}.lock", self.feature_slug))
       }
   }
   ```
2. `resolve_repo_root()`: Use `git rev-parse --show-toplevel` to get canonical repo root. Cache the result.
3. Lock files live in `<repo_root>/.kasmos/locks/<feature_slug>.lock`.
4. Ensure the locks directory is created on first use.

**Files**: `crates/kasmos/src/serve/lock.rs`
**Validation**: Lock key is deterministic for same repo + feature.

### Subtask T028 - Implement persistent lock record with heartbeat

**Purpose**: Create the lock file with owner metadata, timestamps, and heartbeat refresh capability.

**Steps**:
1. Lock record structure (written as JSON to lock file):
   ```rust
   #[derive(Serialize, Deserialize)]
   pub struct LockRecord {
       pub lock_key: String,
       pub repo_root: String,
       pub feature_slug: String,
       pub owner_id: String,        // process PID + hostname
       pub owner_session: String,    // Zellij session name
       pub owner_tab: String,        // Zellij tab name
       pub acquired_at: DateTime<Utc>,
       pub last_heartbeat_at: DateTime<Utc>,
       pub expires_at: DateTime<Utc>,
       pub status: LockStatus,
   }

   #[derive(Serialize, Deserialize)]
   pub enum LockStatus { Active, Stale, Released }
   ```
2. `acquire()`: Write lock file atomically (write to temp, rename).
3. `heartbeat()`: Update `last_heartbeat_at` and `expires_at`.
4. `release()`: Set status to `Released` and remove lock file.
5. Use file-level advisory locking (`nix::fcntl::flock` with `LOCK_EX | LOCK_NB`) around read-modify-write operations to prevent race conditions between processes.
6. Owner ID: use `format!("{}@{}", std::process::id(), hostname)` for a stable process identity.

**Files**: `crates/kasmos/src/serve/lock.rs`
**Validation**: Lock file is created, heartbeat updates timestamps, release cleans up.

### Subtask T029 - Enforce conflict response for active locks

**Purpose**: When another process holds an active lock, refuse binding and provide clear owner details.

**Steps**:
1. Before acquiring a lock, check if a lock file exists and is active:
   ```rust
   pub fn check_conflict(key: &LockKey) -> Result<LockConflict> {
       let lock_path = key.lock_file_path();
       if !lock_path.exists() { return Ok(LockConflict::None); }
       let record = read_lock_record(&lock_path)?;
       match record.status {
           LockStatus::Active if !is_stale(&record) => {
               Ok(LockConflict::ActiveOwner(record))
           }
           LockStatus::Active => Ok(LockConflict::Stale(record)),
           LockStatus::Released | LockStatus::Stale => Ok(LockConflict::None),
       }
   }
   ```
2. On `ActiveOwner`: Return `FEATURE_LOCK_CONFLICT` error code (matches contract) with owner details:
   ```
   Feature '011-mcp-agent-swarm-orchestration' is already owned by:
     PID: 12345@hostname
     Session: kasmos
     Acquired: 2026-02-14T10:00:00Z
     Last heartbeat: 2026-02-14T10:05:00Z
   ```
3. On conflict, the launch flow exits immediately without creating session/tab.

**Files**: `crates/kasmos/src/serve/lock.rs`
**Validation**: Second process gets conflict error with owner details.

### Subtask T030 - Implement stale detection with configurable timeout

**Purpose**: Detect when a lock has not received a heartbeat within the configured timeout (default 15 minutes).

**Steps**:
1. Stale detection logic:
   ```rust
   fn is_stale(record: &LockRecord, config: &LockConfig) -> bool {
       let now = Utc::now();
       let stale_threshold = chrono::Duration::minutes(config.stale_timeout_minutes as i64);
       now - record.last_heartbeat_at > stale_threshold
   }
   ```
2. The timeout is configurable via `LockConfig.stale_timeout_minutes` (default: 15).
3. When stale is detected, the lock's status transitions to `Stale` conceptually (may or may not be written to file at detection time).

**Parallel?**: Can run alongside T029 once lock record schema exists.
**Files**: `crates/kasmos/src/serve/lock.rs`
**Validation**: Lock with old heartbeat is correctly identified as stale.

### Subtask T031 - Implement confirmation-gated stale takeover

**Purpose**: When a stale lock is detected, require explicit user confirmation before taking ownership.

**Steps**:
1. In the launch flow, when `check_conflict` returns `LockConflict::Stale(record)`:
   ```
   Feature '011-mcp-agent-swarm-orchestration' has a stale lock:
     PID: 12345@hostname (may be dead)
     Session: kasmos
     Last heartbeat: 2026-02-14T08:00:00Z (2h 30m ago)

   Take over ownership? [y/N]:
   ```
2. If user confirms: Overwrite the lock file with new owner.
3. If user declines: Exit without launching.
4. For the MCP tool path (non-interactive), the error code is `STALE_LOCK_CONFIRMATION_REQUIRED` and the caller must provide an explicit confirmation flag.
5. Never silently take over a stale lock. Fail closed without confirmation.

**Files**: `crates/kasmos/src/serve/lock.rs`, `crates/kasmos/src/launch/mod.rs`
**Validation**: Stale lock prompts for confirmation. Declining exits. Confirming acquires.

### Subtask T032 - Add tests for lock behavior

**Purpose**: Comprehensive tests for acquisition, heartbeat, stale detection, takeover, and conflicts.

**Steps**:
1. Test lock acquisition creates valid lock file
2. Test heartbeat refresh updates timestamps
3. Test conflict detection for active lock
4. Test stale detection after timeout
5. Test takeover with confirmation
6. Test release removes lock
7. Test concurrent acquisition race (use temp dirs for isolation)
8. Use `tempfile` crate (already a dev-dependency) for test isolation

**Files**: Test module in `crates/kasmos/src/serve/lock.rs`
**Validation**: `cargo test` passes with lock tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Race conditions during concurrent lock acquisition | Atomic file ops + advisory locking via nix flock |
| Accidental silent takeover | Fail closed unless confirmation token is explicit |
| Lock file corruption | Atomic write (temp + rename). Validate JSON on read. |
| Stale process detection false positives | Use heartbeat timeout, not process existence check (cross-platform reliable) |

## Review Guidance

- Verify lock key is deterministic: same repo + feature always produces same key
- Verify atomic file operations (write-temp-rename pattern)
- Verify advisory locking around read-modify-write
- Verify stale detection uses configurable timeout
- Verify takeover ALWAYS requires explicit confirmation
- Verify error codes match contract: `FEATURE_LOCK_CONFLICT`, `STALE_LOCK_CONFIRMATION_REQUIRED`

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-14T22:40:00Z – opencode – shell_pid=3957848 – lane=doing – Assigned agent via workflow command
- 2026-02-15T00:52:49Z – opencode – shell_pid=3957848 – lane=for_review – Ready for review
- 2026-02-15T00:55:14Z – opencode – shell_pid=3957848 – lane=done – Review passed: feature locking with flock, atomic writes, stale detection, 251 tests pass
