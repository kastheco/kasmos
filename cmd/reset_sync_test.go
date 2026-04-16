package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultRepoResetSync_EnforcementDisabled_UninstallsClaudeHook verifies that
// when [enforcement] claude = false is set in config.toml, defaultRepoResetSync
// removes the globally-installed claude enforcement hook instead of installing it.
func TestDefaultRepoResetSync_EnforcementDisabled_UninstallsClaudeHook(t *testing.T) {
	// Isolate HOME so os.UserHomeDir() returns our temp dir.
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
	settingsContent := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"` + hookFile + `"}]}]}}`
	require.NoError(t, os.WriteFile(filepath.Join(settingsDir, "settings.json"), []byte(settingsContent), 0o644))

	// Create a repo with config.toml that disables claude enforcement.
	repo := makeFakeRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "claude"
    model = "claude-sonnet-4-6"

[enforcement]
  claude = false
`), 0o644))

	var buf bytes.Buffer
	err := defaultRepoResetSync(repo, &buf)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "REMOVED (enforcement disabled)", "expected disabled message in output")
	assert.NoFileExists(t, hookFile, "enforcement hook script should have been removed")
}

// TestDefaultRepoResetSync_EnforcementEnabled_InstallsHook verifies that when no
// [enforcement] section is present (default), defaultRepoResetSync installs the hook.
func TestDefaultRepoResetSync_EnforcementEnabled_InstallsHook(t *testing.T) {
	// Isolate HOME so os.UserHomeDir() returns our temp dir.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Put a fake "claude" binary in PATH so harness.Detect() succeeds.
	binDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir)

	// Create a repo with config.toml that has no enforcement section (default = enabled).
	repo := makeFakeRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(repo, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(repo, ".kasmos", "config.toml"), []byte(`
[agents]
  [agents.coder]
    enabled = true
    program = "claude"
    model = "claude-sonnet-4-6"
`), 0o644))

	var buf bytes.Buffer
	err := defaultRepoResetSync(repo, &buf)
	require.NoError(t, err)

	output := buf.String()
	// Default enforcement (no [enforcement] section) keeps hooks installed.
	assert.Contains(t, output, "OK", "expected OK for installed enforcement hook")
	assert.NotContains(t, output, "REMOVED (enforcement disabled)")

	// Hook script should have been created.
	hookFile := filepath.Join(tmpHome, ".claude", "hooks", "enforce-cli-tools.sh")
	assert.FileExists(t, hookFile)
}
