---
work_package_id: WP01
title: Config schema and pane-tracker auto-detection
lane: "doing"
dependencies: []
base_branch: main
base_commit: 8e32626467b9c4feb8da950f6d308cd6e52ea858
created_at: '2026-02-17T09:50:51.891875+00:00'
phase: Wave 1
assignee: coder
shell_pid: "3854650"
---

# WP01 - Config schema and pane-tracker auto-detection

## Objective

Add `pane_tracker_dir` to the config system and implement auto-detection of the zellij-pane-tracker installation directory. This is the foundation that WP01b builds on for the interactive prompt and MCP fixup.

## Context

Currently the opencode.jsonc template hardcodes `/opt/zellij-pane-tracker/mcp-server/index.ts` as the MCP server command. The goal is to make this configurable. This WP adds the config plumbing and detection logic; WP01b wires it into the interactive setup flow.

## Detailed Steps

### 1. Add `pane_tracker_dir` to `PathsConfig`

In `crates/kasmos/src/config.rs`:

```rust
// In PathsConfig struct (around line 73):
pub struct PathsConfig {
    pub zellij_binary: String,
    pub spec_kitty_binary: String,
    pub specs_root: String,
    /// Installation directory of zellij-pane-tracker (contains mcp-server/).
    pub pane_tracker_dir: String,
}

// In Default for PathsConfig (around line 136):
impl Default for PathsConfig {
    fn default() -> Self {
        Self {
            zellij_binary: "zellij".to_string(),
            spec_kitty_binary: "spec-kitty".to_string(),
            specs_root: "kitty-specs".to_string(),
            pane_tracker_dir: "/opt/zellij-pane-tracker".to_string(),
        }
    }
}
```

### 2. Wire up TOML and env loading

In `crates/kasmos/src/config.rs`:

- Add `pane_tracker_dir: Option<String>` to `PathsConfigFile` (around line 562)
- Add TOML loading in `load_from_file()` (around line 368-378):
  ```rust
  if let Some(v) = paths.pane_tracker_dir {
      self.paths.pane_tracker_dir = v;
  }
  ```
- Add env override in `load_from_env()` (around line 250-255):
  ```rust
  if let Ok(val) = std::env::var("KASMOS_PATHS_PANE_TRACKER_DIR") {
      self.paths.pane_tracker_dir = val;
  }
  ```

### 3. Implement `detect_pane_tracker_dir()` in setup

In `crates/kasmos/src/setup/mod.rs`, add a detection function:

```rust
/// Auto-detect the zellij-pane-tracker installation directory.
///
/// Searches common locations for a directory containing `mcp-server/index.ts`.
/// Returns the first match or falls back to the config default.
fn detect_pane_tracker_dir(config: &Config) -> String {
    let candidates = [
        // Configured value (from kasmos.toml or env)
        Some(config.paths.pane_tracker_dir.clone()),
        // Common install locations
        Some("/opt/zellij-pane-tracker".to_string()),
        std::env::var("HOME").ok().map(|h| format!("{h}/zellij-pane-tracker")),
        std::env::var("HOME").ok().map(|h| format!("{h}/.local/share/zellij-pane-tracker")),
        std::env::var("HOME").ok().map(|h| format!("{h}/src/zellij-pane-tracker")),
    ];

    for candidate in candidates.into_iter().flatten() {
        let mcp_script = PathBuf::from(&candidate).join("mcp-server/index.ts");
        if mcp_script.is_file() {
            return candidate;
        }
    }

    // No valid location found; return the configured default so the user sees
    // it as the pre-filled prompt value and can correct it.
    config.paths.pane_tracker_dir.clone()
}
```

### 4. Enhance `check_pane_tracker()` to also validate MCP server

Extend the existing `check_pane_tracker()` in `crates/kasmos/src/setup/mod.rs` to also check that the MCP server script exists at the configured path. After the existing WASM plugin checks, add:

```rust
// After the WASM plugin pass, also check MCP server availability
let mcp_script = PathBuf::from(&config.paths.pane_tracker_dir)
    .join("mcp-server/index.ts");
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
            "Set pane_tracker_dir in kasmos.toml or run `kasmos setup` to configure.\n\
             Expected: {}/mcp-server/index.ts",
            config.paths.pane_tracker_dir
        )),
    };
}
```

Note: `check_pane_tracker()` currently takes no arguments. You will need to change its signature to accept a `&Config` parameter, and update the call site in `validate_environment_with_repo()` accordingly.

## Files to modify

- `crates/kasmos/src/config.rs`
- `crates/kasmos/src/setup/mod.rs`

## Validation

- `cargo build` succeeds
- `cargo test` passes (update `setup_passes_when_dependencies_are_present` test to create a fake MCP server script, or make the MCP script check a Warn not Fail)
- `Config::default().paths.pane_tracker_dir` returns `"/opt/zellij-pane-tracker"`
- Setting `KASMOS_PATHS_PANE_TRACKER_DIR=/custom/path` and loading config reflects it

## Activity Log
