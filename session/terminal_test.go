package session

import (
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedTerminal_CapturesOsc52ClipboardReadRequests(t *testing.T) {
	term := NewDummyTerminal()
	defer term.Close()

	_, err := term.emu.Write([]byte(ansi.RequestPrimaryClipboard))
	require.NoError(t, err)

	selection, ok := term.PollClipboardRequest()
	require.True(t, ok)
	require.Equal(t, byte(ansi.PrimaryClipboard), selection)
}

func TestEmbeddedTerminal_IgnoresInitialBlankRender(t *testing.T) {
	term := NewDummyTerminal()
	defer term.Close()
	term.Resize(80, 24)
	go term.renderLoop()

	_, err := term.emu.Write([]byte(ansi.EraseEntireDisplay))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed := term.Render()
	require.False(t, changed)
	require.Empty(t, content)

	_, err = term.emu.Write([]byte("agent output"))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed = term.Render()
	require.True(t, changed)
	require.Contains(t, content, "agent output")
}

func TestEmbeddedTerminal_IgnoresBlankRenderAfterOutput(t *testing.T) {
	term := NewDummyTerminal()
	defer term.Close()
	term.Resize(80, 24)
	go term.renderLoop()

	_, err := term.emu.Write([]byte("agent output"))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)
	content, changed := term.Render()
	require.True(t, changed)
	require.Contains(t, content, "agent output")

	_, err = term.emu.Write([]byte(ansi.EraseEntireDisplay))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed = term.Render()
	require.False(t, changed)
	require.Empty(t, content)

	_, err = term.emu.Write([]byte("new output"))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed = term.Render()
	require.True(t, changed)
	require.Contains(t, content, "new output")
}
