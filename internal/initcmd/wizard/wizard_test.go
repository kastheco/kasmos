package wizard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateToTOMLConfig(t *testing.T) {
	temp := "0.7"
	state := &State{
		Agents: []AgentState{
			{Role: "coder", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6",
				Temperature: temp, Effort: "high", Enabled: true},
			{Role: "reviewer", Harness: "claude", Model: "claude-opus-4-6",
				Temperature: "", Effort: "high", Enabled: true},
			{Role: "planner", Harness: "codex", Model: "gpt-5-codex",
				Temperature: "", Effort: "", Enabled: false},
			{Role: "chat", Harness: "opencode", Model: "anthropic/claude-sonnet-4-6",
				Temperature: "0.3", Effort: "high", Enabled: true},
		},
		PhaseMapping: map[string]string{
			"implementing":   "coder",
			"spec_review":    "reviewer",
			"quality_review": "reviewer",
			"planning":       "planner",
		},
	}

	tc := state.ToTOMLConfig()

	// Verify phases
	assert.Equal(t, "coder", tc.Phases["implementing"])
	assert.Equal(t, "reviewer", tc.Phases["spec_review"])

	// Verify agents
	coder, ok := tc.Agents["coder"]
	require.True(t, ok)
	assert.Equal(t, "opencode", coder.Program)
	assert.Equal(t, "anthropic/claude-sonnet-4-6", coder.Model)
	assert.NotNil(t, coder.Temperature)
	assert.InDelta(t, 0.7, *coder.Temperature, 0.001)
	assert.True(t, coder.Enabled)

	// Verify disabled agent
	planner := tc.Agents["planner"]
	assert.False(t, planner.Enabled)
	require.NotNil(t, tc.UI.AutoAdvanceWaves)
	require.NotNil(t, tc.UI.AutoReviewFix)
	assert.True(t, *tc.UI.AutoAdvanceWaves)
	assert.True(t, *tc.UI.AutoReviewFix)
	require.NotNil(t, tc.UI.AutoAdvance)
	assert.True(t, *tc.UI.AutoAdvance)

	// Verify nil temperature when empty
	reviewer := tc.Agents["reviewer"]
	assert.Nil(t, reviewer.Temperature)
}

func TestStateToAgentConfigs(t *testing.T) {
	t.Run("chat is fanned out to all selected harnesses", func(t *testing.T) {
		state := &State{
			SelectedHarness: []string{"opencode", "claude"},
			Agents: []AgentState{
				{Role: "coder", Harness: "opencode", Model: "model-1", Enabled: true},
				{Role: "reviewer", Harness: "claude", Model: "model-2", Enabled: true},
				{Role: "planner", Harness: "codex", Model: "model-3", Enabled: false},
				{Role: "chat", Harness: "opencode", Model: "model-4", Enabled: true},
			},
		}

		configs := state.ToAgentConfigs()

		// coder + reviewer + chat×2 (one per harness); planner disabled
		assert.Len(t, configs, 4)
		assert.Equal(t, "coder", configs[0].Role)
		assert.Equal(t, "reviewer", configs[1].Role)
		// chat fanned out: opencode then claude
		assert.Equal(t, "chat", configs[2].Role)
		assert.Equal(t, "opencode", configs[2].Harness)
		assert.Equal(t, "chat", configs[3].Role)
		assert.Equal(t, "claude", configs[3].Harness)
	})

	t.Run("chat with single harness emits one entry", func(t *testing.T) {
		state := &State{
			SelectedHarness: []string{"opencode"},
			Agents: []AgentState{
				{Role: "coder", Harness: "opencode", Model: "model-1", Enabled: true},
				{Role: "chat", Harness: "opencode", Model: "model-4", Enabled: true},
			},
		}

		configs := state.ToAgentConfigs()
		assert.Len(t, configs, 2)
		assert.Equal(t, "chat", configs[1].Role)
		assert.Equal(t, "opencode", configs[1].Harness)
	})
}

func TestDefaultAgentRoles(t *testing.T) {
	roles := DefaultAgentRoles()
	assert.Equal(t, []string{"coder", "architect", "reviewer", "planner", "chat", "fixer", "master"}, roles)
}

func TestRoleDefaults(t *testing.T) {
	defaults := RoleDefaults()

	t.Run("has all four roles", func(t *testing.T) {
		assert.Contains(t, defaults, "coder")
		assert.Contains(t, defaults, "reviewer")
		assert.Contains(t, defaults, "planner")
		assert.Contains(t, defaults, "chat")
		assert.Contains(t, defaults, "fixer")
		assert.Contains(t, defaults, "master")
	})

	t.Run("coder defaults", func(t *testing.T) {
		c := defaults["coder"]
		assert.Equal(t, "claude", c.Harness)
		assert.Equal(t, "claude-sonnet-4-6", c.Model)
		assert.Equal(t, "medium", c.Effort)
		assert.Equal(t, "0.1", c.Temperature)
		assert.True(t, c.Enabled)
	})

	t.Run("architect defaults", func(t *testing.T) {
		e := defaults["architect"]
		assert.Equal(t, "opencode", e.Harness)
		assert.Equal(t, "openai/gpt-5.4", e.Model)
		assert.Equal(t, "xhigh", e.Effort)
		assert.Equal(t, "0.2", e.Temperature)
		assert.True(t, e.Enabled)
	})

	t.Run("planner defaults", func(t *testing.T) {
		p := defaults["planner"]
		assert.Equal(t, "claude", p.Harness)
		assert.Equal(t, "claude-opus-4-6", p.Model)
		assert.Equal(t, "high", p.Effort)
		assert.Equal(t, "0.3", p.Temperature)
		assert.True(t, p.Enabled)
	})

	t.Run("reviewer defaults", func(t *testing.T) {
		r := defaults["reviewer"]
		assert.Equal(t, "claude", r.Harness)
		assert.Equal(t, "claude-sonnet-4-6", r.Model)
		assert.Equal(t, "medium", r.Effort)
		assert.Equal(t, "0.2", r.Temperature)
		assert.True(t, r.Enabled)
	})

	t.Run("chat defaults", func(t *testing.T) {
		ch := defaults["chat"]
		assert.Equal(t, "opencode", ch.Harness)
		assert.Equal(t, "openai/gpt-5.4", ch.Model)
		assert.Equal(t, "medium", ch.Effort)
		assert.Equal(t, "0.3", ch.Temperature)
		assert.True(t, ch.Enabled)
	})

	t.Run("fixer defaults", func(t *testing.T) {
		f := defaults["fixer"]
		assert.Equal(t, "claude", f.Harness)
		assert.Equal(t, "claude-opus-4-6", f.Model)
		assert.Equal(t, "high", f.Effort)
		assert.Equal(t, "0.1", f.Temperature)
		assert.True(t, f.Enabled)
	})

	t.Run("master defaults", func(t *testing.T) {
		m := defaults["master"]
		assert.Equal(t, "opencode", m.Harness)
		assert.Equal(t, "openai/gpt-5.4", m.Model)
		assert.Equal(t, "high", m.Effort)
		assert.Equal(t, "0.2", m.Temperature)
		assert.True(t, m.Enabled)
	})
}

func TestIsCustomized(t *testing.T) {
	t.Run("matches defaults returns false", func(t *testing.T) {
		// Coder prefers "claude"; with only "opencode" available the effective
		// default resolves to "opencode", so set harness to match.
		a := RoleDefaults()["coder"]
		a.Harness = "opencode"
		assert.False(t, IsCustomized(a, []string{"opencode"}))
	})

	t.Run("preferred harness available is not customized", func(t *testing.T) {
		// Coder prefers "claude"; with both harnesses available the effective
		// default stays "claude", so a stock coder is not customized.
		a := RoleDefaults()["coder"]
		assert.False(t, IsCustomized(a, []string{"claude", "opencode"}))
	})

	t.Run("different model returns true", func(t *testing.T) {
		a := RoleDefaults()["coder"]
		a.Model = "anthropic/claude-opus-4-6"
		assert.True(t, IsCustomized(a, []string{"claude"}))
	})

	t.Run("different harness returns true", func(t *testing.T) {
		// Coder prefers "claude"; effective default with ["claude"] is "claude".
		// Setting harness to "opencode" marks it as customized.
		a := RoleDefaults()["coder"]
		a.Harness = "opencode"
		assert.True(t, IsCustomized(a, []string{"claude"}))
	})

	t.Run("different effort returns true", func(t *testing.T) {
		a := RoleDefaults()["coder"]
		a.Effort = "high"
		assert.True(t, IsCustomized(a, []string{"claude"}))
	})

	t.Run("different temperature returns true", func(t *testing.T) {
		a := RoleDefaults()["coder"]
		a.Temperature = "0.5"
		assert.True(t, IsCustomized(a, []string{"claude"}))
	})

	t.Run("disabled returns true", func(t *testing.T) {
		a := RoleDefaults()["coder"]
		a.Enabled = false
		assert.True(t, IsCustomized(a, []string{"claude"}))
	})

	t.Run("unknown role returns false", func(t *testing.T) {
		a := AgentState{Role: "unknown", Enabled: true}
		assert.False(t, IsCustomized(a, []string{"opencode"}))
	})
}
