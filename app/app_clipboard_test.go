package app

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/require"
)

func TestPreviewTick_BridgesEmbeddedClipboardRequest(t *testing.T) {
	h := newTestHome()
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminal.EnqueueClipboardRequest(ansi.PrimaryClipboard)

	model, cmd := h.Update(previewTickMsg{})
	updated := model.(*home)

	require.True(t, updated.previewClipboardPending)
	require.Equal(t, byte(ansi.PrimaryClipboard), updated.previewClipboardTarget)
	require.NotNil(t, cmd)
}

func TestClipboardMsg_ForwardsResponseToEmbeddedTerminal(t *testing.T) {
	h := newTestHome()
	term := session.NewDummyTerminal()
	h.previewTerminal = term
	h.previewClipboardPending = true
	h.previewClipboardTarget = ansi.SystemClipboard

	model, cmd := h.Update(tea.ClipboardMsg{Content: "image-bytes"})
	updated := model.(*home)

	require.False(t, updated.previewClipboardPending)
	require.Zero(t, updated.previewClipboardTarget)
	require.Nil(t, cmd)

	sent := term.SentKeys()
	require.Len(t, sent, 1)
	require.Equal(t, []byte(ansi.SetSystemClipboard("image-bytes")), sent[0])
}

func TestPasteMsg_ForwardsBracketedPasteToEmbeddedTerminal(t *testing.T) {
	h := newTestHome()
	term := session.NewDummyTerminal()
	h.previewTerminal = term
	h.state = stateFocusAgent

	model, cmd := h.Update(tea.PasteMsg{Content: "hello"})
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)

	sent := term.SentKeys()
	require.Len(t, sent, 1)
	require.Equal(t, []byte("\x1b[200~hello\x1b[201~"), sent[0])
}

func TestPasteMsg_EmptyContentForwardsCtrlVToEmbeddedTerminal(t *testing.T) {
	h := newTestHome()
	term := session.NewDummyTerminal()
	h.previewTerminal = term
	h.state = stateFocusAgent

	model, cmd := h.Update(tea.PasteMsg{Content: ""})
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)

	sent := term.SentKeys()
	require.Len(t, sent, 1)
	require.Equal(t, []byte{0x16}, sent[0])
}

func TestPasteMsg_EmptyContentAttachesClipboardImageForSDKFocusMode(t *testing.T) {
	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "/tmp/clipboard.png", nil
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-agent",
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

	model, cmd := h.Update(tea.PasteMsg{Content: ""})
	updated := model.(*home)

	require.NotNil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)
	require.Equal(t, []string{"/tmp/clipboard.png"}, updated.tabbedWindow.SDKComposerImages())
}

func TestHandleKeyPress_TmuxFocusMode_CtrlVPastesClipboardText(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "hello", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	h := newTestHome()
	term := session.NewDummyTerminal()
	h.previewTerminal = term
	h.state = stateFocusAgent

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)

	sent := term.SentKeys()
	require.Len(t, sent, 1)
	require.Equal(t, []byte("\x1b[200~hello\x1b[201~"), sent[0])
}

func TestHandleKeyPress_TmuxFocusMode_CtrlShiftVPastesClipboardText(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "hello", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	h := newTestHome()
	term := session.NewDummyTerminal()
	h.previewTerminal = term
	h.state = stateFocusAgent

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'V', Mod: tea.ModCtrl | tea.ModShift})
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)

	sent := term.SentKeys()
	require.Len(t, sent, 1)
	require.Equal(t, []byte("\x1b[200~hello\x1b[201~"), sent[0])
}

func TestHandleKeyPress_TmuxFocusMode_CtrlVFallsBackToRawCtrlVWhenClipboardTextUnavailable(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	h := newTestHome()
	term := session.NewDummyTerminal()
	h.previewTerminal = term
	h.state = stateFocusAgent

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	updated := model.(*home)

	require.Nil(t, cmd)
	require.Equal(t, stateFocusAgent, updated.state)

	sent := term.SentKeys()
	require.Len(t, sent, 1)
	require.Equal(t, []byte{0x16}, sent[0])
}
