# AGENTS.md

## Startup checklist
- Read `README.md` for project overview.
- Check `kitty-specs/` for feature specifications.
- This is a Rust workspace — crates live in `crates/`.
- Primary binary: `crates/kasmos/` — the Zellij orchestrator.

## Repository layout
- `crates/kasmos/`: Main orchestrator binary
- `kitty-specs/`: Feature specifications (spec-kitty)
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
- `opencode`: AI coding agent (launched in Zellij panes)
- `git`: Version control
