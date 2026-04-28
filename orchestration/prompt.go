package orchestration

import (
	"fmt"
	"strings"

	"github.com/kastheco/kasmos/config/taskparser"
)

// compactFailuresOnlyGoTestCmd runs the full test suite and strips passing lines so only
// failures reach the agent's context. Shell-portable: works in both bash and zsh.
const compactFailuresOnlyGoTestCmd = `tmp=$(mktemp); test_status=0; go test ./... >"$tmp" 2>&1 || test_status=$?; rg -v '^(ok\b|\?\s.*\[no test files\]|PASS$)' "$tmp" || true; rm -f "$tmp"; if [ "$test_status" -eq 0 ]; then echo 'tests passed'; else echo "tests failed (exit $test_status)"; (exit "$test_status"); fi`

// verboseFailuresOnlyGoTestCmd runs a scoped verbose test and strips all passing-line noise
// (RUN/PAUSE/CONT/PASS/SKIP) so only failure details reach the agent's context. Shell-portable.
const verboseFailuresOnlyGoTestCmd = `tmp=$(mktemp); test_status=0; go test ./pkg/... -run Test<Name> -v >"$tmp" 2>&1 || test_status=$?; rg -v '^(=== (RUN|PAUSE|CONT|NAME)|--- (PASS|SKIP)|PASS$|ok\b|\?\s.*\[no test files\])' "$tmp" || true; rm -f "$tmp"; if [ "$test_status" -eq 0 ]; then echo 'tests passed'; else echo "tests failed (exit $test_status)"; (exit "$test_status"); fi`

// BuildTaskPrompt constructs the prompt for a single task instance.
func BuildTaskPrompt(planFile string, plan *taskparser.Plan, task taskparser.Task, waveNumber, totalWaves, peerCount int, project string, meta *TaskMeta) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Implement Task %d: %s\n\n", task.Number, task.Title))

	// Inline coder rules — avoids the context cost of loading the kasmos-coder skill
	sb.WriteString("## Rules\n\n")
	sb.WriteString("- Implement ONLY this task. Do not modify files outside your scope.\n")
	sb.WriteString("- Do NOT load agent skills — rules are inlined here.\n")
	sb.WriteString("- Use `rg` (not grep), `sd` (not sed), `fd` (not find), `comby`/`ast-grep` for structural changes.\n")
	sb.WriteString("- Run scoped tests before committing: `" + verboseFailuresOnlyGoTestCmd + "`\n")
	sb.WriteString("- Verify build: `go build ./...`\n")
	sb.WriteString("- Commit: `git add <specific-files> && git commit -m \"feat(task-N): description\"`\n")
	sb.WriteString(fmt.Sprintf("- When done: signal completion with MCP `signal_create` (signal_type: \"implement-task-finished\", plan_file: \"%s\", project: \"%s\", payload: \"{\\\"wave_number\\\":%d,\\\"task_number\\\":%d}\"). Then stop.\n\n",
		planFile, project, waveNumber, task.Number))

	// Plan context
	header := plan.HeaderContext()
	if header != "" {
		sb.WriteString("## Plan Context\n\n")
		sb.WriteString(header)
		sb.WriteString("\n")
	}

	// Wave context
	sb.WriteString(fmt.Sprintf("## Wave %d of %d\n\n", waveNumber, totalWaves))

	// Parallel awareness — only for multi-task waves
	if peerCount > 1 {
		sb.WriteString(fmt.Sprintf("## Parallel Execution\n\n"))
		sb.WriteString(fmt.Sprintf("You are Task %d of %d in Wave %d. %d other agents are working in parallel on this same worktree.\n\n",
			task.Number, peerCount, waveNumber, peerCount-1))

		sb.WriteString("Your assigned files are listed in the Task Instructions below. Prioritize those files. ")
		sb.WriteString("If you must touch a shared file (go.mod, go.sum, imports), make minimal surgical changes - ")
		sb.WriteString("do not reorganize, reformat, or refactor anything outside your task scope.\n\n")

		sb.WriteString("CRITICAL - shared worktree rules:\n")
		sb.WriteString("- NEVER run `git add .` or `git add -A` - you will commit other agents' in-progress work\n")
		sb.WriteString("- NEVER run `git stash` or `git reset` - you will destroy sibling agents' changes\n")
		sb.WriteString("- NEVER run `git checkout -- <file>` on files you didn't modify - you will revert a sibling's edits\n")
		sb.WriteString("- NEVER run formatters/linters across the whole project (e.g. `go fmt ./...`) - scope them to your files only\n")
		sb.WriteString("- NEVER try to fix test failures in files outside your task - they may be caused by incomplete parallel work\n")
		sb.WriteString("If you encounter a build failure caused by missing types, functions, or interfaces that your task ")
		sb.WriteString("imports from a package being modified by a sibling agent: this is an import dependency that should have ")
		sb.WriteString("been in a separate wave. Do NOT stub, mock, or work around it. Commit whatever work you have completed ")
		sb.WriteString("so far, report the dependency in your commit message (e.g. 'partial: blocked on task N types'), and stop.\n\n")
		sb.WriteString("- DO `git add` only the specific files you changed\n")
		sb.WriteString("- DO commit frequently with your task number in the message\n")
		sb.WriteString("- DO expect untracked files and uncommitted changes that are not yours - ignore them\n\n")
	}

	// Task body
	sb.WriteString("## Task Instructions\n\n")
	sb.WriteString(task.Body)
	sb.WriteString("\n")

	if meta != nil && len(meta.VerifyChecks) > 0 {
		sb.WriteString("\n## Verification Commands\n\n")
		for _, check := range meta.VerifyChecks {
			sb.WriteString("- `" + check + "`\n")
		}
	}

	return sb.String()
}

// BuildBlueprintSkipPrompt builds the prompt for a single coder agent that must
// implement all tasks in a small plan sequentially. Used when the plan's task
// count is at or below the blueprint_skip_threshold so wave orchestration is skipped.
// The agent signals implement_finished directly when done, which triggers the
// existing review flow without any wave orchestration machinery.
func BuildBlueprintSkipPrompt(planFile string, plan *taskparser.Plan, project string) string {
	var sb strings.Builder

	// Count total tasks for the header message.
	totalTasks := 0
	for _, wave := range plan.Waves {
		totalTasks += len(wave.Tasks)
	}

	sb.WriteString(fmt.Sprintf("Implement all %d task(s) for plan: %s\n\n", totalTasks, planFile))

	// Inline coder rules — avoids context cost of loading the kasmos-coder skill.
	sb.WriteString("## Rules\n\n")
	sb.WriteString("- Implement ALL tasks in this plan sequentially.\n")
	sb.WriteString("- Do NOT load agent skills — rules are inlined here.\n")
	sb.WriteString("- Use `rg` (not grep), `sd` (not sed), `fd` (not find), `comby`/`ast-grep` for structural changes.\n")
	sb.WriteString("- Run scoped tests before committing: `" + verboseFailuresOnlyGoTestCmd + "`\n")
	sb.WriteString("- Verify build: `go build ./...`\n")
	sb.WriteString("- Commit: `git add <specific-files> && git commit -m \"feat(task-N): description\"`\n")
	sb.WriteString(fmt.Sprintf("- When done with ALL tasks: signal completion with MCP `signal_create` (signal_type: \"implement-finished\", plan_file: %q, project: %q). Then stop.\n\n", planFile, project))

	// Plan context header.
	header := plan.HeaderContext()
	if header != "" {
		sb.WriteString("## Plan Context\n\n")
		sb.WriteString(header)
		sb.WriteString("\n")
	}

	// All tasks, grouped by wave for context but implemented sequentially.
	for _, wave := range plan.Waves {
		for _, task := range wave.Tasks {
			sb.WriteString(fmt.Sprintf("## Task %d: %s\n\n", task.Number, task.Title))
			sb.WriteString(task.Body)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// BuildFixerPrompt builds the prompt for a fixer agent responding to reviewer
// feedback. Unlike implementation prompts, it scopes work to cited review
// findings and tells the agent not to resume broad plan execution.
func BuildFixerPrompt(planFile, project, feedback string, reviewRound int) string {
	var sb strings.Builder

	trimmedFeedback := strings.TrimSpace(feedback)
	if reviewRound < 1 {
		reviewRound = 1
	}

	sb.WriteString(fmt.Sprintf("Address reviewer feedback for plan: %s\n\n", planFile))
	sb.WriteString(fmt.Sprintf("Current fix round: %d\n\n", reviewRound))
	sb.WriteString("## Rules\n\n")
	sb.WriteString("- You are a fixer responding to review findings, not an implementer.\n")
	sb.WriteString(fmt.Sprintf("- Retrieve the full plan: prefer MCP `task_show` (filename: %q, project: %q); fall back to `kas task show %s` for context, but do NOT resume broad plan implementation.\n", planFile, project, planFile))
	sb.WriteString("- Fix only the cited review findings with minimal, targeted changes.\n")
	sb.WriteString("- Investigate root causes before editing code.\n")
	sb.WriteString("- Use `rg` (not grep), `sd` (not sed), `fd` (not find), `comby`/`ast-grep` for structural changes.\n")
	sb.WriteString("- Run targeted verification for the affected area first; run broader tests only as needed.\n")
	sb.WriteString(fmt.Sprintf("- When done: signal completion with MCP `signal_create` (signal_type: \"implement-finished\", plan_file: %q, project: %q). Then stop.\n\n", planFile, project))

	sb.WriteString("## Reviewer feedback\n\n")
	if trimmedFeedback != "" {
		sb.WriteString(trimmedFeedback)
		sb.WriteString("\n")
	} else {
		sb.WriteString("No structured reviewer feedback was attached. Inspect the latest reviewer output or PR review comments before making changes, and constrain work to those cited findings only.\n")
	}

	return sb.String()
}

// BuildPlannerPrompt returns the initial prompt for a planner agent session.
// The prompt explicitly requires ## Wave N headers because kasmos uses them
// for wave orchestration — without them, implementation cannot start.
// Shared between the TUI and daemon planner-spawn paths so prompt drift
// across entry points is impossible.
func BuildPlannerPrompt(planFile, planName, description, project string) string {
	return fmt.Sprintf(
		"Plan %s. Goal: %s. "+
			"Use the `kasmos-planner` skill. "+
			"The plan MUST include ## Wave N sections (at minimum ## Wave 1) "+
			"grouping all tasks — kasmos requires Wave headers to orchestrate implementation. "+
			"After writing the plan, store it with MCP `task_update_content` (filename: \"%[3]s\", project: \"%[4]s\") "+
			"and then signal completion with MCP `signal_create` (signal_type: \"planner-finished\", plan_file: \"%[3]s\", project: \"%[4]s\"). "+
			"If MCP is unavailable, fall back to `kas task update-content %[3]s` (pipe content) and `kas signal emit planner_finished %[3]s`.",
		planName, description, planFile, project,
	)
}

// PlannerPromptOptions carries per-profile options for draft-mode planner prompts.
type PlannerPromptOptions struct {
	// Profile is the named agent profile for this planner instance.
	Profile string
	// Primary indicates this planner should also write a preview to the task store.
	Primary bool
	// DraftMode instructs the planner to write a draft cache file and signal
	// planner-draft-finished instead of planner-finished.
	DraftMode bool
	// CachePath is the pre-computed path for the draft cache file.
	// If empty, the path is constructed as .kasmos/cache/<planFile>-planner-<Profile>.md.
	CachePath string
}

// BuildPlannerPromptWithOptions returns the prompt for a planner agent.
// When opts.DraftMode is false (or opts is zero-value), it delegates to BuildPlannerPrompt.
// In draft mode the prompt instructs the agent to write to the profile-specific cache path,
// optionally update the task store as preview content (when opts.Primary is true), signal
// planner-draft-finished with the profile payload, and never emit planner_finished.
func BuildPlannerPromptWithOptions(planFile, planName, description, project string, opts PlannerPromptOptions) string {
	if !opts.DraftMode {
		return BuildPlannerPrompt(planFile, planName, description, project)
	}

	cachePath := opts.CachePath
	if cachePath == "" {
		cachePath = fmt.Sprintf(".kasmos/cache/%s-planner-%s.md", planFile, opts.Profile)
	}

	payload := fmt.Sprintf(`{"planner_id":"%s"}`, opts.Profile)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plan %s. Goal: %s. ", planName, description))
	sb.WriteString("Use the `kasmos-planner` skill. ")
	sb.WriteString("The plan MUST include ## Wave N sections (at minimum ## Wave 1) " +
		"grouping all tasks — kasmos requires Wave headers to orchestrate implementation.\n\n")

	sb.WriteString("## Draft Mode\n\n")
	sb.WriteString(fmt.Sprintf("Profile: %s | Primary: %v\n\n", opts.Profile, opts.Primary))
	sb.WriteString("You are one of several parallel planners. Your output will be aggregated by the architect — " +
		"do NOT signal `planner_finished` or `planner-finished`.\n\n")

	sb.WriteString("## Instructions\n\n")

	step := 1
	sb.WriteString(fmt.Sprintf("%d. Write your complete markdown plan to `%s`\n", step, cachePath))
	step++

	if opts.Primary {
		sb.WriteString(fmt.Sprintf("%d. Write the same draft to the task store as preview content:\n", step))
		sb.WriteString(fmt.Sprintf("   - Prefer MCP `task_update_content` (filename: %q, project: %q)\n", planFile, project))
		sb.WriteString(fmt.Sprintf("   - Fall back to `kas task update-content %s` (pipe content)\n", planFile))
		step++
	}

	sb.WriteString(fmt.Sprintf("%d. Signal draft completion:\n", step))
	sb.WriteString(fmt.Sprintf("   - Prefer MCP `signal_create` (signal_type: \"planner-draft-finished\", plan_file: %q, project: %q, payload: %q)\n",
		planFile, project, payload))
	sb.WriteString(fmt.Sprintf("   - If MCP is unavailable, use `kas signal emit planner_draft_finished %s`\n", planFile))

	sb.WriteString("\n## Constraints\n\n")
	sb.WriteString(fmt.Sprintf("- Cache path: `%s`\n", cachePath))
	sb.WriteString(fmt.Sprintf("- Profile: %q — use this exact value as `planner_id` in the signal payload\n", opts.Profile))
	sb.WriteString(fmt.Sprintf("- Signal payload must be: %s\n", payload))
	sb.WriteString("- NEVER emit `planner_finished` or `planner-finished` — only `planner_draft_finished`\n")
	if !opts.Primary {
		sb.WriteString("- Do NOT update the task store with `task_update_content` — only the primary planner writes preview content\n")
	}

	return sb.String()
}

// ArchitectPromptOptions controls optional behavior for the final architect pass.
// Deprecated: ParallelBaseline and DescriptionHash are no longer used in new code paths.
// They are retained temporarily for compatibility and will be removed in a future wave.
type ArchitectPromptOptions struct {
	// Deprecated: parallel architect baseline has been replaced by multi-planner draft caches.
	// This field is ignored by BuildElaborationPromptWithOptions.
	ParallelBaseline bool
	// Deprecated: description hash is no longer used in new code paths.
	// This field is ignored by BuildElaborationPromptWithOptions.
	DescriptionHash string
}

// BuildElaborationPrompt returns the prompt for the architect-led elaboration pass.
// The architect reads the plan, deeply reads the codebase for each task's files,
// and expands task bodies with detailed implementation instructions.
func BuildElaborationPrompt(planFile, project string) string {
	return BuildElaborationPromptWithOptions(planFile, project, ArchitectPromptOptions{})
}

// BuildElaborationPromptWithOptions returns the prompt for the architect-led
// elaboration pass. opts.ParallelBaseline and opts.DescriptionHash are deprecated
// and ignored; the architect now always reads planner draft caches.
func BuildElaborationPromptWithOptions(planFile, project string, opts ArchitectPromptOptions) string {
	// opts.ParallelBaseline and opts.DescriptionHash are deprecated and unused.
	_ = opts.ParallelBaseline
	_ = opts.DescriptionHash

	return fmt.Sprintf(
		"You are the architect agent. You turn a planner's high-level design into a "+
			"concrete, coder-ready implementation plan. The planner focuses on *what* to build; "+
			"you decide *how* to build it. Before validating the planner draft, derive an independent "+
			"implementation baseline from the goal, codebase surfaces, dependencies, and existing patterns. "+
			"Then compare each planner draft against that architect baseline and the other planner draft peers. "+
			"Rewrite the stored plan by merging the best result from all inputs: keep planner intent where sound, "+
			"use your baseline where it is simpler or more correct, and add hidden integration surfaces, "+
			"non-obvious missing work, edge cases, incorrect file references, or task splits/merges/reordering "+
			"that the codebase requires.\n\n"+
			"Load the `kasmos-architect` skill before starting. Also load `cli-tools`.\n\n"+
			"## Instructions\n\n"+
			"1. Retrieve the plan: prefer MCP `task_show` (filename: \"%[1]s\", project: \"%[2]s\"); fall back to `kas task show %[1]s`\n"+
			"2. Read all planner draft caches for this plan:\n"+
			"   - List `.kasmos/cache/%[1]s-planner-*.md` — each file corresponds to one planner profile\n"+
			"   - Validate each file belongs to the current plan by confirming the filename prefix matches \"%[1]s\"\n"+
			"   - Note the profile extracted from each filename (the segment between \"-planner-\" and \".md\")\n"+
			"   - If no drafts are present, derive your baseline inline from codebase evidence and mention that in the plan summary\n"+
			"3. Read the relevant codebase surfaces before editing the draft. Start with files listed in **Files:** sections, "+
			"then follow neighboring interfaces, function signatures, error handling, data flow, dependencies, and existing patterns.\n"+
			"4. Create your independent solution baseline from the goal and codebase evidence before judging the planner drafts:\n"+
			"   - What implementation path would you choose if no planner task list existed?\n"+
			"   - Which files, waves, dependencies, and integration surfaces does that path require?\n"+
			"   - What hidden integration surfaces or non-obvious missing work must be represented for coders?\n"+
			"5. Compare each planner draft against your architect baseline and the other planner draft peers:\n"+
			"   - Are the planner's listed files correct? Add missing ones, remove irrelevant ones.\n"+
			"   - Is the planner's wave/task decomposition optimal? Merge, split, or reorder as needed.\n"+
			"   - Did the planner miss a simpler approach, hidden dependency, or required integration surface?\n"+
			"   - Did the planner include unnecessary work or conflict with existing patterns?\n"+
			"6. Expand each task body with concrete implementation detail:\n"+
			"   - Exact function signatures to create or modify\n"+
			"   - Existing codebase patterns to follow (with file references)\n"+
			"   - Edge cases and error handling requirements\n"+
			"   - Import paths and dependencies\n"+
			"   - Concrete code snippets where helpful\n"+
			"7. Keep ## Wave headers and the plan header fields (Goal, Architecture, Tech Stack, Size). "+
			"Everything else — task count, task content, file lists, wave assignment — is yours to change.\n"+
			"8. Write the updated plan: prefer MCP `task_update_content` (filename: \"%[1]s\", project: \"%[2]s\"); fall back to `kas task update-content %[1]s` (pipe content)\n"+
			"9. Write the architect metadata cache, including the decision audit, to `.kasmos/cache/%[1]s-architect.json` before signaling.\n"+
			"%[3]s"+
			"10. Signal architect-pass completion: prefer MCP `signal_create` (signal_type: \"elaborator-finished\", plan_file: \"%[1]s\", project: \"%[2]s\")\n"+
			"   - If MCP is unavailable, use `kas signal emit elaborator_finished %[1]s`; if CLI signaling is also unavailable, fallback: `touch .kasmos/signals/elaborator-finished-%[1]s`\n"+
			"   - Keep the role wording as architect in your notes and output; only the completion signal name stays legacy.\n",
		planFile, project, elaborationDecisionAuditInstructions(planFile, project),
	)
}

// elaborationDecisionAuditInstructions returns the decision audit instruction block
// for the architect elaboration pass, including planner_drafts guidance.
func elaborationDecisionAuditInstructions(planFile, project string) string {
	return fmt.Sprintf(
		"   - Preserve the existing wave/task metadata fields and add optional `decision_audit`; do not replace the task metadata with only the audit.\n"+
			"   - `decision_audit.baseline_source` must be one of `parallel_cache`, `inline`, `absent`, or `stale`.\n"+
			"   - Include a short `planner_summary`, a short `baseline_summary`, a `differences` list for each meaningful file, wave, API, UI, docs, or verification change, and a `final_decision` sentence that states the implementation path coders should follow.\n"+
			"   - Include `summary` as the concise overall audit summary.\n"+
			"   - Include `planner_drafts` with one entry per consumed planner draft: each entry must have `profile`, `cache_path` (the `.kasmos/cache/%[1]s-planner-<profile>.md` path), `summary`, and `decision`.\n"+
			"   - Prefer this metadata shape:\n\n"+
			"```json\n"+
			"{\n"+
			"  \"schema_version\": 1,\n"+
			"  \"plan_id\": \"%[1]s\",\n"+
			"  \"decision_audit\": {\n"+
			"    \"schema_version\": 1,\n"+
			"    \"plan_file\": \"%[1]s\",\n"+
			"    \"project\": \"%[2]s\",\n"+
			"    \"created_at\": \"<rfc3339>\",\n"+
			"    \"baseline_source\": \"inline\",\n"+
			"    \"summary\": \"...\",\n"+
			"    \"planner_summary\": \"...\",\n"+
			"    \"baseline_summary\": \"...\",\n"+
			"    \"final_decision\": \"...\",\n"+
			"    \"differences\": [],\n"+
			"    \"planner_drafts\": [\n"+
			"      {\"profile\": \"planner\", \"cache_path\": \".kasmos/cache/%[1]s-planner-planner.md\", \"summary\": \"...\", \"decision\": \"adopted\"}\n"+
			"    ]\n"+
			"  }\n"+
			"}\n"+
			"```\n",
		planFile, project,
	)
}

func architectDecisionAuditInstructions(planFile, project string, includeBaselineCache bool) string {
	baselineCacheNote := ""
	if includeBaselineCache {
		baselineCacheNote = fmt.Sprintf("   - `.kasmos/cache/%s-architect-baseline.json` is advisory input only and must not be treated as final implementation state.\n", planFile)
	}
	return fmt.Sprintf(
		"   - Preserve the existing wave/task metadata fields and add optional `decision_audit`; do not replace the task metadata with only the audit.\n"+
			"   - `decision_audit.baseline_source` must be one of `parallel_cache`, `inline`, `absent`, or `stale`.\n"+
			"   - Include a short `planner_summary`, a short `baseline_summary`, a `differences` list for each meaningful file, wave, API, UI, docs, or verification change, and a `final_decision` sentence that states the implementation path coders should follow.\n"+
			"   - Include `summary` as the concise overall audit summary.\n"+
			"%[3]s"+
			"   - Prefer this metadata shape:\n\n"+
			"```json\n"+
			"{\n"+
			"  \"schema_version\": 1,\n"+
			"  \"plan_id\": \"%[1]s\",\n"+
			"  \"decision_audit\": {\n"+
			"    \"schema_version\": 1,\n"+
			"    \"plan_file\": \"%[1]s\",\n"+
			"    \"project\": \"%[2]s\",\n"+
			"    \"created_at\": \"<rfc3339>\",\n"+
			"    \"baseline_source\": \"parallel_cache\",\n"+
			"    \"summary\": \"...\",\n"+
			"    \"planner_summary\": \"...\",\n"+
			"    \"baseline_summary\": \"...\",\n"+
			"    \"final_decision\": \"...\",\n"+
			"    \"differences\": []\n"+
			"  }\n"+
			"}\n"+
			"```\n",
		planFile, project, baselineCacheNote,
	)
}

// BuildArchitectBaselinePrompt returns the cache-only prompt for a parallel
// architect baseline session. The session must not mutate task lifecycle state.
func BuildArchitectBaselinePrompt(planFile, project, description string) string {
	descriptionHash := ArchitectBaselineDescriptionHash(description)
	return fmt.Sprintf(
		"You are the architect baseline agent for plan %[1]q in project %[2]q. "+
			"Your job is to independently derive an implementation baseline while the planner works. "+
			"Load the `kasmos-architect` skill and `cli-tools` before starting.\n\n"+
			"## Goal\n\n%[3]s\n\n"+
			"## Instructions\n\n"+
			"1. Inspect the live codebase independently from planner output. Do not wait for, read, or rely on the planner draft.\n"+
			"2. Derive the implementation baseline from the goal, product/runtime surfaces, dependencies, state transitions, prompts, config, tests, scaffold mirrors, and existing code patterns.\n"+
			"3. Write exactly one artifact: `.kasmos/cache/%[1]s-architect-baseline.json`.\n"+
			"4. Use this JSON schema and expected identity values:\n\n"+
			"```json\n"+
			"{\n"+
			"  \"schema_version\": 1,\n"+
			"  \"plan_file\": \"%[1]s\",\n"+
			"  \"project\": \"%[2]s\",\n"+
			"  \"description_hash\": \"%[4]s\",\n"+
			"  \"created_at\": \"<rfc3339 timestamp>\",\n"+
			"  \"baseline_markdown\": \"<non-empty markdown baseline>\",\n"+
			"  \"surfaces\": [\"<paths or subsystems>\"],\n"+
			"  \"risks\": [\"<implementation risks>\"],\n"+
			"  \"notes\": [\"<optional notes>\"]\n"+
			"}\n"+
			"```\n\n"+
			"5. Stop after the cache write.\n\n"+
			"## Cache-only constraints\n\n"+
			"- Do not edit any file except `.kasmos/cache/%[1]s-architect-baseline.json`.\n"+
			"- Forbidden: MCP `task_update_content`, `kas task update-content`, task status transitions, or any lifecycle signal.\n"+
			"- Forbidden lifecycle signals include `planner-finished`, `architect-finished`, and `elaborator-finished`.\n"+
			"- Do not mutate task content, task status, or orchestration state.\n",
		planFile, project, description, descriptionHash,
	)
}

// BuildArchitectPrompt returns the prompt for an architect agent session.
// The architect identifies task relationships and emits metadata for planning
// and orchestration decisions.
func BuildArchitectPrompt(planFile, project string) string {
	return fmt.Sprintf(
		"You are the architect agent. Your job: analyze a plan, identify architectural dependencies, and emit compact metadata for downstream orchestration. "+
			"Before validating the planner draft, derive an independent implementation baseline from the goal, codebase surfaces, dependencies, and existing patterns. "+
			"Compare planner vs architect baseline, then rewrite the stored plan by merging the best path and adding hidden integration surfaces or non-obvious missing work the planner missed.\n\n"+
			"Load the `kasmos-architect` and `cli-tools` skills before starting.\n\n"+
			"## Instructions\n\n"+
			"1. Retrieve the plan: prefer MCP `task_show` (filename: \"%[1]s\", project: \"%[2]s\"); fall back to `kas task show %[1]s`\n"+
			"2. Read the relevant codebase surfaces, then create an independent architect solution baseline before judging the planner's tasks/files/waves.\n"+
			"3. Compare planner vs architect baseline and rewrite the plan by merging the best result, including missed dependencies, unnecessary planner work, wrong file assumptions, hidden integration surfaces, and non-obvious missing work.\n"+
			"4. For each task, classify it as `parallel` when it has no file or execution dependency on other tasks in the same wave; otherwise classify it as serial.\n"+
			"5. Estimate token budgets for each task, including required context depth and expected implementation footprint.\n"+
			"6. Write the enriched plan back: prefer MCP `task_update_content` (filename: \"%[1]s\", project: \"%[2]s\"); fall back to `kas task update-content %[1]s` (pipe content)\n"+
			"7. Write architect metadata to `.kasmos/cache/%[1]s-architect.json` using the schema example in `architect-v1.json`, preserving existing wave metadata and adding the decision audit before signaling.\n"+
			"%[3]s"+
			"8. Signal architect-pass completion: prefer MCP `signal_create` (signal_type: \"elaborator-finished\", plan_file: \"%[1]s\", project: \"%[2]s\")\n"+
			"   - If MCP is unavailable, use `kas signal emit elaborator_finished %[1]s`; if CLI signaling is also unavailable, fallback: `touch .kasmos/signals/elaborator-finished-%[1]s`\n"+
			"   - Keep the role wording as architect in your notes and output; only the completion signal name stays legacy.\n",
		planFile, project, architectDecisionAuditInstructions(planFile, project, true),
	)
}

// BuildWaveAnnotationPrompt returns the prompt used when a planner is respawned
// to add ## Wave headers to an existing plan that is missing them.
// It instructs the planner to annotate the plan, persist it, and signal
// completion so kasmos can resume the implementation flow.
func BuildWaveAnnotationPrompt(planFile, project string) string {
	return fmt.Sprintf(
		"The plan %[1]s is missing ## Wave N headers required for kasmos wave orchestration. "+
			"Retrieve the plan content with MCP `task_show` (filename: \"%[1]s\", project: \"%[2]s\") — fall back to `kas task show %[1]s` if MCP is unavailable — then annotate it by wrapping "+
			"all tasks under ## Wave N sections. "+
			"Every plan needs at least ## Wave 1 — even single-task trivial plans. "+
			"Keep all existing task content intact; only add the ## Wave headers.\n\n"+
			"After annotating:\n"+
			"1. Store the updated plan via MCP `task_update_content` (filename: \"%[1]s\", project: \"%[2]s\"); fall back to `kas task update-content %[1]s` (pipe content)\n"+
			"2. Signal completion: prefer MCP `signal_create` (signal_type: \"planner-finished\", plan_file: \"%[1]s\", project: \"%[2]s\")\n"+
			"   - If MCP is unavailable, use `kas signal emit planner_finished %[1]s`; if CLI signaling is also unavailable, fallback: `touch .kasmos/signals/planner-finished-%[1]s`\n"+
			"Do not modify task state directly.",
		planFile, project,
	)
}

// BuildMasterReviewPrompt returns the prompt for the master agent's holistic
// readiness review. The agent runs during the verifying FSM state and gathers
// its own evidence (merge-base diff, verification results) in the session rather
// than receiving pre-computed diffs, mirroring the reviewer/fixer prompt style.
// The agent is expected to self-fix trivial findings per the kasmos-master
// Self-Fix Protocol before signaling verify_failed; only verify_approved and
// verify_failed are valid outcomes.
func BuildMasterReviewPrompt(planFile, project string) string {
	return BuildMasterReviewPromptWithConfig(planFile, project, 80, 2)
}

// BuildMasterReviewPromptWithConfig generates the master agent prompt with
// configurable self-fix line ceiling (selfFixMaxLines) and verify-round cap
// (maxVerifyCycles). Values <= 0 are replaced with the defaults (80 and 2
// respectively).
func BuildMasterReviewPromptWithConfig(planFile, project string, selfFixMaxLines, maxVerifyCycles int) string {
	if selfFixMaxLines <= 0 {
		selfFixMaxLines = 80
	}
	if maxVerifyCycles <= 0 {
		maxVerifyCycles = 2
	}
	return fmt.Sprintf(
		"You are the master readiness review agent. You run during the `verifying` FSM state.\n\n"+
			"Load the `kasmos-master` skill before starting.\n\n"+
			"## Instructions\n\n"+
			"1. Retrieve the plan: prefer MCP `task_show` (filename: %[1]q, project: %[2]q); fall back to `kas task show %[1]s`\n"+
			"2. Gather evidence:\n"+
			"   - Merge-base diff: `MERGE_BASE=$(git merge-base HEAD main) && git diff $MERGE_BASE HEAD`\n"+
			"   - Run verification: `go build ./... && "+compactFailuresOnlyGoTestCmd+"` (or the plan's verify_checks)\n"+
			"3. If you find issues, classify them per the kasmos-master skill's Self-Fix Protocol. "+
			"Trivial allow-list findings (typos, missing exported doc comments, unused imports, format-verb mistakes, "+
			"`typos`/`gofmt` fixes, trivial `go vet` findings — total ≤ %[3]d net lines) MUST be fixed directly in the worktree, "+
			"verified with `gofmt -l .`, `go vet ./...`, `go build ./...`, `"+compactFailuresOnlyGoTestCmd+"`, and `typos`, "+
			"then committed as `fix: <description> (master self-fix)` and approved only after the gate passes. "+
			"Do NOT emit `verify_failed` for findings the protocol marks as self-fixable. "+
			"If a self-fix attempt fails any gate step, run `git restore --staged --worktree .` to drop the changes and emit `verify_failed` with the original finding.\n"+
			"4. Review the implementation holistically against the plan and signal your decision:\n"+
			"   - Approved: prefer MCP `signal_create` (signal_type: \"verify_approved\", plan_file: %[1]q, project: %[2]q); fall back to `kas signal emit verify_approved %[1]s`\n"+
			"   - Changes requested: prefer MCP `signal_create` (signal_type: \"verify_failed\", plan_file: %[1]q, project: %[2]q); fall back to `kas signal emit verify_failed %[1]s`\n"+
			"   - Include your review summary in the signal payload body field.\n\n"+
			"Note: this verify loop is capped at %[4]d round(s). If the cap is reached, the daemon force-promotes to approved.\n\n"+
			"Do not emit `review_approved` or `review_changes_requested` — use `verify_approved` or `verify_failed` above.",
		planFile, project, selfFixMaxLines, maxVerifyCycles,
	)
}
