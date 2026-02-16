//! KDL layout generation for orchestration launch.

use crate::config::Config;
use anyhow::{bail, Result};

/// Launch layout definition for a single orchestration tab.
#[derive(Debug, Clone)]
pub struct OrchestrationLayout {
    /// Manager pane width percentage in the top row.
    pub manager_width_pct: u32,
    /// Message-log pane width percentage in the top row.
    pub message_log_width_pct: u32,
    /// Feature slug bound to this orchestration tab.
    pub feature_slug: String,
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

    /// Full orchestration layout with manager + message-log + dashboard + worker area.
    pub fn to_kdl(&self, manager_command: &ManagerCommand) -> String {
        let dashboard_width_pct = self.dashboard_width_pct();

        format!(
            "layout {{\n{swap_layouts}\n  tab name=\"{tab_name}\" {{\n    pane split_direction=\"vertical\" {{\n      pane size=\"22%\" split_direction=\"horizontal\" {{\n        {manager_pane}\n        pane size=\"{msg_width}%\" name=\"msg-log\"\n        pane size=\"{dash_width}%\" name=\"dashboard\"\n      }}\n      pane name=\"worker-area\"\n    }}\n  }}\n}}\ndefault_mode \"locked\"\n",
            swap_layouts = self.swap_tiled_layouts(),
            tab_name = kdl_escape(&self.feature_slug),
            manager_pane = manager_command.to_kdl_pane(self.manager_width_pct),
            msg_width = self.message_log_width_pct,
            dash_width = dashboard_width_pct,
        )
    }

    /// Minimal fallback layout with manager + message-log only.
    pub fn to_minimal_kdl(&self, manager_command: &ManagerCommand) -> String {
        format!(
            "layout {{\n  tab name=\"{tab_name}\" {{\n    pane split_direction=\"horizontal\" {{\n      {manager_pane}\n      pane size=\"30%\" name=\"msg-log\"\n    }}\n  }}\n}}\ndefault_mode \"locked\"\n",
            tab_name = kdl_escape(&self.feature_slug),
            manager_pane = manager_command.to_kdl_pane(70),
        )
    }

    fn swap_tiled_layouts(&self) -> String {
        let mut out = String::from("  swap_tiled_layout name=\"orchestration-reflow\" {\n");
        let max_panes = self.max_workers + 3;

        for pane_count in 2..=max_panes {
            if pane_count == 2 {
                out.push_str(
                    "    tab max_panes=2 {\n      pane split_direction=\"horizontal\" {\n        pane size=\"70%\"\n        pane size=\"30%\"\n      }\n    }\n",
                );
                continue;
            }

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
}

impl ManagerCommand {
    pub fn from_config(config: &Config, cwd: impl Into<String>, prompt: impl Into<String>) -> Self {
        // kasmos serve runs as an MCP stdio subprocess owned by the manager agent.
        // It is NOT launched as a dedicated pane command in this layout.
        Self {
            cwd: cwd.into(),
            binary: config.agent.opencode_binary.clone(),
            profile: config.agent.opencode_profile.clone(),
            prompt: prompt.into(),
        }
    }

    fn to_kdl_pane(&self, width_pct: u32) -> String {
        let mut args = vec!["\"oc\"".to_string()];
        if let Some(profile) = &self.profile {
            args.push(format!("\"{}\"", kdl_escape(profile)));
            args.insert(1, "\"-p\"".to_string());
        }
        args.push("\"--\"".to_string());
        args.push("\"--agent\"".to_string());
        args.push("\"manager\"".to_string());
        args.push("\"--prompt\"".to_string());
        args.push(format!("\"{}\"", kdl_escape(&self.prompt)));

        format!(
            "pane size=\"{width}%\" name=\"manager\" {{\n          cwd \"{cwd}\"\n          command \"{cmd}\"\n          args {args}\n        }}",
            width = width_pct,
            cwd = kdl_escape(&self.cwd),
            cmd = kdl_escape(&self.binary),
            args = args.join(" "),
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

    let full = layout.to_kdl(manager_command);
    if kdl::KdlDocument::parse(&full).is_ok() {
        return Ok(full);
    }

    tracing::warn!(
        feature = %feature_slug,
        "full layout generation produced invalid KDL; using minimal fallback"
    );
    Ok(layout.to_minimal_kdl(manager_command))
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn full_layout_contains_required_panes() {
        let mut config = Config::default();
        config.session.manager_width_pct = 60;
        config.session.message_log_width_pct = 20;
        config.agent.max_parallel_workers = 4;

        let manager = ManagerCommand::from_config(&config, "/tmp/feature", "prompt text");
        let kdl = generate_layout(&config, "011-feature", &manager).expect("layout");

        assert!(kdl.contains("name=\"manager\""));
        assert!(kdl.contains("name=\"msg-log\""));
        assert!(kdl.contains("name=\"dashboard\""));
        assert!(kdl.contains("swap_tiled_layout"));
        assert!(kdl::KdlDocument::parse(&kdl).is_ok());
    }

    #[test]
    fn swap_layouts_cover_two_through_max_plus_three() {
        let layout = OrchestrationLayout::new(60, 20, "011-feature", 3).expect("layout");
        let swap = layout.swap_tiled_layouts();

        for count in 2..=6 {
            assert!(swap.contains(&format!("tab max_panes={count}")));
        }
    }

    #[test]
    fn invalid_widths_rejected() {
        let err = OrchestrationLayout::new(80, 20, "011-feature", 4).expect_err("invalid");
        assert!(err.to_string().contains("must be < 100"));
    }
}
