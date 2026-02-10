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
    serde_yaml::from_str(yaml_str).map_err(|e| SpecParserError::InvalidFrontmatter {
        file: path.display().to_string(),
        reason: e.to_string(),
    })
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
}
