---
work_package_id: WP06
title: Completion Detector
lane: "doing"
dependencies:
- WP01
base_branch: 001-zellij-agent-orchestrator-WP01
base_commit: eb6d2fdce54e8e2cd1773b50e133e860760a33f2
created_at: '2026-02-09T04:20:33.344634+00:00'
subtasks: [T033, T034, T035, T036, T037, T038, T039]
phase: Phase 3 - Runtime
assignee: ''
agent: "controller-wp06"
shell_pid: "3484851"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP06 – Completion Detector

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **WP01** (core types) and **WP02** (spec parser for WP metadata/path resolution). Both are Wave 1.

**Implementation command**:
```bash
spec-kitty implement WP06 --base WP02
```

## Objectives & Success Criteria

**Objective**: Monitor filesystem events to automatically detect when a work package agent completes its work. The primary signal is spec-kitty lane transitions (YAML frontmatter `lane` field changing to `for_review` or `done`). Secondary signals include git activity and file markers. Events are debounced, deduplicated, and emitted to the wave engine via tokio mpsc channels.

**Success Criteria**:
1. Filesystem watcher detects file modifications in WP task directories
2. YAML frontmatter lane transitions from "doing" → "for_review" or "done" trigger completion events
3. Read-retry with 200ms debounce prevents partial reads
4. Git commit detection works as secondary signal
5. File marker (.done/.complete) detection works as tertiary signal
6. Duplicate events within a time window are suppressed
7. Completion events are emitted via tokio::sync::mpsc channel
8. Watcher can be started and stopped cleanly

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Dependencies**: `notify` crate (filesystem watcher), `tokio` (async runtime, mpsc channels)
- **spec-kitty frontmatter**: YAML between `---` delimiters, `lane` field tracks state
- **Valid completion lanes**: `for_review`, `done`
- **WP task files**: `kitty-specs/{feature}/tasks/WPxx-slug.md`
- **Reference**: [plan.md](../plan.md) WP06 section; [spec.md](../spec.md) FR-006, FR-016
- **Constraint**: Must use IN_CLOSE_WRITE equivalent (notify: `ModifyKind::Data(DataChange::Content)`)
- **Constraint**: Debounce 200ms + 3 retry attempts for partial reads
- **Constraint**: Watch paths are determined from parsed WP metadata (from WP02)

## Subtasks & Detailed Guidance

### Subtask T033 – Filesystem Watcher using notify Crate

**Purpose**: Set up a filesystem watcher that monitors WP task files for modifications.

**Steps**:

1. Create `crates/kasmos/src/detector.rs`:
   ```rust
   use notify::{Watcher, RecommendedWatcher, RecursiveMode, Event, EventKind};
   use notify::event::{ModifyKind, DataChange};
   use tokio::sync::mpsc;
   use std::path::PathBuf;

   pub struct CompletionDetector {
       watcher: Option<RecommendedWatcher>,
       event_tx: mpsc::Sender<CompletionEvent>,
       watch_paths: Vec<PathBuf>,
   }

   #[derive(Debug, Clone)]
   pub struct CompletionEvent {
       pub wp_id: String,
       pub method: CompletionMethod,
       pub timestamp: std::time::SystemTime,
   }

   impl CompletionDetector {
       pub fn new(event_tx: mpsc::Sender<CompletionEvent>) -> Self {
           Self {
               watcher: None,
               event_tx,
               watch_paths: Vec::new(),
           }
       }

       /// Start watching the given paths for completion signals.
       pub async fn start(&mut self, wp_paths: Vec<(String, PathBuf)>) -> Result<()> {
           let tx = self.event_tx.clone();

           // Create watcher with handler
           let mut watcher = notify::recommended_watcher(move |res: notify::Result<Event>| {
               match res {
                   Ok(event) => {
                       // Filter for content modification events
                       if matches!(event.kind, EventKind::Modify(ModifyKind::Data(DataChange::Content))) {
                           for path in &event.paths {
                               // Will be handled by debounce + parse logic
                               let _ = tx.blocking_send(CompletionEvent {
                                   wp_id: String::new(), // Resolved later
                                   method: CompletionMethod::AutoDetected,
                                   timestamp: std::time::SystemTime::now(),
                               });
                           }
                       }
                   }
                   Err(e) => tracing::error!(error = %e, "Filesystem watch error"),
               }
           })?;

           // Watch each WP task file
           for (wp_id, path) in &wp_paths {
               watcher.watch(path, RecursiveMode::NonRecursive)?;
               tracing::debug!(wp_id = wp_id, path = %path.display(), "Watching for completion");
           }

           self.watch_paths = wp_paths.into_iter().map(|(_, p)| p).collect();
           self.watcher = Some(watcher);
           Ok(())
       }

       /// Stop the watcher.
       pub fn stop(&mut self) {
           self.watcher = None;
           tracing::info!("Completion detector stopped");
       }
   }
   ```

2. The actual implementation will need a more sophisticated approach — the raw notify callback should send raw filesystem events to a processing task that handles debouncing and parsing. See T035.

**Files**:
- `crates/kasmos/src/detector.rs` (new, ~80 lines)

### Subtask T034 – Parse WP Frontmatter Lane Transitions

**Purpose**: When a WP task file is modified, parse the YAML frontmatter to check if the `lane` field has changed to a completion state.

**Steps**:

1. Add to `crates/kasmos/src/detector.rs`:
   ```rust
   impl CompletionDetector {
       /// Check if a WP file's frontmatter indicates completion.
       fn check_completion(path: &Path) -> Result<Option<String>> {
           let content = std::fs::read_to_string(path)?;

           // Parse YAML frontmatter (reuse parser from WP02)
           let parts: Vec<&str> = content.splitn(3, "---").collect();
           if parts.len() < 3 {
               return Ok(None);
           }

           // Parse just the lane field
           #[derive(Deserialize)]
           struct LaneCheck {
               #[serde(default)]
               lane: String,
               #[serde(default)]
               work_package_id: String,
           }

           let parsed: LaneCheck = serde_yaml::from_str(parts[1].trim())?;

           match parsed.lane.as_str() {
               "for_review" | "done" => {
                   tracing::info!(
                       wp_id = %parsed.work_package_id,
                       lane = %parsed.lane,
                       "Completion detected via lane transition"
                   );
                   Ok(Some(parsed.work_package_id))
               }
               _ => Ok(None),
           }
       }
   }
   ```

2. This function is called after debounce + retry (T035) to avoid reading partial writes

**Files**:
- `crates/kasmos/src/detector.rs` (continued, ~40 lines)

### Subtask T035 – Read-Retry with 200ms Debounce + 3 Retry Attempts

**Purpose**: Prevent partial reads by debouncing filesystem events and retrying reads that fail to parse.

**Steps**:

1. Create a processing pipeline in the detector:
   ```rust
   impl CompletionDetector {
       /// Process raw filesystem events with debounce and retry.
       /// Spawned as a tokio task.
       async fn process_events(
           mut raw_rx: mpsc::Receiver<PathBuf>,
           completion_tx: mpsc::Sender<CompletionEvent>,
           debounce_ms: u64,
       ) {
           use std::collections::HashMap;
           use tokio::time::{sleep, Duration, Instant};

           // Track last event time per path for debouncing
           let mut last_event: HashMap<PathBuf, Instant> = HashMap::new();
           let debounce = Duration::from_millis(debounce_ms);

           while let Some(path) = raw_rx.recv().await {
               let now = Instant::now();

               // Debounce: skip if we saw this path too recently
               if let Some(last) = last_event.get(&path) {
                   if now.duration_since(*last) < debounce {
                       continue;
                   }
               }
               last_event.insert(path.clone(), now);

               // Wait for debounce period
               sleep(debounce).await;

               // Retry up to 3 times
               for attempt in 0..3 {
                   match Self::check_completion(&path) {
                       Ok(Some(wp_id)) => {
                           let event = CompletionEvent {
                               wp_id,
                               method: CompletionMethod::AutoDetected,
                               timestamp: std::time::SystemTime::now(),
                           };
                           let _ = completion_tx.send(event).await;
                           break;
                       }
                       Ok(None) => break, // File changed but not a completion
                       Err(e) => {
                           if attempt < 2 {
                               tracing::debug!(
                                   path = %path.display(),
                                   attempt = attempt + 1,
                                   error = %e,
                                   "Read failed, retrying after 200ms"
                               );
                               sleep(Duration::from_millis(200)).await;
                           } else {
                               tracing::warn!(
                                   path = %path.display(),
                                   error = %e,
                                   "Read failed after 3 attempts"
                               );
                           }
                       }
                   }
               }
           }
       }
   }
   ```

**Files**:
- `crates/kasmos/src/detector.rs` (continued, ~50 lines)

### Subtask T036 – Git Activity Detection [P]

**Purpose**: Detect git commits in WP worktrees as a secondary completion signal.

**Steps**:

1. Add optional git watcher:
   ```rust
   impl CompletionDetector {
       /// Watch for new git commits in a WP's worktree.
       /// Monitors .git/refs/heads/ for changes.
       fn setup_git_watcher(
           watcher: &mut RecommendedWatcher,
           worktree_path: &Path,
           wp_id: &str,
       ) -> Result<()> {
           let git_refs = worktree_path.join(".git/refs/heads");
           if git_refs.exists() {
               watcher.watch(&git_refs, RecursiveMode::Recursive)?;
               tracing::debug!(wp_id = wp_id, "Git activity watcher enabled");
           } else {
               tracing::debug!(wp_id = wp_id, "No .git/refs/heads found, skipping git watcher");
           }
           Ok(())
       }
   }
   ```

2. Git activity alone doesn't confirm completion — it's a secondary signal that increases confidence when combined with other signals

**Files**:
- `crates/kasmos/src/detector.rs` (continued, ~20 lines)

**Parallel**: Yes — independent detection method.

### Subtask T037 – File Marker Detection [P]

**Purpose**: Detect sentinel files (.done, .complete) in WP worktree root as a tertiary completion signal.

**Steps**:

1. Add file marker detection:
   ```rust
   impl CompletionDetector {
       /// Check for completion marker files in the worktree.
       fn check_file_markers(worktree_path: &Path) -> Option<CompletionMethod> {
           let markers = [".done", ".complete", "DONE"];
           for marker in &markers {
               if worktree_path.join(marker).exists() {
                   return Some(CompletionMethod::FileMarker);
               }
           }
           None
       }

       /// Watch worktree root for marker file creation.
       fn setup_marker_watcher(
           watcher: &mut RecommendedWatcher,
           worktree_path: &Path,
           wp_id: &str,
       ) -> Result<()> {
           watcher.watch(worktree_path, RecursiveMode::NonRecursive)?;
           tracing::debug!(wp_id = wp_id, "File marker watcher enabled");
           Ok(())
       }
   }
   ```

**Files**:
- `crates/kasmos/src/detector.rs` (continued, ~25 lines)

**Parallel**: Yes — independent detection method.

### Subtask T038 – Signal Deduplication

**Purpose**: Prevent the same completion from being processed multiple times when multiple signals fire.

**Steps**:

1. Add deduplication to the processor:
   ```rust
   impl CompletionDetector {
       /// Track which WPs have already been signaled as complete.
       /// Uses a HashSet + time window to prevent re-processing.
       fn is_duplicate(
           processed: &mut HashMap<String, std::time::Instant>,
           wp_id: &str,
           window: Duration,
       ) -> bool {
           let now = std::time::Instant::now();
           if let Some(last) = processed.get(wp_id) {
               if now.duration_since(*last) < window {
                   return true;
               }
           }
           processed.insert(wp_id.to_string(), now);
           false
       }
   }
   ```

2. Deduplication window: 5 seconds (configurable via Config)
3. After the window expires, the same WP can be re-signaled (e.g., if it was retried)

**Files**:
- `crates/kasmos/src/detector.rs` (continued, ~20 lines)

### Subtask T039 – Emit Events via tokio::sync::mpsc

**Purpose**: Send validated, deduplicated completion events to the wave engine through a channel.

**Steps**:

1. The channel is already set up in the `CompletionDetector::new()` constructor (T033)
2. Wire the full pipeline:
   ```rust
   impl CompletionDetector {
       /// Create the full detection pipeline.
       pub fn create_pipeline(
           &self,
           debounce_ms: u64,
       ) -> (mpsc::Sender<PathBuf>, mpsc::Receiver<CompletionEvent>) {
           let (raw_tx, raw_rx) = mpsc::channel::<PathBuf>(100);
           let (completion_tx, completion_rx) = mpsc::channel::<CompletionEvent>(50);

           // Spawn processing task
           tokio::spawn(Self::process_events(raw_rx, completion_tx, debounce_ms));

           (raw_tx, completion_rx)
       }
   }
   ```

3. The wave engine (WP07) will receive from `completion_rx`
4. Channel buffer sizes: 100 for raw events (high frequency), 50 for completion events (deduplicated)

**Files**:
- `crates/kasmos/src/detector.rs` (continued, ~20 lines)

## Test Strategy

- Unit test: parse frontmatter with lane="done" → returns wp_id
- Unit test: parse frontmatter with lane="doing" → returns None
- Unit test: debounce suppresses rapid events (send 5 events in 100ms, only 1 processed)
- Unit test: read-retry succeeds on 2nd attempt (simulate partial write)
- Unit test: deduplication blocks same WP within 5s window, allows after window
- Unit test: file marker detection finds .done file
- Integration test: write to temp file, verify detection pipeline emits event

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| notify crate misses events under high I/O | Medium | Periodic polling fallback every 30s (future enhancement) |
| Partial YAML read during concurrent write | High | 200ms debounce + 3 retry attempts with delay |
| inotify watch limit reached | Low | Max 8 WPs + ancillary = well under default limit (~8192) |
| Channel backpressure under rapid events | Low | Buffer sizes (100/50) are generous for expected load |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] notify watcher correctly filters for content modification events
- [ ] Frontmatter parsing correctly identifies completion lanes
- [ ] Debounce prevents rapid-fire processing
- [ ] Read-retry handles partial writes gracefully
- [ ] Deduplication prevents double-processing
- [ ] Events flow through mpsc channel to consumer
- [ ] Watcher can be started and stopped cleanly
- [ ] All unit tests pass

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP06 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
- 2026-02-09T04:20:33Z – controller-wp06 – shell_pid=3484851 – lane=doing – Assigned agent via workflow command
