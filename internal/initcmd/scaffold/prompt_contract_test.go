package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot returns the path to the repository root relative to this package.
// From internal/initcmd/scaffold/ we go up three levels.
func repoRoot(elem ...string) string {
	return filepath.Join(append([]string{"..", "..", ".."}, elem...)...)
}

func TestCoderPromptParallelSection(t *testing.T) {
	coderFiles := []string{
		repoRoot(".opencode", "agents", "coder.md"),
		repoRoot(".claude", "agents", "coder.md"),
		filepath.Join("templates", "opencode", "agents", "coder.md"),
		filepath.Join("templates", "claude", "agents", "coder.md"),
	}

	required := []string{
		"## Parallel Execution",
		"KASMOS_TASK",
		"shared worktree",
	}

	for _, f := range coderFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read coder prompt %s: %v", f, err)
		}
		text := string(data)

		for _, needle := range required {
			if !strings.Contains(text, needle) {
				t.Errorf("%s missing required text: %q", f, needle)
			}
		}
	}
}

func TestCoderPromptMinimal(t *testing.T) {
	coderFiles := []string{
		repoRoot(".opencode", "agents", "coder.md"),
		repoRoot(".claude", "agents", "coder.md"),
		filepath.Join("templates", "opencode", "agents", "coder.md"),
		filepath.Join("templates", "claude", "agents", "coder.md"),
	}

	forbidden := []string{
		"kasmos-coder",
		"cli-tools",
		"Load the",
	}

	required := []string{
		"KASMOS_TASK",
		"commit",
		"## Parallel Execution",
	}

	for _, f := range coderFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read coder prompt %s: %v", f, err)
		}
		text := string(data)

		for _, needle := range forbidden {
			if strings.Contains(text, needle) {
				t.Errorf("%s must not contain: %q", f, needle)
			}
		}

		for _, needle := range required {
			if !strings.Contains(text, needle) {
				t.Errorf("%s missing required text: %q", f, needle)
			}
		}
	}
}

func TestPlannerPromptBranchPolicy(t *testing.T) {
	data, err := os.ReadFile(repoRoot(".opencode", "agents", "planner.md"))
	if err != nil {
		t.Fatalf("read planner prompt: %v", err)
	}
	text := string(data)

	required := []string{
		"Always commit task files to the main branch.",
		"Do NOT create feature branches for planning work.",
		"Only register implementation plans",
		"never register design docs",
		"task_update_content",
		"planner-finished",
		"KASMOS_MANAGED",
		"Never modify task state directly",
		"plan review",                // planner must reference the review step
		`project: "$KASMOS_PROJECT"`, // project arg must be shown in MCP tool examples
	}

	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("planner prompt missing required policy text: %q", needle)
		}
	}

	if strings.Contains(text, "YYYY-MM-DD") {
		t.Fatalf("planner prompt still references date prefix convention YYYY-MM-DD")
	}

	if strings.Contains(text, "kasmos will detect this and register the plan") {
		t.Fatalf("planner prompt still claims the sentinel registers plan content")
	}
}
