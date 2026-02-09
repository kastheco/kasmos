---
work_package_id: WP04
title: Prompt File Generator
lane: "planned"
dependencies:
- WP01
base_branch: 001-zellij-agent-orchestrator-WP01
base_commit: f3b76ab4fe8fdea32c911fa12382895e6ce13748
created_at: '2026-02-09T03:25:02.950205+00:00'
subtasks: [T020, T021, T022, T023, T024, T025]
phase: Phase 2 - Generation
assignee: ''
agent: "opencode"
shell_pid: "3190789"
review_status: "has_feedback"
reviewed_by: "kas"
history:
- timestamp: '2026-02-09T00:00:00Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP04 – Prompt File Generator

## IMPORTANT: Review Feedback Status

Before starting implementation, check the **Review Feedback** section below.
- If empty → This is fresh work. Proceed with implementation.
- If populated → This WP was previously reviewed and needs changes. Address ALL feedback items before marking as done.

## Review Feedback

**Reviewed by**: kas
**Status**: ❌ Changes Requested
**Date**: 2026-02-09

## Review Feedback - WP04

### 🟠 Major Issue: Shell Script Path Injection Vulnerability

**File:** `crates/kasmos/src/prompt.rs:251-253`

**Problem:** The generated shell script does not quote the file path, making it vulnerable to paths containing spaces or special characters.

**Current code:**
```rust
let script_content = format!(
    "#!/bin/bash\nset -euo pipefail\ncat {} | opencode -p 'context:'\n",
    prompt_path.display()
);
```

**Required fix:**
```rust
let script_content = format!(
    "#!/bin/bash\nset -euo pipefail\ncat '{}' | opencode -p 'context:'\n",
    prompt_path.display()
);
```

**Why this matters:**
- If a WP ID or path contains spaces, the script will fail
- Example: `cat .kasmos/prompts/WP 01.md` is interpreted as 3 arguments by bash
- With quotes: `cat '.kasmos/prompts/WP 01.md'` is correctly interpreted as 1 path

**Test to add:**
```rust
#[test]
fn test_shell_wrapper_script_quotes_path() {
    let script_content = format!(
        "#!/bin/bash\nset -euo pipefail\ncat '{}' | opencode -p 'context:'\n",
        "/path/with spaces/WP01.md"
    );
    assert!(script_content.contains("cat '/path/with spaces/WP01.md'"));
}
```

### Minor Notes (Optional Improvements)

1. **Placeholder implementation** (lines 192-213): The `build_prompt_context` function uses hardcoded placeholder values for subtasks, scope, and constraints. This is acceptable for WP04 scope if actual parsing is deferred to WP02 integration.

2. **Hardcoded feature name** (line 210): Consider passing feature name as a parameter instead of hardcoding `"001-zellij-agent-orchestrator"`.

### Approval Contingent On

Please fix the path quoting issue and verify the generated script works with paths containing spaces. Once fixed, this WP is ready to merge.


## Dependency Rebase Guidance

This WP depends on **WP01** (core types) and **WP02** (spec parser). Ensure both are merged before starting.

**Implementation command**:
```bash
spec-kitty implement WP04 --base WP02
```

## Objectives & Success Criteria

**Objective**: Generate work-package-specific prompt files containing the WP description, scope, dependency context from upstream WPs, and project-level agent instructions (AGENTS.md). Also generate shell wrapper scripts that pipe prompts into OpenCode via stdin.

**Success Criteria**:
1. Each WP gets a unique prompt file at `.kasmos/prompts/WPxx.md`
2. Prompts include WP title, description, subtask list, and scope boundaries
3. Dependency context from upstream WPs is included
4. AGENTS.md content is embedded in each prompt
5. Shell wrapper scripts are executable and correctly pipe prompt→OpenCode
6. Missing AGENTS.md produces a warning, not an error
7. OpenCode binary validation produces clear error if not found

## Context & Constraints

- **Crate location**: `crates/kasmos/`
- **OpenCode prompt injection**: `cat prompt.md | opencode -p "context:"` (confirmed via PR #1230)
- **Prompt storage**: `.kasmos/prompts/` directory
- **Wrapper storage**: `.kasmos/scripts/` directory
- **Reference**: [plan.md](../plan.md) WP04 section; [AGENTS.md](../../AGENTS.md)
- **Constraint**: Prompts should be under 10K characters to avoid OpenCode stdin buffer issues
- **Constraint**: Shell wrappers must be POSIX-compatible bash scripts

## Subtasks & Detailed Guidance

### Subtask T020 – Prompt Template Struct [P]

**Purpose**: Define a Rust struct that represents the prompt template and can render to a markdown string.

**Steps**:

1. Create `crates/kasmos/src/prompt.rs`:
   ```rust
   pub struct PromptContext {
       pub wp_id: String,
       pub wp_title: String,
       pub wp_description: String,    // From spec/plan or WP frontmatter
       pub subtasks: Vec<SubtaskInfo>,
       pub scope_boundaries: String,  // What's in/out of scope
       pub constraints: Vec<String>,
       pub dependency_context: Vec<DependencyContext>,
       pub agents_md: Option<String>, // AGENTS.md content if available
       pub feature_name: String,
       pub feature_dir: PathBuf,
   }

   pub struct SubtaskInfo {
       pub id: String,
       pub description: String,
       pub parallel: bool,
   }

   pub struct DependencyContext {
       pub wp_id: String,
       pub wp_title: String,
       pub summary: String,           // Brief summary of what this dep provides
       pub key_outputs: Vec<String>,  // Files/modules this dep creates
   }

   impl PromptContext {
       /// Render the prompt to a markdown string.
       pub fn render(&self) -> String {
           let mut out = String::new();
           // Header
           out.push_str(&format!("# Agent Prompt: {} – {}\n\n", self.wp_id, self.wp_title));

           // Objective
           out.push_str("## Objective\n\n");
           out.push_str(&self.wp_description);
           out.push_str("\n\n");

           // Scope
           out.push_str("## Scope\n\n");
           out.push_str(&self.scope_boundaries);
           out.push_str("\n\n");

           // Dependencies
           if !self.dependency_context.is_empty() {
               out.push_str("## Upstream Dependencies\n\n");
               for dep in &self.dependency_context {
                   out.push_str(&format!("### {} – {}\n", dep.wp_id, dep.wp_title));
                   out.push_str(&dep.summary);
                   out.push_str("\n\n**Key outputs**: ");
                   out.push_str(&dep.key_outputs.join(", "));
                   out.push_str("\n\n");
               }
           }

           // Subtasks
           out.push_str("## Subtasks\n\n");
           for st in &self.subtasks {
               let parallel_marker = if st.parallel { " [P]" } else { "" };
               out.push_str(&format!("- [ ] **{}**{}: {}\n", st.id, parallel_marker, st.description));
           }
           out.push_str("\n");

           // Constraints
           if !self.constraints.is_empty() {
               out.push_str("## Constraints\n\n");
               for c in &self.constraints {
                   out.push_str(&format!("- {}\n", c));
               }
               out.push_str("\n");
           }

           // AGENTS.md
           if let Some(agents) = &self.agents_md {
               out.push_str("## Project Agent Instructions\n\n");
               out.push_str(agents);
               out.push_str("\n\n");
           }

           out
       }
   }
   ```

**Files**:
- `crates/kasmos/src/prompt.rs` (new, ~100 lines)

**Parallel**: Yes — template struct is independent of I/O operations.

### Subtask T021 – Dependency Context Injection

**Purpose**: For each WP, collect summaries of upstream WPs so the agent understands what's already been implemented.

**Steps**:

1. Add to `crates/kasmos/src/prompt.rs`:
   ```rust
   pub struct PromptGenerator {
       feature_dir: PathBuf,
       agents_md: Option<String>,
   }

   impl PromptGenerator {
       pub fn new(feature_dir: &Path) -> Result<Self> {
           let agents_md = Self::load_agents_md(feature_dir)?;
           Ok(Self { feature_dir: feature_dir.to_owned(), agents_md })
       }

       /// Build dependency context for a WP from its upstream dependencies.
       fn build_dependency_context(
           &self,
           wp: &WorkPackage,
           all_wps: &[WorkPackage],
       ) -> Vec<DependencyContext> {
           wp.dependencies.iter()
               .filter_map(|dep_id| {
                   all_wps.iter().find(|w| w.id == *dep_id)
               })
               .map(|dep_wp| DependencyContext {
                   wp_id: dep_wp.id.clone(),
                   wp_title: dep_wp.title.clone(),
                   summary: format!("Provides {} functionality", dep_wp.title.to_lowercase()),
                   key_outputs: self.infer_key_outputs(dep_wp),
               })
               .collect()
       }

       /// Infer what files/modules a WP produces based on its title/ID.
       fn infer_key_outputs(&self, wp: &WorkPackage) -> Vec<String> {
           // Heuristic based on WP structure — can be enhanced
           vec![format!("crates/kasmos/src/{}.rs", wp.id.to_lowercase())]
       }
   }
   ```

**Files**:
- `crates/kasmos/src/prompt.rs` (continued, ~50 lines)

### Subtask T022 – AGENTS.md Content Inclusion

**Purpose**: Read the project's AGENTS.md file and include its content in every agent prompt.

**Steps**:

1. Add to PromptGenerator:
   ```rust
   impl PromptGenerator {
       fn load_agents_md(feature_dir: &Path) -> Result<Option<String>> {
           // Look for AGENTS.md in project root (parent of kitty-specs/)
           let project_root = feature_dir
               .ancestors()
               .find(|p| p.join("AGENTS.md").exists() || p.join("Cargo.toml").exists())
               .unwrap_or(feature_dir);

           let agents_path = project_root.join("AGENTS.md");
           if agents_path.exists() {
               let content = std::fs::read_to_string(&agents_path)?;
               tracing::info!(path = %agents_path.display(), "Loaded AGENTS.md");
               Ok(Some(content))
           } else {
               tracing::warn!("AGENTS.md not found, prompts will omit project instructions");
               Ok(None)
           }
       }
   }
   ```

**Files**:
- `crates/kasmos/src/prompt.rs` (continued, ~20 lines)

### Subtask T023 – Write Prompts to .kasmos/prompts/

**Purpose**: Write rendered prompt files to the `.kasmos/prompts/` directory.

**Steps**:

1. Add write method:
   ```rust
   impl PromptGenerator {
       /// Generate and write prompt files for all work packages.
       pub fn generate_all(
           &self,
           wps: &[WorkPackage],
           kasmos_dir: &Path,
       ) -> Result<Vec<PathBuf>> {
           let prompts_dir = kasmos_dir.join("prompts");
           std::fs::create_dir_all(&prompts_dir)?;

           let mut paths = Vec::new();
           for wp in wps {
               let ctx = self.build_prompt_context(wp, wps);
               let rendered = ctx.render();

               // Warn if prompt is very long
               if rendered.len() > 10_000 {
                   tracing::warn!(
                       wp_id = %wp.id,
                       chars = rendered.len(),
                       "Prompt exceeds 10K characters, may cause issues with OpenCode stdin"
                   );
               }

               let path = prompts_dir.join(format!("{}.md", wp.id));
               std::fs::write(&path, &rendered)?;
               tracing::debug!(wp_id = %wp.id, path = %path.display(), "Prompt written");
               paths.push(path);
           }

           Ok(paths)
       }
   }
   ```

**Files**:
- `crates/kasmos/src/prompt.rs` (continued, ~30 lines)

### Subtask T024 – Shell Wrapper Scripts for stdin Pipe

**Purpose**: Generate executable shell scripts that pipe prompt files into OpenCode, one per WP.

**Steps**:

1. Add script generation:
   ```rust
   impl PromptGenerator {
       /// Generate shell wrapper scripts for each WP.
       pub fn generate_scripts(
           &self,
           wps: &[WorkPackage],
           kasmos_dir: &Path,
       ) -> Result<Vec<PathBuf>> {
           let scripts_dir = kasmos_dir.join("scripts");
           std::fs::create_dir_all(&scripts_dir)?;

           let mut paths = Vec::new();
           for wp in wps {
               let prompt_path = kasmos_dir.join("prompts").join(format!("{}.md", wp.id));
               let script_content = format!(
                   "#!/bin/bash\nset -euo pipefail\ncat {} | opencode -p 'context:'\n",
                   prompt_path.display()
               );

               let script_path = scripts_dir.join(format!("{}.sh", wp.id));
               std::fs::write(&script_path, &script_content)?;

               // Make executable
               #[cfg(unix)]
               {
                   use std::os::unix::fs::PermissionsExt;
                   let mut perms = std::fs::metadata(&script_path)?.permissions();
                   perms.set_mode(0o755);
                   std::fs::set_permissions(&script_path, perms)?;
               }

               paths.push(script_path);
           }

           Ok(paths)
       }
   }
   ```

**Files**:
- `crates/kasmos/src/prompt.rs` (continued, ~30 lines)

### Subtask T025 – Validate OpenCode Binary in PATH [P]

**Purpose**: Check that the OpenCode binary is available before attempting to launch agent panes.

**Steps**:

1. Add validation:
   ```rust
   /// Verify that a binary is available in PATH.
   pub fn validate_binary_in_path(binary: &str) -> Result<PathBuf> {
       which::which(binary).map_err(|_| {
           KasmosError::Config(ConfigError::InvalidValue {
               field: format!("{}_binary", binary),
               value: binary.to_string(),
               reason: format!("'{}' not found in PATH. Install it or set the full path in config.", binary),
           })
       })
   }
   ```

2. Add `which` crate to dependencies
3. Call for both `opencode` and `zellij` during initialization

**Files**:
- `crates/kasmos/src/prompt.rs` or `crates/kasmos/src/validation.rs` (new, ~20 lines)

**Parallel**: Yes — independent of prompt generation.

## Test Strategy

- Unit test: render prompt with all fields populated → valid markdown with all sections
- Unit test: render prompt with no dependencies → omits "Upstream Dependencies" section
- Unit test: render prompt with no AGENTS.md → omits "Project Agent Instructions" section
- Unit test: prompt length warning triggers at >10K characters
- Unit test: shell wrapper script is valid bash (starts with shebang, references correct path)
- Unit test: validate_binary_in_path finds `bash` (always available) and fails on `nonexistent_binary_xyz`

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Prompt too long for OpenCode stdin | Medium | Length warning + potential truncation |
| AGENTS.md missing | Low | Graceful fallback with warning log |
| Shell wrapper path issues on different systems | Low | Use absolute paths from config |

## Review Guidance

**Acceptance Checkpoints**:
- [ ] Prompts contain WP metadata, subtasks, scope, constraints
- [ ] Dependency context correctly references upstream WPs
- [ ] AGENTS.md content is included when available
- [ ] Shell wrappers are executable and correctly formatted
- [ ] Missing AGENTS.md produces warning, not error
- [ ] OpenCode validation provides actionable error message
- [ ] Unit tests pass

## Activity Log

2026-02-09T00:00:00Z – system – lane=planned – Prompt created.

### Updating Lane Status

To update this work package's lane, either:
1. Edit the `lane` field in the frontmatter directly, or
2. Run: `spec-kitty agent tasks move-task WP04 --to <lane>`

Valid lanes: `planned`, `doing`, `for_review`, `done`

### File Structure

This file lives in `tasks/` (flat directory). Lane status is tracked ONLY in the `lane:` frontmatter field, NOT by directory location.
- 2026-02-09T03:25:03Z – opencode – shell_pid=3190789 – lane=doing – Assigned agent via workflow command
- 2026-02-09T03:36:14Z – opencode – shell_pid=3190789 – lane=for_review – Ready for review: Prompt generation, dependency context injection, AGENTS inclusion, script wrappers, binary validation
- 2026-02-09T03:51:00Z – opencode – shell_pid=3190789 – lane=doing – Started review via workflow command
- 2026-02-09T03:53:15Z – opencode – shell_pid=3190789 – lane=planned – Moved to planned
