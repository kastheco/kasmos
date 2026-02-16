# kasmos

Kasmos is an MCP-first orchestration CLI that runs AI agent swarms -- planner, coder, reviewer, and release roles -- inside a [Zellij](https://zellij.dev) terminal session. A manager agent coordinates work packages through an MCP server while each worker agent operates in its own pane and git worktree.

## Getting Started

### How It Works

Kasmos ties together several tools into a single orchestration flow:

| Component | Role |
|-----------|------|
| **kasmos** | Orchestrator CLI -- launches sessions, spawns agents, exposes MCP tools |
| **Zellij** | Terminal multiplexer that hosts the pane layout (manager, message log, dashboard, workers) |
| **OpenCode** | AI coding agent runtime that drives each pane (manager + workers) |
| **spec-kitty** | Feature specification and task lifecycle tool -- produces the specs, plans, and work packages that kasmos orchestrates |
| **zellij-pane-tracker** | Zellij plugin for pane metadata tracking used by agent coordination |
| **git** | Worktree isolation -- each work package runs in its own checkout |

When you run `kasmos 011`, it resolves the feature spec, acquires a lock, generates a KDL layout, and opens a Zellij session with a manager agent that uses `kasmos serve` as its MCP server. The manager then spawns worker agents (planner, coder, reviewer, release) into separate panes, each scoped to their own git worktree and constrained to role-specific context boundaries.

### Prerequisites

Install the following before running kasmos:

| Dependency | Install |
|------------|---------|
| **Rust toolchain** | [rustup.rs](https://rustup.rs) |
| **Zellij** | `cargo install zellij` or your package manager |
| **OpenCode** | [opencode.ai](https://opencode.ai) -- ensure the `opencode` binary is on PATH |
| **spec-kitty** | Install and ensure `spec-kitty` is on PATH |
| **git** | Your package manager (likely already installed) |
| **just** | `cargo install just` (optional -- for convenience recipes) |
| **zellij-pane-tracker** | See [plugin install](#pane-tracker-plugin) below |

#### Pane Tracker Plugin

```sh
git clone https://github.com/theslyprofessor/zellij-pane-tracker
cd zellij-pane-tracker
rustup target add wasm32-wasip1
cargo build --release
mkdir -p ~/.config/zellij/plugins
cp target/wasm32-wasip1/release/zellij-pane-tracker.wasm ~/.config/zellij/plugins/
```

Then add to `load_plugins { }` in `~/.config/zellij/config.kdl`:

```kdl
"file:~/.config/zellij/plugins/zellij-pane-tracker.wasm"
```

### Install

```sh
git clone https://github.com/kastheco/kasmos.git
cd kasmos
cargo install --path crates/kasmos
```

Or with `just`:

```sh
just build && just install
```

### First Run

Run `kasmos setup` from your project repository. It validates every dependency, generates baseline config assets, and walks you through interactive OpenCode configuration:

```
$ kasmos setup
kasmos setup
[PASS] zellij         /usr/bin/zellij
[PASS] opencode       /usr/local/bin/opencode
[PASS] spec-kitty     /usr/local/bin/spec-kitty
[PASS] pane-tracker   ~/.config/zellij/plugins/zellij-pane-tracker.wasm
[PASS] oc-config      .opencode/opencode.jsonc
[PASS] oc-agents      .opencode/agents/ (5 roles)
[PASS] git            /usr/bin/git (in git repo /home/you/project)
[PASS] config         /home/you/project/kasmos.toml
```

Setup generates:
- `kasmos.toml` -- project-level configuration (session layout, agent limits, polling intervals)
- `.opencode/opencode.jsonc` -- per-project OpenCode config with model/reasoning selections per agent role, MCP server definitions, and file permissions
- `.opencode/agents/*.md` -- agent role definitions (manager, planner, coder, reviewer, release)
- `config/profiles/kasmos/` -- baseline profile templates

If `.opencode/opencode.jsonc` already exists, setup will ask whether to reconfigure. In non-interactive environments (no TTY), template defaults are applied automatically.

### Usage

**List available feature specs:**

```sh
kasmos list
```

**Launch orchestration for a feature:**

```sh
kasmos 011    # resolves spec prefix to kitty-specs/011-*/
```

This opens a Zellij session with the layout:

```
+---manager(60%)---+--msg-log(20%)--+--dashboard(20%)--+  <- top row
|                                                       |
|                    worker-area                         |  <- dynamic panes
|                                                       |
+-------------------------------------------------------+
```

The manager agent reads the spec and plan, determines the workflow phase, and spawns workers wave-by-wave based on work package dependencies.

**Monitor progress:**

```sh
kasmos status 011
```

**Run as MCP server (used internally by the manager agent):**

```sh
kasmos serve
```

### Configuration

Kasmos loads config from `kasmos.toml` in the repo root, with `KASMOS_*` environment variable overrides. Key sections:

| Section | Controls |
|---------|----------|
| `[agent]` | `max_parallel_workers`, `opencode_binary`, `opencode_profile`, `review_rejection_cap` |
| `[session]` | `session_name`, `manager_width_pct`, `message_log_width_pct` |
| `[paths]` | `zellij_binary`, `spec_kitty_binary`, `specs_root` |
| `[communication]` | `poll_interval_secs`, `event_timeout_secs` |
| `[audit]` | `metadata_only`, `debug_full_payload`, `max_bytes`, `max_age_days` |
| `[lock]` | `stale_timeout_minutes` |

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
