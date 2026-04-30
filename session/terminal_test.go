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

func TestEmbeddedTerminal_SuppressesBlankRenders(t *testing.T) {
	// Truly-blank renders (whitespace-only after stripping ANSI) must not
	// reach the cache. They show up during tmux's initial-attach handshake
	// — alternate-screen + clear + repaint — and would briefly wipe the
	// capture-pane snapshot we pre-seeded into the emulator. The snapshot
	// is the authoritative current rendering; transient blanks during the
	// repaint gap should not override it.
	term := NewDummyTerminal()
	defer term.Close()
	term.Resize(80, 24)
	go term.renderLoop()

	_, err := term.emu.Write([]byte(ansi.EraseEntireDisplay))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	_, changed := term.Render()
	require.False(t, changed, "blank render must not reach the cache")

	_, err = term.emu.Write([]byte("agent output"))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed := term.Render()
	require.True(t, changed)
	require.Contains(t, content, "agent output")
}

func TestEmbeddedTerminal_BlankAfterOutputDoesNotWipeCache(t *testing.T) {
	// Once the cache holds real content, a subsequent blank render (e.g.
	// tmux's clear-screen during a re-attach handshake) must not propagate
	// — keeping the slightly-stale prior frame is preferable to flashing
	// blank. Real "the agent has gone genuinely empty" states are
	// vanishingly rare for our use case (every agent session has at least
	// a tmux status line); when they do happen, the slight staleness is
	// acceptable.
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

	_, changed = term.Render()
	require.False(t, changed, "blank render after output must not wipe the cache")

	_, err = term.emu.Write([]byte("new output"))
	require.NoError(t, err)
	term.dataReady <- struct{}{}
	term.WaitForRender(20 * time.Millisecond)

	content, changed = term.Render()
	require.True(t, changed)
	require.Contains(t, content, "new output")
}
