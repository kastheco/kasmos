package scaffoldsync

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
)

// ----------------------------------------------------------------------------
// profilesToAgentConfigs (moved from cmd/kas/scaffold_sync_test.go)
// ----------------------------------------------------------------------------

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
// once per harness program present among all non-chat profiles.
func TestProfilesToAgentConfigs_ChatFanOut(t *testing.T) {
	profiles := map[string]config.AgentProfile{
		"coder":    {Program: "opencode", Model: "m1", Enabled: true},
		"reviewer": {Program: "claude", Model: "m2", Enabled: true},
		"chat":     {Program: "opencode", Model: "chat-model", Enabled: true},
	}
	configs := profilesToAgentConfigs(profiles)

	var chatEntries []harness.AgentConfig
	for _, c := range configs {
		if c.Role == "chat" {
			chatEntries = append(chatEntries, c)
		}
	}
	require.Len(t, chatEntries, 2, "chat should be emitted for every harness")
	harnessNames := []string{chatEntries[0].Harness, chatEntries[1].Harness}
	assert.ElementsMatch(t, []string{"claude", "opencode"}, harnessNames)
	for _, c := range chatEntries {
		assert.Equal(t, "chat-model", c.Model)
		assert.True(t, c.Enabled)
	}
}

// TestProfilesToAgentConfigs_ChatFallback verifies that when no other enabled
// agents exist, chat falls back to its own Program.
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
// chat fan-out includes harnesses from disabled non-chat profiles.
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
	assert.ElementsMatch(t, []string{"claude", "opencode"}, chatHarnesses,
		"disabled harnesses must still receive a chat entry")
}

// ----------------------------------------------------------------------------
// ProjectAgentConfigs
// ----------------------------------------------------------------------------

func TestProjectAgentConfigs_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	_, err := ProjectAgentConfigs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no config found")
}

func TestProjectAgentConfigs_ZeroEnabledAgents(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = false
    program = "claude"
    model = "claude-sonnet-4-6"
`), 0o644))

	_, err := ProjectAgentConfigs(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled agents")
}

func TestProjectAgentConfigs_LoadsAgents(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "opencode"
    model = "openai/gpt-5.4"
`), 0o644))

	agents, err := ProjectAgentConfigs(dir)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	assert.Equal(t, "coder", agents[0].Role)
	assert.Equal(t, "opencode", agents[0].Harness)
}

// ----------------------------------------------------------------------------
// Run — IncludeWorktrees
// ----------------------------------------------------------------------------

func TestRun_IncludeWorktrees_SyncsSiblingWorktrees(t *testing.T) {
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.fixer]
    enabled = true
    program = "opencode"
    model = "openai/gpt-5.4"
`), 0o644))

	worktreeA := filepath.Join(mainRepo, ".worktrees", "plan-a")
	worktreeB := filepath.Join(mainRepo, ".worktrees", "plan-b")
	require.NoError(t, os.MkdirAll(worktreeA, 0o755))
	require.NoError(t, os.MkdirAll(worktreeB, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeA, ".git"), []byte("gitdir: /tmp/fake-a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(worktreeB, ".git"), []byte("gitdir: /tmp/fake-b\n"), 0o644))

	var buf bytes.Buffer
	err := Run(Options{
		RepoRoot:         mainRepo,
		IncludeWorktrees: true,
		Out:              &buf,
	})
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(mainRepo, "opencode.jsonc"))
	assert.FileExists(t, filepath.Join(worktreeA, "opencode.jsonc"))
	assert.FileExists(t, filepath.Join(worktreeB, "opencode.jsonc"))

	output := buf.String()
	assert.Contains(t, output, "Syncing scaffold:")
	assert.Contains(t, output, "Syncing worktree scaffold:")
	assert.Contains(t, output, "done.")
}

// ----------------------------------------------------------------------------
// Run — Trust
// ----------------------------------------------------------------------------

func TestRun_Trust_WritesToTempHomeDir(t *testing.T) {
	// A fake codex binary so harness.Detect() finds "codex" in PATH, but we also
	// need a codex-enabled agent in config.
	// Actually, Trust only does anything when agents contain harness "codex".
	// So set up a config with a codex agent.
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "codex"
    model = "openai/gpt-5.4"
`), 0o644))

	tmpHome := t.TempDir()

	var buf bytes.Buffer
	err := Run(Options{
		RepoRoot: mainRepo,
		Trust:    true,
		HomeDir:  tmpHome,
		Out:      &buf,
	})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Trusting project for codex...")
	// The codex trusted projects file should have been created.
	assert.FileExists(t, filepath.Join(tmpHome, ".codex", "config.toml"))
}

func TestRun_Trust_HomeDirFail_NonFatal(t *testing.T) {
	// Simulate home directory discovery failure by pointing HOME at a path that
	// makes os.UserHomeDir fail. We use an invalid HOME env var.
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "codex"
    model = "openai/gpt-5.4"
`), 0o644))

	// Use an empty HomeDir so resolveHomeDir falls back to os.UserHomeDir().
	// Unset HOME so it fails (on Linux UserHomeDir uses $HOME).
	t.Setenv("HOME", "")

	var buf bytes.Buffer
	// Should not return an error even when home dir fails.
	err := Run(Options{
		RepoRoot: mainRepo,
		Trust:    true,
		Out:      &buf,
	})
	// Run itself must not fail (trust is non-fatal).
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "WARNING: could not get home dir")
}

// ----------------------------------------------------------------------------
// Run — enforcement disabled
// ----------------------------------------------------------------------------

func TestRun_EnforcementDisabled_DoesNotAbort(t *testing.T) {
	// Put a fake "claude" binary in PATH so harness.Detect() succeeds.
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir)
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

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

	// Pre-seed settings.json so UninstallEnforcement has something to act on.
	settingsDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(settingsDir, 0o755))
	hookDir := filepath.Join(settingsDir, "hooks")
	require.NoError(t, os.MkdirAll(hookDir, 0o755))
	hookFile := filepath.Join(hookDir, "enforce-cli-tools.sh")
	require.NoError(t, os.WriteFile(hookFile, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	settingsContent := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + hookFile + `"}]}]}}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsContent), 0o644))

	var buf bytes.Buffer
	err := Run(Options{
		RepoRoot: mainRepo,
		HomeDir:  tmpHome,
		Out:      &buf,
	})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Configuring enforcement hooks...")
	assert.Contains(t, output, "REMOVED (enforcement disabled)")
	assert.NoFileExists(t, hookFile)
}

// ----------------------------------------------------------------------------
// Run — nil Out defaults to io.Discard (no panic)
// ----------------------------------------------------------------------------

func TestRun_NilOut_UsesDiscard(t *testing.T) {
	mainRepo := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(mainRepo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(mainRepo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "opencode"
    model = "openai/gpt-5.4"
`), 0o644))

	// Out is nil — must not panic.
	err := Run(Options{
		RepoRoot: mainRepo,
	})
	require.NoError(t, err)
}
