//! WP dependency graph adapter for `tui-nodes`.
//!
//! Builds a `NodeGraph` from an `OrchestrationRun`, mapping work packages to
//! nodes and their dependency relationships to directed edges. Includes cycle
//! detection to prevent infinite layout loops.
//!
//! Not yet wired into the TUI — will be used by the dependency graph tab.

use std::collections::{HashMap, HashSet};

use ratatui::style::{Color, Style};
use ratatui::widgets::Paragraph;
use ratatui::{layout::Rect, Frame};
use tui_nodes::{Connection, NodeGraph, NodeLayout};

use crate::types::{OrchestrationRun, WPState};

/// Minimum node width in terminal columns.
const NODE_WIDTH: u16 = 20;
/// Node height in terminal rows (border + 1 content line + border).
const NODE_HEIGHT: u16 = 3;

/// Map a `WPState` to a border/fill color for graph nodes.
///
/// Color mapping per spec (research.md R-7):
/// - Pending → DarkGray
/// - Active → Yellow
/// - Completed → Green
/// - Failed → Red
/// - ForReview → Magenta
/// - Paused → Blue
fn state_to_style(state: WPState) -> Style {
    match state {
        WPState::Pending => Style::default().fg(Color::DarkGray),
        WPState::Active => Style::default().fg(Color::Yellow),
        WPState::Completed => Style::default().fg(Color::Green),
        WPState::Failed => Style::default().fg(Color::Red),
        WPState::ForReview => Style::default().fg(Color::Magenta),
        WPState::Paused => Style::default().fg(Color::Blue),
    }
}

/// Detect cycles in the WP dependency graph using DFS.
///
/// Returns the set of WP IDs that participate in at least one cycle.
pub fn detect_cycles(run: &OrchestrationRun) -> Vec<String> {
    // Build adjacency: wp_id -> list of dependents (WPs that depend on it)
    // But for cycle detection, we track: for each WP, its dependencies
    let wp_ids: HashSet<&str> = run.work_packages.iter().map(|wp| wp.id.as_str()).collect();

    // Build dependency map: wp_id -> dependencies
    let deps: HashMap<&str, Vec<&str>> = run
        .work_packages
        .iter()
        .map(|wp| {
            let valid_deps: Vec<&str> = wp
                .dependencies
                .iter()
                .filter(|d| wp_ids.contains(d.as_str()))
                .map(|d| d.as_str())
                .collect();
            (wp.id.as_str(), valid_deps)
        })
        .collect();

    let mut cycle_nodes = HashSet::new();
    let mut visited = HashSet::new();
    let mut rec_stack = HashSet::new();

    for wp in &run.work_packages {
        if !visited.contains(wp.id.as_str()) {
            let mut path = Vec::new();
            dfs_detect_cycle(
                wp.id.as_str(),
                &deps,
                &mut visited,
                &mut rec_stack,
                &mut path,
                &mut cycle_nodes,
            );
        }
    }

    cycle_nodes.into_iter().map(|s| s.to_string()).collect()
}

fn dfs_detect_cycle<'a>(
    node: &'a str,
    deps: &HashMap<&'a str, Vec<&'a str>>,
    visited: &mut HashSet<&'a str>,
    rec_stack: &mut HashSet<&'a str>,
    path: &mut Vec<&'a str>,
    cycle_nodes: &mut HashSet<&'a str>,
) {
    visited.insert(node);
    rec_stack.insert(node);
    path.push(node);

    if let Some(neighbors) = deps.get(node) {
        for &neighbor in neighbors {
            if !visited.contains(neighbor) {
                dfs_detect_cycle(neighbor, deps, visited, rec_stack, path, cycle_nodes);
            } else if rec_stack.contains(neighbor) {
                // Found a cycle — mark all nodes in the cycle
                let cycle_start = path.iter().position(|&n| n == neighbor).unwrap_or(0);
                for &cycle_node in &path[cycle_start..] {
                    cycle_nodes.insert(cycle_node);
                }
                cycle_nodes.insert(neighbor);
            }
        }
    }

    path.pop();
    rec_stack.remove(node);
}

/// Render the WP dependency graph into the given area.
///
/// Builds a `NodeGraph` from the orchestration run's work packages and their
/// dependency relationships, then renders it using `tui-nodes`.
///
/// If cycles are detected, a warning banner is rendered and a text-based
/// fallback is used instead of `tui-nodes` (which cannot handle cycles).
pub fn render_dependency_graph(run: &OrchestrationRun, frame: &mut Frame, area: Rect) {
    if run.work_packages.is_empty() {
        let empty = Paragraph::new("No work packages to display")
            .style(Style::default().fg(Color::DarkGray));
        frame.render_widget(empty, area);
        return;
    }

    // Check for cycles
    let cycle_wp_ids = detect_cycles(run);
    let has_cycles = !cycle_wp_ids.is_empty();

    // If cycles detected, render a text-based fallback instead of tui-nodes
    // (tui-nodes panics on cyclic graphs)
    if has_cycles {
        render_cycle_fallback(run, &cycle_wp_ids, frame, area);
        return;
    }

    if area.width < 4 || area.height < 3 {
        return;
    }

    // Build WP ID → node index mapping
    let wp_index: HashMap<&str, usize> = run
        .work_packages
        .iter()
        .enumerate()
        .map(|(i, wp)| (wp.id.as_str(), i))
        .collect();

    // Build node labels in an arena so they outlive the NodeLayout borrows
    // without leaking memory (unlike Box::leak which would leak on every frame).
    let labels: Vec<String> = run
        .work_packages
        .iter()
        .map(|wp| format!("{}: {}", wp.id, truncate_title(&wp.title, 14)))
        .collect();

    // Build node layouts (borrows from labels arena)
    let nodes: Vec<NodeLayout> = run
        .work_packages
        .iter()
        .zip(labels.iter())
        .map(|(wp, label)| {
            let style = state_to_style(wp.state);
            NodeLayout::new((NODE_WIDTH, NODE_HEIGHT))
                .with_title(label.as_str())
                .with_border_style(style)
        })
        .collect();

    // Build connections from dependency relationships.
    // In tui-nodes: Connection::new(from_node, from_port, to_node, to_port)
    // Dependency semantics: wp.dependencies lists upstream WPs that this WP depends on.
    // Visual: dependency (from_node) → dependent (to_node), i.e., edge from dep to wp.
    let mut connections = Vec::new();
    let mut port_counters_out: HashMap<usize, usize> = HashMap::new();
    let mut port_counters_in: HashMap<usize, usize> = HashMap::new();

    for wp in &run.work_packages {
        let to_idx = match wp_index.get(wp.id.as_str()) {
            Some(&idx) => idx,
            None => continue,
        };

        for dep_id in &wp.dependencies {
            let from_idx = match wp_index.get(dep_id.as_str()) {
                Some(&idx) => idx,
                None => continue, // skip missing dependencies
            };

            let from_port = *port_counters_out.entry(from_idx).or_insert(0);
            let to_port = *port_counters_in.entry(to_idx).or_insert(0);
            port_counters_out.insert(from_idx, from_port + 1);
            port_counters_in.insert(to_idx, to_port + 1);

            connections.push(Connection::new(from_idx, from_port, to_idx, to_port));
        }
    }

    // Build and render the node graph
    let mut graph = NodeGraph::new(
        nodes,
        connections,
        area.width as usize,
        area.height as usize,
    );
    graph.calculate();

    let mut state = ();
    frame.render_stateful_widget(graph, area, &mut state);
}

/// Render a text-based fallback when cycles are detected in the dependency graph.
///
/// Shows a warning banner and lists all WPs with their dependencies as text,
/// highlighting cycle participants in red.
fn render_cycle_fallback(
    run: &OrchestrationRun,
    cycle_wp_ids: &[String],
    frame: &mut Frame,
    area: Rect,
) {
    use ratatui::text::{Line, Span};
    use ratatui::widgets::Block;

    if area.height < 2 {
        return;
    }

    // Warning banner
    let banner_area = Rect {
        x: area.x,
        y: area.y,
        width: area.width,
        height: 1,
    };
    let warning = Paragraph::new(format!(
        " ⚠ Cycle detected in dependencies: {}",
        cycle_wp_ids.join(", ")
    ))
    .style(Style::default().fg(Color::Black).bg(Color::Yellow));
    frame.render_widget(warning, banner_area);

    // Text listing of WPs and their dependencies
    let list_area = Rect {
        x: area.x,
        y: area.y + 1,
        width: area.width,
        height: area.height - 1,
    };

    let mut lines: Vec<Line> = Vec::new();
    lines.push(Line::from(""));

    for wp in &run.work_packages {
        let is_in_cycle = cycle_wp_ids.contains(&wp.id);
        let state_style = state_to_style(wp.state);
        let id_style = if is_in_cycle {
            Style::default().fg(Color::Red)
        } else {
            state_style
        };

        let deps_str = if wp.dependencies.is_empty() {
            "(none)".to_string()
        } else {
            wp.dependencies.join(", ")
        };

        lines.push(Line::from(vec![
            Span::styled(format!("  {:>6}", wp.id), id_style),
            Span::raw("  "),
            Span::styled(format!("{:?}", wp.state), state_style),
            Span::raw("  deps: "),
            Span::raw(deps_str),
            if is_in_cycle {
                Span::styled(" ⚠ CYCLE", Style::default().fg(Color::Red))
            } else {
                Span::raw("")
            },
        ]));
    }

    let block = Block::default()
        .borders(ratatui::widgets::Borders::ALL)
        .title(" Dependency Graph (cycle detected — text view) ");
    let paragraph = Paragraph::new(lines).block(block);
    frame.render_widget(paragraph, list_area);
}

/// Truncate a title string to a maximum length, adding "…" if truncated.
///
/// Uses `char_indices` to find a safe UTF-8 boundary rather than slicing on
/// raw byte offsets (which would panic on multi-byte characters).
fn truncate_title(title: &str, max_len: usize) -> String {
    if title.chars().count() <= max_len {
        title.to_string()
    } else {
        let end = title
            .char_indices()
            .nth(max_len.saturating_sub(1))
            .map_or(title.len(), |(i, _)| i);
        format!("{}…", &title[..end])
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;
    use crate::types::{OrchestrationRun, ProgressionMode, RunState, Wave, WaveState, WorkPackage};
    use ratatui::backend::TestBackend;
    use ratatui::Terminal;

    fn create_dependency_run() -> OrchestrationRun {
        let work_packages = vec![
            WorkPackage {
                id: "WP01".to_string(),
                title: "Setup".to_string(),
                state: WPState::Completed,
                dependencies: vec![],
                wave: 0,
                pane_id: None,
                pane_name: "wp01".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            },
            WorkPackage {
                id: "WP02".to_string(),
                title: "Build".to_string(),
                state: WPState::Active,
                dependencies: vec!["WP01".to_string()],
                wave: 1,
                pane_id: None,
                pane_name: "wp02".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            },
            WorkPackage {
                id: "WP03".to_string(),
                title: "Test".to_string(),
                state: WPState::Pending,
                dependencies: vec!["WP02".to_string()],
                wave: 2,
                pane_id: None,
                pane_name: "wp03".to_string(),
                worktree_path: None,
                prompt_path: None,
                started_at: None,
                completed_at: None,
                completion_method: None,
                failure_count: 0,
            },
        ];

        OrchestrationRun {
            id: "run-dep".to_string(),
            feature: "feature".to_string(),
            feature_dir: std::path::PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages,
            waves: vec![Wave {
                index: 0,
                wp_ids: vec!["WP01".to_string(), "WP02".to_string(), "WP03".to_string()],
                state: WaveState::Active,
            }],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        }
    }

    #[test]
    fn test_cycle_detection_no_cycles() {
        let run = create_dependency_run();
        let cycles = detect_cycles(&run);
        assert!(cycles.is_empty(), "Expected no cycles in linear chain");
    }

    #[test]
    fn test_cycle_detection_with_cycle() {
        let mut run = create_dependency_run();
        // Create cycle: WP01 depends on WP03 (WP01→WP02→WP03→WP01)
        run.work_packages[0].dependencies.push("WP03".to_string());
        let cycles = detect_cycles(&run);
        assert!(!cycles.is_empty(), "Expected cycle to be detected");
        // All three WPs should be in the cycle
        assert!(
            cycles.contains(&"WP01".to_string()),
            "WP01 should be in cycle"
        );
        assert!(
            cycles.contains(&"WP02".to_string()),
            "WP02 should be in cycle"
        );
        assert!(
            cycles.contains(&"WP03".to_string()),
            "WP03 should be in cycle"
        );
    }

    #[test]
    fn test_render_dependency_graph_does_not_panic() {
        let run = create_dependency_run();
        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| {
                render_dependency_graph(&run, frame, frame.area());
            })
            .expect("render dependency graph");
    }

    #[test]
    fn test_render_dependency_graph_empty_run() {
        let run = OrchestrationRun {
            id: "run-empty".to_string(),
            feature: "feature".to_string(),
            feature_dir: std::path::PathBuf::from("/tmp/feature"),
            config: Config::default(),
            work_packages: vec![],
            waves: vec![],
            state: RunState::Running,
            started_at: None,
            completed_at: None,
            mode: ProgressionMode::Continuous,
        };
        let backend = TestBackend::new(80, 20);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| {
                render_dependency_graph(&run, frame, frame.area());
            })
            .expect("render empty dependency graph");
    }

    #[test]
    fn test_render_dependency_graph_with_cycle_shows_warning() {
        let mut run = create_dependency_run();
        run.work_packages[0].dependencies.push("WP03".to_string());

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| {
                render_dependency_graph(&run, frame, frame.area());
            })
            .expect("render dependency graph with cycles");

        // Verify warning text appears in output
        let buf = terminal.backend().buffer().clone();
        let mut rendered = String::new();
        for y in 0..buf.area.height {
            for x in 0..buf.area.width {
                rendered.push_str(buf[(x, y)].symbol());
            }
        }
        assert!(
            rendered.contains("Cycle detected"),
            "Warning banner should appear for cyclic graphs"
        );
    }

    #[test]
    fn test_render_dependency_graph_no_deps() {
        // All WPs independent — no connections
        let mut run = create_dependency_run();
        for wp in &mut run.work_packages {
            wp.dependencies.clear();
        }

        let backend = TestBackend::new(120, 40);
        let mut terminal = Terminal::new(backend).expect("create terminal");
        terminal
            .draw(|frame| {
                render_dependency_graph(&run, frame, frame.area());
            })
            .expect("render independent WPs graph");
    }

    #[test]
    fn test_state_to_style_mapping() {
        // Verify each state maps to the expected color
        assert_eq!(state_to_style(WPState::Pending).fg, Some(Color::DarkGray));
        assert_eq!(state_to_style(WPState::Active).fg, Some(Color::Yellow));
        assert_eq!(state_to_style(WPState::Completed).fg, Some(Color::Green));
        assert_eq!(state_to_style(WPState::Failed).fg, Some(Color::Red));
        assert_eq!(state_to_style(WPState::ForReview).fg, Some(Color::Magenta));
        assert_eq!(state_to_style(WPState::Paused).fg, Some(Color::Blue));
    }

    #[test]
    fn test_truncate_title() {
        assert_eq!(truncate_title("short", 10), "short");
        assert_eq!(truncate_title("a very long title here", 10), "a very lo…");
        assert_eq!(truncate_title("exact len!", 10), "exact len!");
    }
}
