---
work_package_id: WP11
title: Agent Profiles and Prompt Context Boundaries
lane: "for_review"
dependencies: [WP02]
base_branch: 011-mcp-agent-swarm-orchestration-WP02
base_commit: 839ff563e7dfa7894ce4b53b37f439478bf887a6
created_at: '2026-02-14T22:27:45.338853+00:00'
subtasks:
- T063
- T064
- T065
- T066
- T067
- T068
phase: Phase 3 - Setup UX, Role Context, and End-to-End Hardening
assignee: ''
agent: ''
shell_pid: "3114343"
review_status: ''
reviewed_by: ''
history:
- timestamp: '2026-02-14T16:27:48Z'
  lane: planned
  agent: system
  shell_pid: ''
  action: Prompt generated via /spec-kitty.tasks
---

# Work Package Prompt: WP11 - Agent Profiles and Prompt Context Boundaries

## Important: Review Feedback Status

- **Has review feedback?**: Check the `review_status` field above.

---

## Review Feedback

*[This section is empty initially.]*

---

## Implementation Command

```bash
spec-kitty implement WP11 --base WP02
```

---

## Objectives & Success Criteria

Implement role-specific prompt/context assembly that enforces scope boundaries and OpenCode runtime consistency (FR-025, FR-028-033). After this WP:

1. Manager prompts include broadest context: full spec, plan, task board, architecture memory, project structure
2. Coder prompts include narrow context: specific WP task file only, coding standards, scoped architecture
3. Reviewer prompts include medium context: WP task file, coder changes, acceptance criteria, standards
4. Release prompts include broad structural context: all WP statuses, branch structure, merge target
5. Planner prompts include medium-broad context: full spec, plan, workflow state, architecture memory, workflow intelligence - but NOT individual WP task files or coding standards (FR-033)
6. All roles use a single agent runtime (OpenCode via ocx) - FR-025
6. Profile assets exist under `config/profiles/kasmos/` with role-specific configurations

## Context & Constraints

- **Depends on WP02**: Config system available
- **Spec FR-025**: Single agent runtime (OpenCode) for all roles
- **Spec FR-028**: Manager = broadest context
- **Spec FR-029**: Coder = narrowest (WP task file as contract, standards, scoped arch memory)
- **Spec FR-030**: Reviewer = medium (WP task file, changes, acceptance criteria, standards)
- **Spec FR-031**: Release = broad structural (all WP statuses, branch structure, merge target)
- **Spec FR-033**: Planner = medium-broad (full spec, plan, workflow state, architecture, workflow intelligence; NOT WP task files or coding standards)
- **Existing code**: `crates/kasmos/src/prompt.rs` (490 lines) has `PromptGenerator` and `PromptContext` for generating agent prompts. `crates/kasmos/src/session.rs` builds opencode commands.
- **Plan**: Profile assets at `config/profiles/kasmos/` with `opencode.jsonc` and role `.md` files

## Subtasks & Detailed Guidance

### Subtask T063 - Define/update OpenCode profile assets

**Purpose**: Create the OpenCode configuration and role-specific prompt templates that define how each agent role is configured.

**Steps**:
1. Create `config/profiles/kasmos/opencode.jsonc`:
   ```jsonc
   {
     // OpenCode profile for kasmos agent swarm
     "mcpServers": {
       "kasmos": {
         "command": "kasmos",
         "args": ["serve"],
         "type": "stdio"
       },
       "zellij-pane-tracker": {
         // Pane tracking MCP server config
       }
     }
   }
   ```
2. Create role-specific prompt templates:
   - `config/profiles/kasmos/agent/manager.md` - Manager system prompt template
   - `config/profiles/kasmos/agent/planner.md` - Planner (planning phase worker) template
   - `config/profiles/kasmos/agent/coder.md` - Coder system prompt template
   - `config/profiles/kasmos/agent/reviewer.md` - Reviewer system prompt template
   - `config/profiles/kasmos/agent/release.md` - Release agent system prompt template
3. Each template defines:
   - Role identity and responsibilities
   - Available MCP tools and how to use them
   - Communication protocol (how to send messages to msg-log)
   - Context boundaries (what to read, what NOT to read)
   - Completion signaling protocol

**Files**: `config/profiles/kasmos/opencode.jsonc`, `config/profiles/kasmos/agent/*.md`
**Validation**: Profile directory has all required files with correct structure.

### Subtask T064 - Rewrite prompt.rs for role-aware context assembly

**Purpose**: Replace the current prompt generation with role-aware context assembly that respects scope boundaries.

**Steps**:
1. Refactor `crates/kasmos/src/prompt.rs` to support role-based prompt generation:
   ```rust
   pub enum AgentRole {
       Manager,
       Planner,
       Coder,
       Reviewer,
       Release,
   }

   pub struct RolePromptBuilder {
       role: AgentRole,
       feature_slug: String,
       feature_dir: PathBuf,
       wp_id: Option<String>,
       wp_file: Option<PathBuf>,
       additional_context: Option<String>,
   }

   impl RolePromptBuilder {
       pub fn build(&self) -> Result<String> {
           match self.role {
               AgentRole::Manager => self.build_manager_prompt(),
               AgentRole::Planner => self.build_planner_prompt(),
               AgentRole::Coder => self.build_coder_prompt(),
               AgentRole::Reviewer => self.build_reviewer_prompt(),
               AgentRole::Release => self.build_release_prompt(),
           }
       }
   }
   ```
2. Each role builder loads the corresponding template from `config/profiles/kasmos/agent/` and injects context.
3. Preserve the existing `PromptContext` and `PromptGenerator` behind `#[cfg(feature = "tui")]` for legacy compatibility.
4. The new `RolePromptBuilder` is the primary API for MCP-mode prompts.

**Files**: `crates/kasmos/src/prompt.rs`
**Validation**: Each role produces a prompt with appropriate context.

### Subtask T065 - Implement manager bootstrap prompt contract

**Purpose**: Define the manager's startup prompt that instructs it on assessment, confirmation gates, and kasmos serve subprocess ownership.

**Steps**:
1. Manager prompt includes:
   - Full feature spec summary (read from spec.md)
   - Plan summary (read from plan.md)
   - Task board overview (read from tasks.md - all WP statuses)
   - Architecture memory (read from `.kittify/memory/architecture.md`)
   - Workflow intelligence (read from `.kittify/memory/workflow-intelligence.md`)
   - Constitution reference (path to `.kittify/memory/constitution.md`)
   - Project structure overview
   - Instructions for `kasmos serve` MCP tools
   - Explicit instruction: assess phase, present summary, wait for confirmation
2. The prompt is assembled by reading actual file contents and embedding relevant sections.
3. Keep the prompt focused despite breadth - summarize large documents rather than including them verbatim.
4. Include explicit tool usage guidance: which MCP tools to use for each operation.

**Files**: `crates/kasmos/src/prompt.rs`, `config/profiles/kasmos/agent/manager.md`
**Validation**: Manager prompt includes all required context references.

### Subtask T066 - Implement worker prompt contract for message-log communication

**Purpose**: All worker prompts must include instructions for structured message-log communication.

**Steps**:
1. Every worker prompt (coder, reviewer, release) must include:
   ```
   ## Communication Protocol
   When you reach a milestone, send a structured message to the msg-log pane:
   - Use the zellij-pane-tracker run-in-pane tool
   - Target pane: "msg-log"
   - Message format: echo '[KASMOS:<your_id>:<event>] {"wp_id":"<wp>", ...}'

   Events you must send:
   - STARTED: When you begin work
   - PROGRESS: At significant milestones
   - DONE: When your task is complete
   - ERROR: If you encounter a blocking error
   - NEEDS_INPUT: If you need user/manager input
   ```
2. Coder-specific additions: REVIEW_PASS and REVIEW_REJECT are NOT sent by coders.
3. Reviewer-specific additions: Send REVIEW_PASS or REVIEW_REJECT with feedback payload.
4. Embed this protocol in each role's prompt template.

**Files**: `config/profiles/kasmos/agent/coder.md`, `config/profiles/kasmos/agent/reviewer.md`, `config/profiles/kasmos/agent/release.md`
**Validation**: Worker prompts include communication protocol instructions.

### Subtask T067 - Enforce context minimization rules by role

**Purpose**: Ensure coders don't receive full spec, reviewers don't receive other WPs, etc.

**Steps**:
1. Define context allowlists per role:
   ```rust
   fn allowed_context(role: &AgentRole) -> ContextBoundary {
       match role {
           AgentRole::Manager => ContextBoundary {
               spec: true, plan: true, all_tasks: true,
               architecture: true, workflow_intelligence: true,
               constitution: true, project_structure: true,
           },
           AgentRole::Coder => ContextBoundary {
               spec: false, plan: false, all_tasks: false,
               architecture: true,  // scoped to relevant subsystems
               workflow_intelligence: false,
               constitution: true,
               project_structure: false,
           },
           AgentRole::Reviewer => ContextBoundary {
               spec: false, plan: false, all_tasks: false,
               architecture: true,  // scoped to affected areas
               workflow_intelligence: false,
               constitution: true,
               project_structure: false,
           },
           AgentRole::Release => ContextBoundary {
               spec: false, plan: false, all_tasks: true,  // statuses only
               architecture: false,
               workflow_intelligence: false,
               constitution: true,
               project_structure: true,
           },
           AgentRole::Planner => ContextBoundary {
               spec: true, plan: true, all_tasks: false,
               architecture: true, workflow_intelligence: true,
               constitution: true, project_structure: true,
           },
       }
   }
   ```
2. The prompt builder uses this boundary to include or exclude context sections.
3. **Coder gets**: WP task file (as contract), constitution, scoped architecture memory
4. **Coder does NOT get**: Full spec, other WPs, plan, workflow intelligence
5. This is enforced at prompt generation time - the builder physically doesn't read excluded files.

**Files**: `crates/kasmos/src/prompt.rs`
**Validation**: Coder prompt does not contain spec content. Reviewer prompt does not contain other WP content.

### Subtask T068 - Add prompt snapshot tests and boundary validation

**Purpose**: Test that each role's prompt contains required context and excludes forbidden context.

**Steps**:
1. Snapshot test for each role using a test feature directory:
   - Manager prompt contains: spec summary, plan summary, all WP statuses, architecture
   - Coder prompt contains: specific WP task file content, constitution reference
   - Coder prompt does NOT contain: spec content, plan content, other WP content
   - Reviewer prompt contains: WP task file, acceptance criteria, constitution
   - Release prompt contains: all WP statuses, branch info, constitution
2. Boundary validation tests:
   - Verify `allowed_context` returns correct boundaries for each role
   - Verify prompt builder respects boundaries (doesn't read excluded files)
3. Use insta or similar for snapshot testing, or simple string assertion tests.

**Files**: Test modules in `crates/kasmos/src/prompt.rs`
**Validation**: `cargo test` passes with prompt boundary tests.

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Prompts leak unnecessary context to coder role | Explicit allowlists and snapshot assertions verify exclusion |
| Manager prompt too vague for deterministic orchestration | Codify explicit stage gates and tool usage loops in template |
| Profile assets out of sync with code | Generate from templates, test that they match expected structure |

## Review Guidance

- Verify manager gets broadest context (FR-028)
- Verify coder gets narrowest context (FR-029) - NO spec, plan, or other WPs
- Verify reviewer gets medium context (FR-030) - task file + changes + criteria
- Verify release gets structural context (FR-031)
- Verify all roles use OpenCode (FR-025)
- Verify communication protocol is in all worker prompts
- Verify snapshot tests catch boundary violations

## Activity Log

- 2026-02-14T16:27:48Z - system - lane=planned - Prompt generated via /spec-kitty.tasks
- 2026-02-14T22:25:37Z – unknown – shell_pid=3674747 – lane=planned – Moved to planned
- 2026-02-14T22:27:10Z – unknown – shell_pid=3674747 – lane=planned – Moved to planned
- 2026-02-15T00:51:10Z – unknown – shell_pid=3114343 – lane=for_review – Ready for review
