# Feature Specification: Standalone TUI Preview Mode

**Feature ID**: `009-standalone-tui-preview`
**Created**: 2026-02-12
**Status**: Draft
**Predecessor**: `002-ratatui-tui-controller-panel` (TUI foundation), `008-wire-tui-mode` (TUI wiring)

## Problem Statement

There is no way to launch the kasmos TUI without starting a full orchestration — Zellij session, git worktrees, spec parsing, agent spawning, and wave engine execution. This makes iterating on the TUI painful: every change requires triggering a real feature implementation just to see the UI render.

Developers working on TUI improvements (layout tweaks, new widgets, color schemes, keybinding changes) need a fast feedback loop that shows the TUI with realistic data cycling through all possible states, without any external dependencies.

## Scope

Add a `kasmos tui` subcommand that launches the TUI with animated mock data. No Zellij, no git, no specs, no orchestration — just the UI with fake work packages cycling through states on a timer.

### In Scope

- New `Tui` subcommand variant in the CLI with a `--count` flag
- New `tui_preview` module that generates mock `OrchestrationRun` data and animates state transitions
- Background animation loop that advances WP states every ~3 seconds
- Automatic cycle reset when all WPs reach Completed

### Out of Scope

- Changes to the existing TUI module (`src/tui/`)
- Changes to the orchestration engine, wave engine, or start flow
- Persistent state or configuration for preview mode
- Network, git, or Zellij interactions of any kind
- Custom animation speed flag (hardcoded ~3s is sufficient for v1)

## Architecture Context

The TUI (`src/tui/mod.rs:85-123`) is already perfectly decoupled from Zellij and the orchestration engine. Its `run()` function accepts exactly two channels:

```rust
pub async fn run(
    mut watch_rx: watch::Receiver<OrchestrationRun>,
    action_tx: mpsc::Sender<EngineAction>,
) -> anyhow::Result<()>
```

- `watch::Receiver<OrchestrationRun>` — state updates in (TUI reads)
- `mpsc::Sender<EngineAction>` — commands out (TUI writes)

Zero Zellij imports exist in the TUI module. The existing test helper `create_test_run()` in `app.rs:912-948` already proves mock `OrchestrationRun` construction works. The preview module simply needs to:

1. Create a `watch::channel` with an initial mock `OrchestrationRun`
2. Create an `mpsc::channel` (receiver is dropped — actions are discarded)
3. Spawn a background task that mutates and broadcasts state updates
4. Call `tui::run(watch_rx, action_tx)` — the existing entry point, unchanged

## Requirements

### Functional Requirements

- **FR-001**: `kasmos tui` MUST launch the ratatui TUI with mock data and no external dependencies (no Zellij, no git, no spec files, no orchestration engine).
- **FR-002**: The `--count <N>` flag MUST control the number of simulated work packages (default: 12). Values below 1 MUST be rejected with a clap `value_parser` constraint (e.g., `value_parser = clap::value_parser!(usize).range(1..)`).
- **FR-003**: Mock work packages MUST be distributed across 3 waves with realistic titles, IDs (`WP01`..`WP{count}`), and inter-wave dependencies.
- **FR-004**: The initial state MUST include a mix of WP states: some Pending, some Active, some Completed, one Failed, one ForReview — to immediately showcase all kanban lanes.
- **FR-005**: A background animation loop MUST advance one WP's state every ~3 seconds following the transition chain: `Pending -> Active -> ForReview -> Completed`.
- **FR-006**: Active WPs MUST have a ~15% chance of transitioning to `Failed` instead of `ForReview`, to exercise the failure display path.
- **FR-007**: Wave states MUST be updated to reflect their constituent WPs (Pending, Active, Completed, PartiallyFailed).
- **FR-008**: When all WPs reach `Completed`, the animation MUST reset all WPs back to their initial states and loop forever.
- **FR-009**: All three TUI tabs MUST work: Dashboard (kanban with WP cards), Review (ForReview queue), and Logs (state transition log entries).
- **FR-010**: Pressing `q` MUST quit the TUI cleanly (same as production TUI behavior).
- **FR-011**: The `EngineAction` channel receiver MUST be present but actions MUST be silently discarded (no-op sink). TUI keybindings that send actions (approve, reject, restart, etc.) should not error.
- **FR-012**: The run state MUST be set to `Running` during animation and `Completed` briefly when all WPs finish before resetting.

### Non-Functional Requirements

- **NFR-001**: The preview module MUST NOT import anything from `zellij`, `git`, `session`, `engine`, `detector`, `parser`, or `start` modules.
- **NFR-002**: The preview module MUST NOT perform any filesystem I/O beyond what the TUI terminal setup requires.
- **NFR-003**: The existing `tui::run()` function MUST be called without modification — zero changes to `src/tui/`.
- **NFR-004**: Total new code MUST be under ~120 lines (the module is intentionally minimal).
- **NFR-005**: The preview MUST work on any terminal that supports the existing TUI (crossterm alternate screen).

## Clarifications

### Session 2026-02-12

- Q: Should CLI commands be tracked as contracts? → A: Yes. Add a `## CLI Contract` section with structured tables covering the entire kasmos CLI scope, not just `kasmos tui`. → Extracted to repo-level `contracts/cli-contract.md`.

## CLI Contract

> **Canonical contract**: [`contracts/cli-contract.md`](../../../contracts/cli-contract.md)
> **Freshness check**: `scripts/check-cli-contract.sh`

The kasmos CLI is a public interface contract. Changes to subcommand names, flag names, defaults, or argument semantics are breaking changes and must be versioned accordingly.

### Top-Level Commands

| Subcommand | Description | Since |
|------------|-------------|-------|
| `kasmos list` | List unfinished feature specs from `kitty-specs/` | v0.1 |
| `kasmos start <feature>` | Start orchestration for a feature | v0.1 |
| `kasmos status [feature]` | Show orchestration status | v0.1 |
| `kasmos cmd [--feature <f>] <command>` | Send controller command via FIFO | v0.1 |
| `kasmos attach <feature>` | Attach to existing Zellij session | v0.1 |
| `kasmos stop [feature]` | Gracefully stop orchestration | v0.1 |
| `kasmos tui [--count <N>]` | Launch TUI with animated mock data (**NEW**) | v0.2 |

### `kasmos list`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| *(none)* | — | — | — | — | Prints "No kitty-specs/ directory found." if `kitty-specs/` missing |

### `kasmos start`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `<feature>` | String (positional) | Yes | — | Must match exactly one `kitty-specs/###-*` directory by prefix | Errors on zero matches ("no prefix match found") or multiple matches ("Ambiguous feature") |
| `--mode` | String | No | `"wave-gated"` | Accepted: `"continuous"`, `"wave-gated"` | Clap rejects unrecognized values |
| `--tui` | Flag (bool) | No | `false` | — | — |

### `kasmos status`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `[feature]` | String (positional, optional) | No | Auto-detect from `.kasmos/` in cwd | Same prefix matching as `start` when provided | Same resolution errors as `start` |

### `kasmos cmd`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `--feature` | String (optional) | No | Auto-detect from cwd | Same prefix matching as `start` when provided | Same resolution errors |
| `<command>` | Subcommand | Yes | — | Must be valid FifoCommand variant | Clap rejects unknown subcommands |

**FIFO Subcommands:**

| Subcommand | Argument | Type | Description |
|------------|----------|------|-------------|
| `status` | — | — | Display orchestration state table |
| `restart <wp_id>` | `wp_id` | String | Restart a failed/crashed WP |
| `pause <wp_id>` | `wp_id` | String | Pause a running WP |
| `resume <wp_id>` | `wp_id` | String | Resume a paused WP |
| `focus <wp_id>` | `wp_id` | String | Navigate to WP pane |
| `zoom <wp_id>` | `wp_id` | String | Focus and zoom pane to full view |
| `abort` | — | — | Gracefully shutdown orchestration |
| `advance` | — | — | Confirm wave advancement (wave-gated) |
| `finalize` | — | — | Mark orchestration completed |
| `force-advance <wp_id>` | `wp_id` | String | Skip failed WP, unblock dependents |
| `retry <wp_id>` | `wp_id` | String | Re-run a failed WP |
| `approve <wp_id>` | `wp_id` | String | Approve a WP in review |
| `reject <wp_id>` | `wp_id` | String | Reject a WP (relaunch for rework) |
| `help` | — | — | Show command help |

All `wp_id` arguments follow the format `WP##` (e.g., `WP01`, `WP12`). FIFO commands require an active orchestration session with a valid `.kasmos/cmd.pipe`; error: "No command pipe found" or "No active kasmos command reader" if session not running.

### `kasmos attach`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `<feature>` | String (positional) | Yes | — | Same prefix matching as `start` | Same resolution errors |

### `kasmos stop`

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `[feature]` | String (positional, optional) | No | Auto-detect from `.kasmos/` in cwd | Same prefix matching as `start` when provided | Same resolution errors |

### `kasmos tui` (NEW — this feature)

| Argument | Type | Required | Default | Validation | Error Behavior |
|----------|------|----------|---------|------------|----------------|
| `--count` | `usize` | No | `12` | Must be ≥ 1. Clap rejects non-numeric values. | Clap error on non-numeric; values of 0 should clamp to 1 or error (see FR-002) |

**No external dependencies**: Does not require Zellij, git, `kitty-specs/`, or a running orchestration session.

## Detailed Design

### CLI Changes (`main.rs`)

Add a `Tui` variant to the `Commands` enum:

```rust
/// Launch the TUI with animated mock data (no orchestration)
Tui {
    /// Number of simulated work packages
    #[arg(long, default_value = "12")]
    count: usize,
},
```

Add `mod tui_preview;` declaration and a match arm:

```rust
Commands::Tui { count } => {
    tui_preview::run(count).await.context("TUI preview failed")?;
}
```

Update the `after_help` text to include `kasmos tui`.

### Preview Module (`tui_preview.rs`)

**`pub async fn run(count: usize)`** — entry point:

1. Generate an `OrchestrationRun` with `count` WPs across 3 waves
2. Create `watch::channel(initial_run)` and `mpsc::channel(64)`
3. Spawn `tokio::spawn(animation_loop(watch_tx, initial_run_clone))`
4. Drop the `mpsc::Receiver` (actions silently discarded)
5. Call `kasmos::tui::run(watch_rx, action_tx).await`

**`fn generate_mock_run(count: usize) -> OrchestrationRun`**:

- WP IDs: `WP01` through `WP{count}` (zero-padded to 2 digits)
- Titles: Realistic development task names (e.g., "Add CLI argument parser", "Implement state machine", "Write integration tests")
- Wave assignment: WPs divided into 3 waves (wave 0: first third, wave 1: second third, wave 2: final third)
- Dependencies: Wave 1 WPs depend on wave 0 WPs; wave 2 WPs depend on wave 1 WPs
- Initial states: Wave 0 WPs start Active, wave 1 WPs start Pending, wave 2 WPs start Pending. Override one wave-0 WP to Completed, one to Failed, one to ForReview (if count allows)
- `started_at`: Set to `SystemTime::now()` for Active/Completed/Failed/ForReview WPs
- `feature`: `"preview-demo"`
- `feature_dir`: `PathBuf::from("/tmp/kasmos-preview")`
- `config`: `Config::default()`
- `mode`: `ProgressionMode::WaveGated`
- `state`: `RunState::Running`

**`async fn animation_loop(watch_tx, initial_run)`**:

```
loop {
    sleep(~3 seconds)
    pick a random non-Completed WP
    advance its state:
        Pending -> Active (set started_at)
        Active -> ForReview (85%) or Failed (15%)
        ForReview -> Completed (set completed_at)
        Failed -> Active (retry)
    update wave states based on constituent WPs
    watch_tx.send(updated_run)
    if all WPs Completed:
        sleep(2 seconds)  // brief pause to show completion
        reset to initial_run
        watch_tx.send(initial_run)
}
```

### Key Types Used

| Type | Location | Role |
|------|----------|------|
| `OrchestrationRun` | `src/types.rs:14-45` | Root state struct sent via watch channel |
| `WorkPackage` | `src/types.rs:51-91` | Individual WP with id, title, state, wave, timing |
| `WPState` | `src/types.rs:119-140` | Pending, Active, Completed, Failed, Paused, ForReview |
| `RunState` | `src/types.rs:151-172` | Initializing, Running, Paused, Completed, Failed, Aborted |
| `Wave` | `src/types.rs:97-107` | Wave index, wp_ids, state |
| `WaveState` | `src/types.rs:174-190` | Pending, Active, Completed, PartiallyFailed |
| `EngineAction` | `src/command_handlers.rs:16-41` | Commands the TUI can send (discarded in preview) |
| `Config` | `src/config.rs:17-54` | Runtime config with `Default` impl |
| `ProgressionMode` | `src/types.rs:193-201` | Continuous or WaveGated |

### Files Changed

| File | Change | Lines |
|------|--------|-------|
| `crates/kasmos/src/main.rs` | Add `Tui` variant, `mod tui_preview`, match arm, help text | ~15 |
| `crates/kasmos/src/tui_preview.rs` | **New file** — mock data generation + animation loop | ~80-100 |

### Files NOT Changed

- `src/tui/mod.rs` — no changes
- `src/tui/app.rs` — no changes
- `src/tui/event.rs` — no changes
- `src/tui/keybindings.rs` — no changes
- `src/tui/tabs/` — no changes
- `src/tui/widgets/` — no changes
- `src/start.rs` — no changes
- `src/engine.rs` — no changes
- `src/lib.rs` — no changes (tui_preview is a binary-only module, not a library export)
- All other orchestration modules — no changes

## Usage

```bash
# Launch with default 12 WPs
kasmos tui

# Launch with custom WP count
kasmos tui --count 25

# Launch with minimal WPs
kasmos tui --count 3
```

## Acceptance Criteria

1. **AC-01**: `kasmos tui` launches the TUI with mock data and no Zellij/git/orchestration dependencies.
2. **AC-02**: WPs animate through state transitions automatically every ~3 seconds.
3. **AC-03**: All three TUI tabs work (Dashboard kanban, Review queue, Logs with state transition entries).
4. **AC-04**: The `--count` flag controls the number of simulated WPs.
5. **AC-05**: Pressing `q` quits cleanly with terminal restoration.
6. **AC-06**: The animation cycle resets when all WPs reach Completed.
7. **AC-07**: No changes to existing TUI code (`src/tui/`) or orchestration logic.
8. **AC-08**: `cargo build` succeeds with no warnings in the new code.
9. **AC-09**: `cargo test` passes with no regressions.
10. **AC-10**: TUI keybindings that dispatch `EngineAction` (approve, reject, restart, etc.) do not panic or error — actions are silently dropped.

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `tui_logger::init_logger` called twice (preview + TUI) | Low | Crash | TUI's `run()` already handles this; preview doesn't call it separately |
| `watch_tx.send()` fails if TUI exits | Low | Animation task panics | Use `let _ = watch_tx.send(...)` to ignore send errors |
| `mpsc::Receiver` dropped causes `action_tx.try_send()` to return `Err` | Low | None — already handled | TUI keybindings use `let _ = app.action_tx.try_send(...)` which discards errors. No panic, no log noise. |
| Mock data doesn't exercise all TUI code paths | Low | Missed rendering bugs | Initial state includes all WP states; animation cycles through all transitions |
