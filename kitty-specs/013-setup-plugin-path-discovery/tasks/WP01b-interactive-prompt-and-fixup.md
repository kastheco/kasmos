---
work_package_id: WP01b
title: Interactive pane-tracker path prompt and MCP fixup
lane: "planned"
dependencies: [WP01]
phase: "Wave 2"
assignee: "coder"
---

# WP01b - Interactive pane-tracker path prompt and MCP fixup

## Objective

Wire the pane-tracker auto-detection (from WP01) into the interactive `kasmos setup` flow and implement the MCP command path fixup so generated `.opencode/opencode.jsonc` files use the correct path instead of the hardcoded `/opt/zellij-pane-tracker`.

## Context

### Current interactive flow (`interactive_opencode_config()`)

Located at `crates/kasmos/src/setup/mod.rs:549-623`. The flow is:

1. Parse the embedded template
2. Discover available models
3. Show role defaults
4. Ask "Customize per role?" -> model/reasoning selection
5. Call `apply_selections_and_fixup()` to patch model/reasoning and external_directory paths
6. Serialize to JSON

### Current fixup flow (`apply_selections_and_fixup()`)

Located at `crates/kasmos/src/setup/mod.rs:494-519`. It:

1. Patches model/reasoning per role
2. Calls `fixup_external_directory()` to replace `~/dev/kasmos` with actual repo root

Neither touches the `mcp` section of the config.

### Target template section

In `config/profiles/kasmos/opencode.jsonc` lines 135-146:

```json
"mcp": {
    "zellij": {
      "type": "local",
      "command": ["bun", "run", "/opt/zellij-pane-tracker/mcp-server/index.ts"],
      "enabled": true,
    },
    ...
}
```

## Detailed Steps

### 1. Add pane-tracker path prompt to `interactive_opencode_config()`

After the model/reasoning customization block (around line 618) and before `apply_selections_and_fixup()`, add:

```rust
// Detect pane-tracker installation
let detected_dir = detect_pane_tracker_dir(/* pass config or use the function from WP01 */);
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
```

### 2. Pass pane-tracker path to `apply_selections_and_fixup()`

Update the function signature:

```rust
fn apply_selections_and_fixup(
    config: &mut serde_json::Value,
    selections: &BTreeMap<String, (String, String)>,
    repo_root: &Path,
    pane_tracker_dir: &str,  // <-- NEW
)
```

Update both call sites (interactive at ~line 620, non-interactive at ~line 654).

For non-interactive mode (around line 650-655), auto-detect without prompting:

```rust
let pane_tracker_dir = detect_pane_tracker_dir(&config_obj);
// where config_obj is the Config loaded at the top of run()
```

Note: you'll need to thread the `Config` reference into `install_opencode_config()` or pass just the detected path.

### 3. Implement `fixup_mcp_pane_tracker_path()`

Add a new fixup function and call it from `apply_selections_and_fixup()`:

```rust
/// Replace `/opt/zellij-pane-tracker` in mcp.zellij.command with the actual path.
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
        if let Some(s) = elem.as_str() {
            if s.contains("/opt/zellij-pane-tracker") {
                *elem = serde_json::Value::String(
                    s.replace("/opt/zellij-pane-tracker", pane_tracker_dir),
                );
            }
        }
    }
}
```

Call it at the end of `apply_selections_and_fixup()`:

```rust
fixup_mcp_pane_tracker_path(config, pane_tracker_dir);
```

### 4. Verify the full flow

The resulting `.opencode/opencode.jsonc` should have:

```json
"mcp": {
    "zellij": {
      "type": "local",
      "command": ["bun", "run", "<user-provided-path>/mcp-server/index.ts"],
      "enabled": true
    }
}
```

## Files to modify

- `crates/kasmos/src/setup/mod.rs`

## Validation

- `cargo build` succeeds
- `cargo test` passes
- Running `kasmos setup` interactively shows a pane-tracker directory prompt with auto-detected default
- The generated `.opencode/opencode.jsonc` contains the user-provided path, NOT `/opt/zellij-pane-tracker`
- Non-interactive mode (piped stdin) uses auto-detected path without prompting
- If user enters a path where `mcp-server/index.ts` doesn't exist, validation rejects it

## Activity Log
