//! Feature detection pipeline: arg -> branch -> directory -> none.

use anyhow::{Context, Result, bail};
use std::path::{Path, PathBuf};

/// The source that resolved the feature selection.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FeatureSource {
    /// Explicit CLI argument (`kasmos <prefix>`).
    Arg(String),
    /// Current git branch name.
    Branch(String),
    /// Current working directory under `kitty-specs/<feature>/`.
    Directory(String),
    /// No resolvable source.
    None,
}

/// Feature detection result.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct FeatureDetection {
    /// Which source resolved the feature.
    pub source: FeatureSource,
    /// Full feature slug directory (e.g. `011-mcp-agent-swarm-orchestration`).
    pub feature_slug: Option<String>,
    /// Absolute path to the feature directory.
    pub feature_dir: Option<PathBuf>,
}

/// Detect feature using priority: arg -> branch -> directory -> none.
pub fn detect_feature(spec_prefix: Option<&str>, specs_root: &Path) -> Result<FeatureDetection> {
    detect_feature_with(
        spec_prefix,
        specs_root,
        || current_branch_name(),
        || std::env::current_dir().context("Failed to get current directory"),
    )
}

fn detect_feature_with<FBranch, FCwd>(
    spec_prefix: Option<&str>,
    specs_root: &Path,
    get_branch: FBranch,
    get_cwd: FCwd,
) -> Result<FeatureDetection>
where
    FBranch: Fn() -> Result<Option<String>>,
    FCwd: Fn() -> Result<PathBuf>,
{
    if let Some(prefix) = spec_prefix {
        let resolved = crate::feature_arg::resolve_feature_dir(prefix)
            .with_context(|| format!("Failed to resolve feature from arg '{}'", prefix))?;
        let slug = feature_slug_from_dir(&resolved).ok_or_else(|| {
            anyhow::anyhow!(
                "Resolved feature path has no valid directory name: {}",
                resolved.display()
            )
        })?;
        return Ok(FeatureDetection {
            source: FeatureSource::Arg(prefix.to_string()),
            feature_slug: Some(slug),
            feature_dir: Some(resolved),
        });
    }

    if let Some(branch) = get_branch()?
        && let Some(prefix) = branch_feature_prefix(&branch)
        && let Some(dir) = resolve_by_prefix(specs_root, &prefix)?
    {
        let slug = feature_slug_from_dir(&dir).ok_or_else(|| {
            anyhow::anyhow!(
                "Resolved feature path has no valid directory name: {}",
                dir.display()
            )
        })?;
        return Ok(FeatureDetection {
            source: FeatureSource::Branch(branch),
            feature_slug: Some(slug),
            feature_dir: Some(dir),
        });
    }

    let cwd = get_cwd()?;
    if let Some(dir) = resolve_from_directory(specs_root, &cwd) {
        let slug = feature_slug_from_dir(&dir).ok_or_else(|| {
            anyhow::anyhow!(
                "Resolved feature path has no valid directory name: {}",
                dir.display()
            )
        })?;
        return Ok(FeatureDetection {
            source: FeatureSource::Directory(cwd.display().to_string()),
            feature_slug: Some(slug),
            feature_dir: Some(dir),
        });
    }

    Ok(FeatureDetection {
        source: FeatureSource::None,
        feature_slug: None,
        feature_dir: None,
    })
}

fn current_branch_name() -> Result<Option<String>> {
    let output = std::process::Command::new("git")
        .args(["branch", "--show-current"])
        .output()
        .context("Failed to execute git branch --show-current")?;

    if !output.status.success() {
        return Ok(None);
    }

    let branch = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if branch.is_empty() {
        Ok(None)
    } else {
        Ok(Some(branch))
    }
}

fn branch_feature_prefix(branch: &str) -> Option<String> {
    branch.split_once('-').and_then(|(prefix, _)| {
        if prefix.len() == 3 && prefix.chars().all(|c| c.is_ascii_digit()) {
            Some(prefix.to_string())
        } else {
            None
        }
    })
}

fn resolve_by_prefix(specs_root: &Path, prefix: &str) -> Result<Option<PathBuf>> {
    if !specs_root.is_dir() {
        return Ok(None);
    }

    let mut matches = Vec::new();
    let want = format!("{}-", prefix);

    for entry in std::fs::read_dir(specs_root)
        .with_context(|| format!("Failed to read {}", specs_root.display()))?
    {
        let entry = entry.with_context(|| {
            format!(
                "Failed to read an entry while scanning {}",
                specs_root.display()
            )
        })?;

        if !entry.path().is_dir() {
            continue;
        }

        let Some(name) = entry.file_name().to_str().map(ToOwned::to_owned) else {
            continue;
        };

        if name.starts_with(&want) {
            matches.push(entry.path());
        }
    }

    matches.sort();

    match matches.len() {
        0 => Ok(None),
        1 => Ok(Some(matches.remove(0).canonicalize().with_context(
            || format!("Failed to canonicalize path for prefix {}", prefix),
        )?)),
        _ => {
            let names = matches
                .iter()
                .filter_map(|p| p.file_name().and_then(|n| n.to_str()))
                .collect::<Vec<_>>()
                .join(", ");
            bail!(
                "Ambiguous feature prefix '{}': multiple matches under {}: {}",
                prefix,
                specs_root.display(),
                names
            )
        }
    }
}

fn resolve_from_directory(specs_root: &Path, cwd: &Path) -> Option<PathBuf> {
    let specs_root = specs_root.canonicalize().ok()?;
    let cwd = cwd.canonicalize().ok()?;

    for ancestor in cwd.ancestors() {
        let parent = ancestor.parent()?;
        if parent == specs_root {
            return Some(ancestor.to_path_buf());
        }
    }

    None
}

fn feature_slug_from_dir(path: &Path) -> Option<String> {
    path.file_name()?.to_str().map(ToOwned::to_owned)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_specs_root() -> (tempfile::TempDir, PathBuf) {
        let tmp = tempfile::tempdir().expect("create tempdir");
        let specs_root = tmp.path().join("kitty-specs");
        std::fs::create_dir_all(&specs_root).expect("create specs root");
        (tmp, specs_root)
    }

    #[test]
    fn arg_detection_resolves_correctly() {
        let (tmp, specs_root) = make_specs_root();
        std::fs::create_dir_all(specs_root.join("011-test-feature")).expect("create feature");

        let old_cwd = std::env::current_dir().expect("cwd");
        std::env::set_current_dir(tmp.path()).expect("set cwd");

        let detection = detect_feature_with(
            Some("011"),
            &specs_root,
            || Ok(None),
            || Ok(tmp.path().to_path_buf()),
        )
        .expect("detect from arg");

        std::env::set_current_dir(old_cwd).expect("restore cwd");

        assert!(matches!(detection.source, FeatureSource::Arg(ref s) if s == "011"));
        assert_eq!(detection.feature_slug.as_deref(), Some("011-test-feature"));
        assert!(detection.feature_dir.is_some());
    }

    #[test]
    fn branch_detection_parses_prefix() {
        let (_tmp, specs_root) = make_specs_root();
        std::fs::create_dir_all(specs_root.join("011-test-feature")).expect("create feature");

        let detection = detect_feature_with(
            None,
            &specs_root,
            || Ok(Some("011-something".to_string())),
            || Ok(PathBuf::from("/tmp")),
        )
        .expect("detect from branch");

        assert!(matches!(
            detection.source,
            FeatureSource::Branch(ref b) if b == "011-something"
        ));
        assert_eq!(detection.feature_slug.as_deref(), Some("011-test-feature"));
    }

    #[test]
    fn none_detection_when_no_sources_available() {
        let (_tmp, specs_root) = make_specs_root();

        let detection = detect_feature_with(
            None,
            &specs_root,
            || Ok(Some("main".to_string())),
            || Ok(PathBuf::from("/tmp")),
        )
        .expect("detect none");

        assert_eq!(detection.source, FeatureSource::None);
        assert!(detection.feature_slug.is_none());
        assert!(detection.feature_dir.is_none());
    }

    #[test]
    fn ambiguous_prefix_returns_error() {
        let (_tmp, specs_root) = make_specs_root();
        std::fs::create_dir_all(specs_root.join("011-one")).expect("create feature");
        std::fs::create_dir_all(specs_root.join("011-two")).expect("create feature");

        let err = detect_feature_with(
            None,
            &specs_root,
            || Ok(Some("011-branch".to_string())),
            || Ok(PathBuf::from("/tmp")),
        )
        .expect_err("expected ambiguity error");

        assert!(err.to_string().contains("Ambiguous feature prefix"));
    }
}
