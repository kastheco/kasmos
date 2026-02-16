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
