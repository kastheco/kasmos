# AGENTS.md

## Startup checklist
- Read `README.md` for project overview.
- Read `.kittify/memory/` for project constitution, architecture knowledge, and workflow intelligence.
- Check `kitty-specs/` for feature specifications.
- This is a Rust workspace -- crates live in `crates/`.
- Primary binary: `crates/kasmos/` -- the Zellij orchestrator.

## Repository layout
- `crates/kasmos/`: Main orchestrator binary
- `kitty-specs/`: Feature specifications (spec-kitty)
- `.kittify/memory/`: Persistent project memory (constitution, architecture, workflow learnings)
- `.kittify/`: spec-kitty project configuration, scripts, missions
- `docs/`: Documentation

## Build / run commands
- Build: `cargo build`
- Run: `cargo run -p kasmos`
- Test: `cargo test`

## Code style (Rust)
- Use Rust 2024 edition conventions
- Prefer explicit error handling with `thiserror` / `anyhow`
- Use `tokio` for async runtime
- Follow standard Rust naming: `snake_case` functions, `PascalCase` types
- Keep modules small and focused

## External tools
- `zellij`: Terminal multiplexer (must be in PATH)
- `spec-kitty`: Feature specification tool
- `opencode`: AI coding agent harness (launched in Zellij panes)
- `git`: Version control

## Agent harness: OpenCode only

kasmos uses **OpenCode** as the sole agent harness for spawning worker agents. This is a hard rule:

- Worker panes are launched via `opencode [-p <profile>] -- --agent <role> --prompt <prompt>`.
- The `opencode_binary` and `opencode_profile` are configured in `kasmos.toml` under `[agent]`.
- **Never invoke a model-specific CLI** (e.g., `claude`, `gemini`, `aider`) directly. OpenCode is the abstraction layer -- it handles model selection, permissions, and session management.
- kasmos is **model-agnostic**. The model running behind OpenCode is configured in OpenCode's own config, not in kasmos. Do not assume or hardcode any specific model provider.
- When spawning workers programmatically, always go through kasmos MCP tools (`spawn_worker`) or `opencode`. Never shell out to a bare model CLI.
- If you are the manager agent and need to delegate work to a new pane, use `kasmos serve`'s `spawn_worker` MCP tool, which handles the OpenCode invocation internally.

## Worktree awareness

kasmos uses git worktrees at `.worktrees/<feature_slug>-<wp_id>/` for WP isolation. When modifying code that deals with file paths -- especially task file watching, file scanning, or agent CWD setup -- always consider whether the path should point to the main repo or the worktree. See `.kittify/memory/architecture.md` for the full explanation and known issues.

Key rule: agents work in worktrees, so any file they modify is the worktree copy. Watchers/detectors that need to see agent changes must watch the worktree path, not the main repo path.

## Zellij constraints

- There is no `list-panes` or `focus-pane-by-name` CLI command (as of Zellij 0.41+).
- Inside a Zellij session, use `zellij action <cmd>` directly (no `--session` flag).
- Pane tracking is internal via `SessionManager` HashMap -- do not assume Zellij provides pane discovery.
- See `.kittify/memory/architecture.md` for session layout and pane naming conventions.

## Persistent memory

When you discover something significant about the codebase architecture, runtime behavior, or integration quirks, record it in `.kittify/memory/`. This directory is symlinked into worktrees so all sessions share it.

- `constitution.md`: Project technical standards and governance (do not modify without discussion).
- `architecture.md`: Codebase structure, type locations, subsystem interactions, known issues.
- `workflow-intelligence.md`: Lessons from the spec-kitty planning lifecycle.
