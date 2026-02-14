---
work_package_id: "WP03"
subtasks:
  - "T014"
  - "T015"
  - "T016"
  - "T017"
  - "T018"
  - "T019"
  - "T020"
title: "Launch Layout and Session Bootstrap"
phase: "Phase 1 - Launch Topology and MCP Runtime Skeleton"
lane: "planned"
assignee: ""
agent: ""
shell_pid: ""
review_status: ""
reviewed_by: ""
dependencies: ["WP02"]
history:
  - timestamp: "2026-02-14T16:27:48Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP03 - Launch Layout and Session Bootstrap

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP03 --base WP02
```

---

## Objectives & Success Criteria

Implement the launch flow that creates the orchestration tab layout, supports inside/outside Zellij behavior, and primes manager startup. After this WP:

1. `kasmos 011` from outside Zellij creates a new session named "kasmos" with an orchestration tab
2. Running `kasmos 012` from inside an existing Zellij session creates a new orchestration tab (not a new session)
3. The orchestration tab contains: manager pane (60%), message-log pane (20%), dashboard pane (20%), and empty worker area below
4. No dedicated MCP tab/process is created - `kasmos serve` runs as a manager-spawned MCP stdio subprocess
5. The manager pane receives initial prompt instructions including bound feature and phase assessment
6. Layout fallback works (manager + message-log only) when advanced layout generation fails

## Context & Constraints

- **Depends on WP02**: Config, feature detection, and preflight are available
- **Plan reference**: Session Layout section shows manager(60%) + message-log(20%) + dashboard(20%) + worker rows
- **Existing code**: `crates/kasmos/src/layout.rs` (980 lines) has `LayoutGenerator` for KDL layout generation. `crates/kasmos/src/zellij.rs` (550 lines) has `ZellijCli` trait. `crates/kasmos/src/session.rs` (718 lines) has `SessionManager`.
- **Research**: Inside-session Zellij commands use `zellij action <cmd>` directly (no `--session` flag). Outside-session commands use `zellij --session <name> action <cmd>`.
- **Key constraint**: Launch does NOT mutate WP state. It only establishes the orchestration runtime environment.

## Subtasks & Detailed Guidance

### Subtask T014 - Implement orchestration layout generator with swap-layout and dashboard

**Purpose**: Generate KDL layout for the orchestration tab with manager pane, message-log pane, dashboard pane, and dynamic worker area. Include swap-layout KDL blocks for automatic reflow when workers are added/removed (FR-007, US5, FR-032).

**Steps**:
1. Create/populate `crates/kasmos/src/launch/layout.rs`:
   ```rust
   pub struct OrchestrationLayout {
       pub manager_width_pct: u32,
       pub message_log_width_pct: u32,
       pub feature_slug: String,
   }

   impl OrchestrationLayout {
       pub fn to_kdl(&self) -> String {
           // Generate KDL layout string
       }
   }
   ```
2. Layout structure (KDL format for Zellij):
   - Top row: manager pane (60% width, named "manager") + message-log pane (20%, named "msg-log") + dashboard pane (20%, named "dashboard")
   - Bottom area: empty initially (workers spawned dynamically later)
   - Use Zellij's `swap_tiled_layout` for dynamic reflow when workers are added/removed (FR-007)
3. Reference the existing `LayoutGenerator` in `crates/kasmos/src/layout.rs` for KDL generation patterns (see `generate_layout()` method and `tab_template_kdl_string()` static method).
4. The existing layout.rs generates layouts for the old orchestration model (controller + agents tab). The new layout is different: single tab with manager + msg-log + worker area.
5. Use config values for width percentages (`session.manager_width_pct`, `session.message_log_width_pct`).
6. Generate multiple `swap_tiled_layout` KDL blocks for pane counts 2 (manager + msg-log only) through `max_workers + 3` (manager + msg-log + dashboard + N workers). Each block defines how panes are arranged for that count, ensuring the header row (manager/msg-log/dashboard) stays fixed while worker rows expand below. This is the mechanism behind FR-007 and US5's automatic reflow requirement.

**Files**: `crates/kasmos/src/launch/layout.rs`
**Validation**: Generated KDL parses correctly. Layout produces expected pane arrangement. Swap layouts exist for multiple pane counts.

### Subtask T015 - Implement session/tab bootstrap behavior

**Purpose**: Handle the two launch scenarios: outside Zellij (create session) vs inside Zellij (create tab).

**Steps**:
1. Create/populate `crates/kasmos/src/launch/session.rs`:
   ```rust
   pub async fn bootstrap(
       config: &Config,
       feature_slug: &str,
       layout: &OrchestrationLayout,
   ) -> Result<()> {
       if is_inside_zellij() {
           create_orchestration_tab(config, feature_slug, layout).await
       } else {
           create_orchestration_session(config, feature_slug, layout).await
       }
   }

   fn is_inside_zellij() -> bool {
       std::env::var("ZELLIJ_SESSION_NAME").is_ok()
   }
   ```
2. **Outside Zellij** (`create_orchestration_session`):
   - Write the KDL layout to a temp file
   - Launch `zellij --layout <path> attach kasmos --create`
   - Session name: `config.session.session_name` (default: "kasmos")
   - If session already exists, kill and recreate (same pattern as existing `bootstrap_start_in_zellij` in main.rs:154-207)
3. **Inside Zellij** (`create_orchestration_tab`):
   - Use `zellij action new-tab --layout <path> --name <feature_slug>`
   - No `--session` flag needed inside a session
4. Use `ZellijCli` trait methods where possible, or add new methods if needed.

**Files**: `crates/kasmos/src/launch/session.rs`
**Validation**: Launch from outside creates session. Launch from inside creates tab.

### Subtask T016 - Enforce manager-spawned MCP stdio subprocess model

**Purpose**: Ensure the launch flow does NOT create a dedicated MCP tab/process. `kasmos serve` runs as an MCP stdio subprocess spawned by the manager agent through its OpenCode MCP configuration.

**Steps**:
1. In the launch layout and session bootstrap code, verify there is NO pane running `kasmos serve`.
2. The manager pane launches OpenCode with an MCP configuration that includes `kasmos serve` as a stdio subprocess. This is configured in the OpenCode profile, not in the launch code.
3. Add a comment in the launch code explicitly noting this design decision:
   ```rust
   // kasmos serve runs as an MCP stdio subprocess owned by the manager agent.
   // It is NOT a dedicated pane or process. The manager's OpenCode profile
   // configures kasmos serve as an MCP server in its mcp config.
   // See: config/profiles/kasmos/opencode.jsonc
   ```
4. Ensure the manager pane command launches OpenCode (not kasmos serve directly).

**Files**: `crates/kasmos/src/launch/session.rs`, `crates/kasmos/src/launch/layout.rs`
**Validation**: No pane in the layout runs `kasmos serve`. Manager pane runs OpenCode.

### Subtask T017 - Implement manager initial prompt seed

**Purpose**: Generate the initial prompt for the manager agent that includes bound feature context, phase assessment instruction, and confirmation-first behavior.

**Steps**:
1. Create/update `crates/kasmos/src/prompt.rs` (or a new function within it) for manager prompt generation:
   ```rust
   pub fn generate_manager_prompt(
       feature_slug: &str,
       feature_dir: &Path,
       phase_hint: &str,
   ) -> String {
       format!(
           "You are the kasmos manager agent for feature '{feature_slug}'.
           Feature directory: {feature_dir}
           ...
           Your first task: Assess the current workflow phase and present a summary.
           Do NOT take any action without explicit user confirmation.
           ..."
       )
   }
   ```
2. The prompt should instruct the manager to:
   - Assess workflow phase by checking which artifacts exist (spec.md, plan.md, tasks.md, task lanes)
   - Present a summary of current state and next recommended action
   - Wait for explicit user confirmation before proceeding (FR-009)
   - Use `kasmos serve` MCP tools for orchestration operations
3. Include context references: spec.md path, plan.md path, tasks.md path, constitution path, architecture memory path.
4. This prompt is passed to the OpenCode command that runs in the manager pane.

**Parallel?**: Yes - can proceed once required prompt inputs (feature_slug, feature_dir) are known.
**Files**: `crates/kasmos/src/prompt.rs`
**Validation**: Generated prompt contains feature slug, phase assessment instructions, and confirmation gate.

### Subtask T018 - Add minimal-layout fallback path

**Purpose**: When advanced layout generation fails, fall back to a minimal layout with just manager + message-log panes.

**Steps**:
1. In the layout generation code, wrap the full layout builder in a fallback:
   ```rust
   pub fn generate_layout(config: &Config, feature_slug: &str) -> Result<String> {
       match generate_full_layout(config, feature_slug) {
           Ok(layout) => Ok(layout),
           Err(e) => {
               tracing::warn!("Full layout generation failed: {e}. Using minimal fallback.");
               generate_minimal_layout(config, feature_slug)
           }
       }
   }
   ```
2. Minimal layout: manager pane (70% width) + message-log pane (30% width), no worker area, no dashboard.
3. Log the fallback reason clearly so it's visible in diagnostics.

**Files**: `crates/kasmos/src/launch/layout.rs`
**Validation**: Deliberately trigger a layout failure and verify fallback produces a usable layout.

### Subtask T019 - Wire launch flow end-to-end

**Purpose**: Connect all launch components into the single `launch::run()` entry point.

**Steps**:
1. Implement the full flow in `crates/kasmos/src/launch/mod.rs`:
   ```rust
   pub async fn run(spec_prefix: Option<&str>) -> Result<()> {
       // 1. Load config
       let config = Config::load()?;
       // 2. Check for specs
       check_specs_exist(&config)?;
       // 3. Preflight dependency checks
       preflight_checks(&config)?;
       // 4. Detect or select feature
       let feature = detect_or_select_feature(spec_prefix, &config)?;
       // 5. Generate layout
       let layout = generate_layout(&config, &feature.slug)?;
       // 6. Bootstrap session/tab
       session::bootstrap(&config, &feature.slug, &layout).await?;
       Ok(())
   }
   ```
2. Wire this into `main.rs` dispatch for both `spec_prefix` present and absent cases.
3. Ensure the order is strict: config -> specs check -> preflight -> detection/selection -> layout -> session. No Zellij commands before step 6.

**Files**: `crates/kasmos/src/launch/mod.rs`, `crates/kasmos/src/main.rs`
**Validation**: Full launch flow works end-to-end for explicit feature arg case.

### Subtask T020 - Add launch integration tests

**Purpose**: Test the launch flow for explicit feature arg, branch inference, and selector path behavior.

**Steps**:
1. Unit tests for layout generation (KDL output correctness)
2. Unit tests for session bootstrap logic (inside/outside detection)
3. Integration-style tests (may need to mock Zellij calls):
   - Explicit feature arg resolves correctly and reaches layout generation
   - Branch inference extracts prefix and resolves feature
   - Missing dependency stops before any Zellij commands
   - No specs path exits cleanly

**Files**: Test modules in launch submodules
**Validation**: `cargo test` passes with new tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Zellij command differences across environments | Encapsulate shell calls in zellij.rs wrappers with robust error mapping |
| Launch fallback masking real layout bugs | Log fallback reason. Keep test coverage for normal layout path. |
| Session name collision | Use deterministic session name from config. Kill-and-recreate pattern for reruns. |

## Review Guidance

- Verify no pane in the layout runs `kasmos serve` directly
- Verify inside-Zellij creates tab, outside-Zellij creates session
- Verify launch order: config -> specs -> preflight -> detect -> layout -> session
- Verify fallback layout is functional
- Verify manager prompt includes feature binding, phase assessment, and confirmation gate

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
