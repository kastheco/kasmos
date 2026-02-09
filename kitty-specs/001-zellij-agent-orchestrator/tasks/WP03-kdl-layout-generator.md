---
work_package_id: WP03
title: KDL Layout Generator
lane: "for_review"
dependencies:
- WP01
base_branch: 001-zellij-agent-orchestrator-WP01
base_commit: f3b76ab4fe8fdea32c911fa12382895e6ce13748
created_at: '2026-02-09T03:22:41.605579+00:00'
subtasks: [T012, T013, T014, T015, T016, T017, T018, T019]
phase: Phase 2 - Generation
assignee: ''
agent: "opencode"
shell_pid: "3018616"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP03 – KDL Layout Generator

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This WP depends on **WP01** (core types) and **WP02** (spec parser). Ensure both are merged before starting.

**Implementation command**:
```bash
spec-kitty implement WP03 --base WP02
```

(WP02 already includes WP01 since they're both Wave 1 and WP02's branch will be based on the same master as WP01.)

## Objectives & Success Criteria

**Objective**: Generate valid Zellij KDL layout files dynamically based on the number and arrangement of active work packages, with a 3-column structure (controller left, agent grid right), embedded pane commands for OpenCode launch, and per-pane naming for ID discovery.

**Success Criteria**:
1. Generated KDL is syntactically valid (parses with kdl crate)
2. Layout has controller pane at 40% width on the left
3. Agent grid occupies 60% width with adaptive row/column layout
4. Each agent pane has: name attribute, command node with stdin pipe, cwd to worktree
5. Grid adapts correctly for 1, 2, 4, 8 agent panes
6. Generated file writes to .kasmos/layout.kdl
7. Unit tests verify layout structure for various pane counts

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Dependencies to add**: `kdl` (v5 — use latest, API: KdlDocument, KdlNode, KdlEntry)
- **Zellij layout format**: KDL-based, documented at zellij.dev/documentation/layouts
- **Zellij version**: 0.41+ (supports `name` attribute on panes, `command` nodes)
- **Reference**: [plan.md](../plan.md) WP03 section, architecture decisions
- **Constraint**: KDL v5 builds documents programmatically (not string templates)
- **Constraint**: Controller pane runs `opencode` (the operator's agent session)
- **Constraint**: Agent panes run shell wrapper scripts that pipe prompts into OpenCode

**Zellij KDL layout structure example** (for reference):
```kdl
layout {
    pane split_direction="vertical" {
        pane size="40%" name="controller" {
            command "opencode"
        }
        pane size="60%" split_direction="horizontal" {
            pane split_direction="vertical" {
                pane name="WP01" {
                    command "bash"
                    args "-c" "cat /path/to/prompt.md | opencode -p 'context:'"
                    cwd "/path/to/worktree"
                }
                pane name="WP02" {
                    command "bash"
                    args "-c" "cat /path/to/prompt.md | opencode -p 'context:'"
                    cwd "/path/to/worktree"
                }
            }
            pane split_direction="vertical" {
                pane name="WP03" {
                    command "bash"
                    args "-c" "cat /path/to/prompt.md | opencode -p 'context:'"
                    cwd "/path/to/worktree"
                }
            }
        }
    }
}
```

## Subtasks & Detailed Guidance

### Subtask T012 – KDL Layout Template Engine

**Purpose**: Create the layout generation module using the kdl v5 crate to build KDL documents programmatically.

**Steps**:

1. Create `crates/kasmos/src/layout.rs`:
   ```rust
   use kdl::{KdlDocument, KdlNode, KdlEntry, KdlValue};

   pub struct LayoutGenerator {
       controller_width: u32,  // Percentage (default 40)
   }

   impl LayoutGenerator {
       pub fn new(config: &Config) -> Self { ... }

       /// Generate a complete Zellij KDL layout for the given work packages.
       pub fn generate(&self, wps: &[&WorkPackage], feature_dir: &Path) -> Result<KdlDocument> {
           let mut doc = KdlDocument::new();
           let mut layout_node = KdlNode::new("layout");

           // Build the layout tree
           let root_pane = self.build_root_split(wps, feature_dir)?;
           layout_node.children_mut().insert(root_pane);

           doc.nodes_mut().push(layout_node);
           Ok(doc)
       }
   }
   ```

2. Study the kdl v5 crate API carefully — it uses `KdlNode`, `KdlEntry` (arguments + properties), and `KdlDocument`
3. Each pane attribute (name, size, split_direction) is a `KdlEntry` property on the pane node

**Files**:
- `crates/kasmos/src/layout.rs` (new, ~40 lines initial structure)

### Subtask T013 – 3-Column Layout Structure

**Purpose**: Build the top-level split with controller on the left and agent container on the right.

**Steps**:

1. Add to `crates/kasmos/src/layout.rs`:
   ```rust
   impl LayoutGenerator {
       fn build_root_split(&self, wps: &[&WorkPackage], feature_dir: &Path) -> Result<KdlNode> {
           // Root: vertical split
           let mut root = KdlNode::new("pane");
           root.entries_mut().push(KdlEntry::new_prop("split_direction", "vertical"));

           // Left: controller pane (40%)
           let controller = self.build_controller_pane();
           root.children_mut().insert(controller);

           // Right: agent grid (60%)
           let agent_width = 100 - self.controller_width;
           let grid = self.build_agent_grid(wps, agent_width, feature_dir)?;
           root.children_mut().insert(grid);

           Ok(root)
       }

       fn build_controller_pane(&self) -> KdlNode {
           let mut pane = KdlNode::new("pane");
           pane.entries_mut().push(KdlEntry::new_prop("size", format!("{}%", self.controller_width)));
           pane.entries_mut().push(KdlEntry::new_prop("name", "controller"));

           // Controller runs opencode directly (operator's session)
           let mut cmd = KdlNode::new("command");
           cmd.entries_mut().push(KdlEntry::new("opencode"));
           pane.children_mut().insert(cmd);

           pane
       }
   }
   ```

**Files**:
- `crates/kasmos/src/layout.rs` (continued, ~40 lines)

### Subtask T014 – Adaptive Grid Sizing

**Purpose**: Calculate optimal row/column arrangement for N agent panes to fill the available space.

**Steps**:

1. Add grid calculation:
   ```rust
   impl LayoutGenerator {
       /// Calculate grid dimensions for n panes.
       /// cols = ceil(sqrt(n)), rows = ceil(n / cols)
       fn grid_dimensions(n: usize) -> (usize, usize) {
           if n == 0 { return (0, 0); }
           if n == 1 { return (1, 1); }
           let cols = (n as f64).sqrt().ceil() as usize;
           let rows = (n as f64 / cols as f64).ceil() as usize;
           (rows, cols)
       }

       fn build_agent_grid(&self, wps: &[&WorkPackage], width_pct: u32, feature_dir: &Path) -> Result<KdlNode> {
           let mut grid = KdlNode::new("pane");
           grid.entries_mut().push(KdlEntry::new_prop("size", format!("{}%", width_pct)));
           grid.entries_mut().push(KdlEntry::new_prop("split_direction", "horizontal"));

           let (rows, cols) = Self::grid_dimensions(wps.len());

           for row_idx in 0..rows {
               let mut row_node = KdlNode::new("pane");
               row_node.entries_mut().push(KdlEntry::new_prop("split_direction", "vertical"));

               let start = row_idx * cols;
               let end = (start + cols).min(wps.len());

               for wp in &wps[start..end] {
                   let pane = self.build_agent_pane(wp, feature_dir)?;
                   row_node.children_mut().insert(pane);
               }

               grid.children_mut().insert(row_node);
           }

           Ok(grid)
       }
   }
   ```

2. Test grid_dimensions: 1→(1,1), 2→(1,2), 3→(2,2), 4→(2,2), 5→(2,3), 8→(3,3), 9→(3,3)

**Files**:
- `crates/kasmos/src/layout.rs` (continued, ~40 lines)

### Subtask T015 – Embed Pane Commands with stdin Pipe Pattern

**Purpose**: Each agent pane must run a shell command that pipes the prompt file into OpenCode via stdin.

**Steps**:

1. Add agent pane builder:
   ```rust
   impl LayoutGenerator {
       fn build_agent_pane(&self, wp: &WorkPackage, feature_dir: &Path) -> Result<KdlNode> {
           let mut pane = KdlNode::new("pane");

           // Name attribute for pane discovery
           pane.entries_mut().push(KdlEntry::new_prop("name", wp.pane_name.clone()));

           // Command: bash -c "cat prompt.md | opencode -p 'context:'"
           let prompt_path = feature_dir.join(".kasmos/prompts").join(format!("{}.md", wp.id));
           let cmd_str = format!(
               "cat {} | opencode -p 'context:'",
               prompt_path.display()
           );

           let mut cmd = KdlNode::new("command");
           cmd.entries_mut().push(KdlEntry::new("bash"));
           pane.children_mut().insert(cmd);

           let mut args = KdlNode::new("args");
           args.entries_mut().push(KdlEntry::new("-c"));
           args.entries_mut().push(KdlEntry::new(cmd_str));
           pane.children_mut().insert(args);

           Ok(pane)
       }
   }
   ```

**Files**:
- `crates/kasmos/src/layout.rs` (continued, ~30 lines)

### Subtask T016 – Per-Pane cwd to WP Worktree Path

**Purpose**: Each agent pane's working directory must be set to the WP's git worktree so file operations are isolated.

**Steps**:

1. Add `cwd` node to agent pane:
   ```rust
   // Inside build_agent_pane, after command/args:
   if let Some(worktree_path) = &wp.worktree_path {
       let mut cwd = KdlNode::new("cwd");
       cwd.entries_mut().push(KdlEntry::new(worktree_path.display().to_string()));
       pane.children_mut().insert(cwd);
   }
   ```

2. Worktree path will be populated by the session manager (WP05) before layout generation

**Files**:
- `crates/kasmos/src/layout.rs` (continued, ~10 lines)

### Subtask T017 – KDL Name Attribute per Pane for ID Discovery

**Purpose**: Each pane needs a unique `name` attribute so `zellij action list-panes` can map names to pane IDs.

**Steps**:

1. Already handled in T015 — the `name` property is set from `wp.pane_name`
2. Ensure pane_name defaults to wp.id (e.g., "WP01") if not explicitly set
3. Add controller pane name ("controller") — already done in T013
4. Document the naming convention: agent panes use WP ID, controller uses "controller"

**Files**:
- No additional files — this is covered by T013 and T015.

**Notes**: This subtask is primarily about ensuring the naming convention is consistent and documented. The actual implementation is spread across T013 (controller) and T015 (agent panes).

### Subtask T018 – Write KDL to .kasmos/layout.kdl

**Purpose**: Write the generated KDL document to the `.kasmos/layout.kdl` file, creating the directory if needed.

**Steps**:

1. Add write function:
   ```rust
   impl LayoutGenerator {
       /// Write the generated layout to disk.
       pub fn write_layout(&self, doc: &KdlDocument, kasmos_dir: &Path) -> Result<PathBuf> {
           // 1. Create .kasmos/ directory if it doesn't exist
           std::fs::create_dir_all(kasmos_dir)?;

           // 2. Serialize KDL document to string
           let kdl_string = doc.to_string();

           // 3. Write to .kasmos/layout.kdl
           let layout_path = kasmos_dir.join("layout.kdl");
           std::fs::write(&layout_path, &kdl_string)?;

           tracing::info!(path = %layout_path.display(), "KDL layout written");
           Ok(layout_path)
       }
   }
   ```

**Files**:
- `crates/kasmos/src/layout.rs` (continued, ~20 lines)

### Subtask T019 – Validate Generated KDL Syntax [P]

**Purpose**: Parse the generated KDL string back through the kdl crate to verify it produces valid syntax.

**Steps**:

1. Add validation function:
   ```rust
   impl LayoutGenerator {
       /// Validate KDL by parsing the serialized output.
       pub fn validate_kdl(kdl_string: &str) -> Result<()> {
           kdl_string.parse::<KdlDocument>()
               .map_err(|e| anyhow::anyhow!("Generated KDL is invalid: {}", e))?;
           Ok(())
       }
   }
   ```

2. Call this after `doc.to_string()` in the write path as a safety check

**Files**:
- `crates/kasmos/src/layout.rs` (continued, ~15 lines)

**Parallel**: Yes — validation logic is independent (just needs a KDL string input).

## Test Strategy

- Unit test: `grid_dimensions(1)` through `grid_dimensions(8)` produce correct (rows, cols)
- Unit test: generate layout for 1 agent pane → valid KDL with controller + 1 agent
- Unit test: generate layout for 4 agent panes → 2x2 grid
- Unit test: generate layout for 8 agent panes → 3x3 grid (last row has 2 panes)
- Unit test: validate round-trip (generate → serialize → parse → matches expected structure)
- Unit test: each pane has `name`, `command`, `args`, `cwd` nodes
- Unit test: controller pane has 40% size and name="controller"

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| KDL v5 API differs from expected | High | Read crate docs carefully, write T019 validation early |
| Zellij rejects generated layout | High | Test with actual Zellij if possible, fallback to string template |
| Grid math produces bad layouts for edge cases | Medium | Extensive unit tests for grid_dimensions (1-16 panes) |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] Generated KDL parses successfully with kdl crate
- [ ] Controller pane at 40% width with correct name
- [ ] Agent grid adapts for 1, 2, 4, 8 panes
- [ ] Each pane has name, command (bash), args (cat | opencode), cwd
- [ ] .kasmos/layout.kdl file is written correctly
- [ ] Round-trip validation passes
- [ ] Unit tests cover all grid sizes

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP03 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
- 2026-02-09T03:22:41Z – opencode – shell_pid=3018616 – lane=doing – Assigned agent via workflow command
- 2026-02-09T03:42:41Z – opencode – shell_pid=3018616 – lane=for_review – Ready for review: KDL layout generator with LayoutGenerator struct, adaptive grid, shell-escaped commands, round-trip validation, 55 passing tests
