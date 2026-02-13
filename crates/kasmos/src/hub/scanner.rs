//! Feature scanner — reads `kitty-specs/` and produces feature status entries.
//!
//! The scanner is synchronous (`Send + Sync`) so it can run inside
//! `tokio::task::spawn_blocking` without blocking the async event loop.

use std::path::{Path, PathBuf};

// Re-use the frontmatter struct shape from list_specs.
#[derive(serde::Deserialize)]
struct WpFrontmatter {
    lane: Option<String>,
}

// ---------------------------------------------------------------------------
// Status types
// ---------------------------------------------------------------------------

/// Status of a feature's specification file.
#[derive(Debug, Clone, PartialEq)]
pub enum SpecStatus {
    /// `spec.md` missing or zero-length.
    Empty,
    /// `spec.md` exists and is non-empty.
    Present,
}

/// Status of a feature's implementation plan.
#[derive(Debug, Clone, PartialEq)]
pub enum PlanStatus {
    /// `plan.md` does not exist.
    Absent,
    /// `plan.md` exists.
    Present,
}

/// Progress of work packages for a feature.
#[derive(Debug, Clone, PartialEq)]
pub enum TaskProgress {
    /// `tasks/` directory missing or no `WPxx-*.md` files.
    NoTasks,
    /// Some WPs exist, not all done.
    InProgress { done: usize, total: usize },
    /// All WPs have lane `"done"`.
    Complete { total: usize },
}

/// Status of orchestration for a feature.
#[derive(Debug, Clone, PartialEq)]
pub enum OrchestrationStatus {
    /// No lock file or dead PID, no Zellij session.
    None,
    /// Live lock file PID AND Zellij session exists.
    Running,
    /// No live process but Zellij session exists (EXITED state).
    Completed,
}

/// A feature discovered in `kitty-specs/`.
#[derive(Debug, Clone)]
pub struct FeatureEntry {
    /// Feature number for sorting and display (e.g., `"010"`).
    pub number: String,
    /// Feature slug for display (e.g., `"hub-tui-navigator"`).
    pub slug: String,
    /// Full directory name (e.g., `"010-hub-tui-navigator"`).
    pub full_slug: String,
    /// Whether the feature has a specification.
    pub spec_status: SpecStatus,
    /// Whether the feature has a plan.
    pub plan_status: PlanStatus,
    /// WP completion state.
    pub task_progress: TaskProgress,
    /// Whether orchestration is running.
    pub orchestration_status: OrchestrationStatus,
    /// Absolute path to `kitty-specs/<full_slug>/`.
    pub feature_dir: PathBuf,
}

// ---------------------------------------------------------------------------
// Scanner
// ---------------------------------------------------------------------------

/// Scans `kitty-specs/` and builds a sorted list of [`FeatureEntry`] values.
pub struct FeatureScanner {
    specs_root: PathBuf,
}

impl FeatureScanner {
    pub fn new(specs_root: PathBuf) -> Self {
        Self { specs_root }
    }

    /// Perform a full scan. Returns an empty vec if the directory doesn't exist.
    pub fn scan(&self) -> Vec<FeatureEntry> {
        let entries = match std::fs::read_dir(&self.specs_root) {
            Ok(rd) => rd,
            Err(_) => return Vec::new(),
        };

        // Collect Zellij sessions once for all features.
        let zellij_sessions = list_zellij_sessions();

        // Derive repo root from specs_root parent (kitty-specs/ lives at repo root).
        let repo_root = self.specs_root.parent().unwrap_or(Path::new("."));

        let mut features: Vec<FeatureEntry> = entries
            .filter_map(|e| e.ok())
            .filter(|e| e.path().is_dir())
            .filter_map(|e| {
                let name = e.file_name();
                let name_str = name.to_str()?;
                let (number, slug) = name_str.split_once('-')?;

                let feature_dir = e.path();
                let full_slug = name_str.to_string();

                let spec_status = check_spec_status(&feature_dir);
                let plan_status = check_plan_status(&feature_dir);
                let task_progress = check_task_progress(&feature_dir);
                let orchestration_status = check_orchestration_status(
                    &full_slug,
                    repo_root,
                    &feature_dir,
                    &zellij_sessions,
                );

                Some(FeatureEntry {
                    number: number.to_string(),
                    slug: slug.to_string(),
                    full_slug,
                    spec_status,
                    plan_status,
                    task_progress,
                    orchestration_status,
                    feature_dir,
                })
            })
            .collect();

        features.sort_by(|a, b| a.number.cmp(&b.number));
        features
    }
}

// ---------------------------------------------------------------------------
// Status checks
// ---------------------------------------------------------------------------

fn check_spec_status(feature_dir: &Path) -> SpecStatus {
    let spec_path = feature_dir.join("spec.md");
    if spec_path.is_file() && std::fs::metadata(&spec_path).map_or(false, |m| m.len() > 0) {
        SpecStatus::Present
    } else {
        SpecStatus::Empty
    }
}

fn check_plan_status(feature_dir: &Path) -> PlanStatus {
    if feature_dir.join("plan.md").is_file() {
        PlanStatus::Present
    } else {
        PlanStatus::Absent
    }
}

fn check_task_progress(feature_dir: &Path) -> TaskProgress {
    let tasks_dir = feature_dir.join("tasks");
    let rd = match std::fs::read_dir(&tasks_dir) {
        Ok(rd) => rd,
        Err(_) => return TaskProgress::NoTasks,
    };

    let mut done = 0usize;
    let mut total = 0usize;

    for entry in rd.filter_map(|e| e.ok()) {
        let name = entry.file_name();
        let Some(name_str) = name.to_str() else {
            continue;
        };
        if !name_str.starts_with("WP") || !name_str.ends_with(".md") {
            continue;
        }

        total += 1;

        if let Some(lane) = extract_lane(&entry.path()) {
            if lane == "done" {
                done += 1;
            }
        }
    }

    if total == 0 {
        TaskProgress::NoTasks
    } else if done == total {
        TaskProgress::Complete { total }
    } else {
        TaskProgress::InProgress { done, total }
    }
}

fn extract_lane(path: &Path) -> Option<String> {
    let content = std::fs::read_to_string(path).ok()?;
    let body = content.strip_prefix("---")?;
    let end = body.find("\n---")?;
    let fm: WpFrontmatter = serde_yml::from_str(&body[..end]).ok()?;
    fm.lane
}

/// Determine orchestration status for a feature.
///
/// Checks both the feature directory itself and the worktree directory
/// (`.worktrees/<full_slug>/.kasmos/run.lock`) for a lock file.
fn check_orchestration_status(
    full_slug: &str,
    repo_root: &Path,
    feature_dir: &Path,
    zellij_sessions: &[String],
) -> OrchestrationStatus {
    // Session name convention: kasmos-<full_slug>
    let session_name = format!("kasmos-{full_slug}");
    let has_session = zellij_sessions.iter().any(|s| s == &session_name);

    // Check lock file in feature dir and worktree dir.
    let lock_paths = [
        feature_dir.join(".kasmos/run.lock"),
        repo_root.join(format!(".worktrees/{full_slug}/.kasmos/run.lock")),
    ];

    let pid_alive = lock_paths.iter().any(|lock_path| {
        if let Ok(content) = std::fs::read_to_string(lock_path) {
            if let Ok(pid) = content.trim().parse::<u32>() {
                return is_pid_alive(pid);
            }
        }
        false
    });

    match (pid_alive, has_session) {
        (true, true) => OrchestrationStatus::Running,
        (false, true) => OrchestrationStatus::Completed,
        _ => OrchestrationStatus::None,
    }
}

fn is_pid_alive(pid: u32) -> bool {
    // SAFETY: kill(pid, 0) checks process existence without sending a signal.
    unsafe { libc::kill(pid as i32, 0) == 0 }
}

/// List active Zellij sessions. Returns empty vec if Zellij is unavailable.
fn list_zellij_sessions() -> Vec<String> {
    let output = match std::process::Command::new("zellij")
        .args(["list-sessions", "--short", "--no-formatting"])
        .output()
    {
        Ok(o) if o.status.success() => o,
        _ => return Vec::new(),
    };

    String::from_utf8_lossy(&output.stdout)
        .lines()
        .map(|l| l.trim().to_string())
        .filter(|l| !l.is_empty())
        .collect()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    /// Helper: create a feature directory structure in a temp dir.
    struct TestFixture {
        _tmp: tempfile::TempDir,
        specs_root: PathBuf,
    }

    impl TestFixture {
        fn new() -> Self {
            let tmp = tempfile::tempdir().unwrap();
            let specs_root = tmp.path().join("kitty-specs");
            std::fs::create_dir_all(&specs_root).unwrap();
            Self {
                _tmp: tmp,
                specs_root,
            }
        }

        fn add_feature(&self, name: &str) -> PathBuf {
            let dir = self.specs_root.join(name);
            std::fs::create_dir_all(&dir).unwrap();
            dir
        }

        fn scanner(&self) -> FeatureScanner {
            FeatureScanner::new(self.specs_root.clone())
        }
    }

    fn wp_frontmatter(lane: &str) -> String {
        format!("---\nlane: \"{lane}\"\n---\n# WP")
    }

    // -- Empty / missing --

    #[test]
    fn empty_specs_directory() {
        let fix = TestFixture::new();
        let features = fix.scanner().scan();
        assert!(features.is_empty());
    }

    #[test]
    fn missing_specs_directory() {
        let scanner = FeatureScanner::new(PathBuf::from("/tmp/nonexistent-kasmos-test-dir"));
        let features = scanner.scan();
        assert!(features.is_empty());
    }

    // -- Spec status --

    #[test]
    fn feature_with_empty_spec() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        // No spec.md at all
        let features = fix.scanner().scan();
        assert_eq!(features.len(), 1);
        assert_eq!(features[0].spec_status, SpecStatus::Empty);

        // Zero-length spec.md
        std::fs::write(dir.join("spec.md"), "").unwrap();
        let features = fix.scanner().scan();
        assert_eq!(features[0].spec_status, SpecStatus::Empty);
    }

    #[test]
    fn feature_with_present_spec() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        std::fs::write(dir.join("spec.md"), "# Spec\nContent here").unwrap();

        let features = fix.scanner().scan();
        assert_eq!(features.len(), 1);
        assert_eq!(features[0].spec_status, SpecStatus::Present);
    }

    // -- Plan status --

    #[test]
    fn feature_without_plan() {
        let fix = TestFixture::new();
        fix.add_feature("001-alpha");

        let features = fix.scanner().scan();
        assert_eq!(features[0].plan_status, PlanStatus::Absent);
    }

    #[test]
    fn feature_with_plan() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        std::fs::write(dir.join("plan.md"), "# Plan").unwrap();

        let features = fix.scanner().scan();
        assert_eq!(features[0].plan_status, PlanStatus::Present);
    }

    // -- Task progress --

    #[test]
    fn feature_with_no_tasks_dir() {
        let fix = TestFixture::new();
        fix.add_feature("001-alpha");

        let features = fix.scanner().scan();
        assert_eq!(features[0].task_progress, TaskProgress::NoTasks);
    }

    #[test]
    fn feature_with_empty_tasks_dir() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        std::fs::create_dir_all(dir.join("tasks")).unwrap();

        let features = fix.scanner().scan();
        assert_eq!(features[0].task_progress, TaskProgress::NoTasks);
    }

    #[test]
    fn feature_with_in_progress_tasks() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        let tasks = dir.join("tasks");
        std::fs::create_dir_all(&tasks).unwrap();
        std::fs::write(tasks.join("WP01-setup.md"), wp_frontmatter("done")).unwrap();
        std::fs::write(tasks.join("WP02-impl.md"), wp_frontmatter("doing")).unwrap();
        std::fs::write(tasks.join("WP03-test.md"), wp_frontmatter("planned")).unwrap();

        let features = fix.scanner().scan();
        assert_eq!(
            features[0].task_progress,
            TaskProgress::InProgress { done: 1, total: 3 }
        );
    }

    #[test]
    fn feature_with_all_tasks_done() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        let tasks = dir.join("tasks");
        std::fs::create_dir_all(&tasks).unwrap();
        std::fs::write(tasks.join("WP01-setup.md"), wp_frontmatter("done")).unwrap();
        std::fs::write(tasks.join("WP02-impl.md"), wp_frontmatter("done")).unwrap();

        let features = fix.scanner().scan();
        assert_eq!(
            features[0].task_progress,
            TaskProgress::Complete { total: 2 }
        );
    }

    #[test]
    fn non_wp_files_ignored_in_tasks() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        let tasks = dir.join("tasks");
        std::fs::create_dir_all(&tasks).unwrap();
        std::fs::write(tasks.join("WP01-setup.md"), wp_frontmatter("done")).unwrap();
        std::fs::write(tasks.join("README.md"), "# Readme").unwrap();
        std::fs::write(tasks.join("notes.txt"), "notes").unwrap();

        let features = fix.scanner().scan();
        assert_eq!(
            features[0].task_progress,
            TaskProgress::Complete { total: 1 }
        );
    }

    #[test]
    fn malformed_frontmatter_treated_as_not_done() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-alpha");
        let tasks = dir.join("tasks");
        std::fs::create_dir_all(&tasks).unwrap();
        std::fs::write(tasks.join("WP01-setup.md"), wp_frontmatter("done")).unwrap();
        // Malformed: no frontmatter delimiters
        std::fs::write(tasks.join("WP02-broken.md"), "no frontmatter here").unwrap();

        let features = fix.scanner().scan();
        assert_eq!(
            features[0].task_progress,
            TaskProgress::InProgress { done: 1, total: 2 }
        );
    }

    // -- Sorting --

    #[test]
    fn features_sorted_by_number() {
        let fix = TestFixture::new();
        fix.add_feature("003-charlie");
        fix.add_feature("001-alpha");
        fix.add_feature("002-bravo");

        let features = fix.scanner().scan();
        assert_eq!(features.len(), 3);
        assert_eq!(features[0].number, "001");
        assert_eq!(features[1].number, "002");
        assert_eq!(features[2].number, "003");
    }

    // -- Slug parsing --

    #[test]
    fn invalid_directory_names_skipped() {
        let fix = TestFixture::new();
        fix.add_feature("001-alpha");
        // No hyphen — should be skipped
        fix.add_feature("nodash");
        // Valid
        fix.add_feature("002-bravo");

        let features = fix.scanner().scan();
        assert_eq!(features.len(), 2);
        assert_eq!(features[0].slug, "alpha");
        assert_eq!(features[1].slug, "bravo");
    }

    #[test]
    fn multi_hyphen_slug_parsed_correctly() {
        let fix = TestFixture::new();
        fix.add_feature("010-hub-tui-navigator");

        let features = fix.scanner().scan();
        assert_eq!(features.len(), 1);
        assert_eq!(features[0].number, "010");
        assert_eq!(features[0].slug, "hub-tui-navigator");
        assert_eq!(features[0].full_slug, "010-hub-tui-navigator");
    }

    // -- Orchestration status (limited — no live PID mocking) --

    #[test]
    fn orchestration_none_when_no_lock_no_session() {
        let fix = TestFixture::new();
        fix.add_feature("001-alpha");

        let features = fix.scanner().scan();
        assert_eq!(features[0].orchestration_status, OrchestrationStatus::None);
    }

    // -- Full integration --

    #[test]
    fn full_feature_scan() {
        let fix = TestFixture::new();
        let dir = fix.add_feature("001-my-feature");
        std::fs::write(dir.join("spec.md"), "# Spec\nContent here").unwrap();
        std::fs::write(dir.join("plan.md"), "# Plan").unwrap();
        let tasks = dir.join("tasks");
        std::fs::create_dir_all(&tasks).unwrap();
        std::fs::write(tasks.join("WP01-setup.md"), wp_frontmatter("done")).unwrap();
        std::fs::write(tasks.join("WP02-impl.md"), wp_frontmatter("doing")).unwrap();

        let features = fix.scanner().scan();
        assert_eq!(features.len(), 1);
        let f = &features[0];
        assert_eq!(f.number, "001");
        assert_eq!(f.slug, "my-feature");
        assert_eq!(f.full_slug, "001-my-feature");
        assert_eq!(f.spec_status, SpecStatus::Present);
        assert_eq!(f.plan_status, PlanStatus::Present);
        assert_eq!(
            f.task_progress,
            TaskProgress::InProgress { done: 1, total: 2 }
        );
        assert_eq!(f.orchestration_status, OrchestrationStatus::None);
    }
}
