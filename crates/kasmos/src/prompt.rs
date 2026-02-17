//! Prompt construction for role-based MCP agents.

use crate::error::{ConfigError, Result};
use crate::parser::{WPFrontmatter, parse_frontmatter};
use std::fs;
use std::path::{Path, PathBuf};

const PROFILE_ROOT: &str = "config/profiles/kasmos";

/// Prompt-side agent role. Wraps `registry::AgentRole` for spawnable roles
/// and adds `Manager` which is the orchestrator (never spawned as a worker).
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum AgentRole {
    Manager,
    Worker(crate::serve::registry::AgentRole),
}

impl AgentRole {
    pub const PLANNER: Self = Self::Worker(crate::serve::registry::AgentRole::Planner);
    pub const CODER: Self = Self::Worker(crate::serve::registry::AgentRole::Coder);
    pub const REVIEWER: Self = Self::Worker(crate::serve::registry::AgentRole::Reviewer);
    pub const RELEASE: Self = Self::Worker(crate::serve::registry::AgentRole::Release);

    pub fn template_name(self) -> &'static str {
        match self {
            Self::Manager => "manager.md",
            Self::Worker(role) => match role {
                crate::serve::registry::AgentRole::Planner => "planner.md",
                crate::serve::registry::AgentRole::Coder => "coder.md",
                crate::serve::registry::AgentRole::Reviewer => "reviewer.md",
                crate::serve::registry::AgentRole::Release => "release.md",
            },
        }
    }

    pub fn as_str(self) -> &'static str {
        match self {
            Self::Manager => "manager",
            Self::Worker(role) => role.as_str(),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ContextBoundary {
    pub spec: bool,
    pub plan: bool,
    pub all_tasks: bool,
    pub architecture: bool,
    pub workflow_intelligence: bool,
    pub constitution: bool,
    pub project_structure: bool,
    pub wp_task_file: bool,
    pub coding_standards: bool,
}

pub fn allowed_context(role: AgentRole) -> ContextBoundary {
    use crate::serve::registry::AgentRole as WR;
    match role {
        AgentRole::Manager => ContextBoundary {
            spec: true,
            plan: true,
            all_tasks: true,
            architecture: true,
            workflow_intelligence: true,
            constitution: true,
            project_structure: true,
            wp_task_file: false,
            coding_standards: false,
        },
        AgentRole::Worker(WR::Coder) => ContextBoundary {
            spec: false,
            plan: false,
            all_tasks: false,
            architecture: true,
            workflow_intelligence: false,
            constitution: true,
            project_structure: false,
            wp_task_file: true,
            coding_standards: true,
        },
        AgentRole::Worker(WR::Reviewer) => ContextBoundary {
            spec: false,
            plan: false,
            all_tasks: false,
            architecture: true,
            workflow_intelligence: false,
            constitution: true,
            project_structure: false,
            wp_task_file: true,
            coding_standards: true,
        },
        AgentRole::Worker(WR::Release) => ContextBoundary {
            spec: false,
            plan: false,
            all_tasks: true,
            architecture: false,
            workflow_intelligence: false,
            constitution: true,
            project_structure: true,
            wp_task_file: false,
            coding_standards: false,
        },
        AgentRole::Worker(WR::Planner) => ContextBoundary {
            spec: true,
            plan: true,
            all_tasks: false,
            architecture: true,
            workflow_intelligence: true,
            constitution: true,
            project_structure: true,
            wp_task_file: false,
            coding_standards: false,
        },
    }
}

pub struct RolePromptBuilder {
    role: AgentRole,
    feature_slug: String,
    feature_dir: PathBuf,
    wp_id: Option<String>,
    wp_file: Option<PathBuf>,
    additional_context: Option<String>,
}

impl RolePromptBuilder {
    pub fn new(
        role: AgentRole,
        feature_slug: impl Into<String>,
        feature_dir: impl Into<PathBuf>,
    ) -> Self {
        Self {
            role,
            feature_slug: feature_slug.into(),
            feature_dir: feature_dir.into(),
            wp_id: None,
            wp_file: None,
            additional_context: None,
        }
    }

    pub fn with_wp_id(mut self, wp_id: impl Into<String>) -> Self {
        self.wp_id = Some(wp_id.into());
        self
    }

    pub fn with_wp_file(mut self, wp_file: impl Into<PathBuf>) -> Self {
        self.wp_file = Some(wp_file.into());
        self
    }

    pub fn with_additional_context(mut self, additional_context: impl Into<String>) -> Self {
        self.additional_context = Some(additional_context.into());
        self
    }

    pub fn build(&self) -> Result<String> {
        let repo_root = self.find_repo_root()?;
        let template = self.read_role_template(&repo_root)?;
        let boundary = allowed_context(self.role);
        let context = self.build_context_sections(&boundary)?;

        let mut rendered = template.replace("{{FEATURE_SLUG}}", &self.feature_slug);
        let wp_id = self.wp_id.as_deref().unwrap_or("N/A");
        rendered = rendered.replace("{{WP_ID}}", wp_id);

        if rendered.contains("{{CONTEXT}}") {
            rendered = rendered.replace("{{CONTEXT}}", &context);
        } else {
            rendered.push_str("\n\n## Runtime Context\n\n");
            rendered.push_str(&context);
        }

        Ok(rendered)
    }

    fn build_context_sections(&self, boundary: &ContextBoundary) -> Result<String> {
        let mut sections = Vec::new();
        let repo_root = self.find_repo_root()?;

        if boundary.spec {
            let spec_path = self.feature_dir.join("spec.md");
            let spec = read_file_if_exists(&spec_path)?;
            if let Some(spec) = spec {
                sections.push(format!(
                    "## Spec Summary\n\nPath: `{}`\n\n{}",
                    spec_path.display(),
                    summarize_markdown(&spec, 12)
                ));
            }
        }

        if boundary.plan {
            let plan_path = self.feature_dir.join("plan.md");
            let plan = read_file_if_exists(&plan_path)?;
            if let Some(plan) = plan {
                sections.push(format!(
                    "## Plan Summary\n\nPath: `{}`\n\n{}",
                    plan_path.display(),
                    summarize_markdown(&plan, 12)
                ));
            }
        }

        if boundary.all_tasks {
            let task_overview = self.build_task_overview()?;
            if !task_overview.is_empty() {
                sections.push(format!("## Task Board Overview\n\n{task_overview}"));
            }
        }

        if boundary.wp_task_file {
            let wp_file = self.resolve_wp_file()?;
            let wp_content = fs::read_to_string(&wp_file)?;
            sections.push(format!(
                "## WP Task Contract\n\nPath: `{}`\n\n{}",
                wp_file.display(),
                wp_content
            ));

            if self.role == AgentRole::REVIEWER {
                let acceptance = extract_section(&wp_content, "Objectives & Success Criteria")
                    .or_else(|| extract_section(&wp_content, "Review Guidance"));
                if let Some(acceptance) = acceptance {
                    sections.push(format!("## Acceptance Criteria\n\n{acceptance}"));
                }
            }
        }

        if boundary.architecture {
            let architecture_path = repo_root.join(".kittify/memory/architecture.md");
            let architecture = read_file_if_exists(&architecture_path)?;
            if let Some(architecture) = architecture {
                sections.push(format!(
                    "## Architecture Notes\n\nPath: `{}`\n\n{}",
                    architecture_path.display(),
                    summarize_markdown(&architecture, 12)
                ));
            }
        }

        if boundary.workflow_intelligence {
            let workflow_path = repo_root.join(".kittify/memory/workflow-intelligence.md");
            let workflow = read_file_if_exists(&workflow_path)?;
            if let Some(workflow) = workflow {
                sections.push(format!(
                    "## Workflow Intelligence\n\nPath: `{}`\n\n{}",
                    workflow_path.display(),
                    summarize_markdown(&workflow, 10)
                ));
            }
        }

        if boundary.constitution {
            let constitution_path = repo_root.join(".kittify/memory/constitution.md");
            let constitution = read_file_if_exists(&constitution_path)?;
            if let Some(constitution) = constitution {
                sections.push(format!(
                    "## Constitution\n\nPath: `{}`\n\n{}",
                    constitution_path.display(),
                    summarize_markdown(&constitution, 10)
                ));
            }
        }

        if boundary.coding_standards {
            let constitution_path = repo_root.join(".kittify/memory/constitution.md");
            let constitution = read_file_if_exists(&constitution_path)?;
            if let Some(constitution) = constitution {
                let standards = extract_section(&constitution, "Technical Standards")
                    .unwrap_or_else(|| summarize_markdown(&constitution, 8));
                sections.push(format!(
                    "## Coding Standards\n\n### Technical Standards\n\n{standards}"
                ));
            }
        }

        if boundary.project_structure {
            let structure = self.build_project_structure_overview()?;
            if !structure.is_empty() {
                sections.push(format!("## Project Structure\n\n{structure}"));
            }
        }

        if self.role == AgentRole::RELEASE {
            sections.push("## Branch and Merge Target\n\n- Merge target: `main`\n- Release lane input: all WPs currently in `for_review` or `done`\n- Validate branch consistency before merge".to_string());
        }

        if let Some(additional) = &self.additional_context {
            sections.push(format!("## Additional Context\n\n{additional}"));
        }

        Ok(sections.join("\n\n"))
    }

    fn resolve_wp_file(&self) -> Result<PathBuf> {
        if let Some(path) = &self.wp_file {
            return Ok(path.clone());
        }

        let wp_id = self
            .wp_id
            .as_ref()
            .ok_or_else(|| ConfigError::InvalidValue {
                field: "wp_id".to_string(),
                value: "<missing>".to_string(),
                reason: "WP-scoped role requires wp_id or explicit wp_file".to_string(),
            })?;

        let tasks_dir = self.feature_dir.join("tasks");
        if !tasks_dir.exists() {
            return Err(ConfigError::InvalidValue {
                field: "tasks_dir".to_string(),
                value: tasks_dir.display().to_string(),
                reason: "tasks directory missing".to_string(),
            }
            .into());
        }

        for entry in fs::read_dir(&tasks_dir)? {
            let entry = entry?;
            let path = entry.path();
            if !path.is_file() {
                continue;
            }
            if let Some(name) = path.file_name().and_then(|n| n.to_str())
                && name.starts_with(wp_id)
                && name.ends_with(".md")
            {
                return Ok(path);
            }
        }

        Err(ConfigError::InvalidValue {
            field: "wp_file".to_string(),
            value: wp_id.clone(),
            reason: "could not resolve WP task file".to_string(),
        }
        .into())
    }

    fn build_task_overview(&self) -> Result<String> {
        let tasks_dir = self.feature_dir.join("tasks");
        if !tasks_dir.exists() {
            return Ok(String::new());
        }

        let mut rows = Vec::new();
        for entry in fs::read_dir(&tasks_dir)? {
            let entry = entry?;
            let path = entry.path();
            if !path.is_file() {
                continue;
            }
            if let Some(name) = path.file_name().and_then(|n| n.to_str())
                && name.starts_with("WP")
                && name.ends_with(".md")
            {
                let frontmatter: WPFrontmatter = parse_frontmatter(&path)?;
                rows.push(format!(
                    "- {} [{}] - {}",
                    frontmatter.work_package_id, frontmatter.lane, frontmatter.title
                ));
            }
        }

        rows.sort();
        Ok(rows.join("\n"))
    }

    fn build_project_structure_overview(&self) -> Result<String> {
        let repo_root = self.find_repo_root()?;
        let mut dirs = Vec::new();
        for entry in fs::read_dir(repo_root)? {
            let entry = entry?;
            let path = entry.path();
            if path.is_dir()
                && let Some(name) = path.file_name().and_then(|n| n.to_str())
                && !name.starts_with('.')
            {
                dirs.push(format!("- `{name}/`"));
            }
        }
        dirs.sort();
        Ok(dirs.join("\n"))
    }

    fn find_repo_root(&self) -> Result<PathBuf> {
        self.feature_dir
            .ancestors()
            .find(|path| path.join("Cargo.toml").exists() || path.join(".kittify").exists())
            .map(Path::to_path_buf)
            .ok_or_else(|| {
                ConfigError::InvalidValue {
                    field: "feature_dir".to_string(),
                    value: self.feature_dir.display().to_string(),
                    reason: "could not discover repository root".to_string(),
                }
                .into()
            })
    }

    fn read_role_template(&self, repo_root: &Path) -> Result<String> {
        let template_path = repo_root
            .join(PROFILE_ROOT)
            .join("agent")
            .join(self.role.template_name());

        fs::read_to_string(&template_path).map_err(|err| {
            ConfigError::InvalidValue {
                field: "profile_template".to_string(),
                value: template_path.display().to_string(),
                reason: err.to_string(),
            }
            .into()
        })
    }
}

pub(crate) fn summarize_markdown(content: &str, max_lines: usize) -> String {
    let mut kept = Vec::new();
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.is_empty() {
            continue;
        }
        if trimmed.starts_with('#') {
            kept.push(trimmed.to_string());
            continue;
        }
        kept.push(trimmed.to_string());
        if kept.len() >= max_lines {
            break;
        }
    }

    if kept.is_empty() {
        return String::new();
    }

    kept.join("\n")
}

fn extract_section(content: &str, heading: &str) -> Option<String> {
    let needle = format!("## {heading}");
    let mut in_section = false;
    let mut lines = Vec::new();

    for line in content.lines() {
        if line.trim() == needle {
            in_section = true;
            continue;
        }
        if in_section && line.starts_with("## ") {
            break;
        }
        if in_section {
            lines.push(line.to_string());
        }
    }

    if lines.is_empty() {
        None
    } else {
        Some(lines.join("\n").trim().to_string())
    }
}

pub(crate) fn read_file_if_exists(path: &Path) -> Result<Option<String>> {
    if path.exists() {
        Ok(Some(fs::read_to_string(path)?))
    } else {
        Ok(None)
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
    use tempfile::tempdir;

    #[test]
    fn test_allowed_context_boundaries() {
        let manager = allowed_context(AgentRole::Manager);
        assert!(manager.spec);
        assert!(manager.plan);
        assert!(manager.all_tasks);

        let coder = allowed_context(AgentRole::CODER);
        assert!(coder.wp_task_file);
        assert!(!coder.spec);
        assert!(!coder.plan);
        assert!(!coder.all_tasks);

        let planner = allowed_context(AgentRole::PLANNER);
        assert!(planner.spec);
        assert!(planner.plan);
        assert!(!planner.wp_task_file);
        assert!(!planner.coding_standards);
    }

    #[test]
    fn test_validate_binary_in_path_success() {
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
    fn test_coder_prompt_excludes_spec_and_plan() {
        let fixture = Fixture::new();
        let prompt = RolePromptBuilder::new(AgentRole::CODER, "011-test", fixture.feature_dir())
            .with_wp_id("WP01")
            .build()
            .unwrap();

        assert!(prompt.contains("WP Task Contract"));
        assert!(prompt.contains("Technical Standards"));
        assert!(!prompt.contains("SPEC_SECRET_SENTINEL"));
        assert!(!prompt.contains("PLAN_SECRET_SENTINEL"));
        assert!(!prompt.contains("WP02 [planned]"));
    }

    #[test]
    fn test_manager_prompt_contains_broad_context() {
        let fixture = Fixture::new();
        let prompt = RolePromptBuilder::new(AgentRole::Manager, "011-test", fixture.feature_dir())
            .build()
            .unwrap();

        assert!(prompt.contains("Spec Summary"));
        assert!(prompt.contains("Plan Summary"));
        assert!(prompt.contains("Task Board Overview"));
        assert!(prompt.contains("Workflow Intelligence"));
    }

    #[test]
    fn test_reviewer_prompt_has_acceptance_and_additional_context() {
        let fixture = Fixture::new();
        let prompt = RolePromptBuilder::new(AgentRole::REVIEWER, "011-test", fixture.feature_dir())
            .with_wp_id("WP01")
            .with_additional_context("Changed files: crates/kasmos/src/prompt.rs")
            .build()
            .unwrap();

        assert!(prompt.contains("Acceptance Criteria"));
        assert!(prompt.contains("Changed files"));
        assert!(!prompt.contains("SPEC_SECRET_SENTINEL"));
    }

    struct Fixture {
        #[allow(dead_code)] // held to keep the TempDir alive
        root: tempfile::TempDir,
        feature_dir: PathBuf,
    }

    impl Fixture {
        fn new() -> Self {
            let root = tempdir().unwrap();
            let root_path = root.path();

            fs::write(root_path.join("Cargo.toml"), "[workspace]\nmembers = []\n").unwrap();

            let profile_dir = root_path.join("config/profiles/kasmos/agent");
            fs::create_dir_all(&profile_dir).unwrap();
            for (name, role) in [
                ("manager.md", "manager"),
                ("planner.md", "planner"),
                ("coder.md", "coder"),
                ("reviewer.md", "reviewer"),
                ("release.md", "release"),
            ] {
                fs::write(
                    profile_dir.join(name),
                    format!("# {role}\n\nFeature: {{FEATURE_SLUG}}\nWP: {{WP_ID}}\n\n{{CONTEXT}}"),
                )
                .unwrap();
            }

            let memory_dir = root_path.join(".kittify/memory");
            fs::create_dir_all(&memory_dir).unwrap();
            fs::write(
                memory_dir.join("architecture.md"),
                "# Architecture\n\nARCH_SENTINEL",
            )
            .unwrap();
            fs::write(
                memory_dir.join("workflow-intelligence.md"),
                "# Workflow\n\nWORKFLOW_SENTINEL",
            )
            .unwrap();
            fs::write(
                memory_dir.join("constitution.md"),
                "# Constitution\n\n## Technical Standards\n\n- Rust 2024\n- tokio",
            )
            .unwrap();

            let feature_dir = root_path.join("kitty-specs/011-test");
            let tasks_dir = feature_dir.join("tasks");
            fs::create_dir_all(&tasks_dir).unwrap();
            fs::write(
                feature_dir.join("spec.md"),
                "# Spec\n\nSPEC_SECRET_SENTINEL",
            )
            .unwrap();
            fs::write(
                feature_dir.join("plan.md"),
                "# Plan\n\nPLAN_SECRET_SENTINEL",
            )
            .unwrap();

            fs::write(
                tasks_dir.join("WP01-prompt.md"),
                "---\nwork_package_id: WP01\ntitle: Prompt\nlane: doing\n---\n\n## Objectives & Success Criteria\n\n- Criterion A\n\n## Review Guidance\n\n- Verify contract",
            )
            .unwrap();
            fs::write(
                tasks_dir.join("WP02-followup.md"),
                "---\nwork_package_id: WP02\ntitle: Followup\nlane: planned\n---\n",
            )
            .unwrap();

            Self { root, feature_dir }
        }

        fn feature_dir(&self) -> PathBuf {
            self.feature_dir.clone()
        }
    }

}
