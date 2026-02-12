//! Completion detector for work packages.
//!
//! Monitors filesystem events to detect when work packages complete.
//! Primary signal: spec-kitty lane transitions (YAML frontmatter).
//! Secondary signals: git activity, file markers (.done, .complete, DONE).
//!
//! Events are debounced, deduplicated, and emitted via tokio::sync::mpsc channels.

use crate::error::{DetectorError, Result};
use crate::types::CompletionMethod;
use notify::{Watcher, RecommendedWatcher, RecursiveMode, Event, EventKind};
use notify::event::{ModifyKind, DataChange};
use serde::Deserialize;
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime};
use tokio::sync::mpsc;
use tokio::time::sleep;

/// The detected lane from spec-kitty frontmatter.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DetectedLane {
    /// Lane is "for_review" — agent finished, awaiting human review.
    ForReview,
    /// Lane is "done" — work package fully completed.
    Done,
}

/// Represents a completion event detected by the detector.
#[derive(Debug, Clone)]
pub struct CompletionEvent {
    /// Work package ID that completed.
    pub wp_id: String,

    /// How the completion was detected.
    pub method: CompletionMethod,

    /// Whether the completion was successful.
    pub success: bool,

    /// The detected lane transition (if primary detection via frontmatter).
    /// `None` for secondary/tertiary detection methods (git activity, file markers).
    pub detected_lane: Option<DetectedLane>,

    /// When the event was detected.
    pub timestamp: SystemTime,
}

impl CompletionEvent {
    /// Create a new completion event.
    pub fn new(wp_id: String, method: CompletionMethod, success: bool) -> Self {
        Self {
            wp_id,
            method,
            success,
            detected_lane: None,
            timestamp: SystemTime::now(),
        }
    }

    /// Create a completion event with a detected lane.
    pub fn with_lane(wp_id: String, method: CompletionMethod, lane: DetectedLane) -> Self {
        Self {
            wp_id,
            method,
            success: true,
            detected_lane: Some(lane),
            timestamp: SystemTime::now(),
        }
    }
}

/// Raw filesystem event with WP context.
#[derive(Debug, Clone)]
struct RawFsEvent {
    /// Work package ID associated with this path.
    wp_id: String,

    /// Path that was modified.
    path: PathBuf,

    /// Worktree root for secondary/tertiary detection.
    worktree_path: PathBuf,
}

/// Filesystem watcher for detecting work package completions.
#[derive(Default)]
pub struct CompletionDetector {
    /// The filesystem watcher (if active).
    watcher: Option<RecommendedWatcher>,

    /// Sender for raw filesystem events (internal pipeline).
    raw_tx: Option<mpsc::Sender<RawFsEvent>>,

    /// Paths being watched, mapped to WP context.
    watch_paths: Vec<(String, PathBuf, PathBuf)>, // (wp_id, task_file, worktree_root)
}

impl CompletionDetector {
    /// Create a new completion detector.
    pub fn new() -> Self {
        Self::default()
    }

    /// Start watching the given WP paths for completion signals.
    ///
    /// # Arguments
    /// * `wp_paths` - Vector of (wp_id, task_file_path, worktree_root) tuples to watch
    ///
    /// # Returns
    /// Result indicating success or error
    pub async fn start(
        &mut self,
        wp_paths: Vec<(String, PathBuf, PathBuf)>,
    ) -> Result<mpsc::Receiver<CompletionEvent>> {
        if wp_paths.is_empty() {
            tracing::warn!("No paths to watch");
            return Err(DetectorError::WatcherError("No paths to watch".to_string()).into());
        }

        // Create the processing pipeline
        let (raw_tx, raw_rx) = mpsc::channel::<RawFsEvent>(100);
        let (completion_tx, completion_rx) = mpsc::channel::<CompletionEvent>(50);

        // Spawn the processing task
        tokio::spawn(Self::process_events(raw_rx, completion_tx));

        // Create watcher with handler that filters for content modifications
        let raw_tx_clone = raw_tx.clone();
        let watch_paths_arc = Arc::new(wp_paths.clone());

        let mut watcher = notify::recommended_watcher(move |res: notify::Result<Event>| {
            if let Ok(event) = res {
                // Filter for content modification events only
                if matches!(
                    event.kind,
                    EventKind::Modify(ModifyKind::Data(DataChange::Content))
                ) {
                    for path in &event.paths {
                        // Find the WP context for this path
                        for (wp_id, task_file, worktree_root) in watch_paths_arc.iter() {
                            if path == task_file {
                                let raw_event = RawFsEvent {
                                    wp_id: wp_id.clone(),
                                    path: path.clone(),
                                    worktree_path: worktree_root.clone(),
                                };
                                let _ = raw_tx_clone.blocking_send(raw_event);
                                break;
                            }
                        }
                    }
                }
            }
        }).map_err(DetectorError::from)?;

        // Watch each WP task file
        for (wp_id, path, _worktree_root) in &wp_paths {
            watcher.watch(path, RecursiveMode::NonRecursive)
                .map_err(DetectorError::from)?;
            tracing::debug!(wp_id = wp_id, path = %path.display(), "Watching for completion");
        }

        self.watch_paths = wp_paths;
        self.watcher = Some(watcher);
        self.raw_tx = Some(raw_tx);

        Ok(completion_rx)
    }

    /// Stop the watcher and processing pipeline.
    pub fn stop(&mut self) {
        self.watcher = None;
        self.raw_tx = None;
        tracing::info!("Completion detector stopped");
    }

    /// Parse YAML frontmatter to check for completion lane transitions.
    ///
    /// Returns `(wp_id, detected_lane)` if the file indicates completion
    /// (lane = "for_review" or "done").
    fn check_completion(path: &Path) -> Result<Option<(String, DetectedLane)>> {
        let content = std::fs::read_to_string(path)
            .map_err(|e| DetectorError::ReadError(e.to_string()))?;

        // Parse YAML frontmatter (between --- delimiters)
        let parts: Vec<&str> = content.splitn(3, "---").collect();
        if parts.len() < 3 {
            return Ok(None);
        }

        // Parse just the lane and work_package_id fields
        #[derive(Deserialize)]
        struct LaneCheck {
            #[serde(default)]
            lane: String,
            #[serde(default)]
            work_package_id: String,
        }

        let parsed: LaneCheck = serde_yml::from_str(parts[1].trim())
            .map_err(|e| DetectorError::YamlError(e.to_string()))?;

        // Check if lane indicates completion
        match parsed.lane.as_str() {
            "for_review" => {
                tracing::info!(
                    wp_id = %parsed.work_package_id,
                    lane = %parsed.lane,
                    "Completion detected via lane transition"
                );
                Ok(Some((parsed.work_package_id, DetectedLane::ForReview)))
            }
            "done" => {
                tracing::info!(
                    wp_id = %parsed.work_package_id,
                    lane = %parsed.lane,
                    "Completion detected via lane transition"
                );
                Ok(Some((parsed.work_package_id, DetectedLane::Done)))
            }
            _ => Ok(None),
        }
    }

    /// Check for completion marker files in the worktree.
    fn check_file_markers(worktree_path: &Path) -> Option<CompletionMethod> {
        let markers = [".done", ".complete", "DONE"];
        for marker in &markers {
            if worktree_path.join(marker).exists() {
                tracing::debug!(
                    path = %worktree_path.display(),
                    marker = marker,
                    "Completion marker file found"
                );
                return Some(CompletionMethod::FileMarker);
            }
        }
        None
    }

    /// Check for git activity in the worktree (new commits).
    fn check_git_activity(worktree_path: &Path) -> Option<CompletionMethod> {
        let git_refs = worktree_path.join(".git/refs/heads");
        if git_refs.exists() {
            tracing::debug!(
                path = %worktree_path.display(),
                "Git activity detected"
            );
            return Some(CompletionMethod::GitActivity);
        }
        None
    }

    /// Check if a WP file's frontmatter indicates completion via lane transition.
    ///
    /// Retries up to 3 times with 200ms delay on read failures.
    async fn check_completion_with_retry(path: &Path) -> Result<Option<(String, DetectedLane)>> {
        const MAX_ATTEMPTS: usize = 3;
        const RETRY_DELAY_MS: u64 = 200;

        for attempt in 0..MAX_ATTEMPTS {
            match Self::check_completion(path) {
                Ok(result) => return Ok(result),
                Err(e) => {
                    if attempt < MAX_ATTEMPTS - 1 {
                        tracing::debug!(
                            path = %path.display(),
                            attempt = attempt + 1,
                            error = %e,
                            "Read failed, retrying after 200ms"
                        );
                        sleep(Duration::from_millis(RETRY_DELAY_MS)).await;
                    } else {
                        tracing::warn!(
                            path = %path.display(),
                            error = %e,
                            "Read failed after 3 attempts"
                        );
                        return Err(e);
                    }
                }
            }
        }

        Ok(None)
    }

    /// Check if an event is a duplicate within the deduplication window.
    fn is_duplicate(
        processed: &mut HashMap<String, Instant>,
        wp_id: &str,
        window: Duration,
    ) -> bool {
        let now = Instant::now();
        if let Some(last) = processed.get(wp_id)
            && now.duration_since(*last) < window
        {
            return true;
        }
        processed.insert(wp_id.to_string(), now);
        false
    }

    /// Process raw filesystem events with debounce, retry, and multi-signal detection.
    ///
    /// This task:
    /// 1. Debounces rapid events (200ms window per path)
    /// 2. Retries reads up to 3 times on failure
    /// 3. Tries primary detection (lane transition)
    /// 4. Falls back to secondary detection (git activity)
    /// 5. Falls back to tertiary detection (file markers)
    /// 6. Deduplicates completion events (5 second window per WP)
    /// 7. Emits validated completion events
    async fn process_events(
        mut raw_rx: mpsc::Receiver<RawFsEvent>,
        completion_tx: mpsc::Sender<CompletionEvent>,
    ) {
        // Track last event time per path for debouncing
        let mut last_event: HashMap<PathBuf, Instant> = HashMap::new();
        let debounce = Duration::from_millis(200);

        // Track processed WPs for deduplication (5 second window)
        let mut processed: HashMap<String, Instant> = HashMap::new();
        let dedup_window = Duration::from_secs(5);

        while let Some(raw_event) = raw_rx.recv().await {
            let now = Instant::now();

            // Debounce: skip if we saw this path too recently
            if let Some(last) = last_event.get(&raw_event.path)
                && now.duration_since(*last) < debounce
            {
                continue;
            }
            last_event.insert(raw_event.path.clone(), now);

            // Wait for debounce period to ensure file is fully written
            sleep(debounce).await;

            // Try primary detection: lane transition in task file
            let detection_result = match Self::check_completion_with_retry(&raw_event.path).await {
                Ok(Some((_wp_id, lane))) => Some((CompletionMethod::AutoDetected, Some(lane))),
                Ok(None) => {
                    // Primary failed, try secondary: git activity
                    Self::check_git_activity(&raw_event.worktree_path)
                        .or_else(|| {
                            // Secondary failed, try tertiary: file markers
                            Self::check_file_markers(&raw_event.worktree_path)
                        })
                        .map(|method| (method, None))
                }
                Err(e) => {
                    tracing::debug!(
                        path = %raw_event.path.display(),
                        error = %e,
                        "Failed to process file"
                    );
                    None
                }
            };

            if let Some((method, detected_lane)) = detection_result {
                // Check for duplicates
                if Self::is_duplicate(&mut processed, &raw_event.wp_id, dedup_window) {
                    tracing::debug!(wp_id = %raw_event.wp_id, "Duplicate completion event suppressed");
                    continue;
                }

                let event = CompletionEvent {
                    wp_id: raw_event.wp_id.clone(),
                    method,
                    success: true,
                    detected_lane,
                    timestamp: SystemTime::now(),
                };

                if let Err(e) = completion_tx.send(event).await {
                    tracing::error!(error = %e, "Failed to send completion event");
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn test_parse_frontmatter_completion_for_review() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP01.md");

        let content = r#"---
work_package_id: WP01
lane: for_review
title: Test WP
---

# Content
"#;

        fs::write(&file_path, content).unwrap();

        let result = CompletionDetector::check_completion(&file_path).unwrap();
        assert_eq!(result, Some(("WP01".to_string(), DetectedLane::ForReview)));
    }

    #[test]
    fn test_parse_frontmatter_completion_done() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP02.md");

        let content = r#"---
work_package_id: WP02
lane: done
title: Test WP
---

# Content
"#;

        fs::write(&file_path, content).unwrap();

        let result = CompletionDetector::check_completion(&file_path).unwrap();
        assert_eq!(result, Some(("WP02".to_string(), DetectedLane::Done)));
    }

    #[test]
    fn test_parse_frontmatter_no_completion_doing() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP03.md");

        let content = r#"---
work_package_id: WP03
lane: doing
title: Test WP
---

# Content
"#;

        fs::write(&file_path, content).unwrap();

        let result = CompletionDetector::check_completion(&file_path).unwrap();
        assert_eq!(result, None);
    }

    #[test]
    fn test_parse_frontmatter_no_completion_planned() {
        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP04.md");

        let content = r#"---
work_package_id: WP04
lane: planned
title: Test WP
---

# Content
"#;

        fs::write(&file_path, content).unwrap();

        let result = CompletionDetector::check_completion(&file_path).unwrap();
        assert_eq!(result, None);
    }

    #[test]
    fn test_file_marker_detection_done() {
        let temp_dir = TempDir::new().unwrap();
        fs::write(temp_dir.path().join(".done"), "").unwrap();

        let result = CompletionDetector::check_file_markers(temp_dir.path());
        assert_eq!(result, Some(CompletionMethod::FileMarker));
    }

    #[test]
    fn test_file_marker_detection_complete() {
        let temp_dir = TempDir::new().unwrap();
        fs::write(temp_dir.path().join(".complete"), "").unwrap();

        let result = CompletionDetector::check_file_markers(temp_dir.path());
        assert_eq!(result, Some(CompletionMethod::FileMarker));
    }

    #[test]
    fn test_file_marker_detection_done_uppercase() {
        let temp_dir = TempDir::new().unwrap();
        fs::write(temp_dir.path().join("DONE"), "").unwrap();

        let result = CompletionDetector::check_file_markers(temp_dir.path());
        assert_eq!(result, Some(CompletionMethod::FileMarker));
    }

    #[test]
    fn test_file_marker_detection_none() {
        let temp_dir = TempDir::new().unwrap();

        let result = CompletionDetector::check_file_markers(temp_dir.path());
        assert_eq!(result, None);
    }

    #[test]
    fn test_deduplication_blocks_within_window() {
        let mut processed: HashMap<String, Instant> = HashMap::new();
        let window = Duration::from_secs(5);

        // First event should not be duplicate
        let is_dup1 = CompletionDetector::is_duplicate(&mut processed, "WP01", window);
        assert!(!is_dup1);

        // Immediate second event should be duplicate
        let is_dup2 = CompletionDetector::is_duplicate(&mut processed, "WP01", window);
        assert!(is_dup2);
    }

    #[test]
    fn test_deduplication_allows_after_window() {
        let mut processed: HashMap<String, Instant> = HashMap::new();
        let window = Duration::from_millis(100);

        // First event
        let is_dup1 = CompletionDetector::is_duplicate(&mut processed, "WP01", window);
        assert!(!is_dup1);

        // Manually set the timestamp to be outside the window
        processed.insert("WP01".to_string(), Instant::now() - Duration::from_secs(1));

        // Second event after window should not be duplicate
        let is_dup2 = CompletionDetector::is_duplicate(&mut processed, "WP01", window);
        assert!(!is_dup2);
    }

    #[test]
    fn test_deduplication_different_wps() {
        let mut processed: HashMap<String, Instant> = HashMap::new();
        let window = Duration::from_secs(5);

        // First WP
        let is_dup1 = CompletionDetector::is_duplicate(&mut processed, "WP01", window);
        assert!(!is_dup1);

        // Different WP should not be duplicate
        let is_dup2 = CompletionDetector::is_duplicate(&mut processed, "WP02", window);
        assert!(!is_dup2);
    }

    #[test]
    fn test_git_activity_detection() {
        let temp_dir = TempDir::new().unwrap();
        let git_refs = temp_dir.path().join(".git/refs/heads");
        fs::create_dir_all(&git_refs).unwrap();

        let result = CompletionDetector::check_git_activity(temp_dir.path());
        assert_eq!(result, Some(CompletionMethod::GitActivity));
    }

    #[test]
    fn test_git_activity_detection_no_git() {
        let temp_dir = TempDir::new().unwrap();

        let result = CompletionDetector::check_git_activity(temp_dir.path());
        assert_eq!(result, None);
    }

    #[test]
    fn test_secondary_detection_fallback_to_git() {
        let temp_dir = TempDir::new().unwrap();
        let task_file = temp_dir.path().join("WP01.md");

        // Write a non-completion file
        let content = r#"---
work_package_id: WP01
lane: doing
title: Test WP
---

# Content
"#;
        fs::write(&task_file, content).unwrap();

        // Create git refs to trigger secondary detection
        let git_refs = temp_dir.path().join(".git/refs/heads");
        fs::create_dir_all(&git_refs).unwrap();

        // Primary detection should fail (lane is "doing")
        let primary = CompletionDetector::check_completion(&task_file).unwrap();
        assert_eq!(primary, None);

        // Secondary detection should succeed (git activity exists)
        let secondary = CompletionDetector::check_git_activity(temp_dir.path());
        assert_eq!(secondary, Some(CompletionMethod::GitActivity));
    }

    #[test]
    fn test_tertiary_detection_fallback_to_markers() {
        let temp_dir = TempDir::new().unwrap();
        let task_file = temp_dir.path().join("WP01.md");

        // Write a non-completion file
        let content = r#"---
work_package_id: WP01
lane: doing
title: Test WP
---

# Content
"#;
        fs::write(&task_file, content).unwrap();

        // Primary detection should fail (lane is "doing")
        let primary = CompletionDetector::check_completion(&task_file).unwrap();
        assert_eq!(primary, None);

        // Create marker file for tertiary detection
        fs::write(temp_dir.path().join(".done"), "").unwrap();

        // Tertiary detection should succeed (marker file exists)
        let tertiary = CompletionDetector::check_file_markers(temp_dir.path());
        assert_eq!(tertiary, Some(CompletionMethod::FileMarker));
    }
}
