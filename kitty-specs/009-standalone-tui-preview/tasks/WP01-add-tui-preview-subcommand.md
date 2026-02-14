---
work_package_id: "WP01"
subtasks:
  - "T001"
  - "T002"
  - "T003"
  - "T004"
  - "T005"
  - "T006"
title: "Add kasmos tui Subcommand with Animated Mock Data"
phase: "Phase 1 - Implementation"
lane: "done"
assignee: "claude"
agent: "reviewer"
shell_pid: "3010557"
review_status: "approved"
reviewed_by: "claude"
dependencies: []
history:
  - timestamp: "2026-02-12T23:49:43Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP01 - Add kasmos tui Subcommand with Animated Mock Data

## Important: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** - Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

No dependencies -- create worktree from master:

```bash
spec-kitty implement WP01
```

---

## Objectives & Success Criteria

Add a `kasmos tui` subcommand that launches the existing ratatui TUI with animated mock data. No Zellij, no git, no orchestration engine, no spec files required.

**Success criteria:**
- `cargo build -p kasmos` compiles with zero warnings in new code
- `cargo run -p kasmos -- tui` launches TUI with 12 animated mock WPs
- `cargo run -p kasmos -- tui --count 3` launches TUI with 3 WPs
- `cargo run -p kasmos -- tui --count 0` prints clap validation error
- Pressing `Alt+q` quits cleanly with terminal restored
- WPs cycle through states every ~3 seconds (deterministic)
- When all WPs complete, animation resets and loops
- All three TUI tabs render correctly (Dashboard, Review, Logs)
- TUI keybindings (approve, reject, restart, etc.) don't panic or error
- `cargo test -p kasmos` passes with no regressions
- Total new code is under ~120 lines

## Context & Constraints

**Architecture**: The TUI is fully decoupled from Zellij. It accepts `watch::Receiver<OrchestrationRun>` for state and `mpsc::Sender<EngineAction>` for commands. We exploit this by feeding mock data through the watch channel and dropping the action receiver (a no-op sink).

**Key references:**
- Spec: `kitty-specs/009-standalone-tui-preview/spec.md`
- Plan: `kitty-specs/009-standalone-tui-preview/plan.md`
- Data model: `kitty-specs/009-standalone-tui-preview/data-model.md`
- Research: `kitty-specs/009-standalone-tui-preview/research.md`
- CLI contract: `contracts/cli-contract.md` (repo-level canonical contract)
- TUI entry point: `crates/kasmos/src/tui/mod.rs:85-123`
- Existing test mock: `crates/kasmos/src/tui/app.rs:912-948`
- Types: `crates/kasmos/src/types.rs`
- EngineAction: `crates/kasmos/src/command_handlers.rs:16-41`
- Config: `crates/kasmos/src/config.rs` (has `Default` impl)

**Constraints:**
- Zero changes to any file in `crates/kasmos/src/tui/`
- Zero new crate dependencies
- Binary-only module: `tui_preview.rs` declared via `mod tui_preview;` in `main.rs`, NOT exported in `lib.rs`
- Deterministic animation (no `rand` crate): round-robin WP selection, `tick_count % 7 == 0` for ~14.3% failure path
- Drop the `mpsc::Receiver` immediately -- TUI keybindings use `let _ = try_send(...)` which handles closed channels

## Subtasks & Detailed Guidance

### Subtask T001 - Add `mod tui_preview` and `Tui` Variant to Commands Enum

**Purpose**: Register the new subcommand with clap so `kasmos tui [--count N]` is recognized by the CLI parser.

**Steps**:
1. Open `crates/kasmos/src/main.rs`
2. Add `mod tui_preview;` after the existing module declarations (after line 13, alongside `mod stop;`)
3. Add the `Tui` variant to the `Commands` enum (after the `Stop` variant, before the closing brace):

```rust
/// Launch the TUI with animated mock data (no orchestration)
Tui {
    /// Number of simulated work packages (minimum: 1)
    #[arg(long, default_value = "12", value_parser = clap::value_parser!(usize).range(1..))]
    count: usize,
},
```

**Files**: `crates/kasmos/src/main.rs` (modify, ~5 lines added)

**Validation**:
- [ ] `Tui` variant compiles with clap derive macros
- [ ] `--count 0` is rejected by the value_parser range constraint
- [ ] `--count` defaults to 12 when omitted

---

### Subtask T002 - Add Match Arm for `Commands::Tui` in main()

**Purpose**: Wire the new subcommand to the preview module's entry point.

**Steps**:
1. In `crates/kasmos/src/main.rs`, find the `match cli.command` block (line 91)
2. Add a new match arm after the `Commands::Stop` arm (before the closing brace of the match):

```rust
Commands::Tui { count } => {
    tui_preview::run(count)
        .await
        .context("TUI preview failed")?;
}
```

**Files**: `crates/kasmos/src/main.rs` (modify, ~4 lines added)

**Validation**:
- [ ] `cargo build -p kasmos` compiles (once T004 creates the module)
- [ ] Running `kasmos tui` invokes `tui_preview::run(12)`

---

### Subtask T003 - Update `after_help` Text with `kasmos tui`

**Purpose**: Make `kasmos tui` discoverable in the CLI help output.

**Steps**:
1. In `crates/kasmos/src/main.rs`, find the `after_help` string in the `#[command(...)]` attribute (starts at line 20)
2. Add a line in the "Quick Start" section:

```
  kasmos tui                            Preview TUI with animated mock data
```

3. Optionally add to the "Typical Workflow" section:

```
  kasmos tui                           Preview/iterate on TUI without orchestration
```

**Files**: `crates/kasmos/src/main.rs` (modify, ~2 lines added)

**Validation**:
- [ ] `cargo run -p kasmos -- --help` shows `kasmos tui` in both Quick Start and command list

---

### Subtask T004 - Create `tui_preview.rs` with `run()` Entry Point

**Purpose**: The module entry point that sets up channels, spawns the animation task, and launches the TUI.

**Steps**:
1. Create `crates/kasmos/src/tui_preview.rs`
2. Add imports:

```rust
use std::time::Duration;
use anyhow::Result;
use tokio::sync::{mpsc, watch};
use kasmos::command_handlers::EngineAction;
```

3. Implement `pub async fn run(count: usize) -> Result<()>`:

```rust
pub async fn run(count: usize) -> Result<()> {
    let initial_run = generate_mock_run(count);
    let (watch_tx, watch_rx) = watch::channel(initial_run.clone());
    let (action_tx, _action_rx) = mpsc::channel::<EngineAction>(64);
    // _action_rx is dropped here -- TUI try_send() calls silently fail

    tokio::spawn(animation_loop(watch_tx, initial_run));

    kasmos::tui::run(watch_rx, action_tx).await
}
```

**Key design decisions:**
- `_action_rx` is dropped immediately. The TUI sends `EngineAction` via `action_tx.try_send()` which returns `Err(Closed)`, but all call sites use `let _ = try_send(...)` so this is safe.
- `initial_run` is cloned: one copy for the watch channel seed, one for the animation loop (used for cycle reset).
- `animation_loop` is spawned as a detached task. When the TUI exits (`tui::run` returns), the tokio runtime shuts down and the animation task is cancelled.

**Files**: `crates/kasmos/src/tui_preview.rs` (new file, ~15 lines for this function)

**Validation**:
- [ ] Function compiles and is callable from main.rs match arm
- [ ] TUI launches and receives the initial mock state
- [ ] Animation loop runs in background
- [ ] Pressing `q` exits the TUI cleanly (animation task is cancelled by runtime shutdown)

---

### Subtask T005 - Implement `generate_mock_run()`

**Purpose**: Create a realistic `OrchestrationRun` with `count` work packages across 3 waves, with mixed initial states that showcase all kanban lanes immediately.

**Steps**:
1. In `crates/kasmos/src/tui_preview.rs`, implement:

```rust
fn generate_mock_run(count: usize) -> kasmos::OrchestrationRun
```

2. **Static title list** -- define a `const` or `static` array of realistic dev task names. Examples:
   - "Initialize project scaffolding"
   - "Add CLI argument parser"
   - "Implement state machine"
   - "Create database schema"
   - "Build REST API endpoints"
   - "Design component layout"
   - "Add authentication flow"
   - "Write integration tests"
   - "Configure CI pipeline"
   - "Optimize query performance"
   - "Add error handling middleware"
   - "Implement WebSocket handler"
   - Use modular indexing: `TITLES[i % TITLES.len()]` to handle any count

3. **Wave assignment**: Divide WPs into 3 waves:
   - `wave_size = count.div_ceil(3)` (use integer ceiling division)
   - WP index `i` belongs to wave `(i / wave_size).min(2)`
   - Generate `Wave` structs with `wp_ids` collecting the IDs per wave

4. **Dependencies**: 
   - Wave 0 WPs: no dependencies
   - Wave 1 WPs: depend on all wave 0 WP IDs
   - Wave 2 WPs: depend on all wave 1 WP IDs

5. **Initial states** (to populate all kanban lanes on startup):
   - Wave 0 WPs default to `WPState::Active` with `started_at = Some(SystemTime::now())`
   - Wave 1 and 2 WPs default to `WPState::Pending` with `started_at = None`
   - Override specific wave 0 WPs (if count >= 3):
     - Index 0: `WPState::Completed` (set `completed_at`, `completion_method = Some(CompletionMethod::AutoDetected)`)
     - Index 1: `WPState::Failed` (set `failure_count = 1`)
     - Index 2: `WPState::ForReview`
   - If count < 3, assign states to whatever WPs exist (at least one Active)

6. **OrchestrationRun fields**:
   - `id`: `"preview-run-1".to_string()`
   - `feature`: `"preview-demo".to_string()`
   - `feature_dir`: `PathBuf::from("/tmp/kasmos-preview")`
   - `config`: `kasmos::Config::default()`
   - `state`: `kasmos::RunState::Running`
   - `started_at`: `Some(SystemTime::now())`
   - `completed_at`: `None`
   - `mode`: `kasmos::ProgressionMode::WaveGated`

7. **Wave states**: Derive from constituent WPs using the wave state derivation rules from data-model.md:
   - All Pending -> `WaveState::Pending`
   - Any Active/ForReview -> `WaveState::Active`
   - All Completed -> `WaveState::Completed`
   - Any Failed (and no Active) -> `WaveState::PartiallyFailed`

**Files**: `crates/kasmos/src/tui_preview.rs` (~35-45 lines for this function + title list)

**Edge cases**:
- `count = 1`: Single WP in wave 0, state Active. Only 1 wave populated, other 2 waves empty (or adjust wave logic to handle `count < 3` gracefully)
- `count = 2`: Two WPs -- one Completed, one Active. Ensure at least one non-Completed for animation to work
- Large count (e.g., 100): Title list cycles via modulo, wave distribution still works

**Validation**:
- [ ] `generate_mock_run(12)` produces 12 WPs across 3 waves (4/4/4)
- [ ] `generate_mock_run(3)` produces 3 WPs across 3 waves (1/1/1) with Completed, Failed, ForReview states
- [ ] `generate_mock_run(1)` produces 1 WP in Active state without panicking
- [ ] All kanban lanes populated on startup (Pending, Active, ForReview, Completed, Failed)
- [ ] Dependencies correctly wire wave 1 -> wave 0, wave 2 -> wave 1

---

### Subtask T006 - Implement `animation_loop()`

**Purpose**: Background task that deterministically advances WP states every ~3 seconds, cycling through the full state machine, and resets when all complete.

**Steps**:
1. In `crates/kasmos/src/tui_preview.rs`, implement:

```rust
async fn animation_loop(
    watch_tx: watch::Sender<kasmos::OrchestrationRun>,
    initial_run: kasmos::OrchestrationRun,
)
```

2. **Main loop structure**:

```rust
let mut run = initial_run.clone();
let mut tick: usize = 0;

loop {
    tokio::time::sleep(Duration::from_secs(3)).await;

    // Find next non-Completed WP (round-robin)
    // Advance its state
    // Update wave states
    // Broadcast
    // Check if all completed -> reset

    tick += 1;
}
```

3. **WP selection** (deterministic round-robin):
   - Collect indices of non-Completed WPs
   - If empty, all are completed (handle reset)
   - Select: `target_idx = tick % non_completed.len()`
   - Get the WP at that position

4. **State transitions** (deterministic):
   ```
   Pending   -> Active      (set started_at = Some(SystemTime::now()))
   Active    -> Failed      if tick % 7 == 0 (set failure_count += 1)
   Active    -> ForReview   if tick % 7 != 0
   ForReview -> Completed   (set completed_at = Some(SystemTime::now()),
                              completion_method = Some(CompletionMethod::AutoDetected))
   Failed    -> Active      (retry: set started_at = Some(SystemTime::now()))
   ```

5. **Wave state derivation** after each transition:
   - For each wave, examine all WPs in that wave
   - Apply the derivation rules from data-model.md:
     - All Pending -> `WaveState::Pending`
     - Any Active or ForReview -> `WaveState::Active`
     - All Completed -> `WaveState::Completed`
     - Any Failed and no Active/ForReview -> `WaveState::PartiallyFailed`

6. **Broadcast**: `let _ = watch_tx.send(run.clone());` -- the `let _ =` silently handles the case where the TUI has exited and the receiver is dropped.

7. **Cycle reset** (when all WPs reach Completed):
   - Set `run.state = RunState::Completed`
   - Set `run.completed_at = Some(SystemTime::now())`
   - Broadcast the completed state: `let _ = watch_tx.send(run.clone());`
   - Sleep 2 seconds: `tokio::time::sleep(Duration::from_secs(2)).await;`
   - Reset: `run = initial_run.clone();`
   - Reset tick counter: `tick = 0;`
   - Broadcast the reset state: `let _ = watch_tx.send(run.clone());`
   - Continue loop (do not increment tick on reset iteration)

8. **Graceful exit**: When the TUI exits, `watch_tx.send()` returns `Err` (receiver dropped). The `let _ =` handles this. The animation task continues running until the tokio runtime shuts down, which happens when `main()` returns after `tui::run()` completes. No explicit cancellation needed.

**Files**: `crates/kasmos/src/tui_preview.rs` (~30-40 lines for this function)

**Edge cases**:
- All WPs start Completed (e.g., if `generate_mock_run` produces a degenerate state): Immediate reset on first tick
- Single WP (`count = 1`): Cycles through Active -> ForReview -> Completed -> reset
- `tick % 7 == 0` on first Active WP: That WP goes to Failed, next tick it retries

**Validation**:
- [ ] WPs advance states every ~3 seconds
- [ ] State transitions follow the deterministic state machine exactly
- [ ] Wave states update correctly after each WP transition
- [ ] `tick % 7 == 0` produces Failed transitions (~14.3% of Active transitions)
- [ ] Cycle resets with 2-second pause when all WPs complete
- [ ] Animation loops forever until `q` is pressed
- [ ] No panics when TUI exits (watch_tx.send errors silently ignored)

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| `tui_logger::init_logger` double-call | Preview module does NOT call it -- `tui::run()` handles logger init internally |
| `watch_tx.send()` fails after TUI exits | Use `let _ = watch_tx.send(...)` -- silently ignore errors |
| `mpsc::Receiver` dropped causes `try_send` errors | TUI keybindings already use `let _ = try_send(...)` -- no panic, no log noise |
| `count = 0` crashes mock generation | Prevented by clap `value_parser!(usize).range(1..)` -- never reaches `run()` |
| Wave division by zero for small counts | Use `count.div_ceil(3)` and `.min(2)` guards |

## Review Guidance

**Key acceptance checkpoints for `/spec-kitty.review`:**

1. **Zero changes to `crates/kasmos/src/tui/`**: Diff should show NO modifications in the `tui/` directory
2. **No forbidden imports in `tui_preview.rs`**: Must NOT import from `zellij`, `git`, `session`, `engine`, `detector`, `parser`, or `start` modules
3. **Code size**: Total new code should be ~80-120 lines across both files
4. **`--count` validation**: Verify `value_parser!(usize).range(1..)` is used (not manual validation)
5. **Deterministic animation**: Confirm `tick % 7 == 0` is the failure check (not `rand`)
6. **Channel safety**: Verify `_action_rx` is dropped (not kept alive) and all `watch_tx.send()` calls use `let _ =`
7. **Cycle reset**: Verify 2-second pause between completion and reset
8. **Build clean**: `cargo build -p kasmos` with zero warnings; `cargo test -p kasmos` passes

## Activity Log

- 2026-02-12T23:49:43Z - system - lane=planned - Prompt created.
- 2026-02-13T01:09:44Z – claude – shell_pid=2882823 – lane=doing – Implementation complete, moving to doing
- 2026-02-13T01:09:56Z – claude – shell_pid=2883561 – lane=for_review – All subtasks complete, submitting for code review
- 2026-02-13T01:12:18Z – claude – shell_pid=2883561 – lane=done – Review VERIFIED: all FR/NFR/AC requirements met, zero TUI changes, 263 tests pass
- 2026-02-13T02:00:53Z – claude – shell_pid=3000130 – lane=doing – Started implementation via workflow command
- 2026-02-13T02:03:01Z – claude – shell_pid=3000130 – lane=for_review – Implementation complete: kasmos tui subcommand with animated mock data. cargo build and cargo test pass.
- 2026-02-13T02:03:40Z – reviewer – shell_pid=3010557 – lane=doing – Started review via workflow command
- 2026-02-13T02:08:18Z – reviewer – shell_pid=3010557 – lane=done – Review passed: initial state threshold fixed, all acceptance criteria met. cargo build/test clean.
