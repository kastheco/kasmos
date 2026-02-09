---
work_package_id: WP02
title: Spec Parser & Dependency Graph
lane: "done"
dependencies: []
base_branch: master
base_commit: c30ed233eb04a3ec899bb12249aac94e9a83b77e
created_at: '2026-02-09T02:23:17.200174+00:00'
subtasks: [T006, T007, T008, T009, T010, T011]
phase: Phase 1 - Foundation
assignee: ''
agent: "opencode"
shell_pid: "3190789"
review_status: "approved"
reviewed_by: "kas"
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP02 – Spec Parser & Dependency Graph

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

*(Empty — no review feedback yet)*

## Dependency Rebase Guidance

This is a root work package with no dependencies. Branch from `master` directly.

**Implementation command**:
```bash
spec-kitty implement WP02
```

## Objectives & Success Criteria

**Objective**: Parse kitty-specs feature directories to extract work package metadata from YAML frontmatter, build a dependency directed acyclic graph (DAG), perform topological sorting, detect cycles, and compute wave groupings for execution ordering.

**Success Criteria**:
1. Can scan a feature's `tasks/` directory and find all WP markdown files
2. Correctly parses YAML frontmatter including dependencies array
3. Builds adjacency list DAG from dependency declarations
4. Topological sort produces valid execution order
5. Circular dependencies are detected and rejected with the cycle path named
6. Wave groups match topological depth layers
7. Unit tests pass for all scenarios including edge cases (empty deps, single WP, max graph)
8. `cargo build` succeeds with no warnings

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **Dependencies to add**: (uses serde, serde_yaml already from WP01)
- **Feature directory structure**: `kitty-specs/{feature-slug}/tasks/WPxx-slug.md`
- **Frontmatter format**: YAML between `---` delimiters at file start
- **Reference**: [spec.md](../spec.md) FR-001, FR-011; [plan.md](../plan.md) WP02 section
- **Constraint**: Parser must handle missing optional fields gracefully (only `work_package_id` and `dependencies` are required for DAG building)
- **Constraint**: Must work with spec-kitty's actual frontmatter format (see this file's own frontmatter as reference)

## Subtasks & Detailed Guidance

### Subtask T006 – Parse kitty-specs Feature Directory Structure [P]

**Purpose**: Scan a feature directory to discover all work package markdown files and return their paths.

**Steps**:

1. Create `crates/kasmos/src/parser.rs`:
   ```rust
   use std::path::{Path, PathBuf};

   pub struct FeatureDir {
       pub path: PathBuf,
       pub spec_path: PathBuf,
       pub plan_path: PathBuf,
       pub tasks_dir: PathBuf,
       pub wp_files: Vec<PathBuf>,
   }

   impl FeatureDir {
       /// Scan the feature directory and discover all WP files.
       /// WP files match pattern: tasks/WPxx-*.md
       pub fn scan(feature_path: &Path) -> Result<Self, SpecParserError> {
           // 1. Verify feature_path exists and is a directory
           // 2. Check for spec.md and plan.md (warn if missing)
           // 3. Scan tasks/ for WPxx-*.md files
           // 4. Sort by WP number for deterministic ordering
           ...
       }
   }
   ```

2. Use `std::fs::read_dir` to enumerate files, filter by pattern `WP\d+-.*\.md`
3. Sort files by WP number (parse numeric prefix) for deterministic ordering
4. Return `SpecParserError::FeatureDirNotFound` if directory doesn't exist

**Files**:
- `crates/kasmos/src/parser.rs` (new, ~80 lines for this subtask)

**Parallel**: Yes — independent of T007-T011.

### Subtask T007 – Extract YAML Frontmatter from WP Markdown Files [P]

**Purpose**: Parse the YAML frontmatter from WP markdown files to extract metadata including work_package_id, title, dependencies, and lane.

**Steps**:

1. Add to `crates/kasmos/src/parser.rs`:
   ```rust
   #[derive(Debug, Clone, Serialize, Deserialize)]
   pub struct WPFrontmatter {
       pub work_package_id: String,
       pub title: String,
       #[serde(default)]
       pub dependencies: Vec<String>,
       #[serde(default = "default_lane")]
       pub lane: String,
       #[serde(default)]
       pub subtasks: Vec<String>,
       #[serde(default)]
       pub phase: String,
   }

   fn default_lane() -> String { "planned".to_string() }

   /// Parse YAML frontmatter from a markdown file.
   /// Frontmatter is delimited by --- at start and end.
   pub fn parse_frontmatter(path: &Path) -> Result<WPFrontmatter, SpecParserError> {
       let content = std::fs::read_to_string(path)
           .map_err(|e| SpecParserError::InvalidFrontmatter {
               file: path.display().to_string(),
               reason: e.to_string(),
           })?;

       // Split on "---" delimiters
       let parts: Vec<&str> = content.splitn(3, "---").collect();
       if parts.len() < 3 {
           return Err(SpecParserError::InvalidFrontmatter {
               file: path.display().to_string(),
               reason: "No YAML frontmatter found (missing --- delimiters)".into(),
           });
       }

       let yaml_str = parts[1].trim();
       serde_yaml::from_str(yaml_str).map_err(|e| SpecParserError::InvalidFrontmatter {
           file: path.display().to_string(),
           reason: e.to_string(),
       })
   }
   ```

2. Handle edge cases: empty frontmatter, missing optional fields, extra YAML fields (serde `deny_unknown_fields` OFF)

**Files**:
- `crates/kasmos/src/parser.rs` (continued, ~60 lines for this subtask)

**Parallel**: Yes — independent of T006, T008-T011 (but uses same file).

### Subtask T008 – Build Dependency DAG

**Purpose**: Construct a directed acyclic graph from parsed WP frontmatter where edges represent "depends on" relationships.

**Steps**:

1. Add to `crates/kasmos/src/graph.rs` (new file):
   ```rust
   use std::collections::HashMap;

   pub struct DependencyGraph {
       /// Adjacency list: WP ID → list of WP IDs it depends on
       pub dependencies: HashMap<String, Vec<String>>,
       /// Reverse adjacency: WP ID → list of WP IDs that depend on it
       pub dependents: HashMap<String, Vec<String>>,
       /// All known WP IDs
       pub nodes: Vec<String>,
   }

   impl DependencyGraph {
       pub fn build(frontmatters: &[WPFrontmatter]) -> Result<Self, SpecParserError> {
           // 1. Collect all WP IDs into nodes set
           // 2. For each WP, validate that all dependencies exist in nodes set
           //    → Return SpecParserError::UnknownDependency if not
           // 3. Build forward (dependencies) and reverse (dependents) adjacency lists
           ...
       }

       /// Check if all dependencies for a given WP are in the completed set
       pub fn deps_satisfied(&self, wp_id: &str, completed: &HashSet<String>) -> bool { ... }
   }
   ```

2. Validate all referenced dependencies exist before building the graph
3. Both forward and reverse adjacency lists are needed — forward for topological sort, reverse for wave engine

**Files**:
- `crates/kasmos/src/graph.rs` (new, ~80 lines)

**Parallel**: No — depends on WPFrontmatter from T007.

### Subtask T009 – Topological Sort via Kahn's Algorithm

**Purpose**: Produce a valid execution order for work packages using Kahn's algorithm, which also naturally detects cycles.

**Steps**:

1. Add to `crates/kasmos/src/graph.rs`:
   ```rust
   impl DependencyGraph {
       /// Perform topological sort using Kahn's algorithm.
       /// Returns ordered list of WP IDs, or error if cycle detected.
       pub fn topological_sort(&self) -> Result<Vec<String>, SpecParserError> {
           // 1. Compute in-degree for each node
           let mut in_degree: HashMap<String, usize> = ...;

           // 2. Initialize queue with zero in-degree nodes
           let mut queue: VecDeque<String> = in_degree.iter()
               .filter(|(_, &deg)| deg == 0)
               .map(|(id, _)| id.clone())
               .collect();

           // 3. Process queue: for each node, decrement in-degree of dependents
           let mut sorted = Vec::new();
           while let Some(node) = queue.pop_front() {
               sorted.push(node.clone());
               for dependent in self.dependents.get(&node).unwrap_or(&vec![]) {
                   let deg = in_degree.get_mut(dependent).unwrap();
                   *deg -= 1;
                   if *deg == 0 {
                       queue.push_back(dependent.clone());
                   }
               }
           }

           // 4. If sorted.len() != nodes.len(), there's a cycle
           if sorted.len() != self.nodes.len() {
               // Find the cycle for error reporting (T010)
               let remaining: Vec<_> = self.nodes.iter()
                   .filter(|n| !sorted.contains(n))
                   .collect();
               return Err(SpecParserError::CircularDependency {
                   cycle: format!("Cycle involving: {}", remaining.join(", ")),
               });
           }

           Ok(sorted)
       }
   }
   ```

**Files**:
- `crates/kasmos/src/graph.rs` (continued, ~50 lines)

**Parallel**: No — depends on DAG from T008.

### Subtask T010 – Cycle Detection with Actionable Error Messages

**Purpose**: When a cycle is detected, provide the exact cycle path so the user can fix it, not just "cycle detected."

**Steps**:

1. Add cycle path finder to `crates/kasmos/src/graph.rs`:
   ```rust
   impl DependencyGraph {
       /// Find the actual cycle path for error reporting.
       /// Uses DFS with coloring (white=unvisited, gray=in-stack, black=done).
       fn find_cycle_path(&self, remaining: &[&String]) -> String {
           // DFS from each remaining node
           // Track path; when we hit a gray node, we found the cycle
           // Return: "WP03 → WP05 → WP07 → WP03"
           ...
       }
   }
   ```

2. The error message should be immediately actionable: "Circular dependency detected: WP03 → WP05 → WP07 → WP03. Remove one of these dependency edges to break the cycle."

**Files**:
- `crates/kasmos/src/graph.rs` (continued, ~40 lines)

**Parallel**: No — extends T009.

### Subtask T011 – Compute Wave Groups from Topological Layers

**Purpose**: Group work packages into waves based on their depth in the dependency graph. WPs at the same depth can execute in parallel.

**Steps**:

1. Add to `crates/kasmos/src/graph.rs`:
   ```rust
   impl DependencyGraph {
       /// Compute wave assignments: WPs at the same topological depth
       /// form a wave and can execute in parallel.
       pub fn compute_waves(&self) -> Result<Vec<Vec<String>>, SpecParserError> {
           // 1. For each WP, compute depth = max(depth of dependencies) + 1
           //    Root nodes (no deps) have depth 0
           let mut depths: HashMap<String, usize> = HashMap::new();

           // Process in topological order (guaranteed to have deps computed first)
           let sorted = self.topological_sort()?;
           for wp_id in &sorted {
               let max_dep_depth = self.dependencies.get(wp_id)
                   .map(|deps| deps.iter()
                       .map(|d| depths.get(d).copied().unwrap_or(0))
                       .max()
                       .unwrap_or(0))
                   .unwrap_or(0);

               let depth = if self.dependencies.get(wp_id).map_or(true, |d| d.is_empty()) {
                   0
               } else {
                   max_dep_depth + 1
               };
               depths.insert(wp_id.clone(), depth);
           }

           // 2. Group by depth → waves
           let max_depth = depths.values().max().copied().unwrap_or(0);
           let mut waves: Vec<Vec<String>> = vec![vec![]; max_depth + 1];
           for (wp_id, depth) in &depths {
               waves[*depth].push(wp_id.clone());
           }

           // 3. Sort WP IDs within each wave for deterministic output
           for wave in &mut waves {
               wave.sort();
           }

           Ok(waves)
       }
   }
   ```

2. Create `Wave` structs from the wave groups (connecting to WP01's types)

**Files**:
- `crates/kasmos/src/graph.rs` (continued, ~50 lines)

**Parallel**: No — depends on topological sort from T009.

## Test Strategy

- Unit test: parse sample WP markdown with valid frontmatter → correct WPFrontmatter struct
- Unit test: parse WP with missing optional fields → defaults applied
- Unit test: parse WP with invalid YAML → SpecParserError::InvalidFrontmatter
- Unit test: build DAG from 4 WPs with known deps → correct adjacency lists
- Unit test: topological sort of linear chain (WP01→WP02→WP03) → [WP01, WP02, WP03]
- Unit test: topological sort of diamond (WP01→{WP02,WP03}→WP04) → WP01 first, WP04 last
- Unit test: cycle detection (WP01→WP02→WP01) → error with cycle path
- Unit test: wave computation → correct groupings matching expected depths
- Unit test: empty feature directory → appropriate error

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Frontmatter format varies across spec-kitty versions | Medium | Parse defensively, only require work_package_id + dependencies |
| Unknown dependencies silently ignored | High | Strict validation: reject any dep not in known WP set |
| Large graphs cause performance issues | Low | Kahn's algorithm is O(V+E), unlikely to matter at <50 WPs |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] Feature directory scanning finds all WP files and sorts by number
- [ ] YAML frontmatter parsing handles valid, partial, and invalid inputs
- [ ] DAG correctly represents dependency relationships in both directions
- [ ] Topological sort produces valid ordering for known test cases
- [ ] Cycle detection names the exact cycle path
- [ ] Wave computation matches expected depth-based groupings
- [ ] All unit tests pass
- [ ] No compiler warnings

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP02 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
- 2026-02-09T02:54:40Z – opencode – shell_pid=3190789 – lane=doing – Assigned agent via workflow command
- 2026-02-09T03:05:52Z – opencode – shell_pid=3190789 – lane=for_review – Ready for review: Implemented parser/frontmatter extraction, DAG/toposort/cycle detection, wave computation, tests+clippy clean
- 2026-02-09T03:20:09Z – opencode – shell_pid=3190789 – lane=done – Done: implementation validated, reviewed, and merged-ready
