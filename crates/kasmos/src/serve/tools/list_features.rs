use crate::config::Config;
use anyhow::{Context, Result};
use schemars::JsonSchema;
use serde::{Deserialize, Serialize};
use std::path::Path;

#[derive(Debug, Clone, Default, Serialize, Deserialize, JsonSchema)]
#[serde(deny_unknown_fields)]
pub struct ListFeaturesInput {}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct ListFeaturesOutput {
    pub ok: bool,
    pub features: Vec<FeatureInfo>,
}

#[derive(Debug, Clone, Serialize, Deserialize, JsonSchema)]
pub struct FeatureInfo {
    pub slug: String,
    pub has_spec: bool,
    pub has_plan: bool,
    pub has_tasks: bool,
}

pub async fn handle(config: &Config) -> Result<ListFeaturesOutput> {
    let specs_root = Path::new(&config.paths.specs_root);
    if !specs_root.is_dir() {
        return Ok(ListFeaturesOutput {
            ok: true,
            features: Vec::new(),
        });
    }

    let mut features = Vec::new();
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

        let Some(slug) = entry.file_name().to_str().map(ToOwned::to_owned) else {
            continue;
        };

        let has_spec = path.join("spec.md").is_file();
        let has_plan = path.join("plan.md").is_file();
        let tasks_dir = path.join("tasks");
        let has_tasks = tasks_dir.is_dir() && has_wp_files(&tasks_dir);

        features.push(FeatureInfo {
            slug,
            has_spec,
            has_plan,
            has_tasks,
        });
    }

    features.sort_by(|a, b| a.slug.cmp(&b.slug));

    Ok(ListFeaturesOutput { ok: true, features })
}

fn has_wp_files(tasks_dir: &Path) -> bool {
    let Ok(entries) = std::fs::read_dir(tasks_dir) else {
        return false;
    };

    entries.filter_map(|entry| entry.ok()).any(|entry| {
        entry
            .file_name()
            .to_str()
            .is_some_and(|name| name.starts_with("WP") && name.ends_with(".md"))
    })
}
