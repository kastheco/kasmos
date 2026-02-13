//! TUI preview mode — animated mock data for TUI development.
//!
//! Launches the TUI with synthetic `OrchestrationRun` data and a background
//! animation loop that cycles work packages through all states. No Zellij,
//! no git, no orchestration engine — just the UI with fake data.

use std::path::PathBuf;
use std::time::{Duration, SystemTime};

use kasmos::command_handlers::EngineAction;
use kasmos::config::Config;
use kasmos::types::{
    CompletionMethod, OrchestrationRun, ProgressionMode, RunState, WPState, Wave, WaveState,
    WorkPackage,
};
use tokio::sync::{mpsc, watch};

/// Realistic work package titles for the mock data.
const TITLES: &[&str] = &[
    "Add CLI argument parser",
    "Implement state machine",
    "Write integration tests",
    "Set up CI pipeline",
    "Add logging framework",
    "Create database schema",
    "Build REST API endpoints",
    "Implement auth middleware",
    "Add WebSocket support",
    "Write unit tests",
    "Create deployment scripts",
    "Add metrics collection",
    "Implement rate limiting",
    "Build admin dashboard",
    "Add error recovery logic",
    "Create data migration tool",
    "Implement caching layer",
    "Add health check endpoint",
    "Build notification system",
    "Write API documentation",
    "Implement search indexer",
    "Add audit logging",
    "Create backup system",
    "Build plugin framework",
    "Add i18n support",
];

/// Entry point for the TUI preview mode.
///
/// Creates mock data, spawns an animation loop, and launches the TUI.
pub async fn run(count: usize) -> anyhow::Result<()> {
    anyhow::ensure!(count >= 1, "count must be at least 1");
    let initial_run = generate_mock_run(count);
    let (watch_tx, watch_rx) = watch::channel(initial_run.clone());
    let (action_tx, _action_rx) = mpsc::channel::<EngineAction>(64);

    tokio::spawn(animation_loop(watch_tx, initial_run));

    kasmos::tui::run(watch_rx, action_tx).await
}

/// Generate a mock `OrchestrationRun` with `count` work packages across 3 waves.
fn generate_mock_run(count: usize) -> OrchestrationRun {
    let wave_size = (count + 2) / 3; // ceiling division for even distribution
    let now = SystemTime::now();

    let mut work_packages = Vec::with_capacity(count);
    for i in 0..count {
        let wave_idx = (i / wave_size).min(2);
        let wp_id = format!("WP{:02}", i + 1);

        // Initial states: wave 0 gets a mix of states to showcase all kanban lanes,
        // waves 1-2 start Pending. For small counts (< 5), start all wave-0 WPs as
        // Active to ensure visible animation.
        let (state, started_at, completed_at, completion_method, failure_count) =
            if wave_idx == 0 && wave_size >= 5 {
                match i % 5 {
                    0 => (WPState::Completed, Some(now), Some(now), Some(CompletionMethod::AutoDetected), 0),
                    1 => (WPState::Active, Some(now), None, None, 0),
                    2 => (WPState::ForReview, Some(now), None, None, 0),
                    3 => (WPState::Failed, Some(now), None, None, 1),
                    _ => (WPState::Active, Some(now), None, None, 0),
                }
            } else if wave_idx == 0 {
                (WPState::Active, Some(now), None, None, 0)
            } else {
                (WPState::Pending, None, None, None, 0)
            };

        // Dependencies: wave 1 depends on wave 0; wave 2 depends on wave 1
        let dependencies = if wave_idx > 0 {
            let dep_wave = wave_idx - 1;
            (0..count)
                .filter(|&j| (j / wave_size).min(2) == dep_wave)
                .map(|j| format!("WP{:02}", j + 1))
                .collect()
        } else {
            vec![]
        };

        work_packages.push(WorkPackage {
            id: wp_id,
            title: TITLES[i % TITLES.len()].to_string(),
            state,
            dependencies,
            wave: wave_idx,
            pane_id: None,
            pane_name: format!("wp{:02}", i + 1),
            worktree_path: None,
            prompt_path: None,
            started_at,
            completed_at,
            completion_method,
            failure_count,
        });
    }

    // Build waves
    let mut waves: Vec<Wave> = (0..3)
        .map(|w| Wave {
            index: w,
            wp_ids: work_packages
                .iter()
                .filter(|wp| wp.wave == w)
                .map(|wp| wp.id.clone())
                .collect(),
            state: WaveState::Pending,
        })
        .filter(|w| !w.wp_ids.is_empty())
        .collect();

    // Derive initial wave states
    for wave in &mut waves {
        wave.state = derive_wave_state(&work_packages, &wave.wp_ids);
    }

    OrchestrationRun {
        id: "preview-run-1".to_string(),
        feature: "preview-demo".to_string(),
        feature_dir: PathBuf::from("/tmp/kasmos-preview"),
        config: Config::default(),
        work_packages,
        waves,
        state: RunState::Running,
        started_at: Some(now),
        completed_at: None,
        mode: ProgressionMode::WaveGated,
    }
}

/// Derive the wave state from its constituent work packages.
fn derive_wave_state(work_packages: &[WorkPackage], wp_ids: &[String]) -> WaveState {
    let states: Vec<WPState> = work_packages
        .iter()
        .filter(|wp| wp_ids.contains(&wp.id))
        .map(|wp| wp.state)
        .collect();

    if states.iter().all(|s| *s == WPState::Completed) {
        WaveState::Completed
    } else if states.iter().any(|s| matches!(s, WPState::Active | WPState::ForReview)) {
        WaveState::Active
    } else if states.iter().any(|s| *s == WPState::Failed) {
        WaveState::PartiallyFailed
    } else {
        WaveState::Pending
    }
}

/// Background animation loop that advances WP states every ~3 seconds.
async fn animation_loop(watch_tx: watch::Sender<OrchestrationRun>, initial_run: OrchestrationRun) {
    let mut run = initial_run.clone();
    let mut tick: usize = 0;

    loop {
        tokio::time::sleep(Duration::from_secs(3)).await;

        // Find non-Completed WPs (round-robin selection)
        let candidates: Vec<usize> = run
            .work_packages
            .iter()
            .enumerate()
            .filter(|(_, wp)| wp.state != WPState::Completed)
            .map(|(i, _)| i)
            .collect();

        if candidates.is_empty() {
            // All completed — show completion state briefly, then reset
            run.state = RunState::Completed;
            run.completed_at = Some(SystemTime::now());
            let _ = watch_tx.send(run);

            tokio::time::sleep(Duration::from_secs(2)).await;

            // Reset to initial state
            run = initial_run.clone();
            let _ = watch_tx.send(run.clone());
            tick = 0;
            continue;
        }

        let idx = candidates[tick % candidates.len()];
        let now = SystemTime::now();

        match run.work_packages[idx].state {
            WPState::Pending => {
                run.work_packages[idx].state = WPState::Active;
                run.work_packages[idx].started_at = Some(now);
            }
            WPState::Active => {
                if tick % 7 == 0 {
                    // ~14.3% chance of failure
                    run.work_packages[idx].state = WPState::Failed;
                    run.work_packages[idx].failure_count += 1;
                } else {
                    run.work_packages[idx].state = WPState::ForReview;
                }
            }
            WPState::ForReview => {
                run.work_packages[idx].state = WPState::Completed;
                run.work_packages[idx].completed_at = Some(now);
                run.work_packages[idx].completion_method = Some(CompletionMethod::AutoDetected);
            }
            WPState::Failed => {
                // Retry: go back to Active
                run.work_packages[idx].state = WPState::Active;
                run.work_packages[idx].started_at = Some(now);
            }
            _ => {}
        }

        // Update wave states
        for wave in &mut run.waves {
            wave.state = derive_wave_state(&run.work_packages, &wave.wp_ids);
        }

        run.state = RunState::Running;
        let _ = watch_tx.send(run.clone());
        tick += 1;
    }
}
