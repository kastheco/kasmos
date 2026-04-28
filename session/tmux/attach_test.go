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
	var got bytes.Buffer
	withTerminalTestHarness(t, terminalTestHarness{
		stdinFD: 9,
		isTTY:   true,
		silence: func(io.Writer) {
			got.WriteString("silence\n")
		},
		outerRestore: func(io.Writer) {
			got.WriteString("restore\n")
		},
	})

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
	// Restore leaves ptmx nil (monitor-only), so SetDetachedSize uses resize-window.
	err := s.Restore()
	require.NoError(t, err)
	// SetDetachedSize should not error; the important thing is it doesn't panic.
	_ = s.SetDetachedSize(120, 40)
}

func TestSetDetachedSize_UsesResizeWindowWhenNoActivePTY(t *testing.T) {
	t.Parallel()
	var ranCmds []string
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, commandString(cmd))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return nil, nil },
	}
	s := NewTmuxSessionWithDeps("test-detached-size", "opencode", false, NewMockPtyFactory(t), cmdExec)
	// No active PTY (ptmx is nil) — should use tmux resize-window.
	require.Nil(t, s.ptmx)
	err := s.SetDetachedSize(120, 40)
	require.NoError(t, err)
	require.Len(t, ranCmds, 1)
	assert.Contains(t, ranCmds[0], "resize-window")
	assert.Contains(t, ranCmds[0], "-x 120")
	assert.Contains(t, ranCmds[0], "-y 40")
	assert.Contains(t, ranCmds[0], "kas_test-detached-size")
}

func TestSetDetachedSize_ZeroDimensionsAreNoOp(t *testing.T) {
	t.Parallel()
	var ranCmds []string
	cmdExec := cmd_test.MockCmdExec{
		RunFunc: func(cmd *exec.Cmd) error {
			ranCmds = append(ranCmds, commandString(cmd))
			return nil
		},
		OutputFunc: func(cmd *exec.Cmd) ([]byte, error) { return nil, nil },
	}
	s := NewTmuxSessionWithDeps("test-zero-size", "opencode", false, NewMockPtyFactory(t), cmdExec)
	// Zero dimensions should not issue resize-window.
	err := s.SetDetachedSize(0, 0)
	require.NoError(t, err)
	assert.Empty(t, ranCmds, "zero dimensions should not issue resize-window")
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
	var mu sync.Mutex
	var events []string
	withTerminalTestHarness(t, terminalTestHarness{
		stdinFD: 7,
		isTTY:   true,
		restore: func(int, *term.State) error {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, "restoreTTY")
			return nil
		},
		drain: func(time.Duration) {
			mu.Lock()
			defer mu.Unlock()
			events = append(events, "drainStdin")
		},
	})

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

func TestAttach_StdinFilterBuffersSplitControlSequences(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "bracketed paste markers",
			parts: []string{"a\x1b[20", "0~hello\x1b[", "201~b"},
			want:  "ahellob",
		},
		{
			name:  "focus report",
			parts: []string{"a\x1b", "[", "Ib"},
			want:  "ab",
		},
		{
			name:  "x10 mouse",
			parts: []string{"a\x1b[M", "  ", " b"},
			want:  "ab",
		},
		{
			name:  "sgr mouse",
			parts: []string{"a\x1b[<0", ";10;", "10Mb"},
			want:  "ab",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var pty bytes.Buffer
			filter := csiInputFilter{}

			for _, part := range tt.parts {
				require.NoError(t, filter.write(&pty, []byte(part)))
			}
			require.NoError(t, filter.flush(&pty))

			assert.Equal(t, tt.want, pty.String())
		})
	}
}

func TestAttach_OuterTerminalSilenceDoesNotToggleMouseModes(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer

	outerTerminalSilence(&out)

	assert.Contains(t, out.String(), cSIDisableBracketedPaste)
	assert.Contains(t, out.String(), cSIDisableFocusReporting)
	assert.NotContains(t, out.String(), "\x1b[?1003l")
	assert.NotContains(t, out.String(), "\x1b[?1006l")
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
