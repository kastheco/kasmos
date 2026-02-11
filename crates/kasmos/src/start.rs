//! Start orchestration for a feature: setup, create Zellij session, attach.

use anyhow::{Context, Result, bail};
use kasmos::ZellijCli;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::SystemTime;
use tokio::sync::{RwLock, mpsc};

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

/// Main start entry point: setup session and attach.
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
    let mut work_packages: Vec<kasmos::WorkPackage> = frontmatters
        .iter()
        .map(|(fm, _wp_file)| {
            let wp_id = &fm.work_package_id;
            let worktree_path = feature_dir
                .parent()
                .map(|parent| {
                    parent
                        .join(".worktrees")
                        .join(format!("{}-{}", feature_name, wp_id))
                })
                .filter(|p| p.exists());

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
                worktree_path,
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

    // Generate KDL layout — in wave-gated mode, only include wave 0 WPs that need launching.
    // WPs already marked as Completed/Failed (from frontmatter) are excluded from the layout.
    let layout_gen = kasmos::LayoutGenerator::new(&config);
    let layout_wps: Vec<&kasmos::WorkPackage> = match progression_mode {
        kasmos::ProgressionMode::WaveGated => {
            work_packages
                .iter()
                .filter(|wp| wp.wave == 0 && wp.state == kasmos::WPState::Pending)
                .collect()
        }
        kasmos::ProgressionMode::Continuous => {
            work_packages
                .iter()
                .filter(|wp| wp.state == kasmos::WPState::Pending)
                .collect()
        }
    };
    tracing::info!(
        pane_count = layout_wps.len(),
        mode = %mode,
        "Generating layout"
    );
    let kdl_doc = if layout_wps.is_empty() {
        // All wave 0 WPs are already completed — generate controller-only layout
        tracing::info!("All wave 0 WPs completed — generating controller-only layout");
        layout_gen.generate_controller_only(&feature_dir)?
    } else {
        layout_gen
            .generate(&layout_wps, &feature_dir)
            .context("Failed to generate KDL layout")?
    };
    let layout_path = layout_gen
        .write_layout(&kdl_doc, &kasmos_dir)
        .context("Failed to write layout file")?;
    tracing::info!(path = %layout_path.display(), "KDL layout written");

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

    let persister = kasmos::StatePersister::new(&kasmos_dir);
    persister.ensure_dir()?;
    persister.save(&run).context("Failed to persist state")?;
    tracing::info!(run_id = %run_id, "Orchestration run initialized");

    // ── Phase 2: Create Zellij session ─────────────────────────────

    let session_name = format!("kasmos-{}", feature_name);

    // Check if session already exists (e.g. from a previous run)
    let zellij = &config.zellij_binary;
    let existing = tokio::process::Command::new(zellij)
        .args(["list-sessions"])
        .output()
        .await
        .ok()
        .and_then(|o| String::from_utf8(o.stdout).ok())
        .unwrap_or_default();

    if existing.contains(&session_name) {
        // Kill active session, then delete (handles both active and EXITED states).
        // EXITED sessions can't be killed but must be deleted, otherwise
        // `attach --create-background` resurrects them with the old/default layout.
        tracing::warn!(session = %session_name, "Removing existing session");
        let _ = tokio::process::Command::new(zellij)
            .args(["kill-session", &session_name])
            .output()
            .await;
        let _ = tokio::process::Command::new(zellij)
            .args(["delete-session", &session_name])
            .output()
            .await;
    }

    // Create background session with layout
    let create_status = tokio::process::Command::new(zellij)
        .args([
            "--layout",
            &layout_path.display().to_string(),
            "attach",
            "--create-background",
            &session_name,
        ])
        .output()
        .await
        .context("Failed to spawn zellij")?;

    if !create_status.status.success() {
        let stderr = String::from_utf8_lossy(&create_status.stderr);
        bail!("Failed to create Zellij session: {}", stderr);
    }
    tracing::info!(session = %session_name, "Zellij session created");

    // ── Phase 3: Start orchestration engine & command pipeline ───

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
    let session_controller = Arc::new(StubSessionController);
    let command_handler = kasmos::CommandHandler::new(
        run_arc.clone(),
        session_controller,
        engine_action_tx,
    );
    let command_handler = Arc::new(command_handler);

    let handler_clone = command_handler.clone();
    let _cmd_bridge_handle = tokio::spawn(async move {
        while let Some(cmd) = command_rx.recv().await {
            match handler_clone.handle(cmd).await {
                Ok(msg) => tracing::info!("{}", msg),
                Err(e) => tracing::error!("Command handling failed: {}", e),
            }
        }
        tracing::debug!("Command bridge task exiting");
    });

    // WaveEngine — drives wave progression, receives completions and actions
    let mut engine =
        kasmos::WaveEngine::new(run_arc.clone(), completion_rx, engine_action_rx, launch_tx);
    let _engine_handle = tokio::spawn(async move {
        if let Err(e) = engine.run().await {
            tracing::error!("Wave engine error: {}", e);
        }
        tracing::info!("Wave engine stopped");
    });

    tracing::info!("Orchestration engine started");

    // Keep completion_tx alive for future detector wiring.
    // When the completion detector is wired in, it will own this sender.
    let _completion_tx = completion_tx;

    // ── Wave launch event handler ────────────────────────────────
    //
    // When the engine launches a new wave, this task:
    // 1. Renames the current tab to "WAVE <previous>" (archiving it)
    // 2. Generates a layout with only the new wave's agent panes
    // 3. Creates a new tab with that layout named "WAVE <current>"
    //
    // For the initial wave (panes created by the session layout), this is a no-op.
    let wave_session_name = session_name.clone();
    let wave_zellij = config.zellij_binary.clone();
    let wave_kasmos_dir = kasmos_dir.clone();
    let wave_config = config.clone();
    let wave_run_arc = run_arc.clone();
    let wave_feature_dir = feature_dir.clone();
    let _wave_handler = tokio::spawn(async move {
        let cli = kasmos::RealZellijCli::new(wave_zellij);
        let layout_gen = kasmos::LayoutGenerator::new(&wave_config);

        while let Some(event) = launch_rx.recv().await {
            if event.is_initial_wave {
                tracing::info!(
                    wave = event.wave_index,
                    wps = ?event.wp_ids,
                    "Initial wave launched (panes created by session layout)"
                );
                continue;
            }

            tracing::info!(
                wave = event.wave_index,
                wps = ?event.wp_ids,
                "Wave launch event — creating new tab"
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

            // Generate wave tab layout (agent-only, no controller)
            let wp_refs: Vec<&kasmos::WorkPackage> = wp_data.iter().collect();
            let wave_layout = match layout_gen.generate_wave_tab(&wp_refs, &wave_feature_dir) {
                Ok(doc) => doc,
                Err(e) => {
                    tracing::error!("Failed to generate wave tab layout: {}", e);
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
                    tracing::error!("Failed to write wave layout: {}", e);
                    continue;
                }
            };

            // Create new tab with the wave layout
            let new_tab_name = format!("WAVE-{}", event.wave_index);
            if let Err(e) = cli
                .new_tab(
                    &wave_session_name,
                    Some(&new_tab_name),
                    Some(&wave_layout_path),
                )
                .await
            {
                tracing::error!("Failed to create new tab for wave {}: {}", event.wave_index, e);
            } else {
                tracing::info!(
                    wave = event.wave_index,
                    tab = %new_tab_name,
                    "New wave tab created with {} panes",
                    wp_data.len()
                );
            }
        }
        tracing::debug!("Wave launch handler exiting");
    });

    // ── Phase 4: Attach interactively ───────────────────────────

    println!("Attaching to session: {}", session_name);
    let attach_status = tokio::process::Command::new(zellij)
        .args(["attach", &session_name])
        .stdin(std::process::Stdio::inherit())
        .stdout(std::process::Stdio::inherit())
        .stderr(std::process::Stdio::inherit())
        .status()
        .await
        .context("Failed to attach to Zellij session")?;

    if !attach_status.success() {
        tracing::warn!("Zellij attach exited with: {}", attach_status);
    }

    tracing::info!("Detached from session");
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
