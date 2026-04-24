package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kastheco/kasmos/config"
	"github.com/kastheco/kasmos/internal/initcmd/harness"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(f float64) *float64 { return &f }

func TestBuildProgramCommand(t *testing.T) {
	registry := harness.NewRegistry()

	tests := []struct {
		name    string
		profile config.AgentProfile
		want    string
	}{
		{
			name:    "empty program returns empty",
			profile: config.AgentProfile{Program: ""},
			want:    "",
		},
		{
			name:    "whitespace-only program returns empty",
			profile: config.AgentProfile{Program: "  "},
			want:    "",
		},
		{
			name: "claude with model and effort",
			profile: config.AgentProfile{
				Program: "claude",
				Model:   "claude-opus-4-6",
				Effort:  "high",
				Flags:   []string{"--verbose"},
			},
			want: "claude --model claude-opus-4-6 --effort high --verbose",
		},
		{
			name: "claude with empty model and effort",
			profile: config.AgentProfile{
				Program: "claude",
			},
			want: "claude",
		},
		{
			name: "claude with only model",
			profile: config.AgentProfile{
				Program: "claude",
				Model:   "claude-sonnet-4-6",
			},
			want: "claude --model claude-sonnet-4-6",
		},
		{
			name: "opencode with model and effort — only extra flags survive",
			profile: config.AgentProfile{
				Program: "opencode",
				Model:   "openai/gpt-5.4",
				Effort:  "xhigh",
				Flags:   []string{"--debug"},
			},
			want: "opencode --debug",
		},
		{
			name: "opencode with model and effort but no extra flags",
			profile: config.AgentProfile{
				Program: "opencode",
				Model:   "openai/gpt-5.4",
				Effort:  "xhigh",
			},
			want: "opencode",
		},
		{
			name: "codex with model effort temperature and extra flags",
			profile: config.AgentProfile{
				Program:     "codex",
				Model:       "gpt-5-codex",
				Effort:      "high",
				Temperature: ptr(0.2),
				Flags:       []string{"--quiet"},
			},
			want: "codex -m gpt-5-codex -c model_reasoning_effort=high -c temperature=0.2 --quiet",
		},
		{
			name: "codex with empty model and effort",
			profile: config.AgentProfile{
				Program: "codex",
			},
			want: "codex",
		},
		{
			name: "unknown program falls back to BuildCommand",
			profile: config.AgentProfile{
				Program: "aider",
				Model:   "gpt-4",
				Effort:  "high",
				Flags:   []string{"--watch"},
			},
			want: "aider --watch",
		},
		{
			name: "inline program with spaces falls back to BuildCommand",
			profile: config.AgentProfile{
				Program: "claude --permission-mode bypassPermissions",
				Model:   "claude-opus-4-6",
				Effort:  "high",
				Flags:   []string{"--verbose"},
			},
			want: "claude --permission-mode bypassPermissions --verbose",
		},
		{
			name: "program with absolute path uses basename for lookup",
			profile: config.AgentProfile{
				Program: "/usr/local/bin/claude",
				Model:   "claude-sonnet-4-6",
				Effort:  "medium",
			},
			want: "/usr/local/bin/claude --model claude-sonnet-4-6 --effort medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildProgramCommand(tt.profile, registry)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestProgramForAgent(t *testing.T) {
	// Create a temp repo with .kasmos/config.toml
	repoDir := t.TempDir()
	kasmosDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

	configContent := `
[phases]
  elaborating = "architect"
  fixer = "fixer"
  implementing = "coder"
  planning = "planner"
  quality_review = "reviewer"

[agents]
  [agents.architect]
    enabled = true
    program = "opencode"
    model = "openai/gpt-5.4"
    temperature = 0.2
    effort = "xhigh"
  [agents.coder]
    enabled = true
    program = "claude"
    model = "claude-sonnet-4-6"
    temperature = 0.1
    effort = "medium"
  [agents.fixer]
    enabled = true
    program = "claude"
    model = "claude-opus-4-6"
    temperature = 0.1
    effort = "high"
  [agents.planner]
    enabled = true
    program = "claude"
    model = "claude-opus-4-6"
    temperature = 0.3
    effort = "high"
  [agents.reviewer]
    enabled = true
    program = "claude"
    model = "claude-sonnet-4-6"
    temperature = 0.2
    effort = "medium"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(kasmosDir, "config.toml"),
		[]byte(configContent),
		0o644,
	))

	registry := harness.NewRegistry()

	tests := []struct {
		name      string
		agentType string
		contains  []string
	}{
		{
			name:      "planner resolves claude with opus model and high effort",
			agentType: session.AgentTypePlanner,
			contains:  []string{"claude", "--model claude-opus-4-6", "--effort high"},
		},
		{
			name:      "coder resolves claude with sonnet model and medium effort",
			agentType: session.AgentTypeCoder,
			contains:  []string{"claude", "--model claude-sonnet-4-6", "--effort medium"},
		},
		{
			name:      "reviewer resolves claude with sonnet model and medium effort",
			agentType: session.AgentTypeReviewer,
			contains:  []string{"claude", "--model claude-sonnet-4-6", "--effort medium"},
		},
		{
			name:      "fixer resolves claude with opus model and high effort",
			agentType: session.AgentTypeFixer,
			contains:  []string{"claude", "--model claude-opus-4-6", "--effort high"},
		},
		{
			name:      "elaborator resolves opencode without model/effort flags",
			agentType: session.AgentTypeElaborator,
			contains:  []string{"opencode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := programForAgentWithRegistry(repoDir, tt.agentType, registry)
			for _, substr := range tt.contains {
				assert.Contains(t, got, substr)
			}
			// OpenCode should NOT contain --model or --effort flags
			if tt.agentType == session.AgentTypeElaborator {
				assert.NotContains(t, got, "--model")
				assert.NotContains(t, got, "--effort")
			}
		})
	}

	t.Run("codex coder resolves effort via codex config override key", func(t *testing.T) {
		repoDir := t.TempDir()
		kasmosDir := filepath.Join(repoDir, ".kasmos")
		require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

		configContent := `
[phases]
  implementing = "coder"

[agents]
  [agents.coder]
    enabled = true
    program = "codex"
    model = "gpt-5.5"
    tier = "fast"
    effort = "low"
`
		require.NoError(t, os.WriteFile(
			filepath.Join(kasmosDir, "config.toml"),
			[]byte(configContent),
			0o644,
		))

		got := programForAgentWithRegistry(repoDir, session.AgentTypeCoder, registry)
		assert.Contains(t, got, "codex -m gpt-5.5")
		assert.Contains(t, got, "-c model_reasoning_effort=low")
		assert.NotContains(t, got, "reasoning.effort")
	})

	t.Run("missing config returns empty", func(t *testing.T) {
		got := programForAgentWithRegistry("/nonexistent/path", session.AgentTypeCoder, registry)
		assert.Equal(t, "", got)
	})
}

func TestExecutionModeForAgent(t *testing.T) {
	repoDir := t.TempDir()
	kasmosDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

	configContent := `
[phases]
  elaborating = "architect"
  implementing = "coder"
  planning = "planner"

[agents]
  [agents.architect]
    enabled = true
    program = "codex"
    execution_mode = "sdk"
  [agents.coder]
    enabled = true
    program = "claude"
  [agents.planner]
    enabled = true
    program = "claude"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(kasmosDir, "config.toml"),
		[]byte(configContent),
		0o644,
	))

	assert.Equal(t, config.ExecutionModeTmux, executionModeForAgent(repoDir, session.AgentTypeCoder))
	assert.Equal(t, config.ExecutionModeTmux, executionModeForAgent(repoDir, session.AgentTypePlanner))
	assert.Equal(t, config.ExecutionModeSDK, executionModeForAgent(repoDir, session.AgentTypeElaborator))
}

func TestSDKSpeedTierForAgent(t *testing.T) {
	repoDir := t.TempDir()
	kasmosDir := filepath.Join(repoDir, ".kasmos")
	require.NoError(t, os.MkdirAll(kasmosDir, 0o755))

	configContent := `
[phases]
  elaborating = "architect"
  implementing = "coder"

[agents]
  [agents.architect]
    enabled = true
    program = "codex"
    execution_mode = "sdk"
    tier = "default"
  [agents.coder]
    enabled = true
    program = "codex"
    execution_mode = "sdk"
    tier = "fast"
`
	require.NoError(t, os.WriteFile(
		filepath.Join(kasmosDir, "config.toml"),
		[]byte(configContent),
		0o644,
	))

	assert.Equal(t, "fast", sdkSpeedTierForAgent(repoDir, session.AgentTypeCoder))
	assert.Equal(t, "flex", sdkSpeedTierForAgent(repoDir, session.AgentTypeElaborator))
	assert.Empty(t, sdkSpeedTierForAgent(repoDir, session.AgentTypeReviewer))
}
