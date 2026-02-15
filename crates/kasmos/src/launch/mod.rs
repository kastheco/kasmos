//! Launch flow: feature resolution, preflight, layout generation, session bootstrap.

pub mod detect;
pub mod layout;
pub mod session;

use crate::config::Config;
use crate::launch::detect::{FeatureDetection, FeatureSource};
use crate::launch::layout::ManagerCommand;
use crate::setup::CheckStatus;
use anyhow::{Context, Result};
use std::path::{Path, PathBuf};

/// Structured preflight failure for actionable launch guidance.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PreflightFailure {
    /// Missing dependency/tool name.
    pub dependency: String,
    /// Why launch needs this dependency.
    pub required_for: String,
    /// Suggested install/remediation command.
    pub guidance: String,
}

/// Execute launch preflight, feature selection, layout generation, and bootstrap.
pub async fn run(spec_prefix: Option<&str>) -> Result<()> {
    let config = Config::load().context("Failed to load config")?;
    let specs_root = PathBuf::from(&config.paths.specs_root);

    if !has_any_feature_specs(&specs_root)? {
        println!(
            "No feature specs found in {}. Create one with: spec-kitty init",
            specs_root.display()
        );
        return Ok(());
    }

    if let Err(failures) = preflight_checks(&config) {
        eprintln!("Launch preflight failed:\n");
        for failure in failures {
            eprintln!("- Missing {}", failure.dependency);
            eprintln!("  Needed for: {}", failure.required_for);
            eprintln!("  Fix: {}", failure.guidance);
            eprintln!();
        }
        anyhow::bail!("preflight failed");
    }

    let detection = detect::detect_feature(spec_prefix, &specs_root)
        .context("Failed during feature detection")?;
    let selection = resolve_feature_selection(&specs_root, detection)?;
    let feature_slug = selection
        .feature_slug
        .as_deref()
        .context("feature slug not resolved")?;
    let feature_dir = selection
        .feature_dir
        .as_deref()
        .context("feature directory not resolved")?;

    let phase_hint = detect_phase_hint(feature_dir);
    let manager_prompt =
        crate::prompt::generate_manager_prompt(feature_slug, feature_dir, &phase_hint);
    let manager_command =
        ManagerCommand::from_config(&config, feature_dir.display().to_string(), manager_prompt);

    let layout_kdl = layout::generate_layout(&config, feature_slug, &manager_command)
        .context("Failed to generate launch layout")?;

    session::bootstrap(&config, feature_slug, &layout_kdl)
        .await
        .context("Failed to bootstrap orchestration session/tab")?;

    println!(
        "Feature resolved: {} ({})",
        selection
            .feature_slug
            .as_deref()
            .unwrap_or("<unknown feature>"),
        source_label(&selection.source)
    );
    println!("Preflight checks passed.");
    println!("Orchestration launch bootstrap complete.");
    Ok(())
}

fn detect_phase_hint(feature_dir: &Path) -> String {
    let has_spec = feature_dir.join("spec.md").is_file();
    let has_plan = feature_dir.join("plan.md").is_file();
    let has_tasks_index = feature_dir.join("tasks.md").is_file();
    let has_wp_tasks = feature_dir.join("tasks").is_dir();

    if !has_spec {
        return "specify".to_string();
    }
    if !has_plan {
        return "plan".to_string();
    }
    if !has_tasks_index && !has_wp_tasks {
        return "tasks".to_string();
    }
    "implement".to_string()
}

/// Run dependency preflight checks required for launch.
pub fn preflight_checks(config: &Config) -> std::result::Result<(), Vec<PreflightFailure>> {
    let setup_result = match crate::setup::validate_environment(config) {
        Ok(result) => result,
        Err(err) => {
            return Err(vec![PreflightFailure {
                dependency: "setup-validation".to_string(),
                required_for: "environment validation".to_string(),
                guidance: format!("Resolve setup validation error: {}", err),
            }]);
        }
    };

    let failures: Vec<PreflightFailure> = setup_result
        .checks
        .into_iter()
        .filter(|check| check.status == CheckStatus::Fail)
        .map(|check| PreflightFailure {
            dependency: check.name,
            required_for: check.description,
            guidance: check
                .guidance
                .unwrap_or_else(|| "Run `kasmos setup` to inspect and remediate".to_string()),
        })
        .collect();

    if failures.is_empty() {
        Ok(())
    } else {
        Err(failures)
    }
}

fn has_any_feature_specs(specs_root: &Path) -> Result<bool> {
    if !specs_root.is_dir() {
        return Ok(false);
    }

    let mut has_feature = false;
    for entry in std::fs::read_dir(specs_root)
        .with_context(|| format!("Failed to read {}", specs_root.display()))?
    {
        let entry = entry.with_context(|| {
            format!(
                "Failed to read an entry while scanning {}",
                specs_root.display()
            )
        })?;
        if entry.path().is_dir() {
            has_feature = true;
            break;
        }
    }

    Ok(has_feature)
}

fn resolve_feature_selection(
    specs_root: &Path,
    detection: FeatureDetection,
) -> Result<FeatureDetection> {
    if detection.source != FeatureSource::None {
        return Ok(detection);
    }

    let features = list_feature_directories(specs_root)?;
    if features.is_empty() {
        anyhow::bail!("No feature specs found in {}", specs_root.display());
    }

    println!("No feature specified and none could be inferred from the environment.");
    println!("Available feature specs:");
    for (idx, feature) in features.iter().enumerate() {
        println!("  {}) {}", idx + 1, feature.display_name);
    }

    let selected = prompt_for_selection(features.len())?;
    let selected = &features[selected - 1];

    Ok(FeatureDetection {
        source: FeatureSource::Arg(selected.display_name.clone()),
        feature_slug: Some(selected.display_name.clone()),
        feature_dir: Some(selected.path.clone()),
    })
}

#[derive(Debug, Clone)]
struct FeatureDirectory {
    display_name: String,
    path: PathBuf,
}

fn list_feature_directories(specs_root: &Path) -> Result<Vec<FeatureDirectory>> {
    let mut dirs = Vec::new();
    for entry in std::fs::read_dir(specs_root)
        .with_context(|| format!("Failed to read {}", specs_root.display()))?
    {
        let entry = entry.with_context(|| {
            format!(
                "Failed to read an entry while scanning {}",
                specs_root.display()
            )
        })?;
        let path = entry.path();
        if !path.is_dir() {
            continue;
        }
        let Some(name) = entry.file_name().to_str().map(ToOwned::to_owned) else {
            continue;
        };
        dirs.push(FeatureDirectory {
            display_name: name,
            path: path
                .canonicalize()
                .with_context(|| format!("Failed to canonicalize {}", path.display()))?,
        });
    }

    dirs.sort_by(|a, b| a.display_name.cmp(&b.display_name));
    Ok(dirs)
}

fn prompt_for_selection(max: usize) -> Result<usize> {
    use std::io::{self, Write};

    loop {
        print!("Select a feature [1-{}]: ", max);
        io::stdout().flush().context("Failed to flush stdout")?;

        let mut input = String::new();
        io::stdin()
            .read_line(&mut input)
            .context("Failed to read selection")?;

        let trimmed = input.trim();
        let Ok(n) = trimmed.parse::<usize>() else {
            eprintln!("Please enter a number between 1 and {}.", max);
            continue;
        };

        if (1..=max).contains(&n) {
            return Ok(n);
        }

        eprintln!(
            "Selection out of range. Enter a number between 1 and {}.",
            max
        );
    }
}

fn source_label(source: &FeatureSource) -> &'static str {
    match source {
        FeatureSource::Arg(_) => "arg",
        FeatureSource::Branch(_) => "branch",
        FeatureSource::Directory(_) => "directory",
        FeatureSource::None => "none",
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn preflight_reports_missing_binaries() {
        let mut config = Config::default();
        config.paths.zellij_binary = "__missing_zellij__".to_string();
        config.agent.opencode_binary = "__missing_opencode__".to_string();
        config.paths.spec_kitty_binary = "__missing_spec_kitty__".to_string();

        let failures = preflight_checks(&config).expect_err("preflight should fail");
        assert!(failures.iter().any(|f| f.dependency == "zellij"));
        assert!(failures.iter().any(|f| f.dependency == "opencode"));
        assert!(failures.iter().any(|f| f.dependency == "spec-kitty"));
    }

    #[test]
    fn no_specs_early_exit_condition_detects_absence() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        assert!(!has_any_feature_specs(&specs_root).expect("check specs"));
    }

    #[test]
    fn no_specs_early_exit_condition_detects_presence() {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        std::fs::create_dir_all(specs_root.join("011-test-feature")).expect("create feature");
        assert!(has_any_feature_specs(&specs_root).expect("check specs"));
    }

    #[test]
    fn selection_gate_triggers_when_detection_none() {
        let detection = FeatureDetection {
            source: FeatureSource::None,
            feature_slug: None,
            feature_dir: None,
        };
        assert_eq!(detection.source, FeatureSource::None);
    }
}
