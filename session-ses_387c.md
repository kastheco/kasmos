# spec-kitty commands skill loading update

**Session ID:** ses_387cabbf6ffe3jcNLxxD3G0KIe
**Created:** 2/19/2026, 5:21:22 PM
**Updated:** 2/19/2026, 5:31:54 PM

---

## User

Update all spec-kitty commands in this repo to explicitly tell the agent to load the spec-kitty skill (.opencode/commands/*) as well as the ones that get installed by this app 'kasmos setup'

---

## Assistant (Manager · claude-opus-4-6 · 617.1s)



_Thinking:_

The user wants me to update spec-kitty commands to explicitly tell the agent to load the spec-kitty skill. Let me first explore the codebase to understand:

1. What spec-kitty commands exist in `.opencode/commands/`
2. What commands get installed by `kasmos setup`
3. How the spec-kitty skill loading works

Let me start by exploring the relevant directories.

Let me first explore the relevant directories to understand the current state of spec-kitty commands and the setup process.

**Tool: task**

**Input:**
```json
{
  "description": "Explore spec-kitty commands",
  "prompt": "I need to understand the full picture of spec-kitty commands in this repo. Please thoroughly explore:\n\n1. All files in `.opencode/commands/` - read every file completely\n2. The `internal/setup/` directory - especially how `kasmos setup` installs commands\n3. Any command templates or embedded command files that get installed\n4. The `.opencode/skills/spec-kitty/SKILL.md` file to understand what the skill provides\n5. Any other references to spec-kitty commands in the codebase\n\nFor each command file found, report:\n- Full file path\n- Full file contents\n- Whether it references the spec-kitty skill or not\n\nAlso check `internal/setup/` for any embedded/template command files that get installed by `kasmos setup`.\n\nReturn ALL of this information in detail - I need to see every command file's full contents.",
  "subagent_type": "explore"
}
```

**Output:**
```
task_id: ses_387ca9908ffeSuCSjfPeNFY66H (for resuming to continue this task if needed)

<task_result>
**Setup Findings**
- `kasmos setup` only scaffolds agent definitions into `.opencode/agents/`; no command installation logic in `/home/kas/dev/kasmos/internal/setup/setup.go` or `/home/kas/dev/kasmos/internal/setup/agents.go`.
- Agent templates are inline strings in `/home/kas/dev/kasmos/internal/setup/agents.go`; dependencies check only in `/home/kas/dev/kasmos/internal/setup/deps.go`.
- No embedded command files found (no `go:embed` usage).
- Command template directories exist but are not installed by `kasmos setup`: `/home/kas/dev/kasmos/.kittify/missions/software-dev/command-templates/`, `/home/kas/dev/kasmos/.kittify/missions/research/command-templates/`, `/home/kas/dev/kasmos/.kittify/missions/documentation/command-templates/`.
- Command template paths referenced by commands but not present in repo: `templates/commands/constitution.md` and `.kittify/templates/commands/research.md`.
- There is a second command set under `/home/kas/dev/kasmos/config/profiles/kasmos/commands/` (15 files) in addition to `/home/kas/dev/kasmos/.opencode/commands/`.

**Command Files (.opencode/commands) — Batch 1**
- Reference rule: “References spec-kitty skill” = explicit mention of `spec-kitty` or `/spec-kitty.*` in the command file; none directly reference `.opencode/skills/spec-kitty` by path.

**Command File**
- Path: `/home/kas/dev/kasmos/.opencode/commands/kas.verify.md`
- References spec-kitty skill: No (no `spec-kitty` mentions)
```markdown
1: ---
2: description: Tiered verification with built-in specialist checks (self-contained)
3: agent: reviewer
4: ---
5: 
6: # /kas.verify - Tiered Verification (Self-Contained)
7: 
8: Run a three-tier verification workflow with early exits:
9: 1) static analysis, 2) reality assessment, 3) optional simplification suggestions.
10: 
11: This command is self-contained and does not depend on external plugin commands or agent files.
12: 
13: ## User Input
14: 
15: ```text
16: $ARGUMENTS
17: ```
18: 
19: Treat arguments as optional scope hints (specific files, directories, or review focus).
20: 
21: ## Phase 0 - Build review context
22: 
23: Run:
24: 
25: ```bash
26: git status
27: git diff --stat
28: git diff
29: git diff --cached
30: ```
31: 
32: If both `git diff` and `git diff --cached` are empty: report `Nothing to verify` and stop.
33: 
34: ## Phase 1 - Change profile and Tier 1 static analysis
35: 
36: ### 1a) Derive change profile
37: 
38: From changed files + diff content, set these booleans:
39: 
40: - `has_code_changes`
41: - `has_error_handling_changes` (try/catch, recover, error wrapping/propagation, fallback paths)
42: - `has_comments_or_docs_changes` (comments, docstrings, markdown/docs edits)
43: - `has_type_or_schema_changes` (types, interfaces, structs, schema definitions, validation contracts)
44: - `has_test_changes`
45: - `docs_only` (all changes are docs)
46: - `config_only` (all changes are config)
47: 
48: ### 1b) Select Tier 1 specialist checks
49: 
50: Run only relevant checks:
51: 
52: - `core-code-reviewer` (run for any code change; also run for config-only changes)
53: - `silent-failure-hunter` (run when `has_error_handling_changes`)
54: - `comment-analyzer` (run when `has_comments_or_docs_changes` and code is touched)
55: - `type-design-analyzer` (run when `has_type_or_schema_changes`)
56: - `pr-test-analyzer` (run when `has_test_changes` OR code changed without corresponding tests)
57: 
58: If `docs_only`, skip Tier 1 and continue to Tier 2.
59: 
60: If your runtime supports subagents, run selected Tier 1 checks in parallel. If not, run sequentially.
61: 
62: ### 1c) Tier 1 check rubrics (embedded)
63: 
64: #### core-code-reviewer
65: 
66: Review for correctness, security, readability, performance, maintainability, and robust error handling.
67: 
68: Focus:
69: - logic bugs and edge cases
70: - unsafe behavior or vulnerabilities
71: - unnecessary complexity or unclear naming
72: - regressions and missing safeguards
73: 
74: #### silent-failure-hunter
75: 
76: Audit error handling with zero tolerance for hidden failures.
77: 
78: Flag:
79: - empty catch/ignore blocks
80: - swallowed errors without propagation
81: - fallback behavior that masks real failures
82: - broad catches that can hide unrelated errors
83: - missing user-actionable error reporting
84: 
85: #### comment-analyzer
86: 
87: Audit comments/docstrings for long-term accuracy.
88: 
89: Flag:
90: - comments that contradict implementation
91: - stale TODO/FIXME and outdated assumptions
92: - comments that describe obvious code instead of intent
93: - missing documentation for non-obvious public behavior
94: 
95: #### type-design-analyzer
96: 
97: Audit type/schema design quality and invariants.
98: 
99: Evaluate:
100: - encapsulation and illegal-state prevention
101: - invariant clarity and enforcement
102: - nullability/optionality correctness
103: - schema strictness vs over-permissiveness
104: 
105: #### pr-test-analyzer
106: 
107: Audit test adequacy for changed behavior.
108: 
109: Flag:
110: - missing coverage for critical paths and edge cases
111: - missing negative/error-path tests
112: - brittle tests tied to implementation details
113: - weak assertions that would miss regressions
114: 
115: ### 1d) Tier 1 exit rules
116: 
117: - Any Critical or High finding -> `BLOCKED` (stop)
118: - Any Medium finding -> `NEEDS_CHANGES` (stop)
119: - Otherwise continue to Tier 2
120: 
121: ## Phase 2 - Reality assessment (required when changes exist)
122: 
123: Run a skeptical completion audit focused on what actually works, not what appears implemented.
124: 
125: Validate:
126: 
127: - behavior correctness end-to-end
128: - integration completeness (no stubs or half-wired seams)
129: - alignment between claimed completion and real implementation
130: - practical operability under realistic failure paths
131: 
132: When feasible, run representative verification commands for touched areas (tests/build/lint) and include results.
133: 
134: Tier 2 exits:
135: 
136: - severe functional gaps -> `BLOCKED`
137: - actionable but non-severe gaps -> `NEEDS_CHANGES`
138: - no material gaps -> `VERIFIED`
139: 
140: ## Phase 3 - Simplification pass (optional, only if VERIFIED)
141: 
142: If `VERIFIED` and code changed, provide non-blocking simplification suggestions.
143: 
144: Focus:
145: 
146: - remove unnecessary abstraction/nesting
147: - improve readability without behavior change
148: - reduce cognitive load and maintenance risk
149: 
150: Do not change code in this command; suggestions only.
151: 
152: ## Output format (strict)
153: 
154: ```markdown
155: DECISION: VERIFIED | NEEDS_CHANGES | BLOCKED
156: TIER_REACHED: 1 | 2 | 3
157: SEVERITY_SUMMARY: Critical=<n>, High=<n>, Medium=<n>, Low=<n>
158: 
159: SCOPE:
160: - arguments: <value or none>
161: - files_reviewed: <count>
162: 
163: CHANGE_PROFILE:
164: - has_code_changes: <true|false>
165: - has_error_handling_changes: <true|false>
166: - has_comments_or_docs_changes: <true|false>
167: - has_type_or_schema_changes: <true|false>
168: - has_test_changes: <true|false>
169: - docs_only: <true|false>
170: - config_only: <true|false>
171: 
172: CHECKS_RUN:
173: - Tier 1: <comma-separated checks or skipped>
174: - Tier 2: reality-assessment <run|skipped>
175: - Tier 3: code-simplifier <run|skipped>
176: 
177: FINDINGS:
178: - [severity] [check] file:line - issue and impact; recommended fix
179: 
180: REALITY_GAPS:
181: - [gap severity] claim vs actual behavior (include evidence)
182: 
183: SIMPLIFICATION_SUGGESTIONS:
184: - optional improvements (only when DECISION=VERIFIED)
185: 
186: NEXT_ACTION:
187: - one concrete operator step
188: ```
189: 
190: ## Rules
191: 
192: - Read-only review; do not modify files
193: - Never auto-fix without explicit user request
194: - Keep findings concrete with `file:line` references
195: - If a check is skipped, state why
196: - Empty diff exits early with `Nothing to verify`
197: 
```

**Command File**
- Path: `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.accept.md`
- References spec-kitty skill: Yes (`spec-kitty accept`, `/spec-kitty.accept`)
```markdown
1: ---
2: description: Validate feature readiness and guide final acceptance steps.
3: ---
4: 
5: # /spec-kitty.accept - Validate Feature Readiness
6: 
7: **Version**: 0.11.0+
8: **Purpose**: Validate all work packages are complete and feature is ready to merge.
9: 
10: ## Agent Routing (Cost Tier)
11: 
12: - Route acceptance checks through the `reviewer` agent (medium tier).
13: - Keep the command runner in controller mode: gather inputs, dispatch checks, and report results.
14: - Escalate to controller-only handling when acceptance blockers are ambiguous or contradictory.
15: - Profile default: `reviewer` -> `anthropic/claude-opus-4-6` with `reasoningEffort: high`.
16: 
17: ## 📍 WORKING DIRECTORY: Run from MAIN repository
18: 
19: **IMPORTANT**: Accept runs from the main repository root, NOT from a WP worktree.
20: 
21: ```bash
22: # If you're in a worktree, return to main first:
23: cd $(git rev-parse --show-toplevel)
24: 
25: # Then run accept:
26: spec-kitty accept
27: ```
28: 
29: ## User Input
30: 
31: ```text
32: $ARGUMENTS
33: ```
34: 
35: You **MUST** consider the user input before proceeding (if not empty).
36: 
37: ## Discovery (mandatory)
38: 
39: Before running the acceptance workflow, gather the following:
40: 
41: 1. **Feature slug** (e.g., `005-awesome-thing`). If omitted, detect automatically.
42: 2. **Acceptance mode**:
43:    - `pr` when the feature will merge via hosted pull request.
44:    - `local` when the feature will merge locally without a PR.
45:    - `checklist` to run the readiness checklist without committing or producing merge instructions.
46: 3. **Validation commands executed** (tests/builds). Collect each command verbatim; omit if none.
47: 4. **Acceptance actor** (optional, defaults to the current agent name).
48: 
49: Ask one focused question per item and confirm the summary before continuing. End the discovery turn with `WAITING_FOR_ACCEPTANCE_INPUT` until all answers are provided.
50: 
51: ## Execution Plan
52: 
53: 0. **Smoke-import gate** (auto-detected, mandatory for Python features):
54:    - Scan the feature's changed files for any Python packages (directories containing `__init__.py`).
55:    - For each discovered package, run:
56:      ```bash
57:      python -c "import <package_dotted_path>"
58:      ```
59:    - If ANY import fails, **stop immediately** and report the ImportError with the exact broken name and source module. Do NOT proceed to the acceptance CLI.
60:    - This catches cross-WP integration bugs (name drift between `__init__.py` re-exports and actual implementations) that per-WP reviews may miss.
61: 1. Compile the acceptance options into an argument list:
62:    - Always include `--actor "__AGENT__"`.
63:    - Append `--feature "<slug>"` when the user supplied a slug.
64:    - Append `--mode <mode>` (`pr`, `local`, or `checklist`).
65:    - Append `--test "<command>"` for each validation command provided.
66: 2. Run `{SCRIPT}` (the CLI wrapper) with the assembled arguments **and** `--json`.
67: 3. Parse the JSON response. It contains:
68:    - `summary.ok` (boolean) and other readiness details.
69:    - `summary.outstanding` categories when issues remain.
70:    - `instructions` (merge steps) and `cleanup_instructions`.
71:    - `notes` (e.g., acceptance commit hash).
72: 4. Present the outcome:
73:    - If `summary.ok` is `false`, list each outstanding category with bullet points and advise the user to resolve them before retrying acceptance.
74:    - If `summary.ok` is `true`, display:
75:      - Acceptance timestamp, actor, and (if present) acceptance commit hash.
76:      - Merge instructions and cleanup instructions as ordered steps.
77:      - Validation commands executed (if any).
78: 5. When the mode is `checklist`, make it clear no commits or merge instructions were produced.
79: 
80: ## Output Requirements
81: 
82: - Summaries must be in plain text (no tables). Use short bullet lists for instructions.
83: - Surface outstanding issues before any congratulations or success messages.
84: - If the JSON payload includes warnings, surface them under an explicit **Warnings** section.
85: - Never fabricate results; only report what the JSON contains.
86: 
87: ## Error Handling
88: 
89: - If the command fails or returns invalid JSON, report the failure and request user guidance (do not retry automatically).
90: - When outstanding issues exist, do **not** attempt to force acceptance—return the checklist and prompt the user to fix the blockers.
91: 
```

**Command File**
- Path: `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.analyze.md`
- References spec-kitty skill: Yes (`/spec-kitty.*`, `spec-kitty agent ...`)
```markdown
1: ---
2: description: Perform a non-destructive cross-artifact consistency and quality analysis across spec.md, plan.md, and tasks.md after task generation.
3: ---
4: 
5: ## User Input
6: 
7: ```text
8: $ARGUMENTS
9: ```
10: 
11: You **MUST** consider the user input before proceeding (if not empty).
12: 
13: ## Goal
14: 
15: Identify inconsistencies, duplications, ambiguities, and underspecified items across the three core artifacts (`spec.md`, `plan.md`, `tasks.md`) before implementation. This command MUST run only after `/tasks` has successfully produced a complete `tasks.md`.
16: 
17: ## Operating Constraints
18: 
19: **STRICTLY READ-ONLY**: Do **not** modify any files. Output a structured analysis report. Offer an optional remediation plan (user must explicitly approve before any follow-up editing commands would be invoked manually).
20: 
21: **Constitution Authority**: The project constitution (`/memory/constitution.md`) is **non-negotiable** within this analysis scope. Constitution conflicts are automatically CRITICAL and require adjustment of the spec, plan, or tasks—not dilution, reinterpretation, or silent ignoring of the principle. If a principle itself needs to change, that must occur in a separate, explicit constitution update outside `/analyze`.
22: 
23: ## Execution Steps
24: 
25: ### 1. Initialize Analysis Context
26: 
27: Run `{SCRIPT}` once from repo root and parse JSON for FEATURE_DIR and AVAILABLE_DOCS. Derive absolute paths:
28: 
29: - SPEC = FEATURE_DIR/spec.md
30: - PLAN = FEATURE_DIR/plan.md
31: - TASKS = FEATURE_DIR/tasks.md
32: 
33: Abort with an error message if any required file is missing (instruct the user to run missing prerequisite command).
34: 
35: ### 2. Load Artifacts (Progressive Disclosure)
36: 
37: Load only the minimal necessary context from each artifact:
38: 
39: **From spec.md:**
40: 
41: - Overview/Context
42: - Functional Requirements
43: - Non-Functional Requirements
44: - User Stories
45: - Edge Cases (if present)
46: 
47: **From plan.md:**
48: 
49: - Architecture/stack choices
50: - Data Model references
51: - Phases
52: - Technical constraints
53: 
54: **From tasks.md:**
55: 
56: - Task IDs
57: - Descriptions
58: - Phase grouping
59: - Parallel markers [P]
60: - Referenced file paths
61: 
62: **From constitution:**
63: 
64: - Load `/memory/constitution.md` for principle validation
65: 
66: ### 3. Build Semantic Models
67: 
68: Create internal representations (do not include raw artifacts in output):
69: 
70: - **Requirements inventory**: Each functional + non-functional requirement with a stable key (derive slug based on imperative phrase; e.g., "User can upload file" → `user-can-upload-file`)
71: - **User story/action inventory**: Discrete user actions with acceptance criteria
72: - **Task coverage mapping**: Map each task to one or more requirements or stories (inference by keyword / explicit reference patterns like IDs or key phrases)
73: - **Constitution rule set**: Extract principle names and MUST/SHOULD normative statements
74: 
75: ### 4. Detection Passes (Token-Efficient Analysis)
76: 
77: Focus on high-signal findings. Limit to 50 findings total; aggregate remainder in overflow summary.
78: 
79: #### A. Duplication Detection
80: 
81: - Identify near-duplicate requirements
82: - Mark lower-quality phrasing for consolidation
83: 
84: #### B. Ambiguity Detection
85: 
86: - Flag vague adjectives (fast, scalable, secure, intuitive, robust) lacking measurable criteria
87: - Flag unresolved placeholders (TODO, TKTK, ???, `<placeholder>`, etc.)
88: 
89: #### C. Underspecification
90: 
91: - Requirements with verbs but missing object or measurable outcome
92: - User stories missing acceptance criteria alignment
93: - Tasks referencing files or components not defined in spec/plan
94: 
95: #### D. Constitution Alignment
96: 
97: - Any requirement or plan element conflicting with a MUST principle
98: - Missing mandated sections or quality gates from constitution
99: 
100: #### E. Coverage Gaps
101: 
102: - Requirements with zero associated tasks
103: - Tasks with no mapped requirement/story
104: - Non-functional requirements not reflected in tasks (e.g., performance, security)
105: 
106: #### F. Inconsistency
107: 
108: - Terminology drift (same concept named differently across files)
109: - Data entities referenced in plan but absent in spec (or vice versa)
110: - Task ordering contradictions (e.g., integration tasks before foundational setup tasks without dependency note)
111: - Conflicting requirements (e.g., one requires Next.js while other specifies Vue)
112: 
113: ### 5. Severity Assignment
114: 
115: Use this heuristic to prioritize findings:
116: 
117: - **CRITICAL**: Violates constitution MUST, missing core spec artifact, or requirement with zero coverage that blocks baseline functionality
118: - **HIGH**: Duplicate or conflicting requirement, ambiguous security/performance attribute, untestable acceptance criterion
119: - **MEDIUM**: Terminology drift, missing non-functional task coverage, underspecified edge case
120: - **LOW**: Style/wording improvements, minor redundancy not affecting execution order
121: 
122: ### 6. Produce Compact Analysis Report
123: 
124: Output a Markdown report (no file writes) with the following structure:
125: 
126: ## Specification Analysis Report
127: 
128: | ID | Category | Severity | Location(s) | Summary | Recommendation |
129: |----|----------|----------|-------------|---------|----------------|
130: | A1 | Duplication | HIGH | spec.md:L120-134 | Two similar requirements ... | Merge phrasing; keep clearer version |
131: 
132: (Add one row per finding; generate stable IDs prefixed by category initial.)
133: 
134: **Coverage Summary Table:**
135: 
136: | Requirement Key | Has Task? | Task IDs | Notes |
137: |-----------------|-----------|----------|-------|
138: 
139: **Constitution Alignment Issues:** (if any)
140: 
141: **Unmapped Tasks:** (if any)
142: 
143: **Metrics:**
144: 
145: - Total Requirements
146: - Total Tasks
147: - Coverage % (requirements with >=1 task)
148: - Ambiguity Count
149: - Duplication Count
150: - Critical Issues Count
151: 
152: ### 7. Provide Next Actions
153: 
154: At end of report, output a concise Next Actions block:
155: 
156: - If CRITICAL issues exist: Recommend resolving before `/implement`
157: - If only LOW/MEDIUM: User may proceed, but provide improvement suggestions
158: - Provide explicit command suggestions: e.g., "Run /spec-kitty.specify with refinement", "Run /plan to adjust architecture", "Manually edit tasks.md to add coverage for 'performance-metrics'"
159: 
160: ### 8. Offer Remediation
161: 
162: Ask the user: "Would you like me to suggest concrete remediation edits for the top N issues?" (Do NOT apply them automatically.)
163: 
164: ## Operating Principles
165: 
166: ### Context Efficiency
167: 
168: - **Minimal high-signal tokens**: Focus on actionable findings, not exhaustive documentation
169: - **Progressive disclosure**: Load artifacts incrementally; don't dump all content into analysis
170: - **Token-efficient output**: Limit findings table to 50 rows; summarize overflow
171: - **Deterministic results**: Rerunning without changes should produce consistent IDs and counts
172: 
173: ### Analysis Guidelines
174: 
175: - **NEVER modify files** (this is read-only analysis)
176: - **NEVER hallucinate missing sections** (if absent, report them accurately)
177: - **Prioritize constitution violations** (these are always CRITICAL)
178: - **Use examples over exhaustive rules** (cite specific instances, not generic patterns)
179: - **Report zero issues gracefully** (emit success report with coverage statistics)
180: 
181: ## Context
182: 
183: {ARGS}
```

**Command File**
- Path: `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.checklist.md`
- References spec-kitty skill: Yes (`/spec-kitty.checklist`, `/spec-kitty.*`)
```markdown
1: ---
2: description: Generate a custom checklist for the current feature based on user requirements.
3: ---
4: 
5: ## Checklist Purpose: "Unit Tests for English"
6: 
7: **CRITICAL CONCEPT**: Checklists are **UNIT TESTS FOR REQUIREMENTS WRITING** - they validate the quality, clarity, and completeness of requirements in a given domain.
8: 
9: **NOT for verification/testing**:
10: - ❌ NOT "Verify the button clicks correctly"
11: - ❌ NOT "Test error handling works"
12: - ❌ NOT "Confirm the API returns 200"
13: - ❌ NOT checking if code/implementation matches the spec
14: 
15: **FOR requirements quality validation**:
16: - ✅ "Are visual hierarchy requirements defined for all card types?" (completeness)
17: - ✅ "Is 'prominent display' quantified with specific sizing/positioning?" (clarity)
18: - ✅ "Are hover state requirements consistent across all interactive elements?" (consistency)
19: - ✅ "Are accessibility requirements defined for keyboard navigation?" (coverage)
20: - ✅ "Does the spec define what happens when logo image fails to load?" (edge cases)
21: 
22: **Metaphor**: If your spec is code written in English, the checklist is its unit test suite. You're testing whether the requirements are well-written, complete, unambiguous, and ready for implementation - NOT whether the implementation works.
23: 
24: ## User Input
25: 
26: ```text
27: $ARGUMENTS
28: ```
29: 
30: You **MUST** consider the user input before proceeding (if not empty).
31: 
32: ## Execution Steps
33: 
34: 1. **Setup**: Run `{SCRIPT}` from repo root and parse JSON for FEATURE_DIR and AVAILABLE_DOCS list.
35:    - All file paths must be absolute.
36: 
37: 2. **Clarify intent (dynamic)**: Derive up to THREE initial contextual clarifying questions (no pre-baked catalog). They MUST:
38:    - Be generated from the user's phrasing + extracted signals from spec/plan/tasks
39:    - Only ask about information that materially changes checklist content
40:    - Be skipped individually if already unambiguous in `$ARGUMENTS`
41:    - Prefer precision over breadth
42: 
43:    Generation algorithm:
44:    1. Extract signals: feature domain keywords (e.g., auth, latency, UX, API), risk indicators ("critical", "must", "compliance"), stakeholder hints ("QA", "review", "security team"), and explicit deliverables ("a11y", "rollback", "contracts").
45:    2. Cluster signals into candidate focus areas (max 4) ranked by relevance.
46:    3. Identify probable audience & timing (author, reviewer, QA, release) if not explicit.
47:    4. Detect missing dimensions: scope breadth, depth/rigor, risk emphasis, exclusion boundaries, measurable acceptance criteria.
48:    5. Formulate questions chosen from these archetypes:
49:       - Scope refinement (e.g., "Should this include integration touchpoints with X and Y or stay limited to local module correctness?")
50:       - Risk prioritization (e.g., "Which of these potential risk areas should receive mandatory gating checks?")
51:       - Depth calibration (e.g., "Is this a lightweight pre-commit sanity list or a formal release gate?")
52:       - Audience framing (e.g., "Will this be used by the author only or peers during PR review?")
53:       - Boundary exclusion (e.g., "Should we explicitly exclude performance tuning items this round?")
54:       - Scenario class gap (e.g., "No recovery flows detected—are rollback / partial failure paths in scope?")
55: 
56:    Question formatting rules:
57:    - If presenting options, generate a compact table with columns: Option | Candidate | Why It Matters
58:    - Limit to A–E options maximum; omit table if a free-form answer is clearer
59:    - Never ask the user to restate what they already said
60:    - Avoid speculative categories (no hallucination). If uncertain, ask explicitly: "Confirm whether X belongs in scope."
61: 
62:    Defaults when interaction impossible:
63:    - Depth: Standard
64:    - Audience: Reviewer (PR) if code-related; Author otherwise
65:    - Focus: Top 2 relevance clusters
66: 
67:    Output the questions (label Q1/Q2/Q3). After answers: if ≥2 scenario classes (Alternate / Exception / Recovery / Non-Functional domain) remain unclear, you MAY ask up to TWO more targeted follow‑ups (Q4/Q5) with a one-line justification each (e.g., "Unresolved recovery path risk"). Do not exceed five total questions. Skip escalation if user explicitly declines more.
68: 
69: 3. **Understand user request**: Combine `$ARGUMENTS` + clarifying answers:
70:    - Derive checklist theme (e.g., security, review, deploy, ux)
71:    - Consolidate explicit must-have items mentioned by user
72:    - Map focus selections to category scaffolding
73:    - Infer any missing context from spec/plan/tasks (do NOT hallucinate)
74: 
75: 4. **Load feature context**: Read from FEATURE_DIR:
76:    - spec.md: Feature requirements and scope
77:    - plan.md (if exists): Technical details, dependencies
78:    - tasks.md (if exists): Implementation tasks
79:    
80:    **Context Loading Strategy**:
81:    - Load only necessary portions relevant to active focus areas (avoid full-file dumping)
82:    - Prefer summarizing long sections into concise scenario/requirement bullets
83:    - Use progressive disclosure: add follow-on retrieval only if gaps detected
84:    - If source docs are large, generate interim summary items instead of embedding raw text
85: 
86: 5. **Generate checklist** - Create "Unit Tests for Requirements":
87:    - Create `FEATURE_DIR/checklists/` directory if it doesn't exist
88:    - Generate unique checklist filename:
89:      - Use short, descriptive name based on domain (e.g., `ux.md`, `api.md`, `security.md`)
90:      - Format: `[domain].md` 
91:      - If file exists, append to existing file
92:    - Number items sequentially starting from CHK001
93:    - Each `/spec-kitty.checklist` run creates a NEW file (never overwrites existing checklists)
94: 
95:    **CORE PRINCIPLE - Test the Requirements, Not the Implementation**:
96:    Every checklist item MUST evaluate the REQUIREMENTS THEMSELVES for:
97:    - **Completeness**: Are all necessary requirements present?
98:    - **Clarity**: Are requirements unambiguous and specific?
99:    - **Consistency**: Do requirements align with each other?
100:    - **Measurability**: Can requirements be objectively verified?
101:    - **Coverage**: Are all scenarios/edge cases addressed?
102:    
103:    **Category Structure** - Group items by requirement quality dimensions:
104:    - **Requirement Completeness** (Are all necessary requirements documented?)
105:    - **Requirement Clarity** (Are requirements specific and unambiguous?)
106:    - **Requirement Consistency** (Do requirements align without conflicts?)
107:    - **Acceptance Criteria Quality** (Are success criteria measurable?)
108:    - **Scenario Coverage** (Are all flows/cases addressed?)
109:    - **Edge Case Coverage** (Are boundary conditions defined?)
110:    - **Non-Functional Requirements** (Performance, Security, Accessibility, etc. - are they specified?)
111:    - **Dependencies & Assumptions** (Are they documented and validated?)
112:    - **Ambiguities & Conflicts** (What needs clarification?)
113:    
114:    **HOW TO WRITE CHECKLIST ITEMS - "Unit Tests for English"**:
115:    
116:    ❌ **WRONG** (Testing implementation):
117:    - "Verify landing page displays 3 episode cards"
118:    - "Test hover states work on desktop"
119:    - "Confirm logo click navigates home"
120:    
121:    ✅ **CORRECT** (Testing requirements quality):
122:    - "Are the exact number and layout of featured episodes specified?" [Completeness]
123:    - "Is 'prominent display' quantified with specific sizing/positioning?" [Clarity]
124:    - "Are hover state requirements consistent across all interactive elements?" [Consistency]
125:    - "Are keyboard navigation requirements defined for all interactive UI?" [Coverage]
126:    - "Is the fallback behavior specified when logo image fails to load?" [Edge Cases]
127:    - "Are loading states defined for asynchronous episode data?" [Completeness]
128:    - "Does the spec define visual hierarchy for competing UI elements?" [Clarity]
129:    
130:    **ITEM STRUCTURE**:
131:    Each item should follow this pattern:
132:    - Question format asking about requirement quality
133:    - Focus on what's WRITTEN (or not written) in the spec/plan
134:    - Include quality dimension in brackets [Completeness/Clarity/Consistency/etc.]
135:    - Reference spec section `[Spec §X.Y]` when checking existing requirements
136:    - Use `[Gap]` marker when checking for missing requirements
137:    
138:    **EXAMPLES BY QUALITY DIMENSION**:
139:    
140:    Completeness:
141:    - "Are error handling requirements defined for all API failure modes? [Gap]"
142:    - "Are accessibility requirements specified for all interactive elements? [Completeness]"
143:    - "Are mobile breakpoint requirements defined for responsive layouts? [Gap]"
144:    
145:    Clarity:
146:    - "Is 'fast loading' quantified with specific timing thresholds? [Clarity, Spec §NFR-2]"
147:    - "Are 'related episodes' selection criteria explicitly defined? [Clarity, Spec §FR-5]"
148:    - "Is 'prominent' defined with measurable visual properties? [Ambiguity, Spec §FR-4]"
149:    
150:    Consistency:
151:    - "Do navigation requirements align across all pages? [Consistency, Spec §FR-10]"
152:    - "Are card component requirements consistent between landing and detail pages? [Consistency]"
153:    
154:    Coverage:
155:    - "Are requirements defined for zero-state scenarios (no episodes)? [Coverage, Edge Case]"
156:    - "Are concurrent user interaction scenarios addressed? [Coverage, Gap]"
157:    - "Are requirements specified for partial data loading failures? [Coverage, Exception Flow]"
158:    
159:    Measurability:
160:    - "Are visual hierarchy requirements measurable/testable? [Acceptance Criteria, Spec §FR-1]"
161:    - "Can 'balanced visual weight' be objectively verified? [Measurability, Spec §FR-2]"
162: 
163:    **Scenario Classification & Coverage** (Requirements Quality Focus):
164:    - Check if requirements exist for: Primary, Alternate, Exception/Error, Recovery, Non-Functional scenarios
165:    - For each scenario class, ask: "Are [scenario type] requirements complete, clear, and consistent?"
166:    - If scenario class missing: "Are [scenario type] requirements intentionally excluded or missing? [Gap]"
167:    - Include resilience/rollback when state mutation occurs: "Are rollback requirements defined for migration failures? [Gap]"
168: 
169:    **Traceability Requirements**:
170:    - MINIMUM: ≥80% of items MUST include at least one traceability reference
171:    - Each item should reference: spec section `[Spec §X.Y]`, or use markers: `[Gap]`, `[Ambiguity]`, `[Conflict]`, `[Assumption]`
172:    - If no ID system exists: "Is a requirement & acceptance criteria ID scheme established? [Traceability]"
173: 
174:    **Surface & Resolve Issues** (Requirements Quality Problems):
175:    Ask questions about the requirements themselves:
176:    - Ambiguities: "Is the term 'fast' quantified with specific metrics? [Ambiguity, Spec §NFR-1]"
177:    - Conflicts: "Do navigation requirements conflict between §FR-10 and §FR-10a? [Conflict]"
178:    - Assumptions: "Is the assumption of 'always available podcast API' validated? [Assumption]"
179:    - Dependencies: "Are external podcast API requirements documented? [Dependency, Gap]"
180:    - Missing definitions: "Is 'visual hierarchy' defined with measurable criteria? [Gap]"
181: 
182:    **Content Consolidation**:
183:    - Soft cap: If raw candidate items > 40, prioritize by risk/impact
184:    - Merge near-duplicates checking the same requirement aspect
185:    - If >5 low-impact edge cases, create one item: "Are edge cases X, Y, Z addressed in requirements? [Coverage]"
186: 
187:    **🚫 ABSOLUTELY PROHIBITED** - These make it an implementation test, not a requirements test:
188:    - ❌ Any item starting with "Verify", "Test", "Confirm", "Check" + implementation behavior
189:    - ❌ References to code execution, user actions, system behavior
190:    - ❌ "Displays correctly", "works properly", "functions as expected"
191:    - ❌ "Click", "navigate", "render", "load", "execute"
192:    - ❌ Test cases, test plans, QA procedures
193:    - ❌ Implementation details (frameworks, APIs, algorithms)
194:    
195:    **✅ REQUIRED PATTERNS** - These test requirements quality:
196:    - ✅ "Are [requirement type] defined/specified/documented for [scenario]?"
197:    - ✅ "Is [vague term] quantified/clarified with specific criteria?"
198:    - ✅ "Are requirements consistent between [section A] and [section B]?"
199:    - ✅ "Can [requirement] be objectively measured/verified?"
200:    - ✅ "Are [edge cases/scenarios] addressed in requirements?"
201:    - ✅ "Does the spec define [missing aspect]?"
202: 
203: 6. **Structure Reference**: Generate the checklist following the canonical template in `templates/checklist-template.md` for title, meta section, category headings, and ID formatting. If template is unavailable, use: H1 title, purpose/created meta lines, `##` category sections containing `- [ ] CHK### <requirement item>` lines with globally incrementing IDs starting at CHK001.
204: 
205: 7. **Report**: Output full path to created checklist, item count, and remind user that each run creates a new file. Summarize:
206:    - Focus areas selected
207:    - Depth level
208:    - Actor/timing
209:    - Any explicit user-specified must-have items incorporated
210: 
211: **Important**: Each `/spec-kitty.checklist` command invocation creates a checklist file using short, descriptive names unless file already exists. This allows:
212: 
213: - Multiple checklists of different types (e.g., `ux.md`, `test.md`, `security.md`)
214: - Simple, memorable filenames that indicate checklist purpose
215: - Easy identification and navigation in the `checklists/` folder
216: 
217: To avoid clutter, use descriptive types and clean up obsolete checklists when done.
218: 
219: ## Example Checklist Types & Sample Items
220: 
221: **UX Requirements Quality:** `ux.md`
222: 
223: Sample items (testing the requirements, NOT the implementation):
224: - "Are visual hierarchy requirements defined with measurable criteria? [Clarity, Spec §FR-1]"
225: - "Is the number and positioning of UI elements explicitly specified? [Completeness, Spec §FR-1]"
226: - "Are interaction state requirements (hover, focus, active) consistently defined? [Consistency]"
227: - "Are accessibility requirements specified for all interactive elements? [Coverage, Gap]"
228: - "Is fallback behavior defined when images fail to load? [Edge Case, Gap]"
229: - "Can 'prominent display' be objectively measured? [Measurability, Spec §FR-4]"
230: 
231: **API Requirements Quality:** `api.md`
232: 
233: Sample items:
234: - "Are error response formats specified for all failure scenarios? [Completeness]"
235: - "Are rate limiting requirements quantified with specific thresholds? [Clarity]"
236: - "Are authentication requirements consistent across all endpoints? [Consistency]"
237: - "Are retry/timeout requirements defined for external dependencies? [Coverage, Gap]"
238: - "Is versioning strategy documented in requirements? [Gap]"
239: 
240: **Performance Requirements Quality:** `performance.md`
241: 
242: Sample items:
243: - "Are performance requirements quantified with specific metrics? [Clarity]"
244: - "Are performance targets defined for all critical user journeys? [Coverage]"
245: - "Are performance requirements under different load conditions specified? [Completeness]"
246: - "Can performance requirements be objectively measured? [Measurability]"
247: - "Are degradation requirements defined for high-load scenarios? [Edge Case, Gap]"
248: 
249: **Security Requirements Quality:** `security.md`
250: 
251: Sample items:
252: - "Are authentication requirements specified for all protected resources? [Coverage]"
253: - "Are data protection requirements defined for sensitive information? [Completeness]"
254: - "Is the threat model documented and requirements aligned to it? [Traceability]"
255: - "Are security requirements consistent with compliance obligations? [Consistency]"
256: - "Are security failure/breach response requirements defined? [Gap, Exception Flow]"
257: 
258: ## Anti-Examples: What NOT To Do
259: 
260: **❌ WRONG - These test implementation, not requirements:**
261: 
262: ```markdown
263: - [ ] CHK001 - Verify landing page displays 3 episode cards [Spec §FR-001]
264: - [ ] CHK002 - Test hover states work correctly on desktop [Spec §FR-003]
265: - [ ] CHK003 - Confirm logo click navigates to home page [Spec §FR-010]
266: - [ ] CHK004 - Check that related episodes section shows 3-5 items [Spec §FR-005]
267: ```
268: 
269: **✅ CORRECT - These test requirements quality:**
270: 
271: ```markdown
272: - [ ] CHK001 - Are the number and layout of featured episodes explicitly specified? [Completeness, Spec §FR-001]
273: - [ ] CHK002 - Are hover state requirements consistently defined for all interactive elements? [Consistency, Spec §FR-003]
274: - [ ] CHK003 - Are navigation requirements clear for all clickable brand elements? [Clarity, Spec §FR-010]
275: - [ ] CHK004 - Is the selection criteria for related episodes documented? [Gap, Spec §FR-005]
276: - [ ] CHK005 - Are loading state requirements defined for asynchronous episode data? [Gap]
277: - [ ] CHK006 - Can "visual hierarchy" requirements be objectively measured? [Measurability, Spec §FR-001]
278: ```
279: 
280: **Key Differences:**
281: - Wrong: Tests if the system works correctly
282: - Correct: Tests if the requirements are written correctly
283: - Wrong: Verification of behavior
284: - Correct: Validation of requirement quality
285: - Wrong: "Does it do X?" 
286: - Correct: "Is X clearly specified?"
```

This output is too large for a single response. Do you want me to continue with the next batch (recommended: `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.clarify.md`, `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.constitution.md`, `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.dashboard.md`, `/home/kas/dev/kasmos/.opencode/commands/spec-kitty.implement.md`)? If you want a different order (e.g., SKILL.md or config profile commands first), tell me which and I’ll switch.
</task_result>
```

---

## User

also, add to AGENTS.md to instruct agents to automatically load these below skills based on the following strings being found in prompts, filepaths or content you read:
1. "kitty" or "kittify" - load spec-kitty skill (.opencode/skills/spec-kitty/SKILL.md)
2. "tmux" or "pane" - load tmux-orchestration skill (.opencode/skills/tmux-orchestration/SKILL.md)
3. "tui" or "worker" or "app" or "launch" or "settings" or "keybind" - load tui-design skill (.opencode/skills/tui-design/SKILL.md)

---

## Assistant (Manager · claude-opus-4-6 · 8.9s)



_Thinking:_

The user wants two things:

1. Update all spec-kitty commands (in `.opencode/commands/` and the ones installed by `kasmos setup`) to explicitly tell the agent to load the spec-kitty skill
2. Add instructions to `AGENTS.md` for automatic skill loading based on keyword triggers

Let me first get the full picture of all the files I need to modify. I need to:
- See all spec-kitty command files in `.opencode/commands/`
- See the command files in `config/profiles/kasmos/commands/`
- See `AGENTS.md`
- See the setup code that installs commands
- Understand the skill loading mechanism


- Now I'm going to explore all these files to understand the current structure and what needs to be updated.

**Tool: todowrite**

**Input:**
```json
{
  "todos": [
    {
      "content": "Explore all spec-kitty command files in .opencode/commands/ and config/profiles/kasmos/commands/",
      "status": "in_progress",
      "priority": "high"
    },
    {
      "content": "Explore kasmos setup code for command installation",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Add skill-loading instruction to all spec-kitty commands in .opencode/commands/",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Add skill-loading instruction to all spec-kitty commands in config/profiles/kasmos/commands/",
      "status": "pending",
      "priority": "high"
    },
    {
      "content": "Update AGENTS.md with automatic skill loading rules based on keyword triggers",
      "status": "pending",
      "priority": "high"
    }
  ]
}
```

**Output:**
```
[
  {
    "content": "Explore all spec-kitty command files in .opencode/commands/ and config/profiles/kasmos/commands/",
    "status": "in_progress",
    "priority": "high"
  },
  {
    "content": "Explore kasmos setup code for command installation",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Add skill-loading instruction to all spec-kitty commands in .opencode/commands/",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Add skill-loading instruction to all spec-kitty commands in config/profiles/kasmos/commands/",
    "status": "pending",
    "priority": "high"
  },
  {
    "content": "Update AGENTS.md with automatic skill loading rules based on keyword triggers",
    "status": "pending",
    "priority": "high"
  }
]
```

---

## Assistant (Manager · claude-opus-4-6 · 5.5s)

**Tool: glob**

**Input:**
```json
{
  "pattern": ".opencode/commands/spec-kitty*.md"
}
```

**Output:**
```
No files found
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "config/profiles/kasmos/commands/spec-kitty*.md"
}
```

**Output:**
```
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.review.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.merge.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.accept.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.tasks.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.status.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.specify.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.research.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.plan.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.implement.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.dashboard.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.constitution.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.clarify.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.checklist.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.analyze.md
```

**Tool: glob**

**Input:**
```json
{
  "pattern": "config/profiles/kasmos/commands/*.md"
}
```

**Output:**
```
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.review.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.merge.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.accept.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.tasks.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.status.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.specify.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.research.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.plan.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.implement.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.dashboard.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.constitution.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.clarify.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.checklist.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/spec-kitty.analyze.md
/home/kas/dev/kasmos/config/profiles/kasmos/commands/kas.verify.md
```

**Tool: glob**

**Input:**
```json
{
  "pattern": ".opencode/commands/*.md"
}
```

**Output:**
```
No files found
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/kas/dev/kasmos/AGENTS.md"
}
```

**Output:**
```
<path>/home/kas/dev/kasmos/AGENTS.md</path>
<type>file</type>
<content>1: # AGENTS.md
2: 
3: ## Startup checklist
4: - Read `README.md` for project overview.
5: - Read `.kittify/memory/` for project constitution, architecture knowledge, and workflow intelligence.
6: - Check `kitty-specs/` for feature specifications.
7: - This is a Go module. Source lives at the repository root.
8: - Primary binary: `cmd/kasmos/` - the TUI agent orchestrator.
9: 
10: ## Repository layout
11: - `cmd/kasmos/`: Entry point (main.go)
12: - `internal/tui/`: bubbletea TUI (model, update, view, styles, keys)
13: - `internal/worker/`: Worker backend interface, subprocess backend, output buffer
14: - `internal/task/`: Task source adapters (spec-kitty, GSD, ad-hoc)
15: - `internal/persist/`: Session persistence (JSON)
16: - `internal/setup/`: `kasmos setup` subcommand (agent scaffolding, dep validation)
17: - `kitty-specs/`: Feature specifications (spec-kitty)
18: - `design-artifacts/`: TUI visual design (mockups, layout, styles, keybinds)
19: - `.kittify/memory/`: Persistent project memory (constitution, architecture, workflow learnings)
20: - `.kittify/`: spec-kitty project configuration, scripts, missions
21: 
22: ## Build / run commands
23: - Build: `go build ./cmd/kasmos`
24: - Run: `go run ./cmd/kasmos`
25: - Test: `go test ./...`
26: - Test (integration): `KASMOS_INTEGRATION=1 go test ./...`
27: - Lint: `golangci-lint run`
28: 
29: ## Code style (Go)
30: - Follow standard Go conventions (gofmt, go vet)
31: - Use `internal/` for non-exported packages
32: - Prefer explicit error handling with `fmt.Errorf` wrapping
33: - Use table-driven tests
34: - Follow standard Go naming: camelCase unexported, PascalCase exported
35: - Keep packages small and focused
36: 
37: ## External tools
38: - `opencode`: AI coding agent harness (workers spawned via `opencode run`)
39: - `spec-kitty`: Feature specification tool
40: - `git`: Version control
41: 
42: ## Agent harness: OpenCode only
43: 
44: kasmos uses **OpenCode** as the sole agent harness for spawning worker agents. This is a hard rule:
45: 
46: - Workers are spawned via `opencode run --agent <role> "prompt"`.
47: - **Never invoke a model-specific CLI** (e.g., `claude`, `gemini`, `aider`) directly. OpenCode is the abstraction layer.
48: - kasmos is **model-agnostic**. The model running behind OpenCode is configured in OpenCode's own config, not in kasmos.
49: - Session continuation uses `opencode run --continue -s <session_id> "follow-up"`.
50: 
51: ## Persistent memory
52: 
53: When you discover something significant about the codebase architecture, runtime behavior, or integration quirks, record it in `.kittify/memory/`.
54: 
55: - `constitution.md`: Project technical standards and governance (do not modify without discussion).
56: - `architecture.md`: Codebase structure, type locations, subsystem interactions, known issues.
57: - `workflow-intelligence.md`: Lessons from the spec-kitty planning lifecycle.
58: 

(End of file - total 58 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/kas/dev/kasmos/internal/setup"
}
```

**Output:**
```
<path>/home/kas/dev/kasmos/internal/setup</path>
<type>directory</type>
<entries>
agents.go
deps_test.go
deps.go
setup_test.go
setup.go

(5 entries)
</entries>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/kas/dev/kasmos/.opencode/skills/spec-kitty/SKILL.md",
  "limit": 30
}
```

**Output:**
```
<path>/home/kas/dev/kasmos/.opencode/skills/spec-kitty/SKILL.md</path>
<type>file</type>
<content>1: ---
2: name: spec-kitty
3: description: >
4:   Use this skill when working in a spec-kitty project — any repo initialized with
5:   `spec-kitty init`. Covers the full lifecycle: specify → clarify → plan → tasks →
6:   implement → review → accept → merge. Includes git worktree management, parallel
7:   work package execution, optional steps (clarify, analyze, checklist), the kanban
8:   dashboard, and the `spec-kitty agent` programmatic API. Trigger on any mention of
9:   spec-kitty, spec-driven development, SDD workflow, `/spec-kitty.*` slash commands,
10:   work packages (WP01, WP02), or kitty-specs.
11: ---
12: 
13: # Spec-Kitty: Agent Operator Guide
14: 
15: You are an AI coding agent operating inside a spec-kitty project. This skill teaches
16: you how to navigate the full spec-driven development lifecycle from an agent's
17: perspective — what to run, when, and why.
18: 
19: ---
20: 
21: ## Core Concepts
22: 
23: **Project** — One git repo, initialized once with `spec-kitty init`. Contains all
24: missions, features, and `.kittify/` automation. Never re-initialize.
25: 
26: **Feature** — A single unit of work (e.g., "Add user auth"). Created with
27: `/spec-kitty.specify`. Has its own spec, plan, tasks, and work packages under
28: `kitty-specs/<NNN-feature-name>/`.
29: 
30: **Work Package (WP)** — A subset of tasks within a feature (WP01, WP02, etc.).

(File has more lines. Use 'offset' parameter to read beyond line 30)
</content>
```

---

## Assistant (Manager · claude-opus-4-6 · 48.0s)

_Thinking:_

Now I have a good picture. The spec-kitty commands are all in `config/profiles/kasmos/commands/` (14 spec-kitty commands + 1 kas.verify). There are NO commands in `.opencode/commands/` currently.

Let me now:
1. Read the setup code to understand how commands get installed
2. Read all the spec-kitty command files in `config/profiles/kasmos/commands/`

Let me read the setup.go and agents.go to understand installation, and read all the command files.

Good — so the spec-kitty commands live in `config/profiles/kasmos/commands/`, not in `.opencode/commands/`. Let me now read all the command files and the setup code to understand installation.

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/kas/dev/kasmos/internal/setup/setup.go"
}
```

**Output:**
```
<path>/home/kas/dev/kasmos/internal/setup/setup.go</path>
<type>file</type>
<content>1: package setup
2: 
3: import (
4: 	"fmt"
5: 	"os"
6: 	"path/filepath"
7: )
8: 
9: func Run() error {
10: 	fmt.Println("kasmos setup")
11: 	fmt.Println()
12: 
13: 	fmt.Println("Checking dependencies...")
14: 	results := CheckDependencies()
15: 	allFound := true
16: 	for _, r := range results {
17: 		if r.Found {
18: 			fmt.Printf("  [OK]      %-12s found (%s)\n", r.Name, r.Path)
19: 			continue
20: 		}
21: 
22: 		fmt.Printf("  [MISSING] %-12s NOT FOUND\n", r.Name)
23: 		if r.InstallHint != "" {
24: 			fmt.Printf("    Install: %s\n", r.InstallHint)
25: 		}
26: 		if r.Required {
27: 			allFound = false
28: 		}
29: 	}
30: 	fmt.Println()
31: 
32: 	if !allFound {
33: 		return fmt.Errorf("required dependencies missing")
34: 	}
35: 
36: 	root, err := findProjectRoot()
37: 	if err != nil {
38: 		return fmt.Errorf("find project root: %w", err)
39: 	}
40: 
41: 	fmt.Println("Scaffolding agent definitions...")
42: 	created, skipped, err := WriteAgentDefinitions(root)
43: 	if err != nil {
44: 		return fmt.Errorf("write agents: %w", err)
45: 	}
46: 	fmt.Printf("  %d created, %d skipped (already exist)\n", created, skipped)
47: 	fmt.Println()
48: 	fmt.Println("Setup complete!")
49: 
50: 	return nil
51: }
52: 
53: func findProjectRoot() (string, error) {
54: 	dir, err := os.Getwd()
55: 	if err != nil {
56: 		return "", err
57: 	}
58: 
59: 	for {
60: 		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
61: 			return dir, nil
62: 		}
63: 		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
64: 			return dir, nil
65: 		}
66: 
67: 		parent := filepath.Dir(dir)
68: 		if parent == dir {
69: 			return "", fmt.Errorf("no go.mod or .git found in parent directories")
70: 		}
71: 		dir = parent
72: 	}
73: }
74: 

(End of file - total 74 lines)
</content>
```

**Tool: read**

**Input:**
```json
{
  "filePath": "/home/kas/dev/kasmos/internal/setup/agents.go"
}
```

**Output:**
```
<path>/home/kas/dev/kasmos/internal/setup/agents.go</path>
<type>file</type>
<content>1: package setup
2: 
3: import (
4: 	"fmt"
5: 	"os"
6: 	"path/filepath"
7: )
8: 
9: type AgentDef struct {
10: 	Filename string
11: 	Content  string
12: }
13: 
14: var agentDefinitions = []AgentDef{
15: 	{Filename: "planner.md", Content: plannerTemplate},
16: 	{Filename: "coder.md", Content: coderTemplate},
17: 	{Filename: "reviewer.md", Content: reviewerTemplate},
18: 	{Filename: "release.md", Content: releaseTemplate},
19: }
20: 
21: func WriteAgentDefinitions(dir string) (created, skipped int, err error) {
22: 	agentDir := filepath.Join(dir, ".opencode", "agents")
23: 	if err := os.MkdirAll(agentDir, 0o755); err != nil {
24: 		return 0, 0, fmt.Errorf("create agent dir: %w", err)
25: 	}
26: 
27: 	for _, agent := range agentDefinitions {
28: 		path := filepath.Join(agentDir, agent.Filename)
29: 		if _, err := os.Stat(path); err == nil {
30: 			skipped++
31: 			continue
32: 		} else if !os.IsNotExist(err) {
33: 			return created, skipped, fmt.Errorf("stat %s: %w", agent.Filename, err)
34: 		}
35: 
36: 		if err := os.WriteFile(path, []byte(agent.Content), 0o644); err != nil {
37: 			return created, skipped, fmt.Errorf("write %s: %w", agent.Filename, err)
38: 		}
39: 		created++
40: 	}
41: 
42: 	return created, skipped, nil
43: }
44: 
45: const plannerTemplate = `---
46: name: planner
47: description: Research and planning agent for work package preparation
48: ---
49: 
50: # Planner Agent
51: 
52: ## Role
53: - Analyze requirements, constraints, and architecture before coding starts.
54: - Produce implementation plans, milestones, and risk lists.
55: - Keep scope aligned with the selected work package.
56: 
57: ## Capabilities
58: - Read repository files and related documentation.
59: - Compare options and recommend a concrete path.
60: - Define acceptance checks and verification steps.
61: 
62: ## Constraints
63: - Read-only filesystem behavior: do not edit source files.
64: - Do not run destructive commands or change git history.
65: - Do not implement code changes directly.
66: 
67: ## Deliverables
68: 1. Problem statement with assumptions and unknowns.
69: 2. Step-by-step implementation plan.
70: 3. Test and validation strategy.
71: 4. Risks, dependencies, and handoff notes for coder.
72: `
73: 
74: const coderTemplate = `---
75: name: coder
76: description: Implementation agent for building and testing changes
77: ---
78: 
79: # Coder Agent
80: 
81: ## Role
82: - Implement approved plans with robust, production-ready code.
83: - Keep changes scoped, readable, and maintainable.
84: - Update or add tests for new behavior.
85: 
86: ## Capabilities
87: - Full tool access for editing files and running commands.
88: - Build, test, and verify code using project workflows.
89: - Refactor related code when needed to keep quality high.
90: 
91: ## Constraints
92: - Follow existing repository conventions and architecture.
93: - Fail fast on invalid states and surface clear errors.
94: - Avoid unrelated edits and never commit secrets.
95: 
96: ## Done Criteria
97: 1. Code compiles and tests pass for touched areas.
98: 2. New behavior is covered by tests.
99: 3. Notes include what changed and why.
100: 4. Work is ready for reviewer handoff.
101: `
102: 
103: const reviewerTemplate = `---
104: name: reviewer
105: description: Review agent focused on correctness, security, and quality
106: ---
107: 
108: # Reviewer Agent
109: 
110: ## Role
111: - Audit implementation for correctness and hidden regressions.
112: - Verify behavior against requirements and acceptance criteria.
113: - Confirm code clarity, maintainability, and safety.
114: 
115: ## Capabilities
116: - Read repository files, diffs, and test output.
117: - Run non-destructive validation commands and test suites.
118: - Identify edge cases and missing coverage.
119: 
120: ## Constraints
121: - Read-only review posture: do not modify source files.
122: - Block approval when critical issues are unresolved.
123: - Prioritize correctness, security, and reliability over speed.
124: 
125: ## Review Output
126: 1. Pass/fail recommendation with rationale.
127: 2. Findings grouped by severity.
128: 3. Required fixes before merge.
129: 4. Optional improvements for follow-up work.
130: `
131: 
132: const releaseTemplate = `---
133: name: release
134: description: Finalization agent for merge, cleanup, and handoff
135: ---
136: 
137: # Release Agent
138: 
139: ## Role
140: - Finalize approved work and prepare release-ready state.
141: - Coordinate branch hygiene, merge steps, and closure tasks.
142: - Ensure final documentation and notes are complete.
143: 
144: ## Capabilities
145: - Run final verification commands and status checks.
146: - Perform non-destructive git operations for integration.
147: - Prepare concise release notes and completion summary.
148: 
149: ## Constraints
150: - Respect repository policies and protected branch rules.
151: - Avoid force pushes and unsafe history rewriting.
152: - Stop and report if unresolved blockers remain.
153: 
154: ## Final Checklist
155: 1. Review findings are resolved.
156: 2. Required tests and checks are green.
157: 3. Merge/finalization steps are documented.
158: 4. Cleanup and follow-up actions are recorded.
159: `
160: 

(End of file - total 160 lines)
</content>
```

**Tool: task**

**Input:**
```json
{
  "description": "Read all spec-kitty commands",
  "prompt": "Read the FULL contents of every file in /home/kas/dev/kasmos/config/profiles/kasmos/commands/ and return them to me. There are 15 files. I need the complete contents of each one, especially the YAML frontmatter at the top (the --- delimited block) and the first few sections of each file. For each file, report:\n1. Full filename\n2. The complete YAML frontmatter block\n3. Whether the file already contains any mention of \"skill\" or \"SKILL.md\" or \"spec-kitty skill\"\n4. The first 20 lines of the file\n\nReturn ALL of this information.",
  "subagent_type": "explore"
}
```

**Error:**
```
Tool execution aborted
```

---

