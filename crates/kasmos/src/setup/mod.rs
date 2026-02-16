//! Environment validation and default config generation.

use crate::config::Config;
use anyhow::{Context, Result};
use std::io::IsTerminal;
use std::path::{Path, PathBuf};
use std::process::Command;

const STATUS_PASS: &str = "[PASS]";
const STATUS_FAIL: &str = "[FAIL]";
const STATUS_WARN: &str = "[WARN]";
const STATUS_PASS_COLOR: &str = "\x1b[32m[PASS]\x1b[0m";
const STATUS_FAIL_COLOR: &str = "\x1b[31m[FAIL]\x1b[0m";
const STATUS_WARN_COLOR: &str = "\x1b[33m[WARN]\x1b[0m";

/// Setup command result.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SetupResult {
    pub checks: Vec<CheckResult>,
    pub all_passed: bool,
}

/// One environment check outcome.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct CheckResult {
    pub name: String,
    pub required_for: String,
    pub description: String,
    pub status: CheckStatus,
    pub guidance: Option<String>,
}

/// Setup check status.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CheckStatus {
    Pass,
    Fail,
    Warn,
}

/// Run setup checks and generate missing baseline assets.
pub async fn run() -> Result<()> {
    let config = Config::load().context("Failed to load config")?;
    let repo_root = detect_repo_root();
    let mut result = validate_environment_with_repo(&config, repo_root.clone())?;

    println!("kasmos setup");
    print_results(&result);

    if let Some(repo_root) = repo_root.as_deref() {
        let created = ensure_baseline_assets(repo_root, &config)?;
        if created.is_empty() {
            println!("\nNo new baseline assets created.");
        } else {
            println!("\nCreated baseline assets:");
            for path in created {
                println!("- {}", path.display());
            }
        }
    }

    // Install agent definitions into repo-level .opencode/agents/.
    if let Some(repo_root) = repo_root.as_deref() {
        match install_opencode_agents(repo_root) {
            Ok(created) if created.is_empty() => {
                println!("\nOpenCode agents: all roles already installed.");
            }
            Ok(created) => {
                println!("\nInstalled OpenCode agent definitions:");
                for path in &created {
                    println!("- {}", path.display());
                }
            }
            Err(err) => {
                println!("\nWarning: could not install OpenCode agents: {err}");
            }
        }
    }

    // Re-evaluate any checks that setup may have fixed (e.g. oc-agents)
    // so the final verdict reflects the post-install state.
    recheck_after_install(repo_root.as_deref(), &mut result);

    if result.all_passed {
        println!("\nAll checks passed.");
        Ok(())
    } else {
        anyhow::bail!("setup checks failed")
    }
}

/// Validate runtime dependencies and repo context.
pub fn validate_environment(config: &Config) -> Result<SetupResult> {
    validate_environment_with_repo(config, detect_repo_root())
}

fn validate_environment_with_repo(
    config: &Config,
    repo_root: Option<PathBuf>,
) -> Result<SetupResult> {
    let mut checks = vec![
        check_binary(
            &config.paths.zellij_binary,
            "zellij",
            "creating/switching orchestration sessions and panes",
            "Install zellij (for example: cargo install zellij)",
        ),
        check_binary(
            &config.agent.opencode_binary,
            "opencode",
            "spawning manager/worker agents",
            "Install OpenCode and ensure its launcher binary is on PATH",
        ),
        check_binary(
            &config.paths.spec_kitty_binary,
            "spec-kitty",
            "feature/task lifecycle commands",
            "Install spec-kitty and ensure `spec-kitty` is on PATH",
        ),
        check_pane_tracker(),
    ];

    if let Some(root) = repo_root.as_deref() {
        checks.push(check_opencode_agents(root));
    }

    checks.push(check_git(repo_root.as_deref()));
    checks.push(check_config_file(repo_root.as_deref()));

    let all_passed = checks.iter().all(|c| c.status != CheckStatus::Fail);

    Ok(SetupResult { checks, all_passed })
}

fn check_binary(binary: &str, name: &str, required_for: &str, guidance: &str) -> CheckResult {
    match which::which(binary) {
        Ok(path) => CheckResult {
            name: name.to_string(),
            required_for: required_for.to_string(),
            description: format_binary_description(&path),
            status: CheckStatus::Pass,
            guidance: None,
        },
        Err(_) => CheckResult {
            name: name.to_string(),
            required_for: required_for.to_string(),
            description: format!("{} not found in PATH", binary),
            status: CheckStatus::Fail,
            guidance: Some(guidance.to_string()),
        },
    }
}

fn check_pane_tracker() -> CheckResult {
    let required_for = "pane metadata tracking for agent coordination";
    let plugin_dir = zellij_plugin_dir();
    let plugin_path = plugin_dir.join("zellij-pane-tracker.wasm");

    if !plugin_path.is_file() {
        return CheckResult {
            name: "pane-tracker".to_string(),
            required_for: required_for.to_string(),
            description: "zellij-pane-tracker.wasm not found".to_string(),
            status: CheckStatus::Fail,
            guidance: Some(format!(
                "Install the Zellij pane-tracker plugin:\n\
                 \x20      git clone https://github.com/theslyprofessor/zellij-pane-tracker\n\
                 \x20      cd zellij-pane-tracker && rustup target add wasm32-wasip1 && cargo build --release\n\
                 \x20      mkdir -p {dir} && cp target/wasm32-wasip1/release/zellij-pane-tracker.wasm {dir}/",
                dir = plugin_dir.display()
            )),
        };
    }

    // Plugin file exists. Check if Zellij config loads it.
    if !zellij_config_loads_pane_tracker() {
        return CheckResult {
            name: "pane-tracker".to_string(),
            required_for: required_for.to_string(),
            description: format!("{} (not loaded in zellij config)", plugin_path.display()),
            status: CheckStatus::Warn,
            guidance: Some(
                "Add to load_plugins {{ }} in ~/.config/zellij/config.kdl:\n\
                 \x20      \"file:~/.config/zellij/plugins/zellij-pane-tracker.wasm\""
                    .to_string(),
            ),
        };
    }

    CheckResult {
        name: "pane-tracker".to_string(),
        required_for: required_for.to_string(),
        description: plugin_path.display().to_string(),
        status: CheckStatus::Pass,
        guidance: None,
    }
}

fn zellij_config_dir() -> PathBuf {
    if let Ok(dir) = std::env::var("ZELLIJ_CONFIG_DIR") {
        return PathBuf::from(dir);
    }
    if let Ok(home) = std::env::var("HOME") {
        return PathBuf::from(home).join(".config/zellij");
    }
    PathBuf::from(".config/zellij")
}

fn zellij_plugin_dir() -> PathBuf {
    zellij_config_dir().join("plugins")
}

fn zellij_config_loads_pane_tracker() -> bool {
    let config_path = zellij_config_dir().join("config.kdl");
    std::fs::read_to_string(config_path)
        .map(|content| content.contains("zellij-pane-tracker"))
        .unwrap_or(false)
}

fn check_git(repo_root: Option<&Path>) -> CheckResult {
    let required_for = "repository inspection and worktree management";

    let git_path = match which::which("git") {
        Ok(path) => path,
        Err(_) => {
            return CheckResult {
                name: "git".to_string(),
                required_for: required_for.to_string(),
                description: "git not found in PATH".to_string(),
                status: CheckStatus::Fail,
                guidance: Some("Install git and ensure `git` is on PATH".to_string()),
            };
        }
    };

    match repo_root {
        Some(repo_root) => CheckResult {
            name: "git".to_string(),
            required_for: required_for.to_string(),
            description: format!(
                "{} (in git repo {})",
                format_binary_description(&git_path),
                repo_root.display()
            ),
            status: CheckStatus::Pass,
            guidance: None,
        },
        None => CheckResult {
            name: "git".to_string(),
            required_for: required_for.to_string(),
            description: "not inside a git repository".to_string(),
            status: CheckStatus::Fail,
            guidance: Some(
                "Run `kasmos setup` from a repository containing a .git directory".to_string(),
            ),
        },
    }
}

fn check_config_file(repo_root: Option<&Path>) -> CheckResult {
    let required_for = "loading project defaults and local overrides";

    let Some(repo_root) = repo_root else {
        return CheckResult {
            name: "config".to_string(),
            required_for: required_for.to_string(),
            description: "unable to resolve repo root for kasmos.toml".to_string(),
            status: CheckStatus::Warn,
            guidance: Some("Run from the repository root to generate kasmos.toml".to_string()),
        };
    };

    let path = repo_root.join("kasmos.toml");
    if path.is_file() {
        CheckResult {
            name: "config".to_string(),
            required_for: required_for.to_string(),
            description: format!("{}", path.display()),
            status: CheckStatus::Pass,
            guidance: None,
        }
    } else {
        CheckResult {
            name: "config".to_string(),
            required_for: required_for.to_string(),
            description: "kasmos.toml not found (using defaults)".to_string(),
            status: CheckStatus::Warn,
            guidance: Some("Run `kasmos setup` to generate a baseline kasmos.toml".to_string()),
        }
    }
}

fn print_results(result: &SetupResult) {
    let colorize = should_colorize();

    for check in &result.checks {
        let label = status_label(check.status, colorize);

        println!("{} {:<14} {}", label, check.name, check.description);
        if let Some(guidance) = &check.guidance
            && check.status == CheckStatus::Fail
        {
            println!("       guidance: {}", guidance);
        }
    }
}

fn should_colorize() -> bool {
    std::io::stdout().is_terminal() && std::env::var_os("NO_COLOR").is_none()
}

fn status_label(status: CheckStatus, colorize: bool) -> &'static str {
    if colorize {
        match status {
            CheckStatus::Pass => STATUS_PASS_COLOR,
            CheckStatus::Fail => STATUS_FAIL_COLOR,
            CheckStatus::Warn => STATUS_WARN_COLOR,
        }
    } else {
        match status {
            CheckStatus::Pass => STATUS_PASS,
            CheckStatus::Fail => STATUS_FAIL,
            CheckStatus::Warn => STATUS_WARN,
        }
    }
}

fn detect_repo_root() -> Option<PathBuf> {
    let cwd = std::env::current_dir().ok()?;
    crate::git::find_repo_root(&cwd).ok()
}

fn ensure_baseline_assets(repo_root: &Path, config: &Config) -> Result<Vec<PathBuf>> {
    let mut created = Vec::new();

    let config_path = repo_root.join("kasmos.toml");
    if !config_path.exists() {
        let toml = toml::to_string_pretty(config).context("Failed to serialize default config")?;
        std::fs::write(&config_path, toml)
            .with_context(|| format!("Failed to write {}", config_path.display()))?;
        created.push(config_path);
    }

    let profile_root = repo_root.join("config/profiles/kasmos");
    let agent_root = profile_root.join("agent");
    std::fs::create_dir_all(&agent_root)
        .with_context(|| format!("Failed to create {}", agent_root.display()))?;

    write_if_missing(
        &profile_root.join("opencode.jsonc"),
        default_opencode_profile(),
        &mut created,
    )?;
    write_if_missing(
        &agent_root.join("manager.md"),
        default_agent_prompt("manager"),
        &mut created,
    )?;
    write_if_missing(
        &agent_root.join("planner.md"),
        default_agent_prompt("planner"),
        &mut created,
    )?;
    write_if_missing(
        &agent_root.join("coder.md"),
        default_agent_prompt("coder"),
        &mut created,
    )?;
    write_if_missing(
        &agent_root.join("reviewer.md"),
        default_agent_prompt("reviewer"),
        &mut created,
    )?;
    write_if_missing(
        &agent_root.join("release.md"),
        default_agent_prompt("release"),
        &mut created,
    )?;

    Ok(created)
}

fn write_if_missing(path: &Path, contents: String, created: &mut Vec<PathBuf>) -> Result<()> {
    if path.exists() {
        return Ok(());
    }

    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("Failed to create {}", parent.display()))?;
    }

    std::fs::write(path, contents)
        .with_context(|| format!("Failed to write {}", path.display()))?;
    created.push(path.to_path_buf());
    Ok(())
}

fn format_binary_description(path: &Path) -> String {
    match command_version(path) {
        Some(version) => format!("{} ({})", path.display(), version),
        None => path.display().to_string(),
    }
}

fn command_version(path: &Path) -> Option<String> {
    let output = Command::new(path).arg("--version").output().ok()?;
    if !output.status.success() {
        return None;
    }

    let stdout = String::from_utf8_lossy(&output.stdout);
    if let Some(line) = stdout.lines().map(str::trim).find(|line| !line.is_empty()) {
        return Some(line.to_string());
    }

    let stderr = String::from_utf8_lossy(&output.stderr);
    stderr
        .lines()
        .map(str::trim)
        .find(|line| !line.is_empty())
        .map(ToOwned::to_owned)
}

fn default_opencode_profile() -> String {
    r#"{
  // Generated by `kasmos setup`
  "mcpServers": {
    "kasmos": {
      "command": "kasmos",
      "args": ["serve"]
    }
  }
}
"#
    .to_string()
}

fn default_agent_prompt(role: &str) -> String {
    format!(
        "# {} role\n\nGenerated by `kasmos setup`.\nUpdate this prompt to match your team workflow.\n",
        role
    )
}

/// Re-evaluate fixable checks after install steps have run.
///
/// Some checks (like `oc-agents`) may have failed during the initial
/// validation but were subsequently fixed by the install step.  This
/// function re-runs those checks and patches the result so the final
/// pass/fail verdict reflects reality.
fn recheck_after_install(repo_root: Option<&Path>, result: &mut SetupResult) {
    if let Some(root) = repo_root {
        for check in &mut result.checks {
            if check.name == "oc-agents" && check.status != CheckStatus::Pass {
                *check = check_opencode_agents(root);
            }
        }
    }
    result.all_passed = result.checks.iter().all(|c| c.status != CheckStatus::Fail);
}

/// All kasmos agent roles that need opencode agent definitions.
const KASMOS_AGENT_ROLES: &[&str] = &["manager", "planner", "coder", "reviewer", "release"];

/// Check that opencode agent definitions exist for all kasmos roles.
///
/// Looks for `.opencode/agents/<role>.md` in the repo root (per-project agents).
fn check_opencode_agents(repo_root: &Path) -> CheckResult {
    let required_for = "agent spawning (opencode --agent <role>)";
    let agent_dir = repo_root.join(".opencode/agents");

    let missing: Vec<&str> = KASMOS_AGENT_ROLES
        .iter()
        .filter(|role| !agent_dir.join(format!("{}.md", role)).is_file())
        .copied()
        .collect();

    if missing.is_empty() {
        CheckResult {
            name: "oc-agents".to_string(),
            required_for: required_for.to_string(),
            description: format!(
                "all {} roles in .opencode/agents/",
                KASMOS_AGENT_ROLES.len(),
            ),
            status: CheckStatus::Pass,
            guidance: None,
        }
    } else {
        CheckResult {
            name: "oc-agents".to_string(),
            required_for: required_for.to_string(),
            description: format!("missing agents: {}", missing.join(", ")),
            status: CheckStatus::Fail,
            guidance: Some(format!(
                "Run `kasmos setup` to install agent definitions to {}",
                agent_dir.display()
            )),
        }
    }
}

/// Install agent definitions into the repo-level `.opencode/agents/` directory.
///
/// Only writes files that do not already exist, preserving user customisations.
fn install_opencode_agents(repo_root: &Path) -> Result<Vec<PathBuf>> {
    let agent_dir = repo_root.join(".opencode/agents");
    std::fs::create_dir_all(&agent_dir)
        .with_context(|| format!("Failed to create {}", agent_dir.display()))?;

    let mut created = Vec::new();

    for role in KASMOS_AGENT_ROLES {
        write_if_missing(
            &agent_dir.join(format!("{role}.md")),
            default_opencode_agent(role),
            &mut created,
        )?;
    }

    Ok(created)
}

/// Default opencode agent definition for a kasmos role.
///
/// These are the files opencode reads from `profiles/<profile>/agent/<role>.md`
/// to register agent types. They use YAML frontmatter for metadata.
fn default_opencode_agent(role: &str) -> String {
    match role {
        "manager" => r#"---
description: Orchestrator agent that coordinates worker agents through kasmos MCP tools
mode: all
---

# Manager

You are the orchestration manager for kasmos feature development.

## Responsibilities

- Assess feature readiness and determine workflow phase
- Spawn and coordinate planner, coder, reviewer, and release workers
- Use kasmos MCP tools for workflow state and worker lifecycle
- Monitor progress through message-log events
- Make lane transition decisions based on worker outcomes

## Rules

- Always check workflow_status before spawning workers
- Respect the review_rejection_cap for review cycles
- Use structured messages for all worker coordination
- Do not implement code directly - delegate to worker agents
"#
        .to_string(),

        "planner" => r#"---
description: Planning agent for converting requirements into structured plans and work packages
mode: all
---

# Planner

You convert feature specifications into actionable plans and work package decompositions.

## Responsibilities

- Analyze spec.md to understand feature requirements
- Produce architecture-quality plans
- Decompose plans into work packages with clear contracts
- Maintain consistency across spec, plan, and task artifacts

## Rules

- Keep plans actionable for implementers
- Define clear acceptance criteria per work package
- Identify dependencies between work packages
- Prefer explicit assumptions over hidden ambiguity
"#
        .to_string(),

        "coder" => r#"---
description: Implementation agent for writing and modifying code
mode: all
---

# Coder

You are a software engineer focused on implementing robust, correct code.

## Responsibilities

- Implement features and fixes exactly as specified in the prompt
- Follow existing project conventions and patterns
- Write clean, readable code
- Run verification after changes (build, lint, type-check, tests)
- Return clear summaries of changes made

## Rules

- Early exit: guard clauses at function tops, minimal nesting
- Parse don't validate: parse at boundaries, trust internally
- Fail fast: invalid states halt with descriptive errors
- Fix lint/type errors in code you modify
- NEVER commit code or leave debug statements
"#
        .to_string(),

        "reviewer" => r#"---
description: Code reviewer for correctness, security, and quality
mode: all
---

# Reviewer

You are an expert code reviewer providing detailed, actionable feedback.

## Process

1. Identify scope - list all files to review
2. Analyze - correctness, security, performance, style
3. Classify - severity: Critical > Major > Minor > Nitpick
4. Report - structured findings with file:line references

## Rules

- NEVER modify files
- NEVER approve without completing full review
- Be specific with file:line references
- Include positive observations alongside issues
"#
        .to_string(),

        "release" => r#"---
description: Release agent for merge, finalization, and cleanup operations
mode: all
---

# Release

You handle feature integration, merge execution, and release finalization steps.

## Responsibilities

- Execute merge preflight checks and merge strategy steps
- Resolve straightforward merge/process issues safely
- Report merge outcomes, cleanup, and follow-up actions clearly

## Rules

- Prefer safe, reversible git operations unless explicitly told otherwise
- Never skip required validation gates
- Keep execution logs concise and operator-focused
- Escalate when conflicts affect critical interfaces or safety-sensitive paths
"#
        .to_string(),

        _ => format!(
            "---\ndescription: {} agent\nmode: all\n---\n\n# {}\n\nGenerated by `kasmos setup`.\n",
            role, role
        ),
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::launch;
    use std::sync::Mutex;

    static ENV_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn setup_passes_when_dependencies_are_present() {
        let _guard = ENV_LOCK.lock().expect("env lock");
        let old_path = std::env::var("PATH").ok();
        let old_cwd = std::env::current_dir().expect("cwd");
        let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

        let repo = tempfile::tempdir().expect("repo tempdir");
        std::fs::create_dir_all(repo.path().join(".git")).expect("create .git");

        let bin = repo.path().join("bin");
        std::fs::create_dir_all(&bin).expect("create bin");
        create_executable(&bin.join("zellij"));
        create_executable(&bin.join("ocx"));
        create_executable(&bin.join("spec-kitty"));
        create_executable(&bin.join("git"));

        // Create fake Zellij config dir with plugin and config.kdl
        let zellij_config = repo.path().join("zellij-config");
        let plugin_dir = zellij_config.join("plugins");
        std::fs::create_dir_all(&plugin_dir).expect("create plugin dir");
        std::fs::write(plugin_dir.join("zellij-pane-tracker.wasm"), b"fake-wasm")
            .expect("write fake wasm");
        std::fs::write(
            zellij_config.join("config.kdl"),
            "load_plugins {\n    \"file:~/.config/zellij/plugins/zellij-pane-tracker.wasm\"\n}\n",
        )
        .expect("write fake zellij config");

        // Create repo-level .opencode/agents/ with agent definitions.
        let oc_agent_dir = repo.path().join(".opencode/agents");
        std::fs::create_dir_all(&oc_agent_dir).expect("create oc agent dir");
        for role in KASMOS_AGENT_ROLES {
            std::fs::write(oc_agent_dir.join(format!("{role}.md")), "# stub\n")
                .expect("write fake agent");
        }

        unsafe {
            std::env::set_var("PATH", bin.display().to_string());
            std::env::set_var("ZELLIJ_CONFIG_DIR", zellij_config.display().to_string());
            std::env::set_current_dir(repo.path()).expect("set cwd");
        }

        let outcome = std::panic::catch_unwind(|| {
            let config = Config::default();
            let result = validate_environment(&config).expect("validate environment");

            assert!(result.all_passed);
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "zellij" && c.status == CheckStatus::Pass)
            );
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "pane-tracker" && c.status == CheckStatus::Pass)
            );
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "oc-agents" && c.status == CheckStatus::Pass)
            );
        });

        unsafe {
            std::env::set_current_dir(old_cwd).expect("restore cwd");
            if let Some(path) = old_path {
                std::env::set_var("PATH", path);
            } else {
                std::env::remove_var("PATH");
            }
            if let Some(dir) = old_zellij_config {
                std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
            } else {
                std::env::remove_var("ZELLIJ_CONFIG_DIR");
            }
        }

        assert!(outcome.is_ok(), "setup pass test panicked");
    }

    #[test]
    fn setup_fails_when_dependency_is_missing() {
        let _guard = ENV_LOCK.lock().expect("env lock");

        let mut config = Config::default();
        config.paths.zellij_binary = "__missing_zellij__".to_string();

        let result = validate_environment(&config).expect("validate environment");
        assert!(!result.all_passed);
        assert!(result.checks.iter().any(|check| {
            check.name == "zellij" && check.status == CheckStatus::Fail && check.guidance.is_some()
        }));
    }

    #[test]
    fn setup_generates_assets_idempotently() {
        let repo = tempfile::tempdir().expect("repo tempdir");
        let config = Config::default();

        let first = ensure_baseline_assets(repo.path(), &config).expect("first setup run");
        assert!(!first.is_empty());

        let second = ensure_baseline_assets(repo.path(), &config).expect("second setup run");
        assert!(second.is_empty());

        assert!(repo.path().join("kasmos.toml").is_file());
        assert!(
            repo.path()
                .join("config/profiles/kasmos/opencode.jsonc")
                .is_file()
        );
        assert!(
            repo.path()
                .join("config/profiles/kasmos/agent/manager.md")
                .is_file()
        );
        assert!(
            repo.path()
                .join("config/profiles/kasmos/agent/planner.md")
                .is_file()
        );
    }

    #[test]
    fn install_opencode_agents_creates_missing_roles() {
        let repo = tempfile::tempdir().expect("tempdir");
        let agent_dir = repo.path().join(".opencode/agents");

        let created = install_opencode_agents(repo.path()).expect("install agents");
        assert_eq!(created.len(), KASMOS_AGENT_ROLES.len());

        for role in KASMOS_AGENT_ROLES {
            let path = agent_dir.join(format!("{role}.md"));
            assert!(path.is_file(), "missing agent: {role}");

            let content = std::fs::read_to_string(&path).expect("read agent");
            assert!(content.contains("---"), "missing frontmatter in {role}");
        }

        // Second run should be idempotent.
        let second = install_opencode_agents(repo.path()).expect("second install");
        assert!(second.is_empty(), "expected no new files on second run");
    }

    #[test]
    fn check_opencode_agents_reports_missing() {
        let repo = tempfile::tempdir().expect("tempdir");
        let agent_dir = repo.path().join(".opencode/agents");
        std::fs::create_dir_all(&agent_dir).expect("create agent dir");

        // Only install 2 of 5 roles.
        std::fs::write(agent_dir.join("coder.md"), "# stub\n").expect("write coder");
        std::fs::write(agent_dir.join("reviewer.md"), "# stub\n").expect("write reviewer");

        let check = check_opencode_agents(repo.path());

        assert_eq!(check.status, CheckStatus::Fail);
        assert!(check.description.contains("manager"));
        assert!(check.description.contains("planner"));
        assert!(check.description.contains("release"));
        assert!(!check.description.contains("coder"));
        assert!(!check.description.contains("reviewer"));
    }

    #[test]
    fn launch_preflight_uses_setup_validation_engine() {
        let mut config = Config::default();
        config.paths.zellij_binary = "__missing_zellij__".to_string();

        let failures = launch::preflight_checks(&config).expect_err("preflight should fail");
        assert!(failures.iter().any(|f| f.dependency == "zellij"));
    }

    fn create_executable(path: &Path) {
        std::fs::write(path, "#!/bin/sh\nexit 0\n").expect("write executable");
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            let mut perms = std::fs::metadata(path).expect("metadata").permissions();
            perms.set_mode(0o755);
            std::fs::set_permissions(path, perms).expect("set permissions");
        }
    }
}
