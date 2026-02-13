//! Start orchestration for a feature: setup, create Zellij session, attach.

use anyhow::{Context, Result, bail};
// ZellijCli trait used via Arc<dyn ...> in ReviewCoordinator
#[allow(unused_imports)]
use kasmos::ZellijCli;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::SystemTime;
use tokio::sync::{RwLock, mpsc, watch};

/// Acquires a run lock to prevent concurrent orchestrations.
fn acquire_lock(kasmos_dir: &Path) -> Result<LockGuard> {
    let lock_path = kasmos_dir.join("run.lock");

    if lock_path.exists() {
        let content = std::fs::read_to_string(&lock_path).unwrap_or_default();
        if let Ok(pid) = content.trim().parse::<u32>()
            && is_pid_alive(pid) {
                bail!(
                    "Another orchestration is running (PID {}). \
                     Use 'kasmos stop' first or remove {}",
                    pid,
                    lock_path.display()
                );
            }
        tracing::warn!(path = %lock_path.display(), "Removing stale lock file");
    }

    let pid = std::process::id();
    std::fs::write(&lock_path, pid.to_string()).context("Failed to write lock file")?;

    Ok(LockGuard { lock_path })
}

fn is_pid_alive(pid: u32) -> bool {
    unsafe { libc::kill(pid as i32, 0) == 0 }
}

/// RAII guard that removes the lock file on drop.
struct LockGuard {
    lock_path: PathBuf,
}

impl Drop for LockGuard {
    fn drop(&mut self) {
        if self.lock_path.exists() {
            let _ = std::fs::remove_file(&self.lock_path);
            tracing::debug!(path = %self.lock_path.display(), "Lock released");
        }
    }
}

/// Main start entry point: setup session, launch TUI dashboard.
pub async fn run(feature: &str, mode: &str) -> Result<()> {
    let _span = tracing::info_span!("start", feature = %feature).entered();
    tracing::info!("Starting orchestration");

    // Resolve feature directory
    let feature_dir = crate::feature_arg::resolve_feature_dir(feature)?;
    let feature_name = feature_dir
        .file_name()
        .and_then(|n| n.to_str())
        .unwrap_or("unknown")
        .to_string();

    // Load config
    let mut config = kasmos::Config::default();
    let config_path = feature_dir.join(".kasmos/config.toml");
    if config_path.exists() {
        config
            .load_from_file(&config_path)
            .context("Failed to load config file")?;
    }
    config
        .load_from_env()
        .context("Failed to load config from environment")?;
    config.validate().context("Invalid configuration")?;

    let kasmos_dir = feature_dir.join(&config.kasmos_dir);
    std::fs::create_dir_all(&kasmos_dir).context("Failed to create .kasmos directory")?;

    let _lock = acquire_lock(&kasmos_dir)?;

    let progression_mode = match mode {
        "wave-gated" | "wave_gated" => kasmos::ProgressionMode::WaveGated,
        _ => kasmos::ProgressionMode::Continuous,
    };

    tracing::info!(
        mode = %mode,
        kasmos_dir = %kasmos_dir.display(),
        "Configuration loaded"
    );

    // ── Phase 1: Pre-session setup ──────────────────────────────────

    // Validate required binaries
    kasmos::prompt::validate_binary_in_path(&config.zellij_binary)
        .context("zellij binary not found")?;
    kasmos::prompt::validate_binary_in_path(&config.opencode_binary)
        .context("ocx binary not found")?;
    tracing::info!("Required binaries validated");

    // Initialize git worktree manager
    let worktree_mgr = kasmos::WorktreeManager::new(&feature_dir, &feature_name)
        .context("Failed to initialize git worktree manager")?;
    let base_ref = worktree_mgr
        .current_ref()
        .context("Failed to determine current git ref")?;
    tracing::info!(
        repo_root = %worktree_mgr.repo_root().display(),
        base_ref = %base_ref,
        "Git worktree manager ready"
    );

    // Scan feature directory for WP files
    let feature_scan = kasmos::FeatureDir::scan(&feature_dir)
        .context("Failed to scan feature directory")?;
    tracing::info!(wp_count = feature_scan.wp_files.len(), "Feature directory scanned");

    if feature_scan.wp_files.is_empty() {
        bail!(
            "No work package files found in {}/tasks/. Expected WPxx-*.md files.",
            feature_dir.display()
        );
    }

    // Parse frontmatter
    let mut frontmatters = Vec::new();
    for wp_file in &feature_scan.wp_files {
        let fm = kasmos::parse_frontmatter(wp_file)
            .with_context(|| format!("Failed to parse {}", wp_file.display()))?;
        frontmatters.push((fm, wp_file.clone()));
    }
    tracing::info!(count = frontmatters.len(), "Frontmatter parsed");

    // Build WorkPackages, honouring the frontmatter lane so that WPs
    // already marked "done" or "for_review" aren't re-launched.
    // worktree_path is initially None — we'll set it after computing waves.
    let mut work_packages: Vec<kasmos::WorkPackage> = frontmatters
        .iter()
        .map(|(fm, _wp_file)| {
            let wp_id = &fm.work_package_id;

            let (state, completion_method) = lane_to_wp_state(&fm.lane);
            if state != kasmos::WPState::Pending {
                tracing::info!(
                    wp_id = %wp_id,
                    lane = %fm.lane,
                    state = ?state,
                    "WP already progressed — seeding initial state from frontmatter"
                );
            }

            kasmos::WorkPackage {
                id: wp_id.clone(),
                title: fm.title.clone(),
                state,
                dependencies: fm.dependencies.clone(),
                wave: 0,
                pane_id: None,
                pane_name: format!("{}-pane", wp_id.to_lowercase()),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method,
                failure_count: 0,
            }
        })
        .collect();

    // Build dependency graph and compute waves
    let graph = kasmos::DependencyGraph::new(&work_packages);
    let wave_groups = graph.compute_waves().context("Failed to compute wave assignments")?;

    for (wave_idx, wave_wp_ids) in wave_groups.iter().enumerate() {
        for wp_id in wave_wp_ids {
            if let Some(wp) = work_packages.iter_mut().find(|w| &w.id == wp_id) {
                wp.wave = wave_idx;
            }
        }
    }

    let waves: Vec<kasmos::Wave> = wave_groups
        .iter()
        .enumerate()
        .map(|(idx, wp_ids)| kasmos::Wave {
            index: idx,
            wp_ids: wp_ids.clone(),
            state: kasmos::WaveState::Pending,
        })
        .collect();

    tracing::info!(waves = waves.len(), "Dependency graph computed: {} waves", waves.len());
    for (i, wave) in waves.iter().enumerate() {
        tracing::info!(wave = i, wps = ?wave.wp_ids, "Wave {}", i);
    }

    // ── Locate or create git worktrees ──
    //
    // Each WP needs its own worktree for isolation. spec-kitty may have
    // already created worktrees via `spec-kitty implement WPxx`. For any
    // WP without a pre-existing worktree, kasmos creates one automatically
    // so agents never run in a shared working tree.
    worktree_mgr.prune().ok(); // clean up stale references first
    let mut worktree_found = 0usize;
    let mut worktree_created = 0usize;
    for wp in work_packages.iter_mut() {
        // Skip WPs that are already terminal — they don't need worktrees
        if matches!(
            wp.state,
            kasmos::WPState::Completed | kasmos::WPState::Failed
        ) {
            tracing::debug!(
                wp_id = %wp.id,
                state = ?wp.state,
                "Skipping worktree for terminal-state WP"
            );
            continue;
        }

        if let Some(path) = worktree_mgr.find_worktree(&wp.id) {
            tracing::info!(
                wp_id = %wp.id,
                wave = wp.wave,
                path = %path.display(),
                "Found existing worktree"
            );
            wp.worktree_path = Some(path);
            worktree_found += 1;
        } else {
            // No pre-existing worktree — create one to ensure WP isolation
            match worktree_mgr.ensure_worktree(&wp.id, &base_ref) {
                Ok(path) => {
                    tracing::info!(
                        wp_id = %wp.id,
                        wave = wp.wave,
                        path = %path.display(),
                        "Created worktree for WP isolation"
                    );
                    wp.worktree_path = Some(path);
                    worktree_created += 1;
                }
                Err(e) => {
                    tracing::error!(
                        wp_id = %wp.id,
                        error = %e,
                        "Failed to create worktree — WP will run in feature dir (NO ISOLATION)"
                    );
                    // Fall back to feature_dir — bad but non-fatal
                }
            }
        }
    }
    tracing::info!(
        found = worktree_found,
        created = worktree_created,
        total = work_packages.len(),
        "Git worktrees ready"
    );

    // Generate prompt files
    let prompt_gen = kasmos::PromptGenerator::new(&feature_dir)
        .context("Failed to initialize prompt generator")?;
    let prompt_paths = prompt_gen
        .generate_all(&work_packages, &kasmos_dir)
        .context("Failed to generate prompt files")?;
    let _script_paths = prompt_gen
        .generate_scripts(&work_packages, &kasmos_dir)
        .context("Failed to generate scripts")?;

    for (wp, prompt_path) in work_packages.iter_mut().zip(prompt_paths.iter()) {
        wp.prompt_path = Some(prompt_path.clone());
    }
    tracing::info!(count = prompt_paths.len(), "Prompt files generated");

    // Persist orchestration state
    let run_id = format!("run-{}", chrono::Utc::now().format("%Y%m%d-%H%M%S"));
    let run = kasmos::OrchestrationRun {
        id: run_id.clone(),
        feature: feature_name.clone(),
        feature_dir: feature_dir.clone(),
        config: config.clone(),
        work_packages,
        waves,
        state: kasmos::RunState::Running,
        started_at: Some(SystemTime::now()),
        completed_at: None,
        mode: progression_mode,
    };

    // Create watch channel for TUI state broadcasting
    let (watch_tx, watch_rx) = watch::channel(run.clone());

    let persister = kasmos::StatePersister::new(&kasmos_dir);
    persister.ensure_dir()?;
    persister.save(&run).context("Failed to persist state")?;
    tracing::info!(run_id = %run_id, "Orchestration run initialized");

    // Session name used for logging and identifiers (no separate session created).
    let session_name = format!("kasmos-{}", feature_name);

    // ── Phase 2: Start orchestration engine & command pipeline ───

    let run_arc = Arc::new(RwLock::new(run));

    // Channels
    let (command_tx, mut command_rx) = mpsc::channel::<kasmos::ControllerCommand>(64);
    let (engine_action_tx, engine_action_rx) = mpsc::channel::<kasmos::EngineAction>(64);
    let (completion_tx, completion_rx) = mpsc::channel::<kasmos::CompletionEvent>(64);
    let (launch_tx, mut launch_rx) = mpsc::channel::<kasmos::WaveLaunchEvent>(16);

    // CommandReader — creates the FIFO and spawns the reader task
    let command_reader = kasmos::CommandReader::new(&kasmos_dir, command_tx)
        .context("Failed to create command FIFO reader")?;
    let _reader_handle = command_reader
        .start()
        .await
        .context("Failed to start command reader")?;
    tracing::info!("Command FIFO reader started");

    // Shutdown coordinator
    let (shutdown_coordinator, _shutdown_rx) = kasmos::ShutdownCoordinator::new(&kasmos_dir);

    // Signal handlers (Ctrl-C triggers graceful shutdown)
    let shutdown_flag = shutdown_coordinator.flag();
    let _signal_handle = kasmos::setup_signal_handlers(Box::new(move || {
        shutdown_flag.store(true, std::sync::atomic::Ordering::SeqCst);
    }));

    // Command handler bridge: ControllerCommand → EngineAction
    // Uses a stub SessionController for now; focus/zoom commands will log but not navigate.
    // Clone action_tx before moving into CommandHandler — TUI needs its own sender.
    let tui_action_tx = engine_action_tx.clone();
    let session_controller = Arc::new(StubSessionController);
    let command_handler = kasmos::CommandHandler::new(
        run_arc.clone(),
        session_controller,
        engine_action_tx,
    );
    let command_handler = Arc::new(command_handler);

    let handler_clone = command_handler.clone();
    let bridge_kasmos_dir = kasmos_dir.clone();
    let _cmd_bridge_handle = tokio::spawn(async move {
        let status_path = bridge_kasmos_dir.join("status.txt");

        while let Some(cmd) = command_rx.recv().await {
            let is_status = matches!(cmd, kasmos::ControllerCommand::Status);

            match handler_clone.handle(cmd).await {
                Ok(msg) => {
                    if is_status {
                        // Write status to file instead of logging (avoids tracing garbling)
                        if let Err(e) = std::fs::write(&status_path, &msg) {
                            tracing::error!("Failed to write status file: {}", e);
                        } else {
                            tracing::info!(
                                path = %status_path.display(),
                                "Status written to file. View with: kasmos status"
                            );
                        }
                    } else {
                        tracing::info!("{}", msg);
                    }
                }
                Err(e) => tracing::error!("Command handling failed: {}", e),
            }
        }
        tracing::debug!("Command bridge task exiting");
    });

    // Review automation channel
    let (review_tx, review_rx) = mpsc::channel::<kasmos::ReviewRequest>(16);

    // WaveEngine — drives wave progression, receives completions and actions
    let engine_persister = Arc::new(persister);
    let mut engine =
        kasmos::WaveEngine::new(run_arc.clone(), completion_rx, engine_action_rx, launch_tx)
            .with_persister(engine_persister)
            .with_watch_tx(watch_tx);
    engine.set_review_tx(review_tx);
    let _engine_handle = tokio::spawn(async move {
        if let Err(e) = engine.run().await {
            tracing::error!("Wave engine error: {}", e);
        }
        tracing::info!("Wave engine stopped");
    });

    tracing::info!("Orchestration engine started");

    // ── Completion detector ─────────────────────────────────────
    //
    // Watches WP task files for spec-kitty lane transitions (frontmatter).
    // When a lane changes to "for_review" or "done", emits a CompletionEvent
    // that the engine uses to update WP state and progress waves.
    let detector_paths: Vec<(String, PathBuf, PathBuf)> = {
        let run = run_arc.read().await;
        run.work_packages
            .iter()
            .filter(|wp| wp.state == kasmos::WPState::Pending || wp.state == kasmos::WPState::Active)
            .filter_map(|wp| {
                // Find the task file for this WP
                let task_file = feature_scan
                    .wp_files
                    .iter()
                    .find(|f| {
                        f.file_name()
                            .and_then(|n| n.to_str())
                            .map(|n| n.starts_with(&wp.id))
                            .unwrap_or(false)
                    })
                    .cloned();

                let task_file = task_file?;
                // Worktree root: use worktree_path if set, otherwise feature_dir
                let worktree_root = wp
                    .worktree_path
                    .clone()
                    .unwrap_or_else(|| feature_dir.clone());

                Some((wp.id.clone(), task_file, worktree_root))
            })
            .collect()
    };

    // _detector must live until the session ends — if dropped, the watcher stops.
    let _detector: Option<kasmos::CompletionDetector>;

    if !detector_paths.is_empty() {
        let mut detector = kasmos::CompletionDetector::new();
        match detector.start(detector_paths.clone()).await {
            Ok(mut detector_rx) => {
                tracing::info!(
                    watch_count = detector_paths.len(),
                    "Completion detector started"
                );

                // Bridge: forward detector events to the engine's completion channel
                let _detector_bridge = tokio::spawn(async move {
                    while let Some(event) = detector_rx.recv().await {
                        tracing::info!(
                            wp_id = %event.wp_id,
                            method = ?event.method,
                            "Completion event detected"
                        );
                        if let Err(e) = completion_tx.send(event).await {
                            tracing::error!("Failed to forward completion event: {}", e);
                            break;
                        }
                    }
                    tracing::debug!("Completion detector bridge exiting");
                });

                _detector = Some(detector);
            }
            Err(e) => {
                tracing::error!("Failed to start completion detector: {}", e);
                // Non-fatal: orchestration continues, but state won't auto-update
                _detector = None;
                let _completion_tx = completion_tx;
            }
        }
    } else {
        tracing::info!("No WPs to watch — completion detector not started");
        _detector = None;
        let _completion_tx = completion_tx;
    }

    // ── Wave launch event handler ────────────────────────────────
    //
    // When the engine launches a wave, this task creates a single "agents"
    // tab in the current Zellij session containing all the wave's WP panes
    // in a grid layout. Result: Tab 1 = TUI dashboard, Tab 2 = agents.
    let wave_run_arc = run_arc.clone();
    let wave_kasmos_dir = kasmos_dir.clone();
    let wave_config = config.clone();
    let wave_feature_dir = feature_dir.clone();
    let _wave_handler = tokio::spawn(async move {
        let layout_gen = kasmos::LayoutGenerator::new(&wave_config);

        while let Some(event) = launch_rx.recv().await {
            tracing::info!(
                wave = event.wave_index,
                wps = ?event.wp_ids,
                "Wave launch — creating agents tab"
            );

            // Collect WP data for the layout
            let wp_data: Vec<kasmos::WorkPackage> = {
                let run = wave_run_arc.read().await;
                event
                    .wp_ids
                    .iter()
                    .filter_map(|id| run.work_packages.iter().find(|wp| wp.id == *id).cloned())
                    .collect()
            };

            if wp_data.is_empty() {
                tracing::warn!(wave = event.wave_index, "No WP data found for wave launch");
                continue;
            }

            // Generate a grid layout for all WPs in this wave
            let wp_refs: Vec<&kasmos::WorkPackage> = wp_data.iter().collect();
            let wave_layout = match layout_gen.generate_wave_tab(&wp_refs, &wave_feature_dir) {
                Ok(doc) => doc,
                Err(e) => {
                    tracing::error!("Failed to generate agents layout: {}", e);
                    continue;
                }
            };

            let wave_layout_path = match layout_gen.write_wave_layout(
                &wave_layout,
                &wave_kasmos_dir,
                event.wave_index,
            ) {
                Ok(path) => path,
                Err(e) => {
                    tracing::error!("Failed to write agents layout: {}", e);
                    continue;
                }
            };

            // Create the agents tab with the grid layout
            let tab_name = format!("agents-w{}", event.wave_index);
            let result = tokio::process::Command::new("zellij")
                .args([
                    "action",
                    "new-tab",
                    "--layout",
                    &wave_layout_path.display().to_string(),
                    "--name",
                    &tab_name,
                ])
                .output()
                .await;

            match result {
                Ok(out) if out.status.success() => {
                    tracing::info!(
                        wave = event.wave_index,
                        tab = %tab_name,
                        panes = wp_data.len(),
                        "Agents tab created"
                    );
                }
                Ok(out) => {
                    tracing::error!(
                        wave = event.wave_index,
                        "Failed to create agents tab: {}",
                        String::from_utf8_lossy(&out.stderr)
                    );
                }
                Err(e) => {
                    tracing::error!(wave = event.wave_index, "Failed to run zellij: {}", e);
                }
            }

            // Switch back to the TUI tab so the user stays on the dashboard
            let _ = tokio::process::Command::new("zellij")
                .args(["action", "go-to-tab", "1"])
                .output()
                .await;
        }
        tracing::debug!("Wave launch handler exiting");
    });

    // ReviewCoordinator — spawns reviewer panes when WPs enter ForReview
    let review_coordinator = kasmos::ReviewCoordinator::new(
        session_name.clone(),
        config.opencode_binary.clone(),
        std::sync::Arc::new(kasmos::RealZellijCli::new(config.zellij_binary.clone())),
        review_rx,
        kasmos::ReviewAutomationPolicy::default(),
        kasmos_dir.clone(),
    );
    let _review_handle = tokio::spawn(async move {
        review_coordinator.run().await;
    });
    tracing::info!("Review coordinator started");

    // ── Phase 4: Launch TUI dashboard ─────────────────────────────

    tracing::info!("Launching TUI dashboard");
    match kasmos::tui::run(watch_rx, tui_action_tx).await {
        Ok(()) => {
            tracing::info!("TUI exited normally");
        }
        Err(e) => {
            tracing::error!("TUI error: {}", e);
            eprintln!("TUI error: {e:#}");
        }
    }

    tracing::info!("Session ended");
    Ok(())
}

/// Map a spec-kitty frontmatter lane to the corresponding WPState.
///
/// Lane values used by spec-kitty:
///   "planned"    → Pending   (not yet started)
///   "doing"      → Active    (currently being worked on)
///   "for_review" → ForReview (awaiting operator review)
///   "done"       → Completed (finished)
///
/// Unknown lanes default to Pending.
fn lane_to_wp_state(
    lane: &str,
) -> (kasmos::WPState, Option<kasmos::CompletionMethod>) {
    match lane {
        "done" => (
            kasmos::WPState::Completed,
            Some(kasmos::CompletionMethod::Manual),
        ),
        "for_review" => (kasmos::WPState::ForReview, None),
        "doing" => (kasmos::WPState::Active, None),
        // "planned" and anything else → Pending
        _ => (kasmos::WPState::Pending, None),
    }
}

/// Stub session controller for pane focus/zoom operations.
///
/// Logs the operation but does not perform real Zellij navigation.
/// This will be replaced with a real `SessionManager`-backed implementation
/// once the session manager is integrated into the orchestrator lifecycle.
struct StubSessionController;

#[async_trait::async_trait]
impl kasmos::SessionController for StubSessionController {
    async fn focus_pane(&self, wp_id: &str) -> kasmos::Result<()> {
        tracing::warn!(wp_id = %wp_id, "Focus not yet wired to session manager");
        Ok(())
    }

    async fn focus_and_zoom(&self, wp_id: &str) -> kasmos::Result<()> {
        tracing::warn!(wp_id = %wp_id, "Zoom not yet wired to session manager");
        Ok(())
    }
}
