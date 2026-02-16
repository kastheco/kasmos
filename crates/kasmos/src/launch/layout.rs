//! KDL layout generation for orchestration launch.
//!
//! The initial layout is intentionally minimal: just the manager pane
//! (running opencode) plus a zjstatus bar.  Additional panes (msg-log,
//! dashboard, workers) are created dynamically by the MCP server when
//! the manager kicks off work.

use crate::config::Config;
use anyhow::{bail, Context, Result};
use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

/// Launch layout definition for a single orchestration tab.
#[derive(Debug, Clone)]
pub struct OrchestrationLayout {
    /// Manager pane width percentage when header row is active.
    pub manager_width_pct: u32,
    /// Message-log pane width percentage when header row is active.
    pub message_log_width_pct: u32,
    /// Feature slug bound to this orchestration tab.
    pub feature_slug: String,
    /// Maximum parallel workers for swap layout rules.
    max_workers: usize,
}

impl OrchestrationLayout {
    pub fn new(
        manager_width_pct: u32,
        message_log_width_pct: u32,
        feature_slug: impl Into<String>,
        max_workers: usize,
    ) -> Result<Self> {
        let feature_slug = feature_slug.into();
        if feature_slug.is_empty() {
            bail!("feature slug cannot be empty");
        }

        let header_total = manager_width_pct + message_log_width_pct;
        if !(1..100).contains(&manager_width_pct) || !(1..100).contains(&message_log_width_pct) {
            bail!("manager/message-log widths must be between 1 and 99");
        }
        if header_total >= 100 {
            bail!("manager + message-log width must be < 100");
        }
        if max_workers == 0 {
            bail!("max_workers must be greater than zero");
        }

        Ok(Self {
            manager_width_pct,
            message_log_width_pct,
            feature_slug,
            max_workers,
        })
    }

    fn dashboard_width_pct(&self) -> u32 {
        100 - self.manager_width_pct - self.message_log_width_pct
    }

    /// Initial layout: just the manager pane + zjstatus bar.
    /// msg-log, dashboard, and worker panes are added dynamically later.
    pub fn to_kdl(&self, manager_command: &ManagerCommand) -> String {
        format!(
            "\
layout {{
{default_tab_template}
{swap_layouts}
  tab name=\"{tab_name}\" {{
    {manager_pane}
  }}
}}
default_mode \"locked\"
",
            default_tab_template = Self::zjstatus_tab_template(),
            swap_layouts = self.swap_tiled_layouts(),
            tab_name = kdl_escape(&self.feature_slug),
            manager_pane = manager_command.to_kdl_pane(),
        )
    }

    /// Minimal fallback layout (same idea, just manager).
    pub fn to_minimal_kdl(&self, manager_command: &ManagerCommand) -> String {
        format!(
            "\
layout {{
{default_tab_template}
  tab name=\"{tab_name}\" {{
    {manager_pane}
  }}
}}
default_mode \"locked\"
",
            default_tab_template = Self::zjstatus_tab_template(),
            tab_name = kdl_escape(&self.feature_slug),
            manager_pane = manager_command.to_kdl_pane(),
        )
    }

    /// zjstatus + zjstatus-hints bar as a default_tab_template.
    /// Rose-pine-moon theme matching the user's Zellij config.
    fn zjstatus_tab_template() -> String {
        r##"  default_tab_template {
    children
    pane size=1 borderless=true {
      plugin location="zjstatus" {
        color_bg   "#393552"
        color_fg   "#e0def4"
        color_sel  "#44415a"
        color_blue "#3e8fb0"
        color_gold "#f6c177"
        color_rose "#eb6f92"
        color_pine "#c4a7e7"
        color_foam "#9ccfd8"

        format_left   "{mode} {tabs}"
        format_center ""
        format_right  "{pipe_zjstatus_hints}{datetime} "
        format_space  ""

        hide_frame_for_single_pane "true"

        mode_normal        "#[bg=$blue,fg=$bg,bold] NORMAL "
        mode_locked        "#[bg=$bg,fg=$fg,dim] LOCKED "
        mode_pane          "#[bg=$pine,fg=$bg,bold] PANE "
        mode_tab           "#[bg=$gold,fg=$bg,bold] TAB "
        mode_resize        "#[bg=$foam,fg=$bg,bold] RESIZE "
        mode_move          "#[bg=$foam,fg=$bg,bold] MOVE "
        mode_scroll        "#[bg=$blue,fg=$bg,bold] SCROLL "
        mode_search        "#[bg=$blue,fg=$bg,bold] SEARCH "
        mode_enter_search  "#[bg=$blue,fg=$bg,bold] SEARCH "
        mode_session       "#[bg=$rose,fg=$bg,bold] SESSION "
        mode_rename_tab    "#[bg=$gold,fg=$bg,bold] RENAME TAB "
        mode_rename_pane   "#[bg=$pine,fg=$bg,bold] RENAME PANE "

        tab_normal              "#[bg=$sel,fg=$fg] {name} "
        tab_normal_fullscreen   "#[bg=$sel,fg=$fg] {name} [] "
        tab_normal_sync         "#[bg=$sel,fg=$fg] {name} <> "
        tab_active              "#[bg=$blue,fg=$bg,bold] {name} "
        tab_active_fullscreen   "#[bg=$blue,fg=$bg,bold] {name} [] "
        tab_active_sync         "#[bg=$blue,fg=$bg,bold] {name} <> "
        tab_separator           "#[bg=$bg] "

        pipe_zjstatus_hints_format "{output}"

        datetime          "#[bg=$bg,fg=$fg,dim]{format}"
        datetime_format   "%H:%M"
        datetime_timezone "America/New_York"
      }
    }
  }"##
        .to_string()
    }

    fn swap_tiled_layouts(&self) -> String {
        let mut out = String::from("  swap_tiled_layout name=\"orchestration-reflow\" {\n");
        // +1 for the manager pane already present at start
        let max_panes = self.max_workers + 4; // manager + msg-log + dashboard + workers

        // 1 pane: just the manager (initial state)
        out.push_str("    tab max_panes=1 {\n      pane\n    }\n");

        // 2-3 panes: side-by-side (manager + extras as they get added)
        out.push_str("    tab max_panes=3 {\n      pane split_direction=\"horizontal\" {\n        pane\n        children\n      }\n    }\n");

        // 4+ panes: header row (22% height) + worker area below
        for pane_count in 4..=max_panes {
            out.push_str(&format!(
                "    tab max_panes={pane_count} {{\n      pane split_direction=\"vertical\" {{\n        pane size=\"22%\" split_direction=\"horizontal\" {{\n          pane size=\"{mgr}%\"\n          pane size=\"{msg}%\"\n          pane size=\"{dash}%\"\n        }}\n        pane {{\n          children\n        }}\n      }}\n    }}\n",
                mgr = self.manager_width_pct,
                msg = self.message_log_width_pct,
                dash = self.dashboard_width_pct()
            ));
        }

        out.push_str("  }\n");
        out
    }
}

/// Manager pane launch command encoded into KDL.
#[derive(Debug, Clone)]
pub struct ManagerCommand {
    pub cwd: String,
    pub binary: String,
    pub profile: Option<String>,
    pub prompt: String,
    /// Path to the prompt file (written during layout generation).
    pub prompt_file: Option<PathBuf>,
}

impl ManagerCommand {
    pub fn from_config(config: &Config, cwd: impl Into<String>, prompt: impl Into<String>) -> Self {
        Self {
            cwd: cwd.into(),
            binary: config.agent.opencode_binary.clone(),
            profile: config.agent.opencode_profile.clone(),
            prompt: prompt.into(),
            prompt_file: None,
        }
    }

    /// Write the prompt to a temp file so the KDL layout can reference it
    /// via shell expansion instead of inlining potentially huge text.
    pub fn write_prompt_file(&mut self) -> Result<()> {
        let ts = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .context("system clock before unix epoch")?
            .as_nanos();
        let path = std::env::temp_dir().join(format!("kasmos-prompt-{ts}.txt"));
        std::fs::write(&path, &self.prompt)
            .with_context(|| format!("failed to write prompt file {}", path.display()))?;
        self.prompt_file = Some(path);
        Ok(())
    }

    fn to_kdl_pane(&self) -> String {
        // Unset OPENCODE_DISABLE_PROJECT_CONFIG so the manager reads
        // .opencode/opencode.jsonc (which provides kasmos MCP servers).
        // This env var is set by an enclosing opencode session and would
        // otherwise be inherited by the Zellij pane.
        let shell_cmd = if let Some(ref prompt_path) = self.prompt_file {
            let path = prompt_path.display();
            format!(
                "unset OPENCODE_DISABLE_PROJECT_CONFIG; {binary} --agent manager --prompt \"$(cat {path})\"",
                binary = self.binary,
            )
        } else {
            // Fallback: inline (may break for long prompts)
            format!(
                "unset OPENCODE_DISABLE_PROJECT_CONFIG; {binary} --agent manager --prompt {prompt}",
                binary = self.binary,
                prompt = shell_escape::escape(std::borrow::Cow::Borrowed(&self.prompt)),
            )
        };

        format!(
            "pane name=\"manager\" {{\n      cwd \"{cwd}\"\n      command \"bash\"\n      args \"-c\" \"{cmd}\"\n    }}",
            cwd = kdl_escape(&self.cwd),
            cmd = kdl_escape(&shell_cmd),
        )
    }
}

pub fn generate_layout(
    config: &Config,
    feature_slug: &str,
    manager_command: &ManagerCommand,
) -> Result<String> {
    let layout = match OrchestrationLayout::new(
        config.session.manager_width_pct,
        config.session.message_log_width_pct,
        feature_slug,
        config.agent.max_parallel_workers,
    ) {
        Ok(layout) => layout,
        Err(err) => {
            tracing::warn!(
                feature = %feature_slug,
                error = %err,
                "invalid full-layout settings; using minimal fallback"
            );
            return Ok(OrchestrationLayout {
                manager_width_pct: 70,
                message_log_width_pct: 20,
                feature_slug: feature_slug.to_string(),
                max_workers: config.agent.max_parallel_workers.max(1),
            }
            .to_minimal_kdl(manager_command));
        }
    };

    // Zellij's own KDL parser handles plugin configs (zjstatus etc.) that
    // the kdl v6 crate rejects under strict KDL v2 rules.  Skip client-side
    // validation -- Zellij will report any real parse errors.
    Ok(layout.to_kdl(manager_command))
}

fn kdl_escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for c in s.chars() {
        match c {
            '\\' => out.push_str("\\\\"),
            '"' => out.push_str("\\\""),
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            _ => out.push(c),
        }
    }
    out
}

/// Clean up prompt temp file if it exists.
pub fn cleanup_prompt_file(manager_command: &ManagerCommand) {
    if let Some(ref path) = manager_command.prompt_file {
        let _ = std::fs::remove_file(path);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn full_layout_contains_required_elements() {
        let mut config = Config::default();
        config.session.manager_width_pct = 60;
        config.session.message_log_width_pct = 20;
        config.agent.max_parallel_workers = 4;

        let manager = ManagerCommand::from_config(&config, "/tmp/feature", "prompt text");
        let kdl = generate_layout(&config, "011-feature", &manager).expect("layout");

        assert!(kdl.contains("name=\"manager\""));
        assert!(kdl.contains("zjstatus"));
        assert!(kdl.contains("swap_tiled_layout"));
        assert!(kdl.contains("default_tab_template"));
    }

    #[test]
    fn initial_layout_has_only_manager_pane() {
        let config = Config::default();
        let manager = ManagerCommand::from_config(&config, "/tmp/feature", "prompt text");
        let kdl = generate_layout(&config, "011-feature", &manager).expect("layout");

        // Should NOT contain msg-log or dashboard in the initial tab
        assert!(!kdl.contains("name=\"msg-log\""));
        assert!(!kdl.contains("name=\"dashboard\""));
        // Should contain manager
        assert!(kdl.contains("name=\"manager\""));
    }

    #[test]
    fn swap_layouts_handle_growth() {
        let layout = OrchestrationLayout::new(60, 20, "011-feature", 3).expect("layout");
        let swap = layout.swap_tiled_layouts();

        assert!(swap.contains("tab max_panes=1"));
        assert!(swap.contains("tab max_panes=3"));
        // 4 through max_workers + 4 = 7
        for count in 4..=7 {
            assert!(swap.contains(&format!("tab max_panes={count}")));
        }
    }

    #[test]
    fn zjstatus_template_is_valid_kdl_fragment() {
        let tmpl = OrchestrationLayout::zjstatus_tab_template();
        assert!(tmpl.contains("zjstatus"));
        assert!(tmpl.contains("zjstatus_hints"));
        assert!(tmpl.contains("rose-pine-moon") || tmpl.contains("#393552"));
    }

    #[test]
    fn prompt_file_write_and_cleanup() {
        let config = Config::default();
        let mut manager = ManagerCommand::from_config(&config, "/tmp", "test prompt content");
        manager.write_prompt_file().expect("write prompt");

        let path = manager.prompt_file.as_ref().expect("prompt file set");
        assert!(path.exists());
        let content = std::fs::read_to_string(path).expect("read prompt file");
        assert_eq!(content, "test prompt content");

        cleanup_prompt_file(&manager);
        assert!(!path.exists());
    }

    #[test]
    fn manager_pane_uses_bash_with_cat() {
        let config = Config::default();
        let mut manager = ManagerCommand::from_config(&config, "/tmp/feature", "some prompt");
        manager.write_prompt_file().expect("write");
        let pane = manager.to_kdl_pane();

        assert!(pane.contains("command \"bash\""));
        assert!(pane.contains("$(cat"));
        assert!(pane.contains("--agent manager"));

        cleanup_prompt_file(&manager);
    }

    #[test]
    fn invalid_widths_rejected() {
        let err = OrchestrationLayout::new(80, 20, "011-feature", 4).expect_err("invalid");
        assert!(err.to_string().contains("must be < 100"));
    }
}
