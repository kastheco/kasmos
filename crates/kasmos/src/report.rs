//! Post-run summary report generation.
//!
//! Called by the orchestrator at run completion. Not yet wired into the
//! binary entry point — will be integrated when the finalize command lands.

use anyhow::{Context, Result};
use std::path::Path;
use std::time::SystemTime;

/// Generate a markdown summary report at `.kasmos/report.md`.
pub fn generate_report(kasmos_dir: &Path, run: &kasmos::OrchestrationRun) -> Result<()> {
    let report_path = kasmos_dir.join("report.md");
    let content = format_report(run);

    std::fs::write(&report_path, &content).context("Failed to write report")?;

    tracing::info!(path = %report_path.display(), "Report generated");
    Ok(())
}

fn format_report(run: &kasmos::OrchestrationRun) -> String {
    let mut report = String::new();

    // Header
    report.push_str(&format!("# Orchestration Report: {}\n\n", run.id));
    report.push_str(&format!("**Feature:** {}\n", run.feature));
    report.push_str(&format!("**State:** {:?}\n", run.state));
    report.push_str(&format!("**Mode:** {:?}\n", run.mode));

    if let Some(started) = run.started_at {
        report.push_str(&format!("**Started:** {}\n", format_system_time(started)));
    }
    if let Some(completed) = run.completed_at {
        report.push_str(&format!(
            "**Completed:** {}\n",
            format_system_time(completed)
        ));
    }

    report.push('\n');

    // Summary statistics
    let total = run.work_packages.len();
    let completed = run
        .work_packages
        .iter()
        .filter(|wp| wp.state == kasmos::WPState::Completed)
        .count();
    let failed = run
        .work_packages
        .iter()
        .filter(|wp| wp.state == kasmos::WPState::Failed)
        .count();
    let pending = run
        .work_packages
        .iter()
        .filter(|wp| wp.state == kasmos::WPState::Pending)
        .count();
    let active = run
        .work_packages
        .iter()
        .filter(|wp| wp.state == kasmos::WPState::Active)
        .count();

    report.push_str("## Summary\n\n");
    report.push_str("| Metric | Value |\n");
    report.push_str("|--------|-------|\n");
    report.push_str(&format!("| Total WPs | {} |\n", total));
    report.push_str(&format!("| Completed | {} |\n", completed));
    report.push_str(&format!("| Failed | {} |\n", failed));
    report.push_str(&format!("| Pending | {} |\n", pending));
    report.push_str(&format!("| Active | {} |\n", active));
    report.push_str(&format!("| Waves | {} |\n", run.waves.len()));
    report.push('\n');

    // Per-WP details
    if !run.work_packages.is_empty() {
        report.push_str("## Work Packages\n\n");
        report.push_str("| WP | Title | State | Attempts | Duration |\n");
        report.push_str("|----|-------|-------|----------|----------|\n");

        for wp in &run.work_packages {
            let duration = match (&wp.started_at, &wp.completed_at) {
                (Some(start), Some(end)) => format_duration(*start, *end),
                (Some(_), None) => "in progress".to_string(),
                _ => "-".to_string(),
            };

            let attempts = wp.failure_count
                + if wp.state == kasmos::WPState::Completed {
                    1
                } else {
                    0
                };

            report.push_str(&format!(
                "| {} | {} | {:?} | {} | {} |\n",
                wp.id, wp.title, wp.state, attempts, duration
            ));
        }
        report.push('\n');
    }

    // Wave summary
    if !run.waves.is_empty() {
        report.push_str("## Waves\n\n");
        for wave in &run.waves {
            report.push_str(&format!(
                "- **Wave {}**: {:?} — WPs: {}\n",
                wave.index,
                wave.state,
                wave.wp_ids.join(", ")
            ));
        }
        report.push('\n');
    }

    report
}

fn format_system_time(time: SystemTime) -> String {
    match time.duration_since(SystemTime::UNIX_EPOCH) {
        Ok(duration) => {
            let secs = duration.as_secs();
            let datetime =
                chrono::DateTime::from_timestamp(secs as i64, 0).unwrap_or_else(chrono::Utc::now);
            datetime.to_rfc3339()
        }
        Err(_) => "unknown".to_string(),
    }
}

fn format_duration(start: SystemTime, end: SystemTime) -> String {
    match end.duration_since(start) {
        Ok(duration) => {
            let secs = duration.as_secs();
            if secs < 60 {
                format!("{}s", secs)
            } else if secs < 3600 {
                format!("{}m {}s", secs / 60, secs % 60)
            } else {
                format!("{}h {}m", secs / 3600, (secs % 3600) / 60)
            }
        }
        Err(_) => "-".to_string(),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use kasmos::types::*;
    use std::time::{Duration, SystemTime};
    use tempfile::TempDir;

    fn test_run() -> kasmos::OrchestrationRun {
        let now = SystemTime::now();
        let later = now + Duration::from_secs(330); // 5m 30s

        kasmos::OrchestrationRun {
            id: "run-20260210-120000".into(),
            feature: "test-feature".into(),
            feature_dir: std::path::PathBuf::from("/tmp/test"),
            config: kasmos::Config::default(),
            work_packages: vec![
                WorkPackage {
                    id: "WP01".into(),
                    title: "First Package".into(),
                    state: WPState::Completed,
                    dependencies: vec![],
                    wave: 0,
                    pane_id: None,
                    pane_name: "wp01-pane".into(),
                    worktree_path: None,
                    prompt_path: None,
                    started_at: Some(now),
                    completed_at: Some(later),
                    completion_method: Some(CompletionMethod::AutoDetected),
                    failure_count: 0,
                },
                WorkPackage {
                    id: "WP02".into(),
                    title: "Second Package".into(),
                    state: WPState::Failed,
                    dependencies: vec!["WP01".into()],
                    wave: 1,
                    pane_id: None,
                    pane_name: "wp02-pane".into(),
                    worktree_path: None,
                    prompt_path: None,
                    started_at: Some(later),
                    completed_at: None,
                    completion_method: None,
                    failure_count: 2,
                },
            ],
            waves: vec![
                Wave {
                    index: 0,
                    wp_ids: vec!["WP01".into()],
                    state: WaveState::Completed,
                },
                Wave {
                    index: 1,
                    wp_ids: vec!["WP02".into()],
                    state: WaveState::Active,
                },
            ],
            state: RunState::Running,
            started_at: Some(now),
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[test]
    fn test_report_generation() {
        let temp_dir = TempDir::new().expect("tmp");
        let run = test_run();

        generate_report(temp_dir.path(), &run).expect("report");

        let report_path = temp_dir.path().join("report.md");
        assert!(report_path.exists());

        let content = std::fs::read_to_string(&report_path).expect("read");
        assert!(content.contains("run-20260210-120000"));
        assert!(content.contains("test-feature"));
        assert!(content.contains("WP01"));
        assert!(content.contains("WP02"));
        assert!(content.contains("5m 30s")); // WP01 duration
    }

    #[test]
    fn test_report_format_with_empty_run() {
        let run = kasmos::OrchestrationRun {
            id: "empty-run".into(),
            feature: "empty".into(),
            feature_dir: std::path::PathBuf::from("/tmp"),
            config: kasmos::Config::default(),
            work_packages: vec![],
            waves: vec![],
            state: RunState::Completed,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        };

        let content = format_report(&run);
        assert!(content.contains("empty-run"));
        assert!(content.contains("Total WPs | 0"));
    }

    #[test]
    fn test_duration_formatting() {
        let start = SystemTime::now();
        let end_45s = start + Duration::from_secs(45);
        let end_5m30s = start + Duration::from_secs(330);
        let end_2h30m = start + Duration::from_secs(9000);

        assert_eq!(format_duration(start, end_45s), "45s");
        assert_eq!(format_duration(start, end_5m30s), "5m 30s");
        assert_eq!(format_duration(start, end_2h30m), "2h 30m");
    }
}
