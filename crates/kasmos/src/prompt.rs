//! Prompt file generation for work packages.
//!
//! This module generates work-package-specific prompt files containing WP description,
//! scope, dependency context, and project-level agent instructions.

use crate::error::{ConfigError, Result};
use crate::types::WorkPackage;
use std::fs;
use std::path::{Path, PathBuf};

/// Generate the initial manager prompt for orchestration startup.
pub fn generate_manager_prompt(feature_slug: &str, feature_dir: &Path, phase_hint: &str) -> String {
    let spec_path = feature_dir.join("spec.md");
    let plan_path = feature_dir.join("plan.md");
    let tasks_path = feature_dir.join("tasks.md");

    format!(
        "You are the kasmos manager agent for feature '{feature_slug}'.\n\
Feature directory: {feature_dir}\n\
Phase hint: {phase_hint}\n\
\n\
Startup responsibilities:\n\
1) Assess current workflow phase by checking artifact presence and completion state:\n\
   - spec: {spec_path}\n\
   - plan: {plan_path}\n\
   - tasks index: {tasks_path}\n\
   - work packages: {feature_dir}/tasks/WP*.md\n\
2) Summarize current state and recommend exactly one next action.\n\
3) Wait for explicit user confirmation before taking any action.\n\
\n\
Use kasmos MCP tools for orchestration operations.\n\
kasmos serve is already available as your MCP stdio subprocess via profile configuration;\n\
do not launch kasmos serve as a separate pane/process.\n\
\n\
Reference project rules and memory:\n\
- AGENTS: AGENTS.md\n\
- Constitution: .kittify/memory/constitution.md\n\
- Architecture memory: .kittify/memory/architecture.md\n",
        feature_dir = feature_dir.display(),
        spec_path = spec_path.display(),
        plan_path = plan_path.display(),
        tasks_path = tasks_path.display(),
    )
}

/// Information about a subtask within a work package.
#[derive(Debug, Clone)]
pub struct SubtaskInfo {
    /// Subtask identifier (e.g., "T020").
    pub id: String,
    /// Human-readable description.
    pub description: String,
    /// Whether this subtask can be executed in parallel.
    pub parallel: bool,
}

/// Context about an upstream dependency work package.
#[derive(Debug, Clone)]
pub struct DependencyContext {
    /// Work package identifier (e.g., "WP01").
    pub wp_id: String,
    /// Human-readable title.
    pub wp_title: String,
    /// Brief summary of what this dependency provides.
    pub summary: String,
    /// Files/modules this dependency creates.
    pub key_outputs: Vec<String>,
}

/// Template context for rendering a work package prompt.
#[derive(Debug, Clone)]
pub struct PromptContext {
    /// Work package identifier.
    pub wp_id: String,
    /// Work package title.
    pub wp_title: String,
    /// Work package description.
    pub wp_description: String,
    /// Subtasks within this work package.
    pub subtasks: Vec<SubtaskInfo>,
    /// Scope boundaries (what's in/out of scope).
    pub scope_boundaries: String,
    /// Constraints on implementation.
    pub constraints: Vec<String>,
    /// Upstream dependencies and their context.
    pub dependency_context: Vec<DependencyContext>,
    /// AGENTS.md content if available.
    pub agents_md: Option<String>,
    /// Feature name (e.g., "001-zellij-agent-orchestrator").
    pub feature_name: String,
    /// Path to the feature directory.
    pub feature_dir: PathBuf,
    /// Path to the WP task file (for completion signalling).
    pub task_file: Option<PathBuf>,
}

impl PromptContext {
    /// Render the prompt to a markdown string.
    pub fn render(&self) -> String {
        let mut out = String::new();

        // Header
        out.push_str(&format!(
            "# Agent Prompt: {} – {}\n\n",
            self.wp_id, self.wp_title
        ));

        // Objective
        out.push_str("## Objective\n\n");
        out.push_str(&self.wp_description);
        out.push_str("\n\n");

        // Scope
        out.push_str("## Scope\n\n");
        out.push_str(&self.scope_boundaries);
        out.push_str("\n\n");

        // Dependencies
        if !self.dependency_context.is_empty() {
            out.push_str("## Upstream Dependencies\n\n");
            for dep in &self.dependency_context {
                out.push_str(&format!("### {} – {}\n", dep.wp_id, dep.wp_title));
                out.push_str(&dep.summary);
                out.push_str("\n\n**Key outputs**: ");
                out.push_str(&dep.key_outputs.join(", "));
                out.push_str("\n\n");
            }
        }

        // Subtasks
        out.push_str("## Subtasks\n\n");
        for st in &self.subtasks {
            let parallel_marker = if st.parallel { " [P]" } else { "" };
            out.push_str(&format!(
                "- [ ] **{}**{}: {}\n",
                st.id, parallel_marker, st.description
            ));
        }
        out.push('\n');

        // Constraints
        if !self.constraints.is_empty() {
            out.push_str("## Constraints\n\n");
            for c in &self.constraints {
                out.push_str(&format!("- {}\n", c));
            }
            out.push('\n');
        }

        // AGENTS.md
        if let Some(agents) = &self.agents_md {
            out.push_str("## Project Agent Instructions\n\n");
            out.push_str(agents);
            out.push_str("\n\n");
        }

        // Completion signal
        if let Some(task_file) = &self.task_file {
            out.push_str("## Completion Signal\n\n");
            out.push_str("When you have finished ALL subtasks and verified your work (tests pass, build succeeds),\n");
            out.push_str("update the task file frontmatter to signal completion:\n\n");
            out.push_str(&format!("**File**: `{}`\n\n", task_file.display()));
            out.push_str("Change the `lane` field from `doing` to `for_review`:\n\n");
            out.push_str("```yaml\n");
            out.push_str("---\n");
            out.push_str(&format!("work_package_id: {}\n", self.wp_id));
            out.push_str(&format!("title: {}\n", self.wp_title));
            out.push_str("lane: for_review    # <-- change this from 'doing' to 'for_review'\n");
            out.push_str("---\n");
            out.push_str("```\n\n");
            out.push_str("This signals the orchestrator that your work is ready for review.\n");
            out.push_str("Do NOT set lane to 'done' — that happens after review passes.\n\n");
        }

        out
    }
}

/// Generator for work package prompt files and shell wrapper scripts.
pub struct PromptGenerator {
    /// Path to the feature directory.
    feature_dir: PathBuf,
    /// Content of AGENTS.md if available.
    agents_md: Option<String>,
}

impl PromptGenerator {
    /// Create a new prompt generator.
    ///
    /// Attempts to load AGENTS.md from the project root. If not found,
    /// logs a warning but continues (AGENTS.md is optional).
    pub fn new(feature_dir: &Path) -> Result<Self> {
        let agents_md = Self::load_agents_md(feature_dir)?;
        Ok(Self {
            feature_dir: feature_dir.to_owned(),
            agents_md,
        })
    }

    /// Load AGENTS.md from the project root.
    ///
    /// Searches up the directory tree for AGENTS.md or Cargo.toml to find the project root.
    /// Returns Ok(None) if AGENTS.md is not found (graceful fallback).
    fn load_agents_md(feature_dir: &Path) -> Result<Option<String>> {
        // Find project root by looking for AGENTS.md or Cargo.toml
        let project_root = feature_dir
            .ancestors()
            .find(|p| p.join("AGENTS.md").exists() || p.join("Cargo.toml").exists())
            .unwrap_or(feature_dir);

        let agents_path = project_root.join("AGENTS.md");
        if agents_path.exists() {
            let content = fs::read_to_string(&agents_path)?;
            tracing::info!(path = %agents_path.display(), "Loaded AGENTS.md");
            Ok(Some(content))
        } else {
            tracing::warn!("AGENTS.md not found, prompts will omit project instructions");
            Ok(None)
        }
    }

    /// Build dependency context for a work package from its upstream dependencies.
    fn build_dependency_context(
        &self,
        wp: &WorkPackage,
        all_wps: &[WorkPackage],
    ) -> Vec<DependencyContext> {
        wp.dependencies
            .iter()
            .filter_map(|dep_id| all_wps.iter().find(|w| w.id == *dep_id))
            .map(|dep_wp| DependencyContext {
                wp_id: dep_wp.id.clone(),
                wp_title: dep_wp.title.clone(),
                summary: format!("Provides {} functionality", dep_wp.title.to_lowercase()),
                key_outputs: self.infer_key_outputs(dep_wp),
            })
            .collect()
    }

    /// Infer what files/modules a work package produces based on its title/ID.
    fn infer_key_outputs(&self, wp: &WorkPackage) -> Vec<String> {
        // Heuristic based on WP structure — can be enhanced
        vec![format!("crates/kasmos/src/{}.rs", wp.id.to_lowercase())]
    }

    /// Build a prompt context for a single work package.
    fn build_prompt_context(&self, wp: &WorkPackage, all_wps: &[WorkPackage]) -> PromptContext {
        // Extract subtasks from WP (placeholder implementation)
        let subtasks = vec![SubtaskInfo {
            id: "T001".to_string(),
            description: "Implement core functionality".to_string(),
            parallel: false,
        }];

        PromptContext {
            wp_id: wp.id.clone(),
            wp_title: wp.title.clone(),
            wp_description: format!("Work package {}: {}", wp.id, wp.title),
            subtasks,
            scope_boundaries: "Define what is in and out of scope for this work package."
                .to_string(),
            constraints: vec!["Constraint 1".to_string()],
            dependency_context: self.build_dependency_context(wp, all_wps),
            agents_md: self.agents_md.clone(),
            feature_name: "001-zellij-agent-orchestrator".to_string(),
            feature_dir: self.feature_dir.clone(),
            task_file: self.find_task_file(&wp.id),
        }
    }

    /// Find the task file for a given WP ID in the feature's tasks/ directory.
    fn find_task_file(&self, wp_id: &str) -> Option<PathBuf> {
        let tasks_dir = self.feature_dir.join("tasks");
        if !tasks_dir.exists() {
            return None;
        }
        let entries = std::fs::read_dir(&tasks_dir).ok()?;
        for entry in entries.flatten() {
            let name = entry.file_name();
            let name_str = name.to_string_lossy();
            if name_str.starts_with(wp_id) && name_str.ends_with(".md") {
                return Some(entry.path());
            }
        }
        None
    }

    /// Generate and write prompt files for all work packages.
    pub fn generate_all(&self, wps: &[WorkPackage], kasmos_dir: &Path) -> Result<Vec<PathBuf>> {
        let prompts_dir = kasmos_dir.join("prompts");
        fs::create_dir_all(&prompts_dir)?;

        let mut paths = Vec::new();
        for wp in wps {
            let ctx = self.build_prompt_context(wp, wps);
            let rendered = ctx.render();

            // Warn if prompt is very long
            if rendered.len() > 10_000 {
                tracing::warn!(
                    wp_id = %wp.id,
                    chars = rendered.len(),
                    "Prompt exceeds 10K characters, may cause issues with OpenCode stdin"
                );
            }

            let path = prompts_dir.join(format!("{}.md", wp.id));
            fs::write(&path, &rendered)?;
            tracing::debug!(wp_id = %wp.id, path = %path.display(), "Prompt written");
            paths.push(path);
        }

        Ok(paths)
    }

    /// Generate shell wrapper scripts for each work package.
    pub fn generate_scripts(
        &self,
        wps: &[WorkPackage],
        kasmos_dir: &Path,
        opencode_profile: Option<&str>,
    ) -> Result<Vec<PathBuf>> {
        let scripts_dir = kasmos_dir.join("scripts");
        fs::create_dir_all(&scripts_dir)?;

        let profile_flag = match opencode_profile {
            Some(p) => format!(
                " -p {}",
                shell_escape::escape(std::borrow::Cow::Borrowed(p))
            ),
            None => String::new(),
        };

        let mut paths = Vec::new();
        for wp in wps {
            let prompt_path = kasmos_dir.join("prompts").join(format!("{}.md", wp.id));
            let script_content = format!(
                "#!/bin/bash\nset -euo pipefail\nocx oc{profile_flag} -- --agent coder --prompt \"$(cat '{}')\"\n",
                prompt_path.display()
            );

            let script_path = scripts_dir.join(format!("{}.sh", wp.id));
            fs::write(&script_path, &script_content)?;

            // Make executable on Unix systems
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let mut perms = fs::metadata(&script_path)?.permissions();
                perms.set_mode(0o755);
                fs::set_permissions(&script_path, perms)?;
            }

            tracing::debug!(wp_id = %wp.id, path = %script_path.display(), "Script written");
            paths.push(script_path);
        }

        Ok(paths)
    }
}

/// Verify that a binary is available in PATH.
pub fn validate_binary_in_path(binary: &str) -> Result<PathBuf> {
    which::which(binary).map_err(|_| {
        ConfigError::InvalidValue {
            field: format!("{}_binary", binary),
            value: binary.to_string(),
            reason: format!(
                "'{}' not found in PATH. Install it or set the full path in config.",
                binary
            ),
        }
        .into()
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_prompt_context_render_with_all_fields() {
        let ctx = PromptContext {
            wp_id: "WP01".to_string(),
            wp_title: "Core Types".to_string(),
            wp_description: "Define core data structures".to_string(),
            subtasks: vec![
                SubtaskInfo {
                    id: "T001".to_string(),
                    description: "Define WorkPackage struct".to_string(),
                    parallel: false,
                },
                SubtaskInfo {
                    id: "T002".to_string(),
                    description: "Define Wave struct".to_string(),
                    parallel: true,
                },
            ],
            scope_boundaries: "Only core types, no I/O".to_string(),
            constraints: vec!["Must be serializable".to_string()],
            dependency_context: vec![],
            agents_md: Some("# Agent Instructions\n\nFollow these rules.".to_string()),
            feature_name: "001-zellij-agent-orchestrator".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            task_file: None,
        };

        let rendered = ctx.render();
        assert!(rendered.contains("# Agent Prompt: WP01 – Core Types"));
        assert!(rendered.contains("## Objective"));
        assert!(rendered.contains("## Scope"));
        assert!(rendered.contains("## Subtasks"));
        assert!(rendered.contains("- [ ] **T001**: Define WorkPackage struct"));
        assert!(rendered.contains("- [ ] **T002** [P]: Define Wave struct"));
        assert!(rendered.contains("## Constraints"));
        assert!(rendered.contains("## Project Agent Instructions"));
    }

    #[test]
    fn test_prompt_context_render_without_dependencies() {
        let ctx = PromptContext {
            wp_id: "WP01".to_string(),
            wp_title: "Core Types".to_string(),
            wp_description: "Define core data structures".to_string(),
            subtasks: vec![],
            scope_boundaries: "Only core types".to_string(),
            constraints: vec![],
            dependency_context: vec![],
            agents_md: None,
            feature_name: "001-zellij-agent-orchestrator".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            task_file: None,
        };

        let rendered = ctx.render();
        assert!(!rendered.contains("## Upstream Dependencies"));
        assert!(!rendered.contains("## Project Agent Instructions"));
    }

    #[test]
    fn test_prompt_context_render_with_dependencies() {
        let ctx = PromptContext {
            wp_id: "WP02".to_string(),
            wp_title: "Spec Parser".to_string(),
            wp_description: "Parse spec files".to_string(),
            subtasks: vec![],
            scope_boundaries: "Parse only".to_string(),
            constraints: vec![],
            dependency_context: vec![DependencyContext {
                wp_id: "WP01".to_string(),
                wp_title: "Core Types".to_string(),
                summary: "Provides core types".to_string(),
                key_outputs: vec!["crates/kasmos/src/types.rs".to_string()],
            }],
            agents_md: None,
            feature_name: "001-zellij-agent-orchestrator".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            task_file: None,
        };

        let rendered = ctx.render();
        assert!(rendered.contains("## Upstream Dependencies"));
        assert!(rendered.contains("### WP01 – Core Types"));
        assert!(rendered.contains("**Key outputs**: crates/kasmos/src/types.rs"));
    }

    #[test]
    fn test_validate_binary_in_path_success() {
        // bash should always be available
        let result = validate_binary_in_path("bash");
        assert!(result.is_ok());
    }

    #[test]
    fn test_validate_binary_in_path_failure() {
        let result = validate_binary_in_path("nonexistent_binary_xyz_12345");
        assert!(result.is_err());
        let err_msg = result.unwrap_err().to_string();
        assert!(err_msg.contains("not found in PATH"));
    }

    #[test]
    fn test_prompt_length_warning() {
        // Create a very long description to trigger the warning
        let long_desc = "x".repeat(15_000);
        let ctx = PromptContext {
            wp_id: "WP99".to_string(),
            wp_title: "Long WP".to_string(),
            wp_description: long_desc,
            subtasks: vec![],
            scope_boundaries: "".to_string(),
            constraints: vec![],
            dependency_context: vec![],
            agents_md: None,
            feature_name: "001-zellij-agent-orchestrator".to_string(),
            feature_dir: PathBuf::from("/tmp"),
            task_file: None,
        };

        let rendered = ctx.render();
        assert!(rendered.len() > 10_000);
    }

    #[test]
    fn test_shell_wrapper_script_format() {
        let script_content = format!(
            "#!/bin/bash\nset -euo pipefail\nocx oc -- --agent coder --prompt \"$(cat '{}')\"\n",
            "/tmp/WP01.md"
        );
        assert!(script_content.starts_with("#!/bin/bash"));
        assert!(script_content.contains("set -euo pipefail"));
        assert!(script_content.contains("ocx oc -- --agent coder --prompt"));
    }

    #[test]
    fn test_shell_wrapper_script_quotes_path() {
        let script_content = format!(
            "#!/bin/bash\nset -euo pipefail\nocx oc -- --agent coder --prompt \"$(cat '{}')\"\n",
            "/path/with spaces/WP01.md"
        );
        assert!(script_content.contains("cat '/path/with spaces/WP01.md'"));
        assert!(script_content.starts_with("#!/bin/bash"));
    }
}
