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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	// serial: overrides readClipboardText and captureClipboardImage seams
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
	// serial: overrides readClipboardText and captureClipboardImage seams
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
	// serial: overrides readClipboardText and captureClipboardImage seams
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
	t.Parallel()
	assert.True(t, pasteContentLooksBinary("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	assert.False(t, pasteContentLooksBinary("plain pasted text"))
}

func TestHandleKeyPress_SDKFocusMode_CtrlShiftVPrefersClipboardImage(t *testing.T) {
	// serial: overrides readClipboardText and captureClipboardImage seams
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
	// serial: overrides readClipboardText and captureClipboardImage seams
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// newSDKFocusHome is a test helper that creates a home model in SDK focus mode
// with a started SDK instance already selected.
func newSDKFocusHome(t *testing.T) (*home, *session.Instance) {
	t.Helper()
	h := newTestHome()
	inst, err := session.NewInstance(session.InstanceOptions{
		Title:         "sdk-cursor",
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
	return h, inst
}

// seedSDKText types each character through handleKeyPress so that the preview
// state (including lastSDKComposerOwner) is properly initialised before cursor
// movement tests run. Direct AppendSDKComposerText calls would bypass
// UpdatePreview and cause the first mutateSDKComposer call to reset the text.
func seedSDKText(h *home, text string) {
	for _, r := range text {
		if r == '\n' {
			h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift}) //nolint:errcheck
		} else {
			h.handleKeyPress(tea.KeyPressMsg{Code: r, Text: string(r)}) //nolint:errcheck
		}
	}
}

func TestHandleKeyPress_SDKFocusMode_TypeInsertedAtCursor(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "hello world")

	// Move left 4 times: cursor should be at position 7 (before 'o' in 'world')
	for range 4 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	// Type 'X'
	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'X', Text: "X"})
	updated := model.(*home)

	assert.Equal(t, "hello wXorld", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, 8, updated.tabbedWindow.SDKComposerCursor())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_CtrlLeftWordMovement(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "one two three")
	// Cursor starts at end (13)

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl})
	assert.Equal(t, 8, model.(*home).tabbedWindow.SDKComposerCursor()) // before "three"

	model, _ = model.(*home).handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl})
	assert.Equal(t, 4, model.(*home).tabbedWindow.SDKComposerCursor()) // before "two"

	model, _ = model.(*home).handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl})
	assert.Equal(t, 0, model.(*home).tabbedWindow.SDKComposerCursor()) // before "one"
}

func TestHandleKeyPress_SDKFocusMode_CtrlRightWordMovement(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "one two three")
	// Move to beginning first
	for range 13 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl})
	assert.Equal(t, 3, model.(*home).tabbedWindow.SDKComposerCursor()) // after "one"

	model, _ = model.(*home).handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl})
	assert.Equal(t, 7, model.(*home).tabbedWindow.SDKComposerCursor()) // after "two"

	model, _ = model.(*home).handleKeyPress(tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl})
	assert.Equal(t, 13, model.(*home).tabbedWindow.SDKComposerCursor()) // after "three"
}

func TestHandleKeyPress_SDKFocusMode_BackspaceVsDelete(t *testing.T) {
	t.Parallel()

	t.Run("backspace removes rune before cursor", func(t *testing.T) {
		h, _ := newSDKFocusHome(t)
		seedSDKText(h, "hello world")
		// Move cursor to position 5 (after "hello")
		for range 6 {
			h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
		}

		model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyBackspace})
		updated := model.(*home)
		assert.Equal(t, "hell world", updated.tabbedWindow.SDKComposerText())
		assert.Equal(t, 4, updated.tabbedWindow.SDKComposerCursor())
	})

	t.Run("delete removes rune at cursor", func(t *testing.T) {
		h, _ := newSDKFocusHome(t)
		seedSDKText(h, "hello world")
		// Move cursor to position 5 (after "hello", before " world")
		for range 6 {
			h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
		}

		model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDelete})
		updated := model.(*home)
		assert.Equal(t, "helloworld", updated.tabbedWindow.SDKComposerText())
		assert.Equal(t, 5, updated.tabbedWindow.SDKComposerCursor())
	})
}

func TestHandleKeyPress_SDKFocusMode_CtrlBackspaceDeletesWordBackward(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "hello world")
	// cursor at end (11)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyBackspace, Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, "hello ", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, 6, updated.tabbedWindow.SDKComposerCursor())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_CtrlDeleteDeletesWordForward(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "hello world")
	// Move to position 6 (before "world")
	for range 5 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDelete, Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, "hello ", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, 6, updated.tabbedWindow.SDKComposerCursor())
	assert.NotNil(t, cmd)
}

func TestHandleKeyPress_SDKFocusMode_HomeAndEnd(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "line1\nline2\nline3")

	// Move cursor to position 9 (inside "line2")
	for range 8 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	t.Run("Home moves to line start", func(t *testing.T) {
		hh, _ := newSDKFocusHome(t)
		seedSDKText(hh, "line1\nline2\nline3")
		for range 8 {
			hh.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
		}
		model, _ := hh.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyHome})
		assert.Equal(t, 6, model.(*home).tabbedWindow.SDKComposerCursor()) // start of "line2"
	})

	t.Run("End moves to line end", func(t *testing.T) {
		hh, _ := newSDKFocusHome(t)
		seedSDKText(hh, "line1\nline2\nline3")
		for range 8 {
			hh.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
		}
		model, _ := hh.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnd})
		assert.Equal(t, 11, model.(*home).tabbedWindow.SDKComposerCursor()) // end of "line2" (before \n)
	})
}

func TestHandleKeyPress_SDKFocusMode_UpDownMovement(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "line1\nline2\nline3")
	// Cursor at end (17), move up

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyUp})
	cur := model.(*home).tabbedWindow.SDKComposerCursor()
	// Should be in line2, same column as line3 end (col 5) → pos 11
	assert.Equal(t, 11, cur)

	model, _ = model.(*home).handleKeyPress(tea.KeyPressMsg{Code: tea.KeyDown})
	cur = model.(*home).tabbedWindow.SDKComposerCursor()
	// Back to line3 end (17)
	assert.Equal(t, 17, cur)
}

func TestHandleKeyPress_SDKFocusMode_EnterSubmitResetsCursor(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "hello world")
	// Move cursor somewhere in the middle
	for range 5 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)

	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, 0, updated.tabbedWindow.SDKComposerCursor())
}

func TestHandleKeyPress_SDKFocusMode_CtrlCResetsCursor(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "hello world")
	// Move cursor to middle
	for range 5 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	model, _ := h.handleKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, 0, updated.tabbedWindow.SDKComposerCursor())
}

func TestHandleKeyPress_SDKFocusMode_PasteInsertsAtCursor(t *testing.T) {
	// serial: overrides readClipboardText and captureClipboardImage seams
	origRead := readClipboardText
	readClipboardText = func() (string, error) { return "XY", nil }
	t.Cleanup(func() { readClipboardText = origRead })

	origCapture := captureClipboardImage
	captureClipboardImage = func(_ context.Context) (string, error) {
		return "", errClipboardImageNotFound
	}
	t.Cleanup(func() { captureClipboardImage = origCapture })

	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "hello world")
	// Move cursor 5 positions left (before " world" → cursor at 6)
	for range 5 {
		h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyLeft}) //nolint:errcheck
	}

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	updated := model.(*home)

	// "hello world"[6] == 'w', so "XY" is inserted before 'w': "hello XYworld"
	assert.Equal(t, "hello XYworld", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, 8, updated.tabbedWindow.SDKComposerCursor())
	assert.NotNil(t, cmd)
}
