package app

import (
	"os"
	"testing"

	"github.com/kastheco/kasmos/session"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestShortcutInstance(t *testing.T, status session.Status) *session.Instance {
	t.Helper()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title: "test-inst", Path: os.TempDir(), Program: "opencode",
	})
	require.NoError(t, err)
	inst.MarkStartedForTest()
	inst.SetStatus(status)

	return inst
}

func TestHandleKeyPress_NumberShortcutPassthroughWhenPreviewActive(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "1", Code: '1'})
	updated := model.(*home)

	require.Len(t, updated.previewTerminal.SentKeys(), 1)
	assert.Equal(t, []byte("1"), updated.previewTerminal.SentKeys()[0])
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_NumberShortcutNoOpWithoutPreviewTerminal(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = nil

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "1", Code: '1'})
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_NumberShortcutNoOpWhenInstancePaused(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Paused)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.Title

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "2", Code: '2'})
	updated := model.(*home)

	assert.Empty(t, updated.previewTerminal.SentKeys())
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_NumberShortcutPassthroughDigits(t *testing.T) {
	tests := []struct {
		name  string
		digit string
		code  rune
	}{
		{name: "one", digit: "1", code: '1'},
		{name: "two", digit: "2", code: '2'},
		{name: "three", digit: "3", code: '3'},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHome()
			inst := newTestShortcutInstance(t, session.Running)

			h.nav.AddInstance(inst)()
			h.nav.SetSelectedInstance(0)
			h.previewTerminal = session.NewDummyTerminal()
			h.previewTerminalInstance = inst.Title

			model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: tt.digit, Code: tt.code})
			updated := model.(*home)

			require.Len(t, updated.previewTerminal.SentKeys(), 1)
			assert.Equal(t, []byte(tt.digit), updated.previewTerminal.SentKeys()[0])
			assert.Equal(t, stateDefault, updated.state)
			assert.Nil(t, cmd)
		})
	}
}
