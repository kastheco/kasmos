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

    // Steps 3-13 would wire: validation, scanning, graph, prompts, layout,
    // session, signals, detector, health, commands, engine.
    // These depend on modules from WP02-WP07 that aren't fully available yet.
    // Stubbing the orchestration loop structure:

    // Setup shutdown coordinator
    let (shutdown_coordinator, _shutdown_rx) = kasmos::ShutdownCoordinator::new(&kasmos_dir);

    // Setup signal handlers
    let shutdown_flag = shutdown_coordinator.flag();
    let _signal_handle = kasmos::setup_signal_handlers(Box::new(move || {
        shutdown_flag.store(true, std::sync::atomic::Ordering::SeqCst);
    }));

    // Create initial orchestration run
    let run = kasmos::OrchestrationRun {
        id: format!("run-{}", chrono::Utc::now().format("%Y%m%d-%H%M%S")),
        feature: feature.to_string(),
        feature_dir: feature_dir.clone(),
        config: config.clone(),
        work_packages: vec![], // Would be populated by WP02 scanner
        waves: vec![],         // Would be populated by WP02 graph
        state: kasmos::RunState::Initializing,
        started_at: Some(SystemTime::now()),
        completed_at: None,
        mode: progression_mode,
    };

    // Persist initial state
    let state_path = kasmos_dir.join("state.json");
    let state_json =
        serde_json::to_string_pretty(&run).context("Failed to serialize initial state")?;
    std::fs::write(&state_path, &state_json).context("Failed to write initial state")?;

    tracing::info!(run_id = %run.id, "Orchestration initialized");

    // TODO: Steps 3-13 — wire scanner, graph, prompts, layout, session,
    // detector, health, commands, engine when WP02-WP07 modules are integrated.
    // For now, the launch infrastructure is in place.

    // Step 14: On exit — generate report, persist final state, cleanup
    super::report::generate_report(&kasmos_dir, &run).context("Failed to generate report")?;

    kasmos::cleanup_artifacts(&kasmos_dir);

    tracing::info!("Orchestration complete");
    Ok(())
}
