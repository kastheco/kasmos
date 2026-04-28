package tmux

import (
	"io"
	"testing"
	"time"

	"golang.org/x/term"
)

type terminalTestHarness struct {
	stdinFD      int
	isTTY        bool
	makeRaw      func(int) (*term.State, error)
	restore      func(int, *term.State) error
	drain        func(time.Duration)
	silence      func(io.Writer)
	outerRestore func(io.Writer)
}

func withTerminalTestHarness(t *testing.T, h terminalTestHarness) {
	t.Helper()
	withFastTmuxTimings(t)

	oldStdinFD := stdinFD
	oldIsTTY := terminalIsTTY
	oldMakeRaw := terminalMakeRaw
	oldRestore := terminalRestore
	oldDrain := drainStdin
	oldSilence := outerTerminalSilence
	oldOuterRestore := outerTerminalRestore
	t.Cleanup(func() {
		stdinFD = oldStdinFD
		terminalIsTTY = oldIsTTY
		terminalMakeRaw = oldMakeRaw
		terminalRestore = oldRestore
		drainStdin = oldDrain
		outerTerminalSilence = oldSilence
		outerTerminalRestore = oldOuterRestore
	})

	fd := h.stdinFD
	if fd == 0 {
		fd = 9
	}
	state := &term.State{}

	stdinFD = func() int { return fd }
	terminalIsTTY = func(int) bool { return h.isTTY }
	terminalMakeRaw = func(fd int) (*term.State, error) {
		if h.makeRaw != nil {
			return h.makeRaw(fd)
		}
		return state, nil
	}
	terminalRestore = func(fd int, state *term.State) error {
		if h.restore != nil {
			return h.restore(fd, state)
		}
		return nil
	}
	drainStdin = func(budget time.Duration) {
		if h.drain != nil {
			h.drain(budget)
		}
	}
	outerTerminalSilence = func(w io.Writer) {
		if h.silence != nil {
			h.silence(w)
		}
	}
	outerTerminalRestore = func(w io.Writer) {
		if h.outerRestore != nil {
			h.outerRestore(w)
		}
	}
}

func withNonInteractiveAttachUnitHarness(t *testing.T) {
	t.Helper()
	withTerminalTestHarness(t, terminalTestHarness{
		stdinFD: 9,
		isTTY:   false,
	})
}
