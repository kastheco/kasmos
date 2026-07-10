package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/internal/initcmd/scaffold"
)

func TestNewScaffoldCmd_HasSyncSubcommand(t *testing.T) {
	cmd := newScaffoldCmd()
	assert.Equal(t, "scaffold", cmd.Use)
	sub, _, err := cmd.Find([]string{"sync"})
	require.NoError(t, err)
	assert.Equal(t, "sync", sub.Use)
	worktreeSub, _, err := cmd.Find([]string{"worktree"})
	require.NoError(t, err)
	assert.Equal(t, "worktree [path]", worktreeSub.Use)
}

// Note: TestProfilesToAgentConfigs* tests have been moved to
// internal/initcmd/scaffoldsync/runner_test.go since that logic now
// belongs to the library package.

func TestResolveCheckoutRoot_RegularRepo(t *testing.T) {
	// Regular repo: .git directory at root.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o755))

	// Resolves from root itself.
	got, err := resolveCheckoutRoot(root)
	require.NoError(t, err)
	assert.Equal(t, root, got)

	// Resolves from a subdirectory — must walk up to root, not return the subdir.
	sub := filepath.Join(root, "pkg", "foo")
	require.NoError(t, os.MkdirAll(sub, 0o755))
	got, err = resolveCheckoutRoot(sub)
	require.NoError(t, err)
	assert.Equal(t, root, got)
}

func TestResolveCheckoutRoot_Worktree(t *testing.T) {
	// Worktree: .git is a file, not a directory.
	// resolveCheckoutRoot must stop here (not navigate to the main repo root).
	wt := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: /main/.git/worktrees/wt"), 0o644))

	got, err := resolveCheckoutRoot(wt)
	require.NoError(t, err)
	assert.Equal(t, wt, got, "must return worktree root, not main repo root")
}

func TestResolveCheckoutRoot_NotARepo(t *testing.T) {
	dir := filepath.Join(string(os.PathSeparator), "kasmos-definitely-not-a-repo", "nested")
	_, err := resolveCheckoutRoot(dir)
	assert.Error(t, err)
}

func TestScaffoldSync_RequiresTomlConfig(t *testing.T) {
	// Isolate HOME so GetConfigDir() migration cannot copy legacy config.toml
	// from ~/.config/kasmos (or similar) into the temp project dir.
	t.Setenv("HOME", t.TempDir())

	dir := t.TempDir()
	t.Chdir(dir)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))

	var buf bytes.Buffer
	cmd := newScaffoldSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err := cmd.RunE(cmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestScaffoldWorktree_WritesMissingAgentFiles(t *testing.T) {
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.fixer]
    enabled = true
    program = "opencode"
    model = "openai/gpt-5.4"

[phases]
  fixer = "fixer"
`), 0o644))

	worktree := filepath.Join(mainRepo, ".worktrees", "plan-test")
	require.NoError(t, os.MkdirAll(worktree, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: /tmp/fake\n"), 0o644))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(mainRepo))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldCwd)) })

	var buf bytes.Buffer
	cmd := newScaffoldWorktreeCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	err = cmd.RunE(cmd, []string{worktree})
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(worktree, "opencode.jsonc"))
	assert.FileExists(t, filepath.Join(worktree, ".opencode", "agents", "fixer.md"))
	assert.FileExists(t, filepath.Join(worktree, ".agents", "skills", "kasmos-fixer", "SKILL.md"))
	assert.Contains(t, buf.String(), "Syncing worktree scaffold")
}

func TestListExistingWorktrees_FiltersDirectoriesWithGitMarker(t *testing.T) {
	repo := t.TempDir()
	wtDir := filepath.Join(repo, ".worktrees")
	require.NoError(t, os.MkdirAll(wtDir, 0o755))

	keepA := filepath.Join(wtDir, "a")
	keepB := filepath.Join(wtDir, "b")
	skipFile := filepath.Join(wtDir, "README")
	skipDir := filepath.Join(wtDir, "not-a-worktree")

	require.NoError(t, os.MkdirAll(keepA, 0o755))
	require.NoError(t, os.MkdirAll(keepB, 0o755))
	require.NoError(t, os.MkdirAll(skipDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(keepA, ".git"), []byte("gitdir: /tmp/a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(keepB, ".git"), []byte("gitdir: /tmp/b\n"), 0o644))
	require.NoError(t, os.WriteFile(skipFile, []byte("x"), 0o644))

	worktrees, err := listExistingWorktrees(repo)
	require.NoError(t, err)
	assert.Equal(t, []string{keepA, keepB}, worktrees)
}

func TestScaffoldSync_WorktreesFlagSyncsSiblingWorktrees(t *testing.T) {
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.fixer]
    enabled = true
    program = "opencode"
    model = "openai/gpt-5.4"

[phases]
  fixer = "fixer"
`), 0o644))

	worktreeA := filepath.Join(mainRepo, ".worktrees", "plan-a")
	worktreeB := filepath.Join(mainRepo, ".worktrees", "plan-b")
	require.NoError(t, os.MkdirAll(worktreeA, 0o755))
	require.NoError(t, os.MkdirAll(worktreeB, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeA, ".git"), []byte("gitdir: /tmp/fake-a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeB, ".git"), []byte("gitdir: /tmp/fake-b\n"), 0o644))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(mainRepo))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldCwd)) })

	var buf bytes.Buffer
	cmd := newScaffoldSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--worktrees"})
	require.NoError(t, cmd.Execute())

	assert.FileExists(t, filepath.Join(mainRepo, "opencode.jsonc"))
	assert.FileExists(t, filepath.Join(worktreeA, "opencode.jsonc"))
	assert.FileExists(t, filepath.Join(worktreeB, "opencode.jsonc"))
	assert.Contains(t, buf.String(), "Syncing worktree scaffold")
}

func TestScaffoldSync_RefreshesSignalPromptCopies(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "planner", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
		{Role: "planner", Harness: "opencode", Model: "anthropic/claude-opus-4-6", Enabled: true},
		{Role: "reviewer", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
	}

	_, err := scaffold.ScaffoldAll(dir, agents, nil, false)
	require.NoError(t, err)

	targets := []struct {
		path       string
		sourcePath string
		model      string
	}{
		{
			path:       filepath.Join(dir, ".claude", "agents", "planner.md"),
			sourcePath: filepath.Join("..", "..", "internal", "initcmd", "scaffold", "templates", "claude", "agents", "planner.md"),
			model:      "claude-opus-4-6",
		},
		{
			path:       filepath.Join(dir, ".opencode", "agents", "planner.md"),
			sourcePath: filepath.Join("..", "..", "internal", "initcmd", "scaffold", "templates", "opencode", "agents", "planner.md"),
		},
		{
			path:       filepath.Join(dir, ".agents", "skills", "kasmos-architect", "SKILL.md"),
			sourcePath: filepath.Join("..", "..", "internal", "initcmd", "scaffold", "templates", "skills", "kasmos-architect", "SKILL.md"),
		},
		{
			path:       filepath.Join(dir, ".agents", "skills", "kasmos-reviewer", "SKILL.md"),
			sourcePath: filepath.Join("..", "..", "internal", "initcmd", "scaffold", "templates", "skills", "kasmos-reviewer", "SKILL.md"),
		},
		{
			path:       filepath.Join(dir, ".agents", "skills", "kasmos-master", "SKILL.md"),
			sourcePath: filepath.Join("..", "..", "internal", "initcmd", "scaffold", "templates", "skills", "kasmos-master", "SKILL.md"),
		},
	}

	for _, target := range targets {
		require.NoError(t, os.WriteFile(target.path, []byte("stale signal instructions\n"), 0o644))
	}

	var buf bytes.Buffer
	err = syncScaffoldTarget(&buf, "Syncing scaffold", dir, agents)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, ".claude/agents/planner.md")
	assert.Contains(t, output, ".opencode/agents/planner.md")
	assert.Contains(t, output, ".agents/skills/kasmos-architect/SKILL.md")
	assert.Contains(t, output, ".agents/skills/kasmos-reviewer/SKILL.md")
	assert.Contains(t, output, ".agents/skills/kasmos-master/SKILL.md")

	for _, target := range targets {
		expected, err := os.ReadFile(target.sourcePath)
		require.NoError(t, err)

		want := string(expected)
		if target.model != "" {
			want = strings.ReplaceAll(want, "{{MODEL}}", target.model)
		}

		got, err := os.ReadFile(target.path)
		require.NoError(t, err)
		assert.Equal(t, want, string(got), target.path)
	}

	reviewerSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "kasmos-reviewer", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(reviewerSkill), "Do not write legacy `.kasmos/signals/review-*` files directly.")

	architectSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "kasmos-architect", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(architectSkill), "use MCP `signal_create` (signal_type: \"elaborator-finished\", plan_file: \"<plan-file>\", project: \"$KASMOS_PROJECT\") after the round-trip check succeeds.")
	assert.Contains(t, string(architectSkill), "Before validating the planner draft, create an independent solution baseline")
	assert.Contains(t, string(architectSkill), "compare planner vs architect baseline and merge")
	assert.Contains(t, string(architectSkill), "## planner draft cache mode")
	assert.Contains(t, string(architectSkill), ".kasmos/cache/<plan-file>-planner-*.md")
	assert.Contains(t, string(architectSkill), ".kasmos/cache/<plan-file>-architect.json")
	assert.Contains(t, string(architectSkill), "decision_audit")
	assert.Contains(t, string(architectSkill), "planner_summary")
	assert.Contains(t, string(architectSkill), "baseline_summary")
	assert.Contains(t, string(architectSkill), "final_decision")
	assert.Contains(t, string(architectSkill), "`baseline_source`: one of `planner_drafts`, `inline`, `absent`, or `stale`")
	assert.Contains(t, string(architectSkill), "`planner_drafts`: list the planner draft cache paths, profile ids, and decisions used for the final result")
	assert.Contains(t, string(architectSkill), "kas task validate-architect-meta <plan-file>")
	assert.NotContains(t, string(architectSkill), "architect-baseline")
	assert.NotContains(t, string(architectSkill), "<plan-file>-architect-baseline.json")
	assert.NotContains(t, string(architectSkill), "parallel_planner_architect")
}

// TestScaffoldSync_EnforcementDisabled_UninstallsClaudeHook verifies that when
// [enforcement] claude = false is set in config.toml, the scaffold sync command
// removes the globally-installed claude enforcement hook instead of installing it.
func TestScaffoldSync_EnforcementDisabled_UninstallsClaudeHook(t *testing.T) {
	// Isolate HOME so UserHomeDir() returns our temp dir.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Put a fake "claude" binary in PATH so harness.Detect() succeeds.
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir)

	// Pre-seed the enforcement hook under HOME so UninstallEnforcement has something to remove.
	hookDir := filepath.Join(tmpHome, ".claude", "hooks")
	require.NoError(t, os.MkdirAll(hookDir, 0o755))
	hookFile := filepath.Join(hookDir, "enforce-cli-tools.sh")
	require.NoError(t, os.WriteFile(hookFile, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	settingsDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0o755))
	settingsContent := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + hookFile + `"}]}]}}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsContent), 0o644))

	// Create a repo with config.toml that disables claude enforcement.
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "claude"
    model = "claude-sonnet-4-6"

[enforcement]
  claude = false
`), 0o644))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(mainRepo))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldCwd)) })

	var buf bytes.Buffer
	cmd := newScaffoldSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	require.NoError(t, cmd.RunE(cmd, nil))

	output := buf.String()
	assert.Contains(t, output, "REMOVED (enforcement disabled)", "expected disabled message in output")
	assert.NoFileExists(t, hookFile, "enforcement hook script should have been removed")
}

// TestScaffoldSync_EnforcementDisabled_RemovesStaleCodexArtifacts verifies that
// `kas scaffold sync` honours [enforcement] codex = false even when no codex
// agents are enabled by removing stale .codex/hooks/enforce-cli-tools.sh and
// the kasmos PreToolUse entry left behind from a previous setup. Regression
// guard for the gating bug Copilot flagged on PR #123.
func TestScaffoldSync_EnforcementDisabled_RemovesStaleCodexArtifacts(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Put a fake "claude" binary in PATH so harness.Detect() succeeds for the
	// enabled claude profile. Codex is intentionally absent — no codex agents
	// will be passed into the scaffold pipeline.
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir)

	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "claude"
    model = "claude-sonnet-4-6"

[enforcement]
  codex = false
`), 0o644))

	// Pre-seed stale codex enforcement artifacts as if a previous setup had
	// installed them with enforcement enabled.
	codexHooksDir := filepath.Join(mainRepo, ".codex", "hooks")
	require.NoError(t, os.MkdirAll(codexHooksDir, 0o755))
	staleScript := filepath.Join(codexHooksDir, "enforce-cli-tools.sh")
	require.NoError(t, os.WriteFile(staleScript, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	staleHooksJSON := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": ".codex/hooks/enforce-cli-tools.sh" }
        ]
      }
    ]
  }
}`
	staleHooksJSONPath := filepath.Join(mainRepo, ".codex", "hooks.json")
	require.NoError(t, os.WriteFile(staleHooksJSONPath, []byte(staleHooksJSON), 0o644))

	oldCwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(mainRepo))
	t.Cleanup(func() { require.NoError(t, os.Chdir(oldCwd)) })

	var buf bytes.Buffer
	cmd := newScaffoldSyncCmd()
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)

	require.NoError(t, cmd.RunE(cmd, nil))

	assert.NoFileExists(t, staleScript,
		"stale codex enforcement script should be removed when codex = false and no codex agents are configured")

	hooksData, err := os.ReadFile(staleHooksJSONPath)
	require.NoError(t, err)
	assert.NotContains(t, string(hooksData), "enforce-cli-tools.sh",
		"stale kasmos PreToolUse entry should be stripped from .codex/hooks.json")

	// .codex/AGENTS.md must NOT be created — no codex agents were configured.
	assert.NoFileExists(t, filepath.Join(mainRepo, ".codex", "AGENTS.md"),
		"cleanup-only path must not create new codex scaffold files")
}
