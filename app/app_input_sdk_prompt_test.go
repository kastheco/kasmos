package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleKeyPress_SendPrompt_SDKEntersFocusMode(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-architect",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'i', Text: "i"})
	updated := model.(*home)

	assert.NotNil(t, cmd)
	assert.Equal(t, stateFocusAgent, updated.state)
	assert.True(t, updated.tabbedWindow.IsFocusMode())
	assert.False(t, updated.overlays.IsActive())
}

func TestHandleKeyPress_SendPrompt_TmuxStillEntersFocusMode(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "tmux-agent",
		Path:    t.TempDir(),
		Program: "claude",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title
	h.keySent = true

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'i', Text: "i"})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.True(t, updated.tabbedWindow.IsFocusMode())
	assert.NotNil(t, cmd)
	assert.False(t, updated.overlays.IsActive())
}

func TestEnterFocusMode_AllowsSDKPlaceholder(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-placeholder",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	inst.SetStatus(session.Ready)
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)

	cmd := h.enterFocusMode()
	assert.NotNil(t, cmd)
	assert.Equal(t, stateFocusAgent, h.state)
	assert.True(t, h.tabbedWindow.IsFocusMode())
}

func TestHandleKeyPress_SDKFocusMode_TypesInlineWithoutOverlay(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-inline",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'a', Text: "a"})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.False(t, updated.overlays.IsActive())
	assert.Equal(t, "a", updated.tabbedWindow.SDKComposerText())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_CtrlVPastesClipboardText(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "pasted text", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-inline",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Equal(t, "pasted text", updated.tabbedWindow.SDKComposerText())
	assert.NotNil(t, cmd)
}
