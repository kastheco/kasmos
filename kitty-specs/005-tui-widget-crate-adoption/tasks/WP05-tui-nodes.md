---
work_package_id: "WP05"
subtasks:
  - "T023"
  - "T024"
  - "T025"
  - "T026"
  - "T027"
  - "T028"
  - "T029"
title: "Adopt tui-nodes — WP Dependency Graph Visualization"
phase: "Phase 2 - Crate Adoptions"
lane: "doing"
assignee: ""
agent: "reviewer"
shell_pid: "1312308"
review_status: ""
reviewed_by: ""
dependencies: ["WP01"]
history:
  - timestamp: "2026-02-12T00:00:00Z"
    lane: "planned"
    agent: "system"
    shell_pid: ""
    action: "Prompt generated via /spec-kitty.tasks"
---

# Work Package Prompt: WP05 – Adopt tui-nodes — WP Dependency Graph Visualization

## ⚠️ IMPORTANT: Review Feedback Status

**Read this first if you are implementing this task!**

- **Has review feedback?**: Check the `review_status` field above. If it says `has_feedback`, scroll to the **Review Feedback** section immediately.
- **You must address all feedback** before your work is complete.
- **Mark as acknowledged**: When you understand the feedback and begin addressing it, update `review_status: acknowledged` in the frontmatter.

---

## Review Feedback

> **Populated by `/spec-kitty.review`** – Reviewers add detailed feedback here when work needs changes.

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP05 --base WP01
```

Depends on WP01 (ratatui-macros). New code should use macro syntax.

---

## Objectives & Success Criteria

1. Add `tui-nodes` 0.10.x as a dependency.
2. Introduce `DashboardViewMode` enum (`Kanban` | `DependencyGraph`) and `view_mode` field on `DashboardState` (AD-4).
3. Create a new `widgets/dependency_graph.rs` module with a graph builder that converts `OrchestrationRun` to a `NodeGraph`.
4. Color-code graph nodes by WP state (FR-012): Pending→DarkGray, Active→Yellow, Completed→Green, Failed→Red, ForReview→Magenta, Paused→Blue.
5. Toggle between kanban and graph views with `v` key (FR-013).
6. Disable lane navigation keys (`j`/`k`/`h`/`l`) when in graph mode.
7. Detect circular dependencies and render a warning banner instead of crashing (edge case).
8. **SC-008**: Correctly render all nodes and edges for a feature with ≥10 WPs.
9. **SC-005/SC-006**: `cargo test` and `cargo clippy` pass with zero regressions.

## Context & Constraints

- **Architecture Decision AD-4** (plan.md): DashboardViewMode toggle, `v` key, lane nav disabled in graph mode.
- **Research R-7** (research.md): tui-nodes API — `NodeGraph`, `NodeLayout`, `Connection`, `LineType`. Low documentation coverage (6.67%).
- **data-model.md**: DashboardViewMode enum, DashboardState gains `view_mode`. Graph derived from OrchestrationRun on each render frame.
- **Spec US5**: All acceptance scenarios for graph visualization.
- **Edge cases**: Circular dependencies (render warning), 20+ WPs (rely on layout algorithm), narrow terminal (scrollable if needed).
- **Important**: tui-nodes has very low documentation. Implementer MUST check the crate's examples and source code on [crates.io](https://crates.io/crates/tui-nodes) or GitHub before starting. The API surface is small (4 types) but usage patterns may differ from R-7 assumptions.

### Files in scope

| File | Changes |
|------|---------|
| `crates/kasmos/Cargo.toml` | Add tui-nodes dependency |
| `crates/kasmos/src/tui/app.rs` | DashboardViewMode enum, view_mode field, render dispatch |
| `crates/kasmos/src/tui/widgets/mod.rs` | New module (graph adapter) |
| `crates/kasmos/src/tui/widgets/dependency_graph.rs` | New file (graph builder + cycle detection) |
| `crates/kasmos/src/tui/keybindings.rs` | `v` key toggle, nav disable in graph mode |
| `crates/kasmos/src/tui/mod.rs` | Register widgets module |

---

## Subtasks & Detailed Guidance

### Subtask T023 – Add `tui-nodes` dependency to Cargo.toml

- **Purpose**: Introduce the node graph widget crate.
- **Steps**:
  1. Add to `crates/kasmos/Cargo.toml` `[dependencies]`:
     ```toml
     tui-nodes = "0.10"
     ```
  2. Run `cargo check -p kasmos` to verify resolution.
- **Files**: `crates/kasmos/Cargo.toml`
- **Notes**: tui-nodes 0.10.x requires ratatui ^0.30 (confirmed in R-1).

### Subtask T024 – Add `DashboardViewMode` enum and `view_mode` field

- **Purpose**: Track which view is active in the Dashboard tab.
- **Steps**:
  1. Add the enum to `crates/kasmos/src/tui/app.rs`:
     ```rust
     /// Which view is active in the Dashboard tab.
     #[derive(Debug, Clone, Copy, PartialEq, Eq, Default)]
     pub enum DashboardViewMode {
         /// Standard kanban lane view (Planned / Doing / Review / Done).
         #[default]
         Kanban,
         /// Directed graph showing WP dependency relationships.
         DependencyGraph,
     }
     ```
  2. Add `view_mode` field to `DashboardState`:
     ```rust
     /// Current Dashboard sub-view mode (Kanban vs DependencyGraph).
     pub view_mode: DashboardViewMode,
     ```
  3. Initialize in `Default` impl:
     ```rust
     view_mode: DashboardViewMode::default(), // Kanban
     ```
- **Files**: `crates/kasmos/src/tui/app.rs`
- **Parallel?**: Yes — different code section from T025 (new module).

### Subtask T025 – Create `widgets/` module with dependency graph builder

- **Purpose**: Encapsulate the tui-nodes integration in a dedicated module.
- **Steps**:
  1. Create `crates/kasmos/src/tui/widgets/mod.rs`:
     ```rust
     //! TUI widget adapters for third-party crate integration.
     
     pub mod dependency_graph;
     ```
  
  2. Create `crates/kasmos/src/tui/widgets/dependency_graph.rs`:
     ```rust
     //! WP dependency graph visualization using tui-nodes.
     //!
     //! Converts an OrchestrationRun's work packages and their dependency
     //! relationships into a tui-nodes NodeGraph for rendering in the Dashboard.
     
     use ratatui::style::{Color, Style};
     use crate::types::{OrchestrationRun, WPState};
     // Import tui-nodes types — verify exact paths from crate source
     // use tui_nodes::{NodeGraph, NodeLayout, Connection};
     ```

  3. Implement `state_to_style()`:
     ```rust
     /// Map a WP state to a visual style for graph nodes.
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
     ```

  4. Implement `build_dependency_graph()`:
     ```rust
     /// Build a tui-nodes graph from an OrchestrationRun.
     ///
     /// Returns the graph and a boolean indicating if cycles were detected.
     pub fn build_dependency_graph(run: &OrchestrationRun) -> (/* NodeGraph type */, bool) {
         let has_cycles = detect_cycles(run);
         
         // Build the graph
         // NOTE: Verify the actual tui-nodes API by checking the crate source.
         // The API shown in R-7 may not be exact. The crate has 4 main types:
         // NodeGraph, NodeLayout, Connection, LineType.
         
         let mut graph = /* NodeGraph::new() or equivalent */;
         
         for wp in &run.work_packages {
             let style = state_to_style(wp.state);
             let label = format!("{}: {}", wp.id, wp.title);
             // Add node with label and style
             // graph.add_node(NodeLayout::new(&wp.id, &label, style));
         }
         
         for wp in &run.work_packages {
             for dep in &wp.dependencies {
                 // Add directed edge from dependency to dependent
                 // graph.add_connection(Connection::new(dep, &wp.id));
             }
         }
         
         (graph, has_cycles)
     }
     ```
     
     **CRITICAL**: The pseudo-code above uses placeholder API calls. You MUST verify the actual tui-nodes 0.10.0 API before implementing. Check:
     - How to create a NodeGraph
     - How to add nodes (what parameters)
     - How to add connections/edges
     - How to set node styles/colors
     - How to render the graph as a Widget

  5. **API Discovery**: Before writing the implementation:
     ```bash
     # Check the crate's public API
     cargo doc -p tui-nodes --open
     # Or check the source
     # The crate is small — 4 main types
     ```

- **Files**: `crates/kasmos/src/tui/widgets/mod.rs` (new), `crates/kasmos/src/tui/widgets/dependency_graph.rs` (new)
- **Parallel?**: Yes — new files, doesn't conflict with app.rs changes.
- **Notes**: tui-nodes has 6.67% documentation coverage. The examples directory in the crate repo is the best reference.

### Subtask T026 – Implement cycle detection for WP dependency graphs

- **Purpose**: Detect circular dependencies before passing to tui-nodes to prevent infinite loops.
- **Steps**:
  1. Add a cycle detection function to `dependency_graph.rs`:
     ```rust
     use std::collections::{HashMap, HashSet};
     
     /// Detect cycles in the WP dependency graph using DFS.
     ///
     /// Returns true if any cycle is found.
     pub fn detect_cycles(run: &OrchestrationRun) -> bool {
         let adj: HashMap<&str, Vec<&str>> = run.work_packages.iter()
             .map(|wp| (wp.id.as_str(), wp.dependencies.iter().map(|d| d.as_str()).collect()))
             .collect();
         
         let mut visited = HashSet::new();
         let mut in_stack = HashSet::new();
         
         for wp in &run.work_packages {
             if !visited.contains(wp.id.as_str()) {
                 if dfs_has_cycle(&wp.id, &adj, &mut visited, &mut in_stack) {
                     return true;
                 }
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
     ```
  2. The `build_dependency_graph()` function calls `detect_cycles()` and returns the result alongside the graph.
- **Files**: `crates/kasmos/src/tui/widgets/dependency_graph.rs`
- **Parallel?**: Yes — standalone utility function.
- **Notes**: Add a unit test for cycle detection with a known cyclic graph (A→B→C→A).

### Subtask T027 – Update `render_dashboard()` for graph view dispatch

- **Purpose**: When `view_mode == DependencyGraph`, render the node graph instead of the kanban columns.
- **Steps**:
  1. In `render_dashboard()`, add a dispatch at the top:
     ```rust
     fn render_dashboard(&self, frame: &mut Frame, area: Rect) {
         match self.dashboard.view_mode {
             DashboardViewMode::Kanban => self.render_dashboard_kanban(frame, area),
             DashboardViewMode::DependencyGraph => self.render_dashboard_graph(frame, area),
         }
     }
     ```
  2. Rename the existing kanban rendering to `render_dashboard_kanban()`.
  3. Add the new graph rendering method:
     ```rust
     fn render_dashboard_graph(&self, frame: &mut Frame, area: Rect) {
         use crate::tui::widgets::dependency_graph;
         
         let (graph, has_cycles) = dependency_graph::build_dependency_graph(&self.run);
         
         // Reserve 1 line at top for view mode indicator + cycle warning
         let [header_area, graph_area] = vertical![==1, *=0].areas(area);
         
         // Header: view mode indicator
         let header_text = if has_cycles {
             line![
                 span!(Style::default().fg(Color::Yellow); "⚠ Dependency Graph"),
                 " ",
                 span!(Style::default().fg(Color::Red); "(circular dependencies detected!)"),
                 "  ",
                 span!(Style::default().fg(Color::DarkGray); "[v] kanban")
             ]
         } else {
             line![
                 span!(Style::default().fg(Color::Yellow); "Dependency Graph"),
                 "  ",
                 span!(Style::default().fg(Color::DarkGray); "[v] kanban")
             ]
         };
         frame.render_widget(Paragraph::new(header_text), header_area);
         
         // Render the graph
         // NOTE: Verify the actual Widget impl on NodeGraph.
         // It may be frame.render_widget(graph, graph_area) or
         // frame.render_stateful_widget(graph, graph_area, &mut state)
         frame.render_widget(graph, graph_area);
     }
     ```
  4. Also add a view mode indicator to the kanban view:
     ```rust
     // At the bottom of render_dashboard_kanban, in the hint area:
     // Add "[v] graph" to the hint text
     ```
- **Files**: `crates/kasmos/src/tui/app.rs`
- **Notes**:
  - The graph is rebuilt on every render frame from the current `self.run`. This is fine — WP count is small (typically <30) and graph construction is O(nodes + edges).
  - If the graph exceeds the viewport, tui-nodes may handle scrolling internally. If not, consider limiting the visible area or adding a scroll offset in a future iteration.

### Subtask T028 – Add `v` key toggle and disable lane nav in graph mode

- **Purpose**: Toggle between kanban and graph views; prevent nav key confusion in graph mode.
- **Steps**:
  1. In `handle_dashboard_key()` in `crates/kasmos/src/tui/keybindings.rs`, add the `v` key:
     ```rust
     KeyCode::Char('v') => {
         app.dashboard.view_mode = match app.dashboard.view_mode {
             DashboardViewMode::Kanban => DashboardViewMode::DependencyGraph,
             DashboardViewMode::DependencyGraph => DashboardViewMode::Kanban,
         };
     }
     ```
  2. Wrap the existing navigation and action keys in a `Kanban` mode guard:
     ```rust
     fn handle_dashboard_key(app: &mut App, key: KeyEvent) {
         // View toggle works in all modes
         if key.code == KeyCode::Char('v') {
             app.dashboard.view_mode = match app.dashboard.view_mode {
                 DashboardViewMode::Kanban => DashboardViewMode::DependencyGraph,
                 DashboardViewMode::DependencyGraph => DashboardViewMode::Kanban,
             };
             return;
         }
         
         // All other dashboard keys only work in Kanban mode
         if app.dashboard.view_mode != DashboardViewMode::Kanban {
             return;
         }
         
         // ... existing j/k/h/l navigation and A/R/P/F/T action keys ...
     }
     ```
  3. Import `DashboardViewMode` in keybindings.rs:
     ```rust
     use super::app::{App, DashboardViewMode, Tab};
     ```
- **Files**: `crates/kasmos/src/tui/keybindings.rs`
- **Parallel?**: Yes — different file from T025/T026/T027.
- **Notes**: The `v` key was chosen per clarification (spec.md line 22). Ensure it doesn't conflict with other Dashboard keys — current Dashboard keys are `j/k/h/l/A/R/P/F/T`, no `v`.

### Subtask T029 – Register `widgets` module in `tui/mod.rs`

- **Purpose**: Make the new widgets module visible to the rest of the TUI crate.
- **Steps**:
  1. In `crates/kasmos/src/tui/mod.rs`, add:
     ```rust
     pub mod widgets;
     ```
  2. Remove or update the comment on line 11: `// tabs/ and widgets/ will be added in later WPs`
- **Files**: `crates/kasmos/src/tui/mod.rs`

---

## Risks & Mitigations

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| tui-nodes API differs from R-7 assumptions | High | Check crate source/examples BEFORE implementing. API is small (4 types). |
| Graph layout unreadable for 20+ WPs | Low | Rely on tui-nodes layout algorithm; add scrolling in future iteration |
| Cycle detection false positives | Very Low | Well-tested DFS algorithm with unit tests |
| NodeGraph Widget impl differs (StatefulWidget vs Widget) | Medium | Check crate source for the Widget impl |
| Performance: graph rebuild per frame | Negligible | O(nodes + edges), WP count small |

## Review Guidance

- **Graph rendering**: Load a feature with ≥10 WPs and known dependencies. Verify all nodes and edges appear correctly (SC-008).
- **State coloring**: Transition WPs through states — node colors should update immediately.
- **View toggle**: Press `v` in Dashboard — should switch between kanban and graph. Press `v` again to return.
- **Nav disabled**: In graph mode, `j`/`k`/`h`/`l` should do nothing. `v` should still work.
- **Cycle warning**: If a feature has circular WP dependencies, a warning banner should appear.
- **Parallel WPs**: WPs with no dependencies should appear as disconnected nodes.
- **Linear chain**: WPs with linear dependencies (WP01→WP02→WP03) should show a clear directed edge chain.
- **Tests pass**: `cargo test -p kasmos` — existing tests plus cycle detection tests.

## Activity Log

- 2026-02-12T00:00:00Z – system – lane=planned – Prompt created.
- 2026-02-12T12:32:06Z – coder – lane=doing – Implementation complete, moving to doing
- 2026-02-12T12:32:27Z – coder – lane=for_review – Submitted for review via swarm
- 2026-02-12T12:32:27Z – reviewer – shell_pid=1312308 – lane=doing – Started review via workflow command
