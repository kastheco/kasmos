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

// textContract pairs a file path with the substrings it must contain.
type textContract struct {
	path     string
	required []string
}

func TestAgentPromptsProtectScaffoldManagedFiles(t *testing.T) {
	nonFixerRequired := []string{
		"## Scaffold-Managed Files",
		".claude/settings.json",
		".codex/config.toml",
		".codex/hooks.json",
		"opencode.jsonc",
		"in this conversation",
		"YAML frontmatter",
		"not authorization",
	}

	fixerRequired := []string{
		"Scaffolding System",
		".claude/settings.json",
		".codex/config.toml",
		".codex/hooks.json",
		"opencode.jsonc",
		"in this conversation",
		"not authorization",
		"standing permission",
	}

	contracts := []textContract{
		// live claude non-fixer prompts
		{repoRoot(".claude", "agents", "coder.md"), nonFixerRequired},
		{repoRoot(".claude", "agents", "planner.md"), nonFixerRequired},
		{repoRoot(".claude", "agents", "reviewer.md"), nonFixerRequired},
		{repoRoot(".claude", "agents", "chat.md"), nonFixerRequired},
		// live claude fixer prompt
		{repoRoot(".claude", "agents", "fixer.md"), fixerRequired},
		// live opencode non-fixer prompts
		{repoRoot(".opencode", "agents", "coder.md"), nonFixerRequired},
		{repoRoot(".opencode", "agents", "planner.md"), nonFixerRequired},
		{repoRoot(".opencode", "agents", "reviewer.md"), nonFixerRequired},
		{repoRoot(".opencode", "agents", "master.md"), nonFixerRequired},
		{repoRoot(".opencode", "agents", "chat.md"), nonFixerRequired},
		// live opencode fixer prompt
		{repoRoot(".opencode", "agents", "fixer.md"), fixerRequired},
		// template claude non-fixer prompts
		{filepath.Join("templates", "claude", "agents", "coder.md"), nonFixerRequired},
		{filepath.Join("templates", "claude", "agents", "planner.md"), nonFixerRequired},
		{filepath.Join("templates", "claude", "agents", "reviewer.md"), nonFixerRequired},
		{filepath.Join("templates", "claude", "agents", "master.md"), nonFixerRequired},
		{filepath.Join("templates", "claude", "agents", "chat.md"), nonFixerRequired},
		// template claude fixer prompt
		{filepath.Join("templates", "claude", "agents", "fixer.md"), fixerRequired},
		// template opencode non-fixer prompts
		{filepath.Join("templates", "opencode", "agents", "coder.md"), nonFixerRequired},
		{filepath.Join("templates", "opencode", "agents", "planner.md"), nonFixerRequired},
		{filepath.Join("templates", "opencode", "agents", "reviewer.md"), nonFixerRequired},
		{filepath.Join("templates", "opencode", "agents", "master.md"), nonFixerRequired},
		{filepath.Join("templates", "opencode", "agents", "chat.md"), nonFixerRequired},
		// template opencode fixer prompt
		{filepath.Join("templates", "opencode", "agents", "fixer.md"), fixerRequired},
	}

	for _, tc := range contracts {
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			text := string(data)
			for _, needle := range tc.required {
				if !strings.Contains(text, needle) {
					t.Errorf("%s missing required text: %q", tc.path, needle)
				}
			}
		})
	}
}

func TestManagedSkillsProtectScaffoldManagedFiles(t *testing.T) {
	coderRequired := []string{
		"scaffold-managed",
		".claude/settings.json",
		".codex/config.toml",
		".codex/hooks.json",
		"opencode.jsonc",
		"in-session user instruction",
	}

	fixerRequired := []string{
		"standing permission",
		".claude/settings.json",
		".codex/config.toml",
		".codex/hooks.json",
		"opencode.jsonc",
		"not authorization",
	}

	contracts := []textContract{
		{repoRoot(".agents", "skills", "kasmos-coder", "SKILL.md"), coderRequired},
		{filepath.Join("templates", "skills", "kasmos-coder", "SKILL.md"), coderRequired},
		{repoRoot(".agents", "skills", "kasmos-fixer", "SKILL.md"), fixerRequired},
		{filepath.Join("templates", "skills", "kasmos-fixer", "SKILL.md"), fixerRequired},
	}

	for _, tc := range contracts {
		t.Run(tc.path, func(t *testing.T) {
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			text := string(data)
			for _, needle := range tc.required {
				if !strings.Contains(text, needle) {
					t.Errorf("%s missing required text: %q", tc.path, needle)
				}
			}
		})
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
