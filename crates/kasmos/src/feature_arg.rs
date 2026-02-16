use anyhow::{Context, Result, bail};
use std::path::{Path, PathBuf};

pub fn feature_dir_from_specs_root(specs_root: &Path, feature_slug: &str) -> PathBuf {
    if specs_root
        .file_name()
        .and_then(|name| name.to_str())
        .is_some_and(|name| name == feature_slug)
    {
        specs_root.to_path_buf()
    } else {
        specs_root.join(feature_slug)
    }
}

pub fn resolve_existing_feature_dir(specs_root: &Path, feature_slug: &str) -> Result<PathBuf> {
    let as_path = PathBuf::from(feature_slug);
    let candidate = if as_path.is_dir() {
        as_path
    } else {
        feature_dir_from_specs_root(specs_root, feature_slug)
    };

    canonicalize_existing_feature_dir(&candidate)
}

fn canonicalize_existing_feature_dir(candidate: &Path) -> Result<PathBuf> {
    if candidate.exists() && !candidate.is_dir() {
        bail!(
            "Feature path exists but is not a directory: {}",
            candidate.display()
        );
    }

    if !candidate.is_dir() {
        bail!("Feature directory not found: {}", candidate.display());
    }

    candidate.canonicalize().with_context(|| {
        format!(
            "Failed to canonicalize feature path: {}",
            candidate.display()
        )
    })
}

pub fn resolve_feature_dir(feature: &str) -> Result<PathBuf> {
    let feature_path = PathBuf::from(feature);
    let specs_root = PathBuf::from("kitty-specs");

    if feature_path.is_dir() {
        return resolve_existing_feature_dir(&specs_root, feature);
    }

    if feature_path.exists() {
        bail!(
            "Feature path exists but is not a directory: {}",
            feature_path.display()
        );
    }

    // Exact match: full slug provided (e.g. "010-hub-tui-navigator")
    let exact_path = feature_dir_from_specs_root(&specs_root, feature);
    if exact_path.is_dir() {
        return resolve_existing_feature_dir(&specs_root, feature);
    }

    // Prefix match: numeric prefix provided (e.g. "010")
    let entries = std::fs::read_dir(&specs_root)
        .with_context(|| format!("Failed to read {}", specs_root.display()))?;

    let prefix = format!("{}-", feature);
    let mut matches: Vec<String> = Vec::new();

    for entry in entries {
        let entry = entry.with_context(|| {
            format!(
                "Failed to read an entry while scanning {}",
                specs_root.display()
            )
        })?;

        if !entry.path().is_dir() {
            continue;
        }

        let name = entry.file_name();
        let Some(name_str) = name.to_str() else {
            continue;
        };

        if name_str.starts_with(&prefix) {
            matches.push(name_str.to_string());
        }
    }

    matches.sort();

    match matches.len() {
        1 => {
            let resolved = specs_root.join(&matches[0]);
            resolved.canonicalize().with_context(|| {
                format!(
                    "Failed to canonicalize resolved feature directory for '{}': {}",
                    feature,
                    resolved.display()
                )
            })
        }
        0 => bail!(
            "Could not resolve feature '{}': no prefix match found under {}",
            feature,
            specs_root.display()
        ),
        _ => bail!(
            "Ambiguous feature '{}': multiple prefix matches under {}: {}",
            feature,
            specs_root.display(),
            matches.join(", ")
        ),
    }
}
