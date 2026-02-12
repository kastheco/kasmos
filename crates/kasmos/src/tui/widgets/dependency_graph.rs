//! WP dependency graph visualization using tui-nodes.
//!
//! Converts an [`OrchestrationRun`]'s work packages and their dependency
//! relationships into a tui-nodes [`NodeGraph`] for rendering in the Dashboard.

use std::collections::{HashMap, HashSet};

use ratatui::layout::Rect;
use ratatui::style::{Color, Style};
use ratatui::widgets::BorderType;
use tui_nodes::{Connection, NodeGraph, NodeLayout};

use crate::types::{OrchestrationRun, WPState};

/// Map a WP state to a visual border style for graph nodes.
pub fn state_to_style(state: WPState) -> Style {
    match state {
        WPState::Pending => Style::default().fg(Color::DarkGray),
        WPState::Active => Style::default().fg(Color::Yellow),
        WPState::Completed => Style::default().fg(Color::Green),
        WPState::Failed => Style::default().fg(Color::Red),
        WPState::ForReview => Style::default().fg(Color::Magenta),
        WPState::Paused => Style::default().fg(Color::Blue),
    }
}

/// Build a tui-nodes graph from an [`OrchestrationRun`].
///
/// Returns the graph and a boolean indicating whether cycles were detected.
/// The caller must call `graph.calculate()` before rendering.
///
/// # Arguments
///
/// * `run` — The orchestration run whose WPs become graph nodes.
/// * `area` — The available render area (width/height needed by `NodeGraph::new`).
pub fn build_dependency_graph<'a>(run: &'a OrchestrationRun, area: Rect) -> (NodeGraph<'a>, bool) {
    let has_cycles = detect_cycles(run);

    // Build an index from WP id → node index for connection lookups.
    let id_to_index: HashMap<&str, usize> = run
        .work_packages
        .iter()
        .enumerate()
        .map(|(i, wp)| (wp.id.as_str(), i))
        .collect();

    // Estimate node size: title width + border padding, fixed height of 3 rows.
    let nodes: Vec<NodeLayout<'a>> = run
        .work_packages
        .iter()
        .map(|wp| {
            let label = format!("{}: {}", wp.id, wp.title);
            // Width = label length + 2 (borders), minimum 12 for short titles.
            // Height = 3 (top border + 1 content line + bottom border).
            let width = (label.len() as u16 + 2).max(12);
            let height = 3;
            NodeLayout::new((width, height))
                .with_title(Box::leak(label.into_boxed_str()))
                .with_border_style(state_to_style(wp.state))
                .with_border_type(BorderType::Rounded)
        })
        .collect();

    // Build connections: for each WP, create an edge from each dependency → this WP.
    // tui-nodes uses numeric node indices and port indices.
    let mut connections = Vec::new();
    for (to_idx, wp) in run.work_packages.iter().enumerate() {
        for (port, dep_id) in wp.dependencies.iter().enumerate() {
            if let Some(&from_idx) = id_to_index.get(dep_id.as_str()) {
                // Edge from the dependency node (output port 0) to this node (input port).
                // We use port 0 for the from-side and increment the to-port per dependency.
                connections.push(Connection::new(from_idx, 0, to_idx, port));
            }
        }
    }

    let width = area.width as usize;
    let height = area.height as usize;

    let mut graph = NodeGraph::new(nodes, connections, width, height);
    graph.calculate();

    (graph, has_cycles)
}

/// Detect cycles in the WP dependency graph using DFS.
///
/// Returns `true` if any cycle is found.
pub fn detect_cycles(run: &OrchestrationRun) -> bool {
    let adj: HashMap<&str, Vec<&str>> = run
        .work_packages
        .iter()
        .map(|wp| {
            (
                wp.id.as_str(),
                wp.dependencies.iter().map(|d| d.as_str()).collect(),
            )
        })
        .collect();

    let mut visited = HashSet::new();
    let mut in_stack = HashSet::new();

    for wp in &run.work_packages {
        if !visited.contains(wp.id.as_str())
            && dfs_has_cycle(wp.id.as_str(), &adj, &mut visited, &mut in_stack)
        {
            return true;
        }
    }
    false
}

fn dfs_has_cycle<'a>(
    node: &'a str,
    adj: &HashMap<&str, Vec<&'a str>>,
    visited: &mut HashSet<&'a str>,
    in_stack: &mut HashSet<&'a str>,
) -> bool {
    visited.insert(node);
    in_stack.insert(node);

    if let Some(deps) = adj.get(node) {
        for dep in deps {
            if !visited.contains(dep) {
                if dfs_has_cycle(dep, adj, visited, in_stack) {
                    return true;
                }
            } else if in_stack.contains(dep) {
                return true; // Back edge — cycle detected
            }
        }
    }

    in_stack.remove(node);
    false
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::types::{OrchestrationRun, ProgressionMode, RunState, Wave, WaveState, WorkPackage};

    fn make_wp(id: &str, deps: Vec<String>) -> WorkPackage {
        WorkPackage {
            id: id.to_string(),
            title: format!("Work package {id}"),
            state: WPState::Pending,
            dependencies: deps,
            wave: 0,
            pane_id: None,
            pane_name: id.to_ascii_lowercase(),
            worktree_path: None,
            prompt_path: None,
            started_at: None,
            completed_at: None,
            completion_method: None,
            failure_count: 0,
        }
    }

    fn make_run(wps: Vec<WorkPackage>) -> OrchestrationRun {
        OrchestrationRun {
            id: "run-test".to_string(),
            feature: "test-feature".to_string(),
            feature_dir: std::path::PathBuf::from("/tmp/test"),
            config: Config::default(),
            work_packages: wps,
            waves: vec![Wave {
                index: 0,
                wp_ids: vec![],
                state: WaveState::Pending,
            }],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[test]
    fn test_no_cycle_in_linear_chain() {
        let run = make_run(vec![
            make_wp("WP01", vec![]),
            make_wp("WP02", vec!["WP01".to_string()]),
            make_wp("WP03", vec!["WP02".to_string()]),
        ]);
        assert!(!detect_cycles(&run));
    }

    #[test]
    fn test_cycle_detected_a_b_c_a() {
        let run = make_run(vec![
            make_wp("A", vec!["C".to_string()]),
            make_wp("B", vec!["A".to_string()]),
            make_wp("C", vec!["B".to_string()]),
        ]);
        assert!(detect_cycles(&run));
    }

    #[test]
    fn test_no_cycle_with_shared_dependency() {
        // Diamond: WP03 and WP04 both depend on WP01; WP05 depends on both.
        let run = make_run(vec![
            make_wp("WP01", vec![]),
            make_wp("WP02", vec![]),
            make_wp("WP03", vec!["WP01".to_string()]),
            make_wp("WP04", vec!["WP01".to_string()]),
            make_wp("WP05", vec!["WP03".to_string(), "WP04".to_string()]),
        ]);
        assert!(!detect_cycles(&run));
    }

    #[test]
    fn test_no_cycle_with_no_dependencies() {
        let run = make_run(vec![
            make_wp("WP01", vec![]),
            make_wp("WP02", vec![]),
            make_wp("WP03", vec![]),
        ]);
        assert!(!detect_cycles(&run));
    }

    #[test]
    fn test_self_cycle() {
        let run = make_run(vec![make_wp("WP01", vec!["WP01".to_string()])]);
        assert!(detect_cycles(&run));
    }

    #[test]
    fn test_state_to_style_maps_all_states() {
        // Ensure all WPState variants produce distinct styles.
        let states = [
            WPState::Pending,
            WPState::Active,
            WPState::Completed,
            WPState::Failed,
            WPState::ForReview,
            WPState::Paused,
        ];
        for state in &states {
            let style = state_to_style(*state);
            assert_ne!(
                style,
                Style::default(),
                "state {state:?} should have a color"
            );
        }
    }
}
