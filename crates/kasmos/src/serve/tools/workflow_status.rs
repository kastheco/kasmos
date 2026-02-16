use crate::graph::DependencyGraph;
use crate::parser::{FeatureDir, parse_frontmatter};
use crate::serve::KasmosServer;
use crate::serve::registry::{WorkerEntry, WorkerStatus};
use crate::types::{WPState, WorkPackage};
use anyhow::{Context, Result, anyhow};
use chrono::{DateTime, Utc};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct WorkflowStatusInput {
    pub feature_slug: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WorkflowStatusOutput {
    pub ok: bool,
    pub snapshot: WorkflowSnapshot,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WorkflowSnapshot {
    pub feature_slug: String,
    pub phase: String,
    pub waves: Vec<WaveInfo>,
    pub lock: LockInfo,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub active_workers: Vec<WorkerEntry>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_event_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct WaveInfo {
    pub wave: u64,
    pub wp_ids: Vec<String>,
    pub complete: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct LockInfo {
    pub state: LockState,
    pub owner_id: Option<String>,
    pub expires_at: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum LockState {
    Active,
    Stale,
    None,
}

#[derive(Debug, Clone)]
struct ParsedWp {
    wp_id: String,
    dependencies: Vec<String>,
    lane: String,
}

#[derive(Debug, Clone)]
struct ArtifactSnapshot {
    has_spec: bool,
    has_plan: bool,
    has_tasks_md: bool,
    has_clarification_artifacts: bool,
    has_analysis_artifacts: bool,
    has_release_complete_artifacts: bool,
    task_file_count: usize,
    parsed_wps: Vec<ParsedWp>,
}

#[derive(Debug, Deserialize)]
struct StoredLockRecord {
    #[serde(default)]
    status: Option<String>,
    #[serde(default)]
    owner_id: Option<String>,
    #[serde(default)]
    expires_at: Option<String>,
    #[serde(default)]
    last_heartbeat_at: Option<String>,
}

pub async fn handle(
    input: WorkflowStatusInput,
    server: &KasmosServer,
) -> Result<WorkflowStatusOutput> {
    let feature_dir = super::resolve_feature_dir(
        Path::new(&server.config.paths.specs_root),
        &input.feature_slug,
    )?;
    let snapshot = scan_artifacts(&feature_dir)?;
    let lock = read_lock_info(&server.config, &feature_dir, &input.feature_slug)?;
    let phase = determine_phase(&snapshot);
    let waves = compute_waves(&snapshot)?;
    let (active_workers, last_event_at) = worker_snapshot(server).await;

    Ok(WorkflowStatusOutput {
        ok: true,
        snapshot: WorkflowSnapshot {
            feature_slug: input.feature_slug,
            phase,
            waves,
            lock,
            active_workers,
            last_event_at,
        },
    })
}

async fn worker_snapshot(server: &KasmosServer) -> (Vec<WorkerEntry>, Option<String>) {
    let workers = {
        let registry = server.registry.read().await;
        registry.list().cloned().collect::<Vec<_>>()
    };

    let active_workers = workers
        .iter()
        .filter(|worker| worker.status == WorkerStatus::Active)
        .cloned()
        .collect::<Vec<_>>();

    let last_event_at = workers
        .iter()
        .filter_map(|worker| {
            worker
                .updated_at
                .as_deref()
                .or(Some(worker.spawned_at.as_str()))
        })
        .filter_map(parse_rfc3339_utc)
        .max()
        .map(|timestamp| timestamp.to_rfc3339());

    (active_workers, last_event_at)
}

fn parse_rfc3339_utc(value: &str) -> Option<DateTime<Utc>> {
    DateTime::parse_from_rfc3339(value)
        .ok()
        .map(|timestamp| timestamp.with_timezone(&Utc))
}

fn scan_artifacts(feature_dir: &Path) -> Result<ArtifactSnapshot> {
    let has_spec = feature_dir.join("spec.md").is_file();
    let has_plan = feature_dir.join("plan.md").is_file();
    let has_tasks_md = feature_dir.join("tasks.md").is_file();
    let has_clarification_artifacts = has_any_file(
        feature_dir,
        &["clarify.md", "clarifications.md", "clarification.md"],
    );
    let has_analysis_artifacts = has_any_file(
        feature_dir,
        &["analysis.md", "analyze.md", "analysis-report.md"],
    );
    let has_release_complete_artifacts = has_any_file(
        feature_dir,
        &["released.md", ".release-complete", "release-complete.md"],
    );

    let feature = FeatureDir::scan(feature_dir).with_context(|| {
        format!(
            "Failed to scan feature directory {} for task files",
            feature_dir.display()
        )
    })?;
    let task_file_count = feature.wp_files.len();
    let mut parsed_wps = Vec::new();
    for wp_file in feature.wp_files {
        match parse_frontmatter(&wp_file) {
            Ok(fm) => parsed_wps.push(ParsedWp {
                wp_id: fm.work_package_id,
                dependencies: fm.dependencies,
                lane: fm.lane,
            }),
            Err(err) => {
                tracing::warn!(
                    file = %wp_file.display(),
                    error = %err,
                    "Skipping unparsable task file while computing workflow status"
                );
            }
        }
    }

    Ok(ArtifactSnapshot {
        has_spec,
        has_plan,
        has_tasks_md,
        has_clarification_artifacts,
        has_analysis_artifacts,
        has_release_complete_artifacts,
        task_file_count,
        parsed_wps,
    })
}

fn has_any_file(dir: &Path, candidates: &[&str]) -> bool {
    candidates.iter().any(|name| dir.join(name).is_file())
}

fn determine_phase(snapshot: &ArtifactSnapshot) -> String {
    if !snapshot.has_spec {
        return "spec_only".to_string();
    }

    if snapshot.task_file_count > 0 {
        let any_doing = snapshot.parsed_wps.iter().any(|wp| wp.lane == "doing");
        if any_doing {
            return "implementing".to_string();
        }

        let any_for_review = snapshot.parsed_wps.iter().any(|wp| wp.lane == "for_review");
        if any_for_review {
            return "reviewing".to_string();
        }

        let all_done = !snapshot.parsed_wps.is_empty()
            && snapshot.parsed_wps.iter().all(|wp| wp.lane == "done");
        if all_done {
            if snapshot.has_release_complete_artifacts {
                return "complete".to_string();
            }
            return "releasing".to_string();
        }

        return "tasked".to_string();
    }

    if snapshot.has_plan {
        if snapshot.has_tasks_md && snapshot.has_analysis_artifacts {
            return "analyzing".to_string();
        }
        return "planned".to_string();
    }

    if snapshot.has_clarification_artifacts {
        return "clarifying".to_string();
    }

    "spec_only".to_string()
}

fn compute_waves(snapshot: &ArtifactSnapshot) -> Result<Vec<WaveInfo>> {
    if snapshot.parsed_wps.is_empty() {
        return Ok(Vec::new());
    }

    let graph_input = snapshot
        .parsed_wps
        .iter()
        .map(|wp| WorkPackage {
            id: wp.wp_id.clone(),
            title: wp.wp_id.clone(),
            state: WPState::Pending,
            dependencies: wp.dependencies.clone(),
            wave: 0,
            pane_id: None,
            pane_name: format!("{}-pane", wp.wp_id.to_lowercase()),
            worktree_path: None,
            prompt_path: None,
            started_at: None,
            completed_at: None,
            completion_method: None,
            failure_count: 0,
        })
        .collect::<Vec<_>>();

    let graph = DependencyGraph::new(&graph_input);
    let waves = graph.compute_waves()?;

    let lane_by_wp = snapshot
        .parsed_wps
        .iter()
        .map(|wp| (wp.wp_id.clone(), wp.lane.clone()))
        .collect::<std::collections::HashMap<_, _>>();

    Ok(waves
        .into_iter()
        .enumerate()
        .map(|(index, wp_ids)| {
            let complete = wp_ids
                .iter()
                .all(|id| lane_by_wp.get(id).is_some_and(|lane| lane == "done"));
            WaveInfo {
                wave: index as u64,
                wp_ids,
                complete,
            }
        })
        .collect())
}

fn read_lock_info(
    config: &crate::config::Config,
    feature_dir: &Path,
    feature_slug: &str,
) -> Result<LockInfo> {
    let lock_path = lock_file_path(
        Path::new(&config.paths.specs_root),
        feature_dir,
        feature_slug,
    )?;
    if !lock_path.is_file() {
        return Ok(LockInfo {
            state: LockState::None,
            owner_id: None,
            expires_at: None,
        });
    }

    let raw = std::fs::read_to_string(&lock_path)
        .with_context(|| format!("Failed to read lock file {}", lock_path.display()))?;
    let record: StoredLockRecord = serde_json::from_str(&raw)
        .with_context(|| format!("Failed to parse lock file {}", lock_path.display()))?;

    let stale = is_stale_lock(config, &record);
    let state = if stale {
        LockState::Stale
    } else {
        match record.status.as_deref() {
            Some("released") => LockState::None,
            Some("stale") => LockState::Stale,
            _ => LockState::Active,
        }
    };

    Ok(LockInfo {
        state,
        owner_id: record.owner_id,
        expires_at: record.expires_at,
    })
}

fn lock_file_path(specs_root: &Path, feature_dir: &Path, feature_slug: &str) -> Result<PathBuf> {
    let specs_root = specs_root.canonicalize().with_context(|| {
        format!(
            "Failed to canonicalize specs root while locating lock file: {}",
            specs_root.display()
        )
    })?;

    let specs_dir = if specs_root == feature_dir {
        feature_dir.parent().ok_or_else(|| {
            anyhow!(
                "Unable to resolve specs directory from feature path {}",
                feature_dir.display()
            )
        })?
    } else if feature_dir.starts_with(&specs_root) {
        specs_root.as_path()
    } else {
        feature_dir.parent().ok_or_else(|| {
            anyhow!(
                "Unable to resolve specs directory from feature path {}",
                feature_dir.display()
            )
        })?
    };

    let repo_root = specs_dir.parent().ok_or_else(|| {
        anyhow!(
            "Unable to locate repository root from specs path {}",
            specs_dir.display()
        )
    })?;

    Ok(repo_root
        .join(".kasmos")
        .join("locks")
        .join(format!("{}.lock", feature_slug)))
}

fn is_stale_lock(config: &crate::config::Config, record: &StoredLockRecord) -> bool {
    if matches!(record.status.as_deref(), Some("stale")) {
        return true;
    }

    if let Some(expires_at) = record.expires_at.as_deref()
        && let Ok(ts) = DateTime::parse_from_rfc3339(expires_at)
    {
        return Utc::now() > ts.with_timezone(&Utc);
    }

    if let Some(last_heartbeat_at) = record.last_heartbeat_at.as_deref()
        && let Ok(ts) = DateTime::parse_from_rfc3339(last_heartbeat_at)
    {
        let age = Utc::now() - ts.with_timezone(&Utc);
        return age > chrono::Duration::minutes(config.lock.stale_timeout_minutes as i64);
    }

    false
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::serve::registry::AgentRole;
    use tempfile::tempdir;

    fn write_wp(feature_dir: &Path, file_name: &str, lane: &str, deps: &[&str]) {
        let deps_yaml = if deps.is_empty() {
            "[]".to_string()
        } else {
            format!("[{}]", deps.join(", "))
        };
        let content = format!(
            "---\nwork_package_id: {}\ntitle: test\nlane: {}\ndependencies: {}\n---\n\n# body\n",
            file_name.split('-').next().unwrap_or("WP00"),
            lane,
            deps_yaml
        );
        std::fs::write(feature_dir.join("tasks").join(file_name), content).expect("write wp");
    }

    #[tokio::test]
    async fn workflow_status_reports_implementing_and_waves() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        let feature_dir = specs_root.join("011-alpha");
        std::fs::create_dir_all(feature_dir.join("tasks")).expect("mkdir tasks");
        std::fs::write(feature_dir.join("spec.md"), "# spec").expect("write spec");
        std::fs::write(feature_dir.join("plan.md"), "# plan").expect("write plan");
        std::fs::write(feature_dir.join("tasks.md"), "# tasks").expect("write tasks md");
        write_wp(&feature_dir, "WP01-root.md", "doing", &[]);
        write_wp(&feature_dir, "WP02-child.md", "planned", &["WP01"]);

        let mut config = crate::config::Config::default();
        config.paths.specs_root = specs_root.display().to_string();
        let server = crate::serve::KasmosServer::new(config).expect("server");

        let output = handle(
            WorkflowStatusInput {
                feature_slug: "011-alpha".to_string(),
            },
            &server,
        )
        .await
        .expect("status");

        assert!(output.ok);
        assert_eq!(output.snapshot.phase, "implementing");
        assert_eq!(output.snapshot.waves.len(), 2);
        assert_eq!(output.snapshot.waves[0].wp_ids, vec!["WP01"]);
        assert_eq!(output.snapshot.waves[1].wp_ids, vec!["WP02"]);
        assert!(!output.snapshot.waves[0].complete);
        assert!(!output.snapshot.waves[1].complete);
    }

    #[tokio::test]
    async fn workflow_status_reports_reviewing_when_no_doing() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        let feature_dir = specs_root.join("011-beta");
        std::fs::create_dir_all(feature_dir.join("tasks")).expect("mkdir tasks");
        std::fs::write(feature_dir.join("spec.md"), "# spec").expect("write spec");
        std::fs::write(feature_dir.join("plan.md"), "# plan").expect("write plan");
        std::fs::write(feature_dir.join("tasks.md"), "# tasks").expect("write tasks md");
        write_wp(&feature_dir, "WP01-root.md", "for_review", &[]);

        let mut config = crate::config::Config::default();
        config.paths.specs_root = specs_root.display().to_string();
        let server = crate::serve::KasmosServer::new(config).expect("server");

        let output = handle(
            WorkflowStatusInput {
                feature_slug: "011-beta".to_string(),
            },
            &server,
        )
        .await
        .expect("status");

        assert_eq!(output.snapshot.phase, "reviewing");
        assert!(output.snapshot.active_workers.is_empty());
        assert!(output.snapshot.last_event_at.is_none());
    }

    #[tokio::test]
    async fn workflow_status_includes_active_workers_and_last_event_at() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        let feature_dir = specs_root.join("011-workers");
        std::fs::create_dir_all(feature_dir.join("tasks")).expect("mkdir tasks");
        std::fs::write(feature_dir.join("spec.md"), "# spec").expect("write spec");

        let mut config = crate::config::Config::default();
        config.paths.specs_root = specs_root.display().to_string();
        let server = crate::serve::KasmosServer::new(config).expect("server");

        crate::serve::tools::spawn_worker::handle(
            &server,
            crate::serve::tools::spawn_worker::SpawnWorkerInput {
                wp_id: "WP01".to_string(),
                role: AgentRole::Coder,
                prompt: "work".to_string(),
                feature_slug: "011-workers".to_string(),
                worktree_path: None,
            },
        )
        .await
        .expect("spawn worker");

        let output = handle(
            WorkflowStatusInput {
                feature_slug: "011-workers".to_string(),
            },
            &server,
        )
        .await
        .expect("status");

        assert_eq!(output.snapshot.active_workers.len(), 1);
        assert_eq!(output.snapshot.active_workers[0].wp_id, "WP01");
        assert!(output.snapshot.last_event_at.is_some());
    }

    #[test]
    fn determine_phase_returns_tasked_when_all_planned() {
        let snapshot = ArtifactSnapshot {
            has_spec: true,
            has_plan: true,
            has_tasks_md: true,
            has_clarification_artifacts: false,
            has_analysis_artifacts: false,
            has_release_complete_artifacts: false,
            task_file_count: 2,
            parsed_wps: vec![
                ParsedWp {
                    wp_id: "WP01".to_string(),
                    dependencies: vec![],
                    lane: "planned".to_string(),
                },
                ParsedWp {
                    wp_id: "WP02".to_string(),
                    dependencies: vec!["WP01".to_string()],
                    lane: "planned".to_string(),
                },
            ],
        };
        assert_eq!(determine_phase(&snapshot), "tasked");
    }

    #[test]
    fn can_parse_frontmatter_yaml_segment() {
        let content = "---\nwork_package_id: WP01\nlane: planned\n---\n\n# body\n";
        let parts: Vec<&str> = content.splitn(3, "---").collect();
        let parsed: serde_yml::Value =
            serde_yml::from_str(parts[1].trim()).expect("frontmatter value");
        assert_eq!(parsed["work_package_id"], "WP01");
    }

    #[test]
    fn lock_file_path_resolves_repo_root_from_specs_root() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("specs");
        let feature_dir = specs_root.join("011-alpha");
        std::fs::create_dir_all(&feature_dir).expect("mkdir feature");

        let path = lock_file_path(&specs_root, &feature_dir, "011-alpha").expect("lock path");
        assert_eq!(
            path,
            tmp.path()
                .join(".kasmos")
                .join("locks")
                .join("011-alpha.lock")
        );
    }

    #[test]
    fn lock_file_path_handles_specs_root_set_to_feature_dir() {
        let tmp = tempdir().expect("tempdir");
        let specs_root = tmp.path().join("specs");
        let feature_dir = specs_root.join("011-beta");
        std::fs::create_dir_all(&feature_dir).expect("mkdir feature");

        let path = lock_file_path(&feature_dir, &feature_dir, "011-beta").expect("lock path");
        assert_eq!(
            path,
            tmp.path()
                .join(".kasmos")
                .join("locks")
                .join("011-beta.lock")
        );
    }
}
