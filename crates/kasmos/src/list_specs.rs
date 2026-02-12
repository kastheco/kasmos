//! List unfinished feature specs from kitty-specs/.

use anyhow::{Context, Result};
use std::path::Path;

#[derive(serde::Deserialize)]
struct WpFrontmatter {
    lane: Option<String>,
}

struct FeatureStatus {
    id: String,
    slug: String,
    done_count: usize,
    total_count: usize,
    has_spec_content: bool,
}

impl FeatureStatus {
    fn is_finished(&self) -> bool {
        self.has_spec_content && self.total_count > 0 && self.done_count == self.total_count
    }

    fn status_label(&self) -> &str {
        if !self.has_spec_content {
            "[empty spec]"
        } else if self.total_count == 0 {
            "[no tasks]"
        } else {
            // Caller handles the dynamic case
            unreachable!()
        }
    }
}

pub fn run() -> Result<()> {
    let specs_root = Path::new("kitty-specs");
    if !specs_root.is_dir() {
        println!("No kitty-specs/ directory found.");
        return Ok(());
    }

    let mut entries: Vec<_> = std::fs::read_dir(specs_root)
        .context("Failed to read kitty-specs/")?
        .filter_map(|e| e.ok())
        .filter(|e| e.path().is_dir())
        .collect();
    entries.sort_by_key(|e| e.file_name());

    let mut unfinished: Vec<FeatureStatus> = Vec::new();

    for entry in &entries {
        let name = entry.file_name();
        let Some(name_str) = name.to_str() else {
            continue;
        };
        let Some((id, slug)) = name_str.split_once('-') else {
            continue;
        };

        let dir = entry.path();
        let spec_path = dir.join("spec.md");
        let has_spec_content =
            spec_path.is_file() && std::fs::metadata(&spec_path).map_or(false, |m| m.len() > 0);

        let tasks_dir = dir.join("tasks");
        let (done_count, total_count) = if tasks_dir.is_dir() {
            scan_wp_lanes(&tasks_dir)
        } else {
            (0, 0)
        };

        let status = FeatureStatus {
            id: id.to_string(),
            slug: slug.to_string(),
            done_count,
            total_count,
            has_spec_content,
        };

        if !status.is_finished() {
            unfinished.push(status);
        }
    }

    if unfinished.is_empty() {
        println!("All features complete.");
    } else {
        println!("Unfinished features:\n");
        let max_slug = unfinished.iter().map(|f| f.slug.len()).max().unwrap_or(0);
        for f in &unfinished {
            let label = if !f.has_spec_content || f.total_count == 0 {
                f.status_label().to_string()
            } else {
                format!("[{}/{} done]", f.done_count, f.total_count)
            };
            println!(
                "  {}  {:<width$}  {}",
                f.id,
                f.slug,
                label,
                width = max_slug
            );
        }
    }

    Ok(())
}

fn scan_wp_lanes(tasks_dir: &Path) -> (usize, usize) {
    let Ok(entries) = std::fs::read_dir(tasks_dir) else {
        return (0, 0);
    };

    let mut done = 0usize;
    let mut total = 0usize;

    for entry in entries.filter_map(|e| e.ok()) {
        let name = entry.file_name();
        let Some(name_str) = name.to_str() else {
            continue;
        };
        if !name_str.starts_with("WP") || !name_str.ends_with(".md") {
            continue;
        }

        total += 1;

        if let Some(lane) = extract_lane(&entry.path()) {
            if lane == "done" {
                done += 1;
            }
        }
    }

    (done, total)
}

fn extract_lane(path: &Path) -> Option<String> {
    let content = std::fs::read_to_string(path).ok()?;
    let body = content.strip_prefix("---")?;
    let end = body.find("\n---")?;
    let fm: WpFrontmatter = serde_yml::from_str(&body[..end]).ok()?;
    fm.lane
}
