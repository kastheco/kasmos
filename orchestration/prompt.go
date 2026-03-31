package orchestration

import (
	"fmt"
	"strings"

	"github.com/kastheco/kasmos/config/taskparser"
)

// BuildTaskPrompt constructs the prompt for a single task instance.
func BuildTaskPrompt(planFile string, plan *taskparser.Plan, task taskparser.Task, waveNumber, totalWaves, peerCount int, meta *TaskMeta) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Implement Task %d: %s\n\n", task.Number, task.Title))

	// Inline coder rules — avoids the context cost of loading the kasmos-coder skill
	sb.WriteString("## Rules\n\n")
	sb.WriteString("- Implement ONLY this task. Do not modify files outside your scope.\n")
	sb.WriteString("- Do NOT load agent skills — rules are inlined here.\n")
	sb.WriteString("- Use `rg` (not grep), `sd` (not sed), `fd` (not find), `comby`/`ast-grep` for structural changes.\n")
	sb.WriteString("- Run scoped tests before committing: `go test ./pkg/... -run Test<Name> -v`\n")
	sb.WriteString("- Verify build: `go build ./...`\n")
	sb.WriteString("- Commit: `git add <specific-files> && git commit -m \"feat(task-N): description\"`\n")
	sb.WriteString("- When done: stop and return to the prompt. Do NOT emit lifecycle signals yourself; kasmos detects managed task completion automatically.\n\n")

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
func BuildBlueprintSkipPrompt(planFile string, plan *taskparser.Plan) string {
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
	sb.WriteString("- Run scoped tests before committing: `go test ./pkg/... -run Test<Name> -v`\n")
	sb.WriteString("- Verify build: `go build ./...`\n")
	sb.WriteString("- Commit: `git add <specific-files> && git commit -m \"feat(task-N): description\"`\n")
	sb.WriteString("- When done with ALL tasks: stop and return to the prompt. Do NOT emit lifecycle signals yourself; kasmos handles the review handoff.\n\n")

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
func BuildFixerPrompt(planFile, feedback string, reviewRound int) string {
	var sb strings.Builder

	trimmedFeedback := strings.TrimSpace(feedback)
	if reviewRound < 1 {
		reviewRound = 1
	}

	sb.WriteString(fmt.Sprintf("Address reviewer feedback for plan: %s\n\n", planFile))
	sb.WriteString(fmt.Sprintf("Current fix round: %d\n\n", reviewRound))
	sb.WriteString("## Rules\n\n")
	sb.WriteString("- You are a fixer responding to review findings, not an implementer.\n")
	sb.WriteString(fmt.Sprintf("- Retrieve the full plan with `kas task show %s` for context, but do NOT resume broad plan implementation.\n", planFile))
	sb.WriteString("- Fix only the cited review findings with minimal, targeted changes.\n")
	sb.WriteString("- Investigate root causes before editing code.\n")
	sb.WriteString("- Use `rg` (not grep), `sd` (not sed), `fd` (not find), `comby`/`ast-grep` for structural changes.\n")
	sb.WriteString("- Run targeted verification for the affected area first; run broader tests only as needed.\n")
	sb.WriteString("- Do not emit completion signals from this fixer prompt; kasmos advances the review loop after you stop.\n")
	sb.WriteString("- When done, stop so kasmos can continue the review loop.\n\n")

	sb.WriteString("## Reviewer feedback\n\n")
	if trimmedFeedback != "" {
		sb.WriteString(trimmedFeedback)
		sb.WriteString("\n")
	} else {
		sb.WriteString("No structured reviewer feedback was attached. Inspect the latest reviewer output or PR review comments before making changes, and constrain work to those cited findings only.\n")
	}

	return sb.String()
}

// BuildElaborationPrompt returns the prompt for the architect-led elaboration pass.
// The architect reads the plan, deeply reads the codebase for each task's files,
// and expands task bodies with detailed implementation instructions.
func BuildElaborationPrompt(planFile string) string {
	return fmt.Sprintf(
		"You are the architect agent. Your job: enrich a plan's task descriptions with "+
			"detailed implementation instructions so coder agents make fewer decisions.\n\n"+
			"Load the `kasmos-architect` skill before starting. Also load `cli-tools`.\n\n"+
			"## Instructions\n\n"+
			"1. Retrieve the plan: `kas task show %[1]s`\n"+
			"2. For each task, read the codebase files listed in its **Files:** section. "+
			"Study existing patterns, interfaces, function signatures, error handling, "+
			"and data flow in those files and their neighbors.\n"+
			"3. Expand each task body with concrete implementation detail:\n"+
			"   - Exact function signatures to create or modify\n"+
			"   - Existing codebase patterns to follow (with file references)\n"+
			"   - Edge cases and error handling requirements\n"+
			"   - Import paths and dependencies\n"+
			"   - Concrete code snippets where helpful\n"+
			"4. Preserve the plan structure — do not change wave organization, "+
			"task numbering, file lists, or the header fields. Only expand task bodies.\n"+
			"5. Write the updated plan: pipe content to `kas task update-content %[1]s`\n"+
			"6. Signal architect-pass completion with the retained external contract: prefer MCP `signal_create` (signal_type: \"elaborator-finished\", plan_file: \"%[1]s\")\n"+
			"   - If MCP is unavailable, use `kas signal emit elaborator_finished %[1]s`; if CLI signaling is also unavailable, fallback: `touch .kasmos/signals/elaborator-finished-%[1]s`\n"+
			"   - Keep the role wording as architect in your notes and output; only the completion signal name stays legacy.\n",
		planFile,
	)
}

// BuildArchitectPrompt returns the prompt for an architect agent session.
// The architect identifies task relationships and emits metadata for planning
// and orchestration decisions.
func BuildArchitectPrompt(planFile string) string {
	return fmt.Sprintf(
		"You are the architect agent. Your job: analyze a plan, identify architectural dependencies, and emit compact metadata for downstream orchestration.\n\n"+
			"Load the `kasmos-architect` and `cli-tools` skills before starting.\n\n"+
			"## Instructions\n\n"+
			"1. Retrieve the plan: `kas task show %[1]s`\n"+
			"2. For each task, classify it as `parallel` when it has no file or execution dependency on other tasks in the same wave; otherwise classify it as serial.\n"+
			"3. Estimate token budgets for each task, including required context depth and expected implementation footprint.\n"+
			"4. Write the enriched plan back: pipe content to `kas task update-content %[1]s`\n"+
			"5. Write architect metadata to `.kasmos/cache/%[1]s-architect.json` using the schema example in `architect-v1.json`.\n"+
			"6. Signal completion: `touch .kasmos/signals/architect-finished-%[1]s`\n"+
			"7. Note: app/FSM consumption of this new architect-finished signal is follow-up work and should be implemented separately.\n",
		planFile,
	)
}

// BuildWaveAnnotationPrompt returns the prompt used when a planner is respawned
// to add ## Wave headers to an existing plan that is missing them.
// It instructs the planner to annotate the plan, persist it, and signal
// completion so kasmos can resume the implementation flow.
func BuildWaveAnnotationPrompt(planFile string) string {
	return fmt.Sprintf(
		"The plan %[1]s is missing ## Wave N headers required for kasmos wave orchestration. "+
			"Retrieve the plan content with `kas task show %[1]s`, then annotate it by wrapping "+
			"all tasks under ## Wave N sections. "+
			"Every plan needs at least ## Wave 1 — even single-task trivial plans. "+
			"Keep all existing task content intact; only add the ## Wave headers.\n\n"+
			"After annotating:\n"+
			"1. Store the updated plan via `kas task update-content %[1]s` (pipe the content)\n"+
			"2. Signal completion: prefer MCP `signal_create` (signal_type: \"planner-finished\", plan_file: \"%[1]s\")\n"+
			"   - If MCP is unavailable, use `kas signal emit planner_finished %[1]s`; if CLI signaling is also unavailable, fallback: `touch .kasmos/signals/planner-finished-%[1]s`\n"+
			"Do not edit plan-state.json directly.",
		planFile,
	)
}

// BuildMasterReviewPrompt defines the review task prompt for the kasmos-master role.
// Signal consumption is intentionally left for follow-up app/FSM work, so this builder
// only standardizes the instructions and completion signal contract.
func BuildMasterReviewPrompt(planFile, diffContent, testResults string) string {
	return fmt.Sprintf(
		"You are the master review agent. Load the `kasmos-master` skill, read the plan with "+
			"`kas task show %[1]s`, then review the proposed change for plan alignment and merge readiness.\n\n"+
			"## Review Task\n"+
			"- Determine whether the diff should be merged and signal your decision with `touch .kasmos/signals/master-approved-%[1]s` when complete.\n\n"+
			"## Test Results\n%s\n\n"+
			"## Diff\n%s\n",
		planFile,
		testResults,
		diffContent,
	)
}
