---
work_package_id: WP04
title: Tests for plugin path discovery and zjstatus check
lane: "done"
dependencies: [WP01, WP01b, WP02]
phase: "Wave 3"
assignee: "coder"
reviewed_by: "kas"
review_status: "approved"
---

# WP04 - Tests for plugin path discovery and zjstatus check

## Objective

Add comprehensive tests for all new functionality introduced in WP01, WP01b, and WP02. Update existing tests that may be affected by the new zjstatus check and config field.

## Context

The test module in `crates/kasmos/src/setup/mod.rs` (starting at line 902) already has:

- `setup_passes_when_dependencies_are_present` -- creates fake binaries and plugins
- `setup_fails_when_dependency_is_missing`
- `setup_generates_assets_idempotently`
- `install_opencode_agents_creates_missing_roles`
- `check_opencode_agents_reports_missing`
- `launch_preflight_uses_setup_validation_engine`

The test module in `crates/kasmos/src/config.rs` (starting at line 653) has:

- `default_config_validates`
- `partial_toml_loads_with_defaults`
- `invalid_values_produce_clear_errors`
- `env_overrides_take_precedence`

## Detailed Steps

### 1. Config tests (`config.rs`)

Add to the existing test module:

```rust
#[test]
fn pane_tracker_dir_default() {
    let config = Config::default();
    assert_eq!(config.paths.pane_tracker_dir, "/opt/zellij-pane-tracker");
}

#[test]
fn pane_tracker_dir_from_toml() {
    let tmp = tempfile::tempdir().expect("create tempdir");
    let path = tmp.path().join("kasmos.toml");
    std::fs::write(
        &path,
        r#"
[paths]
pane_tracker_dir = "/home/user/zellij-pane-tracker"
"#,
    )
    .expect("write toml");

    let mut config = Config::default();
    config.load_from_file(&path).expect("load toml");
    assert_eq!(
        config.paths.pane_tracker_dir,
        "/home/user/zellij-pane-tracker"
    );
}

#[test]
fn pane_tracker_dir_from_env() {
    let _guard = ENV_TEST_LOCK.lock().expect("env lock");

    let mut config = Config::default();
    unsafe {
        std::env::set_var("KASMOS_PATHS_PANE_TRACKER_DIR", "/custom/tracker");
    }
    config.load_from_env().expect("load env");
    unsafe {
        std::env::remove_var("KASMOS_PATHS_PANE_TRACKER_DIR");
    }

    assert_eq!(config.paths.pane_tracker_dir, "/custom/tracker");
}
```

### 2. zjstatus check tests (`setup/mod.rs`)

Add to the existing test module:

```rust
#[test]
fn zjstatus_check_passes_when_present() {
    let _guard = ENV_LOCK.lock().expect("env lock");
    let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

    let tmp = tempfile::tempdir().expect("tempdir");
    let plugin_dir = tmp.path().join("plugins");
    std::fs::create_dir_all(&plugin_dir).expect("create plugin dir");
    std::fs::write(plugin_dir.join("zjstatus.wasm"), b"fake-wasm")
        .expect("write fake zjstatus");

    unsafe {
        std::env::set_var("ZELLIJ_CONFIG_DIR", tmp.path().display().to_string());
    }

    let check = check_zjstatus();
    assert_eq!(check.status, CheckStatus::Pass);
    assert_eq!(check.name, "zjstatus");

    unsafe {
        if let Some(dir) = old_zellij_config {
            std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
        } else {
            std::env::remove_var("ZELLIJ_CONFIG_DIR");
        }
    }
}

#[test]
fn zjstatus_check_fails_when_missing() {
    let _guard = ENV_LOCK.lock().expect("env lock");
    let old_zellij_config = std::env::var("ZELLIJ_CONFIG_DIR").ok();

    let tmp = tempfile::tempdir().expect("tempdir");
    // No plugins directory, no zjstatus.wasm

    unsafe {
        std::env::set_var("ZELLIJ_CONFIG_DIR", tmp.path().display().to_string());
    }

    let check = check_zjstatus();
    assert_eq!(check.status, CheckStatus::Fail);
    assert!(check.guidance.is_some());
    assert!(check.guidance.unwrap().contains("zjstatus"));

    unsafe {
        if let Some(dir) = old_zellij_config {
            std::env::set_var("ZELLIJ_CONFIG_DIR", dir);
        } else {
            std::env::remove_var("ZELLIJ_CONFIG_DIR");
        }
    }
}
```

### 3. MCP path fixup test (`setup/mod.rs`)

```rust
#[test]
fn fixup_mcp_pane_tracker_path_replaces_opt() {
    let json_str = r#"{
        "mcp": {
            "zellij": {
                "type": "local",
                "command": ["bun", "run", "/opt/zellij-pane-tracker/mcp-server/index.ts"],
                "enabled": true
            }
        }
    }"#;

    let mut config: serde_json::Value =
        serde_json::from_str(json_str).expect("parse json");

    fixup_mcp_pane_tracker_path(&mut config, "/home/user/zellij-pane-tracker");

    let command = config["mcp"]["zellij"]["command"]
        .as_array()
        .expect("command array");
    assert_eq!(command[2].as_str().unwrap(),
        "/home/user/zellij-pane-tracker/mcp-server/index.ts");
}

#[test]
fn fixup_mcp_pane_tracker_path_noop_when_no_mcp_section() {
    let mut config: serde_json::Value =
        serde_json::from_str(r#"{"agent": {}}"#).expect("parse json");

    // Should not panic
    fixup_mcp_pane_tracker_path(&mut config, "/some/path");
}
```

### 4. Pane-tracker auto-detection test (`setup/mod.rs`)

```rust
#[test]
fn detect_pane_tracker_dir_finds_valid_path() {
    let tmp = tempfile::tempdir().expect("tempdir");
    let mcp_dir = tmp.path().join("mcp-server");
    std::fs::create_dir_all(&mcp_dir).expect("create mcp dir");
    std::fs::write(mcp_dir.join("index.ts"), "// mcp server").expect("write index.ts");

    let mut config = Config::default();
    config.paths.pane_tracker_dir = tmp.path().display().to_string();

    let detected = detect_pane_tracker_dir(&config);
    assert_eq!(detected, tmp.path().display().to_string());
}

#[test]
fn detect_pane_tracker_dir_falls_back_to_default() {
    let config = Config::default();
    // None of the candidate paths will have mcp-server/index.ts in a test env
    let detected = detect_pane_tracker_dir(&config);
    // Should fall back to the config default
    assert_eq!(detected, config.paths.pane_tracker_dir);
}
```

### 5. Update `setup_passes_when_dependencies_are_present`

The existing test (line 910) needs to also create a fake `zjstatus.wasm`. Add:

```rust
std::fs::write(plugin_dir.join("zjstatus.wasm"), b"fake-wasm")
    .expect("write fake zjstatus wasm");
```

And add the zjstatus assertion:

```rust
assert!(
    result.checks.iter()
        .any(|c| c.name == "zjstatus" && c.status == CheckStatus::Pass)
);
```

## Files to modify

- `crates/kasmos/src/setup/mod.rs` (test module)
- `crates/kasmos/src/config.rs` (test module)

## Validation

- `cargo test` passes with all new and existing tests
- `cargo clippy -p kasmos -- -D warnings` clean

## Activity Log
- 2026-02-17T09:53:54Z – unknown – lane=done – Code already on main - all WP04 tests present and passing
