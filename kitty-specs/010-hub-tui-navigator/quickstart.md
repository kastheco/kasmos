# Quickstart: Hub TUI Navigator

## Prerequisites

- Rust toolchain (workspace edition 2024)
- Zellij terminal multiplexer in PATH
- OpenCode (`ocx`) in PATH (for agent pane actions)
- spec-kitty in PATH (for feature management)

## Build

```bash
cargo build -p kasmos
```

## Run

### Launch the Hub TUI (default — no args)

```bash
# Must be inside a Zellij session for full functionality:
kasmos
```

This opens the interactive hub TUI showing all features from `kitty-specs/`.

### Run Outside Zellij (read-only mode)

```bash
# Works outside Zellij but pane/tab actions are unavailable:
kasmos
# Hub displays features with a warning that actions require Zellij
```

### Start Orchestration (TUI default)

```bash
# New default — launches orchestration TUI:
kasmos start 010-hub-tui-navigator

# Old behavior — direct Zellij attach:
kasmos start 010-hub-tui-navigator --no-tui

# Backward compatible — --tui flag silently accepted:
kasmos start 010-hub-tui-navigator --tui
```

### CLI Help

```bash
kasmos --help           # Shows all subcommands + hub TUI as default
kasmos start --help     # Shows --no-tui flag, mode options
```

## Hub Keybindings

| Key | Action |
|---|---|
| `j` / `k` | Move selection up/down |
| `Enter` | Select feature / trigger action |
| `Esc` | Back to list view |
| `n` | New feature (opens name prompt) |
| `r` | Manual refresh |
| `Shift+Enter` | Start implementation (wave-gated mode) |
| `Alt+q` | Quit hub |

## Test

```bash
cargo test -p kasmos
```

Key test areas:
- `hub::scanner` — Feature state detection from filesystem
- `hub::actions` — HubAction resolution from FeatureEntry state
- CLI parsing — Optional subcommand, `--no-tui` flag, backward compat

## Architecture Overview

```
main.rs
  └── command: Option<Commands>
        ├── None → hub::run()          ← NEW
        ├── Some(Start { .. }) → start::run()  ← MODIFIED (TUI default)
        └── Some(List|Status|...) → unchanged

hub/
  ├── mod.rs       — event loop (reuses tui::setup_terminal)
  ├── app.rs       — App state, rendering, event handling
  ├── scanner.rs   — FeatureScanner (reads kitty-specs/)
  ├── actions.rs   — Zellij pane/tab dispatch
  └── keybindings.rs
```
