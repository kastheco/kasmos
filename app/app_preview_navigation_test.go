package app

import (
	"testing"

	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/require"
)

func TestInstanceChanged_AttachedTmuxSelectionClearsSDKPreview(t *testing.T) {
	// Keyboard navigation and nav-row mouse clicks both route through instanceChanged.
	h := newTestHome()
	h.tabbedWindow.SetSize(100, 30)

	sdkInst := &session.Instance{
		Title:            "sdk-agent",
		Path:             t.TempDir(),
		Status:           session.Running,
		ExecutionMode:    session.ExecutionModeSDK,
		CachedContent:    "sdk-only-marker",
		CachedContentSet: true,
	}
	sdkInst.MarkStartedForTest()
	tmuxInst := &session.Instance{
		Title:         "claude-agent",
		Path:          t.TempDir(),
		Status:        session.Running,
		ExecutionMode: session.ExecutionModeTmux,
	}
	tmuxInst.MarkStartedForTest()

	h.nav.AddInstance(sdkInst)()
	h.nav.AddInstance(tmuxInst)()
	require.True(t, h.nav.SelectInstance(sdkInst))
	h.tabbedWindow.SetInstance(sdkInst)
	require.NoError(t, h.tabbedWindow.UpdatePreview(sdkInst))
	require.Contains(t, h.tabbedWindow.String(), "sdk-only-marker")

	require.True(t, h.nav.SelectInstance(tmuxInst))
	h.previewRequested = true
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = tmuxInst.IdentityKey()
	t.Cleanup(h.previewTerminal.Close)

	require.NotNil(t, h.instanceChanged())
	require.NotContains(t, h.tabbedWindow.String(), "sdk-only-marker")
}

func TestInstanceChanged_StoppedTmuxSelectionShowsCachedPreview(t *testing.T) {
	h := newTestHome()
	h.tabbedWindow.SetSize(100, 30)

	inst := &session.Instance{
		Title:            "stopped-agent",
		Path:             t.TempDir(),
		Status:           session.Ready,
		ExecutionMode:    session.ExecutionModeTmux,
		CachedContent:    "last visible output",
		CachedContentSet: true,
	}
	inst.MarkStartedDeadForTest()

	h.nav.AddInstance(inst)()
	require.True(t, h.nav.SelectInstance(inst))
	h.previewRequested = true

	require.Nil(t, h.instanceChanged())
	rendered := h.tabbedWindow.String()
	require.Contains(t, rendered, "last visible output")
	require.NotContains(t, rendered, "connecting")
}
