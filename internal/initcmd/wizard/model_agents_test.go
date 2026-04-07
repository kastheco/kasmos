package wizard

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentStep_BrowseNavigation(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "reviewer", Harness: "opencode", Model: "gpt-5-codex", Enabled: true},
		{Role: "planner", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
		{Role: "chat", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "fixer", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude", "opencode"}, nil)
	assert.Equal(t, 0, s.cursor)
	assert.Equal(t, agentBrowseMode, s.mode)

	s.cursorDown()
	assert.Equal(t, 1, s.cursor)

	s.cursorDown()
	assert.Equal(t, 2, s.cursor)

	// chat and fixer are now navigable
	s.cursorDown()
	assert.Equal(t, 3, s.cursor)

	s.cursorDown()
	assert.Equal(t, 4, s.cursor)

	s.cursorDown()               // clamped at last index
	assert.Equal(t, 4, s.cursor) // no overflow past fixer
}

func TestAgentStep_ToggleEnabled(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Enabled: true},
		{Role: "reviewer", Harness: "claude", Enabled: true},
		{Role: "planner", Harness: "claude", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude"}, nil)
	s.cursor = 0
	s.toggleEnabled()
	assert.False(t, s.agents[0].Enabled)
	s.toggleEnabled()
	assert.True(t, s.agents[0].Enabled)
}

func TestAgentStep_DetailPanelContent(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Effort: "medium", Temperature: "0.1", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude"}, nil)
	detail := s.renderDetailPanel(60, 20)
	assert.Contains(t, detail, "coder")
	assert.NotContains(t, detail, "CODER")
	assert.Contains(t, detail, "claude-sonnet-4-6")
	assert.Contains(t, detail, "medium")
}

func TestAgentStep_EnterEditMode(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}
	modelCache := map[string][]string{
		"claude": {"claude-sonnet-4-6", "claude-opus-4-6", "claude-sonnet-4-5", "claude-haiku-4-5"},
	}

	s := newAgentStep(agents, []string{"claude", "opencode"}, modelCache)
	s.enterEditMode()
	assert.Equal(t, agentEditMode, s.mode)
	assert.Equal(t, 0, s.editField)
}

func TestAgentStep_EditFieldCycle(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude", "opencode"}, nil)
	s.enterEditMode()

	s.nextField()
	assert.Equal(t, 1, s.editField)

	s.nextField()
	assert.Equal(t, 2, s.editField)

	s.nextField()
	assert.Equal(t, 0, s.editField)
}

func TestAgentStep_ExitEditMode(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude"}, nil)
	s.enterEditMode()
	s.exitEditMode()
	assert.Equal(t, agentBrowseMode, s.mode)
}

func TestAgentStep_HarnessCycle(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Enabled: true},
	}
	harnesses := []string{"claude", "opencode"}

	s := newAgentStep(agents, harnesses, nil)
	s.enterEditMode()
	s.editField = 0

	s.cycleFieldValue(1)
	assert.Equal(t, "opencode", s.agents[0].Harness)

	s.cycleFieldValue(1)
	assert.Equal(t, "claude", s.agents[0].Harness)

	s.cycleFieldValue(-1)
	assert.Equal(t, "opencode", s.agents[0].Harness)
}

func TestAgentStep_EffortCycle(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Effort: "medium", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude"}, nil)
	s.effortLevels = map[string][]string{"claude": {"", "low", "medium", "high", "max"}}
	s.enterEditMode()
	s.editField = 2

	s.cycleFieldValue(1)
	assert.Equal(t, "high", s.agents[0].Effort)
}

// agentByRole finds an agent by role name, failing the test if not found.
func agentByRole(t *testing.T, agents []AgentState, role string) AgentState {
	t.Helper()
	for _, a := range agents {
		if a.Role == role {
			return a
		}
	}
	t.Fatalf("agent with role %q not found", role)
	return AgentState{}
}

func TestAgentStepPrePopulatesFromExisting(t *testing.T) {
	temp := 0.5
	existing := &config.TOMLConfigResult{
		Profiles: map[string]config.AgentProfile{
			"coder": {
				Program:     "opencode",
				Model:       "anthropic/claude-sonnet-4-6",
				Temperature: &temp,
				Effort:      "high",
				Enabled:     true,
			},
		},
	}

	agents := initAgentsFromExisting([]string{"claude", "opencode"}, existing)

	coder := agentByRole(t, agents, "coder")
	assert.Equal(t, "opencode", coder.Harness)
	assert.Equal(t, "high", coder.Effort)
	assert.Equal(t, "0.5", coder.Temperature)

	// architect gets defaults when not present in existing config
	arch := agentByRole(t, agents, "architect")
	assert.Equal(t, "opencode", arch.Harness)
	assert.Equal(t, "openai/gpt-5.4", arch.Model)
	assert.Equal(t, "xhigh", arch.Effort)
	assert.Equal(t, "0.2", arch.Temperature)

	reviewer := agentByRole(t, agents, "reviewer")
	assert.Equal(t, "claude", reviewer.Harness)
	assert.Equal(t, "claude-sonnet-4-6", reviewer.Model)
	assert.Equal(t, "medium", reviewer.Effort)
	assert.Equal(t, "0.2", reviewer.Temperature)
}

func TestInitAgentsFromExisting_UsesPreferredHarnessWhenSelected(t *testing.T) {
	agents := initAgentsFromExisting([]string{"claude", "opencode"}, nil)

	// coder prefers "claude" — available, so it should be selected
	coder := agentByRole(t, agents, "coder")
	assert.Equal(t, "claude", coder.Harness)

	// architect prefers "opencode" — available, so it should be selected
	arch := agentByRole(t, agents, "architect")
	assert.Equal(t, "opencode", arch.Harness)
}

func TestInitAgentsFromExisting_FallsBackWhenPreferredHarnessUnavailable(t *testing.T) {
	// Only "opencode" is available; claude-preferring roles must fall back to it
	agents := initAgentsFromExisting([]string{"opencode"}, nil)

	coder := agentByRole(t, agents, "coder")
	assert.Equal(t, "opencode", coder.Harness, "coder should fall back to opencode when claude unavailable")

	reviewer := agentByRole(t, agents, "reviewer")
	assert.Equal(t, "opencode", reviewer.Harness, "reviewer should fall back to opencode when claude unavailable")

	// architect already prefers opencode, so no fallback needed
	arch := agentByRole(t, agents, "architect")
	assert.Equal(t, "opencode", arch.Harness)
}

func TestInitAgentsFromExisting_FallsBackToClaudeWhenOpenCodeUnavailable(t *testing.T) {
	// Only "claude" is available; opencode-preferring roles must fall back to it
	agents := initAgentsFromExisting([]string{"claude"}, nil)

	arch := agentByRole(t, agents, "architect")
	assert.Equal(t, "claude", arch.Harness, "architect should fall back to claude when opencode unavailable")

	chat := agentByRole(t, agents, "chat")
	assert.Equal(t, "claude", chat.Harness, "chat should fall back to claude when opencode unavailable")
}

func TestAgentStep_IgnoresLegacyElaboratorProfile(t *testing.T) {
	temp := 0.7
	existing := &config.TOMLConfigResult{
		Profiles: map[string]config.AgentProfile{
			"elaborator": {
				Program:     "opencode",
				Model:       "anthropic/claude-opus-4-6",
				Temperature: &temp,
				Effort:      "max",
				Enabled:     true,
			},
		},
	}

	agents := initAgentsFromExisting([]string{"opencode"}, existing)

	// Legacy elaborator profiles remain ignored rather than migrated.
	// The architect role is the canonical path now, so the wizard must keep its
	// own defaults instead of silently inheriting stale settings from the
	// deprecated compatibility-only profile name.
	// find the architect agent
	var arch AgentState
	for _, a := range agents {
		require.NotEqual(t, "elaborator", a.Role, "legacy elaborator role should not be scaffolded back into the wizard")
		if a.Role == "architect" {
			arch = a
			break
		}
	}
	require.Equal(t, "architect", arch.Role, "architect role must be present")
	assert.Equal(t, "opencode", arch.Harness, "architect should keep the default harness")
	assert.Equal(t, "openai/gpt-5.4", arch.Model)
	assert.Equal(t, "xhigh", arch.Effort)
	assert.Equal(t, "0.2", arch.Temperature)
	assert.True(t, arch.Enabled)
}

func TestAgentStep_HarnessCycleAutoSelectsFirstModel(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "opencode", Model: "openai/gpt-5.4", Enabled: true},
	}
	modelCache := map[string][]string{
		"opencode": {"openai/gpt-5.4", "openai/gpt-4o"},
		"claude":   {"claude-opus-4-6", "claude-sonnet-4-6"},
	}

	s := newAgentStep(agents, []string{"opencode", "claude"}, modelCache)
	s.enterEditMode()
	s.editField = 0

	s.cycleFieldValue(1) // opencode -> claude
	assert.Equal(t, "claude", s.agents[0].Harness)
	assert.Equal(t, "claude-opus-4-6", s.agents[0].Model, "model should auto-select first from new harness cache")
}

func TestAgentStep_HarnessCycleKeepsSharedModel(t *testing.T) {
	sharedModel := "shared/model-v1"
	agents := []AgentState{
		{Role: "coder", Harness: "opencode", Model: sharedModel, Enabled: true},
	}
	modelCache := map[string][]string{
		"opencode": {sharedModel, "openai/gpt-5.4"},
		"claude":   {"claude-opus-4-6", sharedModel},
	}

	s := newAgentStep(agents, []string{"opencode", "claude"}, modelCache)
	s.enterEditMode()
	s.editField = 0

	s.cycleFieldValue(1) // opencode -> claude
	assert.Equal(t, "claude", s.agents[0].Harness)
	assert.Equal(t, sharedModel, s.agents[0].Model, "model should be preserved when still valid in new harness cache")
}

func TestAgentStep_ModelFieldEnterSelectsAndAdvances(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
	}
	modelCache := map[string][]string{
		"claude": {"claude-sonnet-4-6", "claude-opus-4-6", "claude-haiku-4-5"},
	}

	s := newAgentStep(agents, []string{"claude"}, modelCache)
	s.enterEditMode()
	s.editField = 1
	s.moveModelCursor(2) // point at claude-haiku-4-5

	_, _ = s.updateEditMode(tea.KeyPressMsg{Code: tea.KeyEnter})
	assert.Equal(t, "claude-haiku-4-5", s.agents[0].Model, "enter should select the highlighted model")
	assert.Equal(t, 2, s.editField, "enter on model field should advance to next field (effort)")
}

func TestAgentStep_StaleEffortNormalizedOnHarnessSwitch(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "opencode", Model: "openai/gpt-5.4", Effort: "xhigh", Enabled: true},
	}
	modelCache := map[string][]string{
		"opencode": {"openai/gpt-5.4"},
		"claude":   {"claude-sonnet-4-6"},
	}

	s := newAgentStep(agents, []string{"opencode", "claude"}, modelCache)
	s.enterEditMode()
	s.editField = 0

	s.cycleFieldValue(1) // opencode -> claude
	assert.Equal(t, "claude", s.agents[0].Harness)
	assert.NotEqual(t, "xhigh", s.agents[0].Effort, "stale xhigh effort must be normalized when switching to claude")
}

func TestAgentStep_ViewSeparatorFillsPanelHeight(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "anthropic/claude-sonnet-4-6", Enabled: true},
		{Role: "reviewer", Harness: "opencode", Model: "openai/gpt-5-codex", Enabled: true},
		{Role: "planner", Harness: "claude", Model: "anthropic/claude-opus-4-6", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude", "opencode"}, nil)
	view := s.View(100, 20)
	assert.Equal(t, 20, strings.Count(view, "┊"))
}

func TestTruncateForCell(t *testing.T) {
	assert.Equal(t, "", truncateForCell("abc", 0))
	assert.Equal(t, "abc", truncateForCell("abc", 3))
	assert.Equal(t, "ab...", truncateForCell("abcdef", 5))
}

func TestAgentStep_QReturnsStepCancelMsg(t *testing.T) {
	s := newAgentStep([]AgentState{{Role: "coder", Harness: "claude", Enabled: true}}, []string{"claude"}, nil)
	next, cmd := s.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	require.NotNil(t, cmd)
	_, ok := next.(*agentStepModel)
	require.True(t, ok)
	msg := cmd()
	_, ok = msg.(stepCancelMsg)
	assert.True(t, ok)
}

func TestRenderTemperatureFieldHasNoSideEffects(t *testing.T) {
	s := newAgentStep([]AgentState{{Role: "coder", Harness: "opencode", Enabled: true}}, []string{"opencode"}, nil)
	s.enterEditMode()
	s.editField = 0
	s.syncTemperatureInput()
	assert.False(t, s.tempInput.Focused())
	_ = s.renderTemperatureField(true)
	assert.False(t, s.tempInput.Focused())
}
