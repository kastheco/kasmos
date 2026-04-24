package harness

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runCLIToolsEnforcementScript(t *testing.T, command string) (int, string) {
	t.Helper()

	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "enforce-cli-tools.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte(CLIToolsEnforcementScript), 0o755))

	jqPath := filepath.Join(dir, "jq")
	const fakeJQ = `#!/bin/sh
input=$(cat)
prefix='{"tool_input":{"command":"'
suffix='"}}'
input=${input#"$prefix"}
input=${input%"$suffix"}
printf '%s' "$input"
`
	require.NoError(t, os.WriteFile(jqPath, []byte(fakeJQ), 0o755))

	cmd := exec.Command("bash", scriptPath)
	cmd.Env = append(os.Environ(), "PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	cmd.Stdin = strings.NewReader(fmt.Sprintf(`{"tool_input":{"command":"%s"}}`, command))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return 0, stderr.String()
	}

	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	return exitErr.ExitCode(), stderr.String()
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()

	t.Run("registers all built-in harnesses", func(t *testing.T) {
		assert.NotNil(t, r.Get("claude"))
		assert.NotNil(t, r.Get("opencode"))
		assert.NotNil(t, r.Get("codex"))
		assert.Nil(t, r.Get("nonexistent"))
	})

	t.Run("All returns stable order", func(t *testing.T) {
		assert.Equal(t, []string{"opencode", "claude", "codex"}, r.All())
	})

	t.Run("DetectAll returns results for every harness", func(t *testing.T) {
		results := r.DetectAll()
		require.Len(t, results, 3)
		assert.Equal(t, "opencode", results[0].Name)
		assert.Equal(t, "claude", results[1].Name)
		assert.Equal(t, "codex", results[2].Name)
	})
}

func TestClaudeAdapter(t *testing.T) {
	c := &Claude{}

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "claude", c.Name())
	})

	t.Run("ListModels returns static list", func(t *testing.T) {
		models, err := c.ListModels()
		require.NoError(t, err)
		assert.Contains(t, models, "claude-sonnet-4-6")
		assert.Contains(t, models, "claude-opus-4-6")
		assert.Len(t, models, 4)
	})

	t.Run("BuildFlags with model and effort", func(t *testing.T) {
		flags := c.BuildFlags(AgentConfig{
			Model:  "claude-opus-4-6",
			Effort: "high",
		})
		assert.Equal(t, []string{"--model", "claude-opus-4-6", "--effort", "high"}, flags)
	})

	t.Run("BuildFlags skips empty fields", func(t *testing.T) {
		flags := c.BuildFlags(AgentConfig{})
		assert.Empty(t, flags)
	})

	t.Run("SupportsTemperature is false", func(t *testing.T) {
		assert.False(t, c.SupportsTemperature())
	})

	t.Run("SupportsEffort is true", func(t *testing.T) {
		assert.True(t, c.SupportsEffort())
	})

	t.Run("ListEffortLevels ignores model and includes max", func(t *testing.T) {
		levels := c.ListEffortLevels("anything")
		assert.Equal(t, []string{"", "low", "medium", "high", "max"}, levels)
	})
}

func TestCodexAdapter(t *testing.T) {
	c := &Codex{}

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "codex", c.Name())
	})

	t.Run("ListModels returns default", func(t *testing.T) {
		models, err := c.ListModels()
		require.NoError(t, err)
		assert.Equal(t, []string{"gpt-5-codex", "gpt-5.4", "gpt-5.3-codex"}, models)
	})

	t.Run("BuildFlags with all fields", func(t *testing.T) {
		temp := 0.3
		flags := c.BuildFlags(AgentConfig{
			Model:       "gpt-5-codex",
			Effort:      "high",
			Temperature: &temp,
		})
		assert.Equal(t, []string{
			"-m", "gpt-5-codex",
			"-c", "model_reasoning_effort=high",
			"-c", "temperature=0.3",
		}, flags)
	})

	t.Run("SupportsTemperature is true", func(t *testing.T) {
		assert.True(t, c.SupportsTemperature())
	})

	t.Run("SupportsEffort is true", func(t *testing.T) {
		assert.True(t, c.SupportsEffort())
	})

	t.Run("ListEffortLevels ignores model and includes xhigh", func(t *testing.T) {
		levels := c.ListEffortLevels("anything")
		assert.Equal(t, []string{"", "low", "medium", "high", "xhigh"}, levels)
	})
}

func TestCodexAdapter_InstallEnforcement(t *testing.T) {
	c := &Codex{}
	assert.NoError(t, c.InstallEnforcement())
}

func TestClaudeAdapter_InstallEnforcement(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	c := &Claude{}
	require.NoError(t, c.InstallEnforcement())

	// Hook script written and executable
	hookPath := filepath.Join(tmpHome, ".claude", "hooks", "enforce-cli-tools.sh")
	assert.FileExists(t, hookPath)
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	assert.True(t, info.Mode()&0o111 != 0, "hook script must be executable")

	// settings.json has PreToolUse entry
	settingsPath := filepath.Join(tmpHome, ".claude", "settings.json")
	assert.FileExists(t, settingsPath)
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "enforce-cli-tools.sh")
	assert.Contains(t, string(data), "PreToolUse")

	// Idempotent: running again doesn't duplicate
	require.NoError(t, c.InstallEnforcement())
	data2, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(data2), "enforce-cli-tools.sh"),
		"must not duplicate hook entry on re-run")
}

func TestCLIToolsEnforcementScript_SSHRemoteCommands(t *testing.T) {
	t.Run("blocks local grep", func(t *testing.T) {
		code, stderr := runCLIToolsEnforcementScript(t, "grep foo")
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr, "BLOCKED: 'grep' is banned")
	})

	t.Run("allows remote grep inside ssh command", func(t *testing.T) {
		code, stderr := runCLIToolsEnforcementScript(t, "ssh host 'printf foo | grep foo'")
		assert.Equal(t, 0, code)
		assert.Empty(t, stderr)
	})

	t.Run("still blocks local grep after ssh pipeline", func(t *testing.T) {
		code, stderr := runCLIToolsEnforcementScript(t, "ssh host 'printf foo | grep foo' | grep foo")
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr, "BLOCKED: 'grep' is banned")
	})

	t.Run("keeps blocking local shell expansion before ssh", func(t *testing.T) {
		code, stderr := runCLIToolsEnforcementScript(t, "ssh host $(grep foo)")
		assert.Equal(t, 2, code)
		assert.Contains(t, stderr, "BLOCKED: 'grep' is banned")
	})
}

func TestClaudeAdapter_InstallEnforcement_PreservesExisting(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Pre-populate settings.json with existing hooks
	claudeDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o755))
	existing := `{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt",
        "hooks": [{ "type": "command", "command": "notify.sh" }]
      }
    ]
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644))

	c := &Claude{}
	require.NoError(t, c.InstallEnforcement())

	data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)
	// Both old and new hooks present
	assert.Contains(t, string(data), "notify.sh")
	assert.Contains(t, string(data), "enforce-cli-tools.sh")
	assert.Contains(t, string(data), "PreToolUse")
	assert.Contains(t, string(data), "Notification")
}

func TestOpenCodeAdapter_InstallEnforcement(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	o := &OpenCode{}
	require.NoError(t, o.InstallEnforcement())

	pluginPath := filepath.Join(tmpHome, ".config", "opencode", "plugins", "enforce-cli-tools.js")
	assert.FileExists(t, pluginPath)
	data, err := os.ReadFile(pluginPath)
	require.NoError(t, err)
	assert.Contains(t, string(data), "tool.execute.before")
	assert.Contains(t, string(data), "grep")
	assert.Contains(t, string(data), "rg")

	require.NoError(t, o.InstallEnforcement())
}

func TestClaudeAdapter_UninstallEnforcement(t *testing.T) {
	t.Run("no-op when nothing installed", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		c := &Claude{}
		assert.NoError(t, c.UninstallEnforcement())
	})

	t.Run("removes hook entry and script after install", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		c := &Claude{}
		require.NoError(t, c.InstallEnforcement())

		hookPath := filepath.Join(tmpHome, ".claude", "hooks", "enforce-cli-tools.sh")
		assert.FileExists(t, hookPath)

		require.NoError(t, c.UninstallEnforcement())

		// Script file removed
		assert.NoFileExists(t, hookPath)

		// settings.json should no longer reference enforce-cli-tools.sh
		settingsPath := filepath.Join(tmpHome, ".claude", "settings.json")
		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		assert.NotContains(t, string(data), "enforce-cli-tools.sh")
	})

	t.Run("idempotent: uninstall twice does not error", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		c := &Claude{}
		require.NoError(t, c.InstallEnforcement())
		require.NoError(t, c.UninstallEnforcement())
		require.NoError(t, c.UninstallEnforcement())
	})

	t.Run("preserves unrelated hooks when removing enforcement", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		claudeDir := filepath.Join(tmpHome, ".claude")
		require.NoError(t, os.MkdirAll(claudeDir, 0o755))
		existing := `{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt",
        "hooks": [{ "type": "command", "command": "notify.sh" }]
      }
    ]
  }
}`
		require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte(existing), 0o644))

		c := &Claude{}
		require.NoError(t, c.InstallEnforcement())
		require.NoError(t, c.UninstallEnforcement())

		data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
		require.NoError(t, err)
		// Notification hook preserved
		assert.Contains(t, string(data), "notify.sh")
		assert.Contains(t, string(data), "Notification")
		// Enforcement hook gone
		assert.NotContains(t, string(data), "enforce-cli-tools.sh")
	})
}

func TestCodexAdapter_UninstallEnforcement(t *testing.T) {
	c := &Codex{}
	assert.NoError(t, c.UninstallEnforcement())
	// Second call also no-ops cleanly
	assert.NoError(t, c.UninstallEnforcement())
}

func TestOpenCodeAdapter_UninstallEnforcement(t *testing.T) {
	t.Run("no-op when nothing installed", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		o := &OpenCode{}
		assert.NoError(t, o.UninstallEnforcement())
	})

	t.Run("removes plugin file after install", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		o := &OpenCode{}
		require.NoError(t, o.InstallEnforcement())

		pluginPath := filepath.Join(tmpHome, ".config", "opencode", "plugins", "enforce-cli-tools.js")
		assert.FileExists(t, pluginPath)

		require.NoError(t, o.UninstallEnforcement())
		assert.NoFileExists(t, pluginPath)
	})

	t.Run("idempotent: uninstall twice does not error", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)

		o := &OpenCode{}
		require.NoError(t, o.InstallEnforcement())
		require.NoError(t, o.UninstallEnforcement())
		require.NoError(t, o.UninstallEnforcement())
	})
}

func TestOpenCodeAdapter(t *testing.T) {
	o := &OpenCode{}

	t.Run("Name", func(t *testing.T) {
		assert.Equal(t, "opencode", o.Name())
	})

	t.Run("BuildFlags returns only extra flags", func(t *testing.T) {
		flags := o.BuildFlags(AgentConfig{
			Model:      "anthropic/claude-sonnet-4-6",
			ExtraFlags: []string{"--verbose"},
		})
		// opencode uses project config, not CLI flags for model
		assert.Equal(t, []string{"--verbose"}, flags)
	})

	t.Run("SupportsTemperature is true", func(t *testing.T) {
		assert.True(t, o.SupportsTemperature())
	})

	t.Run("SupportsEffort is true", func(t *testing.T) {
		assert.True(t, o.SupportsEffort())
	})

	t.Run("ListEffortLevels varies by model", func(t *testing.T) {
		t.Run("anthropic model gets claude levels", func(t *testing.T) {
			levels := o.ListEffortLevels("anthropic/claude-sonnet-4-6")
			assert.Equal(t, []string{"", "low", "medium", "high", "max"}, levels)
		})

		t.Run("codex model gets codex levels", func(t *testing.T) {
			levels := o.ListEffortLevels("gpt-5.3-codex")
			assert.Equal(t, []string{"", "low", "medium", "high", "xhigh"}, levels)
		})

		t.Run("openai prefixed model gets xhigh", func(t *testing.T) {
			levels := o.ListEffortLevels("openai/gpt-5.4")
			assert.Equal(t, []string{"", "low", "medium", "high", "xhigh"}, levels)
		})

		t.Run("bare gpt model gets xhigh", func(t *testing.T) {
			levels := o.ListEffortLevels("gpt-5.4")
			assert.Equal(t, []string{"", "low", "medium", "high", "xhigh"}, levels)
		})

		t.Run("other model gets generic levels", func(t *testing.T) {
			levels := o.ListEffortLevels("deepseek/deepseek-r1")
			assert.Equal(t, []string{"", "low", "medium", "high"}, levels)
		})
	})
}
