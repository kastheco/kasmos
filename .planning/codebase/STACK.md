# Technology Stack

**Analysis Date:** 2026-02-16

## Languages

**Primary:**
- Rust (2024 edition) - All application code in `crates/kasmos/`

**Secondary:**
- Bash - Orchestration scripts in `scripts/` (e.g., `scripts/sk-start.sh`, `scripts/review-cycle.sh`)
- KDL - Zellij layout definitions (generated at runtime in `crates/kasmos/src/launch/layout.rs`)
- TOML - Configuration (`kasmos.toml`, `Cargo.toml`)
- YAML - Work package frontmatter in `kitty-specs/*/tasks/WP*.md` files
- Markdown - Agent prompt templates in `config/profiles/kasmos/agent/*.md`, slash commands in `config/profiles/kasmos/commands/*.md`
- JSONC - OpenCode MCP server configuration in `config/profiles/kasmos/opencode.jsonc`

## Runtime

**Environment:**
- Rust stable (rustc 1.92.0, cargo 1.92.0)
- Linux (primary target, uses POSIX-specific APIs such as `libc::flock`)

**Package Manager:**
- Cargo (workspace-level)
- Lockfile: `Cargo.lock` present and committed

**No `.nvmrc`, `rust-toolchain.toml`, or pinned toolchain file detected.** Rust version is implicitly stable-latest.

## Frameworks

**Core:**
- `tokio` 1.49.0 (full features) - Async runtime for all I/O, process spawning, timers
- `rmcp` 0.15 (features: server, transport-io) - MCP protocol server framework for `kasmos serve`
- `clap` 4.5.58 (derive) - CLI argument parsing in `crates/kasmos/src/main.rs`

**Serialization:**
- `serde` 1.0.228 (derive) - Core serialization framework
- `serde_json` 1.0.149 - JSON for MCP protocol, audit logs, message events
- `serde_yml` 0.0.12 - YAML frontmatter parsing for work package task files
- `toml` 0.9 - Configuration file parsing (`kasmos.toml`)

**Testing:**
- Built-in `#[cfg(test)]` with `cargo test` - No external test framework
- `tempfile` 3.25.0 - Temporary directories/files for test fixtures

**Build/Dev:**
- `just` - Task runner (`Justfile` in repo root)
- `cargo clippy` - Linting (enforced: `-D warnings`)
- No formatter config (`.rustfmt.toml`) detected - uses default `rustfmt`

## Key Dependencies

**Critical (core functionality):**
- `rmcp` 0.15 - MCP server protocol implementation; defines tool routing, JSON-RPC transport. Used in `crates/kasmos/src/serve/mod.rs`
- `tokio` 1.49.0 - Async runtime underpinning all I/O. Entry point: `#[tokio::main]` in `crates/kasmos/src/main.rs`
- `clap` 4.5.58 - CLI surface definition. Drives all subcommand routing in `main.rs`
- `serde` + `serde_json` - Serialization backbone for MCP tool I/O, audit logs, config, and state
- `schemars` 1.2 - JSON Schema generation for MCP tool input/output types (used with `#[derive(JsonSchema)]`)

**Infrastructure:**
- `tracing` 0.1.44 + `tracing-subscriber` 0.3.22 (env-filter, fmt) - Structured logging via `RUST_LOG` env var. See `crates/kasmos/src/logging.rs`
- `thiserror` 2.0 - Typed error enums in `crates/kasmos/src/error.rs`
- `anyhow` 1.0.101 - Contextual error handling throughout application code
- `chrono` 0.4.43 (serde) - Timestamps for audit logs, lock records, worker entries
- `kdl` 6.5 - KDL document parsing/validation for Zellij layout generation
- `regex` 1 - Message protocol parsing (`[KASMOS:sender:EVENT]` pattern) in `crates/kasmos/src/serve/messages.rs`
- `libc` 0.2 - Direct `flock()` syscalls for advisory file locking
- `which` 8.0 - Binary discovery for preflight checks (`zellij`, `ocx`, `spec-kitty`, `pane-tracker`)
- `shell-escape` 0.1 - Safe shell argument escaping for agent script generation
- `tempfile` 3.25.0 - Temp files for pane message writing and layout generation

## Configuration

**Primary Config:**
- `kasmos.toml` at repo root - Sectioned TOML configuration
- Precedence: defaults -> `kasmos.toml` -> `KASMOS_*` environment overrides
- Discovery: walks up from cwd looking for `kasmos.toml`
- See `crates/kasmos/src/config.rs` for all fields and defaults

**Config Sections:**
- `[agent]` - `max_parallel_workers`, `opencode_binary`, `opencode_profile`, `review_rejection_cap`
- `[communication]` - `poll_interval_secs`, `event_timeout_secs`
- `[paths]` - `zellij_binary`, `spec_kitty_binary`, `specs_root`
- `[session]` - `session_name`, `manager_width_pct`, `message_log_width_pct`, `max_workers_per_row`
- `[audit]` - `metadata_only`, `debug_full_payload`, `max_bytes`, `max_age_days`
- `[lock]` - `stale_timeout_minutes`

**Environment Variables (key overrides):**
- `KASMOS_AGENT_MAX_PARALLEL_WORKERS` - Max worker panes
- `KASMOS_AGENT_OPENCODE_BINARY` - OpenCode binary path
- `KASMOS_AGENT_OPENCODE_PROFILE` - OpenCode profile name
- `KASMOS_PATHS_ZELLIJ_BINARY` - Zellij binary path
- `KASMOS_PATHS_SPEC_KITTY_BINARY` - spec-kitty binary path
- `KASMOS_SESSION_SESSION_NAME` - Zellij session name
- `RUST_LOG` - Log level/filter for tracing-subscriber
- `ZELLIJ_SESSION_NAME` - Detected at runtime to determine if already inside Zellij
- Legacy aliases: `KASMOS_MAX_PANES`, `KASMOS_MODE`, `KASMOS_ZELLIJ`, `KASMOS_OPENCODE`, etc.

**Agent Profiles:**
- `config/profiles/kasmos/opencode.jsonc` - MCP server config for OpenCode agents
- `config/profiles/kasmos/agent/manager.md` - Manager agent prompt template
- `config/profiles/kasmos/agent/planner.md` - Planner agent prompt template
- `config/profiles/kasmos/agent/coder.md` - Coder agent prompt template
- `config/profiles/kasmos/agent/reviewer.md` - Reviewer agent prompt template
- `config/profiles/kasmos/agent/release.md` - Release agent prompt template
- `config/profiles/kasmos/commands/spec-kitty.*.md` - Spec-kitty slash command definitions

**Build:**
- `Cargo.toml` workspace at repo root, members: `crates/*`
- `Justfile` provides `just build`, `just test`, `just lint`, `just install`, `just swarm`, `just launch`

## Platform Requirements

**Development:**
- Rust stable toolchain (1.92.0+ recommended)
- Linux (POSIX APIs: `flock`, `/etc/hostname`)
- `git` in PATH
- `zellij` in PATH (0.41+ supported, 0.44+ ANSI format handled)
- `ocx` (OpenCode CLI) in PATH
- `spec-kitty` in PATH
- `pane-tracker` or `zellij-pane-tracker` in PATH
- `just` (optional, for Justfile tasks)

**Production/Deployment:**
- Installed via `cargo install --path crates/kasmos` (or `just install`)
- Binary: `kasmos` installed to `~/.cargo/bin/`
- Runs as a local CLI tool / MCP stdio server (not a network service)
- No container, cloud, or CI/CD deployment pipeline detected

---

*Stack analysis: 2026-02-16*
