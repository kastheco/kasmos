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
    let result = validate_environment_with_repo(&config, repo_root.clone())?;

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

    if result.all_passed {
        println!("\nAll required checks passed.");
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
    let checks = vec![
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
        check_git(repo_root.as_deref()),
        check_config_file(repo_root.as_deref()),
    ];

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
    let required_for = "structured pane message/event tracking";
    let candidates = ["pane-tracker", "zellij-pane-tracker"];

    for candidate in candidates {
        if let Ok(path) = which::which(candidate) {
            return CheckResult {
                name: "pane-tracker".to_string(),
                required_for: required_for.to_string(),
                description: format!("{} ({})", candidate, format_binary_description(&path)),
                status: CheckStatus::Pass,
                guidance: None,
            };
        }
    }

    CheckResult {
        name: "pane-tracker".to_string(),
        required_for: required_for.to_string(),
        description: "pane tracker binary not found".to_string(),
        status: CheckStatus::Fail,
        guidance: Some(
            "Install pane-tracker and expose `pane-tracker` or `zellij-pane-tracker` in PATH"
                .to_string(),
        ),
    }
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

        let repo = tempfile::tempdir().expect("repo tempdir");
        std::fs::create_dir_all(repo.path().join(".git")).expect("create .git");

        let bin = repo.path().join("bin");
        std::fs::create_dir_all(&bin).expect("create bin");
        create_executable(&bin.join("zellij"));
        create_executable(&bin.join("ocx"));
        create_executable(&bin.join("spec-kitty"));
        create_executable(&bin.join("pane-tracker"));
        create_executable(&bin.join("git"));

        unsafe {
            std::env::set_var("PATH", bin.display().to_string());
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
        });

        unsafe {
            std::env::set_current_dir(old_cwd).expect("restore cwd");
            if let Some(path) = old_path {
                std::env::set_var("PATH", path);
            } else {
                std::env::remove_var("PATH");
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
