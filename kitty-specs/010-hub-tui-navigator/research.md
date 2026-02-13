# Research: Hub TUI Navigator

## R-001: Zellij Pane Direction API

**Decision**: Use `zellij action new-pane --direction right -- <command> <args>` for opening agent panes to the right of the hub.

**Rationale**: Zellij CLI (verified on installed version) supports `--direction <DIRECTION>` on `new-pane` with values `right` and `down`. It also supports `--name <NAME>` for naming panes, `--cwd <CWD>` for working directory, and `-- <COMMAND>` for running a command in the pane.

**Key Finding — Inside vs Outside Session**:
- **Inside a Zellij session**: `zellij action <cmd>` operates on the current session implicitly. No `--session` flag needed.
- **Outside a session**: `zellij --session <name> action <cmd>` required (the existing `ZellijCli` trait pattern).

Since the hub runs *inside* a Zellij pane, it should use direct `zellij action` calls (via `tokio::process::Command`) for its own pane/tab operations, without the `--session` flag. The `ZellijCli` trait's session-based methods are still needed for session *listing* (orchestration detection from `list-sessions`).

**Implementation**: Create a `hub::zellij_actions` module with thin wrappers around `zellij action` calls:
- `open_pane_right(command, args, cwd, name)` → `zellij action new-pane --direction right [--name <name>] [--cwd <cwd>] -- <command> <args>`
- `open_new_tab(command, args, name)` → `zellij action new-tab [--name <name>]` + pane with command
- `go_to_tab(name)` → `zellij action go-to-tab-name <name>`
- `query_tab_names()` → `zellij action query-tab-names`

**Alternatives considered**:
- Using the existing `ZellijCli` trait with `--session` flag: Would work but adds unnecessary complexity since the hub is always inside a session. The `--session` approach is for remote control.
- Using KDL layouts for pane creation: Overkill for single-pane operations.

## R-002: OpenCode `--prompt` Flag

**Decision**: Use `ocx oc -- --prompt "<slash-command>" --agent controller` to launch OpenCode with a pre-loaded slash command.

**Rationale**: OpenCode CLI (verified from `ocx oc -- --help`) supports:
- `--prompt <string>`: Pre-loads a prompt/command that executes on session start
- `--agent <name>`: Selects a specific agent (e.g., `controller`)
- `--model <provider/model>`: Overrides the default model
- `--continue`: Continue last session
- `--session <id>`: Resume a specific session

**Implementation**: The hub constructs the full command for `zellij action new-pane`:
```
zellij action new-pane --direction right --name "spec-010" -- \
  ocx oc -- --prompt "/spec-kitty.specify" --agent controller
```

For planning:
```
zellij action new-pane --direction right --name "plan-010" -- \
  ocx oc -- --prompt "/spec-kitty.plan" --agent controller
```

For task generation:
```
zellij action new-pane --direction right --name "tasks-010" -- \
  ocx oc -- --prompt "/spec-kitty.tasks" --agent controller
```

**Alternatives considered**:
- Launching bare OpenCode and relying on operator to type the command: Worse UX, defeats the purpose of the hub as a launcher.
- Writing a temporary script that OpenCode reads on startup: Unnecessary given the `--prompt` flag exists.

## R-003: clap Optional Subcommand Pattern

**Decision**: Use `Option<Commands>` for the subcommand field in the `Cli` struct.

**Rationale**: clap 4.x supports `#[command(subcommand)] command: Option<Commands>` which makes the subcommand optional. When no subcommand is provided, `command` is `None`. This is the idiomatic pattern for "default command when no args" in clap.

**Implementation**:
```rust
#[derive(Parser)]
struct Cli {
    #[command(subcommand)]
    command: Option<Commands>,
}

// main():
match cli.command {
    None => hub::run().await?,
    Some(Commands::List) => { ... },
    // ...
}
```

**Key detail**: The `after_help` text in the `#[command(...)]` attribute needs updating to document the new default behavior.

**Alternatives considered**:
- Adding a `Hub` subcommand: Forces users to type `kasmos hub` instead of bare `kasmos`.
- Using `#[command(default_subcommand)]` (doesn't exist in clap 4.x).

## R-004: TUI Event Loop — Inside Zellij Considerations

**Decision**: The hub TUI runs a standard ratatui/crossterm event loop inside a Zellij pane, same as the existing orchestration TUI.

**Rationale**: The existing orchestration TUI (`tui/mod.rs`) already runs inside Zellij panes successfully. The same crossterm backend with raw mode and alternate screen works within Zellij's terminal emulation. Key observations:
- Mouse capture (`EnableMouseCapture`) works inside Zellij panes
- crossterm key events are received correctly (Zellij forwards them)
- Alternate screen works (Zellij renders the pane's alternate screen buffer)
- Terminal resize events are forwarded by Zellij

**Caveat**: Some key combinations may be captured by Zellij before reaching the application. The hub should avoid key combos that conflict with Zellij defaults (e.g., `Ctrl+p` for Zellij's pane mode). The existing TUI already handles this — hub keybindings should follow the same conventions.

## R-005: Feature Scanner — Lock File and PID Liveness Check

**Decision**: Check `.kasmos/run.lock` in the feature's worktree directory for orchestration status. Verify PID liveness with `kill(pid, 0)`.

**Rationale**: The existing `start.rs` already implements `acquire_lock()` and `is_pid_alive()` using this exact pattern. The hub scanner reuses this logic:
1. For each feature in `kitty-specs/`, check if a `.worktrees/<feature>/.kasmos/run.lock` or `<feature-dir>/.kasmos/run.lock` exists
2. If exists, parse the PID and check `libc::kill(pid, 0) == 0`
3. If PID alive → orchestration running; if PID dead → stale lock (ignore)

Combine with `ZellijCli::list_sessions()` to check for `kasmos-<feature>` sessions (for attach capability).

**Alternatives considered**:
- Only checking Zellij sessions: Misses orchestration status details (which wave, WP progress).
- Using a Unix socket or IPC: Overkill for status detection.

## R-006: Hub Logging — TUI and tracing Coexistence

**Decision**: The hub does NOT initialize tracing logging. Since it's a standalone TUI (not running alongside an orchestration engine), there's no tracing output to manage. Errors are displayed inline in the TUI.

**Rationale**: The existing orchestration TUI initializes `tui-logger` to capture tracing events into the Logs tab. The hub has no logs tab and no engine — it only needs to display errors from Zellij/OpenCode command failures. These can be shown as status messages or popup dialogs within the TUI.

If the hub is launched with `RUST_LOG` set, tracing to stderr would corrupt the TUI. The hub should either skip `init_logging()` or initialize a file-based subscriber for debugging.

**Alternatives considered**:
- Adding a logs panel to the hub: Overkill — the hub has very few events to log.
- Using `tui-logger`: Only needed if the hub had a persistent log view.
