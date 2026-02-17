---
work_package_id: WP02
title: zjstatus.wasm setup validation
lane: "doing"
dependencies: []
base_branch: main
base_commit: f3978873a0d7303208a59decfd70868fe527042a
created_at: '2026-02-17T09:51:21.273774+00:00'
phase: Wave 1
assignee: coder
shell_pid: "3856010"
---

# WP02 - zjstatus.wasm setup validation

## Objective

Add a `check_zjstatus()` validation to `kasmos setup` and launch preflight. This mirrors the existing `check_pane_tracker()` pattern. Currently, generated Zellij layouts reference `file:~/.config/zellij/plugins/zjstatus.wasm` but setup never verifies it exists -- leading to cryptic Zellij plugin errors at launch time.

## Context

In `crates/kasmos/src/layout.rs` line 258-260, every generated layout includes:

```rust
plugin.entries_mut().push(kdl_str_prop(
    "location",
    "file:~/.config/zellij/plugins/zjstatus.wasm",
));
```

The existing `check_pane_tracker()` function at `setup/mod.rs:180-223` is the exact pattern to follow. It uses `zellij_plugin_dir()` to resolve the plugin directory.

## Detailed Steps

### 1. Add `check_zjstatus()` function

In `crates/kasmos/src/setup/mod.rs`, add after `check_pane_tracker()`:

```rust
fn check_zjstatus() -> CheckResult {
    let required_for = "status bar in Zellij layouts (zjstatus plugin)";
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
```

### 2. Add to the checks vector

In `validate_environment_with_repo()` (around line 126-158), add `check_zjstatus()` to the checks vector, right after `check_pane_tracker()`:

```rust
let mut checks = vec![
    check_binary(/* zellij */),
    check_binary(/* opencode */),
    check_binary(/* spec-kitty */),
    check_pane_tracker(),
    check_zjstatus(),         // <-- NEW
];
```

This automatically makes it part of launch preflight too, since `launch/mod.rs` calls `validate_environment()`.

### 3. Update the test `setup_passes_when_dependencies_are_present`

The existing test (around line 910-993) creates fake plugin files. Add a fake `zjstatus.wasm`:

```rust
// After the existing pane-tracker.wasm creation:
std::fs::write(plugin_dir.join("zjstatus.wasm"), b"fake-wasm")
    .expect("write fake zjstatus wasm");
```

And add an assertion:

```rust
assert!(
    result.checks.iter()
        .any(|c| c.name == "zjstatus" && c.status == CheckStatus::Pass)
);
```

## Files to modify

- `crates/kasmos/src/setup/mod.rs`

## Validation

- `cargo build` succeeds
- `cargo test` passes (including the updated `setup_passes_when_dependencies_are_present` test)
- Running `kasmos setup` shows a `zjstatus` check line
- If `~/.config/zellij/plugins/zjstatus.wasm` doesn't exist, shows `[FAIL]` with install URL
- If it exists, shows `[PASS]`

## Activity Log
