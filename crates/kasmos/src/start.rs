//! Start orchestration for a feature: setup, create Zellij session, attach.

use anyhow::{Context, Result, bail};
use std::path::{Path, PathBuf};
use std::time::SystemTime;

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

    // Build WorkPackages
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

            kasmos::WorkPackage {
                id: wp_id.clone(),
                title: fm.title.clone(),
                state: kasmos::WPState::Pending,
                dependencies: fm.dependencies.clone(),
                wave: 0,
                pane_id: None,
                pane_name: format!("{}-pane", wp_id.to_lowercase()),
                worktree_path,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
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

    // Generate KDL layout — in wave-gated mode, only include wave 0 WPs
    let layout_gen = kasmos::LayoutGenerator::new(&config);
    let layout_wps: Vec<&kasmos::WorkPackage> = match progression_mode {
        kasmos::ProgressionMode::WaveGated => {
            work_packages.iter().filter(|wp| wp.wave == 0).collect()
        }
        kasmos::ProgressionMode::Continuous => work_packages.iter().collect(),
    };
    tracing::info!(
        pane_count = layout_wps.len(),
        mode = %mode,
        "Generating layout"
    );
    let kdl_doc = layout_gen
        .generate(&layout_wps, &feature_dir)
        .context("Failed to generate KDL layout")?;
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

    // ── Phase 2: Create session and attach ──────────────────────────

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

    // Attach interactively — hands control to the user
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
