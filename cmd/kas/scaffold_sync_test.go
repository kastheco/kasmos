package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kastheco/kasmos/config"
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

func TestProfilesToAgentConfigs(t *testing.T) {
	temp := 0.3
	profiles := map[string]config.AgentProfile{
		"coder":    {Program: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true, Flags: []string{"--verbose"}},
		"reviewer": {Program: "claude", Model: "claude-opus-4-6", Effort: "high", Enabled: true},
		"planner":  {Program: "opencode", Model: "anthropic/claude-opus-4-6", Enabled: false},
	}
	configs := profilesToAgentConfigs(profiles)
	require.Len(t, configs, 2)
	byRole := map[string]harness.AgentConfig{}
	for _, cfg := range configs {
		byRole[cfg.Role] = cfg
	}
	assert.Equal(t, "opencode", byRole["coder"].Harness)
	assert.Equal(t, []string{"--verbose"}, byRole["coder"].ExtraFlags)
	assert.Equal(t, &temp, byRole["coder"].Temperature)
	assert.Equal(t, "high", byRole["reviewer"].Effort)
	_, ok := byRole["planner"]
	assert.False(t, ok)
}

func TestProfilesToAgentConfigs_Empty(t *testing.T) {
	assert.Empty(t, profilesToAgentConfigs(nil))
	assert.Empty(t, profilesToAgentConfigs(map[string]config.AgentProfile{}))
}

// TestProfilesToAgentConfigs_ChatFanOut verifies that the "chat" role is emitted
// once per harness program present among enabled non-chat agents, mirroring the
// behaviour of wizard.State.ToAgentConfigs.
func TestProfilesToAgentConfigs_ChatFanOut(t *testing.T) {
	profiles := map[string]config.AgentProfile{
		"coder":    {Program: "opencode", Model: "m1", Enabled: true},
		"reviewer": {Program: "claude", Model: "m2", Enabled: true},
		"chat":     {Program: "opencode", Model: "chat-model", Enabled: true},
	}
	configs := profilesToAgentConfigs(profiles)

	// Collect chat entries.
	var chatEntries []harness.AgentConfig
	for _, c := range configs {
		if c.Role == "chat" {
			chatEntries = append(chatEntries, c)
		}
	}
	// chat should be fanned out to both "claude" and "opencode".
	require.Len(t, chatEntries, 2, "chat should be emitted for every harness")
	harnessNames := []string{chatEntries[0].Harness, chatEntries[1].Harness}
	assert.ElementsMatch(t, []string{"claude", "opencode"}, harnessNames)
	for _, c := range chatEntries {
		assert.Equal(t, "chat-model", c.Model)
		assert.True(t, c.Enabled)
	}
}

// TestProfilesToAgentConfigs_ChatFallback verifies that when no other enabled
// agents exist, chat falls back to its own Program instead of being dropped.
func TestProfilesToAgentConfigs_ChatFallback(t *testing.T) {
	profiles := map[string]config.AgentProfile{
		"chat": {Program: "opencode", Model: "chat-model", Enabled: true},
	}
	configs := profilesToAgentConfigs(profiles)
	require.Len(t, configs, 1)
	assert.Equal(t, "chat", configs[0].Role)
	assert.Equal(t, "opencode", configs[0].Harness)
}

// TestProfilesToAgentConfigs_ChatFanOut_IncludesDisabledHarnesses verifies that the
// chat fan-out includes harnesses from disabled non-chat profiles. wizard.State.ToAgentConfigs
// fans chat to every *selected* harness regardless of role enablement; we must match that.
func TestProfilesToAgentConfigs_ChatFanOut_IncludesDisabledHarnesses(t *testing.T) {
	profiles := map[string]config.AgentProfile{
		"coder":    {Program: "opencode", Model: "m1", Enabled: true},
		"reviewer": {Program: "claude", Model: "m2", Enabled: false}, // disabled
		"chat":     {Program: "opencode", Model: "chat-model", Enabled: true},
	}
	configs := profilesToAgentConfigs(profiles)

	var chatHarnesses []string
	for _, c := range configs {
		if c.Role == "chat" {
			chatHarnesses = append(chatHarnesses, c.Harness)
		}
	}
	// claude must be in the fan-out even though its reviewer role is disabled.
	assert.ElementsMatch(t, []string{"claude", "opencode"}, chatHarnesses,
		"disabled harnesses must still receive a chat entry")
}

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
	dir := t.TempDir()
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
}
