package scaffold

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertValidJSON strips JSONC-style line comments and asserts the result is valid JSON.
func assertValidJSON(t *testing.T, content string) {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		lines = append(lines, line)
	}
	var parsed interface{}
	require.NoError(t, json.Unmarshal([]byte(strings.Join(lines, "\n")), &parsed),
		"rendered opencode.jsonc must be valid JSON:\n%s", content)
}

var allTools = []string{"sg", "comby", "difft", "sd", "yq", "mlr", "glow", "typos", "scc", "tokei", "watchexec", "hyperfine", "procs", "mprocs"}

func TestValidateRole(t *testing.T) {
	t.Run("valid roles pass", func(t *testing.T) {
		for _, role := range []string{"coder", "reviewer", "planner", "my-agent", "agent_1"} {
			assert.NoError(t, validateRole(role), "role: %q", role)
		}
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		for _, role := range []string{"../etc/passwd", "../../.bashrc", "a/b", "a\\b"} {
			assert.Error(t, validateRole(role), "role: %q", role)
		}
	})

	t.Run("empty role rejected", func(t *testing.T) {
		assert.Error(t, validateRole(""))
	})
}

func TestScaffoldClaudeProject(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "reviewer", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
	}

	_, err := WriteClaudeProject(dir, agents, allTools, false)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "coder.md"))
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "reviewer.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".claude", "agents", "planner.md"))

	// MCP config must be written at the project root.
	mcpPath := filepath.Join(dir, ".mcp.json")
	assert.FileExists(t, mcpPath)
	data, err := os.ReadFile(mcpPath)
	require.NoError(t, err)
	var cfg map[string]any
	require.NoError(t, json.Unmarshal(data, &cfg))
	servers, ok := cfg["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key must be present")
	kasmos, ok := servers["kasmos"].(map[string]any)
	require.True(t, ok, "kasmos entry must be present")
	assert.Equal(t, "http", kasmos["type"])
	assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	assert.NotContains(t, kasmos, "command", "stdio command key must not be present")
	assert.NotContains(t, kasmos, "args", "stdio args key must not be present")

	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	assert.FileExists(t, settingsPath)
	settingsData, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))
	assert.Equal(t, true, settings["enableAllProjectMcpServers"])
	enabledServers, ok := settings["enabledMcpjsonServers"].([]any)
	require.True(t, ok, "enabledMcpjsonServers key must be present")
	assert.Contains(t, enabledServers, "kasmos")
	assertAllMCPToolsAllowed(t, settings)
}

func assertAllMCPToolsAllowed(t *testing.T, settings map[string]any) {
	t.Helper()
	perms, ok := settings["permissions"].(map[string]any)
	require.True(t, ok, "permissions key must be present")
	allowRaw, ok := perms["allow"].([]any)
	require.True(t, ok, "permissions.allow must be present and an array")
	var allowed []string
	for _, entry := range allowRaw {
		if s, ok := entry.(string); ok {
			allowed = append(allowed, s)
		}
	}
	for _, tool := range kasmosMCPToolPermissions {
		assert.Contains(t, allowed, tool, "permissions.allow must contain %q", tool)
	}
}

func TestEnsureClaudeProjectSettings(t *testing.T) {
	t.Run("creates file when missing", func(t *testing.T) {
		dir := t.TempDir()

		result, err := EnsureClaudeProjectSettings(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: filepath.Join(".claude", "settings.json"), Created: true}, result)

		data, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))
		assert.Equal(t, true, settings["enableAllProjectMcpServers"])
		assert.Contains(t, settings["enabledMcpjsonServers"], "kasmos")
		assertAllMCPToolsAllowed(t, settings)
	})

	t.Run("preserves existing settings and appends kasmos once", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
		existing := `{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt",
        "hooks": [{ "type": "command", "command": "notify.sh" }]
      }
    ]
  },
  "enabledMcpjsonServers": ["clickup"]
}`
		require.NoError(t, os.WriteFile(settingsPath, []byte(existing), 0o644))

		result, err := EnsureClaudeProjectSettings(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: filepath.Join(".claude", "settings.json"), Created: true}, result)

		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))
		assert.Equal(t, true, settings["enableAllProjectMcpServers"])
		assert.Contains(t, settings["enabledMcpjsonServers"], "clickup")
		assert.Contains(t, settings["enabledMcpjsonServers"], "kasmos")
		assert.Contains(t, settings["hooks"], "Notification")
		assertAllMCPToolsAllowed(t, settings)

		result, err = EnsureClaudeProjectSettings(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: filepath.Join(".claude", "settings.json"), Created: false}, result)

		data, err = os.ReadFile(settingsPath)
		require.NoError(t, err)
		assert.Equal(t, 1, strings.Count(string(data), `"kasmos"`))
	})

	t.Run("does not duplicate existing mcp entries", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
		// Pre-populate with a subset of kasmos MCP tools already present.
		pre := map[string]any{
			"enableAllProjectMcpServers": true,
			"enabledMcpjsonServers":      []any{"kasmos"},
			"permissions": map[string]any{
				"allow": []any{
					"mcp__kasmos__grep",
					"mcp__kasmos__read_file",
					"mcp__kasmos__git_status",
				},
			},
		}
		preBytes, err := json.Marshal(pre)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(settingsPath, preBytes, 0o644))

		_, err = EnsureClaudeProjectSettings(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))
		assertAllMCPToolsAllowed(t, settings)

		// Verify no duplicates: count occurrences of each pre-populated tool.
		content := string(data)
		assert.Equal(t, 1, strings.Count(content, `"mcp__kasmos__grep"`), "mcp__kasmos__grep must appear exactly once")
		assert.Equal(t, 1, strings.Count(content, `"mcp__kasmos__read_file"`), "mcp__kasmos__read_file must appear exactly once")
		assert.Equal(t, 1, strings.Count(content, `"mcp__kasmos__git_status"`), "mcp__kasmos__git_status must appear exactly once")
	})

	t.Run("preserves deny rules", func(t *testing.T) {
		dir := t.TempDir()
		settingsPath := filepath.Join(dir, ".claude", "settings.json")
		require.NoError(t, os.MkdirAll(filepath.Dir(settingsPath), 0o755))
		pre := map[string]any{
			"permissions": map[string]any{
				"deny": []any{"Agent(Explore)", "Agent(Plan)"},
			},
		}
		preBytes, err := json.Marshal(pre)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(settingsPath, preBytes, 0o644))

		_, err = EnsureClaudeProjectSettings(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(settingsPath)
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))
		perms, ok := settings["permissions"].(map[string]any)
		require.True(t, ok, "permissions must be present")
		deny, ok := perms["deny"].([]any)
		require.True(t, ok, "permissions.deny must be present")
		assert.Contains(t, deny, "Agent(Explore)", "deny list must be preserved")
		assert.Contains(t, deny, "Agent(Plan)", "deny list must be preserved")
		assertAllMCPToolsAllowed(t, settings)
	})
}

func TestWriteClaudeMCPConfig(t *testing.T) {
	t.Run("creates file with kasmos entry", func(t *testing.T) {
		dir := t.TempDir()
		result, err := WriteClaudeMCPConfig(dir, false)
		require.NoError(t, err)
		assert.True(t, result.Created)
		assert.FileExists(t, filepath.Join(dir, ".mcp.json"))
	})

	t.Run("skips existing file when force=false", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644))

		result, err := WriteClaudeMCPConfig(dir, false)
		require.NoError(t, err)
		assert.False(t, result.Created)
	})

	t.Run("overwrites existing file when force=true", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644))

		result, err := WriteClaudeMCPConfig(dir, true)
		require.NoError(t, err)
		assert.True(t, result.Created)

		data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
		require.NoError(t, err)
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))
		servers := cfg["mcpServers"].(map[string]any)
		kasmos := servers["kasmos"].(map[string]any)
		assert.Equal(t, "http", kasmos["type"])
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	})
}

func TestEnsureClaudeMCPEntry(t *testing.T) {
	t.Run("creates file when missing", func(t *testing.T) {
		dir := t.TempDir()
		result, err := EnsureClaudeMCPEntry(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: ".mcp.json", Created: true}, result)
		data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
		require.NoError(t, err)
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))
		servers := cfg["mcpServers"].(map[string]any)
		assert.Contains(t, servers, "kasmos")
	})

	t.Run("adds kasmos to existing file without disturbing other servers", func(t *testing.T) {
		dir := t.TempDir()
		existing := `{"mcpServers":{"other-server":{"type":"stdio","command":"foo"}}}`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte(existing), 0o644))

		result, err := EnsureClaudeMCPEntry(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: ".mcp.json", Created: true}, result)

		data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
		require.NoError(t, err)
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))
		servers := cfg["mcpServers"].(map[string]any)
		assert.Contains(t, servers, "kasmos", "kasmos must be added")
		assert.Contains(t, servers, "other-server", "existing server must be preserved")
	})

	t.Run("is idempotent when kasmos already uses shared http endpoint", func(t *testing.T) {
		dir := t.TempDir()
		initial := `{"mcpServers":{"kasmos":{"type":"http","url":"http://127.0.0.1:7434/mcp"}}}`
		dest := filepath.Join(dir, ".mcp.json")
		require.NoError(t, os.WriteFile(dest, []byte(initial), 0o644))
		info1, _ := os.Stat(dest)

		result, err := EnsureClaudeMCPEntry(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: ".mcp.json", Created: false}, result)

		info2, _ := os.Stat(dest)
		assert.Equal(t, info1.ModTime(), info2.ModTime(), "file must not be rewritten when already correct")
	})

	t.Run("migrates stdio to shared http endpoint", func(t *testing.T) {
		dir := t.TempDir()
		initial := `{"mcpServers":{"kasmos":{"type":"stdio","command":"/usr/local/bin/kas","args":["mcp"]}}}`
		dest := filepath.Join(dir, ".mcp.json")
		require.NoError(t, os.WriteFile(dest, []byte(initial), 0o644))

		result, err := EnsureClaudeMCPEntry(dir)
		require.NoError(t, err)
		assert.Equal(t, WriteResult{Path: ".mcp.json", Created: true}, result)

		data, err := os.ReadFile(dest)
		require.NoError(t, err)
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))
		servers := cfg["mcpServers"].(map[string]any)
		kasmos := servers["kasmos"].(map[string]any)
		assert.Equal(t, "http", kasmos["type"])
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
		assert.NotContains(t, kasmos, "command", "stdio command key must be removed")
		assert.NotContains(t, kasmos, "args", "stdio args key must be removed")
	})
}

func TestScaffoldOpenCodeProject(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Enabled: true},
		{Role: "chat", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Enabled: true},
	}

	_, err := WriteOpenCodeProject(dir, agents, allTools, false)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "coder.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "chat.md"))
}

func TestScaffoldCodexProject(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
	}

	_, err := WriteCodexProject(dir, agents, allTools, false, true)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".codex", "AGENTS.md"))
}

func TestScaffoldSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	existing := filepath.Join(agentDir, "coder.md")
	require.NoError(t, os.WriteFile(existing, []byte("custom content"), 0o644))

	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Enabled: true},
	}

	results, err := WriteClaudeProject(dir, agents, allTools, false) // force=false
	require.NoError(t, err)

	// File preserved
	content, err := os.ReadFile(existing)
	require.NoError(t, err)
	assert.Equal(t, "custom content", string(content))

	// Result correctly shows skipped for the coder agent
	require.GreaterOrEqual(t, len(results), 1)
	coderResult := results[0]
	assert.False(t, coderResult.Created)
}

func TestScaffoldForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	agentDir := filepath.Join(dir, ".claude", "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o755))

	existing := filepath.Join(agentDir, "coder.md")
	require.NoError(t, os.WriteFile(existing, []byte("old content"), 0o644))

	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Enabled: true},
	}

	results, err := WriteClaudeProject(dir, agents, allTools, true) // force=true
	require.NoError(t, err)

	// Content overwritten
	content, err := os.ReadFile(existing)
	require.NoError(t, err)
	assert.NotEqual(t, "old content", string(content))

	// Result correctly shows created
	require.GreaterOrEqual(t, len(results), 1)
	assert.True(t, results[0].Created)
}

func TestScaffoldAll_MixedHarnesses(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "reviewer", Harness: "opencode", Model: "anthropic/claude-opus-4-6", Enabled: true},
		{Role: "planner", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
	}

	results, err := ScaffoldAll(dir, agents, allTools, false)
	require.NoError(t, err)

	// All three harness directories created
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "coder.md"))
	assert.FileExists(t, filepath.Join(dir, ".opencode", "agents", "reviewer.md"))
	assert.FileExists(t, filepath.Join(dir, ".codex", "AGENTS.md"))

	// Results only include actually-created files
	assert.GreaterOrEqual(t, len(results), 3)
	for _, r := range results {
		assert.True(t, r.Created, "expected all results to be created in fresh dir")
	}
}

// TestScaffoldAll_CodexWritesCodexMCPConfig verifies that a codex scaffold
// writes a project-local .codex/config.toml with the kasmos MCP server
// registered via the shared HTTP endpoint. Codex CLI reads this file natively
// for trusted projects — it does not understand Claude's .mcp.json format.
func TestScaffoldAll_CodexWritesCodexMCPConfig(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
	}

	_, err := ScaffoldAll(dir, agents, allTools, false)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(dir, ".codex", "AGENTS.md"))
	assert.NoDirExists(t, filepath.Join(dir, ".claude"),
		"claude harness must not be scaffolded for a codex-only repo")
	assert.NoFileExists(t, filepath.Join(dir, ".mcp.json"),
		"codex-only scaffold must not emit .mcp.json — that is Claude Code's format")

	cfgPath := filepath.Join(dir, ".codex", "config.toml")
	require.FileExists(t, cfgPath)
	data, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	var parsed map[string]any
	_, decodeErr := toml.Decode(string(data), &parsed)
	require.NoError(t, decodeErr, ".codex/config.toml must be valid TOML:\n%s", data)

	servers, ok := parsed["mcp_servers"].(map[string]any)
	require.True(t, ok, "mcp_servers table must be present")
	kasmos, ok := servers["kasmos"].(map[string]any)
	require.True(t, ok, "kasmos entry must be present")
	assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	assert.Equal(t, "auto", kasmos["default_tools_approval_mode"], "server-level approval mode must be set")
	assert.NotContains(t, kasmos, "command", "stdio command key must not be present")
	assert.NotContains(t, kasmos, "args", "stdio args key must not be present")
}

func TestScaffoldRejectsPathTraversalRole(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "../../../.bashrc", Harness: "claude", Enabled: true},
	}

	_, err := WriteClaudeProject(dir, agents, allTools, false)
	assert.Error(t, err)
}

func TestScaffoldFiltersByHarness(t *testing.T) {
	dir := t.TempDir()
	// Pass a mix: only the claude agent should be written
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "fixer", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "reviewer", Harness: "opencode", Model: "anthropic/claude-opus-4-6", Enabled: true},
	}

	results, err := WriteClaudeProject(dir, agents, allTools, false)
	require.NoError(t, err)

	// Only coder.md and fixer.md created (claude only — no opencode reviewer)
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "coder.md"))
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "fixer.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".opencode", "agents", "reviewer.md"))
	require.GreaterOrEqual(t, len(results), 1)
	// First result is the per-role coder agent
	assert.Equal(t, ".claude/agents/coder.md", results[0].Path)
}

func TestToolsReferenceInjected(t *testing.T) {
	t.Run("claude agents include tools reference", func(t *testing.T) {
		dir := t.TempDir()
		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		}

		_, err := WriteClaudeProject(dir, agents, allTools, false)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dir, ".claude", "agents", "coder.md"))
		require.NoError(t, err)
		assert.NotContains(t, string(content), "{{TOOLS_REFERENCE}}")
		// Skill-load directives are stripped; coder template is now minimal
		assert.Contains(t, string(content), "KASMOS_TASK")
		assert.NotContains(t, string(content), "cli-tools")
		assert.NotContains(t, string(content), "CLI Tools (MANDATORY)")
	})

	t.Run("opencode agents include tools reference", func(t *testing.T) {
		dir := t.TempDir()
		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Enabled: true},
		}

		_, err := WriteOpenCodeProject(dir, agents, allTools, false)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dir, ".opencode", "agents", "coder.md"))
		require.NoError(t, err)
		assert.NotContains(t, string(content), "{{TOOLS_REFERENCE}}")
		// Skill-load directives are stripped; coder template is now minimal
		assert.Contains(t, string(content), "KASMOS_TASK")
		assert.NotContains(t, string(content), "cli-tools")
		assert.NotContains(t, string(content), "CLI Tools (MANDATORY)")
	})

	t.Run("codex AGENTS.md includes tools reference", func(t *testing.T) {
		dir := t.TempDir()
		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
		}

		_, err := WriteCodexProject(dir, agents, allTools, false, true)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dir, ".codex", "AGENTS.md"))
		require.NoError(t, err)
		assert.NotContains(t, string(content), "{{TOOLS_REFERENCE}}")
		assert.Contains(t, string(content), "ast-grep")
	})

	t.Run("model placeholder is substituted", func(t *testing.T) {
		dir := t.TempDir()
		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
		}

		_, err := WriteClaudeProject(dir, agents, allTools, false)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dir, ".claude", "agents", "coder.md"))
		require.NoError(t, err)
		assert.NotContains(t, string(content), "{{MODEL}}")
		assert.Contains(t, string(content), "claude-opus-4-6")
	})

}

func TestWriteProjectSkills(t *testing.T) {
	dir := t.TempDir()

	results, err := WriteProjectSkills(dir, false)
	require.NoError(t, err)

	// Generic project skills written (including cli-tools and kasmos-cli)
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-cli", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-architect", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-fixer", "SKILL.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".agents", "skills", "kasmos-elaborator", "SKILL.md"))
	assert.NoFileExists(t, filepath.Join(dir, ".agents", "skills", "tui-design", "SKILL.md"))

	fixerSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "kasmos-fixer", "SKILL.md"))
	require.NoError(t, err)
	assert.Contains(t, string(fixerSkill), "Scaffolding System Protocol (always before editing skills/agent commands)")

	architectSkill, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "kasmos-architect", "SKILL.md"))
	require.NoError(t, err)
	architectTemplate, err := templates.ReadFile("templates/skills/kasmos-architect/SKILL.md")
	require.NoError(t, err)
	assert.Equal(t, string(architectTemplate), string(architectSkill))
	assert.Contains(t, string(architectSkill), "compatibility note: emit `elaborator-finished` exactly as written until the gateway is renamed")
	assert.Contains(t, string(architectSkill), "signal shim, not an active elaborator role")
	assert.Contains(t, string(architectSkill), "## parallel baseline mode")
	assert.Contains(t, string(architectSkill), ".kasmos/cache/<plan-file>-architect-baseline.json")
	assert.Contains(t, string(architectSkill), "validate `plan_file`, `project`, `description_hash`, `schema_version`, and non-empty `baseline_markdown`")
	assert.Contains(t, string(architectSkill), ".kasmos/cache/<plan-file>-architect.json")
	assert.Contains(t, string(architectSkill), "decision_audit")
	assert.Contains(t, string(architectSkill), "planner_summary")
	assert.Contains(t, string(architectSkill), "baseline_summary")
	assert.Contains(t, string(architectSkill), "final_decision")
	assert.Contains(t, string(architectSkill), "`baseline_source`: one of `parallel_cache`, `inline`, `absent`, or `stale`")
	assert.Contains(t, string(architectSkill), "advisory input and must not be treated as final implementation state")
	assert.NotContains(t, string(architectSkill), "**elaborator** agent")

	// cli-tools resource files included
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "ast-grep.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "comby.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "difftastic.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "sd.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "yq.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "typos.md"))
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "resources", "scc.md"))

	// Results track what was written
	assert.Greater(t, len(results), 0)
	for _, r := range results {
		assert.True(t, r.Created)
	}
}

func TestWriteProjectSkills_SkipsExisting(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".agents", "skills", "cli-tools")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	customFile := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(customFile, []byte("custom"), 0o644))

	_, err := WriteProjectSkills(dir, false) // force=false
	require.NoError(t, err)

	content, err := os.ReadFile(customFile)
	require.NoError(t, err)
	assert.Equal(t, "custom", string(content))
}

func TestWriteProjectSkills_ForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, ".agents", "skills", "cli-tools")
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	customFile := filepath.Join(skillDir, "SKILL.md")
	require.NoError(t, os.WriteFile(customFile, []byte("old"), 0o644))

	_, err := WriteProjectSkills(dir, true) // force=true
	require.NoError(t, err)

	content, err := os.ReadFile(customFile)
	require.NoError(t, err)
	assert.NotEqual(t, "old", string(content))
}

func TestSymlinkHarnessSkills(t *testing.T) {
	dir := t.TempDir()

	// Create canonical skill dirs (simulating WriteProjectSkills already ran)
	for _, name := range []string{"cli-tools", "writing-plans", "executing-plans"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".agents", "skills", name), 0o755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, ".agents", "skills", name, "SKILL.md"),
			[]byte("test"), 0o644))
	}

	// Symlink for claude
	err := SymlinkHarnessSkills(dir, "claude")
	require.NoError(t, err)

	for _, name := range []string{"cli-tools", "writing-plans", "executing-plans"} {
		link := filepath.Join(dir, ".claude", "skills", name)
		target, err := os.Readlink(link)
		require.NoError(t, err, "skill %s should be symlinked", name)
		assert.Equal(t, filepath.Join("..", "..", ".agents", "skills", name), target)

		// Symlink should resolve to actual content
		content, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
		require.NoError(t, err)
		assert.Equal(t, "test", string(content))
	}

	// Symlink for opencode
	err = SymlinkHarnessSkills(dir, "opencode")
	require.NoError(t, err)

	for _, name := range []string{"cli-tools", "writing-plans", "executing-plans"} {
		link := filepath.Join(dir, ".opencode", "skills", name)
		_, err := os.Readlink(link)
		require.NoError(t, err, "skill %s should be symlinked for opencode", name)
	}
}

func TestSymlinkHarnessSkills_ReplacesExisting(t *testing.T) {
	dir := t.TempDir()

	// Create canonical
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".agents", "skills", "cli-tools"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".agents", "skills", "cli-tools", "SKILL.md"),
		[]byte("new"), 0o644))

	// Create stale symlink
	skillsDir := filepath.Join(dir, ".claude", "skills")
	require.NoError(t, os.MkdirAll(skillsDir, 0o755))
	require.NoError(t, os.Symlink("/nonexistent", filepath.Join(skillsDir, "cli-tools")))

	err := SymlinkHarnessSkills(dir, "claude")
	require.NoError(t, err)

	// Should have replaced the stale symlink
	content, err := os.ReadFile(filepath.Join(skillsDir, "cli-tools", "SKILL.md"))
	require.NoError(t, err)
	assert.Equal(t, "new", string(content))
}

func TestScaffoldAll_IncludesSkills(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Enabled: true},
	}

	results, err := ScaffoldAll(dir, agents, allTools, false)
	require.NoError(t, err)

	// Skills written to canonical location
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "SKILL.md"))

	// Symlinks created for each active harness
	for _, h := range []string{"claude", "opencode"} {
		link := filepath.Join(dir, "."+h, "skills", "cli-tools")
		_, err := os.Readlink(link)
		assert.NoError(t, err, "%s should have cli-tools symlink", h)
	}

	// Codex not scaffolded (no codex agent), so no codex symlinks
	assert.NoFileExists(t, filepath.Join(dir, ".codex", "skills"))

	// Results include skill files
	var skillResults int
	for _, r := range results {
		if strings.HasPrefix(filepath.ToSlash(r.Path), ".agents/skills/") {
			skillResults++
		}
	}
	assert.Greater(t, skillResults, 0)
}

func TestWriteOpenCodeProject_GeneratesConfig(t *testing.T) {
	dir := t.TempDir()
	temp := 0.1
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true},
		{Role: "planner", Harness: "opencode", Model: "anthropic/claude-opus-4-6", Temperature: ptrFloat(0.5), Effort: "max", Enabled: true},
		{Role: "reviewer", Harness: "opencode", Model: "openai/gpt-5.3-codex", Temperature: ptrFloat(0.2), Effort: "xhigh", Enabled: true},
	}

	results, err := WriteOpenCodeProject(dir, agents, allTools, false)
	require.NoError(t, err)

	// Config file created
	configPath := filepath.Join(dir, "opencode.jsonc")
	assert.FileExists(t, configPath)

	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	s := string(content)

	// Schema present
	assert.Contains(t, s, `"$schema": "https://opencode.ai/config.json"`)

	// Disabled built-in agents
	assert.Contains(t, s, `"build"`)
	assert.Contains(t, s, `"plan"`)
	assert.Contains(t, s, `"disable": true`)

	// Chat agent with fixed defaults
	assert.Contains(t, s, `"chat"`)
	assert.Contains(t, s, `"anthropic/claude-sonnet-4-6"`)

	// Wizard-configured agents have correct models
	assert.Contains(t, s, `"anthropic/claude-opus-4-6"`)
	assert.Contains(t, s, `"openai/gpt-5.3-codex"`)

	// Temperature rendered as bare numbers (no quotes)
	assert.Contains(t, s, "0.1")
	assert.Contains(t, s, "0.5")
	assert.Contains(t, s, "0.2")

	// Effort values present
	assert.Contains(t, s, `"reasoningEffort": "medium"`)
	assert.Contains(t, s, `"reasoningEffort": "max"`)
	assert.Contains(t, s, `"reasoningEffort": "xhigh"`)

	// No raw placeholders left
	assert.NotContains(t, s, "{{")
	assert.NotContains(t, s, "}}")

	// Output must be valid JSON
	assertValidJSON(t, s)

	// Dynamic paths resolved (home dir and project dir)
	homeDir, _ := os.UserHomeDir()
	assert.Contains(t, s, homeDir)
	assert.Contains(t, s, dir)

	// Config is in the results list
	var found bool
	for _, r := range results {
		if r.Path == "opencode.jsonc" {
			found = true
			assert.True(t, r.Created)
		}
	}
	assert.True(t, found, "opencode.jsonc should be in results")
}

func TestWriteOpenCodeProject_NoEffort(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: ptrFloat(0.1), Effort: "", Enabled: true},
	}

	_, err := WriteOpenCodeProject(dir, agents, nil, false)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	require.NoError(t, err)
	s := string(content)

	// Coder block should NOT have reasoningEffort line
	coderIdx := strings.Index(s, `"coder"`)
	require.Greater(t, coderIdx, 0)
	// Look at the next ~500 chars after "coder" for the effort line
	coderSection := s[coderIdx:min(coderIdx+500, len(s))]
	assert.NotContains(t, coderSection, "reasoningEffort")
}

func TestWriteOpenCodeProject_NoTemp(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: nil, Effort: "medium", Enabled: true},
	}

	_, err := WriteOpenCodeProject(dir, agents, nil, false)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	require.NoError(t, err)
	s := string(content)

	// Coder block should NOT have temperature line
	coderIdx := strings.Index(s, `"coder"`)
	require.Greater(t, coderIdx, 0)
	coderSection := s[coderIdx:min(coderIdx+500, len(s))]
	assert.NotContains(t, coderSection, "temperature")
}

func TestWriteOpenCodeProject_ValidJSONC_OnlyCoder(t *testing.T) {
	// Regression: when planner+reviewer are removed (non-opencode harness),
	// the preceding coder block must not have a trailing comma.
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: ptrFloat(0.1), Effort: "medium", Enabled: true},
		{Role: "reviewer", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
	}
	_, err := WriteOpenCodeProject(dir, agents, nil, false)
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	require.NoError(t, err)
	assertValidJSON(t, string(content))
}

func TestWriteOpenCodeProject_ValidJSONC_NoWizardAgents(t *testing.T) {
	// Regression: when all three wizard roles are removed (none use opencode harness),
	// only chat+build+plan remain and the output must still be valid JSON.
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
	}
	_, err := WriteOpenCodeProject(dir, agents, nil, false)
	require.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	require.NoError(t, err)
	assertValidJSON(t, string(content))
	assert.Contains(t, string(content), `"mcp"`)
	assert.Contains(t, string(content), `"kasmos"`)
}

func TestPatchWorktreeConfig_AddsKasmosMCPToExistingConfig(t *testing.T) {
	dir := t.TempDir()
	opencodeConfig := `{
	  "agent": {
	    "coder": {
	      "model": "anthropic/claude-sonnet-4-6"
	    }
	  },
	  "mcp": {
	    "clickup": {
	      "type": "remote",
	      "url": "https://mcp.clickup.com/mcp",
	      "enabled": true
	    }
	  }
	}`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	temp := 0.2
	agents := []harness.AgentConfig{{
		Role:        "coder",
		Harness:     "opencode",
		Model:       "anthropic/claude-sonnet-4-6",
		Temperature: &temp,
		Effort:      "medium",
	}}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(updated))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	mcp, ok := parsed["mcp"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, mcp, "clickup")
	assert.Contains(t, mcp, "kasmos")

	kasmos, ok := mcp["kasmos"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "remote", kasmos["type"])
	assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	assert.NotContains(t, kasmos, "command", "command key must not be present in remote transport entry")
	assert.Equal(t, true, kasmos["enabled"])
}

func TestPatchWorktreeConfig_MigratesLocalEntryToRemote(t *testing.T) {
	dir := t.TempDir()
	opencodeConfig := `{
	  "agent": {
	    "coder": {
	      "model": "anthropic/claude-sonnet-4-6"
	    }
	  },
	  "mcp": {
	    "clickup": {
	      "type": "remote",
	      "url": "https://mcp.clickup.com/mcp",
	      "enabled": true
	    },
	    "kasmos": {
	      "type": "local",
	      "command": ["/usr/local/bin/kas", "mcp"],
	      "enabled": true
	    }
	  }
	}`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	temp := 0.2
	agents := []harness.AgentConfig{{
		Role:        "coder",
		Harness:     "opencode",
		Model:       "anthropic/claude-sonnet-4-6",
		Temperature: &temp,
		Effort:      "medium",
	}}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(updated))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	mcp, ok := parsed["mcp"].(map[string]any)
	require.True(t, ok)

	// clickup must be preserved untouched
	clickup, ok := mcp["clickup"].(map[string]any)
	require.True(t, ok, "clickup MCP server must be preserved")
	assert.Equal(t, "remote", clickup["type"])
	assert.Equal(t, "https://mcp.clickup.com/mcp", clickup["url"])

	// kasmos must be migrated to shared remote http transport
	kasmos, ok := mcp["kasmos"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "remote", kasmos["type"])
	assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	assert.NotContains(t, kasmos, "command", "command key must be removed when migrating to remote transport")
	assert.Equal(t, true, kasmos["enabled"])
}

func TestPatchWorktreeConfig_MigratesLocalEntry_RemovesCommandKey(t *testing.T) {
	// Regression: a stale OpenCode local entry must have command removed after migration.
	dir := t.TempDir()
	opencodeConfig := `{
	  "mcp": {
	    "kasmos": {
	      "type": "local",
	      "command": ["/home/user/.local/bin/kas", "mcp"],
	      "enabled": true
	    }
	  }
	}`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	agents := []harness.AgentConfig{{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6"}}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(data))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(data, &parsed))
	mcp := parsed["mcp"].(map[string]any)
	kasmos := mcp["kasmos"].(map[string]any)

	assert.Equal(t, "remote", kasmos["type"])
	assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	assert.NotContains(t, kasmos, "command", "stale command key must be removed after migration")
}

// TestWriteOpenCodeProject_IncludesNonOpencodeAgents verifies that agent roles
// configured for a different harness (e.g. claude) are still written to
// opencode.jsonc. Kasmos controls which harness is used at orchestration time;
// opencode.jsonc just needs the block present so the agent is defined.
func TestWriteOpenCodeProject_IncludesNonOpencodeAgents(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: ptrFloat(0.1), Effort: "medium", Enabled: true},
		{Role: "reviewer", Harness: "claude", Model: "claude-opus-4-6", Temperature: ptrFloat(0.2), Effort: "medium", Enabled: true},
	}

	_, err := WriteOpenCodeProject(dir, agents, nil, false)
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	require.NoError(t, err)
	s := string(content)

	// Coder block present (opencode harness)
	assert.Contains(t, s, `"coder"`)
	assert.Contains(t, s, `"anthropic/claude-sonnet-4-6"`)

	// Reviewer block also present even though harness is claude
	assert.Contains(t, s, `"reviewer"`)
	assert.Contains(t, s, `"anthropic/claude-opus-4-6"`)
}

func TestRun_OpencodeConfigGenerated(t *testing.T) {
	// This is a scaffold-level check — the integration test in initcmd_test.go
	// already tests the TOML write path. Just verify the config file shows up
	// in ScaffoldAll results when opencode agents are present.
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6",
			Temperature: ptrFloat(0.1), Effort: "medium", Enabled: true},
	}

	results, err := ScaffoldAll(dir, agents, nil, false)
	require.NoError(t, err)

	var hasConfig bool
	for _, r := range results {
		if r.Path == "opencode.jsonc" {
			hasConfig = true
		}
	}
	assert.True(t, hasConfig, "ScaffoldAll should produce opencode.jsonc for opencode agents")
}

func TestEnsureRuntimeDirs_CreatesAll(t *testing.T) {
	dir := t.TempDir()
	results, err := EnsureRuntimeDirs(dir)
	require.NoError(t, err)

	// All dirs should be created on first run.
	assert.Len(t, results, len(runtimeDirs), "should create all runtime dirs")

	for _, rel := range runtimeDirs {
		info, err := os.Stat(filepath.Join(dir, rel))
		require.NoError(t, err, "dir %s must exist", rel)
		assert.True(t, info.IsDir(), "%s must be a directory", rel)
	}
}

func TestEnsureRuntimeDirs_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// First run creates dirs.
	_, err := EnsureRuntimeDirs(dir)
	require.NoError(t, err)

	// Second run should create nothing new.
	results, err := EnsureRuntimeDirs(dir)
	require.NoError(t, err)
	assert.Empty(t, results, "second run should not create any dirs")
}

func TestScaffold_IncludesFixerAgent(t *testing.T) {
	dir := t.TempDir()
	temp := 0.1
	agents := []harness.AgentConfig{
		{Harness: "opencode", Role: "coder", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true},
		{Harness: "opencode", Role: "fixer", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "low", Enabled: true},
	}
	results, err := WriteOpenCodeProject(dir, agents, nil, true)
	require.NoError(t, err)

	// Fixer agent is now wizard-managed: scaffolded when included in agents list
	fixerPath := filepath.Join(dir, ".opencode", "agents", "fixer.md")
	assert.FileExists(t, fixerPath)

	content, err := os.ReadFile(fixerPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "fixer")
	assert.Contains(t, string(content), "Scaffolding System (first step for skills/agent commands)")

	// Check opencode.jsonc includes fixer block
	var foundConfig bool
	for _, r := range results {
		if strings.Contains(r.Path, "opencode.jsonc") {
			foundConfig = true
		}
	}
	assert.True(t, foundConfig)
}

func TestScaffold_IncludesMasterAgent(t *testing.T) {
	dir := t.TempDir()
	temp := 0.2
	agents := []harness.AgentConfig{
		{Harness: "opencode", Role: "master", Model: "openai/gpt-5.4", Temperature: &temp, Effort: "high", Enabled: true},
		{Harness: "opencode", Role: "coder", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true},
	}

	_, err := WriteOpenCodeProject(dir, agents, nil, true)
	require.NoError(t, err)

	masterPath := filepath.Join(dir, ".opencode", "agents", "master.md")
	assert.FileExists(t, masterPath)

	content, err := os.ReadFile(filepath.Join(dir, "opencode.jsonc"))
	require.NoError(t, err)
	assert.Contains(t, string(content), `"master"`)
	assert.Contains(t, string(content), `"openai/gpt-5.4"`)
}

func TestPatchWorktreeConfig_UpdatesModelTempEffortPreservesOtherFields(t *testing.T) {
	dir := t.TempDir()
	opencodeConfig := `{
	  "agent": {
	    "planner": {
	      "permission": "read",
	      "textVerbosity": "concise",
	      "model": "anthropic/claude-sonnet-4-6",
	      "temperature": 0.3,
	      "reasoningEffort": "low"
	    }
	  }
	}`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	temp := 0.7
	agents := []harness.AgentConfig{{
		Role:        "planner",
		Harness:     "claude",
		Model:       "claude-opus-4-6",
		Temperature: &temp,
		Effort:      "high",
	}}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(updated))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	agent, ok := parsed["agent"].(map[string]any)
	require.True(t, ok)
	planner, ok := agent["planner"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "anthropic/claude-opus-4-6", planner["model"])
	assert.Equal(t, temp, planner["temperature"])
	assert.Equal(t, "high", planner["reasoningEffort"])
	assert.Equal(t, "read", planner["permission"])
	assert.Equal(t, "concise", planner["textVerbosity"])
}

func TestPatchWorktreeConfig_AddsMissingAgentBlocks(t *testing.T) {
	dir := t.TempDir()
	opencodeConfig := `{
	  "agent": {
	    "planner": {
	      "model": "anthropic/claude-sonnet-4-6",
	      "temperature": 0.2,
	      "reasoningEffort": "low"
	    }
	  }
	}`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	temp := 0.7
	effort := "high"
	agents := []harness.AgentConfig{
		{
			Role:        "planner",
			Harness:     "claude",
			Model:       "claude-opus-4-6",
			Temperature: &temp,
			Effort:      effort,
		},
		{
			Role:        "architect",
			Harness:     "opencode",
			Model:       "anthropic/claude-opus-4-6",
			Temperature: &temp,
			Effort:      effort,
		},
	}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(updated))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	agent, ok := parsed["agent"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, agent, "architect")

	plannerCfg, ok := agent["planner"].(map[string]any)
	require.True(t, ok)
	architectCfg, ok := agent["architect"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-opus-4-6", plannerCfg["model"])
	assert.Equal(t, temp, plannerCfg["temperature"])
	assert.Equal(t, effort, plannerCfg["reasoningEffort"])
	assert.Equal(t, "anthropic/claude-opus-4-6", architectCfg["model"])
	assert.Equal(t, temp, architectCfg["temperature"])
	assert.Equal(t, effort, architectCfg["reasoningEffort"])
}

func TestPatchWorktreeConfig_CreatesAgentBlockWhenMissing(t *testing.T) {
	dir := t.TempDir()
	opencodeConfig := `{
	  "mcp": {
	    "clickup": {
	      "type": "remote",
	      "url": "https://mcp.clickup.com/mcp",
	      "enabled": true
	    }
	  }
	}`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	temp := 0.4
	agents := []harness.AgentConfig{{
		Role:        "coder",
		Harness:     "opencode",
		Model:       "anthropic/claude-sonnet-4-6",
		Temperature: &temp,
		Effort:      "medium",
	}}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(updated))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))

	mcp, ok := parsed["mcp"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, mcp, "clickup")
	assert.Contains(t, mcp, "kasmos")

	agent, ok := parsed["agent"].(map[string]any)
	require.True(t, ok)
	coder, ok := agent["coder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", coder["model"])
	assert.Equal(t, temp, coder["temperature"])
	assert.Equal(t, "medium", coder["reasoningEffort"])
}

func TestPatchWorktreeConfig_UsesHarnessForModelNormalization(t *testing.T) {
	dir := t.TempDir()
	opencodeConfig := `{
	  "agent": {
	    "coder": {
	      "model": "legacy/model",
	      "temperature": 0.4,
	      "reasoningEffort": "medium"
	    }
	  }
}
`
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, []byte(opencodeConfig), 0o644))

	temp := 0.5
	agents := []harness.AgentConfig{{
		Role:        "coder",
		Harness:     "codex",
		Model:       "gpt-5-codex",
		Temperature: &temp,
		Effort:      "medium",
	}}

	err := PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	updated, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assertValidJSON(t, string(updated))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(updated, &parsed))
	agent, ok := parsed["agent"].(map[string]any)
	require.True(t, ok)
	coder, ok := agent["coder"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gpt-5-codex", coder["model"])
}

func TestPatchWorktreeConfig_Idempotent_NoRewriteWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	opencodeConfigBytes, err := json.Marshal(map[string]any{
		"mcp": map[string]any{
			"kasmos": map[string]any{
				"type":    "remote",
				"url":     "http://127.0.0.1:7434/mcp",
				"enabled": true,
			},
		},
		"agent": map[string]any{
			"coder": map[string]any{
				"model":           "anthropic/claude-sonnet-4-6",
				"temperature":     0.3,
				"reasoningEffort": "medium",
			},
		},
	})
	require.NoError(t, err)
	configPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(configPath, opencodeConfigBytes, 0o644))

	temp := 0.3
	agents := []harness.AgentConfig{{
		Role:        "coder",
		Harness:     "opencode",
		Model:       "anthropic/claude-sonnet-4-6",
		Temperature: &temp,
		Effort:      "medium",
	}}

	before, err := os.ReadFile(configPath)
	require.NoError(t, err)

	err = PatchWorktreeConfig(dir, agents)
	require.NoError(t, err)

	after, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Equal(t, before, after)
}

func TestLoadReviewPrompt_ContainsTieredStructure(t *testing.T) {
	prompt := LoadReviewPrompt("test-plan.md", "test-plan", "myproject", 2, "Round 1 — changes required")
	assert.Contains(t, prompt, "Phase 0")
	assert.Contains(t, prompt, "Phase 1")
	assert.Contains(t, prompt, "Phase 2")
	assert.Contains(t, prompt, "Phase 3")
	assert.Contains(t, prompt, "change profile")
	assert.Contains(t, prompt, "DECISION:")
	assert.Contains(t, prompt, "Current review round: 2")
	assert.Contains(t, prompt, "Round 1 — changes required")
	assert.Contains(t, prompt, "{{true|false}}", "change profile option markers must remain in rendered output")
	assert.NotContains(t, prompt, "{{PLAN_FILE}}")
	assert.NotContains(t, prompt, "{{PLAN_FILENAME}}")
	assert.NotContains(t, prompt, "{{PLAN_NAME}}")
	assert.NotContains(t, prompt, "{{PROJECT}}")
}

func TestLoadReviewPrompt_UsesMergeBase(t *testing.T) {
	prompt := LoadReviewPrompt("test-plan.md", "test-plan", "myproject", 1, "")
	assert.Contains(t, prompt, "merge-base")
	assert.Contains(t, prompt, "MERGE_BASE")
	assert.NotContains(t, prompt, "git diff main..HEAD",
		"bare main..HEAD without merge-base will break in worktrees with diverged main")
	assert.NotContains(t, prompt, "git diff --stat main..HEAD")
	assert.NotContains(t, prompt, "git diff --name-only main..HEAD")
	assert.NotContains(t, prompt, "git log main..HEAD")
}

func TestLoadReviewPrompt_UsesGatewayReviewSignals(t *testing.T) {
	prompt := LoadReviewPrompt("test-plan.md", "test-plan", "myproject", 1, "")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"review-approved\", plan_file: \"test-plan.md\", project: \"myproject\"")
	assert.Contains(t, prompt, "signal_create` (signal_type: \"review-changes\", plan_file: \"test-plan.md\", project: \"myproject\"")
	assert.Contains(t, prompt, "kas signal emit review_approved test-plan.md")
	assert.Contains(t, prompt, "kas signal emit review_changes_requested test-plan.md")
	assert.NotContains(t, prompt, ".kasmos/signals/review-approved-")
	assert.NotContains(t, prompt, ".kasmos/signals/review-changes-")
}

func TestLoadReviewPrompt_SubstitutesProject(t *testing.T) {
	prompt := LoadReviewPrompt("test-plan.md", "test-plan", "myproject", 1, "")
	assert.Contains(t, prompt, `project: "myproject"`)
	assert.NotContains(t, prompt, "{{PROJECT}}")
}

func TestLoadReviewPrompt_MentionsReadinessReviewHandoff(t *testing.T) {
	prompt := LoadReviewPrompt("test-plan.md", "test-plan", "myproject", 1, "")
	assert.Contains(t, prompt, "Readiness review handoff")
	assert.Contains(t, prompt, "verifying")
	assert.Contains(t, prompt, "auto_readiness_review")
	assert.NotContains(t, prompt, "readiness_reviewing")

	// triage buckets
	assert.Contains(t, prompt, "blocker")
	assert.Contains(t, prompt, "quality")
	assert.Contains(t, prompt, "note")

	// self-fix ceiling knob
	assert.Contains(t, prompt, "readiness_self_fix_max_lines")

	// shared static gate language
	assert.Contains(t, prompt, "post-fix")
}

func ptrFloat(f float64) *float64 { return &f }

func TestSyncScaffold_UpdatesSkillsAndAgentPrompts(t *testing.T) {
	dir := t.TempDir()
	temp := 0.1
	agents := []harness.AgentConfig{{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Temperature: &temp, Enabled: true}, {Role: "planner", Harness: "claude", Model: "claude-opus-4-6", Temperature: &temp, Enabled: true}, {Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true}, {Role: "planner", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true}}
	_, err := ScaffoldAll(dir, agents, nil, false)
	require.NoError(t, err)
	skillFile := filepath.Join(dir, ".agents", "skills", "cli-tools", "SKILL.md")
	require.NoError(t, os.WriteFile(skillFile, []byte("old"), 0o644))
	agentFile := filepath.Join(dir, ".claude", "agents", "coder.md")
	require.NoError(t, os.WriteFile(agentFile, []byte("old"), 0o644))
	plannerFile := filepath.Join(dir, ".claude", "agents", "planner.md")
	require.NoError(t, os.WriteFile(plannerFile, []byte("old"), 0o644))
	opencodePlannerFile := filepath.Join(dir, ".opencode", "agents", "planner.md")
	require.NoError(t, os.WriteFile(opencodePlannerFile, []byte("old"), 0o644))
	cfgPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"agent":{"coder":{"model":"anthropic/claude-sonnet-4-6","temperature":0.1,"reasoningEffort":"medium","customField":"preserved"}}}`), 0o644))
	results, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Contains(t, results, WriteResult{Path: ".mcp.json", Created: false})
	content, err := os.ReadFile(skillFile)
	require.NoError(t, err)
	assert.NotEqual(t, "old", string(content))
	content, err = os.ReadFile(agentFile)
	require.NoError(t, err)
	assert.NotEqual(t, "old", string(content))
	content, err = os.ReadFile(plannerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "task_update_content")
	assert.Contains(t, string(content), "signal_create")
	assert.Contains(t, string(content), "kas signal emit planner_finished <plan-file>")
	assert.NotContains(t, string(content), ".kasmos/signals/planner-finished-<plan-file>")
	content, err = os.ReadFile(opencodePlannerFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "task_update_content")
	assert.Contains(t, string(content), "signal_create")
	assert.Contains(t, string(content), "kas signal emit planner_finished <plan-file>")
	assert.NotContains(t, string(content), ".kasmos/signals/planner-finished-<plan-file>")
	content, err = os.ReadFile(cfgPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "preserved")
	assert.Contains(t, string(content), `"kasmos"`)
	_, err = os.Readlink(filepath.Join(dir, ".claude", "skills", "cli-tools"))
	assert.NoError(t, err)

	settingsData, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)
	var syncSettings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &syncSettings))
	assertAllMCPToolsAllowed(t, syncSettings)
}

func TestSyncScaffold_CreatesFromScratch(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true}}
	results, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	assert.Contains(t, results, WriteResult{Path: ".mcp.json", Created: true})
	assert.FileExists(t, filepath.Join(dir, ".agents", "skills", "cli-tools", "SKILL.md"))
	assert.FileExists(t, filepath.Join(dir, ".claude", "agents", "coder.md"))
	_, err = os.Readlink(filepath.Join(dir, ".claude", "skills", "cli-tools"))
	assert.NoError(t, err)

	settingsData, err := os.ReadFile(filepath.Join(dir, ".claude", "settings.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(settingsData, &settings))
	assertAllMCPToolsAllowed(t, settings)
}

func TestSyncScaffold_RendersConfigWhenMissing(t *testing.T) {
	dir := t.TempDir()
	temp := 0.1
	agents := []harness.AgentConfig{{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6", Temperature: &temp, Effort: "medium", Enabled: true}}
	results, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	configPath := filepath.Join(dir, "opencode.jsonc")
	assert.FileExists(t, configPath)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "anthropic/claude-sonnet-4-6")
	assert.Contains(t, results, WriteResult{Path: "opencode.jsonc", Created: true})
}

// TestSyncScaffold_SkipsOpencodeConfigForNonOpencodeRepo verifies that SyncScaffold
// does not create opencode.jsonc when no opencode agents are configured and
// no existing file is present — matching ScaffoldAll behaviour.
func TestSyncScaffold_SkipsOpencodeConfigForNonOpencodeRepo(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}
	_, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	assert.NoFileExists(t, filepath.Join(dir, "opencode.jsonc"),
		"opencode.jsonc must not be created for claude-only repos")
}

// TestSyncScaffold_CodexOnlyWritesMCPJSON verifies that SyncScaffold writes the
// TestSyncScaffold_CodexWritesCodexMCPConfig covers the sync path for codex-only
// repos: a fresh .codex/config.toml is emitted, and existing non-kasmos sections
// (e.g. model_providers, other mcp_servers entries) are preserved when the
// kasmos block is inserted.
func TestSyncScaffold_CodexWritesCodexMCPConfig(t *testing.T) {
	t.Run("creates fresh .codex/config.toml for codex-only", func(t *testing.T) {
		dir := t.TempDir()
		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
		}
		_, err := SyncScaffold(dir, agents)
		require.NoError(t, err)

		assert.NoDirExists(t, filepath.Join(dir, ".claude"),
			"claude harness must not be scaffolded when syncing a codex-only repo")
		assert.NoFileExists(t, filepath.Join(dir, ".mcp.json"),
			"codex-only sync must not emit .mcp.json — that is Claude Code's format")

		cfgPath := filepath.Join(dir, ".codex", "config.toml")
		require.FileExists(t, cfgPath)
		data, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		var parsed map[string]any
		_, decodeErr := toml.Decode(string(data), &parsed)
		require.NoError(t, decodeErr)
		kasmos, ok := parsed["mcp_servers"].(map[string]any)["kasmos"].(map[string]any)
		require.True(t, ok, "kasmos entry must be present")
		assert.Equal(t, "workspace-write", parsed["sandbox_mode"],
			"fresh codex scaffold must opt into workspace-write sandbox mode")
		sandboxCfg, ok := parsed["sandbox_workspace_write"].(map[string]any)
		require.True(t, ok, "sandbox_workspace_write table must be present")
		assert.Equal(t, true, sandboxCfg["network_access"],
			"workspace-write sandbox must allow network access for the shared MCP endpoint")
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
		assert.Equal(t, "auto", kasmos["default_tools_approval_mode"], "server-level approval mode must be set")
		assert.NotContains(t, kasmos, "command")
		assert.NotContains(t, kasmos, "args")
	})

	t.Run("preserves unrelated mcp_servers entries and user comments", func(t *testing.T) {
		dir := t.TempDir()
		existing := `# user's custom codex config — do not touch my notes
model = "gpt-5.3-codex"

[mcp_servers.other]
command = "/keep/me"
args = ["serve"]
`
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644))

		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
		}
		_, err := SyncScaffold(dir, agents)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		content := string(data)

		assert.Contains(t, content, "# user's custom codex config — do not touch my notes",
			"user comments must be preserved")
		assert.Contains(t, content, `model = "gpt-5.3-codex"`,
			"top-level keys must be preserved")

		var parsed map[string]any
		_, decodeErr := toml.Decode(content, &parsed)
		require.NoError(t, decodeErr)
		servers, ok := parsed["mcp_servers"].(map[string]any)
		require.True(t, ok)
		kasmos, ok := servers["kasmos"].(map[string]any)
		require.True(t, ok, "kasmos entry must be inserted")
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
		other, ok := servers["other"].(map[string]any)
		require.True(t, ok, "unrelated MCP servers must be preserved")
		assert.Equal(t, "/keep/me", other["command"])
	})
}

// TestEnsureCodexMCPEntry exercises the .codex/config.toml merge logic in
// isolation: fresh create, idempotent re-run, refresh-in-place for stale
// stdio entries, preservation of unrelated sections.
func TestEnsureCodexMCPEntry(t *testing.T) {
	t.Run("creates fresh config when file is absent", func(t *testing.T) {
		dir := t.TempDir()

		result, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		assert.True(t, result.Created)
		assert.Equal(t, filepath.Join(".codex", "config.toml"), result.Path)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		var parsed map[string]any
		_, decodeErr := toml.Decode(string(data), &parsed)
		require.NoError(t, decodeErr)
		kasmos, ok := parsed["mcp_servers"].(map[string]any)["kasmos"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "workspace-write", parsed["sandbox_mode"])
		sandboxCfg, ok := parsed["sandbox_workspace_write"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, sandboxCfg["network_access"])
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
	})

	t.Run("is idempotent when entry is already correct", func(t *testing.T) {
		dir := t.TempDir()

		_, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		before, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)

		result, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		assert.False(t, result.Created, "second call must be a no-op")

		after, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		assert.Equal(t, before, after, "file bytes must not change on idempotent ensure")
	})

	t.Run("refreshes otherwise-correct kasmos entry when sandbox config is missing", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `# keep this note
model = "gpt-5.3-codex"

[mcp_servers.kasmos]
url = "http://127.0.0.1:7434/mcp"
default_tools_approval_mode = "auto"

[mcp_servers.other]
command = "/keep/me"
`
		cfgPath := filepath.Join(dir, ".codex", "config.toml")
		require.NoError(t, os.WriteFile(cfgPath, []byte(existing), 0o644))

		result, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		assert.True(t, result.Created, "missing sandbox config must trigger a rewrite")

		data, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "# keep this note")
		assert.Contains(t, content, `model = "gpt-5.3-codex"`)

		var parsed map[string]any
		_, decodeErr := toml.Decode(content, &parsed)
		require.NoError(t, decodeErr)
		assert.Equal(t, "workspace-write", parsed["sandbox_mode"])
		sandboxCfg, ok := parsed["sandbox_workspace_write"].(map[string]any)
		require.True(t, ok, "sandbox_workspace_write table must be inserted")
		assert.Equal(t, true, sandboxCfg["network_access"])

		servers := parsed["mcp_servers"].(map[string]any)
		kasmos := servers["kasmos"].(map[string]any)
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
		assert.Equal(t, "auto", kasmos["default_tools_approval_mode"])

		other := servers["other"].(map[string]any)
		assert.Equal(t, "/keep/me", other["command"])
	})

	t.Run("refreshes stale stdio entry and preserves other sections", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `# top-level notes
model = "gpt-5.3-codex"

[mcp_servers.kasmos]
command = "/some/old/kas"
args = ["mcp"]

[mcp_servers.other]
command = "/keep/me"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644))

		result, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		assert.True(t, result.Created)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		content := string(data)

		assert.Contains(t, content, "# top-level notes", "user comments outside the kasmos block must survive")
		assert.Contains(t, content, `model = "gpt-5.3-codex"`)

		var parsed map[string]any
		_, decodeErr := toml.Decode(content, &parsed)
		require.NoError(t, decodeErr)
		servers := parsed["mcp_servers"].(map[string]any)

		kasmos := servers["kasmos"].(map[string]any)
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
		assert.NotContains(t, kasmos, "command", "stale stdio command must be stripped")
		assert.NotContains(t, kasmos, "args", "stale stdio args must be stripped")

		other, ok := servers["other"].(map[string]any)
		require.True(t, ok, "sibling mcp_servers entries must be preserved")
		assert.Equal(t, "/keep/me", other["command"])
	})

	t.Run("appends kasmos block when file exists without one", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `model = "gpt-5.3-codex"

[mcp_servers.other]
command = "/keep/me"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644))

		_, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		var parsed map[string]any
		_, decodeErr := toml.Decode(string(data), &parsed)
		require.NoError(t, decodeErr)
		servers := parsed["mcp_servers"].(map[string]any)
		assert.Contains(t, servers, "kasmos")
		assert.Contains(t, servers, "other")
	})

	t.Run("removes nested tool subtables and preserves unrelated sections", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		// Fixture: kasmos block with per-tool override subtables reopened later in
		// the file, plus unrelated sections that must survive untouched.
		existing := `# project config

[mcp_servers.kasmos]
url = "http://127.0.0.1:7434/mcp"

[mcp_servers.other]
command = "/keep/me"

[mcp_servers.kasmos.tools.read_file]
approval_mode = "prompt"

[features]
codex_hooks = true

[mcp_servers."kasmos".tools.grep]
approval_mode = "prompt"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644))

		result, err := EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		assert.True(t, result.Created, "stale config must be rewritten")

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		content := string(data)

		// Nested tool tables must be gone.
		assert.NotContains(t, content, "mcp_servers.kasmos.tools",
			"per-tool subtables must be removed")

		// Unrelated sections must survive.
		assert.Contains(t, content, "# project config", "top-level comment must be preserved")
		assert.Contains(t, content, "[mcp_servers.other]", "unrelated mcp server must survive")
		assert.Contains(t, content, "[features]", "features table must survive")
		assert.Contains(t, content, "codex_hooks = true", "features flag must survive")

		var parsed map[string]any
		_, decodeErr := toml.Decode(content, &parsed)
		require.NoError(t, decodeErr, "rewritten config must be valid TOML")

		kasmos := parsed["mcp_servers"].(map[string]any)["kasmos"].(map[string]any)
		assert.Equal(t, "http://127.0.0.1:7434/mcp", kasmos["url"])
		assert.Equal(t, "auto", kasmos["default_tools_approval_mode"],
			"server-level approval mode must be written")
		assert.NotContains(t, kasmos, "tools", "per-tool subtable must not appear in parsed entry")

		other := parsed["mcp_servers"].(map[string]any)["other"].(map[string]any)
		assert.Equal(t, "/keep/me", other["command"])

		beforeSecondRun := content
		result, err = EnsureCodexMCPEntry(dir)
		require.NoError(t, err)
		assert.False(t, result.Created, "already-clean config must stay idempotent")

		data, err = os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		assert.Equal(t, beforeSecondRun, string(data), "second run must not rewrite the file")
	})
}

// TestPatchCodexTOML_RemovesAllKasmosDescendantBlocks proves that patchCodexTOML
// removes descendant kasmos table headers wherever they appear in the file,
// including quoted and reopened tables after unrelated sections.
func TestPatchCodexTOML_RemovesAllKasmosDescendantBlocks(t *testing.T) {
	input := `[mcp_servers.kasmos]
url = "http://127.0.0.1:7434/mcp"

[mcp_servers.other]
command = "/keep/me"

[mcp_servers.kasmos.tools.read_file]
approval_mode = "prompt"

[mcp_servers."kasmos".tools.grep]
approval_mode = "prompt"
`

	patched := patchCodexTOML(input)
	assert.NotContains(t, patched, "mcp_servers.kasmos.tools",
		"patchCodexTOML must remove nested tool tables")
	assert.NotContains(t, patched, `mcp_servers."kasmos".tools`,
		"patchCodexTOML must remove quoted nested tool tables")
	assert.Contains(t, patched, "default_tools_approval_mode",
		"patchCodexTOML must write the server-level approval mode")
	assert.Contains(t, patched, "[mcp_servers.other]",
		"patchCodexTOML must preserve unrelated sibling server")
	assert.Equal(t, 1, strings.Count(patched, "[mcp_servers.kasmos]"),
		"patchCodexTOML must leave exactly one root kasmos block")
}

func TestEnsureCodexTrustedProjectEntry(t *testing.T) {
	t.Run("creates fresh trust entry when config is absent", func(t *testing.T) {
		home := t.TempDir()
		project := filepath.Join(home, "work", "repo")

		result, err := EnsureCodexTrustedProjectEntry(home, project)
		require.NoError(t, err)
		assert.True(t, result.Created)
		assert.Equal(t, filepath.Join(home, ".codex", "config.toml"), result.Path)

		data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
		require.NoError(t, err)
		var parsed map[string]any
		_, decodeErr := toml.Decode(string(data), &parsed)
		require.NoError(t, decodeErr)
		projects, ok := parsed["projects"].(map[string]any)
		require.True(t, ok)
		entry, ok := projects[project].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "trusted", entry["trust_level"])
	})

	t.Run("is idempotent when trust entry is already correct", func(t *testing.T) {
		home := t.TempDir()
		project := filepath.Join(home, "work", "repo")

		_, err := EnsureCodexTrustedProjectEntry(home, project)
		require.NoError(t, err)
		before, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
		require.NoError(t, err)

		result, err := EnsureCodexTrustedProjectEntry(home, project)
		require.NoError(t, err)
		assert.False(t, result.Created, "second call must be a no-op")

		after, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("updates trust level in place and preserves sibling config", func(t *testing.T) {
		home := t.TempDir()
		project := filepath.Join(home, "work", "repo")
		otherProject := filepath.Join(home, "work", "other")
		cfgPath := filepath.Join(home, ".codex", "config.toml")
		require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0o755))
		existing := fmt.Sprintf(`# keep this note
model = "gpt-5.4"

[projects.%s]
trust_level = "untrusted"
sandbox_mode = "workspace-write"

[projects.%s]
trust_level = "trusted"
`, strconv.Quote(project), strconv.Quote(otherProject))
		require.NoError(t, os.WriteFile(cfgPath, []byte(existing), 0o644))

		result, err := EnsureCodexTrustedProjectEntry(home, project)
		require.NoError(t, err)
		assert.True(t, result.Created)

		data, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		content := string(data)
		assert.Contains(t, content, "# keep this note")
		assert.Contains(t, content, `model = "gpt-5.4"`)
		assert.Contains(t, content, `sandbox_mode = "workspace-write"`)

		var parsed map[string]any
		_, decodeErr := toml.Decode(content, &parsed)
		require.NoError(t, decodeErr)
		projects := parsed["projects"].(map[string]any)
		entry := projects[project].(map[string]any)
		assert.Equal(t, "trusted", entry["trust_level"])
		assert.Equal(t, "workspace-write", entry["sandbox_mode"])
		other := projects[otherProject].(map[string]any)
		assert.Equal(t, "trusted", other["trust_level"])
	})
}

// TestWriteCodexEnforcementHook_WritesExecutableScript covers the hook script
// installer: the file is written at the expected relative path, has exec
// permissions, and contains the shared enforcement body (not just an empty
// stub).
func TestWriteCodexEnforcementHook_WritesExecutableScript(t *testing.T) {
	dir := t.TempDir()

	result, err := WriteCodexEnforcementHook(dir, false)
	require.NoError(t, err)
	assert.True(t, result.Created)
	assert.Equal(t, ".codex/hooks/enforce-cli-tools.sh", result.Path)

	hookPath := filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh")
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0o111, "script must be executable")

	body, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.Contains(t, string(body), "#!/bin/bash")
	assert.Contains(t, string(body), "BLOCKED: 'grep' is banned",
		"script body must come from the shared enforcement source of truth")
}

func TestCLIToolsEnforcementTemplate_MatchesSharedScript(t *testing.T) {
	body, err := templates.ReadFile("templates/claude/enforce-cli-tools.sh")
	require.NoError(t, err)
	assert.Equal(t, harness.CLIToolsEnforcementScript, string(body))
}

// TestEnsureCodexHooksJSON exercises the hooks.json merge logic: fresh create,
// idempotent re-run, and preservation of user-added events and matcher groups.
func TestEnsureCodexHooksJSON(t *testing.T) {
	t.Run("creates fresh hooks.json with PreToolUse Bash entry", func(t *testing.T) {
		dir := t.TempDir()

		result, err := EnsureCodexHooksJSON(dir)
		require.NoError(t, err)
		assert.True(t, result.Created)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))

		preToolUse, ok := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
		require.True(t, ok, "hooks.PreToolUse must be an array")
		require.Len(t, preToolUse, 1)
		group := preToolUse[0].(map[string]any)
		assert.Equal(t, "Bash", group["matcher"])
		hook := group["hooks"].([]any)[0].(map[string]any)
		assert.Equal(t, "command", hook["type"])
		assert.Equal(t, ".codex/hooks/enforce-cli-tools.sh", hook["command"])
	})

	t.Run("is idempotent when entry already present", func(t *testing.T) {
		dir := t.TempDir()
		_, err := EnsureCodexHooksJSON(dir)
		require.NoError(t, err)
		before, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)

		result, err := EnsureCodexHooksJSON(dir)
		require.NoError(t, err)
		assert.False(t, result.Created)

		after, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after))
	})

	t.Run("preserves user-added events and matchers", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "notify.sh" }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "user-lint.sh" }
        ]
      }
    ]
  }
}
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "hooks.json"), []byte(existing), 0o644))

		_, err := EnsureCodexHooksJSON(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))

		hooks := settings["hooks"].(map[string]any)
		assert.Contains(t, hooks, "Stop", "user-added Stop hook must survive")

		preToolUse := hooks["PreToolUse"].([]any)
		assert.Len(t, preToolUse, 2, "user Bash matcher group must coexist with kasmos's")

		var sawUser, sawKasmos bool
		for _, g := range preToolUse {
			group := g.(map[string]any)
			for _, h := range group["hooks"].([]any) {
				cmd := h.(map[string]any)["command"].(string)
				if cmd == "user-lint.sh" {
					sawUser = true
				}
				if cmd == ".codex/hooks/enforce-cli-tools.sh" {
					sawKasmos = true
				}
			}
		}
		assert.True(t, sawUser, "user-lint.sh must still be registered")
		assert.True(t, sawKasmos, "kasmos enforcement hook must be registered")
	})
}

// TestEnsureCodexFeaturesFlag covers the [features] codex_hooks switch that
// codex CLI requires before it will even read hooks.json.
func TestEnsureCodexFeaturesFlag(t *testing.T) {
	t.Run("creates fresh file with features table", func(t *testing.T) {
		dir := t.TempDir()

		result, err := EnsureCodexFeaturesFlag(dir)
		require.NoError(t, err)
		assert.True(t, result.Created)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		var parsed map[string]any
		_, decodeErr := toml.Decode(string(data), &parsed)
		require.NoError(t, decodeErr)
		features := parsed["features"].(map[string]any)
		assert.Equal(t, true, features["codex_hooks"])
	})

	t.Run("is idempotent when flag already true", func(t *testing.T) {
		dir := t.TempDir()
		_, err := EnsureCodexFeaturesFlag(dir)
		require.NoError(t, err)
		before, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)

		result, err := EnsureCodexFeaturesFlag(dir)
		require.NoError(t, err)
		assert.False(t, result.Created)

		after, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		assert.Equal(t, before, after)
	})

	t.Run("adds codex_hooks to existing features table without clobbering siblings", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `[features]
# user-enabled experimental flag
some_other_flag = true

[mcp_servers.other]
command = "/keep/me"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644))

		_, err := EnsureCodexFeaturesFlag(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		content := string(data)

		assert.Contains(t, content, "# user-enabled experimental flag",
			"user comments inside [features] must survive")
		assert.Contains(t, content, "some_other_flag = true",
			"sibling feature flags must survive")

		var parsed map[string]any
		_, decodeErr := toml.Decode(content, &parsed)
		require.NoError(t, decodeErr)
		features := parsed["features"].(map[string]any)
		assert.Equal(t, true, features["codex_hooks"])
		assert.Equal(t, true, features["some_other_flag"])

		other := parsed["mcp_servers"].(map[string]any)["other"].(map[string]any)
		assert.Equal(t, "/keep/me", other["command"], "unrelated tables must survive")
	})

	t.Run("appends features table when file has other content but no features", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `model = "gpt-5.3-codex"

[mcp_servers.kasmos]
url = "http://127.0.0.1:7434/mcp"
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(existing), 0o644))

		_, err := EnsureCodexFeaturesFlag(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		var parsed map[string]any
		_, decodeErr := toml.Decode(string(data), &parsed)
		require.NoError(t, decodeErr)
		assert.Equal(t, "gpt-5.3-codex", parsed["model"])
		assert.Equal(t, true, parsed["features"].(map[string]any)["codex_hooks"])
		assert.Equal(t, "http://127.0.0.1:7434/mcp",
			parsed["mcp_servers"].(map[string]any)["kasmos"].(map[string]any)["url"])
	})
}

// TestWriteCodexProject_WiresHooks verifies that the higher-level scaffold
// entrypoint installs the hook script, hooks.json, and feature flag together.
// Regression guard for the codex /mcp listing plus CLI enforcement.
func TestWriteCodexProject_WiresHooks(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
	}

	_, err := WriteCodexProject(dir, agents, allTools, false, true)
	require.NoError(t, err)

	hookPath := filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh")
	assert.FileExists(t, hookPath)
	info, err := os.Stat(hookPath)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode().Perm()&0o111)

	hooksJSON, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(hooksJSON, &settings))
	preToolUse := settings["hooks"].(map[string]any)["PreToolUse"].([]any)
	require.Len(t, preToolUse, 1)
	cmd := preToolUse[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"]
	assert.Equal(t, ".codex/hooks/enforce-cli-tools.sh", cmd)

	cfgData, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	require.NoError(t, err)
	var cfg map[string]any
	_, decodeErr := toml.Decode(string(cfgData), &cfg)
	require.NoError(t, decodeErr)
	assert.Equal(t, true, cfg["features"].(map[string]any)["codex_hooks"],
		"codex_hooks feature flag must be set or hooks.json is ignored")
	assert.Equal(t, "http://127.0.0.1:7434/mcp",
		cfg["mcp_servers"].(map[string]any)["kasmos"].(map[string]any)["url"])
}

// TestWriteCodexProject_EnforcementDisabled verifies that when enforcementEnabled=false
// the enforcement script, hooks.json PreToolUse entry, and codex_hooks feature flag
// are not written but AGENTS.md and the MCP entry still land on disk.
func TestWriteCodexProject_EnforcementDisabled(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
	}

	_, err := WriteCodexProject(dir, agents, allTools, false, false)
	require.NoError(t, err)

	// AGENTS.md and config.toml (MCP entry) must still be written.
	assert.FileExists(t, filepath.Join(dir, ".codex", "AGENTS.md"))
	cfgData, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	require.NoError(t, err)
	assert.Contains(t, string(cfgData), "kasmos", "MCP entry must be present even when enforcement is disabled")

	// Enforcement script must NOT be written.
	assert.NoFileExists(t, filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh"),
		"enforcement script must not be written when enforcement is disabled")

	// hooks.json must NOT exist (or must not contain the kasmos PreToolUse entry).
	hooksPath := filepath.Join(dir, ".codex", "hooks.json")
	if _, statErr := os.Stat(hooksPath); statErr == nil {
		hooksData, err := os.ReadFile(hooksPath)
		require.NoError(t, err)
		assert.NotContains(t, string(hooksData), "enforce-cli-tools.sh",
			"hooks.json must not reference the enforcement script when enforcement is disabled")
	}

	// codex_hooks feature flag must NOT be set.
	var cfg map[string]any
	_, decodeErr := toml.Decode(string(cfgData), &cfg)
	require.NoError(t, decodeErr)
	features, _ := cfg["features"].(map[string]any)
	flag, _ := features["codex_hooks"].(bool)
	assert.False(t, flag, "codex_hooks feature flag must not be set when enforcement is disabled")
}

// TestRemoveCodexEnforcementHook verifies that disabling enforcement on a
// previously scaffolded project removes the hook script and the kasmos
// PreToolUse entry while leaving all other hooks.json content intact and
// leaving an existing codex_hooks feature flag untouched.
func TestRemoveCodexEnforcementHook(t *testing.T) {
	t.Run("removes script and kasmos hooks.json entry", func(t *testing.T) {
		dir := t.TempDir()
		agents := []harness.AgentConfig{
			{Role: "coder", Harness: "codex", Model: "gpt-5.3-codex", Enabled: true},
		}
		// First install with enforcement enabled.
		_, err := WriteCodexProject(dir, agents, allTools, false, true)
		require.NoError(t, err)

		// Confirm the artifacts exist.
		require.FileExists(t, filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh"))

		// Now remove.
		results, err := RemoveCodexEnforcementHook(dir)
		require.NoError(t, err)
		assert.NotEmpty(t, results, "should report at least one change")
		for _, r := range results {
			assert.True(t, r.Created, "each changed path should report Created=true: %s", r.Path)
		}

		// Script must be gone.
		assert.NoFileExists(t, filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh"))

		// hooks.json must no longer reference the enforcement script.
		hooksData, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)
		assert.NotContains(t, string(hooksData), "enforce-cli-tools.sh")
	})

	t.Run("preserves user Stop hooks and unrelated Bash matchers", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          { "type": "command", "command": "notify.sh" }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "user-lint.sh" }
        ]
      },
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": ".codex/hooks/enforce-cli-tools.sh" }
        ]
      }
    ]
  }
}
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "hooks.json"), []byte(existing), 0o644))

		_, err := RemoveCodexEnforcementHook(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))

		hooks := settings["hooks"].(map[string]any)
		assert.Contains(t, hooks, "Stop", "user Stop hook must survive")

		preToolUse, ok := hooks["PreToolUse"].([]any)
		require.True(t, ok)
		assert.Len(t, preToolUse, 1, "only kasmos group must be removed; user group must remain")

		cmd := preToolUse[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
		assert.Equal(t, "user-lint.sh", cmd, "user-lint.sh must still be present")
	})

	t.Run("leaves existing codex_hooks feature flag untouched", func(t *testing.T) {
		dir := t.TempDir()
		// Pre-populate config.toml with the feature flag set.
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		cfgContent := "[features]\ncodex_hooks = true\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(cfgContent), 0o644))

		// Plant a hooks.json with the kasmos entry.
		hooksContent := `{
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
}
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "hooks.json"), []byte(hooksContent), 0o644))

		_, err := RemoveCodexEnforcementHook(dir)
		require.NoError(t, err)

		// Feature flag in config.toml must be untouched.
		cfgData, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
		require.NoError(t, err)
		var cfg map[string]any
		_, decodeErr := toml.Decode(string(cfgData), &cfg)
		require.NoError(t, decodeErr)
		flag, _ := cfg["features"].(map[string]any)["codex_hooks"].(bool)
		assert.True(t, flag, "codex_hooks feature flag must be left untouched by RemoveCodexEnforcementHook")
	})

	t.Run("shared group keeps user hook and removes only kasmos entry", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, ".codex"), 0o755))
		existing := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": "user-lint.sh" },
          { "type": "command", "command": ".codex/hooks/enforce-cli-tools.sh" }
        ]
      }
    ]
  }
}
`
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "hooks.json"), []byte(existing), 0o644))

		_, err := RemoveCodexEnforcementHook(dir)
		require.NoError(t, err)

		data, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
		require.NoError(t, err)
		var settings map[string]any
		require.NoError(t, json.Unmarshal(data, &settings))

		hooks := settings["hooks"].(map[string]any)
		preToolUse, ok := hooks["PreToolUse"].([]any)
		require.True(t, ok, "PreToolUse must still exist when user hooks remain")
		require.Len(t, preToolUse, 1, "shared group must survive with remaining hooks")

		group := preToolUse[0].(map[string]any)
		hooksList := group["hooks"].([]any)
		require.Len(t, hooksList, 1, "only kasmos entry should be removed from group")
		cmd := hooksList[0].(map[string]any)["command"].(string)
		assert.Equal(t, "user-lint.sh", cmd)
		assert.NotContains(t, string(data), "enforce-cli-tools.sh")
	})

	t.Run("missing files are treated as success", func(t *testing.T) {
		dir := t.TempDir()
		results, err := RemoveCodexEnforcementHook(dir)
		require.NoError(t, err)
		assert.Empty(t, results, "no changes when nothing to remove")
	})
}

// TestLoadEnforcementConfigForDir_Worktree verifies that a worktree directory
// correctly resolves the enforcement config from the main repository root.
func TestLoadEnforcementConfigForDir_Worktree(t *testing.T) {
	// Build a minimal git worktree layout:
	//   <mainRepo>/              — main repo root
	//     .git/                  — real .git directory
	//     .kasmos/config.toml   — config with enforcement disabled for codex
	//   <worktreeDir>/           — simulated worktree
	//     .git                   — file pointing at worktree git dir
	//     <gitDir>/              — worktree-specific git dir
	//       commondir            — points back to mainRepo/.git

	mainRepo := t.TempDir()
	worktreeDir := t.TempDir()

	// Create main repo .git directory.
	mainGitDir := filepath.Join(mainRepo, ".git")
	require.NoError(t, os.MkdirAll(mainGitDir, 0o755))

	// Create worktree-specific git dir.
	worktreeGitDir := filepath.Join(worktreeDir, ".git_wt")
	require.NoError(t, os.MkdirAll(worktreeGitDir, 0o755))

	// .git file in worktree points at worktreeGitDir.
	gitFileContent := "gitdir: " + worktreeGitDir
	require.NoError(t, os.WriteFile(filepath.Join(worktreeDir, ".git"), []byte(gitFileContent), 0o644))

	// commondir in worktreeGitDir points at main repo's .git.
	require.NoError(t, os.WriteFile(filepath.Join(worktreeGitDir, "commondir"), []byte(mainGitDir), 0o644))

	// Write config.toml in main repo with enforcement disabled for codex.
	kasmosDir := filepath.Join(mainRepo, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))
	cfgContent := "[enforcement]\ncodex = false\n"
	require.NoError(t, os.WriteFile(filepath.Join(kasmosDir, "config.toml"), []byte(cfgContent), 0o644))

	cfg, err := loadEnforcementConfigForDir(worktreeDir)
	require.NoError(t, err)
	require.NotNil(t, cfg, "should find config from main repo root")
	assert.False(t, cfg.IsEnforcementEnabled("codex"),
		"worktree scaffold should read enforcement=false from main repo config")
}

// TestSyncScaffold_PatchesExistingOpencodeConfigEvenWithoutOpencodeHarness verifies
// that an existing opencode.jsonc is still patched even when no opencode agents are
// passed — keeping a pre-existing config in sync when the user later switches profiles.
func TestSyncScaffold_PatchesExistingOpencodeConfigEvenWithoutOpencodeHarness(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "opencode.jsonc")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`{"agent":{"coder":{"model":"old","customKey":"keep"}}}`), 0o644))

	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}
	_, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	// File must still exist (patched, not deleted).
	assert.FileExists(t, cfgPath)
}

func TestWriteFile_ContentComparison(t *testing.T) {
	content := []byte("hello world\n")
	different := []byte("different content\n")

	t.Run("force=true, file does not exist, writes and returns true", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.txt")

		written, err := writeFile(path, content, true)
		require.NoError(t, err)
		assert.True(t, written)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("force=true, file exists with same content, skips and returns false", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.txt")
		require.NoError(t, os.WriteFile(path, content, 0o644))

		written, err := writeFile(path, content, true)
		require.NoError(t, err)
		assert.False(t, written)
	})

	t.Run("force=true, file exists with different content, overwrites and returns true", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.txt")
		require.NoError(t, os.WriteFile(path, different, 0o644))

		written, err := writeFile(path, content, true)
		require.NoError(t, err)
		assert.True(t, written)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})

	t.Run("force=false, file exists, skips and returns false", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.txt")
		require.NoError(t, os.WriteFile(path, different, 0o644))

		written, err := writeFile(path, content, false)
		require.NoError(t, err)
		assert.False(t, written)
		// original content must be untouched
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, different, got)
	})

	t.Run("force=false, file does not exist, writes and returns true", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "file.txt")

		written, err := writeFile(path, content, false)
		require.NoError(t, err)
		assert.True(t, written)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, content, got)
	})
}

// TestSyncScaffold_ContentAwareSync verifies that syncing an already-up-to-date
// scaffold directory reports zero files as created/changed.
func TestSyncScaffold_ContentAwareSync(t *testing.T) {
	dir := t.TempDir()
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}

	// First sync — all files should be created.
	firstResults, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	created := 0
	for _, r := range firstResults {
		if r.Created {
			created++
		}
	}
	assert.Greater(t, created, 0, "first sync should create at least one file")

	// Second sync — no content changed, so nothing should be reported as created.
	secondResults, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	for _, r := range secondResults {
		assert.False(t, r.Created, "file %q should be unchanged on second sync", r.Path)
	}

	// Mutate one of the written files, then sync again — only that file should be updated.
	// Skip directory results (Path ends with "/") from EnsureRuntimeDirs.
	var mutated string
	for _, r := range firstResults {
		if r.Created && !strings.HasSuffix(r.Path, "/") {
			mutated = r.Path
			break
		}
	}
	require.NotEmpty(t, mutated, "need a created file to mutate")
	mutatedAbs := filepath.Join(dir, mutated)
	require.NoError(t, os.WriteFile(mutatedAbs, []byte("tampered content\n"), 0o644))

	thirdResults, err := SyncScaffold(dir, agents)
	require.NoError(t, err)
	updatedPaths := []string{}
	for _, r := range thirdResults {
		if r.Created {
			updatedPaths = append(updatedPaths, r.Path)
		}
	}
	assert.Equal(t, []string{mutated}, updatedPaths, "only the mutated file should be reported as updated")
}

// seedStaleCodexEnforcement creates a pre-existing .codex/ directory containing
// the kasmos enforcement script and a hooks.json with the kasmos PreToolUse entry,
// simulating a project that was previously scaffolded with enforcement enabled.
func seedStaleCodexEnforcement(t *testing.T, dir string) {
	t.Helper()
	hooksDir := filepath.Join(dir, ".codex", "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(hooksDir, "enforce-cli-tools.sh"),
		[]byte("#!/bin/sh\nexit 0\n"), 0o755))
	hooksJSON := `{
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
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".codex", "hooks.json"), []byte(hooksJSON), 0o644))
}

// writeCodexEnforcementDisabledConfig writes a .kasmos/config.toml that disables codex enforcement.
func writeCodexEnforcementDisabledConfig(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".kasmos"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, ".kasmos", "config.toml"),
		[]byte("[enforcement]\ncodex = false\n"),
		0o644,
	))
}

// TestScaffoldAll_RemovesStaleCodexEnforcement_WhenDisabledAndNoCodexAgents verifies
// the cleanup-only path: when [enforcement] codex = false and no codex agents are
// passed to ScaffoldAll, any pre-existing .codex/hooks/enforce-cli-tools.sh and
// kasmos PreToolUse entry must be removed even though WriteCodexProject never runs.
// Regression guard for the gating bug Copilot flagged on PR #123.
func TestScaffoldAll_RemovesStaleCodexEnforcement_WhenDisabledAndNoCodexAgents(t *testing.T) {
	dir := t.TempDir()
	writeCodexEnforcementDisabledConfig(t, dir)
	seedStaleCodexEnforcement(t, dir)

	// No codex agents — only a claude agent so ScaffoldAll has something to do.
	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}
	_, err := ScaffoldAll(dir, agents, allTools, false)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh"),
		"stale enforcement script must be removed when codex enforcement is disabled")

	hooksData, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(hooksData), "enforce-cli-tools.sh",
		"stale kasmos PreToolUse entry must be removed when codex enforcement is disabled")

	// .codex/AGENTS.md must NOT be created — no codex agents were scaffolded.
	assert.NoFileExists(t, filepath.Join(dir, ".codex", "AGENTS.md"),
		"cleanup-only path must not create new codex scaffold files")
}

// TestSyncScaffold_RemovesStaleCodexEnforcement_WhenDisabledAndNoCodexAgents mirrors
// the ScaffoldAll regression for SyncScaffold's cleanup-only path.
func TestSyncScaffold_RemovesStaleCodexEnforcement_WhenDisabledAndNoCodexAgents(t *testing.T) {
	dir := t.TempDir()
	writeCodexEnforcementDisabledConfig(t, dir)
	seedStaleCodexEnforcement(t, dir)

	agents := []harness.AgentConfig{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}
	_, err := SyncScaffold(dir, agents)
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(dir, ".codex", "hooks", "enforce-cli-tools.sh"),
		"stale enforcement script must be removed when codex enforcement is disabled")

	hooksData, err := os.ReadFile(filepath.Join(dir, ".codex", "hooks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(hooksData), "enforce-cli-tools.sh",
		"stale kasmos PreToolUse entry must be removed when codex enforcement is disabled")

	assert.NoFileExists(t, filepath.Join(dir, ".codex", "AGENTS.md"),
		"cleanup-only path must not create new codex scaffold files")
}
