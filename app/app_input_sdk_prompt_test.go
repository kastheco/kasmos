package app

import (
	"context"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/session"
	"github.com/kastheco/kasmos/session/tmux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingExecutionSession struct {
	sentKeys []string
}

func (r *recordingExecutionSession) Start(string) error     { return nil }
func (r *recordingExecutionSession) Restore() error         { return nil }
func (r *recordingExecutionSession) Close() error           { return nil }
func (r *recordingExecutionSession) DoesSessionExist() bool { return true }
func (r *recordingExecutionSession) SendKeys(keys string) error {
	r.sentKeys = append(r.sentKeys, keys)
	return nil
}
func (r *recordingExecutionSession) TapEnter() error                                    { return nil }
func (r *recordingExecutionSession) SendPermissionResponse(tmux.PermissionChoice) error { return nil }
func (r *recordingExecutionSession) CapturePaneContent() (string, error)                { return "", nil }
func (r *recordingExecutionSession) CapturePaneContentWithOptions(string, string) (string, error) {
	return "", nil
}
func (r *recordingExecutionSession) HasUpdated() (bool, bool) { return false, false }
func (r *recordingExecutionSession) HasUpdatedWithContent() (bool, bool, string, bool) {
	return false, false, "", false
}
func (r *recordingExecutionSession) GetPanePID() (int, error)                     { return 0, nil }
func (r *recordingExecutionSession) Attach() (chan struct{}, error)               { return nil, nil }
func (r *recordingExecutionSession) DetachSafely() error                          { return nil }
func (r *recordingExecutionSession) SetDetachedSize(int, int) error               { return nil }
func (r *recordingExecutionSession) GetSanitizedName() string                     { return "recording-session" }
func (r *recordingExecutionSession) SetAgentType(string)                          {}
func (r *recordingExecutionSession) SetInitialPrompt(string)                      {}
func (r *recordingExecutionSession) SetNoFlicker(bool)                            {}
func (r *recordingExecutionSession) SetTaskEnv(int, int, int)                     {}
func (r *recordingExecutionSession) SetProject(string)                            {}
func (r *recordingExecutionSession) SetSessionTitle(string)                       {}
func (r *recordingExecutionSession) SetTitleFunc(func(string, time.Time, string)) {}
func (r *recordingExecutionSession) SetSDKSpeedTier(string)                       {}

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
	h.previewTerminalInstance = inst.IdentityKey()
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

	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "", errClipboardImageNotFound
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

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

func TestHandleKeyPress_SDKFocusMode_CtrlVPrefersClipboardImageWhenAvailable(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "text fallback", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "/tmp/clipboard.png", nil
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

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
	assert.Empty(t, updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, []string{"/tmp/clipboard.png"}, updated.tabbedWindow.SDKComposerImages())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_CtrlShiftVPastesClipboardText(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "pasted text", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "", errClipboardImageNotFound
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

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

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'V', Mod: tea.ModCtrl | tea.ModShift})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Equal(t, "pasted text", updated.tabbedWindow.SDKComposerText())
	assert.NotNil(t, cmd)
}

func TestPasteContentLooksBinary_DetectsPNGSignature(t *testing.T) {
	assert.True(t, pasteContentLooksBinary("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	assert.False(t, pasteContentLooksBinary("plain pasted text"))
}

func TestHandleKeyPress_SDKFocusMode_CtrlShiftVPrefersClipboardImage(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "text fallback", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "/tmp/clipboard.png", nil
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

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

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'V', Mod: tea.ModCtrl | tea.ModShift})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Empty(t, updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, []string{"/tmp/clipboard.png"}, updated.tabbedWindow.SDKComposerImages())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_CtrlVWithRawPNGPrefersClipboardImage(t *testing.T) {
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "/tmp/clipboard.png", nil
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

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
	assert.Empty(t, updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, []string{"/tmp/clipboard.png"}, updated.tabbedWindow.SDKComposerImages())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_CtrlCClearsComposer(t *testing.T) {
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
	h.tabbedWindow.AppendSDKComposerText("draft prompt")
	h.tabbedWindow.AppendSDKComposerImage("/tmp/clipboard.png")

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.Empty(t, updated.tabbedWindow.SDKComposerImages())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_EscapeSendsStopRequest(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-inline",
		Path:          t.TempDir(),
		Program:       "codex",
		ExecutionMode: session.ExecutionModeSDK,
	})
	require.NoError(t, err)
	exec := &recordingExecutionSession{}
	inst.SetExecutionSessionForTest(exec)
	inst.MarkStartedForTest()
	inst.SetStatus(session.Ready)
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.True(t, updated.tabbedWindow.IsFocusMode())
	assert.Equal(t, []string{"\x03"}, exec.sentKeys)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_EscapeKeepsPlaceholderFocused(t *testing.T) {
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
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.True(t, updated.tabbedWindow.IsFocusMode())
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_TmuxCodexFocusMode_ShiftEnterUsesLiteralTmuxInjection(t *testing.T) {
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:   "codex-tmux",
		Path:    t.TempDir(),
		Program: "codex",
	})
	require.NoError(t, err)
	exec := &recordingExecutionSession{}
	inst.SetExecutionSessionForTest(exec)
	inst.MarkStartedForTest()
	h.nav.AddInstance(inst)
	h.nav.SelectInstance(inst)
	h.state = stateFocusAgent
	h.tabbedWindow.SetFocusMode(true)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Nil(t, cmd)
	require.Equal(t, []string{string(kittyCSIu(13, tea.ModShift))}, exec.sentKeys)
	assert.Empty(t, updated.previewTerminal.SentKeys(), "shift+enter must bypass the attached tmux client path")
}
