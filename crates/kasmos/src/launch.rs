//! Launch orchestration for a feature.

use anyhow::{Context, Result, bail};
use std::path::{Path, PathBuf};
use std::time::SystemTime;

/// Resolves a feature argument to its directory path.
fn resolve_feature_dir(feature: &str) -> Result<PathBuf> {
    let path = PathBuf::from(feature);
    if !path.exists() {
        bail!("Feature directory does not exist: {}", path.display());
    }
    if !path.is_dir() {
        bail!("Feature path is not a directory: {}", path.display());
    }
    path
        .canonicalize()
        .context("Failed to canonicalize feature path")
}

/// Acquires a run lock to prevent concurrent orchestrations.
fn acquire_lock(kasmos_dir: &Path) -> Result<LockGuard> {
    let lock_path = kasmos_dir.join("run.lock");

    if lock_path.exists() {
        let content = std::fs::read_to_string(&lock_path).unwrap_or_default();
        // Check if PID is still alive
        if let Ok(pid) = content.trim().parse::<u32>()
            && is_pid_alive(pid) {
                bail!(
                    "Another orchestration is running (PID {}). \
                     Use 'kasmos stop' first or remove {}",
                    pid,
                    lock_path.display()
                );
            }
        // Stale lock — remove it
        tracing::warn!(path = %lock_path.display(), "Removing stale lock file");
    }

    let pid = std::process::id();
    std::fs::write(&lock_path, pid.to_string()).context("Failed to write lock file")?;

    Ok(LockGuard { lock_path })
}

fn is_pid_alive(pid: u32) -> bool {
    // Signal 0 checks if process exists without sending a signal
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

/// Main launch entry point — wires all 14 modules in order.
pub async fn run(feature: &str, mode: &str) -> Result<()> {
    let _span = tracing::info_span!("launch", feature = %feature).entered();

    // Step 1: Init logging (already done in main)
    tracing::info!("Starting orchestration");

    // Step 2: Load config
    let mut config = kasmos::Config::default();

    // Try to load from file, but don't fail if it doesn't exist
    let config_path = PathBuf::from(feature).join(".kasmos/config.toml");
    if config_path.exists() {
        config
            .load_from_file(&config_path)
            .context("Failed to load config file")?;
    }

    // Load from environment variables
    config
        .load_from_env()
        .context("Failed to load config from environment")?;

    // Validate config
    config.validate().context("Invalid configuration")?;

    // Resolve feature directory
    let feature_dir = resolve_feature_dir(feature)?;
    let kasmos_dir = feature_dir.join(&config.kasmos_dir);
    std::fs::create_dir_all(&kasmos_dir).context("Failed to create .kasmos directory")?;

    // Acquire run lock
    let _lock = acquire_lock(&kasmos_dir)?;

    // Parse progression mode
    let progression_mode = match mode {
        "wave-gated" | "wave_gated" => kasmos::ProgressionMode::WaveGated,
        _ => kasmos::ProgressionMode::Continuous,
    };

    tracing::info!(
        mode = %mode,
        kasmos_dir = %kasmos_dir.display(),
        "Configuration loaded"
    );

    // ── Phase 1: Pre-session setup (fail-fast on any error) ──────────

    // Step 3: Validate required binaries
    kasmos::prompt::validate_binary_in_path(&config.zellij_binary)
        .context("zellij binary not found")?;
    kasmos::prompt::validate_binary_in_path(&config.opencode_binary)
        .context("opencode binary not found")?;
    tracing::info!("Required binaries validated");

    // Step 4: Scan feature directory for WP files
    let feature_scan = kasmos::FeatureDir::scan(&feature_dir)
        .context("Failed to scan feature directory")?;
    tracing::info!(wp_count = feature_scan.wp_files.len(), "Feature directory scanned");

    if feature_scan.wp_files.is_empty() {
        bail!(
            "No work package files found in {}/tasks/. Expected WPxx-*.md files.",
            feature_dir.display()
        );
    }

    // Step 5: Parse frontmatter from each WP file
    let mut frontmatters = Vec::new();
    for wp_file in &feature_scan.wp_files {
        let fm = kasmos::parse_frontmatter(wp_file)
            .with_context(|| format!("Failed to parse {}", wp_file.display()))?;
        frontmatters.push((fm, wp_file.clone()));
    }
    tracing::info!(count = frontmatters.len(), "Frontmatter parsed");

    // Step 6: Build initial WorkPackages (without wave assignments yet)
    let feature_name = feature_dir
        .file_name()
        .and_then(|n| n.to_str())
        .unwrap_or("unknown")
        .to_string();

    let mut work_packages: Vec<kasmos::WorkPackage> = frontmatters
        .iter()
        .map(|(fm, _wp_file)| {
            let wp_id = &fm.work_package_id;
            // Resolve worktree path: look in .worktrees/<feature>-<WP_ID>/
            let worktree_path = feature_dir
                .parent()
                .map(|parent| {
                    parent
                        .join(".worktrees")
                        .join(format!("{}-{}", feature_name, wp_id))
                })
                .filter(|p| p.exists());

            kasmos::WorkPackage {
                id: wp_id.clone(),
                title: fm.title.clone(),
                state: kasmos::WPState::Pending,
                dependencies: fm.dependencies.clone(),
                wave: 0, // assigned below
                pane_id: None,
                pane_name: format!("{}-pane", wp_id.to_lowercase()),
                worktree_path,
                prompt_path: None, // set after prompt generation
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            }
        })
        .collect();

    // Step 7: Build dependency graph and compute wave assignments
    let graph = kasmos::DependencyGraph::new(&work_packages);
    let wave_groups = graph.compute_waves().context("Failed to compute wave assignments")?;

    // Assign wave indices to work packages
    for (wave_idx, wave_wp_ids) in wave_groups.iter().enumerate() {
        for wp_id in wave_wp_ids {
            if let Some(wp) = work_packages.iter_mut().find(|w| &w.id == wp_id) {
                wp.wave = wave_idx;
            }
        }
    }

    // Build Wave structs
    let waves: Vec<kasmos::Wave> = wave_groups
        .iter()
        .enumerate()
        .map(|(idx, wp_ids)| kasmos::Wave {
            index: idx,
            wp_ids: wp_ids.clone(),
            state: kasmos::WaveState::Pending,
        })
        .collect();

    tracing::info!(
        waves = waves.len(),
        "Dependency graph computed: {} waves",
        waves.len()
    );
    for (i, wave) in waves.iter().enumerate() {
        tracing::info!(wave = i, wps = ?wave.wp_ids, "Wave {}", i);
    }

    // Step 8: Generate prompt files and shell scripts
    let prompt_gen = kasmos::PromptGenerator::new(&feature_dir)
        .context("Failed to initialize prompt generator")?;
    let prompt_paths = prompt_gen
        .generate_all(&work_packages, &kasmos_dir)
        .context("Failed to generate prompt files")?;
    let _script_paths = prompt_gen
        .generate_scripts(&work_packages, &kasmos_dir)
        .context("Failed to generate scripts")?;

    // Assign prompt paths back to work packages
    for (wp, prompt_path) in work_packages.iter_mut().zip(prompt_paths.iter()) {
        wp.prompt_path = Some(prompt_path.clone());
    }
    tracing::info!(count = prompt_paths.len(), "Prompt files generated");

    // Step 9: Generate KDL layout
    let layout_gen = kasmos::LayoutGenerator::new(&config);
    let wp_refs: Vec<&kasmos::WorkPackage> = work_packages.iter().collect();
    let kdl_doc = layout_gen
        .generate(&wp_refs, &feature_dir)
        .context("Failed to generate KDL layout")?;
    let layout_path = layout_gen
        .write_layout(&kdl_doc, &kasmos_dir)
        .context("Failed to write layout file")?;
    tracing::info!(path = %layout_path.display(), "KDL layout written");

    // Step 10: Create orchestration run and persist initial state
    let run_id = format!("run-{}", chrono::Utc::now().format("%Y%m%d-%H%M%S"));
    let run = kasmos::OrchestrationRun {
        id: run_id.clone(),
        feature: feature.to_string(),
        feature_dir: feature_dir.clone(),
        config: config.clone(),
        work_packages,
        waves,
        state: kasmos::RunState::Initializing,
        started_at: Some(SystemTime::now()),
        completed_at: None,
        mode: progression_mode,
    };

    let persister = kasmos::StatePersister::new(&kasmos_dir);
    persister.ensure_dir()?;
    persister.save(&run).context("Failed to persist initial state")?;
    tracing::info!(run_id = %run_id, "Orchestration run initialized");

    // Step 11: Start Zellij session with layout
    let session_name = format!("kasmos-{}", feature_name);
    let zellij_cli = std::sync::Arc::new(kasmos::RealZellijCli::new(config.zellij_binary.clone()));
    let config_arc = std::sync::Arc::new(config.clone());
    let mut session_manager =
        kasmos::SessionManager::new(session_name.clone(), zellij_cli, config_arc)
            .context("Failed to create session manager")?;
    session_manager
        .start_session_with_layout(&layout_path)
        .await
        .context("Failed to start Zellij session")?;
    tracing::info!(session = %session_name, "Zellij session started");

    // ── Phase 2: Post-session wiring (graceful error handling) ─────

    // Wrap run in Arc<RwLock> for shared state
    let run = std::sync::Arc::new(tokio::sync::RwLock::new(run));

    // Step 12: Setup shutdown coordinator and signal handlers
    let (shutdown_coordinator, shutdown_rx) = kasmos::ShutdownCoordinator::new(&kasmos_dir);
    let shutdown_flag = shutdown_coordinator.flag();
    let _signal_handle = kasmos::setup_signal_handlers(Box::new(move || {
        shutdown_flag.store(true, std::sync::atomic::Ordering::SeqCst);
    }));

    // Step 13: Start completion detector
    let mut detector = kasmos::CompletionDetector::new();
    let wp_watch_paths: Vec<(String, PathBuf, PathBuf)> = {
        let r = run.read().await;
        r.work_packages
            .iter()
            .filter_map(|wp| {
                // Watch the task file in the feature dir (for lane transitions)
                let task_file = feature_scan
                    .wp_files
                    .iter()
                    .find(|f| {
                        f.file_name()
                            .and_then(|n| n.to_str())
                            .map(|s| s.starts_with(&wp.id))
                            .unwrap_or(false)
                    })
                    .cloned()?;
                let worktree = wp.worktree_path.clone().unwrap_or(feature_dir.clone());
                Some((wp.id.clone(), task_file, worktree))
            })
            .collect()
    };

    let completion_rx = if !wp_watch_paths.is_empty() {
        match detector.start(wp_watch_paths).await {
            Ok(rx) => {
                tracing::info!("Completion detector started");
                rx
            }
            Err(e) => {
                tracing::warn!(error = %e, "Failed to start completion detector, using empty channel");
                let (_tx, rx) = tokio::sync::mpsc::channel(1);
                rx
            }
        }
    } else {
        tracing::warn!("No WP paths to watch");
        let (_tx, rx) = tokio::sync::mpsc::channel(1);
        rx
    };

    // Step 14: Start health monitor
    let (crash_tx, mut crash_rx) = tokio::sync::mpsc::channel::<kasmos::CrashEvent>(32);
    let (health_reg_tx, health_reg_rx) =
        tokio::sync::mpsc::channel::<kasmos::PaneRegistration>(32);
    let health_monitor = kasmos::HealthMonitor::new(config.poll_interval_secs, crash_tx);

    // Register initial panes for health monitoring
    {
        let r = run.read().await;
        for wp in &r.work_packages {
            let _ = health_reg_tx
                .send(kasmos::PaneRegistration::Register {
                    wp_id: wp.id.clone(),
                    pane_name: wp.pane_name.clone(),
                })
                .await;
        }
    }

    // Placeholder health checker — health monitor needs a PaneHealthChecker impl.
    // For now, spawn a task that runs the health monitor with a no-op checker.
    struct NoOpChecker;
    #[async_trait::async_trait]
    impl kasmos::PaneHealthChecker for NoOpChecker {
        async fn list_live_panes(&self) -> kasmos::Result<Vec<String>> {
            // TODO: Wire to real Zellij pane listing when available
            Ok(Vec::new())
        }
    }

    let health_shutdown_rx = shutdown_rx.clone();
    let _health_handle = tokio::spawn(async move {
        health_monitor
            .run(NoOpChecker, health_shutdown_rx, health_reg_rx)
            .await;
    });

    // Bridge crash events → completion events (failure)
    let (merged_completion_tx, merged_completion_rx) =
        tokio::sync::mpsc::channel::<kasmos::CompletionEvent>(64);

    // Forward detector completions into merged channel
    let fwd_tx = merged_completion_tx.clone();
    let _detector_fwd = tokio::spawn(async move {
        let mut completion_rx = completion_rx;
        while let Some(event) = completion_rx.recv().await {
            if fwd_tx.send(event).await.is_err() {
                break;
            }
        }
    });

    // Forward crash events as failed completions
    let crash_fwd_tx = merged_completion_tx.clone();
    let _crash_fwd = tokio::spawn(async move {
        while let Some(crash) = crash_rx.recv().await {
            let event = kasmos::CompletionEvent::new(
                crash.wp_id,
                kasmos::CompletionMethod::Manual,
                false,
            );
            if crash_fwd_tx.send(event).await.is_err() {
                break;
            }
        }
    });
    drop(merged_completion_tx); // Only forwarding tasks hold senders

    // Step 15: Start command reader (FIFO)
    let (cmd_tx, mut cmd_rx) = tokio::sync::mpsc::channel::<kasmos::ControllerCommand>(32);
    let cmd_reader = kasmos::CommandReader::new(&kasmos_dir, cmd_tx)
        .context("Failed to create command reader")?;
    let _cmd_handle = cmd_reader.start().await.context("Failed to start command reader")?;
    tracing::info!("Command FIFO reader started");

    // Step 16: Wire command handler and engine action channel
    let (action_tx, action_rx) = tokio::sync::mpsc::channel::<kasmos::EngineAction>(32);

    // Spawn command dispatch loop
    let cmd_run = run.clone();
    let cmd_action_tx = action_tx.clone();
    let _cmd_dispatch = tokio::spawn(async move {
        // Minimal SessionController for command handler
        struct NoOpSessionCtrl;
        #[async_trait::async_trait]
        impl kasmos::SessionController for NoOpSessionCtrl {
            async fn focus_pane(&self, _wp_id: &str) -> kasmos::Result<()> {
                Ok(())
            }
            async fn focus_and_zoom(&self, _wp_id: &str) -> kasmos::Result<()> {
                Ok(())
            }
        }

        let handler = kasmos::CommandHandler::new(
            cmd_run,
            std::sync::Arc::new(NoOpSessionCtrl),
            cmd_action_tx,
        );

        while let Some(cmd) = cmd_rx.recv().await {
            match handler.handle(cmd).await {
                Ok(msg) => tracing::info!(response = %msg, "Command handled"),
                Err(e) => tracing::warn!(error = %e, "Command handler error"),
            }
        }
    });
    drop(action_tx); // Only command dispatch holds a sender

    // Step 17: Transition to Running and launch the wave engine
    {
        let mut r = run.write().await;
        r.state = kasmos::RunState::Running;
    }
    persister.save(&*run.read().await)?;

    tracing::info!("Starting wave engine");
    let mut engine = kasmos::WaveEngine::new(run.clone(), merged_completion_rx, action_rx);
    let engine_result = engine.run().await;

    // ── Phase 3: Cleanup (always runs) ─────────────────────────────

    // Mark completion time
    {
        let mut r = run.write().await;
        r.completed_at = Some(SystemTime::now());
    }

    // Persist final state
    let final_run = run.read().await;
    persister.save(&final_run).context("Failed to persist final state")?;

    // Generate report
    super::report::generate_report(&kasmos_dir, &final_run)
        .context("Failed to generate report")?;

    // Stop detector
    detector.stop();

    // Trigger shutdown to stop health monitor and other watchers
    shutdown_coordinator.trigger();

    // Cleanup transient artifacts
    kasmos::cleanup_artifacts(&kasmos_dir);

    match &engine_result {
        Ok(()) => tracing::info!(state = ?final_run.state, "Orchestration complete"),
        Err(e) => tracing::error!(error = %e, "Orchestration ended with error"),
    }

    drop(final_run);
    engine_result.map_err(|e| anyhow::anyhow!(e))?;

    tracing::info!("Orchestration complete");
    Ok(())
}
