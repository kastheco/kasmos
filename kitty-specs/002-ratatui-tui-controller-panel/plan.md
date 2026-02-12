# Implementation Plan: Ratatui TUI Controller Panel

**Branch**: `002-ratatui-tui-controller-panel` | **Date**: 2026-02-10 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `kitty-specs/002-ratatui-tui-controller-panel/spec.md`

## Summary

Replace the passive controller pane with an interactive ratatui TUI that provides a kanban dashboard, review workflow, notification bar, and log viewer. The TUI runs inside the existing Zellij controller pane as the sole operator interface, sends commands via the existing `mpsc<EngineAction>` channel, and receives state updates via a new `tokio::sync::watch` channel broadcasting `OrchestrationRun` snapshots. At `for_review`, kasmos triggers a tiered review runner (slash-command mode or built-in prompt mode), captures results, and feeds them back into the Review tab and orchestration state.

## Planning Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| State flow (engine→TUI) | `tokio::sync::watch` | Zero-cost when idle, instant updates, no polling overhead |
| TUI architecture | Direct state mutation on `App` struct | Simpler, less boilerplate; complexity managed by clear module boundaries |
| Controller pane | TUI only | kasmos TUI IS the operator interface; no shell toggle |
| Rendering stack | ratatui + crossterm | Rust-native, matches existing ecosystem |
| Command dispatch (TUI→engine) | Reuse existing `mpsc<EngineAction>` channel | TUI becomes a peer producer alongside FIFO |
| FIFO compatibility | Keep existing FIFO unchanged | Both TUI and FIFO send to same CommandHandler→EngineAction pipeline |
| Review trigger mode | Configurable: `slash` or `prompt` | Supports `/kas:verify`-style workflows and model-agnostic fallback |
| Review execution model | Default `openai/gpt-5.3-codex` (high reasoning) | Provides a deterministic default while keeping model override support |

## Technical Context

**Language/Version**: Rust (edition 2024, workspace)
**Primary Dependencies**: ratatui = "0.29", crossterm = "0.28" (pinned for deterministic builds; ratatui re-export preferred where applicable)
**Storage**: Hybrid — runtime state in `Arc<RwLock<OrchestrationRun>>`, persisted review results in `.kasmos/review-results.json` (atomic write on update, load on startup for restart consistency).
**Testing**: `cargo test` — unit tests for App state logic, rendering snapshots via `ratatui::backend::TestBackend`
**Target Platform**: Linux terminal (256-color, Unicode box-drawing)
**Project Type**: Single Rust crate (extends existing `kasmos` crate)
**Performance Goals**: <100ms input latency, 30fps render, handles 50+ WPs
**Constraints**: Must not block the tokio runtime; crossterm event reader on dedicated task
**Scale/Scope**: 3 tabs (Dashboard, Review, Logs), ~10 new source files, ~2000-3000 LOC

**Runtime Integrations**:
- `opencode` runner for implementation/review agents
- Optional slash-command plugin flow (`/kas:verify` / `/kas:review`) when available in pane environment

## Constitution Check

**Constitution Source**: `.kittify/memory/constitution.md`

**Compliance Verification**:
- **Rust 2024**: Aligned - project uses Rust edition 2024
- **Tests Required**: Aligned - all features must have corresponding tests via `cargo test`
- **Non-blocking TUI/Event Loop**: Aligned - async event handling, no render loop blocking
- **Stack Alignment**: Aligned - ratatui/tokio/Zellij matches constitution standards
- **Platform Support**: Aligned - Linux primary, macOS best-effort

**Status**: Compliant. Test coverage is mandatory and included in task breakdown (WP10).

## Project Structure

### Documentation (this feature)

```
kitty-specs/002-ratatui-tui-controller-panel/
├── plan.md              # This file
├── research.md          # Phase 0: ratatui patterns, watch channel design
├── data-model.md        # Phase 1: App state model, notification types
└── tasks.md             # Phase 2 output (NOT created by /spec-kitty.plan)
```

### Source Code (repository root)

```
crates/kasmos/src/
├── tui/                    # NEW — TUI module
│   ├── mod.rs              # TUI runner: terminal setup, event loop, watch integration
│   ├── app.rs              # App state struct: tabs, selection, notifications, scroll positions
│   ├── event.rs            # Event types: Key, Mouse, Tick, Render, StateUpdate
│   ├── tabs/
│   │   ├── mod.rs          # Tab enum + trait
│   │   ├── dashboard.rs    # Kanban board: WPs grouped by lane, wave info, action buttons
│   │   ├── review.rs       # Review queue: WPs in for_review, approve/reject/request-changes
│   │   └── logs.rs         # Scrollable, filterable orchestration event log
│   ├── widgets/
│   │   ├── mod.rs
│   │   ├── notification_bar.rs  # Persistent cross-tab notification strip
│   │   ├── wp_card.rs           # WP display card with state badge + actions
│   │   └── action_buttons.rs    # Contextual action buttons per WP state
│   └── keybindings.rs      # Keymap definitions: vim nav, tab switch, WP actions
├── engine.rs               # MODIFIED — add watch::Sender for state broadcasts
├── command_handlers.rs     # UNCHANGED — TUI sends EngineAction directly
├── commands.rs             # UNCHANGED — FIFO remains parallel input
├── types.rs                # MINOR — add Notification struct, InputNeeded signal types
├── lib.rs                  # MODIFIED — re-export tui module
└── launch.rs (binary)      # MODIFIED — wire TUI into startup, pass watch/action channels
```

**Structure Decision**: All TUI code lives in a `tui/` submodule within the existing kasmos crate. No new crate needed — the TUI is tightly coupled to kasmos types and channels. The `tui/` module encapsulates all rendering, event handling, and UI state.

## Architecture

### Channel Topology

```
                    ┌─────────────────┐
                    │  WaveEngine     │
                    │                 │
  completion_rx ──▶ │  handles events │ ──▶ watch_tx.send(run.clone())
  action_rx ──────▶ │  mutates state  │     (after every state change)
                    └─────────────────┘
                           ▲
                           │ mpsc<EngineAction>
              ┌────────────┴────────────┐
              │                         │
     ┌────────┴──────┐        ┌────────┴──────┐
     │ CommandHandler │        │   TUI App     │
     │ (from FIFO)   │        │ (direct send) │
     └───────┬───────┘        └───────┬───────┘
             │                        │
    mpsc<ControllerCommand>    crossterm events
             │                   + watch_rx
     ┌───────┴───────┐        (state snapshots)
     │ CommandReader  │
     │ (FIFO pipe)    │
     └────────────────┘
```

### TUI Event Loop (async, in dedicated tokio task)

```
loop {
    tokio::select! {
        // 1. Crossterm terminal events (keys, mouse, resize)
        Some(event) = crossterm_events.next() => {
            handle_input(event, &mut app, &action_tx);
        }
        // 2. State updates from engine via watch channel
        Ok(()) = watch_rx.changed() => {
            app.update_state(watch_rx.borrow().clone());
        }
        // 3. Tick for animations/elapsed time updates
        _ = tick_interval.tick() => {
            app.on_tick();
        }
    }
    terminal.draw(|f| app.render(f))?;
    if app.should_quit { break; }
}
```

### App State Model

```rust
struct App {
    // State from engine (updated via watch)
    run: OrchestrationRun,

    // UI state
    active_tab: Tab,
    notifications: Vec<Notification>,

    // Per-tab state
    dashboard: DashboardState,  // selected_wp, scroll offset
    review: ReviewState,        // selected_review_item, scroll offset
    logs: LogsState,            // entries, filter text, scroll offset

    // Control
    action_tx: mpsc::Sender<EngineAction>,
    should_quit: bool,
}
```

### Key Integration Points

1. **launch.rs step 16.5** (new): Create `watch::channel(initial_run)`, pass `watch_tx` to WaveEngine, pass `watch_rx` to TUI
2. **engine.rs**: After every `handle_completion()` and `handle_action()`, call `watch_tx.send(run.read().clone())`
3. **launch.rs step 17.5** (new): Spawn TUI task with `watch_rx`, `action_tx.clone()`, terminal handle
4. **TUI sends `EngineAction` directly** — bypasses CommandHandler/FIFO for lower latency
5. **FIFO remains untouched** — external scripts still work, both produce to same `action_rx`

### Review Automation Pipeline

```
CompletionDetector (lane=for_review)
        │
        ▼
WaveEngine transitions WP -> ForReview
        │
        ▼
ReviewRunner enqueues WP review job
        │
        ├── slash mode: inject "/kas:verify" (or configured command) into reviewer pane
        │
        └── prompt mode: run opencode reviewer prompt (default model gpt-5.3-codex, reasoning high)
        │
        ▼
ReviewResultStore persists result -> TUI Review tab + Notifications + Logs
```

### Review Automation Config (proposed)

```toml
[review]
enabled = true
trigger_lane = "for_review"
mode = "prompt"                     # "slash" | "prompt"
slash_command = "/kas:verify"
fallback_to_prompt = true
agent = "reviewer"
model = "openai/gpt-5.3-codex"
reasoning = "high"
timeout_seconds = 900
policy = "auto_then_manual_approve" # manual_only | auto_then_manual_approve | auto_and_mark_done
```

### Contextual Actions per WP State

| WP State | Available Actions |
|----------|-------------------|
| Pending | (none — waiting for deps/wave) |
| Active | Pause |
| Paused | Resume |
| Failed | Restart, Retry, Force-Advance |
| Completed | (none) |
| for_review (lane) | Approve, Reject, Request Changes |

*Wave-gated mode adds global "Advance" action at wave boundaries.*

### Notification Types

| Type | Trigger | Action |
|------|---------|--------|
| Review Pending | WP enters `for_review` lane | Jump to Review tab, focus WP |
| Failure | WP enters `Failed` state | Jump to Dashboard, focus failed WP |
| Input Needed | Agent writes `.input-needed` marker | Focus/zoom agent's Zellij pane |

### Keybindings

| Key | Action |
|-----|--------|
| `1`/`2`/`3` | Switch tabs (Dashboard/Review/Logs) |
| `j`/`k` | Navigate WP list up/down |
| `h`/`l` | Navigate between lanes (Dashboard) |
| `Enter` | Select WP / activate action |
| `a` | Approve (Review tab) |
| `r` | Reject (Review tab) |
| `c` | Request Changes (Review tab) |
| `R` | Restart selected WP |
| `P` | Pause/Resume toggle |
| `F` | Force-advance |
| `T` | Retry |
| `A` | Advance wave (wave-gated) |
| `n` | Jump to next notification |
| `/` | Filter logs (Logs tab) |
| `q` | Quit TUI |

## New Dependencies

Add to `crates/kasmos/Cargo.toml`:
```toml
ratatui = "0.29"
crossterm = "0.28"
```

## Complexity Tracking

*No constitution violations to track.*
