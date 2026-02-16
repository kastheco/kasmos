# kasmos

Kasmos is an MCP-first orchestration CLI for running planner, coder, reviewer, and release agents in Zellij.

## Command Surface

- `kasmos [SPEC_PREFIX]` launches orchestration for a feature (example: `kasmos 011`)
- `kasmos serve` runs the MCP stdio server used by the manager agent
- `kasmos setup` validates dependencies and writes baseline config assets
- `kasmos list` lists available feature specs
- `kasmos status [feature]` prints workflow progress for a feature

## Typical Workflow

1. Run `kasmos setup`
2. Launch with `kasmos 011`
3. Monitor with `kasmos status 011`
4. Use `kasmos serve` when embedding through an MCP client

## Architecture

- Zellij hosts the session/tab/pane layout
- `kasmos` provides launch, setup, status, and MCP tool handlers
- Manager/worker agents communicate through the message log protocol
- Workflow and lock state are derived from spec-kitty artifacts plus lock files

## Dependencies

kasmos requires these external tools at runtime. `kasmos setup` validates most of these automatically.

### Required binaries

| Tool | Purpose | Install |
|------|---------|---------|
| `zellij` | Terminal multiplexer hosting all sessions/panes | [zellij.dev](https://zellij.dev/documentation/installation) |
| `ocx` / OpenCode | AI agent launcher | Project docs |
| `spec-kitty` | Feature/task lifecycle management | [spec-kitty docs](https://github.com/theslyprofessor/spec-kitty) |
| `git` | Repository and worktree management | System package manager |
| `bun` | Runs the pane-tracker MCP server | [bun.sh](https://bun.sh) |

### Required Zellij plugins

Install to `~/.config/zellij/plugins/`:

| Plugin | Purpose | Source |
|--------|---------|--------|
| `zjstatus.wasm` | Status bar in generated layouts | [github.com/dj95/zjstatus](https://github.com/dj95/zjstatus/releases) |
| `zellij-pane-tracker.wasm` | Pane metadata tracking for agent coordination | [github.com/theslyprofessor/zellij-pane-tracker](https://github.com/theslyprofessor/zellij-pane-tracker) |

### Companion projects

| Project | Purpose | Default location |
|---------|---------|-----------------|
| zellij-pane-tracker (repo checkout) | MCP server for inter-agent pane communication | Configurable via `kasmos.toml` `[paths].pane_tracker_dir` |

> **Note:** `kasmos setup` auto-detects the pane-tracker installation directory and writes it into `.opencode/opencode.jsonc`. Override with `[paths].pane_tracker_dir` in `kasmos.toml` or `KASMOS_PATHS_PANE_TRACKER_DIR` env var.

## Legacy TUI Feature Gate

- Default builds use the MCP-first command surface
- Legacy TUI modules are preserved behind feature flag `tui`
- Build legacy path with `cargo build --features tui`
- Test legacy path with `cargo test --features tui`

## Build And Test

- `cargo build`
- `cargo test`
- `cargo clippy -p kasmos -- -D warnings`

For feature-specific flow examples, see `kitty-specs/011-mcp-agent-swarm-orchestration/quickstart.md`.
