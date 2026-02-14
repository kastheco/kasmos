---
work_package_id: WP05
title: Action Resolution & Zellij Wrappers
lane: done
dependencies:
- WP02
subtasks:
- T021
- T022
- T023
- T024
- T025
phase: Phase 3 - Views & Actions
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
- timestamp: '2026-02-13T06:38:00Z'
  lane: done
  agent: claude-sonnet-4-5
  shell_pid: ''
  action: Completed WP05 implementation
---

# Work Package Prompt: WP05 - Action Resolution & Zellij Wrappers

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

- Define `HubAction` enum with all contextual actions
- Implement `resolve_actions()` that maps `FeatureEntry` state to available actions
- Implement inside-session Zellij wrappers for pane/tab operations (no `--session` flag)
- Implement `NewFeaturePrompt` input mode for creating new features
- Unit tests verify correct action availability for every feature lifecycle state
- Zero incorrect action offerings (SC-002)

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-002, AD-003, AD-006)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-004 through FR-008, FR-018)
- **Data Model**: `kitty-specs/010-hub-tui-navigator/data-model.md` (HubAction table, state transitions)
- **Research**: `kitty-specs/010-hub-tui-navigator/research.md` (R-001: Zellij Pane Direction API, R-002: OpenCode --prompt Flag)
- **Dependencies**: WP02 (FeatureEntry types), WP03 (App struct for InputMode)

### Key Architectural Decisions

- Inside-session Zellij commands use `zellij action ...` without `--session` flag (R-001)
- Zellij wrappers use `tokio::process::Command` directly, not the `ZellijCli` trait
- Action resolution is a pure function: `FeatureEntry` state in, `Vec<HubAction>` out
- `NewFeaturePrompt` is an inline text input in the hub, not a multi-step wizard

## Subtasks & Detailed Guidance

### Subtask T021 - Define HubAction enum

- **Purpose**: Create the action type that represents what the operator can do with a feature.
- **Steps**:
  1. Create `crates/kasmos/src/hub/actions.rs`
  2. Add `pub mod actions;` to `crates/kasmos/src/hub/mod.rs`
  3. Define:

```rust
/// Contextual action available for a feature.
#[derive(Debug, Clone, PartialEq)]
pub enum HubAction {
    /// Open OpenCode pane for spec creation
    CreateSpec { feature_slug: String },
    /// Create a new feature (prompt for name first)
    NewFeature,
    /// Open OpenCode pane for clarification
    Clarify { feature_slug: String },
    /// Open OpenCode pane for planning
    Plan { feature_slug: String },
    /// Open OpenCode pane for task generation
    GenerateTasks { feature_slug: String },
    /// Start implementation in continuous mode
    StartContinuous { feature_slug: String },
    /// Start implementation in wave-gated mode
    StartWaveGated { feature_slug: String },
    /// Attach to running orchestration
    Attach { feature_slug: String },
    /// View feature details
    ViewDetails,
}

impl HubAction {
    /// Human-readable label for display in the TUI.
    pub fn label(&self) -> &str {
        match self {
            Self::CreateSpec { .. } => "Create Spec",
            Self::NewFeature => "New Feature",
            Self::Clarify { .. } => "Clarify",
            Self::Plan { .. } => "Plan",
            Self::GenerateTasks { .. } => "Generate Tasks",
            Self::StartContinuous { .. } => "Start (continuous)",
            Self::StartWaveGated { .. } => "Start (wave-gated)",
            Self::Attach { .. } => "Attach",
            Self::ViewDetails => "View Details",
        }
    }
}
```

- **Files**: `crates/kasmos/src/hub/actions.rs` (new), `crates/kasmos/src/hub/mod.rs` (add module)
- **Parallel?**: No (T022 depends on this)

### Subtask T022 - Implement resolve_actions()

- **Purpose**: Map feature state to available actions per the data model state machine.
- **Steps**:
  1. Implement in `crates/kasmos/src/hub/actions.rs`:

```rust
use super::scanner::{FeatureEntry, SpecStatus, PlanStatus, TaskProgress, OrchestrationStatus};

/// Resolve available actions for a feature based on its current state.
pub fn resolve_actions(entry: &FeatureEntry) -> Vec<HubAction> {
    let mut actions = vec![HubAction::ViewDetails];
    let slug = entry.full_slug.clone();

    match entry.orchestration_status {
        OrchestrationStatus::Running => {
            actions.push(HubAction::Attach { feature_slug: slug });
            return actions;
        }
        _ => {}
    }

    match entry.spec_status {
        SpecStatus::Empty => {
            actions.push(HubAction::CreateSpec { feature_slug: slug });
        }
        SpecStatus::Present => {
            match entry.plan_status {
                PlanStatus::Absent => {
                    actions.push(HubAction::Clarify { feature_slug: slug.clone() });
                    actions.push(HubAction::Plan { feature_slug: slug });
                }
                PlanStatus::Present => {
                    match &entry.task_progress {
                        TaskProgress::NoTasks => {
                            actions.push(HubAction::GenerateTasks { feature_slug: slug });
                        }
                        TaskProgress::InProgress { .. } => {
                            actions.push(HubAction::StartContinuous { feature_slug: slug.clone() });
                            actions.push(HubAction::StartWaveGated { feature_slug: slug });
                        }
                        TaskProgress::Complete { .. } => {
                            // Feature is complete -- no start actions
                        }
                    }
                }
            }
        }
    }

    actions
}
```

  2. The function returns `ViewDetails` for every feature (always available)
  3. When orchestration is running, only `Attach` and `ViewDetails` are available
  4. Otherwise, actions follow the lifecycle: CreateSpec -> Clarify/Plan -> GenerateTasks -> Start

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: No (depends on T021)
- **Notes**: The state machine matches the data model's HubAction table exactly. `Clarify` and `Plan` are both available when spec is present but no plan -- the operator chooses which workflow to run.

### Subtask T023 - Implement inside-session Zellij wrappers

- **Purpose**: Create thin wrappers around `zellij action` commands for use inside a Zellij session.
- **Steps**:
  1. Add to `crates/kasmos/src/hub/actions.rs`:

```rust
use tokio::process::Command;

/// Open a new pane to the right with a command.
pub async fn open_pane_right(
    name: &str,
    command: &str,
    args: &[&str],
    cwd: Option<&str>,
) -> anyhow::Result<()> {
    let mut cmd_args = vec!["action", "new-pane", "--direction", "right"];
    if !name.is_empty() {
        cmd_args.push("--name");
        cmd_args.push(name);
    }
    if let Some(dir) = cwd {
        cmd_args.push("--cwd");
        cmd_args.push(dir);
    }
    cmd_args.push("--");
    cmd_args.push(command);
    cmd_args.extend_from_slice(args);

    let output = Command::new("zellij")
        .args(&cmd_args)
        .output()
        .await?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        anyhow::bail!("zellij action new-pane failed: {}", stderr);
    }
    Ok(())
}

/// Open a new tab with a command.
pub async fn open_new_tab(
    name: &str,
    command: &str,
    args: &[&str],
) -> anyhow::Result<()> {
    let mut cmd_args: Vec<String> = vec![
        "action".to_string(),
        "new-tab".to_string(),
    ];
    if !name.is_empty() {
        cmd_args.push("--name".to_string());
        cmd_args.push(name.to_string());
    }
    // To run a command in the new tab, we use layout-based approach
    // or run the command after tab creation
    // For simplicity, use: zellij action new-tab --name <name> --cwd <cwd>
    // Then the command runs in the default shell
    // Alternative: use zellij run in the new tab
    // Actually, `zellij action new-tab` doesn't support `-- command`.
    // We need: zellij run --name <name> -- <command> <args>
    // But that runs in the current tab. For a new tab with a command,
    // we can use a layout string or a different approach.
    //
    // Correct approach: `zellij action new-tab` creates the tab,
    // then we need to run the command in it. But since we want the
    // tab to BE the command, use:
    // `zellij action new-tab --layout <inline-layout>`
    // Or simpler: just run `kasmos start <feature>` which creates its own session.
    //
    // For the hub, the simplest approach is:
    // 1. Write a minimal KDL layout to a temp file
    // 2. `zellij action new-tab --layout <path> --name <name>`
    //
    // Even simpler for kasmos start: since kasmos start creates its own
    // Zellij session, the hub should just run the command and let it handle
    // session creation. But we want it in a NEW TAB of the current session.
    //
    // Resolution: Use `zellij run` which opens a new pane with the command.
    // For a new tab: write a temp layout file.

    // For now, create tab then run command approach:
    let tab_args: Vec<&str> = cmd_args.iter().map(|s| s.as_str()).collect();
    let output = Command::new("zellij")
        .args(&tab_args)
        .output()
        .await?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        anyhow::bail!("zellij action new-tab failed: {}", stderr);
    }

    // Run the command in the new tab
    if !command.is_empty() {
        let mut run_args = vec!["run", "--"];
        run_args.push(command);
        run_args.extend_from_slice(args);
        let _ = Command::new("zellij")
            .args(&run_args)
            .output()
            .await?;
    }

    Ok(())
}

/// Switch to an existing tab by name.
pub async fn go_to_tab(name: &str) -> anyhow::Result<()> {
    let output = Command::new("zellij")
        .args(["action", "go-to-tab-name", name])
        .output()
        .await?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        anyhow::bail!("zellij action go-to-tab-name failed: {}", stderr);
    }
    Ok(())
}

/// Query all tab names in the current session.
pub async fn query_tab_names() -> anyhow::Result<Vec<String>> {
    let output = Command::new("zellij")
        .args(["action", "query-tab-names"])
        .output()
        .await?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        anyhow::bail!("zellij action query-tab-names failed: {}", stderr);
    }

    let stdout = String::from_utf8_lossy(&output.stdout);
    Ok(stdout
        .lines()
        .map(|l| l.trim().to_string())
        .filter(|l| !l.is_empty())
        .collect())
}
```

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes (independent of T022)
- **Notes**: These wrappers use `zellij` directly (not `config.zellij_binary`) since the hub runs inside Zellij where the binary is always available. The `open_new_tab` function is more complex because `zellij action new-tab` doesn't directly support `-- command` syntax. The implementer should research the best approach for their Zellij version -- options include temp KDL layouts or `zellij run` after tab creation.

### Subtask T024 - Implement NewFeaturePrompt input mode

- **Purpose**: Allow the operator to type a new feature name before launching spec creation.
- **Steps**:
  1. In `crates/kasmos/src/hub/app.rs`, ensure `InputMode::NewFeaturePrompt { input: String }` is defined (from T012)
  2. In `crates/kasmos/src/hub/keybindings.rs`, add handling for `InputMode::NewFeaturePrompt`:
     - Character keys -> append to `input`
     - `Backspace` -> remove last character
     - `Enter` -> finalize: create the feature directory, dispatch `CreateSpec` action
     - `Esc` -> cancel: return to `InputMode::Normal`
  3. In `crates/kasmos/src/hub/app.rs`, add rendering for the prompt:
     - Show an input field at the bottom of the screen: "New feature name: [input_text]|"
     - Show cursor position
  4. When Enter is pressed with a valid name:
     - Create `kitty-specs/<NNN>-<slug>/` directory (auto-assign next number)
     - Dispatch `CreateSpec` action for the new feature

- **Files**: `crates/kasmos/src/hub/keybindings.rs`, `crates/kasmos/src/hub/app.rs`
- **Parallel?**: Yes (independent of T021-T023)
- **Notes**: The feature number should be auto-assigned as the next available number (scan existing features, find max, add 1, zero-pad to 3 digits). The slug is derived from the user's input (lowercase, spaces to hyphens, strip special chars).

### Subtask T025 - Write action resolution unit tests

- **Purpose**: Verify correct action availability for every feature lifecycle state.
- **Steps**:
  1. Add `#[cfg(test)] mod tests` in `crates/kasmos/src/hub/actions.rs`
  2. Write tests for each state combination from the data model:

```rust
#[test]
fn test_empty_spec_offers_create_spec() {
    let entry = make_entry(SpecStatus::Empty, PlanStatus::Absent, TaskProgress::NoTasks, OrchestrationStatus::None);
    let actions = resolve_actions(&entry);
    assert!(actions.contains(&HubAction::CreateSpec { feature_slug: "001-test".to_string() }));
    assert!(actions.contains(&HubAction::ViewDetails));
    assert_eq!(actions.len(), 2);
}

#[test]
fn test_spec_present_no_plan_offers_clarify_and_plan() {
    let entry = make_entry(SpecStatus::Present, PlanStatus::Absent, TaskProgress::NoTasks, OrchestrationStatus::None);
    let actions = resolve_actions(&entry);
    assert!(actions.iter().any(|a| matches!(a, HubAction::Clarify { .. })));
    assert!(actions.iter().any(|a| matches!(a, HubAction::Plan { .. })));
}

#[test]
fn test_plan_present_no_tasks_offers_generate_tasks() {
    let entry = make_entry(SpecStatus::Present, PlanStatus::Present, TaskProgress::NoTasks, OrchestrationStatus::None);
    let actions = resolve_actions(&entry);
    assert!(actions.iter().any(|a| matches!(a, HubAction::GenerateTasks { .. })));
}

#[test]
fn test_tasks_in_progress_offers_start() {
    let entry = make_entry(SpecStatus::Present, PlanStatus::Present, TaskProgress::InProgress { done: 1, total: 5 }, OrchestrationStatus::None);
    let actions = resolve_actions(&entry);
    assert!(actions.iter().any(|a| matches!(a, HubAction::StartContinuous { .. })));
    assert!(actions.iter().any(|a| matches!(a, HubAction::StartWaveGated { .. })));
}

#[test]
fn test_running_offers_only_attach() {
    let entry = make_entry(SpecStatus::Present, PlanStatus::Present, TaskProgress::InProgress { done: 1, total: 5 }, OrchestrationStatus::Running);
    let actions = resolve_actions(&entry);
    assert!(actions.iter().any(|a| matches!(a, HubAction::Attach { .. })));
    assert!(!actions.iter().any(|a| matches!(a, HubAction::StartContinuous { .. })));
}

#[test]
fn test_complete_offers_only_view_details() {
    let entry = make_entry(SpecStatus::Present, PlanStatus::Present, TaskProgress::Complete { total: 5 }, OrchestrationStatus::None);
    let actions = resolve_actions(&entry);
    assert_eq!(actions, vec![HubAction::ViewDetails]);
}
```

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: No (depends on T021-T022)

## Test Strategy

- **Unit tests**: Action resolution tests in `crates/kasmos/src/hub/actions.rs`
- **Run**: `cargo test -p kasmos -- hub::actions`
- **Coverage**: Every `HubAction` variant must be tested as both present and absent
- **Manual testing**: Verify Zellij wrappers work inside a Zellij session

## Risks & Mitigations

- **Zellij command failures**: All wrappers return `Result` -- errors displayed in TUI status bar
- **Shell injection**: Validate feature slugs before passing to Zellij commands (reuse `validate_identifier` from `crates/kasmos/src/zellij.rs`)
- **New tab with command**: Zellij's `new-tab` doesn't support `-- command` directly -- may need temp layout file approach

## Review Guidance

- Verify action resolution matches data-model.md state machine exactly
- Verify Zellij wrappers don't use `--session` flag (inside-session commands)
- Verify input validation on NewFeaturePrompt (no shell metacharacters)
- Run `cargo test -p kasmos -- hub::actions`

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T06:38:00Z - claude-sonnet-4-5 - lane=done - Completed WP05 implementation
