//! MCP tool implementations for the kasmos server.

use anyhow::{Context, Result, bail};
use std::path::{Path, PathBuf};

pub mod despawn_worker;
pub mod infer_feature;
pub mod list_features;
pub mod list_workers;
pub mod read_messages;
pub mod spawn_worker;
pub mod transition_wp;
pub mod wait_for_event;
pub mod workflow_status;

fn resolve_feature_dir(specs_root: &Path, feature_slug: &str) -> Result<PathBuf> {
    let as_path = PathBuf::from(feature_slug);
    let candidate = if as_path.is_dir() {
        as_path
    } else {
        specs_root.join(feature_slug)
    };

    if candidate.exists() && !candidate.is_dir() {
        bail!(
            "Feature path exists but is not a directory: {}",
            candidate.display()
        );
    }

    if !candidate.is_dir() {
        bail!("Feature directory not found: {}", candidate.display());
    }

    candidate
        .canonicalize()
        .with_context(|| format!("Failed to canonicalize {}", candidate.display()))
}
