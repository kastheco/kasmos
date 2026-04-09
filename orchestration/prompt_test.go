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
	// MCP-first readiness signal emission
	assert.Contains(t, prompt, "signal_create` (signal_type: \"readiness-approved\"")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"readiness-changes-requested\"")
	// CLI fallback
	assert.Contains(t, prompt, "kas signal emit readiness_approved my-feature")
	assert.Contains(t, prompt, "kas signal emit readiness_changes_requested my-feature")
	// Evidence gathering
	assert.Contains(t, prompt, "Merge-base diff")
	assert.Contains(t, prompt, "MERGE_BASE")
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
	assert.Contains(t, prompt, "Preserve")
	// Must reference reading the codebase
	assert.Contains(t, prompt, "codebase")
}

func TestBuildArchitectPrompt(t *testing.T) {
	prompt := BuildArchitectPrompt("my-feature", "myproject")

	assert.Contains(t, prompt, "kasmos-architect")
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
