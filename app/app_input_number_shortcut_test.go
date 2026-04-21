package app

import (
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/kastheco/kasmos/session"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestShortcutInstance(t *testing.T, status session.Status, started bool) *session.Instance {
	t.Helper()

	inst, err := session.NewInstance(session.InstanceOptions{
		Title: "test-inst", Path: os.TempDir(), Program: "opencode",
	})
	require.NoError(t, err)
	if started {
		inst.MarkStartedForTest()
	}
	inst.SetStatus(status)

	return inst
}

func setEmbeddedTerminalPTYForTest(t *testing.T, term *session.EmbeddedTerminal, ptmx *os.File) {
	t.Helper()

	field := reflect.ValueOf(term).Elem().FieldByName("ptmx")
	reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(reflect.ValueOf(ptmx))
}

func TestHandleKeyPress_NumberShortcutPassthroughWhenPreviewActive(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running, true)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "1", Code: '1'})
	updated := model.(*home)

	require.Len(t, updated.previewTerminal.SentKeys(), 1)
	assert.Equal(t, []byte("1"), updated.previewTerminal.SentKeys()[0])
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_NumberShortcutNoOpWithoutPreviewTerminal(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running, true)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = nil

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "1", Code: '1'})
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_NumberShortcutNoOpWithoutSelectedInstance(t *testing.T) {
	h := newTestHome()
	h.previewTerminal = session.NewDummyTerminal()

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "1", Code: '1'})
	updated := model.(*home)

	assert.Empty(t, updated.previewTerminal.SentKeys())
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
	assert.False(t, updated.toastManager.HasActiveToasts())
}

func TestHandleKeyPress_NumberShortcutNoOpWhenInstanceNotStarted(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running, false)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "2", Code: '2'})
	updated := model.(*home)

	assert.Empty(t, updated.previewTerminal.SentKeys())
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
	assert.False(t, updated.toastManager.HasActiveToasts())
}

func TestHandleKeyPress_NumberShortcutNoOpWhenInstancePaused(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Paused, true)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "2", Code: '2'})
	updated := model.(*home)

	assert.Empty(t, updated.previewTerminal.SentKeys())
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_NumberShortcutHandlesSendKeyError(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running, true)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()

	ptmx, err := os.CreateTemp(t.TempDir(), "closed-pty")
	require.NoError(t, err)
	require.NoError(t, ptmx.Close())
	setEmbeddedTerminalPTYForTest(t, h.previewTerminal, ptmx)

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: "3", Code: '3'})
	updated := model.(*home)

	assert.NotNil(t, cmd)
	assert.Equal(t, stateDefault, updated.state)
	assert.True(t, updated.toastManager.HasActiveToasts())
	assert.Empty(t, updated.previewTerminal.SentKeys())
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
			inst := newTestShortcutInstance(t, session.Running, true)

			h.nav.AddInstance(inst)()
			h.nav.SetSelectedInstance(0)
			h.previewTerminal = session.NewDummyTerminal()
			h.previewTerminalInstance = inst.IdentityKey()

			model, cmd := h.handleKeyPress(tea.KeyPressMsg{Text: tt.digit, Code: tt.code})
			updated := model.(*home)

			require.Len(t, updated.previewTerminal.SentKeys(), 1)
			assert.Equal(t, []byte(tt.digit), updated.previewTerminal.SentKeys()[0])
			assert.Equal(t, stateDefault, updated.state)
			assert.Nil(t, cmd)
		})
	}
}

func TestHandleKeyPress_CtrlOPassesThroughToPTY(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running, true)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = session.NewDummyTerminal()
	h.previewTerminalInstance = inst.IdentityKey()

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	updated := model.(*home)

	require.Len(t, updated.previewTerminal.SentKeys(), 1)
	assert.Equal(t, []byte{0x0F}, updated.previewTerminal.SentKeys()[0])
	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}

func TestHandleKeyPress_CtrlONoOpWithoutPreviewTerminal(t *testing.T) {
	h := newTestHome()
	inst := newTestShortcutInstance(t, session.Running, true)

	h.nav.AddInstance(inst)()
	h.nav.SetSelectedInstance(0)
	h.previewTerminal = nil

	model, cmd := h.handleKeyPress(tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl})
	updated := model.(*home)

	assert.Equal(t, stateDefault, updated.state)
	assert.Nil(t, cmd)
}
