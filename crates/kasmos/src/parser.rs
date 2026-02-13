use crate::error::SpecParserError;
use serde::{Deserialize, Serialize};
use std::path::{Path, PathBuf};

/// Work package frontmatter extracted from YAML.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WPFrontmatter {
    pub work_package_id: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub dependencies: Vec<String>,
    #[serde(default = "default_lane")]
    pub lane: String,
    #[serde(default)]
    pub subtasks: Vec<String>,
    #[serde(default)]
    pub phase: String,
}

fn default_lane() -> String {
    "planned".to_string()
}

/// Map a [`WPState`] to the spec-kitty lane string used in task file frontmatter.
pub fn wp_state_to_lane(state: crate::types::WPState) -> &'static str {
    use crate::types::WPState;
    match state {
        WPState::Completed => "done",
        WPState::ForReview => "for_review",
        WPState::Active => "doing",
        WPState::Pending | WPState::Paused | WPState::Failed => "planned",
    }
}

/// Feature directory structure containing work packages.
pub struct FeatureDir {
    pub path: PathBuf,
    pub spec_path: PathBuf,
    pub plan_path: PathBuf,
    pub tasks_dir: PathBuf,
    pub wp_files: Vec<PathBuf>,
}

impl FeatureDir {
    /// Scan the feature directory and discover all WP files.
    /// WP files match pattern: tasks/WPxx-*.md
    pub fn scan(feature_path: &Path) -> Result<Self, SpecParserError> {
        // Verify feature_path exists and is a directory
        if !feature_path.is_dir() {
            return Err(SpecParserError::FeatureDirNotFound {
                path: feature_path.display().to_string(),
            });
        }

        let spec_path = feature_path.join("spec.md");
        let plan_path = feature_path.join("plan.md");
        let tasks_dir = feature_path.join("tasks");

        // Check for tasks directory
        if !tasks_dir.is_dir() {
            return Err(SpecParserError::FeatureDirNotFound {
                path: tasks_dir.display().to_string(),
            });
        }

        // Scan tasks/ for WPxx-*.md files
        let mut wp_files = Vec::new();

        for entry in std::fs::read_dir(&tasks_dir)? {
            let entry = entry?;
            let path = entry.path();

            if path.is_file()
                && let Some(file_name) = path.file_name()
                && let Some(name_str) = file_name.to_str()
                && name_str.starts_with("WP")
                && name_str.ends_with(".md")
            {
                wp_files.push(path);
            }
        }

        // Sort by WP number for deterministic ordering
        wp_files.sort_by(|a, b| {
            let a_num = extract_wp_number(a);
            let b_num = extract_wp_number(b);
            a_num.cmp(&b_num)
        });

        Ok(FeatureDir {
            path: feature_path.to_path_buf(),
            spec_path,
            plan_path,
            tasks_dir,
            wp_files,
        })
    }
}

/// Extract WP number from filename for sorting (e.g., "WP02" → 2)
fn extract_wp_number(path: &Path) -> u32 {
    if let Some(file_name) = path.file_name()
        && let Some(name_str) = file_name.to_str()
        && let Some(num_part) = name_str.strip_prefix("WP")
        && let Some(dash_pos) = num_part.find('-')
        && let Ok(num) = num_part[..dash_pos].parse::<u32>()
    {
        return num;
    }
    u32::MAX // Sort unknown formats to the end
}

/// Parse YAML frontmatter from a markdown file.
/// Frontmatter is delimited by --- at start and end.
pub fn parse_frontmatter(path: &Path) -> Result<WPFrontmatter, SpecParserError> {
    let content = std::fs::read_to_string(path)?;

    // Split on "---" delimiters
    let parts: Vec<&str> = content.splitn(3, "---").collect();
    if parts.len() < 3 {
        return Err(SpecParserError::InvalidFrontmatter {
            file: path.display().to_string(),
            reason: "No YAML frontmatter found (missing --- delimiters)".into(),
        });
    }

    let yaml_str = parts[1].trim();
    serde_yml::from_str(yaml_str).map_err(|e| SpecParserError::InvalidFrontmatter {
        file: path.display().to_string(),
        reason: e.to_string(),
    })
}

/// Update the `lane` field in a WP task file's YAML frontmatter.
///
/// Finds the task file matching `wp_id` (e.g., "WP06") in `feature_dir/tasks/`,
/// then rewrites its YAML frontmatter with the new lane value while preserving
/// the rest of the file content.
///
/// Returns `Ok(())` if the lane was updated, or an error if the file wasn't found
/// or couldn't be parsed. Logs a warning and returns `Ok(())` if the tasks
/// directory doesn't exist (idempotent for tests).
pub fn update_task_file_lane(
    feature_dir: &Path,
    wp_id: &str,
    new_lane: &str,
) -> std::result::Result<(), SpecParserError> {
    let tasks_dir = feature_dir.join("tasks");
    if !tasks_dir.is_dir() {
        tracing::warn!(
            wp_id = %wp_id,
            dir = %tasks_dir.display(),
            "Tasks directory not found — skipping lane write-back"
        );
        return Ok(());
    }

    // Find the WP file: tasks/WP<nn>-*.md
    let wp_file = find_wp_task_file(&tasks_dir, wp_id)?;

    // Read content and replace the lane in frontmatter
    let content = std::fs::read_to_string(&wp_file)?;
    let updated = replace_frontmatter_lane(&content, new_lane).ok_or_else(|| {
        SpecParserError::InvalidFrontmatter {
            file: wp_file.display().to_string(),
            reason: "Could not locate lane field in frontmatter".into(),
        }
    })?;

    std::fs::write(&wp_file, updated)?;

    tracing::debug!(
        wp_id = %wp_id,
        lane = %new_lane,
        file = %wp_file.display(),
        "Task file lane updated"
    );
    Ok(())
}

/// Find the task file for a given WP ID in the tasks directory.
fn find_wp_task_file(tasks_dir: &Path, wp_id: &str) -> std::result::Result<PathBuf, SpecParserError> {
    for entry in std::fs::read_dir(tasks_dir)? {
        let entry = entry?;
        let path = entry.path();
        if path.is_file()
            && let Some(name) = path.file_name()
            && let Some(name_str) = name.to_str()
            && name_str.starts_with(wp_id)
            && name_str.ends_with(".md")
        {
            return Ok(path);
        }
    }
    Err(SpecParserError::InvalidFrontmatter {
        file: format!("{}/{}-*.md", tasks_dir.display(), wp_id),
        reason: format!("No task file found for {}", wp_id),
    })
}

/// Replace the `lane:` value in YAML frontmatter, preserving everything else.
///
/// Handles both `lane: value` and `lane: "value"` forms.
fn replace_frontmatter_lane(content: &str, new_lane: &str) -> Option<String> {
    // Split on --- delimiters: parts[0] is before first ---, parts[1] is YAML, parts[2] is body
    let parts: Vec<&str> = content.splitn(3, "---").collect();
    if parts.len() < 3 {
        return None;
    }

    let yaml = parts[1];

    // Replace lane field using line-by-line approach to preserve formatting
    let mut found = false;
    let updated_yaml: String = yaml
        .lines()
        .map(|line| {
            let trimmed = line.trim_start();
            if trimmed.starts_with("lane:") {
                found = true;
                // Preserve leading whitespace
                let indent = &line[..line.len() - trimmed.len()];
                format!("{}lane: {}", indent, new_lane)
            } else {
                line.to_string()
            }
        })
        .collect::<Vec<_>>()
        .join("\n");

    if !found {
        return None;
    }

    Some(format!("---{}---{}", updated_yaml, parts[2]))
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    #[test]
    fn test_parse_frontmatter_valid() {
        let yaml_content = r#"---
work_package_id: WP01
title: Core Types & Config
dependencies: []
lane: planned
---

# Work Package Content
"#;

        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP01-test.md");
        fs::write(&file_path, yaml_content).unwrap();

        let result = parse_frontmatter(&file_path).unwrap();
        assert_eq!(result.work_package_id, "WP01");
        assert_eq!(result.title, "Core Types & Config");
        assert_eq!(result.dependencies.len(), 0);
        assert_eq!(result.lane, "planned");
    }

    #[test]
    fn test_parse_frontmatter_with_dependencies() {
        let yaml_content = r#"---
work_package_id: WP02
title: Spec Parser
dependencies: [WP01]
---

# Content
"#;

        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP02-test.md");
        fs::write(&file_path, yaml_content).unwrap();

        let result = parse_frontmatter(&file_path).unwrap();
        assert_eq!(result.work_package_id, "WP02");
        assert_eq!(result.dependencies, vec!["WP01"]);
    }

    #[test]
    fn test_parse_frontmatter_missing_optional_fields() {
        let yaml_content = r#"---
work_package_id: WP03
---

# Content
"#;

        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("WP03-test.md");
        fs::write(&file_path, yaml_content).unwrap();

        let result = parse_frontmatter(&file_path).unwrap();
        assert_eq!(result.work_package_id, "WP03");
        assert_eq!(result.title, "");
        assert_eq!(result.dependencies.len(), 0);
        assert_eq!(result.lane, "planned");
    }

    #[test]
    fn test_parse_frontmatter_invalid_no_delimiters() {
        let yaml_content = "No frontmatter here";

        let temp_dir = TempDir::new().unwrap();
        let file_path = temp_dir.path().join("invalid.md");
        fs::write(&file_path, yaml_content).unwrap();

        let result = parse_frontmatter(&file_path);
        assert!(result.is_err());
    }

    #[test]
    fn test_feature_dir_scan() {
        let temp_dir = TempDir::new().unwrap();
        let tasks_dir = temp_dir.path().join("tasks");
        fs::create_dir(&tasks_dir).unwrap();

        // Create some WP files
        fs::write(
            tasks_dir.join("WP01-core.md"),
            "---\nwork_package_id: WP01\n---\n",
        )
        .unwrap();
        fs::write(
            tasks_dir.join("WP02-parser.md"),
            "---\nwork_package_id: WP02\n---\n",
        )
        .unwrap();
        fs::write(
            tasks_dir.join("WP03-graph.md"),
            "---\nwork_package_id: WP03\n---\n",
        )
        .unwrap();
        fs::write(tasks_dir.join("README.md"), "Not a WP file").unwrap();

        let feature_dir = FeatureDir::scan(temp_dir.path()).unwrap();
        assert_eq!(feature_dir.wp_files.len(), 3);

        // Verify sorting by WP number
        let names: Vec<_> = feature_dir
            .wp_files
            .iter()
            .filter_map(|p| p.file_name().and_then(|n| n.to_str()))
            .collect();
        assert_eq!(
            names,
            vec!["WP01-core.md", "WP02-parser.md", "WP03-graph.md"]
        );
    }

    #[test]
    fn test_feature_dir_not_found() {
        let result = FeatureDir::scan(Path::new("/nonexistent/path"));
        assert!(result.is_err());
    }

    #[test]
    fn test_replace_frontmatter_lane() {
        let content = "---\nwork_package_id: WP06\ntitle: Agent Pane Launch\nlane: for_review\n---\n\n# Body\n";
        let updated = replace_frontmatter_lane(content, "done").unwrap();
        assert!(updated.contains("lane: done"));
        assert!(!updated.contains("lane: for_review"));
        assert!(updated.contains("# Body"));
        assert!(updated.contains("work_package_id: WP06"));
    }

    #[test]
    fn test_replace_frontmatter_lane_no_lane_field() {
        let content = "---\nwork_package_id: WP06\ntitle: Test\n---\n\n# Body\n";
        assert!(replace_frontmatter_lane(content, "done").is_none());
    }

    #[test]
    fn test_replace_frontmatter_lane_no_frontmatter() {
        let content = "No frontmatter here";
        assert!(replace_frontmatter_lane(content, "done").is_none());
    }

    #[test]
    fn test_update_task_file_lane() {
        let temp_dir = TempDir::new().unwrap();
        let tasks_dir = temp_dir.path().join("tasks");
        fs::create_dir(&tasks_dir).unwrap();

        let content = "---\nwork_package_id: WP06\ntitle: Agent Pane Launch\nlane: for_review\n---\n\n# WP06\n";
        fs::write(tasks_dir.join("WP06-agent-pane-launch.md"), content).unwrap();

        update_task_file_lane(temp_dir.path(), "WP06", "done").unwrap();

        let updated = fs::read_to_string(tasks_dir.join("WP06-agent-pane-launch.md")).unwrap();
        assert!(updated.contains("lane: done"));
        assert!(!updated.contains("lane: for_review"));
        assert!(updated.contains("# WP06"));
    }

    #[test]
    fn test_update_task_file_lane_missing_tasks_dir() {
        let temp_dir = TempDir::new().unwrap();
        // No tasks/ directory — should return Ok (idempotent)
        let result = update_task_file_lane(temp_dir.path(), "WP01", "done");
        assert!(result.is_ok());
    }

    #[test]
    fn test_update_task_file_lane_file_not_found() {
        let temp_dir = TempDir::new().unwrap();
        let tasks_dir = temp_dir.path().join("tasks");
        fs::create_dir(&tasks_dir).unwrap();

        let result = update_task_file_lane(temp_dir.path(), "WP99", "done");
        assert!(result.is_err());
    }

    #[test]
    fn test_wp_state_to_lane() {
        use crate::types::WPState;
        assert_eq!(wp_state_to_lane(WPState::Completed), "done");
        assert_eq!(wp_state_to_lane(WPState::ForReview), "for_review");
        assert_eq!(wp_state_to_lane(WPState::Active), "doing");
        assert_eq!(wp_state_to_lane(WPState::Pending), "planned");
        assert_eq!(wp_state_to_lane(WPState::Paused), "planned");
        assert_eq!(wp_state_to_lane(WPState::Failed), "planned");
    }
}
