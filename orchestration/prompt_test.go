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
	assert.Contains(t, prompt, "Compare planner vs architect baseline")
	assert.Contains(t, prompt, "merging the best")
	assert.Contains(t, prompt, "hidden integration surfaces")
	assert.Contains(t, prompt, "non-obvious missing work")
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
	assert.NotContains(t, prompt, "architect-baseline.json")
}

func TestBuildArchitectBaselinePrompt(t *testing.T) {
	description := "build the parallel planner architect baseline"
	descriptionHash := ArchitectBaselineDescriptionHash(description)

	prompt := BuildArchitectBaselinePrompt("my-feature", "myproject", description)

	assert.Contains(t, prompt, "kasmos-architect")
	assert.Contains(t, prompt, "cli-tools")
	assert.Contains(t, prompt, "Inspect the live codebase independently from planner output")
	assert.Contains(t, prompt, "goal")
	assert.Contains(t, prompt, "surfaces")
	assert.Contains(t, prompt, "dependencies")
	assert.Contains(t, prompt, "patterns")
	assert.Contains(t, prompt, ".kasmos/cache/my-feature-architect-baseline.json")
	assert.Contains(t, prompt, `"schema_version": 1`)
	assert.Contains(t, prompt, `"plan_file": "my-feature"`)
	assert.Contains(t, prompt, `"project": "myproject"`)
	assert.Contains(t, prompt, `"description_hash": "`+descriptionHash+`"`)
	assert.Contains(t, prompt, `"baseline_markdown":`)
	assert.Contains(t, prompt, "Stop after the cache write")
	assert.Contains(t, prompt, "Do not edit any file except `.kasmos/cache/my-feature-architect-baseline.json`")
	assert.Contains(t, prompt, "Forbidden: MCP `task_update_content`")
	assert.Contains(t, prompt, "task status transitions")
	assert.Contains(t, prompt, "planner-finished")
	assert.Contains(t, prompt, "architect-finished")
	assert.Contains(t, prompt, "elaborator-finished")
	assert.Contains(t, prompt, "Do not mutate task content, task status, or orchestration state")
	assert.NotContains(t, prompt, "use MCP `task_update_content`")
	assert.NotContains(t, prompt, "signal_create")
}

func TestBuildElaborationPromptWithOptions_ParallelBaseline(t *testing.T) {
	prompt := BuildElaborationPromptWithOptions("my-feature", "myproject", ArchitectPromptOptions{
		ParallelBaseline: true,
		DescriptionHash:  "abc123",
	})

	assert.Contains(t, prompt, "task_show")
	assert.Contains(t, prompt, ".kasmos/cache/my-feature-architect-baseline.json")
	assert.Contains(t, prompt, "`plan_file` equals \"my-feature\"")
	assert.Contains(t, prompt, "`project` equals \"myproject\"")
	assert.Contains(t, prompt, "`description_hash` equals \"abc123\"")
	assert.Contains(t, prompt, "`schema_version` equals 1")
	assert.Contains(t, prompt, "`baseline_markdown` is non-empty")
	assert.Contains(t, prompt, "advisory input, not authoritative implementation state")
	assert.Contains(t, prompt, "merge the planner draft plus cached baseline")
	assert.Contains(t, prompt, "missing, corrupt, stale, or incomplete")
	assert.Contains(t, prompt, "current inline independent baseline")
	assert.Contains(t, prompt, "mention that fallback in the plan summary")
	assert.Contains(t, prompt, "task_update_content")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"my-feature\", project: \"myproject\")")
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
	assert.Contains(t, prompt, "architect-finished")
	assert.Contains(t, prompt, "architect-v1.json")
	assert.Contains(t, prompt, "parallel")
	// Signal completion should prefer MCP with filesystem fallback
	assert.Contains(t, prompt, "signal_create")
}

func TestBuildElaborationPrompt_RetainsLegacySignalName(t *testing.T) {
	prompt := BuildElaborationPrompt("my-feature", "myproject")

	assert.Contains(t, prompt, "only the completion signal name stays legacy")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"elaborator-finished\", plan_file: \"my-feature\", project: \"myproject\")")
	assert.Contains(t, prompt, "kas signal emit elaborator_finished my-feature")
}
