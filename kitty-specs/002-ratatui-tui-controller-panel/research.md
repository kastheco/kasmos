# Research: Ratatui TUI Controller Panel

## R1: Ratatui + Tokio Async Integration

**Decision**: Use `crossterm::event::EventStream` in a dedicated tokio task, merged with `tokio::sync::watch` state updates via `tokio::select!`.

**Rationale**: Crossterm's `EventStream` provides a `futures::Stream` that integrates natively with tokio's select. This avoids blocking the runtime. The ratatui async counter tutorial confirms this as the standard pattern. The TUI event task polls three sources:
1. `crossterm_events.next()` — terminal input
2. `watch_rx.changed()` — engine state updates
3. `tick_interval.tick()` — periodic redraws for elapsed timers

**Alternatives considered**:
- `crossterm::event::poll()` in a loop (synchronous) — blocks tokio, rejected
- Separate thread for TUI (non-async) — loses ability to share tokio channels, rejected

## R2: Watch Channel for State Broadcasting

**Decision**: `tokio::sync::watch::channel<OrchestrationRun>` — engine sends full state snapshot after each mutation.

**Rationale**: Watch channel is ideal because:
- Only the latest value matters (TUI always wants current state, not history)
- Single producer (engine), single consumer (TUI) — watch supports this exactly
- `watch::Receiver::changed()` is cancel-safe and works in `tokio::select!`
- Zero overhead when TUI hasn't polled yet (engine just overwrites)

The `OrchestrationRun` struct derives `Clone` (via serde), so sending a clone is straightforward. For 50 WPs this is ~few KB — negligible.

**Alternatives considered**:
- `broadcast` channel — keeps history, TUI doesn't need it, wastes memory
- Polling `Arc<RwLock<>>` on tick — constant CPU, 100ms latency, unnecessary contention
- `mpsc` events per state change — TUI would need to reconstruct full state

## R3: Direct State Pattern for TUI App

**Decision**: `App` struct with direct field mutation. No message enum intermediary.

**Rationale**: The TUI has ~3 tabs with straightforward state (selected index, scroll offset, filter text). Event handlers mutate `App` fields directly. This matches ratatui's demo app pattern and avoids Elm-style boilerplate (Action enum, update match, etc.) for a UI where most interactions are simple state toggles.

Complexity is managed by putting tab-specific state into sub-structs (`DashboardState`, `ReviewState`, `LogsState`) and tab-specific rendering into separate modules.

**Alternatives considered**:
- Elm-style with `Action` enum — adds indirection for every keystroke, overkill for this scope
- Component trait with dynamic dispatch — over-abstraction for 3 fixed tabs

## R4: Terminal Lifecycle in Zellij Pane

**Decision**: The TUI runs inside an existing Zellij pane (controller pane). Terminal setup/teardown uses crossterm's `EnterAlternateScreen` / `enable_raw_mode` and reverse on exit.

**Rationale**: The TUI is launched as the process running in the controller pane. Zellij provides the PTY; crossterm+ratatui handle the terminal protocol within it. No Zellij plugin API needed — the TUI is a regular terminal application from Zellij's perspective.

On panic/crash: Install a panic hook that restores terminal state before unwinding, preventing corrupted terminal output.

## R5: FIFO Coexistence

**Decision**: Keep FIFO command reader unchanged. Both FIFO and TUI send `EngineAction` to the same `action_rx` channel. TUI sends directly; FIFO goes through `CommandReader` → `CommandHandler` → `action_tx`.

**Rationale**: The existing FIFO pipeline is well-tested and used by spec-kitty scripts. The TUI doesn't replace it — it's an additional input source. Since `mpsc::Sender` is `Clone`, the TUI just holds its own clone of `action_tx`.

For `Status` and `Focus`/`Zoom` commands (which don't map to `EngineAction`), the TUI handles them internally — it already has the full state via `watch_rx` and can call `SessionManager` for focus/zoom.

## R6: Notification System

**Decision**: `Vec<Notification>` on `App` state, derived from comparing previous and current `OrchestrationRun` snapshots on each watch update.

**Rationale**: Notifications are purely a UI concept — the engine doesn't need to know about them. On each state update, the TUI diffs:
- New WPs entering `for_review` → add ReviewPending notification
- New WPs entering `Failed` → add Failure notification
- New `.input-needed` marker files → add InputNeeded notification

Notifications are dismissed when:
- WP leaves the triggering state
- Operator explicitly jumps to the notification

This keeps notification logic entirely in the TUI layer.

## R7: Input-Needed Signal Detection

**Decision**: Agent writes a `.input-needed` marker file in the WP worktree. TUI polls for these files on each tick (~1s interval).

**Rationale**: Matches the existing completion detection pattern (file markers in worktrees). The marker file can contain the agent's question/message as plaintext content. The TUI reads the file content and displays it in the notification. When the agent resumes, it deletes the marker.

File path: `{worktree_path}/.input-needed`
