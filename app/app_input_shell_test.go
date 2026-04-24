package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kastheco/kasmos/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectShellCommandSubmittedMsgs(cmd tea.Cmd) (submitted []shellCommandSubmittedMsg) {
	for _, msg := range runCommandTree(cmd) {
		if shellMsg, ok := msg.(shellCommandSubmittedMsg); ok {
			submitted = append(submitted, shellMsg)
		}
	}
	return submitted
}

func collectShellCommandSubmittedMsgsFromSlice(msgs []tea.Msg) (submitted []shellCommandSubmittedMsg) {
	for _, msg := range msgs {
		if shellMsg, ok := msg.(shellCommandSubmittedMsg); ok {
			submitted = append(submitted, shellMsg)
		}
	}
	return submitted
}

func runCommandTree(cmd tea.Cmd) (msgs []tea.Msg) {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	switch msg := msg.(type) {
	case tea.BatchMsg:
		for _, sub := range msg {
			msgs = append(msgs, runCommandTree(sub)...)
		}
	case shellCommandSubmittedMsg:
		msgs = append(msgs, msg)
	case promptSubmittedMsg:
		msgs = append(msgs, msg)
	default:
		if msg != nil {
			msgs = append(msgs, msg)
		}
	}
	return msgs
}

func collectPromptSubmittedMsgs(cmd tea.Cmd) (submitted []promptSubmittedMsg) {
	for _, msg := range runCommandTree(cmd) {
		if promptMsg, ok := msg.(promptSubmittedMsg); ok {
			submitted = append(submitted, promptMsg)
		}
	}
	return submitted
}

func collectPromptSubmittedMsgsFromSlice(msgs []tea.Msg) (submitted []promptSubmittedMsg) {
	for _, msg := range msgs {
		if promptMsg, ok := msg.(promptSubmittedMsg); ok {
			submitted = append(submitted, promptMsg)
		}
	}
	return submitted
}

func TestSDKFocus_ExclamationEntersShellMode(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.True(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.NotNil(t, cmd)
}

func TestSDKFocus_ExclamationWithTextIsLiteral(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	seedSDKText(h, "foo")

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: '!', Text: "!"})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.False(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.Equal(t, "foo!", updated.tabbedWindow.SDKComposerText())
	assert.NotNil(t, cmd)
}

func TestSDKFocus_BackspaceOnEmptyExitsShellMode(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	h.tabbedWindow.SetSDKComposerShellMode(true)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyBackspace})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.False(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.NotNil(t, cmd)
}

func TestSDKFocus_CtrlCClearsShellMode(t *testing.T) {
	t.Parallel()
	h, _ := newSDKFocusHome(t)
	h.tabbedWindow.SetSDKComposerShellMode(true)
	seedSDKText(h, "git status")

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, stateFocusAgent, updated.state)
	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.False(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.NotNil(t, cmd)
}

func TestSDKFocus_EnterInShellModeRoutesShellSubmit(t *testing.T) {
	t.Parallel()
	h, inst := newSDKFocusHome(t)
	exec := &recordingExecutionSession{}
	inst.SetExecutionSessionForTest(exec)
	h.tabbedWindow.SetSDKComposerShellMode(true)
	seedSDKText(h, "echo")

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	require.NotNil(t, cmd)

	msgs := runCommandTree(cmd)
	shellMsgs := collectShellCommandSubmittedMsgsFromSlice(msgs)
	require.Len(t, shellMsgs, 1)
	require.NoError(t, shellMsgs[0].err)
	assert.Same(t, inst, shellMsgs[0].instance)
	assert.Equal(t, "shell ran: echo", shellMsgs[0].auditMsg)
	assert.False(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.Equal(t, "", updated.tabbedWindow.SDKComposerText())
	assert.Equal(t, []string{"echo"}, exec.shellCommands)
	assert.Empty(t, exec.sentKeys)
	assert.Zero(t, exec.tapEnters)
	assert.Empty(t, exec.localImagePrompts)
	assert.Empty(t, collectPromptSubmittedMsgsFromSlice(msgs))
}

func TestSDKFocus_EnterInShellModeRoutesDaemonPlaceholder(t *testing.T) {
	origClient := newDaemonActionClient
	t.Cleanup(func() { newDaemonActionClient = origClient })

	var (
		gotProject string
		gotTitle   string
		gotCommand string
		sendCalled bool
	)
	newDaemonActionClient = func() daemonActionClient {
		return &stubDaemonActionClient{
			sendPromptFunc: func(project, title, prompt string) error {
				sendCalled = true
				return nil
			},
			runShellCommandFunc: func(project, title, command string) error {
				gotProject = project
				gotTitle = title
				gotCommand = command
				return nil
			},
		}
	}

	h := newTestHome()
	h.taskStoreProject = "proj"
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
	require.NoError(t, h.tabbedWindow.UpdatePreview(inst))
	h.tabbedWindow.SetSDKComposerShellMode(true)
	seedSDKText(h, "ls")

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
	updated := model.(*home)
	require.NotNil(t, cmd)

	msgs := runCommandTree(cmd)
	shellMsgs := collectShellCommandSubmittedMsgsFromSlice(msgs)
	require.Len(t, shellMsgs, 1)
	require.NoError(t, shellMsgs[0].err)
	assert.Equal(t, "proj", gotProject)
	assert.Equal(t, "sdk-placeholder", gotTitle)
	assert.Equal(t, "ls", gotCommand)
	assert.False(t, sendCalled)
	assert.False(t, updated.tabbedWindow.SDKComposerShellMode())
	assert.Empty(t, collectPromptSubmittedMsgsFromSlice(msgs))
}
