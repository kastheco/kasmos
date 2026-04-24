package app

import (
	"os"
	"testing"

	"github.com/kastheco/kasmos/session"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleKeyPress_ExclamationEntersFocusMode(t *testing.T) {
	t.Parallel()
	h := newTestHome()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title: "test-inst", Path: os.TempDir(), Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Running)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_ExclamationInDefaultStateStillFocusesTmux(t *testing.T) {
	t.Parallel()
	h := newTestHome()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "tmux-inst",
		Path:          os.TempDir(),
		Program:       "opencode",
		ExecutionMode: session.ExecutionModeTmux,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Running)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.False(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_ExclamationNoOpWithoutRunningInstance(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_ShiftITogglesInfoHeader(t *testing.T) {
	t.Parallel()
	h := newTestHome()
	wasShowing := h.tabbedWindow.IsShowingInfo()
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'I', Text: "I"})
	updated := model.(*home)

	// I toggles the compact info header, not the instance tab index.
	assert.Equal(t, !wasShowing, updated.tabbedWindow.IsShowingInfo())
	assert.Equal(t, stateDefault, updated.state)
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd(), "I should request a follow-up resize after toggling the header")
}
