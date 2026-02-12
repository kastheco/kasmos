---
work_package_id: WP12
title: "TUI Launch Integration — Wire tui::run() into Runtime"
lane: "doing"
dependencies:
  - WP01
  - WP02
base_branch: master
created_at: '2026-02-12T02:20:00+00:00'
subtasks:
  - T063
  - T064
  - T065
  - T066
  - T067
  - T068
phase: "Phase 4 - Integration"
assignee: 'unassigned'
agent: "reviewer"
shell_pid: "805216"
review_status: ''
reviewed_by: ''
history:
  - timestamp: '2026-02-12T02:20:00Z'
lane: doing
    agent: system
    shell_pid: ''
    action: "Manual WP created — TUI runtime wiring gap identified during 006 orchestration"
---

# Work Package Prompt: WP12 — TUI Launch Integration

## Problem Statement

The entire TUI module (`crates/kasmos/src/tui/`) is compiled and tested (WP01-WP10 all done), but **never executes at runtime**. Three specific gaps prevent the TUI from running:

1. **No `watch::channel`**: The engine (`engine.rs`) mutates `Arc<RwLock<OrchestrationRun>>` directly but never broadcasts state to a `tokio::sync::watch` channel. `tui::run()` requires `watch::Receiver<OrchestrationRun>` (`tui/mod.rs:85`).

2. **Controller pane launches OpenCode, not the TUI**: `layout.rs:340-366` (`build_controller_pane`) generates a Zellij pane that runs `ocx oc` — an OpenCode session. Nothing launches the kasmos TUI binary.

3. **No `kasmos tui` subcommand**: There is no CLI entry point that calls `tui::run()`. The TUI needs either a subcommand (`kasmos tui`) that the controller pane launches, or the TUI must run in-process inside the `start.rs` orchestration pipeline before Zellij attach.

## Objectives & Success Criteria

- `kasmos start <feature>` shows the ratatui TUI in the controller pane (not an OpenCode session)
- The TUI receives live state updates as WPs transition between states
- The TUI can send `EngineAction` commands to the engine (quit, tab switch, future: approve/reject)
- The TUI runs inside Zellij without terminal corruption (raw mode + alternate screen inside a Zellij pane)
- Pressing `q` in the TUI exits cleanly and restores the terminal
- All existing tests pass
- `cargo build` succeeds with no warnings in new code

## Context & Constraints

- **TUI entry point**: `tui/mod.rs:84-115` — `pub async fn run(watch_rx, action_tx)` — fully implemented, needs channels.
- **Engine**: `engine.rs:35-65` — `WaveEngine` holds `Arc<RwLock<OrchestrationRun>>`, mutates via `.write().await`. No watch broadcast exists.
- **Start pipeline**: `start.rs:348-426` — Phase 3 creates channels (`command_tx`, `engine_action_tx`, `completion_tx`, `launch_tx`), spawns engine, command bridge, and wave handler. No TUI spawn exists.
- **Controller pane layout**: `layout.rs:340-366` — `build_controller_pane` runs `ocx oc` with cwd set to `feature_dir`.
- **Config**: `config.rs:53` — `controller_width_pct: u32` (default 40), `opencode_binary: String` (default "ocx").
- **CLI**: `main.rs:44-80` — Commands enum has Start/Status/Cmd/Attach/Stop. No `Tui` variant.
- **Zellij pane constraints**: Zellij panes are terminal emulators. Raw mode + alternate screen work inside panes, but the TUI process must be a standalone binary or subcommand that Zellij launches.

## Design Decision: In-Process vs Subcommand

Two approaches are viable:

### Option A — `kasmos tui` subcommand (Recommended)

The controller pane runs `kasmos tui --session <name>` which connects to the running engine via IPC (unix socket or shared state file). This is simpler for Zellij but requires an IPC mechanism.

### Option B — In-process TUI before Zellij attach

`start.rs` runs the TUI directly in the main process instead of attaching to Zellij. The TUI and engine share the same tokio runtime, channels are in-process. This avoids IPC but means the TUI replaces the interactive Zellij attach (`start.rs:604-612`).

**Problem with Option B**: The operator can't see individual WP panes while the TUI is running because the TUI takes over the terminal. The TUI would need to run *inside* a Zellij pane alongside the agent panes.

### Option C — Hybrid: in-process TUI inside Zellij controller pane

`start.rs` creates the Zellij session with a controller pane that runs `kasmos tui --run-dir .kasmos`. The main process (`kasmos start`) creates the session, spawns the engine as a background daemon, then the controller pane's `kasmos tui` connects to the engine via shared state (mmap'd file or unix socket for the watch channel, and FIFO for actions).

**Recommended approach: Option C with unix socket IPC.**

The engine runs as a daemon process (spawned by `kasmos start`). The TUI connects to it via a unix socket at `.kasmos/engine.sock`. The socket carries:
- Engine → TUI: serialized `OrchestrationRun` snapshots (JSON, sent on every state change)
- TUI → Engine: serialized `EngineAction` commands

This cleanly separates the TUI process from the engine process and works naturally with Zellij's pane model.

**Simpler alternative for v1**: Skip the socket. The TUI reads state from `.kasmos/state.json` (already written by `StatePersister` on every mutation) and writes commands to `.kasmos/cmd.pipe` (the existing FIFO). This requires no new IPC — just poll the state file and write to the FIFO. The watch channel becomes a file-polling loop inside the TUI.

## Subtasks & Detailed Guidance

### T063 — Add `kasmos tui` subcommand

**Purpose**: CLI entry point that launches the TUI connected to a running orchestration.

**Steps**:
1. Add `Tui` variant to `Commands` enum in `main.rs`:
   ```rust
   /// Launch the TUI controller for a running orchestration
   Tui {
       /// Feature spec ID or prefix
       feature: Option<String>,
       /// Path to .kasmos directory (auto-detected if not specified)
       #[arg(long)]
       run_dir: Option<String>,
   },
   ```

2. Create `crates/kasmos/src/tui_cmd.rs`:
   ```rust
   pub async fn run(feature: Option<&str>, run_dir: Option<&str>) -> Result<()> {
       // 1. Resolve .kasmos directory (from feature arg or cwd)
       // 2. Load OrchestrationRun from state.json
       // 3. Create channels (state polling + FIFO writing)
       // 4. Call kasmos::tui::run(watch_rx, action_tx)
   }
   ```

3. Wire in `main.rs`:
   ```rust
   Commands::Tui { feature, run_dir } => {
       tui_cmd::run(feature.as_deref(), run_dir.as_deref())
           .await
           .context("TUI failed")?;
   }
   ```

**Files**: `main.rs`, `tui_cmd.rs` (new)

### T064 — State file polling adapter (file → watch channel)

**Purpose**: Bridge the persisted `state.json` into a `tokio::sync::watch` channel that `tui::run()` consumes.

**Steps**:
1. In `tui_cmd.rs`, create a state polling task:
   ```rust
   async fn poll_state(
       state_path: PathBuf,
       watch_tx: watch::Sender<OrchestrationRun>,
   ) {
       let mut last_modified = None;
       loop {
           if let Ok(metadata) = tokio::fs::metadata(&state_path).await {
               let modified = metadata.modified().ok();
               if modified != last_modified {
                   last_modified = modified;
                   if let Ok(data) = tokio::fs::read_to_string(&state_path).await {
                       if let Ok(run) = serde_json::from_str::<OrchestrationRun>(&data) {
                           let _ = watch_tx.send(run);
                       }
                   }
               }
           }
           tokio::time::sleep(Duration::from_millis(250)).await;
       }
   }
   ```

2. Alternatively, use `notify` (already a dependency for `CompletionDetector`) to watch `state.json` for changes instead of polling. This gives sub-second updates.

**Files**: `tui_cmd.rs`

**Notes**: The `StatePersister` already writes `state.json` on every engine mutation (`engine.rs:158-165`). The polling interval of 250ms matches the TUI tick rate.

### T065 — Action channel adapter (TUI actions → FIFO)

**Purpose**: Bridge `EngineAction` from the TUI's `mpsc::Sender` to the existing `.kasmos/cmd.pipe` FIFO.

**Steps**:
1. In `tui_cmd.rs`, create an action bridge task:
   ```rust
   async fn bridge_actions(
       mut action_rx: mpsc::Receiver<EngineAction>,
       fifo_path: PathBuf,
   ) {
       while let Some(action) = action_rx.recv().await {
           let cmd_str = match action {
               EngineAction::Restart(id) => format!("restart {}", id),
               EngineAction::Pause(id) => format!("pause {}", id),
               EngineAction::Resume(id) => format!("resume {}", id),
               EngineAction::Advance => "advance".to_string(),
               EngineAction::Abort => "abort".to_string(),
               EngineAction::ForceAdvance(id) => format!("force-advance {}", id),
               EngineAction::Retry(id) => format!("retry {}", id),
           };
           // Write to FIFO (same format as `kasmos cmd`)
           if let Err(e) = write_to_fifo(&fifo_path, &cmd_str).await {
               tracing::error!("Failed to write action to FIFO: {}", e);
           }
       }
   }
   ```

2. Reuse the FIFO writing logic from `cmd.rs` / `sendmsg.rs`.

**Files**: `tui_cmd.rs`, potentially `sendmsg.rs` (extract shared write function)

### T066 — Change controller pane to launch `kasmos tui`

**Purpose**: Replace the `ocx oc` command in the controller pane layout with `kasmos tui`.

**Steps**:
1. In `layout.rs`, update `build_controller_pane`:
   ```rust
   fn build_controller_pane(&self, feature_dir: &Path) -> KdlNode {
       let mut pane = KdlNode::new("pane");
       pane.entries_mut().push(kdl_str_prop(
           "size",
           &format!("{}%", self.controller_width_pct),
       ));
       pane.entries_mut().push(kdl_str_prop("name", "controller"));
       pane.entries_mut()
           .push(kdl_bool_prop("start_suspended", false));

       let mut cwd = KdlNode::new("cwd");
       cwd.entries_mut()
           .push(kdl_str_arg(&feature_dir.display().to_string()));
       pane.ensure_children().nodes_mut().push(cwd);

       // Launch kasmos TUI instead of ocx oc
       let mut command = KdlNode::new("command");
       command.entries_mut().push(kdl_str_arg("kasmos"));
       pane.ensure_children().nodes_mut().push(command);

       let mut args = KdlNode::new("args");
       args.entries_mut().push(kdl_str_arg("tui"));
       pane.ensure_children().nodes_mut().push(args);

       pane
   }
   ```

2. Update `build_terminal_pane` similarly if it also launches `ocx oc`.

3. Update layout tests that assert on the controller pane command.

**Files**: `layout.rs`

**Notes**: The `kasmos` binary must be in PATH when Zellij launches the pane. This is true in normal usage since the user already ran `kasmos start`. Consider adding `--run-dir` to point at the `.kasmos` directory explicitly in case cwd detection fails.

### T067 — Handle Zellij + raw mode interaction

**Purpose**: Ensure the ratatui TUI renders correctly inside a Zellij pane (not the root terminal).

**Steps**:
1. Verify that `EnterAlternateScreen` works inside Zellij panes. Zellij panes support alternate screen, but behavior may differ from bare terminal.

2. Test mouse capture: `EnableMouseCapture` may conflict with Zellij's own mouse handling. If so, disable mouse capture when running inside Zellij:
   ```rust
   let inside_zellij = std::env::var("ZELLIJ").is_ok();
   if !inside_zellij {
       execute!(stdout, EnableMouseCapture)?;
   }
   ```

3. Test terminal resize events: Zellij sends `SIGWINCH` when panes resize. Verify crossterm's `EventStream` captures these correctly.

4. Test that pressing `q` exits the TUI cleanly and returns the pane to a shell (or closes the pane if run as the pane command).

**Files**: `tui/mod.rs` (setup_terminal, restore_terminal)

### T068 — Tests for TUI launch integration

**Purpose**: Verify the state polling, action bridging, and subcommand wiring.

**Steps**:
1. Unit test: `poll_state` sends updated `OrchestrationRun` on `watch_tx` when `state.json` changes.
2. Unit test: `bridge_actions` writes correct FIFO command strings for each `EngineAction` variant.
3. Unit test: `kasmos tui --help` shows correct usage.
4. Unit test: `build_controller_pane` generates `kasmos tui` command instead of `ocx oc`.
5. Integration test (manual): `kasmos start 002` shows the ratatui TUI in the controller pane with live-updating WP states.

**Files**: `tui_cmd.rs` (tests), `layout.rs` (update existing tests)

## Risks & Mitigations

- **Zellij + alternate screen**: Zellij panes support alternate screen but some terminal escape sequences may behave differently. Mitigation: test early, fall back to non-alternate-screen rendering if needed.
- **Mouse capture conflict**: Zellij captures mouse events for its own UI. Mitigation: detect `$ZELLIJ` env var and disable mouse capture inside Zellij panes.
- **State file race condition**: The engine may write `state.json` while the TUI is reading it. Mitigation: `StatePersister` already uses atomic writes (write to temp file + rename). The TUI's JSON parse failure is non-fatal (skip and retry next poll).
- **FIFO blocking**: Writing to the FIFO blocks if no reader is connected. The engine's `CommandReader` should already be running. Mitigation: use `O_NONBLOCK` on the FIFO write side and handle `EAGAIN`.
- **`kasmos` binary not in PATH**: The Zellij pane needs to find `kasmos`. Mitigation: use `std::env::current_exe()` to get the absolute path and embed it in the layout.

## Review Guidance

- Verify `cargo build -p kasmos` compiles with zero warnings
- Verify `cargo test -p kasmos` — all existing tests pass + new tests
- Manual test: `kasmos start <feature>` shows the TUI in the controller pane
- Manual test: WP state changes appear in the TUI within 1 second
- Manual test: press `q` to exit TUI, verify terminal is restored cleanly
- Manual test: TUI renders correctly when Zellij pane is resized

## Activity Log

- 2026-02-12T10:22:54Z – coder – lane=for_review – Submitted for review via swarm
- 2026-02-12T10:22:55Z – reviewer – shell_pid=805216 – lane=doing – Started review via workflow command
