package wizard

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAgentStep_BrowseNavigation(t *testing.T) {
	agents := []AgentState{
		{Role: "coder", Harness: "claude", Model: "claude-sonnet-4-6", Enabled: true},
		{Role: "reviewer", Harness: "opencode", Model: "gpt-5.3-codex", Enabled: true},
		{Role: "planner", Harness: "claude", Model: "claude-opus-4-6", Enabled: true},
	}

	s := newAgentStep(agents, []string{"claude", "opencode"}, nil)
	assert.Equal(t, 0, s.cursor)
	assert.Equal(t, agentBrowseMode, s.mode)

	s.cursorDown()
	assert.Equal(t, 1, s.cursor)

	s.cursorDown()
	assert.Equal(t, 2, s.cursor)

	s.cursorDown()               // chat is skipped in navigation
	assert.Equal(t, 2, s.cursor) // clamped at planner
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
	assert.Contains(t, detail, "CODER")
	assert.Contains(t, detail, "claude-sonnet-4-6")
	assert.Contains(t, detail, "medium")
}
