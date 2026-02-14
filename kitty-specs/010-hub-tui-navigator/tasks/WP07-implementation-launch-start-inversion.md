---
work_package_id: WP07
title: Implementation Launch & Start Inversion
lane: done
dependencies:
- WP05
subtasks:
- T031
- T032
- T033
- T034
- T035
- T036
- T037
phase: Phase 4 - Actions
assignee: ''
agent: ''
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-13T03:53:23Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-13T04:15:00Z'
  lane: for_review
  agent: claude-sonnet-4-5
  shell_pid: ''
  action: Implementation complete - all subtasks verified
- timestamp: '2026-02-13T12:00:00Z'
  lane: done
  agent: release opencode agent
  shell_pid: ''
  action: Acceptance validation passed
---

# Work Package Prompt: WP07 - Implementation Launch & Start Inversion

## Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** -- Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Markdown Formatting
Wrap HTML/XML tags in backticks: `` `<div>` ``, `` `<script>` ``
Use language identifiers in code blocks: ````rust`, ````bash`

---

## Objectives & Success Criteria

- Implement `StartContinuous` and `StartWaveGated` actions from the hub (new Zellij tab)
- Implement `Attach` action (switch to existing Zellij tab)
- Implement mode selection UX: Enter=continuous, Shift+Enter=wave-gated, >6 WP confirmation dialog
- Invert `kasmos start` to default to TUI mode with `--no-tui` opt-out
- Change `--mode` default from `wave-gated` to `continuous`
- Hide `--tui` flag for backward compatibility
- Implementation launches within 3 seconds (SC-004)
- 100% backward compatibility for existing CLI usage (SC-006)

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-004: kasmos start TUI Inversion, AD-006: Mode Selection UX)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-007, FR-008, FR-011, FR-012, FR-013, User Stories 4 and 5)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md` (InputMode::ConfirmDialog)
- **Dependencies**: WP05 (Zellij wrappers), WP06 (action dispatch pattern)
- **Key source file**: `crates/kasmos/src/start.rs` (lines 53-620) -- the main start entry point to modify

### Key Architectural Decisions

- **AD-004**: Add `--no-tui` flag (default false), change `--mode` default to `continuous`, hide `--tui`
- **AD-006**: Enter=continuous (>6 WP confirmation), Shift+Enter=wave-gated
- When `!no_tui` (default): after creating Zellij session and starting engine, call `tui::run()` instead of `zellij attach`
- When `no_tui`: use existing `zellij attach` behavior (lines 604-616 of `start.rs`)

## Subtasks & Detailed Guidance

### Subtask T031 - Implement StartContinuous action

- **Purpose**: Launch implementation in continuous mode from the hub.
- **Steps**:
  1. In `crates/kasmos/src/hub/actions.rs`, add to `dispatch_action()`:
  2. For `HubAction::StartContinuous { feature_slug }`:
     ```rust
     // Launch kasmos start in a new Zellij tab
     open_new_tab(
         &format!("kasmos-{}", feature_slug),
         "kasmos",
         &["start", &feature_slug, "--mode", "continuous"],
     ).await?;
     ```
  3. The new tab runs `kasmos start <feature>` which (after T037) defaults to TUI mode

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes (independent of T032)
- **Notes**: The `open_new_tab` wrapper from WP05 handles the Zellij tab creation. The tab name `kasmos-<feature>` matches the session naming convention used by `start.rs` (line 302).

### Subtask T032 - Implement StartWaveGated action

- **Purpose**: Launch implementation in wave-gated mode from the hub.
- **Steps**:
  1. For `HubAction::StartWaveGated { feature_slug }`:
     ```rust
     open_new_tab(
         &format!("kasmos-{}", feature_slug),
         "kasmos",
         &["start", &feature_slug, "--mode", "wave-gated"],
     ).await?;
     ```

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes

### Subtask T033 - Implement mode selection UX

- **Purpose**: Handle Enter vs Shift+Enter for mode selection, with >6 WP confirmation.
- **Steps**:
  1. In `crates/kasmos/src/hub/keybindings.rs`, when the selected action is a Start action:
     - **Enter** (`KeyCode::Enter` without Shift): Default to continuous mode
       - If feature has >6 WPs (check `task_progress`), show confirmation dialog
       - Otherwise, dispatch `StartContinuous` immediately
     - **Shift+Enter** (`KeyCode::Enter` with `KeyModifiers::SHIFT`): Always dispatch `StartWaveGated`
  2. The >6 WP check uses the `TaskProgress::InProgress { total, .. }` or counts WPs from the scanner

- **Files**: `crates/kasmos/src/hub/keybindings.rs`
- **Parallel?**: No (depends on T031, T032)
- **Notes**: crossterm reports Shift+Enter as `KeyCode::Enter` with `KeyModifiers::SHIFT`. Test this on the target terminal -- some terminals may not distinguish Shift+Enter from Enter. If Shift+Enter is unreliable, consider an alternative keybinding (e.g., `w` for wave-gated).

### Subtask T034 - Implement ConfirmDialog input mode

- **Purpose**: Show a confirmation dialog when starting continuous mode with >6 WPs.
- **Steps**:
  1. In `crates/kasmos/src/hub/app.rs`, ensure `InputMode::ConfirmDialog` is defined
  2. Add a `pending_action: Option<HubAction>` field to `App` for storing the action to confirm
  3. When the dialog is triggered:
     ```rust
     app.input_mode = InputMode::ConfirmDialog {
         message: format!(
             "This feature has {} WPs. Use wave-gated mode instead? [y/n/Enter=continuous]",
             total_wps
         ),
     };
     app.pending_action = Some(HubAction::StartContinuous { feature_slug: slug.clone() });
     ```
  4. In keybindings, handle `ConfirmDialog` mode:
     - `y` or `Y` -> dispatch `StartWaveGated` instead, return to Normal
     - `n` or `Enter` -> dispatch the pending `StartContinuous`, return to Normal
     - `Esc` -> cancel, return to Normal
  5. Render the dialog as a centered popup overlay:
     ```
     +----------------------------------+
     | This feature has 9 WPs.          |
     | Use wave-gated mode instead?     |
     |                                  |
     | [y] Wave-gated  [n/Enter] Cont.  |
     | [Esc] Cancel                     |
     +----------------------------------+
     ```

- **Files**: `crates/kasmos/src/hub/app.rs`, `crates/kasmos/src/hub/keybindings.rs`
- **Parallel?**: No (depends on T033)
- **Notes**: Use `ratatui::widgets::Paragraph` inside a `ratatui::widgets::Block` with `Borders::ALL` for the popup. Center it using `ratatui::layout::Rect` calculations.

### Subtask T035 - Implement Attach action

- **Purpose**: Switch to an existing orchestration tab.
- **Steps**:
  1. In `crates/kasmos/src/hub/actions.rs`, add to `dispatch_action()`:
  2. For `HubAction::Attach { feature_slug }`:
     ```rust
     go_to_tab(&format!("kasmos-{}", feature_slug)).await?;
     ```
  3. If the tab doesn't exist (go_to_tab fails), show an error status message

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes (independent of T031-T034)
- **Notes**: The tab name `kasmos-<feature>` must match what `start.rs` creates. The `go_to_tab` wrapper from WP05 handles the Zellij command.

### Subtask T036 - Add --no-tui flag and change --mode default

- **Purpose**: Restructure the `Start` command's CLI flags per AD-004.
- **Steps**:
  1. In `crates/kasmos/src/main.rs`, modify the `Start` variant:

**Current** (lines 49-55):
```rust
Start {
    /// Feature spec ID or prefix
    feature: String,
    /// Progression mode: continuous or wave-gated
    #[arg(long, default_value = "wave-gated")]
    mode: String,
},
```

**Target**:
```rust
Start {
    /// Feature spec ID or prefix (e.g. "002" or "002-ratatui-tui-controller-panel")
    feature: String,
    /// Progression mode: continuous or wave-gated
    #[arg(long, default_value = "continuous")]
    mode: String,
    /// Skip TUI dashboard, attach directly to Zellij session
    #[arg(long)]
    no_tui: bool,
    /// [deprecated] TUI is now the default; this flag is accepted for backward compatibility
    #[arg(long, hide = true)]
    tui: bool,
},
```

  2. Update the match arm to pass `no_tui` to `start::run()`:
```rust
Some(Commands::Start { feature, mode, no_tui, tui: _ }) => {
    start::run(&feature, &mode, no_tui)
        .await
        .context("Start failed")?;
}
```

- **Files**: `crates/kasmos/src/main.rs`
- **Parallel?**: Yes (independent of T031-T035)
- **Notes**: The `tui: bool` field is hidden and ignored -- it exists only for backward compatibility (FR-013). The `--mode` default changes from `wave-gated` to `continuous` per AD-004.

### Subtask T037 - Modify start::run() for TUI default

- **Purpose**: Make `start::run()` launch the orchestration TUI by default instead of `zellij attach`.
- **Steps**:
  1. Change `start::run()` signature to accept `no_tui: bool`:
     ```rust
     pub async fn run(feature: &str, mode: &str, no_tui: bool) -> Result<()> {
     ```
  2. Replace the "Phase 4: Attach interactively" section (lines 602-618) with:

```rust
// -- Phase 4: Interactive session --
if no_tui {
    // Legacy behavior: attach directly to Zellij session
    println!("Attaching to session: {}", session_name);
    let attach_status = tokio::process::Command::new(zellij)
        .args(["attach", &session_name])
        .stdin(std::process::Stdio::inherit())
        .stdout(std::process::Stdio::inherit())
        .stderr(std::process::Stdio::inherit())
        .status()
        .await
        .context("Failed to attach to Zellij session")?;

    if !attach_status.success() {
        tracing::warn!("Zellij attach exited with: {}", attach_status);
    }
} else {
    // Default: launch orchestration TUI dashboard
    // Create a watch channel for the engine to send state updates
    let (watch_tx, watch_rx) = tokio::sync::watch::channel(run_arc.read().await.clone());

    // Spawn a task to forward state updates from run_arc to the watch channel
    let tui_run_arc = run_arc.clone();
    let _state_forwarder = tokio::spawn(async move {
        let mut interval = tokio::time::interval(std::time::Duration::from_millis(500));
        loop {
            interval.tick().await;
            let state = tui_run_arc.read().await.clone();
            if watch_tx.send(state).is_err() {
                break; // TUI closed
            }
        }
    });

    // Create an action channel for the TUI to send commands to the engine
    let (tui_action_tx, mut tui_action_rx) = mpsc::channel::<kasmos::EngineAction>(64);

    // Bridge TUI actions to the engine
    let bridge_action_tx = engine_action_tx.clone();
    let _action_bridge = tokio::spawn(async move {
        while let Some(action) = tui_action_rx.recv().await {
            if bridge_action_tx.send(action).await.is_err() {
                break;
            }
        }
    });

    // Run the TUI (blocks until user quits)
    kasmos::tui::run(watch_rx, tui_action_tx).await?;
}
```

  3. Note: The `engine_action_tx` channel already exists in `start.rs` (line 354). The TUI needs a clone of it.
  4. The `watch` channel bridges the `Arc<RwLock<OrchestrationRun>>` to the TUI's expected `watch::Receiver`.

- **Files**: `crates/kasmos/src/start.rs`
- **Parallel?**: No (depends on T036 for the new parameter)
- **Notes**: This is the most complex subtask. The key challenge is bridging the engine's `Arc<RwLock<OrchestrationRun>>` state to the TUI's `watch::Receiver<OrchestrationRun>`. A periodic forwarder task reads the state and sends it through the watch channel. The TUI's `run()` function already handles the watch channel pattern (see `crates/kasmos/src/tui/mod.rs` lines 84-115).

**Important**: The `engine_action_tx` is currently created at line 354 and moved into the `CommandHandler` at line 383. To also give it to the TUI, either:
- Clone it before moving into CommandHandler
- Or restructure to share via Arc

The simplest approach is to clone before the CommandHandler creation:
```rust
let tui_engine_action_tx = engine_action_tx.clone();
// ... existing CommandHandler creation uses engine_action_tx ...
// ... later, TUI uses tui_engine_action_tx ...
```

## Test Strategy

- **Build verification**: `cargo build -p kasmos` succeeds
- **CLI tests**:
  - `kasmos start --help` shows `--no-tui` flag and `--mode` default as `continuous`
  - `kasmos start <feature>` launches TUI (manual verification)
  - `kasmos start <feature> --no-tui` attaches directly (manual verification)
  - `kasmos start <feature> --tui` is silently accepted (no error)
- **Hub action tests**: Trigger Start from hub, verify new tab appears with orchestration

## Risks & Mitigations

- **TUI inversion breaking scripts**: `--no-tui` provides escape hatch; `--tui` silently accepted (FR-013)
- **Shift+Enter detection**: May not work in all terminals. Mitigation: test on target platform; provide alternative keybinding
- **Watch channel bridging**: Periodic forwarder adds slight latency (500ms). Acceptable for TUI updates.
- **engine_action_tx ownership**: Must clone before moving into CommandHandler. Verify channel isn't dropped prematurely.

## Review Guidance

- Verify `--no-tui` flag works correctly (default false = TUI, true = attach)
- Verify `--tui` flag is hidden and silently accepted
- Verify `--mode` default changed from `wave-gated` to `continuous`
- Verify watch channel bridging correctly forwards state to TUI
- Verify engine_action_tx is cloned before CommandHandler takes ownership
- Run `cargo build` and verify `kasmos start --help` output

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T04:15:00Z - claude-sonnet-4-5 - lane=for_review - Implementation complete
- 2026-02-13T12:00:00Z - release opencode agent - lane=done - Acceptance validation passed
