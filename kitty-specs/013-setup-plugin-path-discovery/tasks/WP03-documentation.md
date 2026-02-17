---
work_package_id: WP03
title: Dependency documentation
lane: "done"
dependencies: []
base_branch: main
base_commit: abeda7b5876b81200f3f41d7158844665fae79ea
created_at: '2026-02-17T09:51:21.993340+00:00'
phase: Wave 1
assignee: coder
shell_pid: "3856130"
reviewed_by: "kas"
review_status: "approved"
---

# WP03 - Dependency documentation

## Objective

Document all external runtime dependencies in README.md and update INTEGRATIONS.md with the zjstatus plugin entry. Users should be able to read the README and know exactly what needs to be installed before running `kasmos setup`.

## Detailed Steps

### 1. Add Dependencies section to README.md

Insert a "Dependencies" section after the "Architecture" section (before "Legacy TUI Feature Gate"). Use a table format:

```markdown
## Dependencies

kasmos requires these external tools at runtime. Run `kasmos setup` to validate.

### Required binaries

| Tool | Purpose | Install |
|------|---------|---------|
| `zellij` | Terminal multiplexer hosting all sessions/panes | [zellij.dev](https://zellij.dev/documentation/installation) |
| `ocx` / OpenCode | AI agent launcher | Project docs |
| `spec-kitty` | Feature/task lifecycle management | `pip install spec-kitty` |
| `git` | Repository and worktree management | System package manager |
| `bun` | Runs the pane-tracker MCP server | [bun.sh](https://bun.sh) |

### Required Zellij plugins

These WASM plugins must be in `~/.config/zellij/plugins/`:

| Plugin | Purpose | Source |
|--------|---------|--------|
| `zjstatus.wasm` | Status bar in generated layouts | [github.com/dj95/zjstatus](https://github.com/dj95/zjstatus/releases) |
| `zellij-pane-tracker.wasm` | Pane metadata tracking for agent coordination | [github.com/theslyprofessor/zellij-pane-tracker](https://github.com/theslyprofessor/zellij-pane-tracker) |

### Required companion projects

| Project | Purpose | Default location |
|---------|---------|-----------------|
| `zellij-pane-tracker` (repo checkout) | MCP server for inter-agent pane communication | Configurable via `kasmos.toml` `[paths].pane_tracker_dir` |

> `kasmos setup` auto-detects the pane-tracker installation directory
> and writes it into `.opencode/opencode.jsonc`. Override with
> `[paths].pane_tracker_dir` in `kasmos.toml` or `KASMOS_PATHS_PANE_TRACKER_DIR` env var.

### Optional Zellij plugins

| Plugin | Purpose | Source |
|--------|---------|--------|
| `zjstatus-hints` | Keybinding hints piped into zjstatus bar | Loaded globally in `config.kdl` |
| `zjframes` | Pane frame toggling | Loaded globally in `config.kdl` |
```

### 2. Add zjstatus entry to INTEGRATIONS.md

In `.planning/codebase/INTEGRATIONS.md`, add a new section after the "pane-tracker / zellij-pane-tracker" entry (around line 75):

```markdown
**zjstatus (Zellij Status Bar Plugin):**
- Purpose: Renders the status bar in all kasmos-generated Zellij layouts
- Plugin file: `~/.config/zellij/plugins/zjstatus.wasm`
- Source: https://github.com/dj95/zjstatus
- Integration: Hardcoded in `crates/kasmos/src/layout.rs` (`build_tab_template()`)
- Setup validation: `check_zjstatus()` in `crates/kasmos/src/setup/mod.rs`
- Configuration: Rose Pine Moon theme with zjstatus-hints pipe integration
- Features used: mode indicators, tab styles, datetime, pipe format (zjstatus_hints)
```

### 3. Update quickstart.md prerequisites

In `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`, update the Prerequisites section to include zjstatus:

```markdown
## Prerequisites

- Rust stable toolchain (2024 edition support)
- `zellij` in `PATH`
- `opencode` in `PATH`
- `pane-tracker` (or `zellij-pane-tracker`) in `PATH`
- `zjstatus.wasm` in `~/.config/zellij/plugins/`
- `zellij-pane-tracker.wasm` in `~/.config/zellij/plugins/`
- `bun` in `PATH` (runs pane-tracker MCP server)
```

## Files to modify

- `README.md`
- `.planning/codebase/INTEGRATIONS.md`
- `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`

## Validation

- README has a Dependencies section with tables for binaries, plugins, and companion projects
- INTEGRATIONS.md has a zjstatus entry with all fields matching the pane-tracker entry format
- quickstart.md lists zjstatus and bun in prerequisites
- No encoding issues (UTF-8 only, no smart quotes)

## Activity Log
- 2026-02-17T09:53:48Z – unknown – shell_pid=3856130 – lane=done – Code already on main - all tests pass
