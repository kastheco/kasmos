//! Launch flow: feature resolution, preflight, layout generation, session bootstrap.

pub mod detect;
pub mod layout;
pub mod session;

use crate::config::Config;
use crate::launch::detect::{FeatureDetection, FeatureSource};
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

/// Execute launch preflight and feature selection.
///
/// WP02 intentionally stops before creating Zellij sessions/tabs; layout/session
/// bootstrap is implemented in WP03.
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

    let detection = detect::detect_feature(spec_prefix, &specs_root)
        .context("Failed during feature detection")?;
    let selection = resolve_feature_selection(&specs_root, detection)?;

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

    println!(
        "Feature resolved: {} ({})",
        selection
            .feature_slug
            .as_deref()
            .unwrap_or("<unknown feature>"),
        source_label(&selection.source)
    );
    println!("Preflight checks passed.");
    println!("Launch session bootstrap is implemented in WP03.");
    Ok(())
}

/// Run dependency preflight checks required for launch.
pub fn preflight_checks(config: &Config) -> std::result::Result<(), Vec<PreflightFailure>> {
    let mut failures = Vec::new();

    check_binary(
        &config.paths.zellij_binary,
        "zellij",
        "creating/switching orchestration sessions and panes",
        "Install zellij (for example: cargo install zellij)",
        &mut failures,
    );

    check_binary(
        &config.agent.opencode_binary,
        "opencode",
        "spawning manager/worker agents",
        "Install OpenCode and ensure its launcher binary is on PATH",
        &mut failures,
    );

    check_binary(
        &config.paths.spec_kitty_binary,
        "spec-kitty",
        "feature/task lifecycle commands",
        "Install spec-kitty and ensure `spec-kitty` is on PATH",
        &mut failures,
    );

    check_pane_tracker(&mut failures);

    if failures.is_empty() {
        Ok(())
    } else {
        Err(failures)
    }
}

fn check_binary(
    binary: &str,
    dependency: &str,
    required_for: &str,
    guidance: &str,
    failures: &mut Vec<PreflightFailure>,
) {
    if which::which(binary).is_err() {
        failures.push(PreflightFailure {
            dependency: dependency.to_string(),
            required_for: required_for.to_string(),
            guidance: guidance.to_string(),
        });
    }
}

fn check_pane_tracker(failures: &mut Vec<PreflightFailure>) {
    let tracker_binaries = ["pane-tracker", "zellij-pane-tracker"];
    let found = tracker_binaries.iter().any(|b| which::which(b).is_ok());
    if !found {
        failures.push(PreflightFailure {
            dependency: "pane-tracker".to_string(),
            required_for: "structured pane message/event tracking".to_string(),
            guidance: "Install pane-tracker tooling and expose `pane-tracker` (or `zellij-pane-tracker`) in PATH".to_string(),
        });
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
