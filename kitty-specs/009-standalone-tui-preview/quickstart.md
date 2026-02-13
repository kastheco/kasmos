# Quickstart: 009-standalone-tui-preview

## Prerequisites

- Rust toolchain (same as existing kasmos build)
- No additional tools required (no Zellij, no git worktrees, no spec-kitty)

## Build & Run

```bash
# Build the project (from repository root)
cargo build -p kasmos

# Launch TUI preview with default 12 WPs
cargo run -p kasmos -- tui

# Launch with custom WP count
cargo run -p kasmos -- tui --count 25
```

## What You'll See

1. **Dashboard tab** (Tab 1): Kanban board with 4 lanes — Planned, Doing, For Review, Done. WPs animate between lanes every ~3 seconds.
2. **Review tab** (Tab 2): Shows WPs in ForReview state. Approve/reject keybindings work (actions silently discarded).
3. **Logs tab** (Tab 3): State transition events logged as they happen.

## Keybindings

All existing TUI keybindings work. Press `Alt+q` to quit.

## Development Workflow

The intended workflow for TUI iteration:

```bash
# Terminal 1: Edit TUI code
vim crates/kasmos/src/tui/app.rs

# Terminal 2: Rebuild and preview
cargo run -p kasmos -- tui
# See your changes → Ctrl+C or q → edit → repeat
```

## Verification

```bash
# Ensure it builds cleanly
cargo build -p kasmos 2>&1 | grep -c warning  # should be 0

# Ensure no test regressions
cargo test -p kasmos
```

## Files to Implement

| File | Action | Description |
|------|--------|-------------|
| `crates/kasmos/src/main.rs` | Modify | Add `Tui` variant, `mod tui_preview`, match arm, help text |
| `crates/kasmos/src/tui_preview.rs` | Create | Mock data generator + deterministic animation loop |
