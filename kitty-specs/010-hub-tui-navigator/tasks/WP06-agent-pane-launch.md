---
work_package_id: WP06
title: Agent Pane Launch
lane: done
dependencies:
- WP05
subtasks:
- T026
- T027
- T028
- T029
- T030
phase: Phase 4 - Actions
assignee: ''
agent: claude-sonnet-4-5
shell_pid: ''
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-13T03:53:23Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
- timestamp: '2026-02-13T08:15:00Z'
  lane: for_review
  agent: claude-sonnet-4-5
  shell_pid: ''
  action: Completed WP06 implementation - all subtasks verified
- timestamp: '2026-02-13T12:00:00Z'
  lane: done
  agent: release opencode agent
  shell_pid: ''
  action: Acceptance validation passed
---

# Work Package Prompt: WP06 - Agent Pane Launch

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

- Implement all four OpenCode agent pane launch actions: CreateSpec, Clarify, Plan, GenerateTasks
- Each action opens a Zellij pane to the right running OpenCode with the correct spec-kitty slash command
- Multiple panes stack alongside existing ones (FR-018)
- Agent panes open within 2 seconds (SC-003)
- Hub remains visible and interactive when panes are open (FR-009)
- OpenCode binary validation before launch

## Context & Constraints

- **Plan**: `kitty-specs/010-hub-tui-navigator/plan.md` (AD-002: Hub Module Structure)
- **Spec**: `kitty-specs/010-hub-tui-navigator/spec.md` (FR-004, FR-005, FR-006, FR-009, FR-018)
- **Research**: `kitty-specs/010-hub-tui-navigator/research.md` (R-001: Zellij Pane Direction API, R-002: OpenCode --prompt Flag)
- **Dependencies**: WP05 (action resolution + Zellij wrappers)

### Key Architectural Decisions

- All agent pane commands follow the pattern: `zellij action new-pane --direction right --name "<action>-<feature>" -- ocx oc -- --prompt "<slash-cmd>" --agent controller`
- Pane names use convention: `spec-<number>`, `clarify-<number>`, `plan-<number>`, `tasks-<number>`
- New panes stack alongside existing ones (Zellij handles stacking automatically with `--direction right`)
- The hub does NOT wait for the agent to complete -- it fires and forgets

## Subtasks & Detailed Guidance

### Subtask T026 - Implement CreateSpec action dispatch

- **Purpose**: Open an OpenCode pane for spec creation when the operator triggers CreateSpec.
- **Steps**:
  1. In `crates/kasmos/src/hub/actions.rs`, implement `pub async fn dispatch_action(action: &HubAction) -> anyhow::Result<()>` (or add to existing dispatch function)
  2. For `HubAction::CreateSpec { feature_slug }`:
     ```rust
     open_pane_right(
         &format!("spec-{}", &feature_slug[..3]),  // e.g., "spec-010"
         "ocx",
         &["oc", "--", "--prompt", "/spec-kitty.specify", "--agent", "controller"],
         Some(&format!("kitty-specs/{}", feature_slug)),  // cwd = feature dir
     ).await?;
     ```
  3. Before launching, validate that `ocx` (or the configured OpenCode binary) is in PATH
  4. If validation fails, return an error that the hub displays as a status message

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes (structurally identical to T027-T029)
- **Notes**: The `--agent controller` flag ensures OpenCode uses the controller agent profile which has access to spec-kitty commands. The `--prompt` flag pre-loads the slash command so it executes immediately on session start.

### Subtask T027 - Implement Clarify action dispatch

- **Purpose**: Open an OpenCode pane for clarification workflow.
- **Steps**:
  1. For `HubAction::Clarify { feature_slug }`:
     ```rust
     open_pane_right(
         &format!("clarify-{}", &feature_slug[..3]),
         "ocx",
         &["oc", "--", "--prompt", "/spec-kitty.clarify", "--agent", "controller"],
         Some(&format!("kitty-specs/{}", feature_slug)),
     ).await?;
     ```

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes
- **Notes**: Clarify is available when spec is present but no plan. It opens the same type of pane as CreateSpec but with a different slash command.

### Subtask T028 - Implement Plan action dispatch

- **Purpose**: Open an OpenCode pane for planning workflow.
- **Steps**:
  1. For `HubAction::Plan { feature_slug }`:
     ```rust
     open_pane_right(
         &format!("plan-{}", &feature_slug[..3]),
         "ocx",
         &["oc", "--", "--prompt", "/spec-kitty.plan", "--agent", "controller"],
         Some(&format!("kitty-specs/{}", feature_slug)),
     ).await?;
     ```

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes

### Subtask T029 - Implement GenerateTasks action dispatch

- **Purpose**: Open an OpenCode pane for task generation workflow.
- **Steps**:
  1. For `HubAction::GenerateTasks { feature_slug }`:
     ```rust
     open_pane_right(
         &format!("tasks-{}", &feature_slug[..3]),
         "ocx",
         &["oc", "--", "--prompt", "/spec-kitty.tasks", "--agent", "controller"],
         Some(&format!("kitty-specs/{}", feature_slug)),
     ).await?;
     ```

- **Files**: `crates/kasmos/src/hub/actions.rs`
- **Parallel?**: Yes

### Subtask T030 - Wire action dispatch to hub keybindings

- **Purpose**: Connect the action dispatch to the hub's keyboard event handling.
- **Steps**:
  1. In `crates/kasmos/src/hub/keybindings.rs`, when Enter is pressed in list view:
     a. Resolve actions for the selected feature: `let actions = resolve_actions(&app.features[app.selected]);`
     b. If there's exactly one non-ViewDetails action, dispatch it immediately
     c. If there are multiple actions, show them in the detail view (or a popup) for selection
     d. If the only action is ViewDetails, enter detail view
  2. In detail view, Enter on a specific action dispatches it
  3. Action dispatch is async -- use `tokio::spawn` to avoid blocking the event loop:
     ```rust
     let action = selected_action.clone();
     tokio::spawn(async move {
         if let Err(e) = dispatch_action(&action).await {
             // Error handling -- need to communicate back to app
             eprintln!("Action failed: {}", e);
         }
     });
     ```
  4. After dispatching, show a status message: "Launched: Create Spec for 010-hub-tui-navigator"
  5. If `app.is_read_only()`, show error instead of dispatching

- **Files**: `crates/kasmos/src/hub/keybindings.rs`, `crates/kasmos/src/hub/app.rs`
- **Parallel?**: No (depends on T026-T029)
- **Notes**: The keybinding handler needs to be async-aware. Since `handle_event` is called from the event loop, it can return an `Option<HubAction>` that the event loop dispatches asynchronously. Alternatively, use a channel to send actions from the keybinding handler to the event loop.

**Recommended pattern**:
```rust
// In keybindings.rs:
pub fn handle_event(app: &mut App, event: Event) -> Option<HubAction> {
    // ... key handling ...
    // Return Some(action) when an action should be dispatched
    None
}

// In mod.rs event loop:
if let Some(action) = keybindings::handle_event(&mut app, event) {
    let action_clone = action.clone();
    tokio::spawn(async move {
        if let Err(e) = actions::dispatch_action(&action_clone).await {
            // handle error
        }
    });
    app.status_message = Some(format!("Launched: {}", action.label()));
}
```

## Test Strategy

- **Manual testing**: Inside a Zellij session, trigger each action and verify the correct pane opens
- **Verification**: Check pane name matches convention, OpenCode receives correct `--prompt` flag
- **Edge cases**: OpenCode not in PATH (error message), Zellij pane creation failure (error message)

## Risks & Mitigations

- **OpenCode not installed**: Validate binary before launch, show clear error
- **Pane naming conflicts**: Use feature number prefix to avoid collisions
- **Async dispatch**: Use `tokio::spawn` to avoid blocking the event loop
- **Multiple rapid dispatches**: Each dispatch is independent -- Zellij handles pane stacking

## Review Guidance

- Verify all four agent actions use the correct spec-kitty slash command
- Verify pane naming convention is consistent
- Verify OpenCode binary validation happens before launch
- Verify async dispatch doesn't block the event loop
- Verify read-only mode prevents dispatch

## Activity Log

- 2026-02-13T03:53:23Z - system - lane=planned - Prompt created.
- 2026-02-13T08:15:00Z - claude-sonnet-4-5 - lane=for_review - Completed WP06 implementation
- 2026-02-13T12:00:00Z - release opencode agent - lane=done - Acceptance validation passed
