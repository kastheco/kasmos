//! KDL layout generator for Zellij orchestration sessions.
//!
//! This module generates valid Zellij KDL layout files that organize
//! a controller pane and multiple agent panes in an adaptive grid layout.

use kdl::{KdlDocument, KdlEntry, KdlEntryFormat, KdlNode, KdlValue};
use shell_escape::escape;
use std::borrow::Cow;
use std::path::{Path, PathBuf};
use tracing::{debug, info};

/// Escape a string for KDL quoted representation.
fn kdl_escape(s: &str) -> String {
    let mut out = String::with_capacity(s.len() + 2);
    out.push('"');
    for c in s.chars() {
        match c {
            '\\' | '"' => {
                out.push('\\');
                out.push(c);
            }
            '\n' => out.push_str("\\n"),
            '\r' => out.push_str("\\r"),
            '\t' => out.push_str("\\t"),
            _ => out.push(c),
        }
    }
    out.push('"');
    out
}

/// Create a KDL property entry with explicitly quoted string value.
///
/// The `kdl` v6 crate outputs bare identifiers for simple strings (e.g., `key=value`),
/// but Zellij requires quoted string values (e.g., `key="value"`).
fn kdl_str_prop(key: &str, value: &str) -> KdlEntry {
    let mut entry = KdlEntry::new_prop(key, KdlValue::String(value.to_string()));
    entry.set_format(KdlEntryFormat {
        value_repr: kdl_escape(value),
        leading: " ".to_string(),
        ..Default::default()
    });
    entry
}

/// Create a KDL positional argument entry with explicitly quoted string value.
fn kdl_str_arg(value: &str) -> KdlEntry {
    let mut entry = KdlEntry::new(KdlValue::String(value.to_string()));
    entry.set_format(KdlEntryFormat {
        value_repr: kdl_escape(value),
        leading: " ".to_string(),
        ..Default::default()
    });
    entry
}

/// Create a KDL boolean property entry (e.g., `start_suspended=#false`).
/// Uses KDL v2 boolean syntax (`#true`/`#false`).
fn kdl_bool_prop(key: &str, value: bool) -> KdlEntry {
    let repr = if value { "#true" } else { "#false" };
    let mut entry = KdlEntry::new_prop(key, KdlValue::Bool(value));
    entry.set_format(KdlEntryFormat {
        value_repr: repr.to_string(),
        leading: " ".to_string(),
        ..Default::default()
    });
    entry
}

/// Create a KDL integer property entry (e.g., `size=1`).
fn kdl_int_prop(key: &str, value: i128) -> KdlEntry {
    let mut entry = KdlEntry::new_prop(key, KdlValue::Integer(value));
    entry.set_format(KdlEntryFormat {
        value_repr: value.to_string(),
        leading: " ".to_string(),
        ..Default::default()
    });
    entry
}

use crate::config::Config;
use crate::error::{KasmosError, LayoutError};
use crate::types::WorkPackage;

/// Generates Zellij KDL layout files for orchestration sessions.
///
/// The layout consists of a controller pane (left side) and an adaptive grid
/// of agent panes (right side), with dimensions calculated based on the number
/// of work packages.
pub struct LayoutGenerator {
    controller_width_pct: u32,
    opencode_binary: String,
    opencode_profile: Option<String>,
}

impl LayoutGenerator {
    /// Create a new layout generator from configuration.
    ///
    /// # Arguments
    /// * `config` - The kasmos configuration containing controller width and opencode binary path
    ///
    /// # Returns
    /// A new LayoutGenerator instance
    pub fn new(config: &Config) -> Self {
        Self {
            controller_width_pct: config.controller_width_pct,
            opencode_binary: config.opencode_binary.clone(),
            opencode_profile: config.opencode_profile.clone(),
        }
    }

    /// Generate a complete KDL layout document for the given work packages.
    ///
    /// # Arguments
    /// * `work_packages` - Slice of work packages to include in the layout
    /// * `feature_dir` - Path to the feature directory (for context)
    ///
    /// # Returns
    /// A KdlDocument representing the complete layout, or an error
    ///
    /// # Errors
    /// Returns `KasmosError::Layout` if:
    /// - work_packages is empty
    /// - KDL generation fails
    pub fn generate(
        &self,
        work_packages: &[&WorkPackage],
        feature_dir: &Path,
    ) -> Result<KdlDocument, KasmosError> {
        // Guard: validate input
        if work_packages.is_empty() {
            return Err(
                LayoutError::InvalidPaneCount("work_packages cannot be empty".to_string()).into(),
            );
        }

        debug!(
            "Generating KDL layout for {} work packages",
            work_packages.len()
        );

        // Build the root layout document
        let mut doc = KdlDocument::new();
        let layout_node = self.build_layout_node(work_packages, feature_dir)?;
        doc.nodes_mut().push(layout_node);

        // Start in locked mode so TUI apps (opencode) receive all keypresses.
        // Ctrl+G toggles back to normal Zellij mode for pane navigation.
        let mut default_mode = KdlNode::new("default_mode");
        default_mode.entries_mut().push(kdl_str_arg("locked"));
        doc.nodes_mut().push(default_mode);

        // Validate the generated KDL
        let kdl_string = doc.to_string();
        Self::validate_kdl(&kdl_string)?;

        info!(
            "Generated KDL layout with {} panes",
            work_packages.len() + 1
        );
        Ok(doc)
    }

    /// Calculate adaptive grid dimensions for a given number of panes.
    ///
    /// Uses a square-ish layout: cols = ceil(sqrt(n)), rows = ceil(n / cols)
    ///
    /// # Arguments
    /// * `pane_count` - Number of panes to arrange
    ///
    /// # Returns
    /// A tuple of (rows, cols)
    ///
    /// # Examples
    /// ```ignore
    /// assert_eq!(LayoutGenerator::grid_dimensions(1), (1, 1));
    /// assert_eq!(LayoutGenerator::grid_dimensions(4), (2, 2));
    /// assert_eq!(LayoutGenerator::grid_dimensions(5), (2, 3));
    /// ```
    pub fn grid_dimensions(pane_count: usize) -> (usize, usize) {
        if pane_count == 0 {
            return (0, 0);
        }

        let cols = ((pane_count as f64).sqrt().ceil()) as usize;
        let rows = ((pane_count as f64) / cols as f64).ceil() as usize;

        (rows, cols)
    }

    /// Build the root layout node with tab template (zjstatus) and content tab.
    fn build_layout_node(
        &self,
        work_packages: &[&WorkPackage],
        feature_dir: &Path,
    ) -> Result<KdlNode, KasmosError> {
        let mut layout = KdlNode::new("layout");

        // default_tab_template: adds zjstatus (bottom, Rose Pine Moon themed with zjstatus-hints)
        let tab_template = Self::build_tab_template();
        layout.ensure_children().nodes_mut().push(tab_template);

        // tab: the actual orchestration content
        let mut tab = KdlNode::new("tab");
        tab.entries_mut().push(kdl_str_prop("name", "kasmos"));

        // Main vertical split: controller (left) and agent grid (right)
        let mut main_split = KdlNode::new("pane");
        main_split
            .entries_mut()
            .push(kdl_str_prop("split_direction", "vertical"));

        let controller = self.build_controller_pane(feature_dir);
        main_split.ensure_children().nodes_mut().push(controller);

        let agent_grid = self.build_agent_grid(work_packages)?;
        main_split.ensure_children().nodes_mut().push(agent_grid);

        tab.ensure_children().nodes_mut().push(main_split);
        layout.ensure_children().nodes_mut().push(tab);
        Ok(layout)
    }

    /// Return the `default_tab_template` as a raw KDL string for embedding
    /// in hand-crafted layouts (e.g. the bootstrap layout in main.rs).
    ///
    /// Includes zjstatus (bottom, Rose Pine Moon with zjstatus-hints integration).
    pub fn tab_template_kdl_string() -> String {
        let template = Self::build_tab_template();
        let mut doc = KdlDocument::new();
        doc.nodes_mut().push(template);
        doc.to_string()
            .replace("#true", "true")
            .replace("#false", "false")
    }

    /// Build the `default_tab_template` with zjstatus + zjstatus-hints (Rose Pine Moon).
    ///
    /// Keybinding hints come from the `zjstatus-hints` background plugin (loaded
    /// globally in `config.kdl`), piped into zjstatus via `{pipe_zjstatus_hints}`.
    /// Pane frame management is handled by the `zjframes` background plugin.
    fn build_tab_template() -> KdlNode {
        let mut template = KdlNode::new("default_tab_template");

        // children placeholder (where tab content goes)
        let children = KdlNode::new("children");
        template.ensure_children().nodes_mut().push(children);

        // zjstatus bar pane (1 row, borderless, at the bottom — Rose Pine Moon theme)
        let mut bar_pane = KdlNode::new("pane");
        bar_pane.entries_mut().push(kdl_int_prop("size", 1));
        bar_pane
            .entries_mut()
            .push(kdl_bool_prop("borderless", true));

        let mut plugin = KdlNode::new("plugin");
        plugin.entries_mut().push(kdl_str_prop(
            "location",
            "file:~/.config/zellij/plugins/zjstatus.wasm",
        ));

        // Rose Pine Moon zjstatus configuration — matches opencode-kas.kdl theme.
        // Clean, minimal style (no powerline glyphs).
        let zjstatus_config: &[(&str, &str)] = &[
            // -- Rose Pine Moon palette
            ("color_bg", "#393552"),
            ("color_fg", "#e0def4"),
            ("color_sel", "#44415a"),
            ("color_blue", "#3e8fb0"),
            ("color_gold", "#f6c177"),
            ("color_rose", "#eb6f92"),
            ("color_pine", "#c4a7e7"),
            ("color_foam", "#9ccfd8"),
            // Format
            ("format_left", "{mode} {tabs}"),
            ("format_center", ""),
            ("format_right", "{pipe_zjstatus_hints}{datetime} "),
            ("format_space", ""),
            // Frame (zjframes handles frame toggling globally)
            ("hide_frame_for_single_pane", "true"),
            // zjstatus-hints pipe format (keybinding hints from background plugin)
            ("pipe_zjstatus_hints_format", "{output}"),
            // Mode indicators
            ("mode_normal", "#[bg=$blue,fg=$bg,bold] NORMAL "),
            ("mode_locked", "#[bg=$bg,fg=$fg,dim] LOCKED "),
            ("mode_pane", "#[bg=$pine,fg=$bg,bold] PANE "),
            ("mode_tab", "#[bg=$gold,fg=$bg,bold] TAB "),
            ("mode_resize", "#[bg=$foam,fg=$bg,bold] RESIZE "),
            ("mode_move", "#[bg=$foam,fg=$bg,bold] MOVE "),
            ("mode_scroll", "#[bg=$blue,fg=$bg,bold] SCROLL "),
            ("mode_search", "#[bg=$blue,fg=$bg,bold] SEARCH "),
            ("mode_enter_search", "#[bg=$blue,fg=$bg,bold] SEARCH "),
            ("mode_session", "#[bg=$rose,fg=$bg,bold] SESSION "),
            ("mode_rename_tab", "#[bg=$gold,fg=$bg,bold] RENAME TAB "),
            ("mode_rename_pane", "#[bg=$pine,fg=$bg,bold] RENAME PANE "),
            ("mode_tmux", "#[bg=$gold,fg=$bg,bold] TMUX "),
            // Tab styles
            ("tab_normal", "#[bg=$sel,fg=$fg] {name} "),
            ("tab_normal_fullscreen", "#[bg=$sel,fg=$fg] {name} [] "),
            ("tab_normal_sync", "#[bg=$sel,fg=$fg] {name} <> "),
            ("tab_active", "#[bg=$blue,fg=$bg,bold] {name} "),
            (
                "tab_active_fullscreen",
                "#[bg=$blue,fg=$bg,bold] {name} [] ",
            ),
            ("tab_active_sync", "#[bg=$blue,fg=$bg,bold] {name} <> "),
            ("tab_separator", "#[bg=$bg] "),
            // Datetime
            ("datetime", "#[bg=$bg,fg=$fg,dim]{format}"),
            ("datetime_format", "%H:%M"),
            ("datetime_timezone", "America/New_York"),
        ];

        for (key, value) in zjstatus_config {
            let mut node = KdlNode::new(*key);
            node.entries_mut().push(kdl_str_arg(value));
            plugin.ensure_children().nodes_mut().push(node);
        }

        bar_pane.ensure_children().nodes_mut().push(plugin);
        template.ensure_children().nodes_mut().push(bar_pane);

        template
    }

    /// Build the controller pane node.
    ///
    /// The controller pane runs `ocx oc` (opencode via ocx) and takes up the configured
    /// percentage of the terminal width.
    fn build_controller_pane(&self, feature_dir: &Path) -> KdlNode {
        let mut pane = KdlNode::new("pane");
        pane.entries_mut().push(kdl_str_prop(
            "size",
            &format!("{}%", self.controller_width_pct),
        ));
        pane.entries_mut().push(kdl_str_prop("name", "controller"));
        pane.entries_mut()
            .push(kdl_bool_prop("start_suspended", false));

        let mut cwd = KdlNode::new("cwd");
        cwd.entries_mut()
            .push(kdl_str_arg(&feature_dir.display().to_string()));
        pane.ensure_children().nodes_mut().push(cwd);

        let mut command = KdlNode::new("command");
        command
            .entries_mut()
            .push(kdl_str_arg(&self.opencode_binary));
        pane.ensure_children().nodes_mut().push(command);

        let mut args = KdlNode::new("args");
        args.entries_mut().push(kdl_str_arg("oc"));
        if let Some(ref profile) = self.opencode_profile {
            args.entries_mut().push(kdl_str_arg("-p"));
            args.entries_mut().push(kdl_str_arg(profile));
        }
        pane.ensure_children().nodes_mut().push(args);

        pane
    }

    /// Build the terminal pane for controller-only layouts.
    ///
    /// Opens in the repository root (two levels up from feature_dir:
    /// `kitty-specs/<slug>/` → repo root).
    fn build_terminal_pane(&self, feature_dir: &Path) -> KdlNode {
        // feature_dir is kitty-specs/<slug>/, parent is kitty-specs/, grandparent is repo root
        let repo_root = feature_dir
            .parent()
            .and_then(|p| p.parent())
            .unwrap_or(feature_dir);
        let mut terminal = KdlNode::new("pane");
        terminal
            .entries_mut()
            .push(kdl_str_prop("name", "terminal"));
        let mut cwd = KdlNode::new("cwd");
        cwd.entries_mut()
            .push(kdl_str_arg(&repo_root.display().to_string()));
        terminal.ensure_children().nodes_mut().push(cwd);
        terminal
    }

    /// Build a TUI controller pane that runs `kasmos tui-ctrl <feature>`.
    ///
    /// Used in controller-only layouts where there are no agent panes to display.
    /// The TUI shows orchestration status, wave progress, and keybinding hints.
    fn build_tui_pane(&self, feature_dir: &Path) -> KdlNode {
        // Run from repo root so relative paths work
        let repo_root = feature_dir
            .parent()
            .and_then(|p| p.parent())
            .unwrap_or(feature_dir);
        let feature_name = feature_dir
            .file_name()
            .and_then(|n| n.to_str())
            .unwrap_or("unknown");

        let mut pane = KdlNode::new("pane");
        pane.entries_mut().push(kdl_str_prop("name", "controller"));
        pane.entries_mut()
            .push(kdl_bool_prop("start_suspended", false));

        let mut cwd = KdlNode::new("cwd");
        cwd.entries_mut()
            .push(kdl_str_arg(&repo_root.display().to_string()));
        pane.ensure_children().nodes_mut().push(cwd);

        let mut command = KdlNode::new("command");
        command.entries_mut().push(kdl_str_arg("kasmos"));
        pane.ensure_children().nodes_mut().push(command);

        let mut args = KdlNode::new("args");
        args.entries_mut().push(kdl_str_arg("tui-ctrl"));
        args.entries_mut().push(kdl_str_arg(feature_name));
        pane.ensure_children().nodes_mut().push(args);

        pane
    }

    /// Build a single agent pane node.
    ///
    /// Each agent pane runs `ocx oc --prompt "$(cat <file>)"` via bash.
    fn build_agent_pane(&self, wp: &WorkPackage) -> KdlNode {
        let mut pane = KdlNode::new("pane");
        pane.entries_mut().push(kdl_str_prop("name", &wp.pane_name));
        pane.entries_mut()
            .push(kdl_bool_prop("start_suspended", false));

        // Command: bash
        let mut command = KdlNode::new("command");
        command.entries_mut().push(kdl_str_arg("bash"));
        pane.ensure_children().nodes_mut().push(command);

        // Args: -c "ocx oc --prompt \"$(cat <prompt>)\""
        let mut args = KdlNode::new("args");
        args.entries_mut().push(kdl_str_arg("-c"));

        let ocx = escape(Cow::Borrowed(&self.opencode_binary));
        let profile_flag = match &self.opencode_profile {
            Some(p) => format!(" -p {}", escape(Cow::Borrowed(p))),
            None => String::new(),
        };
        let shell_cmd = if let Some(prompt_path) = &wp.prompt_path {
            let path = escape(Cow::Owned(prompt_path.display().to_string()));
            format!("{ocx} oc{profile_flag} -- --agent coder --prompt \"$(cat {path})\"")
        } else {
            format!("{ocx} oc{profile_flag} -- --agent coder")
        };

        args.entries_mut().push(kdl_str_arg(&shell_cmd));
        pane.ensure_children().nodes_mut().push(args);

        // Cwd: working directory if specified
        if let Some(worktree_path) = &wp.worktree_path {
            let mut cwd = KdlNode::new("cwd");
            cwd.entries_mut()
                .push(kdl_str_arg(&worktree_path.display().to_string()));
            pane.ensure_children().nodes_mut().push(cwd);
        }

        pane
    }

    /// Build the agent grid pane containing all agent panes.
    ///
    /// Arranges agents in rows and columns based on adaptive grid dimensions.
    fn build_agent_grid(&self, work_packages: &[&WorkPackage]) -> Result<KdlNode, KasmosError> {
        let (rows, cols) = Self::grid_dimensions(work_packages.len());

        let mut grid = KdlNode::new("pane");
        grid.entries_mut().push(kdl_str_prop(
            "size",
            &format!("{}%", 100 - self.controller_width_pct),
        ));
        grid.entries_mut()
            .push(kdl_str_prop("split_direction", "horizontal"));

        let mut pane_idx = 0;

        // Create rows
        for _row in 0..rows {
            // Guard: don't create empty rows
            if pane_idx >= work_packages.len() {
                break;
            }

            let mut row = KdlNode::new("pane");
            row.entries_mut()
                .push(kdl_str_prop("split_direction", "vertical"));

            // Calculate how many panes this row should have
            let panes_in_row = std::cmp::min(cols, work_packages.len() - pane_idx);

            // Create columns within this row
            for _ in 0..panes_in_row {
                let agent_pane = self.build_agent_pane(work_packages[pane_idx]);
                row.ensure_children().nodes_mut().push(agent_pane);
                pane_idx += 1;
            }

            grid.ensure_children().nodes_mut().push(row);
        }

        Ok(grid)
    }

    /// Build a full-width agent grid for standalone wave tabs.
    fn build_agent_grid_fullwidth(
        &self,
        work_packages: &[&WorkPackage],
    ) -> Result<KdlNode, KasmosError> {
        let (rows, cols) = Self::grid_dimensions(work_packages.len());

        let mut grid = KdlNode::new("pane");
        grid.entries_mut()
            .push(kdl_str_prop("split_direction", "horizontal"));

        let mut pane_idx = 0;

        for _row in 0..rows {
            if pane_idx >= work_packages.len() {
                break;
            }

            let mut row = KdlNode::new("pane");
            row.entries_mut()
                .push(kdl_str_prop("split_direction", "vertical"));

            let panes_in_row = std::cmp::min(cols, work_packages.len() - pane_idx);

            for _ in 0..panes_in_row {
                let agent_pane = self.build_agent_pane(work_packages[pane_idx]);
                row.ensure_children().nodes_mut().push(agent_pane);
                pane_idx += 1;
            }

            grid.ensure_children().nodes_mut().push(row);
        }

        Ok(grid)
    }

    /// Generate a controller-only layout (no agent panes).
    ///
    /// Uses the kasmos TUI dashboard in the main pane and a terminal pane
    /// on the side for manual commands.
    pub fn generate_controller_only(&self, feature_dir: &Path) -> Result<KdlDocument, KasmosError> {
        debug!("Generating controller-only KDL layout");

        let mut doc = KdlDocument::new();

        let mut layout = KdlNode::new("layout");
        layout
            .ensure_children()
            .nodes_mut()
            .push(Self::build_tab_template());

        let mut tab = KdlNode::new("tab");
        tab.entries_mut().push(kdl_str_prop("name", "kasmos"));

        let mut main_split = KdlNode::new("pane");
        main_split
            .entries_mut()
            .push(kdl_str_prop("split_direction", "vertical"));
        main_split
            .ensure_children()
            .nodes_mut()
            .push(self.build_tui_pane(feature_dir));
        main_split
            .ensure_children()
            .nodes_mut()
            .push(self.build_terminal_pane(feature_dir));

        tab.ensure_children().nodes_mut().push(main_split);
        layout.ensure_children().nodes_mut().push(tab);
        doc.nodes_mut().push(layout);

        // Don't use locked mode for controller-only — the TUI handles its
        // own input and the terminal pane needs normal shell interaction.
        let mut default_mode = KdlNode::new("default_mode");
        default_mode.entries_mut().push(kdl_str_arg("normal"));
        doc.nodes_mut().push(default_mode);

        let kdl_string = doc.to_string();
        Self::validate_kdl(&kdl_string)?;

        info!("Generated controller-only KDL layout");
        Ok(doc)
    }

    /// Generate an agent-only layout for wave tabs (no controller pane).
    pub fn generate_wave_tab(
        &self,
        work_packages: &[&WorkPackage],
        _feature_dir: &Path,
    ) -> Result<KdlDocument, KasmosError> {
        if work_packages.is_empty() {
            return Err(
                LayoutError::InvalidPaneCount("work_packages cannot be empty".to_string()).into(),
            );
        }

        debug!(
            "Generating wave tab KDL layout for {} agent panes",
            work_packages.len()
        );

        let mut doc = KdlDocument::new();
        let mut layout = KdlNode::new("layout");

        // default_tab_template: ensures compact-bar (bottom) is present on wave tabs
        let tab_template = Self::build_tab_template();
        layout.ensure_children().nodes_mut().push(tab_template);

        // Wrap agent grid in a tab node so default_tab_template applies
        let mut tab = KdlNode::new("tab");
        let agent_grid = self.build_agent_grid_fullwidth(work_packages)?;
        tab.ensure_children().nodes_mut().push(agent_grid);
        layout.ensure_children().nodes_mut().push(tab);

        doc.nodes_mut().push(layout);

        let mut default_mode = KdlNode::new("default_mode");
        default_mode.entries_mut().push(kdl_str_arg("locked"));
        doc.nodes_mut().push(default_mode);

        let kdl_string = doc.to_string();
        Self::validate_kdl(&kdl_string)?;

        info!(
            "Generated wave tab KDL layout with {} agent panes",
            work_packages.len()
        );
        Ok(doc)
    }

    /// Write a wave-specific layout file without replacing `layout.kdl`.
    pub fn write_wave_layout(
        &self,
        doc: &KdlDocument,
        output_dir: &Path,
        wave_index: usize,
    ) -> Result<PathBuf, KasmosError> {
        std::fs::create_dir_all(output_dir)?;

        let output_path = output_dir.join(format!("wave-{}.kdl", wave_index));
        let kdl_string = doc
            .to_string()
            .replace("#true", "true")
            .replace("#false", "false");

        std::fs::write(&output_path, &kdl_string)?;
        info!("Wrote wave layout to {}", output_path.display());
        Ok(output_path)
    }

    /// Write the KDL document to a file.
    ///
    /// # Arguments
    /// * `doc` - The KdlDocument to write
    /// * `output_dir` - Directory where the layout.kdl file will be written
    ///
    /// # Returns
    /// The path to the written file, or an error
    ///
    /// # Errors
    /// Returns `KasmosError::Io` if file operations fail
    pub fn write_layout(
        &self,
        doc: &KdlDocument,
        output_dir: &Path,
    ) -> Result<PathBuf, KasmosError> {
        // Create output directory if it doesn't exist
        std::fs::create_dir_all(output_dir)?;

        let output_path = output_dir.join("layout.kdl");
        // Downgrade KDL v2 booleans to v1 for Zellij 0.44 compatibility
        let kdl_string = doc
            .to_string()
            .replace("#true", "true")
            .replace("#false", "false");

        std::fs::write(&output_path, &kdl_string)?;

        info!("Wrote layout to {}", output_path.display());
        Ok(output_path)
    }

    /// Validate KDL by parsing it back.
    ///
    /// # Arguments
    /// * `kdl_string` - The KDL string to validate
    ///
    /// # Returns
    /// Ok(()) if valid, or a KasmosError if parsing fails
    fn validate_kdl(kdl_string: &str) -> Result<(), KasmosError> {
        KdlDocument::parse(kdl_string).map_err(|e| {
            KasmosError::Layout(LayoutError::KdlValidation(format!(
                "Failed to parse KDL: {}",
                e
            )))
        })?;

        debug!("KDL validation passed");
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::types::WPState;
    use std::path::PathBuf;

    fn make_test_wp(id: &str, pane_name: &str) -> WorkPackage {
        WorkPackage {
            id: id.to_string(),
            title: format!("Test WP {}", id),
            state: WPState::Pending,
            dependencies: vec![],
            wave: 0,
            pane_id: None,
            pane_name: pane_name.to_string(),
            worktree_path: Some(PathBuf::from(format!("/tmp/worktree/{}", id))),
            prompt_path: Some(PathBuf::from(format!("/tmp/prompts/{}/prompt.md", id))),
            started_at: None,
            completed_at: None,
            completion_method: None,
            failure_count: 0,
        }
    }

    #[test]
    fn test_grid_dimensions() {
        assert_eq!(LayoutGenerator::grid_dimensions(1), (1, 1));
        assert_eq!(LayoutGenerator::grid_dimensions(2), (1, 2));
        assert_eq!(LayoutGenerator::grid_dimensions(3), (2, 2));
        assert_eq!(LayoutGenerator::grid_dimensions(4), (2, 2));
        assert_eq!(LayoutGenerator::grid_dimensions(5), (2, 3));
        assert_eq!(LayoutGenerator::grid_dimensions(6), (2, 3));
        assert_eq!(LayoutGenerator::grid_dimensions(7), (3, 3));
        assert_eq!(LayoutGenerator::grid_dimensions(8), (3, 3));
    }

    #[test]
    fn test_generate_single_pane() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wp = make_test_wp("WP01", "agent-1");
        let wps = vec![&wp];

        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        assert!(kdl_str.contains("layout"));
        assert!(kdl_str.contains("controller"));
        assert!(kdl_str.contains("agent-1"));
        assert!(kdl_str.contains("ocx"));
    }

    #[test]
    fn test_generate_four_panes() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wps: Vec<_> = (1..=4)
            .map(|i| make_test_wp(&format!("WP{:02}", i), &format!("agent-{}", i)))
            .collect();
        let wp_refs: Vec<_> = wps.iter().collect();

        let doc = generator
            .generate(&wp_refs, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        assert!(kdl_str.contains("agent-1"));
        assert!(kdl_str.contains("agent-2"));
        assert!(kdl_str.contains("agent-3"));
        assert!(kdl_str.contains("agent-4"));
    }

    #[test]
    fn test_generate_eight_panes() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wps: Vec<_> = (1..=8)
            .map(|i| make_test_wp(&format!("WP{:02}", i), &format!("agent-{}", i)))
            .collect();
        let wp_refs: Vec<_> = wps.iter().collect();

        let doc = generator
            .generate(&wp_refs, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        for i in 1..=8 {
            assert!(kdl_str.contains(&format!("agent-{}", i)));
        }
    }

    #[test]
    fn test_validate_kdl_round_trip() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wp = make_test_wp("WP01", "agent-1");
        let wps = vec![&wp];

        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        // Should not panic or error
        let result = LayoutGenerator::validate_kdl(&kdl_str);
        assert!(result.is_ok());
    }

    #[test]
    fn test_controller_pane_size() {
        let config = Config {
            controller_width_pct: 35,
            ..Default::default()
        };
        let generator = LayoutGenerator::new(&config);

        let wp = make_test_wp("WP01", "agent-1");
        let wps = vec![&wp];

        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        assert!(kdl_str.contains("size=\"35%\""));
    }

    #[test]
    fn test_controller_pane_cwd_uses_feature_dir() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wp = make_test_wp("WP01", "agent-1");
        let wps = vec![&wp];

        let feature_dir = Path::new("/tmp/feature-123");
        let doc = generator.generate(&wps, feature_dir).expect("generate");
        let kdl_str = doc.to_string();

        assert!(kdl_str.contains("cwd \"/tmp/feature-123\""));
    }

    #[test]
    fn test_write_layout() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wp = make_test_wp("WP01", "agent-1");
        let wps = vec![&wp];

        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");

        let temp_dir = std::env::temp_dir().join("kasmos_layout_test");
        let result = generator.write_layout(&doc, &temp_dir);

        assert!(result.is_ok());
        let path = result.unwrap();
        assert!(path.exists());
        assert!(path.ends_with("layout.kdl"));

        // Clean up
        let _ = std::fs::remove_file(&path);
        let _ = std::fs::remove_dir(&temp_dir);
    }

    #[test]
    fn test_pane_attributes() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let wp = make_test_wp("WP01", "test-pane");
        let wps = vec![&wp];

        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        // Check pane name (now quoted for Zellij compatibility)
        assert!(kdl_str.contains("name=\"test-pane\""));

        // Check command (quoted)
        assert!(kdl_str.contains("command \"bash\""));

        // Check args with ocx oc invocation
        assert!(kdl_str.contains("args \"-c\""));
        assert!(kdl_str.contains("ocx oc"));

        // Check cwd (quoted)
        assert!(kdl_str.contains("cwd \"/tmp/worktree/WP01\""));
    }

    #[test]
    fn test_pane_without_worktree_path() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let mut wp = make_test_wp("WP01", "agent-1");
        wp.worktree_path = None;

        let wps = vec![&wp];
        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        // Should still generate valid KDL
        assert!(kdl_str.contains("agent-1"));
        assert!(kdl_str.contains("command \"bash\""));
    }

    #[test]
    fn test_pane_without_prompt_path() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let mut wp = make_test_wp("WP01", "agent-1");
        wp.prompt_path = None;

        let wps = vec![&wp];
        let doc = generator
            .generate(&wps, Path::new("/tmp"))
            .expect("generate");
        let kdl_str = doc.to_string();

        // Should still generate valid KDL with ocx oc command
        assert!(kdl_str.contains("agent-1"));
        assert!(kdl_str.contains("ocx oc"));
    }

    #[test]
    fn test_generate_empty_work_packages() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);
        let wps: Vec<&WorkPackage> = vec![];
        let result = generator.generate(&wps, Path::new("/tmp"));
        assert!(result.is_err());
    }

    #[test]
    fn test_command_injection_prevention() {
        let config = Config::default();
        let generator = LayoutGenerator::new(&config);

        let mut wp = make_test_wp("WP01", "test-pane");
        wp.prompt_path = Some(PathBuf::from("/tmp/test; rm -rf /"));
        wp.worktree_path = Some(PathBuf::from("/tmp/worktree/$(whoami)"));

        let wps: Vec<&WorkPackage> = vec![&wp];
        let result = generator.generate(&wps, Path::new("/tmp")).unwrap();
        let kdl_string = result.to_string();

        // The path should be escaped/quoted so it's treated as a literal string
        // shell-escape will quote the entire path, preventing command injection
        assert!(kdl_string.contains("ocx oc"));
        // The path should be present but safely escaped (quoted with single quotes)
        assert!(
            kdl_string.contains("'/tmp/test; rm -rf /'")
                || kdl_string.contains("\"/tmp/test; rm -rf /\"")
        );
    }
}
