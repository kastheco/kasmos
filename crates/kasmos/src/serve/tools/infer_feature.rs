use crate::config::Config;
use crate::launch::detect::{FeatureSource, detect_feature};
use anyhow::{Result, bail};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct InferFeatureInput {
    pub spec_prefix: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct InferFeatureOutput {
    pub ok: bool,
    pub source: InferFeatureSource,
    pub feature_slug: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, JsonSchema)]
#[serde(rename_all = "lowercase")]
pub enum InferFeatureSource {
    Arg,
    Branch,
    Directory,
    None,
}

pub async fn handle(config: &Config, input: InferFeatureInput) -> Result<InferFeatureOutput> {
    if let Some(spec_prefix) = input.spec_prefix {
        let slug =
            resolve_feature_slug_by_prefix(Path::new(&config.paths.specs_root), &spec_prefix)?;
        return Ok(InferFeatureOutput {
            ok: true,
            source: InferFeatureSource::Arg,
            feature_slug: Some(slug),
        });
    }

    let detection = detect_feature(None, Path::new(&config.paths.specs_root))?;

    let source = match detection.source {
        FeatureSource::Arg(_) => InferFeatureSource::Arg,
        FeatureSource::Branch(_) => InferFeatureSource::Branch,
        FeatureSource::Directory(_) => InferFeatureSource::Directory,
        FeatureSource::None => InferFeatureSource::None,
    };

    Ok(InferFeatureOutput {
        ok: true,
        source,
        feature_slug: detection.feature_slug,
    })
}

fn resolve_feature_slug_by_prefix(specs_root: &Path, spec_prefix: &str) -> Result<String> {
    if !specs_root.is_dir() {
        bail!("Failed to read {}", specs_root.display());
    }

    let mut matches = Vec::new();
    let prefix = format!("{}-", spec_prefix);

    for entry in std::fs::read_dir(specs_root)? {
        let entry = entry?;
        if !entry.path().is_dir() {
            continue;
        }

        let Some(name) = entry.file_name().to_str().map(ToOwned::to_owned) else {
            continue;
        };

        if name == spec_prefix || name.starts_with(&prefix) {
            matches.push(name);
        }
    }

    matches.sort();
    match matches.len() {
        1 => Ok(matches.remove(0)),
        0 => bail!(
            "Could not resolve feature '{}': no prefix match found under {}",
            spec_prefix,
            specs_root.display()
        ),
        _ => bail!(
            "Ambiguous feature '{}': multiple prefix matches under {}: {}",
            spec_prefix,
            specs_root.display(),
            matches.join(", ")
        ),
    }
}
