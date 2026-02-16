//! MCP tool implementations for the kasmos server.

use anyhow::Result;
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
    crate::feature_arg::resolve_existing_feature_dir(specs_root, feature_slug)
}
