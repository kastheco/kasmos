//! `kasmos new` -- launch a planning agent to create a new feature specification.

use anyhow::{bail, Context, Result};
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::config::Config;
use crate::prompt::{read_file_if_exists, summarize_markdown};

/// Verify that opencode and spec-kitty binaries are available in PATH.
fn preflight_check(config: &Config) -> Result<()> {
    which::which(&config.agent.opencode_binary).with_context(|| {
        format!(
            "'{}' not found in PATH. Install opencode or set agent.opencode_binary in kasmos.toml.",
            config.agent.opencode_binary
        )
    })?;

    which::which(&config.paths.spec_kitty_binary).with_context(|| {
        format!(
            "'{}' not found in PATH. Install spec-kitty or set paths.spec_kitty_binary in kasmos.toml.",
            config.paths.spec_kitty_binary
        )
    })?;

    Ok(())
}

/// Walk up from CWD looking for `Cargo.toml` or `.kittify/` to find the project root.
fn find_repo_root() -> Result<PathBuf> {
    let cwd = std::env::current_dir().context("Failed to get current directory")?;

    for ancestor in cwd.ancestors() {
        if ancestor.join("Cargo.toml").is_file() || ancestor.join(".kittify").is_dir() {
            return Ok(ancestor.to_path_buf());
        }
    }

    bail!(
        "Could not find project root (no Cargo.toml or .kittify/ found). \
         Run this command from inside a kasmos project directory."
    );
}

/// Build the planning agent prompt with project context.
fn build_prompt(repo_root: &Path, description: Option<&str>) -> Result<String> {
    let mut sections = Vec::new();

    // Role instruction header
    sections.push(
        "You are a planning agent for the kasmos project. \
         Your task is to create a new feature specification.\n\
         \n\
         Run the `/spec-kitty.specify` command to start the specification workflow."
            .to_string(),
    );

    // Optional feature description
    if let Some(desc) = description {
        sections.push(format!("## Initial Feature Description\n\n{desc}"));
    }

    // Project context from .kittify/memory/
    let memory_dir = repo_root.join(".kittify/memory");

    let memory_files = [
        ("constitution.md", "Project Constitution"),
        ("architecture.md", "Architecture"),
        ("workflow-intelligence.md", "Workflow Intelligence"),
    ];

    for (filename, heading) in &memory_files {
        let path = memory_dir.join(filename);
        if let Some(content) =
            read_file_if_exists(&path).with_context(|| format!("reading {}", path.display()))?
        {
            let summary = summarize_markdown(&content, 80);
            if !summary.is_empty() {
                sections.push(format!("## {heading}\n\n{summary}"));
            }
        }
    }

    // List existing specs from kitty-specs/
    let specs_dir = repo_root.join("kitty-specs");
    if specs_dir.is_dir() {
        let mut spec_names: Vec<String> = std::fs::read_dir(&specs_dir)
            .with_context(|| format!("reading {}", specs_dir.display()))?
            .filter_map(|e| e.ok())
            .filter(|e| e.path().is_dir())
            .filter_map(|e| e.file_name().to_str().map(String::from))
            .collect();
        spec_names.sort();

        if !spec_names.is_empty() {
            let list = spec_names
                .iter()
                .map(|n| format!("- {n}"))
                .collect::<Vec<_>>()
                .join("\n");
            sections.push(format!("## Existing Specifications\n\n{list}"));
        }
    }

    // List top-level project directories (excluding dotfiles)
    let mut top_dirs: Vec<String> = std::fs::read_dir(repo_root)
        .with_context(|| format!("reading {}", repo_root.display()))?
        .filter_map(|e| e.ok())
        .filter(|e| e.path().is_dir())
        .filter_map(|e| {
            let name = e.file_name();
            let name_str = name.to_str()?;
            if name_str.starts_with('.') {
                None
            } else {
                Some(name_str.to_string())
            }
        })
        .collect();
    top_dirs.sort();

    if !top_dirs.is_empty() {
        let list = top_dirs
            .iter()
            .map(|d| format!("- {d}/"))
            .collect::<Vec<_>>()
            .join("\n");
        sections.push(format!("## Project Structure (top-level)\n\n{list}"));
    }

    Ok(sections.join("\n\n"))
}

/// Spawn opencode in the current terminal as a planning agent.
fn spawn_opencode(config: &Config, prompt: &str) -> Result<i32> {
    let mut cmd = Command::new(&config.agent.opencode_binary);

    // -p profile switching is only supported by ocx, not opencode native
    // if let Some(ref profile) = config.agent.opencode_profile {
    //     cmd.arg("-p").arg(profile);
    // }

    cmd.arg("--agent")
        .arg("planner")
        .arg("--prompt")
        .arg(prompt);

    let status = cmd
        .status()
        .with_context(|| format!("Failed to spawn '{}'", config.agent.opencode_binary))?;

    Ok(status.code().unwrap_or(1))
}

/// Entry point for `kasmos new [description...]`.
pub fn run(description: Option<&str>) -> Result<i32> {
    let config = Config::load().context("Failed to load config")?;
    preflight_check(&config)?;
    let repo_root = find_repo_root()?;
    let prompt = build_prompt(&repo_root, description)?;
    spawn_opencode(&config, &prompt)
}

#[cfg(test)]
mod tests {
    use super::*;

    // ── Fixture helper ──────────────────────────────────────────────

    fn setup_test_repo() -> tempfile::TempDir {
        let root = tempfile::tempdir().unwrap();
        std::fs::write(root.path().join("Cargo.toml"), "[workspace]\n").unwrap();
        let memory = root.path().join(".kittify/memory");
        std::fs::create_dir_all(&memory).unwrap();
        std::fs::write(
            memory.join("constitution.md"),
            "# Constitution\n\n## Technical Standards\n\n- Rust 2024\n- tokio async",
        )
        .unwrap();
        std::fs::write(
            memory.join("architecture.md"),
            "# Architecture\n\nARCH_CONTENT_SENTINEL",
        )
        .unwrap();
        let specs = root.path().join("kitty-specs/011-test-feature");
        std::fs::create_dir_all(&specs).unwrap();
        root
    }

    // ── T008: Pre-flight validation ─────────────────────────────────

    #[test]
    fn preflight_fails_when_opencode_missing() {
        let mut config = Config::default();
        config.agent.opencode_binary = "__nonexistent_opencode_xyz__".to_string();

        let err = preflight_check(&config).unwrap_err();
        let msg = err.to_string();
        assert!(
            msg.contains("__nonexistent_opencode_xyz__"),
            "error should mention the binary name, got: {msg}"
        );
        assert!(
            msg.contains("not found in PATH"),
            "error should say 'not found in PATH', got: {msg}"
        );
    }

    #[test]
    fn preflight_fails_when_spec_kitty_missing() {
        let mut config = Config::default();
        config.agent.opencode_binary = "bash".to_string();
        config.paths.spec_kitty_binary = "__nonexistent_spec_kitty_xyz__".to_string();

        let err = preflight_check(&config).unwrap_err();
        let msg = err.to_string();
        assert!(
            msg.contains("__nonexistent_spec_kitty_xyz__"),
            "error should mention the spec-kitty binary name, got: {msg}"
        );
    }

    #[test]
    fn preflight_passes_with_real_binaries() {
        let mut config = Config::default();
        config.agent.opencode_binary = "bash".to_string();
        config.paths.spec_kitty_binary = "bash".to_string();

        assert!(preflight_check(&config).is_ok());
    }

    // ── T009: Prompt construction ───────────────────────────────────

    #[test]
    fn prompt_contains_specify_instruction() {
        let repo = setup_test_repo();
        let prompt = build_prompt(repo.path(), None).unwrap();

        assert!(
            prompt.contains("/spec-kitty.specify"),
            "prompt should contain '/spec-kitty.specify', got:\n{prompt}"
        );
    }

    #[test]
    fn prompt_includes_description_when_provided() {
        let repo = setup_test_repo();
        let prompt = build_prompt(repo.path(), Some("add dark mode toggle")).unwrap();

        assert!(
            prompt.contains("add dark mode toggle"),
            "prompt should contain the description text"
        );
        assert!(
            prompt.contains("Initial Feature Description"),
            "prompt should contain 'Initial Feature Description' heading"
        );
    }

    #[test]
    fn prompt_omits_description_when_not_provided() {
        let repo = setup_test_repo();
        let prompt = build_prompt(repo.path(), None).unwrap();

        assert!(
            !prompt.contains("Initial Feature Description"),
            "prompt should NOT contain 'Initial Feature Description' when no description given"
        );
    }

    #[test]
    fn prompt_includes_project_context() {
        let repo = setup_test_repo();
        let prompt = build_prompt(repo.path(), None).unwrap();

        assert!(
            prompt.contains("Rust 2024"),
            "prompt should contain constitution content 'Rust 2024'"
        );
        assert!(
            prompt.contains("ARCH_CONTENT_SENTINEL"),
            "prompt should contain architecture sentinel"
        );
        assert!(
            prompt.contains("011-test-feature"),
            "prompt should list the existing spec directory"
        );
    }

    // ── T010: Prompt degradation ────────────────────────────────────

    #[test]
    fn prompt_handles_missing_memory_gracefully() {
        let root = tempfile::tempdir().unwrap();
        std::fs::write(root.path().join("Cargo.toml"), "[workspace]\n").unwrap();
        // No .kittify/memory/ at all

        let prompt = build_prompt(root.path(), None).unwrap();

        assert!(
            prompt.contains("/spec-kitty.specify"),
            "prompt should still contain the specify instruction"
        );
        assert!(
            !prompt.contains("Constitution"),
            "prompt should NOT contain 'Constitution' when memory dir is absent"
        );
        assert!(
            !prompt.contains("Architecture"),
            "prompt should NOT contain 'Architecture' when memory dir is absent"
        );
    }

    #[test]
    fn prompt_handles_partial_memory() {
        let root = tempfile::tempdir().unwrap();
        std::fs::write(root.path().join("Cargo.toml"), "[workspace]\n").unwrap();
        let memory = root.path().join(".kittify/memory");
        std::fs::create_dir_all(&memory).unwrap();
        std::fs::write(
            memory.join("constitution.md"),
            "# Constitution\n\n## Technical Standards\n\n- Rust 2024\n- tokio async",
        )
        .unwrap();
        // No architecture.md

        let prompt = build_prompt(root.path(), None).unwrap();

        assert!(
            prompt.contains("Rust 2024"),
            "prompt should contain constitution content"
        );
        assert!(
            !prompt.contains("Architecture"),
            "prompt should NOT contain 'Architecture' when architecture.md is absent"
        );
    }
}
