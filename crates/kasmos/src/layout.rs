//! KDL layout generator for Zellij orchestration sessions.
//!
//! This module generates valid Zellij KDL layout files that organize
//! a controller pane and multiple agent panes in an adaptive grid layout.

use kdl::{KdlDocument, KdlNode, KdlValue};
use shell_escape::escape;
use std::borrow::Cow;
use std::path::{Path, PathBuf};
use tracing::{debug, info};

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
        _feature_dir: &Path,
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
        let layout_node = self.build_layout_node(work_packages)?;
        doc.nodes_mut().push(layout_node);

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

    /// Build the root layout node containing controller and agent grid.
    fn build_layout_node(&self, work_packages: &[&WorkPackage]) -> Result<KdlNode, KasmosError> {
        let mut layout = KdlNode::new("layout");

        // Main vertical split: controller (left) and agent grid (right)
        let mut main_split = KdlNode::new("pane");
        main_split.insert("split_direction", KdlValue::String("vertical".to_string()));

        // Add controller pane
        let controller = self.build_controller_pane();
        main_split.ensure_children().nodes_mut().push(controller);

        // Add agent grid pane
        let agent_grid = self.build_agent_grid(work_packages)?;
        main_split.ensure_children().nodes_mut().push(agent_grid);

        layout.ensure_children().nodes_mut().push(main_split);
        Ok(layout)
    }

    /// Build the controller pane node.
    ///
    /// The controller pane runs the opencode binary and takes up the configured
    /// percentage of the terminal width.
    fn build_controller_pane(&self) -> KdlNode {
        let mut pane = KdlNode::new("pane");
        pane.insert(
            "size",
            KdlValue::String(format!("{}%", self.controller_width_pct)),
        );
        pane.insert("name", KdlValue::String("controller".to_string()));

        let mut command = KdlNode::new("command");
        command.push(KdlValue::String(self.opencode_binary.clone()));

        pane.ensure_children().nodes_mut().push(command);
        pane
    }

    /// Build a single agent pane node.
    ///
    /// Each agent pane runs a bash command that pipes the prompt file to opencode.
    fn build_agent_pane(&self, wp: &WorkPackage) -> KdlNode {
        let mut pane = KdlNode::new("pane");
        pane.insert("name", KdlValue::String(wp.pane_name.clone()));

        // Command: bash
        let mut command = KdlNode::new("command");
        command.push(KdlValue::String("bash".to_string()));
        pane.ensure_children().nodes_mut().push(command);

        // Args: -c "cat <prompt> | opencode -p 'context:'"
        let mut args = KdlNode::new("args");
        args.push(KdlValue::String("-c".to_string()));

        let pipe_cmd = if let Some(prompt_path) = &wp.prompt_path {
            format!(
                "cat {} | {} -p 'context:'",
                escape(Cow::Owned(prompt_path.display().to_string())),
                escape(Cow::Borrowed(&self.opencode_binary))
            )
        } else {
            format!(
                "{} -p 'context:'",
                escape(Cow::Borrowed(&self.opencode_binary))
            )
        };

        args.push(KdlValue::String(pipe_cmd));
        pane.ensure_children().nodes_mut().push(args);

        // Cwd: working directory if specified
        if let Some(worktree_path) = &wp.worktree_path {
            let mut cwd = KdlNode::new("cwd");
            cwd.push(KdlValue::String(worktree_path.display().to_string()));
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
        grid.insert(
            "size",
            KdlValue::String(format!("{}%", 100 - self.controller_width_pct)),
        );
        grid.insert(
            "split_direction",
            KdlValue::String("horizontal".to_string()),
        );

        let mut pane_idx = 0;

        // Create rows
        for _row in 0..rows {
            // Guard: don't create empty rows
            if pane_idx >= work_packages.len() {
                break;
            }

            let mut row = KdlNode::new("pane");
            row.insert("split_direction", KdlValue::String("vertical".to_string()));

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
        let kdl_string = doc.to_string();

        std::fs::write(&output_path, kdl_string)?;

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
        assert!(kdl_str.contains("opencode"));
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
        let mut config = Config::default();
        config.controller_width_pct = 35;
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

        // Check pane name
        assert!(kdl_str.contains("name=test-pane"));

        // Check command
        assert!(kdl_str.contains("command bash"));

        // Check args with pipe
        assert!(kdl_str.contains("args -c"));
        assert!(kdl_str.contains("opencode"));

        // Check cwd
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
        assert!(kdl_str.contains("command bash"));
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

        // Should still generate valid KDL with just opencode command
        assert!(kdl_str.contains("agent-1"));
        assert!(kdl_str.contains("opencode"));
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
        // Check that the command contains the escaped path (quoted)
        assert!(kdl_string.contains("cat"));
        assert!(kdl_string.contains("opencode"));
        // The path should be present but safely escaped (quoted with single quotes)
        assert!(
            kdl_string.contains("'/tmp/test; rm -rf /'")
                || kdl_string.contains("\"/tmp/test; rm -rf /\"")
        );
    }
}
