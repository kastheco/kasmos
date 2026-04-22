package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleKeyPress_PageDownMovesSidebarSelection(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	h.nav.SetSize(80, 16)
	plans := []ui.PlanDisplay{
		{Filename: "g"},
		{Filename: "f"},
		{Filename: "e"},
		{Filename: "d"},
		{Filename: "c"},
		{Filename: "b"},
		{Filename: "a"},
	}
	h.nav.SetData(plans, nil, nil, nil, nil)

	updatedModel, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyPgDown})
	updated := updatedModel.(*home)

	assert.Greater(t, updated.nav.GetSelectedIdx(), 0)
	assert.Equal(t, slotNav, updated.focusSlot)
}

func TestHandleKeyPress_PageDownIgnoredInFocusMode(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	h.state = stateFocusAgent
	h.previewTerminal = session.NewDummyTerminal()
	h.nav.SetData([]ui.PlanDisplay{{Filename: "b"}, {Filename: "a"}}, nil, nil, nil, nil)
	require.True(t, h.nav.SelectByID(ui.SidebarPlanPrefix+"b"))

	updatedModel, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyPgDown})
	updated := updatedModel.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Equal(t, ui.SidebarPlanPrefix+"b", updated.nav.GetSelectedID())
}

func TestHandleKeyPress_SearchPageDownMovesSidebarSelection(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	h.state = stateSearch
	h.nav.SetSize(80, 16)
	h.nav.ActivateSearch()
	plans := []ui.PlanDisplay{
		{Filename: "g"},
		{Filename: "f"},
		{Filename: "e"},
		{Filename: "d"},
		{Filename: "c"},
		{Filename: "b"},
		{Filename: "a"},
	}
	h.nav.SetData(plans, nil, nil, nil, nil)

	updatedModel, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyPgDown})
	updated := updatedModel.(*home)

	assert.Greater(t, updated.nav.GetSelectedIdx(), 0)
	assert.Equal(t, stateSearch, updated.state)
}
