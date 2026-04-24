package tmux

import (
	"bytes"
	"io"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/kastheco/kasmos/cmd/cmd_test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"
)

func TestAttach_SilencesOuterTerminal(t *testing.T) {
	// serial: mutates tmux terminal globals
	withFastTmuxTimings(t)
	oldStdinFD := stdinFD
	oldIsTTY := terminalIsTTY
	oldMakeRaw := terminalMakeRaw
	oldRestore := terminalRestore
	oldSilence := outerTerminalSilence
	oldOuterRestore := outerTerminalRestore
	oldDrain := drainStdin
	defer func() {
		stdinFD = oldStdinFD
		terminalIsTTY = oldIsTTY
		terminalMakeRaw = oldMakeRaw
		terminalRestore = oldRestore
		outerTerminalSilence = oldSilence
		outerTerminalRestore = oldOuterRestore
		drainStdin = oldDrain
	}()

	stdinFD = func() int { return 9 }
	terminalIsTTY = func(int) bool { return true }
	state := &term.State{}
	terminalMakeRaw = func(int) (*term.State, error) { return state, nil }
	terminalRestore = func(int, *term.State) error { return nil }
	drainStdin = func(time.Duration) {}

	var got bytes.Buffer
	outerTerminalSilence = func(io.Writer) {
		got.WriteString("silence\n")
	}
	outerTerminalRestore = func(io.Writer) {
		got.WriteString("restore\n")
	}

	ptyFactory := NewMockPtyFactory(t)
	s := NewTmuxSessionWithDeps("test-attach-silence", "opencode", false,
		ptyFactory, cmd_test.NewMockExecutor())
	require.NoError(t, s.Restore())

	_, err := s.Attach()
	require.NoError(t, err)
	s.Detach()

	assert.Equal(t, "silence\nrestore\n", got.String())
}

func TestDetachSafely_WhenNotAttached(t *testing.T) {
	s := NewTmuxSessionWithDeps("test-detach", "opencode", false, NewMockPtyFactory(t), cmd_test.NewMockExecutor())
	// Not attached — should be a no-op
	err := s.DetachSafely()
	assert.NoError(t, err)
}

func TestSetDetachedSize(t *testing.T) {
	ptyFactory := NewMockPtyFactory(t)
	cmdExec := cmd_test.MockCmdExec{
		RunFunc:    func(cmd *exec.Cmd) error { return nil },
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return []byte(""), nil },
	}
	s := NewTmuxSessionWithDeps("test-size", "opencode", false, ptyFactory, cmdExec)
	// Restore to get a PTY
	err := s.Restore()
	require.NoError(t, err)
	// SetDetachedSize should not error with a valid PTY
	// May error on mock PTY (not a real terminal) — that's OK for unit test
	// The important thing is it doesn't panic
	_ = s.SetDetachedSize(120, 40)
}

func TestDetachSafely_IsIdempotent(t *testing.T) {
	s := NewTmuxSessionWithDeps("test-idem", "opencode", false, NewMockPtyFactory(t), cmd_test.NewMockExecutor())
	// Multiple calls when not attached should all return nil.
	assert.NoError(t, s.DetachSafely())
	assert.NoError(t, s.DetachSafely())
	assert.NoError(t, s.DetachSafely())
}

func TestUpdateWindowSize_NilPTY(t *testing.T) {
	s := NewTmuxSessionWithDeps("test-winsize", "opencode", false, NewMockPtyFactory(t), cmd_test.NewMockExecutor())
	// ptmx is nil — updateWindowSize should be a no-op.
	assert.Nil(t, s.ptmx)
	err := s.updateWindowSize(80, 24)
	assert.NoError(t, err)
}

// TestDetach_DrainsStdinBeforeRestoringTTY verifies that Detach drains stdin
// while stdin is still in raw mode, preventing late-arriving terminal query
// responses (e.g. DA1 replies like "\e[?62;22;52c") from leaking to whatever
// process reads stdin after kasmos relinquishes the terminal.
func TestDetach_DrainsStdinBeforeRestoringTTY(t *testing.T) {
	// serial: mutates tmux timing globals
	withFastTmuxTimings(t)
	oldStdinFD := stdinFD
	oldIsTTY := terminalIsTTY
	oldMakeRaw := terminalMakeRaw
	oldRestore := terminalRestore
	oldDrain := drainStdin
	oldOuterRestore := outerTerminalRestore
	defer func() {
		stdinFD = oldStdinFD
		terminalIsTTY = oldIsTTY
		terminalMakeRaw = oldMakeRaw
		terminalRestore = oldRestore
		drainStdin = oldDrain
		outerTerminalRestore = oldOuterRestore
	}()

	stdinFD = func() int { return 7 }
	terminalIsTTY = func(int) bool { return true }
	state := &term.State{}
	terminalMakeRaw = func(int) (*term.State, error) { return state, nil }

	var mu sync.Mutex
	var events []string
	terminalRestore = func(int, *term.State) error {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, "restoreTTY")
		return nil
	}
	drainStdin = func(budget time.Duration) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, "drainStdin")
	}
	outerTerminalRestore = func(io.Writer) {}

	s := NewTmuxSessionWithDeps("test-detach-drain", "opencode", false,
		NewMockPtyFactory(t), cmd_test.NewMockExecutor())
	require.NoError(t, s.enterRawInputMode())
	// Pretend Attach populated the cancel/wg/attachCh state so Detach walks
	// the full teardown path instead of early-returning.
	s.attachCh = make(chan struct{})
	s.cancel = func() {}
	s.wg = &sync.WaitGroup{}

	s.Detach()

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"drainStdin", "restoreTTY"}, events,
		"stdin must be drained while still in raw mode, before terminalRestore runs")
}

func TestAttach_StdinFilterDropsBracketedPasteMarkers(t *testing.T) {
	var pty bytes.Buffer

	require.NoError(t, filteredWrite(&pty, []byte("\x1b[200~hello\x1b[201~")))

	assert.Equal(t, "hello", pty.String())
}

func TestAttach_StdinFilterPreservesEsc(t *testing.T) {
	var pty bytes.Buffer

	require.NoError(t, filteredWrite(&pty, []byte("\x1bq")))

	assert.Equal(t, "\x1bq", pty.String())
}

func TestAttach_StdinFilterDropsFocusReport(t *testing.T) {
	var pty bytes.Buffer

	require.NoError(t, filteredWrite(&pty, []byte("a\x1b[I\x1b[Ob")))

	assert.Equal(t, "ab", pty.String())
}

func TestAttach_StdinFilterDropsMouseTracking(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "x10", in: "a\x1b[M   b"},
		{name: "sgr", in: "a\x1b[<0;10;10Mb"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var pty bytes.Buffer

			require.NoError(t, filteredWrite(&pty, []byte(tt.in)))

			assert.Equal(t, "ab", pty.String())
		})
	}
}

func TestAttach_StdinFilterDropsDA1Reply(t *testing.T) {
	var pty bytes.Buffer

	require.NoError(t, filteredWrite(&pty, []byte("a\x1b[?62;22;52cb")))

	assert.Equal(t, "ab", pty.String())
}

func TestRawInputMode_RestoresTTYState(t *testing.T) {
	oldStdinFD := stdinFD
	oldIsTTY := terminalIsTTY
	oldMakeRaw := terminalMakeRaw
	oldRestore := terminalRestore
	defer func() {
		stdinFD = oldStdinFD
		terminalIsTTY = oldIsTTY
		terminalMakeRaw = oldMakeRaw
		terminalRestore = oldRestore
	}()

	stdinFD = func() int { return 42 }
	terminalIsTTY = func(fd int) bool {
		require.Equal(t, 42, fd)
		return true
	}

	state := &term.State{}
	madeRaw := false
	restored := false
	terminalMakeRaw = func(fd int) (*term.State, error) {
		madeRaw = true
		require.Equal(t, 42, fd)
		return state, nil
	}
	terminalRestore = func(fd int, got *term.State) error {
		restored = true
		require.Equal(t, 42, fd)
		require.Same(t, state, got)
		return nil
	}

	s := &TmuxSession{}
	require.NoError(t, s.enterRawInputMode())
	require.True(t, madeRaw)
	require.Same(t, state, s.rawInputState)
	require.Equal(t, 42, s.stdinFD)

	s.exitRawInputMode()
	require.True(t, restored)
	require.Nil(t, s.rawInputState)
	require.Zero(t, s.stdinFD)
}
