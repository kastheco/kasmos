package orchestration

import (
	"testing"

	"github.com/kastheco/kasmos/config/taskparser"
	"github.com/stretchr/testify/assert"
)

func TestBuildTaskPrompt(t *testing.T) {
	plan := &taskparser.Plan{
		Goal:         "Build a feature",
		Architecture: "Modular approach",
		TechStack:    "Go, bubbletea",
	}
	task := taskparser.Task{
		Number: 2,
		Title:  "Update Tests",
		Body:   "**Step 1:** Write the test\n\n**Step 2:** Run it",
	}

	prompt := BuildTaskPrompt("feature", plan, task, 1, 3, 4, "myproject", nil)

	// Plan context
	assert.Contains(t, prompt, "Build a feature")
	assert.Contains(t, prompt, "Modular approach")
	assert.Contains(t, prompt, "Go, bubbletea")
	// Rules section must be inlined (no skill-load instruction)
	assert.NotContains(t, prompt, "Load the `kasmos-coder` skill")
	assert.Contains(t, prompt, "## Rules")
	// Filtered test recipe must be inlined (not bare go test)
	assert.Contains(t, prompt, "mktemp")
	assert.Contains(t, prompt, "rg -v")
	assert.Contains(t, prompt, "tests passed")

	// Task identity
	assert.Contains(t, prompt, "Task 2")
	assert.Contains(t, prompt, "Update Tests")
	assert.Contains(t, prompt, "Write the test")
	assert.Contains(t, prompt, "Wave 1 of 3")

	// Parallel awareness (multi-task)
	assert.Contains(t, prompt, "Task 2 of 4")
	assert.Contains(t, prompt, "3 other agents")
	assert.Contains(t, prompt, "NEVER run `git add .`")
	assert.Contains(t, prompt, "NEVER run `git stash`")
	assert.Contains(t, prompt, "NEVER run `git checkout --")
	assert.Contains(t, prompt, "formatters/linters")
	assert.Contains(t, prompt, "test failures in files outside your task")
	assert.Contains(t, prompt, "build failure caused by missing types")
	assert.Contains(t, prompt, "surgical changes")
	assert.Contains(t, prompt, "signal_create")
	assert.Contains(t, prompt, "implement-task-finished")
	assert.Contains(t, prompt, `project: "myproject"`)
}

func TestBuildTaskPrompt_InlineCoderRules(t *testing.T) {
	plan := &taskparser.Plan{Goal: "Test feature"}
	task := taskparser.Task{Number: 1, Title: "Do thing", Body: "Make the change"}

	prompt := BuildTaskPrompt("feature", plan, task, 1, 1, 1, "testproject", nil)

	assert.NotContains(t, prompt, "kasmos-coder")
	assert.NotContains(t, prompt, "cli-tools")
	assert.NotContains(t, prompt, "Load the")

	assert.Contains(t, prompt, "## Rules")
	assert.Contains(t, prompt, "git add <specific-files>")
	assert.Contains(t, prompt, "feat(task-N):")
	assert.Contains(t, prompt, "-run Test")
	assert.Contains(t, prompt, "go build ./...")
	assert.Contains(t, prompt, "signal_create")
	assert.Contains(t, prompt, `"implement-task-finished"`)
	assert.Contains(t, prompt, `\"wave_number\":1,\"task_number\":1`)
	// Filtered test recipe must be inlined
	assert.Contains(t, prompt, "mktemp")
	assert.Contains(t, prompt, "rg -v")
	assert.Contains(t, prompt, "tests passed")
}

func TestBuildTaskPrompt_PreservesMdPlanTokenWhenProvided(t *testing.T) {
	plan := &taskparser.Plan{Goal: "Test feature"}
	task := taskparser.Task{Number: 1, Title: "Do thing", Body: "Make the change"}

	prompt := BuildTaskPrompt("feature.md", plan, task, 1, 1, 1, "testproject", nil)

	assert.NotContains(t, prompt, "kas signal emit implement_task_finished feature.md")
	assert.NotContains(t, prompt, "implement-task-finished-w1-t1-feature.md")
}

func TestBuildTaskPrompt_ContainsSignalEmit(t *testing.T) {
	plan := &taskparser.Plan{Waves: []taskparser.Wave{{Number: 1, Tasks: []taskparser.Task{{Number: 1, Title: "test", Body: "do stuff"}}}}}
	prompt := BuildTaskPrompt("my-plan", plan, plan.Waves[0].Tasks[0], 1, 1, 1, "testproject", nil)
	assert.NotContains(t, prompt, "kas signal emit implement_task_finished my-plan")
	assert.NotContains(t, prompt, "implement-task-finished-w1-t1-my-plan")
}

func TestBuildTaskPrompt_SingleTask(t *testing.T) {
	plan := &taskparser.Plan{Goal: "Simple"}
	task := taskparser.Task{Number: 1, Title: "Only Task", Body: "Do it"}

	prompt := BuildTaskPrompt("feature", plan, task, 1, 1, 1, "testproject", nil)

	// Single task shouldn't mention parallel coordination
	assert.NotContains(t, prompt, "parallel")
	assert.NotContains(t, prompt, "NEVER run")
	assert.NotContains(t, prompt, "other agents")
	assert.NotContains(t, prompt, "build failure caused by missing types")
}

func TestBuildTaskPrompt_WithMeta(t *testing.T) {
	plan := &taskparser.Plan{Goal: "Feature X", Architecture: "Modular", TechStack: "Go"}
	task := taskparser.Task{Number: 1, Title: "Add widget", Body: "Implement the widget"}
	meta := &TaskMeta{
		TaskNumber:     1,
		VerifyChecks:   []string{"go test ./widget/... -v", "go vet ./widget/..."},
		ContextRefs:    []string{"ref://widget-interface"},
		PreferredModel: "openai/gpt-5.3-codex-spark",
	}

	prompt := BuildTaskPrompt("feat", plan, task, 1, 2, 1, "testproject", meta)

	assert.Contains(t, prompt, "go test ./widget/... -v")
	assert.Contains(t, prompt, "go vet ./widget/...")
	assert.Contains(t, prompt, "## Verification Commands")
	assert.NotContains(t, prompt, "ref://widget-interface")
	assert.NotContains(t, prompt, "openai/gpt-5.3-codex-spark")
}

func TestBuildTaskPrompt_NilMeta(t *testing.T) {
	plan := &taskparser.Plan{Goal: "Simple"}
	task := taskparser.Task{Number: 1, Title: "Only Task", Body: "Do it"}

	prompt := BuildTaskPrompt("feat", plan, task, 1, 1, 1, "testproject", nil)

	assert.NotContains(t, prompt, "## Verification Commands")
	assert.Contains(t, prompt, "## Rules")
	assert.Contains(t, prompt, "Task 1")
}

func TestBuildWaveAnnotationPrompt(t *testing.T) {
	prompt := BuildWaveAnnotationPrompt("my-feature", "myproject")
	assert.Contains(t, prompt, "kas task show my-feature")
	assert.Contains(t, prompt, "## Wave")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"planner-finished\", plan_file: \"my-feature\", project: \"myproject\")")
	// CLI fallback remains documented
	assert.Contains(t, prompt, "kas signal emit planner_finished my-feature")
	// Fallback filesystem sentinel still present
	assert.Contains(t, prompt, "planner-finished-my-feature")
	assert.NotContains(t, prompt, "The plan at docs/plans/")
}

func TestBuildFixerPrompt(t *testing.T) {
	prompt := BuildFixerPrompt("my-feature", "myproject", "- [app.go:42] fix the failing review handoff", 3)

	assert.Contains(t, prompt, "Address reviewer feedback for plan: my-feature")
	assert.Contains(t, prompt, "Current fix round: 3")
	assert.Contains(t, prompt, "kas task show my-feature")
	assert.Contains(t, prompt, `project: "myproject"`)
	assert.Contains(t, prompt, "not an implementer")
	assert.Contains(t, prompt, "fix the failing review handoff")
	assert.Contains(t, prompt, "signal_create")
	assert.Contains(t, prompt, "implement-finished")
	assert.NotContains(t, prompt, "execute all tasks sequentially")
}

func TestBuildFixerPrompt_WithoutFeedback(t *testing.T) {
	prompt := BuildFixerPrompt("my-feature", "myproject", "   ", 2)

	assert.Contains(t, prompt, "No structured reviewer feedback was attached")
	assert.Contains(t, prompt, "Inspect the latest reviewer output or PR review comments")
	assert.Contains(t, prompt, "Current fix round: 2")
	assert.NotContains(t, prompt, "execute all tasks sequentially")
}

func TestBuildMasterReviewPrompt(t *testing.T) {
	prompt := BuildMasterReviewPrompt("my-feature", "myproject")

	assert.Contains(t, prompt, "my-feature")
	assert.Contains(t, prompt, "kasmos-master")
	assert.Contains(t, prompt, `project: "myproject"`)
	// Runs during verifying FSM state
	assert.Contains(t, prompt, "verifying")
	// MCP-first verify signal emission (canonical underscore form)
	assert.Contains(t, prompt, "signal_create` (signal_type: \"verify_approved\"")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"verify_failed\"")
	// CLI fallback
	assert.Contains(t, prompt, "kas signal emit verify_approved my-feature")
	assert.Contains(t, prompt, "kas signal emit verify_failed my-feature")
	// Evidence gathering
	assert.Contains(t, prompt, "Merge-base diff")
	assert.Contains(t, prompt, "MERGE_BASE")
	// Compact failures-only command is embedded (wraps go test ./... with failure filtering)
	assert.Contains(t, prompt, compactFailuresOnlyGoTestCmd)
	// Self-Fix Protocol integration
	assert.Contains(t, prompt, "Self-Fix Protocol")
	assert.Contains(t, prompt, "(master self-fix)")
	assert.Contains(t, prompt, "git restore")
	assert.Contains(t, prompt, "verify_failed")
	// No readiness-specific signal types (now uses verify-*)
	assert.NotContains(t, prompt, "readiness-approved")
	assert.NotContains(t, prompt, "readiness-changes-requested")
	// No filesystem sentinels
	assert.NotContains(t, prompt, "touch .kasmos/signals/master-approved")
	// No pre-computed diff arguments
	assert.NotContains(t, prompt, "## Diff")
	assert.NotContains(t, prompt, "## Test Results")
}

func TestBuildElaborationPrompt(t *testing.T) {
	prompt := BuildElaborationPrompt("my-feature", "myproject")

	assert.Contains(t, prompt, "kasmos-architect")
	assert.NotContains(t, prompt, "kasmos-elaborator")
	assert.Contains(t, prompt, "independent solution baseline")
	assert.Contains(t, prompt, "Before validating the planner draft")
	// Architect compares each planner draft against its own baseline
	assert.Contains(t, prompt, "Compare each planner draft against your architect baseline")
	assert.Contains(t, prompt, "merging the best")
	assert.Contains(t, prompt, "hidden integration surfaces")
	assert.Contains(t, prompt, "non-obvious missing work")
	// Must instruct reading planner draft caches
	assert.Contains(t, prompt, ".kasmos/cache/my-feature-planner-*.md")
	assert.Contains(t, prompt, "planner_drafts")
	// Must reference the plan file for retrieval via MCP
	assert.Contains(t, prompt, "task_show")
	assert.Contains(t, prompt, "kas task show my-feature") // CLI fallback
	assert.Contains(t, prompt, `project: "myproject"`)
	// Must reference updating the plan via MCP
	assert.Contains(t, prompt, "task_update_content")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"my-feature\", project: \"myproject\")")
	// CLI fallback remains documented
	assert.Contains(t, prompt, "kas signal emit elaborator_finished my-feature")
	// Fallback filesystem sentinel still present
	assert.Contains(t, prompt, "elaborator-finished-my-feature")
	// Role wording should stay on architect even though the external signal stays legacy.
	assert.Contains(t, prompt, "Signal architect-pass completion")
	assert.Contains(t, prompt, "only the completion signal name stays legacy")
	assert.NotContains(t, prompt, "elaborator agent")
	// Must instruct to expand task bodies
	assert.Contains(t, prompt, "implementation detail")
	// Must instruct to preserve structure
	assert.Contains(t, prompt, "Keep ## Wave headers")
	assert.Contains(t, prompt, "plan header fields")
	// Must reference reading the codebase
	assert.Contains(t, prompt, "codebase")
	assert.Contains(t, prompt, ".kasmos/cache/my-feature-architect.json")
	assert.Contains(t, prompt, "decision_audit")
	assert.Contains(t, prompt, "planner_summary")
	assert.Contains(t, prompt, "baseline_summary")
	assert.Contains(t, prompt, "final_decision")
	assert.Contains(t, prompt, "`parallel_cache`, `inline`, `absent`, or `stale`")
	assert.NotContains(t, prompt, "architect-baseline.json")
	assert.NotContains(t, prompt, "raw planner snapshot")
	assert.NotContains(t, prompt, "architect-finished")
}

func TestBuildElaborationPromptWithOptions_PlannerDraftGuidance(t *testing.T) {
	// Deprecated opts are silently ignored; both zero and non-zero opts produce
	// identical planner-draft guidance in the new code path.
	for _, opts := range []ArchitectPromptOptions{
		{},
		{ParallelBaseline: true, DescriptionHash: "abc123"},
	} {
		prompt := BuildElaborationPromptWithOptions("my-feature", "myproject", opts)

		// Planner draft caches are always referenced
		assert.Contains(t, prompt, ".kasmos/cache/my-feature-planner-*.md", "opts=%+v", opts)
		assert.Contains(t, prompt, "planner_drafts", "opts=%+v", opts)

		// Step numbering in new prompt
		assert.Contains(t, prompt, "2. Read all planner draft caches", "opts=%+v", opts)
		assert.Contains(t, prompt, "3. Read the relevant codebase surfaces", "opts=%+v", opts)
		assert.Contains(t, prompt, "4. Create your independent solution baseline", "opts=%+v", opts)
		assert.Contains(t, prompt, "9. Write the architect metadata cache", "opts=%+v", opts)
		assert.Contains(t, prompt, "10. Signal architect-pass completion", "opts=%+v", opts)

		// Legacy baseline cache must NOT be referenced
		assert.NotContains(t, prompt, "architect-baseline.json", "opts=%+v", opts)
		assert.NotContains(t, prompt, "advisory parallel architect baseline cache", "opts=%+v", opts)
		assert.NotContains(t, prompt, "description_hash", "opts=%+v", opts)

		// Standard elaboration fields still present
		assert.Contains(t, prompt, "task_update_content", "opts=%+v", opts)
		assert.Contains(t, prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"my-feature\", project: \"myproject\")", "opts=%+v", opts)
		assert.Contains(t, prompt, "decision_audit", "opts=%+v", opts)
		assert.Contains(t, prompt, "planner_summary", "opts=%+v", opts)
		assert.Contains(t, prompt, "baseline_summary", "opts=%+v", opts)
		assert.Contains(t, prompt, "final_decision", "opts=%+v", opts)
	}
}

func TestBuildArchitectPrompt(t *testing.T) {
	prompt := BuildArchitectPrompt("my-feature", "myproject")

	assert.Contains(t, prompt, "kasmos-architect")
	assert.Contains(t, prompt, "independent implementation baseline")
	assert.Contains(t, prompt, "Compare planner vs architect baseline")
	assert.Contains(t, prompt, "merging the best")
	assert.Contains(t, prompt, "hidden integration surfaces")
	assert.Contains(t, prompt, "non-obvious missing work")
	assert.Contains(t, prompt, "task_show")
	assert.Contains(t, prompt, "kas task show my-feature") // CLI fallback
	assert.Contains(t, prompt, `project: "myproject"`)
	assert.Contains(t, prompt, "task_update_content")
	assert.Contains(t, prompt, "architect-v1.json")
	assert.Contains(t, prompt, "parallel")
	assert.Contains(t, prompt, ".kasmos/cache/my-feature-architect.json")
	assert.Contains(t, prompt, "decision_audit")
	assert.Contains(t, prompt, "planner_summary")
	assert.Contains(t, prompt, "baseline_summary")
	assert.Contains(t, prompt, "final_decision")
	assert.Contains(t, prompt, "differences")
	assert.NotContains(t, prompt, "architect-baseline.json")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"my-feature\", project: \"myproject\")")
	// Signal completion should prefer MCP with filesystem fallback
	assert.Contains(t, prompt, "signal_create")
	assert.NotContains(t, prompt, "architect-finished")
	assert.NotContains(t, prompt, "raw planner snapshot")
}

func TestBuildPlannerPromptWithOptions_LegacyPath(t *testing.T) {
	// DraftMode false should delegate to BuildPlannerPrompt
	prompt := BuildPlannerPromptWithOptions("my-plan", "my-plan", "build X", "myproject", PlannerPromptOptions{})
	legacyPrompt := BuildPlannerPrompt("my-plan", "my-plan", "build X", "myproject")
	assert.Equal(t, legacyPrompt, prompt)
}

func TestBuildPlannerPromptWithOptions_DraftMode_NonPrimary(t *testing.T) {
	opts := PlannerPromptOptions{
		Profile:   "gpt",
		Primary:   false,
		DraftMode: true,
	}
	prompt := BuildPlannerPromptWithOptions("my-plan", "my-plan", "build X", "myproject", opts)

	// Must mention profile and draft mode context
	assert.Contains(t, prompt, "Profile: gpt")
	assert.Contains(t, prompt, "Draft Mode")
	// Must write to profile-specific cache path
	assert.Contains(t, prompt, ".kasmos/cache/my-plan-planner-gpt.md")
	// Must signal planner-draft-finished with correct payload
	assert.Contains(t, prompt, `signal_type: "planner-draft-finished"`)
	assert.Contains(t, prompt, `planner_draft_finished`)
	assert.Contains(t, prompt, `{"planner_id":"gpt"}`)
	// Must NOT instruct signaling planner_finished — only planner_draft_finished
	assert.NotContains(t, prompt, `signal_type: "planner_finished"`)
	assert.NotContains(t, prompt, `signal_type: "planner-finished"`)
	// Non-primary: must NOT instruct to update task store
	assert.NotContains(t, prompt, "Prefer MCP `task_update_content`")
	// Must reference plan file and project
	assert.Contains(t, prompt, `"my-plan"`)
	assert.Contains(t, prompt, `"myproject"`)
}

func TestBuildPlannerPromptWithOptions_DraftMode_Primary(t *testing.T) {
	opts := PlannerPromptOptions{
		Profile:   "planner",
		Primary:   true,
		DraftMode: true,
	}
	prompt := BuildPlannerPromptWithOptions("my-plan", "my-plan", "build X", "myproject", opts)

	// Primary must update task store as preview
	assert.Contains(t, prompt, "task_update_content")
	assert.Contains(t, prompt, "preview content")
	assert.Contains(t, prompt, ".kasmos/cache/my-plan-planner-planner.md")
	assert.Contains(t, prompt, `{"planner_id":"planner"}`)
	// Must NOT instruct signaling planner_finished
	assert.NotContains(t, prompt, `signal_type: "planner_finished"`)
	assert.NotContains(t, prompt, `signal_type: "planner-finished"`)
}

func TestBuildPlannerPromptWithOptions_DraftMode_CustomCachePath(t *testing.T) {
	opts := PlannerPromptOptions{
		Profile:   "claude",
		Primary:   false,
		DraftMode: true,
		CachePath: ".kasmos/cache/custom-path.md",
	}
	prompt := BuildPlannerPromptWithOptions("my-plan", "my-plan", "build X", "myproject", opts)

	// Custom CachePath must be embedded verbatim
	assert.Contains(t, prompt, ".kasmos/cache/custom-path.md")
	assert.NotContains(t, prompt, ".kasmos/cache/my-plan-planner-claude.md")
}

func TestBuildElaborationPrompt_RetainsLegacySignalName(t *testing.T) {
	prompt := BuildElaborationPrompt("my-feature", "myproject")

	assert.Contains(t, prompt, "only the completion signal name stays legacy")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"my-feature\", project: \"myproject\")")
	assert.Contains(t, prompt, "kas signal emit elaborator_finished my-feature")
}
