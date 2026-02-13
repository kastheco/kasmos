//! Hub action resolution and Zellij session wrappers.
//!
//! Defines the `HubAction` enum representing contextual actions available
//! for a feature, and `resolve_actions()` which maps `FeatureEntry` state
//! to available actions per the data model state machine.
//!
//! Also provides thin async wrappers around `zellij action ...` commands
//! for use inside an existing Zellij session (no `--session` flag).

use tokio::process::Command;

use super::scanner::{FeatureEntry, OrchestrationStatus, PlanStatus, SpecStatus, TaskProgress};

// ---------------------------------------------------------------------------
// HubAction enum (T021)
// ---------------------------------------------------------------------------

/// Contextual action available for a feature.
#[derive(Debug, Clone, PartialEq)]
pub enum HubAction {
    /// Open OpenCode pane for spec creation.
    CreateSpec { feature_slug: String },
    /// Create a new feature (prompt for name first).
    NewFeature,
    /// Open OpenCode pane for clarification.
    Clarify { feature_slug: String },
    /// Open OpenCode pane for planning.
    Plan { feature_slug: String },
    /// Open OpenCode pane for task generation.
    GenerateTasks { feature_slug: String },
    /// Start implementation in continuous mode.
    StartContinuous { feature_slug: String },
    /// Start implementation in wave-gated mode.
    StartWaveGated { feature_slug: String },
    /// Attach to running orchestration.
    Attach { feature_slug: String },
    /// View feature details.
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

// ---------------------------------------------------------------------------
// Action resolution (T022)
// ---------------------------------------------------------------------------

/// Resolve available actions for a feature based on its current state.
///
/// The state machine follows the feature lifecycle:
/// - Running orchestration -> only Attach + ViewDetails
/// - Empty spec -> CreateSpec + ViewDetails
/// - Present spec, no plan -> Clarify + Plan + ViewDetails
/// - Present spec + plan, no tasks -> GenerateTasks + ViewDetails
/// - Tasks in progress -> StartContinuous + StartWaveGated + ViewDetails
/// - All tasks complete -> ViewDetails only
pub fn resolve_actions(entry: &FeatureEntry) -> Vec<HubAction> {
    let mut actions = vec![HubAction::ViewDetails];
    let slug = entry.full_slug.clone();

    // Running orchestration takes precedence: only Attach is offered.
    if entry.orchestration_status == OrchestrationStatus::Running {
        actions.push(HubAction::Attach {
            feature_slug: slug,
        });
        return actions;
    }

    match entry.spec_status {
        SpecStatus::Empty => {
            actions.push(HubAction::CreateSpec {
                feature_slug: slug,
            });
        }
        SpecStatus::Present => match entry.plan_status {
            PlanStatus::Absent => {
                actions.push(HubAction::Clarify {
                    feature_slug: slug.clone(),
                });
                actions.push(HubAction::Plan {
                    feature_slug: slug,
                });
            }
            PlanStatus::Present => match &entry.task_progress {
                TaskProgress::NoTasks => {
                    actions.push(HubAction::GenerateTasks {
                        feature_slug: slug,
                    });
                }
                TaskProgress::InProgress { .. } => {
                    // Wave-gated first (primary/default), continuous second.
                    actions.push(HubAction::StartWaveGated {
                        feature_slug: slug.clone(),
                    });
                    actions.push(HubAction::StartContinuous {
                        feature_slug: slug,
                    });
                }
                TaskProgress::Complete { .. } => {
                    // Feature is complete -- no start actions, only ViewDetails.
                }
            },
        },
    }

    actions
}

// ---------------------------------------------------------------------------
// Action dispatch (WP06 T026-T029, WP07 T031/T032/T035)
// ---------------------------------------------------------------------------

/// Validate that a binary exists in PATH.
fn validate_binary(name: &str) -> anyhow::Result<()> {
    match std::process::Command::new("which")
        .arg(name)
        .output()
    {
        Ok(output) if output.status.success() => Ok(()),
        _ => anyhow::bail!("{name} not found in PATH"),
    }
}

/// Extract the feature number prefix (first 3 chars) from a feature slug.
fn feature_number_prefix(feature_slug: &str) -> &str {
    if feature_slug.len() >= 3 {
        &feature_slug[..3]
    } else {
        feature_slug
    }
}

/// Dispatch a hub action asynchronously.
///
/// This is the central dispatch function called from the event loop when
/// a keybinding triggers an action. Each action maps to a Zellij command.
pub async fn dispatch_action(action: &HubAction) -> anyhow::Result<()> {
    match action {
        // -- WP06: Agent pane launches (T026-T029) --
        HubAction::CreateSpec { feature_slug } => {
            validate_binary("ocx")?;
            let prefix = feature_number_prefix(feature_slug);
            open_pane_right(
                &format!("spec-{prefix}"),
                "ocx",
                &["oc", "--", "--prompt", "/spec-kitty.specify", "--agent", "controller"],
                Some(&format!("kitty-specs/{feature_slug}")),
            )
            .await
        }
        HubAction::Clarify { feature_slug } => {
            validate_binary("ocx")?;
            let prefix = feature_number_prefix(feature_slug);
            open_pane_right(
                &format!("clarify-{prefix}"),
                "ocx",
                &["oc", "--", "--prompt", "/spec-kitty.clarify", "--agent", "controller"],
                Some(&format!("kitty-specs/{feature_slug}")),
            )
            .await
        }
        HubAction::Plan { feature_slug } => {
            validate_binary("ocx")?;
            let prefix = feature_number_prefix(feature_slug);
            open_pane_right(
                &format!("plan-{prefix}"),
                "ocx",
                &["oc", "--", "--prompt", "/spec-kitty.plan", "--agent", "controller"],
                Some(&format!("kitty-specs/{feature_slug}")),
            )
            .await
        }
        HubAction::GenerateTasks { feature_slug } => {
            validate_binary("ocx")?;
            let prefix = feature_number_prefix(feature_slug);
            open_pane_right(
                &format!("tasks-{prefix}"),
                "ocx",
                &["oc", "--", "--prompt", "/spec-kitty.tasks", "--agent", "controller"],
                Some(&format!("kitty-specs/{feature_slug}")),
            )
            .await
        }

        // -- WP07: Implementation launch (T031/T032) --
        // Check for existing tab first to avoid duplicates.
        HubAction::StartContinuous { feature_slug } => {
            let tab_name = format!("kasmos-{feature_slug}");
            if tab_exists(&tab_name).await {
                go_to_tab(&tab_name).await
            } else {
                open_new_tab(
                    &tab_name,
                    "kasmos",
                    &["start", feature_slug, "--mode", "continuous"],
                )
                .await
            }
        }
        HubAction::StartWaveGated { feature_slug } => {
            let tab_name = format!("kasmos-{feature_slug}");
            if tab_exists(&tab_name).await {
                go_to_tab(&tab_name).await
            } else {
                open_new_tab(
                    &tab_name,
                    "kasmos",
                    &["start", feature_slug, "--mode", "wave-gated"],
                )
                .await
            }
        }

        // -- WP07: Attach (T035) --
        HubAction::Attach { feature_slug } => {
            go_to_tab(&format!("kasmos-{feature_slug}")).await
        }

        // -- Non-dispatchable actions --
        HubAction::NewFeature => {
            // Handled by the NewFeaturePrompt input mode, not dispatch.
            Ok(())
        }
        HubAction::ViewDetails => {
            // Handled by view navigation, not dispatch.
            Ok(())
        }
    }
}

// ---------------------------------------------------------------------------
// Inside-session Zellij wrappers (T023)
// ---------------------------------------------------------------------------

/// Open a new pane to the right with a command.
///
/// Uses `zellij action new-pane --direction right` which runs inside the
/// current Zellij session (no `--session` flag needed).
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

    let output = Command::new("zellij").args(&cmd_args).output().await?;

    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        anyhow::bail!("zellij action new-pane failed: {}", stderr);
    }
    Ok(())
}

/// Switch to an existing tab by name.
///
/// Uses `zellij action go-to-tab-name`.
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

/// Check whether a tab with the given name exists in the current session.
async fn tab_exists(name: &str) -> bool {
    match query_tab_names().await {
        Ok(tabs) => tabs.iter().any(|t| t == name),
        Err(_) => false,
    }
}

/// Query all tab names in the current Zellij session.
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

/// Open a new tab with a command.
///
/// **Limitation**: `zellij action new-tab` does not support `-- command`
/// syntax directly. This function creates the tab first, then runs the
/// command in it via `zellij run`. The command will open in a new pane
/// within the newly created (and focused) tab.
pub async fn open_new_tab(name: &str, command: &str, args: &[&str]) -> anyhow::Result<()> {
    // Step 1: Create the tab.
    let mut tab_args = vec!["action", "new-tab"];
    if !name.is_empty() {
        tab_args.push("--name");
        tab_args.push(name);
    }

    let output = Command::new("zellij").args(&tab_args).output().await?;
    if !output.status.success() {
        let stderr = String::from_utf8_lossy(&output.stderr);
        anyhow::bail!("zellij action new-tab failed: {}", stderr);
    }

    // Step 2: Run the command in the new (now-focused) tab.
    if !command.is_empty() {
        let mut run_args = vec!["run", "--"];
        run_args.push(command);
        run_args.extend_from_slice(args);
        let output = Command::new("zellij").args(&run_args).output().await?;
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            anyhow::bail!("zellij run in new tab failed: {}", stderr);
        }
    }

    Ok(())
}

/// Switch to an existing "Hub" tab or create one if it doesn't exist.
///
/// Queries tab names, and if a tab named "Hub" exists, switches to it.
/// Otherwise creates a new tab named "Hub".
pub async fn open_or_switch_to_hub() -> anyhow::Result<()> {
    let tabs = query_tab_names().await?;
    if tabs.iter().any(|t| t == "Hub") {
        go_to_tab("Hub").await
    } else {
        // Create a new Hub tab (no command -- just a shell).
        let tab_args = ["action", "new-tab", "--name", "Hub"];
        let output = Command::new("zellij").args(tab_args).output().await?;
        if !output.status.success() {
            let stderr = String::from_utf8_lossy(&output.stderr);
            anyhow::bail!("zellij action new-tab (Hub) failed: {}", stderr);
        }
        Ok(())
    }
}

// ---------------------------------------------------------------------------
// New-feature helpers (T024 support)
// ---------------------------------------------------------------------------

/// Sanitize user input into a valid feature slug.
///
/// Lowercases, replaces whitespace with hyphens, strips non-alphanumeric
/// characters (except hyphens), and collapses consecutive hyphens.
pub fn slugify(input: &str) -> String {
    let s: String = input
        .trim()
        .to_lowercase()
        .chars()
        .map(|c| if c.is_ascii_alphanumeric() || c == '-' { c } else { '-' })
        .collect();
    // Collapse consecutive hyphens and trim leading/trailing hyphens.
    let mut result = String::new();
    let mut prev_hyphen = true; // start as true to trim leading hyphens
    for c in s.chars() {
        if c == '-' {
            if !prev_hyphen {
                result.push('-');
            }
            prev_hyphen = true;
        } else {
            result.push(c);
            prev_hyphen = false;
        }
    }
    // Trim trailing hyphen.
    if result.ends_with('-') {
        result.pop();
    }
    result
}

/// Compute the next feature number by scanning existing features.
///
/// Finds the maximum numeric prefix, adds 1, and zero-pads to 3 digits.
pub fn next_feature_number(features: &[FeatureEntry]) -> String {
    let max_num = features
        .iter()
        .filter_map(|f| f.number.parse::<u32>().ok())
        .max()
        .unwrap_or(0);
    format!("{:03}", max_num + 1)
}

// ---------------------------------------------------------------------------
// Tests (T025)
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use std::path::PathBuf;

    fn make_entry(
        spec: SpecStatus,
        plan: PlanStatus,
        tasks: TaskProgress,
        orch: OrchestrationStatus,
    ) -> FeatureEntry {
        FeatureEntry {
            number: "001".to_string(),
            slug: "test".to_string(),
            full_slug: "001-test".to_string(),
            spec_status: spec,
            plan_status: plan,
            task_progress: tasks,
            orchestration_status: orch,
            feature_dir: PathBuf::from("kitty-specs/001-test"),
        }
    }

    // -- resolve_actions tests --

    #[test]
    fn empty_spec_offers_create_spec_and_view_details() {
        let entry = make_entry(
            SpecStatus::Empty,
            PlanStatus::Absent,
            TaskProgress::NoTasks,
            OrchestrationStatus::None,
        );
        let actions = resolve_actions(&entry);
        assert!(actions.contains(&HubAction::CreateSpec {
            feature_slug: "001-test".to_string(),
        }));
        assert!(actions.contains(&HubAction::ViewDetails));
        assert_eq!(actions.len(), 2);
    }

    #[test]
    fn spec_present_no_plan_offers_clarify_and_plan() {
        let entry = make_entry(
            SpecStatus::Present,
            PlanStatus::Absent,
            TaskProgress::NoTasks,
            OrchestrationStatus::None,
        );
        let actions = resolve_actions(&entry);
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::Clarify { .. })));
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::Plan { .. })));
        assert!(actions.contains(&HubAction::ViewDetails));
        assert_eq!(actions.len(), 3);
    }

    #[test]
    fn plan_present_no_tasks_offers_generate_tasks() {
        let entry = make_entry(
            SpecStatus::Present,
            PlanStatus::Present,
            TaskProgress::NoTasks,
            OrchestrationStatus::None,
        );
        let actions = resolve_actions(&entry);
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::GenerateTasks { .. })));
        assert!(actions.contains(&HubAction::ViewDetails));
        assert_eq!(actions.len(), 2);
    }

    #[test]
    fn tasks_in_progress_offers_start_modes() {
        let entry = make_entry(
            SpecStatus::Present,
            PlanStatus::Present,
            TaskProgress::InProgress { done: 1, total: 5 },
            OrchestrationStatus::None,
        );
        let actions = resolve_actions(&entry);
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::StartContinuous { .. })));
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::StartWaveGated { .. })));
        assert!(actions.contains(&HubAction::ViewDetails));
        assert_eq!(actions.len(), 3);
    }

    #[test]
    fn running_offers_only_attach_and_view_details() {
        let entry = make_entry(
            SpecStatus::Present,
            PlanStatus::Present,
            TaskProgress::InProgress { done: 1, total: 5 },
            OrchestrationStatus::Running,
        );
        let actions = resolve_actions(&entry);
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::Attach { .. })));
        assert!(actions.contains(&HubAction::ViewDetails));
        // No start actions when running.
        assert!(!actions
            .iter()
            .any(|a| matches!(a, HubAction::StartContinuous { .. })));
        assert!(!actions
            .iter()
            .any(|a| matches!(a, HubAction::StartWaveGated { .. })));
        assert_eq!(actions.len(), 2);
    }

    #[test]
    fn complete_feature_offers_only_view_details() {
        let entry = make_entry(
            SpecStatus::Present,
            PlanStatus::Present,
            TaskProgress::Complete { total: 5 },
            OrchestrationStatus::None,
        );
        let actions = resolve_actions(&entry);
        assert_eq!(actions, vec![HubAction::ViewDetails]);
    }

    #[test]
    fn completed_orchestration_follows_normal_rules() {
        // Completed orchestration (not running) follows the feature lifecycle.
        let entry = make_entry(
            SpecStatus::Present,
            PlanStatus::Present,
            TaskProgress::InProgress { done: 3, total: 5 },
            OrchestrationStatus::Completed,
        );
        let actions = resolve_actions(&entry);
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::StartContinuous { .. })));
        assert!(actions
            .iter()
            .any(|a| matches!(a, HubAction::StartWaveGated { .. })));
        assert!(actions.contains(&HubAction::ViewDetails));
    }

    // -- HubAction::label() tests --

    #[test]
    fn all_labels_are_non_empty() {
        let variants: Vec<HubAction> = vec![
            HubAction::CreateSpec {
                feature_slug: "x".into(),
            },
            HubAction::NewFeature,
            HubAction::Clarify {
                feature_slug: "x".into(),
            },
            HubAction::Plan {
                feature_slug: "x".into(),
            },
            HubAction::GenerateTasks {
                feature_slug: "x".into(),
            },
            HubAction::StartContinuous {
                feature_slug: "x".into(),
            },
            HubAction::StartWaveGated {
                feature_slug: "x".into(),
            },
            HubAction::Attach {
                feature_slug: "x".into(),
            },
            HubAction::ViewDetails,
        ];
        for v in &variants {
            assert!(!v.label().is_empty(), "label for {:?} is empty", v);
        }
    }

    #[test]
    fn labels_are_distinct() {
        let variants: Vec<HubAction> = vec![
            HubAction::CreateSpec {
                feature_slug: "x".into(),
            },
            HubAction::NewFeature,
            HubAction::Clarify {
                feature_slug: "x".into(),
            },
            HubAction::Plan {
                feature_slug: "x".into(),
            },
            HubAction::GenerateTasks {
                feature_slug: "x".into(),
            },
            HubAction::StartContinuous {
                feature_slug: "x".into(),
            },
            HubAction::StartWaveGated {
                feature_slug: "x".into(),
            },
            HubAction::Attach {
                feature_slug: "x".into(),
            },
            HubAction::ViewDetails,
        ];
        let labels: Vec<&str> = variants.iter().map(|v| v.label()).collect();
        let mut deduped = labels.clone();
        deduped.sort();
        deduped.dedup();
        assert_eq!(labels.len(), deduped.len(), "duplicate labels found");
    }

    // -- slugify tests --

    #[test]
    fn slugify_basic() {
        assert_eq!(slugify("My Feature"), "my-feature");
    }

    #[test]
    fn slugify_strips_special_chars() {
        assert_eq!(slugify("hello world!@#$%"), "hello-world");
    }

    #[test]
    fn slugify_collapses_hyphens() {
        assert_eq!(slugify("a---b"), "a-b");
    }

    #[test]
    fn slugify_trims_whitespace() {
        assert_eq!(slugify("  spaced  "), "spaced");
    }

    #[test]
    fn slugify_empty_string() {
        assert_eq!(slugify(""), "");
    }

    #[test]
    fn slugify_preserves_existing_hyphens() {
        assert_eq!(slugify("hub-tui-navigator"), "hub-tui-navigator");
    }

    // -- next_feature_number tests --

    #[test]
    fn next_number_empty_list() {
        assert_eq!(next_feature_number(&[]), "001");
    }

    #[test]
    fn next_number_increments_max() {
        let features = vec![
            make_entry(
                SpecStatus::Empty,
                PlanStatus::Absent,
                TaskProgress::NoTasks,
                OrchestrationStatus::None,
            ),
        ];
        // number is "001", so next is "002".
        assert_eq!(next_feature_number(&features), "002");
    }

    #[test]
    fn next_number_finds_max_across_gaps() {
        let mut f1 = make_entry(
            SpecStatus::Empty,
            PlanStatus::Absent,
            TaskProgress::NoTasks,
            OrchestrationStatus::None,
        );
        f1.number = "003".to_string();
        let mut f2 = make_entry(
            SpecStatus::Empty,
            PlanStatus::Absent,
            TaskProgress::NoTasks,
            OrchestrationStatus::None,
        );
        f2.number = "010".to_string();
        assert_eq!(next_feature_number(&[f1, f2]), "011");
    }

    // -- feature_number_prefix tests --

    #[test]
    fn prefix_extracts_three_chars() {
        assert_eq!(feature_number_prefix("010-hub-tui-navigator"), "010");
    }

    #[test]
    fn prefix_short_slug() {
        assert_eq!(feature_number_prefix("ab"), "ab");
    }

    // -- validate_binary tests --

    #[test]
    fn validate_binary_finds_existing() {
        // `ls` should always exist
        assert!(validate_binary("ls").is_ok());
    }

    #[test]
    fn validate_binary_rejects_missing() {
        assert!(validate_binary("nonexistent-kasmos-test-binary-xyz").is_err());
    }
}
