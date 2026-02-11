use anyhow::{Context, Result, bail};
use std::path::PathBuf;

pub fn resolve_feature_dir(feature: &str) -> Result<PathBuf> {
    let feature_path = PathBuf::from(feature);

    if feature_path.is_dir() {
        return feature_path
            .canonicalize()
            .with_context(|| format!("Failed to canonicalize feature path: {}", feature));
    }

    if feature_path.exists() {
        bail!(
            "Feature path exists but is not a directory: {}",
            feature_path.display()
        );
    }

    let specs_root = PathBuf::from("kitty-specs");
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
