---
work_package_id: WP09
title: State Persistence
lane: "done"
dependencies:
- WP01
- WP05
- WP06
base_branch: 001-zellij-agent-orchestrator-WP01
base_commit: eb6d2fdce54e8e2cd1773b50e133e860760a33f2
created_at: '2026-02-09T23:10:03.053594+00:00'
subtasks: [T054, T055, T056, T057]
phase: Phase 5 - Resilience
assignee: ''
agent: "controller-review"
shell_pid: "2147072"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP09 – State Persistence

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **WP01** (core types to serialize), **WP05** (session manager for pane verification during reconciliation), and **WP06** (completion events trigger state saves).

**Implementation command**:
```bash
spec-kitty implement WP09 --base WP06
```

## Objectives & Success Criteria

**Objective**: Persist the full OrchestrationRun state to disk after every state transition, using atomic writes to prevent corruption. Support state reconciliation on reattach — when the operator reattaches to a detached session, load the persisted state and reconcile it against live Zellij pane status. Detect and warn about stale state files.

**Success Criteria**:
1. State is serialized to `.kasmos/state.json` as valid JSON
2. Every state transition triggers an atomic write (tmp file → rename)
3. State file survives process crash (no partial writes on disk)
4. On reattach, state is loaded and reconciled against live panes
5. Stale state (older than session) is detected and flagged
6. State round-trips correctly: serialize → deserialize → identical struct
7. Unit tests cover all reconciliation scenarios

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **State file**: `.kasmos/state.json` (human-readable, pretty-printed)
- **Atomic write**: Write to `.kasmos/state.json.tmp` then `std::fs::rename` to `.kasmos/state.json`
- **Reference**: [plan.md](../plan.md) WP09 section; [spec.md](../spec.md) FR-009, User Story 6
- **Constraint**: State must capture all fields needed to resume orchestration (WP states, pane mappings, wave progress, timestamps)
- **Constraint**: Reconciliation must handle: pane still running, pane crashed, pane completed while detached
- **Constraint**: `std::fs::rename` is atomic on the same filesystem (POSIX guarantee)

## Subtasks & Detailed Guidance

### Subtask T054 – Serialize OrchestrationRun to .kasmos/state.json

**Purpose**: Serialize the full orchestration state to a JSON file that can be loaded on restart or reattach.

**Steps**:

1. Create `crates/kasmos/src/persistence.rs`:
   ```rust
   use std::path::{Path, PathBuf};
   use serde_json;

   pub struct StatePersister {
       state_path: PathBuf,
       tmp_path: PathBuf,
   }

   impl StatePersister {
       pub fn new(kasmos_dir: &Path) -> Self {
           Self {
               state_path: kasmos_dir.join("state.json"),
               tmp_path: kasmos_dir.join("state.json.tmp"),
           }
       }

       /// Save the current orchestration state to disk.
       pub fn save(&self, run: &OrchestrationRun) -> Result<()> {
           let json = serde_json::to_string_pretty(run)
               .map_err(|e| StateError::Corrupted(format!("Serialization failed: {}", e)))?;

           // Atomic write (T055)
           self.atomic_write(&json)?;

           tracing::debug!(path = %self.state_path.display(), "State persisted");
           Ok(())
       }

       /// Load state from disk.
       pub fn load(&self) -> Result<Option<OrchestrationRun>> {
           if !self.state_path.exists() {
               return Ok(None);
           }

           let content = std::fs::read_to_string(&self.state_path)
               .map_err(|e| StateError::Corrupted(format!("Failed to read state: {}", e)))?;

           let run: OrchestrationRun = serde_json::from_str(&content)
               .map_err(|e| StateError::Corrupted(format!("Failed to parse state: {}", e)))?;

           tracing::info!(path = %self.state_path.display(), "State loaded");
           Ok(Some(run))
       }
   }
   ```

2. Ensure all types in OrchestrationRun derive Serialize + Deserialize (covered in WP01)
3. Use `serde_json::to_string_pretty` for human readability (debugging aid)
4. Call `save()` after every WP state transition, wave advance, and command execution

**Files**:
- `crates/kasmos/src/persistence.rs` (new, ~60 lines)

### Subtask T055 – Atomic Write (tmp + rename)

**Purpose**: Prevent state file corruption by writing to a temporary file first, then atomically renaming.

**Steps**:

1. Add to `StatePersister`:
   ```rust
   impl StatePersister {
       /// Atomic write: write to tmp file, then rename.
       /// POSIX rename is atomic on the same filesystem.
       fn atomic_write(&self, content: &str) -> Result<()> {
           // 1. Write to temporary file
           std::fs::write(&self.tmp_path, content)
               .map_err(|e| StateError::Corrupted(
                   format!("Failed to write tmp state: {}", e)
               ))?;

           // 2. Atomic rename
           std::fs::rename(&self.tmp_path, &self.state_path)
               .map_err(|e| StateError::Corrupted(
                   format!("Failed to rename state file: {}", e)
               ))?;

           Ok(())
       }

       /// Ensure .kasmos directory exists.
       pub fn ensure_dir(&self) -> Result<()> {
           if let Some(parent) = self.state_path.parent() {
               std::fs::create_dir_all(parent)?;
           }
           Ok(())
       }
   }
   ```

2. The tmp file and final file MUST be on the same filesystem for atomic rename
3. If the process crashes between write and rename, only the tmp file is affected — the previous state.json remains intact

**Files**:
- `crates/kasmos/src/persistence.rs` (continued, ~25 lines)

### Subtask T056 – State Reconciliation Decision Table for Reattach

**Purpose**: When the operator reattaches to a session, reconcile the persisted state with the live Zellij pane status to detect WPs that completed, crashed, or changed while detached.

**Steps**:

1. Add reconciliation logic:
   ```rust
   impl StatePersister {
       /// Reconcile persisted state against live Zellij panes.
       /// Returns a list of state corrections that were applied.
       pub async fn reconcile(
           &self,
           run: &mut OrchestrationRun,
           session: &SessionManager,
       ) -> Result<Vec<StateCorrection>> {
           let mut corrections = Vec::new();

           // Refresh pane list from Zellij
           let live_panes = session.list_live_panes().await?;
           let live_pane_names: HashSet<String> = live_panes.iter()
               .map(|p| p.name.clone())
               .collect();

           for wp in &mut run.work_packages {
               let pane_exists = live_pane_names.contains(&wp.pane_name);

               match (&wp.state, pane_exists) {
                   // Running in state + pane exists → continue as-is
                   (WPState::Active, true) => {
                       tracing::debug!(wp_id = %wp.id, "Active WP still running");
                   }

                   // Running in state + pane MISSING → crashed while detached
                   (WPState::Active, false) => {
                       tracing::warn!(wp_id = %wp.id, "Active WP pane missing — marking as crashed");
                       wp.state = WPState::Failed;
                       wp.failure_count += 1;
                       corrections.push(StateCorrection {
                           wp_id: wp.id.clone(),
                           from: WPState::Active,
                           to: WPState::Failed,
                           reason: "Pane missing on reattach".into(),
                       });
                   }

                   // Completed in state + pane exists → pane might be stale, close it
                   (WPState::Completed, true) => {
                       tracing::info!(wp_id = %wp.id, "Completed WP still has pane — scheduling close");
                       corrections.push(StateCorrection {
                           wp_id: wp.id.clone(),
                           from: WPState::Completed,
                           to: WPState::Completed,
                           reason: "Stale pane — will close".into(),
                       });
                   }

                   // Failed in state → offer retry
                   (WPState::Failed, _) => {
                       tracing::info!(wp_id = %wp.id, "Failed WP — use 'retry {}' to relaunch", wp.id);
                   }

                   // Pending → no action needed
                   (WPState::Pending, _) => {}

                   // Paused + pane exists → resume possible
                   (WPState::Paused, true) => {
                       tracing::info!(wp_id = %wp.id, "Paused WP — use 'resume {}' to continue", wp.id);
                   }

                   // Paused + pane missing → crashed while paused
                   (WPState::Paused, false) => {
                       wp.state = WPState::Failed;
                       corrections.push(StateCorrection {
                           wp_id: wp.id.clone(),
                           from: WPState::Paused,
                           to: WPState::Failed,
                           reason: "Paused pane missing on reattach".into(),
                       });
                   }

                   _ => {}
               }
           }

           // Persist corrected state
           if !corrections.is_empty() {
               self.save(run)?;
               tracing::info!(corrections = corrections.len(), "State reconciled");
           }

           Ok(corrections)
       }
   }

   #[derive(Debug)]
   pub struct StateCorrection {
       pub wp_id: String,
       pub from: WPState,
       pub to: WPState,
       pub reason: String,
   }
   ```

**Files**:
- `crates/kasmos/src/persistence.rs` (continued, ~80 lines)

### Subtask T057 – Stale State Detection [P]

**Purpose**: Detect when the state file is older than the Zellij session, indicating the state may not reflect reality.

**Steps**:

1. Add stale detection:
   ```rust
   impl StatePersister {
       /// Check if state file is stale relative to a reference time.
       pub fn check_staleness(&self, session_start: std::time::SystemTime) -> Result<Option<StalenessWarning>> {
           let metadata = std::fs::metadata(&self.state_path)?;
           let state_mtime = metadata.modified()?;

           if state_mtime < session_start {
               let age = session_start.duration_since(state_mtime)
                   .unwrap_or_default();
               tracing::warn!(
                   age_secs = age.as_secs(),
                   "State file is older than session — may be stale"
               );
               return Ok(Some(StalenessWarning {
                   state_age: age,
                   state_mtime,
                   session_start,
               }));
           }

           Ok(None)
       }
   }

   #[derive(Debug)]
   pub struct StalenessWarning {
       pub state_age: std::time::Duration,
       pub state_mtime: std::time::SystemTime,
       pub session_start: std::time::SystemTime,
   }
   ```

2. Check staleness during reattach flow (before reconciliation)
3. If stale, warn the operator but proceed with reconciliation anyway

**Files**:
- `crates/kasmos/src/persistence.rs` (continued, ~30 lines)

**Parallel**: Yes — staleness check is independent of write/reconciliation logic.

## Test Strategy

- Unit test: serialize OrchestrationRun → deserialize → identical (round-trip)
- Unit test: atomic write creates tmp then renames (verify no partial state.json exists during write)
- Unit test: load from nonexistent file → Ok(None)
- Unit test: load from corrupted file → StateError::Corrupted
- Unit test: reconcile Active+pane_missing → Failed correction
- Unit test: reconcile Active+pane_exists → no correction
- Unit test: reconcile Completed+pane_exists → stale pane correction
- Unit test: staleness detection with old state file

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| State file corruption from crash | High | Atomic write (tmp + rename) guarantees either old or new state |
| State diverges from Zellij reality | Medium | Reconciliation on every reattach, check live pane status |
| Frequent writes cause I/O pressure | Low | State transitions are infrequent (~seconds apart), not a hot path |
| Serialization adds new required fields | Medium | Use `#[serde(default)]` for backward compatibility |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] State serializes to valid, pretty-printed JSON
- [ ] Atomic write prevents partial state files
- [ ] Load handles missing, valid, and corrupted files correctly
- [ ] Reconciliation covers all state×pane combinations
- [ ] Staleness detection works with timestamp comparison
- [ ] State round-trips without data loss
- [ ] All unit tests pass

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP09 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
- 2026-02-09T23:10:03Z – opencode – shell_pid=1757173 – lane=doing – Assigned agent via workflow command
- 2026-02-10T02:05:59Z – opencode – shell_pid=1757173 – lane=for_review – Ready for review: Complete state persistence implementation with atomic writes, reconciliation, and staleness detection. All 53 tests passing.
- 2026-02-10T02:11:39Z – controller-review – shell_pid=2147072 – lane=doing – Started review via workflow command
- 2026-02-10T02:13:35Z – controller-review – shell_pid=2147072 – lane=done – REVIEW APPROVED by opencode: All 7 acceptance checkpoints met. Atomic writes, exhaustive reconciliation (8 state×pane cases), staleness detection implemented. 53/53 tests passing (10 new). Code quality excellent. Ready for merge.
