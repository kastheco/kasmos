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

func TestEmbeddedTerminal_PropagatesBlankRenderAfterOutput(t *testing.T) {
	// After the terminal has emitted any printable output, a subsequent blank
	// render (e.g. the agent clears the screen / idles to an empty prompt) is
	// real state and must propagate to the cache. Suppressing it here was the
	// proximate cause of idle TUI/SDK previews going blank: the live emulator
	// state diverges from the cache, then the staleness detector tears the
	// terminal down and the now-empty cache is what the user sees.
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
	require.True(t, changed, "blank render after output must reach the cache")
	require.NotContains(t, content, "agent output")

	_, err = term.emu.Write([]byte("new output"))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed = term.Render()
	require.True(t, changed)
	require.Contains(t, content, "new output")
}
