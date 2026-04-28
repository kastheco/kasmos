package tmux

import (
	"context"
	"fmt"
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

const (
	cSIDisableBracketedPaste = "\x1b[?2004l"
	cSIEnableBracketedPaste  = "\x1b[?2004h"
	cSIDisableFocusReporting = "\x1b[?1004l"
	cSIEnableFocusReporting  = "\x1b[?1004h"
)

var outerTerminalSilence = func(w io.Writer) {
	_, _ = io.WriteString(w, cSIDisableBracketedPaste)
	_, _ = io.WriteString(w, cSIDisableFocusReporting)
}

var outerTerminalRestore = func(w io.Writer) {
	_, _ = io.WriteString(w, cSIEnableBracketedPaste)
	_, _ = io.WriteString(w, cSIEnableFocusReporting)
}

// detachWaitTimeout is the maximum time to wait for attach goroutines to exit
// during Detach. Exposed as a var so tests can shorten it without real-time delays.
var detachWaitTimeout = 500 * time.Millisecond

// Attach connects the calling terminal to the tmux session.
// It creates a fresh tmux attach-session PTY handle, disables mouse on the
// enclosing outer tmux session (if kasmos is running inside tmux), then
// spawns two goroutines:
//  1. PTY output → os.Stdout (io.Copy)
//  2. os.Stdin → PTY, with Ctrl+Q (0x11) and Ctrl+Space (0x00) as detach keys
//
// Window-size monitoring is started via monitorWindowSize.
//
// Returns a channel that is closed when Detach completes.
func (t *TmuxSession) Attach() (chan struct{}, error) {
	if t.attachCh != nil {
		return nil, fmt.Errorf("already attached to session %s", t.sanitizedName)
	}

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
	// Silence outer-terminal modes so their generated escape sequences do not
	// flow through raw stdin into the inner program's prompt buffer.
	outerTerminalSilence(os.Stdout)
	if err := t.enterRawInputMode(); err != nil {
		outerTerminalRestore(os.Stdout)
		t.restoreOuterMouse()
		return nil, err
	}

	// Create a fresh attach PTY handle for this active session.
	handle, err := t.ptyFactory.Start(exec.Command("tmux", "attach-session", "-t", t.sanitizedName))
	if err != nil {
		t.exitRawInputMode()
		outerTerminalRestore(os.Stdout)
		t.restoreOuterMouse()
		return nil, fmt.Errorf("attach-session: start PTY: %w", err)
	}
	if handle == nil {
		t.exitRawInputMode()
		outerTerminalRestore(os.Stdout)
		t.restoreOuterMouse()
		return nil, fmt.Errorf("attach-session: PTY factory returned nil handle")
	}
	f := handle.File()
	if f == nil {
		_ = handle.Close()
		t.exitRawInputMode()
		outerTerminalRestore(os.Stdout)
		t.restoreOuterMouse()
		return nil, fmt.Errorf("attach-session: PTY handle returned nil file")
	}
	t.ptmxHandle = handle
	t.ptmx = f

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
		filter := csiInputFilter{}

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
				_ = filter.flushBareEsc(ptmx)
				// Timeout or EOF — re-check context on next iteration.
				continue
			}

			for _, b := range buf[:n] {
				if b == 0x11 || b == 0x00 { // Ctrl+Q or Ctrl+Space
					t.Detach()
					return
				}
			}
			_ = filter.write(ptmx, buf[:n])
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
//  1. Cancels context and closes/reaps the active PTY handle to unblock goroutines.
//  2. Waits for goroutines with a timeout.
//  3. Restores tmux mouse state.
//  4. Drains any stale bytes from stdin.
//  5. Restores stdin from raw mode.
//  6. Closes the attach channel (signals callers that detach is complete).
//
// Unlike the previous implementation, Detach does NOT call Restore() or
// recreate a background PTY. Preview and status monitoring continue through
// capture-pane without an attached PTY client.
func (t *TmuxSession) Detach() {
	// Cancel context to signal the stdin goroutine to exit on its next loop.
	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	// Close and reap the active PTY handle — this unblocks io.Copy(os.Stdout, ptmx).
	if err := t.closeActivePty("detach"); err != nil {
		log.ErrorLog.Printf("Detach: %v", err)
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
	outerTerminalRestore(os.Stdout)

	// Signal any waiter that detach is complete.
	if t.attachCh != nil {
		close(t.attachCh)
		t.attachCh = nil
	}
}

type csiInputFilter struct {
	pending []byte
}

func filteredWrite(ptmx io.Writer, buf []byte) error {
	filter := csiInputFilter{}
	if err := filter.write(ptmx, buf); err != nil {
		return err
	}
	return filter.flush(ptmx)
}

func (f *csiInputFilter) write(ptmx io.Writer, buf []byte) error {
	if len(f.pending) > 0 {
		merged := make([]byte, 0, len(f.pending)+len(buf))
		merged = append(merged, f.pending...)
		merged = append(merged, buf...)
		f.pending = nil
		buf = merged
	}

	for i := 0; i < len(buf); {
		if buf[i] != 0x1b {
			if _, err := ptmx.Write(buf[i : i+1]); err != nil {
				return err
			}
			i++
			continue
		}

		if i+1 >= len(buf) {
			f.pending = append(f.pending[:0], buf[i:]...)
			return nil
		}
		if buf[i+1] != '[' {
			if _, err := ptmx.Write(buf[i : i+1]); err != nil {
				return err
			}
			i++
			continue
		}

		end, drop := filteredCSIEnd(buf[i+2:])
		if end < 0 {
			f.pending = append(f.pending[:0], buf[i:]...)
			return nil
		}
		end += i + 2
		if !drop {
			if _, err := ptmx.Write(buf[i : end+1]); err != nil {
				return err
			}
		}
		i = end + 1
	}
	return nil
}

func (f *csiInputFilter) flushBareEsc(ptmx io.Writer) error {
	if len(f.pending) == 1 && f.pending[0] == 0x1b {
		return f.flush(ptmx)
	}
	return nil
}

func (f *csiInputFilter) flush(ptmx io.Writer) error {
	if len(f.pending) == 0 {
		return nil
	}
	_, err := ptmx.Write(f.pending)
	f.pending = nil
	return err
}

func filteredCSIEnd(seq []byte) (int, bool) {
	if len(seq) == 0 {
		return -1, false
	}
	switch seq[0] {
	case 'I', 'O':
		return 0, true
	case 'M':
		if len(seq) < 4 {
			return -1, false
		}
		return 3, true
	}
	if len(seq) >= 4 && (string(seq[:4]) == "200~" || string(seq[:4]) == "201~") {
		return 3, true
	}
	if seq[0] == '<' {
		for i, b := range seq {
			if b == 'M' || b == 'm' {
				return i, true
			}
			if b >= 0x40 && b <= 0x7e && b != ';' && b != '<' {
				return i, false
			}
		}
		return -1, false
	}
	if seq[0] == '?' {
		for i, b := range seq {
			if b == 'c' {
				return i, true
			}
			if b >= 0x40 && b <= 0x7e && b != ';' && b != '?' {
				return i, false
			}
		}
		return -1, false
	}
	for i, b := range seq {
		if b >= 0x40 && b <= 0x7e {
			return i, false
		}
	}
	return -1, false
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

// SetDetachedSize resizes the tmux pane when no interactive PTY is attached.
// When an active PTY exists (during Attach), it uses pty.Setsize directly.
// When detached, it uses tmux resize-window so the pane stays correctly sized
// for capture-pane previews without needing a background attached client.
func (t *TmuxSession) SetDetachedSize(width, height int) error {
	if t.ptmx != nil {
		return t.updateWindowSize(uint16(width), uint16(height))
	}
	if width > 0 && height > 0 {
		cmd := exec.Command("tmux", "resize-window",
			"-x", fmt.Sprintf("%d", width),
			"-y", fmt.Sprintf("%d", height),
			"-t", t.sanitizedName)
		return t.cmdExec.Run(cmd)
	}
	return nil
}

// updateWindowSize calls pty.Setsize to resize the PTY file descriptor.
// It is a no-op when no PTY is active (ptmx == nil).
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
