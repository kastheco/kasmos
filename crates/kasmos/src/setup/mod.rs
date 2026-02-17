//! Environment validation and default config generation.

use crate::config::Config;
use anyhow::{Context, Result};
use dialoguer::{theme::ColorfulTheme, Confirm, FuzzySelect, Input, Select};
use std::collections::BTreeMap;
use std::io::IsTerminal;
use std::path::{Path, PathBuf};
use std::process::Command;

const STATUS_PASS: &str = "[PASS]";
const STATUS_FAIL: &str = "[FAIL]";
const STATUS_WARN: &str = "[WARN]";
const STATUS_PASS_COLOR: &str = "\x1b[32m[PASS]\x1b[0m";
const STATUS_FAIL_COLOR: &str = "\x1b[31m[FAIL]\x1b[0m";
const STATUS_WARN_COLOR: &str = "\x1b[33m[WARN]\x1b[0m";

/// The opencode.jsonc template, embedded at compile time from
/// config/profiles/kasmos/opencode.jsonc.
const OPENCODE_CONFIG_TEMPLATE: &str =
    include_str!("../../../../config/profiles/kasmos/opencode.jsonc");

/// Valid reasoning effort levels for opencode agent configs.
const REASONING_EFFORTS: &[&str] = &["low", "medium", "high", "max"];

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
    println!("kasmos setup");

    let mut zellij_config_warning = None;
    let zellij_config_update = match ensure_pane_tracker_loaded_in_zellij_config() {
        Ok(update) => update,
        Err(err) => {
            zellij_config_warning = Some(err.to_string());
            None
        }
    };

    let mut result = validate_environment_with_repo(&config, repo_root.clone())?;
    print_results(&result);

    if let Some(path) = zellij_config_update {
        println!("\nUpdated Zellij config:\n- {}", path.display());
    }
    if let Some(warning) = zellij_config_warning {
        println!("\nWarning: could not update Zellij config: {warning}");
    }

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

    // Install opencode config into repo-level .opencode/opencode.jsonc.
    if let Some(repo_root) = repo_root.as_deref() {
        match install_opencode_config(repo_root, &config.agent.opencode_binary, &config) {
            Ok(Some(path)) => {
                println!("\nInstalled OpenCode config:\n- {}", path.display());
            }
            Ok(None) => {
                println!("\nOpenCode config: already installed.");
            }
            Err(err) => {
                println!("\nWarning: could not install OpenCode config: {err}");
            }
        }
    }

    // Install agent definitions and project-level MCP config into .opencode/.
    if let Some(repo_root) = repo_root.as_deref() {
        match install_opencode_project_config(repo_root) {
            Ok(created) if created.is_empty() => {
                println!("\nOpenCode project config: already installed.");
            }
            Ok(created) => {
                println!("\nInstalled OpenCode project config:");
                for path in &created {
                    println!("- {}", path.display());
                }
            }
            Err(err) => {
                println!("\nWarning: could not install OpenCode project config: {err}");
            }
        }

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

    // Install spec-kitty command definitions into repo-level .opencode/commands/.
    if let Some(repo_root) = repo_root.as_deref() {
        match install_opencode_commands(repo_root) {
            Ok(created) if created.is_empty() => {
                println!("\nOpenCode commands: all spec-kitty commands already installed.");
            }
            Ok(created) => {
                println!("\nInstalled OpenCode command definitions:");
                for path in &created {
                    println!("- {}", path.display());
                }
            }
            Err(err) => {
                println!("\nWarning: could not install OpenCode commands: {err}");
            }
        }
    }

    // Re-evaluate any checks that setup may have fixed (e.g. oc-config, oc-agents, oc-commands)
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
        check_pane_tracker(config),
        check_zjstatus(),
    ];

    if let Some(root) = repo_root.as_deref() {
        checks.push(check_opencode_config(root));
        checks.push(check_opencode_agents(root));
        checks.push(check_opencode_commands(root));
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

/// Auto-detect the zellij-pane-tracker installation directory.
///
/// Searches common locations for a directory containing `mcp-server/index.ts`.
/// Returns the first match or falls back to the config default.
fn detect_pane_tracker_dir(config: &Config) -> String {
    let candidates = [
        Some(config.paths.pane_tracker_dir.clone()),
        Some("/opt/zellij-pane-tracker".to_string()),
        std::env::var("HOME")
            .ok()
            .map(|h| format!("{h}/zellij-pane-tracker")),
        std::env::var("HOME")
            .ok()
            .map(|h| format!("{h}/.local/share/zellij-pane-tracker")),
        std::env::var("HOME")
            .ok()
            .map(|h| format!("{h}/src/zellij-pane-tracker")),
    ];

    for candidate in candidates.into_iter().flatten() {
        let mcp_script = PathBuf::from(&candidate).join("mcp-server/index.ts");
        if mcp_script.is_file() {
            return candidate;
        }
    }

    config.paths.pane_tracker_dir.clone()
}

fn check_pane_tracker(config: &Config) -> CheckResult {
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

    // After WASM check passes, also validate MCP server exists at configured path
    let detected_dir = detect_pane_tracker_dir(config);
    let mcp_script = PathBuf::from(&detected_dir).join("mcp-server/index.ts");
    if !mcp_script.is_file() {
        return CheckResult {
            name: "pane-tracker".to_string(),
            required_for: required_for.to_string(),
            description: format!(
                "{} (MCP server not found at {})",
                plugin_path.display(),
                mcp_script.display()
            ),
            status: CheckStatus::Warn,
            guidance: Some(format!(
                "Set [paths].pane_tracker_dir in kasmos.toml or run `kasmos setup` to configure.\n\
                 \x20      Expected: {}/mcp-server/index.ts",
                detected_dir
            )),
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

fn check_zjstatus() -> CheckResult {
    let required_for = "status bar in generated Zellij layouts";
    let plugin_dir = zellij_plugin_dir();
    let plugin_path = plugin_dir.join("zjstatus.wasm");

    if !plugin_path.is_file() {
        return CheckResult {
            name: "zjstatus".to_string(),
            required_for: required_for.to_string(),
            description: "zjstatus.wasm not found".to_string(),
            status: CheckStatus::Fail,
            guidance: Some(format!(
                "Install the zjstatus plugin:\n\
                 \x20      Download from https://github.com/dj95/zjstatus/releases\n\
                 \x20      mkdir -p {dir} && cp zjstatus.wasm {dir}/",
                dir = plugin_dir.display()
            )),
        };
    }

    CheckResult {
        name: "zjstatus".to_string(),
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

fn ensure_pane_tracker_loaded_in_zellij_config() -> Result<Option<PathBuf>> {
    let plugin_path = zellij_plugin_dir().join("zellij-pane-tracker.wasm");
    if !plugin_path.is_file() {
        return Ok(None);
    }

    let plugin_url = format!("file:{}", plugin_path.display());
    let config_path = zellij_config_dir().join("config.kdl");

    if !config_path.exists() {
        if let Some(parent) = config_path.parent() {
            std::fs::create_dir_all(parent)
                .with_context(|| format!("Failed to create {}", parent.display()))?;
        }
        let contents = format!("load_plugins {{\n    \"{}\"\n}}\n", plugin_url);
        std::fs::write(&config_path, contents)
            .with_context(|| format!("Failed to write {}", config_path.display()))?;
        return Ok(Some(config_path));
    }

    let content = std::fs::read_to_string(&config_path)
        .with_context(|| format!("Failed to read {}", config_path.display()))?;
    if content.contains("zellij-pane-tracker") {
        return Ok(None);
    }

    let updated = insert_pane_tracker_into_load_plugins(&content, &plugin_url).unwrap_or_else(|| {
        let mut appended = String::with_capacity(content.len() + plugin_url.len() + 40);
        appended.push_str(&content);
        if !appended.ends_with('\n') {
            appended.push('\n');
        }
        appended.push_str("load_plugins {\n    \"");
        appended.push_str(&plugin_url);
        appended.push_str("\"\n}\n");
        appended
    });

    if updated == content {
        return Ok(None);
    }

    std::fs::write(&config_path, updated)
        .with_context(|| format!("Failed to write {}", config_path.display()))?;
    Ok(Some(config_path))
}

fn insert_pane_tracker_into_load_plugins(content: &str, plugin_url: &str) -> Option<String> {
    let load_index = content.find("load_plugins")?;
    let brace_index = content[load_index..].find('{')? + load_index;

    let mut depth = 0;
    let mut end_index = None;
    for (offset, ch) in content[brace_index..].char_indices() {
        match ch {
            '{' => depth += 1,
            '}' => {
                depth -= 1;
                if depth == 0 {
                    end_index = Some(brace_index + offset);
                    break;
                }
            }
            _ => {}
        }
    }

    let end_index = end_index?;
    let block = &content[brace_index + 1..end_index];
    let indent = block
        .lines()
        .filter_map(|line| {
            if line.trim().is_empty() {
                None
            } else {
                Some(
                    line.chars()
                        .take_while(|c| c.is_whitespace())
                        .collect::<String>(),
                )
            }
        })
        .next()
        .unwrap_or_else(|| "    ".to_string());

    let mut updated = String::with_capacity(content.len() + plugin_url.len() + indent.len() + 8);
    updated.push_str(&content[..end_index]);
    if !content[..end_index].ends_with('\n') {
        updated.push('\n');
    }
    updated.push_str(&indent);
    updated.push('"');
    updated.push_str(plugin_url);
    updated.push('"');
    updated.push('\n');
    updated.push_str(&content[end_index..]);
    Some(updated)
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
    find_repo_root(&cwd).ok()
}

/// Find the git repository root by walking up from the given path.
fn find_repo_root(start: &Path) -> anyhow::Result<PathBuf> {
    let mut current = start.to_path_buf();
    if current.exists() {
        current = current.canonicalize().unwrap_or_else(|_| current.clone());
    }
    loop {
        if current.join(".git").exists() {
            return Ok(current);
        }
        if !current.pop() {
            anyhow::bail!("No git repository found at or above {}", start.display());
        }
    }
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

/// Discover available models by running `opencode models`.
///
/// Returns a list of model IDs. Falls back to an empty vec on failure.
fn discover_models(opencode_binary: &str) -> Vec<String> {
    let output = Command::new(opencode_binary).arg("models").output();

    match output {
        Ok(out) if out.status.success() => String::from_utf8_lossy(&out.stdout)
            .lines()
            .map(str::trim)
            .filter(|line| !line.is_empty())
            .map(str::to_string)
            .collect(),
        _ => Vec::new(),
    }
}

/// Extract per-role (model, reasoningEffort) defaults from parsed template.
fn extract_template_defaults(config: &serde_json::Value) -> BTreeMap<String, (String, String)> {
    let mut defaults = BTreeMap::new();

    let Some(agents) = config.get("agent").and_then(|a| a.as_object()) else {
        return defaults;
    };

    for (role, cfg) in agents {
        let model = cfg
            .get("model")
            .and_then(|v| v.as_str())
            .unwrap_or("anthropic/claude-opus-4-6")
            .to_string();
        let reasoning = cfg
            .get("reasoningEffort")
            .and_then(|v| v.as_str())
            .unwrap_or("medium")
            .to_string();
        defaults.insert(role.clone(), (model, reasoning));
    }

    defaults
}

/// Apply model/reasoning selections and fix external_directory paths.
fn apply_selections_and_fixup(
    config: &mut serde_json::Value,
    selections: &BTreeMap<String, (String, String)>,
    repo_root: &Path,
    pane_tracker_dir: &str,
) {
    if let Some(agents) = config.get_mut("agent").and_then(|a| a.as_object_mut()) {
        for (role, (model, reasoning)) in selections {
            let Some(agent_cfg) = agents.get_mut(role).and_then(|v| v.as_object_mut()) else {
                continue;
            };

            agent_cfg.insert("model".to_string(), serde_json::Value::String(model.clone()));
            agent_cfg.insert(
                "reasoningEffort".to_string(),
                serde_json::Value::String(reasoning.clone()),
            );
        }
    }

    let repo_root = repo_root.display().to_string();
    if let Some(agents) = config.get_mut("agent").and_then(|a| a.as_object_mut()) {
        for (_role, agent_cfg) in agents.iter_mut() {
            fixup_external_directory(agent_cfg, &repo_root);
        }
    }

    fixup_mcp_pane_tracker_path(config, pane_tracker_dir);
}

/// Replace `/opt/zellij-pane-tracker` in mcp.zellij.command with the actual install path.
fn fixup_mcp_pane_tracker_path(config: &mut serde_json::Value, pane_tracker_dir: &str) {
    let command = config
        .get_mut("mcp")
        .and_then(|m| m.get_mut("zellij"))
        .and_then(|z| z.get_mut("command"))
        .and_then(|c| c.as_array_mut());

    let Some(command) = command else {
        return;
    };

    for elem in command.iter_mut() {
        if let Some(s) = elem.as_str()
            && s.contains("/opt/zellij-pane-tracker")
        {
            *elem = serde_json::Value::String(
                s.replace("/opt/zellij-pane-tracker", pane_tracker_dir),
            );
        }
    }
}

/// Replace `~/dev/kasmos` path keys with actual repo root in
/// permission.external_directory.
fn fixup_external_directory(agent_cfg: &mut serde_json::Value, repo_root: &str) {
    let ext_dir = agent_cfg
        .get_mut("permission")
        .and_then(|p| p.get_mut("external_directory"))
        .and_then(|e| e.as_object_mut());

    let Some(ext_dir) = ext_dir else {
        return;
    };

    let old_keys: Vec<String> = ext_dir
        .keys()
        .filter(|k| k.contains("~/dev/kasmos"))
        .cloned()
        .collect();

    for old_key in old_keys {
        let Some(value) = ext_dir.remove(&old_key) else {
            continue;
        };
        let new_key = old_key.replace("~/dev/kasmos", repo_root);
        ext_dir.insert(new_key, value);
    }
}

/// Interactively build opencode config from the embedded template.
fn interactive_opencode_config(
    repo_root: &Path,
    opencode_binary: &str,
    kasmos_config: &Config,
) -> Result<String> {
    let mut config: serde_json::Value = json5::from_str(OPENCODE_CONFIG_TEMPLATE)
        .context("Failed to parse embedded opencode.jsonc template")?;

    let models = discover_models(opencode_binary);
    let defaults = extract_template_defaults(&config);

    println!("\nAgent role defaults (from template):");
    for role in KASMOS_AGENT_ROLES {
        if let Some((model, reasoning)) = defaults.get(*role) {
            println!("  {:<12} {:<36} reasoning: {}", role, model, reasoning);
        }
    }
    println!();

    let customize = Confirm::with_theme(&ColorfulTheme::default())
        .with_prompt("Customize per role?")
        .default(false)
        .interact()
        .context("Interactive prompt cancelled")?;

    let selections = if customize {
        let mut selections = BTreeMap::new();

        for role in KASMOS_AGENT_ROLES {
            let (default_model, default_reasoning) = defaults.get(*role).cloned().unwrap_or_else(|| {
                (
                    "anthropic/claude-opus-4-6".to_string(),
                    "medium".to_string(),
                )
            });

            let model = if models.is_empty() {
                Input::<String>::with_theme(&ColorfulTheme::default())
                    .with_prompt(format!("  {} model", role))
                    .default(default_model)
                    .interact_text()
                    .context("Interactive prompt cancelled")?
            } else {
                let default_index = models.iter().position(|m| m == &default_model).unwrap_or(0);
                let index = FuzzySelect::with_theme(&ColorfulTheme::default())
                    .with_prompt(format!("  {} model", role))
                    .items(&models)
                    .default(default_index)
                    .interact()
                    .context("Interactive prompt cancelled")?;
                models[index].clone()
            };

            let reasoning_index = REASONING_EFFORTS
                .iter()
                .position(|r| *r == default_reasoning)
                .unwrap_or(1);
            let selected_reasoning = Select::with_theme(&ColorfulTheme::default())
                .with_prompt(format!("  {} reasoning effort", role))
                .items(REASONING_EFFORTS)
                .default(reasoning_index)
                .interact()
                .context("Interactive prompt cancelled")?;

            selections.insert(
                role.to_string(),
                (model, REASONING_EFFORTS[selected_reasoning].to_string()),
            );
        }

        selections
    } else {
        defaults
    };

    // Detect and prompt for pane-tracker installation directory
    let detected_dir = detect_pane_tracker_dir(kasmos_config);
    let pane_tracker_dir: String = Input::with_theme(&ColorfulTheme::default())
        .with_prompt("zellij-pane-tracker install directory")
        .default(detected_dir)
        .validate_with(|input: &String| -> Result<(), String> {
            let script = PathBuf::from(input).join("mcp-server/index.ts");
            if script.is_file() {
                Ok(())
            } else {
                Err(format!(
                    "mcp-server/index.ts not found at {}/mcp-server/index.ts",
                    input
                ))
            }
        })
        .interact_text()
        .context("Interactive prompt cancelled")?;

    apply_selections_and_fixup(&mut config, &selections, repo_root, &pane_tracker_dir);

    serde_json::to_string_pretty(&config).context("Failed to serialize opencode config")
}

/// Install opencode config into `.opencode/opencode.jsonc`.
///
/// Returns Some(path) when a file is written, None when skipped.
fn install_opencode_config(
    repo_root: &Path,
    opencode_binary: &str,
    kasmos_config: &Config,
) -> Result<Option<PathBuf>> {
    let config_path = repo_root.join(".opencode/opencode.jsonc");

    if config_path.exists() {
        if !std::io::stdin().is_terminal() {
            return Ok(None);
        }

        let reconfigure = Confirm::with_theme(&ColorfulTheme::default())
            .with_prompt("OpenCode config already exists. Reconfigure?")
            .default(false)
            .interact()
            .unwrap_or(false);

        if !reconfigure {
            return Ok(None);
        }
    }

    if !std::io::stdin().is_terminal() {
        println!("  (non-interactive: using template defaults)");
        let mut config: serde_json::Value = json5::from_str(OPENCODE_CONFIG_TEMPLATE)
            .context("Failed to parse embedded opencode.jsonc template")?;
        let defaults = extract_template_defaults(&config);
        let pane_tracker_dir = detect_pane_tracker_dir(kasmos_config);
        apply_selections_and_fixup(&mut config, &defaults, repo_root, &pane_tracker_dir);

        let contents = serde_json::to_string_pretty(&config)
            .context("Failed to serialize opencode config")?;

        let config_path = repo_root.join(".opencode/opencode.jsonc");
        if let Some(parent) = config_path.parent() {
            std::fs::create_dir_all(parent)
                .with_context(|| format!("Failed to create {}", parent.display()))?;
        }
        std::fs::write(&config_path, contents)
            .with_context(|| format!("Failed to write {}", config_path.display()))?;

        return Ok(Some(config_path));
    }

    let contents = interactive_opencode_config(repo_root, opencode_binary, kasmos_config)?;

    if let Some(parent) = config_path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("Failed to create {}", parent.display()))?;
    }

    std::fs::write(&config_path, contents)
        .with_context(|| format!("Failed to write {}", config_path.display()))?;

    Ok(Some(config_path))
}

/// Install opencode config with explicit per-role model/reasoning selections.
#[cfg(test)]
fn install_opencode_config_with_values(
    repo_root: &Path,
    selections: &BTreeMap<String, (String, String)>,
    kasmos_config: &Config,
) -> Result<PathBuf> {
    let mut config: serde_json::Value = json5::from_str(OPENCODE_CONFIG_TEMPLATE)
        .context("Failed to parse embedded opencode.jsonc template")?;

    let pane_tracker_dir = detect_pane_tracker_dir(kasmos_config);
    apply_selections_and_fixup(&mut config, selections, repo_root, &pane_tracker_dir);

    let contents = serde_json::to_string_pretty(&config)
        .context("Failed to serialize opencode config")?;

    let config_path = repo_root.join(".opencode/opencode.jsonc");
    if let Some(parent) = config_path.parent() {
        std::fs::create_dir_all(parent)
            .with_context(|| format!("Failed to create {}", parent.display()))?;
    }
    std::fs::write(&config_path, contents)
        .with_context(|| format!("Failed to write {}", config_path.display()))?;

    Ok(config_path)
}

fn default_opencode_profile() -> String {
    OPENCODE_CONFIG_TEMPLATE.to_string()
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
            match check.name.as_str() {
                "oc-config" if check.status != CheckStatus::Pass => {
                    *check = check_opencode_config(root);
                }
                "oc-agents" if check.status != CheckStatus::Pass => {
                    *check = check_opencode_agents(root);
                }
                "oc-commands" if check.status != CheckStatus::Pass => {
                    *check = check_opencode_commands(root);
                }
                _ => {}
            }
        }
    }
    result.all_passed = result.checks.iter().all(|c| c.status != CheckStatus::Fail);
}

/// Check that .opencode/opencode.jsonc exists and contains kasmos MCP config.
fn check_opencode_config(repo_root: &Path) -> CheckResult {
    let required_for = "MCP server config for kasmos orchestration tools";
    let config_path = repo_root.join(".opencode/opencode.jsonc");

    if !config_path.is_file() {
        return CheckResult {
            name: "oc-config".to_string(),
            required_for: required_for.to_string(),
            description: ".opencode/opencode.jsonc not found".to_string(),
            status: CheckStatus::Fail,
            guidance: Some(
                "Run `kasmos setup` to install project-level OpenCode config".to_string(),
            ),
        };
    }

    // Quick sanity check: the file should reference "kasmos" MCP server.
    let content = std::fs::read_to_string(&config_path).unwrap_or_default();
    if !content.contains("\"kasmos\"") {
        return CheckResult {
            name: "oc-config".to_string(),
            required_for: required_for.to_string(),
            description: ".opencode/opencode.jsonc missing kasmos MCP server".to_string(),
            status: CheckStatus::Warn,
            guidance: Some(
                "Delete .opencode/opencode.jsonc and re-run `kasmos setup` to regenerate"
                    .to_string(),
            ),
        };
    }

    CheckResult {
        name: "oc-config".to_string(),
        required_for: required_for.to_string(),
        description: format!("{}", config_path.display()),
        status: CheckStatus::Pass,
        guidance: None,
    }
}

/// Install project-level OpenCode config (.opencode/opencode.jsonc) with MCP servers.
fn install_opencode_project_config(repo_root: &Path) -> Result<Vec<PathBuf>> {
    let oc_dir = repo_root.join(".opencode");
    std::fs::create_dir_all(&oc_dir)
        .with_context(|| format!("Failed to create {}", oc_dir.display()))?;

    let mut created = Vec::new();
    write_if_missing(
        &oc_dir.join("opencode.jsonc"),
        default_opencode_project_config(),
        &mut created,
    )?;
    Ok(created)
}

/// Default project-level OpenCode config with kasmos MCP servers.
fn default_opencode_project_config() -> String {
    r#"{
  // Project-level OpenCode config for kasmos.
  // Installed by `kasmos setup`. Provides MCP servers for orchestration.
  // Merges with global config (~/.config/opencode/opencode.json) and
  // any active profile (OPENCODE_CONFIG_DIR).
  "mcp": {
    "kasmos": {
      "type": "local",
      "command": [
        "kasmos",
        "serve"
      ],
      "enabled": true
    },
    "zellij": {
      "type": "local",
      "command": [
        "bun",
        "run",
        "/opt/zellij-pane-tracker/mcp-server/index.ts"
      ],
      "enabled": true
    }
  }
}
"#
    .to_string()
}

/// All kasmos agent roles that need opencode agent definitions.
const KASMOS_AGENT_ROLES: &[&str] = &["manager", "planner", "coder", "reviewer", "release"];

/// All spec-kitty slash commands that need opencode command definitions.
const SPEC_KITTY_COMMANDS: &[&str] = &[
    "spec-kitty.accept",
    "spec-kitty.analyze",
    "spec-kitty.checklist",
    "spec-kitty.clarify",
    "spec-kitty.constitution",
    "spec-kitty.dashboard",
    "spec-kitty.implement",
    "spec-kitty.merge",
    "spec-kitty.plan",
    "spec-kitty.research",
    "spec-kitty.review",
    "spec-kitty.specify",
    "spec-kitty.status",
    "spec-kitty.tasks",
];

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

/// Install spec-kitty command definitions into `.opencode/commands/`.
///
/// Copies from the profile source at `config/profiles/kasmos/commands/` into
/// the repo-level `.opencode/commands/` directory. Only writes files that do
/// not already exist, preserving user customisations.
fn install_opencode_commands(repo_root: &Path) -> Result<Vec<PathBuf>> {
    let source_dir = repo_root.join("config/profiles/kasmos/commands");
    let target_dir = repo_root.join(".opencode/commands");
    std::fs::create_dir_all(&target_dir)
        .with_context(|| format!("Failed to create {}", target_dir.display()))?;

    let mut created = Vec::new();

    for cmd in SPEC_KITTY_COMMANDS {
        let filename = format!("{cmd}.md");
        let source = source_dir.join(&filename);
        let target = target_dir.join(&filename);

        if target.exists() {
            continue;
        }

        if source.is_file() {
            let content = std::fs::read_to_string(&source)
                .with_context(|| format!("Failed to read {}", source.display()))?;
            std::fs::write(&target, content)
                .with_context(|| format!("Failed to write {}", target.display()))?;
            created.push(target);
        }
    }

    Ok(created)
}

/// Check that spec-kitty slash-command definitions exist in `.opencode/commands/`.
fn check_opencode_commands(repo_root: &Path) -> CheckResult {
    let required_for = "spec-kitty slash commands (/spec-kitty.specify, etc.)";
    let cmd_dir = repo_root.join(".opencode/commands");

    let missing: Vec<&str> = SPEC_KITTY_COMMANDS
        .iter()
        .filter(|cmd| !cmd_dir.join(format!("{}.md", cmd)).is_file())
        .copied()
        .collect();

    if missing.is_empty() {
        CheckResult {
            name: "oc-commands".to_string(),
            required_for: required_for.to_string(),
            description: format!(
                "all {} spec-kitty commands in .opencode/commands/",
                SPEC_KITTY_COMMANDS.len(),
            ),
            status: CheckStatus::Pass,
            guidance: None,
        }
    } else {
        CheckResult {
            name: "oc-commands".to_string(),
            required_for: required_for.to_string(),
            description: format!("missing commands: {}", missing.join(", ")),
            status: CheckStatus::Warn,
            guidance: Some(format!(
                "Run `kasmos setup` to install command definitions to {}",
                cmd_dir.display()
            )),
        }
    }
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
        create_executable(&bin.join("opencode"));
        create_executable(&bin.join("spec-kitty"));
        create_executable(&bin.join("git"));

        // Create fake Zellij config dir with plugin and config.kdl
        let zellij_config = repo.path().join("zellij-config");
        let plugin_dir = zellij_config.join("plugins");
        std::fs::create_dir_all(&plugin_dir).expect("create plugin dir");
        std::fs::write(plugin_dir.join("zellij-pane-tracker.wasm"), b"fake-wasm")
            .expect("write fake wasm");
        std::fs::write(plugin_dir.join("zjstatus.wasm"), b"fake-wasm")
            .expect("write fake zjstatus wasm");
        std::fs::write(
            zellij_config.join("config.kdl"),
            "load_plugins {\n    \"file:~/.config/zellij/plugins/zellij-pane-tracker.wasm\"\n}\n",
        )
        .expect("write fake zellij config");

        // Create repo-level .opencode/ with project config and agent definitions.
        let oc_dir = repo.path().join(".opencode");
        let oc_agent_dir = oc_dir.join("agents");
        std::fs::create_dir_all(&oc_agent_dir).expect("create oc agent dir");
        std::fs::write(
            oc_dir.join("opencode.jsonc"),
            r#"{"mcp":{"kasmos":{"type":"local","command":["kasmos","serve"],"enabled":true}}}"#,
        )
        .expect("write fake oc config");
        for role in KASMOS_AGENT_ROLES {
            std::fs::write(oc_agent_dir.join(format!("{role}.md")), "# stub\n")
                .expect("write fake agent");
        }
        // Create repo-level .opencode/commands/ with spec-kitty command stubs.
        let oc_cmd_dir = repo.path().join(".opencode/commands");
        std::fs::create_dir_all(&oc_cmd_dir).expect("create oc commands dir");
        for cmd in SPEC_KITTY_COMMANDS {
            std::fs::write(oc_cmd_dir.join(format!("{cmd}.md")), "# stub\n")
                .expect("write fake command");
        }

        // Create fake pane-tracker MCP server directory
        let pane_tracker_dir = repo.path().join("pane-tracker");
        let mcp_server_dir = pane_tracker_dir.join("mcp-server");
        std::fs::create_dir_all(&mcp_server_dir).expect("create mcp-server dir");
        std::fs::write(mcp_server_dir.join("index.ts"), "// stub")
            .expect("write fake mcp server script");

        unsafe {
            std::env::set_var("PATH", bin.display().to_string());
            std::env::set_var("ZELLIJ_CONFIG_DIR", zellij_config.display().to_string());
            std::env::set_current_dir(repo.path()).expect("set cwd");
        }

        let pane_tracker_dir_str = pane_tracker_dir.display().to_string();
        let outcome = std::panic::catch_unwind(move || {
            let mut config = Config::default();
            config.paths.pane_tracker_dir = pane_tracker_dir_str;
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
                    .any(|c| c.name == "zjstatus" && c.status == CheckStatus::Pass)
            );
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "oc-config" && c.status == CheckStatus::Pass)
            );
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "oc-agents" && c.status == CheckStatus::Pass)
            );
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "oc-config" && c.status == CheckStatus::Pass)
            );
            assert!(
                result
                    .checks
                    .iter()
                    .any(|c| c.name == "oc-commands" && c.status == CheckStatus::Pass)
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
    fn discover_models_parses_output() {
        let raw =
            "anthropic/claude-opus-4-6\nopenai/gpt-5.3-codex\n\n  ollama/qwen3:30b  \n";
        let models: Vec<String> = raw
            .lines()
            .map(str::trim)
            .filter(|l| !l.is_empty())
            .map(String::from)
            .collect();

        assert_eq!(
            models,
            vec![
                "anthropic/claude-opus-4-6",
                "openai/gpt-5.3-codex",
                "ollama/qwen3:30b",
            ]
        );
    }

    #[test]
    fn template_parses_as_json5() {
        let config: serde_json::Value =
            json5::from_str(OPENCODE_CONFIG_TEMPLATE).expect("template should parse as json5");
        assert!(config.get("agent").is_some(), "template must have agent key");
        assert!(config.get("mcp").is_some(), "template must have mcp key");
    }

    #[test]
    fn extract_template_defaults_gets_all_roles() {
        let config: serde_json::Value =
            json5::from_str(OPENCODE_CONFIG_TEMPLATE).expect("parse template");
        let defaults = extract_template_defaults(&config);

        for role in KASMOS_AGENT_ROLES {
            assert!(defaults.contains_key(*role), "missing default for {role}");
        }

        assert_eq!(defaults["coder"].0, "openai/gpt-5.3-codex");
        assert_eq!(defaults["coder"].1, "medium");
        assert_eq!(defaults["planner"].1, "max");
    }

    #[test]
    fn config_path_fixup_replaces_kasmos_paths() {
        let mut config: serde_json::Value =
            json5::from_str(OPENCODE_CONFIG_TEMPLATE).expect("parse template");
        let defaults = extract_template_defaults(&config);

        let repo_root = Path::new("/home/alice/myproject");
        apply_selections_and_fixup(&mut config, &defaults, repo_root, "/opt/zellij-pane-tracker");

        let ext_dir = config["agent"]["coder"]["permission"]["external_directory"]
            .as_object()
            .expect("external_directory should be an object");

        assert!(
            !ext_dir.keys().any(|k| k.contains("~/dev/kasmos")),
            "~/dev/kasmos paths should be replaced"
        );
        assert!(
            ext_dir.keys().any(|k| k.contains("/home/alice/myproject")),
            "repo root paths should be present"
        );
    }

    #[test]
    fn config_with_selections_patches_model() {
        let mut config: serde_json::Value =
            json5::from_str(OPENCODE_CONFIG_TEMPLATE).expect("parse template");
        let mut selections = extract_template_defaults(&config);
        selections.insert(
            "coder".to_string(),
            ("google/gemini-2.5-pro".to_string(), "high".to_string()),
        );

        let repo_root = Path::new("/tmp/test-repo");
        apply_selections_and_fixup(&mut config, &selections, repo_root, "/opt/zellij-pane-tracker");

        assert_eq!(
            config["agent"]["coder"]["model"].as_str().expect("coder model"),
            "google/gemini-2.5-pro"
        );
        assert_eq!(
            config["agent"]["coder"]["reasoningEffort"]
                .as_str()
                .expect("coder reasoning"),
            "high"
        );
        assert_eq!(
            config["agent"]["planner"]["reasoningEffort"]
                .as_str()
                .expect("planner reasoning"),
            "max"
        );
    }

    #[test]
    fn install_opencode_config_creates_file() {
        let repo = tempfile::tempdir().expect("tempdir");
        let config_path = repo.path().join(".opencode/opencode.jsonc");
        assert!(!config_path.exists());

        let kasmos_config = Config::default();
        let result = install_opencode_config(repo.path(), "opencode", &kasmos_config);
        assert!(result.is_ok(), "install should succeed: {result:?}");
        assert!(config_path.exists(), "config file should exist");

        let content = std::fs::read_to_string(&config_path).expect("read config");
        let parsed: serde_json::Value = serde_json::from_str(&content).expect("valid JSON output");
        assert!(parsed.get("agent").is_some());
        assert!(parsed.get("mcp").is_some());
    }

    #[test]
    fn check_opencode_config_pass_and_fail() {
        let repo = tempfile::tempdir().expect("tempdir");

        // Missing file -> Fail
        let missing = check_opencode_config(repo.path());
        assert_eq!(missing.status, CheckStatus::Fail);
        assert_eq!(missing.name, "oc-config");

        // File present but missing kasmos key -> Warn
        let oc_dir = repo.path().join(".opencode");
        std::fs::create_dir_all(&oc_dir).expect("create .opencode dir");
        std::fs::write(oc_dir.join("opencode.jsonc"), "{}")
            .expect("write opencode config");
        let warn = check_opencode_config(repo.path());
        assert_eq!(warn.status, CheckStatus::Warn);

        // File present with kasmos key -> Pass
        std::fs::write(
            oc_dir.join("opencode.jsonc"),
            r#"{"mcp":{"kasmos":{"type":"local","command":["kasmos","serve"]}}}"#,
        )
        .expect("write valid config");
        let present = check_opencode_config(repo.path());
        assert_eq!(present.status, CheckStatus::Pass);
    }

    #[test]
    fn install_opencode_config_with_values_writes_file() {
        let repo = tempfile::tempdir().expect("tempdir");
        let mut selections = BTreeMap::new();
        selections.insert(
            "coder".to_string(),
            (
                "openai/gpt-5.3-codex".to_string(),
                "medium".to_string(),
            ),
        );
        selections.insert(
            "manager".to_string(),
            (
                "anthropic/claude-opus-4-6".to_string(),
                "high".to_string(),
            ),
        );
        selections.insert(
            "planner".to_string(),
            (
                "anthropic/claude-opus-4-6".to_string(),
                "max".to_string(),
            ),
        );
        selections.insert(
            "reviewer".to_string(),
            (
                "anthropic/claude-opus-4-6".to_string(),
                "high".to_string(),
            ),
        );
        selections.insert(
            "release".to_string(),
            (
                "openai/gpt-5.3-codex".to_string(),
                "medium".to_string(),
            ),
        );

        let kasmos_config = Config::default();
        let path = install_opencode_config_with_values(repo.path(), &selections, &kasmos_config)
            .expect("install opencode config with values");
        assert!(path.is_file(), "installed config should be a file");

        let content = std::fs::read_to_string(&path).expect("read installed config");
        let parsed: serde_json::Value = serde_json::from_str(&content).expect("valid JSON");
        assert_eq!(
            parsed["agent"]["planner"]["reasoningEffort"]
                .as_str()
                .expect("planner reasoning"),
            "max"
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
    fn install_opencode_project_config_creates_and_is_idempotent() {
        let repo = tempfile::tempdir().expect("tempdir");

        let created = install_opencode_project_config(repo.path()).expect("install config");
        assert_eq!(created.len(), 1);

        let config_path = repo.path().join(".opencode/opencode.jsonc");
        assert!(config_path.is_file());
        let content = std::fs::read_to_string(&config_path).expect("read config");
        assert!(content.contains("\"kasmos\""));
        assert!(content.contains("\"zellij\""));

        // Second run should be idempotent.
        let second = install_opencode_project_config(repo.path()).expect("second install");
        assert!(second.is_empty());
    }

    #[test]
    fn check_opencode_config_detects_missing_and_present() {
        let repo = tempfile::tempdir().expect("tempdir");

        // Missing: should fail.
        let check = check_opencode_config(repo.path());
        assert_eq!(check.status, CheckStatus::Fail);

        // Present but missing kasmos key: should warn.
        let oc_dir = repo.path().join(".opencode");
        std::fs::create_dir_all(&oc_dir).expect("create .opencode");
        std::fs::write(oc_dir.join("opencode.jsonc"), r#"{"mcp":{}}"#).expect("write empty config");
        let check = check_opencode_config(repo.path());
        assert_eq!(check.status, CheckStatus::Warn);

        // Present with kasmos: should pass.
        std::fs::write(
            oc_dir.join("opencode.jsonc"),
            r#"{"mcp":{"kasmos":{"type":"local","command":["kasmos","serve"]}}}"#,
        )
        .expect("write valid config");
        let check = check_opencode_config(repo.path());
        assert_eq!(check.status, CheckStatus::Pass);
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
    fn install_opencode_commands_copies_from_profile() {
        let repo = tempfile::tempdir().expect("tempdir");
        let cmd_dir = repo.path().join(".opencode/commands");

        // Create profile source files for a subset of commands.
        let source_dir = repo.path().join("config/profiles/kasmos/commands");
        std::fs::create_dir_all(&source_dir).expect("create source dir");
        std::fs::write(
            source_dir.join("spec-kitty.specify.md"),
            "---\ndescription: Create spec\n---\n# specify\n",
        )
        .expect("write source command");
        std::fs::write(
            source_dir.join("spec-kitty.plan.md"),
            "---\ndescription: Create plan\n---\n# plan\n",
        )
        .expect("write source command");

        let created = install_opencode_commands(repo.path()).expect("install commands");
        assert_eq!(created.len(), 2, "should install 2 commands that have sources");

        let specify = cmd_dir.join("spec-kitty.specify.md");
        assert!(specify.is_file(), "specify command should exist");
        let content = std::fs::read_to_string(&specify).expect("read command");
        assert!(content.contains("Create spec"));

        // Second run should be idempotent.
        let second = install_opencode_commands(repo.path()).expect("second install");
        assert!(second.is_empty(), "expected no new files on second run");
    }

    #[test]
    fn check_opencode_commands_reports_missing() {
        let repo = tempfile::tempdir().expect("tempdir");
        let cmd_dir = repo.path().join(".opencode/commands");
        std::fs::create_dir_all(&cmd_dir).expect("create commands dir");

        // Install only 2 commands.
        std::fs::write(cmd_dir.join("spec-kitty.specify.md"), "# stub\n")
            .expect("write specify");
        std::fs::write(cmd_dir.join("spec-kitty.plan.md"), "# stub\n")
            .expect("write plan");

        let check = check_opencode_commands(repo.path());

        assert_eq!(check.status, CheckStatus::Warn);
        assert!(check.description.contains("spec-kitty.tasks"));
        assert!(!check.description.contains("spec-kitty.specify"));
        assert!(!check.description.contains("spec-kitty.plan"));
    }

    #[test]
    fn check_opencode_commands_passes_when_all_present() {
        let repo = tempfile::tempdir().expect("tempdir");
        let cmd_dir = repo.path().join(".opencode/commands");
        std::fs::create_dir_all(&cmd_dir).expect("create commands dir");

        for cmd in SPEC_KITTY_COMMANDS {
            std::fs::write(cmd_dir.join(format!("{cmd}.md")), "# stub\n")
                .expect("write command");
        }

        let check = check_opencode_commands(repo.path());
        assert_eq!(check.status, CheckStatus::Pass);
    }

    #[test]
    fn launch_preflight_uses_setup_validation_engine() {
        let mut config = Config::default();
        config.paths.zellij_binary = "__missing_zellij__".to_string();

        let failures = launch::preflight_checks(&config).expect_err("preflight should fail");
        assert!(failures.iter().any(|f| f.dependency == "zellij"));
    }

    #[test]
    fn ensure_pane_tracker_config_created_when_missing() {
        let _guard = ENV_LOCK.lock().expect("env lock");
        let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

        let tmp = tempfile::tempdir().expect("tempdir");
        let plugin_dir = tmp.path().join("plugins");
        std::fs::create_dir_all(&plugin_dir).expect("create plugins dir");
        std::fs::write(plugin_dir.join("zellij-pane-tracker.wasm"), b"fake-wasm")
            .expect("write fake wasm");

        unsafe {
            std::env::set_var("ZELLIJ_CONFIG_DIR", tmp.path().display().to_string());
        }

        let updated =
            ensure_pane_tracker_loaded_in_zellij_config().expect("ensure pane tracker config");
        let config_path = tmp.path().join("config.kdl");
        assert!(config_path.is_file());
        let content = std::fs::read_to_string(&config_path).expect("read config");
        assert!(content.contains("zellij-pane-tracker"));
        assert!(updated.is_some());

        unsafe {
            if let Some(dir) = old_zellij_config {
                std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
            } else {
                std::env::remove_var("ZELLIJ_CONFIG_DIR");
            }
        }
    }

    #[test]
    fn ensure_pane_tracker_appends_to_existing_load_plugins() {
        let _guard = ENV_LOCK.lock().expect("env lock");
        let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

        let tmp = tempfile::tempdir().expect("tempdir");
        let plugin_dir = tmp.path().join("plugins");
        std::fs::create_dir_all(&plugin_dir).expect("create plugins dir");
        std::fs::write(plugin_dir.join("zellij-pane-tracker.wasm"), b"fake-wasm")
            .expect("write fake wasm");
        std::fs::write(
            tmp.path().join("config.kdl"),
            "load_plugins {\n    \"file:/tmp/other.wasm\"\n}\n",
        )
        .expect("write config");

        unsafe {
            std::env::set_var("ZELLIJ_CONFIG_DIR", tmp.path().display().to_string());
        }

        let updated =
            ensure_pane_tracker_loaded_in_zellij_config().expect("ensure pane tracker config");
        let content =
            std::fs::read_to_string(tmp.path().join("config.kdl")).expect("read config");
        assert!(content.contains("file:/tmp/other.wasm"));
        assert!(content.contains("zellij-pane-tracker"));
        assert!(updated.is_some());

        unsafe {
            if let Some(dir) = old_zellij_config {
                std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
            } else {
                std::env::remove_var("ZELLIJ_CONFIG_DIR");
            }
        }
    }

    #[test]
    fn zjstatus_check_passes_when_present() {
        let _guard = ENV_LOCK.lock().expect("env lock");
        let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

        let tmp = tempfile::tempdir().expect("tempdir");
        let plugin_dir = tmp.path().join("plugins");
        std::fs::create_dir_all(&plugin_dir).expect("create plugins dir");
        std::fs::write(plugin_dir.join("zjstatus.wasm"), b"fake-wasm")
            .expect("write fake zjstatus");

        unsafe {
            std::env::set_var("ZELLIJ_CONFIG_DIR", tmp.path().display().to_string());
        }

        let result = check_zjstatus();

        unsafe {
            if let Some(dir) = old_zellij_config {
                std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
            } else {
                std::env::remove_var("ZELLIJ_CONFIG_DIR");
            }
        }

        assert_eq!(result.status, CheckStatus::Pass);
    }

    #[test]
    fn zjstatus_check_fails_when_missing() {
        let _guard = ENV_LOCK.lock().expect("env lock");
        let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

        let tmp = tempfile::tempdir().expect("tempdir");
        // No plugins directory -- zjstatus.wasm won't exist.

        unsafe {
            std::env::set_var("ZELLIJ_CONFIG_DIR", tmp.path().display().to_string());
        }

        let result = check_zjstatus();

        unsafe {
            if let Some(dir) = old_zellij_config {
                std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
            } else {
                std::env::remove_var("ZELLIJ_CONFIG_DIR");
            }
        }

        assert_eq!(result.status, CheckStatus::Fail);
        assert!(
            result
                .guidance
                .as_deref()
                .unwrap_or("")
                .contains("zjstatus"),
            "guidance should mention zjstatus"
        );
    }

    #[test]
    fn fixup_mcp_pane_tracker_path_replaces_opt() {
        let mut config: serde_json::Value = serde_json::json!({
            "mcp": {
                "zellij": {
                    "command": [
                        "bun",
                        "run",
                        "/opt/zellij-pane-tracker/mcp-server/index.ts"
                    ]
                }
            }
        });

        fixup_mcp_pane_tracker_path(&mut config, "/home/user/zellij-pane-tracker");

        let command = config["mcp"]["zellij"]["command"]
            .as_array()
            .expect("command should be an array");
        assert_eq!(
            command[2].as_str().expect("third element"),
            "/home/user/zellij-pane-tracker/mcp-server/index.ts"
        );
    }

    #[test]
    fn fixup_mcp_pane_tracker_path_noop_when_no_mcp_section() {
        let mut config: serde_json::Value = serde_json::json!({"agent": {}});

        // Should not panic.
        fixup_mcp_pane_tracker_path(&mut config, "/home/user/zellij-pane-tracker");

        // Config unchanged -- still has agent key and no mcp key.
        assert!(config.get("agent").is_some());
        assert!(config.get("mcp").is_none());
    }

    #[test]
    fn detect_pane_tracker_dir_finds_valid_path() {
        let tmp = tempfile::tempdir().expect("tempdir");
        let mcp_dir = tmp.path().join("mcp-server");
        std::fs::create_dir_all(&mcp_dir).expect("create mcp-server dir");
        std::fs::write(mcp_dir.join("index.ts"), "// stub").expect("write index.ts");

        let mut config = Config::default();
        config.paths.pane_tracker_dir = tmp.path().display().to_string();

        let detected = detect_pane_tracker_dir(&config);
        assert_eq!(detected, tmp.path().display().to_string());
    }

    #[test]
    fn detect_pane_tracker_dir_falls_back_to_default() {
        let config = Config::default();
        // The default /opt/zellij-pane-tracker won't have mcp-server/index.ts
        // in a test environment, so detect should fall back to the config default.
        let detected = detect_pane_tracker_dir(&config);
        assert_eq!(detected, config.paths.pane_tracker_dir);
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
