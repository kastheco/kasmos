package tmux

import (
	"context"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/kastheco/kasmos/log"
	"golang.org/x/term"
)

var (
	stdinFD         = func() int { return int(os.Stdin.Fd()) }
	terminalIsTTY   = term.IsTerminal
	terminalMakeRaw = term.MakeRaw
	terminalRestore = term.Restore
)

// detachWaitTimeout is the maximum time to wait for attach goroutines to exit
// during Detach. Exposed as a var so tests can shorten it without real-time delays.
var detachWaitTimeout = 500 * time.Millisecond

// Attach connects the calling terminal to the tmux session.
// It disables mouse on the enclosing outer tmux session (if kasmos is running
// inside tmux), then spawns two goroutines:
//  1. PTY output → os.Stdout (io.Copy)
//  2. os.Stdin → PTY, with Ctrl+Q (0x11) and Ctrl+Space (0x00) as detach keys
//
// Window-size monitoring is started via monitorWindowSize.
//
// Returns a channel that is closed when Detach completes.
func (t *TmuxSession) Attach() (chan struct{}, error) {
	// Detect and disable outer tmux mouse so the inner session gets raw events.
	outer := outerTmuxSession()
	t.outerMouseWasEnabled = outerMouseEnabled(outer)
	if t.outerMouseWasEnabled && outer != "" {
		_ = exec.Command("tmux", "set-option", "-t", outer, "mouse", "off").Run()
	}
	// Enable mouse on the inner session so the attached program (e.g. claude code)
	// can handle scroll events. Start() disables mouse for the kasmos preview
	// viewport, but during interactive attach the inner program needs it.
	_ = exec.Command("tmux", "set-option", "-t", t.sanitizedName, "mouse", "on").Run()
	if err := t.enterRawInputMode(); err != nil {
		t.restoreOuterMouse()
		return nil, err
	}

	ch := make(chan struct{})
	t.attachCh = ch

	ctx, cancel := context.WithCancel(context.Background())
	t.ctx = ctx
	t.cancel = cancel
	wg := &sync.WaitGroup{}
	t.wg = wg

	// Capture ptmx locally so goroutines don't race with Detach setting it to nil.
	ptmx := t.ptmx

	// Goroutine 1: stream PTY output to os.Stdout.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(os.Stdout, ptmx)
	}()

	// Goroutine 2: read stdin, write to PTY; detach on Ctrl+Q or Ctrl+Space.
	wg.Add(1)
	go func() {
		defer wg.Done()

		buf := make([]byte, 4096)

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Short read deadline allows the context-check loop to run.
			_ = os.Stdin.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err := os.Stdin.Read(buf)
			_ = os.Stdin.SetReadDeadline(time.Time{})
			if err != nil {
				// Timeout or EOF — re-check context on next iteration.
				continue
			}

			for _, b := range buf[:n] {
				if b == 0x11 || b == 0x00 { // Ctrl+Q or Ctrl+Space
					t.Detach()
					return
				}
			}
			_, _ = ptmx.Write(buf[:n])
		}
	}()

	// Start platform-specific window-size monitoring.
	t.monitorWindowSize()

	// Set initial window size to match the current terminal immediately,
	// since monitorWindowSize only responds to subsequent SIGWINCH signals.
	if sz, err := pty.GetsizeFull(os.Stdout); err == nil && sz.Cols > 0 && sz.Rows > 0 {
		_ = t.updateWindowSize(sz.Cols, sz.Rows)
	}

	return ch, nil
}

// Detach disconnects from the tmux session:
//  1. Cancels context and closes the PTY to unblock the attach goroutines.
//  2. Waits for goroutines with a 500 ms timeout.
//  3. Restores tmux mouse state (may write escape sequences to the terminal).
//  4. Drains any stale bytes from stdin — late-arriving terminal query
//     responses (notably DA1 replies like "\e[?62;22;52c") would otherwise
//     sit in the tty input buffer and leak to whatever process reads stdin
//     after kasmos relinquishes the terminal.
//  5. Restores stdin from raw mode.
//  6. Calls Restore() to create a background monitoring PTY.
//  7. Closes the attach channel (signals callers that detach is complete).
func (t *TmuxSession) Detach() {
	// Cancel context to signal the stdin goroutine to exit on its next loop.
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	// Close the PTY — this causes the io.Copy(os.Stdout, ptmx) goroutine to return.
	if t.ptmx != nil {
		_ = t.ptmx.Close()
		t.ptmx = nil
	}

	// Wait for goroutines with a timeout.
	// The stdin goroutine uses short read deadlines so it will exit promptly
	// once ctx is cancelled, but we don't want to block forever.
	if t.wg != nil {
		done := make(chan struct{})
		go func() {
			t.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(detachWaitTimeout):
			log.InfoLog.Printf("Detach: goroutines did not exit within timeout")
		}
		t.wg = nil
	}

	// Restore tmux mouse state before draining — tmux may write mouse-enable
	// sequences that cascade into terminal query responses we want to catch.
	t.restoreOuterMouse()
	// Disable mouse on the inner session so the kasmos preview viewport
	// handles scroll events instead of tmux entering copy-mode.
	_ = exec.Command("tmux", "set-option", "-t", t.sanitizedName, "mouse", "off").Run()

	// Drain pending stdin bytes while still in raw mode so reads are
	// byte-oriented and do not block on line buffering.
	drainStdin(100 * time.Millisecond)

	t.exitRawInputMode()

	// Recreate a background PTY for status monitoring (HasUpdated ticks).
	if err := t.Restore(); err != nil {
		log.ErrorLog.Printf("Detach: error restoring background PTY: %v", err)
	}

	// Signal any waiter that detach is complete.
	if t.attachCh != nil {
		close(t.attachCh)
		t.attachCh = nil
	}
}

// drainStdin reads and discards any bytes currently buffered on stdin, up to
// the given total budget. Overridable for tests.
var drainStdin = func(budget time.Duration) {
	fd := stdinFD()
	if !terminalIsTTY(fd) {
		return
	}
	defer func() { _ = os.Stdin.SetReadDeadline(time.Time{}) }()

	deadline := time.Now().Add(budget)
	buf := make([]byte, 256)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return
		}
		perRead := min(20*time.Millisecond, remaining)
		if err := os.Stdin.SetReadDeadline(time.Now().Add(perRead)); err != nil {
			return
		}
		n, err := os.Stdin.Read(buf)
		if n == 0 && err != nil {
			// No bytes within the window — input buffer is empty.
			return
		}
		// Discard what we read and keep draining.
	}
}

// DetachSafely is like Detach but safe to call when not attached (returns nil).
func (t *TmuxSession) DetachSafely() error {
	if t.attachCh == nil {
		return nil
	}
	t.Detach()
	return nil
}

// SetDetachedSize resizes the background PTY to the given terminal dimensions.
// Used to keep the tmux pane correctly sized while no interactive client is attached.
func (t *TmuxSession) SetDetachedSize(width, height int) error {
	return t.updateWindowSize(uint16(width), uint16(height))
}

// updateWindowSize calls pty.Setsize to resize the PTY file descriptor.
func (t *TmuxSession) updateWindowSize(cols, rows uint16) error {
	if t.ptmx == nil {
		return nil
	}
	return pty.Setsize(t.ptmx, &pty.Winsize{Cols: cols, Rows: rows})
}

// restoreOuterMouse re-enables mouse mode on the outer tmux session if Attach
// disabled it.
func (t *TmuxSession) restoreOuterMouse() {
	if !t.outerMouseWasEnabled {
		return
	}
	outer := outerTmuxSession()
	if outer == "" {
		return
	}
	_ = exec.Command("tmux", "set-option", "-t", outer, "mouse", "on").Run()
	t.outerMouseWasEnabled = false
}

func (t *TmuxSession) enterRawInputMode() error {
	fd := stdinFD()
	if !terminalIsTTY(fd) {
		t.stdinFD = 0
		t.rawInputState = nil
		return nil
	}
	state, err := terminalMakeRaw(fd)
	if err != nil {
		return err
	}
	t.stdinFD = fd
	t.rawInputState = state
	return nil
}

func (t *TmuxSession) exitRawInputMode() {
	if t.rawInputState == nil {
		t.stdinFD = 0
		return
	}
	_ = terminalRestore(t.stdinFD, t.rawInputState)
	t.rawInputState = nil
	t.stdinFD = 0
}
