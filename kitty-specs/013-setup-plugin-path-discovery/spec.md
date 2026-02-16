# 013 - Setup Plugin Path Discovery

## Problem

`kasmos setup` hardcodes the zellij-pane-tracker installation directory to `/opt/zellij-pane-tracker` when generating the opencode MCP config (`opencode.jsonc`). Users who install the pane tracker elsewhere get a broken MCP server entry. Additionally, the `zjstatus.wasm` plugin referenced in generated Zellij layouts is never validated during setup, and external dependencies are not documented in the README.

### Hardcoded path

In `config/profiles/kasmos/opencode.jsonc` (the compile-time embedded template):

```json
"zellij": {
  "type": "local",
  "command": ["bun", "run", "/opt/zellij-pane-tracker/mcp-server/index.ts"],
  "enabled": true
}
```

`apply_selections_and_fixup()` already replaces `~/dev/kasmos` with the real repo root, but nothing touches `/opt/zellij-pane-tracker`.

### Missing zjstatus check

`layout.rs` hardcodes `file:~/.config/zellij/plugins/zjstatus.wasm` in every generated layout. If zjstatus is missing, Zellij sessions fail to start with a cryptic plugin error. Setup validates `zellij-pane-tracker.wasm` but not `zjstatus.wasm`.

### Missing dependency docs

The README lists commands but not external runtime dependencies. `INTEGRATIONS.md` documents pane-tracker but not zjstatus.

## Requirements

### FR-001: Interactive pane-tracker path prompt

During `kasmos setup`, when installing the opencode config interactively, prompt for the zellij-pane-tracker installation directory. Auto-detect if possible (search common locations, check if the MCP server script exists). Default to the auto-detected path or `/opt/zellij-pane-tracker` as fallback. In non-interactive mode, use auto-detection or the fallback silently.

### FR-002: Pane-tracker path fixup in generated config

`apply_selections_and_fixup()` must replace the hardcoded `/opt/zellij-pane-tracker` in the MCP server command with the user-provided path. The replacement must work for the `mcp.zellij.command` array in the opencode.jsonc template.

### FR-003: Config persistence of pane-tracker path

Store the resolved pane-tracker path in `PathsConfig` so it persists in `kasmos.toml` and can be overridden via `KASMOS_PATHS_PANE_TRACKER_DIR` env var. The setup check for the pane-tracker WASM plugin should also validate that the MCP server script exists at this path.

### FR-004: zjstatus.wasm validation

Add a setup check for `zjstatus.wasm` in the Zellij plugin directory. If missing, report `[FAIL]` with installation guidance. Include this in the shared validation engine so launch preflight also catches it.

### FR-005: Dependency documentation

Add a "Dependencies" section to `README.md` listing all required external tools and Zellij plugins. Update `INTEGRATIONS.md` with zjstatus entry.

## Acceptance Criteria

- AC-001: `kasmos setup` prompts for pane-tracker path when installing opencode config interactively
- AC-002: Generated `.opencode/opencode.jsonc` contains the user-provided pane-tracker path (not `/opt`)
- AC-003: `pane_tracker_dir` is persisted in `kasmos.toml` under `[paths]`
- AC-004: `kasmos setup` validates `zjstatus.wasm` presence and reports pass/fail
- AC-005: README documents all external dependencies
- AC-006: Non-interactive setup auto-detects or uses fallback without prompting
- AC-007: `cargo test` passes with no regressions
- AC-008: Launch preflight includes the new zjstatus check
